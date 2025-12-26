package benchmarks

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/serialize"
)

// MockMessage implements the Message interface for benchmarking
type MockMessage struct {
	ID        uuid.UUID
	Timestamp time.Time
	Value     string
	Count     uint32
}

func (m *MockMessage) BitSize() int {
	return serialize.BitSizeUUID(m.ID) +
		serialize.BitSizeTime(m.Timestamp) +
		serialize.BitSizeString(m.Value) +
		serialize.BitSizeUInt32(m.Count)
}

func (m *MockMessage) Serialize(writer *serialize.Writer) {
	serialize.SerializeUUID(writer, m.ID)
	serialize.SerializeTime(writer, m.Timestamp)
	serialize.SerializeString(writer, m.Value)
	serialize.SerializeUInt32(writer, m.Count)
}

func (m *MockMessage) Deserialize(reader *serialize.Reader) error {
	if err := serialize.DeserializeUUID(&m.ID, reader); err != nil {
		return err
	}
	if err := serialize.DeserializeTime(&m.Timestamp, reader); err != nil {
		return err
	}
	if err := serialize.DeserializeString(&m.Value, reader); err != nil {
		return err
	}
	return serialize.DeserializeUInt32(&m.Count, reader)
}

func (m *MockMessage) ToJSON() ([]byte, error) { return nil, nil }
func (m *MockMessage) FromJSON([]byte) error   { return nil }
func (m *MockMessage) ToBytes() []byte         { return nil }
func (m *MockMessage) FromBytes([]byte) error  { return nil }

// BenchmarkMessageSerialization benchmarks message serialization
func BenchmarkMessageSerialization(b *testing.B) {
	msg := &MockMessage{
		ID:        uuid.New(),
		Timestamp: time.Now(),
		Value:     "Hello, World! This is a test message.",
		Count:     42,
	}

	b.Run("Serialize", func(b *testing.B) {
		size := msg.BitSize()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			writer := serialize.NewWriter(serialize.BitsToBytes(size))
			msg.Serialize(writer)
			_ = writer.Bytes()
		}
	})

	b.Run("Deserialize", func(b *testing.B) {
		size := msg.BitSize()
		writer := serialize.NewWriter(serialize.BitsToBytes(size))
		msg.Serialize(writer)
		bs := writer.Bytes()

		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			reader := serialize.NewReader(bs)
			var result MockMessage
			result.Deserialize(reader)
		}
	})
}

// BenchmarkRequestSerialization benchmarks full RPC request serialization
func BenchmarkRequestSerialization(b *testing.B) {
	ctx := context.Background()
	requestID := uint64(12345)
	serviceID := uint64(1)
	methodID := uint64(2)
	msg := &MockMessage{
		ID:        uuid.New(),
		Timestamp: time.Now(),
		Value:     "Test request message",
		Count:     100,
	}

	b.Run("Request", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			writer := serialize.NewWriter(
				serialize.BitsToBytes(
					rpc.BitSizePrefix() +
						rpc.BitSizeContext(ctx) +
						serialize.BitSizeUInt64(requestID) +
						serialize.BitSizeUInt64(serviceID) +
						serialize.BitSizeUInt64(methodID) +
						msg.BitSize()))

			rpc.SerializePrefix(writer, rpc.RequestPrefix)
			rpc.SerializeContext(writer, ctx)
			serialize.SerializeUInt64(writer, requestID)
			serialize.SerializeUInt64(writer, serviceID)
			serialize.SerializeUInt64(writer, methodID)
			msg.Serialize(writer)
			_ = writer.Bytes()
		}
	})

	b.Run("Response", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			writer := serialize.NewWriter(
				serialize.BitsToBytes(
					rpc.BitSizePrefix() +
						serialize.BitSizeUInt64(requestID) +
						serialize.BitSizeUInt8(rpc.MessageResponse) +
						msg.BitSize()))

			rpc.SerializePrefix(writer, rpc.ResponsePrefix)
			serialize.SerializeUInt64(writer, requestID)
			serialize.SerializeUInt8(writer, rpc.MessageResponse)
			msg.Serialize(writer)
			_ = writer.Bytes()
		}
	})
}

// BenchmarkRequestDeserialization benchmarks full RPC request deserialization
func BenchmarkRequestDeserialization(b *testing.B) {
	ctx := context.Background()
	requestID := uint64(12345)
	serviceID := uint64(1)
	methodID := uint64(2)
	msg := &MockMessage{
		ID:        uuid.New(),
		Timestamp: time.Now(),
		Value:     "Test request message",
		Count:     100,
	}

	// Prepare request
	writer := serialize.NewWriter(
		serialize.BitsToBytes(
			rpc.BitSizePrefix() +
				rpc.BitSizeContext(ctx) +
				serialize.BitSizeUInt64(requestID) +
				serialize.BitSizeUInt64(serviceID) +
				serialize.BitSizeUInt64(methodID) +
				msg.BitSize()))

	rpc.SerializePrefix(writer, rpc.RequestPrefix)
	rpc.SerializeContext(writer, ctx)
	serialize.SerializeUInt64(writer, requestID)
	serialize.SerializeUInt64(writer, serviceID)
	serialize.SerializeUInt64(writer, methodID)
	msg.Serialize(writer)
	bs := writer.Bytes()

	b.Run("Request", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			reader := serialize.NewReader(bs)

			var prefix [16]byte
			rpc.DeserializePrefix(&prefix, reader)

			var reqCtx context.Context = context.Background()
			rpc.DeserializeContext(&reqCtx, reader)

			var reqID, svcID, methID uint64
			serialize.DeserializeUInt64(&reqID, reader)
			serialize.DeserializeUInt64(&svcID, reader)
			serialize.DeserializeUInt64(&methID, reader)

			var resultMsg MockMessage
			resultMsg.Deserialize(reader)
		}
	})

	// Prepare response
	writer2 := serialize.NewWriter(
		serialize.BitsToBytes(
			rpc.BitSizePrefix() +
				serialize.BitSizeUInt64(requestID) +
				serialize.BitSizeUInt8(rpc.MessageResponse) +
				msg.BitSize()))

	rpc.SerializePrefix(writer2, rpc.ResponsePrefix)
	serialize.SerializeUInt64(writer2, requestID)
	serialize.SerializeUInt8(writer2, rpc.MessageResponse)
	msg.Serialize(writer2)
	bs2 := writer2.Bytes()

	b.Run("Response", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			reader := serialize.NewReader(bs2)

			var prefix [16]byte
			rpc.DeserializePrefix(&prefix, reader)

			var reqID uint64
			serialize.DeserializeUInt64(&reqID, reader)

			var responseType uint8
			serialize.DeserializeUInt8(&responseType, reader)

			var resultMsg MockMessage
			resultMsg.Deserialize(reader)
		}
	})
}

// BenchmarkContextOperations benchmarks context serialization
func BenchmarkContextOperations(b *testing.B) {
	b.Run("EmptyContext", func(b *testing.B) {
		ctx := context.Background()
		size := rpc.BitSizeContext(ctx)

		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			writer := serialize.NewWriter(serialize.BitsToBytes(size))
			rpc.SerializeContext(writer, ctx)
		}
	})

	b.Run("ContextWithMetadata", func(b *testing.B) {
		metadata := rpc.NewMetadata()
		metadata.PutString("key1", "value1")
		metadata.PutString("key2", "value2")
		metadata.PutString("key3", "value3")
		ctx := rpc.NewContextWithMetadata(context.Background(), metadata)

		size := rpc.BitSizeContext(ctx)

		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			writer := serialize.NewWriter(serialize.BitsToBytes(size))
			rpc.SerializeContext(writer, ctx)
		}
	})

	b.Run("DeserializeEmptyContext", func(b *testing.B) {
		ctx := context.Background()
		size := rpc.BitSizeContext(ctx)
		writer := serialize.NewWriter(serialize.BitsToBytes(size))
		rpc.SerializeContext(writer, ctx)
		bs := writer.Bytes()

		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			reader := serialize.NewReader(bs)
			var resultCtx context.Context = context.Background()
			rpc.DeserializeContext(&resultCtx, reader)
		}
	})

	b.Run("DeserializeContextWithMetadata", func(b *testing.B) {
		metadata := rpc.NewMetadata()
		metadata.PutString("key1", "value1")
		metadata.PutString("key2", "value2")
		metadata.PutString("key3", "value3")
		ctx := rpc.NewContextWithMetadata(context.Background(), metadata)

		size := rpc.BitSizeContext(ctx)
		writer := serialize.NewWriter(serialize.BitsToBytes(size))
		rpc.SerializeContext(writer, ctx)
		bs := writer.Bytes()

		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			reader := serialize.NewReader(bs)
			var resultCtx context.Context = context.Background()
			rpc.DeserializeContext(&resultCtx, reader)
		}
	})
}

// BenchmarkPrefixOperations benchmarks prefix serialization
func BenchmarkPrefixOperations(b *testing.B) {
	b.Run("Serialize", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			writer := serialize.NewWriter(16)
			rpc.SerializePrefix(writer, rpc.RequestPrefix)
		}
	})

	b.Run("Deserialize", func(b *testing.B) {
		writer := serialize.NewWriter(16)
		rpc.SerializePrefix(writer, rpc.RequestPrefix)
		bs := writer.Bytes()

		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			reader := serialize.NewReader(bs)
			var prefix [16]byte
			rpc.DeserializePrefix(&prefix, reader)
		}
	})
}

// BenchmarkRespondWith benchmarks response generation
func BenchmarkRespondWith(b *testing.B) {
	requestID := uint64(12345)
	msg := &MockMessage{
		ID:        uuid.New(),
		Timestamp: time.Now(),
		Value:     "Response message",
		Count:     200,
	}

	b.Run("rpc.RespondWithMessage", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = rpc.RespondWithMessage(requestID, msg)
		}
	})

	b.Run("rpc.RespondWithError", func(b *testing.B) {
		err := context.Canceled
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = rpc.RespondWithError(requestID, err)
		}
	})
}

// BenchmarkMetadataOperations benchmarks metadata operations
func BenchmarkMetadataOperations(b *testing.B) {
	b.Run("PutString", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			metadata := rpc.NewMetadata()
			metadata.PutString("key1", "value1")
			metadata.PutString("key2", "value2")
			metadata.PutString("key3", "value3")
		}
	})

	b.Run("GetString", func(b *testing.B) {
		metadata := rpc.NewMetadata()
		metadata.PutString("key1", "value1")
		metadata.PutString("key2", "value2")
		metadata.PutString("key3", "value3")

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			metadata.GetString("key1")
			metadata.GetString("key2")
			metadata.GetString("key3")
		}
	})
}
