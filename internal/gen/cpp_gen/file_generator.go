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
	Enums      string
	Typedefs   string
	Consts     string
	Messages   string
	// Servers  string
	Clients string
}

const fileTemplateStr = `
{{.Header}}
{{.Imports}}{{ range .Namespaces }}
namespace {{.}} { {{end}}
{{.Enums}}
{{.Typedefs}}
{{.Consts}}
{{.Messages}}
{{.Clients}}{{ range .Namespaces }}
} {{end}}
`

var (
	fileTemplate = template.Must(template.New("fileTemplate").Parse(fileTemplateStr))
)

func generateFileCppCode(baseDir string, pkg *parse.Package, file *parse.File) (string, error) {

	headerCode, err := generateHeaderCpp(file)
	if err != nil {
		return "", err
	}

	namespaces := convertPackageNameToCppNamespaces(pkg.Name)

	importCode, err := generateImportsCppCode(baseDir, file)
	if err != nil {
		return "", err
	}

	var enumCode []string
	for _, msg := range file.EnumsSortedByKey() {
		enum, err := generateEnumCppCode(msg)
		if err != nil {
			return "", err
		}
		enumCode = append(enumCode, enum)
	}

	var typedefCode []string
	for _, msg := range file.TypedefsSortedByKey() {
		typdef, err := generateTypedefCppCode(msg)
		if err != nil {
			return "", err
		}
		typedefCode = append(typedefCode, typdef)
	}

	var constsCode []string
	for _, c := range file.ConstsSortedByKey() {
		consts, err := generateConstCppCode(c)
		if err != nil {
			return "", err
		}
		constsCode = append(constsCode, consts)
	}

	var messageCode []string
	for _, msg := range file.MessagesSortedByDependenciesAndKeys() {
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
		Enums:      strings.Join(enumCode, "\n"),
		Typedefs:   strings.Join(typedefCode, "\n"),
		Consts:     strings.Join(constsCode, "\n"),
		Messages:   strings.Join(messageCode, "\n"),
		// Servers:    strings.Join(serverCode, "\n"),
		Clients: strings.Join(clientCode, "\n"),
	}

	buf := &bytes.Buffer{}
	err = fileTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
