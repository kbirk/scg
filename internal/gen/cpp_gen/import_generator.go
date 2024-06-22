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
		"scg/serialize.h",
		"scg/reader.h",
		"scg/writer.h",
		"scg/timestamp.h",
		"scg/uuid.h",
		"nlohmann/json.hpp",
	}
	messageIncludes = []string{
		"scg/typedef.h",
		"scg/serialize.h",
		"scg/reader.h",
		"scg/writer.h",
		"scg/timestamp.h",
		"scg/uuid.h",
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

func generateImportsCppCode(baseDir string, file *parse.File) (string, error) {

	args := IncludeArgs{}

	if len(file.Typedefs) > 0 {
		args.LibIncludes = append(args.LibIncludes, typedefIncludes...)
	}

	if len(file.MessageDefinitions) > 0 {
		args.LibIncludes = append(args.LibIncludes, messageIncludes...)
	}

	if len(file.ServiceDefinitions) > 0 {
		args.LibIncludes = append(args.LibIncludes, serviceIncludes...)
	}

	args.LibIncludes = util.RemoveDuplicates(args.LibIncludes)

	for _, dep := range file.GetFileDependencies() {
		args.SrcIncludes = append(args.SrcIncludes, filepath.Join(baseDir, getOutputFileName(dep.File.Name)))
	}

	buf := &bytes.Buffer{}
	err := importTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
