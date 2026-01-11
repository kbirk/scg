#include <iostream>
#include <vector>
#include <string>
#include <gperftools/profiler.h>

#include "benchmark.h"
#include "scg/serialize.h"
#include "scg/writer.h"
#include "scg/reader.h"
#include "scg/uuid.h"
#include "scg/timestamp.h"
#include "benchmark/benchmark.h"

using namespace scg::serialize;
using ::benchmark::Benchmark;
using ::benchmark::RunBenchmark;

// BenchmarkGeneratedMessageSmall benchmarks small generated messages
void BenchmarkGeneratedMessageSmallSerialize(Benchmark& b) {
	benchmark::SmallMessage msg;
	msg.id = 12345;
	msg.value = 67890;

	Writer writer(bits_to_bytes(bit_size(msg)));

	b.resetTimer();

	for (int i = 0; i < b.N; ++i) {
		writer.clear();
		serialize(writer, msg);
	}
}

void BenchmarkGeneratedMessageSmallDeserialize(Benchmark& b) {
	benchmark::SmallMessage msg;
	msg.id = 12345;
	msg.value = 67890;

	Writer writer(bits_to_bytes(bit_size(msg)));
	serialize(writer, msg);
	std::vector<uint8_t> data = writer.bytes();

	b.resetTimer();

	for (int i = 0; i < b.N; ++i) {
		ReaderView reader(data);
		benchmark::SmallMessage result;
		deserialize(result, reader);
		BENCHMARK_DONT_OPTIMIZE(result);
	}
}

// BenchmarkGeneratedMessageEcho benchmarks echo request/response messages
void BenchmarkGeneratedMessageEchoRequestSerialize(Benchmark& b) {
	benchmark::EchoRequest req;
	req.message = "Hello, World! This is a test message for benchmarking purposes.";
	req.timestamp = static_cast<uint64_t>(std::chrono::duration_cast<std::chrono::nanoseconds>(
		std::chrono::system_clock::now().time_since_epoch()).count());

	Writer writer(bits_to_bytes(bit_size(req)));

	b.resetTimer();

	for (int i = 0; i < b.N; ++i) {
		writer.clear();
		serialize(writer, req);
	}
}

void BenchmarkGeneratedMessageEchoRequestDeserialize(Benchmark& b) {
	benchmark::EchoRequest req;
	req.message = "Hello, World! This is a test message for benchmarking purposes.";
	req.timestamp = static_cast<uint64_t>(std::chrono::duration_cast<std::chrono::nanoseconds>(
		std::chrono::system_clock::now().time_since_epoch()).count());

	Writer writer(bits_to_bytes(bit_size(req)));
	serialize(writer, req);
	std::vector<uint8_t> data = writer.bytes();

	b.resetTimer();

	for (int i = 0; i < b.N; ++i) {
		ReaderView reader(data);
		benchmark::EchoRequest result;
		deserialize(result, reader);
		BENCHMARK_DONT_OPTIMIZE(result);
	}
}

void BenchmarkGeneratedMessageEchoResponseSerialize(Benchmark& b) {
	benchmark::EchoResponse resp;
	resp.message = "Hello, World! This is a test message for benchmarking purposes.";
	resp.timestamp = static_cast<uint64_t>(std::chrono::duration_cast<std::chrono::nanoseconds>(
		std::chrono::system_clock::now().time_since_epoch()).count());
	resp.serverTimestamp = static_cast<uint64_t>(std::chrono::duration_cast<std::chrono::nanoseconds>(
		std::chrono::system_clock::now().time_since_epoch()).count());

	Writer writer(bits_to_bytes(bit_size(resp)));

	b.resetTimer();

	for (int i = 0; i < b.N; ++i) {
		writer.clear();
		serialize(writer, resp);
	}
}

void BenchmarkGeneratedMessageEchoResponseDeserialize(Benchmark& b) {
	benchmark::EchoResponse resp;
	resp.message = "Hello, World! This is a test message for benchmarking purposes.";
	resp.timestamp = static_cast<uint64_t>(std::chrono::duration_cast<std::chrono::nanoseconds>(
		std::chrono::system_clock::now().time_since_epoch()).count());
	resp.serverTimestamp = static_cast<uint64_t>(std::chrono::duration_cast<std::chrono::nanoseconds>(
		std::chrono::system_clock::now().time_since_epoch()).count());

	Writer writer(bits_to_bytes(bit_size(resp)));
	serialize(writer, resp);
	std::vector<uint8_t> data = writer.bytes();

	b.resetTimer();

	for (int i = 0; i < b.N; ++i) {
		ReaderView reader(data);
		benchmark::EchoResponse result;
		deserialize(result, reader);
		BENCHMARK_DONT_OPTIMIZE(result);
	}
}

// BenchmarkGeneratedMessageProcess benchmarks complex business messages
void BenchmarkGeneratedMessageProcessRequestSerialize(Benchmark& b) {
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

	b.resetTimer();

	for (int i = 0; i < b.N; ++i) {
		writer.clear();
		serialize(writer, req);
	}
}

void BenchmarkGeneratedMessageProcessRequestDeserialize(Benchmark& b) {
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

	b.resetTimer();

	for (int i = 0; i < b.N; ++i) {
		ReaderView reader(data);
		benchmark::ProcessRequest result;
		deserialize(result, reader);
		BENCHMARK_DONT_OPTIMIZE(result);
	}
}

void BenchmarkGeneratedMessageProcessResponseSerialize(Benchmark& b) {
	benchmark::ProcessResponse resp;
	resp.id = "order-12345";
	resp.processedAt = scg::type::timestamp();
	resp.status = benchmark::ProcessStatus::SUCCESS;
	resp.message = "Order processed successfully";
	resp.stats.itemsProcessed = 2;
	resp.stats.totalAmount = 89.97;
	resp.stats.processingTimeMs = 42;

	Writer writer(bits_to_bytes(bit_size(resp)));

	b.resetTimer();

	for (int i = 0; i < b.N; ++i) {
		writer.clear();
		serialize(writer, resp);
	}
}

void BenchmarkGeneratedMessageProcessResponseDeserialize(Benchmark& b) {
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

	b.resetTimer();

	for (int i = 0; i < b.N; ++i) {
		ReaderView reader(data);
		benchmark::ProcessResponse result;
		deserialize(result, reader);
		BENCHMARK_DONT_OPTIMIZE(result);
	}
}

// BenchmarkGeneratedMessageNested benchmarks deeply nested messages
void BenchmarkGeneratedMessageNestedSerialize(Benchmark& b) {
	benchmark::NestedMessage msg;
	msg.level1.name = "Level 1";
	msg.level1.level2.name = "Level 2";
	msg.level1.level2.level3.name = "Level 3";
	msg.level1.level2.level3.values = {"value1", "value2", "value3", "value4", "value5"};
	msg.level1.level2.level3.counts["count1"] = 10;
	msg.level1.level2.level3.counts["count2"] = 20;
	msg.level1.level2.level3.counts["count3"] = 30;

	Writer writer(bits_to_bytes(bit_size(msg)));

	b.resetTimer();

	for (int i = 0; i < b.N; ++i) {
		writer.clear();
		serialize(writer, msg);
	}
}

void BenchmarkGeneratedMessageNestedDeserialize(Benchmark& b) {
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

	b.resetTimer();

	for (int i = 0; i < b.N; ++i) {
		ReaderView reader(data);
		benchmark::NestedMessage result;
		deserialize(result, reader);
		BENCHMARK_DONT_OPTIMIZE(result);
	}
}

// BenchmarkGeneratedMessageLargePayload benchmarks large payload messages
void BenchmarkGeneratedMessageLargePayload1KBSerialize(Benchmark& b) {
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

	b.resetTimer();

	for (int i = 0; i < b.N; ++i) {
		writer.clear();
		serialize(writer, req);
	}
}

void BenchmarkGeneratedMessageLargePayload1KBDeserialize(Benchmark& b) {
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

	b.resetTimer();

	for (int i = 0; i < b.N; ++i) {
		ReaderView reader(data);
		benchmark::LargePayloadRequest result;
		deserialize(result, reader);		BENCHMARK_DONT_OPTIMIZE(result);	}
}

void BenchmarkGeneratedMessageLargePayload10KBSerialize(Benchmark& b) {
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

	b.resetTimer();

	for (int i = 0; i < b.N; ++i) {
		writer.clear();
		serialize(writer, req);
	}
}

void BenchmarkGeneratedMessageLargePayload10KBDeserialize(Benchmark& b) {
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

	b.resetTimer();

	for (int i = 0; i < b.N; ++i) {
		ReaderView reader(data);
		benchmark::LargePayloadRequest result;
		deserialize(result, reader);
		BENCHMARK_DONT_OPTIMIZE(result);
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
