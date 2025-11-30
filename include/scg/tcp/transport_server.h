#pragma once

#define ASIO_STANDALONE 1
#include <asio.hpp>
#include <memory>
#include <mutex>
#include <queue>

#include "scg/transport.h"
#include "scg/tcp/connection.h"

namespace scg {
namespace tcp {

struct ServerTransportConfig {
    int port = 8080;
    bool noDelay = true;  // Disable Nagle's algorithm by default
};

class ServerTransportTCP : public rpc::ServerTransport {
public:
    ServerTransportTCP(const ServerTransportConfig& config)
        : config_(config) {
        ctx_ = std::make_shared<TCPContext>();
        acceptor_ = std::make_unique<asio::ip::tcp::acceptor>(ctx_->io_context);
    }

    ~ServerTransportTCP() {
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
        auto socket = std::make_shared<asio::ip::tcp::socket>(ctx_->io_context);
        acceptor_->async_accept(*socket, [this, socket](const std::error_code& ec) {
            if (!ec) {
                auto conn = std::make_shared<TCPConnection>(std::move(*socket), config_.noDelay);
                conn->start();

                {
                    std::lock_guard<std::mutex> lock(mutex_);
                    pending_connections_.push(conn);
                }
            }

            if (acceptor_->is_open()) {
                startAccept();
            }
        });
    }

    ServerTransportConfig config_;
    std::shared_ptr<TCPContext> ctx_;
    std::unique_ptr<asio::ip::tcp::acceptor> acceptor_;

    std::queue<std::shared_ptr<TCPConnection>> pending_connections_;
    std::mutex mutex_;
};

} // namespace tcp
} // namespace scg
