#include "client_test_suite.h"
#include "scg/tcp/transport_client.h"

ClientTransportFactory createTCPTransport() {
    return []() {
        scg::tcp::ClientTransportConfig transportConfig;
        transportConfig.host = "localhost";
        transportConfig.port = 9001;
        return std::make_shared<scg::tcp::ClientTransportTCP>(transportConfig);
    };
}

void test_tcp_suite() {
    TestSuiteConfig config;
    config.createTransport = createTCPTransport();
    config.maxRetries = 10;
    runTestSuite(config);
}

// helper method to reduce redundant test typing
#define TEST(x) {#x, x}

TEST_LIST = {
    TEST(test_tcp_suite),
    { NULL, NULL }
};
