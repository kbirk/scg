package serialize

import (
	"fmt"
)

type Reader struct {
	bytes []byte
	pos   int
}

func NewReader(data []byte) *Reader {
	return &Reader{
		bytes: data,
		pos:   0,
	}
}

func (b *Reader) Read(n int) ([]byte, error) {
	if n < 0 {
		// return remaining
		b.pos = len(b.bytes)
		return b.bytes[b.pos:], nil
	}

	if b.pos+n > len(b.bytes) {
		return nil, fmt.Errorf("not enough data")
	}
	data := b.bytes[b.pos : b.pos+n]
	b.pos += n
	return data, nil
}
