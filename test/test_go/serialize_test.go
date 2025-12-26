package test

import (
	"context"
	"testing"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/serialize"
	"github.com/kbirk/scg/test/files/output/basic"
	"github.com/stretchr/testify/assert"
)

func TestSerializeContext(t *testing.T) {

	s := &basic.StructA{
		ValInt8:    1,
		ValFloat32: 1.0,
		ValBool:    true,
		ValMapStringTimestamp: map[string]time.Time{
			"now": time.Now(),
		},
	}

	metadata := rpc.NewMetadata()
	metadata.PutString("key1", "val1")
	metadata.Put("key2", s)
	metadata.PutBytes("key3", []byte{1, 2, 3})

	input := rpc.NewContextWithMetadata(context.Background(), metadata)

	size := rpc.BitSizeContext(input)
	writer := serialize.NewWriter(serialize.BitsToBytes(size))
	rpc.SerializeContext(writer, input)

	bs := writer.Bytes()

	output := context.Background()
	reader := serialize.NewReader(bs)
	rpc.DeserializeContext(&output, reader)

	md := rpc.GetMetadataFromContext(output)

	assert.Equal(t, metadata, md)
}

func TestSerializeEnum(t *testing.T) {
	input := basic.SomeEnum_ValueA
	size := input.BitSize()
	writer := serialize.NewWriter(serialize.BitsToBytes(size))
	input.Serialize(writer)

	bs := writer.Bytes()

	var output basic.SomeEnum
	reader := serialize.NewReader(bs)
	output.Deserialize(reader)

	assert.Equal(t, input, output)

	input = basic.SomeEnum_ValueB
	size = input.BitSize()
	writer = serialize.NewWriter(serialize.BitsToBytes(size))
	input.Serialize(writer)

	bs = writer.Bytes()

	reader = serialize.NewReader(bs)
	output.Deserialize(reader)

	assert.Equal(t, input, output)
}

func TestSerializeBasic(t *testing.T) {
	input := basic.BasicStruct{
		ValInt8:             -1,
		ValInt16:            -2,
		ValInt32:            -3,
		ValInt64:            -4,
		ValUint8:            5,
		ValUint16:           6,
		ValUint32:           7,
		ValUint64:           8,
		ValString:           "Hello, 世界",
		ValArrayString:      []string{"a", "b", "c"},
		ValMapStringFloat32: map[string]float32{"key1": 1.0, "key2": 2.0},
		ValTimestamp:        time.Now(),
		ValEnum:             basic.SomeEnum_ValueA,
		ValListEnum:         []basic.SomeEnum{basic.SomeEnum_ValueA, basic.SomeEnum_ValueB},
		ValMapEnumString: map[basic.SomeEnum]string{
			basic.SomeEnum_ValueA: "a",
			basic.SomeEnum_ValueB: "b",
		},
		ValMapStringEnum: map[string]basic.SomeEnum{
			"a": basic.SomeEnum_ValueA,
			"b": basic.SomeEnum_ValueB,
		},
		ValEmpty: basic.EmptyStruct{},
		ValNestedEmpty: basic.NestedEmpty{
			Empty: basic.EmptyStruct{},
		},
		ValByteArray: []byte{1, 2, 3},
	}
	size := input.BitSize()
	writer := serialize.NewWriter(serialize.BitsToBytes(size))
	input.Serialize(writer)

	bs := writer.Bytes()

	var output basic.BasicStruct
	reader := serialize.NewReader(bs)
	output.Deserialize(reader)

	assert.Equal(t, input.ValInt8, output.ValInt8)
	assert.Equal(t, input.ValInt16, output.ValInt16)
	assert.Equal(t, input.ValInt32, output.ValInt32)
	assert.Equal(t, input.ValInt64, output.ValInt64)
	assert.Equal(t, input.ValUint8, output.ValUint8)
	assert.Equal(t, input.ValUint16, output.ValUint16)
	assert.Equal(t, input.ValUint32, output.ValUint32)
	assert.Equal(t, input.ValUint64, output.ValUint64)
	assert.Equal(t, input.ValString, output.ValString)
	assert.Equal(t, input.ValArrayString, output.ValArrayString)
	assert.Equal(t, input.ValMapStringFloat32, output.ValMapStringFloat32)
	assert.True(t, input.ValTimestamp.Equal(output.ValTimestamp))
	assert.Equal(t, input.ValEnum, output.ValEnum)
	assert.Equal(t, input.ValListEnum, output.ValListEnum)
	assert.Equal(t, input.ValMapEnumString, output.ValMapEnumString)
	assert.Equal(t, input.ValByteArray, output.ValByteArray)
}

func TestSerializeComplicated(t *testing.T) {

	input := basic.ComplicatedStruct{
		StructAMap: map[string]basic.StructA{
			"key1": {
				ValInt8:    1,
				ValFloat32: 1.0,
				ValBool:    true,
				ValMapUint8String: map[uint8]string{
					1: "a",
					2: "b",
				},
				ValTimestamp: []time.Time{time.Now()},
				ValMapStringTimestamp: map[string]time.Time{
					"key1": time.Now(),
					"key2": time.Now(),
				},
			},
			"key2": {
				ValInt8:    2,
				ValFloat32: 2.0,
				ValBool:    false,
				ValMapUint8String: map[uint8]string{
					1: "a",
					2: "b",
				},
				ValTimestamp: []time.Time{time.Now(), time.Now()},
				ValMapStringTimestamp: map[string]time.Time{
					"key111": time.Now(),
					"key222": time.Now(),
				},
			},
		},
		StructBArray: []basic.StructB{
			{
				ValArrayInt: []int32{1, 2, 3},
				ValMapStringInt: map[string]int32{
					"key1": 1,
					"key2": 2,
				},
				ValMapUint8MapUint16String: map[int8]map[int16]string{
					1: {
						2: "a",
					},
				},
			},
		},
		StructC: basic.StructC{
			Str:  "my-str",
			Strs: []basic.BasicString{"a", "b"},
			StrMap: map[basic.BasicString]basic.BasicString{
				"key1": "val1",
				"key2": "val2",
			},
		},
	}
	size := input.BitSize()
	writer := serialize.NewWriter(serialize.BitsToBytes(size))
	input.Serialize(writer)

	bs := writer.Bytes()

	var output basic.ComplicatedStruct
	reader := serialize.NewReader(bs)
	output.Deserialize(reader)

	for key, val := range input.StructAMap {
		assert.Equal(t, val.ValInt8, output.StructAMap[key].ValInt8)
		assert.Equal(t, val.ValFloat32, output.StructAMap[key].ValFloat32)
		assert.Equal(t, val.ValBool, output.StructAMap[key].ValBool)
		assert.Equal(t, val.ValMapUint8String, output.StructAMap[key].ValMapUint8String)
		for i, v := range val.ValTimestamp {
			assert.True(t, v.Equal(output.StructAMap[key].ValTimestamp[i]))
		}
		for k, v := range val.ValMapStringTimestamp {
			assert.True(t, v.Equal(output.StructAMap[key].ValMapStringTimestamp[k]))
		}
	}
	assert.Equal(t, input.StructBArray, output.StructBArray)
	assert.Equal(t, input.StructC, output.StructC)
}
