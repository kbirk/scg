#pragma once

#define ASIO_STANDALONE
#include <asio.hpp>

#include "scg/transport.h"
#include "scg/unix/transport_client.h"
#include <memory>
#include <string>
#include <vector>
#include <unistd.h>

namespace scg {
namespace unix_socket {

struct ServerTransportConfig {
    std::string socketPath;
    uint32_t maxSendMessageSize = 0; // 0 for no limit
    uint32_t maxRecvMessageSize = 0; // 0 for no limit
};

class ServerTransportUnix : public scg::rpc::ServerTransport {
public:
    ServerTransportUnix(const ServerTransportConfig& config)
        : config_(config), acceptor_(io_context_) {
        // Remove existing socket file if it exists
        ::unlink(config_.socketPath.c_str());
    }

    ~ServerTransportUnix() {
        close();
    }

    error::Error listen() override {
        try {
            asio::local::stream_protocol::endpoint endpoint(config_.socketPath);
            acceptor_.open(endpoint.protocol());
            acceptor_.bind(endpoint);
            acceptor_.listen();
            return nullptr;
        } catch (const std::exception& e) {
            return error::Error(e.what());
        }
    }

    std::pair<std::shared_ptr<scg::rpc::Connection>, error::Error> accept() override {
        try {
            asio::local::stream_protocol::socket socket(io_context_);
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
            return {std::make_shared<ConnectionUnix>(std::move(socket), config_.maxSendMessageSize, config_.maxRecvMessageSize), nullptr};
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
        ::unlink(config_.socketPath.c_str());
        return nullptr;
    }

private:
    ServerTransportConfig config_;
    asio::io_context io_context_;
    asio::local::stream_protocol::acceptor acceptor_;
};

} // namespace unix_socket
} // namespace scg
