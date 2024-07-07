package serialize

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
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

func TestSerializeUUID(t *testing.T) {

	input := uuid.New()

	size := BitSizeUUID(input)

	writer := NewFixedSizeWriter(BitsToBytes(size))
	SerializeUUID(writer, input)

	bs := writer.Bytes()
	reader := NewReader(bs)

	var output uuid.UUID
	err := DeserializeUUID(&output, reader)
	require.NoError(t, err)

	assert.Equal(t, input, output)
}

func TestSerializeTime(t *testing.T) {

	input := time.Now()

	size := BitSizeTime(input)

	writer := NewFixedSizeWriter(BitsToBytes(size))
	SerializeTime(writer, input)

	bs := writer.Bytes()
	reader := NewReader(bs)

	var output time.Time
	err := DeserializeTime(&output, reader)
	require.NoError(t, err)

	assert.True(t, input.Equal(output))
}

func TestSerializeString(t *testing.T) {

	input := "Hello, World! This is my test string 12312341234! \\@#$%@&^&%^\n newline \t _yay 世界"

	size := BitSizeString(input)

	writer := NewFixedSizeWriter(BitsToBytes(size))
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

	size := BitSizeBool(input)

	writer := NewFixedSizeWriter(BitsToBytes(size))
	SerializeBool(writer, input)

	bs := writer.Bytes()
	reader := NewReader(bs)

	var output bool
	err := DeserializeBool(&output, reader)
	require.NoError(t, err)

	assert.Equal(t, input, output)

	input = false

	size = BitSizeBool(input)

	writer = NewFixedSizeWriter(BitsToBytes(size))
	SerializeBool(writer, input)

	bs = writer.Bytes()
	reader = NewReader(bs)

	err = DeserializeBool(&output, reader)
	require.NoError(t, err)

	assert.Equal(t, input, output)
}

func TestSerializeUInt8(t *testing.T) {

	numSteps := 256
	for i := 0; i < numSteps; i++ {
		input := uint8(i)

		size := BitSizeUInt8(input)

		writer := NewFixedSizeWriter(BitsToBytes(size))
		SerializeUInt8(writer, input)

		bs := writer.Bytes()
		reader := NewReader(bs)

		var output uint8
		err := DeserializeUInt8(&output, reader)
		require.NoError(t, err)

		assert.Equal(t, input, output)
	}
}

func TestSerializeInt8(t *testing.T) {

	numSteps := 256
	for i := numSteps / 2; i < numSteps/2; i++ {
		input := int8(i)

		size := BitSizeInt8(input)

		writer := NewFixedSizeWriter(BitsToBytes(size))
		SerializeInt8(writer, input)

		bs := writer.Bytes()
		reader := NewReader(bs)

		var output int8
		err := DeserializeInt8(&output, reader)
		require.NoError(t, err)

		assert.Equal(t, input, output)
	}
}

func TestSerializeUInt16(t *testing.T) {
	for i := 0; i < math.MaxUint16; i++ {
		input := uint16(i)

		size := BitSizeUInt16(input)

		writer := NewFixedSizeWriter(BitsToBytes(size))
		SerializeUInt16(writer, input)

		bs := writer.Bytes()
		reader := NewReader(bs)

		var output uint16
		err := DeserializeUInt16(&output, reader)
		require.NoError(t, err)

		assert.Equal(t, input, output)
	}
}

func TestSerializeInt16(t *testing.T) {
	for i := math.MinInt16; i < math.MaxInt16; i++ {
		input := int16(i)

		size := BitSizeInt16(input)

		writer := NewFixedSizeWriter(BitsToBytes(size))
		SerializeInt16(writer, input)

		bs := writer.Bytes()
		reader := NewReader(bs)

		var output int16
		err := DeserializeInt16(&output, reader)
		require.NoError(t, err)

		assert.Equal(t, input, output)
	}
}

func TestSerializeUInt32(t *testing.T) {

	numSteps := math.MaxUint16
	step := math.MaxUint32 / numSteps
	for i := 0; i < numSteps; i++ {
		input := uint32(i * step)

		size := BitSizeUInt32(input)

		writer := NewFixedSizeWriter(BitsToBytes(size))
		SerializeUInt32(writer, input)

		bs := writer.Bytes()
		reader := NewReader(bs)

		var output uint32
		err := DeserializeUInt32(&output, reader)
		require.NoError(t, err)

		assert.Equal(t, input, output)
	}
}

func TestSerializeInt32(t *testing.T) {
	numSteps := math.MaxUint16
	step := math.MaxUint32 / numSteps
	for i := -numSteps / 2; i < numSteps/2; i++ {
		input := int32(i * int(step))

		size := BitSizeInt32(input)

		writer := NewFixedSizeWriter(BitsToBytes(size))
		SerializeInt32(writer, input)

		bs := writer.Bytes()
		reader := NewReader(bs)

		var output int32
		err := DeserializeInt32(&output, reader)
		require.NoError(t, err)

		assert.Equal(t, input, output)
	}
}

func TestSerializeUInt64(t *testing.T) {

	numSteps := math.MaxUint16
	step := math.MaxUint64 / uint64(numSteps)
	for i := 0; i < numSteps; i++ {
		input := uint64(i) * step

		size := BitSizeUInt64(input)

		writer := NewFixedSizeWriter(BitsToBytes(size))
		SerializeUInt64(writer, input)

		bs := writer.Bytes()
		reader := NewReader(bs)

		var output uint64
		err := DeserializeUInt64(&output, reader)
		require.NoError(t, err)

		assert.Equal(t, input, output)
	}
}

func TestSerializeInt64(t *testing.T) {

	numSteps := math.MaxUint16
	step := math.MaxUint64 / uint64(numSteps)
	for i := -numSteps / 2; i < numSteps/2; i++ {
		input := int64(int64(i) * int64(step))

		size := BitSizeInt64(input)

		writer := NewFixedSizeWriter(BitsToBytes(size))
		SerializeInt64(writer, input)

		bs := writer.Bytes()
		reader := NewReader(bs)

		var output int64
		err := DeserializeInt64(&output, reader)
		require.NoError(t, err)

		assert.Equal(t, input, output)
	}
}
