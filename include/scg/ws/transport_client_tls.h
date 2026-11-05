#pragma once

#ifndef ASIO_STANDALONE
#define ASIO_STANDALONE
#endif
#include <websocketpp/config/asio_client.hpp>
#include <websocketpp/client.hpp>

#include "scg/ws/detail.h"
#include "scg/transport.h"
#include "scg/error.h"
#include "scg/logger.h"
#include <chrono>
#include <memory>
#include <string>

namespace scg {
namespace ws {

struct ClientTransportTLSConfig {
	std::string host;
	int port = 0;
	std::string path = "/";
	// verifyPeer verifies the server certificate chain and that it matches
	// `host`. Defaults to true.
	bool verifyPeer = true;
	std::string caFile; // CA bundle; empty uses the system default paths
	uint32_t maxSendMessageSize = 0;
	uint32_t maxRecvMessageSize = scg::rpc::DEFAULT_MAX_RECV_MESSAGE_SIZE; // 0 disables the cap
	size_t maxWriteQueueBytes = DEFAULT_MAX_WRITE_QUEUE_BYTES; // 0 disables the cap
	// connectTimeout bounds connect() when the caller supplies no deadline
	// (0 = no timeout; websocketpp's own connect/handshake timeouts still apply).
	std::chrono::milliseconds connectTimeout = DEFAULT_CONNECT_TIMEOUT;
};

typedef websocketpp::client<websocketpp::config::asio_tls_client> client_tls;

// ConnectionWSTLS is the client-side secure WebSocket connection.
using ConnectionWSTLS = detail::WSConnection<client_tls>;

class ClientTransportWSTLS : public scg::rpc::ClientTransport
{
public:
	ClientTransportWSTLS(const ClientTransportTLSConfig& config)
		: config_(config)
		, endpoint_(config.maxRecvMessageSize)
	{
		// One TLS context for every connection of this transport (loading the
		// CA bundle per connection is expensive).
		initError_ = buildContext();
		auto ctx = ctx_;
		endpoint_.endpoint().set_tls_init_handler([ctx](websocketpp::connection_hdl) {
			return ctx;
		});
	}

	~ClientTransportWSTLS()
	{
		shutdown();
	}

	std::pair<std::shared_ptr<scg::rpc::Connection>, error::Error> connect() override
	{
		return connectWithDeadline(scg::rpc::deadlineAfter(config_.connectTimeout));
	}

	std::pair<std::shared_ptr<scg::rpc::Connection>, error::Error> connectWithDeadline(std::chrono::steady_clock::time_point deadline) override
	{
		if (initError_) {
			return {nullptr, initError_};
		}
		SCG_LOG_INFO("Connecting to WebSocket TLS server at wss://" + config_.host + ":" + std::to_string(config_.port) + config_.path);
		std::string uri = "wss://" + config_.host + ":" + std::to_string(config_.port) + config_.path;
		return endpoint_.connect(uri, config_.maxSendMessageSize, config_.maxWriteQueueBytes, deadline);
	}

	void shutdown() override
	{
		SCG_LOG_INFO("Shutting down WebSocket TLS client transport");
		endpoint_.shutdown();
	}

private:
	error::Error buildContext()
	{
		ctx_ = websocketpp::lib::make_shared<asio::ssl::context>(asio::ssl::context::tls_client);
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

			if (config_.verifyPeer) {
				ctx_->set_verify_mode(asio::ssl::verify_peer);
				if (!config_.caFile.empty()) {
					ctx_->load_verify_file(config_.caFile);
				} else {
					ctx_->set_default_verify_paths();
				}
				// Check the certificate matches the host we connect to, not
				// just that some trusted CA signed it.
				ctx_->set_verify_callback(asio::ssl::host_name_verification(config_.host));
			} else {
				ctx_->set_verify_mode(asio::ssl::verify_none);
			}
		} catch (std::exception& e) {
			SCG_LOG_ERROR("TLS Init Error: " + std::string(e.what()));
			return error::Error(std::string("TLS initialization failed: ") + e.what());
		}
		return nullptr;
	}

	ClientTransportTLSConfig config_;
	websocketpp::lib::shared_ptr<asio::ssl::context> ctx_;
	error::Error initError_;
	detail::ClientEndpoint<client_tls> endpoint_;
};

} // namespace ws
} // namespace scg
