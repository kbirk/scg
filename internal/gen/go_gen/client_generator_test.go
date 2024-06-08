package go_gen

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kbirk/scg/internal/parse"
)

func TestGenerateClientGo(t *testing.T) {

	parse, err := parse.NewParse("../../../test/data/sample/sample0")
	require.Nil(t, err)

	for _, pkg := range parse.Packages {
		for _, svc := range pkg.ServiceDefinitions {
			str, err := generateClientGoCode(pkg, svc)
			require.Nil(t, err)

			fmt.Println(str)
		}
	}

	fmt.Println("done")
}
