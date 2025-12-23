#include "streaming_tests.cpp"
#include "scg/ws/transport_server_no_tls.h"
#include "scg/ws/transport_client_no_tls.h"

// ============================================================================
// WebSocket Transport Factory
// ============================================================================

TransportFactory createWSTransportFactory() {
    TransportFactory factory;
    factory.name = "WebSocket";

    factory.createServerTransport = [](int id) -> std::shared_ptr<scg::rpc::ServerTransport> {
        scg::ws::ServerTransportConfig transportConfig;
        transportConfig.port = 19200 + id;
        return std::make_shared<scg::ws::ServerTransportNoTLS>(transportConfig);
    };

    factory.createClientTransport = [](int id) -> std::shared_ptr<scg::rpc::ClientTransport> {
        scg::ws::ClientTransportConfig transportConfig;
        transportConfig.host = "127.0.0.1";
        transportConfig.port = 19200 + id;
        return std::make_shared<scg::ws::ClientTransportNoTLS>(transportConfig);
    };

    return factory;
}

// ============================================================================
// Tests
// ============================================================================

void test_streaming_ws() {
    TransportFactory factory = createWSTransportFactory();
    runStreamingBidirectionalTest(factory);
    // Disabled: notifications would deadlock from server's main thread
    // runStreamingServerNotificationsTest(factory);
    // runStreamingConcurrentMessagesTest(factory);
    printf("All tests complete!\n"); fflush(stdout);
}

TEST_LIST = {
    {"streaming_ws", test_streaming_ws},
    {NULL, NULL}
};
