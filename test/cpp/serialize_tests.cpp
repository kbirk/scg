#include <cstdio>
#include <acutest.h>

#include "scg/serialize.h"

void test_serialize_uint8()
{
	uint8_t input = 234U;

	scg::serialize::FixedSizeWriter writer(scg::serialize::calc_byte_size(input));
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

	scg::serialize::FixedSizeWriter writer(scg::serialize::calc_byte_size(input));
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

	scg::serialize::FixedSizeWriter writer(scg::serialize::calc_byte_size(input));
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

	scg::serialize::FixedSizeWriter writer(scg::serialize::calc_byte_size(input));
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

	scg::serialize::FixedSizeWriter writer(scg::serialize::calc_byte_size(input));
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

	scg::serialize::FixedSizeWriter writer(scg::serialize::calc_byte_size(input));
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

	scg::serialize::FixedSizeWriter writer(scg::serialize::calc_byte_size(input));
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

	scg::serialize::FixedSizeWriter writer(scg::serialize::calc_byte_size(input));
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

	scg::serialize::FixedSizeWriter writer(scg::serialize::calc_byte_size(input));
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

	scg::serialize::FixedSizeWriter writer(scg::serialize::calc_byte_size(input));
	scg::serialize::serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	float64_t output = 0;
	auto err = scg::serialize::deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(input == output);
}

void test_serialize_string()
{
	std::string input = "Hello, World! This is my test string 12312341234! \\@#$%@&^&%^\n newline \t _yay";

	scg::serialize::FixedSizeWriter writer(scg::serialize::calc_byte_size(input));
	scg::serialize::serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	std::string output;
	auto err = scg::serialize::deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(input == output);
}

void test_serialize_vector()
{
	std::vector<float64_t> input = { 1.0, -2.0, 3.0, -4.0, 5.0 };

	scg::serialize::FixedSizeWriter writer(scg::serialize::calc_byte_size(input));
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

	scg::serialize::FixedSizeWriter writer(scg::serialize::calc_byte_size(input));
	scg::serialize::serialize(writer, input);

	scg::serialize::Reader reader(writer.bytes());
	std::map<std::string, float64_t> output;
	auto err = scg::serialize::deserialize(output, reader);
	TEST_CHECK(!err);

	TEST_CHECK(input == output);
}

// helper method to reduce redundant test typing
#define TEST(x) {#x, x}

TEST_LIST = {
	// serialize
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
	TEST(test_serialize_vector),
	TEST(test_serialize_map),

	{ NULL, NULL }
};
