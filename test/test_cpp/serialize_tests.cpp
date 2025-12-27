#include <cstdio>
#include <acutest.h>
#include <fstream>

#include "scg/serialize.h"
#include "scg/timestamp.h"
#include "scg/uuid.h"
#include "scg/macro.h"

#include "pingpong/pingpong.h"

using scg::serialize::float32_t;
using scg::serialize::float64_t;

// adl trickery
using scg::serialize::bit_size;
using scg::serialize::serialize;
using scg::serialize::deserialize;

constexpr int32_t NUM_STEPS = 1024;

void test_serialize_context()
{
	scg::context::Context input;
	input.put("key1", "value1");

	scg::serialize::FixedSizeWriter writer(scg::serialize::bits_to_bytes(bit_size(input)));
	serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	scg::context::Context output;
	auto err = deserialize(output, reader);
	TEST_CHECK(!err);

	std::string str1;
	TEST_CHECK(input.get(str1, "key1") == nullptr);

	std::string str2;
	TEST_CHECK(output.get(str2, "key1") == nullptr);

	TEST_CHECK(str1 == str2);
}

void test_serialize_uint8()
{
	uint8_t NUM_STEPS = UINT8_MAX;
	for (uint32_t i=0; i<NUM_STEPS; i++) {
		uint8_t input = i;

		scg::serialize::FixedSizeWriter writer(scg::serialize::bits_to_bytes(bit_size(input)));
		serialize(writer, input);

		scg::serialize::Reader reader(writer.bytes());
		uint8_t output = 0;
		auto err = deserialize(output, reader);
		TEST_CHECK(!err);

		TEST_CHECK(input == output);
	}
}

void test_serialize_int8()
{
	uint8_t MIN = INT8_MIN;
	uint8_t MAX = INT8_MAX;
	for (int32_t i=MIN; i<MAX; i++) {
		int8_t input = i;

		scg::serialize::FixedSizeWriter writer(scg::serialize::bits_to_bytes(bit_size(input)));
		serialize(writer, input);

		scg::serialize::Reader reader(writer.bytes());
		int8_t output = 0;
		auto err = deserialize(output, reader);
		TEST_CHECK(!err);

		TEST_CHECK(input == output);
	}
}

void test_serialize_uint16()
{
	uint16_t NUM_STEPS = UINT16_MAX;
	for (uint32_t i=0; i<NUM_STEPS; i++) {
		uint16_t input = i;

		scg::serialize::FixedSizeWriter writer(scg::serialize::bits_to_bytes(bit_size(input)));
		serialize(writer, input);

		scg::serialize::Reader reader(writer.bytes());
		uint16_t output = 0;
		auto err = deserialize(output, reader);
		TEST_CHECK(!err);

		TEST_CHECK(input == output);
	}
}

void test_serialize_int16()
{
	uint16_t MIN = INT16_MIN;
	uint16_t MAX = INT16_MAX;
	for (int32_t i=MIN; i<MAX; i++) {
		int16_t input = i;

		scg::serialize::FixedSizeWriter writer(scg::serialize::bits_to_bytes(bit_size(input)));
		serialize(writer, input);

		scg::serialize::Reader reader(writer.bytes());
		int16_t output = 0;
		auto err = deserialize(output, reader);
		TEST_CHECK(!err);

		TEST_CHECK(input == output);
	}
}

void test_serialize_uint32()
{
	uint32_t STEP = UINT32_MAX / NUM_STEPS;
	for (uint32_t i=0; i<NUM_STEPS; i++) {
		uint32_t input = i * STEP;

		scg::serialize::FixedSizeWriter writer(scg::serialize::bits_to_bytes(bit_size(input)));
		serialize(writer, input);

		scg::serialize::Reader reader(writer.bytes());
		uint32_t output = 0;
		auto err = deserialize(output, reader);
		TEST_CHECK(!err);

		TEST_CHECK(input == output);
	}
}

void test_serialize_int32()
{
	int32_t STEP = UINT32_MAX / NUM_STEPS;
	for (int32_t i=-NUM_STEPS/2; i<NUM_STEPS/2; i++) {
		int32_t input = i * STEP;

		scg::serialize::FixedSizeWriter writer(scg::serialize::bits_to_bytes(bit_size(input)));
		serialize(writer, input);

		scg::serialize::Reader reader(writer.bytes());
		int32_t output = 0;
		auto err = deserialize(output, reader);
		TEST_CHECK(!err);

		TEST_CHECK(input == output);
	}
}

void test_serialize_uint64()
{
	uint64_t STEP = UINT32_MAX / NUM_STEPS;
	for (uint32_t i=0; i<NUM_STEPS; i++) {
		uint64_t input = i * STEP;

		scg::serialize::FixedSizeWriter writer(scg::serialize::bits_to_bytes(bit_size(input)));
		serialize(writer, input);

		scg::serialize::Reader reader(writer.bytes());
		uint64_t output = 0;
		auto err = deserialize(output, reader);
		TEST_CHECK(!err);

		TEST_CHECK(input == output);
	}
}

void test_serialize_int64()
{
	int64_t STEP = UINT64_MAX / NUM_STEPS;
	for (int32_t i=-NUM_STEPS/2; i<NUM_STEPS/2; i++) {
		int64_t input = i * STEP;

		scg::serialize::FixedSizeWriter writer(scg::serialize::bits_to_bytes(bit_size(input)));
		serialize(writer, input);

		scg::serialize::Reader reader(writer.bytes());
		int64_t output = 0;
		auto err = deserialize(output, reader);
		TEST_CHECK(!err);

		TEST_CHECK(input == output);
	}
}

void test_serialize_float32()
{
	float32_t MAX = 123456.12345f;
	float32_t STEP = MAX / NUM_STEPS;
	for (int32_t i=-NUM_STEPS/2; i<NUM_STEPS/2; i++) {
		float32_t input = i * STEP;

		scg::serialize::FixedSizeWriter writer(scg::serialize::bits_to_bytes(bit_size(input)));
		serialize(writer, input);

		scg::serialize::Reader reader(writer.bytes());
		float32_t output = 0;
		auto err = deserialize(output, reader);
		TEST_CHECK(!err);

		TEST_CHECK(input == output);
	}
}

void test_serialize_float64()
{
	float64_t MAX = 123456789.123456789f;
	float64_t STEP = MAX / NUM_STEPS;
	for (int32_t i=-NUM_STEPS/2; i<NUM_STEPS/2; i++) {
		float64_t input = i * STEP;

		scg::serialize::FixedSizeWriter writer(scg::serialize::bits_to_bytes(bit_size(input)));
		serialize(writer, input);

		scg::serialize::Reader reader(writer.bytes());
		float64_t output = 0;
		auto err = deserialize(output, reader);
		TEST_CHECK(!err);

		TEST_CHECK(input == output);
	}
}

void test_serialize_string()
{
	std::string input = "Hello, World! This is my test string 12312341234! \\@#$%@&^&%^\n newline \t _yay 世界";

	scg::serialize::FixedSizeWriter writer(scg::serialize::bits_to_bytes(bit_size(input)));
	serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	std::string output;
	auto err = deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(input == output);
}

void test_serialize_timestamp()
{
	scg::type::timestamp input;

	scg::serialize::FixedSizeWriter writer(scg::serialize::bits_to_bytes(bit_size(input)));
	serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	scg::type::timestamp output;
	auto err = deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(input == output);
}

void test_serialize_uuid()
{
	scg::type::uuid input = scg::type::uuid::random();

	scg::serialize::FixedSizeWriter writer(scg::serialize::bits_to_bytes(bit_size(input)));
	serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	scg::type::uuid output;
	auto err = deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(input == output);
}

void test_serialize_vector()
{
	std::vector<float64_t> input = { 1.0, -2.0, 3.0, -4.0, 5.0 };

	scg::serialize::FixedSizeWriter writer(scg::serialize::bits_to_bytes(bit_size(input)));
	serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	std::vector<float64_t> output;
	auto err = deserialize(output, reader);
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

	scg::serialize::FixedSizeWriter writer(scg::serialize::bits_to_bytes(bit_size(input)));
	serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	std::map<std::string, float64_t> output;
	auto err = deserialize(output, reader);
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

	pingpong::NestedEmpty nested;
	nested.empty = pingpong::Empty();

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
	input.valTimestamp = scg::type::timestamp();
	input.valBool = true;
	input.valEnum = pingpong::EnumType::ENUM_TYPE_1;
	input.valUUID = scg::type::uuid::random();
	input.valListPayload = {nested1, nested2};
	input.valMapKeyEnum = {
		{pingpong::KeyType("key_1"), pingpong::EnumType::ENUM_TYPE_1},
		{pingpong::KeyType("key_2"), pingpong::EnumType::ENUM_TYPE_2}
	};
	input.valEmpty = pingpong::Empty();
	input.valNestedEmpty = nested;
	input.valByteArray = {0, 1, 2, 3, 4, 5, 6, 7, 8, 9};

	scg::serialize::FixedSizeWriter writer(scg::serialize::bits_to_bytes(bit_size(input)));
	serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	pingpong::TestPayload output;
	auto err = deserialize(output, reader);
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
	TEST_CHECK(output.valUUID == input.valUUID);
	TEST_CHECK(output.valListPayload.size() == 2);
	TEST_CHECK(output.valListPayload[0].valString == nested1.valString);
	TEST_CHECK(output.valListPayload[0].valDouble == nested1.valDouble);
	TEST_CHECK(output.valListPayload[1].valString == nested2.valString);
	TEST_CHECK(output.valListPayload[1].valDouble == nested2.valDouble);
	TEST_CHECK(output.valMapKeyEnum.size() == 2);
	TEST_CHECK(output.valMapKeyEnum[pingpong::KeyType("key_1")] == pingpong::EnumType::ENUM_TYPE_1);
	TEST_CHECK(output.valMapKeyEnum[pingpong::KeyType("key_2")] == pingpong::EnumType::ENUM_TYPE_2);
	TEST_CHECK(output.valByteArray.size() == input.valByteArray.size());
}

void test_stream_writer_reader()
{
	scg::type::uuid id = scg::type::uuid::random();

	std::string filename = "/tmp/test_" + id.toString();

	std::ofstream ofs(filename, std::ios::out | std::ios::binary);
	scg::serialize::StreamWriter writer(ofs);

	pingpong::NestedPayload nested1;
	nested1.valString = "Hello, 世界";
	nested1.valDouble = 3.14;

	pingpong::NestedPayload nested2;
	nested2.valString = "nested again";
	nested2.valDouble = 123.34563456;

	pingpong::NestedEmpty nested;
	nested.empty = pingpong::Empty();

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
	input.valTimestamp = scg::type::timestamp();
	input.valBool = true;
	input.valEnum = pingpong::EnumType::ENUM_TYPE_1;
	input.valUUID = scg::type::uuid::random();
	input.valListPayload = {nested1, nested2};
	input.valMapKeyEnum = {
		{pingpong::KeyType("key_1"), pingpong::EnumType::ENUM_TYPE_1},
		{pingpong::KeyType("key_2"), pingpong::EnumType::ENUM_TYPE_2}
	};
	input.valEmpty = pingpong::Empty();
	input.valNestedEmpty = nested;
	input.valByteArray = {0, 1, 2, 3, 4, 5, 6, 7, 8, 9};

	serialize(writer, input);

	ofs.close();

	std::ifstream ifs(filename, std::ios::in | std::ios::binary);
	TEST_CHECK(ifs.is_open() && ifs.good());

	scg::serialize::StreamReader reader(ifs);

	pingpong::TestPayload output;
	auto err = deserialize(output, reader);
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
	TEST_CHECK(output.valUUID == input.valUUID);
	TEST_CHECK(output.valListPayload.size() == 2);
	TEST_CHECK(output.valListPayload[0].valString == nested1.valString);
	TEST_CHECK(output.valListPayload[0].valDouble == nested1.valDouble);
	TEST_CHECK(output.valListPayload[1].valString == nested2.valString);
	TEST_CHECK(output.valListPayload[1].valDouble == nested2.valDouble);
	TEST_CHECK(output.valMapKeyEnum.size() == 2);
	TEST_CHECK(output.valMapKeyEnum[pingpong::KeyType("key_1")] == pingpong::EnumType::ENUM_TYPE_1);
	TEST_CHECK(output.valMapKeyEnum[pingpong::KeyType("key_2")] == pingpong::EnumType::ENUM_TYPE_2);
	TEST_CHECK(output.valByteArray.size() == input.valByteArray.size());
}

struct TestStructA {
	uint32_t a = 0;
	float64_t b = 1;
};
SCG_SERIALIZABLE_PUBLIC(TestStructA, a, b);

struct TestStructEmpty {};
SCG_SERIALIZABLE_PUBLIC(TestStructEmpty);

struct TestStructDerivedA : TestStructA {
	std::string c = "2";
};
SCG_SERIALIZABLE_DERIVED_PUBLIC(TestStructDerivedA, TestStructA, c);

class TestClassPrivate {
public:
	TestClassPrivate(uint32_t a, float64_t b) : a_(a), b_(b) {}

	uint32_t a() const { return a_; }
	float64_t b() const { return b_; }

SCG_SERIALIZABLE_PRIVATE(TestClassPrivate, a_, b_);

private:
	uint32_t a_ = 0;
	float64_t b_ = 1;
};

class TestDerivedPrivate: public TestClassPrivate {
public:
	TestDerivedPrivate(uint32_t a, float64_t b, std::string c) : TestClassPrivate(a, b), c_(c) {}

	std::string c() const { return c_; }

SCG_SERIALIZABLE_DERIVED_PRIVATE(TestDerivedPrivate, TestClassPrivate, c_);

private:
	std::string c_;
};

void test_serialize_macros()
{
	TestStructA inputA;
	inputA.a = 123;
	inputA.b = 3.14;

	TestStructEmpty inputEmpty;

	scg::serialize::Writer writer;
	serialize(writer, inputA);
	serialize(writer, inputEmpty);

	scg::serialize::Reader reader(writer.bytes());
	TestStructA outputA;
	auto err = deserialize(outputA, reader);
	TEST_CHECK(!err);

	TEST_CHECK(inputA.a == outputA.a);
	TEST_CHECK(inputA.b == outputA.b);

	TestStructEmpty outputEmpty;
	err = deserialize(outputEmpty, reader);
	TEST_CHECK(!err);

	TestStructDerivedA inputDerivedA;
	inputDerivedA.a = 123;
	inputDerivedA.b = 3.14;
	inputDerivedA.c = "456";

	scg::serialize::Writer writer2;
	serialize(writer2, inputDerivedA);

	scg::serialize::Reader reader2(writer2.bytes());
	TestStructDerivedA outputDerivedA;
	err = deserialize(outputDerivedA, reader2);
	TEST_CHECK(!err);

	TEST_CHECK(inputDerivedA.a == outputDerivedA.a);
	TEST_CHECK(inputDerivedA.b == outputDerivedA.b);
	TEST_CHECK(inputDerivedA.c == outputDerivedA.c);

	TestClassPrivate inputPrivate(123, 3.14);

	scg::serialize::Writer writer3;
	serialize(writer3, inputPrivate);

	scg::serialize::Reader reader3(writer3.bytes());
	TestClassPrivate outputPrivate(0, 0);
	err = deserialize(outputPrivate, reader3);
	TEST_CHECK(!err);

	TEST_CHECK(inputPrivate.a() == outputPrivate.a());
	TEST_CHECK(inputPrivate.b() == outputPrivate.b());

	TestDerivedPrivate inputDerivedPrivate(123, 3.14, "456");

	scg::serialize::Writer writer4;
	serialize(writer4, inputDerivedPrivate);

	scg::serialize::Reader reader4(writer4.bytes());
	TestDerivedPrivate outputDerivedPrivate(0, 0, "");
	err = deserialize(outputDerivedPrivate, reader4);
	TEST_CHECK(!err);

	TEST_CHECK(inputDerivedPrivate.a() == outputDerivedPrivate.a());
	TEST_CHECK(inputDerivedPrivate.b() == outputDerivedPrivate.b());
	TEST_CHECK(inputDerivedPrivate.c() == outputDerivedPrivate.c());
}

void test_serialize_multiple_types_in_sequence()
{
	// Create test data of different types
	std::string strValue = "Hello, World! 世界";
	scg::type::uuid uuidValue = scg::type::uuid::random();
	scg::type::timestamp timeValue;
	bool boolValue = true;
	uint8_t uint8Value = 42;
	uint32_t uint32Value = 12345678;
	int64_t int64Value = -9876543210;
	float64_t float64Value = 3.14159;

	// Calculate total size
	int totalSize = bit_size(strValue) +
		bit_size(uuidValue) +
		bit_size(timeValue) +
		bit_size(boolValue) +
		bit_size(uint8Value) +
		bit_size(uint32Value) +
		bit_size(int64Value) +
		bit_size(float64Value);

	// Serialize all types into a single buffer
	scg::serialize::FixedSizeWriter writer(scg::serialize::bits_to_bytes(totalSize));
	serialize(writer, strValue);
	serialize(writer, uuidValue);
	serialize(writer, timeValue);
	serialize(writer, boolValue);
	serialize(writer, uint8Value);
	serialize(writer, uint32Value);
	serialize(writer, int64Value);
	serialize(writer, float64Value);

	// Deserialize all types from the buffer in the same order
	scg::serialize::Reader reader(writer.bytes());

	std::string strOut;
	auto err = deserialize(strOut, reader);
	TEST_CHECK(!err);
	TEST_CHECK(strValue == strOut);

	scg::type::uuid uuidOut;
	err = deserialize(uuidOut, reader);
	TEST_CHECK(!err);
	TEST_CHECK(uuidValue == uuidOut);

	scg::type::timestamp timeOut;
	err = deserialize(timeOut, reader);
	TEST_CHECK(!err);
	TEST_CHECK(timeValue == timeOut);

	bool boolOut;
	err = deserialize(boolOut, reader);
	TEST_CHECK(!err);
	TEST_CHECK(boolValue == boolOut);

	uint8_t uint8Out;
	err = deserialize(uint8Out, reader);
	TEST_CHECK(!err);
	TEST_CHECK(uint8Value == uint8Out);

	uint32_t uint32Out;
	err = deserialize(uint32Out, reader);
	TEST_CHECK(!err);
	TEST_CHECK(uint32Value == uint32Out);

	int64_t int64Out;
	err = deserialize(int64Out, reader);
	TEST_CHECK(!err);
	TEST_CHECK(int64Value == int64Out);

	float64_t float64Out;
	err = deserialize(float64Out, reader);
	TEST_CHECK(!err);
	TEST_CHECK(float64Value == float64Out);
}

void test_serialize_multiple_strings_in_sequence()
{
	// Test multiple strings back-to-back to ensure boundaries are preserved
	std::vector<std::string> strings = {
		"",                                                    // empty string
		"short",                                               // short string
		"Hello, World! This is a longer string with unicode 世界", // long string with unicode
		"another one",                                         // another string
		"final string with special chars \n\t@#$%"             // special chars
	};

	// Calculate total size
	int totalSize = 0;
	for (const auto& s : strings) {
		totalSize += bit_size(s);
	}

	// Serialize all strings
	scg::serialize::FixedSizeWriter writer(scg::serialize::bits_to_bytes(totalSize));
	for (const auto& s : strings) {
		serialize(writer, s);
	}

	// Deserialize all strings
	scg::serialize::Reader reader(writer.bytes());
	for (size_t i = 0; i < strings.size(); i++) {
		std::string actual;
		auto err = deserialize(actual, reader);
		TEST_CHECK(!err);
		TEST_CHECK(strings[i] == actual);
	}
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
	TEST(test_serialize_context),
	TEST(test_stream_writer_reader),
	TEST(test_serialize_macros),
	TEST(test_serialize_multiple_types_in_sequence),
	TEST(test_serialize_multiple_strings_in_sequence),

	{ NULL, NULL }
};
