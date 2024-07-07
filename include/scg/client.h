#pragma once

#define ASIO_STANDALONE 1

#include <cstdint>
#include <functional>
#include <future>
#include <memory>
#include <random>
#include <thread>
#include <websocketpp/config/asio_no_tls_client.hpp>
#include <websocketpp/client.hpp>

#include "scg/error.h"
#include "scg/serialize.h"
#include "scg/reader.h"
#include "scg/writer.h"
#include "scg/const.h"
#include "scg/context.h"
#include "scg/logger.h"
#include "scg/middleware.h"

namespace scg {
namespace rpc {

typedef websocketpp::connection_hdl                    WSConnectionHandle;
typedef websocketpp::config::asio_client::message_type WSMessage;

enum class ConnectionStatus {
	NOT_CONNECTED,
	CONNECTED,
	FAILED
};

struct ClientConfig {
	std::string uri;
	log::LoggingConfig logging;
};

class Client {
public:

	virtual ~Client() = default;

	error::Error connect()
	{
		std::lock_guard<std::mutex> lock(mu_);

		return connectUnsafe();
	}

	error::Error disconnect()
	{
		std::lock_guard<std::mutex> lock(mu_);

		requests_.clear();

		return disconnectUnsafe();
	}

	template <typename T>
	std::pair<serialize::Reader, error::Error> call(const context::Context& ctx, uint64_t serviceID, uint64_t methodID, const T& msg)
	{
		auto [future, err] = sendMessage(ctx, serviceID, methodID, msg);
		if (err) {
			return std::make_pair(serialize::Reader({}), err);
		}

		// TODO: respect any deadlines / timeouts on the context

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

	virtual error::Error connectUnsafe() = 0;
	virtual error::Error disconnectUnsafe() = 0;
	virtual error::Error sendBytesUnsafe(const std::vector<uint8_t>& msg) = 0;

	template <typename Logger>
	void registerLoggerMethods(Logger& logger)
	{
		logger.registerLoggingFuncs(
			conf_.logging.level,
			conf_.logging.debugLogger,
			conf_.logging.infoLogger,
			conf_.logging.warnLogger,
			conf_.logging.errorLogger);
	}


	template <typename T>
	std::pair<std::future<serialize::Reader>,error::Error> sendMessage(const context::Context& ctx, uint64_t serviceID, uint64_t methodID, const T& msg)
	{
		uint64_t requestID = 0;
		{
			std::lock_guard<std::mutex> lock(mu_);

			requestID = requestID_++;
		}

		using scg::serialize::bit_size; // adl trickery

		serialize::FixedSizeWriter writer(
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

		std::lock_guard<std::mutex> lock(mu_);

		auto promise = std::make_shared<std::promise<serialize::Reader>>();

		requests_[requestID] = promise;

		auto err = sendBytesUnsafe(writer.bytes());
		if (err) {
			requests_.erase(requestID);
			return std::make_pair(std::future<serialize::Reader>(), err);
		}

		return std::make_pair(promise->get_future(), nullptr);
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

	template <typename ClientType>
	void onOpenUnsafe(ClientType* client, WSConnectionHandle hdl)
	{
		status_ = ConnectionStatus::CONNECTED;

		promise_.set_value(nullptr);
	}

	template <typename ClientType>
	void onFailUnsafe(ClientType* client, WSConnectionHandle hdl)
	{
		status_ = ConnectionStatus::FAILED;

		auto conn = client->get_con_from_hdl(hdl);

		promise_.set_value(error::Error(conn->get_ec().message()));
	}

	template <typename ClientType>
	void onClose(ClientType* client, WSConnectionHandle hdl)
	{
		std::lock_guard<std::mutex> lock(mu_);

		status_ = ConnectionStatus::NOT_CONNECTED;

		auto conn =  client->get_con_from_hdl(hdl);
	}

	template <typename ClientType>
	void onMessage(ClientType* client, WSConnectionHandle hdl, std::shared_ptr<WSMessage> msg)
	{
		assert(msg->get_opcode() == websocketpp::frame::opcode::binary && "Only binary messages are supported");

		auto payload = msg->get_payload();

		serialize::Reader reader(std::vector<uint8_t>(payload.begin(), payload.end()));

		using scg::serialize::deserialize;

		std::array<uint8_t, 16> prefix;
		deserialize(prefix, reader);

		if (prefix != RESPONSE_PREFIX) {
			client->get_elog().write(websocketpp::log::elevel::fatal, "received message with invalid prefix, closing connection");
			// TODO: resolve the promise with an error
			disconnect();
			return;
		}

		uint64_t requestID = 0;
		serialize::deserialize(requestID, reader);

		std::lock_guard<std::mutex> lock(mu_);

		auto iter = requests_.find(requestID);
		if (iter != requests_.end()) {
			iter->second->set_value(reader);
		} else {
			client->get_elog().write(websocketpp::log::elevel::fatal, "received message with unknown request id, closing connection");
			disconnect();
			return;
		}

		requests_.erase(requestID);
	}

	template <typename ClientType>
	error::Error connectUnsafeImpl(ClientType* client, std::string uri)
	{
		if (status_ != ConnectionStatus::FAILED && status_ != ConnectionStatus::NOT_CONNECTED) {
			return nullptr;
		}

		// connection failed or is closed, we can attempt to reconnect

		std::error_code ec;
		auto conn = client->get_connection(uri, ec);
		if (ec) {
			return error::Error("rpc::Client::connectUnsafe(): could not create connection because: " + ec.message());
		}

		handle_ = conn->get_handle();

		conn->set_open_handler([this, client](WSConnectionHandle hdl) {
			onOpenUnsafe(client, hdl);
		});

		conn->set_fail_handler([this, client](WSConnectionHandle hdl) {
			onFailUnsafe(client, hdl);
		});

		conn->set_close_handler([this, client](WSConnectionHandle hdl) {
			onClose(client, hdl);
		});

		conn->set_message_handler([this, client](WSConnectionHandle hdl, std::shared_ptr<WSMessage> msg) {
			onMessage(client, hdl, msg);
		});

		promise_ = std::promise<error::Error>();
		auto future = promise_.get_future();

		client->connect(conn);

		// wait until the connection resolves
		return future.get();
	}

	template <typename ClientType>
	error::Error disconnectUnsafeImpl(ClientType* client)
	{
		if (status_ != ConnectionStatus::FAILED && status_ != ConnectionStatus::NOT_CONNECTED) {

			std::error_code ec;
			client->close(handle_, websocketpp::close::status::going_away, "User requested disconnect", ec);
			if (ec) {
				return error::Error("rpc::Client::send(): Error closing connection: " + ec.message());
			}
		}

		return nullptr;
	}

	template <typename ClientType>
	error::Error sendBytesUnsafeImpl(ClientType* client, const std::vector<uint8_t>& msg)
	{
		auto err = connectUnsafe();
		if (err) {
			return err;
		}

		if (status_ == ConnectionStatus::CONNECTED) {
			std::error_code ec;
			client->send(handle_, &msg[0], msg.size(), websocketpp::frame::opcode::binary, ec);
			if (ec) {
				return error::Error("rpc::Client::send(): error sending message: " + ec.message());
			}
		}

		return nullptr;
	}

	std::mutex mu_;
	ClientConfig conf_;
	std::shared_ptr<std::thread> thread_;

	ConnectionStatus status_;
	std::promise<error::Error> promise_;
	websocketpp::connection_hdl handle_;

	std::vector<scg::middleware::Middleware> middleware_;

	uint64_t requestID_;
	std::map<uint64_t, std::shared_ptr<std::promise<serialize::Reader>>> requests_;

};

}
}
