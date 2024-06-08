package test

import (
	"testing"

	"github.com/kbirk/scg/pkg/serialize"
	"github.com/kbirk/scg/test/files/output/basic"
	"github.com/stretchr/testify/assert"
)

func TestSerializeBasic(t *testing.T) {
	input := basic.BasicStruct{
		ValInt16:            1,
		ValArrayString:      []string{"a", "b", "c"},
		ValMapStringFloat32: map[string]float32{"key1": 1.0, "key2": 2.0},
	}
	size := input.CalcByteSize()
	writer := serialize.NewFixedSizeWriter(size)
	input.Serialize(writer)

	bs := writer.Bytes()

	var output basic.BasicStruct
	reader := serialize.NewReader(bs)
	output.Deserialize(reader)

	assert.Equal(t, input, output)
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
			},
			"key2": {
				ValInt8:    2,
				ValFloat32: 2.0,
				ValBool:    false,
				ValMapUint8String: map[uint8]string{
					1: "a",
					2: "b",
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
	}
	size := input.CalcByteSize()
	writer := serialize.NewFixedSizeWriter(size)
	input.Serialize(writer)

	bs := writer.Bytes()

	var output basic.ComplicatedStruct
	reader := serialize.NewReader(bs)
	output.Deserialize(reader)

	assert.Equal(t, input, output)
}
