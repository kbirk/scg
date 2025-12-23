#include <cstdio>
#include <thread>
#include <chrono>
#include <atomic>
#include <csignal>

#include "scg/server.h"
#include "scg/unix/transport_server.h"
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
	scg::unix_socket::ServerTransportConfig transportConfig;
	transportConfig.socketPath = "/tmp/scg_test_unix_0.sock";

	// Configure server
	scg::rpc::ServerConfig config;
	config.transport = std::make_shared<scg::unix_socket::ServerTransportUnix>(transportConfig);
	config.errorHandler = [](const scg::error::Error& err) {
		printf("Server error: %s\n", err.message.c_str());
	};

	// Create server
	auto server = std::make_shared<scg::rpc::Server>(config);

	// Create and register service implementation
	auto impl = std::make_shared<PingPongServerImpl>();
	pingpong::registerPingPongServer(server.get(), impl);

	// Start server (non-blocking)
	auto err = server->start();
	if (err) {
		printf("Failed to start server: %s\n", err.message.c_str());
		return 1;
	}

	printf("Server started on Unix socket: %s\n", transportConfig.socketPath.c_str());

	// Main loop - poll for messages
	while (running) {
		// Process any pending messages/connections
		server->process();

		// Small sleep to avoid busy-waiting
		// std::this_thread::sleep_for(std::chrono::milliseconds(1));
	}

	// Stop server
	server->stop();

	return 0;
}
