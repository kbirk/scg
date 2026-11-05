package parse

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/kbirk/scg/internal/util"
)

type Parse struct {
	Version  string
	Files    map[string]*File
	Packages map[string]*Package

	minBitsMemo map[string]int
}

func (p *Parse) ToStringPretty() string {
	black := color.New(color.FgBlack, color.Bold).SprintFunc()
	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	blue := color.New(color.FgBlue, color.Bold).SprintFunc()
	yellow := color.New(color.FgYellow, color.Bold).SprintFunc()
	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	magenta := color.New(color.FgMagenta, color.Bold).SprintFunc()
	white := color.New(color.FgWhite, color.Bold).SprintFunc()

	var sb strings.Builder

	for _, pkgName := range sortedKeys(p.Packages) {
		pkg := p.Packages[pkgName]
		sb.WriteString(fmt.Sprintf("%s: %s\n", magenta("[package]"), white(pkg.Name)))
		for _, enumName := range sortedKeys(pkg.Enums) {
			enum := pkg.Enums[enumName]
			sb.WriteString(fmt.Sprintf("    %s %s {\n", black("[enum]"), white(enum.Name)))
			for _, value := range enum.ValuesByIndex() {
				sb.WriteString(fmt.Sprintf("        %s = %s\n", white(value.Name), white(value.Index)))
			}
			sb.WriteString("    }\n")
		}
		for _, typedefName := range sortedKeys(pkg.Typedefs) {
			typedef := pkg.Typedefs[typedefName]
			sb.WriteString(fmt.Sprintf("    %s %s = %s\n", green("[typedef]"), white(typedef.Name), cyan(typedef.DataTypeDefinition.ToString())))
		}
		for _, msgName := range sortedKeys(pkg.MessageDefinitions) {
			msg := pkg.MessageDefinitions[msgName]
			sb.WriteString(fmt.Sprintf("    %s %s {\n", blue("[message]"), white(msg.Name)))
			for _, field := range msg.FieldsByIndex() {
				sb.WriteString(fmt.Sprintf("        %s %s = %s\n", cyan(field.DataTypeDefinition.ToString()), white(field.Name), white(field.Index)))
			}
			sb.WriteString("    }\n")
		}
		for _, svcName := range sortedKeys(pkg.ServiceDefinitions) {
			svc := pkg.ServiceDefinitions[svcName]
			sb.WriteString(fmt.Sprintf("    %s %s {\n", yellow("[service]"), white(svc.Name)))
			for _, method := range svc.MethodsSortedByKey() {
				sb.WriteString(fmt.Sprintf("        %s (%s) %s\n", white(method.Name), cyan(method.Argument.CustomType), cyan(method.Return.CustomType)))
			}
			sb.WriteString("    }\n")
		}
	}

	return sb.String()
}

type PackageDependency struct {
	PackageName string
	Token       *Token
	// File is the file whose reference (Token) introduced the dependency.
	File *File
}

type Package struct {
	Name                string
	Declaration         *PackageDeclaration
	PackageDependencies map[string]*PackageDependency
	Enums               map[string]*EnumDefinition
	Typedefs            map[string]*TypedefDeclaration
	Consts              map[string]*ConstDeclaration
	ServiceDefinitions  map[string]*ServiceDefinition
	MessageDefinitions  map[string]*MessageDefinition
}

// HashStringToServiceID returns the wire ID of a service of this package.
// IDs are assigned (and checked for collisions across all packages) when the
// parse is resolved; see assignRPCIDs.
func (p *Package) HashStringToServiceID(serviceName string) (uint64, error) {
	svc, ok := p.ServiceDefinitions[serviceName]
	if !ok {
		return 0, fmt.Errorf("Service not found: %s", serviceName)
	}
	if svc.ID == 0 {
		return util.HashStringToUInt64(QualifiedServiceName(p.Name, serviceName)), nil
	}
	return svc.ID, nil
}

// HashStringToMethodID returns the wire ID of a method of a service of this
// package. See HashStringToServiceID.
func (p *Package) HashStringToMethodID(serviceName string, methodName string) (uint64, error) {
	svc, ok := p.ServiceDefinitions[serviceName]
	if !ok {
		return 0, fmt.Errorf("Service not found: %s", serviceName)
	}
	method, ok := svc.Methods[methodName]
	if !ok {
		return 0, fmt.Errorf("Method not found: %s.%s", serviceName, methodName)
	}
	if method.ID == 0 {
		return util.HashStringToUInt64(QualifiedMethodName(p.Name, serviceName, methodName)), nil
	}
	return method.ID, nil
}

func NewParse(inputDir string) (*Parse, error) {
	fileContents, err := searchInputPatternAndReadFiles(inputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to parse input pattern: %s", err.Error())
	}
	files, perr := parseFileContents(inputDir, fileContents)
	if perr != nil {
		return nil, perr.Error()
	}
	p, perr := resolveFilesIntoParse(files)
	if perr != nil {
		return nil, perr.Error()
	}
	return p, nil
}

func NewParseFromFiles(inputDir string, fileContents map[string]string) (*Parse, error) {
	files, perr := parseFileContents(inputDir, fileContents)
	if perr != nil {
		return nil, fmt.Errorf("failed to parse input pattern: %s", perr.Error())
	}
	p, perr := resolveFilesIntoParse(files)
	if perr != nil {
		return nil, perr.Error()
	}
	return p, nil
}

func findSCGFiles(files *[]string, path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			err := findSCGFiles(files, fullPath)
			if err != nil {
				return err
			}
		} else if strings.HasSuffix(entry.Name(), ".scg") {
			*files = append(*files, fullPath)
		}
	}

	return nil
}

func ensureIsDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}
	return nil
}

func searchInputPatternAndReadFiles(inputDir string) (map[string]string, error) {

	err := ensureIsDir(inputDir)
	if err != nil {
		return nil, err
	}

	var paths []string
	err = findSCGFiles(&paths, inputDir)
	if err != nil {
		return nil, err
	}

	fileContents := make(map[string]string)

	for _, path := range paths {
		bs, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		fileContents[path] = string(bs)
	}

	return fileContents, nil
}

func parseFileContents(inputDir string, fileContents map[string]string) (map[string]*File, *ParsingError) {

	files := make(map[string]*File)

	for _, path := range sortedKeys(fileContents) {
		fileContent := fileContents[path]

		fileContent = strings.Replace(fileContent, "\t", "    ", -1)

		relativePathAndFile, err := filepath.Rel(inputDir, path)
		if err != nil {
			return nil, &ParsingError{
				Message:  "internal parsing error: failed to get relative path of file",
				Token:    nil,
				Filename: path,
				Content:  fileContent,
			}
		}

		relativeDir := filepath.Dir(relativePathAndFile)

		f, perr := parseFileContent(path, relativeDir, fileContent)
		if perr != nil {
			perr.Filename = path
			perr.Content = fileContent
			return nil, perr
		}
		f.RelativePath = relativeDir

		files[relativePathAndFile] = f
	}

	return files, nil
}

func resolveFilesIntoParse(files map[string]*File) (*Parse, *ParsingError) {

	parse := &Parse{
		Files:    make(map[string]*File),
		Packages: make(map[string]*Package),
	}

	for _, relativePathAndFile := range sortedKeys(files) {
		f := files[relativePathAndFile]

		parse.Files[relativePathAndFile] = f

		if _, ok := parse.Packages[f.Package.Name]; !ok {
			// create the package if we haven't already
			parse.Packages[f.Package.Name] = &Package{
				Name:                f.Package.Name,
				Declaration:         f.Package,
				PackageDependencies: map[string]*PackageDependency{},
				Enums:               map[string]*EnumDefinition{},
				Consts:              map[string]*ConstDeclaration{},
				Typedefs:            map[string]*TypedefDeclaration{},
				MessageDefinitions:  map[string]*MessageDefinition{},
				ServiceDefinitions:  map[string]*ServiceDefinition{},
			}
		}
		// append definitions from the file to the package
		// Iterate in sorted order and keep the first reference to each package,
		// so errors about a dependency point at a deterministic location.
		for _, key := range sortedKeys(f.CustomTypeDependencies) {
			v := f.CustomTypeDependencies[key]
			if f.Package.Name == v.CustomTypePackage {
				// type exists in the same package, this is fine
				continue
			}
			deps := parse.Packages[f.Package.Name].PackageDependencies
			if _, ok := deps[v.CustomTypePackage]; ok {
				continue
			}
			deps[v.CustomTypePackage] = &PackageDependency{
				PackageName: v.CustomTypePackage,
				Token:       v.Token,
				File:        f,
			}
		}
		for k, v := range f.Enums {
			_, ok := parse.Packages[f.Package.Name].Enums[k]
			if ok {
				return nil, &ParsingError{
					Message:  fmt.Sprintf("enum %s defined multiple times", k),
					Token:    v.Token,
					Filename: f.Name,
					Content:  f.Content,
				}
			}
			parse.Packages[f.Package.Name].Enums[k] = v
		}
		for k, v := range f.Consts {
			_, ok := parse.Packages[f.Package.Name].Consts[k]
			if ok {
				return nil, &ParsingError{
					Message:  fmt.Sprintf("const %s defined multiple times", k),
					Token:    v.Token,
					Filename: f.Name,
					Content:  f.Content,
				}
			}
			parse.Packages[f.Package.Name].Consts[k] = v
		}
		for k, v := range f.Typedefs {
			_, ok := parse.Packages[f.Package.Name].Typedefs[k]
			if ok {
				return nil, &ParsingError{
					Message:  fmt.Sprintf("typedef %s defined multiple times", k),
					Token:    v.Token,
					Filename: f.Name,
					Content:  f.Content,
				}
			}
			parse.Packages[f.Package.Name].Typedefs[k] = v
		}
		for k, v := range f.ServiceDefinitions {
			_, ok := parse.Packages[f.Package.Name].ServiceDefinitions[k]
			if ok {
				return nil, &ParsingError{
					Message:  fmt.Sprintf("Service %s defined multiple times", k),
					Token:    v.Token,
					Filename: f.Name,
					Content:  f.Content,
				}
			}
			parse.Packages[f.Package.Name].ServiceDefinitions[k] = v
		}
		for k, v := range f.MessageDefinitions {
			_, ok := parse.Packages[f.Package.Name].MessageDefinitions[k]
			if ok {
				return nil, &ParsingError{
					Message:  fmt.Sprintf("Message %s defined multiple times", k),
					Token:    v.Token,
					Filename: f.Name,
					Content:  f.Content,
				}
			}
			parse.Packages[f.Package.Name].MessageDefinitions[k] = v
		}
	}

	perr := resolveDefinitions(parse)
	if perr != nil {
		return nil, perr
	}

	return parse, nil
}
