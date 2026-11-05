package cpp_gen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/kbirk/scg/internal/parse"
)

func GenerateCppCode(baseDir string, outputDir string, p *parse.Parse) error {

	enumStrings, err := validateCpp(p)
	if err != nil {
		return err
	}

	ctx := &cppContext{
		parse:       p,
		baseDir:     baseDir,
		enumStrings: enumStrings,
	}

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

		code, err := generateFileCppCodeWithContext(ctx, pkg, file)
		if err != nil {
			return err
		}

		outputFileAndPath := filepath.Join(outputDir, file.RelativePath, getOutputFileName(path))

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
