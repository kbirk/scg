// Unit tests for include/scg/json.h, compiled and run by
// internal/gen/harness_test.go (TestCppJSONUnit) under -std=c++17 and c++20.

#include "scg/json.h"

#include <cmath>
#include <cstdio>
#include <limits>
#include <map>
#include <string>
#include <unordered_map>
#include <vector>

namespace {

int failures = 0;

#define CHECK(cond)                                                          \
	do {                                                                     \
		if (!(cond)) {                                                       \
			std::printf("%s:%d: CHECK failed: %s\n", __FILE__, __LINE__, #cond); \
			failures++;                                                      \
		}                                                                    \
	} while (0)

#define CHECK_THROWS(expr)                                                         \
	do {                                                                           \
		bool threw = false;                                                        \
		try {                                                                      \
			expr;                                                                  \
		} catch (const nlohmann::json::exception&) {                               \
			threw = true;                                                          \
		}                                                                          \
		if (!threw) {                                                              \
			std::printf("%s:%d: expected an exception: %s\n", __FILE__, __LINE__, #expr); \
			failures++;                                                            \
		}                                                                          \
	} while (0)

SCG_TYPEDEF(MyU8, uint8_t);
SCG_TYPEDEF(MyI8, int8_t);
SCG_TYPEDEF(MyU64, uint64_t);
SCG_TYPEDEF(MyI64, int64_t);
SCG_TYPEDEF(MyF32, float);
SCG_TYPEDEF(MyString, std::string);
SCG_TYPEDEF(MyID, scg::type::uuid);

enum class Color {
	RED = 0,
	BLUE = 2,
};

using TimePoint = std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds>;

scg::type::timestamp at(int64_t ns)
{
	return scg::type::timestamp(TimePoint(std::chrono::nanoseconds(ns)));
}

int64_t nanos(const scg::type::timestamp& ts)
{
	return std::chrono::duration_cast<std::chrono::nanoseconds>(ts.timepoint().time_since_epoch()).count();
}

std::vector<uint8_t> bytes(const std::string& s)
{
	return std::vector<uint8_t>(s.begin(), s.end());
}

void test_base64()
{
	// RFC 4648 section 10 test vectors
	const std::pair<const char*, const char*> vectors[] = {
		{"", ""}, {"f", "Zg=="}, {"fo", "Zm8="}, {"foo", "Zm9v"},
		{"foob", "Zm9vYg=="}, {"fooba", "Zm9vYmE="}, {"foobar", "Zm9vYmFy"},
	};
	for (const auto& v : vectors) {
		CHECK(scg::json::base64_encode(bytes(v.first)) == v.second);
		std::vector<uint8_t> out;
		CHECK(scg::json::base64_decode(v.second, out));
		CHECK(out == bytes(v.first));
	}

	std::vector<uint8_t> all(256);
	for (int i = 0; i < 256; ++i) {
		all[i] = static_cast<uint8_t>(i);
	}
	std::vector<uint8_t> out;
	CHECK(scg::json::base64_decode(scg::json::base64_encode(all), out));
	CHECK(out == all);
	CHECK(scg::json::base64_encode({0xfb, 0xff}) == "+/8=");

	// like Go's StdEncoding: newlines are ignored
	CHECK(scg::json::base64_decode("Zm9v\r\nYmFy", out) && out == bytes("foobar"));

	// malformed
	for (const char* bad : {"Zg", "Zg=", "Zm9", "Z===", "Zg==Zg==", "Zm9v!", "Zm-v", "=Zm9", "Zm=v"}) {
		CHECK(!scg::json::base64_decode(bad, out));
	}
}

void test_rfc3339_format()
{
	CHECK(scg::json::format_rfc3339_nano(0) == "1970-01-01T00:00:00Z");
	CHECK(scg::json::format_rfc3339_nano(1) == "1970-01-01T00:00:00.000000001Z");
	CHECK(scg::json::format_rfc3339_nano(100000000) == "1970-01-01T00:00:00.1Z");
	CHECK(scg::json::format_rfc3339_nano(-1) == "1969-12-31T23:59:59.999999999Z");
	CHECK(scg::json::format_rfc3339_nano(-100000000000 + 5) == "1969-12-31T23:58:20.000000005Z");
	CHECK(scg::json::format_rfc3339_nano(1700000000123456789) == "2023-11-14T22:13:20.123456789Z");
	// leap days
	CHECK(scg::json::format_rfc3339_nano(951782400LL * 1000000000) == "2000-02-29T00:00:00Z");
	CHECK(scg::json::format_rfc3339_nano(1709164800LL * 1000000000) == "2024-02-29T00:00:00Z");
	// the full int64 nanosecond range
	CHECK(scg::json::format_rfc3339_nano((std::numeric_limits<int64_t>::min)()) == "1677-09-21T00:12:43.145224192Z");
	CHECK(scg::json::format_rfc3339_nano((std::numeric_limits<int64_t>::max)()) == "2262-04-11T23:47:16.854775807Z");
}

void test_rfc3339_parse()
{
	int64_t ns = 0;
	CHECK(scg::json::parse_rfc3339("1970-01-01T00:00:00Z", ns) && ns == 0);
	CHECK(scg::json::parse_rfc3339("2023-11-14T17:13:20.123456789-05:00", ns) && ns == 1700000000123456789);
	CHECK(scg::json::parse_rfc3339("2023-11-15T03:43:20.5+05:30", ns) && ns == 1700000000500000000);
	CHECK(scg::json::parse_rfc3339("1969-12-31T23:59:59.999999999Z", ns) && ns == -1);
	CHECK(scg::json::parse_rfc3339("2000-02-29T00:00:00Z", ns) && ns == 951782400LL * 1000000000);

	// every fraction length, and digits beyond nanoseconds truncated (as Go)
	const char* fractions[] = {".1", ".12", ".123", ".1234", ".12345", ".123456", ".1234567", ".12345678", ".123456789"};
	const int64_t values[] = {100000000, 120000000, 123000000, 123400000, 123450000, 123456000, 123456700, 123456780, 123456789};
	for (int i = 0; i < 9; ++i) {
		CHECK(scg::json::parse_rfc3339(std::string("1970-01-01T00:00:00") + fractions[i] + "Z", ns) && ns == values[i]);
	}
	CHECK(scg::json::parse_rfc3339("1970-01-01T00:00:00.1234567891Z", ns) && ns == 123456789);

	// range boundaries
	CHECK(scg::json::parse_rfc3339("1677-09-21T00:12:43.145224192Z", ns) && ns == (std::numeric_limits<int64_t>::min)());
	CHECK(scg::json::parse_rfc3339("2262-04-11T23:47:16.854775807Z", ns) && ns == (std::numeric_limits<int64_t>::max)());
	CHECK(!scg::json::parse_rfc3339("1677-09-21T00:12:43.145224191Z", ns));
	CHECK(!scg::json::parse_rfc3339("2262-04-11T23:47:16.854775808Z", ns));
	CHECK(!scg::json::parse_rfc3339("0001-01-01T00:00:00Z", ns));

	// round trip across the whole range, including every leap year boundary
	for (int64_t secs = -9223372036LL; secs <= 9223372035LL; secs += 86400LL * 97 + 12345) {
		int64_t v = secs * 1000000000LL + 987654321;
		CHECK(scg::json::parse_rfc3339(scg::json::format_rfc3339_nano(v), ns) && ns == v);
	}

	// malformed
	for (const char* bad : {
		"", "1970-01-01", "1970-01-01 00:00:00Z", "1970-01-01T00:00:00", "1970-01-01T00:00:00z",
		"1970-1-01T00:00:00Z", "1970-13-01T00:00:00Z", "1970-00-01T00:00:00Z", "1970-01-32T00:00:00Z",
		"2023-02-29T00:00:00Z", "2100-02-29T00:00:00Z", "1900-02-29T00:00:00Z", "1970-04-31T00:00:00Z",
		"1970-01-01T24:00:00Z", "1970-01-01T00:60:00Z", "1970-01-01T00:00:60Z", "1970-01-01T00:00:00.Z",
		"1970-01-01T00:00:00+24:00", "1970-01-01T00:00:00+05:60", "1970-01-01T00:00:00+0500",
		"1970-01-01T00:00:00Z ", "x970-01-01T00:00:00Z", "1970-01-01T00:00:00+05", "1970-01-01T00:00:00ZZ"}) {
		CHECK(!scg::json::parse_rfc3339(bad, ns));
	}
}

void test_timestamp()
{
	CHECK(scg::json::encode(at(0)) == "1970-01-01T00:00:00Z");
	CHECK(scg::json::encode(at(-1)) == "1969-12-31T23:59:59.999999999Z");

	// Go's zero time.Time maps to the minimum timestamp and back
	auto min = at((std::numeric_limits<int64_t>::min)());
	CHECK(scg::json::encode(min) == "0001-01-01T00:00:00Z");
	scg::type::timestamp ts = at(5);
	scg::json::decode(nlohmann::json("0001-01-01T00:00:00Z"), ts);
	CHECK(nanos(ts) == (std::numeric_limits<int64_t>::min)());

	scg::json::decode(nlohmann::json("2023-11-14T17:13:20.123456789-05:00"), ts);
	CHECK(nanos(ts) == 1700000000123456789);
	CHECK_THROWS(scg::json::decode(nlohmann::json("not a time"), ts));
	CHECK_THROWS(scg::json::decode(nlohmann::json(12), ts));
	// instants outside the int64 range saturate, as the binary decoder does:
	// earlier ones to the minimum (Go's zero time), later ones to the maximum
	scg::json::decode(nlohmann::json("1500-01-01T00:00:00Z"), ts);
	CHECK(nanos(ts) == (std::numeric_limits<int64_t>::min)());
	CHECK(scg::json::encode(ts) == "0001-01-01T00:00:00Z");
	scg::json::decode(nlohmann::json("9999-12-31T23:59:59Z"), ts);
	CHECK(nanos(ts) == (std::numeric_limits<int64_t>::max)());
	CHECK(scg::json::encode(ts) == "2262-04-11T23:47:16.854775807Z");
	scg::json::decode(nlohmann::json("0000-01-01T00:00:00Z"), ts);
	CHECK(nanos(ts) == (std::numeric_limits<int64_t>::min)());
	// malformed text still throws
	CHECK_THROWS(scg::json::decode(nlohmann::json("9999-12-31T23:59:60Z"), ts));
}

void test_negative_zero()
{
	// Go writes negative zero as the token -0
	double d = 1;
	scg::json::decode(scg::json::parse("-0"), d);
	CHECK(d == 0 && std::signbit(d));
	float f = 1;
	scg::json::decode(scg::json::parse("-0"), f);
	CHECK(f == 0 && std::signbit(f));
	scg::json::decode(scg::json::parse("0"), f);
	CHECK(f == 0 && !std::signbit(f));
	scg::json::decode(scg::json::parse("-0.0"), d);
	CHECK(d == 0 && std::signbit(d));
	// -0 nested in containers, and as a signed / unsigned integer
	std::vector<double> ds;
	scg::json::decode(scg::json::parse("[-0, 0]"), ds);
	CHECK(ds.size() == 2 && std::signbit(ds[0]) && !std::signbit(ds[1]));
	int32_t i32 = 5;
	scg::json::decode(scg::json::parse("-0"), i32);
	CHECK(i32 == 0);
	uint32_t u32 = 5;
	CHECK_THROWS(scg::json::decode(scg::json::parse("-0"), u32));
	// an integer 0 assigned in code (stored as signed by nlohmann) is plain 0
	scg::json::decode(nlohmann::json(0), f);
	CHECK(f == 0 && !std::signbit(f));
	scg::json::decode(nlohmann::json(int64_t(0)), u32);
	CHECK(u32 == 0);
	scg::json::decode(nlohmann::json(7), u32);
	CHECK(u32 == 7);
	CHECK_THROWS(scg::json::decode(nlohmann::json(-7), u32));
	std::map<double, int> keys;
	scg::json::decode(nlohmann::json::parse("{\"-0\":1}"), keys);
	CHECK(keys.size() == 1 && std::signbit(keys.begin()->first));
	// and it round-trips
	nlohmann::json j = scg::json::encode(-0.0f);
	scg::json::decode(j, f);
	CHECK(f == 0 && std::signbit(f));
}

void test_float32_limits()
{
	float f = 0;
	// Go's strconv.ParseFloat(s, 32) rounds values below the midpoint between
	// FLT_MAX and 2^128 to FLT_MAX
	scg::json::decode(nlohmann::json::parse("3.4028235677973366e38"), f);
	CHECK(f == (std::numeric_limits<float>::max)());
	scg::json::decode(nlohmann::json::parse("-3.4028235677973366e38"), f);
	CHECK(f == -(std::numeric_limits<float>::max)());
	scg::json::decode(nlohmann::json::parse("3.4028235e38"), f);
	CHECK(f == (std::numeric_limits<float>::max)());
	CHECK_THROWS(scg::json::decode(nlohmann::json::parse("3.402823567797337e38"), f));
	CHECK_THROWS(scg::json::decode(nlohmann::json::parse("1e39"), f));
}

void test_utf8_sanitizing()
{
	// invalid bytes become U+FFFD one byte at a time, exactly as Go's
	// encoding/json; valid sequences are copied
	const std::string fffd = "\xEF\xBF\xBD";
	const std::pair<std::string, std::string> cases[] = {
		{"plain", "plain"},
		{"h\xC3\xA9llo \xF0\x9F\x98\x80", "h\xC3\xA9llo \xF0\x9F\x98\x80"},
		{"\xEF\xBF\xBD", fffd},                                  // a real U+FFFD
		{"\x80", fffd},                                           // lone continuation
		{"\xC0\x80", fffd + fffd},                               // overlong NUL
		{"\xC1\xBF", fffd + fffd},                               // overlong
		{"\xE0\x80\x80", fffd + fffd + fffd},                   // overlong 3-byte
		{"\xED\xA0\x80", fffd + fffd + fffd},                   // surrogate U+D800
		{"\xED\x9F\xBF", "\xED\x9F\xBF"},                    // U+D7FF is valid
		{"\xF4\x90\x80\x80", fffd + fffd + fffd + fffd},       // above U+10FFFF
		{"\xF4\x8F\xBF\xBF", "\xF4\x8F\xBF\xBF"},          // U+10FFFF is valid
		{"\xF5\x80", fffd + fffd},
		{"\xE2\x82", fffd + fffd},                               // truncated
		{"a\xE2\x82\xAC" "b\xE2", std::string("a\xE2\x82\xAC" "b") + fffd}, // valid euro, truncated tail
		{"\xF0\x9F\x98", fffd + fffd + fffd},
		{"\xFF\xFE", fffd + fffd},
	};
	for (const auto& c : cases) {
		CHECK(scg::json::sanitize_utf8(c.first) == c.second);
		// values and map keys are sanitized on encode, so dump never throws
		CHECK(scg::json::encode(c.first).get<std::string>() == c.second);
		std::map<std::string, int> m{{c.first, 1}};
		CHECK(scg::json::encode(m).begin().key() == c.second);
		CHECK(!scg::json::encode(MyString(c.first)).dump().empty());
	}
}

void test_null_elements()
{
	// null list elements and map values decode to the Go zero value
	std::vector<scg::type::timestamp> stamps;
	scg::json::decode(nlohmann::json::parse("[null, \"1970-01-01T00:00:00Z\"]"), stamps);
	CHECK(stamps.size() == 2 && nanos(stamps[0]) == (std::numeric_limits<int64_t>::min)() && nanos(stamps[1]) == 0);
	std::map<std::string, scg::type::timestamp> byName;
	scg::json::decode(nlohmann::json::parse("{\"a\": null}"), byName);
	CHECK(byName.size() == 1 && nanos(byName["a"]) == (std::numeric_limits<int64_t>::min)());
	std::vector<int32_t> ints;
	scg::json::decode(nlohmann::json::parse("[null, 5]"), ints);
	CHECK((ints == std::vector<int32_t>{0, 5}));
	std::vector<std::string> strs;
	scg::json::decode(nlohmann::json::parse("[null]"), strs);
	CHECK(strs.size() == 1 && strs[0].empty());
	std::vector<std::vector<uint8_t>> lists;
	scg::json::decode(nlohmann::json::parse("[null, \"AQ==\"]"), lists);
	CHECK(lists.size() == 2 && lists[0].empty() && lists[1].size() == 1);
	CHECK(nanos(scg::json::zero_value<scg::type::timestamp>()) == (std::numeric_limits<int64_t>::min)());
}

void test_legacy_uuid_arrays()
{
	// the array of 16 integers earlier Go code wrote for uuid typedefs
	auto id = scg::type::uuid::fromString("6ba7b810-9dad-11d1-80b4-00c04fd430c8").first;
	const char* legacy = "[107,167,184,16,157,173,17,209,128,180,0,192,79,212,48,200]";
	scg::type::uuid u;
	scg::json::decode(nlohmann::json::parse(legacy), u);
	CHECK(u == id);
	MyID typed;
	scg::json::decode(nlohmann::json::parse(legacy), typed);
	CHECK(static_cast<const scg::type::uuid&>(typed) == id);
	// Go array semantics: short arrays are zero-filled, extra elements ignored
	scg::json::decode(nlohmann::json::parse("[1,2]"), u);
	CHECK(u.toString() == "01020000-0000-0000-0000-000000000000");
	scg::json::decode(nlohmann::json::parse("[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,\"extra\"]"), u);
	CHECK(u.toString() == "01020304-0506-0708-090a-0b0c0d0e0f10");
	CHECK_THROWS(scg::json::decode(nlohmann::json::parse("[256]"), u));
	CHECK_THROWS(scg::json::decode(nlohmann::json::parse("[-1]"), u));
}

void test_enums_are_unsigned()
{
	// enums are uint16 in Go: -0 and signed key forms are rejected
	Color c = Color::RED;
	CHECK_THROWS(scg::json::decode(scg::json::parse("-0"), c));
	CHECK_THROWS(scg::json::decode(nlohmann::json::parse("-1"), c));
	// encoded unsigned, so an in-memory round trip works
	nlohmann::json je = scg::json::encode(Color::BLUE);
	CHECK(je.is_number_unsigned());
	c = Color::RED;
	scg::json::decode(je, c);
	CHECK(c == Color::BLUE);
	scg::json::decode(nlohmann::json::parse("65535"), c);
	CHECK(static_cast<int>(c) == 65535);
	std::map<Color, int> m;
	CHECK_THROWS(scg::json::decode(nlohmann::json::parse("{\"+1\":1}"), m));
	CHECK_THROWS(scg::json::decode(nlohmann::json::parse("{\"-0\":1}"), m));
	CHECK_THROWS(scg::json::decode(nlohmann::json::parse("{\"65536\":1}"), m));
	scg::json::decode(nlohmann::json::parse("{\"2\":1}"), m);
	CHECK(m.size() == 1 && m.begin()->first == Color::BLUE);
}

void test_numbers()
{
	CHECK(scg::json::encode(uint8_t(255)).dump() == "255");
	CHECK(scg::json::encode(int8_t(-128)).dump() == "-128");
	CHECK(scg::json::encode((std::numeric_limits<uint64_t>::max)()).dump() == "18446744073709551615");
	CHECK(scg::json::encode((std::numeric_limits<int64_t>::min)()).dump() == "-9223372036854775808");
	CHECK(scg::json::encode(1.1f).dump() == "1.1");
	CHECK(scg::json::encode(-0.5f).dump() == "-0.5");
	CHECK(scg::json::encode((std::numeric_limits<float>::max)()).dump() == "3.4028235e+38");
	CHECK(scg::json::encode(0.1).dump() == "0.1");
	CHECK_THROWS(scg::json::encode(std::numeric_limits<double>::quiet_NaN()));
	CHECK_THROWS(scg::json::encode(std::numeric_limits<float>::infinity()));

	uint8_t u8 = 0;
	scg::json::decode(nlohmann::json::parse("255"), u8);
	CHECK(u8 == 255);
	CHECK_THROWS(scg::json::decode(nlohmann::json::parse("256"), u8));
	CHECK_THROWS(scg::json::decode(nlohmann::json::parse("-1"), u8));
	CHECK_THROWS(scg::json::decode(nlohmann::json::parse("1.5"), u8));
	CHECK_THROWS(scg::json::decode(nlohmann::json::parse("\"1\""), u8));
	int64_t i64 = 0;
	scg::json::decode(nlohmann::json::parse("-9223372036854775808"), i64);
	CHECK(i64 == (std::numeric_limits<int64_t>::min)());
	CHECK_THROWS(scg::json::decode(nlohmann::json::parse("9223372036854775808"), i64));
	uint64_t u64 = 0;
	scg::json::decode(nlohmann::json::parse("18446744073709551615"), u64);
	CHECK(u64 == (std::numeric_limits<uint64_t>::max)());
	float f = 0;
	scg::json::decode(nlohmann::json::parse("3.4028235e+38"), f);
	CHECK(f == (std::numeric_limits<float>::max)());
	CHECK_THROWS(scg::json::decode(nlohmann::json::parse("3.5e+38"), f));
	scg::json::decode(nlohmann::json::parse("1"), f);
	CHECK(f == 1.0f);

	Color c = Color::RED;
	CHECK(scg::json::encode(Color::BLUE).dump() == "2");
	scg::json::decode(nlohmann::json::parse("2"), c);
	CHECK(c == Color::BLUE);
	CHECK_THROWS(scg::json::decode(nlohmann::json::parse("65536"), c));
}

void test_typedefs()
{
	// typedefs are encoded as their underlying type
	CHECK(scg::json::encode(MyU8(65)).dump() == "65");
	CHECK(scg::json::encode(MyI8(-1)).dump() == "-1");
	CHECK(scg::json::encode(MyU64((std::numeric_limits<uint64_t>::max)())).dump() == "18446744073709551615");
	CHECK(scg::json::encode(MyF32(1.1f)).dump() == "1.1");
	CHECK(scg::json::encode(MyString("s")).dump() == "\"s\"");
	auto id = scg::type::uuid::fromString("6BA7B810-9DAD-11D1-80B4-00C04FD430C8").first;
	CHECK(scg::json::encode(MyID(id)).dump() == "\"6ba7b810-9dad-11d1-80b4-00c04fd430c8\"");

	MyI64 i(0);
	scg::json::decode(nlohmann::json::parse("-5"), i);
	CHECK(static_cast<int64_t>(i) == -5);

	// the existing ADL conversions are unchanged: typedefs as strings
	nlohmann::json legacy = MyI64(42);
	CHECK(legacy.dump() == "\"42\"");
	nlohmann::json legacyU8 = MyU8(65);
	CHECK(legacyU8.dump() == "\"A\"");
}

void test_containers()
{
	// byte lists (and typedef-of-uint8 lists) are base64; int8 lists are arrays
	CHECK(scg::json::encode(std::vector<uint8_t>{1, 2, 3}).dump() == "\"AQID\"");
	CHECK(scg::json::encode(std::vector<MyU8>{MyU8(1), MyU8(2), MyU8(3)}).dump() == "\"AQID\"");
	CHECK(scg::json::encode(std::vector<uint8_t>{}).dump() == "\"\"");
	CHECK(scg::json::encode(std::vector<int8_t>{-1, 2}).dump() == "[-1,2]");
	CHECK(scg::json::encode(std::vector<std::vector<uint8_t>>{{}, {255}}).dump() == "[\"\",\"/w==\"]");
	CHECK(scg::json::encode(std::vector<bool>{true, false}).dump() == "[true,false]");

	std::vector<uint8_t> bs{9};
	scg::json::decode(nlohmann::json("AQID"), bs);
	CHECK((bs == std::vector<uint8_t>{1, 2, 3}));
	scg::json::decode(nlohmann::json::parse("[4,5]"), bs);
	CHECK((bs == std::vector<uint8_t>{4, 5}));
	scg::json::decode(nlohmann::json(nullptr), bs);
	CHECK(bs.empty());
	CHECK_THROWS(scg::json::decode(nlohmann::json("not base64!"), bs));
	CHECK_THROWS(scg::json::decode(nlohmann::json::parse("[256]"), bs));

	// map keys
	std::map<int32_t, std::string> ints{{-1, "a"}, {2, "b"}};
	CHECK(scg::json::encode(ints).dump() == "{\"-1\":\"a\",\"2\":\"b\"}");
	std::map<Color, int> colors{{Color::BLUE, 1}};
	CHECK(scg::json::encode(colors).dump() == "{\"2\":1}");
	std::map<MyString, int> strs{{MyString("k"), 1}};
	CHECK(scg::json::encode(strs).dump() == "{\"k\":1}");
	std::map<MyU64, int> u64s{{MyU64((std::numeric_limits<uint64_t>::max)()), 1}};
	CHECK(scg::json::encode(u64s).dump() == "{\"18446744073709551615\":1}");
	auto id = scg::type::uuid::fromString("6ba7b810-9dad-11d1-80b4-00c04fd430c8").first;
	std::map<MyID, int> ids{{MyID(id), 1}};
	CHECK(scg::json::encode(ids).dump() == "{\"6ba7b810-9dad-11d1-80b4-00c04fd430c8\":1}");

	std::map<int8_t, int> decoded;
	scg::json::decode(nlohmann::json::parse("{\"-128\":1,\"+5\":2}"), decoded);
	CHECK(decoded.size() == 2 && decoded[-128] == 1 && decoded[5] == 2);
	CHECK_THROWS(scg::json::decode(nlohmann::json::parse("{\"128\":1}"), decoded));
	CHECK_THROWS(scg::json::decode(nlohmann::json::parse("{\"x\":1}"), decoded));
	std::unordered_map<MyID, int> decodedIDs;
	scg::json::decode(nlohmann::json::parse("{\"6ba7b810-9dad-11d1-80b4-00c04fd430c8\":3}"), decodedIDs);
	CHECK(decodedIDs.size() == 1 && decodedIDs[MyID(id)] == 3);
	scg::json::decode(nlohmann::json(nullptr), decoded);
	CHECK(decoded.empty());

	// null leaves non-container values unchanged, as Go does
	std::string s = "keep";
	scg::json::decode(nlohmann::json(nullptr), s);
	CHECK(s == "keep");
}

} // namespace

int main()
{
	test_base64();
	test_rfc3339_format();
	test_rfc3339_parse();
	test_timestamp();
	test_negative_zero();
	test_float32_limits();
	test_utf8_sanitizing();
	test_null_elements();
	test_legacy_uuid_arrays();
	test_enums_are_unsigned();
	test_numbers();
	test_typedefs();
	test_containers();
	if (failures > 0) {
		std::printf("%d check(s) failed\n", failures);
		return 1;
	}
	std::printf("all scg::json checks passed\n");
	return 0;
}
