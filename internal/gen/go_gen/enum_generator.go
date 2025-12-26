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
	ValueString         string
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

func ({{.EnumNameNameFirstLetter}} *{{.EnumNamePascalCase}}) BitSize() int {
	return serialize.BitSize{{.EnumUnderlyingTypePascalCase}}(*(*{{.EnumUnderlyingType}})({{.EnumNameNameFirstLetter}}))
}

func ({{.EnumNameNameFirstLetter}} *{{.EnumNamePascalCase}}) Serialize(writer *serialize.Writer) {
	serialize.Serialize{{.EnumUnderlyingTypePascalCase}}(writer, *(*{{.EnumUnderlyingType}})({{.EnumNameNameFirstLetter}}))
}

func ({{.EnumNameNameFirstLetter}} *{{.EnumNamePascalCase}}) Deserialize(reader *serialize.Reader) error {
	return serialize.Deserialize{{.EnumUnderlyingTypePascalCase}}((*{{.EnumUnderlyingType}})({{.EnumNameNameFirstLetter}}), reader)
}

func ({{.EnumNameNameFirstLetter}} {{.EnumNamePascalCase}}) Value() (driver.Value, error) {
	return {{.EnumNamePascalCase}}_ToString[{{.EnumNameNameFirstLetter}}], nil
}

func ({{.EnumNameNameFirstLetter}} *{{.EnumNamePascalCase}}) Scan(src interface{}) error {
	switch src := src.(type) {
	case string:
		*{{.EnumNameNameFirstLetter}}  = {{.EnumNamePascalCase}}String_ToEnum[src]
		return nil
	case []byte:
		*{{.EnumNameNameFirstLetter}}  = {{.EnumNamePascalCase}}String_ToEnum[string(src)]
		return nil
	case nil:
		var def {{.EnumNamePascalCase}}
		*{{.EnumNameNameFirstLetter}} = def
		return nil
	default:
		return fmt.Errorf("cannot scan type %T into type {{.EnumNamePascalCase}}", src)
	}
}

const (
{{- range .EnumValueArgs}}
	{{$.EnumNamePascalCase}}_{{.ValueNamePascalCase}} {{$.EnumNamePascalCase}} = {{.Index}}
{{- end}}
{{- range .EnumValueArgs}}
	{{$.EnumNamePascalCase}}_{{.ValueNamePascalCase}}_String = "{{.ValueString}}"
{{- end}}
)

var (
	{{.EnumNamePascalCase}}_ToString = map[{{.EnumNamePascalCase}}]string{ {{- range .EnumValueArgs}}
		{{$.EnumNamePascalCase}}_{{.ValueNamePascalCase}}: {{$.EnumNamePascalCase}}_{{.ValueNamePascalCase}}_String,{{end}}
	}
	{{.EnumNamePascalCase}}String_ToEnum = map[string]{{.EnumNamePascalCase}}{ {{- range .EnumValueArgs}}
		{{$.EnumNamePascalCase}}_{{.ValueNamePascalCase}}_String:{{$.EnumNamePascalCase}}_{{.ValueNamePascalCase}},{{end}}
	}
)
`

var (
	enumTemplate = template.Must(template.New("enumTemplateGo").Parse(enumTemplateStr))
)

func generateEnumGoCode(enum *parse.EnumDefinition) (string, error) {
	if len(enum.Values) >= 65536 {
		return "", fmt.Errorf("enum %s has too many values", enum.Name)
	}

	dataType := parse.DataTypeUInt16

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
			ValueString:         v.Value,
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
