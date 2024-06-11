package parse

import (
	"path/filepath"
	"sort"
)

const (
	commentDelimiter = '#'
)

type CustomTypeDependency struct {
	CustomTypePackage string
	CustomTypeName    string
	File              *File
	Token             *Token
}

type FileDependency struct {
	File  *File
	Token *Token
}

type File struct {
	Name                   string
	Content                string
	Version                string
	RelativePath           string
	FullPath               string
	Package                *PackageDeclaration
	CustomTypeDependencies map[string]*CustomTypeDependency
	Typedefs               map[string]*TypedefDeclaration
	ServiceDefinitions     map[string]*ServiceDefinition
	MessageDefinitions     map[string]*MessageDefinition
}

func (f *File) GetPackageDependencies() []PackageDependency {
	seen := make(map[string]bool)
	pkgs := []PackageDependency{}
	for _, dep := range f.CustomTypeDependencies {
		if dep.CustomTypePackage == f.Package.Name {
			// don't add package dependencies for the same package
			continue
		}
		_, ok := seen[dep.CustomTypePackage]
		if !ok {
			seen[dep.CustomTypePackage] = true
			pkgs = append(pkgs, PackageDependency{
				PackageName: dep.CustomTypePackage,
				Token:       dep.Token,
			})
		}
	}
	return pkgs
}

func (f *File) GetFileDependencies() []FileDependency {
	seen := make(map[string]bool)
	files := []FileDependency{}
	for _, dep := range f.CustomTypeDependencies {
		_, ok := seen[dep.File.Name]
		if !ok {
			seen[dep.File.Name] = true
			files = append(files, FileDependency{
				File:  dep.File,
				Token: dep.Token,
			})
		}
	}
	return files
}

func (f *File) TypedefsSortedByKey() []*TypedefDeclaration {
	keys := make([]string, 0, len(f.Typedefs))
	for k := range f.Typedefs {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	values := make([]*TypedefDeclaration, 0, len(f.Typedefs))
	for _, k := range keys {
		values = append(values, f.Typedefs[k])
	}

	return values
}

func (f *File) MessagesSortedByKey() []*MessageDefinition {
	keys := make([]string, 0, len(f.MessageDefinitions))
	for k := range f.MessageDefinitions {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	values := make([]*MessageDefinition, 0, len(f.MessageDefinitions))
	for _, k := range keys {
		values = append(values, f.MessageDefinitions[k])
	}

	return values
}

func (f *File) ServicesSortedByKey() []*ServiceDefinition {
	keys := make([]string, 0, len(f.ServiceDefinitions))
	for k := range f.ServiceDefinitions {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	values := make([]*ServiceDefinition, 0, len(f.ServiceDefinitions))
	for _, k := range keys {
		values = append(values, f.ServiceDefinitions[k])
	}

	return values
}

func populateDataTypeComparablePackageIfMissing(packageName string, dataType *DataTypeComparableDefinition) *ParsingError {
	if dataType.Type == DataTypeComparableCustom {
		if dataType.CustomTypePackage == "" {
			dataType.CustomTypePackage = packageName
		}
		if dataType.CustomTypePackage != packageName {
			dataType.ImportedFromOtherPackage = true
		} else {
			dataType.ImportedFromOtherPackage = false
		}
	}
	return nil
}

func populateDataTypePackageIfMissing(packageName string, dataType *DataTypeDefinition) *ParsingError {
	if dataType.Type == DataTypeCustom {
		if dataType.CustomTypePackage == "" {
			dataType.CustomTypePackage = packageName
		}
		if dataType.CustomTypePackage != packageName {
			dataType.ImportedFromOtherPackage = true
		} else {
			dataType.ImportedFromOtherPackage = false
		}
		return nil
	}

	if dataType.Type == DataTypeList {
		if dataType.SubType == nil {
			return &ParsingError{
				Message: "internal parsing error: list subtype not found",
				Token:   dataType.Token,
			}
		}
		return populateDataTypePackageIfMissing(packageName, dataType.SubType)
	}

	if dataType.Type == DataTypeMap {
		if dataType.Key == nil {
			return &ParsingError{
				Message: "internal parsing error: map key not found",
				Token:   dataType.Token,
			}
		}
		if dataType.SubType == nil {
			return &ParsingError{
				Message: "internal parsing error: map subtype not found",
				Token:   dataType.Token,
			}
		}
		err := populateDataTypeComparablePackageIfMissing(packageName, dataType.Key)
		if err != nil {
			return err
		}
		return populateDataTypePackageIfMissing(packageName, dataType.SubType)
	}
	return nil
}

func addCustomComparableTypeDependency(dependencies map[string]*CustomTypeDependency, dataType *DataTypeComparableDefinition) *ParsingError {
	if dataType.Type == DataTypeComparableCustom {
		if _, ok := dependencies[dataType.CustomTypePackage]; !ok {
			dependencies[dataType.CustomTypePackage] = &CustomTypeDependency{
				CustomTypePackage: dataType.CustomTypePackage,
				CustomTypeName:    dataType.CustomType,
				Token:             dataType.Token,
			}
		}
	}
	return nil
}

func addCustomTypeDependency(dependencies map[string]*CustomTypeDependency, dataType *DataTypeDefinition) *ParsingError {

	if dataType.Type == DataTypeList {
		if dataType.SubType == nil {
			return &ParsingError{
				Message: "internal parsing error: list / map subtype not found",
				Token:   dataType.Token,
			}
		}
		return addCustomTypeDependency(dependencies, dataType.SubType)
	}

	if dataType.Type == DataTypeMap {
		if dataType.Key == nil {
			return &ParsingError{
				Message: "internal parsing error: map key not found",
				Token:   dataType.Token,
			}
		}

		err := addCustomComparableTypeDependency(dependencies, dataType.Key)
		if err != nil {
			return err
		}

		if dataType.SubType == nil {
			return &ParsingError{
				Message: "internal parsing error: list / map subtype not found",
				Token:   dataType.Token,
			}
		}
		return addCustomTypeDependency(dependencies, dataType.SubType)
	}

	if dataType.Type == DataTypeCustom {
		if _, ok := dependencies[dataType.CustomTypePackage]; !ok {
			dependencies[dataType.CustomTypePackage] = &CustomTypeDependency{
				CustomTypePackage: dataType.CustomTypePackage,
				CustomTypeName:    dataType.CustomType,
				Token:             dataType.Token,
			}
		}

	}
	return nil
}

func parseFileContent(path string, relativeDir string, input string) (*File, *ParsingError) {

	tokens, perr := tokenizeFile(input)
	if perr != nil {
		return nil, perr
	}

	f := &File{
		FullPath:     path,
		RelativePath: relativeDir,
		Name:         filepath.Base(path),
	}

	pkg, perr := parsePackageDeclaration(tokens)
	if perr != nil {
		return nil, perr
	}

	typedefs, perr := parseTypedefDeclaration(tokens)
	if perr != nil {
		return nil, perr
	}

	messageDefinitions, perr := parseMessageDefinitions(tokens)
	if perr != nil {
		return nil, perr
	}

	dependencies := make(map[string]*CustomTypeDependency)

	for _, typdef := range typedefs {
		// set the file
		typdef.File = f

		// if package is omitted, use the file's package name
		perr := populateDataTypeComparablePackageIfMissing(pkg.Name, typdef.DataTypeDefinition)
		if perr != nil {
			return nil, perr
		}

		// TODO: add custom type dependencies
		perr = addCustomComparableTypeDependency(dependencies, typdef.DataTypeDefinition)
		if perr != nil {
			return nil, perr
		}
	}

	for _, msg := range messageDefinitions {

		// set the file
		msg.File = f

		for _, field := range msg.Fields {
			// if package is omitted, use the file's package name
			perr := populateDataTypePackageIfMissing(pkg.Name, field.DataTypeDefinition)
			if perr != nil {
				return nil, perr
			}

			// add custom type dependencies
			perr = addCustomTypeDependency(dependencies, field.DataTypeDefinition)
			if perr != nil {
				return nil, perr
			}
		}
	}

	serviceDefinitions, perr := parseServiceDefinitions(tokens)
	if perr != nil {
		return nil, perr
	}
	// if package is omitted, use the file's package name
	for _, svc := range serviceDefinitions {

		// set the file
		svc.File = f

		for _, method := range svc.Methods {
			// if package is omitted, use the file's package name
			perr := populateDataTypePackageIfMissing(pkg.Name, method.Argument)
			if perr != nil {
				return nil, perr
			}
			// add custom type dependencies
			perr = addCustomTypeDependency(dependencies, method.Argument)
			if perr != nil {
				return nil, perr
			}

			// if package is omitted, use the file's package name
			perr = populateDataTypePackageIfMissing(pkg.Name, method.Return)
			if perr != nil {
				return nil, perr
			}
			// add custom type dependencies
			perr = addCustomTypeDependency(dependencies, method.Return)
			if perr != nil {
				return nil, perr
			}
		}
	}

	externalCustomTypeDependencies := make(map[string]*CustomTypeDependency)
	for _, dep := range dependencies {
		// check if the custom type is defined in this file
		_, ok := messageDefinitions[dep.CustomTypeName]
		if ok {
			continue
		}

		_, ok = typedefs[dep.CustomTypeName]
		if ok {
			continue
		}

		// custom type depdenency is not defined in this file
		externalCustomTypeDependencies[dep.CustomTypeName] = dep
	}

	f.Content = input
	f.Package = pkg
	f.CustomTypeDependencies = externalCustomTypeDependencies
	f.Typedefs = typedefs
	f.MessageDefinitions = messageDefinitions
	f.ServiceDefinitions = serviceDefinitions

	return f, nil
}
