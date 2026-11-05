package cpp_gen

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

type ConstArgs struct {
	// Prefix is `constexpr`, or `inline const` for types that are not literal
	// types (typedefs of std::string).
	Prefix              string
	ConstNameUpperCase  string
	ConstUnderlyingType string
	ConstValue          string
}

const constTemplateStr = `
{{.Prefix}} {{.ConstUnderlyingType}} {{.ConstNameUpperCase}} = {{.ConstValue}};`

var (
	constTemplate = template.Must(template.New("constTemplateGo").Parse(constTemplateStr))
)

// quoteCppString renders bytes as a portable C++ narrow string literal. Only
// printable ASCII is emitted verbatim; everything else is written as a 3 digit
// octal escape, which (unlike \x escapes) cannot absorb a following character
// and does not depend on the compiler's source character set.
func quoteCppString(s string) string {
	var sb strings.Builder
	sb.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			sb.WriteString(`\"`)
		case c == '\\':
			sb.WriteString(`\\`)
		case c == '\n':
			sb.WriteString(`\n`)
		case c == '\r':
			sb.WriteString(`\r`)
		case c == '\t':
			sb.WriteString(`\t`)
		case c == '?' && i > 0 && s[i-1] == '?':
			// "??x" could form a trigraph on pre-C++17 compilers
			sb.WriteString(`\?`)
		case c >= 0x20 && c < 0x7f:
			sb.WriteByte(c)
		default:
			fmt.Fprintf(&sb, `\%03o`, c)
		}
	}
	sb.WriteByte('"')
	return sb.String()
}

// cppIntLiteral renders a validated integer const value so that it is a
// well-formed literal of a type that can hold it.
func cppIntLiteral(typ parse.DataTypeComparable, value string) string {
	if strings.HasPrefix(value, "0x") {
		// hex literals take an unsigned type when needed, no suffix required
		return value
	}
	switch typ {
	case parse.DataTypeComparableUInt64:
		v, err := strconv.ParseUint(strings.TrimPrefix(value, "+"), 10, 64)
		if err == nil && v > 9223372036854775807 {
			// too large for long long: an unsuffixed decimal literal is ill-formed
			return value + "ULL"
		}
	case parse.DataTypeComparableInt64:
		if value == "-9223372036854775808" {
			// the literal 9223372036854775808 does not fit in long long
			return "(-9223372036854775807LL - 1)"
		}
	}
	return value
}

// cppFloat32Literal renders a validated float32 const value as a float literal
// (with an `f` suffix), so initializing a float from it is not a narrowing
// double to float conversion (MSVC C4305).
func cppFloat32Literal(value string) string {
	if !strings.ContainsAny(value, ".eE") {
		// `5f` is not a valid literal
		return value + ".0f"
	}
	return value + "f"
}

// constIsLiteralType reports whether a const of the given type can be
// constexpr. A typedef of std::string cannot.
func constIsLiteralType(typ *parse.DataTypeComparableDefinition, underlyingType *parse.DataTypeComparableDefinition) bool {
	return !(typ.Type == parse.DataTypeComparableCustom && underlyingType != nil && underlyingType.Type == parse.DataTypeComparableString)
}

func getConstValue(typ *parse.DataTypeComparableDefinition, underlyingType *parse.DataTypeComparableDefinition, value string) (string, error) {
	switch typ.Type {
	case parse.DataTypeComparableUInt8, parse.DataTypeComparableUInt16, parse.DataTypeComparableUInt32, parse.DataTypeComparableUInt64,
		parse.DataTypeComparableInt8, parse.DataTypeComparableInt16, parse.DataTypeComparableInt32, parse.DataTypeComparableInt64:
		if err := parse.ValidateConstValues(typ.Type, value); err != nil {
			return "", err
		}
		return cppIntLiteral(typ.Type, value), nil
	case parse.DataTypeComparableFloat32, parse.DataTypeComparableFloat64:
		if err := parse.ValidateConstValues(typ.Type, value); err != nil {
			return "", err
		}
		if typ.Type == parse.DataTypeComparableFloat32 {
			return cppFloat32Literal(value), nil
		}
		return value, nil
	case parse.DataTypeComparableString:
		str, err := parse.ConstStringValue(value)
		if err != nil {
			return "", err
		}
		return quoteCppString(str), nil
	case parse.DataTypeComparableUUID:
		id, err := parse.ConstUUIDValue(value)
		if err != nil {
			return "", err
		}
		hexBytes := make([]string, len(id))
		for i, b := range id {
			hexBytes[i] = fmt.Sprintf("0x%02x", b)
		}
		return fmt.Sprintf("scg::type::uuid({%s})", strings.Join(hexBytes, ", ")), nil
	case parse.DataTypeComparableCustom:
		if underlyingType == nil {
			return "", fmt.Errorf("underlying type is nil")
		}
		typeName, err := mapDataTypeComparableDefinitionToCppType(typ)
		if err != nil {
			return "", err
		}
		underlyingValue, err := getConstValue(underlyingType, nil, value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s(%s)", typeName, underlyingValue), nil
	}
	return "", nil
}

func generateConstCppCode(constDelc *parse.ConstDeclaration) (string, error) {

	typeName, err := mapDataTypeComparableDefinitionToCppType(constDelc.DataTypeDefinition)
	if err != nil {
		return "", err
	}
	// override std::string since it is not constexpr
	if typeName == "std::string" {
		typeName = "const char*"
	}

	value, err := getConstValue(constDelc.DataTypeDefinition, constDelc.UnderlyingDataTypeDefinition, constDelc.Value)
	if err != nil {
		return "", err
	}

	prefix := "constexpr"
	if !constIsLiteralType(constDelc.DataTypeDefinition, constDelc.UnderlyingDataTypeDefinition) {
		prefix = "inline const"
	}

	args := ConstArgs{
		Prefix:              prefix,
		ConstNameUpperCase:  strings.ToUpper(util.EnsureSnakeCase(constDelc.Name)),
		ConstUnderlyingType: typeName,
		ConstValue:          value,
	}

	buf := &bytes.Buffer{}
	err = constTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
