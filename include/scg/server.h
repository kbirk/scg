#pragma once

#include <cstdint>
#include <functional>
#include <memory>
#include <mutex>
#include <map>
#include <vector>
#include <queue>
#include <array>

#include "scg/error.h"
#include "scg/serialize.h"
#include "scg/reader.h"
#include "scg/writer.h"
#include "scg/const.h"
#include "scg/context.h"
#include "scg/middleware.h"
#include "scg/transport.h"

namespace scg {

// Simple logger interface for server (doesn't depend on websocketpp)
namespace log {
	class Logger {
	public:
		virtual ~Logger() = default;
		virtual void debug(const std::string& msg) = 0;
		virtual void info(const std::string& msg) = 0;
		virtual void warn(const std::string& msg) = 0;
		virtual void error(const std::string& msg) = 0;
	};
}

namespace rpc {

// Forward declarations
class Server;
class ServerGroup;

// Server connection - wraps a transport connection for tracking purposes
class ServerConnection {
public:
	ServerConnection(std::shared_ptr<Connection> conn, uint64_t id)
		: conn_(conn), id_(id), closed_(false) {}

	uint64_t id() const { return id_; }

	error::Error send(const std::vector<uint8_t>& data) {
		std::lock_guard<std::mutex> lock(mu_);
		if (closed_) {
			return error::Error("Connection is closed");
		}
		return conn_->send(data);
	}

	void close() {
		std::lock_guard<std::mutex> lock(mu_);
		if (!closed_) {
			conn_->close();
			closed_ = true;
		}
	}

	bool isClosed() const {
		std::lock_guard<std::mutex> lock(mu_);
		return closed_;
	}

	std::shared_ptr<Connection> connection() { return conn_; }

private:
	std::shared_ptr<Connection> conn_;
	uint64_t id_;
	bool closed_;
	mutable std::mutex mu_;
};

// Message to be processed by the server
struct PendingMessage {
	std::shared_ptr<ServerConnection> connection;
	std::vector<uint8_t> data;
};

// Handler function type for services
using ServiceHandler = std::function<std::vector<uint8_t>(
	const context::Context& ctx,
	const std::vector<middleware::Middleware>& middleware,
	uint64_t requestID,
	serialize::Reader& reader)>;

// Server configuration
struct ServerConfig {
	std::shared_ptr<ServerTransport> transport;
	std::function<void(const error::Error&)> errorHandler;
	std::shared_ptr<log::Logger> logger;
};

// Server group for organizing services and middleware
class ServerGroup {
public:
	ServerGroup() = default;

	void registerService(uint64_t serviceID, ServiceHandler handler) {
		services_[serviceID] = handler;
	}

	void addMiddleware(middleware::Middleware m) {
		middleware_.push_back(m);
	}

	ServiceHandler getService(uint64_t serviceID) const {
		auto it = services_.find(serviceID);
		if (it != services_.end()) {
			return it->second;
		}
		return nullptr;
	}

	const std::vector<middleware::Middleware>& middleware() const {
		return middleware_;
	}

	void setParent(ServerGroup* parent) {
		parent_ = parent;
	}

	ServerGroup* parent() const {
		return parent_;
	}

	void addChild(ServerGroup* child) {
		children_.push_back(child);
	}

private:
	std::map<uint64_t, ServiceHandler> services_;
	std::vector<middleware::Middleware> middleware_;
	ServerGroup* parent_ = nullptr;
	std::vector<ServerGroup*> children_;
};

// Main server class
class Server {
public:
	Server(const ServerConfig& config)
		: config_(config)
		, transport_(config.transport)
		, running_(false)
		, nextConnectionID_(1)
	{
		rootGroup_ = std::make_unique<ServerGroup>();
		activeGroup_ = rootGroup_.get();
	}

	~Server() {
		stop();
	}

	// Start the server (non-blocking)
	error::Error start() {
		std::lock_guard<std::mutex> lock(mu_);

		if (running_) {
			return error::Error("Server is already running");
		}

		if (!transport_) {
			return error::Error("No transport configured");
		}

		auto err = transport_->listen();
		if (err) {
			return err;
		}

		running_ = true;
		logInfo("Server started");
		return nullptr;
	}

	// Process pending messages and connections (non-blocking, poll-based)
	// Returns true if work was done, false if idle
	bool process() {
		bool didWork = false;

		// Poll the transport for I/O events (reads, writes, accepts)
		// This must be called to process async I/O on connections
		// NOTE: Do NOT hold mu_ while polling - handlers will try to acquire it
		std::shared_ptr<ServerTransport> transport;
		{
			std::lock_guard<std::mutex> lock(mu_);
			if (running_ && transport_) {
				transport = transport_;
			}
		}
		if (transport) {
			transport->poll();
		}

		// Accept new connections
		if (acceptNewConnections()) {
			didWork = true;
		}

		// Process pending messages
		if (processMessages()) {
			didWork = true;
		}

		// Clean up closed connections
		if (cleanupConnections()) {
			didWork = true;
		}

		return didWork;
	}

	// Stop the server
	error::Error stop() {
		std::lock_guard<std::mutex> lock(mu_);

		if (!running_) {
			return nullptr;
		}

		running_ = false;

		// Close all active connections
		for (auto& pair : connections_) {
			pair.second->close();
		}
		connections_.clear();

		// Close the transport
		if (transport_) {
			transport_->close();
		}

		// Clear message queue
		while (!messageQueue_.empty()) {
			messageQueue_.pop();
		}

		logInfo("Server stopped");
		return nullptr;
	}

	// Check if server is running
	bool isRunning() const {
		std::lock_guard<std::mutex> lock(mu_);
		return running_;
	}

	// Register a service with the server
	void registerService(uint64_t serviceID, const std::string& /*serviceName*/, ServiceHandler handler) {
		std::lock_guard<std::mutex> lock(mu_);

		if (groupByServiceID_.find(serviceID) != groupByServiceID_.end()) {
			throw std::runtime_error("Service with id " + std::to_string(serviceID) + " already registered");
		}

		activeGroup_->registerService(serviceID, handler);
		groupByServiceID_[serviceID] = activeGroup_;
	}

	// Add middleware to the current group
	void addMiddleware(middleware::Middleware m) {
		std::lock_guard<std::mutex> lock(mu_);
		activeGroup_->addMiddleware(m);
	}

	// Create a new service group
	void group(std::function<void(Server*)> fn) {
		std::lock_guard<std::mutex> lock(mu_);

		auto newGroup = std::make_unique<ServerGroup>();
		newGroup->setParent(activeGroup_);
		activeGroup_->addChild(newGroup.get());

		auto prevGroup = activeGroup_;
		activeGroup_ = newGroup.get();

		mu_.unlock(); // Unlock before calling user function
		fn(this);
		mu_.lock();

		activeGroup_ = prevGroup;
		ownedGroups_.push_back(std::move(newGroup));
	}

private:
	// Accept new connections (non-blocking)
	bool acceptNewConnections() {
		std::shared_ptr<ServerTransport> transport;
		{
			std::lock_guard<std::mutex> lock(mu_);
			if (!running_ || !transport_) {
				return false;
			}
			transport = transport_;
		}

		bool accepted = false;

		while (true) {
			auto [conn, err] = transport->accept();
			if (err || !conn) {
				break;
			}

			uint64_t connID = 0;
			{
				std::lock_guard<std::mutex> lock(mu_);
				connID = nextConnectionID_++;
			}

			auto serverConn = std::make_shared<ServerConnection>(conn, connID);

			conn->setMessageHandler([this, serverConn](const std::vector<uint8_t>& data) {
				onMessage(serverConn, data);
			});

			conn->setCloseHandler([this, connID]() {
				onConnectionClose(connID);
			});

			conn->setFailHandler([this, connID](const error::Error& err) {
				onConnectionFail(connID, err);
			});

			{
				std::lock_guard<std::mutex> lock(mu_);
				if (!running_) {
					serverConn->close();
					break;
				}
				connections_[connID] = serverConn;
			}

			accepted = true;
			logInfo("New client connected (id: " + std::to_string(connID) + ")");
		}

		return accepted;
	}

	// Process messages from the queue
	bool processMessages() {
		std::unique_lock<std::mutex> lock(mu_);

		if (messageQueue_.empty()) {
			return false;
		}

		// Get next message
		PendingMessage msg = messageQueue_.front();
		messageQueue_.pop();

		// Unlock while processing (long operation)
		lock.unlock();

		handleMessage(msg.connection, msg.data);

		return true;
	}

	// Clean up closed connections
	bool cleanupConnections() {
		std::lock_guard<std::mutex> lock(mu_);

		bool cleaned = false;

		auto it = connections_.begin();
		while (it != connections_.end()) {
			if (it->second->isClosed()) {
				it = connections_.erase(it);
				cleaned = true;
			} else {
				++it;
			}
		}

		return cleaned;
	}

	// Called when a message is received
	void onMessage(std::shared_ptr<ServerConnection> conn, const std::vector<uint8_t>& data) {
		std::lock_guard<std::mutex> lock(mu_);

		if (!running_) {
			return;
		}

		// Queue the message for processing
		messageQueue_.push(PendingMessage{conn, data});
	}

	// Called when a connection closes
	void onConnectionClose(uint64_t connID) {
		std::lock_guard<std::mutex> lock(mu_);

		auto it = connections_.find(connID);
		if (it != connections_.end()) {
			it->second->close();
			logInfo("Client disconnected (id: " + std::to_string(connID) + ")");
		}
	}

	// Called when a connection fails
	void onConnectionFail(uint64_t connID, const error::Error& err) {
		std::lock_guard<std::mutex> lock(mu_);

		handleError(err);

		auto it = connections_.find(connID);
		if (it != connections_.end()) {
			it->second->close();
		}
	}

	// Handle a single message
	void handleMessage(std::shared_ptr<ServerConnection> conn, const std::vector<uint8_t>& data) {
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

			// Get service handler and middleware
			ServiceHandler handler;
			std::vector<middleware::Middleware> middlewareStack;
			{
				std::lock_guard<std::mutex> lock(mu_);
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
	ServiceHandler getService(uint64_t serviceID) const {
		auto it = groupByServiceID_.find(serviceID);
		if (it != groupByServiceID_.end()) {
			return it->second->getService(serviceID);
		}
		return nullptr;
	}

	// Get middleware stack for a service
	std::vector<middleware::Middleware> getMiddlewareStack(uint64_t serviceID) const {
		auto it = groupByServiceID_.find(serviceID);
		if (it == groupByServiceID_.end()) {
			return {};
		}

		// Build middleware stack from root to leaf
		std::vector<ServerGroup*> groups;
		ServerGroup* group = it->second;
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
	std::vector<uint8_t> respondWithError(uint64_t requestID, const error::Error& err) {
		using scg::serialize::bit_size; // ADL trickery

		std::string errMsg = err ? err.message : "Unknown error";

		size_t bitSize = bit_size(RESPONSE_PREFIX) +
						 bit_size(requestID) +
						 bit_size(ERROR_RESPONSE) +
						 bit_size(errMsg);

		serialize::FixedSizeWriter writer(serialize::bits_to_bytes(bitSize));
		writer.write(RESPONSE_PREFIX);
		writer.write(requestID);
		writer.write(ERROR_RESPONSE);
		writer.write(errMsg);

		return writer.bytes();
	}

	// Error handling
	void handleError(const error::Error& err) {
		if (err.message == "connection closed") {
			// Normal connection close, don't log as error
			return;
		}

		logError("Error: " + err.message);

		if (config_.errorHandler) {
			config_.errorHandler(err);
		}
	}

	// Logging helpers
	void logDebug(const std::string& msg) {
		if (config_.logger) {
			config_.logger->debug(msg);
		}
	}

	void logInfo(const std::string& msg) {
		if (config_.logger) {
			config_.logger->info(msg);
		}
	}

	void logWarn(const std::string& msg) {
		if (config_.logger) {
			config_.logger->warn(msg);
		}
	}

	void logError(const std::string& msg) {
		if (config_.logger) {
			config_.logger->error(msg);
		}
	}

	ServerConfig config_;
	std::shared_ptr<ServerTransport> transport_;

	std::unique_ptr<ServerGroup> rootGroup_;
	ServerGroup* activeGroup_;
	std::map<uint64_t, ServerGroup*> groupByServiceID_;
	std::vector<std::unique_ptr<ServerGroup>> ownedGroups_;

	bool running_;
	std::map<uint64_t, std::shared_ptr<ServerConnection>> connections_;
	uint64_t nextConnectionID_;

	std::queue<PendingMessage> messageQueue_;

	mutable std::mutex mu_;
};

// Helper function to create an error response
inline std::vector<uint8_t> respondWithError(uint64_t requestID, const error::Error& err) {
	using scg::serialize::bit_size; // ADL trickery

	std::string errMsg = err ? err.message : "Unknown error";

	size_t bitSize = bit_size(RESPONSE_PREFIX) +
					 bit_size(requestID) +
					 bit_size(ERROR_RESPONSE) +
					 bit_size(errMsg);

	serialize::FixedSizeWriter writer(serialize::bits_to_bytes(bitSize));
	writer.write(RESPONSE_PREFIX);
	writer.write(requestID);
	writer.write(ERROR_RESPONSE);
	writer.write(errMsg);

	return writer.bytes();
}

// Helper function to create a message response
template<typename T>
std::vector<uint8_t> respondWithMessage(uint64_t requestID, const T& msg) {
	using scg::serialize::bit_size; // ADL trickery

	size_t bitSize = bit_size(RESPONSE_PREFIX) +
					 bit_size(requestID) +
					 bit_size(MESSAGE_RESPONSE) +
					 bit_size(msg);

	serialize::FixedSizeWriter writer(serialize::bits_to_bytes(bitSize));
	writer.write(RESPONSE_PREFIX);
	writer.write(requestID);
	writer.write(MESSAGE_RESPONSE);
	writer.write(msg);

	return writer.bytes();
}

} // namespace rpc
} // namespace scg
