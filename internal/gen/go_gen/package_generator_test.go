package go_gen

import (
	"fmt"
	"testing"

	"github.com/kbirk/scg/internal/parse"

	"github.com/stretchr/testify/require"
)

func TestGeneratePackageGo(t *testing.T) {

	parse, err := parse.NewParse("../../../test/files/input/sample/sample0")
	require.Nil(t, err)

	file, ok := parse.Files["sample0.scg"]
	require.True(t, ok)

	str, err := generatePackageGoCode(file.Package)
	require.Nil(t, err)

	fmt.Println(str)
	fmt.Println("done")
}
