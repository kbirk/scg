package go_gen

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

type ServiceMethodArgs struct {
	MethodNamePascalCase     string
	MethodNameCamelCase      string
	MethodIDVarName          string
	MethodID                 uint64
	MethodRequestStructName  string
	MethodResponseStructName string
}

type ServerArgs struct {
	ServerNamePascalCase string
	ServerNameCamelCase  string
	ServiceIDVarName     string
	ServerStubStructName string
	ServiceID            uint64
	ServiceMethods       []ServiceMethodArgs
}

const serverTemplateStr = `
const (
	{{.ServiceIDVarName}} uint64 = {{.ServiceID}} {{- range .ServiceMethods}}
	{{.MethodIDVarName}} uint64 = {{.MethodID}}{{end}}
)

type {{.ServerNamePascalCase}} interface { {{- range .ServiceMethods}}
	{{.MethodNamePascalCase}}(context.Context, *{{.MethodRequestStructName}}) (*{{.MethodResponseStructName}}, error){{end}}
}

func Register{{.ServerNamePascalCase}}(server *rpc.Server, {{.ServerNameCamelCase}} {{.ServerNamePascalCase}}) {
	server.RegisterServer({{.ServiceIDVarName}}, &{{.ServerStubStructName}}{ {{.ServerNameCamelCase}} })
}

type {{.ServerStubStructName}} struct {
	server {{.ServerNamePascalCase}}
}

{{range .ServiceMethods}}
func (s *{{$.ServerStubStructName}}) handle{{.MethodNamePascalCase}}(ctx context.Context, requestID uint64, reader *serialize.Reader) []byte {
	req := &{{.MethodRequestStructName}}{}
	err := req.Deserialize(reader)
	if err != nil {
		return rpc.RespondWithError(requestID, err)
	}

	resp, err := s.server.{{.MethodNamePascalCase}}(ctx, req)
	if err != nil {
		return rpc.RespondWithError(requestID, err)
	}

	return rpc.RespondWithMessage(requestID, resp)
}
{{end}}
func (s *{{$.ServerStubStructName}}) HandleWrapper(ctx context.Context, requestID uint64, reader *serialize.Reader) []byte {
	var methodID uint64
	err := serialize.DeserializeUInt64(&methodID, reader)
	if err != nil {
		return rpc.RespondWithError(requestID, err)
	}

	switch methodID { {{- range .ServiceMethods}}
	case {{.MethodIDVarName}}:
		return s.handle{{.MethodNamePascalCase}}(ctx, requestID, reader){{end}}
	default:
		return rpc.RespondWithError(requestID, fmt.Errorf("unrecognized methodID %d", methodID))
	}
}
`

var (
	serverTemplate = template.Must(template.New("serverTemplateGo").Parse(serverTemplateStr))
)

func serviceIDVarName(serviceName string) string {
	return fmt.Sprintf("%sServerID", util.EnsureCamelCase(serviceName))
}

func methodIDVarName(serviceName string, methodName string) string {
	return fmt.Sprintf("%sServer_%sID", util.EnsureCamelCase(serviceName), util.EnsurePascalCase(methodName))
}

func getServerStubStructName(serviceName string) string {
	return fmt.Sprintf("%s_Stub", util.EnsureCamelCase(serviceName))
}

func generateServiceMethodParams(method *parse.ServiceMethodDefinition) (string, string, error) {

	argType, err := mapDataTypeDefinitionToGoType(method.Argument)
	if err != nil {
		return "", "", err
	}
	retType, err := mapDataTypeDefinitionToGoType(method.Return)
	if err != nil {
		return "", "", err
	}
	return argType, retType, nil
}

func getServerNamePascalCase(serviceName string) string {
	return fmt.Sprintf("%s%s", util.EnsurePascalCase(serviceName), "Server")
}
func getServerNameCamelCase(serviceName string) string {
	return fmt.Sprintf("%s%s", util.EnsureCamelCase(serviceName), "Server")
}

func generateServerGoCode(pkg *parse.Package, svc *parse.ServiceDefinition) (string, error) {

	serverID, err := pkg.HashStringToServiceID(svc.Name)
	if err != nil {
		return "", err
	}

	args := ServerArgs{
		ServerNamePascalCase: getServerNamePascalCase(svc.Name),
		ServerNameCamelCase:  getServerNameCamelCase(svc.Name),
		ServiceIDVarName:     serviceIDVarName(svc.Name),
		ServiceID:            serverID,
		ServiceMethods:       []ServiceMethodArgs{},
		ServerStubStructName: getServerStubStructName(svc.Name),
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
		args.ServiceMethods = append(args.ServiceMethods, ServiceMethodArgs{
			MethodNamePascalCase:     util.EnsurePascalCase(name),
			MethodNameCamelCase:      util.EnsureCamelCase(name),
			MethodIDVarName:          methodIDVarName(svc.Name, name),
			MethodID:                 methodID,
			MethodRequestStructName:  methodArgType,
			MethodResponseStructName: methodRetType,
		})
	}

	buf := &bytes.Buffer{}
	err = serverTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
