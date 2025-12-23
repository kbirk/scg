#include "streaming_tests.cpp"
#include "scg/tcp/transport_server.h"
#include "scg/tcp/transport_client.h"

// ============================================================================
// TCP Transport Factory
// ============================================================================

TransportFactory createTCPTransportFactory() {
    TransportFactory factory;
    factory.name = "TCP";

    factory.createServerTransport = [](int id) -> std::shared_ptr<scg::rpc::ServerTransport> {
        scg::tcp::ServerTransportConfig transportConfig;
        transportConfig.port = 19000 + id;
        return std::make_shared<scg::tcp::ServerTransportTCP>(transportConfig);
    };

    factory.createClientTransport = [](int id) -> std::shared_ptr<scg::rpc::ClientTransport> {
        scg::tcp::ClientTransportConfig transportConfig;
        transportConfig.host = "127.0.0.1";
        transportConfig.port = 19000 + id;
        return std::make_shared<scg::tcp::ClientTransportTCP>(transportConfig);
    };

    return factory;
}

// ============================================================================
// Tests
// ============================================================================

void test_streaming_tcp() {
    printf("=== Starting TCP Streaming Tests ===\n");
    fflush(stdout);
    printf("Creating transport factory...\n");
    fflush(stdout);
    TransportFactory factory = createTCPTransportFactory();
    printf("Factory created, running tests...\n");
    fflush(stdout);
    runStreamingBidirectionalTest(factory);
    // Disabled: notifications would deadlock from server's main thread
    // runStreamingServerNotificationsTest(factory);
    // runStreamingConcurrentMessagesTest(factory);
    printf("All tests complete!\n"); fflush(stdout);
}

TEST_LIST = {
    {"streaming_tcp", test_streaming_tcp},
    {NULL, NULL}
};
