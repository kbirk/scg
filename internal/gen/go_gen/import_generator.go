package go_gen

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

type ImportArgs struct {
	CustomPackages []string
	STDPackages    []string
	SCGPackages    []string
}

const importTemplateStr = `
import ( {{ range .STDPackages }}
	"{{.}}"{{end}}
	{{ range .CustomPackages }}
	"{{.}}"{{end}}
	{{ range .SCGPackages }}
	"{{.}}"{{end}}
)
`

var (
	messageImportsSTD = []string{
		"encoding/json",
	}
	messageImportsSCG = []string{
		"github.com/kbirk/scg/pkg/serialize",
	}
	serviceImportsSTD = []string{
		"context",
		"fmt",
	}
	serviceImportsSCG = []string{
		"github.com/kbirk/scg/pkg/rpc",
		"github.com/kbirk/scg/pkg/serialize",
	}
)

var (
	importTemplate = template.Must(template.New("importTemplateGo").Parse(importTemplateStr))
)

func generateImportsGoCode(goBasePackage string, deps []parse.PackageDependency, hasServices bool, hasMessages bool) (string, error) {

	args := ImportArgs{}

	if hasMessages {
		args.STDPackages = append(args.STDPackages, messageImportsSTD...)
		args.SCGPackages = append(args.SCGPackages, messageImportsSCG...)
	}

	if hasServices {
		args.STDPackages = append(args.STDPackages, serviceImportsSTD...)
		args.SCGPackages = append(args.SCGPackages, serviceImportsSCG...)
	}

	args.STDPackages = util.RemoveDuplicates(args.STDPackages)
	args.SCGPackages = util.RemoveDuplicates(args.SCGPackages)

	for _, dep := range deps {
		fmt.Printf("Adding dependincy: %s\n", dep.Package.Name)
		args.CustomPackages = append(args.CustomPackages, fmt.Sprintf("%s/%s", goBasePackage, convertPackageNameToGoPackage(dep.Package.Name)))
	}

	buf := &bytes.Buffer{}
	err := importTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
