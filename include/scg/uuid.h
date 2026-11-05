#pragma once

#include <cstring>
#include <algorithm>
#include <cctype>
#include <cstdint>
#include <functional>
#include <istream>
#include <ostream>
#include <stdexcept>
#include <string>
#include <string_view>
#include <random>

#include "scg/serialize.h"

#ifndef SCG_MINIMAL_INCLUDES
// Kept for source compatibility with code that relies on these transitive
// includes; define SCG_MINIMAL_INCLUDES to skip them (the JSON hooks below are
// templates and do not need nlohmann here).
#include <iomanip>
#include <sstream>
#include "nlohmann/json.hpp"
#endif

namespace scg {
namespace type {

// RFC 4122 UUID
class uuid {
public:

	constexpr uuid(const uint8_t (&bs)[16])
		: bytes_{
			bs[0], bs[1], bs[2], bs[3],
			bs[4], bs[5], bs[6], bs[7],
			bs[8], bs[9], bs[10], bs[11],
			bs[12], bs[13], bs[14], bs[15]
		}
	{
	}

	constexpr inline uuid()
		: bytes_{
			0,0,0,0,
			0,0,0,0,
			0,0,0,0,
			0,0,0,0}
	{
	}

	inline bool isNull() const
	{
		for (int i = 0; i < 16; ++i) {
			if (bytes_[i] != 0) {
				return false;
			}
		}
		return true;
	}

	inline static uuid random()
	{
		// Every byte comes straight from std::random_device (OS entropy). Expanding a single 32-bit
		// seed through a PRNG here would cap the space at 2^32 distinct UUIDs — birthday collisions at
		// ~77k ids — which is fatal for durable identities (these mint database primary keys).
		uuid u;
		std::random_device rd;
		for (int i = 0; i < 16; i += 4) {
			const uint32_t word = static_cast<uint32_t>(rd());
			u.bytes_[i] = static_cast<uint8_t>(word & 0xFF);
			u.bytes_[i + 1] = static_cast<uint8_t>((word >> 8) & 0xFF);
			u.bytes_[i + 2] = static_cast<uint8_t>((word >> 16) & 0xFF);
			u.bytes_[i + 3] = static_cast<uint8_t>((word >> 24) & 0xFF);
		}
		u.bytes_[6] = (u.bytes_[6] & 0x0F) | 0x40;
		u.bytes_[8] = (u.bytes_[8] & 0x3F) | 0x80;
		return u;
	}


	// fromString parses the canonical 8-4-4-4-12 hex form. Any version and
	// variant is accepted (matching Go's uuid.Parse); the null UUID is valid.
	inline static std::pair<uuid, error::Error> fromString(const std::string& str)
	{
		if (!isValid(str)) {
			return std::make_pair(uuid(), error::Error("Invalid UUID string"));
		}

		// isValid guarantees dashes only at 8/13/18/23, so hex pairs never
		// straddle a dash.
		uuid res;
		int byte = 0;
		size_t i = 0;
		while (i < str.size()) {
			if (str[i] == '-') {
				++i;
				continue;
			}
			res.bytes_[byte++] = static_cast<uint8_t>((hexValue(str[i]) << 4) | hexValue(str[i + 1]));
			i += 2;
		}
		return std::make_pair(res, nullptr);
	}

	// isValid reports whether str is a well-formed 36-character UUID string
	// (hex digits with dashes at positions 8, 13, 18 and 23). The version and
	// variant nibbles are not restricted.
	inline static bool isValid(const std::string& str)
	{
		if (str.length() != 36 || str[8] != '-' || str[13] != '-' || str[18] != '-' || str[23] != '-') {
			return false;
		}
		for (size_t i = 0; i < str.length(); ++i) {
			if (i == 8 || i == 13 || i == 18 || i == 23) {
				continue;
			}
			// Cast to unsigned char: passing a negative char to isxdigit is UB.
			if (!std::isxdigit(static_cast<unsigned char>(str[i]))) {
				return false;
			}
		}
		return true;
	}

	inline std::string toString() const
	{
		static const char* digits = "0123456789abcdef";
		std::string out;
		out.reserve(36);
		for (int i = 0; i < 16; ++i) {
			out.push_back(digits[bytes_[i] >> 4]);
			out.push_back(digits[bytes_[i] & 0x0F]);
			if (i == 3 || i == 5 || i == 7 || i == 9) {
				out.push_back('-');
			}
		}
		return out;
	}

	friend bool operator==(const uuid& lhs, const uuid& rhs) {
		return std::equal(lhs.bytes_, lhs.bytes_ + 16, rhs.bytes_);
	}

	friend bool operator!=(const uuid& lhs, const uuid& rhs) {
		return !(lhs == rhs);
	}

	inline friend bool operator<(const uuid& lhs, const uuid& rhs) {
		return std::lexicographical_compare(lhs.bytes_, lhs.bytes_ + 16, rhs.bytes_, rhs.bytes_ + 16);
	}

	inline friend std::ostream& operator<<(std::ostream& os, const uuid& v)
	{
		os << v.toString();
		return os;
	}

	inline friend std::istream& operator>>(std::istream& is, uuid& v)
	{
		std::string str;
		if (!(is >> str)) {
			return is;
		}
		auto [parsed, err] = fromString(str);
		if (err) {
			is.setstate(std::ios::failbit);
			return is;
		}
		v = parsed;
		return is;
	}

	friend inline uint32_t bit_size(const uuid&)
	{
		return scg::serialize::bytes_to_bits(16);
	}

	template <typename WriterType>
	friend inline void serialize(WriterType& writer, const uuid& value)
	{
		// Bulk-write the 16 bytes rather than 16 per-byte writeBits calls through
		// the bit machinery — wire-identical, and a large win on the hot path
		// (matches the Go bulk WriteBytes).
		writer.writeBytes(value.bytes_, 16);
	}

	template <typename ReaderType>
	friend inline error::Error deserialize(uuid& value, ReaderType& reader)
	{
		// Any 16 bytes are accepted (all versions/variants), matching Go.
		return reader.readBytes(value.bytes_, 16);
	}

	friend std::hash<scg::type::uuid>;

private:

	static inline int hexValue(char c)
	{
		if (c >= '0' && c <= '9') return c - '0';
		if (c >= 'a' && c <= 'f') return c - 'a' + 10;
		return c - 'A' + 10;
	}

	uint8_t bytes_[16];
};

// nlohmann json serialization. Templated on the json type so this header does
// not need to include nlohmann/json.hpp; nlohmann finds these via ADL.

template <typename BasicJsonType>
inline void to_json(BasicJsonType& j, const scg::type::uuid& uuid)
{
	j = uuid.toString();
}

template <typename BasicJsonType>
inline void from_json(const BasicJsonType& j, scg::type::uuid& uuid)
{
	auto [res, err] = scg::type::uuid::fromString(j.template get<std::string>());
	if (err != nullptr) {
		throw std::runtime_error(err.message());
	}
	uuid = res;
}

}
}

template<>
struct std::hash<scg::type::uuid> {
	std::size_t operator()(const scg::type::uuid& t) const
	{
		std::string_view sv(reinterpret_cast<const char*>(t.bytes_), 16);
		return std::hash<std::string_view>{}(sv);
	}
};
