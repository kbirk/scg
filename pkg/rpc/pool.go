package rpc

import (
	"sync"

	"github.com/kbirk/scg/pkg/serialize"
)

var (
	// Single pool for writers with a reasonable starting capacity
	writerPool = &sync.Pool{
		New: func() interface{} {
			return serialize.NewWriter(256)
		},
	}
)

// getWriter returns a writer from the pool with the requested capacity
func getWriter(size int) *serialize.Writer {
	w := writerPool.Get().(*serialize.Writer)
	w.Grow(size)
	return w
}

// putWriter returns a writer to the pool after resetting it
func putWriter(w *serialize.Writer) {
	w.Reset()
	// Only return to pool if capacity is reasonable (< 256KB)
	// This prevents memory bloat from very large messages
	if w.Capacity() < 262144 {
		writerPool.Put(w)
	}
}
