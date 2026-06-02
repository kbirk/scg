package serialize

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/google/uuid"
)

func BitSizeUUID(data uuid.UUID) int {
	return BytesToBits(16)
}

func SerializeUUID(writer *Writer, data uuid.UUID) {
	writer.WriteBytes(data[:])
}

func DeserializeUUID(data *uuid.UUID, reader *Reader) error {
	return reader.ReadBytes(data[:])
}

func BitSizeTime(data time.Time) int {
	timeUTC := data.UTC()
	seconds := timeUTC.Unix()                                      // number of seconds since Unix epoch
	nanoseconds := timeUTC.UnixNano() - seconds*int64(time.Second) // remaining nanoseconds

	return BitSizeUInt64(uint64(seconds)) + BitSizeUInt64(uint64(nanoseconds))
}

func SerializeTime(writer *Writer, data time.Time) {
	timeUTC := data.UTC()
	seconds := timeUTC.Unix()                                      // number of seconds since Unix epoch
	nanoseconds := timeUTC.UnixNano() - seconds*int64(time.Second) // remaining nanoseconds

	SerializeUInt64(writer, uint64(seconds))
	SerializeUInt64(writer, uint64(nanoseconds))
}

func DeserializeTime(data *time.Time, reader *Reader) error {
	var seconds uint64
	var nanoseconds uint64
	err := DeserializeUInt64(&seconds, reader)
	if err != nil {
		return err
	}
	err = DeserializeUInt64(&nanoseconds, reader)
	if err != nil {
		return err
	}

	*data = time.Unix(int64(seconds), int64(nanoseconds))
	return nil
}

// CheckLength validates a wire-declared element/byte count against the bytes
// remaining in the reader. It is the guard prescribed for length-prefixed
// allocations: every element of the collections that use it occupies at least
// one byte on the wire (raw bytes, characters, or map entries whose key/value
// each carry their own length prefix), so a count exceeding the remaining bytes
// cannot be satisfied and signals a truncated or hostile frame. Bounding here
// prevents a small frame from forcing a huge make()/resize() before the bytes
// are read — the pre-auth allocation-DoS fix.
func CheckLength(reader *Reader, count uint32) error {
	if int(count) > reader.RemainingBytes() {
		return fmt.Errorf("declared length %d exceeds %d remaining bytes", count, reader.RemainingBytes())
	}
	return nil
}

func BitSizeString(data string) int {
	ln := len(data)
	return BitSizeUInt32(uint32(ln)) + BytesToBits(ln)
}

func SerializeString(writer *Writer, data string) {
	SerializeUInt32(writer, uint32(len(data)))
	writer.WriteBytes([]byte(data))
}

func DeserializeString(data *string, reader *Reader) error {
	var length uint32
	if err := DeserializeUInt32(&length, reader); err != nil {
		return err
	}

	if length == 0 {
		*data = ""
		return nil
	}

	// Bound the declared length against the data actually present before any
	// allocation, so a hostile length cannot force a huge make() (each char is
	// one byte). The fast path's own bound is kept below as a precise check.
	if err := CheckLength(reader, length); err != nil {
		return err
	}

	// Fast path for byte-aligned reads
	if reader.numBitsRead&7 == 0 {
		byteIndex := reader.numBitsRead >> 3
		if int(byteIndex)+int(length) > len(reader.bytes) {
			return fmt.Errorf("Reader does not contain enough data to fill the argument")
		}
		*data = string(reader.bytes[byteIndex : byteIndex+uint32(length)])
		reader.numBitsRead += length * 8
		return nil
	}

	// Use ReadBytes for optimized unaligned reading
	buf := make([]byte, length)
	if err := reader.ReadBytes(buf); err != nil {
		return err
	}
	// NOTE: buf is uniquely allocated and never mutated after this point
	// Safe zero-copy conversion
	*data = unsafe.String(&buf[0], len(buf))
	return nil
}

func BitSizeBool(bool) int {
	return 1
}

func SerializeBool(writer *Writer, data bool) {
	ui := uint8(0)
	if data {
		ui = 1
	}
	writer.WriteBits(ui, 1)
}

func DeserializeBool(data *bool, reader *Reader) error {
	var b byte
	err := reader.ReadBits(&b, 1)
	if err != nil {
		return err
	}
	if b == 1 {
		*data = true
	} else {
		*data = false
	}
	return nil
}

func BitSizeUInt8(data uint8) int {
	return 8
}

func SerializeUInt8(writer *Writer, data uint8) {
	writer.WriteByte(data)
}

func DeserializeUInt8(data *uint8, reader *Reader) error {
	return reader.ReadByte(data)
}

func BitSizeUInt16(data uint16) int {
	return int(varUintBitSize(uint64(data), 2))
}

func SerializeUInt16(writer *Writer, data uint16) {
	varEncodeUint(writer, uint64(data), 2)
}

func DeserializeUInt16(data *uint16, reader *Reader) error {
	var d uint64
	err := varDecodeUint(reader, &d, 2)
	if err != nil {
		return err
	}
	*data = uint16(d)
	return nil
}

func BitSizeUInt32(data uint32) int {
	return int(varUintBitSize(uint64(data), 4))
}

func SerializeUInt32(writer *Writer, data uint32) {
	varEncodeUint(writer, uint64(data), 4)
}

func DeserializeUInt32(data *uint32, reader *Reader) error {
	var d uint64
	err := varDecodeUint(reader, &d, 4)
	if err != nil {
		return err
	}
	*data = uint32(d)
	return nil
}

func BitSizeUInt64(data uint64) int {
	return int(varUintBitSize(data, 8))
}

func SerializeUInt64(writer *Writer, data uint64) {
	varEncodeUint(writer, uint64(data), 8)
}

func DeserializeUInt64(data *uint64, reader *Reader) error {
	return varDecodeUint(reader, data, 8)
}

func BitSizeInt8(data int8) int {
	return BitSizeUInt8(uint8(data))
}

func SerializeInt8(writer *Writer, data int8) {
	SerializeUInt8(writer, uint8(data))
}

func DeserializeInt8(data *int8, reader *Reader) error {
	var ui uint8
	err := DeserializeUInt8(&ui, reader)
	if err != nil {
		return err
	}
	*data = int8(ui)
	return nil
}

func BitSizeInt16(data int16) int {
	return int(varIntBitSize(int64(data), 2))
}

func SerializeInt16(writer *Writer, data int16) {
	varEncodeInt(writer, int64(data), 2)
}

func DeserializeInt16(data *int16, reader *Reader) error {
	var d int64
	err := varDecodeInt(reader, &d, 2)
	if err != nil {
		return err
	}
	*data = int16(d)
	return nil
}

func BitSizeInt32(data int32) int {
	return int(varIntBitSize(int64(data), 4))
}

func SerializeInt32(writer *Writer, data int32) {
	varEncodeInt(writer, int64(data), 4)
}

func DeserializeInt32(data *int32, reader *Reader) error {
	var d int64
	err := varDecodeInt(reader, &d, 4)
	if err != nil {
		return err
	}
	*data = int32(d)
	return nil
}

func BitSizeInt64(data int64) int {
	return int(varIntBitSize(data, 8))
}

func SerializeInt64(writer *Writer, data int64) {
	varEncodeInt(writer, data, 8)
}

func DeserializeInt64(data *int64, reader *Reader) error {
	return varDecodeInt(reader, data, 8)
}

func BitSizeFloat32(data float32) int {
	return BytesToBits(4)
}

func SerializeFloat32(writer *Writer, data float32) {
	writer.WriteBytes(PackFloat32(data))
}

func DeserializeFloat32(data *float32, reader *Reader) error {
	b := []byte{0, 0, 0, 0}
	err := reader.ReadBytes(b)
	if err != nil {
		return err
	}
	*data = UnpackFloat32(b)
	return nil
}

func BitSizeFloat64(data float64) int {
	return BytesToBits(8)
}

func SerializeFloat64(writer *Writer, data float64) {
	writer.WriteBytes(PackFloat64(data))
}

func DeserializeFloat64(data *float64, reader *Reader) error {
	b := []byte{0, 0, 0, 0, 0, 0, 0, 0}
	err := reader.ReadBytes(b)
	if err != nil {
		return err
	}
	*data = UnpackFloat64(b)
	return nil
}
