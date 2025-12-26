package serialize

import (
	"fmt"
)

type Writer struct {
	bytes          []byte
	numBitsWritten uint32
}

func NewWriter(size int) *Writer {
	return &Writer{
		bytes: make([]byte, size),
	}
}

func (w *Writer) WriteBits(val uint8, numBitsToWrite uint32) {
	if numBitsToWrite == 0 {
		return
	}

	// Check capacity before writing
	neededBytes := (w.numBitsWritten + numBitsToWrite + 7) / 8
	if neededBytes > uint32(len(w.bytes)) {
		panic(fmt.Sprintf("insufficient capacity: need %d bytes, have %d", neededBytes, len(w.bytes)))
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
	return w.bytes[:BitsToBytes(int(w.numBitsWritten))]
}

// Reset clears the writer for reuse
func (w *Writer) Reset() {
	w.numBitsWritten = 0
	// Clear the byte array
	for i := range w.bytes {
		w.bytes[i] = 0
	}
}

// Grow ensures the writer has at least the specified capacity
// Call this before reusing a pooled writer
func (w *Writer) Grow(size int) {
	if len(w.bytes) < size {
		w.bytes = make([]byte, size)
		w.numBitsWritten = 0
	}
}

// Capacity returns the current capacity of the writer in bytes
func (w *Writer) Capacity() int {
	return len(w.bytes)
}
