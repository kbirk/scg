package serialize

import "math"

func BitsToBytes(x int) int {
	return int(math.Ceil(float64(x) / 8.0))
}

func BytesToBits(x int) int {
	return x << 3 // same as (x * 8)
}

func getByteOffset(x uint32) uint32 {
	return x >> 3 // same as (x / 8)
}

func getBitOffset(x uint32) uint8 {
	return uint8(x & 0x7) // same as (x % 8)
}

func varUintBitSize(val uint64, numBytes uint32) uint32 {
	size := uint32(0)
	for i := uint32(0); i < numBytes; i++ {
		if val != 0 {
			size += 9
		} else {
			size += 1
			break
		}
		val >>= 8
	}
	return size
}

func varEncodeUint(writer *Writer, val uint64, numBytes uint32) {
	for i := uint32(0); i < numBytes; i++ {
		if val != 0 {
			writer.WriteBits(1, 1)
			writer.WriteBits(uint8(val), 8)
		} else {
			writer.WriteBits(0, 1)
			return
		}
		val >>= 8
	}
}

func varDecodeUint(reader *Reader, val *uint64, numBytes uint32) error {
	*val = 0
	for i := uint32(0); i < numBytes; i++ {
		var flag byte
		if err := reader.ReadBits(&flag, 1); err != nil {
			return err
		}

		if flag == 0 {
			break
		}

		var b byte
		if err := reader.ReadBits(&b, 8); err != nil {
			return err
		}

		*val |= uint64(b) << (8 * i)
	}
	return nil
}

func zigzagEncode(val int64) uint64 {
	return (uint64(val) << 1) ^ uint64(val>>63)
}

func zigzagDecode(encoded uint64) int64 {
	return int64((encoded >> 1) ^ (-(encoded & 1)))
}

func varIntBitSize(val int64, numBytes uint32) uint32 {
	unsignedVal := uint64(val)
	if val < 0 {
		unsignedVal = zigzagEncode(val)
	}

	return 1 + varUintBitSize(unsignedVal, numBytes)
}

func varEncodeInt(writer *Writer, val int64, numBytes uint32) {
	unsignedVal := uint64(val)
	if val < 0 {
		writer.WriteBits(1, 1)
		unsignedVal = zigzagEncode(val)
	} else {
		writer.WriteBits(0, 1)
	}

	varEncodeUint(writer, unsignedVal, numBytes)
}

func varDecodeInt(reader *Reader, val *int64, numBytes uint32) error {

	var sign byte
	if err := reader.ReadBits(&sign, 1); err != nil {
		return err
	}

	unsignedVal := uint64(0)

	err := varDecodeUint(reader, &unsignedVal, numBytes)
	if err != nil {
		return err
	}

	if sign == 1 {
		*val = zigzagDecode(unsignedVal)
	} else {
		*val = int64(unsignedVal)
	}

	return nil
}

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
