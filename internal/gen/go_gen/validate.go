package go_gen

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
}

// goMessageMethods are the methods generated on every message type; a field
// with the same Go name would not compile.
var goMessageMethods = map[string]bool{
	"ToJSON": true, "FromJSON": true, "ToBytes": true, "FromBytes": true,
	"BitSize": true, "Serialize": true, "Deserialize": true,
}

// goIdentifiers records generated identifiers within one scope and reports the
// first collision.
type goIdentifiers struct {
	scope  string
	origin map[string]string
}

func newGoIdentifiers(scope string) *goIdentifiers {
	return &goIdentifiers{scope: scope, origin: map[string]string{}}
}

func (ids *goIdentifiers) add(name string, origin string) error {
	if existing, ok := ids.origin[name]; ok {
		return fmt.Errorf("Go identifier collision in %s: %s and %s both generate `%s`; rename one of them", ids.scope, existing, origin, name)
	}
	ids.origin[name] = origin
	return nil
}

func sortedMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// validateGo rejects schemas whose generated Go code would not compile or
// would be laid out inconsistently, with an error naming the cause.
func validateGo(p *parse.Parse) error {
	if err := validateGoLayout(p); err != nil {
		return err
	}
	for _, pkgName := range sortedMapKeys(p.Packages) {
		if err := validateGoPackage(p, p.Packages[pkgName]); err != nil {
			return err
		}
	}
	return nil
}

func fileDir(file *parse.File) string {
	if file.RelativePath == "" {
		return "."
	}
	return file.RelativePath
}

// validateGoLayout checks the mapping between scg packages, directories and
// output files. Go requires one package per directory, and a package that is
// imported (or whose files reference each other) must live in one directory.
func validateGoLayout(p *parse.Parse) error {
	dirPkgs := map[string]map[string]bool{}
	pkgDirs := map[string]map[string]bool{}
	outputs := map[string]string{}

	for _, path := range sortedMapKeys(p.Files) {
		file := p.Files[path]
		dir := fileDir(file)
		if dirPkgs[dir] == nil {
			dirPkgs[dir] = map[string]bool{}
		}
		dirPkgs[dir][file.Package.Name] = true
		if pkgDirs[file.Package.Name] == nil {
			pkgDirs[file.Package.Name] = map[string]bool{}
		}
		pkgDirs[file.Package.Name][dir] = true

		out := filepath.Join(dir, goOutputFileName(path))
		if existing, ok := outputs[out]; ok {
			return fmt.Errorf("files %s and %s would both generate %s; rename one of them", existing, path, out)
		}
		outputs[out] = path
	}

	for _, dir := range sortedMapKeys(dirPkgs) {
		if pkgs := sortedMapKeys(dirPkgs[dir]); len(pkgs) > 1 {
			return fmt.Errorf("directory %s contains files of packages %s; Go requires exactly one package per directory, move each package into its own directory", dir, strings.Join(pkgs, ", "))
		}
	}

	for _, path := range sortedMapKeys(p.Files) {
		file := p.Files[path]
		qualifiers := map[string]string{}
		for _, dep := range file.GetPackageDependencies() {
			q := goPackageQualifier(dep.PackageName)
			if existing, ok := qualifiers[q]; ok {
				return fmt.Errorf("%s imports packages %s and %s, which both map to the Go package name `%s`; rename one of them", path, existing, dep.PackageName, q)
			}
			qualifiers[q] = dep.PackageName
		}
		for _, dep := range file.GetFileDependencies() {
			depPkg := dep.File.Package.Name
			dirs := sortedMapKeys(pkgDirs[depPkg])
			if len(dirs) <= 1 {
				continue
			}
			if depPkg != file.Package.Name {
				return fmt.Errorf("package %s is imported by %s but is split across directories %s; Go requires an imported package to live in one directory", depPkg, path, strings.Join(dirs, ", "))
			}
			if fileDir(dep.File) != fileDir(file) {
				return fmt.Errorf("%s uses types from %s, which is in the same package %s but a different directory; Go requires a package to live in one directory", path, dep.File.Name, depPkg)
			}
		}
	}
	return nil
}

func validateGoPackage(p *parse.Parse, pkg *parse.Package) error {

	goPkgName := convertPackageNameToGoPackage(pkg.Name)
	if goKeywords[goPkgName] {
		return fmt.Errorf("package %s: the Go package name `%s` is a Go keyword; rename the package", pkg.Name, goPkgName)
	}

	ids := newGoIdentifiers(fmt.Sprintf("package %s", pkg.Name))

	for _, name := range sortedMapKeys(pkg.MessageDefinitions) {
		msg := pkg.MessageDefinitions[name]
		if err := ids.add(util.EnsurePascalCase(msg.Name), fmt.Sprintf("message %s", msg.Name)); err != nil {
			return err
		}
		if err := validateGoMessageFields(msg); err != nil {
			return err
		}
	}

	for _, name := range sortedMapKeys(pkg.Enums) {
		enum := pkg.Enums[name]
		enumName := util.EnsurePascalCase(enum.Name)
		origin := fmt.Sprintf("enum %s", enum.Name)
		if err := ids.add(enumName, origin); err != nil {
			return err
		}
		if err := ids.add(enumName+"_ToString", origin); err != nil {
			return err
		}
		if err := ids.add(enumName+"String_ToEnum", origin); err != nil {
			return err
		}
		for _, v := range enum.ValuesByIndex() {
			valueOrigin := fmt.Sprintf("enum value %s.%s", enum.Name, v.Name)
			valueName := enumName + "_" + util.EnsurePascalCase(v.Name)
			if err := ids.add(valueName, valueOrigin); err != nil {
				return err
			}
			if err := ids.add(valueName+"_String", valueOrigin); err != nil {
				return err
			}
		}
	}

	for _, name := range sortedMapKeys(pkg.Typedefs) {
		typdef := pkg.Typedefs[name]
		origin := fmt.Sprintf("typedef %s", typdef.Name)
		typedefName := util.EnsurePascalCase(typdef.Name)
		if err := ids.add(typedefName, origin); err != nil {
			return err
		}
		if err := ids.add("New"+typedefName, origin); err != nil {
			return err
		}
	}

	for _, name := range sortedMapKeys(pkg.Consts) {
		c := pkg.Consts[name]
		if err := ids.add(util.EnsurePascalCase(c.Name), fmt.Sprintf("const %s", c.Name)); err != nil {
			return err
		}
	}

	for _, name := range sortedMapKeys(pkg.ServiceDefinitions) {
		svc := pkg.ServiceDefinitions[name]
		origin := fmt.Sprintf("service %s", svc.Name)
		svcPascal := util.EnsurePascalCase(svc.Name)
		for _, id := range []string{
			getServerNamePascalCase(svc.Name),
			"Register" + getServerNamePascalCase(svc.Name),
			getServerStubStructName(svc.Name),
			serviceIDVarName(svc.Name),
			svcPascal + "Api",
			svcPascal + "Client",
			"New" + svcPascal + "Client",
		} {
			if err := ids.add(id, origin); err != nil {
				return err
			}
		}
		methods := newGoIdentifiers(fmt.Sprintf("service %s", svc.Name))
		for _, method := range svc.MethodsSortedByKey() {
			methodOrigin := fmt.Sprintf("method %s.%s", svc.Name, method.Name)
			if err := methods.add(util.EnsurePascalCase(method.Name), methodOrigin); err != nil {
				return err
			}
			if err := ids.add(methodIDVarName(svc.Name, method.Name), methodOrigin); err != nil {
				return err
			}
			if method.IsStreaming() {
				if err := ids.add(streamServerTypeName(svc.Name, method.Name), methodOrigin); err != nil {
					return err
				}
				if err := ids.add(streamClientTypeName(svc.Name, method.Name), methodOrigin); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func validateGoMessageFields(msg *parse.MessageDefinition) error {
	fields := newGoIdentifiers(fmt.Sprintf("message %s", msg.Name))
	jsonKeys := newGoIdentifiers(fmt.Sprintf("message %s JSON keys", msg.Name))
	for _, field := range msg.FieldsByIndex() {
		goName := util.EnsurePascalCase(field.Name)
		origin := fmt.Sprintf("field %s", field.Name)
		if goMessageMethods[goName] {
			return fmt.Errorf("message %s: field %s generates the Go field `%s`, which collides with the generated method %s(); rename the field", msg.Name, field.Name, goName, goName)
		}
		if err := fields.add(goName, origin); err != nil {
			return err
		}
		if err := jsonKeys.add(util.EnsureSnakeCase(field.Name), origin); err != nil {
			return err
		}
	}
	return nil
}
