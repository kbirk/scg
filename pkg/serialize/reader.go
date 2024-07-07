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
	totalBitsToRead := numBitsToRead

	*data = 0

	for numBitsToRead > 0 {
		srcByteIndex := getByteOffset(r.numBitsRead)
		srcBitIndex := getBitOffset(r.numBitsRead)
		dstBitIndex := getBitOffset(totalBitsToRead - numBitsToRead)
		srcMask := uint32(1) << srcBitIndex
		dstMask := uint32(1) << dstBitIndex

		if srcByteIndex >= uint32(len(r.bytes)) {
			return fmt.Errorf("Reader does not contain enough data to fill the argument, num bytes available: %d, num bytes needed: %d", len(r.bytes), srcByteIndex+1)
		}
		valByte := r.bytes[srcByteIndex]

		if valByte&byte(srcMask) != 0 {
			*data |= byte(dstMask)
		}
		r.numBitsRead++
		numBitsToRead--
	}

	return nil
}
