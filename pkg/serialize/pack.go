package serialize

import (
	"encoding/binary"
	"math"
	"math/bits"
)

func BitsToBytes(x int) int {
	return (x + 7) >> 3
}

func BytesToBits(x int) int {
	return x << 3 // same as (x * 8)
}

// varSigBytes returns how many 9-bit units (continuation bit + data byte) the
// varint encoding of val emits when limited to numBytes units.
func varSigBytes(val uint64, numBytes uint32) uint32 {
	k := uint32(bits.Len64(val)+7) >> 3
	if k > numBytes {
		k = numBytes
	}
	return k
}

func varUintBitSize(val uint64, numBytes uint32) uint32 {
	k := varSigBytes(val, numBytes)
	size := 9 * k
	if k < numBytes {
		size++ // terminating zero continuation bit
	}
	return size
}

// varBits builds the bit pattern of a varint (optionally preceded by prefixLen
// prefix bits, used for the sign bit of signed varints) into a 128-bit value
// (lo, hi) and returns the total bit count. The encoding is: for each
// significant byte a 1 continuation bit followed by the 8 data bits, then a 0
// bit unless all numBytes units were used. Bit-identical to the original
// per-bit encoder.
func varBits(val uint64, numBytes uint32, prefix uint64, prefixLen uint) (lo, hi uint64, n uint) {
	k := varSigBytes(val, numBytes)
	lo = prefix
	pos := prefixLen
	for j := uint32(0); j < k; j++ {
		chunk := 1 | (val&0xFF)<<1 // 9 bits
		val >>= 8
		switch {
		case pos+9 <= 64:
			lo |= chunk << pos
		case pos >= 64:
			hi |= chunk << (pos - 64)
		default:
			lo |= chunk << pos
			hi |= chunk >> (64 - pos)
		}
		pos += 9
	}
	if k < numBytes {
		pos++ // terminating 0 bit (already zero)
	}
	return lo, hi, pos
}

func varEncodeUint(writer *Writer, val uint64, numBytes uint32) {
	if val == 0 {
		// Single 0 continuation bit. The writer invariant guarantees the bit is
		// already zero, so only capacity and the cursor need updating.
		pos := writer.numBitsWritten + 1
		if need := (pos + 7) >> 3; need > uint64(len(writer.bytes)) {
			writer.grow(need)
		}
		writer.numBitsWritten = pos
		return
	}
	lo, hi, n := varBits(val, numBytes, 0, 0)
	if n <= 64 {
		writer.writeBits64(lo, n)
		return
	}
	writer.writeBits64(lo, 64)
	writer.writeBits64(hi, n-64)
}

func varDecodeUint(reader *Reader, val *uint64, numBytes uint32) error {
	// Hot path: each unit is a 1-bit continuation flag followed (if set) by 8
	// data bits, bit-packed and usually unaligned. The bit extraction is
	// inlined against the backing slice rather than calling ReadBits per unit.
	data := reader.bytes
	n := uint64(len(data))
	pos := reader.numBitsRead
	var result uint64

	for i := uint32(0); i < numBytes; i++ {
		bi := pos >> 3
		if bi >= n {
			return errInsufficientData(int(n), int(bi)+1)
		}
		off := pos & 7
		if (data[bi]>>off)&1 == 0 {
			pos++
			break
		}
		pos++

		// Read 8 data bits at pos.
		bi = pos >> 3
		off = pos & 7
		if bi >= n {
			return errInsufficientData(int(n), int(bi)+1)
		}
		b := data[bi] >> off
		if off != 0 {
			if bi+1 >= n {
				return errInsufficientData(int(n), int(bi)+2)
			}
			b |= data[bi+1] << (8 - off)
		}
		pos += 8

		result |= uint64(b) << (8 * i)
	}

	*val = result
	reader.numBitsRead = pos
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
	// Sign bit first (1 = negative, zigzag-encoded magnitude follows), folded
	// into the same multi-bit write as the varint body.
	var sign uint64
	unsignedVal := uint64(val)
	if val < 0 {
		sign = 1
		unsignedVal = zigzagEncode(val)
	}
	lo, hi, n := varBits(unsignedVal, numBytes, sign, 1)
	if n <= 64 {
		writer.writeBits64(lo, n)
		return
	}
	writer.writeBits64(lo, 64)
	writer.writeBits64(hi, n-64)
}

func varDecodeInt(reader *Reader, val *int64, numBytes uint32) error {
	pos := reader.numBitsRead
	bi := pos >> 3
	if bi >= uint64(len(reader.bytes)) {
		return errInsufficientData(len(reader.bytes), int(bi)+1)
	}
	sign := (reader.bytes[bi] >> (pos & 7)) & 1
	reader.numBitsRead = pos + 1

	unsignedVal := uint64(0)
	if err := varDecodeUint(reader, &unsignedVal, numBytes); err != nil {
		reader.numBitsRead = pos
		return err
	}

	if sign == 1 {
		*val = zigzagDecode(unsignedVal)
	} else {
		*val = int64(unsignedVal)
	}

	return nil
}

func PackFloat32(f float32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, math.Float32bits(f))
	return b
}

func PackFloat64(f float64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, math.Float64bits(f))
	return b
}

func UnpackFloat32(b []byte) float32 {
	return math.Float32frombits(binary.BigEndian.Uint32(b))
}

func UnpackFloat64(b []byte) float64 {
	return math.Float64frombits(binary.BigEndian.Uint64(b))
}
