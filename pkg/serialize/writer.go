package serialize

import (
	"fmt"
)

type FixedSizeWriter struct {
	bytes          []byte
	numBitsWritten uint32
}

func NewFixedSizeWriter(size int) *FixedSizeWriter {
	return &FixedSizeWriter{
		bytes: make([]byte, size),
	}
}

func (w *FixedSizeWriter) WriteBits(val uint8, numBitsToWrite uint32) {
	totalBitsToWrite := numBitsToWrite

	for numBitsToWrite > 0 {
		dstByteIndex := getByteOffset(w.numBitsWritten)
		dstBitIndex := getBitOffset(w.numBitsWritten)
		srcBitIndex := getBitOffset(totalBitsToWrite - numBitsToWrite)

		if srcBitIndex > 7 {
			panic("Invalid source bit index")
		}
		if dstBitIndex > 7 {
			panic("Invalid destination bit index")
		}
		if dstByteIndex >= uint32(len(w.bytes)) {
			panic(fmt.Sprintf("Invalid destination byte index: %d >= %d", dstByteIndex, len(w.bytes)))
		}

		srcMask := uint8(1) << srcBitIndex
		dstMask := uint8(1) << dstBitIndex

		if val&srcMask != 0 {
			w.bytes[dstByteIndex] |= dstMask
		}

		w.numBitsWritten++
		numBitsToWrite--
	}
}

func (w *FixedSizeWriter) Bytes() []byte {
	if BitsToBytes(int(w.numBitsWritten)) != len(w.bytes) {
		panic(fmt.Sprintf("leftover space, missing %d bytes, using only %d of %d", +len(w.bytes)-BitsToBytes(int(w.numBitsWritten)), BitsToBytes(int(w.numBitsWritten)), len(w.bytes)))
	}
	return w.bytes
}

type Writer struct {
	bytes          []byte
	numBitsWritten uint32
}

func NewWriter(bs []byte) *Writer {
	return &Writer{
		bytes: bs,
	}
}

func (w *Writer) WriteBits(val uint8, numBitsToWrite uint32) {
	totalBitsToWrite := numBitsToWrite

	for numBitsToWrite > 0 {
		dstByteIndex := getByteOffset(w.numBitsWritten)
		dstBitIndex := getBitOffset(w.numBitsWritten)
		srcBitIndex := getBitOffset(totalBitsToWrite - numBitsToWrite)

		if srcBitIndex > 7 {
			panic("Invalid source bit index")
		}
		if dstBitIndex > 7 {
			panic("Invalid destination bit index")
		}
		if dstByteIndex >= uint32(len(w.bytes)) {
			w.bytes = append(w.bytes, 0)
		}

		srcMask := uint8(1) << srcBitIndex
		dstMask := uint8(1) << dstBitIndex

		if val&srcMask != 0 {
			w.bytes[dstByteIndex] |= dstMask
		} else {
			w.bytes[dstByteIndex] |= 0x00
		}

		w.numBitsWritten++
		numBitsToWrite--
	}
}

func (w *Writer) Bytes() []byte {
	return w.bytes
}
