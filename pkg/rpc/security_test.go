package rpc

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/kbirk/scg/pkg/serialize"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeserializeContextRejectsOversizedValue verifies that a context frame
// declaring a huge metadata value length is rejected before allocating. Context
// is parsed on every request and, for streaming, on every OPEN *before* auth, so
// this is the pre-auth allocation DoS the streaming work made reachable on a
// public connection.
func TestDeserializeContextRejectsOversizedValue(t *testing.T) {
	// One entry: key "k", value byte-length = ~1 GiB, with no value bytes following.
	w := serialize.NewWriter(32)
	serialize.SerializeUInt32(w, 1)     // entry count
	serialize.SerializeString(w, "k")   // key
	serialize.SerializeUInt32(w, 1<<30) // value byte length (hostile)
	r := serialize.NewReader(w.Bytes())

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	ctx := context.Background()
	err := DeserializeContext(&ctx, r)

	runtime.ReadMemStats(&after)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "remaining")
	// The guard must reject before allocating — far below the 1 GiB the declared
	// length would otherwise force.
	assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(16<<20))
}

// TestDeserializeContextRejectsOversizedCount verifies a context declaring more
// entries than the frame could possibly hold is rejected up front.
func TestDeserializeContextRejectsOversizedCount(t *testing.T) {
	w := serialize.NewWriter(8)
	serialize.SerializeUInt32(w, 1<<30) // entry count (hostile), no entries follow
	r := serialize.NewReader(w.Bytes())

	ctx := context.Background()
	err := DeserializeContext(&ctx, r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remaining")
}

// TestServerStreamContextCancelledOnDie verifies the server handler's context is
// cancelled when the stream dies, so a push-only handler can observe client
// cancellation / connection loss via Context().Done() instead of only on its
// next Send.
func TestServerStreamContextCancelledOnDie(t *testing.T) {
	s := newServerStream(nil, context.Background(), 1, 2, 0)

	select {
	case <-s.Context().Done():
		t.Fatal("context should not be done before the stream dies")
	default:
	}

	cause := errors.New("client gone")
	s.die(cause)

	select {
	case <-s.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("context was not cancelled after die")
	}
	require.Equal(t, cause, context.Cause(s.Context()))
}
