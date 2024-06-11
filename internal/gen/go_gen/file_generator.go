package go_gen

import (
	"bytes"
	"go/format"
	"strings"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
)

type FileArgs struct {
	Header   string
	Package  string
	Imports  string
	Typedefs string
	Messages string
	Servers  string
	Clients  string
}

const fileTemplateStr = `
{{.Header}}
{{.Package}}
{{.Imports}}
{{.Typedefs}}
{{.Messages}}
{{.Servers}}
{{.Clients}}
`

var (
	fileTemplate = template.Must(template.New("fileTemplate").Parse(fileTemplateStr))
)

func generateFileGoCode(goBasePackage string, pkg *parse.Package, file *parse.File) (string, error) {

	headerCode, err := generateHeaderGo(file)
	if err != nil {
		return "", err
	}

	pkgCode, err := generatePackageGoCode(file.Package)
	if err != nil {
		return "", err
	}

	importCode, err := generateImportsGoCode(goBasePackage, file.GetPackageDependencies(), len(file.ServiceDefinitions) > 0, len(file.MessageDefinitions) > 0, len(file.Typedefs) > 0)
	if err != nil {
		return "", err
	}

	var typedefCode []string
	for _, msg := range file.TypedefsSortedByKey() {
		typdef, err := generateTypedefGoCode(msg)
		if err != nil {
			return "", err
		}
		typedefCode = append(typedefCode, typdef)
	}

	var messageCode []string
	for _, msg := range file.MessagesSortedByKey() {
		message, err := generateMessageGoCode(msg)
		if err != nil {
			return "", err
		}
		messageCode = append(messageCode, message)
	}

	var serverCode []string
	for _, svc := range file.ServicesSortedByKey() {
		service, err := generateServerGoCode(pkg, svc)
		if err != nil {
			return "", err
		}
		serverCode = append(serverCode, service)
	}

	var clientCode []string
	for _, svc := range file.ServiceDefinitions {
		client, err := generateClientGoCode(pkg, svc)
		if err != nil {
			return "", err
		}
		clientCode = append(clientCode, client)
	}

	args := FileArgs{
		Header:   headerCode,
		Package:  pkgCode,
		Imports:  importCode,
		Typedefs: strings.Join(typedefCode, "\n"),
		Messages: strings.Join(messageCode, "\n"),
		Servers:  strings.Join(serverCode, "\n"),
		Clients:  strings.Join(clientCode, "\n"),
	}

	buf := &bytes.Buffer{}
	err = fileTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}

	formattedCode, err := format.Source(buf.Bytes())
	if err != nil {
		return "", err
	}

	return string(formattedCode), nil
}
