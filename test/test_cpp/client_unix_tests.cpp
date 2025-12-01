#include <cstdio>
#include <thread>
#include <chrono>

#include <acutest.h>

#include "scg/serialize.h"
#include "scg/client.h"
#include "scg/unix/transport_client.h"
#include "pingpong/pingpong.h"

void test_pingpong_client_unix() {

	scg::unix_socket::ClientTransportConfig transportConfig;
	transportConfig.socketPath = "/tmp/scg_test_unix.sock";

	scg::rpc::ClientConfig config;
	config.transport = std::make_shared<scg::unix_socket::ClientTransportUnix>(transportConfig);

	auto client = std::make_shared<scg::rpc::Client>(config);

	// Retry connection a few times to allow server to start
	scg::error::Error connectErr;
	for (int i = 0; i < 10; i++) {
		connectErr = client->connect();
		if (!connectErr) break;
		std::this_thread::sleep_for(std::chrono::milliseconds(100));
	}

	if (connectErr) {
		printf("Connection failed: %s\n", connectErr.message.c_str());
		TEST_CHECK(false);
		return;
	}

	pingpong::PingPongClient pingPongClient(client);

	uint32_t COUNT = 10;

	for (uint32_t i=0; i<COUNT; i++) {

		scg::context::Context context;
		context.put("key", "value");

		pingpong::NestedPayload nested1;
		nested1.valString = "nested" + std::to_string(i);
		nested1.valDouble = 3.14 + i;

		pingpong::NestedPayload nested2;
		nested2.valString = "nested again" + std::to_string(i);
		nested2.valDouble = 123.34563456 + i;

		pingpong::NestedEmpty nested;
		nested.empty = pingpong::Empty();

		pingpong::TestPayload payload;
		payload.valUint8 = i + 1;
		payload.valUint16 = 256 + i + 2;
		payload.valUint32 = 65535 + i + 3;
		payload.valUint64 = 4294967295 + i + 4;
		payload.valInt8 = -(i + 5);
		payload.valInt16 = -128 - (i + 6);
		payload.valInt32 = -32768 - (i + 7);
		payload.valInt64 = -2147483648 - (i + 8);
		payload.valFloat = 3.14 + i + 9;
		payload.valDouble = -3.14159 + i + 10;
		payload.valString = "hello world " + std::to_string(i + 11);
		payload.valTimestamp = scg::type::timestamp();
		payload.valBool = i % 2 == 0;
		payload.valEnum = pingpong::EnumType::ENUM_TYPE_1;
		payload.valUUID = scg::type::uuid::random();
		payload.valListPayload = {nested1, nested2};
		payload.valMapKeyEnum = {
			{pingpong::KeyType("key_" + std::to_string(i+1)), pingpong::EnumType::ENUM_TYPE_1},
			{pingpong::KeyType("key_" +std::to_string(i+2)), pingpong::EnumType::ENUM_TYPE_2}
		};
		payload.valEmpty = pingpong::Empty();
		payload.valNestedEmpty = nested;
		payload.valByteArray = {0, 1, 2, 3, 4, 5, 6, 7, 8, 9};

		pingpong::PingRequest req;
		req.ping.count = i;
		req.ping.payload = payload;

		auto [res, err] = pingPongClient.ping(context, req);
		if (err != nullptr) {
			printf("ERROR: %s\n", err.message.c_str());
			TEST_CHECK(err == nullptr);
			return;
		}
		TEST_CHECK(err == nullptr);
		TEST_CHECK(res.pong.count == int32_t(i+1));
		TEST_CHECK(res.pong.payload.valUint8 == payload.valUint8);
		TEST_CHECK(res.pong.payload.valUint16 == payload.valUint16);
		TEST_CHECK(res.pong.payload.valUint32 == payload.valUint32);
		TEST_CHECK(res.pong.payload.valUint64 == payload.valUint64);
		TEST_CHECK(res.pong.payload.valInt8 == payload.valInt8);
		TEST_CHECK(res.pong.payload.valInt16 == payload.valInt16);
		TEST_CHECK(res.pong.payload.valInt32 == payload.valInt32);
		TEST_CHECK(res.pong.payload.valInt64 == payload.valInt64);
		TEST_CHECK(res.pong.payload.valFloat == payload.valFloat);
		TEST_CHECK(res.pong.payload.valDouble == payload.valDouble);
		TEST_CHECK(res.pong.payload.valString == payload.valString);
		TEST_CHECK(res.pong.payload.valBool == payload.valBool);
		TEST_CHECK(res.pong.payload.valEnum == payload.valEnum);
		TEST_CHECK(res.pong.payload.valUUID == payload.valUUID);
		TEST_CHECK(res.pong.payload.valListPayload.size() == 2);
		TEST_CHECK(res.pong.payload.valListPayload[0].valString == nested1.valString);
		TEST_CHECK(res.pong.payload.valListPayload[0].valDouble == nested1.valDouble);
		TEST_CHECK(res.pong.payload.valListPayload[1].valString == nested2.valString);
		TEST_CHECK(res.pong.payload.valListPayload[1].valDouble == nested2.valDouble);
		TEST_CHECK(res.pong.payload.valMapKeyEnum.size() == 2);
		TEST_CHECK(res.pong.payload.valMapKeyEnum[pingpong::KeyType("key_" + std::to_string(i+1))] == pingpong::EnumType::ENUM_TYPE_1);
		TEST_CHECK(res.pong.payload.valMapKeyEnum[pingpong::KeyType("key_" + std::to_string(i+2))] == pingpong::EnumType::ENUM_TYPE_2);

		std::this_thread::sleep_for(std::chrono::milliseconds(50));
	}
}

// helper method to reduce redundant test typing
#define TEST(x) {#x, x}

TEST_LIST = {
	TEST(test_pingpong_client_unix),

	{ NULL, NULL }
};
