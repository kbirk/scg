package go_gen

import (
	"bytes"
	"fmt"
	"path"
	"path/filepath"
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
	{{.}}{{end}}
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
	uuidImports = []string{
		"github.com/google/uuid",
	}
	messageImportsSCG = []string{
		"github.com/kbirk/scg/pkg/serialize",
	}
	typedefImportsSTD = []string{
		"database/sql/driver",
		"fmt",
	}
	typedefImportsSCG = []string{
		"github.com/kbirk/scg/pkg/serialize",
	}
	enumImportsSTD = []string{
		"database/sql/driver",
		"fmt",
	}
	enumImportsSCG = []string{
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

func hasUUIDType(dataType *parse.DataTypeDefinition) bool {
	if dataType.Type == parse.DataTypeList {
		return hasUUIDType(dataType.SubType)
	}

	if dataType.Type == parse.DataTypeMap {
		if dataType.Key.Type == parse.DataTypeComparableUUID {
			return true
		}
		return hasUUIDType(dataType.SubType)
	}

	return dataType.Type == parse.DataTypeUUID
}

func hasUUIDConst(file *parse.File) bool {
	for _, c := range file.Consts {
		if c.DataTypeDefinition.Type == parse.DataTypeComparableUUID {
			return true
		}
		if c.UnderlyingDataTypeDefinition != nil && c.UnderlyingDataTypeDefinition.Type == parse.DataTypeComparableUUID {
			return true
		}
	}
	return false
}

// goImportPath returns the import path of the Go package generated for an scg
// package. Generated files are written next to their .scg source (relative to
// the input root), so the import path follows the source directory, not the
// scg package name.
func goImportPath(basePackage string, dir string) string {
	return path.Join(basePackage, filepath.ToSlash(dir))
}

// goImportSpec renders an import spec for a dependency package, aliased when
// the qualifier used in generated code differs from the last import path
// element.
func goImportSpec(basePackage string, pkgName string, dir string) string {
	importPath := goImportPath(basePackage, dir)
	qualifier := goPackageQualifier(pkgName)
	if path.Base(importPath) == qualifier {
		return fmt.Sprintf("%q", importPath)
	}
	return fmt.Sprintf("%s %q", qualifier, importPath)
}

func generateImportsGoCode(basePackage string, file *parse.File) (string, error) {
	return generateImportsGoCodeWithDirs(basePackage, file, nil)
}

// generateImportsGoCodeWithDirs generates the import block. pkgDirs maps each
// scg package name to the directory (relative to the input / output root)
// holding its files; when a package is missing from it, the directory is
// assumed to be the package name with '.' replaced by '_'.
func generateImportsGoCodeWithDirs(basePackage string, file *parse.File, pkgDirs map[string]string) (string, error) {

	args := ImportArgs{}

	if len(file.MessageDefinitions) > 0 {

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
		// check if uuid type
		for _, msg := range file.MessageDefinitions {
			for _, field := range msg.Fields {
				if hasUUIDType(field.DataTypeDefinition) {
					args.STDPackages = append(args.STDPackages, uuidImports...)
				}
			}
		}
	}

	if len(file.Typedefs) > 0 {

		args.STDPackages = append(args.STDPackages, typedefImportsSTD...)
		args.SCGPackages = append(args.SCGPackages, typedefImportsSCG...)

		// check if uuid type (uuid typedefs also implement json.Marshaler)
		for _, typedef := range file.Typedefs {
			if typedef.DataTypeDefinition.Type == parse.DataTypeComparableUUID {
				args.STDPackages = append(args.STDPackages, uuidImports...)
				args.STDPackages = append(args.STDPackages, "encoding/json")
			}
		}
	}

	if len(file.Enums) > 0 {
		args.STDPackages = append(args.STDPackages, enumImportsSTD...)
		args.SCGPackages = append(args.SCGPackages, enumImportsSCG...)
	}

	if len(file.ServiceDefinitions) > 0 {
		args.STDPackages = append(args.STDPackages, serviceImportsSTD...)
		args.SCGPackages = append(args.SCGPackages, serviceImportsSCG...)
	}

	if hasUUIDConst(file) {
		args.STDPackages = append(args.STDPackages, uuidImports...)
	}

	args.STDPackages = util.RemoveDuplicates(args.STDPackages)
	args.SCGPackages = util.RemoveDuplicates(args.SCGPackages)

	for _, dep := range file.GetPackageDependencies() {
		dir, ok := pkgDirs[dep.PackageName]
		if !ok {
			dir = convertPackageNameToGoPackage(dep.PackageName)
		}
		args.CustomPackages = append(args.CustomPackages, goImportSpec(basePackage, dep.PackageName, dir))
	}

	buf := &bytes.Buffer{}
	err := importTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
