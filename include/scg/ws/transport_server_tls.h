#pragma once

#ifndef ASIO_STANDALONE
#define ASIO_STANDALONE
#endif
#include <websocketpp/config/asio.hpp>
#include <websocketpp/server.hpp>

#include "scg/ws/detail.h"
#include "scg/transport.h"
#include "scg/error.h"
#include "scg/logger.h"
#include <memory>
#include <string>

namespace scg {
namespace ws {

struct ServerTransportTLSConfig {
	int port = 0;
	std::string certFile;
	std::string keyFile;
	uint32_t maxSendMessageSize = 0;
	uint32_t maxRecvMessageSize = scg::rpc::DEFAULT_MAX_RECV_MESSAGE_SIZE; // 0 disables the cap
	size_t maxWriteQueueBytes = DEFAULT_MAX_WRITE_QUEUE_BYTES; // 0 disables the cap
};

typedef websocketpp::server<websocketpp::config::asio_tls> server_tls;

// ConnectionWSServerTLS is the server-side secure WebSocket connection.
using ConnectionWSServerTLS = detail::WSConnection<server_tls>;

class ServerTransportWSTLS : public scg::rpc::ServerTransport
{
public:
	ServerTransportWSTLS(const ServerTransportTLSConfig& config)
		: config_(config)
		, endpoint_(config.maxRecvMessageSize, config.maxSendMessageSize, config.maxWriteQueueBytes)
	{
		// One TLS context (certificate chain and key loaded once) shared by
		// every connection.
		initError_ = buildContext();
		auto ctx = ctx_;
		endpoint_.endpoint().set_tls_init_handler([ctx](websocketpp::connection_hdl) {
			return ctx;
		});
	}

	void setOnConnection(std::function<void(std::shared_ptr<scg::rpc::Connection>)> handler) override
	{
		endpoint_.setOnConnection(handler);
	}

	error::Error startListening() override
	{
		if (initError_) {
			SCG_LOG_ERROR("WebSocket TLS server failed to start: " + initError_.message());
			return initError_;
		}
		SCG_LOG_INFO("WebSocket TLS server listening on port " + std::to_string(config_.port));
		auto err = endpoint_.listen(config_.port);
		if (err) {
			SCG_LOG_ERROR("WebSocket TLS server failed to start: " + err.message());
		}
		return err;
	}

	void runEventLoop() override
	{
		// Single io thread — see ServerTransportWS::runEventLoop().
		endpoint_.run();
	}

	void stop() override
	{
		SCG_LOG_INFO("Stopping WebSocket TLS server");
		endpoint_.stop();
	}

private:
	error::Error buildContext()
	{
		ctx_ = websocketpp::lib::make_shared<asio::ssl::context>(asio::ssl::context::tls_server);
		try {
			ctx_->set_options(
				asio::ssl::context::default_workarounds |
				asio::ssl::context::no_sslv2 |
				asio::ssl::context::no_sslv3 |
				asio::ssl::context::no_tlsv1 |
				asio::ssl::context::no_tlsv1_1 |
				asio::ssl::context::single_dh_use);
#if OPENSSL_VERSION_NUMBER >= 0x10100000L
			SSL_CTX_set_min_proto_version(ctx_->native_handle(), TLS1_2_VERSION);
#endif
			ctx_->use_certificate_chain_file(config_.certFile);
			ctx_->use_private_key_file(config_.keyFile, asio::ssl::context::pem);
		} catch (std::exception& e) {
			SCG_LOG_ERROR("TLS Init Error: " + std::string(e.what()));
			return error::Error(std::string("TLS initialization failed: ") + e.what());
		}
		return nullptr;
	}

	ServerTransportTLSConfig config_;
	websocketpp::lib::shared_ptr<asio::ssl::context> ctx_;
	error::Error initError_;
	detail::ServerEndpoint<server_tls> endpoint_;
};

} // namespace ws
} // namespace scg
