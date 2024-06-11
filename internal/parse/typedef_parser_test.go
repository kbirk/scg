package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypedefTokenizer(t *testing.T) {

	content := `
		typedef TestA = uint32;

		typedef
		TestA = string;

		typedef TestB =
		float64
		;
	`

	tokens, err := tokenizeFile(content)
	require.Nil(t, err)

	require.Equal(t, 3, len(tokens))

	for _, token := range tokens {
		match := getContentByTokenRange(content, token.LineStart, token.LineEnd, token.LineStartCharacterPosition, token.LineEndCharacterPosition)
		assert.Equal(t, token.Content, match)
	}
}

func TestTypedefParser(t *testing.T) {
	tokens, err := tokenizeFile(`
		typedef TestA = uint32;
		typedef TestB = string;
		typedef TestC = float64;
	`)
	require.Nil(t, err)

	typdefs, err := parseTypedefDeclaration(tokens)
	require.Nil(t, err)

	require.Equal(t, 3, len(tokens))
	assert.Equal(t, "TestA", typdefs["TestA"].Name)
	assert.Equal(t, DataTypeComparableUInt32, typdefs["TestA"].DataTypeDefinition.Type)
	assert.Equal(t, "TestB", typdefs["TestB"].Name)
	assert.Equal(t, DataTypeComparableString, typdefs["TestB"].DataTypeDefinition.Type)
	assert.Equal(t, "TestC", typdefs["TestC"].Name)
	assert.Equal(t, DataTypeComparableFloat64, typdefs["TestC"].DataTypeDefinition.Type)
}
