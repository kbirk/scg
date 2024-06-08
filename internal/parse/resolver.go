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

func getElementType(field *MessageFieldDefinition, dataType *DataTypeDefinition) (*DataTypeDefinition, *ParsingError) {
	if dataType.Type == DataTypeList || dataType.Type == DataTypeMap {
		if dataType.SubType == nil {
			return nil, &ParsingError{
				Message: "list / map subtype not found",
				Token:   field.Token,
			}
		}
		return getElementType(field, dataType.SubType)
	}
	return dataType, nil
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

		_, ok := traversed[dep.Package.Name]
		if ok {
			return &ParsingError{
				Message: fmt.Sprintf("circular dependency detected between packages %s and %s", pkg.Name, dep.Package.Name),
				Token:   nil,
			}
		}

		traversed[dep.Package.Name] = true

		depPkg, ok := parse.Packages[dep.Package.Name]
		if !ok {
			return &ParsingError{
				Message: fmt.Sprintf("package dependency %s not found", dep.Package.Name),
				Token:   nil,
			}
		}

		perr := resolvePackageDependencies(cloneTraversed(traversed), parse, depPkg)
		if perr != nil {
			return perr
		}
	}

	return nil
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
		case DataTypeList, DataTypeMap:

			// list / map
			elem, perr := getElementType(field, field.DataTypeDefinition)
			if perr != nil {
				return perr
			}

			if elem.Type == DataTypeCustom {
				// custom
				referencedPackage, ok := parse.Packages[elem.CustomTypePackage]
				if !ok {
					return &ParsingError{
						Message: fmt.Sprintf("package %s not found", elem.CustomTypePackage),
						Token:   field.Token,
					}
				}
				referencedMessage, ok := referencedPackage.MessageDefinitions[elem.CustomType]
				if !ok {
					return &ParsingError{
						Message: fmt.Sprintf("type %s not found", elem.CustomType),
						Token:   field.Token,
					}
				}

				perr := resolveMessageTypes(cloneTraversed(traversed), parse, referencedMessage)
				if perr != nil {
					return perr
				}
			}

		case DataTypeCustom:

			// custom
			referencedPackage, ok := parse.Packages[field.DataTypeDefinition.CustomTypePackage]
			if !ok {
				return &ParsingError{
					Message: fmt.Sprintf("package %s not found", field.DataTypeDefinition.CustomTypePackage),
					Token:   field.Token,
				}
			}
			referencedMessage, ok := referencedPackage.MessageDefinitions[field.DataTypeDefinition.CustomType]
			if !ok {
				return &ParsingError{
					Message: fmt.Sprintf("type %s not found", field.DataTypeDefinition.CustomType),
					Token:   field.Token,
				}
			}
			perr := resolveMessageTypes(cloneTraversed(traversed), parse, referencedMessage)
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
			pkg, ok := parse.Packages[dep.Package.Name]
			if !ok {
				return &ParsingError{
					Message:  fmt.Sprintf("package dependency %s referenced in %s not found", dep.Package.Name, file.Name),
					Token:    nil,
					Filename: file.Name,
					Content:  file.Content,
				}
			}
			msg, ok := pkg.MessageDefinitions[dep.DataType.CustomType]
			if !ok {
				return &ParsingError{
					Message:  fmt.Sprintf("type %s referenced in %s not found in package %s", dep.DataType.CustomType, file.Name, pkg.Name),
					Token:    nil,
					Filename: file.Name,
					Content:  file.Content,
				}
			}
			// set file
			dep.File = msg.File
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

	return nil
}
