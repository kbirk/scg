// Differential test helper for include/scg/json.h, driven by
// internal/gen/json_diff_test.go, which computes the expected results with
// Go's encoding/json. Each stdin line is `<kind> <hex input>`; each stdout line
// is the result for that input, or ERR when decoding fails.
//
//   str      encode a string (sanitizing invalid UTF-8), then decode it back:
//            hex of the resulting bytes
//   strkey   the same for a map key
//   f32/f64  decode the JSON number text: hex of the IEEE bits
//   enum     decode the JSON text into a (uint16) enum: decimal value
//   enumkey  decode an object key into an enum: decimal value

#include "scg/json.h"

#include <cstdint>
#include <cstdio>
#include <cstring>
#include <iostream>
#include <map>
#include <string>

namespace {

enum class Kind {
	A = 0,
};

std::string toHex(const std::string& s)
{
	static const char* digits = "0123456789abcdef";
	std::string out;
	for (unsigned char c : s) {
		out += digits[c >> 4];
		out += digits[c & 0x0F];
	}
	return out;
}

std::string fromHex(const std::string& s)
{
	std::string out;
	for (size_t i = 0; i + 1 < s.size(); i += 2) {
		out.push_back(static_cast<char>(std::stoul(s.substr(i, 2), nullptr, 16)));
	}
	return out;
}

std::string bitsHex(uint64_t bits, int width)
{
	char buf[32];
	std::snprintf(buf, sizeof(buf), "%0*llx", width, static_cast<unsigned long long>(bits));
	return buf;
}

std::string run(const std::string& kind, const std::string& input)
{
	if (kind == "str") {
		auto text = scg::json::encode(input).dump();
		return toHex(nlohmann::json::parse(text).get<std::string>());
	}
	if (kind == "strkey") {
		std::map<std::string, int> m{{input, 1}};
		auto text = scg::json::encode(m).dump();
		return toHex(nlohmann::json::parse(text).begin().key());
	}
	if (kind == "f32") {
		float f = 0;
		scg::json::decode(scg::json::parse(input), f);
		uint32_t bits = 0;
		std::memcpy(&bits, &f, sizeof(bits));
		return bitsHex(bits, 8);
	}
	if (kind == "f64") {
		double d = 0;
		scg::json::decode(scg::json::parse(input), d);
		uint64_t bits = 0;
		std::memcpy(&bits, &d, sizeof(bits));
		return bitsHex(bits, 16);
	}
	if (kind == "enum") {
		Kind k = Kind::A;
		scg::json::decode(scg::json::parse(input), k);
		return std::to_string(static_cast<int>(k));
	}
	if (kind == "enumkey") {
		Kind k = Kind::A;
		scg::json::decode_key(input, k);
		return std::to_string(static_cast<int>(k));
	}
	return "UNKNOWN";
}

} // namespace

int main()
{
	std::string line;
	while (std::getline(std::cin, line)) {
		auto space = line.find(' ');
		std::string kind = line.substr(0, space);
		std::string input = space == std::string::npos ? "" : fromHex(line.substr(space + 1));
		try {
			std::cout << run(kind, input) << "\n";
		} catch (const nlohmann::json::exception&) {
			std::cout << "ERR\n";
		}
	}
	return 0;
}
