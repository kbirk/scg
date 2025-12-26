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

	writer := NewWriter(BitsToBytes(size))
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

	writer := NewWriter(BitsToBytes(size))
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

	writer := NewWriter(BitsToBytes(size))
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

	writer := NewWriter(BitsToBytes(size))
	SerializeBool(writer, input)

	bs := writer.Bytes()
	reader := NewReader(bs)

	var output bool
	err := DeserializeBool(&output, reader)
	require.NoError(t, err)

	assert.Equal(t, input, output)

	input = false

	size = BitSizeBool(input)

	writer = NewWriter(BitsToBytes(size))
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

		writer := NewWriter(BitsToBytes(size))
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

		writer := NewWriter(BitsToBytes(size))
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

		writer := NewWriter(BitsToBytes(size))
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

		writer := NewWriter(BitsToBytes(size))
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

		writer := NewWriter(BitsToBytes(size))
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

		writer := NewWriter(BitsToBytes(size))
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

		writer := NewWriter(BitsToBytes(size))
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

		writer := NewWriter(BitsToBytes(size))
		SerializeInt64(writer, input)

		bs := writer.Bytes()
		reader := NewReader(bs)

		var output int64
		err := DeserializeInt64(&output, reader)
		require.NoError(t, err)

		assert.Equal(t, input, output)
	}
}

func TestSerializeMultipleTypesInSequence(t *testing.T) {
	// Create test data of different types
	strValue := "Hello, World! 世界"
	uuidValue := uuid.New()
	timeValue := time.Now()
	boolValue := true
	uint8Value := uint8(42)
	uint32Value := uint32(12345678)
	int64Value := int64(-9876543210)
	float64Value := float64(3.14159)

	// Calculate total size
	totalSize := BitSizeString(strValue) +
		BitSizeUUID(uuidValue) +
		BitSizeTime(timeValue) +
		BitSizeBool(boolValue) +
		BitSizeUInt8(uint8Value) +
		BitSizeUInt32(uint32Value) +
		BitSizeInt64(int64Value) +
		BitSizeFloat64(float64Value)

	// Serialize all types into a single buffer
	writer := NewWriter(BitsToBytes(totalSize))
	SerializeString(writer, strValue)
	SerializeUUID(writer, uuidValue)
	SerializeTime(writer, timeValue)
	SerializeBool(writer, boolValue)
	SerializeUInt8(writer, uint8Value)
	SerializeUInt32(writer, uint32Value)
	SerializeInt64(writer, int64Value)
	SerializeFloat64(writer, float64Value)

	// Deserialize all types from the buffer in the same order
	bs := writer.Bytes()
	reader := NewReader(bs)

	var strOut string
	err := DeserializeString(&strOut, reader)
	require.NoError(t, err)
	assert.Equal(t, strValue, strOut)

	var uuidOut uuid.UUID
	err = DeserializeUUID(&uuidOut, reader)
	require.NoError(t, err)
	assert.Equal(t, uuidValue, uuidOut)

	var timeOut time.Time
	err = DeserializeTime(&timeOut, reader)
	require.NoError(t, err)
	assert.True(t, timeValue.Equal(timeOut))

	var boolOut bool
	err = DeserializeBool(&boolOut, reader)
	require.NoError(t, err)
	assert.Equal(t, boolValue, boolOut)

	var uint8Out uint8
	err = DeserializeUInt8(&uint8Out, reader)
	require.NoError(t, err)
	assert.Equal(t, uint8Value, uint8Out)

	var uint32Out uint32
	err = DeserializeUInt32(&uint32Out, reader)
	require.NoError(t, err)
	assert.Equal(t, uint32Value, uint32Out)

	var int64Out int64
	err = DeserializeInt64(&int64Out, reader)
	require.NoError(t, err)
	assert.Equal(t, int64Value, int64Out)

	var float64Out float64
	err = DeserializeFloat64(&float64Out, reader)
	require.NoError(t, err)
	assert.Equal(t, float64Value, float64Out)
}

func TestSerializeMultipleStringsInSequence(t *testing.T) {
	// Test multiple strings back-to-back to ensure boundaries are preserved
	strings := []string{
		"",      // empty string
		"short", // short string
		"Hello, World! This is a longer string with unicode 世界", // long string with unicode
		"another one", // another string
		"final string with special chars \n\t@#$%", // special chars
	}

	// Calculate total size
	totalSize := 0
	for _, s := range strings {
		totalSize += BitSizeString(s)
	}

	// Serialize all strings
	writer := NewWriter(BitsToBytes(totalSize))
	for _, s := range strings {
		SerializeString(writer, s)
	}

	// Deserialize all strings
	bs := writer.Bytes()
	reader := NewReader(bs)

	for i, expected := range strings {
		var actual string
		err := DeserializeString(&actual, reader)
		require.NoError(t, err, "Failed to deserialize string %d", i)
		assert.Equal(t, expected, actual, "String %d mismatch", i)
	}
}
