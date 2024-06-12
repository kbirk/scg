package serialize

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type StructA struct {
	ValInt8    int8
	ValFloat32 float32
	ValBool    bool
}

type StructB struct {
	ValArrayInt     []int32
	ValMapStringInt map[string]int32
}

type ComplicatedStruct struct {
	StructAMap   map[string]StructA
	StructBArray []StructB
}

type BasicStruct struct {
	ValInt16            int16
	ValArrayString      []string
	ValMapStringFloat32 map[string]float32
}

func TestSerializeString(t *testing.T) {

	input := "this is my test string"

	size := ByteSizeString(input)

	writer := NewFixedSizeWriter(size)
	SerializeString(writer, input)

	bs := writer.Bytes()
	reader := NewReader(bs)

	var output string
	err := DeserializeString(&output, reader)
	require.NoError(t, err)

	assert.Equal(t, input, output)
}

func TestSerializeBool(t *testing.T) {

	input := true

	size := ByteSizeBool(input)

	writer := NewFixedSizeWriter(size)
	SerializeBool(writer, input)

	bs := writer.Bytes()
	reader := NewReader(bs)

	var output bool
	err := DeserializeBool(&output, reader)
	require.NoError(t, err)

	assert.Equal(t, input, output)
}

func TestSerializeUInt8(t *testing.T) {

	input := uint8(224)

	size := ByteSizeUInt8(input)

	writer := NewFixedSizeWriter(size)
	SerializeUInt8(writer, input)

	bs := writer.Bytes()
	reader := NewReader(bs)

	var output uint8
	err := DeserializeUInt8(&output, reader)
	require.NoError(t, err)

	assert.Equal(t, input, output)
}

func TestSerializeInt8(t *testing.T) {

	input := int8(-112)

	size := ByteSizeInt8(input)

	writer := NewFixedSizeWriter(size)
	SerializeInt8(writer, input)

	bs := writer.Bytes()
	reader := NewReader(bs)

	var output int8
	err := DeserializeInt8(&output, reader)
	require.NoError(t, err)

	assert.Equal(t, input, output)
}

func TestSerializeUInt32(t *testing.T) {

	input := uint32(12234523)

	size := ByteSizeUInt32(input)

	writer := NewFixedSizeWriter(size)
	SerializeUInt32(writer, input)

	bs := writer.Bytes()
	reader := NewReader(bs)

	var output uint32
	err := DeserializeUInt32(&output, reader)
	require.NoError(t, err)

	assert.Equal(t, input, output)
}

func TestSerializeInt32(t *testing.T) {

	input := int32(-2147483648)

	size := ByteSizeInt32(input)

	writer := NewFixedSizeWriter(size)
	SerializeInt32(writer, input)

	bs := writer.Bytes()
	reader := NewReader(bs)

	var output int32
	err := DeserializeInt32(&output, reader)
	require.NoError(t, err)

	assert.Equal(t, input, output)
}

func TestSerializeUInt64(t *testing.T) {

	input := uint64(122346575523)

	size := ByteSizeUInt64(input)

	writer := NewFixedSizeWriter(size)
	SerializeUInt64(writer, input)

	bs := writer.Bytes()
	reader := NewReader(bs)

	var output uint64
	err := DeserializeUInt64(&output, reader)
	require.NoError(t, err)

	assert.Equal(t, input, output)
}

func TestSerializeInt64(t *testing.T) {

	input := int64(-2156747483648)

	size := ByteSizeInt64(input)

	writer := NewFixedSizeWriter(size)
	SerializeInt64(writer, input)

	bs := writer.Bytes()
	reader := NewReader(bs)

	var output int64
	err := DeserializeInt64(&output, reader)
	require.NoError(t, err)

	assert.Equal(t, input, output)
}
