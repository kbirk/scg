package parse

import (
	"fmt"
	"strings"
	"unicode"
)

var (
	blockScopedNamedDeclaration = map[string]TokenType{
		"service": ServiceTokenType,
		"message": MessageTokenType,
		"enum":    EnumTokenType,
	}
	packageDeclaration = map[string]TokenType{
		"package": PackageTokenType,
	}
	typedefDeclaration = map[string]TokenType{
		"typedef": TypedefTokenType,
	}
	constDeclaration = map[string]TokenType{
		"const": ConstTokenType,
	}
)

type ParseState struct {
	line         int
	character    int
	inComment    bool
	inWord       bool
	word         string
	tokens       []*Token
	currentToken *Token
}

type TokenParser interface {
	Consume(c rune, state *ParseState) (TokenParser, error)
}

type KeywordTokenParser struct {
}

type BlockScopedNamedDeclarationParser struct {
	expectingName              bool
	expectingOpeningBracket    bool
	consumeUntilClosingBracket bool
}

func (p *BlockScopedNamedDeclarationParser) Consume(c rune, state *ParseState) (TokenParser, error) {

	if c == '\n' {
		state.line++
		state.character = 0
		state.inComment = false
	} else {
		state.character++
	}

	breakWord := false
	if unicode.IsSpace(c) {
		// whitespace
		state.currentToken.Content += string(c)
		breakWord = true

	} else if c == commentDelimiter || state.inComment {
		// comment
		state.inComment = true
		breakWord = true
		state.currentToken.Content += " " // replace comment with whitespace

	} else if !state.inComment {
		// character

		state.currentToken.Content += string(c)

		if c == '{' {
			if !p.expectingOpeningBracket && !p.expectingName {
				return nil, fmt.Errorf("unexpected opening bracket")
			}

			if p.expectingName && state.word == "" {
				return nil, fmt.Errorf("missing name")
			}

			p.expectingName = false
			p.expectingOpeningBracket = false
			p.consumeUntilClosingBracket = true

			state.word = ""
			state.inWord = false

		} else if c == '}' {
			if !p.consumeUntilClosingBracket {
				return nil, fmt.Errorf("unexpected closing bracket")
			}
			state.currentToken.LineEnd = state.line
			state.currentToken.LineEndCharacterPosition = state.character

			state.tokens = append(state.tokens, state.currentToken)
			state.currentToken = nil

			state.word = ""
			state.inWord = false

			// done
			return &KeywordTokenParser{}, nil

		} else {
			if p.expectingOpeningBracket {
				return nil, fmt.Errorf("unexpected character `%s`", string(c))
			}
			state.word += string(c)
			state.inWord = true
		}
	}

	if breakWord && state.inWord {
		if p.expectingName {
			if state.word == "" {
				return nil, fmt.Errorf("missing name")
			}
			p.expectingName = false
			p.expectingOpeningBracket = true

			state.word = ""
			state.inWord = false
		}
	}

	return p, nil
}

type PackageDeclarationParser struct {
	expectingPackageName bool
	expectingSemiColon   bool
}

func (p *PackageDeclarationParser) Consume(c rune, state *ParseState) (TokenParser, error) {
	if c == '\n' {
		state.line++
		state.character = 0
		state.inComment = false
	} else {
		state.character++
	}
	breakWord := false
	if unicode.IsSpace(c) {
		// whitespace
		state.currentToken.Content += string(c)
		breakWord = true

	} else if c == commentDelimiter || state.inComment {
		// comment
		state.inComment = true
		breakWord = true
		state.currentToken.Content += " " // replace comment with whitespace

	} else {
		// character
		state.currentToken.Content += string(c)

		if c == ';' {
			if !p.expectingSemiColon && !state.inWord {
				return nil, fmt.Errorf("unexpected semi-colon")
			}

			state.currentToken.LineEnd = state.line
			state.currentToken.LineEndCharacterPosition = state.character

			state.tokens = append(state.tokens, state.currentToken)
			state.currentToken = nil

			state.word = ""
			state.inWord = false

			// done
			return &KeywordTokenParser{}, nil
		} else {
			if p.expectingSemiColon {
				return nil, fmt.Errorf("unexpected character `%s`", string(c))
			}
			state.word += string(c)
			state.inWord = true
		}
	}

	if breakWord && state.inWord {
		if p.expectingPackageName {
			p.expectingPackageName = false
			p.expectingSemiColon = true
			state.word = ""
			state.inWord = false
		} else {
			return nil, fmt.Errorf("unexpected keyword `%s`", state.word)
		}
	}

	return p, nil
}

type TypedefDeclarationParser struct {
	expectingTypedefName bool
	expectingAssignment  bool
	expectingTypedefType bool
	expectingSemiColon   bool
}

func (p *TypedefDeclarationParser) Consume(c rune, state *ParseState) (TokenParser, error) {
	if c == '\n' {
		state.line++
		state.character = 0
		state.inComment = false
	} else {
		state.character++
	}
	breakWord := false
	if unicode.IsSpace(c) {
		// whitespace
		state.currentToken.Content += string(c)
		breakWord = true

	} else if c == commentDelimiter || state.inComment {
		// comment
		state.inComment = true
		breakWord = true
		state.currentToken.Content += " " // replace comment with whitespace

	} else {
		// character
		state.currentToken.Content += string(c)

		if c == '=' {
			if !p.expectingAssignment && !state.inWord {
				return nil, fmt.Errorf("unexpected '=' operator")
			}

			p.expectingAssignment = false
			p.expectingTypedefType = true

		} else if c == ';' {
			if !p.expectingSemiColon && !state.inWord {
				return nil, fmt.Errorf("unexpected semi-colon")
			}

			state.currentToken.LineEnd = state.line
			state.currentToken.LineEndCharacterPosition = state.character

			state.tokens = append(state.tokens, state.currentToken)
			state.currentToken = nil

			state.word = ""
			state.inWord = false

			// done
			return &KeywordTokenParser{}, nil
		} else {
			if p.expectingAssignment {
				return nil, fmt.Errorf("unexpected character `%s`", string(c))
			}
			if p.expectingSemiColon {
				return nil, fmt.Errorf("unexpected character `%s`", string(c))
			}
			state.word += string(c)
			state.inWord = true
		}
	}

	if breakWord && state.inWord {
		if p.expectingTypedefName {
			p.expectingTypedefName = false
			p.expectingAssignment = true
			state.word = ""
			state.inWord = false
		} else if p.expectingTypedefType {
			p.expectingTypedefType = false
			p.expectingSemiColon = true
			state.word = ""
			state.inWord = false
		} else {
			return nil, fmt.Errorf("unexpected keyword `%s`", state.word)
		}
	}

	return p, nil
}

type ConstDeclarationParser struct {
	expectingConstType  bool
	expectingConstName  bool
	expectingAssignment bool
	expectingConstValue bool
	insideAQuote        bool
	expectingSemiColon  bool
}

func (p *ConstDeclarationParser) isThisQuoteEscaped(state *ParseState) bool {
	// The quote has already been appended to the token content. It is escaped
	// iff it is preceded by an odd number of consecutive backslashes.
	content := state.currentToken.Content
	numBackslashes := 0
	for i := len(content) - 2; i >= 0 && content[i] == '\\'; i-- {
		numBackslashes++
	}
	return numBackslashes%2 == 1
}

func (p *ConstDeclarationParser) Consume(c rune, state *ParseState) (TokenParser, error) {
	if c == '\n' {
		state.line++
		state.character = 0
		state.inComment = false
	} else {
		state.character++
	}

	if p.insideAQuote {
		// Inside a string literal everything is literal content: a comment
		// delimiter or a semi-colon does not terminate anything.
		if c == '\n' {
			return nil, fmt.Errorf("unterminated string literal")
		}
		state.currentToken.Content += string(c)
		if c == '"' && !p.isThisQuoteEscaped(state) {
			p.insideAQuote = false
			// treat the closing quote as the end of the value word
			state.word += string(c)
			state.inWord = true
		}
		return p, nil
	}

	breakWord := false
	if unicode.IsSpace(c) {
		// whitespace
		state.currentToken.Content += string(c)
		breakWord = true

	} else if c == commentDelimiter || state.inComment {
		// comment
		state.inComment = true
		breakWord = true
		state.currentToken.Content += " " // replace comment with whitespace

	} else {
		// character
		state.currentToken.Content += string(c)

		if c == '"' {
			if !p.expectingConstValue {
				return nil, fmt.Errorf("unexpected `\"`")
			}

			if !p.isThisQuoteEscaped(state) {
				if !p.insideAQuote {
					p.insideAQuote = true
				} else {
					p.insideAQuote = false

					// treat this as a word break
					state.word += string(c)
					state.inWord = true
				}
			}

		} else if c == '=' && !p.insideAQuote {
			if !p.expectingAssignment && !state.inWord {
				return nil, fmt.Errorf("unexpected '=' operator")
			}

			p.expectingAssignment = false
			p.expectingConstValue = true

		} else if c == ';' && !p.insideAQuote {
			if !p.expectingSemiColon && !state.inWord {
				return nil, fmt.Errorf("unexpected semi-colon")
			}

			state.currentToken.LineEnd = state.line
			state.currentToken.LineEndCharacterPosition = state.character

			state.tokens = append(state.tokens, state.currentToken)
			state.currentToken = nil

			state.word = ""
			state.inWord = false

			// done
			return &KeywordTokenParser{}, nil
		} else if !p.insideAQuote {
			if p.expectingAssignment {
				return nil, fmt.Errorf("unexpected character `%s`", string(c))
			}
			if p.expectingSemiColon {
				return nil, fmt.Errorf("unexpected character `%s`", string(c))
			}
			state.word += string(c)
			state.inWord = true
		}
	}

	if breakWord && state.inWord {
		if p.expectingConstType {
			p.expectingConstType = false
			p.expectingConstName = true
			state.word = ""
			state.inWord = false
		} else if p.expectingConstName {
			p.expectingConstType = false
			p.expectingAssignment = true
			state.word = ""
			state.inWord = false
		} else if p.expectingConstValue {
			p.expectingConstValue = false
			p.expectingSemiColon = true
			state.word = ""
			state.inWord = false
		} else {
			return nil, fmt.Errorf("unexpected keyword `%s`", state.word)
		}
	}

	return p, nil
}

func (p *KeywordTokenParser) Consume(c rune, state *ParseState) (TokenParser, error) {

	if state.currentToken == nil {
		state.currentToken = &Token{
			Content: "",
		}
	}

	if c == '\n' {
		state.line++
		state.character = 0
		state.inComment = false
	} else {
		state.character++
	}

	breakWord := false
	if unicode.IsSpace(c) {
		// whitespace
		breakWord = true
		if state.inWord {
			// only append to token if we are in a word
			state.currentToken.Content += string(c)
		}

	} else if c == commentDelimiter || state.inComment {
		// comment start
		state.inComment = true
		breakWord = true
		// don't replace comment with whitespace, since we are not "inside" a token

	} else {
		// character
		state.currentToken.Content += string(c)

		if !state.inWord {
			// mark start of word
			state.currentToken.LineStart = state.line
			state.currentToken.LineStartCharacterPosition = state.character - 1
		}

		state.word += string(c)
		state.inWord = true
	}

	if breakWord && state.inWord {

		var tokenType TokenType
		var ok bool
		if tokenType, ok = blockScopedNamedDeclaration[state.word]; ok {
			state.word = ""
			state.inWord = false

			state.currentToken.Type = tokenType
			return &BlockScopedNamedDeclarationParser{
				expectingName: true,
			}, nil
		}

		if tokenType, ok = packageDeclaration[state.word]; ok {
			state.word = ""
			state.inWord = false
			state.currentToken.Type = tokenType
			return &PackageDeclarationParser{
				expectingPackageName: true,
			}, nil
		}

		if tokenType, ok = typedefDeclaration[state.word]; ok {
			state.word = ""
			state.inWord = false
			state.currentToken.Type = tokenType
			return &TypedefDeclarationParser{
				expectingTypedefName: true,
			}, nil
		}

		if tokenType, ok = constDeclaration[state.word]; ok {
			state.word = ""
			state.inWord = false
			state.currentToken.Type = tokenType
			return &ConstDeclarationParser{
				expectingConstType: true,
			}, nil
		}

		return nil, fmt.Errorf("unexpected keyword `%s`", state.word)
	}

	return p, nil
}

// getErrorTokenBasedOnState returns a zero-width token at the tokenizer's
// current position, used to point error messages at the offending character.
func getErrorTokenBasedOnState(content string, state *ParseState) *Token {
	// state.character has already been advanced past the current character,
	// so the current character's 0-based column is one less.
	col := state.character - 1
	if col < 0 {
		col = 0
	}
	return &Token{
		Content:                    "",
		LineStart:                  state.line,
		LineEnd:                    state.line,
		LineStartCharacterPosition: col,
		LineEndCharacterPosition:   col,
	}
}

func tokenizeFile(content string) ([]*Token, *ParsingError) {

	state := &ParseState{
		line:         0,
		character:    0,
		inComment:    false,
		inWord:       false,
		word:         "",
		tokens:       make([]*Token, 0),
		currentToken: nil,
	}

	var err error
	var parser TokenParser

	parser = &KeywordTokenParser{}

	for _, c := range content {
		parser, err = parser.Consume(c, state)
		if err != nil {

			return nil, &ParsingError{
				Message: err.Error(),
				Token:   getErrorTokenBasedOnState(content, state),
			}
		}
	}

	if state.currentToken != nil {
		_, keyword := parser.(*KeywordTokenParser)
		if keyword {
			// parsing an unrecognized token
			trimmed := strings.TrimSpace(state.currentToken.Content)
			if trimmed != "" {
				return nil, &ParsingError{
					Message: fmt.Sprintf("unexpected keyword `%s`", trimmed),
					Token:   getErrorTokenBasedOnState(content, state),
				}
			}
		} else {
			// mid-parse of a recognized token
			return nil, &ParsingError{
				Message: "unexpected end of file",
				Token:   getErrorTokenBasedOnState(content, state),
			}
		}

	}

	return state.tokens, nil
}
