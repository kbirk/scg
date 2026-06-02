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
#include <fstream>

#include <acutest.h>

#include "scg/serialize.h"
#include "scg/server.h"
#include "scg/client.h"
#include "scg/logger.h"
#include "scg/middleware.h"
#include "pingpong/pingpong.h"
#include "basic/service.h"
#include "chat_impl.h"

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

// The Chat streaming service implementation (ChatServerImpl) is defined in the
// shared chat_impl.h so the standalone cross-language test servers behave
// identically to the in-process suite.

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
			printf("Failed to start server: %s\n", err.message().c_str());
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
			printf("Failed to start server: %s\n", err.message().c_str());
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
			printf("ERROR: %s\n", err.message().c_str());
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
		TEST_CHECK(err.message() == "invalid token");
		printf("Got expected error: %s\n", err.message().c_str());
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
		TEST_CHECK(err.message() == "unable to ping the pong");
		printf("Got expected error: %s\n", err.message().c_str());
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
			TEST_CHECK(err.message() == "no metadata");
			printf("TesterA without token: %s (expected)\n", err.message().c_str());
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
			TEST_CHECK(err.message() == "no metadata");
			printf("TesterA without token: %s (expected)\n", err.message().c_str());
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
			TEST_CHECK(err.message() == "rejected");
			printf("TesterB with valid token: %s (expected - rejected by nested middleware)\n", err.message().c_str());
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
		if (err) printf("Large message failed as expected: %s\n", err.message().c_str());
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
			printf("Timeout test passed: %s\n", err.message().c_str());
		}
	}

	client->disconnect();
	ctx.stopServer();
	printf("Context Timeout test passed\n");
}

// Test that after a context timeout, the client can still make successful calls.
// This catches bugs where a timed-out request leaves orphaned state that causes
// the client to disconnect or deadlock when the late response arrives.
inline void runContextTimeoutRecoveryTest(TestContext& ctx) {
	printf("Running Context Timeout Recovery test...\n");

	ctx.startServer();
	auto client = ctx.createClient();

	pingpong::PingPongClient pingPongClient(client);

	// Call 1: Force a timeout. Server sleeps 500ms, client deadline is 100ms.
	{
		scg::context::Context context;
		context.setDeadline(std::chrono::system_clock::now() + std::chrono::milliseconds(100));
		context.put("sleep", "500");

		pingpong::PingRequest req;
		req.ping.count = 1;

		auto [res, err] = pingPongClient.ping(context, req);
		TEST_CHECK(err != nullptr);
		if (err) {
			printf("Call 1 timed out as expected: %s\n", err.message().c_str());
		}
	}

	// Wait for the server to finish processing and send the late response.
	std::this_thread::sleep_for(std::chrono::milliseconds(600));

	// Call 2: Should succeed. If the client disconnected or deadlocked from the
	// orphaned response, this will fail.
	{
		scg::context::Context context;
		context.setDeadline(std::chrono::system_clock::now() + std::chrono::milliseconds(3000));

		pingpong::PingRequest req;
		req.ping.count = 42;

		auto [res, err] = pingPongClient.ping(context, req);
		TEST_CHECK(err == nullptr);
		if (err) {
			printf("ERROR: Call 2 failed after timeout — client is broken: %s\n", err.message().c_str());
		} else {
			TEST_CHECK(res.pong.count == 43);
			printf("Call 2 succeeded after timeout recovery\n");
		}
	}

	client->disconnect();
	ctx.stopServer();
	printf("Context Timeout Recovery test passed\n");
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
// Streaming tests
// ============================================================================

inline void registerStreamingServices(scg::rpc::Server* server) {
	auto pp = std::make_shared<PingPongServerImpl>();
	pingpong::registerPingPongServer(server, pp);
	auto chat = std::make_shared<ChatServerImpl>();
	pingpong::registerChatServer(server, chat);
}

// Full bidi lifecycle: server push on open, client send + server echo,
// half-close, final summary, then a clean close.
inline void runStreamBidiTest(TestContext& ctx) {
	printf("Running Stream Bidi test...\n");

	// startServerWithSetup / stopServer are no-ops against an external server, so
	// this runs unchanged in-process and cross-language.
	ctx.startServerWithSetup(registerStreamingServices);
	auto client = ctx.createClient();
	pingpong::ChatClient chatClient(client);

	scg::context::Context context;
	auto [stream, err] = chatClient.connect(context);
	TEST_CHECK(err == nullptr);
	if (err) { ctx.stopServer(); return; }

	auto welcome = stream->recv();
	TEST_CHECK(welcome.state == scg::rpc::StreamRecvState::Message);
	TEST_CHECK(welcome.message.text == "welcome");

	const int N = 10;
	for (int i = 0; i < N; i++) {
		pingpong::ChatMessage m;
		m.text = "m" + std::to_string(i);
		m.seq = i;
		TEST_CHECK(stream->send(m) == nullptr);

		auto echo = stream->recv();
		TEST_CHECK(echo.state == scg::rpc::StreamRecvState::Message);
		TEST_CHECK(echo.message.text == "echo:m" + std::to_string(i));
		TEST_CHECK(echo.message.seq == i + 1);
	}

	TEST_CHECK(stream->closeSend() == nullptr);

	auto summary = stream->recv();
	TEST_CHECK(summary.state == scg::rpc::StreamRecvState::Message);
	TEST_CHECK(summary.message.text == "summary");
	TEST_CHECK(summary.message.seq == N);

	auto end = stream->recv();
	TEST_CHECK(end.state == scg::rpc::StreamRecvState::Closed);
	TEST_CHECK(end.error == nullptr); // clean EOF

	client->disconnect();
	ctx.stopServer();
	printf("Stream Bidi test passed\n");
}

// A server-side handler error surfaces on the client's next recv().
inline void runStreamServerErrorTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) return;
	printf("Running Stream Server Error test...\n");

	ctx.startServerWithSetup(registerStreamingServices);
	auto client = ctx.createClient();
	pingpong::ChatClient chatClient(client);

	scg::context::Context context;
	auto [stream, err] = chatClient.connect(context);
	TEST_CHECK(err == nullptr);
	if (err) { ctx.stopServer(); return; }

	auto welcome = stream->recv();
	TEST_CHECK(welcome.state == scg::rpc::StreamRecvState::Message);

	pingpong::ChatMessage m;
	m.text = "fail";
	m.seq = 1;
	TEST_CHECK(stream->send(m) == nullptr);

	auto r = stream->recv();
	TEST_CHECK(r.state == scg::rpc::StreamRecvState::Closed);
	TEST_CHECK(r.error != nullptr);
	if (r.error) {
		TEST_CHECK(r.error.message() == "requested failure");
	}

	client->disconnect();
	ctx.stopServer();
	printf("Stream Server Error test passed\n");
}

// Non-blocking poll/drain — the recommended game-loop consumer model.
inline void runStreamPollTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) return;
	printf("Running Stream Poll test...\n");

	ctx.startServerWithSetup(registerStreamingServices);
	auto client = ctx.createClient();
	pingpong::ChatClient chatClient(client);

	scg::context::Context context;
	auto [stream, err] = chatClient.connect(context);
	TEST_CHECK(err == nullptr);
	if (err) { ctx.stopServer(); return; }

	const int N = 5;
	for (int i = 0; i < N; i++) {
		pingpong::ChatMessage m;
		m.text = "p" + std::to_string(i);
		m.seq = i;
		TEST_CHECK(stream->send(m) == nullptr);
	}

	// Drain non-blocking until we've collected the welcome + N echoes.
	std::vector<std::string> got;
	const int expected = N + 1;
	for (int iter = 0; iter < 400 && (int)got.size() < expected; iter++) {
		bool closed = false;
		for (;;) {
			auto r = stream->tryRecv();
			if (r.state == scg::rpc::StreamRecvState::Empty) {
				break;
			}
			if (r.state == scg::rpc::StreamRecvState::Closed) {
				closed = true;
				break;
			}
			got.push_back(r.message.text);
		}
		if (closed) break;
		std::this_thread::sleep_for(std::chrono::milliseconds(5));
	}

	TEST_CHECK((int)got.size() == expected);
	if (!got.empty()) {
		TEST_CHECK(got[0] == "welcome");
	}

	client->disconnect();
	ctx.stopServer();
	printf("Stream Poll test passed\n");
}

// Multiple independent streams multiplexed on one connection.
inline void runStreamConcurrentTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) return;
	printf("Running Stream Concurrent test...\n");

	ctx.startServerWithSetup(registerStreamingServices);
	auto client = ctx.createClient();
	pingpong::ChatClient chatClient(client);

	const int NUM_STREAMS = 8;
	const int MSGS = 20;
	std::atomic<int> errorCount{0};
	std::vector<std::thread> threads;

	for (int s = 0; s < NUM_STREAMS; s++) {
		threads.emplace_back([&, s]() {
			scg::context::Context context;
			auto [stream, err] = chatClient.connect(context);
			if (err) { errorCount++; return; }

			auto welcome = stream->recv();
			if (welcome.state != scg::rpc::StreamRecvState::Message || welcome.message.text != "welcome") {
				errorCount++;
				return;
			}

			for (int i = 0; i < MSGS; i++) {
				std::string text = "s" + std::to_string(s) + "-m" + std::to_string(i);
				pingpong::ChatMessage m;
				m.text = text;
				m.seq = i;
				if (stream->send(m)) { errorCount++; return; }

				auto echo = stream->recv();
				if (echo.state != scg::rpc::StreamRecvState::Message || echo.message.text != "echo:" + text || echo.message.seq != i + 1) {
					errorCount++;
					return;
				}
			}

			if (stream->closeSend()) { errorCount++; return; }
			auto summary = stream->recv();
			if (summary.state != scg::rpc::StreamRecvState::Message) { errorCount++; return; }
			auto end = stream->recv();
			if (end.state != scg::rpc::StreamRecvState::Closed || end.error != nullptr) { errorCount++; return; }
		});
	}

	for (auto& t : threads) {
		t.join();
	}

	TEST_CHECK(errorCount.load() == 0);

	client->disconnect();
	ctx.stopServer();
	printf("Stream Concurrent test passed\n");
}

// Dropping the connection fails in-flight streams with an error.
inline void runStreamConnectionDropTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) return;
	printf("Running Stream Connection Drop test...\n");

	ctx.startServerWithSetup(registerStreamingServices);
	auto client = ctx.createClient();
	pingpong::ChatClient chatClient(client);

	scg::context::Context context;
	auto [stream, err] = chatClient.connect(context);
	TEST_CHECK(err == nullptr);
	if (err) { ctx.stopServer(); return; }

	auto welcome = stream->recv();
	TEST_CHECK(welcome.state == scg::rpc::StreamRecvState::Message);

	// Drop the connection out from under the live stream.
	client->disconnect();

	auto r = stream->recv();
	TEST_CHECK(r.state == scg::rpc::StreamRecvState::Closed);
	TEST_CHECK(r.error != nullptr);

	ctx.stopServer();
	printf("Stream Connection Drop test passed\n");
}

// Stream OPEN is gated by server middleware (auth validated once on open).
inline void runStreamAuthFailTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) return;
	printf("Running Stream Auth Fail test...\n");

	ctx.startServerWithSetup([](scg::rpc::Server* server) {
		server->addMiddleware(authMiddleware);
		registerStreamingServices(server);
	});
	auto client = ctx.createClient();
	pingpong::ChatClient chatClient(client);

	// No token -> auth middleware rejects on OPEN.
	{
		scg::context::Context context;
		auto [stream, err] = chatClient.connect(context);
		TEST_CHECK(err == nullptr);
		if (!err) {
			auto r = stream->recv();
			TEST_CHECK(r.state == scg::rpc::StreamRecvState::Closed);
			TEST_CHECK(r.error != nullptr);
			if (r.error) {
				TEST_CHECK(r.error.message() == "no metadata");
			}
		}
	}

	// With a valid token the stream opens and the welcome push arrives.
	{
		scg::context::Context context;
		context.put("token", VALID_TOKEN);
		auto [stream, err] = chatClient.connect(context);
		TEST_CHECK(err == nullptr);
		if (!err) {
			auto welcome = stream->recv();
			TEST_CHECK(welcome.state == scg::rpc::StreamRecvState::Message);
			TEST_CHECK(welcome.message.text == "welcome");
		}
	}

	client->disconnect();
	ctx.stopServer();
	printf("Stream Auth Fail test passed\n");
}

// Send from one thread while receiving on another on the same stream.
inline void runStreamConcurrentSendRecvTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) return;
	printf("Running Stream Concurrent Send/Recv test...\n");

	ctx.startServerWithSetup(registerStreamingServices);
	auto client = ctx.createClient();
	pingpong::ChatClient chatClient(client);

	scg::context::Context context;
	auto connectResult = chatClient.connect(context);
	auto stream = connectResult.first;
	TEST_CHECK(connectResult.second == nullptr);
	if (connectResult.second) { ctx.stopServer(); return; }

	const int N = 200;
	std::thread sender([&]() {
		for (int i = 0; i < N; i++) {
			pingpong::ChatMessage m;
			m.text = "m" + std::to_string(i);
			m.seq = i;
			stream->send(m);
		}
		stream->closeSend();
	});

	int echoes = 0;
	for (;;) {
		auto r = stream->recv();
		if (r.state == scg::rpc::StreamRecvState::Closed) {
			break;
		}
		if (r.message.text != "welcome" && r.message.text != "summary") {
			echoes++;
		}
	}
	sender.join();

	TEST_CHECK(echoes == N);

	client->disconnect();
	ctx.stopServer();
	printf("Stream Concurrent Send/Recv test passed\n");
}

// send() after closeSend() returns an error.
inline void runStreamSendAfterCloseSendTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) return;
	printf("Running Stream Send-After-CloseSend test...\n");

	ctx.startServerWithSetup(registerStreamingServices);
	auto client = ctx.createClient();
	pingpong::ChatClient chatClient(client);

	scg::context::Context context;
	auto [stream, err] = chatClient.connect(context);
	TEST_CHECK(err == nullptr);
	if (err) { ctx.stopServer(); return; }

	auto welcome = stream->recv();
	TEST_CHECK(welcome.state == scg::rpc::StreamRecvState::Message);

	TEST_CHECK(stream->closeSend() == nullptr);

	pingpong::ChatMessage late;
	late.text = "late";
	TEST_CHECK(stream->send(late) != nullptr);

	client->disconnect();
	ctx.stopServer();
	printf("Stream Send-After-CloseSend test passed\n");
}

// A large payload round-trips over a stream.
inline void runStreamLargeMessageTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) return;
	printf("Running Stream Large Message test...\n");

	ctx.startServerWithSetup(registerStreamingServices);
	auto client = ctx.createClient();
	pingpong::ChatClient chatClient(client);

	scg::context::Context context;
	auto [stream, err] = chatClient.connect(context);
	TEST_CHECK(err == nullptr);
	if (err) { ctx.stopServer(); return; }

	auto welcome = stream->recv();
	TEST_CHECK(welcome.state == scg::rpc::StreamRecvState::Message);

	std::string big(256 * 1024, 'x');
	pingpong::ChatMessage m;
	m.text = big;
	m.seq = 1;
	TEST_CHECK(stream->send(m) == nullptr);

	auto echo = stream->recv();
	TEST_CHECK(echo.state == scg::rpc::StreamRecvState::Message);
	TEST_CHECK(echo.message.text == "echo:" + big);

	client->disconnect();
	ctx.stopServer();
	printf("Stream Large Message test passed\n");
}

// A slow reader whose bounded buffer overflows has its stream terminated with
// an overflow error (the connection and other streams are unaffected).
inline void runStreamBackpressureTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) return;
	printf("Running Stream Backpressure test...\n");

	ctx.startServerWithSetup(registerStreamingServices);

	// Tiny receive buffer so a flood overflows quickly.
	scg::rpc::ClientConfig clientConfig;
	clientConfig.transport = ctx.factory().createClientTransport(ctx.id());
	clientConfig.streamRecvBufferSize = 4;
	auto client = std::make_shared<scg::rpc::Client>(clientConfig);
	TEST_CHECK(connectWithRetries(client, ctx.maxRetries()));

	pingpong::ChatClient chatClient(client);
	scg::context::Context context;
	auto connectResult = chatClient.connect(context);
	auto stream = connectResult.first;
	TEST_CHECK(connectResult.second == nullptr);
	if (connectResult.second) { ctx.stopServer(); return; }

	pingpong::ChatMessage flood;
	flood.text = "flood";
	TEST_CHECK(stream->send(flood) == nullptr);

	// Deliberately don't read for a moment so the bounded buffer overflows.
	std::this_thread::sleep_for(std::chrono::milliseconds(250));

	bool gotOverflow = false;
	for (int i = 0; i < 200; i++) {
		auto r = stream->recv();
		if (r.state == scg::rpc::StreamRecvState::Closed) {
			gotOverflow = (r.error && r.error.message().find("overflow") != std::string::npos);
			break;
		}
	}
	TEST_CHECK(gotOverflow);

	client->disconnect();
	ctx.stopServer();
	printf("Stream Backpressure test passed\n");
}

// The per-connection stream cap rejects streams beyond the limit.
inline void runStreamMaxConcurrentTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) return;
	printf("Running Stream Max Concurrent test...\n");

	scg::rpc::ServerConfig serverConfig;
	serverConfig.transport = ctx.factory().createServerTransport(ctx.id());
	serverConfig.maxConcurrentStreams = 2;
	auto server = std::make_shared<scg::rpc::Server>(serverConfig);
	registerStreamingServices(server.get());
	auto serr = server->start();
	TEST_CHECK(!serr);
	if (serr) return;

	auto client = ctx.createClient();
	pingpong::ChatClient chatClient(client);
	scg::context::Context context;

	auto r1 = chatClient.connect(context);
	TEST_CHECK(r1.second == nullptr);
	auto w1 = r1.first->recv();
	TEST_CHECK(w1.state == scg::rpc::StreamRecvState::Message);
	TEST_CHECK(w1.message.text == "welcome");

	auto r2 = chatClient.connect(context);
	TEST_CHECK(r2.second == nullptr);
	auto w2 = r2.first->recv();
	TEST_CHECK(w2.state == scg::rpc::StreamRecvState::Message);

	// Third exceeds the cap and is rejected on open.
	auto r3 = chatClient.connect(context);
	TEST_CHECK(r3.second == nullptr);
	auto e3 = r3.first->recv();
	TEST_CHECK(e3.state == scg::rpc::StreamRecvState::Closed);
	TEST_CHECK(e3.error != nullptr);
	if (e3.error) {
		TEST_CHECK(e3.error.message().find("max concurrent streams") != std::string::npos);
	}

	client->disconnect();
	server->shutdown();
	printf("Stream Max Concurrent test passed\n");
}

// Server-streaming form: the client sends a single request and reads a stream.
inline void runStreamServerStreamingTest(TestContext& ctx) {
	printf("Running Stream Server-Streaming test...\n");

	ctx.startServerWithSetup(registerStreamingServices);
	auto client = ctx.createClient();
	pingpong::ChatClient chatClient(client);

	scg::context::Context context;
	pingpong::SubscribeRequest req;
	req.count = 5;
	auto cr = chatClient.subscribe(context, req);
	TEST_CHECK(cr.second == nullptr);
	if (cr.second) { ctx.stopServer(); return; }
	auto stream = cr.first;

	int32_t count = 0;
	for (;;) {
		auto r = stream->recv();
		if (r.state == scg::rpc::StreamRecvState::Closed) {
			TEST_CHECK(r.error == nullptr); // clean EOF
			break;
		}
		TEST_CHECK(r.message.seq == count);
		count++;
	}
	TEST_CHECK(count == 5);

	client->disconnect();
	ctx.stopServer();
	printf("Stream Server-Streaming test passed\n");
}

// Client-streaming form: the client sends a stream and reads a single response.
inline void runStreamClientStreamingTest(TestContext& ctx) {
	printf("Running Stream Client-Streaming test...\n");

	ctx.startServerWithSetup(registerStreamingServices);
	auto client = ctx.createClient();
	pingpong::ChatClient chatClient(client);

	scg::context::Context context;
	auto cr = chatClient.upload(context);
	TEST_CHECK(cr.second == nullptr);
	if (cr.second) { ctx.stopServer(); return; }
	auto stream = cr.first;

	int32_t sum = 0;
	for (int32_t i = 1; i <= 5; i++) {
		pingpong::ChatMessage m;
		m.seq = i;
		TEST_CHECK(stream->send(m) == nullptr);
		sum += i;
	}

	auto result = stream->closeAndRecv();
	TEST_CHECK(result.second == nullptr);
	TEST_CHECK(result.first.total == sum);

	client->disconnect();
	ctx.stopServer();
	printf("Stream Client-Streaming test passed\n");
}

// Connection-level keepalive keeps an idle stream healthy (PING/PONG flow
// without disrupting the stream).
inline void runStreamKeepaliveTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) return;
	printf("Running Stream Keepalive test...\n");

	ctx.startServerWithSetup(registerStreamingServices);

	scg::rpc::ClientConfig clientConfig;
	clientConfig.transport = ctx.factory().createClientTransport(ctx.id());
	clientConfig.keepaliveInterval = std::chrono::milliseconds(40);
	clientConfig.keepaliveTimeout = std::chrono::milliseconds(500);
	auto client = std::make_shared<scg::rpc::Client>(clientConfig);
	TEST_CHECK(connectWithRetries(client, ctx.maxRetries()));

	pingpong::ChatClient chatClient(client);
	scg::context::Context context;
	auto cr = chatClient.connect(context);
	TEST_CHECK(cr.second == nullptr);
	if (cr.second) { ctx.stopServer(); return; }
	auto stream = cr.first;

	auto welcome = stream->recv();
	TEST_CHECK(welcome.state == scg::rpc::StreamRecvState::Message);
	TEST_CHECK(welcome.message.text == "welcome");

	// Idle well beyond the keepalive interval; pings keep the connection alive.
	std::this_thread::sleep_for(std::chrono::milliseconds(250));

	pingpong::ChatMessage m;
	m.text = "after-idle";
	m.seq = 1;
	TEST_CHECK(stream->send(m) == nullptr);
	auto echo = stream->recv();
	TEST_CHECK(echo.state == scg::rpc::StreamRecvState::Message);
	TEST_CHECK(echo.message.text == "echo:after-idle");

	client->disconnect();
	ctx.stopServer();
	printf("Stream Keepalive test passed\n");
}

// ============================================================================
// Adversarial / standalone tests. These cannot use the well-behaved factory
// servers, so they build a black-hole peer (a real transport that accepts and
// discards, never replying) or inject raw frames through a bare Connection.
// Both use the transport abstraction, so every case runs on TCP and WS.
// ============================================================================

// BlackHole accepts connections on a real transport and discards everything,
// never replying — used to drive client keepalive timeouts. Transport-agnostic.
class BlackHole {
public:
	explicit BlackHole(std::shared_ptr<scg::rpc::ServerTransport> transport)
		: transport_(std::move(transport)) {
		transport_->setOnConnection([this](std::shared_ptr<scg::rpc::Connection> conn) {
			conn->setMessageHandler([](const std::vector<uint8_t>&) {}); // discard, never reply
			std::lock_guard<std::mutex> lock(mu_);
			conns_.push_back(conn);
		});
		transport_->startListening();
		thread_ = std::thread([this]() { transport_->runEventLoop(); });
	}
	~BlackHole() {
		transport_->stop();
		if (thread_.joinable()) {
			thread_.join();
		}
	}

private:
	std::shared_ptr<scg::rpc::ServerTransport> transport_;
	std::thread thread_;
	std::mutex mu_;
	std::vector<std::shared_ptr<scg::rpc::Connection>> conns_;
};

// streamFrameBytes builds one stream frame: STREAM_PREFIX | streamID | kind | tail.
inline std::vector<uint8_t> streamFrameBytes(uint64_t streamID, uint8_t frameKind, std::vector<uint8_t> tail) {
	scg::serialize::Writer w(64);
	w.write(scg::rpc::STREAM_PREFIX);
	w.write(streamID);
	w.write(frameKind);
	auto b = w.bytes();
	b.insert(b.end(), tail.begin(), tail.end());
	return b;
}

// oversizedOpenFrame builds an OPEN whose context declares a ~1 GiB metadata
// value but supplies none — exercising the pre-auth allocation guard over the
// wire (server must reject, not allocate).
inline std::vector<uint8_t> oversizedOpenFrame() {
	scg::serialize::Writer tail(32);
	tail.write(uint32_t(1));        // context entry count
	std::string k = "k";
	tail.write(k);                  // key
	tail.write(uint32_t(1u << 30)); // value byte length (hostile)
	return streamFrameBytes(200, scg::rpc::STREAM_FRAME_OPEN, tail.bytes());
}

// currentThreadCount reads the process thread count (Linux /proc) for the leak test.
inline int currentThreadCount() {
	std::ifstream f("/proc/self/status");
	std::string line;
	while (std::getline(f, line)) {
		if (line.rfind("Threads:", 0) == 0) {
			try {
				return std::stoi(line.substr(8));
			} catch (...) {
				return -1;
			}
		}
	}
	return -1;
}

inline void expectKeepaliveTimeout(pingpong::ChatClient& chatClient) {
	scg::context::Context ctx;
	auto cr = chatClient.connect(ctx);
	TEST_CHECK(cr.second == nullptr);
	if (cr.second) {
		return;
	}
	auto r = cr.first->recv();
	TEST_CHECK(r.state == scg::rpc::StreamRecvState::Closed);
	TEST_CHECK(r.error != nullptr);
	if (r.error) {
		TEST_CHECK(r.error.message().find("keepalive timeout") != std::string::npos);
	}
}

// Build a real scg server with keepalive enabled on the factory transport.
inline std::shared_ptr<scg::rpc::Server> makeKeepaliveServer(TestContext& ctx) {
	scg::rpc::ServerConfig scfg;
	scfg.transport = ctx.factory().createServerTransport(ctx.id());
	scfg.errorHandler = [](const scg::error::Error&) {};
	scfg.keepaliveInterval = std::chrono::milliseconds(40);
	scfg.keepaliveTimeout = std::chrono::milliseconds(150);
	auto server = std::make_shared<scg::rpc::Server>(scfg);
	pingpong::registerPingPongServer(server.get(), std::make_shared<PingPongServerImpl>());
	pingpong::registerChatServer(server.get(), std::make_shared<ChatServerImpl>());
	return server;
}

// Client keepalive must detect a dead peer (a black-hole) and fail the stream.
inline void runKeepaliveTimeoutTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) return;
	printf("Running Keepalive Timeout test...\n");

	BlackHole blackhole(ctx.factory().createServerTransport(ctx.id()));
	std::this_thread::sleep_for(std::chrono::milliseconds(100));

	scg::rpc::ClientConfig cfg;
	cfg.transport = ctx.factory().createClientTransport(ctx.id());
	cfg.keepaliveInterval = std::chrono::milliseconds(40);
	cfg.keepaliveTimeout = std::chrono::milliseconds(150);
	auto client = std::make_shared<scg::rpc::Client>(cfg);

	pingpong::ChatClient chatClient(client);
	expectKeepaliveTimeout(chatClient);

	client->disconnect();
	printf("Keepalive Timeout test passed\n");
}

// Client keepalive must resume after a reconnect: a second stream must also fail.
inline void runKeepaliveReconnectTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) return;
	printf("Running Keepalive Reconnect test...\n");

	BlackHole blackhole(ctx.factory().createServerTransport(ctx.id()));
	std::this_thread::sleep_for(std::chrono::milliseconds(100));

	scg::rpc::ClientConfig cfg;
	cfg.transport = ctx.factory().createClientTransport(ctx.id());
	cfg.keepaliveInterval = std::chrono::milliseconds(40);
	cfg.keepaliveTimeout = std::chrono::milliseconds(150);
	auto client = std::make_shared<scg::rpc::Client>(cfg);

	pingpong::ChatClient chatClient(client);
	expectKeepaliveTimeout(chatClient); // connection 1
	expectKeepaliveTimeout(chatClient); // connection 2 (reconnect)

	client->disconnect();
	printf("Keepalive Reconnect test passed\n");
}

// Server keepalive must not disturb a well-behaved client (auto-PONG).
inline void runServerKeepaliveHealthyTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) return;
	printf("Running Server Keepalive Healthy test...\n");

	auto server = makeKeepaliveServer(ctx);
	TEST_CHECK(!server->start());
	std::this_thread::sleep_for(std::chrono::milliseconds(150));

	scg::rpc::ClientConfig ccfg;
	ccfg.transport = ctx.factory().createClientTransport(ctx.id());
	auto client = std::make_shared<scg::rpc::Client>(ccfg);
	TEST_CHECK(connectWithRetries(client, ctx.maxRetries()));

	pingpong::ChatClient chatClient(client);
	scg::context::Context cctx;
	auto cr = chatClient.connect(cctx);
	TEST_CHECK(cr.second == nullptr);
	if (cr.second) {
		server->shutdown();
		return;
	}
	auto stream = cr.first;
	auto welcome = stream->recv();
	TEST_CHECK(welcome.state == scg::rpc::StreamRecvState::Message);
	TEST_CHECK(welcome.message.text == "welcome");

	// Idle beyond the keepalive timeout; the server PINGs, the client PONGs.
	std::this_thread::sleep_for(std::chrono::milliseconds(500));

	pingpong::ChatMessage m;
	m.text = "alive";
	m.seq = 1;
	TEST_CHECK(stream->send(m) == nullptr);
	auto echo = stream->recv();
	TEST_CHECK(echo.state == scg::rpc::StreamRecvState::Message);
	TEST_CHECK(echo.message.text == "echo:alive");

	client->disconnect();
	server->shutdown();
	printf("Server Keepalive Healthy test passed\n");
}

// Server keepalive must tear down a connection whose client went silent.
inline void runServerKeepaliveDeadClientTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) return;
	printf("Running Server Keepalive Dead Client test...\n");

	auto server = makeKeepaliveServer(ctx);
	TEST_CHECK(!server->start());
	std::this_thread::sleep_for(std::chrono::milliseconds(150));

	// Raw client: connect, then ignore the server's PINGs (never PONG).
	auto clientTransport = ctx.factory().createClientTransport(ctx.id());
	auto connRes = clientTransport->connect();
	TEST_CHECK(connRes.second == nullptr);
	auto conn = connRes.first;
	auto closed = std::make_shared<std::atomic<bool>>(false);
	conn->setMessageHandler([](const std::vector<uint8_t>&) {});
	conn->setCloseHandler([closed]() { closed->store(true); });
	conn->setFailHandler([closed](const scg::error::Error&) { closed->store(true); });

	bool ok = false;
	auto start = std::chrono::steady_clock::now();
	while (std::chrono::steady_clock::now() - start < std::chrono::seconds(2)) {
		if (closed->load()) {
			ok = true;
			break;
		}
		std::this_thread::sleep_for(std::chrono::milliseconds(10));
	}
	TEST_CHECK(ok);

	clientTransport->shutdown();
	server->shutdown();
	printf("Server Keepalive Dead Client test passed\n");
}

// A server must survive a grab-bag of hostile frames and still serve afterward.
inline void runMalformedFramesTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) return;
	printf("Running Malformed Frames test...\n");

	scg::rpc::ServerConfig scfg;
	scfg.transport = ctx.factory().createServerTransport(ctx.id());
	scfg.errorHandler = [](const scg::error::Error&) {};
	auto server = std::make_shared<scg::rpc::Server>(scfg);
	pingpong::registerPingPongServer(server.get(), std::make_shared<PingPongServerImpl>());
	pingpong::registerChatServer(server.get(), std::make_shared<ChatServerImpl>());
	TEST_CHECK(!server->start());
	std::this_thread::sleep_for(std::chrono::milliseconds(150));

	// Raw client injects hostile frames, then disconnects.
	{
		auto clientTransport = ctx.factory().createClientTransport(ctx.id());
		auto connRes = clientTransport->connect();
		TEST_CHECK(connRes.second == nullptr);
		auto conn = connRes.first;

		std::vector<std::vector<uint8_t>> malformed;
		malformed.push_back({'g', 'a', 'r', 'b', 'a', 'g', 'e', '!', '!', '!', '!', '!', '!', '!', '!', '!', '!', '!'});
		malformed.push_back({});
		{
			scg::serialize::Writer w(16);
			w.write(scg::rpc::STREAM_PREFIX);
			malformed.push_back(w.bytes());
		}
		{
			// A valid unary request prefix followed by garbage — exercises the
			// unary request path's robustness, not just the stream path.
			scg::serialize::Writer w(32);
			w.write(scg::rpc::REQUEST_PREFIX);
			auto b = w.bytes();
			const uint8_t garbage[] = {0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0x11, 0x22};
			b.insert(b.end(), std::begin(garbage), std::end(garbage));
			malformed.push_back(b);
		}
		malformed.push_back(streamFrameBytes(7, 0xFF, {}));
		malformed.push_back(streamFrameBytes(123, scg::rpc::STREAM_FRAME_MESSAGE, {0x01, 0x02, 0x03}));
		malformed.push_back(streamFrameBytes(124, scg::rpc::STREAM_FRAME_HALF_CLOSE, {}));
		malformed.push_back(streamFrameBytes(125, scg::rpc::STREAM_FRAME_CLOSE, {0x00}));
		malformed.push_back(streamFrameBytes(200, scg::rpc::STREAM_FRAME_OPEN, {0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0x11}));
		malformed.push_back(oversizedOpenFrame());
		malformed.push_back(streamFrameBytes(0, scg::rpc::STREAM_FRAME_PING, {}));
		malformed.push_back(streamFrameBytes(0, scg::rpc::STREAM_FRAME_PONG, {}));

		for (const auto& m : malformed) {
			conn->send(m);
		}
		std::this_thread::sleep_for(std::chrono::milliseconds(100));
		clientTransport->shutdown();
	}

	std::this_thread::sleep_for(std::chrono::milliseconds(100));

	// The server must still be alive and serving.
	scg::rpc::ClientConfig ccfg;
	ccfg.transport = ctx.factory().createClientTransport(ctx.id());
	auto client = std::make_shared<scg::rpc::Client>(ccfg);
	TEST_CHECK(connectWithRetries(client, ctx.maxRetries()));

	pingpong::PingPongClient pp(client);
	scg::context::Context pctx;
	pingpong::PingRequest req;
	req.ping.count = 41;
	auto pr = pp.ping(pctx, req);
	TEST_CHECK(pr.second == nullptr);
	TEST_CHECK(pr.first.pong.count == 42);

	pingpong::ChatClient chat(client);
	auto cr = chat.connect(pctx);
	TEST_CHECK(cr.second == nullptr);
	if (!cr.second) {
		auto welcome = cr.first->recv();
		TEST_CHECK(welcome.state == scg::rpc::StreamRecvState::Message);
		TEST_CHECK(welcome.message.text == "welcome");
	}

	client->disconnect();
	server->shutdown();
	printf("Malformed Frames test passed\n");
}

// Open and close many streams; the process thread count must return to baseline.
inline void runStreamHandlerNoLeakTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) return;
	printf("Running Stream Handler No-Leak test...\n");

	ctx.startServerWithSetup(registerStreamingServices);

	auto client = ctx.createClient();
	pingpong::ChatClient chat(client);

	auto oneStream = [&]() {
		scg::context::Context cctx;
		auto cr = chat.connect(cctx);
		if (cr.second) {
			return;
		}
		auto stream = cr.first;
		stream->recv(); // welcome
		pingpong::ChatMessage m;
		m.text = "x";
		m.seq = 1;
		stream->send(m);
		stream->recv(); // echo
		stream->closeSend();
		stream->recv(); // summary
		stream->recv(); // closed
	};

	oneStream();
	std::this_thread::sleep_for(std::chrono::milliseconds(200));
	int baseline = currentThreadCount();

	const int N = 200;
	for (int i = 0; i < N; i++) {
		oneStream();
	}

	std::this_thread::sleep_for(std::chrono::milliseconds(400));
	int final = currentThreadCount();

	printf("threads: baseline=%d final=%d after %d streams\n", baseline, final, N);
	TEST_CHECK(baseline > 0);
	TEST_CHECK(final > 0);
	TEST_CHECK(final <= baseline + 5);

	client->disconnect();
	ctx.stopServer();
	printf("Stream Handler No-Leak test passed\n");
}

// A second OPEN reusing a live stream id must be rejected with a CLOSE(error)
// rather than orphaning the existing stream. Driven over the wire so it runs on
// every transport.
inline void runDuplicateStreamIDTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) return;
	printf("Running Duplicate Stream ID test...\n");

	scg::rpc::ServerConfig scfg;
	scfg.transport = ctx.factory().createServerTransport(ctx.id());
	scfg.errorHandler = [](const scg::error::Error&) {};
	auto server = std::make_shared<scg::rpc::Server>(scfg);
	pingpong::registerPingPongServer(server.get(), std::make_shared<PingPongServerImpl>());
	pingpong::registerChatServer(server.get(), std::make_shared<ChatServerImpl>());
	TEST_CHECK(!server->start());
	std::this_thread::sleep_for(std::chrono::milliseconds(150));

	auto clientTransport = ctx.factory().createClientTransport(ctx.id());
	auto connRes = clientTransport->connect();
	TEST_CHECK(connRes.second == nullptr);
	auto conn = connRes.first;

	// Collect the status messages of any CLOSE frames the server sends.
	auto closeMessages = std::make_shared<std::vector<std::string>>();
	auto cmu = std::make_shared<std::mutex>();
	conn->setMessageHandler([closeMessages, cmu](const std::vector<uint8_t>& data) {
		scg::serialize::ReaderView reader(data);
		std::array<uint8_t, 16> prefix;
		if (scg::serialize::deserialize(prefix, reader)) return;
		if (prefix != scg::rpc::STREAM_PREFIX) return;
		uint64_t streamID;
		if (scg::serialize::deserialize(streamID, reader)) return;
		uint8_t kind;
		if (scg::serialize::deserialize(kind, reader)) return;
		if (kind != scg::rpc::STREAM_FRAME_CLOSE) return;
		uint8_t status;
		if (scg::serialize::deserialize(status, reader)) return;
		std::string msg;
		if (scg::serialize::deserialize(msg, reader)) return;
		std::lock_guard<std::mutex> lock(*cmu);
		closeMessages->push_back(msg);
	});

	scg::context::Context octx;
	auto open = scg::rpc::serializeStreamOpen(octx, 1, pingpong::chatServerID, pingpong::chatServer_ConnectID);
	conn->send(open); // first OPEN: registers the stream (handler blocks in recv)
	std::this_thread::sleep_for(std::chrono::milliseconds(100));
	conn->send(open); // duplicate OPEN reusing id 1: must be rejected
	std::this_thread::sleep_for(std::chrono::milliseconds(200));

	bool found = false;
	{
		std::lock_guard<std::mutex> lock(*cmu);
		for (const auto& m : *closeMessages) {
			if (m.find("duplicate stream id") != std::string::npos) {
				found = true;
			}
		}
	}
	TEST_CHECK(found);

	clientTransport->shutdown();
	server->shutdown();
	printf("Duplicate Stream ID test passed\n");
}

// Cancelling a stream from the client side must notify the server and fail a
// blocked recv() with a cancelled error. The C++ analogue of the Go context-
// cancel test.
inline void runStreamClientCancelTest(TestContext& ctx) {
	if (ctx.isUsingExternalServer()) return;
	printf("Running Stream Client Cancel test...\n");

	ctx.startServerWithSetup(registerStreamingServices);

	auto client = ctx.createClient();
	pingpong::ChatClient chat(client);

	scg::context::Context cctx;
	auto cr = chat.connect(cctx);
	TEST_CHECK(cr.second == nullptr);
	if (cr.second) {
		ctx.stopServer();
		return;
	}
	auto stream = cr.first;

	auto welcome = stream->recv();
	TEST_CHECK(welcome.state == scg::rpc::StreamRecvState::Message);
	TEST_CHECK(welcome.message.text == "welcome");

	// Cancel from the client side.
	TEST_CHECK(stream->cancel() == nullptr);

	// A subsequent recv must return a terminal cancelled error.
	auto after = stream->recv();
	TEST_CHECK(after.state == scg::rpc::StreamRecvState::Closed);
	TEST_CHECK(after.error != nullptr);
	if (after.error) {
		TEST_CHECK(after.error.message().find("cancelled") != std::string::npos);
	}

	client->disconnect();
	ctx.stopServer();
	printf("Stream Client Cancel test passed\n");
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
				printf("\n=== Running Context Timeout Recovery Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runContextTimeoutRecoveryTest(ctx);
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

			{
				printf("\n=== Running Stream Server Error Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runStreamServerErrorTest(ctx);
			}

			{
				printf("\n=== Running Stream Poll Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runStreamPollTest(ctx);
			}

			{
				printf("\n=== Running Stream Concurrent Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runStreamConcurrentTest(ctx);
			}

			{
				printf("\n=== Running Stream Connection Drop Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runStreamConnectionDropTest(ctx);
			}

			{
				printf("\n=== Running Stream Auth Fail Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runStreamAuthFailTest(ctx);
			}

			{
				printf("\n=== Running Stream Concurrent Send/Recv Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runStreamConcurrentSendRecvTest(ctx);
			}

			{
				printf("\n=== Running Stream Send-After-CloseSend Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runStreamSendAfterCloseSendTest(ctx);
			}

			{
				printf("\n=== Running Stream Large Message Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runStreamLargeMessageTest(ctx);
			}

			{
				printf("\n=== Running Stream Backpressure Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runStreamBackpressureTest(ctx);
			}

			{
				printf("\n=== Running Stream Max Concurrent Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runStreamMaxConcurrentTest(ctx);
			}

			{
				printf("\n=== Running Stream Keepalive Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runStreamKeepaliveTest(ctx);
			}

			{
				printf("\n=== Running Server Keepalive Healthy Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runServerKeepaliveHealthyTest(ctx);
			}

			{
				printf("\n=== Running Server Keepalive Dead Client Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runServerKeepaliveDeadClientTest(ctx);
			}

			{
				printf("\n=== Running Keepalive Timeout Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runKeepaliveTimeoutTest(ctx);
			}

			{
				printf("\n=== Running Keepalive Reconnect Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runKeepaliveReconnectTest(ctx);
			}

			{
				printf("\n=== Running Malformed Frames Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runMalformedFramesTest(ctx);
			}

			{
				printf("\n=== Running Stream Handler No-Leak Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runStreamHandlerNoLeakTest(ctx);
			}

			{
				printf("\n=== Running Duplicate Stream ID Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runDuplicateStreamIDTest(ctx);
			}

			{
				printf("\n=== Running Stream Client Cancel Test ===\n");
				TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
				runStreamClientCancelTest(ctx);
			}
		}
	}

	// Streaming tests that run both in-process and against an external
	// (cross-language) server — these validate the stream wire framing interops.
	{
		printf("\n=== Running Stream Bidi Test ===\n");
		TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
		runStreamBidiTest(ctx);
	}

	{
		printf("\n=== Running Stream Server-Streaming Test ===\n");
		TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
		runStreamServerStreamingTest(ctx);
	}

	{
		printf("\n=== Running Stream Client-Streaming Test ===\n");
		TestContext ctx(config.factory, id++, config.maxRetries, config.useExternalServer);
		runStreamClientStreamingTest(ctx);
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
