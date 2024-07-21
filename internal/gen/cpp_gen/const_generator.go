package cpp_gen

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/google/uuid"
	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

type ConstArgs struct {
	ConstNameUpperCase  string
	ConstUnderlyingType string
	ConstValue          string
}

const constTemplateStr = `
constexpr {{.ConstUnderlyingType}} {{.ConstNameUpperCase}} = {{.ConstValue}};`

var (
	constTemplate = template.Must(template.New("constTemplateGo").Parse(constTemplateStr))
)

func getConstValue(typ *parse.DataTypeComparableDefinition, underlyingType *parse.DataTypeComparableDefinition, value string) (string, error) {
	switch typ.Type {
	case parse.DataTypeComparableUInt8, parse.DataTypeComparableUInt16, parse.DataTypeComparableUInt32, parse.DataTypeComparableUInt64:
		return value, nil
	case parse.DataTypeComparableInt8, parse.DataTypeComparableInt16, parse.DataTypeComparableInt32, parse.DataTypeComparableInt64:
		return value, nil
	case parse.DataTypeComparableFloat32, parse.DataTypeComparableFloat64:
		return value, nil
	case parse.DataTypeComparableString:
		return value, nil
	case parse.DataTypeComparableUUID:
		bytes, err := uuid.Parse(value)
		if err != nil {
			return "", err
		}
		hexBytes := make([]string, len(bytes))
		for i, b := range bytes {
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
		// override std::string since it is not constexpr
		if typeName == "std::string" {
			typeName = "char*"
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
		typeName = "char*"
	}

	value, err := getConstValue(constDelc.DataTypeDefinition, constDelc.UnderlyingDataTypeDefinition, constDelc.Value)
	if err != nil {
		return "", err
	}

	args := ConstArgs{
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
