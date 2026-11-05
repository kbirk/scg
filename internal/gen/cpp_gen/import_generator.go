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
		"scg/message.h",
		"scg/serialize.h",
		"scg/reader.h",
		"scg/writer.h",
		"scg/timestamp.h",
		"scg/uuid.h",
		"scg/json.h",
		"nlohmann/json.hpp",
	}
	// enumConstIncludes are needed by enum and const declarations; files with
	// messages or typedefs already get them through the lists above.
	enumConstIncludes = []string{
		"scg/serialize.h",
		"scg/uuid.h",
		"scg/typedef.h",
		"cstdint",
		"ostream",
		"string",
		"unordered_map",
	}
	serviceIncludes = []string{
		"scg/client.h",
		"scg/server.h",
		"scg/stream.h",
		"scg/serialize.h",
		"functional",
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

// includePath returns the path used to #include a dependency's generated
// header from the header generated for file. Without a base dir, the path is
// relative to the including header's directory (quoted includes are searched
// there first), so the generated tree works from any include root. With a
// base dir, the path is baseDir joined with the dependency's path relative to
// the input root.
func includePath(baseDir string, file *parse.File, dep *parse.File) string {
	depPath := filepath.Join(dep.RelativePath, getOutputFileName(dep.Name))
	if baseDir != "" {
		return filepath.ToSlash(filepath.Join(baseDir, depPath))
	}
	rel, err := filepath.Rel(file.RelativePath, depPath)
	if err != nil {
		return filepath.ToSlash(depPath)
	}
	return filepath.ToSlash(rel)
}

func generateImportsCppCode(baseDir string, file *parse.File) (string, error) {

	args := IncludeArgs{}

	if len(file.Typedefs) > 0 {
		args.LibIncludes = append(args.LibIncludes, typedefIncludes...)
	}

	if len(file.MessageDefinitions) > 0 {
		args.LibIncludes = append(args.LibIncludes, messageIncludes...)
	}

	if (len(file.Enums) > 0 || len(file.Consts) > 0) && len(file.Typedefs) == 0 && len(file.MessageDefinitions) == 0 {
		args.LibIncludes = append(args.LibIncludes, enumConstIncludes...)
	}

	if len(file.ServiceDefinitions) > 0 {
		args.LibIncludes = append(args.LibIncludes, serviceIncludes...)
	}

	if len(args.LibIncludes) == 0 {
		// the file template always references scg::serialize float aliases
		args.LibIncludes = append(args.LibIncludes, "scg/serialize.h")
	}

	args.LibIncludes = util.RemoveDuplicates(args.LibIncludes)

	for _, dep := range file.GetFileDependencies() {
		args.SrcIncludes = append(args.SrcIncludes, includePath(baseDir, file, dep.File))
	}

	buf := &bytes.Buffer{}
	err := importTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
