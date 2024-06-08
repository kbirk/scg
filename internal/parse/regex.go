package parse

import (
	"regexp"
)

type RegexMatch struct {
	Match    *Token
	Captures []*Token
}

func createSubMatchToken(input *Token, startChar int, endChar int) (*Token, *ParsingError) {

	if startChar == -1 && endChar == -1 {
		// empty capture group
		return &Token{
			Content:                    "",
			LineStart:                  -1,
			LineEnd:                    -1,
			LineStartCharacterPosition: -1,
			LineEndCharacterPosition:   -1,
		}, nil
	}

	if startChar < 0 || startChar >= len(input.Content) {
		return nil, &ParsingError{
			Message: "internal parsing error: startChar out of bounds",
			Token:   input,
		}
	}

	if endChar < 0 || endChar > len(input.Content) {
		return nil, &ParsingError{
			Message: "internal parsing error: endChar out of bounds",
			Token:   input,
		}
	}

	res := &Token{
		Content:                    input.Content[startChar:endChar],
		LineStart:                  -1,
		LineEnd:                    -1,
		LineStartCharacterPosition: -1,
		LineEndCharacterPosition:   -1,
	}

	line := input.LineStart
	character := input.LineStartCharacterPosition

	count := 0
	for _, c := range input.Content {

		if count == startChar {
			res.LineStart = line
			res.LineStartCharacterPosition = character
		}

		if count == endChar {
			res.LineEnd = line
			res.LineEndCharacterPosition = character
		}

		if c == '\n' {
			line++
			character = 0
		} else {
			character++
		}

		count++
	}

	if count == endChar {
		res.LineEnd = line
		res.LineEndCharacterPosition = character
	}

	if res.LineStart == -1 {
		return res, &ParsingError{
			Message: "internal parsing error: could not find start line",
			Token:   input,
		}
	}

	if res.LineEnd == -1 {
		return res, &ParsingError{
			Message: "internal parsing error: could not find end line",
			Token:   input,
		}
	}

	if res.LineStartCharacterPosition == -1 {
		return res, &ParsingError{
			Message: "internal parsing error: could not find start line character position",
			Token:   input,
		}
	}

	if res.LineEndCharacterPosition == -1 {
		return res, &ParsingError{
			Message: "internal parsing error: could not find end line character position",
			Token:   input,
		}
	}

	return res, nil
}

func FindOneMatch(re *regexp.Regexp, input *Token) (*RegexMatch, *ParsingError) {
	indices := re.FindAllStringSubmatchIndex(input.Content, -1)

	if len(indices) == 0 {
		return nil, &ParsingError{
			Message: "could not find any matches",
			Token:   input,
		}
	}

	if len(indices) > 1 {
		return nil, &ParsingError{
			Message: "found multiple matches",
			Token:   input,
		}
	}

	matchIndices := indices[0]

	tokens := make([]*Token, len(matchIndices)/2)
	for j := 0; j < len(matchIndices); j += 2 {

		charStart := matchIndices[j]
		charEnd := matchIndices[j+1]

		token, perr := createSubMatchToken(input, charStart, charEnd)
		if perr != nil {
			return nil, perr
		}

		tokens[j/2] = token
	}

	captures := []*Token{}
	if len(tokens) > 1 {
		captures = tokens[1:]
	}

	return &RegexMatch{
		Match:    tokens[0],
		Captures: captures,
	}, nil
}

func FindOneOrNoMatch(re *regexp.Regexp, input *Token) (*RegexMatch, *ParsingError) {
	indices := re.FindAllStringSubmatchIndex(input.Content, -1)

	if len(indices) == 0 {
		return nil, nil
	}

	if len(indices) > 1 {
		return nil, &ParsingError{
			Message: "found multiple matches",
			Token:   input,
		}
	}

	matchIndices := indices[0]

	tokens := make([]*Token, len(matchIndices)/2)
	for j := 0; j < len(matchIndices); j += 2 {

		charStart := matchIndices[j]
		charEnd := matchIndices[j+1]

		token, perr := createSubMatchToken(input, charStart, charEnd)
		if perr != nil {
			return nil, perr
		}

		tokens[j/2] = token
	}

	captures := []*Token{}
	if len(tokens) > 1 {
		captures = tokens[1:]
	}

	return &RegexMatch{
		Match:    tokens[0],
		Captures: captures,
	}, nil
}
