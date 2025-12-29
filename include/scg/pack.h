#pragma once

#include <cstdint>
#include <cmath>
#include <cstring>
#include <limits>

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

/**
 * Bit packing macros
 * Brian "Beej Jorgensen" Hall
 * http://beej.us/guide/bgnet/output/html/singlepage/bgnet.html#serialization
 */
#define pack754_32(f) (static_cast<uint32_t>(pack754((f), 32, 8)))
#define pack754_64(f) (pack754((f), 64, 11))
#define unpack754_32(i) (static_cast<float32_t>(unpack754((i), 32, 8)))
#define unpack754_64(i) (unpack754((i), 64, 11))

inline constexpr uint64_t pack754(float64_t f, uint32_t bits, uint32_t expbits)
{
	float64_t fnorm = 0;
	int32_t shift = 0;
	int64_t sign = 0;
	int64_t expo = 0;
	int64_t significand = 0;
	uint32_t significandbits = bits - expbits - 1; // -1 for sign bit
	if (f == 0.0) {
		// get this special case out of the way
		return 0;
	}
	// check sign and begin normalization
	if (f < 0) {
		sign = 1;
		fnorm = -f;
	} else {
		sign = 0;
		fnorm = f;
	}
	// get the normalized form of f and track the exponent
	shift = 0;
	while (fnorm >= 2.0) {
		fnorm /= 2.0;
		shift++;
	}
	while (fnorm < 1.0) {
		fnorm *= 2.0;
		shift--;
	}
	fnorm = fnorm - 1.0;
	// calculate the binary form (non-float) of the significand data
	significand = fnorm * ((1LL << significandbits) + 0.5f);
	// get the biased exponent
	expo = shift + ((1 << (expbits - 1)) - 1); // shift + bias
	// return the final answer
	return (sign << (bits - 1)) | (expo << (bits - expbits - 1)) | significand;
}

inline constexpr float64_t unpack754(uint64_t i, uint32_t bits, uint32_t expbits)
{
	float64_t result = 0;
	int64_t shift = 0;
	uint32_t bias = 0;
	uint32_t significandbits = bits - expbits - 1; // -1 for sign bit
	if (i == 0) {
		// get this special case out of the way
		return 0.0;
	}
	// pull the significand
	result = (i & ((1LL << significandbits) - 1)); // mask
	result /= (1LL << significandbits); // convert back to float
	result += 1.0f; // add the one back on
	// deal with the exponent
	bias = (1 << (expbits - 1)) - 1;
	shift = ((i >> significandbits) & ((1LL << expbits) - 1)) - bias;
	while (shift > 0) {
		result *= 2.0;
		shift--;
	}
	while (shift < 0) {
		result /= 2.0;
		shift++;
	}
	// sign it
	result *= (i >> (bits - 1)) & 1 ? -1.0 : 1.0;
	return result;
}

}
}
