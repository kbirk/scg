// C++ WebSocket TLS Client Tests - runs against an external Go server
// Used for cross-language testing: C++ Client + Go Server (TLS)

#include "test_suite.h"
#include "scg/ws/transport_server_tls.h"
#include "scg/ws/transport_client_tls.h"

// ============================================================================
// WebSocket TLS Client-Only Transport Factory (connects to external server)
// ============================================================================

TransportFactory createWebSocketTLSClientTransportFactory() {
	TransportFactory factory;
	factory.name = "WebSocket-TLS-Client";

	// Server transport is not used in external server mode
	factory.createServerTransport = [](int id) -> std::shared_ptr<scg::rpc::ServerTransport> {
	return nullptr;
	};

	factory.createClientTransport = [](int id) -> std::shared_ptr<scg::rpc::ClientTransport> {
	scg::ws::ClientTransportTLSConfig transportConfig;
	transportConfig.host = "localhost";
	transportConfig.port = 8001;  // Must match Go server port (pingpong_server_ws_tls)
	// Self-signed cert, skip verification (verifyPeer defaults to false for TLS client)
	transportConfig.verifyPeer = false;
	transportConfig.path = "/rpc";
	return std::make_shared<scg::ws::ClientTransportWSTLS>(transportConfig);
	};

	factory.createLimitedClientTransport = nullptr;

	return factory;
}

// ============================================================================
// Test Suite Entry Point
// ============================================================================

void test_websocket_tls_client_suite() {
	TestSuiteConfig config;
	config.factory = createWebSocketTLSClientTransportFactory();
	config.startingId = 0;
	config.maxRetries = 30;  // WebSocket needs more retries
	config.useExternalServer = true;  // Connect to external Go server
	config.skipGroupTests = true;     // Server groups require server control
	config.skipEdgeTests = true;      // Edge tests require server control
	runTestSuite(config);
}

// helper method to reduce redundant test typing
#define TEST(x) {#x, x}

TEST_LIST = {
	TEST(test_websocket_tls_client_suite),
	{ NULL, NULL }
};
