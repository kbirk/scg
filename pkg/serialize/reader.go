package serialize

import (
	"fmt"
)

type Reader struct {
	bytes       []byte
	numBitsRead uint32
}

func NewReader(data []byte) *Reader {
	return &Reader{
		bytes: data,
	}
}

func (r *Reader) ReadBits(data *byte, numBitsToRead uint32) error {
	if numBitsToRead == 0 {
		*data = 0
		return nil
	}

	srcByteIndex := r.numBitsRead >> 3
	srcBitOffset := r.numBitsRead & 7

	if int(srcByteIndex) >= len(r.bytes) {
		return fmt.Errorf("Reader does not contain enough data to fill the argument, num bytes available: %d, num bytes needed: %d", len(r.bytes), srcByteIndex+1)
	}

	bitsInFirstByte := 8 - srcBitOffset

	if numBitsToRead <= bitsInFirstByte {
		// Fits in one byte
		val := r.bytes[srcByteIndex] >> srcBitOffset
		mask := byte((1 << numBitsToRead) - 1)
		*data = val & mask
	} else {
		// Spans two bytes
		if int(srcByteIndex)+1 >= len(r.bytes) {
			return fmt.Errorf("Reader does not contain enough data to fill the argument, num bytes available: %d, num bytes needed: %d", len(r.bytes), srcByteIndex+2)
		}

		// Read first part
		val1 := r.bytes[srcByteIndex] >> srcBitOffset
		// Read second part
		val2 := r.bytes[srcByteIndex+1]

		mask2 := byte((1 << (numBitsToRead - bitsInFirstByte)) - 1)
		*data = val1 | ((val2 & mask2) << bitsInFirstByte)
	}

	r.numBitsRead += numBitsToRead
	return nil
}
