#pragma once

#include <chrono>
#include <functional>
#include <iomanip>
#include <iostream>
#include <string>

// Prevent compiler from optimizing away benchmark code
// Similar to Google Benchmark's DoNotOptimize
#define BENCHMARK_DONT_OPTIMIZE(value) \
	asm volatile("" : : "r,m"(value) : "memory")

namespace benchmark {

// Benchmark context object (like Go's *testing.B)
class Benchmark {
public:
	int N;

	Benchmark(int iterations) : N(iterations), timer_started_(false) {}

	void resetTimer() {
		timer_started_ = true;
		start_ = std::chrono::high_resolution_clock::now();
	}

	std::chrono::duration<double, std::nano> elapsed() const {
		auto end = std::chrono::high_resolution_clock::now();
		return end - start_;
	}

	bool timerStarted() const { return timer_started_; }

private:
	std::chrono::high_resolution_clock::time_point start_;
	bool timer_started_;
};

// Benchmark harness (like Go's testing framework)
inline void RunBenchmark(const std::string& name, std::function<void(Benchmark&)> func, int iterations = 10000000) {
	// Warmup
	Benchmark warmup(iterations / 100);
	func(warmup);

	// Actual benchmark
	Benchmark b(iterations);
	func(b);

	if (!b.timerStarted()) {
		// If resetTimer() was never called, measure the whole function
		b.resetTimer();
		Benchmark timed(iterations);
		func(timed);
	}

	double ns_per_op = b.elapsed().count() / iterations;

	std::cout << std::left << std::setw(40) << name << std::right << std::setw(12) << iterations << std::setw(15)
			  << std::fixed << std::setprecision(2) << ns_per_op << " ns/op" << std::endl;
}

} // namespace benchmark
