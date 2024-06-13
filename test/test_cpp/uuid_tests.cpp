#include <cstdio>
#include <acutest.h>

#include <vector>
#include <string>

#include "scg/uuid.h"

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
		TEST_CHECK(scg::uuid::isValid(s));
	}

	auto invalid = std::vector<std::string>{
		"g1234567-1234-4234-a123-123456789abc",
		"12345678-1234-1234-1234-123456789abcz",
		"12345678-1234-1234-1234-123456789ab",
		"12345678-1234-1234-1234-123456789abcde",
		"12345678-1234-2234-1234-123456789abc",
		"12345678-1234-4234-1234-123456789abc",
		"123456781234-4234-1234-123456789abc",
		"12345678-1234-4234-1234-123456789abc-",
		"12345678-1234-4234-1234-123456789abc ",
		" 12345678-1234-4234-1234-123456789abc"
	};
	for (auto& s : invalid) {
		TEST_CHECK(!scg::uuid::isValid(s));
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
		auto [u, err] = scg::uuid::fromString(s);
		TEST_CHECK(err == nullptr);
		TEST_CHECK(u.toString() == s);
	}

	auto invalid = std::vector<std::string>{
		"g1234567-1234-4234-a123-123456789abc",
		"12345678-1234-1234-1234-123456789abcz",
		"12345678-1234-1234-1234-123456789ab",
		"12345678-1234-1234-1234-123456789abcde",
		"12345678-1234-2234-1234-123456789abc",
		"12345678-1234-4234-1234-123456789abc",
		"123456781234-4234-1234-123456789abc",
		"12345678-1234-4234-1234-123456789abc-",
		"12345678-1234-4234-1234-123456789abc ",
		" 12345678-1234-4234-1234-123456789abc"
	};
	for (auto& s : invalid) {
		auto [u, err] = scg::uuid::fromString(s);
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
		auto [u1, err1] = scg::uuid::fromString(valid[i-1]);
		TEST_CHECK(err1 == nullptr);
		auto [u2, err2] = scg::uuid::fromString(valid[i]);
		TEST_CHECK(err2 == nullptr);
		TEST_CHECK(u1 == u1);
		TEST_CHECK(u2 == u2);
		TEST_CHECK(u1 != u2);
	}
}

void test_uuid_random()
{
	std::vector<scg::uuid> uuids;
	for (int i=0; i<1000; i++) {
		uuids.push_back(scg::uuid::random());
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
	auto uuid1 = scg::uuid();
	TEST_CHECK(uuid1.isNull());

	auto uuid2 = scg::uuid::random();
	TEST_CHECK(!uuid2.isNull());
}

void test_map_usage()
{
	std::map<scg::uuid, std::string> m;
	std::unordered_map<scg::uuid, std::string> um;

	auto valid = std::vector<std::string>{
		"d4c04150-4e85-4224-a0d7-e06c135e4dc3",
		"26181c39-ef61-48f9-bc4e-7372f0480853",
		"6ef68936-e847-49d0-8ea2-a59b753b7535",
		"8e0876c9-a1eb-4a5b-b0af-c3eca589b60d",
		"171c7ba4-9748-467e-a03b-ea3d24ec6c1c"
	};

	for (auto& s : valid) {
		auto [u, err] = scg::uuid::fromString(s);
		TEST_CHECK(err == nullptr);
		m[u] = s;
		um[u] = s;
	}

	for (auto& s : valid) {
		auto [u, err] = scg::uuid::fromString(s);
		TEST_CHECK(err == nullptr);
		TEST_CHECK(m[u] == s);
		TEST_CHECK(um[u] == s);
	}
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

	{ NULL, NULL }
};
