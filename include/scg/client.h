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

#ifndef SCG_MINIMAL_INCLUDES
// Kept for source compatibility with code relying on the transitive include.
#include <iostream>
#endif

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
	// the connection is declared dead (defaults to 2*keepaliveInterval; a value
	// not greater than keepaliveInterval is raised to 2*keepaliveInterval, as in
	// the Go client, so a ping always has time to be answered).
	std::chrono::milliseconds keepaliveInterval{0};
	std::chrono::milliseconds keepaliveTimeout{0};
	// connectTimeout bounds an implicit or explicit connect when the call has no
	// deadline (0 = no timeout). A call with a deadline connects within the
	// earlier of that deadline and connectTimeout.
	std::chrono::milliseconds connectTimeout{10000};
};

// Client owns its transport exclusively. The connection callbacks installed when
// connecting (fail/close/message handlers) capture the raw `this` pointer and
// fire on the transport's background I/O thread, so `this` must outlive that
// thread. The destructor guarantees this by stopping/joining the I/O thread
// (via transport->shutdown(), which the destructor calls last) before the Client
// members are torn down — therefore the transport must not be shared with, or
// outlive into, anything else. Streams capture the connection (not the Client),
// and the built-in connections keep their I/O resources alive, so a stream may
// outlive the Client: once the Client is gone its sends fail with an error.
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
		std::unique_lock<std::mutex> lock(mu_);
		return ensureConnectedLocked(lock, deadlineAfter(config_.connectTimeout));
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
		std::unique_lock<std::mutex> lock(mu_);

		auto err = ensureConnectedLocked(lock, connectDeadline(ctx));
		if (err) {
			return std::make_pair(nullptr, err);
		}

		uint64_t streamID = requestID_++;

		// Capture the connection rather than the Client: the stream's send path is
		// bound to the connection's lifetime, so the stream can outlive the
		// Client, and there is no Client<->stream reference cycle. The connection
		// is closed when the Client disconnects, after which send() returns an
		// error.
		auto conn = connection_;
		auto stream = std::make_shared<ClientStream>(
			ctx,
			streamID,
			[conn](std::vector<uint8_t> bs) -> error::Error {
				return conn->sendOwned(std::move(bs));
			},
			config_.streamRecvBufferSize);

		pruneStreamsUnsafe();
		streams_[streamID] = stream;
		lock.unlock();

		err = conn->sendOwned(serializeStreamOpen(ctx, streamID, serviceID, methodID));
		if (err) {
			lock.lock();
			streams_.erase(streamID);
			lock.unlock();
			stream->die(err); // registered nowhere; don't send an abandon CLOSE
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

	// connectDeadline converts a call's (system_clock) deadline into the
	// steady_clock deadline used to bound connecting; without one the configured
	// connectTimeout applies (0 = no timeout). Every conversion saturates, so
	// extreme deadlines (time_point::max(), MSVC's 100 ns system_clock) cannot
	// overflow.
	std::chrono::steady_clock::time_point connectDeadline(const context::Context& ctx) const
	{
		auto fallback = deadlineAfter(config_.connectTimeout);
		if (!ctx.hasDeadline()) {
			return fallback;
		}
		auto sysNow = std::chrono::system_clock::now();
		auto deadline = ctx.getDeadline();
		if (deadline <= sysNow) {
			return std::chrono::steady_clock::now();
		}
		// Positive, so this cannot overflow; ms granularity keeps the later
		// conversion to steady_clock ticks in range.
		auto remaining = std::chrono::duration_cast<std::chrono::milliseconds>(deadline - sysNow);
		auto bounded = remaining.count() <= 0 ? std::chrono::steady_clock::now() : deadlineAfter(remaining);
		return bounded < fallback ? bounded : fallback;
	}

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
			if (auto stream = pair.second.lock()) {
				stream->die(err);
			}
		}
		streams_.clear();
	}

	// pruneStreamsUnsafe drops registry entries whose handles were released.
	void pruneStreamsUnsafe()
	{
		if (streams_.size() < nextStreamPrune_) {
			return;
		}
		for (auto it = streams_.begin(); it != streams_.end();) {
			if (it->second.expired()) {
				it = streams_.erase(it);
			} else {
				++it;
			}
		}
		nextStreamPrune_ = streams_.size() * 2 + 64;
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
				conn->sendOwned(serializeStreamControl(STREAM_FRAME_PONG));
			}
			return;
		}
		if (frameKind == STREAM_FRAME_PONG) {
			return; // liveness already recorded via lastActivity
		}

		std::shared_ptr<ClientStream> stream;
		std::shared_ptr<Connection> conn;
		{
			std::lock_guard<std::mutex> lock(mu_);
			auto it = streams_.find(streamID);
			if (it == streams_.end()) {
				return; // unknown / already-closed stream
			}
			stream = it->second.lock();
			if (!stream) {
				streams_.erase(it); // handle dropped (its destructor cancelled it)
				return;
			}
			conn = connection_;
		}

		switch (frameKind) {
			case STREAM_FRAME_MESSAGE:
				if (stream->deliver(std::move(reader))) {
					// Bounded buffer overflowed: notify the server and drop the stream.
					if (conn) {
						conn->sendOwned(serializeStreamClose(streamID, STREAM_STATUS_ERROR, "stream receive buffer overflow"));
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

	// ensureConnectedLocked connects if needed. It is called with mu_ held but
	// releases it while the transport connects, so other threads (and the I/O
	// thread's callbacks) are not blocked behind a slow connect; concurrent
	// callers wait for the in-flight attempt instead of starting another.
	error::Error ensureConnectedLocked(std::unique_lock<std::mutex>& lock, std::chrono::steady_clock::time_point deadline)
	{
		for (;;) {
			if (status_ == ConnectionStatus::CONNECTED && connection_) {
				return nullptr;
			}
			if (!config_.transport) {
				return error::Error("No transport configured");
			}
			if (connecting_) {
				auto done = [this]() { return !connecting_; };
				if (deadline == std::chrono::steady_clock::time_point::max()) {
					connectCv_.wait(lock, done);
				} else if (!connectCv_.wait_until(lock, deadline, done)) {
					return error::Error("Connection timed out");
				}
				continue;
			}

			// Whatever happens below (including an exception from the
			// transport), the in-flight marker is cleared and waiters are woken
			// with mu_ held again, so later callers retry instead of waiting.
			struct ConnectingGuard {
				Client* client;
				std::unique_lock<std::mutex>& lock;
				~ConnectingGuard()
				{
					if (!lock.owns_lock()) {
						lock.lock();
					}
					client->connecting_ = false;
					client->connectCv_.notify_all();
				}
			};

			std::pair<std::shared_ptr<Connection>, error::Error> result;
			{
				connecting_ = true;
				ConnectingGuard guard{this, lock};
				auto transport = config_.transport;
				lock.unlock();
				// Transports report failures as errors; an exception (e.g.
				// bad_alloc, or a third-party transport) becomes one too.
				try {
					result = transport->connectWithDeadline(deadline);
				} catch (const std::exception& e) {
					result = std::make_pair(nullptr, error::Error(std::string("Connection failed: ") + e.what()));
				} catch (...) {
					result = std::make_pair(nullptr, error::Error("Connection failed: unknown exception"));
				}
			}

			if (result.second) {
				status_ = ConnectionStatus::FAILED;
				return result.second;
			}
			if (!result.first) {
				status_ = ConnectionStatus::FAILED;
				return error::Error("Transport returned no connection");
			}
			installConnectionUnsafe(result.first);
			return nullptr;
		}
	}

	void installConnectionUnsafe(std::shared_ptr<Connection> conn)
	{
		if (connection_) {
			connection_->close();
		}
		connection_ = std::move(conn);
		status_ = ConnectionStatus::CONNECTED;

		// Each connection gets a generation; a fail/close handler from a connection
		// that has since been replaced (e.g. after a reconnect or disconnect) must
		// not tear down the streams/requests of the new connection.
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

		// Installed last: this starts the read loop.
		connection_->setOwnedMessageHandler([this](std::vector<uint8_t>&& data) {
			onMessage(std::move(data));
		});

		startKeepaliveUnsafe();
	}

	// disconnectUnsafe closes the current connection and resets the state so the
	// next call reconnects. Bumping the generation makes the closed connection's
	// late fail/close callbacks no-ops.
	error::Error disconnectUnsafe()
	{
		status_ = ConnectionStatus::NOT_CONNECTED;
		++connectionGeneration_;
		if (connection_) {
			auto err = connection_->close();
			connection_.reset();
			return err;
		}
		return nullptr;
	}

	serialize::Reader createErrorReader(const std::string& err)
	{
		using scg::serialize::bit_size; // adl trickery

		serialize::Writer writer(
			scg::serialize::bits_to_bytes(
				bit_size(ERROR_RESPONSE) +
				bit_size(err)));
		writer.write(ERROR_RESPONSE);
		writer.write(err);

		return serialize::Reader(std::move(writer).take());
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
		// As in the Go client: a timeout not greater than the interval (or
		// unset) is raised to twice the interval.
		auto timeout = config_.keepaliveTimeout > interval ? config_.keepaliveTimeout : interval * 2;
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
			// Ping once idle for half an interval: an idle connection then gets a
			// ping every tick, so idle time stays near one interval. (Pinging
			// only after a full interval lets idle time approach two intervals
			// between pings, and tick jitter then trips a 2x-interval timeout.)
			if (idleNs >= intervalNs / 2) {
				conn->sendOwned(serializeStreamControl(STREAM_FRAME_PING));
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
		disconnectUnsafe();
		status_ = ConnectionStatus::FAILED;
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
		std::vector<uint8_t> copy(data);
		onMessage(std::move(copy));
	}

	void onMessage(std::vector<uint8_t>&& data)
	{
		lastActivityNs_.store(steadyNowNs());

		serialize::Reader reader(std::move(data));

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

		std::shared_ptr<std::promise<serialize::Reader>> promise;
		{
			std::lock_guard<std::mutex> lock(mu_);

			auto iter = requests_.find(requestID);
			if (iter == requests_.end()) {
				// Response for an unknown request ID — this can happen when a context
				// timeout cleaned up the request before the server's response arrived.
				// Just discard the response.
				return;
			}
			promise = std::move(iter->second);
			requests_.erase(iter);
		}
		// Fulfil outside the lock (the waiting caller wakes immediately).
		promise->set_value(std::move(reader));
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
		auto future = promise->get_future();

		std::shared_ptr<Connection> conn;
		{
			std::unique_lock<std::mutex> lock(mu_);
			auto err = ensureConnectedLocked(lock, connectDeadline(ctx));
			if (err) {
				return std::make_tuple(std::future<serialize::Reader>(), 0, err);
			}
			// Register before sending so the response cannot beat the
			// registration.
			requests_[requestID] = promise;
			conn = connection_;
		}

		// Send without holding mu_: the I/O thread needs it to deliver responses.
		auto err = conn->sendOwned(std::move(writer).take());
		if (err) {
			std::lock_guard<std::mutex> lock(mu_);
			requests_.erase(requestID);
			return std::make_tuple(std::future<serialize::Reader>(), 0, err);
		}

		return std::make_tuple(std::move(future), requestID, nullptr);
	}

	std::pair<serialize::Reader, error::Error> receiveMessage(std::future<serialize::Reader>& future)
	{
		auto reader = future.get();

		uint8_t responseType = 0;
		serialize::deserialize(responseType, reader);

		if (responseType == MESSAGE_RESPONSE) {
			return std::make_pair(std::move(reader), nullptr);
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
	std::condition_variable connectCv_;
	bool connecting_ = false;
	ClientConfig config_;
	std::shared_ptr<Connection> connection_;

	ConnectionStatus status_;

	std::vector<scg::middleware::Middleware> middleware_;

	uint64_t requestID_;
	std::map<uint64_t, std::shared_ptr<std::promise<serialize::Reader>>> requests_;
	// Streams are held weakly: dropping the last user handle cancels the stream
	// (see ClientStream::~ClientStream) instead of leaking it here.
	std::map<uint64_t, std::weak_ptr<ClientStream>> streams_;
	size_t nextStreamPrune_ = 64;

	// Bumped on each (re)connect and disconnect so stale connection handlers can
	// be ignored.
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
