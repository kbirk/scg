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
	timestampImportsSTD = []string{
		"time",
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

func hasTimestampType(dataType *parse.DataTypeDefinition) bool {
	if dataType.Type == parse.DataTypeList {
		return hasTimestampType(dataType.SubType)
	}

	if dataType.Type == parse.DataTypeMap {
		return hasTimestampType(dataType.SubType)
	}

	return dataType.Type == parse.DataTypeTimestamp
}

func generateImportsGoCode(goBasePackage string, file *parse.File) (string, error) {

	args := ImportArgs{}

	if len(file.MessageDefinitions) > 0 || len(file.Typedefs) > 0 {
		args.STDPackages = append(args.STDPackages, messageImportsSTD...)
		args.SCGPackages = append(args.SCGPackages, messageImportsSCG...)

		// check if timestamp type
		for _, msg := range file.MessageDefinitions {
			for _, field := range msg.Fields {
				if hasTimestampType(field.DataTypeDefinition) {
					args.STDPackages = append(args.STDPackages, timestampImportsSTD...)
				}
			}
		}
	}

	if len(file.ServiceDefinitions) > 0 {
		args.STDPackages = append(args.STDPackages, serviceImportsSTD...)
		args.SCGPackages = append(args.SCGPackages, serviceImportsSCG...)
	}

	args.STDPackages = util.RemoveDuplicates(args.STDPackages)
	args.SCGPackages = util.RemoveDuplicates(args.SCGPackages)

	for _, dep := range file.GetPackageDependencies() {
		args.CustomPackages = append(args.CustomPackages, fmt.Sprintf("%s/%s", goBasePackage, convertPackageNameToGoPackage(dep.PackageName)))
	}

	buf := &bytes.Buffer{}
	err := importTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
