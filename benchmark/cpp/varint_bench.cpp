#include <iostream>
#include <vector>
#include <gperftools/profiler.h>

#include "benchmark.h"
#include <scg/serialize.h>
#include <scg/writer.h>
#include <scg/reader.h>

using namespace scg::serialize;
using benchmark::Benchmark;
using benchmark::RunBenchmark;

volatile uint64_t sink = 0;

void BenchmarkVarEncodeUint64(Benchmark& b) {
	Writer writer(1024);
	uint64_t val = 0xDEADBEEFCAFEBABE;

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		writer.clear();
		serialize(writer, val);
	}
}

void BenchmarkVarDecodeUint64(Benchmark& b) {
	uint64_t val = 0xDEADBEEFCAFEBABE;
	Writer writer(16);
	serialize(writer, val);
	std::vector<unsigned char> data = writer.bytes();

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		ReaderView reader(data);
		uint64_t out = 0;
		deserialize(out, reader);
		BENCHMARK_DONT_OPTIMIZE(out);
	}
}

void BenchmarkVarEncodeInt64(Benchmark& b) {
	Writer writer(16);
	int64_t val = -1234567890123456789;

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		writer.clear();
		serialize(writer, val);
	}
}

void BenchmarkVarDecodeInt64(Benchmark& b) {
	int64_t val = -1234567890123456789;
	Writer writer(16);
	serialize(writer, val);
	std::vector<unsigned char> data = writer.bytes();

	b.resetTimer();
	for (int i = 0; i < b.N; ++i) {
		ReaderView reader(data);
		int64_t out = 0;
		deserialize(out, reader);
		BENCHMARK_DONT_OPTIMIZE(out);
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
