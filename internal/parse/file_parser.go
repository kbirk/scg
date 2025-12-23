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
	Consts                 map[string]*ConstDeclaration
	Enums                  map[string]*EnumDefinition
	ServiceDefinitions     map[string]*ServiceDefinition
	StreamDefinitions      map[string]*StreamDefinition
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

func (f *File) EnumsSortedByKey() []*EnumDefinition {
	keys := make([]string, 0, len(f.Enums))
	for k := range f.Enums {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	values := make([]*EnumDefinition, 0, len(f.Enums))
	for _, k := range keys {
		values = append(values, f.Enums[k])
	}
	return values
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

func (f *File) ConstsSortedByKey() []*ConstDeclaration {
	keys := make([]string, 0, len(f.Consts))
	for k := range f.Consts {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	values := make([]*ConstDeclaration, 0, len(f.Consts))
	for _, k := range keys {
		values = append(values, f.Consts[k])
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

func (f *File) MessagesSortedByDependenciesAndKeys() []*MessageDefinition {

	// build edges of the dependency graph
	//
	// edge from A -> B means that A is a dependency of B, you require A to have B

	outgoingEdges := map[string][]string{}
	incomingEdges := map[string][]string{}
	for k := range f.MessageDefinitions {
		outgoingEdges[k] = []string{}
		incomingEdges[k] = []string{}
	}

	for _, msg := range f.MessagesSortedByKey() {
		for _, field := range msg.FieldsByIndex() {
			elem := field.DataTypeDefinition.GetElementType()
			if elem.Type == DataTypeCustom {

				_, ok := f.MessageDefinitions[elem.CustomType]
				if !ok {
					// custom type dependency is not defined in this file
					continue
				}

				// add incoming edge
				incomingEdges[msg.Name] = append(incomingEdges[msg.Name], elem.CustomType)

				// add outgoing edge
				outgoingEdges[elem.CustomType] = append(outgoingEdges[elem.CustomType], msg.Name)
			}
		}
	}

	// kahn's algorithm to sort them

	stack := []string{}
	sorted := []string{}

	// add nodes with no incoming edges
	for k, edge := range incomingEdges {
		if len(edge) == 0 {
			stack = append(stack, k)
		}
	}

	// sort for deterministic output
	sort.Slice(stack, func(i, j int) bool { return stack[i] < stack[j] })

	for len(stack) > 0 {
		// pop a node
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		// add to sorted list
		sorted = append(sorted, n)

		outgoingEdgesForN, ok := outgoingEdges[n]
		if !ok {
			panic("missing edge")
		}

		// for each node m with an incoming edge from n
		for _, m := range outgoingEdgesForN {
			// remove edge n -> m
			incomingEdgesForM, ok := incomingEdges[m]
			if !ok {
				panic("missing edge")
			}

			// remove n from incoming edges of m
			indexToRemove := -1
			for i, v := range incomingEdgesForM {
				if v == n {
					indexToRemove = i
					break
				}
			}
			if indexToRemove == -1 {
				panic("missing edge")
			}
			incomingEdgesForM = append(incomingEdgesForM[:indexToRemove], incomingEdgesForM[indexToRemove+1:]...)

			// if m has no other incoming edges then insert m into S
			if len(incomingEdgesForM) == 0 {
				delete(incomingEdges, m)
				stack = append(stack, m)
			} else {
				incomingEdges[m] = incomingEdgesForM
			}
		}

		delete(outgoingEdges, n)
		delete(incomingEdges, n)
	}

	if len(outgoingEdges) > 0 || len(incomingEdges) > 0 {
		panic("circular dependency detected")
	}

	values := make([]*MessageDefinition, 0, len(f.MessageDefinitions))
	for _, k := range sorted {
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

func (f *File) StreamsSortedByKey() []*StreamDefinition {
	keys := make([]string, 0, len(f.StreamDefinitions))
	for k := range f.StreamDefinitions {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	values := make([]*StreamDefinition, 0, len(f.StreamDefinitions))
	for _, k := range keys {
		values = append(values, f.StreamDefinitions[k])
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
		if _, ok := dependencies[dataType.ToString()]; !ok {
			dependencies[dataType.ToString()] = &CustomTypeDependency{
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
		if _, ok := dependencies[dataType.ToString()]; !ok {
			dependencies[dataType.ToString()] = &CustomTypeDependency{
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

	enums, perr := parseEnumDefinitions(tokens)
	if perr != nil {
		return nil, perr
	}

	consts, perr := parseConstDeclarations(tokens)
	if perr != nil {
		return nil, perr
	}

	typedefs, perr := parseTypedefDeclarations(tokens)
	if perr != nil {
		return nil, perr
	}

	messageDefinitions, perr := parseMessageDefinitions(tokens)
	if perr != nil {
		return nil, perr
	}

	dependencies := make(map[string]*CustomTypeDependency)

	for _, enum := range enums {
		enum.File = f
	}

	for _, typdef := range typedefs {
		typdef.File = f
	}

	for _, cosntDecl := range consts {
		cosntDecl.File = f

		// if package is omitted, use the file's package name
		perr := populateDataTypeComparablePackageIfMissing(pkg.Name, cosntDecl.DataTypeDefinition)
		if perr != nil {
			return nil, perr
		}

		// add custom type dependencies
		err := addCustomComparableTypeDependency(dependencies, cosntDecl.DataTypeDefinition)
		if err != nil {
			return nil, err
		}
	}

	for _, msg := range messageDefinitions {

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

	streamDefinitions, perr := parseStreamDefinitions(tokens)
	if perr != nil {
		return nil, perr
	}
	// if package is omitted, use the file's package name
	for _, stream := range streamDefinitions {

		stream.File = f

		for _, method := range stream.Methods {
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
		_, ok := enums[dep.CustomTypeName]
		if ok {
			continue
		}

		_, ok = typedefs[dep.CustomTypeName]
		if ok {
			continue
		}

		_, ok = messageDefinitions[dep.CustomTypeName]
		if ok {
			continue
		}

		_, ok = streamDefinitions[dep.CustomTypeName]
		if ok {
			continue
		}

		// custom type depdenency is not defined in this file
		externalCustomTypeDependencies[dep.CustomTypeName] = dep
	}

	f.Content = input
	f.Package = pkg
	f.CustomTypeDependencies = externalCustomTypeDependencies
	f.Enums = enums
	f.Consts = consts
	f.Typedefs = typedefs
	f.MessageDefinitions = messageDefinitions
	f.ServiceDefinitions = serviceDefinitions
	f.StreamDefinitions = streamDefinitions

	return f, nil
}
