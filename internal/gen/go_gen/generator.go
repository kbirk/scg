package go_gen

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kbirk/scg/internal/parse"
)

func GenerateGoCode(goBasePackage, outputDir string, p *parse.Parse) error {
	for path, file := range p.Files {

		pkg, ok := p.Packages[file.Package.Name]
		if !ok {
			return fmt.Errorf("package not found: %s", file.Package.Name)
		}

		code, err := generateFileGoCode(goBasePackage, pkg, file)
		if err != nil {
			return err
		}

		_, filename := filepath.Split(path)
		filename = filename[:len(filename)-len(filepath.Ext(filename))]

		outputFileAndPath := filepath.Join(outputDir, file.RelativePath, filename+".go")

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
