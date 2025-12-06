#pragma once

#include <cstdio>
#include <thread>
#include <chrono>
#include <functional>
#include <memory>
#include <vector>
#include <atomic>
#include <mutex>
#include <string>

#include <acutest.h>

#include "scg/serialize.h"
#include "scg/client.h"
#include "scg/logger.h"
#include "pingpong/pingpong.h"

// TransportFactory is a type alias for a function that creates client transport
using ClientTransportFactory = std::function<std::shared_ptr<scg::rpc::ClientTransport>()>;

// TestSuiteConfig holds configuration for running the test suite
struct TestSuiteConfig {
    ClientTransportFactory createTransport;
    int maxRetries = 10;
    bool skipConcurrencyTests = false;
};

// Helper function to create the test payload
inline pingpong::TestPayload createTestPayload(uint32_t i) {
    pingpong::NestedPayload nested1;
    nested1.valString = "nested" + std::to_string(i);
    nested1.valDouble = 3.14 + i;

    pingpong::NestedPayload nested2;
    nested2.valString = "nested again" + std::to_string(i);
    nested2.valDouble = 123.34563456 + i;

    pingpong::NestedEmpty nested;
    nested.empty = pingpong::Empty();

    pingpong::TestPayload payload;
    payload.valUint8 = i + 1;
    payload.valUint16 = 256 + i + 2;
    payload.valUint32 = 65535 + i + 3;
    payload.valUint64 = 4294967295ULL + i + 4;
    payload.valInt8 = -(i + 5);
    payload.valInt16 = -128 - (i + 6);
    payload.valInt32 = -32768 - (i + 7);
    payload.valInt64 = -2147483648LL - (i + 8);
    payload.valFloat = 3.14f + i + 9;
    payload.valDouble = -3.14159 + i + 10;
    payload.valString = "hello world " + std::to_string(i + 11);
    payload.valTimestamp = scg::type::timestamp();
    payload.valBool = i % 2 == 0;
    payload.valEnum = pingpong::EnumType::ENUM_TYPE_1;
    payload.valUUID = scg::type::uuid::random();
    payload.valListPayload = {nested1, nested2};
    payload.valMapKeyEnum = {
        {pingpong::KeyType("key_" + std::to_string(i+1)), pingpong::EnumType::ENUM_TYPE_1},
        {pingpong::KeyType("key_" + std::to_string(i+2)), pingpong::EnumType::ENUM_TYPE_2}
    };
    payload.valEmpty = pingpong::Empty();
    payload.valNestedEmpty = nested;
    payload.valByteArray = {0, 1, 2, 3, 4, 5, 6, 7, 8, 9};

    return payload;
}

// Helper function to verify the test payload matches expected values
inline void verifyTestPayload(
    const pingpong::TestPayload& result,
    const pingpong::TestPayload& expected,
    uint32_t i
) {
    TEST_CHECK(result.valUint8 == expected.valUint8);
    TEST_CHECK(result.valUint16 == expected.valUint16);
    TEST_CHECK(result.valUint32 == expected.valUint32);
    TEST_CHECK(result.valUint64 == expected.valUint64);
    TEST_CHECK(result.valInt8 == expected.valInt8);
    TEST_CHECK(result.valInt16 == expected.valInt16);
    TEST_CHECK(result.valInt32 == expected.valInt32);
    TEST_CHECK(result.valInt64 == expected.valInt64);
    TEST_CHECK(result.valFloat == expected.valFloat);
    TEST_CHECK(result.valDouble == expected.valDouble);
    TEST_CHECK(result.valString == expected.valString);
    TEST_CHECK(result.valBool == expected.valBool);
    TEST_CHECK(result.valEnum == expected.valEnum);
    TEST_CHECK(result.valUUID == expected.valUUID);
    TEST_CHECK(result.valListPayload.size() == 2);
    TEST_CHECK(result.valListPayload[0].valString == expected.valListPayload[0].valString);
    TEST_CHECK(result.valListPayload[0].valDouble == expected.valListPayload[0].valDouble);
    TEST_CHECK(result.valListPayload[1].valString == expected.valListPayload[1].valString);
    TEST_CHECK(result.valListPayload[1].valDouble == expected.valListPayload[1].valDouble);
    TEST_CHECK(result.valMapKeyEnum.size() == 2);
    TEST_CHECK(result.valMapKeyEnum.at(pingpong::KeyType("key_" + std::to_string(i+1))) == pingpong::EnumType::ENUM_TYPE_1);
    TEST_CHECK(result.valMapKeyEnum.at(pingpong::KeyType("key_" + std::to_string(i+2))) == pingpong::EnumType::ENUM_TYPE_2);
}

// Generic pingpong client test that works with any transport
inline void runPingPongClientTest(ClientTransportFactory createTransport, int maxRetries = 10) {
    scg::rpc::ClientConfig config;
    config.transport = createTransport();

    auto client = std::make_shared<scg::rpc::Client>(config);

    // Retry connection a few times to allow server to start
    scg::error::Error connectErr;
    for (int i = 0; i < maxRetries; i++) {
        connectErr = client->connect();
        if (!connectErr) break;
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }

    if (connectErr) {
        printf("Connection failed: %s\n", connectErr.message.c_str());
        TEST_CHECK(false);
        return;
    }

    pingpong::PingPongClient pingPongClient(client);

    uint32_t COUNT = 10;

    for (uint32_t i = 0; i < COUNT; i++) {
        scg::context::Context context;
        context.put("key", "value");
        context.put("token", "1234");

        pingpong::TestPayload payload = createTestPayload(i);

        pingpong::PingRequest req;
        req.ping.count = i;
        req.ping.payload = payload;

        auto [res, err] = pingPongClient.ping(context, req);
        if (err != nullptr) {
            printf("ERROR: %s\n", err.message.c_str());
            TEST_CHECK(err == nullptr);
            return;
        }
        TEST_CHECK(err == nullptr);
        TEST_CHECK(res.pong.count == int32_t(i+1));
        verifyTestPayload(res.pong.payload, payload, i);

        std::this_thread::sleep_for(std::chrono::milliseconds(50));
    }
}

// Generic pingpong client test with middleware
inline void runPingPongClientTestWithMiddleware(ClientTransportFactory createTransport, int maxRetries = 10) {
    scg::rpc::ClientConfig config;
    config.transport = createTransport();

    auto client = std::make_shared<scg::rpc::Client>(config);

    // Retry connection a few times to allow server to start
    scg::error::Error connectErr;
    for (int i = 0; i < maxRetries; i++) {
        connectErr = client->connect();
        if (!connectErr) break;
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }

    if (connectErr) {
        printf("Connection failed: %s\n", connectErr.message.c_str());
        TEST_CHECK(false);
        return;
    }

    uint32_t middlewareCount = 0;
    client->middleware([&middlewareCount](scg::context::Context& ctx, const scg::type::Message& req, scg::middleware::Handler next) -> std::pair<scg::type::Message*, scg::error::Error> {
        middlewareCount++;
        return next(ctx, req);
    });

    pingpong::PingPongClient pingPongClient(client);

    uint32_t COUNT = 10;

    for (uint32_t i = 0; i < COUNT; i++) {
        scg::context::Context context;
        context.put("token", "1234");

        pingpong::TestPayload payload = createTestPayload(i);

        pingpong::PingRequest req;
        req.ping.count = i;
        req.ping.payload = payload;

        auto [res, err] = pingPongClient.ping(context, req);
        if (err != nullptr) {
            printf("ERROR: %s\n", err.message.c_str());
            TEST_CHECK(err == nullptr);
            return;
        }
        TEST_CHECK(err == nullptr);
        TEST_CHECK(res.pong.count == int32_t(i+1));
        verifyTestPayload(res.pong.payload, payload, i);

        std::this_thread::sleep_for(std::chrono::milliseconds(50));
    }

    TEST_CHECK(middlewareCount == COUNT);
}

// Concurrent pingpong client test - multiple threads making requests simultaneously
inline void runPingPongConcurrentTest(ClientTransportFactory createTransport, int maxRetries = 10) {
    scg::rpc::ClientConfig config;
    config.transport = createTransport();

    auto client = std::make_shared<scg::rpc::Client>(config);

    // Retry connection a few times to allow server to start
    scg::error::Error connectErr;
    for (int i = 0; i < maxRetries; i++) {
        connectErr = client->connect();
        if (!connectErr) break;
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }

    if (connectErr) {
        printf("Connection failed: %s\n", connectErr.message.c_str());
        TEST_CHECK(false);
        return;
    }

    pingpong::PingPongClient pingPongClient(client);

    const int NUM_THREADS = 10;
    const int REQUESTS_PER_THREAD = 20;

    std::atomic<int> successCount{0};
    std::atomic<int> errorCount{0};
    std::vector<std::thread> threads;

    printf("Starting %d threads, each sending %d requests\n", NUM_THREADS, REQUESTS_PER_THREAD);

    for (int t = 0; t < NUM_THREADS; t++) {
        threads.emplace_back([&, t]() {
            for (int j = 0; j < REQUESTS_PER_THREAD; j++) {
                int32_t expectedCount = t * REQUESTS_PER_THREAD + j;
                std::string expectedPayload = "thread-" + std::to_string(t) + "-request-" + std::to_string(j);

                scg::context::Context context;
                context.put("token", "1234");
                pingpong::PingRequest req;
                req.ping.count = expectedCount;
                req.ping.payload.valString = expectedPayload;

                auto [res, err] = pingPongClient.ping(context, req);

                if (err) {
                    printf("Thread %d, request %d failed: %s\n", t, j, err.message.c_str());
                    errorCount++;
                    continue;
                }

                if (res.pong.count != expectedCount + 1) {
                    printf("Thread %d, request %d: expected count %d, got %d\n",
                           t, j, expectedCount + 1, res.pong.count);
                    errorCount++;
                    continue;
                }

                if (res.pong.payload.valString != expectedPayload) {
                    printf("Thread %d, request %d: expected payload '%s', got '%s'\n",
                           t, j, expectedPayload.c_str(), res.pong.payload.valString.c_str());
                    errorCount++;
                    continue;
                }

                successCount++;
            }
        });
    }

    for (auto& thread : threads) {
        thread.join();
    }

    int totalRequests = NUM_THREADS * REQUESTS_PER_THREAD;
    printf("Completed: %d successful, %d errors out of %d total requests\n",
           successCount.load(), errorCount.load(), totalRequests);

    TEST_CHECK(successCount.load() == totalRequests);
    TEST_CHECK(errorCount.load() == 0);
}

// Run multiple clients test - each thread creates its own client
inline void runMultipleClientsTest(ClientTransportFactory createTransport, int maxRetries = 10) {
    const int NUM_CLIENTS = 5;
    const int REQUESTS_PER_CLIENT = 10;

    std::atomic<int> successCount{0};
    std::atomic<int> errorCount{0};
    std::vector<std::thread> threads;

    printf("Starting %d clients, each sending %d requests\n", NUM_CLIENTS, REQUESTS_PER_CLIENT);
    fflush(stdout);

    for (int c = 0; c < NUM_CLIENTS; c++) {
        threads.emplace_back([&, c]() {
            scg::rpc::ClientConfig config;
            config.transport = createTransport();

            auto client = std::make_shared<scg::rpc::Client>(config);

            // Retry connection
            scg::error::Error connectErr;
            for (int i = 0; i < maxRetries; i++) {
                connectErr = client->connect();
                if (!connectErr) break;
                std::this_thread::sleep_for(std::chrono::milliseconds(100));
            }

            if (connectErr) {
                printf("Client %d connection failed: %s\n", c, connectErr.message.c_str());
                errorCount += REQUESTS_PER_CLIENT;
                return;
            }

            pingpong::PingPongClient pingPongClient(client);

            for (int j = 0; j < REQUESTS_PER_CLIENT; j++) {
                int32_t count = c * 1000 + j;

                scg::context::Context context;
                context.put("token", "1234");
                pingpong::PingRequest req;
                req.ping.count = count;

                auto [res, err] = pingPongClient.ping(context, req);

                if (err) {
                    errorCount++;
                    continue;
                }

                if (res.pong.count == count + 1) {
                    successCount++;
                } else {
                    errorCount++;
                }
            }
        });
    }

    for (auto& thread : threads) {
        thread.join();
    }

    int totalRequests = NUM_CLIENTS * REQUESTS_PER_CLIENT;
    printf("Multiple clients: %d successful out of %d requests\n",
           successCount.load(), totalRequests);

    TEST_CHECK(successCount.load() == totalRequests);
    TEST_CHECK(errorCount.load() == 0);
}

// Run rapid connection churn test - create and destroy connections rapidly
inline void runRapidConnectionChurnTest(ClientTransportFactory createTransport, int maxRetries = 10) {
    const int NUM_ITERATIONS = 20;

    printf("Starting %d rapid connection iterations\n", NUM_ITERATIONS);
    fflush(stdout);

    for (int i = 0; i < NUM_ITERATIONS; i++) {
        scg::rpc::ClientConfig config;
        config.transport = createTransport();

        auto client = std::make_shared<scg::rpc::Client>(config);

        // Retry connection
        scg::error::Error connectErr;
        for (int r = 0; r < maxRetries; r++) {
            connectErr = client->connect();
            if (!connectErr) break;
            std::this_thread::sleep_for(std::chrono::milliseconds(100));
        }

        if (connectErr) {
            printf("Iteration %d: Connection failed: %s\n", i, connectErr.message.c_str());
            TEST_CHECK(false);
            return;
        }

        pingpong::PingPongClient pingPongClient(client);

        scg::context::Context context;
        context.put("token", "1234");
        pingpong::PingRequest req;
        req.ping.count = i;

        auto [res, err] = pingPongClient.ping(context, req);

        if (err) {
            printf("Iteration %d: Request failed: %s\n", i, err.message.c_str());
            TEST_CHECK(false);
            return;
        }

        TEST_CHECK(res.pong.count == i + 1);
    }

    printf("All rapid connection iterations completed successfully\n");
}

// Run large payload test - tests handling of large messages
inline void runLargePayloadTest(ClientTransportFactory createTransport, int maxRetries = 10) {
    scg::rpc::ClientConfig config;
    config.transport = createTransport();

    auto client = std::make_shared<scg::rpc::Client>(config);

    // Retry connection
    scg::error::Error connectErr;
    for (int i = 0; i < maxRetries; i++) {
        connectErr = client->connect();
        if (!connectErr) break;
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }

    if (connectErr) {
        printf("Connection failed: %s\n", connectErr.message.c_str());
        TEST_CHECK(false);
        return;
    }

    pingpong::PingPongClient pingPongClient(client);

    // Test various payload sizes
    struct TestCase {
        const char* name;
        size_t size;
    };

    TestCase testCases[] = {
        {"Small 1KB", 1024},
        {"Medium 100KB", 100 * 1024},
        {"Large 500KB", 500 * 1024},
    };

    for (const auto& tc : testCases) {
        printf("Testing %s payload...\n", tc.name);

        std::string largePayload(tc.size, 'x');

        scg::context::Context context;
        context.put("token", "1234");
        pingpong::PingRequest req;
        req.ping.count = 1;
        req.ping.payload.valString = largePayload;

        auto [res, err] = pingPongClient.ping(context, req);

        if (err) {
            printf("%s payload failed: %s\n", tc.name, err.message.c_str());
            TEST_CHECK(false);
            continue;
        }

        TEST_CHECK(res.pong.payload.valString.size() == tc.size);
        printf("%s payload succeeded\n", tc.name);
    }
}

// Run context metadata test - tests that context metadata is properly passed
inline void runContextMetadataTest(ClientTransportFactory createTransport, int maxRetries = 10) {
    scg::rpc::ClientConfig config;
    config.transport = createTransport();

    auto client = std::make_shared<scg::rpc::Client>(config);

    // Retry connection
    scg::error::Error connectErr;
    for (int i = 0; i < maxRetries; i++) {
        connectErr = client->connect();
        if (!connectErr) break;
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }

    if (connectErr) {
        printf("Connection failed: %s\n", connectErr.message.c_str());
        TEST_CHECK(false);
        return;
    }

    pingpong::PingPongClient pingPongClient(client);

    // Test with context metadata
    scg::context::Context context;
    context.put("key1", "value1");
    context.put("key2", "value2");
    context.put("token", "1234");

    pingpong::PingRequest req;
    req.ping.count = 42;

    auto [res, err] = pingPongClient.ping(context, req);

    if (err) {
        printf("Request with context failed: %s\n", err.message.c_str());
        TEST_CHECK(false);
        return;
    }

    TEST_CHECK(res.pong.count == 43);
    printf("Context metadata test passed\n");
}

// Run empty payload test - tests handling of minimal/empty payloads
inline void runEmptyPayloadTest(ClientTransportFactory createTransport, int maxRetries = 10) {
    scg::rpc::ClientConfig config;
    config.transport = createTransport();

    auto client = std::make_shared<scg::rpc::Client>(config);

    // Retry connection
    scg::error::Error connectErr;
    for (int i = 0; i < maxRetries; i++) {
        connectErr = client->connect();
        if (!connectErr) break;
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }

    if (connectErr) {
        printf("Connection failed: %s\n", connectErr.message.c_str());
        TEST_CHECK(false);
        return;
    }

    pingpong::PingPongClient pingPongClient(client);

    // Test with minimal payload (just count, no payload data)
    scg::context::Context context;
    context.put("token", "1234");
    pingpong::PingRequest req;
    req.ping.count = 0;
    // Leave payload at default values

    auto [res, err] = pingPongClient.ping(context, req);

    if (err) {
        printf("Empty payload request failed: %s\n", err.message.c_str());
        TEST_CHECK(false);
        return;
    }

    TEST_CHECK(res.pong.count == 1);
    printf("Empty payload test passed\n");
}

// Run sequential requests test - tests many sequential requests from same client
inline void runSequentialRequestsTest(ClientTransportFactory createTransport, int maxRetries = 10) {
    scg::rpc::ClientConfig config;
    config.transport = createTransport();

    auto client = std::make_shared<scg::rpc::Client>(config);

    // Retry connection
    scg::error::Error connectErr;
    for (int i = 0; i < maxRetries; i++) {
        connectErr = client->connect();
        if (!connectErr) break;
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }

    if (connectErr) {
        printf("Connection failed: %s\n", connectErr.message.c_str());
        TEST_CHECK(false);
        return;
    }

    pingpong::PingPongClient pingPongClient(client);

    const int NUM_REQUESTS = 100;
    printf("Sending %d sequential requests...\n", NUM_REQUESTS);

    for (int i = 0; i < NUM_REQUESTS; i++) {
        scg::context::Context context;
        context.put("token", "1234");
        pingpong::PingRequest req;
        req.ping.count = i;

        auto [res, err] = pingPongClient.ping(context, req);

        if (err) {
            printf("Request %d failed: %s\n", i, err.message.c_str());
            TEST_CHECK(false);
            return;
        }

        TEST_CHECK(res.pong.count == i + 1);
    }

    printf("All %d sequential requests completed successfully\n", NUM_REQUESTS);
}

// Run the full test suite with given configuration
inline void runTestSuite(const TestSuiteConfig& config) {
    printf("\n=== Running PingPong Test ===\n");
    runPingPongClientTest(config.createTransport, config.maxRetries);

    printf("\n=== Running PingPong with Middleware Test ===\n");
    runPingPongClientTestWithMiddleware(config.createTransport, config.maxRetries);

    printf("\n=== Running Empty Payload Test ===\n");
    runEmptyPayloadTest(config.createTransport, config.maxRetries);

    printf("\n=== Running Context Metadata Test ===\n");
    runContextMetadataTest(config.createTransport, config.maxRetries);

    printf("\n=== Running Sequential Requests Test ===\n");
    runSequentialRequestsTest(config.createTransport, config.maxRetries);

    printf("\n=== Running Large Payload Test ===\n");
    runLargePayloadTest(config.createTransport, config.maxRetries);

    if (!config.skipConcurrencyTests) {
        printf("\n=== Running Concurrent Test ===\n");
        runPingPongConcurrentTest(config.createTransport, config.maxRetries);

        printf("\n=== Running Multiple Clients Test ===\n");
        runMultipleClientsTest(config.createTransport, config.maxRetries);

        printf("\n=== Running Rapid Connection Churn Test ===\n");
        runRapidConnectionChurnTest(config.createTransport, config.maxRetries);
    }

    printf("\n=== All Tests Completed ===\n");
}
