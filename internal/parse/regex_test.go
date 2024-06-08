package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegexMatcher(t *testing.T) {
	content := `
		message TestMessageA {
			byte a = 1;
			uint8 b = 2;

		}

		message TestMessageB {
			uint16 c = 1;

			uint32 d = 2;
		}

		message TestMessageC
		{

			uint64 e = 1;
		}
	`
	tokens, err := tokenizeFile(content)
	require.Nil(t, err)

	assert.Equal(t, 3, len(tokens))

	match, err := FindOneMatch(messageRegex, tokens[0])
	require.Nil(t, err)
	assert.Equal(t, 2, len(match.Captures))

	for _, token := range match.Captures {
		slice := getContentByTokenRange(content, token.LineStart, token.LineEnd, token.LineStartCharacterPosition, token.LineEndCharacterPosition)
		assert.Equal(t, token.Content, slice)
	}

	match, err = FindOneMatch(messageRegex, tokens[1])
	require.Nil(t, err)
	assert.Equal(t, 2, len(match.Captures))

	for _, token := range match.Captures {
		slice := getContentByTokenRange(content, token.LineStart, token.LineEnd, token.LineStartCharacterPosition, token.LineEndCharacterPosition)
		assert.Equal(t, token.Content, slice)
	}

	match, err = FindOneMatch(messageRegex, tokens[2])
	require.Nil(t, err)
	assert.Equal(t, 2, len(match.Captures))

	for _, token := range match.Captures {
		slice := getContentByTokenRange(content, token.LineStart, token.LineEnd, token.LineStartCharacterPosition, token.LineEndCharacterPosition)
		assert.Equal(t, token.Content, slice)
	}

}
