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
	Servers    string
	Clients    string
}

const fileTemplateStr = `
{{.Header}}
{{.Imports}}{{ range .Namespaces }}
namespace {{.}} { {{end}}

// Import float type aliases from scg::serialize namespace
using scg::serialize::float32_t;
using scg::serialize::float64_t;

{{.Enums}}
{{.Typedefs}}
{{.Consts}}
{{.Messages}}
{{.Servers}}
{{.Clients}}{{ range .Namespaces }}
} {{end}}
`

var (
	fileTemplate = template.Must(template.New("fileTemplate").Parse(fileTemplateStr))
)

// cppContext carries generation-wide state.
type cppContext struct {
	parse   *parse.Parse
	baseDir string
	// enumStrings holds, per package, which enum string constants to emit
	// (see planEnumStringNames).
	enumStrings map[string]*enumStringNames
}

func generateFileCppCode(baseDir string, pkg *parse.Package, file *parse.File) (string, error) {
	return generateFileCppCodeWithContext(&cppContext{baseDir: baseDir}, pkg, file)
}

func generateFileCppCodeWithContext(ctx *cppContext, pkg *parse.Package, file *parse.File) (string, error) {

	baseDir := ctx.baseDir

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
		enum, err := generateEnumCppCodeWithNames(msg, ctx.enumStrings[pkg.Name])
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
		message, err := generateMessageCppCode(ctx.parse, msg)
		if err != nil {
			return "", err
		}
		messageCode = append(messageCode, message)
	}

	var serverCode []string
	for _, svc := range file.ServicesSortedByKey() {
		service, err := generateServerCppCode(pkg, svc)
		if err != nil {
			return "", err
		}
		serverCode = append(serverCode, service)
	}

	var clientCode []string
	for _, svc := range file.ServicesSortedByKey() {
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
		Servers:    strings.Join(serverCode, "\n"),
		Clients:    strings.Join(clientCode, "\n"),
	}

	buf := &bytes.Buffer{}
	err = fileTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
