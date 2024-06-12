package serialize

import (
	"time"
	"unsafe"
)

func ByteSizeTime(data time.Time) int {
	return 16
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

func ByteSizeString(data string) int {
	return 4 + len(data)
}

func SerializeString(writer *FixedSizeWriter, data string) {
	SerializeUInt32(writer, uint32(len(data)))
	bs := writer.Next(len(data))
	copy(bs, data)
}

func DeserializeString(data *string, reader *Reader) error {
	var length uint32
	err := DeserializeUInt32(&length, reader)
	if err != nil {
		return err
	}

	bs, err := reader.Read(int(length))
	if err != nil {
		return err
	}
	*data = string(bs)
	return nil
}

func ByteSizeBool(bool) int {
	return 1
}

func SerializeBool(writer *FixedSizeWriter, data bool) {
	val := uint8(0)
	if data {
		val = 1
	}
	SerializeUInt8(writer, val)
}

func DeserializeBool(data *bool, reader *Reader) error {
	bs, err := reader.Read(1)
	if err != nil {
		return err
	}
	if uint8(bs[0]) == 1 {
		*data = true
	} else {
		*data = false
	}
	return nil
}

func ByteSizeUInt8(uint8) int {
	return 1
}

func SerializeUInt8(writer *FixedSizeWriter, data uint8) {
	bs := writer.Next(1)
	bs[0] = byte(data)
}

func DeserializeUInt8(data *uint8, reader *Reader) error {
	bs, err := reader.Read(1)
	if err != nil {
		return err
	}
	*data = uint8(bs[0])
	return nil
}

func ByteSizeUInt16(uint16) int {
	return 2
}

func SerializeUInt16(writer *FixedSizeWriter, data uint16) {
	bs := writer.Next(2)
	bs[0] = byte(data >> 8)
	bs[1] = byte(data)
}

func DeserializeUInt16(data *uint16, reader *Reader) error {
	bs, err := reader.Read(2)
	if err != nil {
		return err
	}
	*data = uint16(bs[0])<<8 | uint16(bs[1])
	return nil
}

func ByteSizeUInt32(uint32) int {
	return 4
}

func SerializeUInt32(writer *FixedSizeWriter, data uint32) {
	bs := writer.Next(4)
	bs[0] = byte(data >> 24)
	bs[1] = byte(data >> 16)
	bs[2] = byte(data >> 8)
	bs[3] = byte(data)
}

func DeserializeUInt32(data *uint32, reader *Reader) error {
	bs, err := reader.Read(4)
	if err != nil {
		return err
	}
	*data = uint32(bs[0])<<24 |
		uint32(bs[1])<<16 |
		uint32(bs[2])<<8 |
		uint32(bs[3])
	return nil
}

func ByteSizeUInt64(uint64) int {
	return 8
}

func SerializeUInt64(writer *FixedSizeWriter, data uint64) {
	bs := writer.Next(8)
	bs[0] = byte(data >> 56)
	bs[1] = byte(data >> 48)
	bs[2] = byte(data >> 40)
	bs[3] = byte(data >> 32)
	bs[4] = byte(data >> 24)
	bs[5] = byte(data >> 16)
	bs[6] = byte(data >> 8)
	bs[7] = byte(data)
}

func DeserializeUInt64(data *uint64, reader *Reader) error {
	bs, err := reader.Read(8)
	if err != nil {
		return err
	}
	*data = uint64(bs[0])<<56 |
		uint64(bs[1])<<48 |
		uint64(bs[2])<<40 |
		uint64(bs[3])<<32 |
		uint64(bs[4])<<24 |
		uint64(bs[5])<<16 |
		uint64(bs[6])<<8 |
		uint64(bs[7])
	return nil
}

func ByteSizeInt8(int8) int {
	return 1
}

func SerializeInt8(writer *FixedSizeWriter, data int8) {
	SerializeUInt8(writer, uint8(data))
}

func DeserializeInt8(data *int8, reader *Reader) error {
	return DeserializeUInt8((*uint8)(unsafe.Pointer(data)), reader)
}

func ByteSizeInt16(int16) int {
	return 2
}

func SerializeInt16(writer *FixedSizeWriter, data int16) {
	SerializeUInt16(writer, uint16(data))
}

func DeserializeInt16(data *int16, reader *Reader) error {
	return DeserializeUInt16((*uint16)(unsafe.Pointer(data)), reader)
}

func ByteSizeInt32(int32) int {
	return 4
}

func SerializeInt32(writer *FixedSizeWriter, data int32) {
	SerializeUInt32(writer, uint32(data))
}

func DeserializeInt32(data *int32, reader *Reader) error {
	return DeserializeUInt32((*uint32)(unsafe.Pointer(data)), reader)
}

func ByteSizeInt64(int64) int {
	return 8
}

func SerializeInt64(writer *FixedSizeWriter, data int64) {
	SerializeUInt64(writer, uint64(data))
}

func DeserializeInt64(data *int64, reader *Reader) error {
	return DeserializeUInt64((*uint64)(unsafe.Pointer(data)), reader)
}

func ByteSizeFloat32(float32) int {
	return 4
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

func ByteSizeFloat64(float64) int {
	return 8
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
