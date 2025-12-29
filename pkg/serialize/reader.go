package serialize

import (
	"encoding/binary"
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

func errInsufficientData(available, needed int) error {
	return fmt.Errorf("Reader does not contain enough data to fill the argument, num bytes available: %d, num bytes needed: %d", available, needed)
}

func (r *Reader) ReadBits(data *byte, numBitsToRead uint32) error {
	if numBitsToRead == 0 {
		*data = 0
		return nil
	}

	srcByteIndex := r.numBitsRead >> 3
	srcBitOffset := r.numBitsRead & 7

	if int(srcByteIndex) >= len(r.bytes) {
		return errInsufficientData(len(r.bytes), int(srcByteIndex)+1)
	}

	// Read from first byte
	val := r.bytes[srcByteIndex] >> srcBitOffset

	bitsInFirstByte := 8 - srcBitOffset

	if numBitsToRead > bitsInFirstByte {
		// Spans two bytes
		if int(srcByteIndex)+1 >= len(r.bytes) {
			return errInsufficientData(len(r.bytes), int(srcByteIndex)+2)
		}
		// Read second part
		val |= r.bytes[srcByteIndex+1] << bitsInFirstByte
	}

	*data = val & byte((1<<numBitsToRead)-1)
	r.numBitsRead += numBitsToRead
	return nil
}

func (r *Reader) ReadByte(data *byte) error {
	if r.numBitsRead&7 == 0 {
		byteIndex := r.numBitsRead >> 3
		if int(byteIndex) >= len(r.bytes) {
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

	if r.numBitsRead&7 == 0 {
		byteIndex := r.numBitsRead >> 3
		neededBytes := int(byteIndex) + len(data)
		if neededBytes > len(r.bytes) {
			return errInsufficientData(len(r.bytes), neededBytes)
		}
		copy(data, r.bytes[byteIndex:])
		r.numBitsRead += uint32(len(data) * 8)
		return nil
	}

	// Unaligned read optimization
	shift := r.numBitsRead & 7
	invShift := 8 - shift
	byteIndex := r.numBitsRead >> 3

	totalBits := r.numBitsRead + uint32(len(data)*8)
	neededBytes := (totalBits + 7) / 8
	if neededBytes > uint32(len(r.bytes)) {
		return errInsufficientData(len(r.bytes), int(neededBytes))
	}

	i := 0
	// Process 8 bytes at a time using encoding/binary
	// We need to read 9 bytes to get 8 bytes of result (due to shift)
	// So we need len(r.bytes) >= byteIndex + 9
	for i <= len(data)-8 && int(byteIndex)+9 <= len(r.bytes) {
		// Read 64 bits from current position
		val1 := binary.LittleEndian.Uint64(r.bytes[byteIndex:])
		// Read the 9th byte (spillover)
		val2 := uint64(r.bytes[byteIndex+8])

		// Combine
		result := (val1 >> shift) | (val2 << (64 - shift))

		// Write result to data
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
	r.numBitsRead += uint32(len(data) * 8)
	return nil
}
