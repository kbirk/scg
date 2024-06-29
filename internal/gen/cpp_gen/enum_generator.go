package cpp_gen

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

type EnumValueArgs struct {
	ValueNameUpperCase string
	ValueString        string
	Index              int
}

type EnumArgs struct {
	EnumNamePascalCase string
	EnumNameUpperCase  string
	EnumValueArgs      []EnumValueArgs
}

const enumTemplateStr = `
enum class {{.EnumNamePascalCase}} {
{{- range .EnumValueArgs}}
	{{.ValueNameUpperCase}} = {{.Index}},{{end}}
};

{{- range .EnumValueArgs}}
constexpr const char* {{.ValueNameUpperCase}}_STRING = "{{.ValueString}}";{{end}}

static const std::unordered_map<std::string, {{.EnumNamePascalCase}}> {{.EnumNameUpperCase}}_STRING_TO_ENUM = {
{{range .EnumValueArgs}}	{ {{.ValueNameUpperCase}}_STRING, {{$.EnumNamePascalCase}}::{{.ValueNameUpperCase}} },
{{end -}}
};
static const std::unordered_map<{{.EnumNamePascalCase}}, std::string> {{.EnumNameUpperCase}}_ENUM_TO_STRING = {
{{range .EnumValueArgs}}	{ {{$.EnumNamePascalCase}}::{{.ValueNameUpperCase}}, {{.ValueNameUpperCase}}_STRING },
{{end -}}
};

inline std::ostream& operator<<(std::ostream& os, const {{.EnumNamePascalCase}}& value) {
	os << {{.EnumNameUpperCase}}_ENUM_TO_STRING.at(value);
	return os;
}
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
			ValueNameUpperCase: strings.ToUpper(util.EnsureSnakeCase(v.Name)),
			ValueString:        v.Value,
			Index:              i,
		})
	}

	args := EnumArgs{
		EnumNamePascalCase: util.EnsurePascalCase(enum.Name),
		EnumNameUpperCase:  strings.ToUpper(util.EnsureSnakeCase(enum.Name)),
		EnumValueArgs:      enumValueArgs,
	}

	buf := &bytes.Buffer{}
	err := enumTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
