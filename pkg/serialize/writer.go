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
	if numBitsToWrite == 0 {
		return
	}

	// Mask val to ensure we only write numBitsToWrite bits
	val &= (1 << numBitsToWrite) - 1

	dstByteIndex := w.numBitsWritten >> 3
	dstBitOffset := w.numBitsWritten & 7

	if int(dstByteIndex) >= len(w.bytes) {
		panic(fmt.Sprintf("Invalid destination byte index: %d >= %d", dstByteIndex, len(w.bytes)))
	}

	// Calculate how many bits fit in the current byte
	bitsInFirstByte := 8 - dstBitOffset

	if numBitsToWrite <= bitsInFirstByte {
		// Fits in one byte
		w.bytes[dstByteIndex] |= val << dstBitOffset
	} else {
		// Spans two bytes
		// Write first part
		w.bytes[dstByteIndex] |= val << dstBitOffset

		// Write second part
		if int(dstByteIndex)+1 >= len(w.bytes) {
			panic(fmt.Sprintf("Invalid destination byte index: %d >= %d", dstByteIndex+1, len(w.bytes)))
		}
		w.bytes[dstByteIndex+1] |= val >> bitsInFirstByte
	}

	w.numBitsWritten += numBitsToWrite
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
	if numBitsToWrite == 0 {
		return
	}

	// Ensure capacity
	neededBytes := (w.numBitsWritten + numBitsToWrite + 7) / 8
	for uint32(len(w.bytes)) < neededBytes {
		w.bytes = append(w.bytes, 0)
	}

	// Mask val to ensure we only write numBitsToWrite bits
	val &= (1 << numBitsToWrite) - 1

	dstByteIndex := w.numBitsWritten >> 3
	dstBitOffset := w.numBitsWritten & 7

	// Calculate how many bits fit in the current byte
	bitsInFirstByte := 8 - dstBitOffset

	if numBitsToWrite <= bitsInFirstByte {
		// Fits in one byte
		w.bytes[dstByteIndex] |= val << dstBitOffset
	} else {
		// Spans two bytes
		// Write first part
		w.bytes[dstByteIndex] |= val << dstBitOffset

		// Write second part
		w.bytes[dstByteIndex+1] |= val >> bitsInFirstByte
	}

	w.numBitsWritten += numBitsToWrite
}

func (w *Writer) Bytes() []byte {
	return w.bytes
}
