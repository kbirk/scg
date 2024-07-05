#pragma once

#include <cstdint>
#include <cmath>

typedef float float32_t;
typedef double float64_t;

namespace scg {
namespace serialize {

inline constexpr uint32_t bits_to_bytes(uint32_t x) {
    return static_cast<uint32_t>(std::ceil(x / 8.0f));
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
