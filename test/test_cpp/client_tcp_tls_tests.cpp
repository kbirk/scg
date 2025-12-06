#include "client_test_suite.h"
#include "scg/tcp/transport_client_tls.h"

ClientTransportFactory createTCPTLSTransport() {
    return []() {
        scg::tcp::ClientTransportTLSConfig transportConfig;
        transportConfig.host = "localhost";
        transportConfig.port = 9002;
        transportConfig.verifyPeer = false;  // Self-signed cert
        return std::make_shared<scg::tcp::ClientTransportTCPTLS>(transportConfig);
    };
}

void test_tcp_tls_suite() {
    TestSuiteConfig config;
    config.createTransport = createTCPTLSTransport();
    config.maxRetries = 10;
    runTestSuite(config);
}

// helper method to reduce redundant test typing
#define TEST(x) {#x, x}

TEST_LIST = {
    TEST(test_tcp_tls_suite),
    { NULL, NULL }
};
