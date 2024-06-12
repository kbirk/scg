package cpp_gen

import (
	"bytes"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

type TypedefArgs struct {
	TypedefNamePascalCase string
	TypedefUnderlyingType string
}

const typedefTemplateStr = `
SCG_TYPEDEF({{.TypedefNamePascalCase}}, {{.TypedefUnderlyingType}});`

var (
	typedefTemplate = template.Must(template.New("typedefTemplateCpp").Parse(typedefTemplateStr))
)

func generateTypedefCppCode(typdef *parse.TypedefDeclaration) (string, error) {

	typeName, err := mapDataTypeComparableDefinitionToCppType(typdef.DataTypeDefinition)
	if err != nil {
		return "", err
	}

	args := TypedefArgs{
		TypedefNamePascalCase: util.EnsurePascalCase(typdef.Name),
		TypedefUnderlyingType: typeName,
	}

	buf := &bytes.Buffer{}
	err = typedefTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
