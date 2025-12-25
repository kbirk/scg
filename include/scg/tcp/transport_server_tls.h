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
		stop();
	}

	void setOnConnection(std::function<void(std::shared_ptr<scg::rpc::Connection>)> handler) override
	{
		onConnectionHandler_ = handler;
	}

	error::Error startListening() override
	{
		try {
			asio::ip::tcp::endpoint endpoint(asio::ip::tcp::v4(), config_.port);
			acceptor_.open(endpoint.protocol());
			acceptor_.set_option(asio::ip::tcp::acceptor::reuse_address(true));
			acceptor_.bind(endpoint);
			acceptor_.listen();

			start_accept();

			return nullptr;
		} catch (const std::exception& e) {
			return error::Error(e.what());
		}
	}

	void runEventLoop() override
	{
		if (io_context_.stopped()) {
			io_context_.restart();
		}
		io_context_.run();
	}

	void stop() override
	{
		if (acceptor_.is_open()) {
			acceptor_.close();
		}
		io_context_.stop();
	}

private:
	void start_accept()
	{
		auto socket = std::make_shared<asio::ip::tcp::socket>(io_context_);
		acceptor_.async_accept(*socket, [this, socket](const asio::error_code& error) {
			if (!error) {
				auto ssl_stream = std::make_shared<asio::ssl::stream<asio::ip::tcp::socket>>(std::move(*socket), ssl_context_);
				ssl_stream->async_handshake(asio::ssl::stream_base::server, [this, ssl_stream](const asio::error_code& error) {
					if (!error) {
						if (onConnectionHandler_) {
							onConnectionHandler_(std::make_shared<ConnectionTLS>(std::move(*ssl_stream), config_.maxSendMessageSize, config_.maxRecvMessageSize));
						}
					}
				});
				start_accept();
			}
		});
	}

	ServerTransportTLSConfig config_;
	asio::io_context io_context_;
	asio::ssl::context ssl_context_;
	asio::ip::tcp::acceptor acceptor_;
	std::function<void(std::shared_ptr<scg::rpc::Connection>)> onConnectionHandler_;
};

} // namespace tcp
} // namespace scg
