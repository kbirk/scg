package parse

import (
	"fmt"
	"regexp"
)

var (
	typedefRegex = regexp.MustCompile(`typedef\s+([a-zA-Z][a-zA-Z_0-9]*)\s*=\s*(uint8|uint16|uint32|uint64|int8|int16|int32|int64|float32|float64|string|uuid)\s*;$`)
)

type TypedefDeclaration struct {
	Name               string
	DataTypeDefinition *DataTypeComparableDefinition
	File               *File
	Token              *Token
}

func parseTypedefDeclarations(tokens []*Token) (map[string]*TypedefDeclaration, *ParsingError) {

	typdefs := map[string]*TypedefDeclaration{}

	for _, token := range tokens {
		if token.Type != TypedefTokenType {
			continue
		}

		match, perr := FindOneMatch(typedefRegex, token)
		if perr != nil || len(match.Captures) != 2 {
			return nil, &ParsingError{
				Message: "invalid typedef definition",
				Token:   token,
			}
		}

		typ, perr := parseDataTypeComparableDefinition(match.Captures[1])
		if perr != nil {
			return nil, perr
		}

		if typ.Type == DataTypeComparableCustom {
			return nil, &ParsingError{
				Message: fmt.Sprintf("typedef cannot be %s, must be plain comparable type", typ.CustomType),
				Token:   token,
			}
		}

		typdef := &TypedefDeclaration{
			Name:               match.Captures[0].Content,
			DataTypeDefinition: typ,
			Token:              token,
		}

		_, ok := typdefs[typdef.Name]
		if ok {
			return nil, &ParsingError{
				Message: fmt.Sprintf("duplicate typedef definition %s", typdef.Name),
				Token:   token,
			}
		}
		typdefs[typdef.Name] = typdef
	}

	return typdefs, nil
}
