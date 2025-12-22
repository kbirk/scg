package parse

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var (
	streamRegex       = regexp.MustCompile(`(?s)stream\s+([a-zA-Z][a-zA-Z_0-9]*)\s*{(.*?)}`)
	streamMethodRegex = regexp.MustCompile(`^(client|server)\s+([a-zA-Z][a-zA-Z_0-9]*)\s*\(\s*((?:[a-zA-Z][a-zA-Z_0-9]*)(?:\.[a-zA-Z][a-zA-Z_0-9]*)*)\s*\)\s*returns\s*\(\s*((?:[a-zA-Z][a-zA-Z_0-9]*)(?:\.[a-zA-Z][a-zA-Z_0-9]*)*)\s*\)\s*;\s*$`)
)

type StreamMethodDirection int

const (
	StreamMethodDirectionClient StreamMethodDirection = iota
	StreamMethodDirectionServer
)

type StreamMethodDefinition struct {
	Name      string
	Direction StreamMethodDirection
	Argument  *DataTypeDefinition
	Return    *DataTypeDefinition
	Token     *Token
}

type StreamDefinition struct {
	Name    string
	Methods map[string]*StreamMethodDefinition
	File    *File
	Token   *Token
}

func tokenizeStreamMethods(input *Token) ([]*Token, *ParsingError) {
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
				Type:                       StreamMethodTokenType,
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

func parseStreamMethodDefinition(input *Token) (*StreamMethodDefinition, *ParsingError) {

	match, perr := FindOneMatch(streamMethodRegex, input)
	if perr != nil || len(match.Captures) != 4 {
		return nil, &ParsingError{
			Message: "invalid stream method definition",
			Token:   input,
		}
	}

	directionStr := match.Captures[0].Content
	name := match.Captures[1].Content
	argumentType := match.Captures[2]
	returnType := match.Captures[3]

	var direction StreamMethodDirection
	if directionStr == "client" {
		direction = StreamMethodDirectionClient
	} else if directionStr == "server" {
		direction = StreamMethodDirectionServer
	} else {
		return nil, &ParsingError{
			Message: "invalid stream method direction, must be 'client' or 'server'",
			Token:   match.Captures[0],
		}
	}

	argumentDefinition, perr := parseDataTypeDefinition(argumentType)
	if perr != nil {
		return nil, perr
	}
	if argumentDefinition.Type != DataTypeCustom {
		return nil, &ParsingError{
			Message: "invalid method argument type, must be a message type",
			Token:   argumentType,
		}
	}

	returnDefinition, perr := parseDataTypeDefinition(returnType)
	if perr != nil {
		return nil, perr
	}
	if returnDefinition.Type != DataTypeCustom {
		return nil, &ParsingError{
			Message: "invalid method return type, must be a message type",
			Token:   returnType,
		}
	}

	return &StreamMethodDefinition{
		Name:      name,
		Direction: direction,
		Argument:  argumentDefinition,
		Return:    returnDefinition,
		Token:     input,
	}, nil
}

func parseStreamDefinitions(tokens []*Token) (map[string]*StreamDefinition, *ParsingError) {

	streams := map[string]*StreamDefinition{}

	for _, token := range tokens {
		if token.Type != StreamTokenType {
			continue
		}

		match, perr := FindOneMatch(streamRegex, token)
		if perr != nil || len(match.Captures) != 2 {
			return nil, &ParsingError{
				Message: "invalid stream definition",
				Token:   token,
			}
		}

		stream := &StreamDefinition{
			Name:    match.Captures[0].Content,
			Methods: map[string]*StreamMethodDefinition{},
			Token:   token,
		}

		methods, perr := tokenizeStreamMethods(match.Captures[1])
		if perr != nil {
			return nil, perr
		}

		for _, method := range methods {
			methodDefinition, perr := parseStreamMethodDefinition(method)
			if perr != nil {
				return nil, perr
			}
			_, ok := stream.Methods[methodDefinition.Name]
			if ok {
				return nil, &ParsingError{
					Message: fmt.Sprintf("duplicate method name %s", methodDefinition.Name),
					Token:   methodDefinition.Token,
				}
			}
			stream.Methods[methodDefinition.Name] = methodDefinition
		}

		_, ok := streams[stream.Name]
		if ok {
			return nil, &ParsingError{
				Message: fmt.Sprintf("duplicate stream name %s", stream.Name),
				Token:   stream.Token,
			}
		}
		streams[stream.Name] = stream
	}

	return streams, nil
}
