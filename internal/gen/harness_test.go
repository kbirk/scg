package gen_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/kbirk/scg/internal/gen/cpp_gen"
	"github.com/kbirk/scg/internal/gen/go_gen"
	"github.com/kbirk/scg/internal/parse"
	"github.com/stretchr/testify/require"
)

// Helpers to compile and run generated code. These tests shell out to the go
// tool and a C++ compiler, so they are skipped with -short.

func absRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(repoRoot)
	require.NoError(t, err)
	return root
}

// goModule creates a temporary Go module that depends on this repository via
// a replace directive, so generated code compiles against the local runtime.
func goModule(t *testing.T, name string) string {
	t.Helper()
	if testing.Short() {
		t.Skip("compiles generated code; skipped with -short")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool not found")
	}
	root := absRepoRoot(t)
	dir := t.TempDir()
	gomod := fmt.Sprintf("module %s\n\ngo 1.23\n\nrequire github.com/kbirk/scg v0.0.0\n\nreplace github.com/kbirk/scg => %s\n", name, root)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644))
	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.sum"), sum, 0644))
	return dir
}

func runGo(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	// everything needed is in the module cache: never hit the network
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off", "GOTOOLCHAIN=local")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("go %s failed in %s: %v\n%s", strings.Join(args, " "), dir, err, out.String())
	}
	return out.String()
}

func generateGoInto(t *testing.T, input string, moduleDir string, subdir string, module string) {
	t.Helper()
	p, err := parse.NewParse(input)
	require.NoError(t, err)
	require.NoError(t, go_gen.GenerateGoCode(module+"/"+subdir, filepath.Join(moduleDir, subdir), p))
}

// cppCompiler returns the C++ compiler and include flags, or skips.
func cppCompiler(t *testing.T) (string, []string) {
	t.Helper()
	if testing.Short() {
		t.Skip("compiles generated code; skipped with -short")
	}
	var cxx string
	for _, c := range []string{os.Getenv("CXX"), "g++", "clang++", "c++"} {
		if c == "" {
			continue
		}
		if path, err := exec.LookPath(c); err == nil {
			cxx = path
			break
		}
	}
	if cxx == "" {
		t.Skip("no C++ compiler found")
	}
	root := absRepoRoot(t)
	thirdParty := filepath.Join(root, "third_party", "include")
	if _, err := os.Stat(filepath.Join(thirdParty, "nlohmann", "json.hpp")); err != nil {
		t.Skip("third_party/include/nlohmann/json.hpp not found")
	}
	return cxx, []string{"-I" + filepath.Join(root, "include"), "-isystem", thirdParty}
}

// compileHeaders checks that every generated header compiles on its own with
// the flags the C++ test build uses.
func compileHeaders(t *testing.T, outDir string) {
	t.Helper()
	cxx, includes := cppCompiler(t)
	var headers []string
	require.NoError(t, filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(path, ".h") {
			headers = append(headers, path)
		}
		return err
	}))
	sort.Strings(headers)
	require.NotEmpty(t, headers)

	var mu sync.Mutex
	var failures []string
	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup
	for _, h := range headers {
		wg.Add(1)
		go func(h string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			src := filepath.Join(t.TempDir(), "tu.cpp")
			if err := os.WriteFile(src, []byte(fmt.Sprintf("#include %q\n", h)), 0644); err != nil {
				panic(err)
			}
			args := append([]string{"-std=c++17", "-Wall", "-Wextra", "-Werror", "-Wpedantic", "-Wno-unused-parameter", "-fsyntax-only", "-I" + outDir}, includes...)
			args = append(args, src)
			out, err := exec.Command(cxx, args...).CombinedOutput()
			if err != nil {
				rel, _ := filepath.Rel(outDir, h)
				mu.Lock()
				failures = append(failures, fmt.Sprintf("%s:\n%s", rel, firstLines(string(out), 15)))
				mu.Unlock()
			}
		}(h)
	}
	wg.Wait()
	if len(failures) > 0 {
		sort.Strings(failures)
		t.Fatalf("generated headers failed to compile:\n%s", strings.Join(failures, "\n"))
	}
}

func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

func corpusCases(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join("testdata", "corpus"))
	require.NoError(t, err)
	var cases []string
	for _, e := range entries {
		if e.IsDir() {
			cases = append(cases, e.Name())
		}
	}
	sort.Strings(cases)
	require.NotEmpty(t, cases)
	return cases
}

// TestCorpusCompilesGo generates every corpus schema plus the repository's
// test schemas, and builds and vets the Go output.
func TestCorpusCompilesGo(t *testing.T) {
	const module = "scgcorpus"
	dir := goModule(t, module)
	for _, c := range corpusCases(t) {
		generateGoInto(t, filepath.Join("testdata", "corpus", c), dir, c, module)
	}
	for _, s := range goldenSchemas {
		generateGoInto(t, filepath.Join(repoRoot, s.input), dir, "repo/"+s.name, module)
	}
	runGo(t, dir, "mod", "tidy")
	runGo(t, dir, "vet", "./...")
}

// TestCorpusCompilesCpp generates every corpus schema plus the repository's
// test schemas, and compiles each C++ header on its own.
func TestCorpusCompilesCpp(t *testing.T) {
	_, _ = cppCompiler(t)
	out := t.TempDir()
	inputs := map[string]string{}
	for _, c := range corpusCases(t) {
		inputs[c] = filepath.Join("testdata", "corpus", c)
	}
	for _, s := range goldenSchemas {
		inputs["repo/"+s.name] = filepath.Join(repoRoot, s.input)
	}
	for name, input := range inputs {
		p, err := parse.NewParse(input)
		require.NoError(t, err, name)
		require.NoError(t, cpp_gen.GenerateCppCode("", filepath.Join(out, name), p), name)
	}
	compileHeaders(t, out)
}

// TestGeneratedGoRuntime runs testdata/runtime/harness_test.go.txt against
// generated code: container length bounds, bulk byte lists, the BitSize
// property and random / mutated input decoding.
func TestGeneratedGoRuntime(t *testing.T) {
	const module = "scgharness"
	dir := goModule(t, module)
	generateGoInto(t, filepath.Join("testdata", "runtime", "schema"), dir, "gen", module)
	harness, err := os.ReadFile(filepath.Join("testdata", "runtime", "harness_test.go.txt"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "harness_test.go"), harness, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "harness.go"), []byte("package harness\n"), 0644))
	runGo(t, dir, "mod", "tidy")
	out := runGo(t, dir, "test", "-count=1", "-v", ".")
	t.Log(out)
}

// TestCppJSONUnit compiles testdata/cpp/json_unit_test.cpp (unit tests for
// include/scg/json.h: base64, RFC 3339, numbers, typedefs, containers) under
// C++17 and C++20 and runs it.
func TestCppJSONUnit(t *testing.T) {
	cxx, includes := cppCompiler(t)
	src, err := filepath.Abs(filepath.Join("testdata", "cpp", "json_unit_test.cpp"))
	require.NoError(t, err)
	for _, std := range []string{"c++17", "c++20"} {
		bin := filepath.Join(t.TempDir(), "json_unit_test")
		args := append([]string{"-std=" + std, "-Wall", "-Wextra", "-Werror", "-Wpedantic", "-O1", "-o", bin, src}, includes...)
		out, err := exec.Command(cxx, args...).CombinedOutput()
		require.NoError(t, err, "%s: compile failed:\n%s", std, firstLines(string(out), 30))
		out, err = exec.Command(bin).CombinedOutput()
		require.NoError(t, err, "%s: json unit tests failed:\n%s", std, string(out))
	}
}

// TestCppGeneratedJSON generates testdata/cpp/schema and runs
// testdata/cpp/json_generated_test.cpp against it: absent and unknown keys,
// null, non-object values and empty messages.
func TestCppGeneratedJSON(t *testing.T) {
	cxx, includes := cppCompiler(t)
	out := t.TempDir()
	p, err := parse.NewParse(filepath.Join("testdata", "cpp", "schema"))
	require.NoError(t, err)
	require.NoError(t, cpp_gen.GenerateCppCode("", out, p))
	src, err := filepath.Abs(filepath.Join("testdata", "cpp", "json_generated_test.cpp"))
	require.NoError(t, err)
	bin := filepath.Join(t.TempDir(), "json_generated_test")
	args := append([]string{"-std=c++17", "-Wall", "-Wextra", "-Werror", "-Wpedantic", "-O1", "-I" + out, "-o", bin, src}, includes...)
	compileOut, err := exec.Command(cxx, args...).CombinedOutput()
	require.NoError(t, err, "compile failed:\n%s", firstLines(string(compileOut), 30))
	runOut, err := exec.Command(bin).CombinedOutput()
	require.NoError(t, err, "generated JSON tests failed:\n%s", string(runOut))
}
