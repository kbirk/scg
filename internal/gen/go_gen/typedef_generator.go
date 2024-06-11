package go_gen

import (
	"bytes"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

type TypedefArgs struct {
	TypedefNamePascalCase           string
	TypedefNameNameFirstLetter      string
	TypedefUnderlyingType           string
	TypedefUnderlyingTypePascalCase string
}

const typedefTemplateStr = `
type {{.TypedefNamePascalCase}} {{.TypedefUnderlyingType}}

func ({{.TypedefNameNameFirstLetter}} *{{.TypedefNamePascalCase}}) CalcByteSize() int {
	return serialize.CalcByteSize{{.TypedefUnderlyingTypePascalCase}}(*(*{{.TypedefUnderlyingType}})({{.TypedefNameNameFirstLetter}}))
}

func ({{.TypedefNameNameFirstLetter}} *{{.TypedefNamePascalCase}}) Serialize(writer *serialize.FixedSizeWriter) {
	serialize.Serialize{{.TypedefUnderlyingTypePascalCase}}(writer, *(*{{.TypedefUnderlyingType}})({{.TypedefNameNameFirstLetter}}))
}

func ({{.TypedefNameNameFirstLetter}} *{{.TypedefNamePascalCase}}) Deserialize(reader *serialize.Reader) error {
	return serialize.Deserialize{{.TypedefUnderlyingTypePascalCase}}((*{{.TypedefUnderlyingType}})({{.TypedefNameNameFirstLetter}}), reader)
}
`

var (
	typedefTemplate = template.Must(template.New("typedefTemplateGo").Parse(typedefTemplateStr))
)

func generateTypedefGoCode(typdef *parse.TypedefDeclaration) (string, error) {

	typeName, err := mapComparableTypeToGoType(typdef.DataTypeDefinition)
	if err != nil {
		return "", err
	}

	typeNamePascaleCase, err := getComparableDataTypeName(typdef.DataTypeDefinition)
	if err != nil {
		return "", err
	}

	args := TypedefArgs{
		TypedefNamePascalCase:           util.EnsurePascalCase(typdef.Name),
		TypedefNameNameFirstLetter:      util.FirstLetterAsLowercase(typdef.Name),
		TypedefUnderlyingType:           typeName,
		TypedefUnderlyingTypePascalCase: typeNamePascaleCase,
	}

	buf := &bytes.Buffer{}
	err = typedefTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
