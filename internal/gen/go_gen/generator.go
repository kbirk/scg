package go_gen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kbirk/scg/internal/parse"
)

func GenerateGoCode(basePackage, outputDir string, p *parse.Parse) error {

	err := validateGo(p)
	if err != nil {
		return err
	}

	ctx := newGoContext(basePackage, p)

	paths := make([]string, 0, len(p.Files))
	for path := range p.Files {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		file := p.Files[path]

		pkg, ok := p.Packages[file.Package.Name]
		if !ok {
			return fmt.Errorf("package not found: %s", file.Package.Name)
		}

		code, err := generateFileGoCodeWithContext(ctx, pkg, file)
		if err != nil {
			return err
		}

		outputFileAndPath := filepath.Join(outputDir, file.RelativePath, goOutputFileName(path))

		err = os.MkdirAll(filepath.Dir(outputFileAndPath), 0755)
		if err != nil {
			return err
		}

		err = os.WriteFile(outputFileAndPath, []byte(code), 0644)
		if err != nil {
			return err
		}
	}

	return nil
}

var (
	// knownGoOS / knownGoArch are the values the go tool treats as build
	// constraints when they appear as a `_suffix` of a file name.
	knownGoOS = map[string]bool{
		"aix": true, "android": true, "darwin": true, "dragonfly": true, "freebsd": true,
		"hurd": true, "illumos": true, "ios": true, "js": true, "linux": true, "nacl": true,
		"netbsd": true, "openbsd": true, "plan9": true, "solaris": true, "wasip1": true,
		"windows": true, "zos": true,
	}
	knownGoArch = map[string]bool{
		"386": true, "amd64": true, "amd64p32": true, "arm": true, "armbe": true, "arm64": true,
		"arm64be": true, "loong64": true, "mips": true, "mipsle": true, "mips64": true,
		"mips64le": true, "mips64p32": true, "mips64p32le": true, "ppc": true, "ppc64": true,
		"ppc64le": true, "riscv": true, "riscv64": true, "s390": true, "s390x": true,
		"sparc": true, "sparc64": true, "wasm": true,
	}
)

// goOutputFileName returns the name of the .go file generated for an .scg
// file. It is the source name with a .go extension, adjusted where the go tool
// would otherwise treat the file specially: names ending in `_test`, or in a
// `_GOOS` / `_GOARCH` suffix (which would be excluded from the build or made
// platform specific), get a `_scg` suffix, and names starting with `_` or `.`
// (which the go tool ignores) get an `scg` prefix.
func goOutputFileName(path string) string {
	_, filename := filepath.Split(path)
	base := filename[:len(filename)-len(filepath.Ext(filename))]

	if strings.HasPrefix(base, "_") || strings.HasPrefix(base, ".") {
		base = "scg" + base
	}

	special := strings.HasSuffix(base, "_test")
	if !special && strings.Contains(base, "_") {
		parts := strings.Split(base, "_")
		last := parts[len(parts)-1]
		if knownGoOS[last] || knownGoArch[last] {
			special = true
		}
	}
	if special {
		base += "_scg"
	}

	return base + ".go"
}
