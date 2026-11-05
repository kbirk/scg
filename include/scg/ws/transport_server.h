#pragma once

#ifndef ASIO_STANDALONE
#define ASIO_STANDALONE
#endif
#include <websocketpp/config/asio_no_tls.hpp>
#include <websocketpp/server.hpp>

#include "scg/ws/detail.h"
#include "scg/transport.h"
#include "scg/error.h"
#include "scg/logger.h"
#include <memory>
#include <string>

namespace scg {
namespace ws {

struct ServerTransportConfig {
	int port = 0;
	uint32_t maxSendMessageSize = 0;
	uint32_t maxRecvMessageSize = scg::rpc::DEFAULT_MAX_RECV_MESSAGE_SIZE; // 0 disables the cap
	size_t maxWriteQueueBytes = DEFAULT_MAX_WRITE_QUEUE_BYTES; // 0 disables the cap
};

typedef websocketpp::server<websocketpp::config::asio> server;

// ConnectionWSServer is the server-side WebSocket connection.
using ConnectionWSServer = detail::WSConnection<server>;

class ServerTransportWS : public scg::rpc::ServerTransport
{
public:
	ServerTransportWS(const ServerTransportConfig& config)
		: config_(config)
		, endpoint_(config.maxRecvMessageSize, config.maxSendMessageSize, config.maxWriteQueueBytes)
	{
	}

	void setOnConnection(std::function<void(std::shared_ptr<scg::rpc::Connection>)> handler) override
	{
		endpoint_.setOnConnection(handler);
	}

	error::Error startListening() override
	{
		SCG_LOG_INFO("WebSocket server listening on port " + std::to_string(config_.port));
		auto err = endpoint_.listen(config_.port);
		if (err) {
			SCG_LOG_ERROR("WebSocket server failed to start: " + err.message());
		}
		return err;
	}

	void runEventLoop() override
	{
		// websocketpp runs on a single io thread; every websocketpp call is made
		// on it (other threads post). Unary handlers run on the server's worker
		// pool and stream handlers on their own threads.
		endpoint_.run();
	}

	// stop stops accepting, closes every connection (close frame, then the
	// socket after a short grace period) and stops the loop.
	void stop() override
	{
		SCG_LOG_INFO("Stopping WebSocket server");
		endpoint_.stop();
	}

private:
	ServerTransportConfig config_;
	detail::ServerEndpoint<server> endpoint_;
};

} // namespace ws
} // namespace scg
