package gen_test

import (
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/kbirk/scg/internal/gen/cpp_gen"
	"github.com/kbirk/scg/internal/gen/go_gen"
	"github.com/kbirk/scg/internal/parse"
	"github.com/stretchr/testify/require"
)

// Golden-file tests: the generated Go and C++ for the repository's test
// schemas is compared with checked-in copies under testdata/golden. After an
// intended generator change, regenerate them with:
//
//	go test ./internal/gen/ -run TestGolden -update

var update = flag.Bool("update", false, "rewrite the golden files")

const repoRoot = "../.."

type goldenSchema struct {
	name        string
	input       string
	basePackage string
}

var goldenSchemas = []goldenSchema{
	{"pingpong", "test/scg/pingpong", "github.com/kbirk/scg/test/scg/generated/pingpong"},
	{"basic", "test/scg/basic", "github.com/kbirk/scg/test/scg/generated/basic"},
	{"sample", "test/scg/sample", "github.com/kbirk/scg/test/scg/generated/sample"},
	{"benchmark", "benchmark/scg/benchmark", "github.com/kbirk/scg/benchmark/scg/generated/benchmark"},
}

func readTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return out
	}
	require.NoError(t, filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		bs, err := os.ReadFile(path)
		out[filepath.ToSlash(rel)] = string(bs)
		return err
	}))
	return out
}

func generateBoth(t *testing.T, s goldenSchema) (goOut string, cppOut string) {
	t.Helper()
	p, err := parse.NewParse(filepath.Join(repoRoot, s.input))
	require.NoError(t, err)
	goOut = t.TempDir()
	require.NoError(t, go_gen.GenerateGoCode(s.basePackage, goOut, p))
	cppOut = t.TempDir()
	require.NoError(t, cpp_gen.GenerateCppCode("", cppOut, p))
	return goOut, cppOut
}

func compareTree(t *testing.T, label string, got map[string]string, goldenDir string) {
	t.Helper()
	if *update {
		require.NoError(t, os.RemoveAll(goldenDir))
		for name, content := range got {
			path := filepath.Join(goldenDir, filepath.FromSlash(name))
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
			require.NoError(t, os.WriteFile(path, []byte(content), 0644))
		}
		return
	}
	want := readTree(t, goldenDir)
	var names []string
	for name := range got {
		names = append(names, name)
	}
	for name := range want {
		if _, ok := got[name]; !ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		g, gok := got[name]
		w, wok := want[name]
		if !gok {
			t.Errorf("%s: %s is in the golden files but was not generated", label, name)
			continue
		}
		if !wok {
			t.Errorf("%s: %s was generated but has no golden file (run with -update)", label, name)
			continue
		}
		if g != w {
			t.Errorf("%s: %s differs from its golden file (run with -update if intended):\n%s", label, name, firstDiff(w, g))
		}
	}
}

func firstDiff(want string, got string) string {
	wl := strings.Split(want, "\n")
	gl := strings.Split(got, "\n")
	for i := 0; i < len(wl) || i < len(gl); i++ {
		var w, g string
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(gl) {
			g = gl[i]
		}
		if w != g {
			return "line " + strconv.Itoa(i+1) + ":\n  want: " + w + "\n  got:  " + g
		}
	}
	return ""
}

func TestGolden(t *testing.T) {
	for _, s := range goldenSchemas {
		goOut, cppOut := generateBoth(t, s)
		compareTree(t, s.name+" (go)", readTree(t, goOut), filepath.Join("testdata", "golden", "go", s.name))
		compareTree(t, s.name+" (cpp)", readTree(t, cppOut), filepath.Join("testdata", "golden", "cpp", s.name))
	}
}
