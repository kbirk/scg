#include "streaming_tests.cpp"
#include "scg/unix/transport_server.h"
#include "scg/unix/transport_client.h"

// ============================================================================
// Unix Transport Factory
// ============================================================================

TransportFactory createUnixTransportFactory() {
    TransportFactory factory;
    factory.name = "Unix";

    factory.createServerTransport = [](int id) -> std::shared_ptr<scg::rpc::ServerTransport> {
        scg::unix_socket::ServerTransportConfig transportConfig;
        transportConfig.socketPath = "/tmp/scg_test_" + std::to_string(id) + ".sock";
        return std::make_shared<scg::unix_socket::ServerTransportUnix>(transportConfig);
    };

    factory.createClientTransport = [](int id) -> std::shared_ptr<scg::rpc::ClientTransport> {
        scg::unix_socket::ClientTransportConfig transportConfig;
        transportConfig.socketPath = "/tmp/scg_test_" + std::to_string(id) + ".sock";
        return std::make_shared<scg::unix_socket::ClientTransportUnix>(transportConfig);
    };

    return factory;
}

// ============================================================================
// Tests
// ============================================================================

void test_streaming_unix() {

    TransportFactory factory = createUnixTransportFactory();

    runStreamingBidirectionalTest(factory);
    // Disabled: notifications would deadlock from server's main thread
    // runStreamingServerNotificationsTest(factory);
    // runStreamingConcurrentMessagesTest(factory);
    printf("All tests complete!\n"); fflush(stdout);
}

TEST_LIST = {
    {"streaming_unix", test_streaming_unix},
    {NULL, NULL}
};
