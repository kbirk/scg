// Client behaviour that needs no network: connect bookkeeping and deadline
// arithmetic, driven through fake transports.

#include <cstdio>
#include <acutest.h>

#include <atomic>
#include <chrono>
#include <memory>
#include <mutex>
#include <stdexcept>
#include <string>

#include "scg/client.h"

namespace {

using steady = std::chrono::steady_clock;

// FakeTransport counts connect attempts, records the deadline it was given and
// either throws (while throwsLeft > 0) or fails with an error.
class FakeTransport : public scg::rpc::ClientTransport {
public:
	std::atomic<int> calls{0};
	std::atomic<int> throwsLeft{0};
	std::mutex mu;
	steady::time_point lastDeadline{};

	std::pair<std::shared_ptr<scg::rpc::Connection>, scg::error::Error> connect() override
	{
		return connectWithDeadline(steady::time_point::max());
	}

	std::pair<std::shared_ptr<scg::rpc::Connection>, scg::error::Error> connectWithDeadline(steady::time_point deadline) override
	{
		calls++;
		{
			std::lock_guard<std::mutex> lock(mu);
			lastDeadline = deadline;
		}
		if (throwsLeft > 0) {
			throwsLeft--;
			throw std::runtime_error("boom");
		}
		return {nullptr, scg::error::Error("refused")};
	}

	void shutdown() override {}

	steady::time_point deadline()
	{
		std::lock_guard<std::mutex> lock(mu);
		return lastDeadline;
	}
};

std::shared_ptr<scg::rpc::Client> makeClient(const std::shared_ptr<FakeTransport>& transport, std::chrono::milliseconds connectTimeout)
{
	scg::rpc::ClientConfig cfg;
	cfg.transport = transport;
	cfg.connectTimeout = connectTimeout;
	return std::make_shared<scg::rpc::Client>(cfg);
}

double msSince(steady::time_point start)
{
	return std::chrono::duration<double, std::milli>(steady::now() - start).count();
}

}

// A transport that throws must not leave the client stuck "connecting": the
// next connect has to call the transport again right away (it used to wait out
// its whole deadline without retrying).
void test_connect_after_transport_throws()
{
	auto transport = std::make_shared<FakeTransport>();
	transport->throwsLeft = 1;
	auto client = makeClient(transport, std::chrono::milliseconds(2000));

	scg::error::Error first;
	bool threw = false;
	try {
		first = client->connect();
	} catch (...) {
		threw = true;
	}
	TEST_CHECK(!threw); // reported as an error, like every other connect failure
	TEST_CHECK(first != nullptr);
	TEST_CHECK(first.message().find("boom") != std::string::npos);
	TEST_MSG("first connect: %s", first.message().c_str());

	auto start = steady::now();
	auto second = client->connect();
	double ms = msSince(start);
	TEST_CHECK(transport->calls == 2);
	TEST_CHECK(second.message() == "refused");
	TEST_CHECK(ms < 500);
	TEST_MSG("second connect took %.0fms: %s", ms, second.message().c_str());
}

// connectTimeout = 0 means no timeout, and huge values saturate instead of
// overflowing (the deadline used to be "now", or wrap into the past).
void test_connect_timeout_zero_and_max()
{
	for (auto timeout : {std::chrono::milliseconds(0), std::chrono::milliseconds::max()}) {
		auto transport = std::make_shared<FakeTransport>();
		auto client = makeClient(transport, timeout);
		client->connect();
		TEST_CHECK(transport->deadline() == steady::time_point::max());
		TEST_MSG("connectTimeout=%lld", (long long)timeout.count());
	}

	// A normal timeout still yields a deadline about that far away.
	auto transport = std::make_shared<FakeTransport>();
	auto client = makeClient(transport, std::chrono::milliseconds(5000));
	auto before = steady::now();
	client->connect();
	auto deadline = transport->deadline();
	TEST_CHECK(deadline >= before + std::chrono::milliseconds(4900));
	TEST_CHECK(deadline <= steady::now() + std::chrono::milliseconds(5100));
}

// A call deadline bounds connecting; extreme deadlines must not overflow.
void test_connect_deadline_from_context()
{
	// No connectTimeout + a call deadline 300 ms away: ~300 ms.
	{
		auto transport = std::make_shared<FakeTransport>();
		auto client = makeClient(transport, std::chrono::milliseconds(0));
		scg::context::Context ctx;
		ctx.setDeadline(std::chrono::system_clock::now() + std::chrono::milliseconds(300));
		auto before = steady::now();
		client->openStream(ctx, 1, 2);
		auto deadline = transport->deadline();
		TEST_CHECK(deadline >= before + std::chrono::milliseconds(200));
		TEST_CHECK(deadline <= steady::now() + std::chrono::milliseconds(400));
	}
	// The earlier of the call deadline and connectTimeout wins.
	{
		auto transport = std::make_shared<FakeTransport>();
		auto client = makeClient(transport, std::chrono::milliseconds(100));
		scg::context::Context ctx;
		ctx.setDeadline(std::chrono::system_clock::now() + std::chrono::hours(1));
		client->openStream(ctx, 1, 2);
		TEST_CHECK(transport->deadline() <= steady::now() + std::chrono::milliseconds(200));
	}
	// system_clock::time_point::max() with no connectTimeout: a deadline
	// centuries away, not one that wrapped into the past.
	{
		auto transport = std::make_shared<FakeTransport>();
		auto client = makeClient(transport, std::chrono::milliseconds(0));
		scg::context::Context ctx;
		ctx.setDeadline(std::chrono::system_clock::time_point::max());
		client->openStream(ctx, 1, 2);
		TEST_CHECK(transport->deadline() > steady::now() + std::chrono::hours(24 * 365 * 100));
	}
	// A deadline already in the past (even time_point::min()): connect "now".
	for (auto past : {std::chrono::system_clock::now() - std::chrono::seconds(1), std::chrono::system_clock::time_point::min()}) {
		auto transport = std::make_shared<FakeTransport>();
		auto client = makeClient(transport, std::chrono::milliseconds(0));
		scg::context::Context ctx;
		ctx.setDeadline(past);
		auto before = steady::now();
		client->openStream(ctx, 1, 2);
		auto deadline = transport->deadline();
		TEST_CHECK(deadline >= before);
		TEST_CHECK(deadline <= steady::now());
	}
}

void test_deadline_helpers_saturate()
{
	using scg::rpc::deadlineAfter;
	using scg::rpc::addSaturating;
	TEST_CHECK(deadlineAfter(std::chrono::milliseconds(0)) == steady::time_point::max());
	TEST_CHECK(deadlineAfter(std::chrono::milliseconds(-5)) == steady::time_point::max());
	TEST_CHECK(deadlineAfter(std::chrono::milliseconds::max()) == steady::time_point::max());
	auto d = deadlineAfter(std::chrono::milliseconds(1000));
	TEST_CHECK(d > steady::now() && d <= steady::now() + std::chrono::milliseconds(1000));
	TEST_CHECK(addSaturating(steady::time_point::max(), std::chrono::seconds(5)) == steady::time_point::max());
	auto now = steady::now();
	TEST_CHECK(addSaturating(now, std::chrono::seconds(5)) == now + std::chrono::seconds(5));
}

#define TEST(x) {#x, x}

TEST_LIST = {
	TEST(test_connect_after_transport_throws),
	TEST(test_connect_timeout_zero_and_max),
	TEST(test_connect_deadline_from_context),
	TEST(test_deadline_helpers_saturate),
	{ NULL, NULL }
};
