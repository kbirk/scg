package serialize

import (
	"bytes"
	"testing"
)

// TestWriteBitsWiderThanEightNearEnd: WriteBits with a count above 8 near the
// end of the buffer must grow the buffer for the full advance (the original
// behaviour) instead of panicking in the 16-bit fast path.
func TestWriteBitsWiderThanEightNearEnd(t *testing.T) {
	w := NewWriter(2)
	w.WriteBits(0xFF, 7)
	w.WriteBits(0xFF, 16)
	if got, want := w.Bytes(), []byte{0xFF, 0x7F, 0x00}; !bytes.Equal(got, want) {
		t.Fatalf("got %x want %x", got, want)
	}
}

// TestReadBitsWiderThanEight: reads with a count above 8 keep the original
// semantics (low byte of the value, cursor advanced by the full count).
func TestReadBitsWiderThanEight(t *testing.T) {
	r := NewReader([]byte{0xFF, 0x7F, 0x00})
	var b byte
	if err := r.ReadBits(&b, 7); err != nil || b != 0x7F {
		t.Fatalf("ReadBits(7) = %#x, %v", b, err)
	}
	if err := r.ReadBits(&b, 16); err != nil || b != 0xFF {
		t.Fatalf("ReadBits(16) = %#x, %v", b, err)
	}
	if r.numBitsRead != 23 {
		t.Fatalf("cursor at %d, want 23", r.numBitsRead)
	}

	// At offset 0 with plenty of data: only the low byte is returned.
	r = NewReader([]byte{0xAB, 0xCD, 0xEF})
	if err := r.ReadBits(&b, 16); err != nil || b != 0xAB {
		t.Fatalf("ReadBits(16) at 0 = %#x, %v", b, err)
	}
	if r.numBitsRead != 16 {
		t.Fatalf("cursor at %d, want 16", r.numBitsRead)
	}
}

// TestBytesCannotLeakIntoNextMessage: appending to the slice returned by
// Bytes must not dirty the writer's spare capacity, which a pooled writer's
// Reset (clearing only the used prefix) would carry into the next message.
func TestBytesCannotLeakIntoNextMessage(t *testing.T) {
	w := NewWriter(16)
	w.WriteByte(1)
	b := w.Bytes()
	if cap(b) != len(b) {
		t.Fatalf("Bytes() capacity %d exceeds length %d", cap(b), len(b))
	}
	_ = append(b, 0xFF, 0xFF)

	w.Reset()
	w.WriteBits(0, 8)
	w.WriteBits(0, 8)
	if got := w.Bytes(); !bytes.Equal(got, []byte{0, 0}) {
		t.Fatalf("reused writer produced %x, want 0000", got)
	}
}
