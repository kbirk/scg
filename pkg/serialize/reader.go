package serialize

import (
	"encoding/binary"
	"fmt"
)

type Reader struct {
	bytes []byte
	// numBitsRead is 64-bit so buffers larger than 512 MiB cannot wrap the
	// cursor and silently re-read earlier data.
	numBitsRead uint64
}

func NewReader(data []byte) *Reader {
	return &Reader{
		bytes: data,
	}
}

// RemainingBytes returns the number of unread bytes still available, counting
// the current partially-read byte as available. It bounds length-prefixed
// allocations against the data actually present so a hostile declared length
// cannot force a huge allocation before the bytes are read.
func (r *Reader) RemainingBytes() int {
	consumed := r.numBitsRead >> 3
	if consumed >= uint64(len(r.bytes)) {
		return 0
	}
	return int(uint64(len(r.bytes)) - consumed)
}

// RemainingBits returns the exact number of unread bits.
func (r *Reader) RemainingBits() uint64 {
	total := uint64(len(r.bytes)) * 8
	if r.numBitsRead >= total {
		return 0
	}
	return total - r.numBitsRead
}

func errInsufficientData(available, needed int) error {
	return fmt.Errorf("Reader does not contain enough data to fill the argument, num bytes available: %d, num bytes needed: %d", available, needed)
}

func (r *Reader) ReadBits(data *byte, numBitsToRead uint32) error {
	pos := r.numBitsRead
	b := r.bytes
	if idx := int(pos >> 3); numBitsToRead <= 8 && idx >= 0 && idx < len(b)-1 {
		// Fast path: two bytes available, so any <= 8-bit read at any offset
		// can be extracted from a 16-bit window without branching on spans.
		v := uint16(b[idx]) | uint16(b[idx+1])<<8
		*data = byte(v>>(pos&7)) & byte((1<<numBitsToRead)-1)
		r.numBitsRead = pos + uint64(numBitsToRead)
		return nil
	}
	return r.readBitsSlow(data, numBitsToRead)
}

// readBitsSlow is the general path (final byte, counts above 8, and the error
// cases), with the original implementation's semantics.
func (r *Reader) readBitsSlow(data *byte, numBitsToRead uint32) error {
	if numBitsToRead == 0 {
		*data = 0
		return nil
	}

	srcByteIndex := r.numBitsRead >> 3
	srcBitOffset := r.numBitsRead & 7
	n := uint64(len(r.bytes))

	if srcByteIndex >= n {
		return errInsufficientData(len(r.bytes), int(srcByteIndex)+1)
	}

	val := r.bytes[srcByteIndex] >> srcBitOffset
	bitsInFirstByte := 8 - srcBitOffset
	if uint64(numBitsToRead) > bitsInFirstByte {
		// Spans two bytes
		if srcByteIndex+1 >= n {
			return errInsufficientData(len(r.bytes), int(srcByteIndex)+2)
		}
		val |= r.bytes[srcByteIndex+1] << bitsInFirstByte
	}

	*data = val & byte((1<<numBitsToRead)-1)
	r.numBitsRead += uint64(numBitsToRead)
	return nil
}

// readBits64 reads numBits (<= 64) bits LSB-first, the inverse of
// Writer.writeBits64.
func (r *Reader) readBits64(numBits uint) (uint64, error) {
	if numBits == 0 {
		return 0, nil
	}
	pos := r.numBitsRead
	end := pos + uint64(numBits)
	n := uint64(len(r.bytes))
	neededBytes := (end + 7) >> 3
	if neededBytes > n {
		return 0, errInsufficientData(len(r.bytes), int(neededBytes))
	}

	idx := pos >> 3
	off := uint(pos & 7)
	b := r.bytes
	var val uint64

	if idx+8 <= n {
		val = binary.LittleEndian.Uint64(b[idx:]) >> off
		if off != 0 && numBits+off > 64 {
			val |= uint64(b[idx+8]) << (64 - off)
		}
	} else if numBits+off <= 32 && idx+4 <= n {
		val = uint64(binary.LittleEndian.Uint32(b[idx:]) >> off)
	} else {
		val = uint64(b[idx]) >> off
		consumed := 8 - off
		for consumed < numBits {
			idx++
			val |= uint64(b[idx]) << consumed
			consumed += 8
		}
	}
	if numBits < 64 {
		val &= (uint64(1) << numBits) - 1
	}
	r.numBitsRead = end
	return val, nil
}

func (r *Reader) ReadByte(data *byte) error {
	if r.numBitsRead&7 == 0 {
		byteIndex := r.numBitsRead >> 3
		if byteIndex >= uint64(len(r.bytes)) {
			return errInsufficientData(len(r.bytes), int(byteIndex)+1)
		}
		*data = r.bytes[byteIndex]
		r.numBitsRead += 8
		return nil
	}
	return r.ReadBits(data, 8)
}

func (r *Reader) ReadBytes(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	if pos := r.numBitsRead; pos&7 == 0 {
		byteIndex := int(pos >> 3)
		if byteIndex < 0 || byteIndex+len(data) > len(r.bytes) {
			return errInsufficientData(len(r.bytes), byteIndex+len(data))
		}
		copy(data, r.bytes[byteIndex:])
		r.numBitsRead = pos + uint64(len(data))*8
		return nil
	}

	n := uint64(len(data))

	// Unaligned read optimization
	shift := r.numBitsRead & 7
	invShift := 8 - shift
	byteIndex := r.numBitsRead >> 3

	totalBits := r.numBitsRead + n*8
	neededBytes := (totalBits + 7) / 8
	if neededBytes > uint64(len(r.bytes)) {
		return errInsufficientData(len(r.bytes), int(neededBytes))
	}

	i := 0
	// Process 8 bytes at a time; reading 8 result bytes touches 9 source bytes.
	for i <= len(data)-8 && byteIndex+9 <= uint64(len(r.bytes)) {
		val1 := binary.LittleEndian.Uint64(r.bytes[byteIndex:])
		val2 := uint64(r.bytes[byteIndex+8])
		result := (val1 >> shift) | (val2 << (64 - shift))
		binary.LittleEndian.PutUint64(data[i:], result)
		byteIndex += 8
		i += 8
	}

	for ; i < len(data); i++ {
		val1 := r.bytes[byteIndex] >> shift
		val2 := r.bytes[byteIndex+1]
		data[i] = val1 | (val2 << invShift)
		byteIndex++
	}
	r.numBitsRead = totalBits
	return nil
}
