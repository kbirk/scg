package parse

// Minimum encoded sizes, in bits, of the wire format's primitive types. These
// are exact lower bounds: no valid encoding of a value of the type is shorter.
// Generated decoders use them to reject container lengths that cannot
// possibly fit in the bytes remaining, and to bound preallocation.
const (
	minBitsBool     = 1   // single bit
	minBitsFixed8   = 8   // byte / uint8 / int8 are written as raw bytes
	minBitsVarUint  = 1   // unsigned varint: a lone zero continuation bit
	minBitsVarInt   = 2   // signed varint: sign bit + unsigned varint
	minBitsFloat32  = 32  // raw IEEE-754
	minBitsFloat64  = 64  // raw IEEE-754
	minBitsUUID     = 128 // raw 16 bytes
	minBitsString   = minBitsVarUint
	minBitsTime     = 2 * minBitsVarUint // seconds + nanoseconds varints
	minBitsLength   = minBitsVarUint     // list / map element count
	minBitsEnumType = minBitsVarUint     // enums are uint16 varints
)

// FixedBitSize returns the exact encoded size of a type whose encoding does
// not depend on its value, and false for variable-size types.
func FixedBitSize(dataType *DataTypeDefinition) (int, bool) {
	switch dataType.Type {
	case DataTypeBool:
		return minBitsBool, true
	case DataTypeByte, DataTypeUInt8, DataTypeInt8:
		return minBitsFixed8, true
	case DataTypeFloat32:
		return minBitsFloat32, true
	case DataTypeFloat64:
		return minBitsFloat64, true
	case DataTypeUUID:
		return minBitsUUID, true
	}
	return 0, false
}

func minBitSizeComparableType(typ DataTypeComparable) int {
	switch typ {
	case DataTypeComparableUInt8, DataTypeComparableInt8:
		return minBitsFixed8
	case DataTypeComparableUInt16, DataTypeComparableUInt32, DataTypeComparableUInt64:
		return minBitsVarUint
	case DataTypeComparableInt16, DataTypeComparableInt32, DataTypeComparableInt64:
		return minBitsVarInt
	case DataTypeComparableFloat32:
		return minBitsFloat32
	case DataTypeComparableFloat64:
		return minBitsFloat64
	case DataTypeComparableString:
		return minBitsString
	case DataTypeComparableUUID:
		return minBitsUUID
	}
	// unknown: the smallest possible non-zero size keeps bounds conservative
	return 1
}

// MinBitSizeComparable returns the minimum encoded size of a map key type.
// It is always at least one bit.
func (p *Parse) MinBitSizeComparable(dataType *DataTypeComparableDefinition) int {
	if dataType.Type != DataTypeComparableCustom {
		return minBitSizeComparableType(dataType.Type)
	}
	if pkg, ok := p.Packages[dataType.CustomTypePackage]; ok {
		if typdef, ok := pkg.Typedefs[dataType.CustomType]; ok {
			return minBitSizeComparableType(typdef.DataTypeDefinition.Type)
		}
		if _, ok := pkg.Enums[dataType.CustomType]; ok {
			return minBitsEnumType
		}
	}
	return 1
}

// MinBitSize returns the minimum encoded size of a value of the given type.
// Only zero-field (or recursively zero-bit) messages are zero bits; lists of
// them are rejected during resolution.
func (p *Parse) MinBitSize(dataType *DataTypeDefinition) int {
	switch dataType.Type {
	case DataTypeBool:
		return minBitsBool
	case DataTypeByte, DataTypeUInt8, DataTypeInt8:
		return minBitsFixed8
	case DataTypeUInt16, DataTypeUInt32, DataTypeUInt64:
		return minBitsVarUint
	case DataTypeInt16, DataTypeInt32, DataTypeInt64:
		return minBitsVarInt
	case DataTypeFloat32:
		return minBitsFloat32
	case DataTypeFloat64:
		return minBitsFloat64
	case DataTypeString:
		return minBitsString
	case DataTypeTimestamp:
		return minBitsTime
	case DataTypeUUID:
		return minBitsUUID
	case DataTypeList, DataTypeMap:
		return minBitsLength
	case DataTypeCustom:
		pkg, ok := p.Packages[dataType.CustomTypePackage]
		if !ok {
			return 1
		}
		if msg, ok := pkg.MessageDefinitions[dataType.CustomType]; ok {
			return p.minBitSizeMessage(msg)
		}
		if typdef, ok := pkg.Typedefs[dataType.CustomType]; ok {
			return minBitSizeComparableType(typdef.DataTypeDefinition.Type)
		}
		if _, ok := pkg.Enums[dataType.CustomType]; ok {
			return minBitsEnumType
		}
	}
	return 1
}

func (p *Parse) minBitSizeMessage(msg *MessageDefinition) int {
	key := messageKey(msg.File.Package.Name, msg.Name)
	if p.minBitsMemo == nil {
		p.minBitsMemo = map[string]int{}
	}
	if v, ok := p.minBitsMemo[key]; ok {
		return v
	}
	total := 0
	for _, field := range msg.FieldsByIndex() {
		total += p.MinBitSize(field.DataTypeDefinition)
	}
	p.minBitsMemo[key] = total
	return total
}
