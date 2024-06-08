package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageParser(t *testing.T) {

	tokens, err := tokenizeFile(`
		package my.package;
	`)
	require.Nil(t, err)

	pkg, err := parsePackageDeclaration(tokens)
	require.Nil(t, err)

	assert.Equal(t, "my.package", pkg.Name)
}
