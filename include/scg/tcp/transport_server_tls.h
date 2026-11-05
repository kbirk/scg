#pragma once

#include "scg/tcp/connection.h"
#include "scg/tcp/transport_server.h"
#include "scg/tcp/transport_client_tls.h"

#include <asio/ssl.hpp>

#include <chrono>
#include <memory>
#include <string>
#include <vector>

namespace scg {
namespace tcp {

struct ServerTransportTLSConfig {
	int port = 0;
	std::string certFile;
	std::string keyFile;
	uint32_t maxSendMessageSize = 0; // 0 for no limit
	uint32_t maxRecvMessageSize = scg::rpc::DEFAULT_MAX_RECV_MESSAGE_SIZE; // 0 disables the cap
	size_t maxWriteQueueBytes = DEFAULT_MAX_WRITE_QUEUE_BYTES; // 0 disables the cap
	// handshakeTimeout closes accepted sockets that do not complete the TLS
	// handshake in time, so idle connections cannot pin file descriptors.
	std::chrono::milliseconds handshakeTimeout{10000};
};

class ServerTransportTCPTLS : public scg::rpc::ServerTransport, public std::enable_shared_from_this<ServerTransportTCPTLS> {
public:
	ServerTransportTCPTLS(const ServerTransportTLSConfig& config)
		: config_(config)
		, ssl_context_(asio::ssl::context::tls_server)
		, io_(std::make_shared<asio::io_context>())
		, acceptor_(*io_)
		, retryTimer_(*io_)
	{
		ssl_context_.set_options(
			asio::ssl::context::default_workarounds
			| asio::ssl::context::no_sslv2
			| asio::ssl::context::no_sslv3
			| asio::ssl::context::no_tlsv1
			| asio::ssl::context::no_tlsv1_1
			| asio::ssl::context::single_dh_use);
#if OPENSSL_VERSION_NUMBER >= 0x10100000L
		SSL_CTX_set_min_proto_version(ssl_context_.native_handle(), TLS1_2_VERSION);
#endif

		ssl_context_.use_certificate_chain_file(config_.certFile);
		ssl_context_.use_private_key_file(config_.keyFile, asio::ssl::context::pem);
	}

	~ServerTransportTCPTLS()
	{
		bool running = state_.markStopping();
		if (!running) {
			closeAll();
			detail::drainIoContext(*io_);
		} else if (io_->get_executor().running_in_this_thread()) {
			closeAll();
			io_->stop();
		} else {
			io_->stop();
		}
	}

	void setOnConnection(std::function<void(std::shared_ptr<scg::rpc::Connection>)> handler) override
	{
		onConnectionHandler_ = handler;
	}

	error::Error startListening() override
	{
		if (weak_from_this().expired()) {
			return error::Error("ServerTransportTCPTLS must be owned by a std::shared_ptr");
		}
		try {
			SCG_LOG_INFO("TCP TLS server listening on port " + std::to_string(config_.port));
			asio::ip::tcp::endpoint endpoint(asio::ip::tcp::v4(), static_cast<asio::ip::port_type>(config_.port));
			acceptor_.open(endpoint.protocol());
			acceptor_.set_option(asio::ip::tcp::acceptor::reuse_address(true));
			acceptor_.bind(endpoint);
			acceptor_.listen();

			state_.clearStopping();
			startAccept();

			return nullptr;
		} catch (const std::exception& e) {
			SCG_LOG_ERROR("TCP TLS server failed to start: " + std::string(e.what()));
			asio::error_code ec;
			acceptor_.close(ec);
			return error::Error(e.what());
		}
	}

	void runEventLoop() override
	{
		// Single io thread — see ServerTransportTCP::runEventLoop().
		if (!state_.beginRun(*io_)) {
			return; // stopped before the loop started
		}
		io_->run();
		detail::drainIoContext(*io_);
		state_.endRun();
	}

	void stop() override
	{
		SCG_LOG_INFO("Stopping TCP TLS server");
		if (state_.markStopping()) {
			auto self = shared_from_this();
			asio::post(*io_, [self]() {
				self->closeAll();
				self->io_->stop();
			});
			return;
		}
		closeAll();
		detail::drainIoContext(*io_);
	}

private:
	struct Handshake {
		Handshake(asio::ip::tcp::socket socket, asio::ssl::context& ctx, asio::io_context& io)
			: stream(std::move(socket), ctx)
			, timer(io)
		{
		}
		asio::ssl::stream<asio::ip::tcp::socket> stream;
		asio::steady_timer timer;
	};

	void closeAll()
	{
		asio::error_code ec;
		acceptor_.close(ec);
		retryTimer_.cancel();
		for (auto& weak : handshakes_) {
			if (auto hs = weak.lock()) {
				hs->timer.cancel();
				hs->stream.next_layer().close(ec);
			}
		}
		handshakes_.clear();
		for (auto& conn : registry_.snapshot()) {
			conn->closeNow();
		}
	}

	void startAccept()
	{
		std::weak_ptr<ServerTransportTCPTLS> weak = shared_from_this();
		acceptor_.async_accept([weak](const std::error_code& error, asio::ip::tcp::socket socket) {
			auto self = weak.lock();
			if (!self) {
				return;
			}
			self->onAccept(error, std::move(socket));
		});
	}

	void onAccept(const std::error_code& error, asio::ip::tcp::socket socket)
	{
		std::weak_ptr<ServerTransportTCPTLS> weak = shared_from_this();
		if (error) {
			if (error == asio::error::operation_aborted || state_.stopping()) {
				return; // clean shutdown
			}
			SCG_LOG_ERROR("TCP TLS server accept error: " + error.message());
			retryTimer_.expires_after(backoff_.next());
			retryTimer_.async_wait([weak](const std::error_code& ec) {
				auto self = weak.lock();
				if (ec || !self) {
					return;
				}
				self->startAccept();
			});
			return;
		}
		backoff_.reset();
		if (state_.stopping()) {
			asio::error_code ec;
			socket.close(ec);
			return;
		}

		auto hs = std::make_shared<Handshake>(std::move(socket), ssl_context_, *io_);
		trackHandshake(hs);

		hs->timer.expires_after(config_.handshakeTimeout);
		hs->timer.async_wait([hs](const std::error_code& ec) {
			if (ec) {
				return; // handshake finished (timer cancelled)
			}
			SCG_LOG_ERROR("TCP TLS handshake timed out");
			asio::error_code ignored;
			hs->stream.next_layer().close(ignored);
		});
		hs->stream.async_handshake(asio::ssl::stream_base::server, [weak, hs](const std::error_code& error) {
			hs->timer.cancel();
			auto self = weak.lock();
			if (error || !self || self->state_.stopping()) {
				return;
			}
			SCG_LOG_INFO("TCP TLS server accepted new connection");
			auto conn = std::make_shared<ConnectionTLS>(std::move(hs->stream), self->config_.maxSendMessageSize, self->config_.maxRecvMessageSize, self->io_, self->config_.maxWriteQueueBytes);
			self->registry_.add(conn);
			if (self->onConnectionHandler_) {
				self->onConnectionHandler_(conn);
			}
		});

		startAccept();
	}

	void trackHandshake(const std::shared_ptr<Handshake>& hs)
	{
		if (handshakes_.size() >= 64) {
			handshakes_.erase(std::remove_if(handshakes_.begin(), handshakes_.end(), [](const std::weak_ptr<Handshake>& w) {
				return w.expired();
			}), handshakes_.end());
		}
		handshakes_.push_back(hs);
	}

	ServerTransportTLSConfig config_;
	asio::ssl::context ssl_context_;
	std::shared_ptr<asio::io_context> io_;
	asio::ip::tcp::acceptor acceptor_;
	asio::steady_timer retryTimer_;
	detail::AcceptBackoff backoff_;
	detail::ServerLoopState state_;
	detail::ConnectionRegistry<ConnectionTLS> registry_;
	std::vector<std::weak_ptr<Handshake>> handshakes_; // loop thread only
	std::function<void(std::shared_ptr<scg::rpc::Connection>)> onConnectionHandler_;
};

} // namespace tcp
} // namespace scg
