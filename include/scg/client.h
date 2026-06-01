#pragma once

#include <cstdint>
#include <cstring>
#include <functional>
#include <future>
#include <memory>
#include <random>
#include <thread>
#include <mutex>
#include <condition_variable>
#include <atomic>
#include <chrono>
#include <map>
#include <iostream>

#include "scg/error.h"
#include "scg/serialize.h"
#include "scg/reader.h"
#include "scg/writer.h"
#include "scg/const.h"
#include "scg/context.h"
#include "scg/logger.h"
#include "scg/middleware.h"
#include "scg/transport.h"
#include "scg/stream.h"

namespace scg {
namespace rpc {

enum class ConnectionStatus {
	NOT_CONNECTED,
	CONNECTED,
	FAILED
};

struct ClientConfig {
	std::shared_ptr<ClientTransport> transport;
	// streamRecvBufferSize bounds each stream's inbound queue (0 = default).
	size_t streamRecvBufferSize = 0;
	// keepaliveInterval, if > 0, enables connection-level keepalive: a PING is
	// sent after this much idle time. keepaliveTimeout is the max idle time before
	// the connection is declared dead (defaults to 2*keepaliveInterval).
	std::chrono::milliseconds keepaliveInterval{0};
	std::chrono::milliseconds keepaliveTimeout{0};
};

class Client {
public:

	Client(const ClientConfig& config)
		: config_(config)
		, status_(ConnectionStatus::NOT_CONNECTED)
	{
		// randomize the starting request id
		std::random_device rd;
		std::mt19937_64 gen(rd());
		std::uniform_int_distribution<uint64_t> dis;
		requestID_ = dis(gen);
	}

	virtual ~Client()
	{
		// Stop + join the persistent keepalive thread before tearing down the
		// connection/transport it may touch.
		stopKeepalive();
		disconnect();
		if (config_.transport) {
			config_.transport->shutdown();
		}
	}

	error::Error connect()
	{
		std::lock_guard<std::mutex> lock(mu_);

		return connectUnsafe();
	}

	error::Error disconnect()
	{
		// Note: the keepalive thread is intentionally not stopped here — it is
		// persistent and idles while disconnected, then resumes on reconnect. It
		// is stopped/joined only by the destructor.
		std::lock_guard<std::mutex> lock(mu_);

		failPendingRequestsUnsafe("Connection closed");
		failStreamsUnsafe(error::Error("connection closed"));

		return disconnectUnsafe();
	}

	// openStream opens a bidirectional stream against the given service/method.
	// The returned ClientStream is registered with the demux before the OPEN
	// frame is sent, so no inbound frame can be missed.
	std::pair<std::shared_ptr<ClientStream>, error::Error> openStream(const context::Context& ctx, uint64_t serviceID, uint64_t methodID)
	{
		std::lock_guard<std::mutex> lock(mu_);

		auto err = connectUnsafe();
		if (err) {
			return std::make_pair(nullptr, err);
		}

		uint64_t streamID = requestID_++;

		// Capture the connection (a shared_ptr that internally uses
		// shared_from_this for async safety) rather than the Client. This binds
		// the stream's send path to the connection's lifetime instead of the
		// Client's, so the stream can safely outlive the Client, and avoids a
		// Client<->stream reference cycle. The connection is closed when the
		// Client disconnects, after which send() returns an error.
		auto conn = connection_;
		auto stream = std::make_shared<ClientStream>(
			ctx,
			streamID,
			[conn](const std::vector<uint8_t>& bs) -> error::Error {
				return conn->send(bs);
			},
			config_.streamRecvBufferSize);

		streams_[streamID] = stream;

		err = sendBytesUnsafe(serializeStreamOpen(ctx, streamID, serviceID, methodID));
		if (err) {
			streams_.erase(streamID);
			return std::make_pair(nullptr, err);
		}

		return std::make_pair(stream, nullptr);
	}

	template <typename T>
	std::pair<serialize::Reader, error::Error> call(const context::Context& ctx, uint64_t serviceID, uint64_t methodID, const T& msg)
	{
		auto [future, requestID, err] = sendMessage(ctx, serviceID, methodID, msg);
		if (err) {
			return std::make_pair(serialize::Reader({}), err);
		}

		if (ctx.hasDeadline()) {
			auto status = future.wait_until(ctx.getDeadline());
			if (status == std::future_status::timeout) {
				// Remove request from map
				std::lock_guard<std::mutex> lock(mu_);
				requests_.erase(requestID);
				return std::make_pair(serialize::Reader({}), error::Error("Request timed out"));
			}
		}

		return receiveMessage(future);
	}

	const std::vector<scg::middleware::Middleware>& middleware()
	{
		return middleware_;
	}

	void middleware(scg::middleware::Middleware middleware)
	{
		middleware_.push_back(middleware);
	}

protected:

	void failPendingRequestsUnsafe(const std::string& error)
	{
		for (auto& pair : requests_) {
			pair.second->set_value(createErrorReader(error));
		}
		requests_.clear();
	}

	void failStreamsUnsafe(const error::Error& err)
	{
		for (auto& pair : streams_) {
			pair.second->die(err);
		}
		streams_.clear();
	}

	void removeStream(uint64_t streamID)
	{
		std::lock_guard<std::mutex> lock(mu_);
		streams_.erase(streamID);
	}

	// handleStreamFrame routes one inbound stream frame to its ClientStream.
	// Runs on the transport I/O thread (via onMessage), preserving per-stream order.
	void handleStreamFrame(serialize::Reader& reader)
	{
		uint64_t streamID = 0;
		if (serialize::deserialize(streamID, reader)) {
			return;
		}
		uint8_t frameKind = 0;
		if (serialize::deserialize(frameKind, reader)) {
			return;
		}

		// Connection-level keepalive frames are not associated with a stream.
		if (frameKind == STREAM_FRAME_PING) {
			std::shared_ptr<Connection> conn;
			{
				std::lock_guard<std::mutex> lock(mu_);
				conn = connection_;
			}
			if (conn) {
				conn->send(serializeStreamControl(STREAM_FRAME_PONG));
			}
			return;
		}
		if (frameKind == STREAM_FRAME_PONG) {
			return; // liveness already recorded via lastActivity
		}

		std::shared_ptr<ClientStream> stream;
		{
			std::lock_guard<std::mutex> lock(mu_);
			auto it = streams_.find(streamID);
			if (it == streams_.end()) {
				return; // unknown / already-closed stream
			}
			stream = it->second;
		}

		switch (frameKind) {
			case STREAM_FRAME_MESSAGE:
				if (stream->deliver(std::move(reader))) {
					// Bounded buffer overflowed: notify the server and drop the stream.
					{
						std::lock_guard<std::mutex> lock(mu_);
						if (connection_) {
							connection_->send(serializeStreamClose(streamID, STREAM_STATUS_ERROR, "stream receive buffer overflow"));
						}
					}
					removeStream(streamID);
				}
				break;
			case STREAM_FRAME_HALF_CLOSE:
				// Server done sending; recv sees a clean EOF, client may still send.
				stream->closeRecv(nullptr);
				break;
			case STREAM_FRAME_CLOSE: {
				uint8_t status = 0;
				serialize::deserialize(status, reader);
				std::string message;
				serialize::deserialize(message, reader);
				if (status == STREAM_STATUS_OK) {
					stream->die(nullptr);
				} else {
					if (message.empty()) {
						message = "stream closed with error";
					}
					stream->die(error::Error(message));
				}
				removeStream(streamID);
				break;
			}
			default:
				break;
		}
	}

	error::Error connectUnsafe()
	{
		if (status_ != ConnectionStatus::FAILED && status_ != ConnectionStatus::NOT_CONNECTED) {
			return nullptr;
		}

		if (!config_.transport) {
			return error::Error("No transport configured");
		}

		auto result = config_.transport->connect();
		if (result.second) {
			status_ = ConnectionStatus::FAILED;
			return result.second;
		}

		connection_ = result.first;
		status_ = ConnectionStatus::CONNECTED;

		// Each connection gets a generation; a fail/close handler from a connection
		// that has since been replaced (e.g. after a reconnect) must not tear down
		// the streams/requests of the new connection.
		uint64_t gen = ++connectionGeneration_;

		// Set up handlers
		connection_->setFailHandler([this, gen](const error::Error& err) {
			std::lock_guard<std::mutex> lock(mu_);
			if (gen != connectionGeneration_) {
				return; // stale handler from a replaced connection
			}
			status_ = ConnectionStatus::FAILED;
			// Fail all pending requests and live streams
			failPendingRequestsUnsafe("Connection failed: " + err.message());
			failStreamsUnsafe(error::Error("connection failed: " + err.message()));
		});

		connection_->setCloseHandler([this, gen]() {
			std::lock_guard<std::mutex> lock(mu_);
			if (gen != connectionGeneration_) {
				return; // stale handler from a replaced connection
			}
			status_ = ConnectionStatus::NOT_CONNECTED;
			// Fail all pending requests and live streams
			failPendingRequestsUnsafe("Connection closed");
			failStreamsUnsafe(error::Error("connection closed"));
		});

		connection_->setMessageHandler([this](const std::vector<uint8_t>& data) {
			onMessage(data);
		});

		startKeepaliveUnsafe();

		return nullptr;
	}

	error::Error disconnectUnsafe()
	{
		if (connection_) {
			auto err = connection_->close();
			connection_.reset();
			return err;
		}
		return nullptr;
	}

	error::Error sendBytesUnsafe(const std::vector<uint8_t>& msg)
	{
		auto err = connectUnsafe();
		if (err) {
			return err;
		}

		if (status_ == ConnectionStatus::CONNECTED && connection_) {
			return connection_->send(msg);
		}

		return error::Error("Connection not available");
	}

	serialize::Reader createErrorReader(std::string err)
	{
		using scg::serialize::bit_size; // adl trickery

		serialize::Writer writer(
			scg::serialize::bits_to_bytes(
				bit_size(ERROR_RESPONSE) +
				bit_size(err)));

		return serialize::Reader(writer.bytes());
	}

	static int64_t steadyNowNs()
	{
		return std::chrono::duration_cast<std::chrono::nanoseconds>(
			std::chrono::steady_clock::now().time_since_epoch()).count();
	}

	// startKeepaliveUnsafe (caller holds mu_) resets the activity clock on every
	// (re)connect, and starts the keepalive thread the first time. The thread is
	// persistent: it lives for the client's lifetime and adapts to whatever the
	// current connection is on each tick, so it transparently survives reconnects.
	// It is stopped (and joined) only by stopKeepalive().
	void startKeepaliveUnsafe()
	{
		if (config_.keepaliveInterval.count() <= 0) {
			return;
		}
		// Reset on every connect so a stale value from a previous connection
		// can't trigger a false timeout on the new one.
		lastActivityNs_.store(steadyNowNs());
		if (keepaliveRunning_.load()) {
			return; // already running; it will pick up the new connection
		}
		keepaliveRunning_.store(true);
		auto interval = config_.keepaliveInterval;
		auto timeout = config_.keepaliveTimeout.count() > 0 ? config_.keepaliveTimeout : interval * 2;
		keepaliveThread_ = std::thread([this, interval, timeout]() {
			keepaliveLoop(interval, timeout);
		});
	}

	void keepaliveLoop(std::chrono::milliseconds interval, std::chrono::milliseconds timeout)
	{
		int64_t intervalNs = std::chrono::duration_cast<std::chrono::nanoseconds>(interval).count();
		int64_t timeoutNs = std::chrono::duration_cast<std::chrono::nanoseconds>(timeout).count();

		for (;;) {
			{
				std::unique_lock<std::mutex> lock(keepaliveMu_);
				keepaliveCv_.wait_for(lock, interval, [this]() { return !keepaliveRunning_.load(); });
				if (!keepaliveRunning_.load()) {
					return;
				}
			}

			// Snapshot the current connection. Between reconnects this is null and
			// we simply idle; on reconnect we pick up the new connection.
			std::shared_ptr<Connection> conn;
			{
				std::lock_guard<std::mutex> lock(mu_);
				if (status_ == ConnectionStatus::CONNECTED) {
					conn = connection_;
				}
			}
			if (!conn) {
				continue;
			}

			int64_t idleNs = steadyNowNs() - lastActivityNs_.load();
			if (idleNs > timeoutNs) {
				onKeepaliveTimeout(conn);
				continue; // stay alive to keep probing future (re)connections
			}
			if (idleNs >= intervalNs) {
				conn->send(serializeStreamControl(STREAM_FRAME_PING));
			}
		}
	}

	// onKeepaliveTimeout declares the given (timed-out) connection dead. It runs
	// on the keepalive thread, so it never joins/stops that thread.
	void onKeepaliveTimeout(const std::shared_ptr<Connection>& timedOut)
	{
		std::lock_guard<std::mutex> lock(mu_);
		// Only tear down if this is still the live connection (a concurrent
		// reconnect may have already replaced it).
		if (connection_ != timedOut) {
			return;
		}
		failPendingRequestsUnsafe("keepalive timeout");
		failStreamsUnsafe(error::Error("keepalive timeout"));
		status_ = ConnectionStatus::FAILED;
		connection_->close();
		connection_.reset();
	}

	void stopKeepalive()
	{
		{
			std::lock_guard<std::mutex> lock(keepaliveMu_);
			keepaliveRunning_.store(false);
		}
		keepaliveCv_.notify_all();
		if (keepaliveThread_.joinable() && keepaliveThread_.get_id() != std::this_thread::get_id()) {
			keepaliveThread_.join();
		}
	}

	void onMessage(const std::vector<uint8_t>& data)
	{
		lastActivityNs_.store(steadyNowNs());

		serialize::Reader reader(data);

		using scg::serialize::deserialize;

		std::array<uint8_t, 16> prefix;
		auto err = deserialize(prefix, reader);
		if (err) {
			disconnect();
			return;
		}

		if (prefix == STREAM_PREFIX) {
			handleStreamFrame(reader);
			return;
		}

		if (prefix != RESPONSE_PREFIX) {
			// We cannot resolve the promise here as we don't have the request ID
			// We disconnect here to prevent the client from deadlocking
			disconnect();
			return;
		}

		uint64_t requestID = 0;
		err = serialize::deserialize(requestID, reader);
		if (err) {
			// We cannot resolve the promise here as we don't have the request ID
			// We disconnect here to prevent the client from deadlocking
			disconnect();
			return;
		}

		std::lock_guard<std::mutex> lock(mu_);

		auto iter = requests_.find(requestID);
		if (iter != requests_.end()) {
			iter->second->set_value(reader);
		} else {
			// Response for an unknown request ID — this can happen when a context
			// timeout cleaned up the request before the server's response arrived.
			// Just discard the response.
			return;
		}

		requests_.erase(requestID);
	}


	template <typename T>
	std::tuple<std::future<serialize::Reader>, uint64_t, error::Error> sendMessage(const context::Context& ctx, uint64_t serviceID, uint64_t methodID, const T& msg)
	{
		// Get request ID first (single lock for ID + promise registration)
		uint64_t requestID = 0;
		{
			std::lock_guard<std::mutex> lock(mu_);
			requestID = requestID_++;
		}

		using scg::serialize::bit_size; // adl trickery

		serialize::Writer writer(
			scg::serialize::bits_to_bytes(
				bit_size(REQUEST_PREFIX) +
				bit_size(ctx) +
				bit_size(requestID) +
				bit_size(serviceID) +
				bit_size(methodID) +
				bit_size(msg)));

		writer.write(REQUEST_PREFIX);
		writer.write(ctx);
		writer.write(requestID);
		writer.write(serviceID);
		writer.write(methodID);
		writer.write(msg);

		auto promise = std::make_shared<std::promise<serialize::Reader>>();

		std::lock_guard<std::mutex> lock(mu_);

		requests_[requestID] = promise;

		auto err = sendBytesUnsafe(writer.bytes());
		if (err) {
			requests_.erase(requestID);
			return std::make_tuple(std::future<serialize::Reader>(), 0, err);
		}

		return std::make_tuple(promise->get_future(), requestID, nullptr);
	}

	std::pair<serialize::Reader, error::Error> receiveMessage(std::future<serialize::Reader>& future)
	{
		auto reader = future.get();

		uint8_t responseType = 0;
		serialize::deserialize(responseType, reader);

		if (responseType == MESSAGE_RESPONSE) {
			return std::make_pair(reader, nullptr);
		}

		std::string errMsg;
		serialize::deserialize(errMsg, reader);

		if (errMsg == "") {
			errMsg = "Unknown error";
		}
		return std::make_pair(serialize::Reader({}), error::Error(errMsg));
	}

private:
	std::mutex mu_;
	ClientConfig config_;
	std::shared_ptr<Connection> connection_;

	ConnectionStatus status_;

	std::vector<scg::middleware::Middleware> middleware_;

	uint64_t requestID_;
	std::map<uint64_t, std::shared_ptr<std::promise<serialize::Reader>>> requests_;
	std::map<uint64_t, std::shared_ptr<ClientStream>> streams_;

	// Bumped on each (re)connect so stale connection handlers can be ignored.
	uint64_t connectionGeneration_ = 0;

	// Keepalive state.
	std::atomic<int64_t> lastActivityNs_{0};
	std::atomic<bool> keepaliveRunning_{false};
	std::thread keepaliveThread_;
	std::mutex keepaliveMu_;
	std::condition_variable keepaliveCv_;

};

}
}
