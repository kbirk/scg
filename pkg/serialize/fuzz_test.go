package serialize

import (
	"bytes"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

// FuzzReaderDecode drives every decoder over arbitrary input in an
// input-chosen order. Decoders must never panic, must never read past the
// buffer, and must not allocate more than the input can justify.
func FuzzReaderDecode(f *testing.F) {
	w := NewWriter(1)
	SerializeUInt32(w, 300)
	SerializeInt64(w, -12345)
	SerializeString(w, "hello")
	SerializeFloat64(w, math.Pi)
	SerializeBool(w, true)
	SerializeUUID(w, uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"))
	SerializeTime(w, time.Unix(1730000000, 5))
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6}, w.Bytes())
	f.Add([]byte{3, 3, 3}, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	f.Add([]byte{}, []byte{})

	f.Fuzz(func(t *testing.T, ops []byte, data []byte) {
		r := NewReader(data)
		for _, op := range ops {
			var err error
			switch op % 13 {
			case 0:
				var v uint8
				err = DeserializeUInt8(&v, r)
			case 1:
				var v uint16
				err = DeserializeUInt16(&v, r)
			case 2:
				var v uint32
				err = DeserializeUInt32(&v, r)
			case 3:
				var v uint64
				err = DeserializeUInt64(&v, r)
			case 4:
				var v int16
				err = DeserializeInt16(&v, r)
			case 5:
				var v int32
				err = DeserializeInt32(&v, r)
			case 6:
				var v int64
				err = DeserializeInt64(&v, r)
			case 7:
				var v string
				err = DeserializeString(&v, r)
				if err == nil && len(v) > len(data) {
					t.Fatalf("decoded a %d-byte string from %d bytes of input", len(v), len(data))
				}
			case 8:
				var v float32
				err = DeserializeFloat32(&v, r)
			case 9:
				var v float64
				err = DeserializeFloat64(&v, r)
			case 10:
				var v bool
				err = DeserializeBool(&v, r)
			case 11:
				var v uuid.UUID
				err = DeserializeUUID(&v, r)
			case 12:
				var v time.Time
				err = DeserializeTime(&v, r)
			}
			if r.numBitsRead > uint64(len(data))*8 {
				t.Fatalf("cursor %d past end of %d-byte input", r.numBitsRead, len(data))
			}
			if err != nil {
				return
			}
		}
	})
}

// FuzzVarintRoundTrip: every value round-trips through every width at every
// bit offset, and BitSize matches the bits written.
func FuzzVarintRoundTrip(f *testing.F) {
	f.Add(uint64(0), int64(0), uint8(0))
	f.Add(uint64(math.MaxUint64), int64(math.MinInt64), uint8(7))
	f.Add(uint64(0x100), int64(-1), uint8(3))

	f.Fuzz(func(t *testing.T, u uint64, i int64, offset uint8) {
		off := uint32(offset % 8)
		w := NewWriter(1)
		w.WriteBits(0x5A, off)
		SerializeUInt16(w, uint16(u))
		SerializeUInt32(w, uint32(u))
		SerializeUInt64(w, u)
		SerializeInt16(w, int16(i))
		SerializeInt32(w, int32(i))
		SerializeInt64(w, i)
		wantBits := int(off) + BitSizeUInt16(uint16(u)) + BitSizeUInt32(uint32(u)) + BitSizeUInt64(u) +
			BitSizeInt16(int16(i)) + BitSizeInt32(int32(i)) + BitSizeInt64(i)
		if int(w.numBitsWritten) != wantBits {
			t.Fatalf("wrote %d bits, BitSize says %d", w.numBitsWritten, wantBits)
		}

		// Byte-identical to the reference per-bit encoder.
		ref := NewWriter(1)
		ref.WriteBits(0x5A, off)
		refVarEncodeUint(ref, uint64(uint16(u)), 2)
		refVarEncodeUint(ref, uint64(uint32(u)), 4)
		refVarEncodeUint(ref, u, 8)
		refVarEncodeInt(ref, int64(int16(i)), 2)
		refVarEncodeInt(ref, int64(int32(i)), 4)
		refVarEncodeInt(ref, i, 8)
		if !bytes.Equal(w.Bytes(), ref.Bytes()) {
			t.Fatalf("encoding differs from reference: %x vs %x", w.Bytes(), ref.Bytes())
		}

		r := NewReader(w.Bytes())
		var skip byte
		if err := r.ReadBits(&skip, off); err != nil {
			t.Fatal(err)
		}
		var u16 uint16
		var u32 uint32
		var u64 uint64
		var i16 int16
		var i32 int32
		var i64 int64
		for _, err := range []error{
			DeserializeUInt16(&u16, r), DeserializeUInt32(&u32, r), DeserializeUInt64(&u64, r),
			DeserializeInt16(&i16, r), DeserializeInt32(&i32, r), DeserializeInt64(&i64, r),
		} {
			if err != nil {
				t.Fatal(err)
			}
		}
		if u16 != uint16(u) || u32 != uint32(u) || u64 != u || i16 != int16(i) || i32 != int32(i) || i64 != i {
			t.Fatalf("round trip mismatch")
		}
	})
}
