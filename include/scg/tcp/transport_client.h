#pragma once

#include "scg/tcp/connection.h"

#include <asio/connect.hpp>

#include <chrono>
#include <future>
#include <memory>
#include <string>
#include <thread>

namespace scg {
namespace tcp {

struct ClientTransportConfig {
	std::string host;
	int port = 0;
	uint32_t maxSendMessageSize = 0; // 0 for no limit
	uint32_t maxRecvMessageSize = scg::rpc::DEFAULT_MAX_RECV_MESSAGE_SIZE; // 0 disables the cap
	size_t maxWriteQueueBytes = DEFAULT_MAX_WRITE_QUEUE_BYTES; // 0 disables the cap
	// connectTimeout bounds connect() when the caller supplies no deadline
	// (0 = no timeout).
	std::chrono::milliseconds connectTimeout = DEFAULT_CONNECT_TIMEOUT;
};

// ConnectionTCP is a length-prefixed framed connection over a raw TCP socket.
class ConnectionTCP : public detail::StreamConnection<asio::ip::tcp::socket, detail::PlainStreamOps> {
public:
	// ioKeepAlive (optional) keeps the socket's io_context alive for as long as
	// the connection exists; transports always pass it.
	ConnectionTCP(asio::ip::tcp::socket socket,
		uint32_t maxSendMessageSize = 0,
		uint32_t maxRecvMessageSize = 0,
		std::shared_ptr<asio::io_context> ioKeepAlive = nullptr,
		size_t maxWriteQueueBytes = DEFAULT_MAX_WRITE_QUEUE_BYTES)
		: StreamConnection(std::move(socket), maxSendMessageSize, maxRecvMessageSize, std::move(ioKeepAlive), maxWriteQueueBytes)
	{
		SCG_LOG_INFO("TCP connection established");
	}
};

class ClientTransportTCP : public scg::rpc::ClientTransport
{
public:
	ClientTransportTCP(const ClientTransportConfig& config)
		: config_(config)
	{
	}

	~ClientTransportTCP()
	{
		shutdown();
	}

	std::pair<std::shared_ptr<scg::rpc::Connection>, error::Error> connect() override
	{
		return connectWithDeadline(scg::rpc::deadlineAfter(config_.connectTimeout));
	}

	// connectWithDeadline resolves and connects asynchronously on the I/O
	// thread; a timer aborts the attempt at the deadline (none for
	// time_point::max()).
	std::pair<std::shared_ptr<scg::rpc::Connection>, error::Error> connectWithDeadline(std::chrono::steady_clock::time_point deadline) override
	{
		SCG_LOG_INFO("Connecting to TCP server at " + config_.host + ":" + std::to_string(config_.port));

		struct State : detail::PendingConnect {
			explicit State(asio::io_context& io) : resolver(io), socket(io), timer(io) {}
			asio::ip::tcp::resolver resolver;
			asio::ip::tcp::socket socket;
			asio::steady_timer timer;
			std::promise<std::pair<std::shared_ptr<ConnectionTCP>, std::error_code>> promise;
			bool done = false;

			void finish(std::shared_ptr<ConnectionTCP> conn, std::error_code ec)
			{
				if (done) {
					return;
				}
				done = true;
				timer.cancel();
				promise.set_value(std::make_pair(std::move(conn), ec));
			}

			void abort() override
			{
				std::error_code ignored;
				resolver.cancel();
				socket.close(ignored);
				finish(nullptr, asio::error::operation_aborted);
			}
		};

		auto ioc = io_.context();
		auto st = std::make_shared<State>(*ioc);
		auto future = st->promise.get_future();
		std::string host = config_.host;
		std::string port = std::to_string(config_.port);

		pending_.add(st);
		bool posted = io_.post([this, st, host, port, deadline, ioc]() {
			if (deadline != std::chrono::steady_clock::time_point::max()) {
				st->timer.expires_at(deadline);
				st->timer.async_wait([st](const std::error_code& ec) {
					if (ec || st->done) {
						return;
					}
					std::error_code ignored;
					st->resolver.cancel();
					st->socket.close(ignored);
					st->finish(nullptr, asio::error::timed_out);
				});
			}
			st->resolver.async_resolve(host, port, [this, st, ioc](const std::error_code& ec, asio::ip::tcp::resolver::results_type results) {
				if (st->done) {
					return;
				}
				if (ec) {
					st->finish(nullptr, ec);
					return;
				}
				asio::async_connect(st->socket, results, [this, st, ioc](const std::error_code& ec, const asio::ip::tcp::endpoint&) {
					if (st->done) {
						return;
					}
					if (ec) {
						st->finish(nullptr, ec);
						return;
					}
					auto conn = std::make_shared<ConnectionTCP>(std::move(st->socket), config_.maxSendMessageSize, config_.maxRecvMessageSize, ioc, config_.maxWriteQueueBytes);
					registry_.add(conn);
					st->finish(conn, std::error_code());
				});
			});
		});
		if (!posted) {
			return {nullptr, error::Error("Transport is shut down")};
		}

		if (!detail::waitForConnect(future, deadline)) {
			return {nullptr, error::Error("Connection timed out")};
		}
		auto result = future.get();
		if (result.second) {
			SCG_LOG_ERROR("TCP connection failed: " + result.second.message());
			return {nullptr, detail::connectError(result.second)};
		}
		return {result.first, nullptr};
	}

	// shutdown closes every connection this transport created, stops and joins
	// the I/O thread. Connections that are still referenced elsewhere (e.g. by a
	// stream handle) stay valid objects; their sends fail with an error.
	void shutdown() override
	{
		SCG_LOG_INFO("Shutting down TCP client transport");
		io_.shutdown([this]() {
			pending_.abortAll();
			for (auto& conn : registry_.snapshot()) {
				conn->closeNow();
			}
		});
	}

private:
	ClientTransportConfig config_;
	detail::ConnectionRegistry<ConnectionTCP> registry_;
	detail::PendingConnects pending_;
	detail::ClientIoThread io_;
};

} // namespace tcp
} // namespace scg
