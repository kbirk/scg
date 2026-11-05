#pragma once

#ifndef ASIO_STANDALONE
#define ASIO_STANDALONE
#endif
#include <websocketpp/config/asio_no_tls_client.hpp>
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

struct ClientTransportConfig {
	std::string host;
	int port = 0;
	std::string path = "/";
	uint32_t maxSendMessageSize = 0;
	uint32_t maxRecvMessageSize = scg::rpc::DEFAULT_MAX_RECV_MESSAGE_SIZE; // 0 disables the cap
	size_t maxWriteQueueBytes = DEFAULT_MAX_WRITE_QUEUE_BYTES; // 0 disables the cap
	// connectTimeout bounds connect() when the caller supplies no deadline
	// (0 = no timeout; websocketpp's own connect/handshake timeouts still apply).
	std::chrono::milliseconds connectTimeout = DEFAULT_CONNECT_TIMEOUT;
};

typedef websocketpp::client<websocketpp::config::asio_client> client;

// ConnectionWS is the client-side WebSocket connection.
using ConnectionWS = detail::WSConnection<client>;

class ClientTransportWS : public scg::rpc::ClientTransport
{
public:
	ClientTransportWS(const ClientTransportConfig& config)
		: config_(config)
		, endpoint_(config.maxRecvMessageSize)
	{
	}

	~ClientTransportWS()
	{
		shutdown();
	}

	std::pair<std::shared_ptr<scg::rpc::Connection>, error::Error> connect() override
	{
		return connectWithDeadline(scg::rpc::deadlineAfter(config_.connectTimeout));
	}

	std::pair<std::shared_ptr<scg::rpc::Connection>, error::Error> connectWithDeadline(std::chrono::steady_clock::time_point deadline) override
	{
		SCG_LOG_INFO("Connecting to WebSocket server at ws://" + config_.host + ":" + std::to_string(config_.port) + config_.path);
		std::string uri = "ws://" + config_.host + ":" + std::to_string(config_.port) + config_.path;
		return endpoint_.connect(uri, config_.maxSendMessageSize, config_.maxWriteQueueBytes, deadline);
	}

	void shutdown() override
	{
		SCG_LOG_INFO("Shutting down WebSocket client transport");
		endpoint_.shutdown();
	}

private:
	ClientTransportConfig config_;
	detail::ClientEndpoint<client> endpoint_;
};

} // namespace ws
} // namespace scg
