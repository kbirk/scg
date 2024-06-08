package serialize

import (
	"fmt"
)

type FixedSizeWriter struct {
	bytes []byte
	wpos  int
}

func NewFixedSizeWriter(size int) *FixedSizeWriter {
	return &FixedSizeWriter{
		bytes: make([]byte, size),
	}
}

func (w *FixedSizeWriter) Next(n int) []byte {
	if w.wpos+n > len(w.bytes) {
		panic(fmt.Sprintf("not enough space, need %d bytes but only %d available", n, len(w.bytes)-w.wpos))
	}
	slice := w.bytes[w.wpos : w.wpos+n]
	w.wpos += n
	return slice
}

func (w *FixedSizeWriter) Bytes() []byte {
	if w.wpos != len(w.bytes) {
		panic(fmt.Sprintf("leftover space, missing %d bytes", +len(w.bytes)-w.wpos))
	}
	return w.bytes[:w.wpos]
}
