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

    std::cout << std::left << std::setw(40) << name
              << std::right << std::setw(12) << iterations
              << std::setw(15) << std::fixed << std::setprecision(2) << ns_per_op << " ns/op" << std::endl;
}

void BenchmarkSerializeUInt8(int n) {
    uint8_t val = 123;
    for (int i = 0; i < n; ++i) {
        Writer writer(bits_to_bytes(bit_size(val)));
        serialize(writer, val);
    }
}

void BenchmarkSerializeUInt8Reuse(int n) {
    uint8_t val = 123;
    Writer writer(bits_to_bytes(bit_size(val)));
    for (int i = 0; i < n; ++i) {
        writer.clear();
        serialize(writer, val);
    }
}

void BenchmarkDeserializeUInt8(int n) {
    uint8_t val = 123;
    Writer writer(bits_to_bytes(bit_size(val)));
    serialize(writer, val);
    std::vector<uint8_t> data = writer.bytes();

    for (int i = 0; i < n; ++i) {
        Reader reader(data);
        uint8_t out;
        deserialize(out, reader);
    }
}

void BenchmarkSerializeUInt32(int n, uint32_t val) {
    for (int i = 0; i < n; ++i) {
        Writer writer(bits_to_bytes(bit_size(val)));
        serialize(writer, val);
    }
}

void BenchmarkSerializeUInt32Reuse(int n, uint32_t val) {
    Writer writer(bits_to_bytes(bit_size(val)));
    for (int i = 0; i < n; ++i) {
        writer.clear();
        serialize(writer, val);
    }
}

void BenchmarkDeserializeUInt32(int n, uint32_t val) {
    Writer writer(bits_to_bytes(bit_size(val)));
    serialize(writer, val);
    std::vector<uint8_t> data = writer.bytes();

    for (int i = 0; i < n; ++i) {
        Reader reader(data);
        uint32_t out;
        deserialize(out, reader);
    }
}

void BenchmarkSerializeString(int n, const std::string& val) {
    for (int i = 0; i < n; ++i) {
        Writer writer(bits_to_bytes(bit_size(val)));
        serialize(writer, val);
    }
}

void BenchmarkSerializeStringReuse(int n, const std::string& val) {
    Writer writer(bits_to_bytes(bit_size(val)));
    for (int i = 0; i < n; ++i) {
        writer.clear();
        serialize(writer, val);
    }
}

void BenchmarkDeserializeString(int n, const std::string& val) {
    Writer writer(bits_to_bytes(bit_size(val)));
    serialize(writer, val);
    std::vector<uint8_t> data = writer.bytes();

    for (int i = 0; i < n; ++i) {
        Reader reader(data);
        std::string out;
        deserialize(out, reader);
    }
}

void BenchmarkSerializeFloat32(int n) {
    float32_t val = 123.456f;
    for (int i = 0; i < n; ++i) {
        Writer writer(bits_to_bytes(bit_size(val)));
        serialize(writer, val);
    }
}

void BenchmarkSerializeFloat32Reuse(int n) {
    float32_t val = 123.456f;
    Writer writer(bits_to_bytes(bit_size(val)));
    for (int i = 0; i < n; ++i) {
        writer.clear();
        serialize(writer, val);
    }
}

void BenchmarkDeserializeFloat32(int n) {
    float32_t val = 123.456f;
    Writer writer(bits_to_bytes(bit_size(val)));
    serialize(writer, val);
    std::vector<uint8_t> data = writer.bytes();

    for (int i = 0; i < n; ++i) {
        Reader reader(data);
        float32_t out;
        deserialize(out, reader);
    }
}

void BenchmarkSerializeFloat64(int n) {
    float64_t val = 123.4567890123;
    for (int i = 0; i < n; ++i) {
        Writer writer(bits_to_bytes(bit_size(val)));
        serialize(writer, val);
    }
}

void BenchmarkSerializeFloat64Reuse(int n) {
    float64_t val = 123.4567890123;
    Writer writer(bits_to_bytes(bit_size(val)));
    for (int i = 0; i < n; ++i) {
        writer.clear();
        serialize(writer, val);
    }
}

void BenchmarkDeserializeFloat64(int n) {
    float64_t val = 123.4567890123;
    Writer writer(bits_to_bytes(bit_size(val)));
    serialize(writer, val);
    std::vector<uint8_t> data = writer.bytes();

    for (int i = 0; i < n; ++i) {
        Reader reader(data);
        float64_t out;
        deserialize(out, reader);
    }
}

void BenchmarkWriteBytesAligned(int n) {
    std::vector<uint8_t> data(1024, 0xAA);
    volatile size_t total_size = 0;
    for (int i = 0; i < n; ++i) {
        Writer writer(data.size());
        writer.writeBytes(data.data(), data.size());
        total_size = total_size + writer.bytes().size();
    }
}

void BenchmarkWriteBytesAlignedReuse(int n) {
    std::vector<uint8_t> data(1024, 0xAA);
    volatile size_t total_size = 0;
    Writer writer(data.size());
    for (int i = 0; i < n; ++i) {
        writer.clear();
        writer.writeBytes(data.data(), data.size());
        total_size = total_size + writer.bytes().size();
    }
}

void BenchmarkWriteBytesUnaligned(int n) {
    std::vector<uint8_t> data(1024, 0xAA);
    volatile size_t total_size = 0;
    for (int i = 0; i < n; ++i) {
        Writer writer(bits_to_bytes(1 + data.size() * 8));
        writer.writeBits(1, 1); // Offset by 1 bit
        writer.writeBytes(data.data(), data.size());
        total_size = total_size + writer.bytes().size();
    }
}

void BenchmarkWriteBytesUnalignedReuse(int n) {
    std::vector<uint8_t> data(1024, 0xAA);
    volatile size_t total_size = 0;
    Writer writer(bits_to_bytes(1 + data.size() * 8));
    for (int i = 0; i < n; ++i) {
        writer.clear();
        writer.writeBits(1, 1); // Offset by 1 bit
        writer.writeBytes(data.data(), data.size());
        total_size = total_size + writer.bytes().size();
    }
}

void BenchmarkReadBytesAligned(int n) {
    std::vector<uint8_t> data(1024, 0xAA);
    Writer writer(data.size());
    writer.writeBytes(data.data(), data.size());
    std::vector<uint8_t> bytes = writer.bytes();

    std::vector<uint8_t> out(1024);
    volatile uint8_t sink = 0;
    for (int i = 0; i < n; ++i) {
        Reader reader(bytes);
        reader.readBytes(out.data(), out.size());
        sink = sink + out[0];
    }
}

void BenchmarkReadBytesUnaligned(int n) {
    std::vector<uint8_t> data(1024, 0xAA);
    Writer writer(bits_to_bytes(1 + data.size() * 8));
    writer.writeBits(1, 1);
    writer.writeBytes(data.data(), data.size());
    std::vector<uint8_t> bytes = writer.bytes();

    std::vector<uint8_t> out(1024);
    volatile uint8_t sink = 0;
    for (int i = 0; i < n; ++i) {
        Reader reader(bytes);
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
    RunBenchmark("BenchmarkSerializeUInt8Reuse", BenchmarkSerializeUInt8Reuse);
    RunBenchmark("BenchmarkDeserializeUInt8", BenchmarkDeserializeUInt8);

    RunBenchmark("BenchmarkSerializeUInt32/Small", [](int n){ BenchmarkSerializeUInt32(n, 10); });
    RunBenchmark("BenchmarkSerializeUInt32Reuse/Small", [](int n){ BenchmarkSerializeUInt32Reuse(n, 10); });
    RunBenchmark("BenchmarkSerializeUInt32/Medium", [](int n){ BenchmarkSerializeUInt32(n, 1000); });
    RunBenchmark("BenchmarkSerializeUInt32Reuse/Medium", [](int n){ BenchmarkSerializeUInt32Reuse(n, 1000); });
    RunBenchmark("BenchmarkSerializeUInt32/Large", [](int n){ BenchmarkSerializeUInt32(n, 100000); });
    RunBenchmark("BenchmarkSerializeUInt32Reuse/Large", [](int n){ BenchmarkSerializeUInt32Reuse(n, 100000); });

    RunBenchmark("BenchmarkDeserializeUInt32/Small", [](int n){ BenchmarkDeserializeUInt32(n, 10); });
    RunBenchmark("BenchmarkDeserializeUInt32/Medium", [](int n){ BenchmarkDeserializeUInt32(n, 1000); });
    RunBenchmark("BenchmarkDeserializeUInt32/Large", [](int n){ BenchmarkDeserializeUInt32(n, 100000); });

    RunBenchmark("BenchmarkSerializeString/Empty", [](int n){ BenchmarkSerializeString(n, ""); });
    RunBenchmark("BenchmarkSerializeStringReuse/Empty", [](int n){ BenchmarkSerializeStringReuse(n, ""); });
    RunBenchmark("BenchmarkSerializeString/Short", [](int n){ BenchmarkSerializeString(n, "hello"); });
    RunBenchmark("BenchmarkSerializeStringReuse/Short", [](int n){ BenchmarkSerializeStringReuse(n, "hello"); });
    RunBenchmark("BenchmarkSerializeString/Medium", [](int n){ BenchmarkSerializeString(n, "Hello, World! This is a medium length string for benchmarking."); });
    RunBenchmark("BenchmarkSerializeStringReuse/Medium", [](int n){ BenchmarkSerializeStringReuse(n, "Hello, World! This is a medium length string for benchmarking."); });

    RunBenchmark("BenchmarkDeserializeString/Empty", [](int n){ BenchmarkDeserializeString(n, ""); });
    RunBenchmark("BenchmarkDeserializeString/Short", [](int n){ BenchmarkDeserializeString(n, "hello"); });
    RunBenchmark("BenchmarkDeserializeString/Medium", [](int n){ BenchmarkDeserializeString(n, "Hello, World! This is a medium length string for benchmarking."); });

    RunBenchmark("BenchmarkSerializeFloat32", BenchmarkSerializeFloat32);
    RunBenchmark("BenchmarkSerializeFloat32Reuse", BenchmarkSerializeFloat32Reuse);
    RunBenchmark("BenchmarkDeserializeFloat32", BenchmarkDeserializeFloat32);
    RunBenchmark("BenchmarkSerializeFloat64", BenchmarkSerializeFloat64);
    RunBenchmark("BenchmarkSerializeFloat64Reuse", BenchmarkSerializeFloat64Reuse);
    RunBenchmark("BenchmarkDeserializeFloat64", BenchmarkDeserializeFloat64);

    RunBenchmark("BenchmarkWriteBytesAligned", BenchmarkWriteBytesAligned, 1000000);
    RunBenchmark("BenchmarkWriteBytesAlignedReuse", BenchmarkWriteBytesAlignedReuse, 1000000);
    RunBenchmark("BenchmarkWriteBytesUnaligned", BenchmarkWriteBytesUnaligned, 1000000);
    RunBenchmark("BenchmarkWriteBytesUnalignedReuse", BenchmarkWriteBytesUnalignedReuse, 1000000);
    RunBenchmark("BenchmarkReadBytesAligned", BenchmarkReadBytesAligned, 1000000);
    RunBenchmark("BenchmarkReadBytesUnaligned", BenchmarkReadBytesUnaligned, 1000000);

    if (profile) ProfilerStop();

    return 0;
}
