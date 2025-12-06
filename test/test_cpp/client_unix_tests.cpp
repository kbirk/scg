#include "client_test_suite.h"
#include "scg/unix/transport_client.h"

ClientTransportFactory createUnixTransport() {
    return []() {
        scg::unix_socket::ClientTransportConfig transportConfig;
        transportConfig.socketPath = "/tmp/scg_test_unix_0.sock";
        return std::make_shared<scg::unix_socket::ClientTransportUnix>(transportConfig);
    };
}

void test_unix_suite() {
    TestSuiteConfig config;
    config.createTransport = createUnixTransport();
    config.maxRetries = 10;
    runTestSuite(config);
}

// helper method to reduce redundant test typing
#define TEST(x) {#x, x}

TEST_LIST = {
    TEST(test_unix_suite),
    { NULL, NULL }
};
