#pragma once

#define ASIO_STANDALONE
#include <asio.hpp>

#include "scg/transport.h"
#include "scg/tcp/transport_client.h"
#include <memory>
#include <string>
#include <vector>

namespace scg {
namespace tcp {

struct ServerTransportConfig {
	int port;
	uint32_t maxSendMessageSize = 0; // 0 for no limit
	uint32_t maxRecvMessageSize = 0; // 0 for no limit
};

class ServerTransportTCP : public scg::rpc::ServerTransport {
public:
	ServerTransportTCP(const ServerTransportConfig& config)
		: config_(config)
		, acceptor_(io_context_)
	{
	}

	~ServerTransportTCP()
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
				if (onConnectionHandler_) {
					onConnectionHandler_(std::make_shared<ConnectionTCP>(std::move(*socket), config_.maxSendMessageSize, config_.maxRecvMessageSize));
				}
				start_accept();
			}
		});
	}

	ServerTransportConfig config_;
	asio::io_context io_context_;
	asio::ip::tcp::acceptor acceptor_;
	std::function<void(std::shared_ptr<scg::rpc::Connection>)> onConnectionHandler_;
};

} // namespace tcp
} // namespace scg
