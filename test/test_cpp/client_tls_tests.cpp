#include <cstdio>
#include <thread>
#include <chrono>

#include <acutest.h>

#include "scg/serialize.h"
#include "scg/client_tls.h"
#include "pingpong/pingpong.h"

void test_pingpong_client_tls() {

	scg::log::LoggingConfig logging;
	logging.level = scg::log::LogLevel::WARN;
	logging.debugLogger = [](std::string msg) {
		printf("DEBUG: %s\n", msg.c_str());
	};
	logging.infoLogger = [](std::string msg) {
		printf("INFO: %s\n", msg.c_str());
	};
	logging.warnLogger = [](std::string msg) {
		printf("WARN: %s\n", msg.c_str());
	};
	logging.errorLogger = [](std::string msg) {
		printf("ERROR: %s\n", msg.c_str());
	};

	scg::rpc::ClientConfig config;
	config.uri = "localhost:8080";
	config.logging = logging;

	auto client = std::make_shared<scg::rpc::ClientTLS>(config);

	pingpong::PingPongClient pingPongClient(client);

	uint32_t COUNT = 10;

	for (uint32_t i=0; i<COUNT; i++) {

		scg::context::Context context;
		context.put("key", "value");

		pingpong::PingRequest req;
		req.ping.count = i;

		auto [res, err] = pingPongClient.ping(context, req);
		TEST_CHECK(err == nullptr);
		TEST_CHECK(res.pong.count == int32_t(i+1));

		std::this_thread::sleep_for(std::chrono::milliseconds(50));
	}
}

// helper method to reduce redundant test typing
#define TEST(x) {#x, x}

TEST_LIST = {
	// pingpong (requires running external server)
	TEST(test_pingpong_client_tls),
	{ NULL, NULL }
};
