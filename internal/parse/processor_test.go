package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessInputPattern(t *testing.T) {

	parse, err := NewParse("../../test/data/sample/sample0")
	require.Nil(t, err)

	assert.Equal(t, 2, len(parse.Packages))
	assert.Equal(t, 1, len(parse.Packages["sample.name"].ServiceDefinitions))
	assert.Equal(t, 4, len(parse.Packages["sample.name"].MessageDefinitions))
}

func TestProcessCircularDeps(t *testing.T) {

	_, err := NewParse("../../test/data/circular")
	require.NotNil(t, err)
}
