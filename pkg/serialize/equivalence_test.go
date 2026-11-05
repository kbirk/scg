package serialize

import (
	"bytes"
	"math"
	"math/rand"
	"testing"
)

// Reference implementations of the original per-bit encoders. The optimized
// encoders must stay bit-identical to these: the wire format is shared with the
// C++ runtime and with already-deployed peers.

func refVarUintBitSize(val uint64, numBytes uint32) uint32 {
	size := uint32(0)
	for i := uint32(0); i < numBytes; i++ {
		if val != 0 {
			size += 9
		} else {
			size += 1
			break
		}
		val >>= 8
	}
	return size
}

func refVarEncodeUint(writer *Writer, val uint64, numBytes uint32) {
	for i := uint32(0); i < numBytes; i++ {
		if val != 0 {
			writer.WriteBits(1, 1)
			writer.WriteBits(uint8(val), 8)
		} else {
			writer.WriteBits(0, 1)
			return
		}
		val >>= 8
	}
}

func refVarEncodeInt(writer *Writer, val int64, numBytes uint32) {
	unsignedVal := uint64(val)
	if val < 0 {
		writer.WriteBits(1, 1)
		unsignedVal = zigzagEncode(val)
	} else {
		writer.WriteBits(0, 1)
	}
	refVarEncodeUint(writer, unsignedVal, numBytes)
}

func interestingUint64s(r *rand.Rand) []uint64 {
	vals := []uint64{0, 1, 2, 0x7F, 0x80, 0xFF, 0x100, 0xFFFF, 0x10000, 0xFFFFFF,
		0xFFFFFFFF, 0x100000000, 0x00FF00FF00FF00FF, 0xFF00000000000000,
		math.MaxUint64, math.MaxUint64 - 1, 1 << 63}
	for shift := 0; shift < 64; shift++ {
		vals = append(vals, uint64(1)<<shift, (uint64(1)<<shift)-1)
	}
	for i := 0; i < 2000; i++ {
		// Random magnitudes across all byte lengths.
		vals = append(vals, r.Uint64()>>uint(r.Intn(64)))
	}
	return vals
}

func encodeWithOffset(offset int, fn func(w *Writer)) []byte {
	w := NewWriter(1) // tiny buffer: exercises growth and the slow tail paths
	for i := 0; i < offset; i++ {
		w.WriteBits(1, 1) // non-zero prefix bits must be preserved
	}
	fn(w)
	return append([]byte(nil), w.Bytes()...)
}

func TestVarintEncodingMatchesReference(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	widths := []uint32{2, 4, 8}
	for _, v := range interestingUint64s(r) {
		for _, nb := range widths {
			val := v
			if nb < 8 {
				val &= (uint64(1) << (8 * nb)) - 1
			}
			if got, want := varUintBitSize(val, nb), refVarUintBitSize(val, nb); got != want {
				t.Fatalf("varUintBitSize(%#x,%d)=%d want %d", val, nb, got, want)
			}
			sv := int64(val)
			if got, want := varIntBitSize(sv, nb), 1+refVarUintBitSize(func() uint64 {
				if sv < 0 {
					return zigzagEncode(sv)
				}
				return uint64(sv)
			}(), nb); got != want {
				t.Fatalf("varIntBitSize(%d,%d)=%d want %d", sv, nb, got, want)
			}
			for offset := 0; offset < 8; offset++ {
				got := encodeWithOffset(offset, func(w *Writer) { varEncodeUint(w, val, nb) })
				want := encodeWithOffset(offset, func(w *Writer) { refVarEncodeUint(w, val, nb) })
				if !bytes.Equal(got, want) {
					t.Fatalf("uint %#x width %d offset %d: got %x want %x", val, nb, offset, got, want)
				}
				got = encodeWithOffset(offset, func(w *Writer) { varEncodeInt(w, sv, nb) })
				want = encodeWithOffset(offset, func(w *Writer) { refVarEncodeInt(w, sv, nb) })
				if !bytes.Equal(got, want) {
					t.Fatalf("int %d width %d offset %d: got %x want %x", sv, nb, offset, got, want)
				}
				// Round trip through the decoders.
				rd := NewReader(got)
				var b byte
				for i := 0; i < offset; i++ {
					if err := rd.ReadBits(&b, 1); err != nil {
						t.Fatal(err)
					}
				}
				var dec int64
				if err := varDecodeInt(rd, &dec, nb); err != nil {
					t.Fatal(err)
				}
				if dec != sv {
					t.Fatalf("int round trip %d width %d offset %d: got %d", sv, nb, offset, dec)
				}
			}
		}
	}
}

func TestFloatAndBoolEncodingMatchesReference(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	f64s := []float64{0, -0.0, 1, -1, math.Pi, math.MaxFloat64, math.SmallestNonzeroFloat64, math.Inf(1), math.Inf(-1), math.NaN()}
	for i := 0; i < 500; i++ {
		f64s = append(f64s, math.Float64frombits(r.Uint64()))
	}
	for _, f := range f64s {
		for offset := 0; offset < 8; offset++ {
			got := encodeWithOffset(offset, func(w *Writer) { SerializeFloat64(w, f) })
			want := encodeWithOffset(offset, func(w *Writer) { w.WriteBytes(PackFloat64(f)) })
			if !bytes.Equal(got, want) {
				t.Fatalf("float64 %v offset %d: got %x want %x", f, offset, got, want)
			}
			f32 := float32(f)
			got = encodeWithOffset(offset, func(w *Writer) { SerializeFloat32(w, f32) })
			want = encodeWithOffset(offset, func(w *Writer) { w.WriteBytes(PackFloat32(f32)) })
			if !bytes.Equal(got, want) {
				t.Fatalf("float32 %v offset %d: got %x want %x", f32, offset, got, want)
			}
			rd := NewReader(got)
			var b byte
			for i := 0; i < offset; i++ {
				_ = rd.ReadBits(&b, 1)
			}
			var dec float32
			if err := DeserializeFloat32(&dec, rd); err != nil {
				t.Fatal(err)
			}
			if math.Float32bits(dec) != math.Float32bits(f32) {
				t.Fatalf("float32 round trip %v: got %v", f32, dec)
			}
		}
	}
	for _, v := range []bool{true, false} {
		for offset := 0; offset < 8; offset++ {
			got := encodeWithOffset(offset, func(w *Writer) { SerializeBool(w, v) })
			want := encodeWithOffset(offset, func(w *Writer) {
				x := uint8(0)
				if v {
					x = 1
				}
				w.WriteBits(x, 1)
			})
			if !bytes.Equal(got, want) {
				t.Fatalf("bool %v offset %d: got %x want %x", v, offset, got, want)
			}
		}
	}
}

// TestWriterResetClearsUsedBytes verifies the writer invariant that bytes past
// the cursor are zero after Reset, including bytes dirtied by the 64-bit fast
// paths, so a pooled writer never leaks stale bits into the next message.
func TestWriterResetClearsUsedBytes(t *testing.T) {
	w := NewWriter(64)
	for i := 0; i < 20; i++ {
		SerializeUInt64(w, math.MaxUint64)
		SerializeFloat64(w, math.Float64frombits(math.MaxUint64))
		w.WriteBits(1, 1)
	}
	w.Reset()
	for i, b := range w.bytes {
		if b != 0 {
			t.Fatalf("byte %d not cleared after Reset: %#x", i, b)
		}
	}
	SerializeUInt32(w, 5)
	fresh := NewWriter(1)
	SerializeUInt32(fresh, 5)
	if !bytes.Equal(w.Bytes(), fresh.Bytes()) {
		t.Fatalf("reused writer produced %x, fresh writer %x", w.Bytes(), fresh.Bytes())
	}
}

func TestBitsToBytes(t *testing.T) {
	for x := 0; x < 10000; x++ {
		if got, want := BitsToBytes(x), int(math.Ceil(float64(x)/8.0)); got != want {
			t.Fatalf("BitsToBytes(%d)=%d want %d", x, got, want)
		}
	}
}

func TestReadBits64RoundTrip(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	for i := 0; i < 5000; i++ {
		n := uint(1 + r.Intn(64))
		v := r.Uint64()
		if n < 64 {
			v &= (uint64(1) << n) - 1
		}
		offset := r.Intn(8)
		bs := encodeWithOffset(offset, func(w *Writer) { w.writeBits64(v, n) })
		rd := NewReader(bs)
		var b byte
		for j := 0; j < offset; j++ {
			_ = rd.ReadBits(&b, 1)
		}
		got, err := rd.readBits64(n)
		if err != nil {
			t.Fatal(err)
		}
		if got != v {
			t.Fatalf("n=%d offset=%d: got %#x want %#x", n, offset, got, v)
		}
		if rem := rd.RemainingBits(); rem >= 8 {
			t.Fatalf("n=%d offset=%d: %d bits left over", n, offset, rem)
		}
	}
}

// TestBitOpsAgainstModel checks WriteBits/ReadBits/writeBits64/readBits64
// against a trivial bit-slice model over random operation sequences, including
// reads and writes that touch the final byte of an exactly-sized buffer.
func TestBitOpsAgainstModel(t *testing.T) {
	r := rand.New(rand.NewSource(4))
	for iter := 0; iter < 300; iter++ {
		var model []bool
		type op struct {
			n   uint
			val uint64
		}
		var ops []op
		w := NewWriter(r.Intn(4)) // sometimes zero-capacity
		for i := 0; i < 50; i++ {
			var o op
			if r.Intn(2) == 0 {
				o.n = uint(r.Intn(9)) // WriteBits supports 0..8
				o.val = uint64(r.Intn(256)) & ((1 << o.n) - 1)
				w.WriteBits(uint8(o.val)|(uint8(r.Intn(256))&^uint8((1<<o.n)-1)), uint32(o.n)) // garbage above n must be masked
			} else {
				o.n = uint(1 + r.Intn(64))
				o.val = r.Uint64()
				if o.n < 64 {
					o.val &= (uint64(1) << o.n) - 1
				}
				w.writeBits64(o.val, o.n)
			}
			for b := uint(0); b < o.n; b++ {
				model = append(model, (o.val>>b)&1 == 1)
			}
			ops = append(ops, o)
		}
		bs := w.Bytes()
		if len(bs) != (len(model)+7)/8 {
			t.Fatalf("length %d want %d", len(bs), (len(model)+7)/8)
		}
		for i, bit := range model {
			if ((bs[i/8]>>(i%8))&1 == 1) != bit {
				t.Fatalf("iter %d bit %d mismatch", iter, i)
			}
		}
		// Exact-size copy so reads hit the final-byte slow paths.
		rd := NewReader(append([]byte(nil), bs...))
		for _, o := range ops {
			var got uint64
			if o.n <= 8 && r.Intn(2) == 0 {
				var b byte
				if err := rd.ReadBits(&b, uint32(o.n)); err != nil {
					t.Fatal(err)
				}
				got = uint64(b)
			} else {
				var err error
				if got, err = rd.readBits64(o.n); err != nil {
					t.Fatal(err)
				}
			}
			if got != o.val {
				t.Fatalf("iter %d: read %#x want %#x (n=%d)", iter, got, o.val, o.n)
			}
		}
		var b byte
		if rd.RemainingBits() < 8 {
			if err := rd.ReadBits(&b, 8); err == nil {
				t.Fatalf("expected insufficient-data error at end")
			}
		}
	}
}
