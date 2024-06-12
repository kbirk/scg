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

	for _, pkg := range p.Packages {
		sb.WriteString(fmt.Sprintf("%s: %s\n", magenta("[package]"), white(pkg.Name)))
		for _, enum := range pkg.Enums {
			sb.WriteString(fmt.Sprintf("    %s %s {\n", black("[enum]"), white(enum.Name)))
			for _, value := range enum.Values {
				sb.WriteString(fmt.Sprintf("        %s = %s\n", white(value.Name), white(value.Index)))
			}
			sb.WriteString("    }\n")
		}
		for _, typedef := range pkg.Typedefs {
			sb.WriteString(fmt.Sprintf("    %s %s = %s\n", green("[typedef]"), white(typedef.Name), cyan(typedef.DataTypeDefinition.ToString())))
		}
		for _, msg := range pkg.MessageDefinitions {
			sb.WriteString(fmt.Sprintf("    %s %s {\n", blue("[message]"), white(msg.Name)))
			for _, field := range msg.FieldsByIndex() {
				sb.WriteString(fmt.Sprintf("        %s %s = %s\n", cyan(field.DataTypeDefinition.ToString()), white(field.Name), white(field.Index)))
			}
			sb.WriteString("    }\n")
		}
		for _, svc := range pkg.ServiceDefinitions {
			sb.WriteString(fmt.Sprintf("    %s %s {\n", yellow("[service]"), white(svc.Name)))
			for _, method := range svc.Methods {
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
}

type Package struct {
	Name                   string
	Declaration            *PackageDeclaration
	PackageDependencies    map[string]*PackageDependency
	Enums                  map[string]*EnumDefinition
	Typedefs               map[string]*TypedefDeclaration
	ServiceDefinitions     map[string]*ServiceDefinition
	MessageDefinitions     map[string]*MessageDefinition
	serverIDCollisionCheck map[uint64]string
	methodIDCollisionCheck map[uint64]map[uint64]string
}

func (p *Package) HashStringToServiceID(serviceName string) (uint64, error) {

	_, ok := p.ServiceDefinitions[serviceName]
	if !ok {
		return 0, fmt.Errorf("Service not found: %s", serviceName)
	}

	if p.serverIDCollisionCheck == nil {
		p.serverIDCollisionCheck = map[uint64]string{}
	}

	serverID := uint64(util.HashStringToUInt64(serviceName))
	existing, ok := p.serverIDCollisionCheck[serverID]
	if ok && existing != serviceName {
		return 0, fmt.Errorf("ServiceID collision detected: %s and %s both hash to %d", serviceName, existing, serverID)
	}
	p.serverIDCollisionCheck[serverID] = serviceName
	return serverID, nil
}

func (p *Package) HashStringToMethodID(serviceName string, methodName string) (uint64, error) {

	if p.methodIDCollisionCheck == nil {
		p.methodIDCollisionCheck = map[uint64]map[uint64]string{}
	}

	serviceID, err := p.HashStringToServiceID(serviceName)
	if err != nil {
		return 0, err
	}

	methodID := uint64(util.HashStringToUInt64(methodName))
	existingMethodIDs, ok := p.methodIDCollisionCheck[serviceID]
	if !ok {
		existingMethodIDs = map[uint64]string{}
	}
	existing, ok := existingMethodIDs[methodID]
	if ok && existing != methodName {
		return 0, fmt.Errorf("MethodID collision detected: %s and %s both hash to %d", methodName, existing, methodID)
	}
	existingMethodIDs[methodID] = methodName
	return methodID, nil
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

	for path, fileContent := range fileContents {

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

	for relativePathAndFile, f := range files {

		parse.Files[relativePathAndFile] = f

		if _, ok := parse.Packages[f.Package.Name]; !ok {
			// create the package if we haven't already
			parse.Packages[f.Package.Name] = &Package{
				Name:                f.Package.Name,
				Declaration:         f.Package,
				PackageDependencies: map[string]*PackageDependency{},
				Enums:               map[string]*EnumDefinition{},
				Typedefs:            map[string]*TypedefDeclaration{},
				MessageDefinitions:  map[string]*MessageDefinition{},
				ServiceDefinitions:  map[string]*ServiceDefinition{},
			}
		}
		// append definitions from the file to the package
		for _, v := range f.CustomTypeDependencies {
			if f.Package.Name == v.CustomTypePackage {
				// type exists in the same package, this is fine
				continue
			}
			parse.Packages[f.Package.Name].PackageDependencies[v.CustomTypePackage] = &PackageDependency{
				PackageName: v.CustomTypePackage,
				Token:       v.Token,
			}
		}
		for k, v := range f.Enums {
			_, ok := parse.Packages[f.Package.Name].Enums[k]
			if ok {
				return nil, &ParsingError{
					Message:  fmt.Sprintf("typedef %s defined multiple times", k),
					Token:    v.Token,
					Filename: f.Name,
					Content:  f.Content,
				}
			}
			parse.Packages[f.Package.Name].Enums[k] = v
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
