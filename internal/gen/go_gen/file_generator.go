package go_gen

import (
	"bytes"
	"go/format"
	"strings"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
	"github.com/pkg/errors"
)

type FileArgs struct {
	Header   string
	Package  string
	Imports  string
	Enums    string
	Typedefs string
	Consts   string
	Messages string
	Servers  string
	Clients  string
}

const fileTemplateStr = `
{{.Header}}
{{.Package}}
{{.Imports}}
{{.Enums}}
{{.Typedefs}}
{{.Consts}}
{{.Messages}}
{{.Servers}}
{{.Clients}}
`

var (
	fileTemplate = template.Must(template.New("fileTemplate").Parse(fileTemplateStr))
)

func generateFileGoCode(basePackage string, pkg *parse.Package, file *parse.File) (string, error) {

	headerCode, err := generateHeaderGo(file)
	if err != nil {
		return "", err
	}

	pkgCode, err := generatePackageGoCode(file.Package)
	if err != nil {
		return "", err
	}

	importCode, err := generateImportsGoCode(basePackage, file)
	if err != nil {
		return "", err
	}

	var enumCode []string
	for _, msg := range file.EnumsSortedByKey() {
		enum, err := generateEnumGoCode(msg)
		if err != nil {
			return "", err
		}
		enumCode = append(enumCode, enum)
	}

	var typedefCode []string
	for _, msg := range file.TypedefsSortedByKey() {
		typdef, err := generateTypedefGoCode(msg)
		if err != nil {
			return "", err
		}
		typedefCode = append(typedefCode, typdef)
	}

	var constsCode []string
	for _, c := range file.ConstsSortedByKey() {
		consts, err := generateConstGoCode(c)
		if err != nil {
			return "", err
		}
		constsCode = append(constsCode, consts)
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
		Enums:    strings.Join(enumCode, "\n"),
		Typedefs: strings.Join(typedefCode, "\n"),
		Consts:   strings.Join(constsCode, "\n"),
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
		return "", errors.Wrap(err, "failed to gofmt code:\n"+string(buf.Bytes()))
	}

	return string(formattedCode), nil
}
