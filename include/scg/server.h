#pragma once

#include <cstdint>
#include <functional>
#include <memory>
#include <mutex>
#include <map>
#include <vector>
#include <queue>
#include <array>
#include <thread>
#include <atomic>

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
		: conn_(conn)
		, id_(id)
		, closed_(false)
	{

	}

	uint64_t id() const
	{
		return id_;
	}

	error::Error send(const std::vector<uint8_t>& data)
	{
		std::lock_guard<std::mutex> lock(mu_);
		if (closed_) {
			return error::Error("Connection is closed");
		}
		return conn_->send(data);
	}

	void close()
	{
		std::lock_guard<std::mutex> lock(mu_);
		if (!closed_) {
			conn_->close();
			closed_ = true;
		}
	}

	bool isClosed() const
	{
		std::lock_guard<std::mutex> lock(mu_);
		return closed_;
	}

	std::shared_ptr<Connection> connection()
	{
		return conn_;
	}

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
	{
		rootGroup_ = std::make_shared<ServerGroup>();
		activeGroup_ = rootGroup_;
	}

	~Server()
	{
		shutdown();
	}

	// Start the server in a background thread (non-blocking)
	error::Error run()
	{
		auto err = start();
		if (err) {
			return err;
		}

		// Start server thread
		serverThread_ = std::thread([this]() {
			logInfo("Server started in background thread");

			while (running_) {
				// Poll the transport for I/O events
				if (transport_) {
					transport_->poll();
				}

				// Accept new connections
				acceptNewConnections();

				// Process pending messages
				processMessages();

				// Clean up closed connections
				cleanupConnections();

				// Small sleep to avoid busy-waiting
				std::this_thread::sleep_for(std::chrono::milliseconds(1));
			}

			logInfo("Server thread stopped");
		});

		return nullptr;
	}

	// Stop the server and wait for thread to finish
	error::Error shutdown()
	{
		// Check if already stopped
		if (!running_) {
			// Join thread if it's still running
			if (serverThread_.joinable()) {
				serverThread_.join();
			}
			return nullptr;
		}

		// Signal shutdown - this will cause the server loop to exit
		running_ = false;

		// Close the transport listener to unblock any pending accepts
		// This ensures the accept loop exits quickly
		if (transport_) {
			transport_->close();
		}

		// Wait for server thread to finish
		if (serverThread_.joinable()) {
			serverThread_.join();
		}

		// Now clean up (thread is stopped, no more concurrent access)
		std::lock_guard<std::mutex> lock(mu_);

		// Close all active connections
		for (auto& pair : connections_) {
			pair.second->close();
		}
		connections_.clear();

		// Clear message queue
		while (!messageQueue_.empty()) {
			messageQueue_.pop();
		}

		logInfo("Server shutdown complete");
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
	error::Error start()
	{
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
		return nullptr;
	}

	// Accept new connections (non-blocking)
	void acceptNewConnections()
	{
		if (!running_ || !transport_) {
			return;
		}

		while (true) {
			auto [conn, err] = transport_->accept();
			if (err || !conn) {
				break;
			}

			uint64_t connID = 0;
			{
				std::lock_guard<std::mutex> lock(mu_);
				connID = nextConnectionID_++;
			}

			auto serverConn = std::make_shared<ServerConnection>(conn, connID);

			// Use weak_ptr in message handler to avoid circular reference
			// (ServerConnection owns conn, conn owns the lambda, lambda would own ServerConnection)
			std::weak_ptr<ServerConnection> weakConn = serverConn;

			conn->setMessageHandler([this, weakConn](const std::vector<uint8_t>& data) {
				// Try to lock the weak_ptr to get a shared_ptr
				if (auto serverConn = weakConn.lock()) {
					onMessage(serverConn, data);
				}
				// If lock() fails, connection is being destroyed, message is dropped
			});

			conn->setCloseHandler([this, connID]() {
				onConnectionClose(connID);
			});

			conn->setFailHandler([this, connID](const error::Error& err) {
				onConnectionFail(connID, err);
			});

			// Store the connection
			{
				std::lock_guard<std::mutex> lock(mu_);
				connections_[connID] = serverConn;
			}

			logInfo("New client connected (id: " + std::to_string(connID) + ")");
		}
	}

	// Process messages from the queue
	void processMessages()
	{
		while (true) {
			std::unique_lock<std::mutex> lock(mu_);

			if (messageQueue_.empty()) {
				return;
			}

			// Get next message
			PendingMessage msg = messageQueue_.front();
			messageQueue_.pop();

			// Unlock while processing (long operation)
			lock.unlock();

			handleMessage(msg.connection, msg.data);
		}
	}

	// Clean up closed connections
	void cleanupConnections()
	{
		std::lock_guard<std::mutex> lock(mu_);

		auto it = connections_.begin();
		while (it != connections_.end()) {
			if (it->second->isClosed()) {
				it = connections_.erase(it);
			} else {
				++it;
			}
		}
	}

	// Called when a message is received
	void onMessage(std::shared_ptr<ServerConnection> conn, const std::vector<uint8_t>& data)
	{
		std::lock_guard<std::mutex> lock(mu_);

		if (!running_) {
			return;
		}

		// Queue the message for processing
		messageQueue_.push(PendingMessage{conn, data});
	}

	// Called when a connection closes
	void onConnectionClose(uint64_t connID)
	{
		std::lock_guard<std::mutex> lock(mu_);

		auto it = connections_.find(connID);
		if (it != connections_.end()) {
			it->second->close();
			logInfo("Client disconnected (id: " + std::to_string(connID) + ")");
		}
	}

	// Called when a connection fails
	void onConnectionFail(uint64_t connID, const error::Error& err)
	{
		std::lock_guard<std::mutex> lock(mu_);

		handleError(err);

		auto it = connections_.find(connID);
		if (it != connections_.end()) {
			it->second->close();
		}
	}

	// Handle a single message
	void handleMessage(std::shared_ptr<ServerConnection> conn, const std::vector<uint8_t>& data)
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
	void handleError(const error::Error& err)
	{
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
	void logDebug(const std::string& msg)
	{
		if (config_.logger) {
			config_.logger->debug(msg);
		}
	}

	void logInfo(const std::string& msg)
	{
		if (config_.logger) {
			config_.logger->info(msg);
		}
	}

	void logWarn(const std::string& msg)
	{
		if (config_.logger) {
			config_.logger->warn(msg);
		}
	}

	void logError(const std::string& msg)
	{
		if (config_.logger) {
			config_.logger->error(msg);
		}
	}

	ServerConfig config_;
	std::shared_ptr<ServerTransport> transport_;

	std::shared_ptr<ServerGroup> rootGroup_;
	std::shared_ptr<ServerGroup> activeGroup_;
	std::map<uint64_t, std::shared_ptr<ServerGroup>> groupByServiceID_;
	std::vector<std::shared_ptr<ServerGroup>> ownedGroups_;

	std::atomic<bool> running_;
	std::map<uint64_t, std::shared_ptr<ServerConnection>> connections_;
	uint64_t nextConnectionID_;

	std::queue<PendingMessage> messageQueue_;

	std::thread serverThread_;
	mutable std::mutex mu_;
};

// Helper function to create an error response
inline std::vector<uint8_t> respondWithError(uint64_t requestID, const error::Error& err)
{
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
std::vector<uint8_t> respondWithMessage(uint64_t requestID, const T& msg)
{
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
