#pragma once

#include <cstdint>
#include <functional>
#include <memory>
#include <mutex>
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

namespace scg {
namespace rpc {

// Forward declarations
class Server;
class ServerGroup;

// Message to be processed by the server
struct PendingMessage {
	std::shared_ptr<Connection> connection;
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

		// Now clean up (thread is stopped, no more concurrent access)
		std::lock_guard<std::mutex> lock(mu_);

		// Close all active connections
		for (auto& pair : connections_) {
			pair.second->close();
		}
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

		// Process messages using thread pool to avoid blocking io_context
		conn->setMessageHandler([this, conn](const std::vector<uint8_t>& data) {
			if (!running_) {
				return;
			}
			// Submit to thread pool to avoid blocking event loop
			asio::post(threadPool_, [this, conn, data]() {
				handleMessage(conn, data);
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
		std::lock_guard<std::mutex> lock(mu_);

		auto it = connections_.find(connID);
		if (it != connections_.end()) {
			connections_.erase(it);
		}
	}

	// Called when a connection fails
	void onConnectionFail(uint64_t connID, const error::Error& err)
	{
		std::lock_guard<std::mutex> lock(mu_);

		handleError(err);

		auto it = connections_.find(connID);
		if (it != connections_.end()) {
			connections_.erase(it);
		}
	}

	// Handle a single message
	void handleMessage(std::shared_ptr<Connection> conn, const std::vector<uint8_t>& data)
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
