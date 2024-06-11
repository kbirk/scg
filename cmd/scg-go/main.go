package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/kbirk/scg/internal/gen/go_gen"
	"github.com/kbirk/scg/internal/parse"
)

const (
	version = "0.0.1"
)

var (
	input       string
	output      string
	basePackage string
)

func main() {

	flag.StringVar(&input, "input", "", "Input dir")
	flag.StringVar(&output, "output", "", "Output dir")
	flag.StringVar(&basePackage, "base-package", "", "Golang base package")

	flag.Parse()

	if input == "" {
		os.Stderr.WriteString("No `--input` argument provided, Set input dir with `--input=\"<glob>\"`\n")
		os.Exit(1)
	}

	if output == "" {
		os.Stderr.WriteString("No `--output` argument provided, Set output dir with `--output=\"<dir>\"`\n")
		os.Exit(1)
	}

	if basePackage == "" {
		os.Stderr.WriteString("No `--base-package` argument provided, Set go base package with `--base-package=\"<package>\"`\n")
		os.Exit(1)
	}

	red := color.New(color.FgRed, color.Bold).SprintFunc()
	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	blue := color.New(color.FgBlue, color.Bold).SprintFunc()
	yellow := color.New(color.FgYellow, color.Bold).SprintFunc()
	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	magenta := color.New(color.FgMagenta, color.Bold).SprintFunc()
	white := color.New(color.FgWhite, color.Bold).SprintFunc()

	p, err := parse.NewParse(input)
	if err != nil {
		os.Stderr.WriteString(red("ERROR: ") + fmt.Sprintf("Failed to parse input: %v\n", err.Error()))
		os.Exit(1)
	}

	err = go_gen.GenerateGoCode(basePackage, output, p)
	if err != nil {
		os.Stderr.WriteString(red("ERROR: ") + fmt.Sprintf("Failed to generate go output: %v\n", err.Error()))
		os.Exit(1)
	}

	if len(p.Files) == 0 {
		os.Stderr.WriteString(red("ERROR: ") + "No files to generate\n")
		os.Exit(1)
	}

	os.Stdout.WriteString(green("SUCCESS: ") + fmt.Sprintf("Generated code for %d files\n", len(p.Files)))

	for _, pkg := range p.Packages {
		os.Stdout.WriteString(fmt.Sprintf("%s: %s\n", magenta("[package]"), white(pkg.Name)))
		for _, typedef := range pkg.Typedefs {
			os.Stdout.WriteString(fmt.Sprintf("    %s %s = %s\n", green("[typedef]"), white(typedef.Name), cyan(typedef.DataTypeDefinition.ToString())))
		}
		for _, msg := range pkg.MessageDefinitions {
			os.Stdout.WriteString(fmt.Sprintf("    %s %s {\n", blue("[message]"), white(msg.Name)))
			for _, field := range msg.FieldsByIndex() {
				os.Stdout.WriteString(fmt.Sprintf("        %s %s = %s\n", cyan(field.DataTypeDefinition.ToString()), white(field.Name), white(field.Index)))
			}
			os.Stdout.WriteString("    }\n")
		}
		for _, svc := range pkg.ServiceDefinitions {
			os.Stdout.WriteString(fmt.Sprintf("    %s %s {\n", yellow("[service]"), white(svc.Name)))
			for _, method := range svc.Methods {
				os.Stdout.WriteString(fmt.Sprintf("        %s (%s) %s\n", white(method.Name), cyan(method.Argument.CustomType), cyan(method.Return.CustomType)))
			}
			os.Stdout.WriteString("    }\n")
		}
	}
}
