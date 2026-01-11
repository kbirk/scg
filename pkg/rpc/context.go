package rpc

import (
	"context"

	"github.com/kbirk/scg/pkg/serialize"
)

type metadataKey struct{}

type Metadata struct {
	vals map[string][]byte
}

func NewMetadata() *Metadata {
	return &Metadata{
		vals: make(map[string][]byte),
	}
}

func (m *Metadata) Put(key string, value Message) {
	m.vals[key] = value.ToBytes()
}

func (m *Metadata) PutBytes(key string, value []byte) {
	m.vals[key] = value
}

func (m *Metadata) PutString(key string, value string) {
	size := serialize.BitSizeString(value)
	writer := serialize.NewWriter(serialize.BitsToBytes(size))
	serialize.SerializeString(writer, value)
	m.vals[key] = writer.Bytes()
}

func (m *Metadata) Append(other *Metadata) {
	for k, v := range other.vals {
		m.vals[k] = v
	}
}

func (m *Metadata) Get(msg Message, key string) (bool, error) {
	bs, ok := m.vals[key]
	if !ok {
		return false, nil
	}
	return true, msg.FromBytes(bs)
}

func (m *Metadata) GetBytes(key string) ([]byte, bool) {
	val, ok := m.vals[key]
	return val, ok
}

func (m *Metadata) GetString(key string) (string, bool, error) {
	bs, ok := m.vals[key]
	if !ok {
		return "", false, nil
	}

	var val string
	reader := serialize.NewReader(bs)
	err := serialize.DeserializeString(&val, reader)
	if err != nil {
		return "", true, err
	}

	return val, true, nil
}

func NewContextWithMetadata(ctx context.Context, metadata *Metadata) context.Context {
	return context.WithValue(ctx, metadataKey{}, metadata)
}

func AppendMetadataToContext(ctx context.Context, metadata *Metadata) context.Context {
	i := ctx.Value(metadataKey{})
	if i == nil {
		return context.WithValue(ctx, metadataKey{}, metadata)
	}
	existing, ok := i.(Metadata)
	if !ok {
		return context.WithValue(ctx, metadataKey{}, metadata)
	}
	existing.Append(metadata)
	return context.WithValue(ctx, metadataKey{}, existing)
}

func GetMetadataFromContext(ctx context.Context) *Metadata {
	v := ctx.Value(metadataKey{})
	if v != nil {
		md, ok := v.(*Metadata)
		if ok {
			return md
		}
	}
	return nil
}

func BitSizeContext(ctx context.Context) int {
	md := GetMetadataFromContext(ctx)
	if md == nil {
		return serialize.BitSizeUInt32(0)
	}

	size := serialize.BitSizeUInt32(uint32(len(md.vals)))
	for k, v := range md.vals {
		size += serialize.BitSizeString(k)
		size += serialize.BitSizeUInt32(uint32(len(v)))
		size += len(v) * 8
	}
	return size
}

func SerializeContext(writer *serialize.Writer, ctx context.Context) {
	md := GetMetadataFromContext(ctx)
	if md == nil {
		serialize.SerializeUInt32(writer, 0)
		return
	}

	serialize.SerializeUInt32(writer, uint32(len(md.vals)))
	for k, v := range md.vals {
		serialize.SerializeString(writer, k)

		serialize.SerializeUInt32(writer, uint32(len(v)))
		for _, b := range v {
			writer.WriteBits(b, 8)
		}
	}
}

func DeserializeContext(ctx *context.Context, reader *serialize.Reader) error {
	var size uint32
	err := serialize.DeserializeUInt32(&size, reader)
	if err != nil {
		return err
	}
	if size > 0 {
		md := NewMetadata()
		for i := 0; i < int(size); i++ {
			var k string
			err = serialize.DeserializeString(&k, reader)
			if err != nil {
				return err
			}

			var size uint32
			err := serialize.DeserializeUInt32(&size, reader)
			if err != nil {
				return err
			}

			val := make([]byte, size)
			for i := 0; i < int(size); i++ {
				err := reader.ReadBits(&val[i], 8)
				if err != nil {
					return err
				}
			}

			md.PutBytes(k, val)
		}
		*ctx = context.WithValue(*ctx, metadataKey{}, md)
	}
	return nil
}
