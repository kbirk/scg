package cpp_gen

import (
	"fmt"
	"testing"

	"github.com/kbirk/scg/internal/parse"
	"github.com/stretchr/testify/require"
)

func TestGenerateFileCpp(t *testing.T) {

	parse, err := parse.NewParse("../../../test/files/input/sample/sample0")
	require.Nil(t, err)

	pkg, ok := parse.Packages["sample.name"]
	require.True(t, ok)

	file, ok := parse.Files["sample0.scg"]
	require.True(t, ok)

	str, err := generateFileCppCode(pkg, file)
	require.Nil(t, err)

	fmt.Println(str)
	fmt.Println("done")
}

func TestGenerateMultipleFileWithImportCpp(t *testing.T) {

	parse, err := parse.NewParse("../../../test/files/input/sample")
	require.Nil(t, err)

	pkg0, ok := parse.Packages["sample.name"]
	require.True(t, ok)

	file0, ok := parse.Files["sample0/sample0.scg"]
	require.True(t, ok)

	pkg1, ok := parse.Packages["another.sample"]
	require.True(t, ok)

	file1, ok := parse.Files["sample1/sample1.scg"]
	require.True(t, ok)

	str0, err := generateFileCppCode(pkg0, file0)
	require.Nil(t, err)

	str1, err := generateFileCppCode(pkg1, file1)
	require.Nil(t, err)

	fmt.Println(str0)
	fmt.Println(str1)
	fmt.Println("done")
}
