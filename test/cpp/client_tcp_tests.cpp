// C++ TCP Client Tests - runs against an external Go server
// Used for cross-language testing: C++ Client + Go Server

#include "test_suite.h"
#include "scg/tcp/transport_server.h"
#include "scg/tcp/transport_client.h"

// ============================================================================
// TCP Client-Only Transport Factory (connects to external server)
// ============================================================================

TransportFactory createTCPClientTransportFactory() {
	TransportFactory factory;
	factory.name = "TCP-Client";

	// Server transport is not used in external server mode, but we need to provide it
	// to satisfy the interface. It won't be called.
	factory.createServerTransport = [](int id) -> std::shared_ptr<scg::rpc::ServerTransport> {
		return nullptr;
	};

	factory.createClientTransport = [](int id) -> std::shared_ptr<scg::rpc::ClientTransport> {
		scg::tcp::ClientTransportConfig transportConfig;
		transportConfig.host = "127.0.0.1";
		transportConfig.port = 9001;  // Must match Go server port (pingpong_server_tcp)
		return std::make_shared<scg::tcp::ClientTransportTCP>(transportConfig);
	};

	factory.createLimitedClientTransport = nullptr;

	return factory;
}

// ============================================================================
// Test Suite Entry Point
// ============================================================================

void test_tcp_client_suite() {
	TestSuiteConfig config;
	config.factory = createTCPClientTransportFactory();
	config.startingId = 0;
	config.maxRetries = 10;
	config.useExternalServer = true;  // Connect to external Go server
	config.skipGroupTests = true;     // Server groups require server control
	config.skipEdgeTests = true;      // Edge tests require server control
	runTestSuite(config);
}

// helper method to reduce redundant test typing
#define TEST(x) {#x, x}

TEST_LIST = {
	TEST(test_tcp_client_suite),
	{ NULL, NULL }
};
