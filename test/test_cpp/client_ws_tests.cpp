// C++ WebSocket Client Tests - runs against an external Go server
// Used for cross-language testing: C++ Client + Go Server

#include "test_suite.h"
#include "scg/ws/transport_server_no_tls.h"
#include "scg/ws/transport_server_tls.h"
#include "scg/ws/transport_client_no_tls.h"
#include "scg/ws/transport_client_tls.h"

// ============================================================================
// WebSocket Client-Only Transport Factory (connects to external server)
// ============================================================================

TransportFactory createWebSocketClientTransportFactory() {
    TransportFactory factory;
    factory.name = "WebSocket-Client";

    // Server transport is not used in external server mode
    factory.createServerTransport = [](int id) -> std::shared_ptr<scg::rpc::ServerTransport> {
        return nullptr;
    };

    factory.createClientTransport = [](int id) -> std::shared_ptr<scg::rpc::ClientTransport> {
        scg::ws::ClientTransportConfig transportConfig;
        transportConfig.host = "localhost";
        transportConfig.port = 8000;  // Must match Go server port (pingpong_server_ws)
        return std::make_shared<scg::ws::ClientTransportNoTLS>(transportConfig);
    };

    factory.createLimitedClientTransport = nullptr;

    return factory;
}

// ============================================================================
// Test Suite Entry Point
// ============================================================================

void test_websocket_client_suite() {
    TestSuiteConfig config;
    config.factory = createWebSocketClientTransportFactory();
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
    TEST(test_websocket_client_suite),
    { NULL, NULL }
};
