package go_gen

import (
	"bytes"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

type ClientMethodArgs struct {
	MethodNamePascalCase     string
	MethodNameCamelCase      string
	MethodIDVarName          string
	MethodID                 uint64
	MethodRequestStructName  string
	MethodResponseStructName string
}

type ClientArgs struct {
	ClientNamePascalCase string
	ClientNameCamelCase  string
	ServiceIDVarName     string
	ServiceID            uint64
	ClientMethods        []ClientMethodArgs
}

const clientTemplateStr = `
type {{.ClientNamePascalCase}}Client struct {
	client *rpc.Client
}

func New{{.ClientNamePascalCase}}Client(client *rpc.Client) *{{.ClientNamePascalCase}}Client {
	return &{{.ClientNamePascalCase}}Client{
		client: client,
	}
}

{{range .ClientMethods}}
func (c *{{$.ClientNamePascalCase}}Client) {{.MethodNamePascalCase}}(ctx context.Context, req *{{.MethodRequestStructName}}) (*{{.MethodResponseStructName}}, error) {
	reader, err := c.client.Call(ctx, {{$.ServiceIDVarName}}, {{.MethodIDVarName}}, req)
	if err != nil {
		return nil, err
	}

	resp := &{{.MethodResponseStructName}}{}
	err = resp.Deserialize(reader)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
{{end}}
`

var (
	clientTemplate = template.Must(template.New("clientTemplateGo").Parse(clientTemplateStr))
)

func generateClientGoCode(pkg *parse.Package, svc *parse.ServiceDefinition) (string, error) {

	serviceID, err := pkg.HashStringToServiceID(svc.Name)
	if err != nil {
		return "", err
	}

	args := ClientArgs{
		ClientNamePascalCase: util.EnsurePascalCase(svc.Name),
		ClientNameCamelCase:  util.EnsureCamelCase(svc.Name),
		ServiceIDVarName:     serviceIDVarName(svc.Name),
		ServiceID:            serviceID,
		ClientMethods:        []ClientMethodArgs{},
	}

	for name, method := range svc.Methods {
		methodID, err := pkg.HashStringToMethodID(svc.Name, name)
		if err != nil {
			return "", err
		}
		methodArgType, methodRetType, err := generateServiceMethodParams(method)
		if err != nil {
			return "", err
		}
		args.ClientMethods = append(args.ClientMethods, ClientMethodArgs{
			MethodNamePascalCase:     util.EnsurePascalCase(name),
			MethodNameCamelCase:      util.EnsureCamelCase(name),
			MethodIDVarName:          methodIDVarName(svc.Name, name),
			MethodID:                 methodID,
			MethodRequestStructName:  methodArgType,
			MethodResponseStructName: methodRetType,
		})
	}

	buf := &bytes.Buffer{}
	err = clientTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
