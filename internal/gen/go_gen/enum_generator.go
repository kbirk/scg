package go_gen

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

type EnumValueArgs struct {
	ValueNamePascalCase string
	Index               int
}

type EnumArgs struct {
	EnumNamePascalCase           string
	EnumNameNameFirstLetter      string
	EnumUnderlyingType           string
	EnumUnderlyingTypePascalCase string
	EnumValueArgs                []EnumValueArgs
}

const enumTemplateStr = `
type {{.EnumNamePascalCase}} {{.EnumUnderlyingType}}

func ({{.EnumNameNameFirstLetter}} *{{.EnumNamePascalCase}}) ByteSize() int {
	return serialize.ByteSize{{.EnumUnderlyingTypePascalCase}}(*(*{{.EnumUnderlyingType}})({{.EnumNameNameFirstLetter}}))
}

func ({{.EnumNameNameFirstLetter}} *{{.EnumNamePascalCase}}) Serialize(writer *serialize.FixedSizeWriter) {
	serialize.Serialize{{.EnumUnderlyingTypePascalCase}}(writer, *(*{{.EnumUnderlyingType}})({{.EnumNameNameFirstLetter}}))
}

func ({{.EnumNameNameFirstLetter}} *{{.EnumNamePascalCase}}) Deserialize(reader *serialize.Reader) error {
	return serialize.Deserialize{{.EnumUnderlyingTypePascalCase}}((*{{.EnumUnderlyingType}})({{.EnumNameNameFirstLetter}}), reader)
}

const ({{- range .EnumValueArgs}}
	{{$.EnumNamePascalCase}}_{{.ValueNamePascalCase}} {{$.EnumNamePascalCase}} = {{.Index}}{{end}}
)
`

var (
	enumTemplate = template.Must(template.New("enumTemplateGo").Parse(enumTemplateStr))
)

func generateEnumGoCode(enum *parse.EnumDefinition) (string, error) {

	dataType := parse.DataTypeUInt8
	if len(enum.Values) >= 256 {
		if len(enum.Values) >= 65536 {
			return "", fmt.Errorf("enum %s has too many values", enum.Name)
		} else {
			dataType = parse.DataTypeUInt16
		}
	}

	typeName, err := mapDataTypeToGoType(dataType)
	if err != nil {
		return "", err
	}

	typeNamePascalCase, err := getDataTypeMethodSuffix(dataType)
	if err != nil {
		return "", err
	}

	var enumValueArgs []EnumValueArgs
	for i, v := range enum.ValuesByIndex() {
		enumValueArgs = append(enumValueArgs, EnumValueArgs{
			ValueNamePascalCase: util.EnsurePascalCase(v.Name),
			Index:               i,
		})
	}

	args := EnumArgs{
		EnumNamePascalCase:           util.EnsurePascalCase(enum.Name),
		EnumNameNameFirstLetter:      util.FirstLetterAsLowercase(enum.Name),
		EnumUnderlyingType:           typeName,
		EnumUnderlyingTypePascalCase: typeNamePascalCase,
		EnumValueArgs:                enumValueArgs,
	}

	buf := &bytes.Buffer{}
	err = enumTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
