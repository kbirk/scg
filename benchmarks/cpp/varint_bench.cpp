#include <iostream>
#include <chrono>
#include <vector>
#include <scg/serialize.h>
#include <scg/writer.h>
#include <scg/reader.h>

using namespace scg::serialize;

void BenchmarkVarEncodeUint64() {
    Writer writer(1024);
    uint64_t val = 0xDEADBEEFCAFEBABE;

    auto start = std::chrono::high_resolution_clock::now();
    int iterations = 10000000;
    for (int i = 0; i < iterations; ++i) {
        writer.clear();
        serialize(writer, val);
    }
    auto end = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> diff = end - start;

    std::cout << "BenchmarkVarEncodeUint64: " << (diff.count() * 1e9 / iterations) << " ns/op" << std::endl;
}

void BenchmarkVarDecodeUint64() {
    Writer writer(16);
    uint64_t val = 0xDEADBEEFCAFEBABE;
    serialize(writer, val);
    std::vector<unsigned char> data = writer.bytes();

    auto start = std::chrono::high_resolution_clock::now();
    int iterations = 10000000;
    for (int i = 0; i < iterations; ++i) {
        Reader reader(data);
        uint64_t out;
        deserialize(out, reader);
    }
    auto end = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> diff = end - start;

    std::cout << "BenchmarkVarDecodeUint64: " << (diff.count() * 1e9 / iterations) << " ns/op" << std::endl;
}

void BenchmarkVarEncodeInt64() {
    Writer writer(1024);
    int64_t val = -1234567890123456789;

    auto start = std::chrono::high_resolution_clock::now();
    int iterations = 10000000;
    for (int i = 0; i < iterations; ++i) {
        writer.clear();
        serialize(writer, val);
    }
    auto end = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> diff = end - start;

    std::cout << "BenchmarkVarEncodeInt64: " << (diff.count() * 1e9 / iterations) << " ns/op" << std::endl;
}

void BenchmarkVarDecodeInt64() {
    Writer writer(16);
    int64_t val = -1234567890123456789;
    serialize(writer, val);
    std::vector<unsigned char> data = writer.bytes();

    auto start = std::chrono::high_resolution_clock::now();
    int iterations = 10000000;
    for (int i = 0; i < iterations; ++i) {
        Reader reader(data);
        int64_t out;
        deserialize(out, reader);
    }
    auto end = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> diff = end - start;

    std::cout << "BenchmarkVarDecodeInt64: " << (diff.count() * 1e9 / iterations) << " ns/op" << std::endl;
}

int main() {
    BenchmarkVarEncodeUint64();
    BenchmarkVarDecodeUint64();
    BenchmarkVarEncodeInt64();
    BenchmarkVarDecodeInt64();
    return 0;
}
