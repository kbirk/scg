#include <cstdio>
#include <thread>
#include <chrono>
#include <atomic>
#include <csignal>

#include "scg/server.h"
#include "scg/tcp/transport_server_tls.h"
#include "pingpong/pingpong.h"

std::atomic<bool> running(true);

void signalHandler(int signum) {
	printf("Interrupt signal (%d) received.\n", signum);
	running = false;
}

class PingPongServerImpl : public pingpong::PingPongServer {
public:
	std::pair<pingpong::PongResponse, scg::error::Error> ping(const scg::context::Context& ctx, const pingpong::PingRequest& req) override {
		// Check for "sleep" metadata
		std::string sleepStr;
		if (!ctx.get(sleepStr, "sleep")) {
			int sleepMs = std::stoi(sleepStr);
			if (sleepMs > 0) {
				std::this_thread::sleep_for(std::chrono::milliseconds(sleepMs));
			}
		}

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

	// Configure transport
	scg::tcp::ServerTransportTLSConfig transportConfig;
	transportConfig.port = 9002;
	transportConfig.certFile = "../../server.crt";
	transportConfig.keyFile = "../../server.key";

	// Configure server
	scg::rpc::ServerConfig config;
	config.transport = std::make_shared<scg::tcp::ServerTransportTCPTLS>(transportConfig);
	config.errorHandler = [](const scg::error::Error& err) {
		printf("Server error: %s\n", err.message().c_str());
	};

	// Create server
	auto server = std::make_shared<scg::rpc::Server>(config);

	// Create and register service implementation
	auto impl = std::make_shared<PingPongServerImpl>();
	pingpong::registerPingPongServer(server.get(), impl);

	// Start server in background thread
	auto err = server->start();
	if (err) {
		printf("Failed to start server: %s\n", err.message().c_str());
		return 1;
	}

	printf("TLS TCP Server started on port 9002\n");

	// Wait for shutdown signal
	while (running) {
		std::this_thread::sleep_for(std::chrono::milliseconds(100));
	}

	// Stop server
	server->shutdown();

	return 0;
}
