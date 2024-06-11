package cpp_gen

import (
	"bytes"
	"path/filepath"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

type IncludeArgs struct {
	LibIncludes []string
	SrcIncludes []string
}

const importTemplateStr = `
{{- range .LibIncludes }}
#include <{{.}}>{{end}}{{ range .SrcIncludes }}
#include "{{.}}"{{end}}
`

var (
	typedefIncludes = []string{
		"scg/typedef.h",
	}
	messageIncludes = []string{
		"scg/serialize.h",
		"nlohmann/json.hpp",
	}
	serviceIncludes = []string{
		"scg/client.h",
		"scg/serialize.h",
	}
)

var (
	importTemplate = template.Must(template.New("importTemplateGo").Parse(importTemplateStr))
)

func getOutputFileName(path string) string {
	_, filename := filepath.Split(path)
	filename = filename[:len(filename)-len(filepath.Ext(filename))]
	return util.EnsureSnakeCase(filename) + ".h"
}

func generateImportsCppCode(deps []parse.FileDependency, hasServices bool, hasMessages bool, hasTypedefs bool) (string, error) {

	args := IncludeArgs{}

	if hasTypedefs {
		args.LibIncludes = append(args.LibIncludes, typedefIncludes...)
	}

	if hasMessages {
		args.LibIncludes = append(args.LibIncludes, messageIncludes...)
	}

	if hasServices {
		args.LibIncludes = append(args.LibIncludes, serviceIncludes...)
	}

	args.LibIncludes = util.RemoveDuplicates(args.LibIncludes)

	for _, dep := range deps {
		args.SrcIncludes = append(args.SrcIncludes, getOutputFileName(dep.File.Name))
	}

	buf := &bytes.Buffer{}
	err := importTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
