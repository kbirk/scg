#pragma once

#include <cstdio>
#include <thread>
#include <chrono>
#include <functional>
#include <memory>
#include <vector>
#include <atomic>
#include <mutex>
#include <string>
#include <stdexcept>

#include <acutest.h>

#include "scg/serialize.h"
#include "scg/server.h"
#include "scg/client.h"
#include "scg/logger.h"
#include "scg/middleware.h"
#include "pingpong/pingpong.h"
#include "../files/output/basic/service.h"

// ============================================================================
// Transport Factory Interface (similar to Go's TransportFactory)
// ============================================================================

// TransportFactory creates server and client transports for testing
// The id parameter allows tests to use unique endpoints (ports/paths)
struct TransportFactory {
	std::function<std::shared_ptr<scg::rpc::ServerTransport>(int id)> createServerTransport;
	std::function<std::shared_ptr<scg::rpc::ClientTransport>(int id)> createClientTransport;
	// Creates a client transport with message size limits for testing MaxMessageSize
	std::function<std::shared_ptr<scg::rpc::ClientTransport>(int id)> createLimitedClientTransport;
	std::string name;
};

// TestSuiteConfig holds configuration for running the test suite
struct TestSuiteConfig {
	TransportFactory factory;
	int startingId = 0;		  // Starting port/ID for test isolation
	int maxRetries = 10;		 // Connection retry count
	bool useExternalServer = false;  // If true, assumes a server is already running externally
	bool skipGroupTests = false;	 // Skip server group tests
	bool skipEdgeTests = false;	  // Skip edge case tests
};

// ============================================================================
// Constants
// ============================================================================

const std::string VALID_TOKEN = "1234";
const std::string INVALID_TOKEN = "invalid";

// ============================================================================
// Server Implementations
// ============================================================================

// PingPong server implementation that echoes back with sleep support
class PingPongServerImpl : public pingpong::PingPongServer {
public:
	std::pair<pingpong::PongResponse, scg::error::Error> ping(
		const scg::context::Context& ctx,
		const pingpong::PingRequest& req
	) override {
		// Check for "sleep" metadata (for timeout testing)
		std::string sleepStr;
		if (!ctx.get(sleepStr, "sleep")) {
			int sleepMs = std::stoi(sleepStr);
			if (sleepMs > 0) {
				std::this_thread::sleep_for(std::chrono::milliseconds(sleepMs));
			}
		}

		pingpong::PongResponse response;
		response.pong.count = req.ping.count + 1;
		response.pong.payload = req.ping.payload;
		return std::make_pair(response, nullptr);
	}
};

// PingPong server implementation that always fails
class PingPongServerFail : public pingpong::PingPongServer {
public:
	std::pair<pingpong::PongResponse, scg::error::Error> ping(
		const scg::context::Context& ctx,
		const pingpong::PingRequest& req
	) override {
		return std::make_pair(pingpong::PongResponse{}, scg::error::Error("unable to ping the pong"));
	}
};

// TesterA server implementation
class TesterAServerImpl : public basic::TesterAServer {
public:
	std::pair<basic::TestResponseA, scg::error::Error> test(
		const scg::context::Context& ctx,
		const basic::TestRequestA& req
	) override {
		basic::TestResponseA response;
		response.a = req.a;
		return std::make_pair(response, nullptr);
	}
};

// TesterB server implementation
class TesterBServerImpl : public basic::TesterBServer {
public:
	std::pair<basic::TestResponseB, scg::error::Error> test(
		const scg::context::Context& ctx,
		const basic::TestRequestB& req
	) override {
		basic::TestResponseB response;
		response.b = req.b;
		return std::make_pair(response, nullptr);
	}
};

// ============================================================================
// Middleware
// ============================================================================

// Auth middleware for testing
inline std::pair<std::shared_ptr<scg::type::Message>, scg::error::Error> authMiddleware(
	scg::context::Context& ctx,
	const scg::type::Message& req,
	scg::middleware::Handler next
) {
	std::string token;
	auto err = ctx.get(token, "token");
	if (err) {
		return std::make_pair(nullptr, scg::error::Error("no metadata"));
	}
	if (token != VALID_TOKEN) {
		return std::make_pair(nullptr, scg::error::Error("invalid token"));
	}
	return next(ctx, req);
}

// ============================================================================
// Test Helpers
// ============================================================================

// Helper to create the test payload
inline pingpong::TestPayload createTestPayload(uint32_t i) {
	pingpong::NestedPayload nested1;
	nested1.valString = "nested" + std::to_string(i);
	nested1.valDouble = 3.14 + i;

	pingpong::NestedPayload nested2;
	nested2.valString = "nested again" + std::to_string(i);
	nested2.valDouble = 123.34563456 + i;

	pingpong::NestedEmpty nested;
	nested.empty = pingpong::Empty();

	pingpong::TestPayload payload;
	payload.valUint8 = i + 1;
	payload.valUint16 = 256 + i + 2;
	payload.valUint32 = 65535 + i + 3;
	payload.valUint64 = 4294967295ULL + i + 4;
	payload.valInt8 = -(i + 5);
	payload.valInt16 = -128 - (i + 6);
	payload.valInt32 = -32768 - (i + 7);
	payload.valInt64 = -2147483648LL - (i + 8);
	payload.valFloat = 3.14f + i + 9;
	payload.valDouble = -3.14159 + i + 10;
	payload.valString = "hello world " + std::to_string(i + 11);
	payload.valTimestamp = scg::type::timestamp();
	payload.valBool = i % 2 == 0;
	payload.valEnum = pingpong::EnumType::ENUM_TYPE_1;
	payload.valUUID = scg::type::uuid::random();
	payload.valListPayload = {nested1, nested2};
	payload.valMapKeyEnum = {
		{pingpong::KeyType("key_" + std::to_string(i+1)), pingpong::EnumType::ENUM_TYPE_1},
		{pingpong::KeyType("key_" + std::to_string(i+2)), pingpong::EnumType::ENUM_TYPE_2}
	};
	payload.valEmpty = pingpong::Empty();
	payload.valNestedEmpty = nested;
	payload.valByteArray = {0, 1, 2, 3, 4, 5, 6, 7, 8, 9};

	return payload;
}

// Helper function to verify the test payload matches expected values
inline void verifyTestPayload(
	const pingpong::TestPayload& result,
	const pingpong::TestPayload& expected,
	uint32_t i
) {
	TEST_CHECK(result.valUint8 == expected.valUint8);
	TEST_CHECK(result.valUint16 == expected.valUint16);
	TEST_CHECK(result.valUint32 == expected.valUint32);
	TEST_CHECK(result.valUint64 == expected.valUint64);
	TEST_CHECK(result.valInt8 == expected.valInt8);
	TEST_CHECK(result.valInt16 == expected.valInt16);
	TEST_CHECK(result.valInt32 == expected.valInt32);
	TEST_CHECK(result.valInt64 == expected.valInt64);
	TEST_CHECK(result.valFloat == expected.valFloat);
	TEST_CHECK(result.valDouble == expected.valDouble);
	TEST_CHECK(result.valString == expected.valString);
	TEST_CHECK(result.valBool == expected.valBool);
	TEST_CHECK(result.valEnum == expected.valEnum);
	TEST_CHECK(result.valUUID == expected.valUUID);
	TEST_CHECK(result.valListPayload.size() == 2);
	TEST_CHECK(result.valListPayload[0].valString == expected.valListPayload[0].valString);
	TEST_CHECK(result.valListPayload[0].valDouble == expected.valListPayload[0].valDouble);
	TEST_CHECK(result.valListPayload[1].valString == expected.valListPayload[1].valString);
	TEST_CHECK(result.valListPayload[1].valDouble == expected.valListPayload[1].valDouble);
	TEST_CHECK(result.valMapKeyEnum.size() == 2);
	TEST_CHECK(result.valMapKeyEnum.at(pingpong::KeyType("key_" + std::to_string(i+1))) == pingpong::EnumType::ENUM_TYPE_1);
	TEST_CHECK(result.valMapKeyEnum.at(pingpong::KeyType("key_" + std::to_string(i+2))) == pingpong::EnumType::ENUM_TYPE_2);
}

// Helper to connect client with retries
inline bool connectWithRetries(std::shared_ptr<scg::rpc::Client>& client, int maxRetries) {
	for (int i = 0; i < maxRetries; i++) {
		auto err = client->connect();
		if (!err) return true;
		std::this_thread::sleep_for(std::chrono::milliseconds(100));
	}
	return false;
}

// ============================================================================
// Test Runner Context - manages server lifecycle for each test
// ============================================================================

class TestContext {
public:
	TestContext(
		const TransportFactory& factory,
		int id,
		int maxRetries,
		bool useExternalServer
	) : factory_(factory), id_(id), maxRetries_(maxRetries), useExternalServer_(useExternalServer) {}

	// Start server with PingPong service (basic implementation)
	void startServer() {
		if (useExternalServer_) return;

		scg::rpc::ServerConfig serverConfig;
		serverConfig.transport = factory_.createServerTransport(id_);

		server_ = std::make_shared<scg::rpc::Server>(serverConfig);
		impl_ = std::make_shared<PingPongServerImpl>();
		pingpong::registerPingPongServer(server_.get(), impl_);

		auto err = server_->start();
		TEST_CHECK(!err);
		if (err) {
			printf("Failed to start server: %s\n", err.message.c_str());
			return;
		}
	}

	// Start server with custom setup callback
	template<typename SetupFunc>
	void startServerWithSetup(SetupFunc setup) {
		if (useExternalServer_) return;

		scg::rpc::ServerConfig serverConfig;
		serverConfig.transport = factory_.createServerTransport(id_);

		server_ = std::make_shared<scg::rpc::Server>(serverConfig);
		setup(server_.get());

		auto err = server_->start();
		TEST_CHECK(!err);
		if (err) {
			printf("Failed to start server: %s\n", err.message.c_str());
			return;
		}
	}

	// Create and connect a client
	std::shared_ptr<scg::rpc::Client> createClient() {
		scg::rpc::ClientConfig clientConfig;
		clientConfig.transport = factory_.createClientTransport(id_);
		auto client = std::make_shared<scg::rpc::Client>(clientConfig);

		TEST_CHECK(connectWithRetries(client, maxRetries_));

		return client;
	}

	// Create and connect a client with message size limits
	std::shared_ptr<scg::rpc::Client> createLimitedClient() {
		if (!factory_.createLimitedClientTransport) {
			return createClient();
		}

		scg::rpc::ClientConfig clientConfig;
		clientConfig.transport = factory_.createLimitedClientTransport(id_);
		auto client = std::make_shared<scg::rpc::Client>(clientConfig);

		TEST_CHECK(connectWithRetries(client, maxRetries_));

		return client;
	}

	// Stop the server
	void stopServer() {
		if (useExternalServer_) return;

		if (server_) {
			server_->shutdown();
		}
	}

	scg::rpc::Server* server() { return server_.get(); }
	bool isUsingExternalServer() const { return useExternalServer_; }
	int maxRetries() const { return maxRetries_; }
	const TransportFactory& factory() const { return factory_; }
	int id() const { return id_; }

private:
	const TransportFactory& factory_;
	int id_;
	int maxRetries_;
	bool useExternalServer_;

	std::shared_ptr<scg::rpc::Server> server_;
	std::shared_ptr<PingPongServerImpl> impl_;
};

// ============================================================================
// Individual Tests (like Go's runXxxTest functions)
// ============================================================================

// Test basic ping-pong functionality
inline void runPingPongTest(TestContext& ctx) {
	printf("Running PingPong test...\n");

	ctx.startServer();
	auto client = ctx.createClient();

	pingpong::PingPongClient pingPongClient(client);

	const uint32_t COUNT = 10;
	for (uint32_t i = 0; i < COUNT; i++) {
		scg::context::Context context;
		context.put("token", VALID_TOKEN);

		pingpong::TestPayload payload = createTestPayload(i);

		pingpong::PingRequest req;
		req.ping.count = i;
		req.ping.payload = payload;

		auto [res, err] = pingPongClient.ping(context, req);
		TEST_CHECK(err == nullptr);
		if (err) {
			printf("ERROR: %s\n", err.message.c_str());
			break;
		}
		TEST_CHECK(res.pong.count == int32_t(i+1));
		verifyTestPayload(res.pong.payload, payload, i);

		std::this_thread::sleep_for(std::chrono::milliseconds(50));
	}

	client->disconnect();
	ctx.stopServer();
	printf("PingPong test passed\n");
}

// Test concurrent ping-pong requests
inline void runPingPongConcurrentTest(TestContext& ctx) {
	printf("Running PingPong Concurrent test...\n");

	ctx.startServer();
	auto client = ctx.createClient();

	pingpong::PingPongClient pingPongClient(client);

	const int NUM_THREADS = 10;
	const int REQUESTS_PER_THREAD = 20;

	std::atomic<int> successCount{0};
	std::atomic<int> errorCount{0};
	std::vector<std::thread> threads;

	printf("Starting %d threads, each sending %d requests\n", NUM_THREADS, REQUESTS_PER_THREAD);

	for (int t = 0; t < NUM_THREADS; t++) {
		threads.emplace_back([&, t]() {
			for (int j = 0; j < REQUESTS_PER_THREAD; j++) {
				int32_t expectedCount = t * REQUESTS_PER_THREAD + j;
				std::string expectedPayload = "thread-" + std::to_string(t) + "-request-" + std::to_string(j);

				scg::context::Context context;
				context.put("token", VALID_TOKEN);
				pingpong::PingRequest req;
				req.ping.count = expectedCount;
				req.ping.payload.valString = expectedPayload;

				auto [res, err] = pingPongClient.ping(context, req);

				if (err) {
					errorCount++;
					continue;
				}

				if (res.pong.count != expectedCount + 1 || res.pong.payload.valString != expectedPayload) {
					errorCount++;
					continue;
				}

				successCount++;
			}
		});
	}

	for (auto& thread : threads) {
		thread.join();
	}

	int totalRequests = NUM_THREADS * REQUESTS_PER_THREAD;
	printf("Completed: %d successful, %d errors out of %d total requests\n",
		   successCount.load(), errorCount.load(), totalRequests);

	TEST_CHECK(successCount.load() == totalRequests);
	TEST_CHECK(errorCount.load() == 0);

	client->disconnect();
	ctx.stopServer();
	printf("PingPong Concurrent test passed\n");
}

// Test middleware on client and server
inline void runMiddlewareTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) {
		printf("Skipping Middleware test (using external server)\n");
		return;
	}

	printf("Running Middleware test...\n");

	std::atomic<int> serverMiddlewareCount{0};

	ctx.startServerWithSetup([&](scg::rpc::Server* server) {
		server->addMiddleware([&serverMiddlewareCount](
			scg::context::Context& ctx,
			const scg::type::Message& req,
			scg::middleware::Handler next
		) -> std::pair<std::shared_ptr<scg::type::Message>, scg::error::Error> {
			serverMiddlewareCount++;
			return next(ctx, req);
		});

		auto impl = std::make_shared<PingPongServerImpl>();
		pingpong::registerPingPongServer(server, impl);
	});

	auto client = ctx.createClient();

	int clientMiddlewareCount = 0;
	client->middleware([&clientMiddlewareCount](
		scg::context::Context& ctx,
		const scg::type::Message& req,
		scg::middleware::Handler next
	) -> std::pair<std::shared_ptr<scg::type::Message>, scg::error::Error> {
		clientMiddlewareCount++;
		return next(ctx, req);
	});

	pingpong::PingPongClient pingPongClient(client);

	const int COUNT = 5;
	for (int i = 0; i < COUNT; i++) {
		scg::context::Context context;
		pingpong::PingRequest req;
		req.ping.count = i;

		auto [res, err] = pingPongClient.ping(context, req);
		TEST_CHECK(err == nullptr);
		TEST_CHECK(res.pong.count == i + 1);
	}

	TEST_CHECK(clientMiddlewareCount == COUNT);
	TEST_CHECK(serverMiddlewareCount == COUNT);
	printf("Client middleware invoked %d times, server middleware invoked %d times\n",
		   clientMiddlewareCount, serverMiddlewareCount.load());

	client->disconnect();
	ctx.stopServer();
	printf("Middleware test passed\n");
}

// Test auth middleware rejection
inline void runAuthFailTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) {
		printf("Skipping Auth Fail test (using external server)\n");
		return;
	}

	printf("Running Auth Fail test...\n");

	ctx.startServerWithSetup([](scg::rpc::Server* server) {
		server->addMiddleware(authMiddleware);
		auto impl = std::make_shared<PingPongServerImpl>();
		pingpong::registerPingPongServer(server, impl);
	});

	auto client = ctx.createClient();
	pingpong::PingPongClient pingPongClient(client);

	scg::context::Context context;
	context.put("token", INVALID_TOKEN);
	pingpong::PingRequest req;
	req.ping.count = 1;

	auto [res, err] = pingPongClient.ping(context, req);
	TEST_CHECK(err != nullptr);
	if (err) {
		TEST_CHECK(err.message == "invalid token");
		printf("Got expected error: %s\n", err.message.c_str());
	}

	client->disconnect();
	ctx.stopServer();
	printf("Auth Fail test passed\n");
}

// Test auth middleware success
inline void runAuthSuccessTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) {
		printf("Skipping Auth Success test (using external server)\n");
		return;
	}

	printf("Running Auth Success test...\n");

	ctx.startServerWithSetup([](scg::rpc::Server* server) {
		server->addMiddleware(authMiddleware);
		auto impl = std::make_shared<PingPongServerImpl>();
		pingpong::registerPingPongServer(server, impl);
	});

	auto client = ctx.createClient();
	pingpong::PingPongClient pingPongClient(client);

	scg::context::Context context;
	context.put("token", VALID_TOKEN);
	pingpong::PingRequest req;
	req.ping.count = 1;

	auto [res, err] = pingPongClient.ping(context, req);
	TEST_CHECK(err == nullptr);
	TEST_CHECK(res.pong.count == 2);

	client->disconnect();
	ctx.stopServer();
	printf("Auth Success test passed\n");
}

// Test server returns error
inline void runServerErrorTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) {
		printf("Skipping Server Error test (using external server)\n");
		return;
	}

	printf("Running Server Error test...\n");

	ctx.startServerWithSetup([](scg::rpc::Server* server) {
		auto impl = std::make_shared<PingPongServerFail>();
		pingpong::registerPingPongServer(server, impl);
	});

	auto client = ctx.createClient();
	pingpong::PingPongClient pingPongClient(client);

	scg::context::Context context;
	pingpong::PingRequest req;
	req.ping.count = 1;

	auto [res, err] = pingPongClient.ping(context, req);
	TEST_CHECK(err != nullptr);
	if (err) {
		TEST_CHECK(err.message == "unable to ping the pong");
		printf("Got expected error: %s\n", err.message.c_str());
	}

	client->disconnect();
	ctx.stopServer();
	printf("Server Error test passed\n");
}

// Test server groups with isolated middleware
inline void runServerGroupsTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) {
		printf("Skipping Server Groups test (using external server)\n");
		return;
	}

	printf("Running Server Groups test...\n");

	ctx.startServerWithSetup([](scg::rpc::Server* server) {
		auto testerAImpl = std::make_shared<TesterAServerImpl>();
		auto testerBImpl = std::make_shared<TesterBServerImpl>();

		// Group A has auth middleware
		server->group([=](scg::rpc::Server* s) {
			s->addMiddleware(authMiddleware);
			basic::registerTesterAServer(s, testerAImpl);
		});

		// Group B has no middleware
		server->group([=](scg::rpc::Server* s) {
			basic::registerTesterBServer(s, testerBImpl);
		});
	});

	auto client = ctx.createClient();

	basic::TesterAClient clientA(client);
	basic::TesterBClient clientB(client);

	// Test A without token - should fail
	{
		scg::context::Context context;
		basic::TestRequestA req;
		req.a = "A";

		auto [res, err] = clientA.test(context, req);
		TEST_CHECK(err != nullptr);
		if (err) {
			TEST_CHECK(err.message == "no metadata");
			printf("TesterA without token: %s (expected)\n", err.message.c_str());
		}
	}

	// Test B without token - should succeed (no middleware)
	{
		scg::context::Context context;
		basic::TestRequestB req;
		req.b = "B";

		auto [res, err] = clientB.test(context, req);
		TEST_CHECK(err == nullptr);
		TEST_CHECK(res.b == "B");
		printf("TesterB without token: success (expected)\n");
	}

	client->disconnect();
	ctx.stopServer();
	printf("Server Groups test passed\n");
}

// Test nested server groups with cascading middleware
inline void runServerNestedGroupsTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) {
		printf("Skipping Server Nested Groups test (using external server)\n");
		return;
	}

	printf("Running Server Nested Groups test...\n");

	// Create a middleware that always rejects
	auto alwaysRejectMiddleware = [](
		scg::context::Context& ctx,
		const scg::type::Message& req,
		scg::middleware::Handler next
	) -> std::pair<std::shared_ptr<scg::type::Message>, scg::error::Error> {
		return std::make_pair(nullptr, scg::error::Error("rejected"));
	};

	ctx.startServerWithSetup([&](scg::rpc::Server* server) {
		auto testerAImpl = std::make_shared<TesterAServerImpl>();
		auto testerBImpl = std::make_shared<TesterBServerImpl>();

		// Outer group has auth middleware
		server->group([=](scg::rpc::Server* s) {
			s->addMiddleware(authMiddleware);
			basic::registerTesterAServer(s, testerAImpl);

			// Nested group adds reject middleware (both auth AND reject apply)
			s->group([=, &alwaysRejectMiddleware](scg::rpc::Server* inner) {
				inner->addMiddleware(alwaysRejectMiddleware);
				basic::registerTesterBServer(inner, testerBImpl);
			});
		});
	});

	auto client = ctx.createClient();

	basic::TesterAClient clientA(client);
	basic::TesterBClient clientB(client);

	// Test A without token - should fail with "no metadata" (auth middleware)
	{
		scg::context::Context context;
		basic::TestRequestA req;
		req.a = "A";

		auto [res, err] = clientA.test(context, req);
		TEST_CHECK(err != nullptr);
		if (err) {
			TEST_CHECK(err.message == "no metadata");
			printf("TesterA without token: %s (expected)\n", err.message.c_str());
		}
	}

	// Test B with valid token - should fail with "rejected" (passes auth but rejected by nested middleware)
	{
		scg::context::Context context;
		context.put("token", VALID_TOKEN);
		basic::TestRequestB req;
		req.b = "B";

		auto [res, err] = clientB.test(context, req);
		TEST_CHECK(err != nullptr);
		if (err) {
			TEST_CHECK(err.message == "rejected");
			printf("TesterB with valid token: %s (expected - rejected by nested middleware)\n", err.message.c_str());
		}
	}

	client->disconnect();
	ctx.stopServer();
	printf("Server Nested Groups test passed\n");
}

// Test duplicate service registration throws
inline void runDuplicateServiceTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) {
		printf("Skipping Duplicate Service test (using external server)\n");
		return;
	}

	printf("Running Duplicate Service test...\n");

	scg::rpc::ServerConfig serverConfig;
	serverConfig.transport = ctx.factory().createServerTransport(ctx.id());

	auto server = std::make_shared<scg::rpc::Server>(serverConfig);

	auto impl1 = std::make_shared<PingPongServerImpl>();
	auto impl2 = std::make_shared<PingPongServerImpl>();

	pingpong::registerPingPongServer(server.get(), impl1);

	bool threw = false;
	try {
		pingpong::registerPingPongServer(server.get(), impl2);
	} catch (const std::exception& e) {
		threw = true;
		printf("Got expected exception: %s\n", e.what());
	}

	TEST_CHECK(threw);
	printf("Duplicate Service test passed\n");
}

// Test graceful shutdown
inline void runGracefulShutdownTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) {
		printf("Skipping Graceful Shutdown test (using external server)\n");
		return;
	}

	printf("Running Graceful Shutdown test...\n");

	ctx.startServer();
	auto client = ctx.createClient();

	pingpong::PingPongClient pingPongClient(client);

	std::atomic<int> successCount{0};
	std::atomic<int> errorCount{0};
	std::vector<std::thread> clientThreads;

	// Start multiple requests
	for (int i = 0; i < 10; i++) {
		clientThreads.emplace_back([&, i]() {
			scg::context::Context context;
			pingpong::PingRequest req;
			req.ping.count = i;

			auto [res, err] = pingPongClient.ping(context, req);
			if (err) {
				errorCount++;
			} else {
				successCount++;
			}
		});
	}

	// Small delay then shutdown
	std::this_thread::sleep_for(std::chrono::milliseconds(10));

	ctx.stopServer();

	// Wait for all client threads
	for (auto& t : clientThreads) {
		t.join();
	}

	printf("Graceful shutdown: %d successful, %d errors\n",
		   successCount.load(), errorCount.load());

	// All requests should complete (either success or error)
	TEST_CHECK(successCount + errorCount == 10);

	client->disconnect();
	printf("Graceful Shutdown test passed\n");
}

// Test high concurrency with request/response verification
inline void runConcurrencyTest(TestContext& ctx) {
	printf("Running Concurrency test...\n");

	ctx.startServer();
	auto client = ctx.createClient();

	pingpong::PingPongClient pingPongClient(client);

	const int NUM_THREADS = 50;
	const int REQUESTS_PER_THREAD = 20;

	std::atomic<int> successCount{0};
	std::atomic<int> errorCount{0};
	std::vector<std::thread> threads;

	printf("Starting %d threads, each sending %d requests\n", NUM_THREADS, REQUESTS_PER_THREAD);

	for (int t = 0; t < NUM_THREADS; t++) {
		threads.emplace_back([&, t]() {
			for (int j = 0; j < REQUESTS_PER_THREAD; j++) {
				int32_t expectedCount = t * REQUESTS_PER_THREAD + j;
				std::string expectedPayload = "thread-" + std::to_string(t) + "-request-" + std::to_string(j);

				scg::context::Context context;
				context.put("token", VALID_TOKEN);
				pingpong::PingRequest req;
				req.ping.count = expectedCount;
				req.ping.payload.valString = expectedPayload;

				auto [res, err] = pingPongClient.ping(context, req);

				if (err) {
					errorCount++;
					continue;
				}

				if (res.pong.count != expectedCount + 1) {
					errorCount++;
					continue;
				}

				if (res.pong.payload.valString != expectedPayload) {
					errorCount++;
					continue;
				}

				successCount++;
			}
		});
	}

	for (auto& thread : threads) {
		thread.join();
	}

	int totalRequests = NUM_THREADS * REQUESTS_PER_THREAD;
	printf("Completed: %d successful, %d errors out of %d total requests\n",
		   successCount.load(), errorCount.load(), totalRequests);

	TEST_CHECK(successCount.load() == totalRequests);
	TEST_CHECK(errorCount.load() == 0);

	client->disconnect();
	ctx.stopServer();
	printf("Concurrency test passed\n");
}

// Test empty payload
inline void runEmptyPayloadTest(TestContext& ctx) {
	printf("Running Empty Payload test...\n");

	ctx.startServer();
	auto client = ctx.createClient();

	pingpong::PingPongClient pingPongClient(client);

	scg::context::Context context;
	context.put("token", VALID_TOKEN);
	pingpong::PingRequest req;
	req.ping.count = 0;
	// Leave payload at default values

	auto [res, err] = pingPongClient.ping(context, req);
	TEST_CHECK(err == nullptr);
	TEST_CHECK(res.pong.count == 1);

	client->disconnect();
	ctx.stopServer();
	printf("Empty Payload test passed\n");
}

// Test context metadata
inline void runContextMetadataTest(TestContext& ctx) {
	printf("Running Context Metadata test...\n");

	ctx.startServer();
	auto client = ctx.createClient();

	pingpong::PingPongClient pingPongClient(client);

	scg::context::Context context;
	context.put("key1", "value1");
	context.put("key2", "value2");
	context.put("token", VALID_TOKEN);

	pingpong::PingRequest req;
	req.ping.count = 42;

	auto [res, err] = pingPongClient.ping(context, req);
	TEST_CHECK(err == nullptr);
	TEST_CHECK(res.pong.count == 43);

	client->disconnect();
	ctx.stopServer();
	printf("Context Metadata test passed\n");
}

// Test sequential requests
inline void runSequentialRequestsTest(TestContext& ctx) {
	printf("Running Sequential Requests test...\n");

	ctx.startServer();
	auto client = ctx.createClient();

	pingpong::PingPongClient pingPongClient(client);

	const int NUM_REQUESTS = 100;
	printf("Sending %d sequential requests...\n", NUM_REQUESTS);

	for (int i = 0; i < NUM_REQUESTS; i++) {
		scg::context::Context context;
		context.put("token", VALID_TOKEN);
		pingpong::PingRequest req;
		req.ping.count = i;

		auto [res, err] = pingPongClient.ping(context, req);
		TEST_CHECK(err == nullptr);
		TEST_CHECK(res.pong.count == i + 1);
	}

	printf("All %d sequential requests completed successfully\n", NUM_REQUESTS);

	client->disconnect();
	ctx.stopServer();
	printf("Sequential Requests test passed\n");
}

// Test large payload
inline void runLargePayloadTest(TestContext& ctx) {
	printf("Running Large Payload test...\n");

	ctx.startServer();
	auto client = ctx.createClient();

	pingpong::PingPongClient pingPongClient(client);

	struct TestCase {
		const char* name;
		size_t size;
	};

	TestCase testCases[] = {
		{"Small 1KB", 1024},
		{"Medium 100KB", 100 * 1024},
		{"Large 500KB", 500 * 1024},
	};

	for (const auto& tc : testCases) {
		printf("Testing %s payload...\n", tc.name);

		std::string largePayload(tc.size, 'x');

		scg::context::Context context;
		context.put("token", VALID_TOKEN);
		pingpong::PingRequest req;
		req.ping.count = 1;
		req.ping.payload.valString = largePayload;

		auto [res, err] = pingPongClient.ping(context, req);
		TEST_CHECK(err == nullptr);
		TEST_CHECK(res.pong.payload.valString.size() == tc.size);
		printf("%s payload succeeded\n", tc.name);
	}

	client->disconnect();
	ctx.stopServer();
	printf("Large Payload test passed\n");
}

// Test max message size
inline void runMaxMessageSizeTest(TestContext& ctx) {
	if (!ctx.factory().createLimitedClientTransport) {
		printf("Skipping Max Message Size test (no limited transport factory)\n");
		return;
	}

	printf("Running Max Message Size test...\n");

	ctx.startServer();
	auto client = ctx.createLimitedClient();

	pingpong::PingPongClient pingPongClient(client);

	// 1. Send a small message (should succeed)
	{
		scg::context::Context context;
		pingpong::PingRequest req;
		req.ping.count = 1;
		req.ping.payload.valString = "small";

		auto [res, err] = pingPongClient.ping(context, req);
		TEST_CHECK(err == nullptr);
		printf("Small message succeeded\n");
	}

	// 2. Send a message that generates a response larger than the limit
	{
		scg::context::Context context;
		pingpong::PingRequest req;
		req.ping.count = 2;
		req.ping.payload.valString = std::string(2048, 'b');

		auto [res, err] = pingPongClient.ping(context, req);
		TEST_CHECK(err != nullptr);
		if (err) printf("Large message failed as expected: %s\n", err.message.c_str());
	}

	client->disconnect();
	ctx.stopServer();
	printf("Max Message Size test passed\n");
}

// Test context timeout
inline void runContextTimeoutTest(TestContext& ctx) {
	printf("Running Context Timeout test...\n");

	ctx.startServer();
	auto client = ctx.createClient();

	pingpong::PingPongClient pingPongClient(client);

	// Test timeout
	{
		scg::context::Context context;
		context.setDeadline(std::chrono::system_clock::now() + std::chrono::milliseconds(100));
		context.put("sleep", "500");

		pingpong::PingRequest req;
		req.ping.count = 2;
		req.ping.payload.valString = "timeout";

		auto [res, err] = pingPongClient.ping(context, req);
		TEST_CHECK(err != nullptr);
		if (err) {
			printf("Timeout test passed: %s\n", err.message.c_str());
		}
	}

	client->disconnect();
	ctx.stopServer();
	printf("Context Timeout test passed\n");
}

// Test multiple clients
inline void runMultipleClientsTest(TestContext& ctx) {
	printf("Running Multiple Clients test...\n");

	ctx.startServer();

	const int NUM_CLIENTS = 5;
	const int REQUESTS_PER_CLIENT = 10;

	std::atomic<int> successCount{0};
	std::atomic<int> errorCount{0};
	std::vector<std::thread> threads;

	printf("Starting %d clients, each sending %d requests\n", NUM_CLIENTS, REQUESTS_PER_CLIENT);

	for (int c = 0; c < NUM_CLIENTS; c++) {
		threads.emplace_back([&, c]() {
			scg::rpc::ClientConfig clientConfig;
			clientConfig.transport = ctx.factory().createClientTransport(ctx.id());
			auto client = std::make_shared<scg::rpc::Client>(clientConfig);

			if (!connectWithRetries(client, ctx.maxRetries())) {
				errorCount += REQUESTS_PER_CLIENT;
				return;
			}

			pingpong::PingPongClient pingPongClient(client);

			for (int j = 0; j < REQUESTS_PER_CLIENT; j++) {
				scg::context::Context context;
				context.put("token", VALID_TOKEN);
				pingpong::PingRequest req;
				req.ping.count = c * 1000 + j;

				auto [res, err] = pingPongClient.ping(context, req);

				if (err || res.pong.count != c * 1000 + j + 1) {
					errorCount++;
				} else {
					successCount++;
				}
			}

			client->disconnect();
		});
	}

	for (auto& thread : threads) {
		thread.join();
	}

	int totalRequests = NUM_CLIENTS * REQUESTS_PER_CLIENT;
	printf("Multiple clients: %d successful out of %d requests\n",
		   successCount.load(), totalRequests);

	TEST_CHECK(successCount.load() == totalRequests);
	TEST_CHECK(errorCount.load() == 0);

	ctx.stopServer();
	printf("Multiple Clients test passed\n");
}

// Test rapid connection churn
inline void runRapidConnectionChurnTest(TestContext& ctx) {
	printf("Running Rapid Connection Churn test...\n");

	ctx.startServer();

	const int NUM_ITERATIONS = 20;
	printf("Starting %d rapid connection iterations\n", NUM_ITERATIONS);

	for (int i = 0; i < NUM_ITERATIONS; i++) {
		scg::rpc::ClientConfig clientConfig;
		clientConfig.transport = ctx.factory().createClientTransport(ctx.id());
		auto client = std::make_shared<scg::rpc::Client>(clientConfig);

		TEST_CHECK(connectWithRetries(client, ctx.maxRetries()));

		pingpong::PingPongClient pingPongClient(client);

		scg::context::Context context;
		context.put("token", VALID_TOKEN);
		pingpong::PingRequest req;
		req.ping.count = i;

		auto [res, err] = pingPongClient.ping(context, req);
		TEST_CHECK(err == nullptr);
		TEST_CHECK(res.pong.count == i + 1);

		client->disconnect();
	}

	printf("All rapid connection iterations completed successfully\n");

	ctx.stopServer();
	printf("Rapid Connection Churn test passed\n");
}

// ============================================================================
// Main Test Suite Runner (like Go's RunTestSuite)
// ============================================================================

inline void runTestSuite(const TestSuiteConfig& config) {
	int id = config.startingId;

	// Basic tests (run with both internal and external server)
	{
		printf("\n=== Running PingPong Test ===\n");
		TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
		runPingPongTest(ctx);
	}

	{
		printf("\n=== Running PingPong Concurrent Test ===\n");
		TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
		runPingPongConcurrentTest(ctx);
	}

	// Tests that require server control (skip when using external server)
	if (!config.useExternalServer) {
		{
			printf("\n=== Running Auth Fail Test ===\n");
			TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
			runAuthFailTest(ctx);
		}

		{
			printf("\n=== Running Auth Success Test ===\n");
			TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
			runAuthSuccessTest(ctx);
		}

		{
			printf("\n=== Running Middleware Test ===\n");
			TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
			runMiddlewareTest(ctx);
		}

		{
			printf("\n=== Running Server Error Test ===\n");
			TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
			runServerErrorTest(ctx);
		}

		if (!config.skipGroupTests) {
			{
				printf("\n=== Running Server Groups Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runServerGroupsTest(ctx);
			}

			{
				printf("\n=== Running Server Nested Groups Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runServerNestedGroupsTest(ctx);
			}

			{
				printf("\n=== Running Duplicate Service Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runDuplicateServiceTest(ctx);
			}
		}

		if (!config.skipEdgeTests) {
			{
				printf("\n=== Running Graceful Shutdown Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runGracefulShutdownTest(ctx);
			}

			{
				printf("\n=== Running Large Payload Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runLargePayloadTest(ctx);
			}

			{
				printf("\n=== Running Max Message Size Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runMaxMessageSizeTest(ctx);
			}

			{
				printf("\n=== Running Context Timeout Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runContextTimeoutTest(ctx);
			}

			{
				printf("\n=== Running Multiple Clients Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runMultipleClientsTest(ctx);
			}

			{
				printf("\n=== Running Rapid Connection Churn Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runRapidConnectionChurnTest(ctx);
			}
		}
	}

	// Tests that work with external server
	{
		printf("\n=== Running Concurrency Test ===\n");
		TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
		runConcurrencyTest(ctx);
	}

	{
		printf("\n=== Running Empty Payload Test ===\n");
		TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
		runEmptyPayloadTest(ctx);
	}

	{
		printf("\n=== Running Context Metadata Test ===\n");
		TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
		runContextMetadataTest(ctx);
	}

	{
		printf("\n=== Running Sequential Requests Test ===\n");
		TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
		runSequentialRequestsTest(ctx);
	}

	printf("\n=== All Tests Completed ===\n");
}
