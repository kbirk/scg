package parse

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var (
	serviceRegex       = regexp.MustCompile(`(?s)service\s+([a-zA-Z][a-zA-Z_0-9]*)\s*{(.*?)}`)
	serviceMethodRegex = regexp.MustCompile(`^rpc\s+([a-zA-Z][a-zA-Z_0-9]*)\s*\(\s*((?:[a-zA-Z][a-zA-Z_0-9]*)(?:\.[a-zA-Z][a-zA-Z_0-9]*)*)\s*\)\s*returns\s*\(\s*((?:[a-zA-Z][a-zA-Z_0-9]*)(?:\.[a-zA-Z][a-zA-Z_0-9]*)*)\s*\)\s*;\s*$`)
)

type ServiceMethodDefinition struct {
	Name     string
	Argument *DataTypeDefinition
	Return   *DataTypeDefinition
	Token    *Token
}

type ServiceDefinition struct {
	Name    string
	Methods map[string]*ServiceMethodDefinition
	File    *File
	Token   *Token
}

func tokenizeServiceMethods(input *Token) ([]*Token, *ParsingError) {
	res := []*Token{}

	startLine := input.LineStart
	startChar := input.LineStartCharacterPosition

	line := input.LineStart
	character := input.LineStartCharacterPosition

	methodContent := ""
	hasFoundNonWhitespace := false

	for _, c := range input.Content {

		if !unicode.IsSpace(c) && !hasFoundNonWhitespace {
			hasFoundNonWhitespace = true
			startLine = line
			startChar = character
		}

		if c == '\n' {
			line++
			character = 0
		} else {
			character++
		}

		if hasFoundNonWhitespace {
			methodContent += string(c)
		}

		if c == ';' {
			res = append(res, &Token{
				Type:                       ServiceMethodTokenType,
				Content:                    methodContent,
				LineStart:                  startLine,
				LineEnd:                    line,
				LineStartCharacterPosition: startChar,
				LineEndCharacterPosition:   character,
			})
			methodContent = ""
			startLine = line
			startChar = character
			hasFoundNonWhitespace = false
		}
	}

	if strings.TrimSpace(methodContent) != "" {
		return nil, &ParsingError{
			Message: "unexpected end of method declaration",
			Token:   input,
		}
	}

	return res, nil
}

func parseMethodDefinition(input *Token) (*ServiceMethodDefinition, *ParsingError) {

	match, perr := FindOneMatch(serviceMethodRegex, input)
	if perr != nil || len(match.Captures) != 3 {
		return nil, &ParsingError{
			Message: perr.Message,
			Token:   input,
		}
	}

	name := match.Captures[0].Content
	argumentType := match.Captures[1]
	returnType := match.Captures[2]

	argumentDefinition, perr := parseDataTypeDefinition(argumentType)
	if perr != nil {
		return nil, perr
	}
	if argumentDefinition.Type != DataTypeCustom {
		return nil, &ParsingError{
			Message: "invalid method argument type, must be a message type",
			Token:   input,
		}
	}

	returnDefinition, perr := parseDataTypeDefinition(returnType)
	if perr != nil {
		return nil, perr
	}
	if returnDefinition.Type != DataTypeCustom {
		return nil, &ParsingError{
			Message: "invalid method return type, must be a message type",
			Token:   input,
		}
	}

	return &ServiceMethodDefinition{
		Name:     name,
		Argument: argumentDefinition,
		Return:   returnDefinition,
		Token:    input,
	}, nil
}

func parseServiceDefinitions(tokens []*Token) (map[string]*ServiceDefinition, *ParsingError) {

	services := map[string]*ServiceDefinition{}

	for _, token := range tokens {
		if token.Type != ServiceTokenType {
			continue
		}

		match, perr := FindOneMatch(serviceRegex, token)
		if perr != nil || len(match.Captures) != 2 {
			return nil, &ParsingError{
				Message: "invalid service definition",
				Token:   token,
			}
		}

		service := &ServiceDefinition{
			Name:    match.Captures[0].Content,
			Methods: map[string]*ServiceMethodDefinition{},
			Token:   token,
		}

		methods, perr := tokenizeServiceMethods(match.Captures[1])
		if perr != nil {
			return nil, perr
		}

		for _, method := range methods {
			methodDefinition, perr := parseMethodDefinition(method)
			if perr != nil {
				return nil, perr
			}
			_, ok := service.Methods[methodDefinition.Name]
			if ok {
				return nil, &ParsingError{
					Message: fmt.Sprintf("duplicate method name %s", methodDefinition.Name),
					Token:   methodDefinition.Token,
				}
			}
			service.Methods[methodDefinition.Name] = methodDefinition
		}

		_, ok := services[service.Name]
		if ok {
			return nil, &ParsingError{
				Message: fmt.Sprintf("duplicate service name %s", service.Name),
				Token:   service.Token,
			}
		}
		services[service.Name] = service
	}

	return services, nil
}
