#include <iostream>
#include <chrono>
#include <vector>
#include <functional>
#include <iomanip>
#include <gperftools/profiler.h>
#include <scg/serialize.h>
#include <scg/writer.h>
#include <scg/reader.h>

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

void BenchmarkVarEncodeUint64(int n) {
    Writer writer(1024);
    uint64_t val = 0xDEADBEEFCAFEBABE;
    for (int i = 0; i < n; ++i) {
        writer.clear();
        serialize(writer, val);
    }
}

void BenchmarkVarDecodeUint64(int n) {
    Writer writer(16);
    uint64_t val = 0xDEADBEEFCAFEBABE;
    serialize(writer, val);
    std::vector<unsigned char> data = writer.bytes();

    for (int i = 0; i < n; ++i) {
        Reader reader(data);
        uint64_t out;
        deserialize(out, reader);
    }
}

void BenchmarkVarEncodeInt64(int n) {
    Writer writer(1024);
    int64_t val = -1234567890123456789;
    for (int i = 0; i < n; ++i) {
        writer.clear();
        serialize(writer, val);
    }
}

void BenchmarkVarDecodeInt64(int n) {
    Writer writer(16);
    int64_t val = -1234567890123456789;
    serialize(writer, val);
    std::vector<unsigned char> data = writer.bytes();

    for (int i = 0; i < n; ++i) {
        Reader reader(data);
        int64_t out;
        deserialize(out, reader);
    }
}

int main(int argc, char** argv) {
    bool profile = false;
    if (argc > 1 && std::string(argv[1]) == "--profile") {
        profile = true;
    }

    if (profile) ProfilerStart("varint_bench.prof");

    std::cout << "Running C++ Varint Benchmarks..." << std::endl;
    std::cout << std::left << std::setw(40) << "Benchmark"
              << std::right << std::setw(12) << "Iterations"
              << std::setw(15) << "ns/op" << std::endl;
    std::cout << std::string(67, '-') << std::endl;

    RunBenchmark("BenchmarkVarEncodeUint64", BenchmarkVarEncodeUint64);
    RunBenchmark("BenchmarkVarDecodeUint64", BenchmarkVarDecodeUint64);
    RunBenchmark("BenchmarkVarEncodeInt64", BenchmarkVarEncodeInt64);
    RunBenchmark("BenchmarkVarDecodeInt64", BenchmarkVarDecodeInt64);

    if (profile) ProfilerStop();

    return 0;
}
