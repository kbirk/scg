package rpc

import (
	"context"
	"testing"
	"time"

	"github.com/kbirk/scg/pkg/serialize"
)

// FuzzDeserializeContext: arbitrary bytes must never panic, hang, or allocate
// beyond what the input can justify (the declared-length guards).
func FuzzDeserializeContext(f *testing.F) {
	md := NewMetadata()
	md.PutString("token", "1234")
	md.PutBytes("raw", []byte{1, 2, 3})
	w := serialize.NewWriter(64)
	SerializeContext(w, NewContextWithMetadata(context.Background(), md))
	f.Add(w.Bytes())
	f.Add([]byte{})
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})

	f.Fuzz(func(t *testing.T, data []byte) {
		ctx := context.Background()
		if err := DeserializeContext(&ctx, serialize.NewReader(data)); err != nil {
			return
		}
		// Whatever decoded must re-encode to a size consistent with BitSize.
		out := serialize.NewWriter(1)
		SerializeContext(out, ctx)
		if got, want := len(out.Bytes()), serialize.BitsToBytes(BitSizeContext(ctx)); got != want {
			t.Fatalf("re-encoded %d bytes, BitSizeContext says %d", got, want)
		}
	})
}

// FuzzServerFrames feeds arbitrary frames (raw, or behind a valid request or
// stream prefix) to a server connection. The server must not panic or hang,
// and must still answer a well-formed request afterwards.
func FuzzServerFrames(f *testing.F) {
	f.Add([]byte("garbage"), uint8(0))
	f.Add([]byte{}, uint8(1))
	f.Add([]byte{0xDE, 0xAD, 0xBE, 0xEF}, uint8(1))
	f.Add([]byte{0x00, 0x01, 0x00}, uint8(2))
	f.Add(requestFrame(f, context.Background(), 3, 1, 1, []byte("hi"))[16:], uint8(1))
	f.Add(serializeStreamOpen(context.Background(), 9, 1, 1)[16:], uint8(2))

	f.Fuzz(func(t *testing.T, body []byte, mode uint8) {
		s := NewServer(ServerConfig{ErrHandler: func(error) {}, MaxConcurrentStreams: 4})
		s.RegisterServer(1, "echo", &echoStub{})
		client, done := startServerConn(s)
		defer func() {
			client.Close()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("server connection loop did not exit")
			}
		}()

		var frame []byte
		switch mode % 3 {
		case 0:
			frame = body
		case 1:
			frame = append(append([]byte(nil), RequestPrefix[:]...), body...)
		case 2:
			frame = append(append([]byte(nil), StreamPrefix[:]...), body...)
		}
		if err := client.Send(frame, 0); err != nil {
			t.Fatal(err)
		}

		// A valid request still gets its answer (responses to the fuzzed frame,
		// if any, are skipped).
		if err := client.Send(requestFrame(t, context.Background(), 1<<40, 1, 1, []byte("ok")), 1); err != nil {
			t.Fatal(err)
		}
		deadline := time.After(5 * time.Second)
		for {
			select {
			case b := <-client.in:
				r := serialize.NewReader(b)
				var prefix [16]byte
				if DeserializePrefix(&prefix, r) != nil || prefix != ResponsePrefix {
					continue
				}
				var id uint64
				if serialize.DeserializeUInt64(&id, r) == nil && id == 1<<40 {
					return
				}
			case <-deadline:
				t.Fatal("server stopped answering after a malformed frame")
			}
		}
	})
}
