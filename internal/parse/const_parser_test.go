package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConstTokenizer(t *testing.T) {

	content := `
		const uint32 MyUint32 = 32;

		const string MyString = "string";

		const float64 MyFloat64 = 64.0;
	`

	tokens, err := tokenizeFile(content)
	require.Nil(t, err)

	require.Equal(t, 3, len(tokens))

	for _, token := range tokens {
		match := getContentByTokenRange(content, token.LineStart, token.LineEnd, token.LineStartCharacterPosition, token.LineEndCharacterPosition)
		assert.Equal(t, token.Content, match)
	}
}

func TestConstParser(t *testing.T) {
	tokens, err := tokenizeFile(`
		const uint32 MyUint32 = 32;
		const string MyString = "string with spaces";
		const float64 MyFloat64 = 64.0;
	`)
	require.Nil(t, err)

	consts, err := parseConstDeclarations(tokens)
	require.Nil(t, err)

	require.Equal(t, 3, len(tokens))
	assert.Equal(t, "MyUint32", consts["MyUint32"].Name)
	assert.Equal(t, DataTypeComparableUInt32, consts["MyUint32"].DataTypeDefinition.Type)
	assert.Equal(t, "MyString", consts["MyString"].Name)
	assert.Equal(t, DataTypeComparableString, consts["MyString"].DataTypeDefinition.Type)
	assert.Equal(t, "MyFloat64", consts["MyFloat64"].Name)
	assert.Equal(t, DataTypeComparableFloat64, consts["MyFloat64"].DataTypeDefinition.Type)
}

func TestConstParserQuoteEscape(t *testing.T) {
	tokens, err := tokenizeFile(`
		const string MyString = "string with \"escaped quotes\"!";
	`)
	require.Nil(t, err)

	consts, err := parseConstDeclarations(tokens)
	require.Nil(t, err)

	require.Equal(t, 1, len(tokens))
	assert.Equal(t, "MyString", consts["MyString"].Name)
	assert.Equal(t, DataTypeComparableString, consts["MyString"].DataTypeDefinition.Type)
}

func TestConstParserQuoteEscapedEscapes(t *testing.T) {
	tokens, err := tokenizeFile(`
		const string MyString = "string with \\ adsfasdfasdf ";
	`)
	require.Nil(t, err)

	consts, err := parseConstDeclarations(tokens)
	require.Nil(t, err)

	require.Equal(t, 1, len(tokens))
	assert.Equal(t, "MyString", consts["MyString"].Name)
	assert.Equal(t, DataTypeComparableString, consts["MyString"].DataTypeDefinition.Type)
}

func TestConstParserStringMustBeQuoted(t *testing.T) {
	_, err := tokenizeFile(`
		const string MyString = "string and no end quote;
	`)
	require.NotNil(t, err)

	_, err = tokenizeFile(`
		const string MyString = "string and end quote escaped \";
	`)
	require.NotNil(t, err)

	_, err = tokenizeFile(`
		const string MyString = no quotes ;
	`)
	require.NotNil(t, err)
}
