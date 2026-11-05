#include <cstdio>
#include <acutest.h>
#include <fstream>
#include <array>
#include <climits>
#include <cstdint>
#include <chrono>
#include <map>
#include <set>
#include <unordered_map>
#include <unordered_set>

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

	scg::serialize::Writer writer(scg::serialize::bits_to_bytes(bit_size(input)));
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

		scg::serialize::Writer writer(scg::serialize::bits_to_bytes(bit_size(input)));
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

		scg::serialize::Writer writer(scg::serialize::bits_to_bytes(bit_size(input)));
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

		scg::serialize::Writer writer(scg::serialize::bits_to_bytes(bit_size(input)));
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

		scg::serialize::Writer writer(scg::serialize::bits_to_bytes(bit_size(input)));
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

		scg::serialize::Writer writer(scg::serialize::bits_to_bytes(bit_size(input)));
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

		scg::serialize::Writer writer(scg::serialize::bits_to_bytes(bit_size(input)));
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

		scg::serialize::Writer writer(scg::serialize::bits_to_bytes(bit_size(input)));
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

		scg::serialize::Writer writer(scg::serialize::bits_to_bytes(bit_size(input)));
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

		scg::serialize::Writer writer(scg::serialize::bits_to_bytes(bit_size(input)));
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

		scg::serialize::Writer writer(scg::serialize::bits_to_bytes(bit_size(input)));
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

	scg::serialize::Writer writer(scg::serialize::bits_to_bytes(bit_size(input)));
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

	scg::serialize::Writer writer(scg::serialize::bits_to_bytes(bit_size(input)));
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

	scg::serialize::Writer writer(scg::serialize::bits_to_bytes(bit_size(input)));
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

	scg::serialize::Writer writer(scg::serialize::bits_to_bytes(bit_size(input)));
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

	scg::serialize::Writer writer(scg::serialize::bits_to_bytes(bit_size(input)));
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

	scg::serialize::Writer writer(scg::serialize::bits_to_bytes(bit_size(input)));
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

	// Serialize all types into a single buffer
	scg::serialize::Writer writer;
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

	// Serialize all strings
	scg::serialize::Writer writer;
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

void test_writer_resize_behavior()
{
	// Test that writer correctly resizes when required space is more than double current capacity
	// Start with a small writer (8 bytes)
	scg::serialize::Writer writer(8);

	// Create data that requires more than 2x the current size (> 16 bytes)
	// Using a 100-byte array to ensure we exceed 2x the initial capacity
	std::vector<uint8_t> largeData(100);
	for (size_t i = 0; i < largeData.size(); i++) {
		largeData[i] = static_cast<uint8_t>(i % 256);
	}

	// Write the large data - this should trigger resize to at least 100 bytes (not just 16)
	writer.writeBytes(largeData.data(), largeData.size());

	// Verify the data was written correctly
	const auto& bs = writer.bytes();
	TEST_CHECK(bs.size() >= 100);

	// Verify the data matches
	for (size_t i = 0; i < 100; i++) {
		TEST_CHECK(bs[i] == static_cast<uint8_t>(i % 256));
	}

	// Test with writeByte as well - start fresh with a small writer
	scg::serialize::Writer writer2(4);

	// Write small amount first
	writer2.writeByte(0xFF);

	// Now write data that requires > 2x current capacity
	// Writer should have ~4 bytes, we'll write enough to require > 8 bytes
	for (int i = 0; i < 80; i++) { // 80 bytes
		writer2.writeByte(static_cast<uint8_t>(i % 256));
	}

	const auto& bs2 = writer2.bytes();
	TEST_CHECK(bs2.size() >= 81);

	// Verify first byte
	TEST_CHECK(bs2[0] == 0xFF);

	// Verify remaining bytes
	for (size_t i = 1; i < 81; i++) {
		TEST_CHECK(bs2[i] == static_cast<uint8_t>((i - 1) % 256));
	}
}

void test_writer_resize_unaligned_write()
{
	// Test resize behavior with unaligned writes (when numBitsWritten is not byte-aligned)
	scg::serialize::Writer writer(4);

	// Write 3 bits to make it unaligned
	writer.writeBits(0b101, 3);

	// Now write a large amount of data that requires > 2x capacity
	std::vector<uint8_t> largeData(50);
	for (size_t i = 0; i < largeData.size(); i++) {
		largeData[i] = static_cast<uint8_t>(i);
	}

	writer.writeBytes(largeData.data(), largeData.size());

	// Deserialize and verify
	scg::serialize::Reader reader(writer.bytes());

	// Read the 3 bits back
	uint8_t bits = 0;
	auto err = reader.readBits(bits, 3);
	TEST_CHECK(!err);
	TEST_CHECK(bits == 0b101);

	// Read the bytes back
	std::vector<uint8_t> result(50);
	err = reader.readBytes(result.data(), result.size());
	TEST_CHECK(!err);

	for (size_t i = 0; i < 50; i++) {
		TEST_CHECK(result[i] == static_cast<uint8_t>(i));
	}
}

// test_deserialize_rejects_oversized_length verifies the length-prefixed
// deserializers bound their allocation against the bytes actually present, so a
// tiny hostile frame cannot force a huge allocation.
void test_deserialize_rejects_oversized_length()
{
	using scg::serialize::Writer;
	using scg::serialize::Reader;

	// A declared string length far exceeding the data is rejected before allocating.
	{
		Writer writer(8);
		serialize(writer, uint32_t(1u << 30)); // ~1 GiB declared
		std::vector<uint8_t> buf = writer.bytes();
		buf.push_back(0xAB);
		buf.push_back(0xCD);
		Reader reader(buf);
		std::string out;
		auto err = deserialize(out, reader);
		TEST_CHECK(err);
	}

	// A declared vector element count far exceeding the data is rejected as the
	// bytes are exhausted (no multi-GiB reserve up front).
	{
		Writer writer(8);
		serialize(writer, uint32_t(1u << 28));
		std::vector<uint8_t> buf = writer.bytes();
		buf.push_back(1);
		buf.push_back(2);
		buf.push_back(3);
		Reader reader(buf);
		std::vector<float64_t> out;
		auto err = deserialize(out, reader);
		TEST_CHECK(err);
	}

	// A genuinely large vector whose elements really are present still round-trips
	// (the bound must not over-reject).
	{
		std::vector<int32_t> input(5000);
		for (size_t i = 0; i < input.size(); i++) {
			input[i] = int32_t(i);
		}
		Writer writer(scg::serialize::bits_to_bytes(bit_size(input)));
		serialize(writer, input);
		Reader reader(writer.bytes());
		std::vector<int32_t> output;
		auto err = deserialize(output, reader);
		TEST_CHECK(!err);
		TEST_CHECK(input == output);
	}
}


// ============================================================================
// Hardening / regression tests
// ============================================================================

namespace hardening {

// Big is a large in-memory element whose wire form is one tag byte that must
// be 1. A count of Big elements over a buffer of zeros fails on the first
// element, so the vector's capacity shows how much was reserved up front.
struct Big {
	std::array<uint8_t, 4096> pad{};
	uint8_t tag = 1;
};

inline uint32_t bit_size(const Big&) { return 8; }

template <typename WriterType>
inline void serialize(WriterType& writer, const Big& value) { writer.writeByte(value.tag); }

template <typename ReaderType>
inline scg::error::Error deserialize(Big& value, ReaderType& reader)
{
	auto err = reader.readByte(value.tag);
	if (err) {
		return err;
	}
	if (value.tag != 1) {
		return scg::error::Error("bad tag");
	}
	return nullptr;
}

// Nothing is a zero-bit element (like a field-less message).
struct Nothing {
	bool operator==(const Nothing&) const { return true; }
};

inline uint32_t bit_size(const Nothing&) { return 0; }

template <typename WriterType>
inline void serialize(WriterType&, const Nothing&) {}

template <typename ReaderType>
inline scg::error::Error deserialize(Nothing&, ReaderType&) { return nullptr; }

template <typename T>
std::vector<uint8_t> encode(const T& value)
{
	using scg::serialize::bit_size;
	using scg::serialize::serialize;
	scg::serialize::Writer writer(scg::serialize::bits_to_bytes(bit_size(value)));
	serialize(writer, value);
	return writer.bytes();
}

template <typename T>
scg::error::Error decode(T& value, const std::vector<uint8_t>& bytes)
{
	using scg::serialize::deserialize;
	scg::serialize::Reader reader(bytes);
	return deserialize(value, reader);
}

} // namespace hardening

// Containers nested inside other containers must resolve the inner overloads
// (previously list<map<...>> hit the static_assert fallback).
void test_serialize_nested_containers()
{
	using hardening::encode;
	using hardening::decode;

	std::vector<std::map<std::string, int32_t>> a = {{{"a", 1}, {"b", -2}}, {}, {{"c", 3}}};
	std::map<std::string, std::vector<std::map<uint8_t, std::set<std::string>>>> b = {{"x", {{{1, {"p", "q"}}}, {}}}};
	std::set<std::vector<uint8_t>> c = {{1, 2, 3}, {4}, {}};
	std::unordered_map<std::string, std::unordered_set<int64_t>> d = {{"k", {1, -2, 3}}};
	std::vector<std::vector<std::map<std::string, std::vector<int16_t>>>> e = {{{{"z", {1, -1}}}}, {}};
	std::array<std::map<uint32_t, std::string>, 2> f = {{{{7, "seven"}}, {}}};

	decltype(a) a2; TEST_CHECK(!decode(a2, encode(a))); TEST_CHECK(a2 == a);
	decltype(b) b2; TEST_CHECK(!decode(b2, encode(b))); TEST_CHECK(b2 == b);
	decltype(c) c2; TEST_CHECK(!decode(c2, encode(c))); TEST_CHECK(c2 == c);
	decltype(d) d2; TEST_CHECK(!decode(d2, encode(d))); TEST_CHECK(d2 == d);
	decltype(e) e2; TEST_CHECK(!decode(e2, encode(e))); TEST_CHECK(e2 == e);
	decltype(f) f2; TEST_CHECK(!decode(f2, encode(f))); TEST_CHECK(f2 == f);
}

// A hostile element count must not drive a huge up-front reservation.
void test_deserialize_hostile_counts()
{
	using scg::serialize::Writer;
	using scg::serialize::Reader;

	// count = 0xFFFFFFFF with no elements present fails fast.
	{
		Writer writer(8);
		serialize(writer, uint32_t(0xFFFFFFFFu));
		std::vector<float64_t> out;
		auto start = std::chrono::steady_clock::now();
		TEST_CHECK(hardening::decode(out, writer.bytes()));
		TEST_CHECK(std::chrono::steady_clock::now() - start < std::chrono::seconds(1));
	}

	// vector<Big> with count == remaining bytes: the reservation is bounded in
	// bytes, not count * sizeof(Big) (which would be ~800 MiB here).
	{
		const uint32_t count = 200000;
		Writer writer(8);
		serialize(writer, count);
		std::vector<uint8_t> buf = writer.bytes();
		buf.resize(buf.size() + count, 0); // zero tags: first element fails
		std::vector<hardening::Big> out;
		auto err = hardening::decode(out, buf);
		TEST_CHECK(err);
		TEST_CHECK(out.capacity() * sizeof(hardening::Big) <= scg::serialize::MAX_RESERVE_BYTES + sizeof(hardening::Big));
		TEST_MSG("capacity=%zu", out.capacity());
	}

	// Zero-size elements (e.g. hand-written field-less structs) are not
	// rejected: whatever the Writer produced must decode, at any count.
	{
		std::vector<hardening::Nothing> in(200000);
		std::vector<hardening::Nothing> out;
		TEST_CHECK(!hardening::decode(out, hardening::encode(in)));
		TEST_CHECK(out.size() == in.size());

		std::map<uint8_t, hardening::Nothing> mapIn = {{1, {}}, {2, {}}};
		std::map<uint8_t, hardening::Nothing> mapOut;
		TEST_CHECK(!hardening::decode(mapOut, hardening::encode(mapIn)));
		TEST_CHECK(mapOut.size() == 2);
	}
}

// Decoding a map/set replaces its contents (matching vector) instead of merging
// into whatever the target held.
void test_deserialize_replaces_container_contents()
{
	using hardening::encode;
	using hardening::decode;

	std::map<std::string, int32_t> m = {{"stale", 1}};
	TEST_CHECK(!decode(m, encode(std::map<std::string, int32_t>{{"fresh", 2}})));
	TEST_CHECK(m.size() == 1 && m.count("fresh") == 1);

	std::unordered_map<std::string, int32_t> um = {{"stale", 1}};
	TEST_CHECK(!decode(um, encode(std::unordered_map<std::string, int32_t>{{"fresh", 2}})));
	TEST_CHECK(um.size() == 1 && um.count("fresh") == 1);

	std::set<int32_t> st = {99};
	TEST_CHECK(!decode(st, encode(std::set<int32_t>{1, 2})));
	TEST_CHECK(st == (std::set<int32_t>{1, 2}));

	std::unordered_set<int32_t> us = {99};
	TEST_CHECK(!decode(us, encode(std::unordered_set<int32_t>{1, 2})));
	TEST_CHECK(us == (std::unordered_set<int32_t>{1, 2}));
}

// list<byte> is bulk encoded but must stay wire-identical to the per-element
// encoding (and to the string encoding), aligned or not.
void test_byte_vector_wire_identical()
{
	using scg::serialize::Writer;
	using scg::serialize::Reader;

	std::vector<uint8_t> data;
	for (int i = 0; i < 300; i++) {
		data.push_back(static_cast<uint8_t>(i * 7));
	}
	std::string asString(data.begin(), data.end());

	for (int misalign = 0; misalign < 8; misalign++) {
		Writer bulk(4);
		Writer manual(4);
		Writer str(4);
		for (int i = 0; i < misalign; i++) {
			serialize(bulk, true);
			serialize(manual, true);
			serialize(str, true);
		}
		serialize(bulk, data);
		serialize(manual, uint32_t(data.size()));
		for (auto b : data) {
			manual.writeByte(b);
		}
		serialize(str, asString);
		TEST_CHECK(bulk.bytes() == manual.bytes());
		TEST_CHECK(bulk.bytes() == str.bytes());

		Reader reader(bulk.bytes());
		for (int i = 0; i < misalign; i++) {
			bool b = false;
			TEST_CHECK(!deserialize(b, reader));
		}
		std::vector<uint8_t> out;
		TEST_CHECK(!deserialize(out, reader));
		TEST_CHECK(out == data);
	}

	// bit_size matches the generic formula.
	TEST_CHECK(bit_size(data) == bit_size(uint32_t(data.size())) + uint32_t(data.size()) * 8);

	// int8 vectors take the same path.
	std::vector<int8_t> signedData = {-128, -1, 0, 1, 127};
	std::vector<int8_t> signedOut;
	TEST_CHECK(!hardening::decode(signedOut, hardening::encode(signedData)));
	TEST_CHECK(signedOut == signedData);

	// A declared length past the end is rejected before allocating.
	{
		Writer writer(8);
		serialize(writer, uint32_t(1u << 30));
		std::vector<uint8_t> out;
		TEST_CHECK(hardening::decode(out, writer.bytes()));
	}
}

// Pre-1970 timestamps use the canonical (Go) form: floor seconds and
// nanoseconds in [0, 1e9). The legacy truncated form still decodes, and
// out-of-range values are rejected instead of overflowing.
void test_writer_size_bytes()
{
	// sizeBytes is the written prefix of a pre-sized buffer (persisted assets
	// store only that prefix)
	scg::serialize::Writer writer(1024);
	TEST_CHECK(writer.sizeBytes() == 0);
	writer.writeBits(uint8_t(1), 1);
	TEST_CHECK(writer.sizeBytes() == 1);
	for (int i = 0; i < 3; ++i) {
		writer.writeBits(uint8_t(0xAB), 8);
	}
	TEST_CHECK(writer.sizeBytes() == 4);
	TEST_CHECK(writer.bytes().size() == 1024);

	std::vector<uint8_t> buffer(64, 0);
	scg::serialize::WriterView view(buffer);
	TEST_CHECK(view.sizeBytes() == 0);
	view.writeBits(uint8_t(0x3), 2);
	view.writeBits(uint8_t(0xFF), 8);
	TEST_CHECK(view.sizeBytes() == 2);
	TEST_CHECK(view.bytes().size() == 64);
}

void test_timestamp_canonical_and_bounds()
{
	using scg::serialize::Writer;
	using scg::serialize::Reader;
	using tp_t = std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds>;

	scg::type::timestamp before1970(tp_t(std::chrono::nanoseconds(-1500000000LL))); // -1.5 s
	{
		Writer writer(scg::serialize::bits_to_bytes(bit_size(before1970)));
		serialize(writer, before1970);
		Reader reader(writer.bytes());
		uint64_t secs = 0;
		uint64_t nanos = 0;
		TEST_CHECK(!deserialize(secs, reader));
		TEST_CHECK(!deserialize(nanos, reader));
		TEST_CHECK(static_cast<int64_t>(secs) == -2);
		TEST_CHECK(nanos == 500000000ULL);

		Reader again(writer.bytes());
		scg::type::timestamp out;
		TEST_CHECK(!deserialize(out, again));
		TEST_CHECK(out == before1970);
	}

	// Legacy C++ encoding: truncated seconds and negative nanoseconds.
	{
		Writer writer(32);
		serialize(writer, static_cast<uint64_t>(int64_t(-1)));
		serialize(writer, static_cast<uint64_t>(int64_t(-500000000)));
		Reader reader(writer.bytes());
		scg::type::timestamp out;
		TEST_CHECK(!deserialize(out, reader));
		TEST_CHECK(out == before1970);
	}

	// Positive values are unchanged: seconds then nanoseconds.
	{
		scg::type::timestamp t(tp_t(std::chrono::nanoseconds(1700000000123456789LL)));
		Writer writer(scg::serialize::bits_to_bytes(bit_size(t)));
		serialize(writer, t);
		Reader reader(writer.bytes());
		uint64_t secs = 0;
		uint64_t nanos = 0;
		TEST_CHECK(!deserialize(secs, reader));
		TEST_CHECK(!deserialize(nanos, reader));
		TEST_CHECK(secs == 1700000000ULL);
		TEST_CHECK(nanos == 123456789ULL);
	}

	// Values outside the int64-nanosecond range decode successfully and saturate
	// to the nearest limit (never an error, never signed overflow), so one
	// unrepresentable timestamp cannot fail a whole message.
	auto decodeNanos = [](uint64_t secs, uint64_t nanos, int64_t& outNanos) -> bool {
		Writer writer(32);
		serialize(writer, secs);
		serialize(writer, nanos);
		Reader reader(writer.bytes());
		scg::type::timestamp out;
		if (deserialize(out, reader)) {
			return false;
		}
		outNanos = out.timepoint().time_since_epoch().count();
		return true;
	};
	int64_t n = 0;
	// Go's zero time.Time (0001-01-01T00:00:00Z) is -62135596800 seconds.
	TEST_CHECK(decodeNanos(uint64_t(int64_t(-62135596800LL)), 0, n));
	TEST_CHECK(n == INT64_MIN);
	// ... and the minimum encodes back as Go's zero time, so it survives a
	// round trip through C++.
	{
		scg::type::timestamp minimum(tp_t(std::chrono::nanoseconds(INT64_MIN)));
		Writer writer(scg::serialize::bits_to_bytes(bit_size(minimum)));
		serialize(writer, minimum);
		TEST_CHECK(writer.bytes().size() == scg::serialize::bits_to_bytes(bit_size(minimum)));
		Reader reader(writer.bytes());
		uint64_t secs = 0;
		uint64_t nanos = 0;
		TEST_CHECK(!deserialize(secs, reader));
		TEST_CHECK(!deserialize(nanos, reader));
		TEST_CHECK(static_cast<int64_t>(secs) == -62135596800LL);
		TEST_CHECK(nanos == 0);
	}
	TEST_CHECK(decodeNanos(uint64_t(1) << 62, 0, n));
	TEST_CHECK(n == INT64_MAX);
	TEST_CHECK(decodeNanos(uint64_t(int64_t(INT64_MIN)), 0, n));
	TEST_CHECK(n == INT64_MIN);
	TEST_CHECK(decodeNanos(uint64_t(9223372036ULL), uint64_t(999999999ULL), n)); // just past INT64_MAX ns
	TEST_CHECK(n == INT64_MAX);
	TEST_CHECK(decodeNanos(uint64_t(INT64_MAX), uint64_t(INT64_MAX), n)); // carry would overflow seconds
	TEST_CHECK(n == INT64_MAX);
	TEST_CHECK(decodeNanos(uint64_t(int64_t(INT64_MIN)), uint64_t(int64_t(INT64_MIN)), n));
	TEST_CHECK(n == INT64_MIN);
	// Non-canonical nanoseconds outside (-1e9, 1e9) are folded into seconds exactly.
	TEST_CHECK(decodeNanos(1, uint64_t(1000000000ULL), n));
	TEST_CHECK(n == 2000000000LL);
	TEST_CHECK(decodeNanos(uint64_t(int64_t(-1)), uint64_t(int64_t(-2500000000LL)), n));
	TEST_CHECK(n == -3500000000LL);
}

// Bytes produced by the previous C++ encoder (pre-1970 values used truncated
// seconds and negative nanoseconds) must keep decoding to the same instant;
// every value the encoder can produce round-trips, including the int64 limits.
void test_timestamp_decodes_legacy_bytes()
{
	using tp_t = std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds>;
	struct Case {
		int64_t ns;
		std::vector<uint8_t> legacy;
	};
	// Generated with the encoder before the canonicalization change.
	const std::vector<Case> cases = {
		{-1500000000LL, {0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01, 0x6e, 0x96, 0x29, 0xfe, 0xff, 0xff, 0xff, 0xff}},
		{-1LL, {0xfe, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}},
		{INT64_MIN, {0xf9, 0x0b, 0xf6, 0xa9, 0xbd, 0xff, 0xff, 0xff, 0xff, 0x01, 0xa2, 0x6c, 0xd8, 0xfc, 0xff, 0xff, 0xff, 0xff}},
		{INT64_MAX, {0x09, 0xf6, 0x0d, 0x5e, 0x52, 0xc0, 0xff, 0xd7, 0xe5, 0xcb, 0x00}},
		{1700000000123456789LL, {0x01, 0xc6, 0x9f, 0x5a, 0x66, 0xc5, 0xe6, 0x5b, 0x0f, 0x00}},
		{0LL, {0x00}},
		{-3155760000000000123LL, {0x01, 0xb3, 0x37, 0x3f, 0xf4, 0xff, 0xff, 0xff, 0xff, 0x0b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
	};
	for (const auto& c : cases) {
		scg::type::timestamp expected{tp_t{std::chrono::nanoseconds{c.ns}}};

		scg::serialize::Reader legacy(c.legacy);
		scg::type::timestamp fromLegacy;
		TEST_CHECK(!deserialize(fromLegacy, legacy));
		TEST_CHECK(fromLegacy == expected);
		TEST_MSG("legacy decode of %lld", (long long)c.ns);

		scg::serialize::Writer writer(scg::serialize::bits_to_bytes(bit_size(expected)));
		serialize(writer, expected);
		scg::serialize::Reader current(writer.bytes());
		scg::type::timestamp roundTrip;
		TEST_CHECK(!deserialize(roundTrip, current));
		TEST_CHECK(roundTrip == expected);
		TEST_MSG("round trip of %lld", (long long)c.ns);

		// Non-negative values keep the exact same bytes.
		if (c.ns >= 0) {
			TEST_CHECK(writer.bytes() == c.legacy);
		}
	}
}

// Byte-at-a-time growth must stay amortized O(1) (capacity used to double on
// every write past the presized buffer).
void test_writer_growth_is_geometric()
{
	scg::serialize::Writer writer(1);
	const size_t n = 1 << 20;
	auto start = std::chrono::steady_clock::now();
	for (size_t i = 0; i < n; i++) {
		writer.writeByte(static_cast<uint8_t>(i));
	}
	TEST_CHECK(writer.bytes().size() == n);
	TEST_CHECK(writer.bytes().capacity() <= 2 * n);
	TEST_CHECK(std::chrono::steady_clock::now() - start < std::chrono::seconds(5));
	for (size_t i = 0; i < n; i += 4099) {
		TEST_CHECK(writer.bytes()[i] == static_cast<uint8_t>(i));
	}

	// Zero-size writers are allowed (zero-bit messages).
	scg::serialize::Writer empty(0);
	TEST_CHECK(empty.bytes().empty());
	std::vector<uint8_t> buf;
	scg::serialize::WriterView view(buf);
	TEST_CHECK(buf.empty());
}

// take() hands the buffer over without a copy.
void test_writer_take()
{
	scg::serialize::Writer writer(8);
	serialize(writer, uint32_t(12345));
	auto expected = writer.bytes();
	const uint8_t* data = writer.bytes().data();
	auto taken = std::move(writer).take();
	TEST_CHECK(taken == expected);
	TEST_CHECK(taken.data() == data);
}

void test_reader_bits_read()
{
	scg::serialize::Writer writer(8);
	serialize(writer, true);
	serialize(writer, uint8_t(7));
	scg::serialize::Reader reader(writer.bytes());
	TEST_CHECK(reader.bitsRead() == 0);
	bool b = false;
	TEST_CHECK(!deserialize(b, reader));
	TEST_CHECK(reader.bitsRead() == 1);
	uint8_t v = 0;
	TEST_CHECK(!deserialize(v, reader));
	TEST_CHECK(reader.bitsRead() == 9);
	scg::serialize::ReaderView view(writer.bytes());
	TEST_CHECK(!deserialize(b, view));
	TEST_CHECK(view.bitsRead() == 1);
}

// helper method to reduce redundant test typing
#define TEST(x) {#x, x}

TEST_LIST = {
	TEST(test_deserialize_rejects_oversized_length),
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
	TEST(test_serialize_macros),
	TEST(test_serialize_multiple_types_in_sequence),
	TEST(test_serialize_multiple_strings_in_sequence),
	TEST(test_writer_resize_behavior),
	TEST(test_writer_resize_unaligned_write),
	TEST(test_serialize_nested_containers),
	TEST(test_deserialize_hostile_counts),
	TEST(test_deserialize_replaces_container_contents),
	TEST(test_byte_vector_wire_identical),
	TEST(test_writer_size_bytes),
	TEST(test_timestamp_canonical_and_bounds),
	TEST(test_timestamp_decodes_legacy_bytes),
	TEST(test_writer_growth_is_geometric),
	TEST(test_writer_take),
	TEST(test_reader_bits_read),

	{ NULL, NULL }
};
