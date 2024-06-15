package parse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

var (
	constRegex               = regexp.MustCompile(`const\s+((?:[a-zA-Z][a-zA-Z_0-9]*)(?:\.[a-zA-Z][a-zA-Z_0-9]*)*)\s+([a-zA-Z][a-zA-Z_0-9]*)\s*=\s*(.*?)\s*;$`)
	validateStringQuoteRegex = regexp.MustCompile(`^"[^"\\]*(\\.[^"\\]*)*"$`)
)

type ConstDeclaration struct {
	Name                         string
	Value                        string
	DataTypeDefinition           *DataTypeComparableDefinition
	UnderlyingDataTypeDefinition *DataTypeComparableDefinition
	File                         *File
	Token                        *Token
}

func parseUint(str string, bitSize int) error {
	if strings.HasPrefix(str, "0x") {
		_, err := strconv.ParseUint(str[2:], 16, bitSize)
		return err
	}
	_, err := strconv.ParseUint(str, 10, bitSize)
	return err
}

func parseInt(str string, bitSize int) error {
	if strings.HasPrefix(str, "0x") {
		_, err := strconv.ParseInt(str[2:], 16, bitSize)
		return err
	}
	_, err := strconv.ParseInt(str, 10, bitSize)
	return err
}

func parseFloat(str string, bitSize int) error {
	_, err := strconv.ParseFloat(str, bitSize)
	return err
}

func parseUUID(str string) error {
	_, err := uuid.Parse(str)
	return err
}

func parseTimestamp(str string) error {
	_, err := strconv.ParseInt(str, 10, 64)
	return err
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
		if !validateStringQuoteRegex.MatchString(valueStr) {
			return fmt.Errorf("invalid quoted string: %s", valueStr)
		}
		return nil
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
