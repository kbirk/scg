package serialize

import (
	"encoding/binary"
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

func panicInsufficientCapacity(needed, have int) {
	panic(fmt.Sprintf("insufficient capacity: need %d bytes, have %d", needed, have))
}

func (w *Writer) WriteBits(val uint8, numBitsToWrite uint32) {
	if numBitsToWrite == 0 {
		return
	}

	// Check capacity before writing
	neededBytes := (w.numBitsWritten + numBitsToWrite + 7) / 8
	if neededBytes > uint32(len(w.bytes)) {
		panicInsufficientCapacity(int(neededBytes), len(w.bytes))
	}

	// Mask val to ensure we only write numBitsToWrite bits
	val &= (1 << numBitsToWrite) - 1

	dstByteIndex := w.numBitsWritten >> 3
	dstBitOffset := w.numBitsWritten & 7

	// Always write to the first byte
	w.bytes[dstByteIndex] |= val << dstBitOffset

	// Calculate how many bits fit in the current byte
	bitsInFirstByte := 8 - dstBitOffset

	if numBitsToWrite > bitsInFirstByte {
		// Spans two bytes, write second part
		w.bytes[dstByteIndex+1] |= val >> bitsInFirstByte
	}

	w.numBitsWritten += numBitsToWrite
}

func (w *Writer) WriteByte(val byte) {
	if w.numBitsWritten&7 == 0 {
		byteIndex := w.numBitsWritten >> 3
		if int(byteIndex)+1 > len(w.bytes) {
			panicInsufficientCapacity(len(w.bytes), int(byteIndex)+1)
		}
		w.bytes[byteIndex] = val
		w.numBitsWritten += 8
	} else {
		w.WriteBits(val, 8)
	}
}

func (w *Writer) WriteBytes(data []byte) {
	if len(data) == 0 {
		return
	}

	if w.numBitsWritten&7 == 0 {
		byteIndex := w.numBitsWritten >> 3
		neededBytes := int(byteIndex) + len(data)
		if neededBytes > len(w.bytes) {
			panicInsufficientCapacity(len(w.bytes), neededBytes)
		}
		copy(w.bytes[byteIndex:], data)
		w.numBitsWritten += uint32(len(data) * 8)
	} else {
		// Unaligned write optimization
		bitOffset := w.numBitsWritten & 7
		byteIndex := w.numBitsWritten >> 3

		totalBits := w.numBitsWritten + uint32(len(data)*8)
		neededBytes := (totalBits + 7) / 8
		if neededBytes > uint32(len(w.bytes)) {
			panicInsufficientCapacity(len(w.bytes), int(neededBytes))
		}

		// Try to write 8 bytes at a time using encoding/binary for safety and portability
		// We need at least 8 bytes in input and enough space in output

		i := 0
		for i <= len(data)-8 && int(byteIndex)+9 <= len(w.bytes) {
			val := binary.LittleEndian.Uint64(data[i:])

			// Write to current position
			// We need to read existing data to OR with it (in case of overlap with previous write)
			current := binary.LittleEndian.Uint64(w.bytes[byteIndex:])
			current |= val << bitOffset
			binary.LittleEndian.PutUint64(w.bytes[byteIndex:], current)

			// Write spillover to the 9th byte
			w.bytes[byteIndex+8] |= byte(val >> (64 - bitOffset))

			byteIndex += 8
			i += 8
		}

		shift := bitOffset
		invShift := 8 - shift

		for ; i < len(data); i++ {
			b := data[i]
			w.bytes[byteIndex] |= b << shift
			byteIndex++
			w.bytes[byteIndex] |= b >> invShift
		}
		w.numBitsWritten += uint32(len(data) * 8)
	}
}

func (w *Writer) Bytes() []byte {
	return w.bytes[:BitsToBytes(int(w.numBitsWritten))]
}

// Reset clears the writer for reuse
func (w *Writer) Reset() {
	w.numBitsWritten = 0
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
