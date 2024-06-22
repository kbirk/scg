package rpc

import (
	"context"

	"github.com/kbirk/scg/pkg/serialize"
)

type metadataKey struct{}

func NewContextWithMetadata(ctx context.Context, metadata map[string]string) context.Context {
	return context.WithValue(ctx, metadataKey{}, metadata)
}

func AppendMetadataToContext(ctx context.Context, metadata map[string]string) context.Context {
	i := ctx.Value(metadataKey{})
	if i == nil {
		return context.WithValue(ctx, metadataKey{}, metadata)
	}
	existing, ok := i.(map[string]string)
	if !ok {
		return context.WithValue(ctx, metadataKey{}, metadata)
	}
	for k, v := range metadata {
		existing[k] = v
	}
	return context.WithValue(ctx, metadataKey{}, existing)
}

func GetMetadataFromContext(ctx context.Context) map[string]string {
	v := ctx.Value(metadataKey{})
	if v != nil {
		md, ok := v.(map[string]string)
		if ok {
			return md
		}
	}
	return nil
}

func ByteSizeContext(ctx context.Context) int {
	md := GetMetadataFromContext(ctx)
	size := serialize.ByteSizeUInt32(uint32(len(md)))
	for k, v := range md {
		size += serialize.ByteSizeString(k)
		size += serialize.ByteSizeString(v)
	}
	return size
}

func SerializeContext(writer *serialize.FixedSizeWriter, ctx context.Context) {
	md := GetMetadataFromContext(ctx)
	serialize.SerializeUInt32(writer, uint32(len(md)))
	for k, v := range md {
		serialize.SerializeString(writer, k)
		serialize.SerializeString(writer, v)
	}
}

func DeserializeContext(ctx *context.Context, reader *serialize.Reader) error {
	var size uint32
	err := serialize.DeserializeUInt32(&size, reader)
	if err != nil {
		return err
	}
	if size > 0 {
		md := make(map[string]string, size)
		for i := 0; i < int(size); i++ {
			var k, v string
			err = serialize.DeserializeString(&k, reader)
			if err != nil {
				return err
			}
			err = serialize.DeserializeString(&v, reader)
			if err != nil {
				return err
			}
			md[k] = v
		}
		*ctx = context.WithValue(*ctx, metadataKey{}, md)
	}
	return nil
}
