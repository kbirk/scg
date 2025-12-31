// C++ TCP TLS Client Tests - runs against an external Go server
// Used for cross-language testing: C++ Client + Go Server (TLS)

#include "test_suite.h"
#include "scg/tcp/transport_server_tls.h"
#include "scg/tcp/transport_client_tls.h"

// ============================================================================
// TCP TLS Client-Only Transport Factory (connects to external server)
// ============================================================================

TransportFactory createTCPTLSClientTransportFactory() {
	TransportFactory factory;
	factory.name = "TCP-TLS-Client";

	// Server transport is not used in external server mode, but we need to provide it
	// to satisfy the interface. It won't be called.
	factory.createServerTransport = [](int id) -> std::shared_ptr<scg::rpc::ServerTransport> {
		return nullptr;
	};

	factory.createClientTransport = [](int id) -> std::shared_ptr<scg::rpc::ClientTransport> {
		scg::tcp::ClientTransportTLSConfig transportConfig;
		transportConfig.host = "127.0.0.1";
		transportConfig.port = 9002;  // Must match Go server port (pingpong_server_tcp_tls)
		transportConfig.verifyPeer = false;  // Self-signed cert, skip verification
		return std::make_shared<scg::tcp::ClientTransportTCPTLS>(transportConfig);
	};

	factory.createLimitedClientTransport = nullptr;

	return factory;
}

// ============================================================================
// Test Suite Entry Point
// ============================================================================

void test_tcp_tls_client_suite() {
	TestSuiteConfig config;
	config.factory = createTCPTLSClientTransportFactory();
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
	TEST(test_tcp_tls_client_suite),
	{ NULL, NULL }
};
