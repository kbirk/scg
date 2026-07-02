#include <acutest.h>

#include <memory>
#include <string>
#include <utility>

#include "basic/service.h"
#include "pingpong/pingpong.h"

using scg::error::Error;

// The concrete client is-a Api: an upcast to the abstract interface must compile,
// which is what lets orchestration code depend on the seam rather than the client.
void test_client_is_a_api()
{
	basic::TesterAClient* client = nullptr;
	basic::TesterAApi* api = client; // implicit upcast
	TEST_CHECK(api == nullptr);
}

// The fake dispatches to its per-method hook when set, called through the interface.
void test_fake_invokes_hook()
{
	basic::TesterAFake fake;
	bool called = false;
	fake.onTest = [&](scg::context::Context& ctx, const basic::TestRequestA& req)
		-> std::pair<basic::TestResponseA, Error> {
		called = true;
		basic::TestResponseA resp;
		resp.a = req.a + "-pong";
		return {resp, nullptr};
	};

	basic::TesterAApi& api = fake;
	scg::context::Context ctx;
	basic::TestRequestA req;
	req.a = "ping";

	auto [resp, err] = api.test(ctx, req);
	TEST_CHECK(!err);
	TEST_CHECK(called);
	TEST_CHECK(resp.a == "ping-pong");
}

// With no hook set, the fake returns a default-constructed response and no error.
void test_fake_default_fallthrough()
{
	basic::TesterAFake fake;
	basic::TesterAApi& api = fake;
	scg::context::Context ctx;
	basic::TestRequestA req;
	req.a = "ping";

	auto [resp, err] = api.test(ctx, req);
	TEST_CHECK(!err);
	TEST_CHECK(resp.a.empty());
}

// The hook can surface an error, which propagates through the interface unchanged.
void test_fake_hook_error()
{
	basic::TesterAFake fake;
	fake.onTest = [](scg::context::Context&, const basic::TestRequestA&)
		-> std::pair<basic::TestResponseA, Error> {
		return {basic::TestResponseA{}, Error("boom")};
	};

	basic::TesterAApi& api = fake;
	scg::context::Context ctx;
	basic::TestRequestA req;

	auto [resp, err] = api.test(ctx, req);
	TEST_CHECK(err);
	TEST_CHECK(err.message() == "boom");
}

// A unary service (PingPong) is fakeable through its interface seam.
void test_pingpong_fake_through_interface()
{
	pingpong::PingPongFake fake;
	fake.onPing = [](scg::context::Context&, const pingpong::PingRequest& r)
		-> std::pair<pingpong::PongResponse, Error> {
		pingpong::PongResponse resp;
		resp.pong.count = r.ping.count + 1;
		return {resp, nullptr};
	};

	pingpong::PingPongApi& api = fake;
	scg::context::Context ctx;
	pingpong::PingRequest req;
	req.ping.count = 41;

	auto [resp, err] = api.ping(ctx, req);
	TEST_CHECK(!err);
	TEST_CHECK(resp.pong.count == 42);
}

// A streaming-only service (Chat) still gets an Api/Client/Fake trio that composes,
// even though its interface has no unary methods.
void test_streaming_only_service_composes()
{
	pingpong::ChatFake fake;
	pingpong::ChatApi* api = &fake;
	TEST_CHECK(api != nullptr);
}

TEST_LIST = {
	{"client_is_a_api", test_client_is_a_api},
	{"fake_invokes_hook", test_fake_invokes_hook},
	{"fake_default_fallthrough", test_fake_default_fallthrough},
	{"fake_hook_error", test_fake_hook_error},
	{"pingpong_fake_through_interface", test_pingpong_fake_through_interface},
	{"streaming_only_service_composes", test_streaming_only_service_composes},
	{NULL, NULL}};
