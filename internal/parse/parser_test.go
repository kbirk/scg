package parse

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func replaceCommentWithWhitespace(line string) string {
	if i := strings.Index(line, "#"); i != -1 {
		// Replace everything after the '#' with spaces
		return line[:i] + strings.Repeat(" ", len(line)-i)
	}
	return line
}

func getContentByTokenRange(content string, lineStart int, lineEnd int, lineStartCharacterPos int, lineEndCharacterPos int) string {

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		line = replaceCommentWithWhitespace(line)
		if i != len(lines)-1 {
			lines[i] = line + "\n"
		} else {
			lines[i] = line
		}
	}

	res := ""
	for i := lineStart; i <= lineEnd; i++ {
		if lineStart == lineEnd && i == lineStart {
			res += lines[i][lineStartCharacterPos:lineEndCharacterPos]
			break
		} else {
			if i == lineStart {
				res += lines[i][lineStartCharacterPos:]
			} else if i == lineEnd {
				res += lines[i][:lineEndCharacterPos]
			} else {
				res += lines[i]
			}
		}
	}
	return res
}

func TestFileTokenizerPretty(t *testing.T) {

	content := `
		package test.custom;

		# my custom type
		message CustomType {
			byte a = 1;
			uint8 b = 2;
			uint16 c = 3;
		}

		# another type
		message OtherType {
			[]string m = 1;
			map[string]string n = 2;
			# some field comment
			[]map[string][]string o = 3;
			CustomType p = 4;
			[]test.custom.CustomType q = 5;
		}

		# my service
		service Test {
			# do a thing
			rpc DoThingA ([]CustomType) returns (CustomType);
			# do another thing
			rpc DoThingB (string, map[string][]OtherType) returns (string);
		}
	`

	tokens, err := tokenizeFile(content)
	require.Nil(t, err)

	assert.Equal(t, 4, len(tokens))

	assert.Equal(t, PackageTokenType, tokens[0].Type)
	assert.Equal(t, MessageTokenType, tokens[1].Type)
	assert.Equal(t, MessageTokenType, tokens[2].Type)
	assert.Equal(t, ServiceTokenType, tokens[3].Type)

	for _, token := range tokens {
		match := getContentByTokenRange(content, token.LineStart, token.LineEnd, token.LineStartCharacterPosition, token.LineEndCharacterPosition)
		assert.Equal(t, token.Content, match)
	}
}

func TestFileTokenizerUgly(t *testing.T) {

	content := `
		 package   test.custom  ;

		 # some comment here

		message CustomType # my comment
		{
			byte a = 1;

			uint8 b = 2
			;
			uint16
			c = 3;

		}

		 message
		  OtherType # another comment
		  {
		[]string m = 1;
			map[string]string n = 2;
			[]map[string][]string o=3;  # what about this?
			CustomType p = 4;
			[]test.custom.CustomType q = 5;}

		service
		TestService{
			rpc DoThingA
			([]CustomType) returns (CustomType);
			rpc DoThingB
			(string, map[string][]OtherType) returns (string);}
	`

	tokens, err := tokenizeFile(content)
	require.Nil(t, err)

	assert.Equal(t, 4, len(tokens))
}

func TestFileTokenizerPackageNoSemiColonErr(t *testing.T) {

	content := `
		package test.custom

		message CustomType {
			byte a = 1;
		}

		service Test {
			rpc DoThingA ([]CustomType) returns (CustomType);
		}
	`
	_, err := tokenizeFile(content)
	assert.NotNil(t, err)
}

func TestFileTokenizerPackageNoNameErr(t *testing.T) {

	content := `
		package;

		message CustomType {
			byte a = 1;
		}
	`
	tokens, err := tokenizeFile(content)

	assert.Nil(t, tokens)
	assert.NotNil(t, err)

	content = `
		package ;

		message CustomType {
			byte a = 1;
		}
	`
	tokens, err = tokenizeFile(content)

	assert.Nil(t, tokens)
	assert.NotNil(t, err)
}

func TestFileTokenizerPackageInvalidNameErr(t *testing.T) {

	content := `
		package te-st.cus\tom;

		message CustomType {
			byte a = 1;
		}

		service Test {
			rpc DoThingA ([]CustomType) returns (CustomType);
		}
	`
	tokens, err := tokenizeFile(content)

	assert.Nil(t, tokens)
	assert.NotNil(t, err)
}

func TestFileTokenizerPackageDuplicateNameErr(t *testing.T) {

	content := `
		package name  another;

		message CustomType {
			byte a = 1;
		}

		service Test {
			rpc DoThingA ([]CustomType) returns (CustomType);
		}
	`
	tokens, err := tokenizeFile(content)

	assert.Nil(t, tokens)
	assert.NotNil(t, err)
}

func TestFileTokenizerMessageNoNameErr(t *testing.T) {

	content := `
		package test.custom;

		message {
			byte a = 1;
		}
	`
	tokens, err := tokenizeFile(content)

	assert.Nil(t, tokens)
	assert.NotNil(t, err)

	content = `
		package test.custom;

		message{
			byte a = 1;
		}
	`
	tokens, err = tokenizeFile(content)

	assert.Nil(t, tokens)
	assert.NotNil(t, err)
}

func TestFileTokenizerMessageMissingOpenBracketErr(t *testing.T) {

	content := `
		package test.custom;

		message CustomType
			byte a = 1;
		}
	`
	tokens, err := tokenizeFile(content)

	assert.Nil(t, tokens)
	assert.NotNil(t, err)
}

func TestFileTokenizerMessageMissingClosingBracketErr(t *testing.T) {

	content := `
		package test.custom;

		message CustomType {
			byte a = 1;
	`
	tokens, err := tokenizeFile(content)

	assert.Nil(t, tokens)
	assert.NotNil(t, err)
}
