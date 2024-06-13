#include <cstdio>
#include <acutest.h>

#include "scg/serialize.h"
#include "scg/timestamp.h"
#include "scg/uuid.h"

#include "pingpong/pingpong.h"

void test_serialize_uint8()
{
	uint8_t input = 234U;

	scg::serialize::FixedSizeWriter writer(scg::serialize::byte_size(input));
	scg::serialize::serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	uint8_t output = 0;
	auto err = scg::serialize::deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(input == output);
}

void test_serialize_int8()
{
	int8_t input = -123;

	scg::serialize::FixedSizeWriter writer(scg::serialize::byte_size(input));
	scg::serialize::serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	int8_t output = 0;
	auto err = scg::serialize::deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(input == output);
}

void test_serialize_uint16()
{
	uint16_t input = 54767U;

	scg::serialize::FixedSizeWriter writer(scg::serialize::byte_size(input));
	scg::serialize::serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	uint16_t output = 0;
	auto err = scg::serialize::deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(input == output);
}

void test_serialize_int16()
{
	int16_t input = -31412;

	scg::serialize::FixedSizeWriter writer(scg::serialize::byte_size(input));
	scg::serialize::serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	int16_t output = 0;
	auto err = scg::serialize::deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(input == output);
}

void test_serialize_uint32()
{
	uint32_t input = 3454234767;

	scg::serialize::FixedSizeWriter writer(scg::serialize::byte_size(input));
	scg::serialize::serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	uint32_t output = 0;
	auto err = scg::serialize::deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(input == output);
}

void test_serialize_int32()
{
	int32_t input = -1454234767;

	scg::serialize::FixedSizeWriter writer(scg::serialize::byte_size(input));
	scg::serialize::serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	int32_t output = 0;
	auto err = scg::serialize::deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(input == output);
}

void test_serialize_uint64()
{
	uint64_t input = 3454363453454234767UL;

	scg::serialize::FixedSizeWriter writer(scg::serialize::byte_size(input));
	scg::serialize::serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	uint64_t output = 0;
	auto err = scg::serialize::deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(input == output);
}

void test_serialize_int64()
{
	int64_t input = -1454363453454234767L;

	scg::serialize::FixedSizeWriter writer(scg::serialize::byte_size(input));
	scg::serialize::serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	int64_t output = 0;
	auto err = scg::serialize::deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(input == output);
}

void test_serialize_float32()
{
	float32_t input = -145234.5634347f;

	scg::serialize::FixedSizeWriter writer(scg::serialize::byte_size(input));
	scg::serialize::serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	float32_t output = 0;
	auto err = scg::serialize::deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(input == output);
}

void test_serialize_float64()
{
	float64_t input = -245234534.56343437;

	scg::serialize::FixedSizeWriter writer(scg::serialize::byte_size(input));
	scg::serialize::serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	float64_t output = 0;
	auto err = scg::serialize::deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(input == output);
}

void test_serialize_string()
{
	std::string input = "Hello, World! This is my test string 12312341234! \\@#$%@&^&%^\n newline \t _yay 世界";

	scg::serialize::FixedSizeWriter writer(scg::serialize::byte_size(input));
	scg::serialize::serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	std::string output;
	auto err = scg::serialize::deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(input == output);
}

void test_serialize_timestamp()
{
	scg::timestamp input = std::chrono::system_clock::now();

	scg::serialize::FixedSizeWriter writer(scg::serialize::byte_size(input));
	scg::serialize::serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	scg::timestamp output;
	auto err = scg::serialize::deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(input == output);
}

void test_serialize_uuid()
{
	scg::uuid input = scg::uuid::random();

	scg::serialize::FixedSizeWriter writer(scg::serialize::byte_size(input));
	scg::serialize::serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	scg::uuid output;
	auto err = scg::serialize::deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(input == output);
}

void test_serialize_vector()
{
	std::vector<float64_t> input = { 1.0, -2.0, 3.0, -4.0, 5.0 };

	scg::serialize::FixedSizeWriter writer(scg::serialize::byte_size(input));
	scg::serialize::serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	std::vector<float64_t> output;
	auto err = scg::serialize::deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(input == output);
}

void test_serialize_map()
{
	std::map<std::string, float64_t> input = {
		{"one", 1.0},
		{"two", 2.0},
		{"three", 3.0},
		{"four", 4.0},
		{"five", 5.0}
	};

	scg::serialize::FixedSizeWriter writer(scg::serialize::byte_size(input));
	scg::serialize::serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	std::map<std::string, float64_t> output;
	auto err = scg::serialize::deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(input == output);
}

void test_serialize_pingpong()
{
	pingpong::NestedPayload nested1;
	nested1.valString = "Hello, 世界";
	nested1.valDouble = 3.14;

	pingpong::NestedPayload nested2;
	nested2.valString = "nested again";
	nested2.valDouble = 123.34563456;

	pingpong::TestPayload input;
	input.valUint8 = 1;
	input.valUint16 = 256 + 2;
	input.valUint32 = 65535 + 3;
	input.valUint64 = 4294967295 + 4;
	input.valInt8 = -(5);
	input.valInt16 = -128 - (6);
	input.valInt32 = -32768 - (7);
	input.valInt64 = -2147483648 - (8);
	input.valFloat = 3.14 + 9;
	input.valDouble = -3.14159 + 10;
	input.valString = "hello world " + std::to_string(11);
	input.valTimestamp = std::chrono::system_clock::now();
	input.valBool = true;
	input.valEnum = pingpong::EnumType::ENUM_TYPE_1;
	input.valUuid = scg::uuid::random();
	input.valListPayload = {nested1, nested2};
	input.valMapKeyEnum = {
		{pingpong::KeyType("key_1"), pingpong::EnumType::ENUM_TYPE_1},
		{pingpong::KeyType("key_2"), pingpong::EnumType::ENUM_TYPE_2}
	};

	scg::serialize::FixedSizeWriter writer(scg::serialize::byte_size(input));
	scg::serialize::serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	pingpong::TestPayload output;
	auto err = scg::serialize::deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(output.valUint8 == input.valUint8);
	TEST_CHECK(output.valUint16 == input.valUint16);
	TEST_CHECK(output.valUint32 == input.valUint32);
	TEST_CHECK(output.valUint64 == input.valUint64);
	TEST_CHECK(output.valInt8 == input.valInt8);
	TEST_CHECK(output.valInt16 == input.valInt16);
	TEST_CHECK(output.valInt32 == input.valInt32);
	TEST_CHECK(output.valInt64 == input.valInt64);
	TEST_CHECK(output.valFloat == input.valFloat);
	TEST_CHECK(output.valDouble == input.valDouble);
	TEST_CHECK(output.valString == input.valString);
	TEST_CHECK(output.valBool == input.valBool);
	TEST_CHECK(output.valEnum == input.valEnum);
	TEST_CHECK(output.valUuid == input.valUuid);
	TEST_CHECK(output.valListPayload.size() == 2);
	TEST_CHECK(output.valListPayload[0].valString == nested1.valString);
	TEST_CHECK(output.valListPayload[0].valDouble == nested1.valDouble);
	TEST_CHECK(output.valListPayload[1].valString == nested2.valString);
	TEST_CHECK(output.valListPayload[1].valDouble == nested2.valDouble);
	TEST_CHECK(output.valMapKeyEnum.size() == 2);
	TEST_CHECK(output.valMapKeyEnum[pingpong::KeyType("key_1")] == pingpong::EnumType::ENUM_TYPE_1);
	TEST_CHECK(output.valMapKeyEnum[pingpong::KeyType("key_2")] == pingpong::EnumType::ENUM_TYPE_2);
}


// helper method to reduce redundant test typing
#define TEST(x) {#x, x}

TEST_LIST = {
	TEST(test_serialize_uint8),
	TEST(test_serialize_int8),
	TEST(test_serialize_uint16),
	TEST(test_serialize_int16),
	TEST(test_serialize_uint32),
	TEST(test_serialize_int32),
	TEST(test_serialize_uint64),
	TEST(test_serialize_int64),
	TEST(test_serialize_float32),
	TEST(test_serialize_float64),
	TEST(test_serialize_string),
	TEST(test_serialize_uuid),
	TEST(test_serialize_timestamp),
	TEST(test_serialize_vector),
	TEST(test_serialize_map),
	TEST(test_serialize_pingpong),

	{ NULL, NULL }
};
