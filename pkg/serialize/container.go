package serialize

import (
	"fmt"
	"unsafe"
)

// This file holds helpers called by scg-generated container (list / map)
// code. It only uses the exported Reader / Writer API.

const (
	// containerPreallocBytesPerInputByte bounds the memory a generated decoder
	// preallocates for a container, before any element has been read, relative
	// to the bytes remaining in the frame. Containers needing more still decode;
	// they grow as their elements arrive.
	containerPreallocBytesPerInputByte = 16
	// containerPreallocMinBudget is the preallocation budget for small frames.
	containerPreallocMinBudget = 64 << 10
)

// containerCapacityHint validates a wire-declared element count against the
// data remaining in the reader and returns how many elements to preallocate.
//
// minElementBits is the minimum encoded size of one element (for maps, of one
// key plus one value), computed by the code generator from the schema. A count
// whose elements could not possibly fit in the remaining bytes is rejected
// before anything is allocated. The hint is further bounded so that the
// preallocation is at most containerPreallocBytesPerInputByte times the bytes
// remaining (or containerPreallocMinBudget for small frames).
func containerCapacityHint(reader *Reader, count uint32, minElementBits int, elementSize uintptr) (int, error) {
	if minElementBits < 1 {
		minElementBits = 1
	}
	remainingBytes := uint64(reader.RemainingBytes())
	remainingBits := remainingBytes * 8
	if uint64(count)*uint64(minElementBits) > remainingBits {
		return 0, fmt.Errorf("declared container length %d exceeds the %d bits remaining (each element is at least %d bits)", count, remainingBits, minElementBits)
	}
	hint := uint64(count)
	if elementSize > 0 {
		budget := remainingBytes * containerPreallocBytesPerInputByte
		if budget < containerPreallocMinBudget {
			budget = containerPreallocMinBudget
		}
		if maxByMemory := budget / uint64(elementSize); hint > maxByMemory {
			hint = maxByMemory
		}
	}
	return int(hint), nil
}

// MakeSlice returns an empty, non-nil slice with capacity for a decoded list
// of count elements, after checking that count elements of at least
// minElementBits bits each can fit in the bytes remaining in the reader.
func MakeSlice[T any](reader *Reader, count uint32, minElementBits int) ([]T, error) {
	var zero T
	hint, err := containerCapacityHint(reader, count, minElementBits, unsafe.Sizeof(zero))
	if err != nil {
		return nil, err
	}
	return make([]T, 0, hint), nil
}

// MakeMap returns an empty map sized for a decoded map of count entries, after
// checking that count entries of at least minEntryBits bits each (one key plus
// one value) can fit in the bytes remaining in the reader.
func MakeMap[K comparable, V any](reader *Reader, count uint32, minEntryBits int) (map[K]V, error) {
	var k K
	var v V
	hint, err := containerCapacityHint(reader, count, minEntryBits, unsafe.Sizeof(k)+unsafe.Sizeof(v))
	if err != nil {
		return nil, err
	}
	return make(map[K]V, hint), nil
}

// BitSizeByteList returns the encoded size of a list<byte> / list<uint8>: a
// uint32 element count followed by one raw byte per element. This is the same
// encoding as an element-by-element list of uint8.
func BitSizeByteList(data []byte) int {
	return BitSizeUInt32(uint32(len(data))) + BytesToBits(len(data))
}

// SerializeByteList writes a list<byte> / list<uint8> in bulk.
func SerializeByteList(writer *Writer, data []byte) {
	SerializeUInt32(writer, uint32(len(data)))
	writer.WriteBytes(data)
}

// DeserializeByteList reads a list<byte> / list<uint8> in bulk. The result is
// always a non-nil slice, matching the element-by-element decoder.
func DeserializeByteList(data *[]byte, reader *Reader) error {
	var length uint32
	if err := DeserializeUInt32(&length, reader); err != nil {
		return err
	}
	if uint64(length) > uint64(reader.RemainingBytes()) {
		return fmt.Errorf("declared byte list length %d exceeds the %d bytes remaining", length, reader.RemainingBytes())
	}
	buf := make([]byte, length)
	if length > 0 {
		if err := reader.ReadBytes(buf); err != nil {
			return err
		}
	}
	*data = buf
	return nil
}
