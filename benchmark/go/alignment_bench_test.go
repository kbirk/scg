package benchmarks

import (
	"testing"

	"github.com/kbirk/scg/pkg/serialize"
)

func BenchmarkWriteBytesAligned(b *testing.B) {
	data := make([]byte, 1024)
	for i := range data {
		data[i] = 0xAA
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		writer := serialize.NewWriter(1024)
		writer.WriteBytes(data)
	}
}

func BenchmarkWriteBytesAlignedReuse(b *testing.B) {
	data := make([]byte, 1024)
	for i := range data {
		data[i] = 0xAA
	}
	writer := serialize.NewWriter(1024)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		writer.Reset()
		writer.WriteBytes(data)
	}
}

func BenchmarkWriteBytesUnaligned(b *testing.B) {
	data := make([]byte, 1024)
	for i := range data {
		data[i] = 0xAA
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		writer := serialize.NewWriter(1024 + 1)
		writer.WriteBits(1, 1)
		writer.WriteBytes(data)
	}
}

func BenchmarkWriteBytesUnalignedReuse(b *testing.B) {
	data := make([]byte, 1024)
	for i := range data {
		data[i] = 0xAA
	}
	writer := serialize.NewWriter(1024 + 1)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		writer.Reset()
		writer.WriteBits(1, 1)
		writer.WriteBytes(data)
	}
}

func BenchmarkReadBytesAligned(b *testing.B) {
	data := make([]byte, 1024)
	for i := range data {
		data[i] = 0xAA
	}
	writer := serialize.NewWriter(1024)
	writer.WriteBytes(data)
	bs := writer.Bytes()

	out := make([]byte, 1024)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		reader := serialize.NewReader(bs)
		reader.ReadBytes(out)
	}
}

func BenchmarkReadBytesUnaligned(b *testing.B) {
	data := make([]byte, 1024)
	for i := range data {
		data[i] = 0xAA
	}
	writer := serialize.NewWriter(1024 + 1)
	writer.WriteBits(1, 1)
	writer.WriteBytes(data)
	bs := writer.Bytes()

	out := make([]byte, 1024)

	var dummy byte
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		reader := serialize.NewReader(bs)
		reader.ReadBits(&dummy, 1)
		reader.ReadBytes(out)
	}
}
