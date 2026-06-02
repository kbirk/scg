package serialize

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemainingBytes(t *testing.T) {
	r := NewReader([]byte{1, 2, 3, 4})
	assert.Equal(t, 4, r.RemainingBytes())

	var b byte
	require.NoError(t, r.ReadByte(&b))
	assert.Equal(t, 3, r.RemainingBytes())

	require.NoError(t, r.ReadBytes(make([]byte, 3)))
	assert.Equal(t, 0, r.RemainingBytes())
}

func TestCheckLength(t *testing.T) {
	r := NewReader([]byte{1, 2, 3, 4})
	require.NoError(t, CheckLength(r, 0))
	require.NoError(t, CheckLength(r, 4))

	err := CheckLength(r, 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remaining")

	// A hostile length far exceeding the buffer must be rejected.
	require.Error(t, CheckLength(r, 1<<30))
}

// TestDeserializeStringRejectsOversizedLength verifies a string whose declared
// length far exceeds the bytes present is rejected by the remaining-bytes guard
// (rather than being read after a huge allocation).
func TestDeserializeStringRejectsOversizedLength(t *testing.T) {
	w := NewWriter(8)
	SerializeUInt32(w, 1<<30) // declare a ~1 GiB string
	w.WriteBytes([]byte("abc"))
	r := NewReader(w.Bytes())

	var s string
	err := DeserializeString(&s, r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remaining")
}
