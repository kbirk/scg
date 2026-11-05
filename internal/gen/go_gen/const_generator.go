package go_gen

import (
	"bytes"
	"fmt"
	"strconv"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

type ConstArgs struct {
	Prefix              string
	ConstNamePascalCase string
	ConstUnderlyingType string
	ConstValue          string
}

const constTemplateStr = `
{{.Prefix}} {{.ConstNamePascalCase}} {{.ConstUnderlyingType}} = {{.ConstValue}};`

var (
	constTemplate = template.Must(template.New("constTemplateGo").Parse(constTemplateStr))
)

func getConstPrefix(typ *parse.DataTypeComparableDefinition, underlyingType *parse.DataTypeComparableDefinition) (string, error) {
	switch typ.Type {
	case parse.DataTypeComparableUInt8, parse.DataTypeComparableUInt16, parse.DataTypeComparableUInt32, parse.DataTypeComparableUInt64,
		parse.DataTypeComparableInt8, parse.DataTypeComparableInt16, parse.DataTypeComparableInt32, parse.DataTypeComparableInt64,
		parse.DataTypeComparableFloat32, parse.DataTypeComparableFloat64,
		parse.DataTypeComparableString:
		return "const", nil
	case parse.DataTypeComparableUUID:
		return "var", nil
	case parse.DataTypeComparableCustom:
		if underlyingType == nil {
			return "", fmt.Errorf("underlying type is nil")
		}
		return getConstPrefix(underlyingType, nil)
	}
	return "", fmt.Errorf("invalid data type %v", typ.Type)
}

func getConstValue(typ *parse.DataTypeComparableDefinition, underlyingType *parse.DataTypeComparableDefinition, value string) (string, error) {
	switch typ.Type {
	case parse.DataTypeComparableUInt8, parse.DataTypeComparableUInt16, parse.DataTypeComparableUInt32, parse.DataTypeComparableUInt64,
		parse.DataTypeComparableInt8, parse.DataTypeComparableInt16, parse.DataTypeComparableInt32, parse.DataTypeComparableInt64,
		parse.DataTypeComparableFloat32, parse.DataTypeComparableFloat64:
		// validated literals (decimal without leading zeros, 0x hex, plain
		// floats) mean the same thing in Go as in the schema
		if err := parse.ValidateConstValues(typ.Type, value); err != nil {
			return "", err
		}
		return value, nil
	case parse.DataTypeComparableString:
		str, err := parse.ConstStringValue(value)
		if err != nil {
			return "", err
		}
		return strconv.Quote(str), nil
	case parse.DataTypeComparableUUID:
		id, err := parse.ConstUUIDValue(value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("uuid.MustParse(%q)", id.String()), nil
	case parse.DataTypeComparableCustom:
		if underlyingType == nil {
			return "", fmt.Errorf("underlying type is nil")
		}
		typeName, err := mapDataTypeComparableDefinitionToGoType(typ)
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

func generateConstGoCode(constDelc *parse.ConstDeclaration) (string, error) {

	typeName, err := mapDataTypeComparableDefinitionToGoType(constDelc.DataTypeDefinition)
	if err != nil {
		return "", err
	}

	prefix, err := getConstPrefix(constDelc.DataTypeDefinition, constDelc.UnderlyingDataTypeDefinition)
	if err != nil {
		return "", err
	}

	value, err := getConstValue(constDelc.DataTypeDefinition, constDelc.UnderlyingDataTypeDefinition, constDelc.Value)
	if err != nil {
		return "", err
	}

	args := ConstArgs{
		ConstNamePascalCase: util.EnsurePascalCase(constDelc.Name),
		ConstUnderlyingType: typeName,
		ConstValue:          value,
		Prefix:              prefix,
	}

	buf := &bytes.Buffer{}
	err = constTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
