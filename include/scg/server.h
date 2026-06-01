#pragma once

#include <cstdint>
#include <functional>
#include <memory>
#include <mutex>
#include <condition_variable>
#include <map>
#include <vector>
#include <array>
#include <thread>

#define ASIO_STANDALONE
#include <asio.hpp>

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

// Message to be processed by the server
struct PendingMessage {
	std::shared_ptr<Connection> connection;
	std::vector<uint8_t> data;
};

// Handler function type for unary services
using ServiceHandler = std::function<std::vector<uint8_t>(
	const context::Context& ctx,
	const std::vector<middleware::Middleware>& middleware,
	uint64_t requestID,
	serialize::Reader& reader)>;

// Handler function type for streaming services. Runs on its own thread and may
// block in stream->recv().
using StreamHandler = std::function<error::Error(
	const context::Context& ctx,
	std::shared_ptr<ServerStream> stream,
	uint64_t methodID)>;

// Zero-size sentinel passed through the middleware chain on stream OPEN so that
// message-oriented middleware (e.g. auth) can gate the stream from its metadata.
struct EmptyStreamMessage : public scg::type::Message {
	std::vector<uint8_t> toJSON() const override { return {}; }
	void fromJSON(const std::vector<uint8_t>&) override {}
	std::vector<uint8_t> toBytes() const override { return {}; }
	scg::error::Error fromBytes(const std::vector<uint8_t>&) override { return nullptr; }
	scg::error::Error fromBytes(const uint8_t*, uint32_t) override { return nullptr; }
};

// Server configuration
struct ServerConfig {
	std::shared_ptr<ServerTransport> transport;
	std::function<void(const error::Error&)> errorHandler;
	// streamRecvBufferSize bounds each stream's inbound queue (0 = default).
	size_t streamRecvBufferSize = 0;
	// maxConcurrentStreams caps live streams per connection (0 = unlimited).
	size_t maxConcurrentStreams = 0;
};

// Server group for organizing services and middleware
class ServerGroup {
public:
	ServerGroup() = default;

	void registerService(uint64_t serviceID, ServiceHandler handler)
	{
		services_[serviceID] = handler;
	}

	void addMiddleware(middleware::Middleware m)
	{
		middleware_.push_back(m);
	}

	ServiceHandler getService(uint64_t serviceID) const
	{
		auto it = services_.find(serviceID);
		if (it != services_.end()) {
			return it->second;
		}
		return nullptr;
	}

	const std::vector<middleware::Middleware>& middleware() const
	{
		return middleware_;
	}

	void setParent(std::shared_ptr<ServerGroup> parent)
	{
		parent_ = parent;
	}

	std::shared_ptr<ServerGroup> parent() const
	{
		return parent_.lock();
	}

	void addChild(std::shared_ptr<ServerGroup> child)
	{
		children_.push_back(child);
	}

private:
	std::map<uint64_t, ServiceHandler> services_;
	std::vector<middleware::Middleware> middleware_;
	std::weak_ptr<ServerGroup> parent_;
	std::vector<std::shared_ptr<ServerGroup>> children_;
};

// Main server class
class Server {
public:
	Server(const ServerConfig& config)
		: config_(config)
		, transport_(config.transport)
		, running_(false)
		, nextConnectionID_(1)
		, threadPool_(std::thread::hardware_concurrency())
	{
		rootGroup_ = std::make_shared<ServerGroup>();
		activeGroup_ = rootGroup_;
	}

	~Server()
	{
		shutdown();
	}

	// Start the server in a background thread (non-blocking)
	error::Error start()
	{
		auto err = initialize();
		if (err) {
			return err;
		}

		// Start transport thread
		transportThread_ = std::thread([this]() {
			transport_->runEventLoop();
		});

		return nullptr;
	}

	// Stop the server and wait for thread to finish
	error::Error shutdown()
	{
		// Check if already stopped
		if (!running_) {
			// Join threads if they're still running
			if (transportThread_.joinable()) {
				transportThread_.join();
			}
			return nullptr;
		}

		// Signal shutdown
		running_ = false;

		// Stop the transport
		if (transport_) {
			transport_->stop();
		}

		// Wait for threads to finish
		if (transportThread_.joinable()) {
			transportThread_.join();
		}

		// Fail all live streams so their handler threads unblock and exit, then
		// wait for every (detached) handler to finish before tearing down state.
		{
			std::unique_lock<std::mutex> lock(mu_);
			for (auto& cp : connStreams_) {
				for (auto& sp : cp.second) {
					sp.second->die(error::Error("server shutting down"));
				}
			}
			connStreams_.clear();
			streamHandlersDone_.wait(lock, [this]() { return activeStreamHandlers_ == 0; });
		}

		// Now clean up (thread is stopped, no more concurrent access)
		std::lock_guard<std::mutex> lock(mu_);

		// Just clear connections - their destructors will call close()
		connections_.clear();

		return nullptr;
	}

	// Check if server is running
	bool isRunning() const
	{
		return running_;
	}

	// Register a service with the server
	void registerService(uint64_t serviceID, const std::string& serviceName, ServiceHandler handler)
	{
		std::lock_guard<std::mutex> lock(mu_);

		if (groupByServiceID_.find(serviceID) != groupByServiceID_.end()) {
			throw std::runtime_error("Service with id " + std::to_string(serviceID) + " already registered");
		}

		if (activeGroup_) {
			activeGroup_->registerService(serviceID, handler);
			groupByServiceID_[serviceID] = activeGroup_;
		}
	}

	// Register a streaming handler for a service. The service must also be
	// registered via registerService (for group/middleware lookup).
	void registerStreamService(uint64_t serviceID, StreamHandler handler)
	{
		std::lock_guard<std::mutex> lock(mu_);
		streamServices_[serviceID] = handler;
	}

	// Add middleware to the current group
	void addMiddleware(middleware::Middleware m)
	{
		std::lock_guard<std::mutex> lock(mu_);
		if (activeGroup_) {
			activeGroup_->addMiddleware(m);
		}
	}

	// Create a new service group
	void group(std::function<void(Server*)> fn)
	{
		std::lock_guard<std::mutex> lock(mu_);

		auto newGroup = std::make_shared<ServerGroup>();
		newGroup->setParent(activeGroup_);
		if (activeGroup_) {
			activeGroup_->addChild(newGroup);
		}

		auto prevGroup = activeGroup_;
		activeGroup_ = newGroup;

		mu_.unlock(); // Unlock before calling user function
		fn(this);
		mu_.lock();

		activeGroup_ = prevGroup;
		ownedGroups_.push_back(newGroup);
	}

private:
	// Start the server (internal helper)
	error::Error initialize()
	{
		std::lock_guard<std::mutex> lock(mu_);

		if (running_) {
			return error::Error("Server is already running");
		}

		if (!transport_) {
			return error::Error("No transport configured");
		}

		transport_->setOnConnection([this](std::shared_ptr<Connection> conn) {
			handleNewConnection(conn);
		});

		auto err = transport_->startListening();
		if (err) {
			return err;
		}

		running_ = true;
		return nullptr;
	}

	// Handle new connection
	void handleNewConnection(std::shared_ptr<Connection> conn)
	{
		if (!running_) {
			return;
		}

		uint64_t connID = nextConnectionID_++;

		// Store the connection first
		{
			std::lock_guard<std::mutex> lock(mu_);
			connections_[connID] = conn;
		}

		// Process messages using thread pool to avoid blocking io_context.
		// Stream frames are routed inline on the I/O thread to preserve per-stream
		// order; unary requests are dispatched to the pool.
		conn->setMessageHandler([this, connID](const std::vector<uint8_t>& data) {
			if (!running_) {
				return;
			}

			serialize::Reader reader(data);
			std::array<uint8_t, 16> prefix;
			if (serialize::deserialize(prefix, reader)) {
				return;
			}

			if (prefix == STREAM_PREFIX) {
				handleStreamFrame(connID, reader);
				return;
			}

			// Submit unary requests to the thread pool to avoid blocking the event loop
			asio::post(threadPool_, [this, connID, data]() {
				handleMessage(connID, data);
			});
		});

		conn->setCloseHandler([this, connID]() {
			onConnectionClose(connID);
		});

		conn->setFailHandler([this, connID](const error::Error& err) {
			onConnectionFail(connID, err);
		});
	}

	// Called when a connection closes
	void onConnectionClose(uint64_t connID)
	{
		failConnStreams(connID, error::Error("connection closed"));

		std::lock_guard<std::mutex> lock(mu_);

		auto it = connections_.find(connID);
		if (it != connections_.end()) {
			connections_.erase(it);
		}
	}

	// Called when a connection fails
	void onConnectionFail(uint64_t connID, const error::Error& err)
	{
		failConnStreams(connID, error::Error("connection failed: " + err.message()));

		std::lock_guard<std::mutex> lock(mu_);

		handleError(err);

		auto it = connections_.find(connID);
		if (it != connections_.end()) {
			connections_.erase(it);
		}
	}

	// Fail all live streams on a connection (disconnect/teardown).
	void failConnStreams(uint64_t connID, const error::Error& err)
	{
		std::map<uint64_t, std::shared_ptr<ServerStream>> streams;
		{
			std::lock_guard<std::mutex> lock(mu_);
			auto it = connStreams_.find(connID);
			if (it != connStreams_.end()) {
				streams = it->second;
				connStreams_.erase(it);
			}
		}
		for (auto& pair : streams) {
			pair.second->die(err);
		}
	}

	void removeStream(uint64_t connID, uint64_t streamID)
	{
		std::lock_guard<std::mutex> lock(mu_);
		auto it = connStreams_.find(connID);
		if (it != connStreams_.end()) {
			it->second.erase(streamID);
			if (it->second.empty()) {
				connStreams_.erase(it);
			}
		}
	}

	// handleStreamFrame routes one inbound stream frame. OPEN spawns a handler
	// thread; MSG/HALF_CLOSE/CLOSE are delivered to the existing stream. Runs on
	// the transport I/O thread.
	void handleStreamFrame(uint64_t connID, serialize::Reader& reader)
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
				auto it = connections_.find(connID);
				if (it != connections_.end()) {
					conn = it->second;
				}
			}
			if (conn) {
				conn->send(serializeStreamControl(STREAM_FRAME_PONG));
			}
			return;
		}
		if (frameKind == STREAM_FRAME_PONG) {
			return;
		}

		if (frameKind == STREAM_FRAME_OPEN) {
			context::Context ctx;
			if (deserialize(ctx, reader)) {
				return;
			}
			uint64_t serviceID = 0;
			if (serialize::deserialize(serviceID, reader)) {
				return;
			}
			uint64_t methodID = 0;
			if (serialize::deserialize(methodID, reader)) {
				return;
			}

			std::shared_ptr<Connection> conn;
			{
				std::lock_guard<std::mutex> lock(mu_);
				auto it = connections_.find(connID);
				if (it == connections_.end()) {
					return;
				}
				conn = it->second;
			}

			auto stream = std::make_shared<ServerStream>(conn, ctx, streamID, config_.streamRecvBufferSize);
			std::string rejectReason;
			bool spawn = false;
			{
				std::lock_guard<std::mutex> lock(mu_);
				if (!running_) {
					return; // shutting down; don't start new handlers
				}
				auto& connMap = connStreams_[connID];
				if (connMap.count(streamID) != 0) {
					rejectReason = "duplicate stream id";
				} else if (config_.maxConcurrentStreams > 0 && connMap.size() >= config_.maxConcurrentStreams) {
					rejectReason = "max concurrent streams exceeded";
				} else {
					connMap[streamID] = stream;
					activeStreamHandlers_++;
					spawn = true;
				}
			}
			if (!rejectReason.empty()) {
				conn->send(serializeStreamClose(streamID, STREAM_STATUS_ERROR, rejectReason));
				return;
			}
			if (spawn) {
				std::thread([this, connID, stream, serviceID, methodID]() {
					runStreamHandler(connID, stream, serviceID, methodID);
				}).detach();
			}
			return;
		}

		std::shared_ptr<ServerStream> stream;
		{
			std::lock_guard<std::mutex> lock(mu_);
			auto cit = connStreams_.find(connID);
			if (cit != connStreams_.end()) {
				auto sit = cit->second.find(streamID);
				if (sit != cit->second.end()) {
					stream = sit->second;
				}
			}
		}
		if (!stream) {
			return;
		}

		switch (frameKind) {
			case STREAM_FRAME_MESSAGE:
				if (stream->deliver(std::move(reader))) {
					// Bounded buffer overflowed: notify the client and drop the stream.
					std::shared_ptr<Connection> conn;
					{
						std::lock_guard<std::mutex> lock(mu_);
						auto it = connections_.find(connID);
						if (it != connections_.end()) {
							conn = it->second;
						}
					}
					if (conn) {
						conn->send(serializeStreamClose(streamID, STREAM_STATUS_ERROR, "stream receive buffer overflow"));
					}
					removeStream(connID, streamID);
				}
				break;
			case STREAM_FRAME_HALF_CLOSE:
				stream->halfClose();
				break;
			case STREAM_FRAME_CLOSE:
				stream->die(error::Error("stream cancelled by client"));
				removeStream(connID, streamID);
				break;
			default:
				break;
		}
	}

	// runStreamHandler runs the handler on a detached per-stream thread and, on
	// every exit path, decrements the active-handler count so shutdown can wait.
	void runStreamHandler(uint64_t connID, std::shared_ptr<ServerStream> stream, uint64_t serviceID, uint64_t methodID)
	{
		runStreamHandlerImpl(connID, stream, serviceID, methodID);

		std::lock_guard<std::mutex> lock(mu_);
		if (--activeStreamHandlers_ == 0) {
			streamHandlersDone_.notify_all();
		}
	}

	// runStreamHandlerImpl authorizes and runs a stream handler to completion,
	// then sends the terminal CLOSE frame.
	void runStreamHandlerImpl(uint64_t connID, std::shared_ptr<ServerStream> stream, uint64_t serviceID, uint64_t methodID)
	{
		uint64_t streamID = stream->streamID();

		std::shared_ptr<Connection> conn;
		StreamHandler handler;
		std::vector<middleware::Middleware> middlewareStack;
		{
			std::lock_guard<std::mutex> lock(mu_);
			auto it = connections_.find(connID);
			if (it != connections_.end()) {
				conn = it->second;
			}
			auto sit = streamServices_.find(serviceID);
			if (sit != streamServices_.end()) {
				handler = sit->second;
			}
			middlewareStack = getMiddlewareStack(serviceID);
		}

		auto finish = [&](uint8_t status, const std::string& msg) {
			if (conn) {
				conn->send(serializeStreamClose(streamID, status, msg));
			}
			removeStream(connID, streamID);
		};

		if (!handler) {
			finish(STREAM_STATUS_ERROR, "service with id " + std::to_string(serviceID) + " does not support streaming");
			return;
		}

		// Authorize once on OPEN by running the middleware chain with a sentinel
		// request; message-oriented middleware (e.g. auth) gates the stream. The
		// sentinel is an owned shared_ptr passed through as the chain's response,
		// so no const-cast / aliasing is needed (the response is discarded).
		context::Context ctxCopy = stream->context();
		auto sentinel = std::make_shared<EmptyStreamMessage>();
		auto mwResult = scg::middleware::applyHandlerChain(
			ctxCopy, *sentinel, middlewareStack,
			[sentinel](scg::context::Context&, const scg::type::Message&) -> std::pair<std::shared_ptr<scg::type::Message>, scg::error::Error> {
				return std::make_pair(sentinel, nullptr);
			});
		if (mwResult.second) {
			finish(STREAM_STATUS_ERROR, mwResult.second.message());
			return;
		}

		auto err = handler(stream->context(), stream, methodID);
		if (err) {
			finish(STREAM_STATUS_ERROR, err.message());
		} else {
			finish(STREAM_STATUS_OK, "");
		}
	}

	StreamHandler getStreamService(uint64_t serviceID) const
	{
		auto it = streamServices_.find(serviceID);
		if (it != streamServices_.end()) {
			return it->second;
		}
		return nullptr;
	}

	// Handle a single message
	void handleMessage(uint64_t connID, const std::vector<uint8_t>& data)
	{
		serialize::Reader reader(data);

		try {
			// Read prefix
			std::array<uint8_t, 16> prefix;
			serialize::deserialize(prefix, reader);

			if (prefix != REQUEST_PREFIX) {
				handleError(error::Error("Unexpected prefix"));
				return;
			}

			// Read context using ADL
			context::Context ctx;
			deserialize(ctx, reader);

			// Read request ID
			uint64_t requestID = 0;
			serialize::deserialize(requestID, reader);

			// Read service ID
			uint64_t serviceID = 0;
			serialize::deserialize(serviceID, reader);

			// Get service handler and middleware and connection
			// Hold shared_ptr to keep connection alive even if removed from map
			ServiceHandler handler;
			std::vector<middleware::Middleware> middlewareStack;
			std::shared_ptr<Connection> conn;
			{
				std::lock_guard<std::mutex> lock(mu_);

				auto it = connections_.find(connID);
				if (it == connections_.end()) {
					return;  // Connection no longer exists
				}
				conn = it->second;  // Copy shared_ptr to keep alive

				handler = getService(serviceID);
				middlewareStack = getMiddlewareStack(serviceID);
			}

			if (!handler) {
				auto response = respondWithError(requestID, error::Error("Service not found"));
				conn->send(response);
				return;
			}

			// Call handler
			auto response = handler(ctx, middlewareStack, requestID, reader);

			// Send response
			conn->send(response);

		} catch (const std::exception& e) {
			handleError(error::Error(std::string("Error handling message: ") + e.what()));
		}
	}

	// Get service handler by ID
	ServiceHandler getService(uint64_t serviceID) const
	{
		auto it = groupByServiceID_.find(serviceID);
		if (it != groupByServiceID_.end()) {
			return it->second->getService(serviceID);
		}
		return nullptr;
	}

	// Get middleware stack for a service
	std::vector<middleware::Middleware> getMiddlewareStack(uint64_t serviceID) const
	{
		auto it = groupByServiceID_.find(serviceID);
		if (it == groupByServiceID_.end()) {
			return {};
		}

		// Build middleware stack from root to leaf
		std::vector<std::shared_ptr<ServerGroup>> groups;
		auto group = it->second;
		while (group) {
			groups.push_back(group);
			group = group->parent();
		}

		// Reverse to get root to leaf order
		std::vector<middleware::Middleware> stack;
		for (auto rit = groups.rbegin(); rit != groups.rend(); ++rit) {
			const auto& mw = (*rit)->middleware();
			stack.insert(stack.end(), mw.begin(), mw.end());
		}

		return stack;
	}

	// Create an error response
	std::vector<uint8_t> respondWithError(uint64_t requestID, const error::Error& err)
	{
		using scg::serialize::bit_size; // ADL trickery

		std::string errMsg = err ? err.message() : "Unknown error";

		size_t bitSize =
			bit_size(RESPONSE_PREFIX) +
			bit_size(requestID) +
			bit_size(ERROR_RESPONSE) +
			bit_size(errMsg);

		serialize::Writer writer(serialize::bits_to_bytes(bitSize));
		writer.write(RESPONSE_PREFIX);
		writer.write(requestID);
		writer.write(ERROR_RESPONSE);
		writer.write(errMsg);

		return writer.bytes();
	}

	// Error handling
	void handleError(const error::Error& err)
	{
		if (err.message() == "connection closed") {
			// Normal connection close, don't log as error
			return;
		}

		if (config_.errorHandler) {
			config_.errorHandler(err);
		}
	}

	ServerConfig config_;
	std::shared_ptr<ServerTransport> transport_;

	std::shared_ptr<ServerGroup> rootGroup_;
	std::shared_ptr<ServerGroup> activeGroup_;
	std::map<uint64_t, std::shared_ptr<ServerGroup>> groupByServiceID_;
	std::vector<std::shared_ptr<ServerGroup>> ownedGroups_;

	std::atomic<bool> running_;
	std::map<uint64_t, std::shared_ptr<Connection>> connections_;
	std::atomic<uint64_t> nextConnectionID_;

	// Streaming state. Handler threads are detached and tracked by a counter so
	// finished handlers free their thread resources immediately (rather than
	// accumulating until shutdown); shutdown waits for the count to reach zero.
	std::map<uint64_t, StreamHandler> streamServices_;
	std::map<uint64_t, std::map<uint64_t, std::shared_ptr<ServerStream>>> connStreams_;
	int activeStreamHandlers_ = 0;
	std::condition_variable streamHandlersDone_;

	asio::thread_pool threadPool_;
	std::thread transportThread_;
	mutable std::mutex mu_;
};

// Helper function to create an error response
inline std::vector<uint8_t> respondWithError(uint64_t requestID, const error::Error& err)
{
	using scg::serialize::bit_size; // ADL trickery

	std::string errMsg = err ? err.message() : "Unknown error";

	size_t bitSize =
		bit_size(RESPONSE_PREFIX) +
		bit_size(requestID) +
		bit_size(ERROR_RESPONSE) +
		bit_size(errMsg);

	serialize::Writer writer(serialize::bits_to_bytes(bitSize));
	writer.write(RESPONSE_PREFIX);
	writer.write(requestID);
	writer.write(ERROR_RESPONSE);
	writer.write(errMsg);

	return writer.bytes();
}

// Helper function to create a message response
template<typename T>
std::vector<uint8_t> respondWithMessage(uint64_t requestID, const T& msg)
{
	using scg::serialize::bit_size; // ADL trickery

	size_t bitSize =
		bit_size(RESPONSE_PREFIX) +
		bit_size(requestID) +
		bit_size(MESSAGE_RESPONSE) +
		bit_size(msg);

	serialize::Writer writer(serialize::bits_to_bytes(bitSize));
	writer.write(RESPONSE_PREFIX);
	writer.write(requestID);
	writer.write(MESSAGE_RESPONSE);
	writer.write(msg);

	return writer.bytes();
}

} // namespace rpc
} // namespace scg
