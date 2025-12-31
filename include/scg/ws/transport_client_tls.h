#pragma once

#define ASIO_STANDALONE
#include <websocketpp/config/asio_client.hpp>
#include <websocketpp/client.hpp>

#include "scg/transport.h"
#include "scg/error.h"
#include "scg/logger.h"
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

struct ClientTransportTLSConfig {
	std::string host;
	int port;
	std::string path = "/";
	bool verifyPeer = true;
	std::string caFile;
	uint32_t maxSendMessageSize = 0;
	uint32_t maxRecvMessageSize = 0;
};

typedef websocketpp::client<websocketpp::config::asio_tls_client> client_tls;

class ConnectionWSTLS : public scg::rpc::Connection, public std::enable_shared_from_this<ConnectionWSTLS> {
public:
	ConnectionWSTLS(client_tls* c, websocketpp::connection_hdl hdl, uint32_t maxSendMessageSize = 0)
		: client_(c)
		, hdl_(hdl)
		, closed_(false)
		, maxSendMessageSize_(maxSendMessageSize)
	{
	}

	~ConnectionWSTLS()
	{
		close();
	}

	error::Error send(const std::vector<uint8_t>& data) override
	{
		if (closed_) {
			return error::Error("Connection closed");
		}

		if (maxSendMessageSize_ > 0 && data.size() > maxSendMessageSize_) {
			return error::Error("Message size exceeds send limit");
		}

		auto self = shared_from_this();
		client_->get_io_service().post([self, data]() {
			websocketpp::lib::error_code ec;
			self->client_->send(self->hdl_, data.data(), data.size(), websocketpp::frame::opcode::binary, ec);
			if (ec) {
				SCG_LOG_ERROR("WebSocket send error: " + ec.message());
			}
		});
		return nullptr;
	}

	void setMessageHandler(std::function<void(const std::vector<uint8_t>&)> handler) override
	{
		messageHandler_ = handler;

		websocketpp::lib::error_code ec;
		client_tls::connection_ptr con = client_->get_con_from_hdl(hdl_, ec);
		if (ec) {
			return;
		}

		auto self = shared_from_this();
		con->set_message_handler([self](websocketpp::connection_hdl, client_tls::message_ptr msg) {
			if (self->messageHandler_) {
				auto& payload = msg->get_payload();
				std::vector<uint8_t> data(payload.begin(), payload.end());
				self->messageHandler_(data);
			}
		});
	}

	void setFailHandler(std::function<void(const error::Error&)> handler) override
	{
		failHandler_ = handler;
		websocketpp::lib::error_code ec;
		client_tls::connection_ptr con = client_->get_con_from_hdl(hdl_, ec);
		if (ec) {
			return;
		}

		auto self = shared_from_this();
		con->set_fail_handler([self, con](websocketpp::connection_hdl) {
			if (self->failHandler_) {
				self->failHandler_(error::Error(con->get_ec().message()));
			}
		});
	}

	void setCloseHandler(std::function<void()> handler) override
	{
		closeHandler_ = handler;
		websocketpp::lib::error_code ec;
		client_tls::connection_ptr con = client_->get_con_from_hdl(hdl_, ec);
		if (ec) {
			return;
		}

		auto self = shared_from_this();
		con->set_close_handler([self](websocketpp::connection_hdl) {
			self->closed_ = true;
			if (self->closeHandler_) {
				self->closeHandler_();
			}
		});
	}

	error::Error close() override
	{
		// Atomically set closed_ to true if it was false
		bool expected = false;
		if (closed_.compare_exchange_strong(expected, true)) {
			SCG_LOG_INFO("WebSocket TLS connection closing");

			// Ensure handlers are cleared so they don't run after destruction
			// and to break potential reference cycles
			websocketpp::lib::error_code ec;
			client_tls::connection_ptr con = client_->get_con_from_hdl(hdl_, ec);
			if (!ec) {
				con->set_message_handler(nullptr);
				con->set_fail_handler(nullptr);
				con->set_close_handler(nullptr);
			}

			messageHandler_ = nullptr;
			failHandler_ = nullptr;
			closeHandler_ = nullptr;

			client_->close(hdl_, websocketpp::close::status::normal, "", ec);
			if (ec) {
				return error::Error(ec.message());
			}
		}
		return nullptr;
	}

private:
	client_tls* client_;
	websocketpp::connection_hdl hdl_;
	std::function<void(const std::vector<uint8_t>&)> messageHandler_;
	std::function<void(const error::Error&)> failHandler_;
	std::function<void()> closeHandler_;
	std::atomic<bool> closed_;
	uint32_t maxSendMessageSize_;
};


class ClientTransportWSTLS : public scg::rpc::ClientTransport
{
public:
	ClientTransportWSTLS(const ClientTransportTLSConfig& config)
		: config_(config)
	{
		client_.clear_access_channels(websocketpp::log::alevel::all);
		client_.clear_error_channels(websocketpp::log::elevel::all);
		client_.init_asio();
		client_.start_perpetual();

		if (config_.maxRecvMessageSize > 0) {
			client_.set_max_message_size(config_.maxRecvMessageSize);
		}

		client_.set_tls_init_handler([this](websocketpp::connection_hdl) {
			return on_tls_init();
		});

		thread_ = std::thread([this]() {
			client_.run();
		});
	}

	~ClientTransportWSTLS()
	{
		shutdown();
	}

	std::pair<std::shared_ptr<scg::rpc::Connection>, error::Error> connect() override
	{
		SCG_LOG_INFO("Connecting to WebSocket TLS server at wss://" + config_.host + ":" + std::to_string(config_.port) + config_.path);
		websocketpp::lib::error_code ec;
		std::string uri = "wss://" + config_.host + ":" + std::to_string(config_.port) + config_.path;
		client_tls::connection_ptr con = client_.get_connection(uri, ec);
		if (ec) {
			return {nullptr, error::Error(ec.message())};
		}

		auto connection = std::make_shared<ConnectionWSTLS>(&client_, con->get_handle(), config_.maxSendMessageSize);

		auto promise = std::make_shared<std::promise<error::Error>>();
		auto future = promise->get_future();

		con->set_open_handler([promise](websocketpp::connection_hdl) {
			promise->set_value(nullptr);
		});

		con->set_fail_handler([promise, con](websocketpp::connection_hdl) {
			promise->set_value(error::Error(con->get_ec().message()));
		});

		client_.connect(con);

		if (future.wait_for(std::chrono::seconds(10)) == std::future_status::timeout) {
			websocketpp::lib::error_code close_ec;
			client_.close(con->get_handle(), websocketpp::close::status::normal, "Timeout", close_ec);
			return {nullptr, error::Error("Connection timed out")};
		}

		auto err = future.get();
		if (err) {
			SCG_LOG_ERROR("WebSocket TLS connection failed: " + err.message);
			return {nullptr, err};
		}

		SCG_LOG_INFO("WebSocket TLS connection established");
		return {connection, nullptr};
	}

	void shutdown() override
	{
		SCG_LOG_INFO("Shutting down WebSocket TLS client transport");
		if (!client_.stopped()) {
			client_.stop_perpetual();
			client_.stop();
		}
		if (thread_.joinable()) {
			thread_.join();
		}
	}

private:
	websocketpp::lib::shared_ptr<asio::ssl::context> on_tls_init()
	{
		auto ctx = websocketpp::lib::make_shared<asio::ssl::context>(asio::ssl::context::sslv23);

		try {
			ctx->set_options(
				asio::ssl::context::default_workarounds |
				asio::ssl::context::no_sslv2 |
				asio::ssl::context::no_sslv3 |
				asio::ssl::context::single_dh_use);

			if (config_.verifyPeer) {
				ctx->set_verify_mode(asio::ssl::verify_peer);
				if (!config_.caFile.empty()) {
					ctx->load_verify_file(config_.caFile);
				} else {
					ctx->set_default_verify_paths();
				}
			} else {
				ctx->set_verify_mode(asio::ssl::verify_none);
			}
		} catch (std::exception& e) {
			SCG_LOG_ERROR("TLS Init Error: " + std::string(e.what()));
		}
		return ctx;
	}

	ClientTransportTLSConfig config_;
	client_tls client_;
	std::thread thread_;
};

} // namespace ws
} // namespace scg
