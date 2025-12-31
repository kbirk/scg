#include <iostream>
#include <thread>
#include <chrono>
#include <vector>
#include <memory>
#include <atomic>
#include <gperftools/profiler.h>

#include "benchmark.h"
#include "benchmark/benchmark.h"
#include <scg/client.h>
#include <scg/server.h>
#include <scg/context.h>
#include <scg/tcp/transport_client.h>
#include <scg/tcp/transport_server.h>
#include <scg/ws/transport_client.h>
#include <scg/ws/transport_server.h>

using ::benchmark::Benchmark;
using ::benchmark::RunBenchmark;

// BenchmarkServiceImpl implements the benchmark service
class BenchmarkServiceImpl : public benchmark::BenchmarkServiceServer {
public:
	std::pair<benchmark::Response, scg::error::Error> call(const scg::context::Context& ctx, const benchmark::Request& req) override {
		benchmark::Response resp;
		return std::make_pair(resp, nullptr);
	}
};

// Helper to setup and run server in background
class ServerRunner {
public:
	ServerRunner(int port, bool useWebSocket = false) : port_(port), useWebSocket_(useWebSocket), running_(false) {}

	void start() {
		serverThread_ = std::thread([this]() {
			if (useWebSocket_) {
				scg::ws::ServerTransportConfig config;
				config.port = port_;

				auto transport = std::make_shared<scg::ws::ServerTransportWS>(config);

				scg::rpc::ServerConfig serverConfig;
				serverConfig.transport = transport;

				server_ = std::make_unique<scg::rpc::Server>(serverConfig);

				auto impl = std::make_shared<BenchmarkServiceImpl>();
				benchmark::registerBenchmarkServiceServer(server_.get(), impl);

				running_ = true;
				server_->start();
			} else {
				scg::tcp::ServerTransportConfig config;
				config.port = port_;

				auto transport = std::make_shared<scg::tcp::ServerTransportTCP>(config);

				scg::rpc::ServerConfig serverConfig;
				serverConfig.transport = transport;

				server_ = std::make_unique<scg::rpc::Server>(serverConfig);

				auto impl = std::make_shared<BenchmarkServiceImpl>();
				benchmark::registerBenchmarkServiceServer(server_.get(), impl);

				running_ = true;
				server_->start();
			}
		});

		// Wait for server to start
		while (!running_) {
			std::this_thread::sleep_for(std::chrono::milliseconds(10));
		}
		std::this_thread::sleep_for(std::chrono::milliseconds(100));
	}

	void stop() {
		if (server_) {
			server_->shutdown();
		}
		if (serverThread_.joinable()) {
			serverThread_.join();
		}
		running_ = false;
	}

	~ServerRunner() {
		stop();
	}

private:
	int port_;
	bool useWebSocket_;
	std::atomic<bool> running_;
	std::unique_ptr<scg::rpc::Server> server_;
	std::thread serverThread_;
};

// Benchmark Echo with simple message
void BenchmarkRPC_TCP_Echo_Simple(Benchmark& b) {
	const int port = 19000;
	ServerRunner server(port);
	server.start();

	// Setup client
	scg::tcp::ClientTransportConfig transportConfig;
	transportConfig.host = "localhost";
	transportConfig.port = port;

	auto transport = std::make_shared<scg::tcp::ClientTransportTCP>(transportConfig);

	scg::rpc::ClientConfig clientConfig;
	clientConfig.transport = transport;

	auto client = std::make_shared<scg::rpc::Client>(clientConfig);
	client->connect();

	// Wait for connection
	std::this_thread::sleep_for(std::chrono::milliseconds(100));

	benchmark::BenchmarkServiceClient benchmarkClient(client);

	benchmark::Request req;

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		scg::context::Context ctx;
		benchmark::Response resp;
		auto err = benchmarkClient.call(&resp, ctx, req);
		if (err) {
			std::cerr << "Call failed: " << err.message() << std::endl;
			break;
		}
	}
	b.stopTimer();

	client->disconnect();
	server.stop();
}

// Benchmark Echo with long message
void BenchmarkRPC_TCP_Echo_LongMessage(Benchmark& b) {
	const int port = 19001;
	ServerRunner server(port);
	server.start();

	// Setup client
	scg::tcp::ClientTransportConfig transportConfig;
	transportConfig.host = "localhost";
	transportConfig.port = port;

	auto transport = std::make_shared<scg::tcp::ClientTransportTCP>(transportConfig);

	scg::rpc::ClientConfig clientConfig;
	clientConfig.transport = transport;

	auto client = std::make_shared<scg::rpc::Client>(clientConfig);
	client->connect();

	std::this_thread::sleep_for(std::chrono::milliseconds(100));

	benchmark::BenchmarkServiceClient benchmarkClient(client);

	benchmark::Request req;

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		scg::context::Context ctx;
		benchmark::Response resp;
		auto err = benchmarkClient.call(&resp, ctx, req);
		if (err) {
			std::cerr << "Call failed: " << err.message() << std::endl;
			break;
		}
	}
	b.stopTimer();

	client->disconnect();
	server.stop();
}

// Benchmark Process with single item
void BenchmarkRPC_TCP_Process_SingleItem(Benchmark& b) {
	const int port = 19002;
	ServerRunner server(port);
	server.start();

	// Setup client
	scg::tcp::ClientTransportConfig transportConfig;
	transportConfig.host = "localhost";
	transportConfig.port = port;

	auto transport = std::make_shared<scg::tcp::ClientTransportTCP>(transportConfig);

	scg::rpc::ClientConfig clientConfig;
	clientConfig.transport = transport;

	auto client = std::make_shared<scg::rpc::Client>(clientConfig);
	client->connect();

	std::this_thread::sleep_for(std::chrono::milliseconds(100));

	benchmark::BenchmarkServiceClient benchmarkClient(client);

	benchmark::Request req;

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		scg::context::Context ctx;
		benchmark::Response resp;
		auto err = benchmarkClient.call(&resp, ctx, req);
		if (err) {
			std::cerr << "Call failed: " << err.message() << std::endl;
			break;
		}
	}
	b.stopTimer();

	client->disconnect();
	server.stop();
}

// Benchmark Process with multiple items
void BenchmarkRPC_TCP_Process_MultipleItems(Benchmark& b) {
	const int port = 19003;
	ServerRunner server(port);
	server.start();

	// Setup client
	scg::tcp::ClientTransportConfig transportConfig;
	transportConfig.host = "localhost";
	transportConfig.port = port;

	auto transport = std::make_shared<scg::tcp::ClientTransportTCP>(transportConfig);

	scg::rpc::ClientConfig clientConfig;
	clientConfig.transport = transport;

	auto client = std::make_shared<scg::rpc::Client>(clientConfig);
	client->connect();

	std::this_thread::sleep_for(std::chrono::milliseconds(100));

	benchmark::BenchmarkServiceClient benchmarkClient(client);

	benchmark::Request req;

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		scg::context::Context ctx;
		benchmark::Response resp;
		auto err = benchmarkClient.call(&resp, ctx, req);
		if (err) {
			std::cerr << "Call failed: " << err.message() << std::endl;
			break;
		}
	}
	b.stopTimer();

	client->disconnect();
	server.stop();
}

// ======================== WebSocket Transport Benchmarks ========================

// Benchmark Echo with simple message
void BenchmarkRPC_WebSocket_Echo_Simple(Benchmark& b) {
	const int port = 19100;
	ServerRunner server(port, true); // WebSocket server
	server.start();

	// Setup client
	scg::ws::ClientTransportConfig transportConfig;
	transportConfig.host = "localhost";
	transportConfig.port = port;
	transportConfig.path = "/";

	auto transport = std::make_shared<scg::ws::ClientTransportWS>(transportConfig);

	scg::rpc::ClientConfig clientConfig;
	clientConfig.transport = transport;

	auto client = std::make_shared<scg::rpc::Client>(clientConfig);
	client->connect();

	std::this_thread::sleep_for(std::chrono::milliseconds(100));

	benchmark::BenchmarkServiceClient benchmarkClient(client);

	benchmark::Request req;

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		scg::context::Context ctx;
		benchmark::Response resp;
		auto err = benchmarkClient.call(&resp, ctx, req);
		if (err) {
			std::cerr << "Call failed: " << err.message() << std::endl;
			break;
		}
	}
	b.stopTimer();

	client->disconnect();
	server.stop();
}

// Benchmark Echo with long message
void BenchmarkRPC_WebSocket_Echo_LongMessage(Benchmark& b) {
	const int port = 19101;
	ServerRunner server(port, true); // WebSocket server
	server.start();

	// Setup client
	scg::ws::ClientTransportConfig transportConfig;
	transportConfig.host = "localhost";
	transportConfig.port = port;
	transportConfig.path = "/";

	auto transport = std::make_shared<scg::ws::ClientTransportWS>(transportConfig);

	scg::rpc::ClientConfig clientConfig;
	clientConfig.transport = transport;

	auto client = std::make_shared<scg::rpc::Client>(clientConfig);
	client->connect();

	std::this_thread::sleep_for(std::chrono::milliseconds(100));

	benchmark::BenchmarkServiceClient benchmarkClient(client);

	benchmark::Request req;

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		scg::context::Context ctx;
		benchmark::Response resp;
		auto err = benchmarkClient.call(&resp, ctx, req);
		if (err) {
			std::cerr << "Call failed: " << err.message() << std::endl;
			break;
		}
	}
	b.stopTimer();

	client->disconnect();
	server.stop();
}

// Benchmark Process with single item
void BenchmarkRPC_WebSocket_Process_SingleItem(Benchmark& b) {
	const int port = 19102;
	ServerRunner server(port, true); // WebSocket server
	server.start();

	// Setup client
	scg::ws::ClientTransportConfig transportConfig;
	transportConfig.host = "localhost";
	transportConfig.port = port;
	transportConfig.path = "/";

	auto transport = std::make_shared<scg::ws::ClientTransportWS>(transportConfig);

	scg::rpc::ClientConfig clientConfig;
	clientConfig.transport = transport;

	auto client = std::make_shared<scg::rpc::Client>(clientConfig);
	client->connect();

	std::this_thread::sleep_for(std::chrono::milliseconds(100));

	benchmark::BenchmarkServiceClient benchmarkClient(client);

	benchmark::Request req;

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		scg::context::Context ctx;
		benchmark::Response resp;
		auto err = benchmarkClient.call(&resp, ctx, req);
		if (err) {
			std::cerr << "Call failed: " << err.message() << std::endl;
			break;
		}
	}
	b.stopTimer();

	client->disconnect();
	server.stop();
}

// Benchmark Process with multiple items
void BenchmarkRPC_WebSocket_Process_MultipleItems(Benchmark& b) {
	const int port = 19103;
	ServerRunner server(port, true); // WebSocket server
	server.start();

	// Setup client
	scg::ws::ClientTransportConfig transportConfig;
	transportConfig.host = "localhost";
	transportConfig.port = port;
	transportConfig.path = "/";

	auto transport = std::make_shared<scg::ws::ClientTransportWS>(transportConfig);

	scg::rpc::ClientConfig clientConfig;
	clientConfig.transport = transport;

	auto client = std::make_shared<scg::rpc::Client>(clientConfig);
	client->connect();

	std::this_thread::sleep_for(std::chrono::milliseconds(100));

	benchmark::BenchmarkServiceClient benchmarkClient(client);

	benchmark::Request req;

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		scg::context::Context ctx;
		benchmark::Response resp;
		auto err = benchmarkClient.call(&resp, ctx, req);
		if (err) {
			std::cerr << "Call failed: " << err.message() << std::endl;
			break;
		}
	}
	b.stopTimer();

	client->disconnect();
	server.stop();
}

int main(int argc, char** argv) {
	bool profile = false;
	if (argc > 1 && std::string(argv[1]) == "--profile") {
		profile = true;
	}

	if (profile) ProfilerStart("rpc_bench.prof");

	std::cout << "Running C++ RPC Benchmarks..." << std::endl;
	std::cout << std::left << std::setw(40) << "Benchmark"
			  << std::right << std::setw(12) << "Iterations"
			  << std::setw(15) << "ns/op" << std::endl;
	std::cout << std::string(67, '-') << std::endl;

	int iterations = profile ? 10000 : 1000;
	RunBenchmark("BenchmarkRPC_TCP/Echo/Simple", BenchmarkRPC_TCP_Echo_Simple, iterations);
	RunBenchmark("BenchmarkRPC_TCP/Echo/LongMessage", BenchmarkRPC_TCP_Echo_LongMessage, iterations);
	RunBenchmark("BenchmarkRPC_TCP/Process/SingleItem", BenchmarkRPC_TCP_Process_SingleItem, iterations);
	RunBenchmark("BenchmarkRPC_TCP/Process/MultipleItems", BenchmarkRPC_TCP_Process_MultipleItems, iterations);
	RunBenchmark("BenchmarkRPC_WebSocket/Echo/Simple", BenchmarkRPC_WebSocket_Echo_Simple, iterations);
	RunBenchmark("BenchmarkRPC_WebSocket/Echo/LongMessage", BenchmarkRPC_WebSocket_Echo_LongMessage, iterations);
	RunBenchmark("BenchmarkRPC_WebSocket/Process/SingleItem", BenchmarkRPC_WebSocket_Process_SingleItem, iterations);
	RunBenchmark("BenchmarkRPC_WebSocket/Process/MultipleItems", BenchmarkRPC_WebSocket_Process_MultipleItems, iterations);

	if (profile) ProfilerStop();

	return 0;
}
