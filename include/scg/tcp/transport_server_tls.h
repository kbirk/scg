#pragma once

#define ASIO_STANDALONE 1
#include <asio.hpp>
#include <asio/ssl.hpp>
#include <memory>
#include <mutex>
#include <queue>

#include "scg/transport.h"
#include "scg/tcp/connection_tls.h"

namespace scg {
namespace tcp {

struct ServerTransportTLSConfig {
    int port = 8443;
    bool noDelay = true;  // Disable Nagle's algorithm by default
    std::string certFile;  // Server certificate file (PEM)
    std::string keyFile;   // Server private key file (PEM)
    std::string password;  // Optional password for private key
};

struct TCPTLSServerContext {
    asio::io_context io_context;
    asio::ssl::context ssl_context;
    asio::executor_work_guard<asio::io_context::executor_type> work_guard;

    TCPTLSServerContext()
        : ssl_context(asio::ssl::context::tls_server)
        , work_guard(asio::make_work_guard(io_context)) {}
};

class ServerTransportTCPTLS : public rpc::ServerTransport {
public:
    ServerTransportTCPTLS(const ServerTransportTLSConfig& config)
        : config_(config) {
        ctx_ = std::make_shared<TCPTLSServerContext>();
        acceptor_ = std::make_unique<asio::ip::tcp::acceptor>(ctx_->io_context);

        // Configure SSL context
        ctx_->ssl_context.set_options(
            asio::ssl::context::default_workarounds |
            asio::ssl::context::no_sslv2 |
            asio::ssl::context::no_sslv3 |
            asio::ssl::context::single_dh_use);

        if (!config_.password.empty()) {
            ctx_->ssl_context.set_password_callback(
                [this](std::size_t, asio::ssl::context_base::password_purpose) {
                    return config_.password;
                });
        }

        ctx_->ssl_context.use_certificate_chain_file(config_.certFile);
        ctx_->ssl_context.use_private_key_file(config_.keyFile, asio::ssl::context::pem);
    }

    ~ServerTransportTCPTLS() {
        close();
    }

    error::Error listen() override {
        if (acceptor_->is_open()) {
            return error::Error("Server is already listening");
        }

        try {
            asio::ip::tcp::endpoint endpoint(asio::ip::tcp::v4(), config_.port);
            acceptor_->open(endpoint.protocol());
            acceptor_->set_option(asio::ip::tcp::acceptor::reuse_address(true));
            acceptor_->bind(endpoint);
            acceptor_->listen();

            startAccept();
        } catch (const std::exception& e) {
            return error::Error("Failed to listen: " + std::string(e.what()));
        }

        return nullptr;
    }

    std::pair<std::shared_ptr<rpc::Connection>, error::Error> accept() override {
        // Drive the IO context to process events
        ctx_->io_context.poll();

        // Reset the io_context so it can be polled again if it ran out of work
        if (ctx_->io_context.stopped()) {
            ctx_->io_context.restart();
        }

        std::lock_guard<std::mutex> lock(mutex_);
        if (pending_connections_.empty()) {
            return {nullptr, nullptr};
        }

        auto conn = pending_connections_.front();
        pending_connections_.pop();
        return {conn, nullptr};
    }

    error::Error close() override {
        if (acceptor_ && acceptor_->is_open()) {
            acceptor_->close();
        }
        return nullptr;
    }

private:
    void startAccept() {
        auto socket = std::make_shared<ssl_socket>(ctx_->io_context, ctx_->ssl_context);

        acceptor_->async_accept(socket->lowest_layer(),
            [this, socket](const std::error_code& ec) {
                if (!ec) {
                    // Perform SSL handshake
                    socket->async_handshake(asio::ssl::stream_base::server,
                        [this, socket](const std::error_code& ec) {
                            if (!ec) {
                                auto conn = std::make_shared<TCPTLSConnection>(socket, config_.noDelay);
                                conn->start();

                                {
                                    std::lock_guard<std::mutex> lock(mutex_);
                                    pending_connections_.push(conn);
                                }
                            }
                            // If handshake fails, just drop the connection
                        });
                }

                if (acceptor_->is_open()) {
                    startAccept();
                }
            });
    }

    ServerTransportTLSConfig config_;
    std::shared_ptr<TCPTLSServerContext> ctx_;
    std::unique_ptr<asio::ip::tcp::acceptor> acceptor_;

    std::queue<std::shared_ptr<TCPTLSConnection>> pending_connections_;
    std::mutex mutex_;
};

} // namespace tcp
} // namespace scg
