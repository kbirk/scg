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

// goReservedQualifiers are identifiers that generated code declares or uses
// at file scope or as local names in bodies that also reference other
// packages' types: the runtime / std imports, and the receivers, parameters and
// locals of generated functions. A dependency package whose Go name equals one
// of them is imported under an alias (see goPackageQualifier), so that e.g. an
// scg package named `rpc` does not clash with the scg rpc runtime import.
var goReservedQualifiers = map[string]bool{
	// imports used by generated code
	"fmt": true, "context": true, "json": true, "time": true, "uuid": true,
	"driver": true, "serialize": true, "rpc": true,
	// locals / parameters used by generated code
	"req": true, "resp": true, "err": true, "reader": true, "writer": true,
	"ctx": true, "handler": true, "middleware": true, "stream": true,
	"size": true, "result": true, "length": true, "arg": true, "def": true,
	"src": true, "data": true, "bs": true, "value": true, "requestID": true,
	"methodID": true, "server": true, "client": true, "impl": true,
	"ok": true, "jsonData": true,
}

// goPackageQualifier returns the identifier used to refer to another scg
// package's types from generated Go code. It is the Go package name, unless
// that would clash with an identifier generated code uses, in which case the
// import is aliased with a `_pkg` suffix. Single letter names are always
// aliased since generated methods use single letter receivers.
func goPackageQualifier(pkgName string) string {
	name := convertPackageNameToGoPackage(pkgName)
	if goReservedQualifiers[name] || len(name) == 1 {
		return name + "_pkg"
	}
	return name
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
