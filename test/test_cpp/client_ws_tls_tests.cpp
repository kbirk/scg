#include "client_test_suite.h"
#include "scg/ws/transport_client_tls.h"

ClientTransportFactory createWebSocketTLSTransport() {
    scg::log::LoggingConfig logging;
    logging.level = scg::log::LogLevel::WARN;
    logging.debugLogger = [](std::string msg) {
        printf("DEBUG: %s\n", msg.c_str());
    };
    logging.infoLogger = [](std::string msg) {
        printf("INFO: %s\n", msg.c_str());
    };
    logging.warnLogger = [](std::string msg) {
        printf("WARN: %s\n", msg.c_str());
    };
    logging.errorLogger = [](std::string msg) {
        printf("ERROR: %s\n", msg.c_str());
    };

    return [logging]() {
        scg::ws::ClientTransportTLSConfig transportConfig;
        transportConfig.host = "localhost";
        transportConfig.port = 8000;
        transportConfig.logging = logging;
        return std::make_shared<scg::ws::ClientTransportTLS>(transportConfig);
    };
}

void test_websocket_tls_suite() {
    TestSuiteConfig config;
    config.createTransport = createWebSocketTLSTransport();
    config.maxRetries = 10;
    runTestSuite(config);
}

// helper method to reduce redundant test typing
#define TEST(x) {#x, x}

TEST_LIST = {
    TEST(test_websocket_tls_suite),
    { NULL, NULL }
};
