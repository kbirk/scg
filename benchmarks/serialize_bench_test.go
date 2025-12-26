package benchmarks

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kbirk/scg/pkg/serialize"
)

// BenchmarkSerializeUInt8 benchmarks uint8 serialization
func BenchmarkSerializeUInt8(b *testing.B) {
	var val uint8 = 123

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		writer := serialize.NewWriter(1)
		serialize.SerializeUInt8(writer, val)
	}
}

// BenchmarkDeserializeUInt8 benchmarks uint8 deserialization
func BenchmarkDeserializeUInt8(b *testing.B) {
	writer := serialize.NewWriter(1)
	serialize.SerializeUInt8(writer, 123)
	bs := writer.Bytes()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		reader := serialize.NewReader(bs)
		var val uint8
		serialize.DeserializeUInt8(&val, reader)
	}
}

// BenchmarkSerializeUInt32 benchmarks uint32 serialization (variable encoding)
func BenchmarkSerializeUInt32(b *testing.B) {
	testCases := []struct {
		name string
		val  uint32
	}{
		{"Small", 10},
		{"Medium", 1000},
		{"Large", 100000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			size := serialize.BitSizeUInt32(tc.val)

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				writer := serialize.NewWriter(serialize.BitsToBytes(size))
				serialize.SerializeUInt32(writer, tc.val)
			}
		})
	}
}

// BenchmarkDeserializeUInt32 benchmarks uint32 deserialization
func BenchmarkDeserializeUInt32(b *testing.B) {
	testCases := []struct {
		name string
		val  uint32
	}{
		{"Small", 10},
		{"Medium", 1000},
		{"Large", 100000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			size := serialize.BitSizeUInt32(tc.val)
			writer := serialize.NewWriter(serialize.BitsToBytes(size))
			serialize.SerializeUInt32(writer, tc.val)
			bs := writer.Bytes()

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				reader := serialize.NewReader(bs)
				var val uint32
				serialize.DeserializeUInt32(&val, reader)
			}
		})
	}
}

// BenchmarkSerializeString benchmarks string serialization
func BenchmarkSerializeString(b *testing.B) {
	testCases := []struct {
		name string
		val  string
	}{
		{"Empty", ""},
		{"Short", "hello"},
		{"Medium", "Hello, World! This is a medium length string for benchmarking."},
		{"Long", string(make([]byte, 1024))},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			size := serialize.BitSizeString(tc.val)

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				writer := serialize.NewWriter(serialize.BitsToBytes(size))
				serialize.SerializeString(writer, tc.val)
			}
		})
	}
}

// BenchmarkDeserializeString benchmarks string deserialization
func BenchmarkDeserializeString(b *testing.B) {
	testCases := []struct {
		name string
		val  string
	}{
		{"Empty", ""},
		{"Short", "hello"},
		{"Medium", "Hello, World! This is a medium length string for benchmarking."},
		{"Long", string(make([]byte, 1024))},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			size := serialize.BitSizeString(tc.val)
			writer := serialize.NewWriter(serialize.BitsToBytes(size))
			serialize.SerializeString(writer, tc.val)
			bs := writer.Bytes()

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				reader := serialize.NewReader(bs)
				var val string
				serialize.DeserializeString(&val, reader)
			}
		})
	}
}

// BenchmarkSerializeUUID benchmarks UUID serialization
func BenchmarkSerializeUUID(b *testing.B) {
	id := uuid.New()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		writer := serialize.NewWriter(16)
		serialize.SerializeUUID(writer, id)
	}
}

// BenchmarkDeserializeUUID benchmarks UUID deserialization
func BenchmarkDeserializeUUID(b *testing.B) {
	id := uuid.New()
	writer := serialize.NewWriter(16)
	serialize.SerializeUUID(writer, id)
	bs := writer.Bytes()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		reader := serialize.NewReader(bs)
		var val uuid.UUID
		serialize.DeserializeUUID(&val, reader)
	}
}

// BenchmarkSerializeTime benchmarks time serialization
func BenchmarkSerializeTime(b *testing.B) {
	now := time.Now()
	size := serialize.BitSizeTime(now)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		writer := serialize.NewWriter(serialize.BitsToBytes(size))
		serialize.SerializeTime(writer, now)
	}
}

// BenchmarkDeserializeTime benchmarks time deserialization
func BenchmarkDeserializeTime(b *testing.B) {
	now := time.Now()
	size := serialize.BitSizeTime(now)
	writer := serialize.NewWriter(serialize.BitsToBytes(size))
	serialize.SerializeTime(writer, now)
	bs := writer.Bytes()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		reader := serialize.NewReader(bs)
		var val time.Time
		serialize.DeserializeTime(&val, reader)
	}
}

// BenchmarkSerializeFloat64 benchmarks float64 serialization
func BenchmarkSerializeFloat64(b *testing.B) {
	val := 3.14159265359

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		writer := serialize.NewWriter(8)
		serialize.SerializeFloat64(writer, val)
	}
}

// BenchmarkDeserializeFloat64 benchmarks float64 deserialization
func BenchmarkDeserializeFloat64(b *testing.B) {
	val := 3.14159265359
	writer := serialize.NewWriter(8)
	serialize.SerializeFloat64(writer, val)
	bs := writer.Bytes()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		reader := serialize.NewReader(bs)
		var v float64
		serialize.DeserializeFloat64(&v, reader)
	}
}

// BenchmarkWriterOperations benchmarks low-level writer operations
func BenchmarkWriterOperations(b *testing.B) {
	b.Run("Writer", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			writer := serialize.NewWriter(1024)
			for j := 0; j < 100; j++ {
				writer.WriteBits(0xFF, 8)
			}
		}
	})
}

// BenchmarkReaderOperations benchmarks low-level reader operations
func BenchmarkReaderOperations(b *testing.B) {
	// Prepare data - create properly sized buffer
	dataSize := 100
	writer := serialize.NewWriter(dataSize)
	for j := 0; j < dataSize; j++ {
		writer.WriteBits(0xFF, 8)
	}
	bs := writer.Bytes()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		reader := serialize.NewReader(bs)
		var val byte
		for j := 0; j < dataSize; j++ {
			reader.ReadBits(&val, 8)
		}
	}
}

// BenchmarkComplexMessage simulates serializing a complex message
func BenchmarkComplexMessage(b *testing.B) {
	type ComplexMessage struct {
		ID        uuid.UUID
		Timestamp time.Time
		Name      string
		Values    []uint32
		Data      map[string]float64
	}

	msg := ComplexMessage{
		ID:        uuid.New(),
		Timestamp: time.Now(),
		Name:      "TestMessage",
		Values:    []uint32{1, 2, 3, 4, 5},
		Data:      map[string]float64{"a": 1.0, "b": 2.0, "c": 3.0},
	}

	// Calculate size
	size := serialize.BitSizeUUID(msg.ID) +
		serialize.BitSizeTime(msg.Timestamp) +
		serialize.BitSizeString(msg.Name) +
		serialize.BitSizeUInt32(uint32(len(msg.Values)))

	for _, v := range msg.Values {
		size += serialize.BitSizeUInt32(v)
	}

	size += serialize.BitSizeUInt32(uint32(len(msg.Data)))
	for k, v := range msg.Data {
		size += serialize.BitSizeString(k) + serialize.BitSizeFloat64(v)
	}

	b.Run("Serialize", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			writer := serialize.NewWriter(serialize.BitsToBytes(size))

			serialize.SerializeUUID(writer, msg.ID)
			serialize.SerializeTime(writer, msg.Timestamp)
			serialize.SerializeString(writer, msg.Name)
			serialize.SerializeUInt32(writer, uint32(len(msg.Values)))
			for _, v := range msg.Values {
				serialize.SerializeUInt32(writer, v)
			}
			serialize.SerializeUInt32(writer, uint32(len(msg.Data)))
			for k, v := range msg.Data {
				serialize.SerializeString(writer, k)
				serialize.SerializeFloat64(writer, v)
			}
			_ = writer.Bytes()
		}
	})

	b.Run("Deserialize", func(b *testing.B) {
		writer := serialize.NewWriter(serialize.BitsToBytes(size))
		serialize.SerializeUUID(writer, msg.ID)
		serialize.SerializeTime(writer, msg.Timestamp)
		serialize.SerializeString(writer, msg.Name)
		serialize.SerializeUInt32(writer, uint32(len(msg.Values)))
		for _, v := range msg.Values {
			serialize.SerializeUInt32(writer, v)
		}
		serialize.SerializeUInt32(writer, uint32(len(msg.Data)))
		for k, v := range msg.Data {
			serialize.SerializeString(writer, k)
			serialize.SerializeFloat64(writer, v)
		}
		bs := writer.Bytes()

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			reader := serialize.NewReader(bs)
			var result ComplexMessage

			serialize.DeserializeUUID(&result.ID, reader)
			serialize.DeserializeTime(&result.Timestamp, reader)
			serialize.DeserializeString(&result.Name, reader)

			var valLen uint32
			serialize.DeserializeUInt32(&valLen, reader)
			result.Values = make([]uint32, valLen)
			for j := uint32(0); j < valLen; j++ {
				serialize.DeserializeUInt32(&result.Values[j], reader)
			}

			var dataLen uint32
			serialize.DeserializeUInt32(&dataLen, reader)
			result.Data = make(map[string]float64, dataLen)
			for j := uint32(0); j < dataLen; j++ {
				var k string
				var v float64
				serialize.DeserializeString(&k, reader)
				serialize.DeserializeFloat64(&v, reader)
				result.Data[k] = v
			}
		}
	})
}
