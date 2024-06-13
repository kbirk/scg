package parse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	messageRegex                 = regexp.MustCompile(`(?s)message\s+([a-zA-Z][a-zA-Z_0-9]*)\s*{(.*?)}`)
	fieldRegex                   = regexp.MustCompile(`^((?:list\s*\<\s*(?:.*)\s*\>)|(?:map\s*\<\s*(?:.*)\s*\>)|(?:.+?))\s+(.+?)\s*=\s*(.+?)\s*;*$`)
	fieldNameRegex               = regexp.MustCompile(`^[a-zA-Z][a-zA-Z_0-9]*$`)
	plainDataTypeRegex           = regexp.MustCompile(`^(byte|bool|uint8|uint16|uint32|uint64|int8|int16|int32|int64|float32|float64|string|timestamp|uuid)$`)
	plainDataTypeComparableRegex = regexp.MustCompile(`^(uint8|uint16|uint32|uint64|int8|int16|int32|int64|float32|float64|string|uuid)$`)
	customDataTypeRegex          = regexp.MustCompile(`^((?:[a-zA-Z][a-zA-Z_0-9]*)(?:\.[a-zA-Z][a-zA-Z_0-9]*)*)$`)
	mapDataTypeRegex             = regexp.MustCompile(`^map\s*\<\s*((?:[a-zA-Z][a-zA-Z_0-9]*)(?:\.[a-zA-Z][a-zA-Z_0-9]*)*)\s*,\s*(.+?)\s*\>$`)
	listDataTypeRegex            = regexp.MustCompile(`^list\s*\<\s*(.+)\s*\>$`)
	validIndexRegex              = regexp.MustCompile(`^\d+$`)
)

type DataType int
type DataTypeComparable int

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
	DataTypeTimestamp
	DataTypeUUID
	DataTypeMap
	DataTypeList
	DataTypeCustom
)

const (
	DataTypeComparableUInt8 DataTypeComparable = iota
	DataTypeComparableUInt16
	DataTypeComparableUInt32
	DataTypeComparableUInt64
	DataTypeComparableInt8
	DataTypeComparableInt16
	DataTypeComparableInt32
	DataTypeComparableInt64
	DataTypeComparableFloat32
	DataTypeComparableFloat64
	DataTypeComparableString
	DataTypeComparableUUID
	DataTypeComparableCustom
)

type DataTypeComparableDefinition struct {
	Type                     DataTypeComparable
	CustomType               string
	CustomTypePackage        string
	UnderlyingType           DataTypeComparable
	ImportedFromOtherPackage bool
	Token                    *Token
}

func (d *DataTypeComparableDefinition) ToString() string {
	if d.Type == DataTypeComparableCustom {
		return fmt.Sprintf("%s.%s", d.CustomTypePackage, d.CustomType)
	}

	return mapComparableTypeEnumToString(d.Type)
}

type DataTypeDefinition struct {
	Type                     DataType
	Key                      *DataTypeComparableDefinition
	CustomType               string
	CustomTypePackage        string
	SubType                  *DataTypeDefinition
	ImportedFromOtherPackage bool
	Token                    *Token
}

func (dt *DataTypeDefinition) GetElementType() *DataTypeDefinition {
	if dt.Type == DataTypeList || dt.Type == DataTypeMap {
		if dt.SubType == nil {
			panic("list / map subtype not found")
		}
		return dt.SubType.GetElementType()
	}
	return dt
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
	case DataTypeTimestamp:
		return "timestamp"
	case DataTypeUUID:
		return "uuid"
	}
	panic("invalid data type")
}

func mapComparableTypeEnumToString(typ DataTypeComparable) string {
	switch typ {
	case DataTypeComparableUInt8:
		return "uint8"
	case DataTypeComparableUInt16:
		return "uint16"
	case DataTypeComparableUInt32:
		return "uint32"
	case DataTypeComparableUInt64:
		return "uint64"
	case DataTypeComparableInt8:
		return "int8"
	case DataTypeComparableInt16:
		return "int16"
	case DataTypeComparableInt32:
		return "int32"
	case DataTypeComparableInt64:
		return "int64"
	case DataTypeComparableFloat32:
		return "float32"
	case DataTypeComparableFloat64:
		return "float64"
	case DataTypeComparableString:
		return "string"
	case DataTypeComparableUUID:
		return "uuid"
	}
	panic("invalid data type")
}

func (d *DataTypeDefinition) ToString() string {
	if d.Type == DataTypeCustom {
		return fmt.Sprintf("%s.%s", d.CustomTypePackage, d.CustomType)
	}

	if d.Type == DataTypeMap {
		return fmt.Sprintf("map<%s, %s>", d.Key.ToString(), d.SubType.ToString())
	}

	if d.Type == DataTypeList {
		return fmt.Sprintf("list<%s>", d.SubType.ToString())
	}

	return mapTypeEnumToString(d.Type)
}

type MessageFieldDefinition struct {
	Name               string
	Index              uint32
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
	case "timestamp":
		return DataTypeTimestamp, nil
	case "uuid":
		return DataTypeUUID, nil
	}
	return 0, fmt.Errorf("invalid data type %s", typ)
}

func mapPlainDataTypeComparableStringToEnum(typ string) (DataTypeComparable, error) {
	switch typ {
	case "uint8":
		return DataTypeComparableUInt8, nil
	case "uint16":
		return DataTypeComparableUInt16, nil
	case "uint32":
		return DataTypeComparableUInt32, nil
	case "uint64":
		return DataTypeComparableUInt64, nil
	case "int8":
		return DataTypeComparableInt8, nil
	case "int16":
		return DataTypeComparableInt16, nil
	case "int32":
		return DataTypeComparableInt32, nil
	case "int64":
		return DataTypeComparableInt64, nil
	case "float32":
		return DataTypeComparableFloat32, nil
	case "float64":
		return DataTypeComparableFloat64, nil
	case "string":
		return DataTypeComparableString, nil
	case "uuid":
		return DataTypeComparableUUID, nil
	}

	return 0, fmt.Errorf("invalid comparable data type %s", typ)
}

func parseDataTypeComparableDefinition(input *Token) (*DataTypeComparableDefinition, *ParsingError) {

	dt := &DataTypeComparableDefinition{}
	dt.Token = input

	// check for plain data type
	match, perr := FindOneOrNoMatch(plainDataTypeComparableRegex, input)
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

		dataType, err := mapPlainDataTypeComparableStringToEnum(match.Captures[0].Content)
		if err != nil {
			return nil, &ParsingError{
				Message: err.Error(),
				Token:   match.Captures[0],
			}
		}

		dt.Type = dataType
		return dt, nil
	}

	// check for custom data type
	match, perr = FindOneOrNoMatch(customDataTypeRegex, input)
	if perr != nil {
		return nil, &ParsingError{
			Message: fmt.Sprintf("invalid field definition: `%s", input.Content),
			Token:   input,
		}
	}

	// if one found, its a typedef type, parse it
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

		dt.Type = DataTypeComparableCustom
		dt.CustomType = typeName
		dt.CustomTypePackage = packageName
		return dt, nil
	}

	return nil, &ParsingError{
		Message: fmt.Sprintf("invalid field definition: `%s", input.Content),
		Token:   input,
	}
}

func parseDataTypeDefinition(input *Token) (*DataTypeDefinition, *ParsingError) {

	dt := &DataTypeDefinition{}
	dt.Token = input

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

		dt.Type = dataType
		return dt, nil
	}

	// check for custom data type
	match, perr = FindOneOrNoMatch(customDataTypeRegex, input)
	if perr != nil {
		return nil, &ParsingError{
			Message: fmt.Sprintf("invalid field definition: `%s", input.Content),
			Token:   input,
		}
	}

	// if one found, its a custom data type, parse it
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

		dt.Type = DataTypeCustom
		dt.CustomType = typeName
		dt.CustomTypePackage = packageName
		return dt, nil
	}

	// container type, parse the outer type

	match, perr = FindOneOrNoMatch(mapDataTypeRegex, input)
	if perr != nil {
		return nil, &ParsingError{
			Message: fmt.Sprintf("invalid field definition: `%s", input.Content),
			Token:   input,
		}
	}

	if match != nil {
		if len(match.Captures) != 2 {
			return nil, &ParsingError{
				Message: "invalid field definition, invalid number of matches found",
				Token:   input,
			}
		}

		// parse the key type
		key, perr := parseDataTypeComparableDefinition(match.Captures[0])
		if perr != nil {
			return nil, perr
		}

		// recurse to parse nested type
		nestedDataType, perr := parseDataTypeDefinition(match.Captures[1])
		if perr != nil {
			return nil, perr
		}

		dt.Type = DataTypeMap
		dt.Key = key
		dt.SubType = nestedDataType

		return dt, nil
	}

	match, perr = FindOneMatch(listDataTypeRegex, input)
	if perr != nil {
		return nil, &ParsingError{
			Message: fmt.Sprintf("invalid field definition: `%s", input.Content),
			Token:   input,
		}
	}

	if len(match.Captures) != 1 {
		return nil, &ParsingError{
			Message: "invalid field definition, invalid number of matches found",
			Token:   input,
		}
	}

	// recurse to parse nested type
	nestedDataType, perr := parseDataTypeDefinition(match.Captures[0])
	if perr != nil {
		return nil, perr
	}

	dt.Type = DataTypeList
	dt.SubType = nestedDataType
	return dt, nil
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
	if !fieldNameRegex.MatchString(name) {
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
		Index:              uint32(index),
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
		indices := make(map[uint32]bool)
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
			_, ok := indices[uint32(i)]
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
