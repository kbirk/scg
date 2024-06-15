package parse

import "fmt"

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
			Message: fmt.Sprintf("package %s not found", pkg.Name),
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

func cloneTraversed(traversed map[string]bool) map[string]bool {
	clone := make(map[string]bool)
	for k, v := range traversed {
		clone[k] = v
	}
	return clone
}

func resolveFileDependencies(traversed map[string]bool, file *File) *ParsingError {

	for _, dep := range file.CustomTypeDependencies {

		_, ok := traversed[dep.File.Name]
		if ok {
			return &ParsingError{
				Message: fmt.Sprintf("circular dependency detected between files %s and %s", file.Name, dep.File.Name),
				Token:   nil,
			}
		}

		traversed[dep.File.Name] = true

		perr := resolveFileDependencies(cloneTraversed(traversed), dep.File)
		if perr != nil {
			return perr
		}
	}

	return nil
}

func resolvePackageDependencies(traversed map[string]bool, parse *Parse, pkg *Package) *ParsingError {

	for _, dep := range pkg.PackageDependencies {

		_, ok := traversed[dep.PackageName]
		if ok {
			return &ParsingError{
				Message: fmt.Sprintf("circular dependency detected between packages %s and %s", pkg.Name, dep.PackageName),
				Token:   dep.Token,
			}
		}

		traversed[dep.PackageName] = true

		depPkg, ok := parse.Packages[dep.PackageName]
		if !ok {
			return &ParsingError{
				Message: fmt.Sprintf("package dependency %s not found", dep.PackageName),
				Token:   dep.Token,
			}
		}

		perr := resolvePackageDependencies(cloneTraversed(traversed), parse, depPkg)
		if perr != nil {
			return perr
		}
	}

	return nil
}

func resolveCustomDataType(traversed map[string]bool, parse *Parse, dataType *DataTypeDefinition) *ParsingError {
	pkg, ok := parse.Packages[dataType.CustomTypePackage]
	if !ok {
		return &ParsingError{
			Message: fmt.Sprintf("package %s not found", dataType.CustomTypePackage),
			Token:   dataType.Token,
		}
	}

	// if referencing a message, continue resolving for circular dependencies
	msg, ok := pkg.MessageDefinitions[dataType.CustomType]
	if ok {
		return resolveMessageTypes(cloneTraversed(traversed), parse, msg)
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

func resolveMessageTypes(traversed map[string]bool, parse *Parse, msg *MessageDefinition) *ParsingError {

	_, ok := traversed[msg.Name]
	if ok {
		return &ParsingError{
			Message: fmt.Sprintf("circular reference detected in definition %s", msg.Name),
			Token:   msg.Token,
		}
	}

	traversed[msg.Name] = true

	for _, field := range msg.Fields {

		switch field.DataTypeDefinition.Type {
		case DataTypeList:
			// list
			elem := field.DataTypeDefinition.GetElementType()

			if elem.Type == DataTypeCustom {
				// custom
				perr := resolveCustomDataType(traversed, parse, elem)
				if perr != nil {
					return perr
				}
			}

		case DataTypeMap:

			// map
			value := field.DataTypeDefinition.GetElementType()

			key := field.DataTypeDefinition.Key
			if key.Type == DataTypeComparableCustom {
				// custom
				perr := resolveCustomDataTypeComparable(parse, key)
				if perr != nil {
					return perr
				}
			}

			if value.Type == DataTypeCustom {
				// custom
				perr := resolveCustomDataType(traversed, parse, value)
				if perr != nil {
					return perr
				}
			}

		case DataTypeCustom:

			// custom
			perr := resolveCustomDataType(traversed, parse, field.DataTypeDefinition)
			if perr != nil {
				return perr
			}
		default:

			//plain
		}
	}

	return nil
}

func resolveDefinitions(parse *Parse) *ParsingError {

	// ensure all dependencies have the file set
	for _, file := range parse.Files {
		for _, dep := range file.CustomTypeDependencies {
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
	for _, file := range parse.Files {
		for _, messageDefinition := range file.MessageDefinitions {
			perr := resolveMessageTypes(map[string]bool{}, parse, messageDefinition)
			if perr != nil {
				perr.Filename = file.Name
				perr.Content = file.Content
				return perr
			}
		}
		for _, serviceDefinition := range file.ServiceDefinitions {
			for _, method := range serviceDefinition.Methods {
				perr := resolveServiceArgumentType(parse, method, method.Argument)
				if perr != nil {
					perr.Filename = file.Name
					perr.Content = file.Content
					return perr
				}

				perr = resolveServiceArgumentType(parse, method, method.Return)
				if perr != nil {
					perr.Filename = file.Name
					perr.Content = file.Content
					return perr
				}
			}
		}
	}

	// ensure file dependencies are not cyclic
	for _, file := range parse.Files {
		perr := resolveFileDependencies(map[string]bool{}, file)
		if perr != nil {
			perr.Filename = file.Name
			perr.Content = file.Content
			return perr
		}
	}

	// ensure package dependencies are not cyclic
	for _, pkg := range parse.Packages {
		perr := resolvePackageDependencies(map[string]bool{}, parse, pkg)
		if perr != nil {
			return perr
		}
	}

	// for const declarations, resolve and inject the underlying types
	for _, pkg := range parse.Packages {
		for _, constDecl := range pkg.Consts {
			if constDecl.DataTypeDefinition.Type == DataTypeComparableCustom {
				underlying, ok := pkg.Typedefs[constDecl.DataTypeDefinition.CustomType]
				if !ok {
					return &ParsingError{
						Message:  fmt.Sprintf("typedef %s not found", constDecl.DataTypeDefinition.CustomType),
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

	return nil
}
