package test

import (
	"runtime"
	"testing"

	"github.com/kbirk/scg/pkg/serialize"
	"github.com/kbirk/scg/test/scg/generated/basic"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGeneratedContainerRejectsOversizedLength verifies the generated container
// deserializers bound their initial allocation against the bytes actually
// present, so a message declaring a huge list length cannot force a multi-GiB
// allocation before the elements are read. StructB's first field is a
// list<int32>; the crafted frame claims ~268M elements but supplies a handful of
// bytes.
func TestGeneratedContainerRejectsOversizedLength(t *testing.T) {
	w := serialize.NewWriter(16)
	serialize.SerializeUInt32(w, 1<<28) // hostile element count (~268M)
	w.WriteBytes([]byte{1, 2, 3, 4, 5, 6, 7, 8})
	r := serialize.NewReader(w.Bytes())

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	var s basic.StructB
	err := s.Deserialize(r)

	runtime.ReadMemStats(&after)

	require.Error(t, err, "oversized list length should be rejected as the data is exhausted")
	// 1<<28 int32s preallocated would be ~1 GiB; the bound keeps it tiny.
	assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(16<<20))
}

// TestGeneratedContainerRoundTripsLargeLegitimateList guards against the bound
// over-rejecting a genuinely large list whose elements really are present.
func TestGeneratedContainerRoundTripsLargeLegitimateList(t *testing.T) {
	original := basic.StructB{
		ValArrayInt:     make([]int32, 10000),
		ValMapStringInt: map[string]int32{},
	}
	for i := range original.ValArrayInt {
		original.ValArrayInt[i] = int32(i)
	}

	w := serialize.NewWriter(serialize.BitsToBytes(original.BitSize()))
	original.Serialize(w)

	var decoded basic.StructB
	require.NoError(t, decoded.Deserialize(serialize.NewReader(w.Bytes())))
	require.Len(t, decoded.ValArrayInt, 10000)
	require.Equal(t, int32(9999), decoded.ValArrayInt[9999])
}
