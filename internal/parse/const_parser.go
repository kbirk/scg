package parse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

var (
	constRegex = regexp.MustCompile(`const\s+((?:[a-zA-Z][a-zA-Z_0-9]*)(?:\.[a-zA-Z][a-zA-Z_0-9]*)*)\s+([a-zA-Z][a-zA-Z_0-9]*)\s*=\s*(.*?)\s*;$`)
)

type ConstDeclaration struct {
	Name                         string
	Value                        string
	DataTypeDefinition           *DataTypeComparableDefinition
	UnderlyingDataTypeDefinition *DataTypeComparableDefinition
	File                         *File
	Token                        *Token
}

var (
	// Integers: plain decimal without leading zeros (a leading zero is
	// ambiguous: Go and C++ would read `010` as octal 8 while the schema
	// validator historically read it as decimal 10), or lowercase-prefixed hex.
	decimalIntRegex = regexp.MustCompile(`^[+-]?(0|[1-9][0-9]*)$`)
	hexIntRegex     = regexp.MustCompile(`^0x[0-9a-fA-F]+$`)
	// Floats: plain decimal / scientific notation only. NaN, Inf, hex floats and
	// digit separators are rejected because they are not portable literals.
	floatRegex = regexp.MustCompile(`^[+-]?([0-9]+(\.[0-9]*)?|\.[0-9]+)([eE][+-]?[0-9]+)?$`)
	// UUIDs must be quoted, canonical 8-4-4-4-12 hex.
	quotedUUIDRegex = regexp.MustCompile(`^"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}"$`)
)

func checkIntLiteral(str string) error {
	if hexIntRegex.MatchString(str) {
		return nil
	}
	if decimalIntRegex.MatchString(str) {
		return nil
	}
	trimmed := strings.TrimLeft(str, "+-")
	if len(trimmed) > 1 && trimmed[0] == '0' && trimmed[1] >= '0' && trimmed[1] <= '9' {
		return fmt.Errorf("leading zeros are not allowed (they would be read as octal by Go and C++): %s", str)
	}
	return fmt.Errorf("invalid integer literal: %s", str)
}

func parseUint(str string, bitSize int) error {
	if err := checkIntLiteral(str); err != nil {
		return err
	}
	if strings.HasPrefix(str, "0x") {
		_, err := strconv.ParseUint(str[2:], 16, bitSize)
		return err
	}
	_, err := strconv.ParseUint(strings.TrimPrefix(str, "+"), 10, bitSize)
	return err
}

func parseInt(str string, bitSize int) error {
	if err := checkIntLiteral(str); err != nil {
		return err
	}
	if strings.HasPrefix(str, "0x") {
		_, err := strconv.ParseInt(str[2:], 16, bitSize)
		return err
	}
	_, err := strconv.ParseInt(str, 10, bitSize)
	return err
}

func parseFloat(str string, bitSize int) error {
	if !floatRegex.MatchString(str) {
		return fmt.Errorf("invalid float literal (only decimal / scientific notation is supported, no NaN or Inf): %s", str)
	}
	if !strings.ContainsAny(str, ".eE") {
		// without a fraction or exponent this is an integer literal in Go and
		// C++, so a leading zero would make it octal there
		if err := checkIntLiteral(str); err != nil {
			return err
		}
	}
	_, err := strconv.ParseFloat(str, bitSize)
	return err
}

func parseUUID(str string) error {
	_, err := ConstUUIDValue(str)
	return err
}

// normalizeCppOnlyEscapes rewrites the escapes that C++ string literals
// accept but Go's do not, `\'` and `\?`, to the characters they denote, so
// the string can be decoded with strconv.Unquote. Escaped backslashes are
// copied through untouched, so `\\'` stays a backslash followed by a quote.
func normalizeCppOnlyEscapes(raw string) string {
	if !strings.Contains(raw, `\'`) && !strings.Contains(raw, `\?`) {
		return raw
	}
	var sb strings.Builder
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c == '\\' && i+1 < len(raw) {
			next := raw[i+1]
			switch next {
			case '\'', '?':
				sb.WriteByte(next)
			default:
				sb.WriteByte(c)
				sb.WriteByte(next)
			}
			i++
			continue
		}
		sb.WriteByte(c)
	}
	return sb.String()
}

// ConstStringValue decodes a quoted string const value (as written in the
// schema, including the surrounding quotes) into its raw bytes. Escapes follow
// Go string literal syntax, plus the C++ escapes `\'` and `\?`; generators
// re-encode the result per language.
func ConstStringValue(raw string) (string, error) {
	if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return "", fmt.Errorf("string const must be double-quoted: %s", raw)
	}
	str, err := strconv.Unquote(normalizeCppOnlyEscapes(raw))
	if err != nil {
		return "", fmt.Errorf("invalid quoted string (supported escapes: \\\\ \\\" \\' \\? \\n \\r \\t \\a \\b \\f \\v \\xHH \\uHHHH \\UHHHHHHHH \\ooo): %s", raw)
	}
	return str, nil
}

// ConstUUIDValue decodes a quoted canonical UUID const value.
func ConstUUIDValue(raw string) (uuid.UUID, error) {
	if !quotedUUIDRegex.MatchString(raw) {
		return uuid.UUID{}, fmt.Errorf("uuid const must be a quoted canonical UUID, e.g. \"123e4567-e89b-12d3-a456-426614174000\": %s", raw)
	}
	return uuid.Parse(raw[1 : len(raw)-1])
}

func ValidateConstValues(dataType DataTypeComparable, valueStr string) error {
	switch dataType {
	case DataTypeComparableUInt8:
		return parseUint(valueStr, 8)
	case DataTypeComparableUInt16:
		return parseUint(valueStr, 16)
	case DataTypeComparableUInt32:
		return parseUint(valueStr, 32)
	case DataTypeComparableUInt64:
		return parseUint(valueStr, 64)
	case DataTypeComparableInt8:
		return parseInt(valueStr, 8)
	case DataTypeComparableInt16:
		return parseInt(valueStr, 16)
	case DataTypeComparableInt32:
		return parseInt(valueStr, 32)
	case DataTypeComparableInt64:
		return parseInt(valueStr, 64)
	case DataTypeComparableFloat32:
		return parseFloat(valueStr, 32)
	case DataTypeComparableFloat64:
		return parseFloat(valueStr, 64)
	case DataTypeComparableString:
		_, err := ConstStringValue(valueStr)
		return err
	case DataTypeComparableUUID:
		return parseUUID(valueStr)
	case DataTypeComparableCustom:
		// we cannot validate it here, validate it when we resolve
		return nil
	default:
		return fmt.Errorf("invalid data type %v", dataType)
	}
}

func parseConstDeclarations(tokens []*Token) (map[string]*ConstDeclaration, *ParsingError) {

	consts := map[string]*ConstDeclaration{}

	for _, token := range tokens {
		if token.Type != ConstTokenType {
			continue
		}

		match, perr := FindOneMatch(constRegex, token)
		if perr != nil || len(match.Captures) != 3 {
			return nil, &ParsingError{
				Message: "invalid const definition",
				Token:   token,
			}
		}

		typ, perr := parseDataTypeComparableDefinition(match.Captures[0])
		if perr != nil {
			return nil, perr
		}

		err := ValidateConstValues(typ.Type, match.Captures[2].Content)
		if err != nil {
			return nil, &ParsingError{
				Message: fmt.Sprintf("invalid value for const %s: %s", match.Captures[2].Content, err.Error()),
				Token:   match.Captures[2],
			}
		}

		constDecl := &ConstDeclaration{
			Name:               match.Captures[1].Content,
			Value:              match.Captures[2].Content,
			DataTypeDefinition: typ,
			Token:              token,
		}

		_, ok := consts[constDecl.Name]
		if ok {
			return nil, &ParsingError{
				Message: fmt.Sprintf("duplicate const definition %s", constDecl.Name),
				Token:   token,
			}
		}
		consts[constDecl.Name] = constDecl
	}

	return consts, nil
}
