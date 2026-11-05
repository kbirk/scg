// C++ half of the cross-language wire compatibility check, see ../main.go
// (binary) and ../json.go (JSON).
//
//   wire_check <go_cases_file> <cpp_cases_out_file>
//   wire_check json-from-go <go_json> <cpp_roundtrip_out> <cpp_native_out>
//   wire_check json-from-roundtrip <go_roundtrip_json>
//
// For every case it decodes the Go bytes, checks the decoded value against the
// expected value (built below, identical to main.go) by comparing their C++
// encodings, checks that the C++ and Go encodings are byte-identical for
// deterministic cases, and writes the C++ encodings for main.go to verify.

#include "wire/wire.h"

#include <chrono>
#include <cstdio>
#include <fstream>
#include <functional>
#include <limits>
#include <map>
#include <sstream>
#include <string>
#include <vector>

using namespace wire;

namespace {

using TimePoint = std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds>;

scg::type::timestamp ts(int64_t sec, int64_t nsec)
{
	return scg::type::timestamp(TimePoint(std::chrono::seconds(sec) + std::chrono::nanoseconds(nsec)));
}

scg::type::uuid id(const std::string& s)
{
	auto res = scg::type::uuid::fromString(s);
	if (res.second) {
		fprintf(stderr, "bad uuid literal %s\n", s.c_str());
		std::exit(2);
	}
	return res.first;
}

std::string toHex(const std::vector<uint8_t>& bs)
{
	static const char* digits = "0123456789abcdef";
	std::string res;
	for (auto b : bs) {
		res += digits[b >> 4];
		res += digits[b & 0xF];
	}
	return res;
}

std::vector<uint8_t> fromHex(const std::string& s)
{
	std::vector<uint8_t> res;
	for (size_t i = 0; i + 1 < s.size(); i += 2) {
		res.push_back(uint8_t(std::stoul(s.substr(i, 2), nullptr, 16)));
	}
	return res;
}

struct Case {
	std::string name;
	bool deterministic;
	std::vector<uint8_t> expected;
	// decodes bytes and returns the C++ re-encoding of the decoded value
	std::function<std::pair<std::vector<uint8_t>, scg::error::Error>(const std::vector<uint8_t>&)> roundTrip;
};

template <typename T>
Case makeCase(const std::string& name, bool deterministic, const T& value)
{
	Case c;
	c.name = name;
	c.deterministic = deterministic;
	c.expected = value.toBytes();
	c.roundTrip = [](const std::vector<uint8_t>& bs) {
		T decoded;
		auto err = decoded.fromBytes(bs);
		if (err) {
			return std::make_pair(std::vector<uint8_t>(), err);
		}
		return std::make_pair(decoded.toBytes(), scg::error::Error());
	};
	return c;
}

// The values below must match main.go exactly.
std::vector<Case> cases()
{
	std::vector<Case> res;

	{
		Scalars v;
		v.b0 = true;
		v.by = 0xAB;
		v.u8 = 200;
		v.u16 = 60000;
		v.u32 = 4000000000u;
		v.u64 = 18446744073709551615ull;
		v.i8 = -5;
		v.i16 = -300;
		v.i32 = 123456;
		v.i64 = -9000000000ll;
		v.f32 = 1.5f;
		v.f64 = -2.25;
		v.s = "h\xc3\xa9llo w\xc3\xb6rld";
		v.color = Color::BLUE;
		v.label = Label("label");
		v.count = Count(77);
		v.ident = Ident(id("123e4567-e89b-42d3-a456-426614174000"));
		v.id = id("12345678-1234-4678-9234-567812345678");
		v.ts = ts(1700000000, 123456789);
		v.b1 = true;
		res.push_back(makeCase("scalars", true, v));
	}
	{
		Scalars v;
		v.ts = ts(0, 0);
		res.push_back(makeCase("scalars_zero", true, v));
	}
	{
		Containers v;
		Inner a;
		a.flag = true;
		a.bits = {true, false, true};
		a.note = "n";
		v.inners = {a, Inner{}};
		v.byteLists = {{1, 2, 3}, {}};
		v.bytes = {0, 1, 2, 254, 255};
		v.emptyList = {};
		v.byColor[Color::GREEN] = Count(9);
		v.nested["k"][-1] = {Color::RED, Color::BLUE};
		v.listOfMaps = {{{"a", 1}}, {}};
		v.byID[id("00000000-0000-4000-8000-000000000001")] = {0.5, -1.0};
		v.empties["e"] = Empty{};
		v.stamps = {ts(1, 2), ts(1700000000, 999999999)};
		res.push_back(makeCase("containers", true, v));
	}
	{
		Containers v;
		v.byColor[Color::RED] = Count(1);
		v.byColor[Color::GREEN] = Count(2);
		v.byColor[Color::BLUE] = Count(3);
		v.nested["a"][1] = {Color::RED};
		v.nested["a"][2] = {Color::GREEN, Color::BLUE};
		v.nested["b"] = {};
		v.empties["x"] = Empty{};
		v.empties["y"] = Empty{};
		v.empties["z"] = Empty{};
		res.push_back(makeCase("multi_entry_maps", false, v));
	}
	{
		UUIDs v;
		v.ids = {
			id("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
			id("6fa459ea-ee8a-3ca4-894e-db77e160355e"),
			id("886313e1-3b8a-5372-9b90-0c9aee199e5d"),
			id("018f3c5e-7b2a-7cde-8f00-123456789abc"),
			id("00000000-0000-0000-0000-000000000000"),
			id("12345678-1234-5678-1234-567812345678"),
		};
		v.typed = Ident(id("6ba7b811-9dad-11d1-80b4-00c04fd430c8"));
		res.push_back(makeCase("uuid_versions", true, v));
	}
	{
		Times v;
		v.stamps = {ts(-100, 5), ts(-1, 999999999), ts(-1, 0), ts(0, 0), ts(-2208988800ll, 0), ts(1, 1)};
		res.push_back(makeCase("pre_epoch_timestamps", true, v));
	}
	{
		// Go's zero time.Time decodes to the minimum timestamp, which must
		// encode back to the zero time
		Times v;
		v.stamps = {scg::type::timestamp(TimePoint(std::chrono::nanoseconds((std::numeric_limits<int64_t>::min)()))), ts(0, 0)};
		res.push_back(makeCase("go_zero_time", true, v));
	}

	return res;
}

} // namespace


std::string abbreviate(const std::string& s)
{
	return s.size() > 400 ? s.substr(0, 400) + "..." : s;
}

std::map<std::string, std::vector<uint8_t>> readCases(const char* path)
{
	std::map<std::string, std::vector<uint8_t>> res;
	std::ifstream in(path);
	std::string line;
	while (std::getline(in, line)) {
		std::istringstream ss(line);
		std::string name, hex;
		ss >> name >> hex;
		if (!name.empty()) {
			res[name] = fromHex(hex);
		}
	}
	return res;
}

// ---------------------------------------------------------------------------
// JSON cases (see json.go)
// ---------------------------------------------------------------------------

struct JsonCase {
	std::string name;
	// binary encoding of the expected value: C++ binary encodings are
	// deterministic, so equal bytes means equal values
	std::vector<uint8_t> expectedBytes;
	std::string expectedJSON;
	// decodes JSON text, returning the binary and JSON re-encodings
	std::function<std::pair<std::vector<uint8_t>, std::string>(const std::string&)> fromJSON;
};

template <typename T>
JsonCase makeJsonCase(const std::string& name, const T& value)
{
	JsonCase c;
	c.name = name;
	c.expectedBytes = value.toBytes();
	auto json = value.toJSON();
	c.expectedJSON = std::string(json.begin(), json.end());
	c.fromJSON = [](const std::string& text) {
		T decoded;
		decoded.fromJSON(std::vector<uint8_t>(text.begin(), text.end()));
		auto json = decoded.toJSON();
		return std::make_pair(decoded.toBytes(), std::string(json.begin(), json.end()));
	};
	return c;
}

// makeNormalizedJsonCase expects the value C++'s own JSON round trip produces
// (e.g. with invalid UTF-8 replaced by U+FFFD), for comparison with the value
// decoded from Go's JSON.
template <typename T>
JsonCase makeNormalizedJsonCase(const std::string& name, const T& value)
{
	JsonCase c = makeJsonCase(name, value);
	c.expectedBytes = c.fromJSON(c.expectedJSON).first;
	return c;
}

scg::type::timestamp maxTime()
{
	return scg::type::timestamp(TimePoint(std::chrono::nanoseconds((std::numeric_limits<int64_t>::max)())));
}

// the C++ representation of Go's zero time.Time
scg::type::timestamp goZeroTime()
{
	return scg::type::timestamp(TimePoint(std::chrono::nanoseconds((std::numeric_limits<int64_t>::min)())));
}

std::vector<uint8_t> jsonByteList(size_t n)
{
	std::vector<uint8_t> bs(n);
	for (size_t i = 0; i < n; ++i) {
		bs[i] = static_cast<uint8_t>(i * 7);
	}
	return bs;
}

JsonScalars zeroScalars()
{
	JsonScalars v;
	v.ts = goZeroTime();
	return v;
}

// The values below must match json.go exactly.
std::vector<JsonCase> jsonCases()
{
	std::vector<JsonCase> res;

	auto uid = id("123e4567-e89b-42d3-a456-426614174000");
	auto nonV4 = id("6ba7b810-9dad-11d1-80b4-00c04fd430c8");

	JsonTypedefs typedefs;
	typedefs.tu8 = Tu8(200);
	typedefs.tu16 = Tu16(65535);
	typedefs.count = Count(7);
	typedefs.tu64 = Tu64((std::numeric_limits<uint64_t>::max)());
	typedefs.ti8 = Ti8(-128);
	typedefs.ti16 = Ti16(-2);
	typedefs.ti32 = Ti32((std::numeric_limits<int32_t>::min)());
	typedefs.ti64 = Ti64((std::numeric_limits<int64_t>::min)());
	typedefs.tf32 = Tf32(-1.5f);
	typedefs.tf64 = Tf64(1e300);
	typedefs.label = Label("lbl");
	typedefs.ident = Ident(nonV4);

	JsonScalars maxScalars;
	maxScalars.b = true;
	maxScalars.by = 255;
	maxScalars.u8 = 255;
	maxScalars.u16 = 65535;
	maxScalars.u32 = (std::numeric_limits<uint32_t>::max)();
	maxScalars.u64 = (std::numeric_limits<uint64_t>::max)();
	maxScalars.i8 = 127;
	maxScalars.i16 = 32767;
	maxScalars.i32 = (std::numeric_limits<int32_t>::max)();
	maxScalars.i64 = (std::numeric_limits<int64_t>::max)();
	maxScalars.f32 = (std::numeric_limits<float>::max)();
	maxScalars.f64 = (std::numeric_limits<double>::max)();
	maxScalars.s = "h\xc3\xa9llo \"quoted\" \\ back\nslash\ttab \x01 <>& \xe4\xb8\x96\xe7\x95\x8c \xf0\x9f\x98\x80 \xe2\x80\xa8";
	maxScalars.id = uid;
	maxScalars.ts = ts(1700000000, 123456789); // 2023-11-14T17:13:20.123456789-05:00
	maxScalars.color = Color::BLUE;
	res.push_back(makeJsonCase("json_scalars_max", maxScalars));

	{
		JsonScalars v;
		v.b = false;
		v.i8 = -128;
		v.i16 = -32768;
		v.i32 = (std::numeric_limits<int32_t>::min)();
		v.i64 = (std::numeric_limits<int64_t>::min)();
		v.f32 = -std::numeric_limits<float>::denorm_min();
		v.f64 = std::numeric_limits<double>::denorm_min();
		v.s = "";
		v.id = scg::type::uuid();
		v.ts = ts(-100, 5);
		v.color = Color::RED;
		res.push_back(makeJsonCase("json_scalars_min", v));
	}
	{
		JsonScalars v;
		v.u16 = 1;
		v.u32 = 1;
		v.u64 = 1;
		v.i16 = -1;
		v.i32 = -1;
		v.i64 = -1;
		v.f32 = 1.1f;
		v.f64 = 0.1;
		v.s = "plain";
		v.id = nonV4;
		v.ts = ts(1, 1);
		v.color = Color::GREEN;
		res.push_back(makeJsonCase("json_scalars_mixed", v));
	}
	res.push_back(makeJsonCase("json_go_zero_scalars", zeroScalars()));
	res.push_back(makeJsonCase("json_typedefs", typedefs));
	{
		JsonContainers v;
		v.bytes = jsonByteList(5000);
		v.u8s = {0, 255};
		v.tu8s = {Tu8(1), Tu8(2), Tu8(3)};
		v.i8s = {-128, 0, 127};
		v.byteLists = {{}, {1, 2, 3}, jsonByteList(64)};
		Inner a;
		a.flag = true;
		a.bits = {true, false};
		a.note = "n";
		v.inners = {a, Inner{}};
		v.stamps = {ts(-1, 999999999), ts(0, 0), ts(1700000000, 100000000)};
		v.ids = {uid, scg::type::uuid()};
		v.idents = {Ident(nonV4)};
		v.colors = {Color::RED, Color::BLUE};
		v.f32s = {1.1f, -0.5f, (std::numeric_limits<float>::max)()};
		v.strings = {"", "a"};
		Inner x;
		x.note = "x";
		v.byString["k"] = {x};
		v.byString["empty"] = {};
		v.byInt[-1]["a"] = (std::numeric_limits<int64_t>::min)();
		v.byInt[2] = {};
		v.byColor[Color::GREEN] = Label("g");
		v.byColor[Color::BLUE] = Label("b");
		v.byUUID[uid] = {ts(5, 6)};
		v.byLabel[Label("l")] = Count(3);
		v.byCount[Count((std::numeric_limits<uint32_t>::max)())] = "max";
		v.byCount[Count(0)] = "zero";
		Inner flagged;
		flagged.flag = true;
		v.byIdent[Ident(nonV4)] = flagged;
		v.byTi8[Ti8(-128)] = Tu8(255);
		v.byTi8[Ti8(127)] = Tu8(0);
		v.byU64[(std::numeric_limits<uint64_t>::max)()] = true;
		v.byU64[0] = false;
		v.byI64[(std::numeric_limits<int64_t>::min)()] = typedefs;
		v.byU8[255] = {1};
		v.byU8[0] = {};
		v.inner.note = "inner";
		v.scalars = maxScalars;
		res.push_back(makeJsonCase("json_containers", v));
	}
	{
		JsonContainers v;
		v.scalars = zeroScalars();
		res.push_back(makeJsonCase("json_nil_containers", v));
	}
	{
		JsonContainers v;
		v.scalars = zeroScalars();
		v.scalars.ts = ts(0, 0);
		res.push_back(makeJsonCase("json_empty_containers", v));
	}
	{
		JsonScalars v = zeroScalars();
		v.f32 = -0.0f;
		v.f64 = -0.0;
		v.s = "z";
		v.ts = ts(0, 0);
		res.push_back(makeJsonCase("json_negative_zero", v));
	}
	{
		JsonContainers v;
		v.byteLists = {{}, {1}};
		v.byString["nil"] = {};
		v.byUUID[uid] = {};
		v.scalars = zeroScalars();
		v.scalars.ts = ts(0, 0);
		res.push_back(makeJsonCase("json_null_elements", v));
	}
	{
		JsonScalars v = zeroScalars();
		v.b = true;
		v.i32 = 7;
		v.ts = ts(10, 0);
		res.push_back(makeJsonCase("json_missing_keys", v));
	}
	{
		JsonTypedefs v;
		v.ident = Ident(nonV4);
		v.label = Label("x");
		res.push_back(makeJsonCase("json_legacy_uuid_typedef", v));
	}
	{
		JsonContainers v;
		v.stamps = {maxTime(), goZeroTime(), ts(0, 0)};
		v.scalars = zeroScalars();
		v.scalars.ts = maxTime();
		res.push_back(makeJsonCase("json_time_saturation", v));
	}
	{
		JsonContainers v;
		v.strings = {"\xff", "a\xE2\x82\xAC" "b\xE2", "\xED\xA0\x80", "\xC0\x80" "ok"};
		v.byLabel[Label("\xC0\x80")] = Count(1);
		v.byLabel[Label("\xF4\x90\x80\x80")] = Count(2);
		v.byString["k\xF5"] = {};
		v.scalars = zeroScalars();
		v.scalars.s = "\xF0\x9F\x98";
		v.scalars.ts = ts(0, 0);
		res.push_back(makeNormalizedJsonCase("json_invalid_utf8", v));
	}
	return res;
}

void writeTextCases(const char* path, const std::vector<std::pair<std::string, std::string>>& texts)
{
	std::ofstream out(path);
	for (const auto& [name, text] : texts) {
		out << name << " " << toHex(std::vector<uint8_t>(text.begin(), text.end())) << "\n";
	}
}

// json-from-go: decode Go's JSON, compare with the expected value, and write
// the C++ re-encoding plus C++'s own encoding of the expected value.
int runJsonFromGo(const char* goFile, const char* roundTripOut, const char* nativeOut)
{
	auto goCases = readCases(goFile);
	std::vector<std::pair<std::string, std::string>> roundTrip, native;
	int failed = 0;
	for (const auto& c : jsonCases()) {
		native.emplace_back(c.name, c.expectedJSON);
		auto it = goCases.find(c.name);
		if (it == goCases.end()) {
			printf("FAIL %s: missing from the Go JSON\n", c.name.c_str());
			failed++;
			continue;
		}
		std::string text(it->second.begin(), it->second.end());
		try {
			auto [bytes, json] = c.fromJSON(text);
			roundTrip.emplace_back(c.name, json);
			if (bytes != c.expectedBytes) {
				printf("FAIL %s: C++ decoded different values from the Go JSON\n  json: %s\n", c.name.c_str(), abbreviate(text).c_str());
				failed++;
				continue;
			}
		} catch (const std::exception& e) {
			printf("FAIL %s: C++ could not decode the Go JSON: %s\n  json: %s\n", c.name.c_str(), e.what(), abbreviate(text).c_str());
			failed++;
			continue;
		}
		printf("PASS %s (C++ decode of Go JSON)\n", c.name.c_str());
	}
	writeTextCases(roundTripOut, roundTrip);
	writeTextCases(nativeOut, native);
	if (failed > 0) {
		printf("%d JSON case(s) failed\n", failed);
		return 1;
	}
	return 0;
}

// json-from-roundtrip: decode Go's re-encoding of C++'s own JSON.
int runJsonFromRoundTrip(const char* goFile)
{
	auto goCases = readCases(goFile);
	int failed = 0;
	for (const auto& c : jsonCases()) {
		auto it = goCases.find(c.name);
		if (it == goCases.end()) {
			printf("FAIL %s: missing from the Go re-encoded JSON\n", c.name.c_str());
			failed++;
			continue;
		}
		std::string text(it->second.begin(), it->second.end());
		try {
			auto [bytes, json] = c.fromJSON(text);
			if (bytes != c.expectedBytes) {
				printf("FAIL %s: C++ decoded different values from Go's re-encoding\n  json: %s\n", c.name.c_str(), abbreviate(text).c_str());
				failed++;
				continue;
			}
		} catch (const std::exception& e) {
			printf("FAIL %s: C++ could not decode Go's re-encoding: %s\n  json: %s\n", c.name.c_str(), e.what(), abbreviate(text).c_str());
			failed++;
			continue;
		}
		printf("PASS %s (C++ decode of Go's re-encoding of C++ JSON)\n", c.name.c_str());
	}
	if (failed > 0) {
		printf("%d JSON case(s) failed\n", failed);
		return 1;
	}
	return 0;
}

int runBinary(const char* goFile, const char* outFile)
{
	auto goCases = readCases(goFile);

	int failed = 0;
	std::ofstream out(outFile);
	for (const auto& c : cases()) {
		out << c.name << " " << toHex(c.expected) << "\n";

		auto it = goCases.find(c.name);
		if (it == goCases.end()) {
			printf("FAIL %s: missing from the Go output\n", c.name.c_str());
			failed++;
			continue;
		}
		const auto& goBytes = it->second;

		auto [reencoded, err] = c.roundTrip(goBytes);
		if (err) {
			printf("FAIL %s: C++ could not decode the Go bytes: %s\n", c.name.c_str(), err.message().c_str());
			failed++;
			continue;
		}
		if (reencoded != c.expected) {
			printf("FAIL %s: C++ decoded different values from the Go bytes\n  got:  %s\n  want: %s\n", c.name.c_str(), toHex(reencoded).c_str(), toHex(c.expected).c_str());
			failed++;
			continue;
		}
		if (c.deterministic && goBytes != c.expected) {
			printf("FAIL %s: Go and C++ encodings differ\n  go:  %s\n  c++: %s\n", c.name.c_str(), toHex(goBytes).c_str(), toHex(c.expected).c_str());
			failed++;
			continue;
		}
		printf("PASS %s (C++ decode of Go bytes)\n", c.name.c_str());
	}

	if (failed > 0) {
		printf("%d wire case(s) failed\n", failed);
		return 1;
	}
	return 0;
}

int main(int argc, char** argv)
{
	if (argc == 5 && std::string(argv[1]) == "json-from-go") {
		return runJsonFromGo(argv[2], argv[3], argv[4]);
	}
	if (argc == 3 && std::string(argv[1]) == "json-from-roundtrip") {
		return runJsonFromRoundTrip(argv[2]);
	}
	if (argc == 3) {
		return runBinary(argv[1], argv[2]);
	}
	fprintf(stderr, "usage: wire_check <go_cases_file> <cpp_cases_out_file>\n"
		"       wire_check json-from-go <go_json> <cpp_roundtrip_out> <cpp_native_out>\n"
		"       wire_check json-from-roundtrip <go_roundtrip_json>\n");
	return 2;
}
