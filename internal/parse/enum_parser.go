package parse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	enumRegex      = regexp.MustCompile(`(?s)enum\s+([a-zA-Z][a-zA-Z_0-9]*)\s*{(.*?)}`)
	enumValueRegex = regexp.MustCompile(`^(.+?)\s*=\s*(.+?)\s*;*$`)
	enumNameRegex  = regexp.MustCompile(`^[a-zA-Z][a-zA-Z_0-9]*$`)
)

type EnumValueDefinition struct {
	Name  string
	Index uint32
	Token *Token
}

type EnumDefinition struct {
	Name   string
	Values map[string]*EnumValueDefinition
	File   *File
	Token  *Token
}

func (m *EnumDefinition) ValuesByIndex() []*EnumValueDefinition {
	res := make([]*EnumValueDefinition, len(m.Values))
	for _, val := range m.Values {
		res[val.Index] = val
	}
	return res
}

func parseValueDefinition(input *Token) (*EnumValueDefinition, *ParsingError) {

	match, perr := FindOneMatch(enumValueRegex, input)
	if perr != nil {
		return nil, &ParsingError{
			Message: fmt.Sprintf("invalid value definition: `%s", input.Content),
			Token:   input,
		}
	}

	nameMatch := match.Captures[0]
	name := nameMatch.Content
	if !fieldNameRegex.MatchString(name) {
		return nil, &ParsingError{
			Message: fmt.Sprintf("invalid value name: %s", name),
			Token:   nameMatch,
		}
	}

	indexMatch := match.Captures[1]
	index, err := strconv.Atoi(indexMatch.Content)
	if err != nil {
		return nil, &ParsingError{
			Message: fmt.Sprintf("invalid value index `%s`", indexMatch.Content),
			Token:   indexMatch,
		}
	}

	return &EnumValueDefinition{
		Name:  name,
		Index: uint32(index),
		Token: input,
	}, nil
}

func tokenizeEnumValues(input *Token) ([]*Token, *ParsingError) {

	res := []*Token{}

	startLine := input.LineStart
	startChar := input.LineStartCharacterPosition

	line := input.LineStart
	character := input.LineStartCharacterPosition

	fieldContent := ""
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
			fieldContent += string(c)
		}

		if c == ';' {
			res = append(res, &Token{
				Type:                       EnumValueTokenType,
				Content:                    fieldContent,
				LineStart:                  startLine,
				LineEnd:                    line,
				LineStartCharacterPosition: startChar,
				LineEndCharacterPosition:   character,
			})
			fieldContent = ""
			startLine = line
			startChar = character
			hasFoundNonWhitespace = false
		}
	}

	if strings.TrimSpace(fieldContent) != "" {
		return nil, &ParsingError{
			Message: "unexpected end of enum declaration",
			Token:   input,
		}
	}

	return res, nil
}

func parseEnumDefinitions(tokens []*Token) (map[string]*EnumDefinition, *ParsingError) {

	enums := map[string]*EnumDefinition{}

	for _, token := range tokens {
		if token.Type != EnumTokenType {
			continue
		}

		match, perr := FindOneMatch(enumRegex, token)
		if perr != nil || len(match.Captures) != 2 {
			return nil, &ParsingError{
				Message: "invalid enum definition",
				Token:   token,
			}
		}

		enum := &EnumDefinition{
			Name:   match.Captures[0].Content,
			Values: map[string]*EnumValueDefinition{},
			Token:  token,
		}

		values, perr := tokenizeEnumValues(match.Captures[1])
		if perr != nil {
			return nil, perr
		}

		if len(values) == 0 {
			return nil, &ParsingError{
				Message: "enum has no values",
				Token:   token,
			}
		}

		for _, value := range values {
			valueDefinition, perr := parseValueDefinition(value)
			if perr != nil {
				return nil, perr
			}

			_, ok := enum.Values[valueDefinition.Name]
			if ok {
				return nil, &ParsingError{
					Message: fmt.Sprintf("duplicate value definition %s", valueDefinition.Name),
					Token:   value,
				}
			}
			enum.Values[valueDefinition.Name] = valueDefinition
		}

		// ensure enum indices are valid and sequential
		indices := make(map[uint32]bool)
		for _, value := range enum.Values {
			// track indices
			_, ok := indices[value.Index]
			if ok {
				return nil, &ParsingError{
					Message: fmt.Sprintf("duplicate index %d in definition %s", value.Index, enum.Name),
					Token:   value.Token,
				}
			}
			indices[value.Index] = true
		}
		for i := 0; i < len(enum.Values); i++ {
			_, ok := indices[uint32(i)]
			if !ok {
				return nil, &ParsingError{
					Message: fmt.Sprintf("missing index %d in enum definition %s", i, enum.Name),
					Token:   enum.Token,
				}
			}
		}

		_, ok := enums[enum.Name]
		if ok {
			return nil, &ParsingError{
				Message: fmt.Sprintf("duplicate enum definition %s", enum.Name),
				Token:   token,
			}
		}
		enums[enum.Name] = enum
	}

	return enums, nil
}
