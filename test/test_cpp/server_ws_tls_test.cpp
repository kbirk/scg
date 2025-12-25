#include <cstdio>
#include <thread>
#include <chrono>
#include <atomic>
#include <csignal>

#include "scg/server.h"
#include "scg/ws/transport_server_tls.h"
#include "pingpong/pingpong.h"

std::atomic<bool> running(true);

void signalHandler(int signum) {
    printf("Interrupt signal (%d) received.\n", signum);
    running = false;
}

class PingPongServerImpl : public pingpong::PingPongServer {
public:
    std::pair<pingpong::PongResponse, scg::error::Error> ping(const scg::context::Context& ctx, const pingpong::PingRequest& req) override {
        // Echo back the payload with incremented count
        pingpong::PongResponse response;
        response.pong.count = req.ping.count + 1;
        response.pong.payload = req.ping.payload;

        return std::make_pair(response, nullptr);
    }
};

int main() {
    // Set up signal handler for graceful shutdown
    signal(SIGINT, signalHandler);
    signal(SIGTERM, signalHandler);

    // Configure logging
    scg::log::LoggingConfig logging;
    logging.level = scg::log::LogLevel::INFO;
    logging.debugLogger = [](std::string msg) {
        printf("DEBUG: %s\n", msg.c_str());
    };
    logging.infoLogger = [](std::string msg) {
        printf("INFO: %s\n", msg.c_str());
    };
    logging.warnLogger = [](std::string msg) {
        printf("WARN: %s\n", msg.c_str());
    };
    logging.errorLogger = [](std::string msg) {
        printf("ERROR: %s\n", msg.c_str());
    };

    // Configure transport with TLS
    scg::ws::ServerTransportTLSConfig transportConfig;
    transportConfig.port = 8000;
    transportConfig.certFile = "../test/server.crt";
    transportConfig.keyFile = "../test/server.key";
    transportConfig.logging = logging;

    // Configure server
    scg::rpc::ServerConfig config;
    config.transport = std::make_shared<scg::ws::ServerTransportTLS>(transportConfig);
    config.errorHandler = [](const scg::error::Error& err) {
        printf("Server error: %s\n", err.message.c_str());
    };

    // Create server
    auto server = std::make_shared<scg::rpc::Server>(config);

    // Create and register service implementation
    auto impl = std::make_shared<PingPongServerImpl>();
    pingpong::registerPingPongServer(server.get(), impl);

    // Start server in background thread
    auto err = server->run();
    if (err) {
        printf("Failed to start server: %s\n", err.message.c_str());
        return 1;
    }

    printf("WebSocket TLS server started on port %d\n", transportConfig.port);

    // Wait for shutdown signal
    while (running) {
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }

    // Stop server
    server->shutdown();

    return 0;
}
