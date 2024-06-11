package go_gen

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateImportsGo(t *testing.T) {

	str, err := generateImportsGoCode("github.com/test", nil, true, true, true)
	require.Nil(t, err)
	fmt.Println(str)

	fmt.Println("done")
}
