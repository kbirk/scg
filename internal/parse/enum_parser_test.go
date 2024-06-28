package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnumTokenizer(t *testing.T) {

	content := `
		enum MyEnum {
			Type = 0;
			Other = 1;
		}

		enum MyEnum2 { Type = 0; Other = 1; }

		enum
		MyEnum3
		{
		Type =
		0;
		Other
		= 1;
		}
	`

	tokens, err := tokenizeFile(content)
	require.Nil(t, err)

	require.Equal(t, 3, len(tokens))

	for _, token := range tokens {
		match := getContentByTokenRange(content, token.LineStart, token.LineEnd, token.LineStartCharacterPosition, token.LineEndCharacterPosition)
		assert.Equal(t, token.Content, match)
	}
}

func TestEnumParser(t *testing.T) {
	tokens, err := tokenizeFile(`
		enum EnumA {
			Type = 0;
			Other = 1;
		}

		enum EnumB {
			TypeA = 0;
			TypeB = 1;
			TypeC = 2;
			TypeD = 3;
		}
	`)
	require.Nil(t, err)

	enums, err := parseEnumDefinitions(tokens)
	require.Nil(t, err)

	require.Equal(t, 2, len(tokens))

	assert.Equal(t, "EnumA", enums["EnumA"].Name)
	assert.Equal(t, 2, len(enums["EnumA"].Values))
	assert.Equal(t, "Type", enums["EnumA"].Values["Type"].Name)
	assert.Equal(t, uint32(0), enums["EnumA"].Values["Type"].Index)
	assert.Equal(t, "Other", enums["EnumA"].Values["Other"].Name)
	assert.Equal(t, uint32(1), enums["EnumA"].Values["Other"].Index)

	assert.Equal(t, "EnumB", enums["EnumB"].Name)
	assert.Equal(t, 4, len(enums["EnumB"].Values))
	assert.Equal(t, "TypeA", enums["EnumB"].Values["TypeA"].Name)
	assert.Equal(t, uint32(0), enums["EnumB"].Values["TypeA"].Index)
	assert.Equal(t, "TypeB", enums["EnumB"].Values["TypeB"].Name)
	assert.Equal(t, uint32(1), enums["EnumB"].Values["TypeB"].Index)
	assert.Equal(t, "TypeC", enums["EnumB"].Values["TypeC"].Name)
	assert.Equal(t, uint32(2), enums["EnumB"].Values["TypeC"].Index)
	assert.Equal(t, "TypeD", enums["EnumB"].Values["TypeD"].Name)
	assert.Equal(t, uint32(3), enums["EnumB"].Values["TypeD"].Index)
}

func TestEnumParserWithValue(t *testing.T) {
	tokens, err := tokenizeFile(`
		enum EnumA {
			Type = 0;
			Other "other123" = 1;
		}

		enum EnumB {
			TypeA "type_a" = 0;
			TypeB = 1;
			TypeC "type_c" = 2;
			TypeD = 3;
		}
	`)
	require.Nil(t, err)

	enums, err := parseEnumDefinitions(tokens)
	require.Nil(t, err)

	require.Equal(t, 2, len(tokens))

	assert.Equal(t, "EnumA", enums["EnumA"].Name)
	assert.Equal(t, 2, len(enums["EnumA"].Values))
	assert.Equal(t, "Type", enums["EnumA"].Values["Type"].Name)
	assert.Equal(t, uint32(0), enums["EnumA"].Values["Type"].Index)
	assert.Equal(t, "Type", enums["EnumA"].Values["Type"].Value)
	assert.Equal(t, "Other", enums["EnumA"].Values["Other"].Name)
	assert.Equal(t, uint32(1), enums["EnumA"].Values["Other"].Index)
	assert.Equal(t, "other123", enums["EnumA"].Values["Other"].Value)

	assert.Equal(t, "EnumB", enums["EnumB"].Name)
	assert.Equal(t, 4, len(enums["EnumB"].Values))
	assert.Equal(t, "TypeA", enums["EnumB"].Values["TypeA"].Name)
	assert.Equal(t, uint32(0), enums["EnumB"].Values["TypeA"].Index)
	assert.Equal(t, "type_a", enums["EnumB"].Values["TypeA"].Value)
	assert.Equal(t, "TypeB", enums["EnumB"].Values["TypeB"].Name)
	assert.Equal(t, uint32(1), enums["EnumB"].Values["TypeB"].Index)
	assert.Equal(t, "TypeB", enums["EnumB"].Values["TypeB"].Value)
	assert.Equal(t, "TypeC", enums["EnumB"].Values["TypeC"].Name)
	assert.Equal(t, uint32(2), enums["EnumB"].Values["TypeC"].Index)
	assert.Equal(t, "type_c", enums["EnumB"].Values["TypeC"].Value)
	assert.Equal(t, "TypeD", enums["EnumB"].Values["TypeD"].Name)
	assert.Equal(t, uint32(3), enums["EnumB"].Values["TypeD"].Index)
	assert.Equal(t, "TypeD", enums["EnumB"].Values["TypeD"].Value)
}
