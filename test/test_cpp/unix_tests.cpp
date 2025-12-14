#include "test_suite.h"
#include "scg/unix/transport_server.h"
#include "scg/unix/transport_client.h"

// ============================================================================
// Unix Socket Transport Factory
// ============================================================================

TransportFactory createUnixTransportFactory() {
    TransportFactory factory;
    factory.name = "Unix";

    factory.createServerTransport = [](int id) -> std::shared_ptr<scg::rpc::ServerTransport> {
        scg::unix_socket::ServerTransportConfig transportConfig;
        transportConfig.socketPath = "/tmp/scg_test_unix_" + std::to_string(id) + ".sock";
        return std::make_shared<scg::unix_socket::ServerTransportUnix>(transportConfig);
    };

    factory.createClientTransport = [](int id) -> std::shared_ptr<scg::rpc::ClientTransport> {
        scg::unix_socket::ClientTransportConfig transportConfig;
        transportConfig.socketPath = "/tmp/scg_test_unix_" + std::to_string(id) + ".sock";
        return std::make_shared<scg::unix_socket::ClientTransportUnix>(transportConfig);
    };

    factory.createLimitedClientTransport = [](int id) -> std::shared_ptr<scg::rpc::ClientTransport> {
        scg::unix_socket::ClientTransportConfig transportConfig;
        transportConfig.socketPath = "/tmp/scg_test_unix_" + std::to_string(id) + ".sock";
        transportConfig.maxSendMessageSize = 1024;
        transportConfig.maxRecvMessageSize = 1024;
        return std::make_shared<scg::unix_socket::ClientTransportUnix>(transportConfig);
    };

    return factory;
}

// ============================================================================
// Test Suite Entry Point
// ============================================================================

void test_unix_suite() {
    TestSuiteConfig config;
    config.factory = createUnixTransportFactory();
    config.startingId = 0;
    config.maxRetries = 10;
    runTestSuite(config);
}

// helper method to reduce redundant test typing
#define TEST(x) {#x, x}

TEST_LIST = {
    TEST(test_unix_suite),
    { NULL, NULL }
};
