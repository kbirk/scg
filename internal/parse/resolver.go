package parse

import (
	"fmt"

	"github.com/kbirk/scg/internal/util"
)

func resolveServiceArgumentType(parse *Parse, method *ServiceMethodDefinition, dataType *DataTypeDefinition) *ParsingError {

	if dataType.Type != DataTypeCustom {
		return &ParsingError{
			Message: "service argument must be a Message type",
			Token:   method.Token,
		}
	}

	pkg, ok := parse.Packages[dataType.CustomTypePackage]
	if !ok {
		return &ParsingError{
			Message: fmt.Sprintf("package %s not found", dataType.CustomTypePackage),
			Token:   method.Token,
		}
	}
	_, ok = pkg.MessageDefinitions[dataType.CustomType]
	if !ok {
		return &ParsingError{
			Message: fmt.Sprintf("type %s not found", dataType.CustomType),
			Token:   method.Token,
		}
	}

	return nil
}

// visit states for the depth-first searches below. Each node is fully
// resolved at most once, so resolution is linear in the size of the graph
// (re-walking shared sub-graphs made it exponential for "diamond" shapes).
const (
	unvisited = iota
	visiting
	visited
)

func resolveFileDependencies(state map[string]int, file *File) *ParsingError {

	switch state[file.ID()] {
	case visiting:
		return &ParsingError{
			Message: fmt.Sprintf("circular dependency detected in file %s", file.Name),
			Token:   nil,
		}
	case visited:
		return nil
	}

	state[file.ID()] = visiting

	for _, dep := range file.GetFileDependencies() {
		perr := resolveFileDependencies(state, dep.File)
		if perr != nil {
			return perr
		}
	}

	state[file.ID()] = visited
	return nil
}

func resolvePackageDependencies(state map[string]int, parse *Parse, pkg *Package) *ParsingError {

	switch state[pkg.Name] {
	case visiting:
		// reported by the caller, which knows the edge that closed the cycle
		return nil
	case visited:
		return nil
	}

	state[pkg.Name] = visiting

	for _, depName := range sortedKeys(pkg.PackageDependencies) {
		dep := pkg.PackageDependencies[depName]

		if state[dep.PackageName] == visiting {
			return withFile(&ParsingError{
				Message: fmt.Sprintf("circular dependency detected between packages %s and %s", pkg.Name, dep.PackageName),
				Token:   dep.Token,
			}, dep.File)
		}

		depPkg, ok := parse.Packages[dep.PackageName]
		if !ok {
			return withFile(&ParsingError{
				Message: fmt.Sprintf("package dependency %s not found", dep.PackageName),
				Token:   dep.Token,
			}, dep.File)
		}

		perr := resolvePackageDependencies(state, parse, depPkg)
		if perr != nil {
			return perr
		}
	}

	state[pkg.Name] = visited
	return nil
}

func resolveCustomDataTypeComparable(parse *Parse, dataType *DataTypeComparableDefinition) *ParsingError {

	pkg, ok := parse.Packages[dataType.CustomTypePackage]
	if !ok {
		return &ParsingError{
			Message: fmt.Sprintf("package %s not found", dataType.CustomTypePackage),
			Token:   dataType.Token,
		}
	}
	_, ok = pkg.Typedefs[dataType.CustomType]
	if ok {
		return nil
	}
	_, ok = pkg.Enums[dataType.CustomType]
	if ok {
		return nil
	}

	return &ParsingError{
		Message: fmt.Sprintf("typedef %s not found", dataType.CustomType),
		Token:   dataType.Token,
	}
}

// messageResolver checks that every type referenced by a message exists and
// that there are no circular message references. Messages are keyed by their
// package-qualified name, so `pa.Foo` referencing `pb.Foo` is not mistaken
// for a cycle.
type messageResolver struct {
	parse *Parse
	state map[string]int
}

func messageKey(pkgName string, msgName string) string {
	return pkgName + "." + msgName
}

func withFile(perr *ParsingError, file *File) *ParsingError {
	if perr != nil && perr.Filename == "" && file != nil {
		perr.Filename = file.Name
		perr.Content = file.Content
	}
	return perr
}

func (r *messageResolver) resolveCustomDataType(dataType *DataTypeDefinition) *ParsingError {
	pkg, ok := r.parse.Packages[dataType.CustomTypePackage]
	if !ok {
		return &ParsingError{
			Message: fmt.Sprintf("package %s not found", dataType.CustomTypePackage),
			Token:   dataType.Token,
		}
	}

	// if referencing a message, continue resolving for circular dependencies
	msg, ok := pkg.MessageDefinitions[dataType.CustomType]
	if ok {
		return r.resolveMessage(msg)
	}

	// if a typedef, we are done
	_, ok = pkg.Typedefs[dataType.CustomType]
	if ok {
		return nil
	}

	// if a enum, we are done
	_, ok = pkg.Enums[dataType.CustomType]
	if ok {
		return nil
	}

	// otherwise return an error
	return &ParsingError{
		Message: fmt.Sprintf("type %s not found", dataType.CustomType),
		Token:   dataType.Token,
	}
}

func (r *messageResolver) resolveMessage(msg *MessageDefinition) *ParsingError {

	key := messageKey(msg.File.Package.Name, msg.Name)

	switch r.state[key] {
	case visiting:
		return withFile(&ParsingError{
			Message: fmt.Sprintf("circular reference detected in definition %s", msg.Name),
			Token:   msg.Token,
		}, msg.File)
	case visited:
		return nil
	}

	r.state[key] = visiting

	for _, field := range msg.FieldsByIndex() {

		switch field.DataTypeDefinition.Type {
		case DataTypeList:
			// list
			elem := field.DataTypeDefinition.GetElementType()

			if elem.Type == DataTypeCustom {
				// custom
				perr := r.resolveCustomDataType(elem)
				if perr != nil {
					return withFile(perr, msg.File)
				}
			}

		case DataTypeMap:

			// map
			value := field.DataTypeDefinition.GetElementType()

			key := field.DataTypeDefinition.Key
			if key.Type == DataTypeComparableCustom {
				// custom
				perr := resolveCustomDataTypeComparable(r.parse, key)
				if perr != nil {
					return withFile(perr, msg.File)
				}
			}

			if value.Type == DataTypeCustom {
				// custom
				perr := r.resolveCustomDataType(value)
				if perr != nil {
					return withFile(perr, msg.File)
				}
			}

		case DataTypeCustom:

			// custom
			perr := r.resolveCustomDataType(field.DataTypeDefinition)
			if perr != nil {
				return withFile(perr, msg.File)
			}
		default:

			//plain
		}
	}

	r.state[key] = visited
	return nil
}

// isZeroBitMessage reports whether a message encodes to zero bits: it has no
// fields, or only fields whose types are themselves zero-bit messages. Must
// only be called once message references are known to be acyclic.
func isZeroBitMessage(parse *Parse, msg *MessageDefinition, memo map[string]bool) bool {
	key := messageKey(msg.File.Package.Name, msg.Name)
	if res, ok := memo[key]; ok {
		return res
	}
	res := true
	for _, field := range msg.FieldsByIndex() {
		if !isZeroBitDataType(parse, field.DataTypeDefinition, memo) {
			res = false
			break
		}
	}
	memo[key] = res
	return res
}

func isZeroBitDataType(parse *Parse, dataType *DataTypeDefinition, memo map[string]bool) bool {
	if dataType.Type != DataTypeCustom {
		// every builtin, list and map encodes to at least one bit
		return false
	}
	pkg, ok := parse.Packages[dataType.CustomTypePackage]
	if !ok {
		return false
	}
	msg, ok := pkg.MessageDefinitions[dataType.CustomType]
	if !ok {
		// enums and typedefs encode to at least one bit
		return false
	}
	return isZeroBitMessage(parse, msg, memo)
}

// checkZeroBitLists rejects list<T> where T encodes to zero bits. A decoder
// cannot bound such a list by the bytes remaining, so a tiny frame could
// declare billions of elements. Maps cannot hit this: every key type
// (integers, strings, uuids, floats, enums, typedefs) is at least one bit.
func checkZeroBitLists(parse *Parse, dataType *DataTypeDefinition, memo map[string]bool) *ParsingError {
	switch dataType.Type {
	case DataTypeList:
		if isZeroBitDataType(parse, dataType.SubType, memo) {
			return &ParsingError{
				Message: fmt.Sprintf("invalid list element type `%s`: message %s encodes to zero bytes (it has no non-empty fields), so a list of it cannot be bounded when decoding; add a field to the message or use a count instead", dataType.SubType.ToString(), dataType.SubType.CustomType),
				Token:   dataType.Token,
			}
		}
		return checkZeroBitLists(parse, dataType.SubType, memo)
	case DataTypeMap:
		return checkZeroBitLists(parse, dataType.SubType, memo)
	}
	return nil
}

// hashRPCName hashes a qualified service / method name into its wire ID. It
// is a variable so tests can force collisions.
var hashRPCName = util.HashStringToUInt64

// assignRPCIDs assigns wire IDs to every service and method. IDs are the
// FNV-1a hash of the package-qualified names "pkg.Service" and
// "pkg.Service/Method", so identically named services in different packages
// do not collide. Collisions are checked across every package in the parse.
func assignRPCIDs(parse *Parse) *ParsingError {

	serviceIDs := map[uint64]string{}
	methodIDs := map[uint64]string{}

	for _, pkgName := range sortedKeys(parse.Packages) {
		pkg := parse.Packages[pkgName]
		for _, svcName := range sortedKeys(pkg.ServiceDefinitions) {
			svc := pkg.ServiceDefinitions[svcName]

			qualifiedService := QualifiedServiceName(pkg.Name, svc.Name)
			svc.ID = hashRPCName(qualifiedService)
			if existing, ok := serviceIDs[svc.ID]; ok {
				return withFile(&ParsingError{
					Message: fmt.Sprintf("service ID collision: %s and %s both hash to %d, rename one of them", qualifiedService, existing, svc.ID),
					Token:   svc.Token,
				}, svc.File)
			}
			serviceIDs[svc.ID] = qualifiedService

			for _, method := range svc.MethodsSortedByKey() {
				qualifiedMethod := QualifiedMethodName(pkg.Name, svc.Name, method.Name)
				method.ID = hashRPCName(qualifiedMethod)
				if existing, ok := methodIDs[method.ID]; ok {
					return withFile(&ParsingError{
						Message: fmt.Sprintf("method ID collision: %s and %s both hash to %d, rename one of them", qualifiedMethod, existing, method.ID),
						Token:   method.Token,
					}, svc.File)
				}
				methodIDs[method.ID] = qualifiedMethod
			}
		}
	}

	return nil
}

// QualifiedServiceName is the string hashed into a service's wire ID.
func QualifiedServiceName(pkgName string, serviceName string) string {
	return pkgName + "." + serviceName
}

// QualifiedMethodName is the string hashed into a method's wire ID.
func QualifiedMethodName(pkgName string, serviceName string, methodName string) string {
	return QualifiedServiceName(pkgName, serviceName) + "/" + methodName
}

func filesSortedByKey(parse *Parse) []*File {
	keys := sortedKeys(parse.Files)
	files := make([]*File, 0, len(keys))
	for _, k := range keys {
		files = append(files, parse.Files[k])
	}
	return files
}

func resolveDefinitions(parse *Parse) *ParsingError {

	files := filesSortedByKey(parse)

	// ensure all dependencies have the file set
	for _, file := range files {
		for _, depKey := range sortedKeys(file.CustomTypeDependencies) {
			dep := file.CustomTypeDependencies[depKey]
			pkg, ok := parse.Packages[dep.CustomTypePackage]
			if !ok {
				return &ParsingError{
					Message:  fmt.Sprintf("package dependency %s referenced in %s not found", dep.CustomTypePackage, file.Name),
					Token:    dep.Token,
					Filename: file.Name,
					Content:  file.Content,
				}
			}

			msg, ok := pkg.MessageDefinitions[dep.CustomTypeName]
			if ok {
				dep.File = msg.File
				continue
			}

			typdef, ok := pkg.Typedefs[dep.CustomTypeName]
			if ok {
				dep.File = typdef.File
				continue
			}

			enum, ok := pkg.Enums[dep.CustomTypeName]
			if ok {
				dep.File = enum.File
				continue
			}

			return &ParsingError{
				Message:  fmt.Sprintf("type %s referenced in %s not found in package %s", dep.CustomTypeName, file.Name, pkg.Name),
				Token:    dep.Token,
				Filename: file.Name,
				Content:  file.Content,
			}
		}
	}

	// ensure all types are defined and contain no circular references
	resolver := &messageResolver{
		parse: parse,
		state: map[string]int{},
	}
	for _, file := range files {
		for _, messageDefinition := range file.MessagesSortedByKey() {
			perr := resolver.resolveMessage(messageDefinition)
			if perr != nil {
				return withFile(perr, file)
			}
		}
		for _, serviceDefinition := range file.ServicesSortedByKey() {
			for _, method := range serviceDefinition.MethodsSortedByKey() {
				perr := resolveServiceArgumentType(parse, method, method.Argument)
				if perr != nil {
					return withFile(perr, file)
				}

				perr = resolveServiceArgumentType(parse, method, method.Return)
				if perr != nil {
					return withFile(perr, file)
				}
			}
		}
	}

	// reject lists of zero-bit messages
	zeroBitMemo := map[string]bool{}
	for _, file := range files {
		for _, msg := range file.MessagesSortedByKey() {
			for _, field := range msg.FieldsByIndex() {
				perr := checkZeroBitLists(parse, field.DataTypeDefinition, zeroBitMemo)
				if perr != nil {
					return withFile(perr, file)
				}
			}
		}
	}

	// ensure file dependencies are not cyclic
	fileState := map[string]int{}
	for _, file := range files {
		perr := resolveFileDependencies(fileState, file)
		if perr != nil {
			return withFile(perr, file)
		}
	}

	// ensure package dependencies are not cyclic
	pkgState := map[string]int{}
	for _, pkgName := range sortedKeys(parse.Packages) {
		perr := resolvePackageDependencies(pkgState, parse, parse.Packages[pkgName])
		if perr != nil {
			return perr
		}
	}

	// for const declarations, resolve and inject the underlying types
	for _, pkgName := range sortedKeys(parse.Packages) {
		pkg := parse.Packages[pkgName]
		for _, constName := range sortedKeys(pkg.Consts) {
			constDecl := pkg.Consts[constName]
			if constDecl.DataTypeDefinition.Type == DataTypeComparableCustom {
				var underlying *TypedefDeclaration
				typedefPkg, ok := parse.Packages[constDecl.DataTypeDefinition.CustomTypePackage]
				if ok {
					underlying, ok = typedefPkg.Typedefs[constDecl.DataTypeDefinition.CustomType]
				}
				if !ok {
					return &ParsingError{
						Message:  fmt.Sprintf("typedef %s not found", constDecl.DataTypeDefinition.ToString()),
						Token:    constDecl.Token,
						Filename: constDecl.File.Name,
						Content:  constDecl.File.Content,
					}
				}

				// validate the value again
				err := ValidateConstValues(underlying.DataTypeDefinition.Type, constDecl.Value)
				if err != nil {
					return &ParsingError{
						Message:  fmt.Sprintf("invalid value for const %s: %s", constDecl.Value, err.Error()),
						Token:    constDecl.Token,
						Filename: constDecl.File.Name,
						Content:  constDecl.File.Content,
					}
				}
				constDecl.UnderlyingDataTypeDefinition = underlying.DataTypeDefinition
			}
		}
	}

	// assign wire IDs to services and methods, rejecting collisions
	perr := assignRPCIDs(parse)
	if perr != nil {
		return perr
	}

	return nil
}
