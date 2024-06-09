package parse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	messageRegex           = regexp.MustCompile(`(?s)message\s+([a-zA-Z][a-zA-Z_0-9]*)\s*{(.*?)}`)
	fieldRegex             = regexp.MustCompile(`^(.+?)\s+(.+?)\s*=\s*(.+?);\s*$`)
	plainDataTypeRegex     = regexp.MustCompile(`^(byte|bool|uint8|uint16|uint32|uint64|int8|int16|int32|int64|float32|float64|string)\s*$`)
	customDataTypeRegex    = regexp.MustCompile(`^((?:[a-zA-Z][a-zA-Z_0-9]*)(?:\.[a-zA-Z][a-zA-Z_0-9]*)*)$`)
	containerDataTypeRegex = regexp.MustCompile(`^(\[\s*\]|map\s*\[\s*(byte|uint8|uint16|uint32|uint64|int8|int16|int32|int64|float32|float64|string)\s*\])\s*(.+)$`)
	listRegex              = regexp.MustCompile(`^\[\s*\]`)
	validIndexRegex        = regexp.MustCompile(`^\d+$`)
)

type DataType int
type DataComparableType int

const (
	DataTypeByte DataType = iota
	DataTypeBool
	DataTypeUInt8
	DataTypeUInt16
	DataTypeUInt32
	DataTypeUInt64
	DataTypeInt8
	DataTypeInt16
	DataTypeInt32
	DataTypeInt64
	DataTypeFloat32
	DataTypeFloat64
	DataTypeString
	DataTypeMap
	DataTypeList
	DataTypeCustom
)

const (
	DataComparableTypeUInt8 DataComparableType = iota
	DataComparableTypeUInt16
	DataComparableTypeUInt32
	DataComparableTypeUInt64
	DataComparableTypeInt8
	DataComparableTypeInt16
	DataComparableTypeInt32
	DataComparableTypeInt64
	DataComparableTypeFloat32
	DataComparableTypeFloat64
	DataComparableTypeString
)

type DataTypeDefinition struct {
	Type                     DataType
	Key                      DataComparableType
	CustomType               string
	CustomTypePackage        string
	SubType                  *DataTypeDefinition
	ImportedFromOtherPackage bool
}

func mapTypeEnumToString(typ DataType) string {
	switch typ {
	case DataTypeByte:
		return "byte"
	case DataTypeBool:
		return "bool"
	case DataTypeUInt8:
		return "uint8"
	case DataTypeUInt16:
		return "uint16"
	case DataTypeUInt32:
		return "uint32"
	case DataTypeUInt64:
		return "uint64"
	case DataTypeInt8:
		return "int8"
	case DataTypeInt16:
		return "int16"
	case DataTypeInt32:
		return "int32"
	case DataTypeInt64:
		return "int64"
	case DataTypeFloat32:
		return "float32"
	case DataTypeFloat64:
		return "float64"
	case DataTypeString:
		return "string"
	}
	panic("invalid data type")
}

func mapComparableTypeEnumToString(typ DataComparableType) string {
	switch typ {
	case DataComparableTypeUInt8:
		return "uint8"
	case DataComparableTypeUInt16:
		return "uint16"
	case DataComparableTypeUInt32:
		return "uint32"
	case DataComparableTypeUInt64:
		return "uint64"
	case DataComparableTypeInt8:
		return "int8"
	case DataComparableTypeInt16:
		return "int16"
	case DataComparableTypeInt32:
		return "int32"
	case DataComparableTypeInt64:
		return "int64"
	case DataComparableTypeFloat32:
		return "float32"
	case DataComparableTypeFloat64:
		return "float64"
	case DataComparableTypeString:
		return "string"
	}
	panic("invalid data type")
}

func (d *DataTypeDefinition) ToString() string {
	if d.Type == DataTypeCustom {
		return fmt.Sprintf("%s.%s", d.CustomTypePackage, d.CustomType)
	}

	if d.Type == DataTypeMap {
		return fmt.Sprintf("map[%s]%s", mapComparableTypeEnumToString(d.Key), d.SubType.ToString())
	}

	if d.Type == DataTypeList {
		return fmt.Sprintf("[]%s", d.SubType.ToString())
	}

	return mapTypeEnumToString(d.Type)
}

type MessageFieldDefinition struct {
	Name               string
	Index              uint
	DataTypeDefinition *DataTypeDefinition
	Token              *Token
}

type MessageDefinition struct {
	Name   string
	Fields map[string]*MessageFieldDefinition
	File   *File
	Token  *Token
}

func (m *MessageDefinition) FieldsByIndex() []*MessageFieldDefinition {
	res := make([]*MessageFieldDefinition, len(m.Fields))
	for _, field := range m.Fields {
		res[field.Index] = field
	}
	return res
}

func mapPlainDataTypeStringToEnum(typ string) (DataType, error) {
	switch typ {
	case "byte":
		return DataTypeByte, nil
	case "bool":
		return DataTypeBool, nil
	case "uint8":
		return DataTypeUInt8, nil
	case "uint16":
		return DataTypeUInt16, nil
	case "uint32":
		return DataTypeUInt32, nil
	case "uint64":
		return DataTypeUInt64, nil
	case "int8":
		return DataTypeInt8, nil
	case "int16":
		return DataTypeInt16, nil
	case "int32":
		return DataTypeInt32, nil
	case "int64":
		return DataTypeInt64, nil
	case "float32":
		return DataTypeFloat32, nil
	case "float64":
		return DataTypeFloat64, nil
	case "string":
		return DataTypeString, nil
	}
	return 0, fmt.Errorf("invalid data type %s", typ)
}

func mapComparableDataTypeStringToEnum(typ string) (DataComparableType, error) {
	switch typ {
	case "uint8":
		return DataComparableTypeUInt8, nil
	case "uint16":
		return DataComparableTypeUInt16, nil
	case "uint32":
		return DataComparableTypeUInt32, nil
	case "uint64":
		return DataComparableTypeUInt64, nil
	case "int8":
		return DataComparableTypeInt8, nil
	case "int16":
		return DataComparableTypeInt16, nil
	case "int32":
		return DataComparableTypeInt32, nil
	case "int64":
		return DataComparableTypeInt64, nil
	case "float32":
		return DataComparableTypeFloat32, nil
	case "float64":
		return DataComparableTypeFloat64, nil
	case "string":
		return DataComparableTypeString, nil
	}

	return 0, fmt.Errorf("invalid comparable data type %s", typ)
}

func parseDataTypeDefinition(input *Token) (*DataTypeDefinition, *ParsingError) {
	// check for plain data type
	match, perr := FindOneOrNoMatch(plainDataTypeRegex, input)
	if perr != nil {
		return nil, &ParsingError{
			Message: fmt.Sprintf("invalid field definition: `%s", input.Content),
			Token:   input,
		}
	}

	// if one found, its a plain data type, parse it
	if match != nil {
		if len(match.Captures) != 1 {
			return nil, &ParsingError{
				Message: "invalid field definition, invalid number of matches found",
				Token:   match.Match,
			}
		}

		dataType, err := mapPlainDataTypeStringToEnum(match.Captures[0].Content)
		if err != nil {
			return nil, &ParsingError{
				Message: err.Error(),
				Token:   match.Captures[0],
			}
		}

		return &DataTypeDefinition{
			Type: dataType,
		}, nil
	}

	// check for custom data type
	match, perr = FindOneOrNoMatch(customDataTypeRegex, input)
	if perr != nil {
		return nil, &ParsingError{
			Message: fmt.Sprintf("invalid field definition: `%s", input.Content),
			Token:   input,
		}
	}

	// if one found, its a plain data type, parse it
	if match != nil {
		if len(match.Captures) != 1 {
			return nil, &ParsingError{
				Message: "invalid field definition, invalid number of matches found",
				Token:   match.Match,
			}
		}
		parts := strings.Split(match.Captures[0].Content, ".")

		typeName := parts[len(parts)-1]
		packageName := ""
		if len(parts) > 1 {
			packageName = strings.Join(parts[:len(parts)-1], ".")
		}

		return &DataTypeDefinition{
			Type:              DataTypeCustom,
			CustomType:        typeName,
			CustomTypePackage: packageName,
		}, nil
	}

	// container type, parse the outer type

	match, perr = FindOneMatch(containerDataTypeRegex, input)
	if perr != nil {
		return nil, &ParsingError{
			Message: fmt.Sprintf("invalid field definition: `%s", input.Content),
			Token:   input,
		}
	}

	if len(match.Captures) != 3 {
		return nil, &ParsingError{
			Message: "invalid field definition, invalid number of matches found",
			Token:   input,
		}
	}

	dt := DataTypeDefinition{}
	typ := match.Captures[0].Content
	var nested *Token

	if listRegex.MatchString(typ) {
		// list

		dt.Type = DataTypeList
		nested = match.Captures[2]
	} else {
		// map

		key, err := mapComparableDataTypeStringToEnum(match.Captures[1].Content)
		if err != nil {
			return nil, &ParsingError{
				Message: err.Error(),
				Token:   match.Captures[1],
			}
		}

		dt.Type = DataTypeMap
		dt.Key = key
		nested = match.Captures[2]
	}

	// recurse to parse nested type
	nestedDataType, perr := parseDataTypeDefinition(nested)
	if perr != nil {
		return nil, perr
	}

	dt.SubType = nestedDataType
	return &dt, nil
}

func parseFieldDefinition(input *Token) (*MessageFieldDefinition, *ParsingError) {

	match, perr := FindOneMatch(fieldRegex, input)
	if perr != nil {
		return nil, &ParsingError{
			Message: fmt.Sprintf("invalid field definition: `%s", input.Content),
			Token:   input,
		}
	}

	typeMatch := match.Captures[0]

	dataType, perr := parseDataTypeDefinition(typeMatch)
	if perr != nil {
		return nil, &ParsingError{
			Message: fmt.Sprintf("invalid field type `%s`", typeMatch.Content),
			Token:   typeMatch,
		}
	}

	nameMatch := match.Captures[1]
	name := nameMatch.Content
	if !validNameRegex.MatchString(name) {
		return nil, &ParsingError{
			Message: fmt.Sprintf("invalid field name: %s", name),
			Token:   nameMatch,
		}
	}

	indexMatch := match.Captures[2]
	index, err := strconv.Atoi(indexMatch.Content)
	if err != nil {
		return nil, &ParsingError{
			Message: fmt.Sprintf("invalid field index `%s`", indexMatch.Content),
			Token:   indexMatch,
		}
	}

	return &MessageFieldDefinition{
		Name:               name,
		DataTypeDefinition: dataType,
		Index:              uint(index),
		Token:              input,
	}, nil
}

func tokenizeMessageFields(input *Token) ([]*Token, *ParsingError) {

	res := []*Token{}

	startLine := input.LineStart
	startChar := input.LineStartCharacterPosition

	line := input.LineStart
	character := input.LineStartCharacterPosition

	fieldContent := ""
	hasFoundNonWhitespace := false

	for _, c := range input.Content {
		if !unicode.IsSpace(c) && !hasFoundNonWhitespace {
			hasFoundNonWhitespace = true
			startLine = line
			startChar = character
		}

		if c == '\n' {
			line++
			character = 0
		} else {
			character++
		}

		if hasFoundNonWhitespace {
			fieldContent += string(c)
		}

		if c == ';' {
			res = append(res, &Token{
				Type:                       MessageFieldTokenType,
				Content:                    fieldContent,
				LineStart:                  startLine,
				LineEnd:                    line,
				LineStartCharacterPosition: startChar,
				LineEndCharacterPosition:   character,
			})
			fieldContent = ""
			startLine = line
			startChar = character
			hasFoundNonWhitespace = false
		}
	}

	if strings.TrimSpace(fieldContent) != "" {
		return nil, &ParsingError{
			Message: "unexpected end of field declaration",
			Token:   input,
		}
	}

	return res, nil
}

func parseMessageDefinitions(tokens []*Token) (map[string]*MessageDefinition, *ParsingError) {

	messages := map[string]*MessageDefinition{}

	for _, token := range tokens {
		if token.Type != MessageTokenType {
			continue
		}

		match, perr := FindOneMatch(messageRegex, token)
		if perr != nil || len(match.Captures) != 2 {
			return nil, &ParsingError{
				Message: "invalid message definition",
				Token:   token,
			}
		}

		message := &MessageDefinition{
			Name:   match.Captures[0].Content,
			Fields: map[string]*MessageFieldDefinition{},
			Token:  token,
		}

		fields, perr := tokenizeMessageFields(match.Captures[1])
		if perr != nil {
			return nil, perr
		}

		for _, field := range fields {
			fieldDefinition, perr := parseFieldDefinition(field)
			if perr != nil {
				return nil, perr
			}

			_, ok := message.Fields[fieldDefinition.Name]
			if ok {
				return nil, &ParsingError{
					Message: fmt.Sprintf("duplicate field definition %s", fieldDefinition.Name),
					Token:   field,
				}
			}
			message.Fields[fieldDefinition.Name] = fieldDefinition
		}

		// ensure message indices are valid and sequential
		indices := make(map[uint]bool)
		for _, field := range message.Fields {

			// track indices
			_, ok := indices[field.Index]
			if ok {
				return nil, &ParsingError{
					Message: fmt.Sprintf("duplicate index %d in definition %s", field.Index, message.Name),
					Token:   field.Token,
				}
			}
			indices[field.Index] = true
		}
		for i := 0; i < len(message.Fields); i++ {
			_, ok := indices[uint(i)]
			if !ok {
				return nil, &ParsingError{
					Message: fmt.Sprintf("missing index %d in message definition %s", i, message.Name),
					Token:   message.Token,
				}
			}
		}

		_, ok := messages[message.Name]
		if ok {
			return nil, &ParsingError{
				Message: fmt.Sprintf("duplicate message definition %s", message.Name),
				Token:   token,
			}
		}
		messages[message.Name] = message
	}

	return messages, nil
}
