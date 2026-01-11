#pragma once

#include <cstdint>
#include <cmath>
#include <cstring>
#include <limits>
#include <array>

#include "scg/error.h"

namespace scg {
namespace serialize {

// Type aliases for float types used in serialization
using float32_t = float;
using float64_t = double;

inline constexpr uint32_t bits_to_bytes(uint32_t x) {
	return (x + 7) / 8;
}

inline constexpr uint32_t bytes_to_bits(uint32_t x) {
	return x << 3; // same as (x * 8)
}

inline constexpr uint32_t get_byte_offset(uint32_t x) {
	return x >> 3; // same as (x / 8)
}

inline constexpr uint8_t get_bit_offset(uint32_t x) {
	return x & 0x7; // same as (x % 8)
}

inline constexpr int clz64(uint64_t x) {
	if (x == 0) return 64;
	int n = 0;
	if ((x & 0xFFFFFFFF00000000ULL) == 0) { n += 32; x <<= 32; }
	if ((x & 0xFFFF000000000000ULL) == 0) { n += 16; x <<= 16; }
	if ((x & 0xFF00000000000000ULL) == 0) { n += 8;  x <<= 8; }
	if ((x & 0xF000000000000000ULL) == 0) { n += 4;  x <<= 4; }
	if ((x & 0xC000000000000000ULL) == 0) { n += 2;  x <<= 2; }
	if ((x & 0x8000000000000000ULL) == 0) { n += 1; }
	return n;
}

inline constexpr uint32_t var_uint_bit_size(uint64_t val, uint32_t num_bytes)
{
	if (val == 0) {
		return 1;
	}
	uint32_t k = (71 - clz64(val)) >> 3;
	if (k < num_bytes) {
		return k * 9 + 1;
	}
	return num_bytes * 9;
}

template <typename WriterType>
inline void var_encode_uint(WriterType& writer, uint64_t val, uint32_t num_bytes)
{
	for (uint32_t i = 0; i < num_bytes; ++i) {
		if (val != 0) {
			writer.writeBits(uint8_t(1), 1);
			writer.writeBits(val & 0xFF, 8);
		} else {
			writer.writeBits(uint8_t(0), 1);
			break;
		}
		val >>= 8;
	}
}

template <typename ReaderType>
inline error::Error var_decode_uint(uint64_t& val, ReaderType& reader, uint32_t num_bytes)
{
	val = 0;
	for (uint32_t i = 0; i < num_bytes; ++i) {
		uint8_t flag = 0;
		auto err = reader.readBits(flag, 1);
		if (err) {
			return err;
		}

		if (!flag) {
			break;
		}

		uint8_t byte = 0;

		err = reader.readBits(byte, 8);
		if (err) {
			return err;
		}

		val |= static_cast<uint64_t>(byte) << (8 * i);
	}
	return nullptr;
}


inline constexpr uint64_t zigzagEncode(int64_t val)
{
	return (static_cast<uint64_t>(val) << 1) ^ static_cast<uint64_t>(val>>63);
}

inline constexpr int64_t zigzagDecode(uint64_t encoded )
{
	return static_cast<int64_t>((encoded >> 1) ^ (-(encoded & 1)));
}

inline constexpr uint32_t var_int_bit_size(int64_t val, uint32_t num_bytes)
{
	uint64_t uv = 0;
	if (val < 0) {
		uv = zigzagEncode(val);
	} else {
		uv = val;
	}

	return 1 + var_uint_bit_size(uv, num_bytes);
}

template <typename WriterType>
inline void var_encode_int(WriterType& writer, int64_t val, uint32_t num_bytes)
{
	uint64_t uv = 0;
	if (val < 0) {
		uv = zigzagEncode(val);
		writer.writeBits(uint8_t(1), 1);
	} else {
		uv = val;
		writer.writeBits(uint8_t(0), 1);
	}

	var_encode_uint(writer, uv, num_bytes);
}

template <typename ReaderType>
inline error::Error var_decode_int(int64_t& val, ReaderType& reader, uint32_t num_bytes)
{
	uint8_t sign = 0;
	auto err = reader.readBits(sign, 1);
	if (err) {
		return err;
	}

	uint64_t uv = 0;
	err = var_decode_uint(uv, reader, num_bytes);
	if (err) {
		return err;
	}

	if (sign) {
		val = zigzagDecode(uv);
	} else {
		val = uv;
	}

	return nullptr;
}


static_assert(std::numeric_limits<float32_t>::is_iec559, "Requires IEEE-754 float32_t");
static_assert(std::numeric_limits<float64_t>::is_iec559, "Requires IEEE-754 float64_t");
static_assert(sizeof(float32_t) == 4, "float32_t must be 32-bit");
static_assert(sizeof(float64_t) == 8, "float64_t must be 64-bit");

inline std::array<std::uint8_t, 4> pack_float32(float32_t f)
{
	std::uint32_t bits;
	std::memcpy(&bits, &f, sizeof(bits));

	return {
		static_cast<uint8_t>(bits >> 24),
		static_cast<uint8_t>(bits >> 16),
		static_cast<uint8_t>(bits >> 8),
		static_cast<uint8_t>(bits)
	};
}

inline std::array<std::uint8_t, 8> pack_float64(float64_t f)
{
	std::uint64_t bits;
	std::memcpy(&bits, &f, sizeof(bits));

	return {
		static_cast<uint8_t>(bits >> 56),
		static_cast<uint8_t>(bits >> 48),
		static_cast<uint8_t>(bits >> 40),
		static_cast<uint8_t>(bits >> 32),
		static_cast<uint8_t>(bits >> 24),
		static_cast<uint8_t>(bits >> 16),
		static_cast<uint8_t>(bits >> 8),
		static_cast<uint8_t>(bits)
	};
}

inline float32_t unpack_float32(const std::uint8_t* b)
{
	std::uint32_t bits =
		(std::uint32_t(b[0]) << 24) |
		(std::uint32_t(b[1]) << 16) |
		(std::uint32_t(b[2]) << 8)  |
		(std::uint32_t(b[3]));

	float32_t f;
	std::memcpy(&f, &bits, sizeof(f));
	return f;
}

inline float64_t unpack_float64(const std::uint8_t* b)
{
	std::uint64_t bits =
		(std::uint64_t(b[0]) << 56) |
		(std::uint64_t(b[1]) << 48) |
		(std::uint64_t(b[2]) << 40) |
		(std::uint64_t(b[3]) << 32) |
		(std::uint64_t(b[4]) << 24) |
		(std::uint64_t(b[5]) << 16) |
		(std::uint64_t(b[6]) << 8)  |
		(std::uint64_t(b[7]));

	float64_t d;
	std::memcpy(&d, &bits, sizeof(d));
	return d;
}

}
}
