package go_gen

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
)

type PackageArgs struct {
	Name string
}

const packageTemplateStr = `
package {{.Name}}
`

var (
	packageTemplate = template.Must(template.New("packageTemplateGo").Parse(packageTemplateStr))
)

func convertPackageNameToGoPackage(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, ".", "_"))
}

func generatePackageGoCode(pkg *parse.PackageDeclaration) (string, error) {

	args := PackageArgs{
		Name: convertPackageNameToGoPackage(pkg.Name),
	}

	buf := &bytes.Buffer{}
	err := packageTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
