#pragma once

#define ASIO_STANDALONE 1
#include <asio.hpp>
#include <thread>
#include <memory>
#include <future>

#include "scg/tcp/connection.h"

namespace scg {
namespace tcp {

struct ClientTransportConfig {
    std::string host = "localhost";
    int port = 8080;
    bool noDelay = true;  // Disable Nagle's algorithm by default
};

class ClientTransportTCP : public rpc::ClientTransport {
public:
    ClientTransportTCP(const ClientTransportConfig& config)
        : config_(config) {

        ctx_ = std::make_shared<TCPContext>();
        thread_ = std::thread([this]() {
            ctx_->io_context.run();
        });
    }

    ~ClientTransportTCP() {
        shutdown();
    }

    std::pair<std::shared_ptr<rpc::Connection>, error::Error> connect() override {
        asio::ip::tcp::resolver resolver(ctx_->io_context);
        auto endpoints = resolver.resolve(config_.host, std::to_string(config_.port));

        auto socket = std::make_shared<asio::ip::tcp::socket>(ctx_->io_context);

        std::promise<error::Error> promise;
        auto future = promise.get_future();

        asio::async_connect(*socket, endpoints,
            [&promise](const std::error_code& ec, const asio::ip::tcp::endpoint& /*endpoint*/) {
                if (!ec) {
                    promise.set_value(nullptr);
                } else {
                    promise.set_value(error::Error(ec.message()));
                }
            });

        auto err = future.get();
        if (err) {
            return {nullptr, err};
        }

        auto conn = std::make_shared<TCPConnection>(std::move(*socket), config_.noDelay);
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
    ClientTransportConfig config_;
    std::shared_ptr<TCPContext> ctx_;
    std::thread thread_;
};

} // namespace tcp
} // namespace scg
