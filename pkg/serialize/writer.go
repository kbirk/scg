package serialize

import (
	"encoding/binary"
)

// Writer is a bit-packed, LSB-first serializer. Invariant: every byte at or
// beyond BitsToBytes(numBitsWritten) is zero, which lets writes OR into the
// buffer without clearing first and lets Reset clear only the used prefix.
type Writer struct {
	bytes []byte
	// numBitsWritten is 64-bit so messages larger than 512 MiB cannot silently
	// wrap the bit cursor and corrupt earlier bytes.
	numBitsWritten uint64
}

func NewWriter(size int) *Writer {
	return &Writer{
		bytes: make([]byte, size),
	}
}

// ensureCapacity grows the buffer (amortized doubling) so it holds at least
// neededBytes bytes. New bytes are zero, preserving the writer invariant.
func (w *Writer) ensureCapacity(neededBytes uint64) {
	if neededBytes > uint64(len(w.bytes)) {
		w.grow(neededBytes)
	}
}

func (w *Writer) grow(neededBytes uint64) {
	newSize := uint64(len(w.bytes)) * 2
	if neededBytes > newSize {
		newSize = neededBytes
	}
	newBytes := make([]byte, newSize)
	copy(newBytes, w.bytes)
	w.bytes = newBytes
}

func (w *Writer) WriteBits(val uint8, numBitsToWrite uint32) {
	pos := w.numBitsWritten
	idx := pos >> 3
	b := w.bytes
	// Mask val to ensure we only write numBitsToWrite bits
	val &= (1 << numBitsToWrite) - 1
	if numBitsToWrite <= 8 && idx+1 < uint64(len(b)) {
		// Fast path: a 16-bit window covers any <= 8-bit write at any offset.
		// (Wider counts take the slow path, which grows the buffer for the
		// full advance exactly as the original implementation did.)
		v := uint16(val) << (pos & 7)
		b[idx] |= byte(v)
		b[idx+1] |= byte(v >> 8)
		w.numBitsWritten = pos + uint64(numBitsToWrite)
		return
	}
	w.writeBitsSlow(val, numBitsToWrite)
}

func (w *Writer) writeBitsSlow(val uint8, numBitsToWrite uint32) {
	if numBitsToWrite == 0 {
		return
	}

	pos := w.numBitsWritten
	neededBytes := (pos + uint64(numBitsToWrite) + 7) >> 3
	if neededBytes > uint64(len(w.bytes)) {
		w.grow(neededBytes)
	}

	dstByteIndex := pos >> 3
	dstBitOffset := pos & 7

	w.bytes[dstByteIndex] |= val << dstBitOffset
	if uint64(numBitsToWrite) > 8-dstBitOffset {
		w.bytes[dstByteIndex+1] |= val >> (8 - dstBitOffset)
	}

	w.numBitsWritten = pos + uint64(numBitsToWrite)
}

// writeBits64 writes the low numBits (<= 64) bits of val, LSB-first. val must
// have no bits set at or above numBits. It is the building block for varints
// and fixed-width scalars: one capacity check and, when the buffer has room,
// a single unaligned 64-bit read-modify-write instead of per-bit calls.
func (w *Writer) writeBits64(val uint64, numBits uint) {
	if numBits == 0 {
		return
	}
	pos := w.numBitsWritten
	end := pos + uint64(numBits)
	neededBytes := (end + 7) >> 3
	if neededBytes > uint64(len(w.bytes)) {
		w.grow(neededBytes)
	}

	idx := pos >> 3
	off := uint(pos & 7)
	b := w.bytes

	if idx+8 <= uint64(len(b)) {
		cur := binary.LittleEndian.Uint64(b[idx:])
		binary.LittleEndian.PutUint64(b[idx:], cur|val<<off)
		if off != 0 && numBits+off > 64 {
			// The top `off` bits of val spill into the ninth byte, which is
			// within neededBytes so it exists.
			b[idx+8] |= byte(val >> (64 - off))
		}
	} else if numBits+off <= 32 && idx+4 <= uint64(len(b)) {
		cur := binary.LittleEndian.Uint32(b[idx:])
		binary.LittleEndian.PutUint32(b[idx:], cur|uint32(val<<off))
	} else {
		// Near the end of an exactly-sized buffer: byte-at-a-time.
		b[idx] |= byte(val << off)
		consumed := 8 - off
		for consumed < numBits {
			idx++
			b[idx] |= byte(val >> consumed)
			consumed += 8
		}
	}
	w.numBitsWritten = end
}

func (w *Writer) WriteByte(val byte) {
	pos := w.numBitsWritten
	if idx := int(pos >> 3); pos&7 == 0 && idx >= 0 && idx < len(w.bytes) {
		w.bytes[idx] = val
		w.numBitsWritten = pos + 8
		return
	}
	w.writeByteSlow(val)
}

func (w *Writer) writeByteSlow(val byte) {
	if w.numBitsWritten&7 == 0 {
		byteIndex := w.numBitsWritten >> 3
		w.grow(byteIndex + 1)
		w.bytes[byteIndex] = val
		w.numBitsWritten += 8
		return
	}
	w.WriteBits(val, 8)
}

func (w *Writer) WriteBytes(data []byte) {
	if len(data) == 0 {
		return
	}

	if pos := w.numBitsWritten; pos&7 == 0 {
		byteIndex := int(pos >> 3)
		if byteIndex+len(data) > len(w.bytes) {
			w.grow(uint64(byteIndex + len(data)))
		}
		copy(w.bytes[byteIndex:], data)
		w.numBitsWritten = pos + uint64(len(data))*8
		return
	}

	n := uint64(len(data))

	// Unaligned write optimization
	bitOffset := w.numBitsWritten & 7
	byteIndex := w.numBitsWritten >> 3

	totalBits := w.numBitsWritten + n*8
	neededBytes := (totalBits + 7) / 8
	if neededBytes > uint64(len(w.bytes)) {
		w.grow(neededBytes)
	}

	// Write 8 bytes at a time while there is room for the 9-byte footprint.
	i := 0
	for i <= len(data)-8 && byteIndex+9 <= uint64(len(w.bytes)) {
		val := binary.LittleEndian.Uint64(data[i:])

		// OR with existing data (the current byte holds earlier bits)
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
	w.numBitsWritten = totalBits
}

// Bytes returns the written bytes. The slice's capacity is capped at its
// length: appending to it must not write into the writer's spare capacity,
// which Reset (clearing only the used prefix) would otherwise leak into the
// next message serialized with a pooled writer.
func (w *Writer) Bytes() []byte {
	n := (w.numBitsWritten + 7) >> 3
	return w.bytes[:n:n]
}

// Reset clears the writer for reuse. Only the used prefix can be non-zero, so
// only that prefix is cleared (O(message) rather than O(capacity)).
func (w *Writer) Reset() {
	n := (w.numBitsWritten + 7) >> 3
	w.numBitsWritten = 0
	if n >= uint64(len(w.bytes)) {
		// Fully used: clear the whole slice. This form compiles to a single
		// memclr of len(w.bytes) with no slicing, which benchmarks measurably
		// faster than clearing an explicit prefix of the same length.
		for i := range w.bytes {
			w.bytes[i] = 0
		}
		return
	}
	clear(w.bytes[:n])
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
