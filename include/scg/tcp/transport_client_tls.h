#pragma once

#include "scg/tcp/connection.h"

#include <asio/connect.hpp>
#include <asio/ssl.hpp>

#include <chrono>
#include <future>
#include <memory>
#include <string>
#include <thread>

namespace scg {
namespace tcp {

struct ClientTransportTLSConfig {
	std::string host;
	int port = 0;
	// verifyPeer verifies the server certificate chain and that it matches
	// `host`. Defaults to true; set it to false only for testing against
	// self-signed certificates.
	bool verifyPeer = true;
	std::string caFile; // CA bundle; empty uses the system default paths
	uint32_t maxSendMessageSize = 0; // 0 for no limit
	uint32_t maxRecvMessageSize = scg::rpc::DEFAULT_MAX_RECV_MESSAGE_SIZE; // 0 disables the cap
	size_t maxWriteQueueBytes = DEFAULT_MAX_WRITE_QUEUE_BYTES; // 0 disables the cap
	// connectTimeout bounds connect() (TCP connect + TLS handshake) when the
	// caller supplies no deadline (0 = no timeout).
	std::chrono::milliseconds connectTimeout = DEFAULT_CONNECT_TIMEOUT;
};

namespace detail {

// TlsStreamOps adapts an asio::ssl::stream for StreamConnection.
struct TlsStreamOps {
	static asio::ip::tcp::socket& lowest(asio::ssl::stream<asio::ip::tcp::socket>& s) { return s.next_layer(); }

	// Small frames are coalesced into one buffer so header and payload share a
	// TLS record; large frames use a gathered write (no payload copy).
	static constexpr size_t coalesceLimit = 64u << 10;

	// A peer that closes without close_notify (stream_truncated) at a frame
	// boundary is treated as a clean close: frames are length-prefixed, so a
	// truncation mid-frame still surfaces as a read error.
	static bool isCleanEof(const asio::error_code& ec)
	{
		return ec == asio::error::eof || ec == asio::ssl::error::stream_truncated;
	}

	// finishClose sends close_notify, then closes the socket (the connection's
	// close timer forces it if the peer never answers).
	template <typename Conn>
	static void finishClose(const std::shared_ptr<Conn>& self)
	{
		self->stream().async_shutdown(asio::bind_executor(self->strand(), [self](const std::error_code&) {
			self->closeLowestNow();
		}));
	}
};

// isIpLiteral reports whether host is an IPv4/IPv6 address literal (SNI must
// not be sent for those).
inline bool isIpLiteral(const std::string& host)
{
	asio::error_code ec;
	asio::ip::make_address(host, ec);
	return !ec;
}

// configureClientVerification sets SNI and host-name verification on a client
// stream.
inline error::Error configureClientStream(asio::ssl::stream<asio::ip::tcp::socket>& stream, const std::string& host, bool verifyPeer)
{
	if (!isIpLiteral(host)) {
		if (!SSL_set_tlsext_host_name(stream.native_handle(), host.c_str())) {
			return error::Error("Failed to set TLS SNI host name");
		}
	}
	if (verifyPeer) {
		stream.set_verify_mode(asio::ssl::verify_peer);
		stream.set_verify_callback(asio::ssl::host_name_verification(host));
	} else {
		stream.set_verify_mode(asio::ssl::verify_none);
	}
	return nullptr;
}

} // namespace detail

// ConnectionTLS is a length-prefixed framed connection over TLS.
class ConnectionTLS : public detail::StreamConnection<asio::ssl::stream<asio::ip::tcp::socket>, detail::TlsStreamOps> {
public:
	ConnectionTLS(asio::ssl::stream<asio::ip::tcp::socket> socket,
		uint32_t maxSendMessageSize = 0,
		uint32_t maxRecvMessageSize = 0,
		std::shared_ptr<asio::io_context> ioKeepAlive = nullptr,
		size_t maxWriteQueueBytes = DEFAULT_MAX_WRITE_QUEUE_BYTES)
		: StreamConnection(std::move(socket), maxSendMessageSize, maxRecvMessageSize, std::move(ioKeepAlive), maxWriteQueueBytes)
	{
		SCG_LOG_INFO("TCP TLS connection established");
	}
};


class ClientTransportTCPTLS : public scg::rpc::ClientTransport
{
public:
	ClientTransportTCPTLS(const ClientTransportTLSConfig& config)
		: config_(config)
		, ssl_context_(asio::ssl::context::tls_client)
	{
		ssl_context_.set_options(
			asio::ssl::context::default_workarounds
			| asio::ssl::context::no_sslv2
			| asio::ssl::context::no_sslv3
			| asio::ssl::context::no_tlsv1
			| asio::ssl::context::no_tlsv1_1);
#if OPENSSL_VERSION_NUMBER >= 0x10100000L
		SSL_CTX_set_min_proto_version(ssl_context_.native_handle(), TLS1_2_VERSION);
#endif

		if (!config_.verifyPeer) {
			ssl_context_.set_verify_mode(asio::ssl::verify_none);
		} else {
			ssl_context_.set_verify_mode(asio::ssl::verify_peer);
			if (!config_.caFile.empty()) {
				ssl_context_.load_verify_file(config_.caFile);
			} else {
				ssl_context_.set_default_verify_paths();
			}
		}
	}

	~ClientTransportTCPTLS()
	{
		shutdown();
	}

	std::pair<std::shared_ptr<scg::rpc::Connection>, error::Error> connect() override
	{
		return connectWithDeadline(scg::rpc::deadlineAfter(config_.connectTimeout));
	}

	std::pair<std::shared_ptr<scg::rpc::Connection>, error::Error> connectWithDeadline(std::chrono::steady_clock::time_point deadline) override
	{
		SCG_LOG_INFO("Connecting to TCP TLS server at " + config_.host + ":" + std::to_string(config_.port));

		struct State : detail::PendingConnect {
			State(asio::io_context& io, asio::ssl::context& ctx) : resolver(io), stream(io, ctx), timer(io) {}
			asio::ip::tcp::resolver resolver;
			asio::ssl::stream<asio::ip::tcp::socket> stream;
			asio::steady_timer timer;
			std::promise<std::pair<std::shared_ptr<ConnectionTLS>, error::Error>> promise;
			bool done = false;

			void finish(std::shared_ptr<ConnectionTLS> conn, error::Error err)
			{
				if (done) {
					return;
				}
				done = true;
				timer.cancel();
				promise.set_value(std::make_pair(std::move(conn), std::move(err)));
			}

			void abort() override
			{
				std::error_code ignored;
				resolver.cancel();
				stream.next_layer().close(ignored);
				finish(nullptr, error::Error("Connection aborted: transport shut down"));
			}
		};

		auto ioc = io_.context();
		auto st = std::make_shared<State>(*ioc, ssl_context_);
		auto future = st->promise.get_future();

		auto err = detail::configureClientStream(st->stream, config_.host, config_.verifyPeer);
		if (err) {
			return {nullptr, err};
		}

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
					st->stream.next_layer().close(ignored);
					st->finish(nullptr, error::Error("Connection timed out"));
				});
			}
			st->resolver.async_resolve(host, port, [this, st, ioc](const std::error_code& ec, asio::ip::tcp::resolver::results_type results) {
				if (st->done) {
					return;
				}
				if (ec) {
					st->finish(nullptr, detail::connectError(ec));
					return;
				}
				asio::async_connect(st->stream.next_layer(), results, [this, st, ioc](const std::error_code& ec, const asio::ip::tcp::endpoint&) {
					if (st->done) {
						return;
					}
					if (ec) {
						st->finish(nullptr, detail::connectError(ec));
						return;
					}
					st->stream.async_handshake(asio::ssl::stream_base::client, [this, st, ioc](const std::error_code& ec) {
						if (st->done) {
							return;
						}
						if (ec) {
							st->finish(nullptr, error::Error("TLS handshake failed: " + ec.message()));
							return;
						}
						auto conn = std::make_shared<ConnectionTLS>(std::move(st->stream), config_.maxSendMessageSize, config_.maxRecvMessageSize, ioc, config_.maxWriteQueueBytes);
						registry_.add(conn);
						st->finish(conn, nullptr);
					});
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
			SCG_LOG_ERROR("TCP TLS connection failed: " + result.second.message());
			return {nullptr, result.second};
		}
		return {result.first, nullptr};
	}

	void shutdown() override
	{
		SCG_LOG_INFO("Shutting down TCP TLS client transport");
		io_.shutdown([this]() {
			pending_.abortAll();
			for (auto& conn : registry_.snapshot()) {
				conn->closeNow();
			}
		});
	}

private:
	ClientTransportTLSConfig config_;
	asio::ssl::context ssl_context_;
	detail::ConnectionRegistry<ConnectionTLS> registry_;
	detail::PendingConnects pending_;
	detail::ClientIoThread io_;
};

} // namespace tcp
} // namespace scg
