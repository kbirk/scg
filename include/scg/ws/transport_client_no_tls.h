#pragma once

#define ASIO_STANDALONE 1

#include <memory>
#include <thread>
#include <random>
#include <future>
#include <websocketpp/config/asio_no_tls_client.hpp>
#include <websocketpp/client.hpp>

#include "scg/transport.h"
#include "scg/logger.h"

namespace scg {
namespace ws {

struct ClientTransportConfig {
	std::string host = "localhost";
	int port = 8080;
	uint32_t maxSendMessageSize = 0; // 0 for no limit
	uint32_t maxRecvMessageSize = 0; // 0 for no limit
	log::LoggingConfig logging;
};

struct WSClientNoTLSConfig : public websocketpp::config::asio_client {
	// override logger
	typedef log::LoggerOverride elog_type;
	typedef log::LoggerOverride alog_type;

	struct transport_config : public websocketpp::config::asio_client::transport_config {
		typedef log::LoggerOverride elog_type;
		typedef log::LoggerOverride alog_type;
	};

	typedef websocketpp::transport::asio::endpoint<transport_config> transport_type;
};

typedef websocketpp::client<WSClientNoTLSConfig> WSClientNoTLS;
typedef websocketpp::connection_hdl WSConnectionHandle;
typedef websocketpp::config::asio_client::message_type WSMessage;

// WebSocket connection implementation (no TLS)
class WebSocketConnection : public rpc::Connection {
public:
	WebSocketConnection(WSClientNoTLS* client, WSConnectionHandle handle, uint32_t maxSendMessageSize = 0, uint32_t maxRecvMessageSize = 0)
		: client_(client)
		, handle_(handle)
		, maxSendMessageSize_(maxSendMessageSize)
		, maxRecvMessageSize_(maxRecvMessageSize)
	{

	}

	error::Error send(const std::vector<uint8_t>& data) override
	{
		if (!client_) {
			return error::Error("Connection is not available");
		}

		if (maxSendMessageSize_ > 0 && data.size() > maxSendMessageSize_) {
			return error::Error("Message size exceeds send limit");
		}

		std::error_code ec;
		client_->send(handle_, &data[0], data.size(), websocketpp::frame::opcode::binary, ec);
		if (ec) {
			return error::Error("Error sending message: " + ec.message());
		}
		return nullptr;
	}

	void setMessageHandler(std::function<void(const std::vector<uint8_t>&)> handler) override
	{
		messageHandler_ = handler;
	}

	void setFailHandler(std::function<void(const error::Error&)> handler) override
	{
		failHandler_ = handler;
	}

	void setCloseHandler(std::function<void()> handler) override
	{
		closeHandler_ = handler;
	}

	error::Error close() override
	{
		if (!client_) {
			return nullptr;
		}

		std::error_code ec;
		client_->close(handle_, websocketpp::close::status::going_away, "User requested disconnect", ec);
		if (ec) {
			return error::Error("Error closing connection: " + ec.message());
		}
		return nullptr;
	}

	// Internal methods called by transport
	void onMessage(std::shared_ptr<WSMessage> msg)
	{
		if (messageHandler_ && msg->get_opcode() == websocketpp::frame::opcode::binary) {
			auto payload = msg->get_payload();
			if (maxRecvMessageSize_ > 0 && payload.size() > maxRecvMessageSize_) {
				if (failHandler_) {
					failHandler_(error::Error("Message size exceeds receive limit"));
				}
				return;
			}
			std::vector<uint8_t> data(payload.begin(), payload.end());
			messageHandler_(data);
		}
	}

	void onOpen()
	{
		// Connection opened - this is handled by the transport's promise mechanism
	}

	void onFail(const error::Error& err)
	{
		if (failHandler_) {
			failHandler_(err);
		}
	}

	void onClose()
	{
		if (closeHandler_) {
			closeHandler_();
		}
	}

private:
	WSClientNoTLS* client_;
	WSConnectionHandle handle_;
	uint32_t maxSendMessageSize_;
	uint32_t maxRecvMessageSize_;
	std::function<void(const std::vector<uint8_t>&)> messageHandler_;
	std::function<void(const error::Error&)> failHandler_;
	std::function<void()> closeHandler_;
};

// WebSocket client transport (no TLS)
class ClientTransportNoTLS : public rpc::ClientTransport {
public:
	ClientTransportNoTLS(const ClientTransportConfig& config)
		: config_(config)
	{
		// set logging parameters
		registerLoggerMethods(client_.get_alog());
		registerLoggerMethods(client_.get_elog());

		client_.init_asio();

		// without this `run` exits once there are no active connections
		client_.start_perpetual();

		// start `run` in its own thread
		thread_ = std::make_shared<std::thread>(&WSClientNoTLS::run, &client_);
	}

	~ClientTransportNoTLS()
	{
		shutdown();
	}

	std::pair<std::shared_ptr<rpc::Connection>, error::Error> connect() override
	{
		std::string uri = "ws://" + config_.host + ":" + std::to_string(config_.port) + "/rpc";

		std::error_code ec;
		auto conn = client_.get_connection(uri, ec);
		if (ec) {
			return std::make_pair(nullptr, error::Error("Could not create connection: " + ec.message()));
		}

		auto handle = conn->get_handle();
		auto wsConn = std::make_shared<WebSocketConnection>(&client_, handle, config_.maxSendMessageSize, config_.maxRecvMessageSize);

		// Create a promise to wait for connection completion
		auto promise = std::make_shared<std::promise<error::Error>>();
		auto future = promise->get_future();

		conn->set_open_handler([wsConn, promise](WSConnectionHandle) {
			wsConn->onOpen();
			promise->set_value(nullptr); // Success
		});

		conn->set_fail_handler([wsConn, promise, &client = client_](WSConnectionHandle hdl) {
			auto conn = client.get_con_from_hdl(hdl);
			auto err = error::Error(conn->get_ec().message());
			wsConn->onFail(err);
			promise->set_value(err); // Failure
		});

		conn->set_close_handler([wsConn](WSConnectionHandle) {
			wsConn->onClose();
		});

		conn->set_message_handler([wsConn](WSConnectionHandle, std::shared_ptr<WSMessage> msg) {
			wsConn->onMessage(msg);
		});

		client_.connect(conn);

		// Wait for connection to complete or fail
		auto connectionResult = future.get();
		if (connectionResult) {
			return std::make_pair(nullptr, connectionResult);
		}

		return std::make_pair(wsConn, nullptr);
	}

	void shutdown() override
	{
		if (thread_) {
			// Stop perpetual mode so run() can exit
			client_.stop_perpetual();

			// Force the io_service to stop immediately rather than waiting for
			// the close handshake timeout (5 seconds by default)
			client_.get_io_service().stop();

			// Wait for the run() thread to exit
			thread_->join();
			thread_.reset();
		}
	}

private:
	template <typename Logger>
	void registerLoggerMethods(Logger& logger)
	{
		logger.registerLoggingFuncs(
			config_.logging.level,
			config_.logging.debugLogger,
			config_.logging.infoLogger,
			config_.logging.warnLogger,
			config_.logging.errorLogger);
	}

	ClientTransportConfig config_;
	WSClientNoTLS client_;
	std::shared_ptr<std::thread> thread_;
};

} // namespace ws
} // namespace scg
