#pragma once

#include <cstdint>
#include <cstring>
#include <functional>
#include <future>
#include <memory>
#include <random>
#include <thread>
#include <mutex>
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

};

}
}
