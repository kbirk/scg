#pragma once

#define ASIO_STANDALONE
#include <websocketpp/config/asio_no_tls_client.hpp>
#include <websocketpp/client.hpp>

#include "scg/transport.h"
#include "scg/error.h"
#include <memory>
#include <string>
#include <thread>
#include <mutex>
#include <atomic>
#include <vector>
#include <iostream>
#include <future>

namespace scg {
namespace ws {

struct ClientTransportConfig {
	std::string host;
	int port;
	std::string path = "/";
	uint32_t maxSendMessageSize = 0;
	uint32_t maxRecvMessageSize = 0;
};

typedef websocketpp::client<websocketpp::config::asio_client> client;

class ConnectionWS : public scg::rpc::Connection {
public:
	ConnectionWS(client* c, websocketpp::connection_hdl hdl, uint32_t maxSendMessageSize = 0)
		: client_(c)
		, hdl_(hdl)
		, closed_(false)
		, maxSendMessageSize_(maxSendMessageSize)
	{
	}

	~ConnectionWS()
	{
		close();
	}

	error::Error send(const std::vector<uint8_t>& data) override
	{
		if (closed_) return error::Error("Connection closed");

		if (maxSendMessageSize_ > 0 && data.size() > maxSendMessageSize_) {
			return error::Error("Message size exceeds send limit");
		}

		websocketpp::lib::error_code ec;
		client_->send(hdl_, data.data(), data.size(), websocketpp::frame::opcode::binary, ec);
		if (ec) {
			return error::Error(ec.message());
		}
		return nullptr;
	}

	void setMessageHandler(std::function<void(const std::vector<uint8_t>&)> handler) override
	{
		messageHandler_ = handler;

		websocketpp::lib::error_code ec;
		client::connection_ptr con = client_->get_con_from_hdl(hdl_, ec);
		if (ec) return;

		con->set_message_handler([this](websocketpp::connection_hdl, client::message_ptr msg) {
			if (messageHandler_) {
				auto& payload = msg->get_payload();
				std::vector<uint8_t> data(payload.begin(), payload.end());
				messageHandler_(data);
			}
		});
	}

	void setFailHandler(std::function<void(const error::Error&)> handler) override
	{
		failHandler_ = handler;
		websocketpp::lib::error_code ec;
		client::connection_ptr con = client_->get_con_from_hdl(hdl_, ec);
		if (ec) return;

		con->set_fail_handler([this, con](websocketpp::connection_hdl) {
			if (failHandler_) failHandler_(error::Error(con->get_ec().message()));
		});
	}

	void setCloseHandler(std::function<void()> handler) override
	{
		closeHandler_ = handler;
		websocketpp::lib::error_code ec;
		client::connection_ptr con = client_->get_con_from_hdl(hdl_, ec);
		if (ec) return;

		con->set_close_handler([this](websocketpp::connection_hdl) {
			closed_ = true;
			if (closeHandler_) closeHandler_();
		});
	}

	error::Error close() override
	{
		if (!closed_) {
			closed_ = true;

			// Ensure handlers are cleared so they don't run after destruction
			// and to break potential reference cycles
			websocketpp::lib::error_code ec;
			client::connection_ptr con = client_->get_con_from_hdl(hdl_, ec);
			if (!ec) {
				con->set_message_handler(nullptr);
				con->set_fail_handler(nullptr);
				con->set_close_handler(nullptr);
			}

			messageHandler_ = nullptr;
			failHandler_ = nullptr;
			closeHandler_ = nullptr;

			client_->close(hdl_, websocketpp::close::status::normal, "", ec);
			if (ec) return error::Error(ec.message());
		}
		return nullptr;
	}

private:
	client* client_;
	websocketpp::connection_hdl hdl_;
	std::function<void(const std::vector<uint8_t>&)> messageHandler_;
	std::function<void(const error::Error&)> failHandler_;
	std::function<void()> closeHandler_;
	std::atomic<bool> closed_;
	uint32_t maxSendMessageSize_;
};

class ClientTransportWS : public scg::rpc::ClientTransport
{
public:
	ClientTransportWS(const ClientTransportConfig& config)
		: config_(config)
	{
		client_.clear_access_channels(websocketpp::log::alevel::all);
		client_.clear_error_channels(websocketpp::log::elevel::all);
		client_.init_asio();
		client_.start_perpetual();

		if (config_.maxRecvMessageSize > 0) {
			client_.set_max_message_size(config_.maxRecvMessageSize);
		}

		thread_ = std::thread([this]() {
			client_.run();
		});
	}

	~ClientTransportWS()
	{
		shutdown();
	}

	std::pair<std::shared_ptr<scg::rpc::Connection>, error::Error> connect() override
	{
		std::cout << "Client connecting..." << std::endl;
		websocketpp::lib::error_code ec;
		std::string uri = "ws://" + config_.host + ":" + std::to_string(config_.port) + config_.path;
		client::connection_ptr con = client_.get_connection(uri, ec);
		if (ec) {
			return {nullptr, error::Error(ec.message())};
		}

		auto connection = std::make_shared<ConnectionWS>(&client_, con->get_handle(), config_.maxSendMessageSize);

		auto promise = std::make_shared<std::promise<error::Error>>();
		auto future = promise->get_future();

		con->set_open_handler([promise](websocketpp::connection_hdl) {
			std::cout << "Client connected" << std::endl;
			promise->set_value(nullptr);
		});

		con->set_fail_handler([promise, con](websocketpp::connection_hdl) {
			std::cout << "Client connection failed: " << con->get_ec().message() << std::endl;
			promise->set_value(error::Error(con->get_ec().message()));
		});

		client_.connect(con);

		// std::cout << "Client waiting for connection..." << std::endl;
		if (future.wait_for(std::chrono::seconds(10)) == std::future_status::timeout) {
			// std::cout << "Client connection timeout" << std::endl;
			return {nullptr, error::Error("Connection timed out")};
		}

		auto err = future.get();
		if (err) {
			return {nullptr, err};
		}

		return {connection, nullptr};
	}

	void shutdown() override
	{
		if (!client_.stopped()) {
			client_.stop_perpetual();
			client_.stop();
		}
		if (thread_.joinable()) {
			thread_.join();
		}
	}

private:
	ClientTransportConfig config_;
	client client_;
	std::thread thread_;
};

} // namespace ws
} // namespace scg
