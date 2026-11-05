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
#include <chrono>
#include <atomic>
#include <exception>
#include <stdexcept>
#include <string>
#include <system_error>

#ifndef ASIO_STANDALONE
#define ASIO_STANDALONE
#endif
#include <asio/post.hpp>
#include <asio/thread_pool.hpp>
#ifndef SCG_MINIMAL_INCLUDES
// Kept for source compatibility with code relying on the transitive include;
// define SCG_MINIMAL_INCLUDES to skip it (saves ~0.5 s per translation unit).
#include <asio.hpp>
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
	// keepaliveInterval, if > 0, enables server-initiated connection keepalive: a
	// PING is sent after this much idle time. keepaliveTimeout is the max idle
	// time before a connection is declared dead and closed (defaults to
	// 2*keepaliveInterval; a value not greater than keepaliveInterval is raised
	// to 2*keepaliveInterval, as in the Go server). This
	// detects a client that vanished without a clean close (e.g. a dropped mobile
	// connection) and prevents leaked stream-handler threads and connection
	// objects.
	std::chrono::milliseconds keepaliveInterval{0};
	std::chrono::milliseconds keepaliveTimeout{0};
	// maxPendingRequests / maxPendingRequestBytes bound the unary requests that
	// are queued or running on the worker pool (0 = unlimited). A request over
	// either bound is answered immediately with a "server overloaded" error
	// instead of queueing without limit. The defaults are far above normal
	// concurrency; the byte bound is 8x the default maximum frame size.
	size_t maxPendingRequests = 10000;
	size_t maxPendingRequestBytes = size_t(256) << 20;
	// workerThreads sizes the unary worker pool (0 = hardware concurrency, at
	// least 1).
	size_t workerThreads = 0;
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
		, threadPool_(workerCount(config))
	{
		rootGroup_ = std::make_shared<ServerGroup>();
		activeGroup_ = rootGroup_;
	}

	~Server()
	{
		shutdown();
		// Every handler has finished; join the (now idle) worker threads before
		// any member they could touch is destroyed.
		threadPool_.join();
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

		// Start the server-initiated keepalive scanner if configured.
		if (config_.keepaliveInterval.count() > 0) {
			keepaliveThread_ = std::thread([this]() {
				keepaliveLoop();
			});
		}

		return nullptr;
	}

	// shutdown stops the server:
	//  1. stops accepting and closes every connection (peers see the close
	//     promptly, so their in-flight calls fail fast instead of hanging);
	//  2. fails every live stream and waits for the stream handlers to return;
	//  3. waits for unary handlers that already started to return (their
	//     responses can no longer be delivered). Requests still queued on the
	//     worker pool never reach their handler: they are answered with a
	//     "server shutting down" error.
	// When it returns no handler is running, so service implementations may be
	// destroyed. Calling it from inside a handler is supported: that handler is
	// not waited for, and if another shutdown is already in progress the call
	// returns immediately instead of waiting (which could deadlock, since the
	// other shutdown waits for this handler).
	error::Error shutdown()
	{
		std::unique_lock<std::mutex> shutdownLock(shutdownMu_, std::defer_lock);
		if (currentHandlerKind() != HandlerKind::None) {
			if (!shutdownLock.try_lock()) {
				// Another shutdown is in progress and waits for this handler.
				// Make sure no new work starts, then let the handler finish.
				std::lock_guard<std::mutex> lock(keepaliveMu_);
				running_ = false;
				return nullptr;
			}
		} else {
			shutdownLock.lock();
		}

		bool wasRunning = false;
		{
			std::lock_guard<std::mutex> lock(keepaliveMu_);
			wasRunning = running_.exchange(false);
		}
		keepaliveCv_.notify_all();
		joinUnlessSelf(keepaliveThread_);

		// Stop the transport: closes the listener and every connection.
		if (wasRunning && transport_) {
			transport_->stop();
		}
		joinUnlessSelf(transportThread_);

		// Fail all live streams so their handler threads unblock and exit, then
		// wait for every (detached) handler to finish before tearing down state.
		std::vector<std::shared_ptr<ServerStream>> streams;
		{
			std::lock_guard<std::mutex> lock(mu_);
			for (auto& cp : connStreams_) {
				for (auto& sp : cp.second) {
					streams.push_back(sp.second);
				}
			}
			connStreams_.clear();
		}
		for (auto& stream : streams) {
			stream->die(error::Error("server shutting down"));
		}
		streams.clear();

		{
			std::unique_lock<std::mutex> lock(mu_);
			int selfStream = currentHandlerKind() == HandlerKind::Stream ? 1 : 0;
			size_t selfUnary = currentHandlerKind() == HandlerKind::Unary ? 1 : 0;
			handlersDone_.wait(lock, [this, selfStream, selfUnary]() {
				return activeStreamHandlers_ <= selfStream && runningUnary_ <= selfUnary;
			});
			connections_.clear();
		}

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
		(void)serviceName; // informational
		std::lock_guard<std::mutex> lock(mu_);

		if (groupByServiceID_.find(serviceID) != groupByServiceID_.end()) {
			throw std::runtime_error("Service with id " + std::to_string(serviceID) + " already registered");
		}

		if (activeGroup_) {
			activeGroup_->registerService(serviceID, handler);
			groupByServiceID_[serviceID] = activeGroup_;
		}
		registry_.reset();
	}

	// Register a streaming handler for a service. The service must also be
	// registered via registerService (for group/middleware lookup).
	void registerStreamService(uint64_t serviceID, StreamHandler handler)
	{
		std::lock_guard<std::mutex> lock(mu_);
		streamServices_[serviceID] = handler;
		registry_.reset();
	}

	// Add middleware to the current group
	void addMiddleware(middleware::Middleware m)
	{
		std::lock_guard<std::mutex> lock(mu_);
		if (activeGroup_) {
			activeGroup_->addMiddleware(m);
		}
		registry_.reset();
	}

	// Create a new service group. Registrations made inside fn belong to it;
	// the previous group is restored afterwards even if fn throws.
	void group(std::function<void(Server*)> fn)
	{
		std::shared_ptr<ServerGroup> newGroup;
		std::shared_ptr<ServerGroup> prevGroup;
		{
			std::lock_guard<std::mutex> lock(mu_);
			newGroup = std::make_shared<ServerGroup>();
			newGroup->setParent(activeGroup_);
			if (activeGroup_) {
				activeGroup_->addChild(newGroup);
			}
			prevGroup = activeGroup_;
			activeGroup_ = newGroup;
			ownedGroups_.push_back(newGroup);
		}

		struct Restore {
			Server* server;
			std::shared_ptr<ServerGroup> prev;
			~Restore()
			{
				std::lock_guard<std::mutex> lock(server->mu_);
				server->activeGroup_ = prev;
			}
		} restore{this, prevGroup};

		fn(this); // called without mu_ held: it registers services
	}

private:
	enum class HandlerKind { None, Unary, Stream };

	// ServiceEntry is a snapshot of everything needed to dispatch one service:
	// its handlers and its fully resolved (root-to-leaf) middleware chain.
	struct ServiceEntry {
		ServiceHandler handler;
		StreamHandler streamHandler;
		std::vector<middleware::Middleware> middleware;
	};
	using Registry = std::map<uint64_t, ServiceEntry>;

	// ConnState is the per-connection state. Liveness is an atomic so the hot
	// inbound-frame path records it without a lock.
	struct ConnState {
		std::shared_ptr<Connection> conn;
		std::shared_ptr<std::atomic<int64_t>> lastActivityNs;
	};

	static size_t workerCount(const ServerConfig& config)
	{
		if (config.workerThreads > 0) {
			return config.workerThreads;
		}
		unsigned hc = std::thread::hardware_concurrency();
		return hc > 0 ? hc : 1;
	}

	static int64_t steadyNowNs()
	{
		return std::chrono::duration_cast<std::chrono::nanoseconds>(
			std::chrono::steady_clock::now().time_since_epoch()).count();
	}

	static HandlerKind& handlerKindSlot()
	{
		static thread_local HandlerKind kind = HandlerKind::None;
		return kind;
	}

	static HandlerKind currentHandlerKind()
	{
		return handlerKindSlot();
	}

	struct HandlerScope {
		explicit HandlerScope(HandlerKind kind) : prev(handlerKindSlot()) { handlerKindSlot() = kind; }
		~HandlerScope() { handlerKindSlot() = prev; }
		HandlerKind prev;
	};

	static void joinUnlessSelf(std::thread& t)
	{
		if (t.joinable() && t.get_id() != std::this_thread::get_id()) {
			t.join();
		}
	}

	// registrySnapshot returns the current dispatch snapshot, rebuilding it after
	// a registration change. Caller holds mu_.
	std::shared_ptr<const Registry> registrySnapshotUnsafe()
	{
		if (!registry_) {
			auto reg = std::make_shared<Registry>();
			for (auto& pair : groupByServiceID_) {
				ServiceEntry entry;
				entry.handler = pair.second->getService(pair.first);
				entry.middleware = getMiddlewareStack(pair.first);
				(*reg)[pair.first] = std::move(entry);
			}
			for (auto& pair : streamServices_) {
				auto& entry = (*reg)[pair.first];
				entry.streamHandler = pair.second;
				if (groupByServiceID_.find(pair.first) != groupByServiceID_.end()) {
					entry.middleware = getMiddlewareStack(pair.first);
				}
			}
			registry_ = reg;
		}
		return registry_;
	}

	std::shared_ptr<const Registry> registrySnapshot()
	{
		std::lock_guard<std::mutex> lock(mu_);
		return registrySnapshotUnsafe();
	}

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

		// The previous run's threads must be gone before restarting.
		joinUnlessSelf(transportThread_);
		joinUnlessSelf(keepaliveThread_);

		registrySnapshotUnsafe();

		transport_->setOnConnection([this](std::shared_ptr<Connection> conn) {
			handleNewConnection(conn);
		});

		auto err = transport_->startListening();
		if (err) {
			return err;
		}

		{
			std::lock_guard<std::mutex> kaLock(keepaliveMu_);
			running_ = true;
		}
		return nullptr;
	}

	// Handle new connection (on the transport I/O thread)
	void handleNewConnection(std::shared_ptr<Connection> conn)
	{
		if (!running_) {
			conn->close();
			return;
		}

		uint64_t connID = nextConnectionID_++;
		auto activity = std::make_shared<std::atomic<int64_t>>(steadyNowNs());

		// Store the connection first
		{
			std::lock_guard<std::mutex> lock(mu_);
			connections_[connID] = ConnState{conn, activity};
		}

		conn->setCloseHandler([this, connID]() {
			onConnectionClose(connID);
		});

		conn->setFailHandler([this, connID](const error::Error& err) {
			onConnectionFail(connID, err);
		});

		// Installed last: this starts the read loop. Stream frames are routed
		// inline on the I/O thread to preserve per-stream order; unary requests
		// are dispatched to the worker pool so the single io thread stays free to
		// read more frames and handlers may block without stalling the loop.
		conn->setOwnedMessageHandler([this, connID, activity](std::vector<uint8_t>&& data) {
			if (!running_) {
				return;
			}

			// Record liveness for the keepalive scanner: any inbound frame
			// (including a client's PONG reply) keeps the connection alive.
			activity->store(steadyNowNs(), std::memory_order_relaxed);

			// Nothing may escape into the transport's event loop.
			try {
				onFrame(connID, std::move(data));
			} catch (const std::exception& e) {
				handleError(error::Error(std::string("Error handling frame: ") + e.what()));
			} catch (...) {
				handleError(error::Error("Error handling frame: unknown exception"));
			}
		});
	}

	// onFrame dispatches one inbound frame (on the I/O thread).
	void onFrame(uint64_t connID, std::vector<uint8_t>&& data)
	{
		size_t frameBytes = data.size();
		serialize::Reader reader(std::move(data));
		std::array<uint8_t, 16> prefix;
		if (serialize::deserialize(prefix, reader)) {
			return;
		}

		if (prefix == STREAM_PREFIX) {
			handleStreamFrame(connID, reader);
			return;
		}

		if (prefix != REQUEST_PREFIX) {
			handleError(error::Error("Unexpected prefix"));
			return;
		}

		if (!admitUnary(frameBytes)) {
			rejectOverloaded(connID, reader);
			return;
		}
		try {
			asio::post(threadPool_, [this, connID, frameBytes, reader = std::move(reader)]() mutable {
				handleMessage(connID, reader, frameBytes);
			});
		} catch (...) {
			releaseUnary(frameBytes);
			throw;
		}
	}

	// admitUnary reserves a pending-request slot (queued or running); false
	// means overloaded.
	bool admitUnary(size_t bytes)
	{
		std::lock_guard<std::mutex> lock(mu_);
		if (pendingUnary_ > 0) {
			if (config_.maxPendingRequests > 0 && pendingUnary_ >= config_.maxPendingRequests) {
				return false;
			}
			if (config_.maxPendingRequestBytes > 0 && pendingUnaryBytes_ + bytes > config_.maxPendingRequestBytes) {
				return false;
			}
		}
		pendingUnary_++;
		pendingUnaryBytes_ += bytes;
		return true;
	}

	void releaseUnary(size_t bytes)
	{
		std::lock_guard<std::mutex> lock(mu_);
		pendingUnary_--;
		pendingUnaryBytes_ -= bytes;
	}

	// beginUnary marks a dequeued request as started, unless the server is
	// shutting down (then the request must not reach its handler). Checked under
	// mu_, which shutdown's wait also uses, so a request either starts before
	// shutdown waits for it or never starts.
	bool beginUnary()
	{
		std::lock_guard<std::mutex> lock(mu_);
		if (!running_) {
			return false;
		}
		runningUnary_++;
		return true;
	}

	void endUnary()
	{
		std::lock_guard<std::mutex> lock(mu_);
		runningUnary_--;
		handlersDone_.notify_all();
	}

	// rejectOverloaded answers a request that was not admitted (I/O thread).
	void rejectOverloaded(uint64_t connID, serialize::Reader& reader)
	{
		context::Context ctx;
		uint64_t requestID = 0;
		if (deserialize(ctx, reader) || serialize::deserialize(requestID, reader)) {
			return;
		}
		auto conn = connectionFor(connID);
		if (conn) {
			conn->sendOwned(respondWithError(requestID, error::Error("server overloaded")));
		}
		handleError(error::Error("server overloaded: rejected unary request"));
	}

	std::shared_ptr<Connection> connectionFor(uint64_t connID)
	{
		std::lock_guard<std::mutex> lock(mu_);
		auto it = connections_.find(connID);
		if (it == connections_.end()) {
			return nullptr;
		}
		return it->second.conn;
	}

	// Called when a connection closes
	void onConnectionClose(uint64_t connID)
	{
		failConnStreams(connID, error::Error("connection closed"));

		std::lock_guard<std::mutex> lock(mu_);
		connections_.erase(connID);
	}

	// Called when a connection fails
	void onConnectionFail(uint64_t connID, const error::Error& err)
	{
		failConnStreams(connID, error::Error("connection failed: " + err.message()));

		{
			std::lock_guard<std::mutex> lock(mu_);
			connections_.erase(connID);
		}

		// User callback: never invoked with mu_ held.
		handleError(err);
	}

	// keepaliveLoop periodically scans connections: it PINGs an idle connection
	// and closes one that has been silent past the timeout. The peer (Go or C++
	// client) auto-replies PONG, which records activity. Closing the connection
	// triggers its close handler, which fails the connection's streams so their
	// handler threads unblock and exit — preventing the dead-client leak. Mirrors
	// the Go server keepalive and the C++ client keepalive thread.
	void keepaliveLoop()
	{
		auto interval = config_.keepaliveInterval;
		// As in the Go server: a timeout not greater than the interval (or
		// unset) is raised to twice the interval.
		auto timeout = config_.keepaliveTimeout > interval
			? config_.keepaliveTimeout
			: interval * 2;
		int64_t intervalNs = std::chrono::duration_cast<std::chrono::nanoseconds>(interval).count();
		int64_t timeoutNs = std::chrono::duration_cast<std::chrono::nanoseconds>(timeout).count();

		while (true) {
			{
				std::unique_lock<std::mutex> lock(keepaliveMu_);
				keepaliveCv_.wait_for(lock, interval, [this]() { return !running_; });
				if (!running_) {
					return;
				}
			}

			// Snapshot the connections, then act — send/close outside any lock
			// (close() re-enters via the connection's close handler).
			std::vector<ConnState> conns;
			{
				std::lock_guard<std::mutex> lock(mu_);
				for (auto& pair : connections_) {
					conns.push_back(pair.second);
				}
			}

			int64_t now = steadyNowNs();
			for (auto& state : conns) {
				int64_t idle = now - state.lastActivityNs->load(std::memory_order_relaxed);
				if (idle > timeoutNs) {
					state.conn->close();
				} else if (idle >= intervalNs / 2) { // see Client::keepaliveLoop
					state.conn->sendOwned(serializeStreamControl(STREAM_FRAME_PING));
				}
			}
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
			auto conn = connectionFor(connID);
			if (conn) {
				conn->sendOwned(serializeStreamControl(STREAM_FRAME_PONG));
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

			auto conn = connectionFor(connID);
			if (!conn) {
				return;
			}

			auto stream = std::make_shared<ServerStream>(conn, ctx, streamID, config_.streamRecvBufferSize);
			std::string rejectReason;
			{
				std::lock_guard<std::mutex> lock(mu_);
				if (!running_) {
					rejectReason = "server shutting down";
				} else {
					auto& connMap = connStreams_[connID];
					if (connMap.count(streamID) != 0) {
						rejectReason = "duplicate stream id";
					} else if (config_.maxConcurrentStreams > 0 && connMap.size() >= config_.maxConcurrentStreams) {
						rejectReason = "max concurrent streams exceeded";
					} else {
						connMap[streamID] = stream;
						activeStreamHandlers_++;
					}
				}
			}
			if (!rejectReason.empty()) {
				conn->sendOwned(serializeStreamClose(streamID, STREAM_STATUS_ERROR, rejectReason));
				return;
			}
			try {
				std::thread([this, connID, stream, serviceID, methodID]() {
					runStreamHandler(connID, stream, serviceID, methodID);
				}).detach();
			} catch (const std::exception& e) {
				// Thread creation failed (e.g. thread limit reached): roll back
				// and reject the stream instead of terminating the process.
				{
					std::lock_guard<std::mutex> lock(mu_);
					auto it = connStreams_.find(connID);
					if (it != connStreams_.end()) {
						it->second.erase(streamID);
						if (it->second.empty()) {
							connStreams_.erase(it);
						}
					}
					activeStreamHandlers_--;
					handlersDone_.notify_all();
				}
				stream->die(error::Error("unable to start stream handler"));
				conn->sendOwned(serializeStreamClose(streamID, STREAM_STATUS_ERROR, "unable to start stream handler"));
				handleError(error::Error(std::string("unable to start stream handler thread: ") + e.what()));
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
					auto conn = connectionFor(connID);
					if (conn) {
						conn->sendOwned(serializeStreamClose(streamID, STREAM_STATUS_ERROR, "stream receive buffer overflow"));
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
		{
			HandlerScope scope(HandlerKind::Stream);
			runStreamHandlerImpl(connID, stream, serviceID, methodID);
		}
		stream.reset();

		std::lock_guard<std::mutex> lock(mu_);
		--activeStreamHandlers_;
		handlersDone_.notify_all();
	}

	// runStreamHandlerImpl authorizes and runs a stream handler to completion,
	// then sends the terminal CLOSE frame. Exceptions from middleware or the
	// handler end the stream with an error instead of terminating the process.
	void runStreamHandlerImpl(uint64_t connID, std::shared_ptr<ServerStream> stream, uint64_t serviceID, uint64_t methodID)
	{
		uint64_t streamID = stream->streamID();

		auto conn = connectionFor(connID);
		auto registry = registrySnapshot();
		const ServiceEntry* entry = nullptr;
		auto it = registry->find(serviceID);
		if (it != registry->end()) {
			entry = &it->second;
		}

		auto finish = [&](uint8_t status, const std::string& msg) {
			if (conn) {
				conn->sendOwned(serializeStreamClose(streamID, status, msg));
			}
			removeStream(connID, streamID);
		};

		if (!entry || !entry->streamHandler) {
			finish(STREAM_STATUS_ERROR, "service with id " + std::to_string(serviceID) + " does not support streaming");
			return;
		}

		std::string failure;
		try {
			// Authorize once on OPEN by running the middleware chain with a sentinel
			// request; message-oriented middleware (e.g. auth) gates the stream. The
			// sentinel is an owned shared_ptr passed through as the chain's response,
			// so no const-cast / aliasing is needed (the response is discarded).
			context::Context ctxCopy = stream->context();
			auto sentinel = std::make_shared<EmptyStreamMessage>();
			auto mwResult = scg::middleware::applyHandlerChain(
				ctxCopy, *sentinel, entry->middleware,
				[sentinel](scg::context::Context&, const scg::type::Message&) -> std::pair<std::shared_ptr<scg::type::Message>, scg::error::Error> {
					return std::make_pair(sentinel, nullptr);
				});
			if (mwResult.second) {
				finish(STREAM_STATUS_ERROR, mwResult.second.message());
				return;
			}

			auto err = entry->streamHandler(stream->context(), stream, methodID);
			if (err) {
				finish(STREAM_STATUS_ERROR, err.message());
			} else {
				finish(STREAM_STATUS_OK, "");
			}
			return;
		} catch (const std::exception& e) {
			failure = std::string("stream handler threw: ") + e.what();
		} catch (...) {
			failure = "stream handler threw an unknown exception";
		}
		stream->die(error::Error(failure));
		finish(STREAM_STATUS_ERROR, failure);
		handleError(error::Error(failure));
	}

	StreamHandler getStreamService(uint64_t serviceID) const
	{
		auto it = streamServices_.find(serviceID);
		if (it != streamServices_.end()) {
			return it->second;
		}
		return nullptr;
	}

	// Handle a single unary request (on a worker thread). The reader is
	// positioned after the request prefix.
	void handleMessage(uint64_t connID, serialize::Reader& reader, size_t frameBytes)
	{
		struct Release {
			Server* server;
			size_t bytes;
			~Release() { server->releaseUnary(bytes); }
		} release{this, frameBytes};

		bool started = beginUnary();
		struct Running {
			Server* server;
			bool active;
			~Running()
			{
				if (active) {
					server->endUnary();
				}
			}
		} running{this, started};
		HandlerScope scope(started ? HandlerKind::Unary : HandlerKind::None);

		// Read context using ADL
		context::Context ctx;
		uint64_t requestID = 0;
		uint64_t serviceID = 0;
		if (deserialize(ctx, reader) ||
			serialize::deserialize(requestID, reader) ||
			serialize::deserialize(serviceID, reader)) {
			handleError(error::Error("Malformed request header"));
			return;
		}

		// Hold shared_ptr to keep connection alive even if removed from map
		auto conn = connectionFor(connID);
		if (!conn) {
			return; // Connection no longer exists
		}
		if (!started) {
			// Queued behind a shutdown: answer without running the handler.
			conn->sendOwned(respondWithError(requestID, error::Error("server shutting down")));
			return;
		}
		auto registry = registrySnapshot();

		std::vector<uint8_t> response;
		auto it = registry->find(serviceID);
		if (it == registry->end() || !it->second.handler) {
			response = respondWithError(requestID, error::Error("Service not found"));
		} else {
			try {
				response = it->second.handler(ctx, it->second.middleware, requestID, reader);
			} catch (const std::exception& e) {
				// Answer the caller instead of leaving it waiting forever.
				handleError(error::Error(std::string("Error handling message: ") + e.what()));
				response = respondWithError(requestID, error::Error(std::string("internal server error: ") + e.what()));
			} catch (...) {
				handleError(error::Error("Error handling message: unknown exception"));
				response = respondWithError(requestID, error::Error("internal server error"));
			}
		}

		sendResponse(conn, requestID, std::move(response));
	}

	// sendResponse delivers a unary response. If it cannot be sent (e.g. it
	// exceeds the transport's send limit) the caller gets an error response
	// instead; if even that fails the connection is closed so the caller's
	// pending request fails rather than hanging.
	void sendResponse(const std::shared_ptr<Connection>& conn, uint64_t requestID, std::vector<uint8_t>&& response)
	{
		auto err = conn->sendOwned(std::move(response));
		if (!err) {
			return;
		}
		handleError(error::Error("failed to send response: " + err.message()));
		auto fallback = conn->sendOwned(respondWithError(requestID, error::Error("failed to send response: " + err.message())));
		if (fallback) {
			conn->close();
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

		uint32_t bitSize =
			bit_size(RESPONSE_PREFIX) +
			bit_size(requestID) +
			bit_size(ERROR_RESPONSE) +
			bit_size(errMsg);

		serialize::Writer writer(serialize::bits_to_bytes(bitSize));
		writer.write(RESPONSE_PREFIX);
		writer.write(requestID);
		writer.write(ERROR_RESPONSE);
		writer.write(errMsg);

		return std::move(writer).take();
	}

	// Error handling. Never call with mu_ held: it runs the user's callback.
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
	std::shared_ptr<const Registry> registry_; // rebuilt lazily after changes

	std::atomic<bool> running_;
	std::map<uint64_t, ConnState> connections_;
	std::atomic<uint64_t> nextConnectionID_;

	// Streaming state. Handler threads are detached and tracked by a counter so
	// finished handlers free their thread resources immediately (rather than
	// accumulating until shutdown); shutdown waits for the count to reach zero.
	std::map<uint64_t, StreamHandler> streamServices_;
	std::map<uint64_t, std::map<uint64_t, std::shared_ptr<ServerStream>>> connStreams_;
	int activeStreamHandlers_ = 0;
	// Unary requests admitted to the worker pool (queued or running), for the
	// pending-request bounds, and those whose handler has started, which
	// shutdown waits for.
	size_t pendingUnary_ = 0;
	size_t pendingUnaryBytes_ = 0;
	size_t runningUnary_ = 0;
	std::condition_variable handlersDone_;

	std::thread transportThread_;
	mutable std::mutex mu_;
	std::mutex shutdownMu_;

	// Server-initiated keepalive: a scanner thread PINGs idle connections and
	// closes silent (dead) ones.
	std::thread keepaliveThread_;
	std::mutex keepaliveMu_;
	std::condition_variable keepaliveCv_;

	// Declared last so it is destroyed first; ~Server joins it explicitly
	// before any other member goes away.
	asio::thread_pool threadPool_;
};

// Helper function to create an error response
inline std::vector<uint8_t> respondWithError(uint64_t requestID, const error::Error& err)
{
	using scg::serialize::bit_size; // ADL trickery

	std::string errMsg = err ? err.message() : "Unknown error";

	uint32_t bitSize =
		bit_size(RESPONSE_PREFIX) +
		bit_size(requestID) +
		bit_size(ERROR_RESPONSE) +
		bit_size(errMsg);

	serialize::Writer writer(serialize::bits_to_bytes(bitSize));
	writer.write(RESPONSE_PREFIX);
	writer.write(requestID);
	writer.write(ERROR_RESPONSE);
	writer.write(errMsg);

	return std::move(writer).take();
}

// Helper function to create a message response
template<typename T>
std::vector<uint8_t> respondWithMessage(uint64_t requestID, const T& msg)
{
	using scg::serialize::bit_size; // ADL trickery

	uint32_t bitSize =
		bit_size(RESPONSE_PREFIX) +
		bit_size(requestID) +
		bit_size(MESSAGE_RESPONSE) +
		bit_size(msg);

	serialize::Writer writer(serialize::bits_to_bytes(bitSize));
	writer.write(RESPONSE_PREFIX);
	writer.write(requestID);
	writer.write(MESSAGE_RESPONSE);
	writer.write(msg);

	return std::move(writer).take();
}

} // namespace rpc
} // namespace scg
