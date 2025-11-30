#pragma once

#define ASIO_STANDALONE 1
#include <asio.hpp>
#include <asio/ssl.hpp>
#include <thread>
#include <memory>
#include <future>

#include "scg/tcp/connection_tls.h"

namespace scg {
namespace tcp {

struct ClientTransportTLSConfig {
    std::string host = "localhost";
    int port = 8443;
    bool noDelay = true;  // Disable Nagle's algorithm by default
    bool verifyPeer = false;  // For testing with self-signed certs
    std::string caFile;  // Optional CA certificate file for verification
};

// Client-specific TLS context with tls_client mode
struct ClientTCPTLSContext {
    asio::io_context io_context;
    asio::ssl::context ssl_context;
    asio::executor_work_guard<asio::io_context::executor_type> work_guard;

    ClientTCPTLSContext()
        : ssl_context(asio::ssl::context::tls_client),
          work_guard(asio::make_work_guard(io_context)) {}
};

class ClientTransportTCPTLS : public rpc::ClientTransport {
public:
    ClientTransportTCPTLS(const ClientTransportTLSConfig& config)
        : config_(config) {

        ctx_ = std::make_shared<ClientTCPTLSContext>();

        // Configure SSL context
        ctx_->ssl_context.set_options(
            asio::ssl::context::default_workarounds |
            asio::ssl::context::no_sslv2 |
            asio::ssl::context::no_sslv3 |
            asio::ssl::context::single_dh_use);

        if (config_.verifyPeer) {
            ctx_->ssl_context.set_verify_mode(asio::ssl::verify_peer);
            if (!config_.caFile.empty()) {
                ctx_->ssl_context.load_verify_file(config_.caFile);
            } else {
                ctx_->ssl_context.set_default_verify_paths();
            }
        } else {
            ctx_->ssl_context.set_verify_mode(asio::ssl::verify_none);
        }

        thread_ = std::thread([this]() {
            ctx_->io_context.run();
        });
    }

    ~ClientTransportTCPTLS() {
        shutdown();
    }

    std::pair<std::shared_ptr<rpc::Connection>, error::Error> connect() override {
        asio::ip::tcp::resolver resolver(ctx_->io_context);

        std::error_code resolve_ec;
        auto endpoints = resolver.resolve(config_.host, std::to_string(config_.port), resolve_ec);
        if (resolve_ec) {
            return {nullptr, error::Error("Failed to resolve host: " + resolve_ec.message())};
        }

        auto socket = std::make_shared<ssl_socket>(ctx_->io_context, ctx_->ssl_context);

        // Set SNI hostname
        if (!SSL_set_tlsext_host_name(socket->native_handle(), config_.host.c_str())) {
            return {nullptr, error::Error("Failed to set SNI hostname")};
        }

        std::promise<error::Error> connect_promise;
        auto connect_future = connect_promise.get_future();

        asio::async_connect(socket->lowest_layer(), endpoints,
            [&connect_promise, socket, this](const std::error_code& ec, const asio::ip::tcp::endpoint& /*endpoint*/) {
                if (ec) {
                    connect_promise.set_value(error::Error(ec.message()));
                    return;
                }

                // Perform SSL handshake
                socket->async_handshake(asio::ssl::stream_base::client,
                    [&connect_promise](const std::error_code& ec) {
                        if (ec) {
                            connect_promise.set_value(error::Error("SSL handshake failed: " + ec.message()));
                        } else {
                            connect_promise.set_value(nullptr);
                        }
                    });
            });

        auto err = connect_future.get();
        if (err) {
            return {nullptr, err};
        }

        auto conn = std::make_shared<TCPTLSConnection>(socket, config_.noDelay);
        conn->start();
        return {conn, nullptr};
    }

    void shutdown() override {
        if (ctx_ && !ctx_->io_context.stopped()) {
            ctx_->work_guard.reset();
            ctx_->io_context.stop();
        }
        if (thread_.joinable()) {
            thread_.join();
        }
    }

private:
    ClientTransportTLSConfig config_;
    std::shared_ptr<ClientTCPTLSContext> ctx_;
    std::thread thread_;
};

} // namespace tcp
} // namespace scg
