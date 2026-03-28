#pragma once

#include <cstdint>
#include <cstring>
#include <functional>
#include <memory>
#include <optional>
#include <random>
#include <thread>
#include <mutex>
#include <condition_variable>
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

namespace scg {
namespace rpc {

enum class ConnectionStatus {
	NOT_CONNECTED,
	CONNECTED,
	FAILED
};

// Lightweight replacement for std::promise/std::future using mutex + condition_variable.
// Avoids the overhead of shared state allocation and exception handling in std::future.
template <typename T>
class ResponseFuture {
public:
	ResponseFuture() = default;

	// Non-copyable, non-movable (used via shared_ptr)
	ResponseFuture(const ResponseFuture&) = delete;
	ResponseFuture& operator=(const ResponseFuture&) = delete;

	void set_value(T value)
	{
		{
			std::lock_guard<std::mutex> lock(mu_);
			value_ = std::move(value);
		}
		cv_.notify_one();
	}

	T get()
	{
		std::unique_lock<std::mutex> lock(mu_);
		cv_.wait(lock, [this] { return value_.has_value(); });
		return std::move(*value_);
	}

	// Returns true if ready before deadline, false if timed out
	template <typename Clock, typename Duration>
	bool wait_until(const std::chrono::time_point<Clock, Duration>& deadline)
	{
		std::unique_lock<std::mutex> lock(mu_);
		return cv_.wait_until(lock, deadline, [this] { return value_.has_value(); });
	}

private:
	std::mutex mu_;
	std::condition_variable cv_;
	std::optional<T> value_;
};

struct ClientConfig {
	std::shared_ptr<ClientTransport> transport;
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
		std::lock_guard<std::mutex> lock(mu_);

		failPendingRequestsUnsafe("Connection closed");

		return disconnectUnsafe();
	}

	template <typename T>
	std::pair<serialize::Reader, error::Error> call(const context::Context& ctx, uint64_t serviceID, uint64_t methodID, const T& msg)
	{
		auto [responseFuture, requestID, err] = sendMessage(ctx, serviceID, methodID, msg);
		if (err) {
			return std::make_pair(serialize::Reader({}), err);
		}

		if (ctx.hasDeadline()) {
			bool ready = responseFuture->wait_until(ctx.getDeadline());
			if (!ready) {
				// Remove request from map
				std::lock_guard<std::mutex> lock(mu_);
				requests_.erase(requestID);
				return std::make_pair(serialize::Reader({}), error::Error("Request timed out"));
			}
		}

		return receiveMessage(responseFuture);
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

		// Set up handlers
		connection_->setFailHandler([this](const error::Error& err) {
			std::lock_guard<std::mutex> lock(mu_);
			status_ = ConnectionStatus::FAILED;
			// Fail all pending requests
			failPendingRequestsUnsafe("Connection failed: " + err.message());
		});

		connection_->setCloseHandler([this]() {
			std::lock_guard<std::mutex> lock(mu_);
			status_ = ConnectionStatus::NOT_CONNECTED;
			// Fail all pending requests
			failPendingRequestsUnsafe("Connection closed");
		});

		connection_->setMessageHandler([this](const std::vector<uint8_t>& data) {
			onMessage(data);
		});

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

	void onMessage(const std::vector<uint8_t>& data)
	{
		serialize::Reader reader(data);

		using scg::serialize::deserialize;

		std::array<uint8_t, 16> prefix;
		auto err = deserialize(prefix, reader);
		if (err || prefix != RESPONSE_PREFIX) {
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
			disconnectUnsafe();  // Already holding lock, use unsafe version
			return;
		}

		requests_.erase(requestID);
	}


	template <typename T>
	std::tuple<std::shared_ptr<ResponseFuture<serialize::Reader>>, uint64_t, error::Error> sendMessage(const context::Context& ctx, uint64_t serviceID, uint64_t methodID, const T& msg)
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

		auto responseFuture = std::make_shared<ResponseFuture<serialize::Reader>>();

		std::lock_guard<std::mutex> lock(mu_);

		requests_[requestID] = responseFuture;

		auto err = sendBytesUnsafe(writer.bytes());
		if (err) {
			requests_.erase(requestID);
			return std::make_tuple(nullptr, 0, err);
		}

		return std::make_tuple(responseFuture, requestID, nullptr);
	}

	std::pair<serialize::Reader, error::Error> receiveMessage(std::shared_ptr<ResponseFuture<serialize::Reader>>& responseFuture)
	{
		auto reader = responseFuture->get();

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
	std::map<uint64_t, std::shared_ptr<ResponseFuture<serialize::Reader>>> requests_;

};

}
}
