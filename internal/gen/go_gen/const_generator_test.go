package go_gen

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kbirk/scg/internal/parse"
)

func TestGenerateConstsGo(t *testing.T) {

	parse, err := parse.NewParse("../../../test/files/input/sample/sample2")
	require.Nil(t, err)

	for _, pkg := range parse.Packages {
		for _, c := range pkg.Consts {
			str, err := generateConstGoCode(c)
			require.Nil(t, err)

			fmt.Println(str)
		}
	}

}
