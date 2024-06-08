package cpp_gen

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kbirk/scg/internal/parse"
)

func TestGenerateMessagesCpp(t *testing.T) {

	p, err := parse.NewParse("../../../test/data/sample/sample0")
	require.Nil(t, err)

	for _, pkg := range p.Packages {
		for _, msg := range pkg.MessageDefinitions {
			str, err := generateMessageCppCode(msg)
			require.Nil(t, err)

			fmt.Println(str)
		}
	}

	fmt.Println("done")
}
