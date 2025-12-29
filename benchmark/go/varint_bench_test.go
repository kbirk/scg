package benchmarks

import (
	"testing"

	"github.com/kbirk/scg/pkg/serialize"
)

func BenchmarkVarEncodeUint64(b *testing.B) {
	writer := serialize.NewWriter(1024)
	val := uint64(0xDEADBEEFCAFEBABE)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writer.Reset()
		serialize.SerializeUInt64(writer, val)
	}
}

func BenchmarkVarDecodeUint64(b *testing.B) {
	val := uint64(0xDEADBEEFCAFEBABE)
	writer := serialize.NewWriter(16)
	serialize.SerializeUInt64(writer, val)
	data := writer.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := serialize.NewReader(data)
		var out uint64
		serialize.DeserializeUInt64(&out, reader)
	}
}

func BenchmarkVarEncodeInt64(b *testing.B) {
	val := int64(-1234567890123456789)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := serialize.NewWriter(16)
		serialize.SerializeInt64(w, val)
	}
}

func BenchmarkVarDecodeInt64(b *testing.B) {
	val := int64(-1234567890123456789)
	writer := serialize.NewWriter(16)
	serialize.SerializeInt64(writer, val)
	data := writer.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := serialize.NewReader(data)
		var out int64
		serialize.DeserializeInt64(&out, reader)
	}
}
