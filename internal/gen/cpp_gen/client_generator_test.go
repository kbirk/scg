package cpp_gen

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kbirk/scg/internal/parse"
)

func TestGenerateClientCpp(t *testing.T) {

	parse, err := parse.NewParse("../../../test/files/input/sample/sample0")
	require.Nil(t, err)

	for _, pkg := range parse.Packages {
		for _, svc := range pkg.ServiceDefinitions {
			str, err := generateClientCppCode(pkg, svc)
			require.Nil(t, err)

			fmt.Println(str)
		}
	}

	fmt.Println("done")
}
