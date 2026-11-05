#pragma once

// Canonical JSON encoding for scg-generated messages, interoperable with the
// Go generated code (Go 1.23 encoding/json is the reference):
//
//   bool                 true / false
//   integers             exact JSON numbers (every width, including the
//                        int64 / uint64 extremes)
//   float32 / float64    JSON numbers; float32 values are converted through
//                        their shortest round-trip decimal, so they are written
//                        as e.g. 1.1, not 1.100000023841858. Negative zero
//                        round-trips (Go writes -0) when the text is parsed
//                        with scg::json::parse. NaN and +-Inf are not
//                        representable and throw on encode.
//   string               JSON string; invalid UTF-8 is replaced byte by byte
//                        with U+FFFD exactly as Go does (see sanitize_utf8)
//   enum                 JSON number (the underlying uint16)
//   uuid                 canonical lowercase 8-4-4-4-12 string; decoding also
//                        accepts the array of 16 integers earlier Go code wrote
//                        for uuid typedefs
//   timestamp            RFC 3339 with Go's RFC3339Nano layout: UTC, "Z",
//                        fraction with trailing zeros trimmed and omitted when
//                        zero. Decoding accepts any offset ("Z" / +-HH:MM) and
//                        any number of fraction digits (beyond 9 truncated).
//                        Go's zero time.Time ("0001-01-01T00:00:00Z") maps to
//                        the minimum representable timestamp and back; other
//                        instants outside 1677-2262 saturate to the minimum /
//                        maximum, as the binary decoder does.
//   typedef              exactly as its underlying type
//   list<T>              JSON array, except lists of byte / uint8 (or a typedef
//                        of uint8), which are standard padded base64 strings
//                        (RFC 4648 section 4), as Go encodes []byte. Decoding
//                        a byte list also accepts an array of numbers.
//   map<K, V>            JSON object; keys are strings as-is, integers / enums
//                        as base-10 strings, uuids as the canonical string
//                        (float keys are written as a decimal, which Go's
//                        encoding/json does not support).
//   message              JSON object keyed by the .scg field names. Decoding
//                        skips keys that are absent (the field keeps its value,
//                        as in Go) and ignores unknown keys.
//   null                 decodes to an empty list / map, leaves a scalar or
//                        message field unchanged, and as a list element or map
//                        value decodes to the Go zero value (see zero_value),
//                        all as Go's encoding/json does.
//
// Known differences from Go's decoder, all for input Go itself never writes:
//   - Go matches object keys case-insensitively as a fallback; only exact keys
//     are matched here.
//   - Go 1.23's time decoder also accepts a comma as the fraction separator,
//     a single-digit hour, and an offset hour of 24; only strict RFC 3339 is
//     accepted here.
//   - Go's uuid parser also accepts the 32-hex-digit, braced and urn:uuid:
//     forms; only the canonical form is accepted. Conversely, plain uuid fields
//     accept the legacy array form, which Go only accepts for uuid typedefs.
//   - Invalid UTF-8 in input JSON, a leading byte order mark, and object keys
//     that are duplicates after normalization are handled by nlohmann::json's
//     parser, which differs from Go in those cases.
//   - A float32 is decoded via double; see the comment in decode.
//   - An absent timestamp field keeps its C++ default (the current time, from
//     scg::type::timestamp's constructor) rather than Go's zero time.
//
// The output is equal in value to Go's, not identical text: nlohmann::json
// spells numbers its own way (16777216.0 and 1e+20 where Go writes 16777216 and
// 100000000000000000000), writes object keys in sorted order rather than field
// order, and does not escape <, > and & as Go does. Compare decoded values,
// never JSON text.
//
// Decoding into a message that already holds data replaces its lists and maps,
// as the binary decoder does; Go merges into an existing map and reuses
// existing list elements.
//
// Generated message to_json / from_json call scg::json::encode / decode for
// each field, and generated fromJSON parses with scg::json::parse. This header deliberately does not change the ADL to_json /
// from_json overloads of scg::type::strong_typedef, timestamp and uuid that
// user code may rely on. Errors are reported by throwing
// nlohmann::json::type_error, like nlohmann's own conversions.

#include <chrono>
#include <cmath>
#include <cstddef>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <limits>
#include <map>
#include <string>
#include <type_traits>
#include <unordered_map>
#include <utility>
#include <vector>

#include "nlohmann/json.hpp"
#include "scg/timestamp.h"
#include "scg/typedef.h"
#include "scg/uuid.h"

namespace scg {
namespace json {

template <typename T>
nlohmann::json encode(const T& value);

template <typename T>
void decode(const nlohmann::json& j, T& value);

template <typename K>
std::string encode_key(const K& key);

template <typename K>
void decode_key(const std::string& text, K& key);

[[noreturn]] inline void fail(const std::string& message)
{
	throw nlohmann::json::type_error::create(302, "scg::json: " + message, nullptr);
}

namespace detail {

// parse_keeping_negative_zero parses like nlohmann::json::parse, except that the
// token -0 becomes the double -0.0. nlohmann stores -0 as the signed integer 0,
// which is indistinguishable from an integer 0 assigned in code, so without
// this decode could not tell Go's negative zero apart from a plain 0.
inline nlohmann::json parse_keeping_negative_zero(const std::string& text, bool allow_exceptions)
{
	return nlohmann::json::parse(text, [](int, nlohmann::json::parse_event_t event, nlohmann::json& parsed) {
		if (event == nlohmann::json::parse_event_t::value && parsed.is_number_integer() &&
			!parsed.is_number_unsigned() && parsed.get<int64_t>() == 0) {
			parsed = -0.0;
		}
		return true;
	}, allow_exceptions);
}

inline bool is_negative_zero(const nlohmann::json& j)
{
	return j.is_number_float() && j.get<double>() == 0 && std::signbit(j.get<double>());
}

} // namespace detail

// parse parses JSON text for decode. Use it rather than nlohmann::json::parse so
// that Go's -0 decodes exactly as in Go: negative zero for a float field, 0 for
// a signed integer, and an error for an unsigned integer or enum. Throws
// nlohmann::json::parse_error on malformed text.
inline nlohmann::json parse(const std::string& text)
{
	return detail::parse_keeping_negative_zero(text, true);
}

namespace detail {

template <typename T>
struct is_strong_typedef : std::false_type {};

template <typename T, typename Tag>
struct is_strong_typedef<scg::type::strong_typedef<T, Tag>> : std::true_type {
	using underlying_type = T;
};

template <typename T>
struct is_vector : std::false_type {};

template <typename T, typename A>
struct is_vector<std::vector<T, A>> : std::true_type {};

template <typename T>
struct is_map : std::false_type {};

template <typename K, typename V, typename C, typename A>
struct is_map<std::map<K, V, C, A>> : std::true_type {};

template <typename K, typename V, typename H, typename E, typename A>
struct is_map<std::unordered_map<K, V, H, E, A>> : std::true_type {};

// byte / uint8 list elements, and typedefs of uint8, are encoded as base64
template <typename T>
struct is_byte_like : std::is_same<T, uint8_t> {};

template <typename Tag>
struct is_byte_like<scg::type::strong_typedef<uint8_t, Tag>> : std::true_type {};

template <typename T>
struct dependent_false : std::false_type {};

template <typename T>
inline uint8_t byte_value(const T& b)
{
	return static_cast<const uint8_t&>(b);
}

inline bool is_digit(char c)
{
	return c >= '0' && c <= '9';
}

} // namespace detail

// ---------------------------------------------------------------------------
// base64 (RFC 4648 section 4, standard alphabet, padded)
// ---------------------------------------------------------------------------

inline std::string base64_encode(const uint8_t* data, size_t size)
{
	static const char* alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
	std::string out;
	out.reserve(((size + 2) / 3) * 4);
	size_t i = 0;
	for (; i + 3 <= size; i += 3) {
		uint32_t n = (uint32_t(data[i]) << 16) | (uint32_t(data[i + 1]) << 8) | uint32_t(data[i + 2]);
		out.push_back(alphabet[(n >> 18) & 0x3F]);
		out.push_back(alphabet[(n >> 12) & 0x3F]);
		out.push_back(alphabet[(n >> 6) & 0x3F]);
		out.push_back(alphabet[n & 0x3F]);
	}
	size_t rest = size - i;
	if (rest == 1) {
		uint32_t n = uint32_t(data[i]) << 16;
		out.push_back(alphabet[(n >> 18) & 0x3F]);
		out.push_back(alphabet[(n >> 12) & 0x3F]);
		out.push_back('=');
		out.push_back('=');
	} else if (rest == 2) {
		uint32_t n = (uint32_t(data[i]) << 16) | (uint32_t(data[i + 1]) << 8);
		out.push_back(alphabet[(n >> 18) & 0x3F]);
		out.push_back(alphabet[(n >> 12) & 0x3F]);
		out.push_back(alphabet[(n >> 6) & 0x3F]);
		out.push_back('=');
	}
	return out;
}

inline std::string base64_encode(const std::vector<uint8_t>& data)
{
	return base64_encode(data.data(), data.size());
}

// base64_decode decodes padded standard base64. Like Go's
// base64.StdEncoding, '\r' and '\n' are ignored and non-zero trailing bits are
// accepted. Returns false for malformed input.
inline bool base64_decode(const std::string& text, std::vector<uint8_t>& out)
{
	auto value = [](char c) -> int {
		if (c >= 'A' && c <= 'Z') {
			return c - 'A';
		}
		if (c >= 'a' && c <= 'z') {
			return c - 'a' + 26;
		}
		if (c >= '0' && c <= '9') {
			return c - '0' + 52;
		}
		if (c == '+') {
			return 62;
		}
		if (c == '/') {
			return 63;
		}
		return -1;
	};

	std::string chars;
	chars.reserve(text.size());
	for (char c : text) {
		if (c != '\r' && c != '\n') {
			chars.push_back(c);
		}
	}
	if (chars.size() % 4 != 0) {
		return false;
	}

	out.clear();
	out.reserve((chars.size() / 4) * 3);
	for (size_t i = 0; i < chars.size(); i += 4) {
		bool last = i + 4 == chars.size();
		int a = value(chars[i]);
		int b = value(chars[i + 1]);
		if (a < 0 || b < 0) {
			return false;
		}
		if (last && chars[i + 2] == '=' && chars[i + 3] == '=') {
			out.push_back(static_cast<uint8_t>((a << 2) | (b >> 4)));
			break;
		}
		int c = value(chars[i + 2]);
		if (c < 0) {
			return false;
		}
		if (last && chars[i + 3] == '=') {
			out.push_back(static_cast<uint8_t>((a << 2) | (b >> 4)));
			out.push_back(static_cast<uint8_t>(((b & 0x0F) << 4) | (c >> 2)));
			break;
		}
		int d = value(chars[i + 3]);
		if (d < 0) {
			return false;
		}
		out.push_back(static_cast<uint8_t>((a << 2) | (b >> 4)));
		out.push_back(static_cast<uint8_t>(((b & 0x0F) << 4) | (c >> 2)));
		out.push_back(static_cast<uint8_t>(((c & 0x03) << 6) | d));
	}
	return true;
}

// ---------------------------------------------------------------------------
// RFC 3339 timestamps
// ---------------------------------------------------------------------------

// days_from_civil returns the number of days since 1970-01-01 of a proleptic
// Gregorian date (Howard Hinnant's algorithm, exact for all int64 ranges used
// here).
inline int64_t days_from_civil(int64_t y, unsigned m, unsigned d)
{
	y -= m <= 2 ? 1 : 0;
	const int64_t era = (y >= 0 ? y : y - 399) / 400;
	const unsigned yoe = static_cast<unsigned>(y - era * 400);
	const unsigned doy = (153 * (m > 2 ? m - 3 : m + 9) + 2) / 5 + d - 1;
	const unsigned doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
	return era * 146097 + static_cast<int64_t>(doe) - 719468;
}

// civil_from_days is the inverse of days_from_civil.
inline void civil_from_days(int64_t z, int64_t& y, unsigned& m, unsigned& d)
{
	z += 719468;
	const int64_t era = (z >= 0 ? z : z - 146096) / 146097;
	const unsigned doe = static_cast<unsigned>(z - era * 146097);
	const unsigned yoe = (doe - doe / 1460 + doe / 36524 - doe / 146096) / 365;
	y = static_cast<int64_t>(yoe) + era * 400;
	const unsigned doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
	const unsigned mp = (5 * doy + 2) / 153;
	d = doy - (153 * mp + 2) / 5 + 1;
	m = mp < 10 ? mp + 3 : mp - 9;
	y += m <= 2 ? 1 : 0;
}

inline bool is_leap_year(int64_t y)
{
	return (y % 4 == 0 && y % 100 != 0) || y % 400 == 0;
}

inline unsigned days_in_month(int64_t y, unsigned m)
{
	static const unsigned days[] = {31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31};
	return m == 2 && is_leap_year(y) ? 29 : days[m - 1];
}

// format_rfc3339_nano formats nanoseconds since the Unix epoch as RFC 3339 in
// UTC, using Go's RFC3339Nano layout.
inline std::string format_rfc3339_nano(int64_t unix_nanos)
{
	const int64_t ns_per_s = 1000000000;
	int64_t secs = unix_nanos / ns_per_s;
	int64_t frac = unix_nanos % ns_per_s;
	if (frac < 0) {
		frac += ns_per_s;
		secs -= 1;
	}
	int64_t days = secs / 86400;
	int64_t sod = secs % 86400;
	if (sod < 0) {
		sod += 86400;
		days -= 1;
	}
	int64_t y = 0;
	unsigned m = 0;
	unsigned d = 0;
	civil_from_days(days, y, m, d);

	char buf[64];
	std::snprintf(buf, sizeof(buf), "%04lld-%02u-%02uT%02u:%02u:%02u",
		static_cast<long long>(y), m, d,
		static_cast<unsigned>(sod / 3600), static_cast<unsigned>((sod % 3600) / 60), static_cast<unsigned>(sod % 60));
	std::string out(buf);
	if (frac != 0) {
		char fbuf[16];
		std::snprintf(fbuf, sizeof(fbuf), "%09lld", static_cast<long long>(frac));
		std::string digits(fbuf);
		while (!digits.empty() && digits.back() == '0') {
			digits.pop_back();
		}
		out += '.';
		out += digits;
	}
	out += 'Z';
	return out;
}

namespace detail {

inline bool parse_fixed_digits(const std::string& s, size_t pos, size_t count, int64_t& out)
{
	if (pos + count > s.size()) {
		return false;
	}
	int64_t v = 0;
	for (size_t i = pos; i < pos + count; ++i) {
		if (!is_digit(s[i])) {
			return false;
		}
		v = v * 10 + (s[i] - '0');
	}
	out = v;
	return true;
}

// parse_rfc3339_parts parses an RFC 3339 date-time into whole seconds since
// the Unix epoch (UTC) and nanoseconds in [0, 1e9).
inline bool parse_rfc3339_parts(const std::string& s, int64_t& unix_seconds, int64_t& nanos)
{
	int64_t year, month, day, hour, minute, second;
	// YYYY-MM-DDTHH:MM:SS
	if (s.size() < 20 || s[4] != '-' || s[7] != '-' || s[10] != 'T' || s[13] != ':' || s[16] != ':') {
		return false;
	}
	if (!parse_fixed_digits(s, 0, 4, year) || !parse_fixed_digits(s, 5, 2, month) ||
		!parse_fixed_digits(s, 8, 2, day) || !parse_fixed_digits(s, 11, 2, hour) ||
		!parse_fixed_digits(s, 14, 2, minute) || !parse_fixed_digits(s, 17, 2, second)) {
		return false;
	}
	if (month < 1 || month > 12 || day < 1 || day > days_in_month(year, static_cast<unsigned>(month)) ||
		hour > 23 || minute > 59 || second > 59) {
		return false;
	}

	size_t pos = 19;
	int64_t frac = 0;
	if (s[pos] == '.') {
		++pos;
		size_t start = pos;
		int64_t scale = 100000000;
		while (pos < s.size() && is_digit(s[pos])) {
			if (scale > 0) {
				// digits beyond nanosecond precision are truncated, as in Go
				frac += (s[pos] - '0') * scale;
				scale /= 10;
			}
			++pos;
		}
		if (pos == start) {
			return false;
		}
	}

	int64_t offset = 0;
	if (pos < s.size() && s[pos] == 'Z') {
		++pos;
	} else if (pos < s.size() && (s[pos] == '+' || s[pos] == '-')) {
		int64_t oh, om;
		if (pos + 6 > s.size() || s[pos + 3] != ':' || !parse_fixed_digits(s, pos + 1, 2, oh) ||
			!parse_fixed_digits(s, pos + 4, 2, om) || oh > 23 || om > 59) {
			return false;
		}
		offset = (oh * 60 + om) * 60;
		if (s[pos] == '-') {
			offset = -offset;
		}
		pos += 6;
	} else {
		return false;
	}
	if (pos != s.size()) {
		return false;
	}

	int64_t days = days_from_civil(year, static_cast<unsigned>(month), static_cast<unsigned>(day));
	unix_seconds = days * 86400 + hour * 3600 + minute * 60 + second - offset;
	nanos = frac;
	return true;
}

// the Unix time of Go's zero time.Time, 0001-01-01T00:00:00Z
constexpr int64_t go_zero_time_unix_seconds = -62135596800;

// seconds_nanos_to_unix_nanos combines seconds and nanoseconds in [0, 1e9),
// returning false when the result does not fit in an int64.
inline bool seconds_nanos_to_unix_nanos(int64_t secs, int64_t nanos, int64_t& out)
{
	const int64_t ns_per_s = 1000000000;
	const int64_t max_secs = (std::numeric_limits<int64_t>::max)() / ns_per_s;      // 9223372036
	const int64_t min_secs = (std::numeric_limits<int64_t>::min)() / ns_per_s - 1;  // -9223372037
	if (secs > max_secs || secs < min_secs) {
		return false;
	}
	if (secs >= 0) {
		int64_t base = secs * ns_per_s;
		if (nanos > (std::numeric_limits<int64_t>::max)() - base) {
			return false;
		}
		out = base + nanos;
		return true;
	}
	// for negative seconds, (secs + 1) * 1e9 cannot overflow
	int64_t base = (secs + 1) * ns_per_s;
	int64_t add = nanos - ns_per_s;
	if (add < (std::numeric_limits<int64_t>::min)() - base) {
		return false;
	}
	out = base + add;
	return true;
}

} // namespace detail

// parse_rfc3339 parses an RFC 3339 date-time into nanoseconds since the Unix
// epoch. Returns false for malformed input, or a time outside the int64
// nanosecond range (1677-09-21T00:12:43.145224192Z to
// 2262-04-11T23:47:16.854775807Z).
inline bool parse_rfc3339(const std::string& text, int64_t& unix_nanos)
{
	int64_t secs = 0;
	int64_t nanos = 0;
	if (!detail::parse_rfc3339_parts(text, secs, nanos)) {
		return false;
	}
	return detail::seconds_nanos_to_unix_nanos(secs, nanos, unix_nanos);
}

// format_timestamp / parse_timestamp convert a timestamp to and from its JSON
// text. Go's zero time.Time maps to the minimum representable timestamp.
inline std::string format_timestamp(const scg::type::timestamp& ts)
{
	int64_t ns = std::chrono::duration_cast<std::chrono::nanoseconds>(ts.timepoint().time_since_epoch()).count();
	if (ns == (std::numeric_limits<int64_t>::min)()) {
		return "0001-01-01T00:00:00Z";
	}
	return format_rfc3339_nano(ns);
}

// parse_timestamp returns false only for malformed text. Like the binary
// decoder, instants outside the representable range saturate: earlier ones to
// the minimum timestamp (which encodes as Go's zero time), later ones (e.g.
// Go's "9999-12-31T23:59:59Z" "never" sentinel) to the maximum.
inline bool parse_timestamp(const std::string& text, scg::type::timestamp& ts)
{
	int64_t secs = 0;
	int64_t nanos = 0;
	if (!detail::parse_rfc3339_parts(text, secs, nanos)) {
		return false;
	}
	int64_t ns = 0;
	if (secs == detail::go_zero_time_unix_seconds && nanos == 0) {
		ns = (std::numeric_limits<int64_t>::min)();
	} else if (!detail::seconds_nanos_to_unix_nanos(secs, nanos, ns)) {
		ns = secs < 0 ? (std::numeric_limits<int64_t>::min)() : (std::numeric_limits<int64_t>::max)();
	}
	using time_point = std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds>;
	ts = scg::type::timestamp(time_point(std::chrono::nanoseconds(ns)));
	return true;
}

// ---------------------------------------------------------------------------
// numbers
// ---------------------------------------------------------------------------

// float_to_shortest_double returns the double closest to the shortest decimal
// that round-trips to f, so a float32 is written as e.g. 1.1, not
// 1.100000023841858. nlohmann::json then formats that double with its own
// algorithm, so the text is not always the shortest float32 decimal Go would
// write, but it always parses back to the same float.
inline double float_to_shortest_double(float f)
{
	char buf[32];
	for (int precision = 1; precision <= 9; ++precision) {
		std::snprintf(buf, sizeof(buf), "%.*g", precision, static_cast<double>(f));
		if (std::strtof(buf, nullptr) == f) {
			return std::strtod(buf, nullptr);
		}
	}
	return static_cast<double>(f);
}

namespace detail {

// parse_int64 parses a base-10 integer with an optional sign.
inline bool parse_int64(const std::string& s, int64_t& out)
{
	size_t i = 0;
	bool negative = false;
	if (i < s.size() && (s[i] == '+' || s[i] == '-')) {
		negative = s[i] == '-';
		++i;
	}
	if (i == s.size()) {
		return false;
	}
	uint64_t limit = negative ? uint64_t(1) << 63 : uint64_t((std::numeric_limits<int64_t>::max)());
	uint64_t v = 0;
	for (; i < s.size(); ++i) {
		if (!is_digit(s[i])) {
			return false;
		}
		uint64_t digit = uint64_t(s[i] - '0');
		if (v > (limit - digit) / 10) {
			return false;
		}
		v = v * 10 + digit;
	}
	out = negative ? static_cast<int64_t>(0 - v) : static_cast<int64_t>(v);
	return true;
}

// parse_uint64 parses a base-10 unsigned integer (digits only).
inline bool parse_uint64(const std::string& s, uint64_t& out)
{
	if (s.empty()) {
		return false;
	}
	const uint64_t limit = (std::numeric_limits<uint64_t>::max)();
	uint64_t v = 0;
	for (char c : s) {
		if (!is_digit(c)) {
			return false;
		}
		uint64_t digit = uint64_t(c - '0');
		if (v > (limit - digit) / 10) {
			return false;
		}
		v = v * 10 + digit;
	}
	out = v;
	return true;
}

template <typename T>
inline void check_signed_range(int64_t v, T& out, const char* what)
{
	if (v < static_cast<int64_t>((std::numeric_limits<T>::min)()) || v > static_cast<int64_t>((std::numeric_limits<T>::max)())) {
		fail(std::string("number ") + std::to_string(v) + " overflows " + what);
	}
	out = static_cast<T>(v);
}

template <typename T>
inline void check_unsigned_range(uint64_t v, T& out, const char* what)
{
	if (v > static_cast<uint64_t>((std::numeric_limits<T>::max)())) {
		fail(std::string("number ") + std::to_string(v) + " overflows " + what);
	}
	out = static_cast<T>(v);
}

template <typename T>
inline void decode_integer(const nlohmann::json& j, T& out)
{
	if (j.is_number_unsigned()) {
		uint64_t v = j.get<uint64_t>();
		if constexpr (std::is_signed<T>::value) {
			if (v > static_cast<uint64_t>((std::numeric_limits<T>::max)())) {
				fail("number " + std::to_string(v) + " overflows a signed integer field");
			}
			out = static_cast<T>(v);
		} else {
			check_unsigned_range(v, out, "an unsigned integer field");
		}
	} else if (j.is_number_integer()) {
		// nlohmann parses negative numbers as signed, but also stores a signed
		// C++ integer assigned in code as signed, so a non-negative value is
		// valid for an unsigned field
		int64_t v = j.get<int64_t>();
		if constexpr (std::is_signed<T>::value) {
			check_signed_range(v, out, "a signed integer field");
		} else {
			if (v < 0) {
				fail("number " + std::to_string(v) + " overflows an unsigned integer field");
			}
			check_unsigned_range(static_cast<uint64_t>(v), out, "an unsigned integer field");
		}
	} else if (is_negative_zero(j)) {
		// the token -0 (see parse): Go's strconv.ParseInt accepts it, ParseUint
		// does not
		if constexpr (std::is_signed<T>::value) {
			out = 0;
		} else {
			fail("number -0 is not a valid unsigned integer");
		}
	} else {
		fail(std::string("expected an integer, got ") + j.type_name());
	}
}

template <typename T>
inline std::string integer_key(const T& v)
{
	if constexpr (std::is_signed<T>::value) {
		return std::to_string(static_cast<int64_t>(v));
	} else {
		return std::to_string(static_cast<uint64_t>(v));
	}
}

template <typename T>
inline void decode_integer_key(const std::string& text, T& out)
{
	if constexpr (std::is_signed<T>::value) {
		int64_t v = 0;
		if (!parse_int64(text, v)) {
			fail("invalid integer map key \"" + text + "\"");
		}
		check_signed_range(v, out, "an integer map key");
	} else {
		uint64_t v = 0;
		if (!parse_uint64(text, v)) {
			fail("invalid unsigned integer map key \"" + text + "\"");
		}
		check_unsigned_range(v, out, "an integer map key");
	}
}

} // namespace detail

// ---------------------------------------------------------------------------
// strings
// ---------------------------------------------------------------------------

// sanitize_utf8 replaces invalid UTF-8 exactly as Go's encoding/json does when
// encoding a string: valid sequences are copied, and each byte that does not
// start a valid sequence (per utf8.DecodeRune: no overlong forms, no surrogates
// U+D800-U+DFFF, nothing above U+10FFFF, no truncated sequences) becomes U+FFFD.
inline std::string sanitize_utf8(const std::string& s)
{
	auto valid_length = [&s](size_t i) -> size_t {
		const unsigned char b0 = static_cast<unsigned char>(s[i]);
		if (b0 < 0x80) {
			return 1;
		}
		size_t need = 0;
		unsigned char lo = 0x80;
		unsigned char hi = 0xBF;
		if (b0 >= 0xC2 && b0 <= 0xDF) {
			need = 1;
		} else if (b0 == 0xE0) {
			need = 2;
			lo = 0xA0;
		} else if ((b0 >= 0xE1 && b0 <= 0xEC) || b0 == 0xEE || b0 == 0xEF) {
			need = 2;
		} else if (b0 == 0xED) {
			need = 2;
			hi = 0x9F;
		} else if (b0 == 0xF0) {
			need = 3;
			lo = 0x90;
		} else if (b0 >= 0xF1 && b0 <= 0xF3) {
			need = 3;
		} else if (b0 == 0xF4) {
			need = 3;
			hi = 0x8F;
		} else {
			return 0;
		}
		if (i + need >= s.size()) {
			// truncated sequence
			return 0;
		}
		for (size_t k = 1; k <= need; ++k) {
			const unsigned char b = static_cast<unsigned char>(s[i + k]);
			const unsigned char min = k == 1 ? lo : 0x80;
			const unsigned char max = k == 1 ? hi : 0xBF;
			if (b < min || b > max) {
				return 0;
			}
		}
		return need + 1;
	};

	// fast path: already valid
	size_t i = 0;
	while (i < s.size()) {
		size_t n = valid_length(i);
		if (n == 0) {
			break;
		}
		i += n;
	}
	if (i == s.size()) {
		return s;
	}

	std::string out(s, 0, i);
	out.reserve(s.size() + 8);
	while (i < s.size()) {
		size_t n = valid_length(i);
		if (n == 0) {
			out += "\xEF\xBF\xBD";
			i += 1;
		} else {
			out.append(s, i, n);
			i += n;
		}
	}
	return out;
}

// expect_object is called by generated message from_json functions: it returns
// false for null (the message is left unchanged, as Go does), true for an
// object, and throws for any other JSON value (Go rejects those as well).
inline bool expect_object(const nlohmann::json& j)
{
	if (j.is_null()) {
		return false;
	}
	if (!j.is_object()) {
		fail(std::string("expected an object, got ") + j.type_name());
	}
	return true;
}

// ---------------------------------------------------------------------------
// encode / decode
// ---------------------------------------------------------------------------

// zero_value returns the value a field holds in Go when it is absent or null:
// numbers 0, strings and containers empty, uuids nil, timestamps the minimum
// timestamp (the representation of Go's zero time.Time), and messages default
// constructed. Used for null list elements and map values.
template <typename T>
T zero_value()
{
	if constexpr (std::is_same<T, scg::type::timestamp>::value) {
		using time_point = std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds>;
		return scg::type::timestamp(time_point(std::chrono::nanoseconds((std::numeric_limits<int64_t>::min)())));
	} else if constexpr (detail::is_strong_typedef<T>::value) {
		return T(zero_value<typename detail::is_strong_typedef<T>::underlying_type>());
	} else {
		return T{};
	}
}

namespace detail {

// decode_legacy_uuid decodes the array of 16 integers that earlier Go
// generated code wrote for uuid typedefs, with Go's array semantics: missing
// trailing elements are zero, extra elements are ignored, and every decoded
// element must be an integer in [0, 255] (null leaves it zero).
inline void decode_legacy_uuid(const nlohmann::json& j, scg::type::uuid& value)
{
	uint8_t bytes[16] = {};
	size_t i = 0;
	for (const auto& item : j) {
		if (i == 16) {
			break;
		}
		if (!item.is_null()) {
			decode_integer(item, bytes[i]);
		}
		++i;
	}
	value = scg::type::uuid(bytes);
}

} // namespace detail

template <typename K>
std::string encode_key(const K& key)
{
	if constexpr (std::is_same<K, std::string>::value) {
		return sanitize_utf8(key);
	} else if constexpr (std::is_same<K, bool>::value) {
		static_assert(detail::dependent_false<K>::value, "bool map keys are not supported");
	} else if constexpr (std::is_enum<K>::value) {
		return std::to_string(static_cast<uint64_t>(key));
	} else if constexpr (std::is_integral<K>::value) {
		return detail::integer_key(key);
	} else if constexpr (std::is_floating_point<K>::value) {
		return encode(key).dump();
	} else if constexpr (std::is_same<K, scg::type::uuid>::value) {
		return key.toString();
	} else if constexpr (detail::is_strong_typedef<K>::value) {
		return encode_key(static_cast<const typename detail::is_strong_typedef<K>::underlying_type&>(key));
	} else {
		static_assert(detail::dependent_false<K>::value, "unsupported map key type");
	}
}

template <typename K>
void decode_key(const std::string& text, K& key)
{
	if constexpr (std::is_same<K, std::string>::value) {
		key = text;
	} else if constexpr (std::is_enum<K>::value) {
		// enums are uint16 in Go: digits only (no sign), as strconv.ParseUint
		uint64_t v = 0;
		if (!detail::parse_uint64(text, v) || v > 65535) {
			fail("invalid enum map key \"" + text + "\"");
		}
		key = static_cast<K>(v);
	} else if constexpr (std::is_integral<K>::value) {
		detail::decode_integer_key(text, key);
	} else if constexpr (std::is_floating_point<K>::value) {
		nlohmann::json j = detail::parse_keeping_negative_zero(text, false);
		if (j.is_discarded() || !j.is_number()) {
			fail("invalid floating point map key \"" + text + "\"");
		}
		decode(j, key);
	} else if constexpr (std::is_same<K, scg::type::uuid>::value) {
		auto parsed = scg::type::uuid::fromString(text);
		if (parsed.second) {
			fail("invalid uuid map key \"" + text + "\"");
		}
		key = parsed.first;
	} else if constexpr (detail::is_strong_typedef<K>::value) {
		decode_key(text, static_cast<typename detail::is_strong_typedef<K>::underlying_type&>(key));
	} else {
		static_assert(detail::dependent_false<K>::value, "unsupported map key type");
	}
}

template <typename T>
nlohmann::json encode(const T& value)
{
	if constexpr (std::is_same<T, bool>::value) {
		return nlohmann::json(value);
	} else if constexpr (std::is_enum<T>::value) {
		// unsigned, as Go's uint16 enums
		return nlohmann::json(static_cast<uint64_t>(value));
	} else if constexpr (std::is_integral<T>::value) {
		if constexpr (std::is_signed<T>::value) {
			return nlohmann::json(static_cast<int64_t>(value));
		} else {
			return nlohmann::json(static_cast<uint64_t>(value));
		}
	} else if constexpr (std::is_floating_point<T>::value) {
		if (!std::isfinite(value)) {
			fail("NaN and infinite floating point values cannot be encoded as JSON");
		}
		if constexpr (std::is_same<T, float>::value) {
			return nlohmann::json(float_to_shortest_double(value));
		} else {
			return nlohmann::json(static_cast<double>(value));
		}
	} else if constexpr (std::is_same<T, std::string>::value) {
		return nlohmann::json(sanitize_utf8(value));
	} else if constexpr (std::is_same<T, scg::type::uuid>::value) {
		return nlohmann::json(value.toString());
	} else if constexpr (std::is_same<T, scg::type::timestamp>::value) {
		return nlohmann::json(format_timestamp(value));
	} else if constexpr (detail::is_strong_typedef<T>::value) {
		return encode(static_cast<const typename detail::is_strong_typedef<T>::underlying_type&>(value));
	} else if constexpr (detail::is_vector<T>::value) {
		using element_type = typename T::value_type;
		if constexpr (detail::is_byte_like<element_type>::value) {
			std::vector<uint8_t> bytes;
			bytes.reserve(value.size());
			for (const auto& b : value) {
				bytes.push_back(detail::byte_value(b));
			}
			return nlohmann::json(base64_encode(bytes));
		} else {
			nlohmann::json arr = nlohmann::json::array();
			for (const auto& element : value) {
				arr.push_back(encode(static_cast<const element_type&>(element)));
			}
			return arr;
		}
	} else if constexpr (detail::is_map<T>::value) {
		nlohmann::json obj = nlohmann::json::object();
		for (const auto& entry : value) {
			obj[encode_key(entry.first)] = encode(entry.second);
		}
		return obj;
	} else {
		// generated messages: their own to_json, found by ADL
		return nlohmann::json(value);
	}
}

template <typename T>
void decode(const nlohmann::json& j, T& value)
{
	if (j.is_null()) {
		// as Go's encoding/json: null empties a list / map, and leaves any other
		// value unchanged
		if constexpr (detail::is_vector<T>::value || detail::is_map<T>::value) {
			value.clear();
		}
		return;
	}

	if constexpr (std::is_same<T, bool>::value) {
		if (!j.is_boolean()) {
			fail(std::string("expected a boolean, got ") + j.type_name());
		}
		value = j.get<bool>();
	} else if constexpr (std::is_enum<T>::value) {
		// enums are uint16 in Go: only unsigned integers (not -0)
		uint16_t v = 0;
		detail::decode_integer(j, v);
		value = static_cast<T>(v);
	} else if constexpr (std::is_integral<T>::value) {
		detail::decode_integer(j, value);
	} else if constexpr (std::is_floating_point<T>::value) {
		if (!j.is_number()) {
			fail(std::string("expected a number, got ") + j.type_name());
		}
		double d = j.get<double>();
		if constexpr (std::is_same<T, float>::value) {
			// Values below the midpoint between FLT_MAX and 2^128 round to
			// FLT_MAX, as in Go's strconv.ParseFloat(s, 32). The midpoint itself
			// is also mapped to FLT_MAX: decimal text slightly below it (which Go
			// accepts) parses to exactly the midpoint double, and converting that
			// to float would round (half to even) to infinity. Limitation: text is
			// converted to double first and then to float, so a decimal that is
			// more precise than any float or double (never produced by Go or by
			// this header) can round differently than Go's direct conversion.
			const double float_rounding_midpoint = 0x1.ffffffp+127;
			if (std::fabs(d) > float_rounding_midpoint) {
				fail("number overflows a float32 field");
			}
			if (std::fabs(d) == float_rounding_midpoint) {
				value = d < 0 ? -(std::numeric_limits<float>::max)() : (std::numeric_limits<float>::max)();
				return;
			}
		}
		value = static_cast<T>(d);
	} else if constexpr (std::is_same<T, std::string>::value) {
		if (!j.is_string()) {
			fail(std::string("expected a string, got ") + j.type_name());
		}
		value = j.get<std::string>();
	} else if constexpr (std::is_same<T, scg::type::uuid>::value) {
		if (j.is_array()) {
			// the legacy form earlier Go code wrote for uuid typedefs
			detail::decode_legacy_uuid(j, value);
			return;
		}
		if (!j.is_string()) {
			fail(std::string("expected a uuid string, got ") + j.type_name());
		}
		auto parsed = scg::type::uuid::fromString(j.get<std::string>());
		if (parsed.second) {
			fail("invalid uuid \"" + j.get<std::string>() + "\"");
		}
		value = parsed.first;
	} else if constexpr (std::is_same<T, scg::type::timestamp>::value) {
		if (!j.is_string()) {
			fail(std::string("expected an RFC 3339 timestamp string, got ") + j.type_name());
		}
		if (!parse_timestamp(j.get<std::string>(), value)) {
			fail("invalid or out of range RFC 3339 timestamp \"" + j.get<std::string>() + "\"");
		}
	} else if constexpr (detail::is_strong_typedef<T>::value) {
		decode(j, static_cast<typename detail::is_strong_typedef<T>::underlying_type&>(value));
	} else if constexpr (detail::is_vector<T>::value) {
		using element_type = typename T::value_type;
		if constexpr (detail::is_byte_like<element_type>::value) {
			if (j.is_string()) {
				std::vector<uint8_t> bytes;
				if (!base64_decode(j.get<std::string>(), bytes)) {
					fail("invalid base64 byte list");
				}
				value.clear();
				value.reserve(bytes.size());
				for (uint8_t b : bytes) {
					value.push_back(element_type(b));
				}
				return;
			}
		}
		if (!j.is_array()) {
			fail(std::string("expected an array, got ") + j.type_name());
		}
		value.clear();
		value.reserve(j.size());
		for (const auto& item : j) {
			// a null element is the Go zero value of the element type
			element_type element = zero_value<element_type>();
			decode(item, element);
			value.push_back(std::move(element));
		}
	} else if constexpr (detail::is_map<T>::value) {
		if (!j.is_object()) {
			fail(std::string("expected an object, got ") + j.type_name());
		}
		value.clear();
		for (auto it = j.begin(); it != j.end(); ++it) {
			typename T::key_type key = zero_value<typename T::key_type>();
			decode_key(it.key(), key);
			// a null value is the Go zero value of the value type
			typename T::mapped_type mapped = zero_value<typename T::mapped_type>();
			decode(it.value(), mapped);
			value.insert_or_assign(std::move(key), std::move(mapped));
		}
	} else {
		// generated messages: their own from_json, found by ADL
		j.get_to(value);
	}
}

} // namespace json
} // namespace scg
