#pragma once

#define ASIO_STANDALONE
#include <websocketpp/config/asio_no_tls.hpp>
#include <websocketpp/server.hpp>

#include "scg/transport.h"
#include "scg/error.h"
#include "scg/logger.h"
#include <memory>
#include <string>
#include <vector>
#include <deque>
#include <mutex>
#include <atomic>

namespace scg {
namespace ws {

struct ServerTransportConfig {
	int port;
	uint32_t maxSendMessageSize = 0;
	uint32_t maxRecvMessageSize = 0;
};

typedef websocketpp::server<websocketpp::config::asio> server;

class ConnectionWSServer : public scg::rpc::Connection, public std::enable_shared_from_this<ConnectionWSServer> {
public:
	ConnectionWSServer(server* s, websocketpp::connection_hdl hdl, uint32_t maxSendMessageSize = 0)
		: server_(s)
		, hdl_(hdl)
		, closed_(false)
		, maxSendMessageSize_(maxSendMessageSize)
	{
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
		server_->get_io_service().post([self, data]() {
			websocketpp::lib::error_code ec;
			self->server_->send(self->hdl_, data.data(), data.size(), websocketpp::frame::opcode::binary, ec);
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
		server::connection_ptr con = server_->get_con_from_hdl(hdl_, ec);
		if (ec) {
			return;
		}

		auto self = shared_from_this();
		con->set_message_handler([self](websocketpp::connection_hdl, server::message_ptr msg) {
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
		server::connection_ptr con = server_->get_con_from_hdl(hdl_, ec);
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
		server::connection_ptr con = server_->get_con_from_hdl(hdl_, ec);
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
			SCG_LOG_INFO("WebSocket connection closing");
			websocketpp::lib::error_code ec;
			server_->close(hdl_, websocketpp::close::status::normal, "", ec);
			if (ec) {
				return error::Error(ec.message());
			}
		}
		return nullptr;
	}

private:
	server* server_;
	websocketpp::connection_hdl hdl_;
	std::function<void(const std::vector<uint8_t>&)> messageHandler_;
	std::function<void(const error::Error&)> failHandler_;
	std::function<void()> closeHandler_;
	std::atomic<bool> closed_;
	uint32_t maxSendMessageSize_;
};


class ServerTransportWS : public scg::rpc::ServerTransport
{
public:
	ServerTransportWS(const ServerTransportConfig& config)
		: config_(config)

	{
		server_.clear_access_channels(websocketpp::log::alevel::all);
		server_.clear_error_channels(websocketpp::log::elevel::all);
		server_.init_asio();

		if (config_.maxRecvMessageSize > 0) {
			server_.set_max_message_size(config_.maxRecvMessageSize);
		}

		server_.set_open_handler([this](websocketpp::connection_hdl hdl) {
			SCG_LOG_INFO("WebSocket server accepted new connection");
			auto conn = std::make_shared<ConnectionWSServer>(&server_, hdl, config_.maxSendMessageSize);
			if (onConnectionHandler_) {
				onConnectionHandler_(conn);
			}
		});
	}

	~ServerTransportWS()
	{
		stop();
	}

	void setOnConnection(std::function<void(std::shared_ptr<scg::rpc::Connection>)> handler) override
	{
		onConnectionHandler_ = handler;
	}

	error::Error startListening() override
	{
		try {
			SCG_LOG_INFO("WebSocket server listening on port " + std::to_string(config_.port));
			server_.set_reuse_addr(true);
			server_.listen(config_.port);
			server_.start_accept();
			return nullptr;
		} catch (const std::exception& e) {
			SCG_LOG_ERROR("WebSocket server failed to start: " + std::string(e.what()));
			return error::Error(e.what());
		}
	}

	void runEventLoop() override
	{
		server_.run();
	}

	void stop() override
	{
		SCG_LOG_INFO("Stopping WebSocket server");
		if (server_.is_listening()) {
			server_.stop_listening();
		}
		server_.stop();
	}

private:
	ServerTransportConfig config_;
	server server_;
	std::function<void(std::shared_ptr<scg::rpc::Connection>)> onConnectionHandler_;
};

} // namespace ws
} // namespace scg
