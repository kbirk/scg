#include <iostream>
#include <vector>
#include <string>
#include <cstring>
#include <gperftools/profiler.h>

#include "benchmark.h"
#include "scg/serialize.h"
#include "scg/writer.h"
#include "scg/reader.h"
#include "scg/uuid.h"
#include "scg/timestamp.h"

using namespace scg::serialize;
using benchmark::Benchmark;
using benchmark::RunBenchmark;

volatile uint64_t sink = 0;

void BenchmarkSerializeUInt8(Benchmark& b) {
	uint8_t val = 123;
	Writer writer(bits_to_bytes(bit_size(val)));

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		writer.clear();
		serialize(writer, val);
	}
}

void BenchmarkDeserializeUInt8(Benchmark& b) {
	uint8_t val = 123;
	Writer writer(bits_to_bytes(bit_size(val)));
	serialize(writer, val);
	std::vector<uint8_t> data = writer.bytes();

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		ReaderView reader(data);
		uint8_t out = 0;
		deserialize(out, reader);
		BENCHMARK_DONT_OPTIMIZE(out);
	}
}

void BenchmarkSerializeUInt32(Benchmark& b, uint32_t val) {
	Writer writer(bits_to_bytes(bit_size(val)));

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		writer.clear();
		serialize(writer, val);
	}
}

void BenchmarkDeserializeUInt32(Benchmark& b, uint32_t val) {
	Writer writer(bits_to_bytes(bit_size(val)));
	serialize(writer, val);
	std::vector<uint8_t> data = writer.bytes();

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		ReaderView reader(data);
		uint32_t out = 0;
		deserialize(out, reader);
		BENCHMARK_DONT_OPTIMIZE(out);
	}
}

void BenchmarkSerializeString(Benchmark& b, const std::string& val) {
	Writer writer(bits_to_bytes(bit_size(val)));

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		writer.clear();
		serialize(writer, val);
	}
}

void BenchmarkDeserializeString(Benchmark& b, const std::string& val) {
	Writer writer(bits_to_bytes(bit_size(val)));
	serialize(writer, val);
	std::vector<uint8_t> data = writer.bytes();

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		ReaderView reader(data);
		std::string out;
		deserialize(out, reader);
		BENCHMARK_DONT_OPTIMIZE(out);
	}
}

void BenchmarkSerializeFloat32(Benchmark& b) {
	float32_t val = 123.456f;
	Writer writer(bits_to_bytes(bit_size(val)));

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		writer.clear();
		serialize(writer, val);
	}
}

void BenchmarkDeserializeFloat32(Benchmark& b) {
	float32_t val = 123.456f;
	Writer writer(bits_to_bytes(bit_size(val)));
	serialize(writer, val);
	std::vector<uint8_t> data = writer.bytes();

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		ReaderView reader(data);
		float32_t out = 0;
		deserialize(out, reader);
		BENCHMARK_DONT_OPTIMIZE(out);
	}
}

void BenchmarkSerializeFloat64(Benchmark& b) {
	float64_t val = 3.14159265359;
	Writer writer(bits_to_bytes(bit_size(val)));

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		writer.clear();
		serialize(writer, val);
	}
}

void BenchmarkDeserializeFloat64(Benchmark& b) {
	float64_t val = 3.14159265359;
	Writer writer(bits_to_bytes(bit_size(val)));
	serialize(writer, val);
	std::vector<uint8_t> data = writer.bytes();

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		ReaderView reader(data);
		float64_t out = 0;
		deserialize(out, reader);
		BENCHMARK_DONT_OPTIMIZE(out);
	}
}

void BenchmarkSerializeUUID(Benchmark& b) {
	scg::type::uuid id = scg::type::uuid::random();
	Writer writer(bits_to_bytes(bit_size(id)));

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		writer.clear();
		serialize(writer, id);
	}
}

void BenchmarkDeserializeUUID(Benchmark& b) {
	scg::type::uuid id = scg::type::uuid::random();
	Writer writer(bits_to_bytes(bit_size(id)));
	serialize(writer, id);
	std::vector<uint8_t> data = writer.bytes();

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		ReaderView reader(data);
		scg::type::uuid out;
		deserialize(out, reader);
		BENCHMARK_DONT_OPTIMIZE(out);
	}
}

void BenchmarkSerializeTimestamp(Benchmark& b) {
	scg::type::timestamp ts;
	Writer writer(bits_to_bytes(bit_size(ts)));

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		writer.clear();
		serialize(writer, ts);
	}
}

void BenchmarkDeserializeTimestamp(Benchmark& b) {
	scg::type::timestamp ts;
	Writer writer(bits_to_bytes(bit_size(ts)));
	serialize(writer, ts);
	std::vector<uint8_t> data = writer.bytes();

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		ReaderView reader(data);
		scg::type::timestamp out;
		deserialize(out, reader);
		BENCHMARK_DONT_OPTIMIZE(out);
	}
}

void BenchmarkWriteBytesAligned(Benchmark& b) {
	std::vector<uint8_t> data(1024, 0xAA);
	volatile size_t total_size = 0;
	Writer writer(data.size());

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		writer.clear();
		writer.writeBytes(data.data(), data.size());
		total_size = total_size + writer.bytes().size();
	}
}

void BenchmarkWriteBytesUnaligned(Benchmark& b) {
	std::vector<uint8_t> data(1024, 0xAA);
	volatile size_t total_size = 0;
	Writer writer(bits_to_bytes(1 + data.size() * 8));

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		writer.clear();
		writer.writeBits(1, 1); // Offset by 1 bit
		writer.writeBytes(data.data(), data.size());
		total_size = total_size + writer.bytes().size();
	}
}

void BenchmarkReadBytesAligned(Benchmark& b) {
	std::vector<uint8_t> data(1024, 0xAA);
	Writer writer(data.size());
	writer.writeBytes(data.data(), data.size());
	std::vector<uint8_t> bytes = writer.bytes();
	std::vector<uint8_t> out(1024);

	b.resetTimer();
	volatile uint8_t sink = 0;
	for (int i = 0; i < b.N; ++i) {
		ReaderView reader(bytes);
		reader.readBytes(out.data(), out.size());
		sink = sink + out[0];
	}
}

void BenchmarkReadBytesUnaligned(Benchmark& b) {
	std::vector<uint8_t> data(1024, 0xAA);
	Writer writer(bits_to_bytes(1 + data.size() * 8));
	writer.writeBits(1, 1);
	writer.writeBytes(data.data(), data.size());
	std::vector<uint8_t> bytes = writer.bytes();
	std::vector<uint8_t> out(1024);

	b.resetTimer();
	volatile uint8_t sink = 0;
	for (int i = 0; i < b.N; ++i) {
		ReaderView reader(bytes);
		uint8_t dummy;
		reader.readBits(dummy, 1);
		reader.readBytes(out.data(), out.size());
		sink = sink + out[0];
	}
}

int main(int argc, char** argv) {
	bool profile = false;
	if (argc > 1 && std::string(argv[1]) == "--profile") {
		profile = true;
	}

	if (profile) ProfilerStart("serialize_bench.prof");

	std::cout << "Running C++ Benchmarks..." << std::endl;
	std::cout << std::left << std::setw(40) << "Benchmark"
			  << std::right << std::setw(12) << "Iterations"
			  << std::setw(15) << "ns/op" << std::endl;
	std::cout << std::string(67, '-') << std::endl;

	RunBenchmark("BenchmarkSerializeUInt8", BenchmarkSerializeUInt8);
	RunBenchmark("BenchmarkDeserializeUInt8", BenchmarkDeserializeUInt8);

	RunBenchmark("BenchmarkSerializeUInt32/Small", [](Benchmark& b){ BenchmarkSerializeUInt32(b, 10); });
	RunBenchmark("BenchmarkSerializeUInt32/Medium", [](Benchmark& b){ BenchmarkSerializeUInt32(b, 1000); });
	RunBenchmark("BenchmarkSerializeUInt32/Large", [](Benchmark& b){ BenchmarkSerializeUInt32(b, 100000); });

	RunBenchmark("BenchmarkDeserializeUInt32/Small", [](Benchmark& b){ BenchmarkDeserializeUInt32(b, 10); });
	RunBenchmark("BenchmarkDeserializeUInt32/Medium", [](Benchmark& b){ BenchmarkDeserializeUInt32(b, 1000); });
	RunBenchmark("BenchmarkDeserializeUInt32/Large", [](Benchmark& b){ BenchmarkDeserializeUInt32(b, 100000); });

	RunBenchmark("BenchmarkSerializeString/Empty", [](Benchmark& b){ BenchmarkSerializeString(b, ""); });
	RunBenchmark("BenchmarkSerializeString/Short", [](Benchmark& b){ BenchmarkSerializeString(b, "hello"); });
	RunBenchmark("BenchmarkSerializeString/Medium", [](Benchmark& b){ BenchmarkSerializeString(b, "Hello, World! This is a medium length string for benchmarking."); });
	RunBenchmark("BenchmarkSerializeString/Long", [](Benchmark& b){ BenchmarkSerializeString(b, std::string(1024, '\0')); });

	RunBenchmark("BenchmarkDeserializeString/Empty", [](Benchmark& b){ BenchmarkDeserializeString(b, ""); });
	RunBenchmark("BenchmarkDeserializeString/Short", [](Benchmark& b){ BenchmarkDeserializeString(b, "hello"); });
	RunBenchmark("BenchmarkDeserializeString/Medium", [](Benchmark& b){ BenchmarkDeserializeString(b, "Hello, World! This is a medium length string for benchmarking."); });
	RunBenchmark("BenchmarkDeserializeString/Long", [](Benchmark& b){ BenchmarkDeserializeString(b, std::string(1024, '\0')); });

	RunBenchmark("BenchmarkSerializeFloat32", BenchmarkSerializeFloat32);
	RunBenchmark("BenchmarkDeserializeFloat32", BenchmarkDeserializeFloat32);

	RunBenchmark("BenchmarkSerializeFloat64", BenchmarkSerializeFloat64);
	RunBenchmark("BenchmarkDeserializeFloat64", BenchmarkDeserializeFloat64);

	RunBenchmark("BenchmarkSerializeUUID", BenchmarkSerializeUUID);
	RunBenchmark("BenchmarkDeserializeUUID", BenchmarkDeserializeUUID);

	RunBenchmark("BenchmarkSerializeTimestamp", BenchmarkSerializeTimestamp);
	RunBenchmark("BenchmarkDeserializeTimestamp", BenchmarkDeserializeTimestamp);

	RunBenchmark("BenchmarkWriteBytesAligned", BenchmarkWriteBytesAligned, 1000000);
	RunBenchmark("BenchmarkWriteBytesUnaligned", BenchmarkWriteBytesUnaligned, 1000000);
	RunBenchmark("BenchmarkReadBytesAligned", BenchmarkReadBytesAligned, 1000000);
	RunBenchmark("BenchmarkReadBytesUnaligned", BenchmarkReadBytesUnaligned, 1000000);

	if (profile) ProfilerStop();

	return 0;
}
