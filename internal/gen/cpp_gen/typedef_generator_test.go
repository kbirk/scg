package cpp_gen

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kbirk/scg/internal/parse"
)

func TestGenerateTypedefsCpp(t *testing.T) {

	parse, err := parse.NewParse("../../../test/files/input/sample/sample2")
	require.Nil(t, err)

	for _, pkg := range parse.Packages {
		for _, typdef := range pkg.Typedefs {
			str, err := generateTypedefCppCode(typdef)
			require.Nil(t, err)

			fmt.Println(str)
		}
	}

}
