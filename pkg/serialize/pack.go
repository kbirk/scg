package serialize

func pack754(f float64, bits uint, expbits uint) uint64 {
	var fnorm float64
	var shift int32
	var sign, expo, significand int64
	significandbits := bits - expbits - 1 // -1 for sign bit
	if f == 0.0 {
		// get this special case out of the way
		return 0
	}
	// check sign and begin normalization
	if f < 0 {
		sign = 1
		fnorm = -f
	} else {
		sign = 0
		fnorm = f
	}
	// get the normalized form of f and track the exponent
	shift = 0
	for fnorm >= 2.0 {
		fnorm /= 2.0
		shift++
	}
	for fnorm < 1.0 {
		fnorm *= 2.0
		shift--
	}
	fnorm = fnorm - 1.0
	// calculate the binary form (non-float) of the significand data
	significand = int64(fnorm * (float64(int64(1)<<significandbits) + 0.5))
	// get the biased exponent
	expo = int64(shift) + ((1 << (expbits - 1)) - 1) // shift + bias
	// return the final answer
	return uint64((sign << (bits - 1)) | (expo << (bits - expbits - 1)) | significand)
}

func unpack754(i uint64, bits uint, expbits uint) float64 {
	var result float64
	var shift int64
	var bias uint32
	significandbits := bits - expbits - 1 // -1 for sign bit
	if i == 0 {
		// get this special case out of the way
		return 0.0
	}
	// pull the significand
	result = float64(i & ((1 << significandbits) - 1)) // mask
	result /= float64(int64(1) << significandbits)     // convert back to float
	result += 1.0                                      // add the one back on
	// deal with the exponent
	bias = (1 << (expbits - 1)) - 1
	shift = int64((i>>significandbits)&((1<<expbits)-1)) - int64(bias)
	for shift > 0 {
		result *= 2.0
		shift--
	}
	for shift < 0 {
		result /= 2.0
		shift++
	}
	// sign it
	if (i>>(bits-1))&1 == 1 {
		result *= -1.0
	}
	return result
}

func Pack754_32(f float32) uint32 {
	return uint32(pack754(float64(f), 32, 8))
}

func Pack754_64(f float64) uint64 {
	return pack754((f), 64, 11)
}

func Unpack754_32(i uint32) float32 {
	return float32(unpack754(uint64(i), 32, 8))
}

func Unpack754_64(i uint64) float64 {
	return unpack754((i), 64, 11)
}
