#include <cstdio>
#include <acutest.h>

#include <vector>
#include <string>

#include "scg/uuid.h"
#include "scg/reader.h"
#include "scg/writer.h"
#include "nlohmann/json.hpp"

#include <sstream>
#include <map>
#include <unordered_map>

void test_uuid_is_valid()
{
	auto valid = std::vector<std::string>{
		"d4c04150-4e85-4224-a0d7-e06c135e4dc3",
		"26181c39-ef61-48f9-bc4e-7372f0480853",
		"6ef68936-e847-49d0-8ea2-a59b753b7535",
		"8e0876c9-a1eb-4a5b-b0af-c3eca589b60d",
		"171c7ba4-9748-467e-a03b-ea3d24ec6c1c"
	};
	for (auto& s : valid) {
		TEST_CHECK(scg::type::uuid::isValid(s));
	}

	auto invalid = std::vector<std::string>{
		"g1234567-1234-4234-a123-123456789abc",
		"12345678-1234-1234-1234-123456789abcz",
		"12345678-1234-1234-1234-123456789ab",
		"12345678-1234-1234-1234-123456789abcde",
		"123456781234-4234-1234-123456789abc",
		"12345678-1234-4234-1234-123456789abc-",
		"12345678-1234-4234-1234-123456789abc ",
		" 12345678-1234-4234-1234-123456789abc"
	};
	for (auto& s : invalid) {
		TEST_CHECK(!scg::type::uuid::isValid(s));
	}
}

void test_uuid_from_string_to_string()
{
	auto valid = std::vector<std::string>{
		"d4c04150-4e85-4224-a0d7-e06c135e4dc3",
		"26181c39-ef61-48f9-bc4e-7372f0480853",
		"6ef68936-e847-49d0-8ea2-a59b753b7535",
		"8e0876c9-a1eb-4a5b-b0af-c3eca589b60d",
		"171c7ba4-9748-467e-a03b-ea3d24ec6c1c"
	};
	for (auto& s : valid) {
		auto [u, err] = scg::type::uuid::fromString(s);
		TEST_CHECK(err == nullptr);
		TEST_CHECK(u.toString() == s);
	}

	auto invalid = std::vector<std::string>{
		"g1234567-1234-4234-a123-123456789abc",
		"12345678-1234-1234-1234-123456789abcz",
		"12345678-1234-1234-1234-123456789ab",
		"12345678-1234-1234-1234-123456789abcde",
		"123456781234-4234-1234-123456789abc",
		"12345678-1234-4234-1234-123456789abc-",
		"12345678-1234-4234-1234-123456789abc ",
		" 12345678-1234-4234-1234-123456789abc"
	};
	for (auto& s : invalid) {
		auto [u, err] = scg::type::uuid::fromString(s);
		TEST_CHECK(err != nullptr);
	}
}

void test_uuid_equality()
{
	auto valid = std::vector<std::string>{
		"d4c04150-4e85-4224-a0d7-e06c135e4dc3",
		"26181c39-ef61-48f9-bc4e-7372f0480853",
		"6ef68936-e847-49d0-8ea2-a59b753b7535",
		"8e0876c9-a1eb-4a5b-b0af-c3eca589b60d",
		"171c7ba4-9748-467e-a03b-ea3d24ec6c1c"
	};

	for (uint32_t i=1; i<valid.size(); i++) {
		auto [u1, err1] = scg::type::uuid::fromString(valid[i-1]);
		TEST_CHECK(err1 == nullptr);
		auto [u2, err2] = scg::type::uuid::fromString(valid[i]);
		TEST_CHECK(err2 == nullptr);
		TEST_CHECK(u1 == u1);
		TEST_CHECK(u2 == u2);
		TEST_CHECK(u1 != u2);
	}
}

void test_uuid_random()
{
	std::vector<scg::type::uuid> uuids;
	for (int i=0; i<1000; i++) {
		uuids.push_back(scg::type::uuid::random());
	}
	for (uint32_t i=0; i<uuids.size(); i++) {
		for (uint32_t j=0; j<uuids.size(); j++) {
			if (i == j) {
				TEST_CHECK(uuids[i] == uuids[j]);
			} else {
				TEST_CHECK(uuids[i] != uuids[j]);
			}
		}
	}
}

void test_uuid_is_null()
{
	auto uuid1 = scg::type::uuid();
	TEST_CHECK(uuid1.isNull());

	auto uuid2 = scg::type::uuid::random();
	TEST_CHECK(!uuid2.isNull());
}

void test_map_usage()
{
	std::map<scg::type::uuid, std::string> m;
	std::unordered_map<scg::type::uuid, std::string> um;

	auto valid = std::vector<std::string>{
		"d4c04150-4e85-4224-a0d7-e06c135e4dc3",
		"26181c39-ef61-48f9-bc4e-7372f0480853",
		"6ef68936-e847-49d0-8ea2-a59b753b7535",
		"8e0876c9-a1eb-4a5b-b0af-c3eca589b60d",
		"171c7ba4-9748-467e-a03b-ea3d24ec6c1c"
	};

	for (auto& s : valid) {
		auto [u, err] = scg::type::uuid::fromString(s);
		TEST_CHECK(err == nullptr);
		m[u] = s;
		um[u] = s;
	}

	for (auto& s : valid) {
		auto [u, err] = scg::type::uuid::fromString(s);
		TEST_CHECK(err == nullptr);
		TEST_CHECK(m[u] == s);
		TEST_CHECK(um[u] == s);
	}
}


// Every UUID version and variant is accepted (matching Go's uuid.Parse): v1,
// v2, v7, the nil UUID and non-RFC variants.
void test_uuid_accepts_all_versions()
{
	auto valid = std::vector<std::string>{
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8", // v1
		"12345678-1234-2234-1234-123456789abc", // v2, variant 0
		"017f22e2-79b0-7cc3-98c4-dc0c0c07398f", // v7
		"00000000-0000-0000-0000-000000000000", // nil
		"FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF"  // max, uppercase
	};
	for (auto& s : valid) {
		TEST_CHECK(scg::type::uuid::isValid(s));
		auto [u, err] = scg::type::uuid::fromString(s);
		TEST_CHECK(err == nullptr);
		std::string lower = s;
		for (auto& c : lower) {
			c = static_cast<char>(std::tolower(static_cast<unsigned char>(c)));
		}
		TEST_CHECK(u.toString() == lower);

		// Binary round trip: the decoder no longer rejects non-v4 values.
		scg::serialize::Writer writer(16);
		serialize(writer, u);
		scg::serialize::Reader reader(writer.bytes());
		scg::type::uuid out;
		TEST_CHECK(deserialize(out, reader) == nullptr);
		TEST_CHECK(out == u);
	}
}

// operator>> parses into the target (it used to discard the result) and sets
// failbit on bad input.
void test_uuid_stream_extraction()
{
	{
		std::istringstream is("6ba7b810-9dad-41d1-80b4-00c04fd430c8");
		scg::type::uuid u;
		is >> u;
		TEST_CHECK(!is.fail());
		TEST_CHECK(u.toString() == "6ba7b810-9dad-41d1-80b4-00c04fd430c8");
	}
	{
		std::istringstream is("not-a-uuid");
		scg::type::uuid u;
		is >> u;
		TEST_CHECK(is.fail());
		TEST_CHECK(u.isNull());
	}
}

// Non-ASCII bytes must be rejected without passing negative chars to isxdigit
// (undefined behaviour; caught by UBSan builds).
void test_uuid_rejects_high_bit_chars()
{
	std::string s = "6ba7b810-9dad-41d1-80b4-00c04fd430c8";
	s[0] = static_cast<char>(0xE9);
	s[35] = static_cast<char>(0xFF);
	TEST_CHECK(!scg::type::uuid::isValid(s));
	auto [u, err] = scg::type::uuid::fromString(s);
	TEST_CHECK(err != nullptr);
}

void test_uuid_json_round_trip()
{
	auto [u, err] = scg::type::uuid::fromString("017f22e2-79b0-7cc3-98c4-dc0c0c07398f");
	TEST_CHECK(err == nullptr);
	nlohmann::json j = u;
	TEST_CHECK(j.get<std::string>() == "017f22e2-79b0-7cc3-98c4-dc0c0c07398f");
	auto back = j.get<scg::type::uuid>();
	TEST_CHECK(back == u);
}

// helper method to reduce redundant test typing
#define TEST(x) {#x, x}

TEST_LIST = {
	TEST(test_uuid_is_valid),
	TEST(test_uuid_from_string_to_string),
	TEST(test_uuid_equality),
	TEST(test_uuid_random),
	TEST(test_uuid_is_null),
	TEST(test_map_usage),
	TEST(test_uuid_accepts_all_versions),
	TEST(test_uuid_stream_extraction),
	TEST(test_uuid_rejects_high_bit_chars),
	TEST(test_uuid_json_round_trip),

	{ NULL, NULL }
};
