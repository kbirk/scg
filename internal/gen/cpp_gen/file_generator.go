package cpp_gen

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
)

type FileArgs struct {
	Header     string
	Namespaces []string
	Imports    string
	Messages   string
	// Servers  string
	Clients string
}

const fileTemplateStr = `
{{.Header}}
{{.Imports}}{{ range .Namespaces }}
namespace {{.}} { {{end}}
{{.Messages}}
{{.Clients}}{{ range .Namespaces }}
} {{end}}
`

var (
	fileTemplate = template.Must(template.New("fileTemplate").Parse(fileTemplateStr))
)

func generateFileCppCode(pkg *parse.Package, file *parse.File) (string, error) {

	headerCode, err := generateHeaderCpp(file)
	if err != nil {
		return "", err
	}

	namespaces := convertPackageNameToCppNamespaces(pkg.Name)

	importCode, err := generateImportsCppCode(file.GetFileDependencies(), len(file.ServiceDefinitions) > 0, len(file.MessageDefinitions) > 0)
	if err != nil {
		return "", err
	}

	var messageCode []string
	for _, msg := range file.MessagesSortedByKey() {
		message, err := generateMessageCppCode(msg)
		if err != nil {
			return "", err
		}
		messageCode = append(messageCode, message)
	}

	// var serverCode []string
	// for _, svc := range file.ServicesSortedByKey() {
	// 	service, err := generateServerGoCode(pkg, svc)
	// 	if err != nil {
	// 		return "", err
	// 	}
	// 	serverCode = append(serverCode, service)
	// }

	var clientCode []string
	for _, svc := range file.ServiceDefinitions {
		client, err := generateClientCppCode(pkg, svc)
		if err != nil {
			return "", err
		}
		clientCode = append(clientCode, client)
	}

	args := FileArgs{
		Header:     headerCode,
		Namespaces: namespaces,
		Imports:    importCode,
		Messages:   strings.Join(messageCode, "\n"),
		// Servers:    strings.Join(serverCode, "\n"),
		Clients: strings.Join(clientCode, "\n"),
	}

	buf := &bytes.Buffer{}
	err = fileTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}

	return string(buf.Bytes()), nil
}
