#include <cstdio>
#include <acutest.h>

#include <memory>
#include <string>
#include <vector>

#include "scg/middleware.h"
#include "scg/context.h"
#include "scg/message.h"

namespace {

struct Msg : scg::type::Message {
	std::vector<uint8_t> toJSON() const override { return {}; }
	void fromJSON(const std::vector<uint8_t>&) override {}
	std::vector<uint8_t> toBytes() const override { return {}; }
	scg::error::Error fromBytes(const std::vector<uint8_t>&) override { return nullptr; }
	scg::error::Error fromBytes(const uint8_t*, uint32_t) override { return nullptr; }
};

using Result = std::pair<std::shared_ptr<scg::type::Message>, scg::error::Error>;

}

// With no middleware the final handler is called directly.
void test_chain_without_middleware()
{
	scg::context::Context ctx;
	Msg req;
	int calls = 0;
	auto result = scg::middleware::applyHandlerChain(ctx, req, {}, [&](scg::context::Context&, const scg::type::Message&) -> Result {
		calls++;
		return {std::make_shared<Msg>(), nullptr};
	});
	TEST_CHECK(calls == 1);
	TEST_CHECK(result.first != nullptr);
	TEST_CHECK(!result.second);
}

// Middleware runs in registration order, wraps the final handler, and sees the
// context mutations of the middleware before it.
void test_chain_order_and_context()
{
	std::vector<std::string> trace;
	std::vector<scg::middleware::Middleware> mw;
	for (int i = 0; i < 3; i++) {
		mw.push_back([&trace, i](scg::context::Context& ctx, const scg::type::Message& req, scg::middleware::Handler next) -> Result {
			trace.push_back("before" + std::to_string(i));
			ctx.put("depth", std::to_string(i));
			auto r = next(ctx, req);
			trace.push_back("after" + std::to_string(i));
			return r;
		});
	}
	scg::context::Context ctx;
	Msg req;
	std::string seenDepth;
	auto result = scg::middleware::applyHandlerChain(ctx, req, mw, [&](scg::context::Context& c, const scg::type::Message&) -> Result {
		trace.push_back("final");
		c.get(seenDepth, "depth");
		return {nullptr, scg::error::Error("handler error")};
	});
	std::vector<std::string> expected = {"before0", "before1", "before2", "final", "after2", "after1", "after0"};
	TEST_CHECK(trace == expected);
	TEST_CHECK(seenDepth == "2");
	TEST_CHECK(result.second.message() == "handler error");

	// The prebuilt-chain helper behaves the same.
	trace.clear();
	auto fn = scg::middleware::buildHandlerFunction(mw, [&](scg::context::Context&, const scg::type::Message&) -> Result {
		trace.push_back("final");
		return {nullptr, nullptr};
	});
	fn(ctx, req);
	TEST_CHECK(trace == expected);
}

// A middleware that does not call next short-circuits the chain.
void test_chain_short_circuit()
{
	std::vector<scg::middleware::Middleware> mw = {
		[](scg::context::Context&, const scg::type::Message&, scg::middleware::Handler) -> Result {
			return {nullptr, scg::error::Error("denied")};
		},
	};
	scg::context::Context ctx;
	Msg req;
	bool called = false;
	auto result = scg::middleware::applyHandlerChain(ctx, req, mw, [&](scg::context::Context&, const scg::type::Message&) -> Result {
		called = true;
		return {nullptr, nullptr};
	});
	TEST_CHECK(!called);
	TEST_CHECK(result.second.message() == "denied");
}

// A middleware may keep `next` and call it more than once.
void test_chain_next_is_reusable()
{
	int finals = 0;
	std::vector<scg::middleware::Middleware> mw = {
		[](scg::context::Context& ctx, const scg::type::Message& req, scg::middleware::Handler next) -> Result {
			auto copy = next;
			next(ctx, req);
			return copy(ctx, req);
		},
	};
	scg::context::Context ctx;
	Msg req;
	scg::middleware::applyHandlerChain(ctx, req, mw, [&](scg::context::Context&, const scg::type::Message&) -> Result {
		finals++;
		return {nullptr, nullptr};
	});
	TEST_CHECK(finals == 2);
}

#define TEST(x) {#x, x}

TEST_LIST = {
	TEST(test_chain_without_middleware),
	TEST(test_chain_order_and_context),
	TEST(test_chain_short_circuit),
	TEST(test_chain_next_is_reusable),
	{ NULL, NULL }
};
