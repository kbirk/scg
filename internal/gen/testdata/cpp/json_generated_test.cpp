// Tests of generated message JSON behaviour, compiled against
// testdata/cpp/schema generated into a temporary directory by
// internal/gen/harness_test.go (TestCppGeneratedJSON).

#include "jg/jg.h"

#include <cmath>
#include <cstdio>
#include <string>
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

template <typename T>
bool fromJSONThrows(T& msg, const std::string& text)
{
	try {
		msg.fromJSON(std::vector<uint8_t>(text.begin(), text.end()));
	} catch (const nlohmann::json::exception&) {
		return true;
	}
	return false;
}

std::string toJSON(const jg::Fields& f)
{
	auto bs = f.toJSON();
	return std::string(bs.begin(), bs.end());
}

} // namespace

int main()
{
	jg::Fields f;
	f.a = 1;
	f.b = "keep";

	// absent keys leave their field unchanged (JSON from an older schema)
	CHECK(!fromJSONThrows(f, "{\"a\": 5}"));
	CHECK(f.a == 5 && f.b == "keep");
	// unknown keys are ignored
	CHECK(!fromJSONThrows(f, "{\"b\": \"x\", \"added_later\": [1, 2]}"));
	CHECK(f.a == 5 && f.b == "x");
	// null leaves the message unchanged
	CHECK(!fromJSONThrows(f, "null"));
	CHECK(f.a == 5 && f.b == "x");
	// anything else that is not an object is rejected, as Go does
	CHECK(fromJSONThrows(f, "[]"));
	CHECK(fromJSONThrows(f, "5"));
	CHECK(fromJSONThrows(f, "\"x\""));
	CHECK(fromJSONThrows(f, "{\"empty\": []}"));
	// a null list element is the Go zero time
	CHECK(!fromJSONThrows(f, "{\"stamps\": [null]}"));
	CHECK(f.stamps.size() == 1 && toJSON(f).find("\"stamps\":[\"0001-01-01T00:00:00Z\"]") != std::string::npos);

	// empty messages require an object (or null) too
	jg::Empty e;
	CHECK(!fromJSONThrows(e, "{}"));
	CHECK(!fromJSONThrows(e, "{\"unknown\": 1}"));
	CHECK(!fromJSONThrows(e, "null"));
	CHECK(fromJSONThrows(e, "[]"));
	CHECK(fromJSONThrows(e, "1"));
	CHECK(fromJSONThrows(e, "\"x\""));
	CHECK(fromJSONThrows(e, "not json"));
	auto bs = e.toJSON();
	CHECK(std::string(bs.begin(), bs.end()) == "{}");

	// an in-memory nlohmann round trip (no text) keeps every field, including
	// enums and unsigned integers
	{
		jg::Fields src;
		src.a = -3;
		src.kind = jg::Kind::B;
		src.count = 7;
		src.ratio = 0.5f;
		nlohmann::json j = src;
		jg::Fields dst = j.get<jg::Fields>();
		CHECK(dst.a == -3 && dst.kind == jg::Kind::B && dst.count == 7 && dst.ratio == 0.5f);
	}

	// JSON built in code stores C++ int literals as signed integers
	{
		nlohmann::json j = nlohmann::json::object();
		j["count"] = 5;
		j["kind"] = 1;
		j["ratio"] = 0;
		jg::Fields dst;
		dst.ratio = 1;
		j.get_to(dst);
		CHECK(dst.count == 5 && dst.kind == jg::Kind::B && dst.ratio == 0 && !std::signbit(dst.ratio));
		j["count"] = -1;
		bool threw = false;
		try {
			j.get_to(dst);
		} catch (const nlohmann::json::exception&) {
			threw = true;
		}
		CHECK(threw);
	}

	// Go's -0 token: negative zero for floats, 0 for signed integers, and an
	// error for unsigned integers and enums (as Go's strconv)
	CHECK(!fromJSONThrows(f, "{\"ratio\": -0, \"a\": -0}"));
	CHECK(f.ratio == 0 && std::signbit(f.ratio) && f.a == 0);
	CHECK(fromJSONThrows(f, "{\"count\": -0}"));
	CHECK(fromJSONThrows(f, "{\"kind\": -0}"));

	if (failures > 0) {
		std::printf("%d check(s) failed\n", failures);
		return 1;
	}
	std::printf("all generated JSON checks passed\n");
	return 0;
}
