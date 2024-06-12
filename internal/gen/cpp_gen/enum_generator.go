package cpp_gen

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
	EnumNamePascalCase string
	EnumValueArgs      []EnumValueArgs
}

const enumTemplateStr = `
enum class {{.EnumNamePascalCase}} { {{- range .EnumValueArgs}}
	{{.ValueNamePascalCase}} = {{.Index}},{{end}}
};
`

var (
	enumTemplate = template.Must(template.New("enumTemplateCpp").Parse(enumTemplateStr))
)

func generateEnumCppCode(enum *parse.EnumDefinition) (string, error) {

	if len(enum.Values) >= 65536 {
		return "", fmt.Errorf("enum %s has too many values", enum.Name)
	}

	var enumValueArgs []EnumValueArgs
	for i, v := range enum.ValuesByIndex() {
		enumValueArgs = append(enumValueArgs, EnumValueArgs{
			ValueNamePascalCase: util.EnsurePascalCase(v.Name),
			Index:               i,
		})
	}

	args := EnumArgs{
		EnumNamePascalCase: util.EnsurePascalCase(enum.Name),
		EnumValueArgs:      enumValueArgs,
	}

	buf := &bytes.Buffer{}
	err := enumTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
