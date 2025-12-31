package serialize

import (
	"encoding/binary"
	"math"
)

func BitsToBytes(x int) int {
	return int(math.Ceil(float64(x) / 8.0))
}

func BytesToBits(x int) int {
	return x << 3 // same as (x * 8)
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
