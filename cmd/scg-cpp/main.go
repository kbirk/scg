package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/kbirk/scg/internal/gen/cpp_gen"
	"github.com/kbirk/scg/internal/parse"
)

const (
	version = "0.0.1"
)

var (
	input   string
	output  string
	baseDir string
)

func main() {

	flag.StringVar(&input, "input", "", "Input dir")
	flag.StringVar(&output, "output", "", "Output dir")
	flag.StringVar(&baseDir, "base-dir", "", "Golang base package")

	flag.Parse()

	if input == "" {
		os.Stderr.WriteString("No `--input` argument provided, Set input dir with `--input=\"<glob>\"`\n")
		os.Exit(1)
	}

	if output == "" {
		os.Stderr.WriteString("No `--output` argument provided, Set output dir with `--output=\"<dir>\"`\n")
		os.Exit(1)
	}

	red := color.New(color.FgRed, color.Bold).SprintFunc()
	green := color.New(color.FgGreen, color.Bold).SprintFunc()

	p, err := parse.NewParse(input)
	if err != nil {
		os.Stderr.WriteString(red("ERROR: ") + fmt.Sprintf("Failed to parse input: %v\n", err.Error()))
		os.Exit(1)
	}

	err = cpp_gen.GenerateCppCode(baseDir, output, p)
	if err != nil {
		os.Stderr.WriteString(red("ERROR: ") + fmt.Sprintf("Failed to generate cpp output: %v\n", err.Error()))
		os.Exit(1)
	}

	if len(p.Files) == 0 {
		os.Stderr.WriteString(red("ERROR: ") + "No files to generate\n")
		os.Exit(1)
	}

	os.Stdout.WriteString(green("SUCCESS: ") + fmt.Sprintf("Generated code for %d files\n", len(p.Files)))
	os.Stdout.WriteString(p.ToStringPretty())
}
