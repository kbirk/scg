#pragma once

#define ASIO_STANDALONE
#include <asio.hpp>
#include <asio/ssl.hpp>

#include "scg/transport.h"
#include "scg/tcp/transport_client_tls.h"
#include <memory>
#include <string>
#include <vector>

namespace scg {
namespace tcp {

struct ServerTransportTLSConfig {
	int port;
	std::string certFile;
	std::string keyFile;
	uint32_t maxSendMessageSize = 0; // 0 for no limit
	uint32_t maxRecvMessageSize = 0; // 0 for no limit
};

class ServerTransportTCPTLS : public scg::rpc::ServerTransport {
public:
	ServerTransportTCPTLS(const ServerTransportTLSConfig& config)
		: config_(config)
		, ssl_context_(asio::ssl::context::tls_server)
		, acceptor_(io_context_)
	{

		ssl_context_.set_options(
			asio::ssl::context::default_workarounds
			| asio::ssl::context::no_sslv2
			| asio::ssl::context::single_dh_use);

		ssl_context_.use_certificate_chain_file(config_.certFile);
		ssl_context_.use_private_key_file(config_.keyFile, asio::ssl::context::pem);
	}

	~ServerTransportTCPTLS()
	{
		close();
	}

	error::Error listen() override
	{
		try {
			asio::ip::tcp::endpoint endpoint(asio::ip::tcp::v4(), config_.port);
			acceptor_.open(endpoint.protocol());
			acceptor_.set_option(asio::ip::tcp::acceptor::reuse_address(true));
			acceptor_.bind(endpoint);
			acceptor_.listen();
			return nullptr;
		} catch (const std::exception& e) {
			return error::Error(e.what());
		}
	}

	std::pair<std::shared_ptr<scg::rpc::Connection>, error::Error> accept() override
	{
		try {
			asio::ip::tcp::socket socket(io_context_);
			asio::error_code ec;
			acceptor_.non_blocking(true);
			acceptor_.accept(socket, ec);

			if (ec) {
				if (ec == asio::error::would_block || ec == asio::error::try_again) {
					return {nullptr, nullptr};
				}
				return {nullptr, error::Error(ec.message())};
			}

			// Connection accepted.
			// For simplicity in this synchronous accept interface, we'll perform a blocking handshake.
			// Ensure socket is blocking for the handshake.
			socket.non_blocking(false);

			asio::ssl::stream<asio::ip::tcp::socket> ssl_stream(std::move(socket), ssl_context_);

			try {
				ssl_stream.handshake(asio::ssl::stream_base::server);
			} catch (const std::exception& e) {
				return {nullptr, error::Error(std::string("Handshake failed: ") + e.what())};
			}

			return {std::make_shared<ConnectionTLS>(std::move(ssl_stream), config_.maxSendMessageSize, config_.maxRecvMessageSize), nullptr};
		} catch (const std::exception& e) {
			return {nullptr, error::Error(e.what())};
		}
	}

	void poll() override
	{
		if (io_context_.stopped()) {
			io_context_.restart();
		}
		io_context_.poll();
	}

	error::Error close() override
	{
		if (acceptor_.is_open()) {
			acceptor_.close();
		}
		return nullptr;
	}

private:
	ServerTransportTLSConfig config_;
	asio::io_context io_context_;
	asio::ssl::context ssl_context_;
	asio::ip::tcp::acceptor acceptor_;
};

} // namespace tcp
} // namespace scg
