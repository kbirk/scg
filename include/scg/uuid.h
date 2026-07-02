#pragma once

#include <cstring>
#include <algorithm>
#include <cstdint>
#include <string>
#include <sstream>
#include <iomanip>
#include <random>

#include "scg/serialize.h"

#include "nlohmann/json.hpp"

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


	inline static std::pair<uuid, error::Error> fromString(const std::string& str)
	{
		if (!isValid(str)) {
			return std::make_pair(uuid(), error::Error("Invalid UUID string"));
		}

		uuid res;

		std::string s = str;
		s.erase(std::remove(s.begin(), s.end(), '-'), s.end());
		for (int i = 0; i < 16; ++i) {
			std::string byte = s.substr(i * 2, 2);
			res.bytes_[i] = static_cast<uint8_t>(std::stoi(byte, nullptr, 16));
		}
		return std::make_pair(res, nullptr);
	}

	inline static bool isValid(std::string str)
	{
		if (str.length() != 36 || str[8] != '-' || str[13] != '-' || str[18] != '-' || str[23] != '-') {
			return false;
		}
		for (size_t i = 0; i < str.length(); ++i) {
			if (i != 8 && i != 13 && i != 18 && i != 23 && !std::isxdigit(str[i])) {
				return false;
			}
		}
		// check that the version is 4
		if (str[14] != '4') {
			return false;
		}
		// check that the variant is 1
		if (str[19] != '8' && str[19] != '9' && str[19] != 'a' && str[19] != 'b' && str[19] != 'A' && str[19] != 'B') {
			return false;
		}
		return true;
	}

	inline std::string toString() const
	{
		std::stringstream ss;
		for (int i = 0; i < 16; ++i) {
			ss << std::hex << std::setw(2) << std::setfill('0') << static_cast<int>(bytes_[i]);
			if (i == 3 || i == 5 || i == 7 || i == 9) {
				ss << "-";
			}
		}
		return ss.str();
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
		is >> str;
		v.fromString(str);
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
		auto err = reader.readBytes(value.bytes_, 16);
		if (err) {
			return err;
		}

		if (value.isNull()) {
			return nullptr;
		}
		if ((value.bytes_[6] & 0xF0) != 0x40) {
			return error::Error("Invalid UUID version: " + value.toString());
		}
		if ((value.bytes_[8] & 0xC0) != 0x80) {
			return error::Error("Invalid UUID variant: " + value.toString());
		}
		return nullptr;
	}

	friend std::hash<scg::type::uuid>;

private:

	uint8_t bytes_[16];
};

// nlohmann json serialization

inline void to_json(nlohmann::json& j, const scg::type::uuid& uuid)
{
	j = uuid.toString();
}

inline void from_json(const nlohmann::json& j, scg::type::uuid& uuid)
{
	auto [res, err] = scg::type::uuid::fromString(j.get<std::string>());
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
