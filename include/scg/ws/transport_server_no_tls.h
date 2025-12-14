#pragma once

#define ASIO_STANDALONE 1

#include <memory>
#include <mutex>
#include <queue>
#include <websocketpp/config/asio_no_tls.hpp>
#include <websocketpp/server.hpp>

#include "scg/transport.h"
#include "scg/logger.h"

namespace scg {
namespace ws {

struct ServerTransportConfig {
    int port = 8080;
    uint32_t maxSendMessageSize = 0; // 0 for no limit
    uint32_t maxRecvMessageSize = 0; // 0 for no limit
    log::LoggingConfig logging;
};

struct WSServerNoTLSConfig : public websocketpp::config::asio {
    // override logger
    typedef log::LoggerOverride elog_type;
    typedef log::LoggerOverride alog_type;

    struct transport_config : public websocketpp::config::asio::transport_config {
        typedef log::LoggerOverride elog_type;
        typedef log::LoggerOverride alog_type;
    };

    typedef websocketpp::transport::asio::endpoint<transport_config> transport_type;
};

typedef websocketpp::server<WSServerNoTLSConfig> WSServerNoTLS;
typedef websocketpp::connection_hdl WSConnectionHandle;
typedef websocketpp::config::asio::message_type WSMessage;

// WebSocket server connection implementation (no TLS)
class WebSocketServerConnection : public rpc::Connection {
public:
    WebSocketServerConnection(WSServerNoTLS* server, WSConnectionHandle handle, uint32_t maxSendMessageSize = 0, uint32_t maxRecvMessageSize = 0)
        : server_(server), handle_(handle), closed_(false), ready_(false), maxSendMessageSize_(maxSendMessageSize), maxRecvMessageSize_(maxRecvMessageSize) {}

    error::Error send(const std::vector<uint8_t>& data) override {
        std::lock_guard<std::mutex> lock(mu_);

        if (closed_) {
            return error::Error("Connection is closed");
        }

        if (maxSendMessageSize_ > 0 && data.size() > maxSendMessageSize_) {
            return error::Error("Message size exceeds send limit");
        }

        if (closed_) {
            return error::Error("Connection is closed");
        }

        if (!server_) {
            return error::Error("Server is not available");
        }

        std::error_code ec;
        server_->send(handle_, &data[0], data.size(), websocketpp::frame::opcode::binary, ec);
        if (ec) {
            return error::Error("Error sending message: " + ec.message());
        }
        return nullptr;
    }

    void setMessageHandler(std::function<void(const std::vector<uint8_t>&)> handler) override {
        std::vector<std::vector<uint8_t>> buffered;
        {
            std::lock_guard<std::mutex> lock(mu_);
            messageHandler_ = handler;
            // Mark as ready - this signals that all handlers are set up
            ready_ = true;
            while (!pendingMessages_.empty()) {
                buffered.push_back(std::move(pendingMessages_.front()));
                pendingMessages_.pop();
            }
        }

        for (auto& data : buffered) {
            if (handler) {
                handler(data);
            }
        }
    }

    void setFailHandler(std::function<void(const error::Error&)> handler) override {
        std::lock_guard<std::mutex> lock(mu_);
        failHandler_ = handler;
    }

    void setCloseHandler(std::function<void()> handler) override {
        std::lock_guard<std::mutex> lock(mu_);
        closeHandler_ = handler;
    }

    error::Error close() override {
        std::lock_guard<std::mutex> lock(mu_);

        if (closed_) {
            return nullptr;
        }

        if (!server_) {
            return nullptr;
        }

        std::error_code ec;
        server_->close(handle_, websocketpp::close::status::going_away, "Server closing connection", ec);
        closed_ = true;
        if (ec) {
            return error::Error("Error closing connection: " + ec.message());
        }
        return nullptr;
    }

    // Internal methods called by transport
    void onMessage(std::shared_ptr<WSMessage> msg) {
        std::function<void(const std::vector<uint8_t>&)> handler;
        std::function<void(const error::Error&)> failHandler;
        std::vector<uint8_t> data;

        {
            std::lock_guard<std::mutex> lock(mu_);
            if (msg->get_opcode() != websocketpp::frame::opcode::binary) {
                return;
            }

            auto payload = msg->get_payload();
            if (maxRecvMessageSize_ > 0 && payload.size() > maxRecvMessageSize_) {
                failHandler = failHandler_;
                if (failHandler) {
                    failHandler(error::Error("Message size exceeds receive limit"));
                }
                return;
            }

            data = std::vector<uint8_t>(payload.begin(), payload.end());

            if (!ready_ || !messageHandler_) {
                pendingMessages_.push(data);
                return;
            }

            handler = messageHandler_;
        }

        // Call handler without holding lock to avoid deadlock
        if (handler) {
            handler(data);
        }
    }

    void onOpen() {
        // Connection opened - connection is ready
    }

    void onFail(const error::Error& err) {
        std::function<void(const error::Error&)> handler;

        {
            std::lock_guard<std::mutex> lock(mu_);
            closed_ = true;
            handler = failHandler_;
        }

        // Call handler without holding lock to avoid deadlock
        if (handler) {
            handler(err);
        }
    }

    void onClose() {
        std::function<void()> handler;

        {
            std::lock_guard<std::mutex> lock(mu_);
            closed_ = true;
            handler = closeHandler_;
        }

        // Call handler without holding lock to avoid deadlock
        if (handler) {
            handler();
        }
    }

private:
    WSServerNoTLS* server_;
    WSConnectionHandle handle_;
    std::function<void(const std::vector<uint8_t>&)> messageHandler_;
    std::function<void(const error::Error&)> failHandler_;
    std::function<void()> closeHandler_;
    bool closed_;
    bool ready_;  // Set to true when message handler is registered
    uint32_t maxSendMessageSize_;
    uint32_t maxRecvMessageSize_;
    std::queue<std::vector<uint8_t>> pendingMessages_;
    mutable std::mutex mu_;
};

// WebSocket server transport (no TLS)
class ServerTransportNoTLS : public rpc::ServerTransport {
public:
    ServerTransportNoTLS(const ServerTransportConfig& config) : config_(config), running_(false) {
        // set logging parameters
        registerLoggerMethods(server_.get_alog());
        registerLoggerMethods(server_.get_elog());

        server_.set_reuse_addr(true);

        // Initialize ASIO
        server_.init_asio();

        // Set handlers
        server_.set_open_handler([this](WSConnectionHandle hdl) {
            onOpen(hdl);
        });

        server_.set_close_handler([this](WSConnectionHandle hdl) {
            onClose(hdl);
        });

        server_.set_fail_handler([this](WSConnectionHandle hdl) {
            onFail(hdl);
        });

        server_.set_message_handler([this](WSConnectionHandle hdl, std::shared_ptr<WSMessage> msg) {
            onMessage(hdl, msg);
        });

        // Configure for non-blocking operation
        server_.clear_access_channels(websocketpp::log::alevel::all);
        server_.clear_error_channels(websocketpp::log::elevel::all);
    }

    ~ServerTransportNoTLS() {
        close();
    }

    error::Error listen() override {
        std::lock_guard<std::mutex> lock(mu_);

        if (running_) {
            return error::Error("Server is already listening");
        }

        try {
            // Listen on the specified port
            server_.listen(config_.port);

            // Start accepting connections
            server_.start_accept();

            // Set running flag
            running_ = true;

            return nullptr;

        } catch (const websocketpp::exception& e) {
            return error::Error("Failed to listen: " + std::string(e.what()));
        } catch (const std::exception& e) {
            return error::Error("Failed to listen: " + std::string(e.what()));
        }
    }

    std::pair<std::shared_ptr<rpc::Connection>, error::Error> accept() override {
        std::unique_lock<std::mutex> lock(mu_);

        if (!running_) {
            return std::make_pair(nullptr, error::Error("Server is not running"));
        }

        // Check if there are any pending connections
        if (pendingConnections_.empty()) {
            return std::make_pair(nullptr, nullptr);
        }

        // Get the next pending connection
        auto conn = pendingConnections_.front();
        pendingConnections_.pop();

        return std::make_pair(conn, nullptr);
    }

    void poll() override {
        std::unique_lock<std::mutex> lock(mu_);

        if (!running_) {
            return;
        }

        // Poll ASIO for any ready handlers (non-blocking). The poll will invoke
        // our websocket callbacks, so we must release the mutex while it runs to
        // avoid deadlocking when those callbacks try to acquire the same lock.
        lock.unlock();
        try {
            while (server_.poll_one() > 0) {
                // Continue processing while there are ready handlers
            }
        } catch (const std::exception&) {
            // Ignore errors
        }
    }

    error::Error close() override {
        std::lock_guard<std::mutex> lock(mu_);

        if (!running_) {
            return nullptr;
        }

        running_ = false;

        // Stop listening
        try {
            server_.stop_listening();
        } catch (...) {
            // Ignore errors during shutdown
        }

        // Close all active connections
        for (auto& pair : activeConnections_) {
            try {
                pair.second->close();
            } catch (...) {
                // Ignore errors during shutdown
            }
        }
        activeConnections_.clear();

        // Clear pending connections
        while (!pendingConnections_.empty()) {
            pendingConnections_.pop();
        }

        // Stop the server
        try {
            server_.stop();
        } catch (...) {
            // Ignore errors during shutdown
        }

        return nullptr;
    }

private:
    void onOpen(WSConnectionHandle hdl) {
        std::lock_guard<std::mutex> lock(mu_);

        if (!running_) {
            return;
        }

        // Create connection wrapper
        auto conn = std::make_shared<WebSocketServerConnection>(&server_, hdl, config_.maxSendMessageSize, config_.maxRecvMessageSize);
        conn->onOpen();

        // Store in active connections
        activeConnections_[hdl] = conn;

        // Add to pending connections queue for accept()
        pendingConnections_.push(conn);
    }

    void onClose(WSConnectionHandle hdl) {
        std::lock_guard<std::mutex> lock(mu_);

        auto it = activeConnections_.find(hdl);
        if (it != activeConnections_.end()) {
            it->second->onClose();
            activeConnections_.erase(it);
        }
    }

    void onFail(WSConnectionHandle hdl) {
        std::lock_guard<std::mutex> lock(mu_);

        auto it = activeConnections_.find(hdl);
        if (it != activeConnections_.end()) {
            auto conn = server_.get_con_from_hdl(hdl);
            auto err = error::Error(conn->get_ec().message());
            it->second->onFail(err);
            activeConnections_.erase(it);
        }
    }

    void onMessage(WSConnectionHandle hdl, std::shared_ptr<WSMessage> msg) {
        std::lock_guard<std::mutex> lock(mu_);

        auto it = activeConnections_.find(hdl);
        if (it != activeConnections_.end()) {
            it->second->onMessage(msg);
        }
    }

    template <typename Logger>
    void registerLoggerMethods(Logger& logger) {
        logger.registerLoggingFuncs(
            config_.logging.level,
            config_.logging.debugLogger,
            config_.logging.infoLogger,
            config_.logging.warnLogger,
            config_.logging.errorLogger);
    }

    ServerTransportConfig config_;
    WSServerNoTLS server_;
    bool running_;

    std::map<WSConnectionHandle, std::shared_ptr<WebSocketServerConnection>, std::owner_less<WSConnectionHandle>> activeConnections_;
    std::queue<std::shared_ptr<rpc::Connection>> pendingConnections_;

    mutable std::mutex mu_;
};

} // namespace ws
} // namespace scg
