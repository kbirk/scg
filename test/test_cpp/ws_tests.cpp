#include "test_suite.h"
#include "scg/ws/transport_server_no_tls.h"
#include "scg/ws/transport_server_tls.h"
#include "scg/ws/transport_client_no_tls.h"
#include "scg/ws/transport_client_tls.h"

// ============================================================================
// WebSocket (No TLS) Transport Factory
// ============================================================================

TransportFactory createWebSocketTransportFactory() {
    TransportFactory factory;
    factory.name = "WebSocket";

    factory.createServerTransport = [](int id) -> std::shared_ptr<scg::rpc::ServerTransport> {
        scg::ws::ServerTransportConfig transportConfig;
        transportConfig.port = 18000 + id;
        return std::make_shared<scg::ws::ServerTransportNoTLS>(transportConfig);
    };

    factory.createClientTransport = [](int id) -> std::shared_ptr<scg::rpc::ClientTransport> {
        scg::ws::ClientTransportConfig transportConfig;
        transportConfig.host = "localhost";
        transportConfig.port = 18000 + id;
        return std::make_shared<scg::ws::ClientTransportNoTLS>(transportConfig);
    };

    factory.createLimitedClientTransport = [](int id) -> std::shared_ptr<scg::rpc::ClientTransport> {
        scg::ws::ClientTransportConfig transportConfig;
        transportConfig.host = "localhost";
        transportConfig.port = 18000 + id;
        transportConfig.maxSendMessageSize = 1024;
        transportConfig.maxRecvMessageSize = 1024;
        return std::make_shared<scg::ws::ClientTransportNoTLS>(transportConfig);
    };

    return factory;
}

// ============================================================================
// WebSocket TLS Transport Factory
// ============================================================================

TransportFactory createWebSocketTLSTransportFactory() {
    TransportFactory factory;
    factory.name = "WebSocket-TLS";

    factory.createServerTransport = [](int id) -> std::shared_ptr<scg::rpc::ServerTransport> {
        scg::ws::ServerTransportTLSConfig transportConfig;
        transportConfig.port = 18100 + id;
        transportConfig.certFile = "../test/server.crt";
        transportConfig.keyFile = "../test/server.key";
        return std::make_shared<scg::ws::ServerTransportTLS>(transportConfig);
    };

    factory.createClientTransport = [](int id) -> std::shared_ptr<scg::rpc::ClientTransport> {
        scg::ws::ClientTransportTLSConfig transportConfig;
        transportConfig.host = "localhost";
        transportConfig.port = 18100 + id;
        return std::make_shared<scg::ws::ClientTransportTLS>(transportConfig);
    };

    factory.createLimitedClientTransport = [](int id) -> std::shared_ptr<scg::rpc::ClientTransport> {
        scg::ws::ClientTransportTLSConfig transportConfig;
        transportConfig.host = "localhost";
        transportConfig.port = 18100 + id;
        transportConfig.maxSendMessageSize = 1024;
        transportConfig.maxRecvMessageSize = 1024;
        return std::make_shared<scg::ws::ClientTransportTLS>(transportConfig);
    };

    return factory;
}

// ============================================================================
// Test Suite Entry Points
// ============================================================================

void test_websocket_suite() {
    TestSuiteConfig config;
    config.factory = createWebSocketTransportFactory();
    config.startingId = 0;
    config.maxRetries = 10;
    runTestSuite(config);
}

void test_websocket_tls_suite() {
    TestSuiteConfig config;
    config.factory = createWebSocketTLSTransportFactory();
    config.startingId = 0;
    config.maxRetries = 10;
    runTestSuite(config);
}

// helper method to reduce redundant test typing
#define TEST(x) {#x, x}

TEST_LIST = {
    TEST(test_websocket_suite),
    TEST(test_websocket_tls_suite),
    { NULL, NULL }
};
