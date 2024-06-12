package go_gen

import (
	"fmt"
	"testing"

	"github.com/kbirk/scg/internal/parse"
	"github.com/stretchr/testify/require"
)

func TestGenerateImportsGo(t *testing.T) {

	file := &parse.File{
		Typedefs: map[string]*parse.TypedefDeclaration{
			"Typedef1": {
				Name: "Typedef1",
			},
		},
		MessageDefinitions: map[string]*parse.MessageDefinition{
			"Message1": {
				Name: "Message1",
			},
		},
		ServiceDefinitions: map[string]*parse.ServiceDefinition{
			"Service1": {
				Name: "Service1",
			},
		},
	}

	str, err := generateImportsGoCode("github.com/test", file)
	require.Nil(t, err)
	fmt.Println(str)

	fmt.Println("done")
}
