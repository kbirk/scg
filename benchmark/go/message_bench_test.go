package benchmarks

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kbirk/scg/benchmark/scg/generated/benchmark"
	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/serialize"
)

// BenchmarkGeneratedMessageSmall benchmarks small generated messages
func BenchmarkGeneratedMessageSmall(b *testing.B) {
	msg := &benchmark.SmallMessage{
		ID:    12345,
		Value: 67890,
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
			var result benchmark.SmallMessage
			result.Deserialize(reader)
		}
	})
}

// BenchmarkGeneratedMessageEcho benchmarks echo request/response messages
func BenchmarkGeneratedMessageEcho(b *testing.B) {
	req := &benchmark.EchoRequest{
		Message:   "Hello, World! This is a test message for benchmarking purposes.",
		Timestamp: uint64(time.Now().UnixNano()),
	}

	resp := &benchmark.EchoResponse{
		Message:         "Hello, World! This is a test message for benchmarking purposes.",
		Timestamp:       uint64(time.Now().UnixNano()),
		ServerTimestamp: uint64(time.Now().UnixNano()),
	}

	b.Run("Request/Serialize", func(b *testing.B) {
		size := req.BitSize()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			writer := serialize.NewWriter(serialize.BitsToBytes(size))
			req.Serialize(writer)
			_ = writer.Bytes()
		}
	})

	b.Run("Request/Deserialize", func(b *testing.B) {
		size := req.BitSize()
		writer := serialize.NewWriter(serialize.BitsToBytes(size))
		req.Serialize(writer)
		bs := writer.Bytes()

		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			reader := serialize.NewReader(bs)
			var result benchmark.EchoRequest
			result.Deserialize(reader)
		}
	})

	b.Run("Response/Serialize", func(b *testing.B) {
		size := resp.BitSize()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			writer := serialize.NewWriter(serialize.BitsToBytes(size))
			resp.Serialize(writer)
			_ = writer.Bytes()
		}
	})

	b.Run("Response/Deserialize", func(b *testing.B) {
		size := resp.BitSize()
		writer := serialize.NewWriter(serialize.BitsToBytes(size))
		resp.Serialize(writer)
		bs := writer.Bytes()

		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			reader := serialize.NewReader(bs)
			var result benchmark.EchoResponse
			result.Deserialize(reader)
		}
	})
}

// BenchmarkGeneratedMessageProcess benchmarks complex business messages
func BenchmarkGeneratedMessageProcess(b *testing.B) {
	req := &benchmark.ProcessRequest{
		ID:        "order-12345",
		CreatedAt: time.Now(),
		User: benchmark.UserInfo{
			UserID:       uuid.New(),
			Username:     "testuser",
			Email:        "testuser@example.com",
			RegisteredAt: time.Now().Add(-365 * 24 * time.Hour),
			Role:         benchmark.UserRole_Admin,
		},
		Items: []benchmark.OrderItem{
			{
				ItemID:     uuid.New(),
				Name:       "Product A",
				Quantity:   2,
				UnitPrice:  19.99,
				TotalPrice: 39.98,
				Attributes: map[string]string{
					"color": "blue",
					"size":  "large",
				},
			},
			{
				ItemID:     uuid.New(),
				Name:       "Product B",
				Quantity:   1,
				UnitPrice:  49.99,
				TotalPrice: 49.99,
				Attributes: map[string]string{
					"warranty": "2 years",
				},
			},
		},
		Metadata: map[string]string{
			"source":   "web",
			"campaign": "summer-sale",
		},
	}

	resp := &benchmark.ProcessResponse{
		ID:          "order-12345",
		ProcessedAt: time.Now(),
		Status:      benchmark.ProcessStatus_Success,
		Message:     "Order processed successfully",
		Stats: benchmark.ProcessingStats{
			ItemsProcessed:   2,
			TotalAmount:      89.97,
			ProcessingTimeMs: 42,
		},
	}

	b.Run("Request/Serialize", func(b *testing.B) {
		size := req.BitSize()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			writer := serialize.NewWriter(serialize.BitsToBytes(size))
			req.Serialize(writer)
			_ = writer.Bytes()
		}
	})

	b.Run("Request/Deserialize", func(b *testing.B) {
		size := req.BitSize()
		writer := serialize.NewWriter(serialize.BitsToBytes(size))
		req.Serialize(writer)
		bs := writer.Bytes()

		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			reader := serialize.NewReader(bs)
			var result benchmark.ProcessRequest
			result.Deserialize(reader)
		}
	})

	b.Run("Response/Serialize", func(b *testing.B) {
		size := resp.BitSize()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			writer := serialize.NewWriter(serialize.BitsToBytes(size))
			resp.Serialize(writer)
			_ = writer.Bytes()
		}
	})

	b.Run("Response/Deserialize", func(b *testing.B) {
		size := resp.BitSize()
		writer := serialize.NewWriter(serialize.BitsToBytes(size))
		resp.Serialize(writer)
		bs := writer.Bytes()

		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			reader := serialize.NewReader(bs)
			var result benchmark.ProcessResponse
			result.Deserialize(reader)
		}
	})
}

// BenchmarkGeneratedMessageNested benchmarks deeply nested messages
func BenchmarkGeneratedMessageNested(b *testing.B) {
	msg := &benchmark.NestedMessage{
		Level1: benchmark.Level1{
			Name: "Level 1",
			Level2: benchmark.Level2{
				Name: "Level 2",
				Level3: benchmark.Level3{
					Name:   "Level 3",
					Values: []string{"value1", "value2", "value3", "value4", "value5"},
					Counts: map[string]int32{
						"count1": 10,
						"count2": 20,
						"count3": 30,
					},
				},
			},
		},
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
			var result benchmark.NestedMessage
			result.Deserialize(reader)
		}
	})
}

// BenchmarkGeneratedMessageLargePayload benchmarks large payload messages
func BenchmarkGeneratedMessageLargePayload(b *testing.B) {
	binaryData := make([]uint8, 1024) // 1KB of data
	for i := range binaryData {
		binaryData[i] = uint8(i % 256)
	}

	req := &benchmark.LargePayloadRequest{
		RequestID:  uuid.New(),
		CreatedAt:  time.Now(),
		BinaryData: binaryData,
		Tags:       []string{"tag1", "tag2", "tag3", "tag4", "tag5"},
		Metrics: map[string]benchmark.MetricValue{
			"cpu": {
				Value:     75.5,
				Timestamp: time.Now(),
				Unit:      "percent",
				Labels:    map[string]string{"host": "server1"},
			},
			"memory": {
				Value:     8192.0,
				Timestamp: time.Now(),
				Unit:      "MB",
				Labels:    map[string]string{"host": "server1"},
			},
		},
	}

	b.Run("1KB/Serialize", func(b *testing.B) {
		size := req.BitSize()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			writer := serialize.NewWriter(serialize.BitsToBytes(size))
			req.Serialize(writer)
			_ = writer.Bytes()
		}
	})

	b.Run("1KB/Deserialize", func(b *testing.B) {
		size := req.BitSize()
		writer := serialize.NewWriter(serialize.BitsToBytes(size))
		req.Serialize(writer)
		bs := writer.Bytes()

		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			reader := serialize.NewReader(bs)
			var result benchmark.LargePayloadRequest
			result.Deserialize(reader)
		}
	})

	// Test with 10KB payload
	binaryData10KB := make([]uint8, 10*1024)
	for i := range binaryData10KB {
		binaryData10KB[i] = uint8(i % 256)
	}
	req.BinaryData = binaryData10KB

	b.Run("10KB/Serialize", func(b *testing.B) {
		size := req.BitSize()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			writer := serialize.NewWriter(serialize.BitsToBytes(size))
			req.Serialize(writer)
			_ = writer.Bytes()
		}
	})

	b.Run("10KB/Deserialize", func(b *testing.B) {
		size := req.BitSize()
		writer := serialize.NewWriter(serialize.BitsToBytes(size))
		req.Serialize(writer)
		bs := writer.Bytes()

		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			reader := serialize.NewReader(bs)
			var result benchmark.LargePayloadRequest
			result.Deserialize(reader)
		}
	})
}

// BenchmarkFullRPCCycle benchmarks complete RPC request/response cycle
func BenchmarkFullRPCCycle(b *testing.B) {
	ctx := context.Background()
	requestID := uint64(12345)
	serviceID := uint64(1)
	methodID := uint64(1)

	req := &benchmark.EchoRequest{
		Message:   "Test message for RPC benchmark",
		Timestamp: uint64(time.Now().UnixNano()),
	}

	resp := &benchmark.EchoResponse{
		Message:         "Test message for RPC benchmark",
		Timestamp:       uint64(time.Now().UnixNano()),
		ServerTimestamp: uint64(time.Now().UnixNano()),
	}

	b.Run("Request/Serialize", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			writer := serialize.NewWriter(
				serialize.BitsToBytes(
					rpc.BitSizePrefix() +
						rpc.BitSizeContext(ctx) +
						serialize.BitSizeUInt64(requestID) +
						serialize.BitSizeUInt64(serviceID) +
						serialize.BitSizeUInt64(methodID) +
						req.BitSize()))

			rpc.SerializePrefix(writer, rpc.RequestPrefix)
			rpc.SerializeContext(writer, ctx)
			serialize.SerializeUInt64(writer, requestID)
			serialize.SerializeUInt64(writer, serviceID)
			serialize.SerializeUInt64(writer, methodID)
			req.Serialize(writer)
			_ = writer.Bytes()
		}
	})

	b.Run("Response/Serialize", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			writer := serialize.NewWriter(
				serialize.BitsToBytes(
					rpc.BitSizePrefix() +
						serialize.BitSizeUInt64(requestID) +
						serialize.BitSizeUInt8(rpc.MessageResponse) +
						resp.BitSize()))

			rpc.SerializePrefix(writer, rpc.ResponsePrefix)
			serialize.SerializeUInt64(writer, requestID)
			serialize.SerializeUInt8(writer, rpc.MessageResponse)
			resp.Serialize(writer)
			_ = writer.Bytes()
		}
	})

	reqBytes := func() []byte {
		writer := serialize.NewWriter(
			serialize.BitsToBytes(
				rpc.BitSizePrefix() +
					rpc.BitSizeContext(ctx) +
					serialize.BitSizeUInt64(requestID) +
					serialize.BitSizeUInt64(serviceID) +
					serialize.BitSizeUInt64(methodID) +
					req.BitSize()))
		rpc.SerializePrefix(writer, rpc.RequestPrefix)
		rpc.SerializeContext(writer, ctx)
		serialize.SerializeUInt64(writer, requestID)
		serialize.SerializeUInt64(writer, serviceID)
		serialize.SerializeUInt64(writer, methodID)
		req.Serialize(writer)
		return writer.Bytes()
	}()

	b.Run("Request/Deserialize", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			reader := serialize.NewReader(reqBytes)

			var prefix [16]byte
			rpc.DeserializePrefix(&prefix, reader)

			var reqCtx context.Context = context.Background()
			rpc.DeserializeContext(&reqCtx, reader)

			var reqID, svcID, methID uint64
			serialize.DeserializeUInt64(&reqID, reader)
			serialize.DeserializeUInt64(&svcID, reader)
			serialize.DeserializeUInt64(&methID, reader)

			var resultReq benchmark.EchoRequest
			resultReq.Deserialize(reader)
		}
	})

	respBytes := func() []byte {
		writer := serialize.NewWriter(
			serialize.BitsToBytes(
				rpc.BitSizePrefix() +
					serialize.BitSizeUInt64(requestID) +
					serialize.BitSizeUInt8(rpc.MessageResponse) +
					resp.BitSize()))
		rpc.SerializePrefix(writer, rpc.ResponsePrefix)
		serialize.SerializeUInt64(writer, requestID)
		serialize.SerializeUInt8(writer, rpc.MessageResponse)
		resp.Serialize(writer)
		return writer.Bytes()
	}()

	b.Run("Response/Deserialize", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			reader := serialize.NewReader(respBytes)

			var prefix [16]byte
			rpc.DeserializePrefix(&prefix, reader)

			var reqID uint64
			serialize.DeserializeUInt64(&reqID, reader)

			var responseType uint8
			serialize.DeserializeUInt8(&responseType, reader)

			var resultResp benchmark.EchoResponse
			resultResp.Deserialize(reader)
		}
	})
}
