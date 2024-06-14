package go_gen

import (
	"fmt"
	"testing"

	"github.com/kbirk/scg/internal/parse"
	"github.com/stretchr/testify/require"
)

func TestGenerateImportsGo(t *testing.T) {

	parse, err := parse.NewParse("../../../test/files/input/sample/sample2")
	require.Nil(t, err)

	for _, file := range parse.Files {
		str, err := generateImportsGoCode("github.com/test", file)
		require.Nil(t, err)
		fmt.Println(str)
	}

	fmt.Println("done")
}
