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

func (w *FixedSizeWriter) Write(bs []byte) {
	n := len(bs)
	slice := w.Next(n)
	copy(slice, bs)
}

func (w *FixedSizeWriter) Bytes() []byte {
	if w.wpos != len(w.bytes) {
		panic(fmt.Sprintf("leftover space, missing %d bytes", +len(w.bytes)-w.wpos))
	}
	return w.bytes[:w.wpos]
}

type Writer struct {
	bytes []byte
}

func NewWriter(bs []byte) *Writer {
	return &Writer{
		bytes: bs,
	}
}

func (w *Writer) Next(n int) []byte {
	w.bytes = append(w.bytes, make([]byte, n)...)
	return w.bytes[len(w.bytes)-n:]
}

func (w *Writer) Write(bs []byte) {
	n := len(bs)
	slice := w.Next(n)
	copy(slice, bs)
}

func (w *Writer) Bytes() []byte {
	return w.bytes
}
