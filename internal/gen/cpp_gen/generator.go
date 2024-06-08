package cpp_gen

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kbirk/scg/internal/parse"
)

func GenerateCppCode(outputDir string, p *parse.Parse) error {
	for path, file := range p.Files {

		pkg, ok := p.Packages[file.Package.Name]
		if !ok {
			return fmt.Errorf("package not found: %s", file.Package.Name)
		}

		code, err := generateFileCppCode(pkg, file)
		if err != nil {
			return err
		}

		_, filename := filepath.Split(path)
		filename = filename[:len(filename)-len(filepath.Ext(filename))]

		outputFileAndPath := filepath.Join(outputDir, file.RelativePath, getOutputFileName(filename))

		err = os.MkdirAll(filepath.Dir(outputFileAndPath), 0755)
		if err != nil {
			return err
		}

		outputFile, err := os.Create(outputFileAndPath)
		if err != nil {
			return err
		}
		defer outputFile.Close()

		_, err = outputFile.WriteString(code)
		if err != nil {
			return err
		}
	}

	return nil
}
