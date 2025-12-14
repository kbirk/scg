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
        : config_(config), acceptor_(io_context_) {
    }

    ~ServerTransportTCP() {
        close();
    }

    error::Error listen() override {
        try {
            asio::ip::tcp::endpoint endpoint(asio::ip::tcp::v4(), config_.port);
            acceptor_.open(endpoint.protocol());
            acceptor_.set_option(asio::ip::tcp::acceptor::reuse_address(true));
            acceptor_.bind(endpoint);
            acceptor_.listen();
            return nullptr;
        } catch (const std::exception& e) {
            return error::Error(e.what());
        }
    }

    std::pair<std::shared_ptr<scg::rpc::Connection>, error::Error> accept() override {
        try {
            asio::ip::tcp::socket socket(io_context_);
            asio::error_code ec;
            acceptor_.non_blocking(true);
            acceptor_.accept(socket, ec);

            if (ec) {
                if (ec == asio::error::would_block || ec == asio::error::try_again) {
                    return {nullptr, nullptr};
                }
                return {nullptr, error::Error(ec.message())};
            }

            // Connection accepted
            return {std::make_shared<ConnectionTCP>(std::move(socket), config_.maxSendMessageSize, config_.maxRecvMessageSize), nullptr};
        } catch (const std::exception& e) {
            return {nullptr, error::Error(e.what())};
        }
    }

    void poll() override {
        if (io_context_.stopped()) {
            io_context_.restart();
        }
        io_context_.poll();
    }

    error::Error close() override {
        if (acceptor_.is_open()) {
            acceptor_.close();
        }
        return nullptr;
    }

private:
    ServerTransportConfig config_;
    asio::io_context io_context_;
    asio::ip::tcp::acceptor acceptor_;
};

} // namespace tcp
} // namespace scg
