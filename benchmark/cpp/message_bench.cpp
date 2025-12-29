#include <iostream>
#include <chrono>
#include <vector>
#include <string>
#include <functional>
#include <iomanip>
#include <gperftools/profiler.h>

#include "scg/serialize.h"
#include "scg/writer.h"
#include "scg/reader.h"
#include "scg/uuid.h"
#include "scg/timestamp.h"
#include "benchmark/benchmark.h"

using namespace scg::serialize;

// Simple benchmark harness
void RunBenchmark(const std::string& name, std::function<void(int)> func, int iterations = 10000000) {
    // Warmup
    func(iterations / 100);

    auto start = std::chrono::high_resolution_clock::now();
    func(iterations);
    auto end = std::chrono::high_resolution_clock::now();

    std::chrono::duration<double, std::nano> elapsed = end - start;
    double ns_per_op = elapsed.count() / iterations;

    std::cout << std::left << std::setw(50) << name
              << std::right << std::setw(12) << iterations
              << std::setw(15) << std::fixed << std::setprecision(2) << ns_per_op << " ns/op" << std::endl;
}

// BenchmarkGeneratedMessageSmall benchmarks small generated messages
void BenchmarkGeneratedMessageSmallSerialize(int n) {
    benchmark::SmallMessage msg;
    msg.id = 12345;
    msg.value = 67890;

    for (int i = 0; i < n; ++i) {
        Writer writer(bits_to_bytes(bit_size(msg)));
        serialize(writer, msg);
    }
}

void BenchmarkGeneratedMessageSmallDeserialize(int n) {
    benchmark::SmallMessage msg;
    msg.id = 12345;
    msg.value = 67890;

    Writer writer(bits_to_bytes(bit_size(msg)));
    serialize(writer, msg);
    std::vector<uint8_t> data = writer.bytes();

    for (int i = 0; i < n; ++i) {
        Reader reader(data);
        benchmark::SmallMessage result;
        deserialize(result, reader);
    }
}

// BenchmarkGeneratedMessageEcho benchmarks echo request/response messages
void BenchmarkGeneratedMessageEchoRequestSerialize(int n) {
    benchmark::EchoRequest req;
    req.message = "Hello, World! This is a test message for benchmarking purposes.";
    req.timestamp = static_cast<uint64_t>(std::chrono::duration_cast<std::chrono::nanoseconds>(
        std::chrono::system_clock::now().time_since_epoch()).count());

    for (int i = 0; i < n; ++i) {
        Writer writer(bits_to_bytes(bit_size(req)));
        serialize(writer, req);
    }
}

void BenchmarkGeneratedMessageEchoRequestDeserialize(int n) {
    benchmark::EchoRequest req;
    req.message = "Hello, World! This is a test message for benchmarking purposes.";
    req.timestamp = static_cast<uint64_t>(std::chrono::duration_cast<std::chrono::nanoseconds>(
        std::chrono::system_clock::now().time_since_epoch()).count());

    Writer writer(bits_to_bytes(bit_size(req)));
    serialize(writer, req);
    std::vector<uint8_t> data = writer.bytes();

    for (int i = 0; i < n; ++i) {
        Reader reader(data);
        benchmark::EchoRequest result;
        deserialize(result, reader);
    }
}

void BenchmarkGeneratedMessageEchoResponseSerialize(int n) {
    benchmark::EchoResponse resp;
    resp.message = "Hello, World! This is a test message for benchmarking purposes.";
    resp.timestamp = static_cast<uint64_t>(std::chrono::duration_cast<std::chrono::nanoseconds>(
        std::chrono::system_clock::now().time_since_epoch()).count());
    resp.serverTimestamp = static_cast<uint64_t>(std::chrono::duration_cast<std::chrono::nanoseconds>(
        std::chrono::system_clock::now().time_since_epoch()).count());

    for (int i = 0; i < n; ++i) {
        Writer writer(bits_to_bytes(bit_size(resp)));
        serialize(writer, resp);
    }
}

void BenchmarkGeneratedMessageEchoResponseDeserialize(int n) {
    benchmark::EchoResponse resp;
    resp.message = "Hello, World! This is a test message for benchmarking purposes.";
    resp.timestamp = static_cast<uint64_t>(std::chrono::duration_cast<std::chrono::nanoseconds>(
        std::chrono::system_clock::now().time_since_epoch()).count());
    resp.serverTimestamp = static_cast<uint64_t>(std::chrono::duration_cast<std::chrono::nanoseconds>(
        std::chrono::system_clock::now().time_since_epoch()).count());

    Writer writer(bits_to_bytes(bit_size(resp)));
    serialize(writer, resp);
    std::vector<uint8_t> data = writer.bytes();

    for (int i = 0; i < n; ++i) {
        Reader reader(data);
        benchmark::EchoResponse result;
        deserialize(result, reader);
    }
}

// BenchmarkGeneratedMessageProcess benchmarks complex business messages
void BenchmarkGeneratedMessageProcessRequestSerialize(int n) {
    benchmark::ProcessRequest req;
    req.id = "order-12345";
    req.createdAt = scg::type::timestamp();
    req.user.userID = scg::type::uuid::random();
    req.user.username = "testuser";
    req.user.email = "testuser@example.com";
    req.user.registeredAt = scg::type::timestamp(std::chrono::system_clock::now() - std::chrono::hours(24 * 365));
    req.user.role = benchmark::UserRole::ADMIN;

    benchmark::OrderItem item1;
    item1.itemID = scg::type::uuid::random();
    item1.name = "Product A";
    item1.quantity = 2;
    item1.unitPrice = 19.99;
    item1.totalPrice = 39.98;
    item1.attributes["color"] = "blue";
    item1.attributes["size"] = "large";
    req.items.push_back(item1);

    benchmark::OrderItem item2;
    item2.itemID = scg::type::uuid::random();
    item2.name = "Product B";
    item2.quantity = 1;
    item2.unitPrice = 49.99;
    item2.totalPrice = 49.99;
    item2.attributes["warranty"] = "2 years";
    req.items.push_back(item2);

    req.metadata["source"] = "web";
    req.metadata["campaign"] = "summer-sale";

    for (int i = 0; i < n; ++i) {
        Writer writer(bits_to_bytes(bit_size(req)));
        serialize(writer, req);
    }
}

void BenchmarkGeneratedMessageProcessRequestDeserialize(int n) {
    benchmark::ProcessRequest req;
    req.id = "order-12345";
    req.createdAt = scg::type::timestamp();
    req.user.userID = scg::type::uuid::random();
    req.user.username = "testuser";
    req.user.email = "testuser@example.com";
    req.user.registeredAt = scg::type::timestamp(std::chrono::system_clock::now() - std::chrono::hours(24 * 365));
    req.user.role = benchmark::UserRole::ADMIN;

    benchmark::OrderItem item1;
    item1.itemID = scg::type::uuid::random();
    item1.name = "Product A";
    item1.quantity = 2;
    item1.unitPrice = 19.99;
    item1.totalPrice = 39.98;
    item1.attributes["color"] = "blue";
    item1.attributes["size"] = "large";
    req.items.push_back(item1);

    benchmark::OrderItem item2;
    item2.itemID = scg::type::uuid::random();
    item2.name = "Product B";
    item2.quantity = 1;
    item2.unitPrice = 49.99;
    item2.totalPrice = 49.99;
    item2.attributes["warranty"] = "2 years";
    req.items.push_back(item2);

    req.metadata["source"] = "web";
    req.metadata["campaign"] = "summer-sale";

    Writer writer(bits_to_bytes(bit_size(req)));
    serialize(writer, req);
    std::vector<uint8_t> data = writer.bytes();

    for (int i = 0; i < n; ++i) {
        Reader reader(data);
        benchmark::ProcessRequest result;
        deserialize(result, reader);
    }
}

void BenchmarkGeneratedMessageProcessResponseSerialize(int n) {
    benchmark::ProcessResponse resp;
    resp.id = "order-12345";
    resp.processedAt = scg::type::timestamp();
    resp.status = benchmark::ProcessStatus::SUCCESS;
    resp.message = "Order processed successfully";
    resp.stats.itemsProcessed = 2;
    resp.stats.totalAmount = 89.97;
    resp.stats.processingTimeMs = 42;

    for (int i = 0; i < n; ++i) {
        Writer writer(bits_to_bytes(bit_size(resp)));
        serialize(writer, resp);
    }
}

void BenchmarkGeneratedMessageProcessResponseDeserialize(int n) {
    benchmark::ProcessResponse resp;
    resp.id = "order-12345";
    resp.processedAt = scg::type::timestamp();
    resp.status = benchmark::ProcessStatus::SUCCESS;
    resp.message = "Order processed successfully";
    resp.stats.itemsProcessed = 2;
    resp.stats.totalAmount = 89.97;
    resp.stats.processingTimeMs = 42;

    Writer writer(bits_to_bytes(bit_size(resp)));
    serialize(writer, resp);
    std::vector<uint8_t> data = writer.bytes();

    for (int i = 0; i < n; ++i) {
        Reader reader(data);
        benchmark::ProcessResponse result;
        deserialize(result, reader);
    }
}

// BenchmarkGeneratedMessageNested benchmarks deeply nested messages
void BenchmarkGeneratedMessageNestedSerialize(int n) {
    benchmark::NestedMessage msg;
    msg.level1.name = "Level 1";
    msg.level1.level2.name = "Level 2";
    msg.level1.level2.level3.name = "Level 3";
    msg.level1.level2.level3.values = {"value1", "value2", "value3", "value4", "value5"};
    msg.level1.level2.level3.counts["count1"] = 10;
    msg.level1.level2.level3.counts["count2"] = 20;
    msg.level1.level2.level3.counts["count3"] = 30;

    for (int i = 0; i < n; ++i) {
        Writer writer(bits_to_bytes(bit_size(msg)));
        serialize(writer, msg);
    }
}

void BenchmarkGeneratedMessageNestedDeserialize(int n) {
    benchmark::NestedMessage msg;
    msg.level1.name = "Level 1";
    msg.level1.level2.name = "Level 2";
    msg.level1.level2.level3.name = "Level 3";
    msg.level1.level2.level3.values = {"value1", "value2", "value3", "value4", "value5"};
    msg.level1.level2.level3.counts["count1"] = 10;
    msg.level1.level2.level3.counts["count2"] = 20;
    msg.level1.level2.level3.counts["count3"] = 30;

    Writer writer(bits_to_bytes(bit_size(msg)));
    serialize(writer, msg);
    std::vector<uint8_t> data = writer.bytes();

    for (int i = 0; i < n; ++i) {
        Reader reader(data);
        benchmark::NestedMessage result;
        deserialize(result, reader);
    }
}

// BenchmarkGeneratedMessageLargePayload benchmarks large payload messages
void BenchmarkGeneratedMessageLargePayload1KBSerialize(int n) {
    benchmark::LargePayloadRequest req;
    req.requestID = scg::type::uuid::random();
    req.createdAt = scg::type::timestamp();
    req.binaryData.resize(1024);
    for (size_t i = 0; i < req.binaryData.size(); ++i) {
        req.binaryData[i] = static_cast<uint8_t>(i % 256);
    }
    req.tags = {"tag1", "tag2", "tag3", "tag4", "tag5"};

    benchmark::MetricValue metric1;
    metric1.value = 75.5;
    metric1.timestamp = scg::type::timestamp();
    metric1.unit = "percent";
    metric1.labels["host"] = "server1";
    req.metrics["cpu"] = metric1;

    benchmark::MetricValue metric2;
    metric2.value = 8192.0;
    metric2.timestamp = scg::type::timestamp();
    metric2.unit = "MB";
    metric2.labels["host"] = "server1";
    req.metrics["memory"] = metric2;

    for (int i = 0; i < n; ++i) {
        Writer writer(bits_to_bytes(bit_size(req)));
        serialize(writer, req);
    }
}

void BenchmarkGeneratedMessageLargePayload1KBDeserialize(int n) {
    benchmark::LargePayloadRequest req;
    req.requestID = scg::type::uuid::random();
    req.createdAt = scg::type::timestamp();
    req.binaryData.resize(1024);
    for (size_t i = 0; i < req.binaryData.size(); ++i) {
        req.binaryData[i] = static_cast<uint8_t>(i % 256);
    }
    req.tags = {"tag1", "tag2", "tag3", "tag4", "tag5"};

    benchmark::MetricValue metric1;
    metric1.value = 75.5;
    metric1.timestamp = scg::type::timestamp();
    metric1.unit = "percent";
    metric1.labels["host"] = "server1";
    req.metrics["cpu"] = metric1;

    benchmark::MetricValue metric2;
    metric2.value = 8192.0;
    metric2.timestamp = scg::type::timestamp();
    metric2.unit = "MB";
    metric2.labels["host"] = "server1";
    req.metrics["memory"] = metric2;

    Writer writer(bits_to_bytes(bit_size(req)));
    serialize(writer, req);
    std::vector<uint8_t> data = writer.bytes();

    for (int i = 0; i < n; ++i) {
        Reader reader(data);
        benchmark::LargePayloadRequest result;
        deserialize(result, reader);
    }
}

void BenchmarkGeneratedMessageLargePayload10KBSerialize(int n) {
    benchmark::LargePayloadRequest req;
    req.requestID = scg::type::uuid::random();
    req.createdAt = scg::type::timestamp();
    req.binaryData.resize(10 * 1024);
    for (size_t i = 0; i < req.binaryData.size(); ++i) {
        req.binaryData[i] = static_cast<uint8_t>(i % 256);
    }
    req.tags = {"tag1", "tag2", "tag3", "tag4", "tag5"};

    benchmark::MetricValue metric1;
    metric1.value = 75.5;
    metric1.timestamp = scg::type::timestamp();
    metric1.unit = "percent";
    metric1.labels["host"] = "server1";
    req.metrics["cpu"] = metric1;

    benchmark::MetricValue metric2;
    metric2.value = 8192.0;
    metric2.timestamp = scg::type::timestamp();
    metric2.unit = "MB";
    metric2.labels["host"] = "server1";
    req.metrics["memory"] = metric2;

    for (int i = 0; i < n; ++i) {
        Writer writer(bits_to_bytes(bit_size(req)));
        serialize(writer, req);
    }
}

void BenchmarkGeneratedMessageLargePayload10KBDeserialize(int n) {
    benchmark::LargePayloadRequest req;
    req.requestID = scg::type::uuid::random();
    req.createdAt = scg::type::timestamp();
    req.binaryData.resize(10 * 1024);
    for (size_t i = 0; i < req.binaryData.size(); ++i) {
        req.binaryData[i] = static_cast<uint8_t>(i % 256);
    }
    req.tags = {"tag1", "tag2", "tag3", "tag4", "tag5"};

    benchmark::MetricValue metric1;
    metric1.value = 75.5;
    metric1.timestamp = scg::type::timestamp();
    metric1.unit = "percent";
    metric1.labels["host"] = "server1";
    req.metrics["cpu"] = metric1;

    benchmark::MetricValue metric2;
    metric2.value = 8192.0;
    metric2.timestamp = scg::type::timestamp();
    metric2.unit = "MB";
    metric2.labels["host"] = "server1";
    req.metrics["memory"] = metric2;

    Writer writer(bits_to_bytes(bit_size(req)));
    serialize(writer, req);
    std::vector<uint8_t> data = writer.bytes();

    for (int i = 0; i < n; ++i) {
        Reader reader(data);
        benchmark::LargePayloadRequest result;
        deserialize(result, reader);
    }
}

int main(int argc, char** argv) {
    bool profile = false;
    if (argc > 1 && std::string(argv[1]) == "--profile") {
        profile = true;
    }

    if (profile) ProfilerStart("message_bench.prof");

    std::cout << "Running C++ Message Benchmarks..." << std::endl;
    std::cout << std::left << std::setw(50) << "Benchmark"
              << std::right << std::setw(12) << "Iterations"
              << std::setw(15) << "ns/op" << std::endl;
    std::cout << std::string(77, '-') << std::endl;

    // Small message benchmarks
    RunBenchmark("BenchmarkGeneratedMessageSmall/Serialize", BenchmarkGeneratedMessageSmallSerialize);
    RunBenchmark("BenchmarkGeneratedMessageSmall/Deserialize", BenchmarkGeneratedMessageSmallDeserialize);

    // Echo message benchmarks
    RunBenchmark("BenchmarkGeneratedMessageEcho/Request/Serialize", BenchmarkGeneratedMessageEchoRequestSerialize);
    RunBenchmark("BenchmarkGeneratedMessageEcho/Request/Deserialize", BenchmarkGeneratedMessageEchoRequestDeserialize);
    RunBenchmark("BenchmarkGeneratedMessageEcho/Response/Serialize", BenchmarkGeneratedMessageEchoResponseSerialize);
    RunBenchmark("BenchmarkGeneratedMessageEcho/Response/Deserialize", BenchmarkGeneratedMessageEchoResponseDeserialize);

    // Process message benchmarks
    RunBenchmark("BenchmarkGeneratedMessageProcess/Request/Serialize", BenchmarkGeneratedMessageProcessRequestSerialize);
    RunBenchmark("BenchmarkGeneratedMessageProcess/Request/Deserialize", BenchmarkGeneratedMessageProcessRequestDeserialize);
    RunBenchmark("BenchmarkGeneratedMessageProcess/Response/Serialize", BenchmarkGeneratedMessageProcessResponseSerialize);
    RunBenchmark("BenchmarkGeneratedMessageProcess/Response/Deserialize", BenchmarkGeneratedMessageProcessResponseDeserialize);

    // Nested message benchmarks
    RunBenchmark("BenchmarkGeneratedMessageNested/Serialize", BenchmarkGeneratedMessageNestedSerialize);
    RunBenchmark("BenchmarkGeneratedMessageNested/Deserialize", BenchmarkGeneratedMessageNestedDeserialize);

    // Large payload benchmarks
    RunBenchmark("BenchmarkGeneratedMessageLargePayload/1KB/Serialize", BenchmarkGeneratedMessageLargePayload1KBSerialize, 1000000);
    RunBenchmark("BenchmarkGeneratedMessageLargePayload/1KB/Deserialize", BenchmarkGeneratedMessageLargePayload1KBDeserialize, 1000000);
    RunBenchmark("BenchmarkGeneratedMessageLargePayload/10KB/Serialize", BenchmarkGeneratedMessageLargePayload10KBSerialize, 100000);
    RunBenchmark("BenchmarkGeneratedMessageLargePayload/10KB/Deserialize", BenchmarkGeneratedMessageLargePayload10KBDeserialize, 100000);

    if (profile) ProfilerStop();

    return 0;
}
