package serialize

import (
	"time"

	"github.com/google/uuid"
)

func BitSizeUUID(data uuid.UUID) int {
	return BytesToBits(16)
}

func SerializeUUID(writer *FixedSizeWriter, data uuid.UUID) {
	for _, b := range data {
		writer.WriteBits(b, 8)
	}
}

func DeserializeUUID(data *uuid.UUID, reader *Reader) error {
	for i := 0; i < 16; i++ {
		var b byte
		if err := reader.ReadBits(&b, 8); err != nil {
			return err
		}
		data[i] = b
	}
	return nil
}

func BitSizeTime(data time.Time) int {
	timeUTC := data.UTC()
	seconds := timeUTC.Unix()                                      // number of seconds since Unix epoch
	nanoseconds := timeUTC.UnixNano() - seconds*int64(time.Second) // remaining nanoseconds

	return BitSizeUInt64(uint64(seconds)) + BitSizeUInt64(uint64(nanoseconds))
}

func SerializeTime(writer *FixedSizeWriter, data time.Time) {
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

func BitSizeString(data string) int {
	ln := len(data)
	return BitSizeUInt32(uint32(ln)) + BytesToBits(ln)
}

func SerializeString(writer *FixedSizeWriter, data string) {
	SerializeUInt32(writer, uint32(len(data)))
	for i := 0; i < len(data); i++ {
		writer.WriteBits(data[i], 8)
	}
}

func DeserializeString(data *string, reader *Reader) error {
	var length uint32
	err := DeserializeUInt32(&length, reader)
	if err != nil {
		return err
	}

	// two allocations...
	bs := make([]byte, length)
	for i := uint32(0); i < length; i++ {
		var b byte
		err := reader.ReadBits(&b, 8)
		if err != nil {
			return err
		}
		bs[i] = b
	}
	*data = string(bs)
	return nil
}

func BitSizeBool(bool) int {
	return 1
}

func SerializeBool(writer *FixedSizeWriter, data bool) {
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

func SerializeUInt8(writer *FixedSizeWriter, data uint8) {
	writer.WriteBits(data, 8)
}

func DeserializeUInt8(data *uint8, reader *Reader) error {
	return reader.ReadBits(data, 8)
}

func BitSizeUInt16(data uint16) int {
	return int(varUintBitSize(uint64(data), 2))
}

func SerializeUInt16(writer *FixedSizeWriter, data uint16) {
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

func SerializeUInt32(writer *FixedSizeWriter, data uint32) {
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

func SerializeUInt64(writer *FixedSizeWriter, data uint64) {
	varEncodeUint(writer, uint64(data), 8)
}

func DeserializeUInt64(data *uint64, reader *Reader) error {
	return varDecodeUint(reader, data, 8)
}

func BitSizeInt8(data int8) int {
	return BitSizeUInt8(uint8(data))
}

func SerializeInt8(writer *FixedSizeWriter, data int8) {
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

func SerializeInt16(writer *FixedSizeWriter, data int16) {
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

func SerializeInt32(writer *FixedSizeWriter, data int32) {
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

func SerializeInt64(writer *FixedSizeWriter, data int64) {
	varEncodeInt(writer, data, 8)
}

func DeserializeInt64(data *int64, reader *Reader) error {
	return varDecodeInt(reader, data, 8)
}

func BitSizeFloat32(data float32) int {
	return BitSizeUInt32(Pack754_32(data))
}

func SerializeFloat32(writer *FixedSizeWriter, data float32) {
	SerializeUInt32(writer, Pack754_32(data))
}

func DeserializeFloat32(data *float32, reader *Reader) error {
	var packed uint32
	err := DeserializeUInt32(&packed, reader)
	if err != nil {
		return err
	}
	*data = Unpack754_32(packed)
	return nil
}

func BitSizeFloat64(data float64) int {
	return BitSizeUInt64(Pack754_64(data))
}

func SerializeFloat64(writer *FixedSizeWriter, data float64) {
	SerializeUInt64(writer, Pack754_64(data))
}

func DeserializeFloat64(data *float64, reader *Reader) error {
	var packed uint64
	err := DeserializeUInt64(&packed, reader)
	if err != nil {
		return err
	}
	*data = Unpack754_64(packed)
	return nil
}
