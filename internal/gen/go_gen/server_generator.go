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

type ServiceStreamMethodArgs struct {
	MethodNamePascalCase string
	MethodIDVarName      string
	MethodID             uint64
	Kind                 string // "bidi" | "server" | "client"
	StreamTypeName       string
	ReqStructName        string // request (argument) message — server receives
	RespStructName       string // response (return) message — server sends
}

type ServerArgs struct {
	ServerNamePascalCase string
	ServerNameCamelCase  string
	ServiceName          string
	ServiceIDVarName     string
	ServerStubStructName string
	ServiceID            uint64
	ServiceMethods       []ServiceMethodArgs
	ServiceStreamMethods []ServiceStreamMethodArgs
}

const serverTemplateStr = `
const (
	{{.ServiceIDVarName}} uint64 = {{.ServiceID}} {{- range .ServiceMethods}}
	{{.MethodIDVarName}} uint64 = {{.MethodID}}{{end}}{{range .ServiceStreamMethods}}
	{{.MethodIDVarName}} uint64 = {{.MethodID}}{{end}}
)

type {{.ServerNamePascalCase}} interface { {{- range .ServiceMethods}}
	{{.MethodNamePascalCase}}(context.Context, *{{.MethodRequestStructName}}) (*{{.MethodResponseStructName}}, error){{end}}{{range .ServiceStreamMethods}}
	{{if eq .Kind "bidi"}}{{.MethodNamePascalCase}}(*{{.StreamTypeName}}) error{{else if eq .Kind "server"}}{{.MethodNamePascalCase}}(*{{.ReqStructName}}, *{{.StreamTypeName}}) error{{else}}{{.MethodNamePascalCase}}(*{{.StreamTypeName}}) (*{{.RespStructName}}, error){{end}}{{end}}
}

func Register{{.ServerNamePascalCase}}(server *rpc.Server, {{.ServerNameCamelCase}} {{.ServerNamePascalCase}}) {
	server.RegisterServer({{.ServiceIDVarName}}, "{{.ServiceName}}", &{{.ServerStubStructName}}{ server, {{.ServerNameCamelCase}} })
}

type {{.ServerStubStructName}} struct {
	server *rpc.Server
	impl {{.ServerNamePascalCase}}
}

{{range .ServiceMethods}}
func (s *{{$.ServerStubStructName}}) handle{{.MethodNamePascalCase}}(ctx context.Context, middleware []rpc.Middleware, requestID uint64, reader *serialize.Reader) []byte {
	req := &{{.MethodRequestStructName}}{}
	err := req.Deserialize(reader)
	if err != nil {
		return rpc.RespondWithError(requestID, err)
	}

	handler := func (ctx context.Context, req rpc.Message) (rpc.Message, error) {
		r, ok := req.(*{{.MethodRequestStructName}})
		if !ok {
			return nil, fmt.Errorf("invalid request type %T", req)
		}
		return s.impl.{{.MethodNamePascalCase}}(ctx, r)
	}

	resp, err := rpc.ApplyHandlerChain(ctx, req, middleware, handler)
	if err != nil {
		return rpc.RespondWithError(requestID, err)
	}

	return rpc.RespondWithMessage(requestID, resp)
}
{{end}}
func (s *{{$.ServerStubStructName}}) HandleWrapper(ctx context.Context, middleware []rpc.Middleware, requestID uint64, reader *serialize.Reader) []byte {
	var methodID uint64
	err := serialize.DeserializeUInt64(&methodID, reader)
	if err != nil {
		return rpc.RespondWithError(requestID, err)
	}

	switch methodID { {{- range .ServiceMethods}}
	case {{.MethodIDVarName}}:
		return s.handle{{.MethodNamePascalCase}}(ctx, middleware, requestID, reader){{end}}
	default:
		return rpc.RespondWithError(requestID, fmt.Errorf("unrecognized methodID %d", methodID))
	}
}

func (s *{{$.ServerStubStructName}}) HandleStreamWrapper(ctx context.Context, stream *rpc.ServerStream, methodID uint64) error {
	switch methodID { {{- range .ServiceStreamMethods}}
	case {{.MethodIDVarName}}:
		{{if eq .Kind "bidi"}}return s.impl.{{.MethodNamePascalCase}}(&{{.StreamTypeName}}{stream: stream}){{else if eq .Kind "server"}}reader, err := stream.Recv()
		if err != nil {
			return err
		}
		req := &{{.ReqStructName}}{}
		if err := req.Deserialize(reader); err != nil {
			return err
		}
		return s.impl.{{.MethodNamePascalCase}}(req, &{{.StreamTypeName}}{stream: stream}){{else}}resp, err := s.impl.{{.MethodNamePascalCase}}(&{{.StreamTypeName}}{stream: stream})
		if err != nil {
			return err
		}
		return stream.Send(resp){{end}}{{end}}
	default:
		return fmt.Errorf("unrecognized stream methodID %d", methodID)
	}
}
{{range .ServiceStreamMethods}}
// {{.StreamTypeName}} is the server handle for the {{.MethodNamePascalCase}} stream.
type {{.StreamTypeName}} struct {
	stream *rpc.ServerStream
}
{{if eq .Kind "bidi"}}
// Recv blocks for the next client message; returns io.EOF on client half-close.
func (s *{{.StreamTypeName}}) Recv() (*{{.ReqStructName}}, error) {
	reader, err := s.stream.Recv()
	if err != nil {
		return nil, err
	}
	req := &{{.ReqStructName}}{}
	if err := req.Deserialize(reader); err != nil {
		return nil, err
	}
	return req, nil
}

// Send pushes a message to the client.
func (s *{{.StreamTypeName}}) Send(resp *{{.RespStructName}}) error {
	return s.stream.Send(resp)
}
{{else if eq .Kind "server"}}
// Send pushes a message to the client.
func (s *{{.StreamTypeName}}) Send(resp *{{.RespStructName}}) error {
	return s.stream.Send(resp)
}
{{else}}
// Recv blocks for the next client message; returns io.EOF on client half-close.
func (s *{{.StreamTypeName}}) Recv() (*{{.ReqStructName}}, error) {
	reader, err := s.stream.Recv()
	if err != nil {
		return nil, err
	}
	req := &{{.ReqStructName}}{}
	if err := req.Deserialize(reader); err != nil {
		return nil, err
	}
	return req, nil
}
{{end}}
// Context returns the context the stream was opened with (carries OPEN metadata).
func (s *{{.StreamTypeName}}) Context() context.Context {
	return s.stream.Context()
}
{{end}}
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

func streamClientTypeName(serviceName string, methodName string) string {
	return fmt.Sprintf("%s_%sStreamClient", util.EnsurePascalCase(serviceName), util.EnsurePascalCase(methodName))
}

func streamServerTypeName(serviceName string, methodName string) string {
	return fmt.Sprintf("%s_%sStreamServer", util.EnsurePascalCase(serviceName), util.EnsurePascalCase(methodName))
}

// streamKind classifies a streaming method by which sides stream.
func streamKind(method *parse.ServiceMethodDefinition) string {
	if method.ArgumentStream && method.ReturnStream {
		return "bidi"
	}
	if method.ReturnStream {
		return "server"
	}
	return "client"
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
		ServiceName:          svc.Name,
		ServiceIDVarName:     serviceIDVarName(svc.Name),
		ServiceID:            serverID,
		ServiceMethods:       []ServiceMethodArgs{},
		ServiceStreamMethods: []ServiceStreamMethodArgs{},
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

		if method.IsStreaming() {
			args.ServiceStreamMethods = append(args.ServiceStreamMethods, ServiceStreamMethodArgs{
				MethodNamePascalCase: util.EnsurePascalCase(name),
				MethodIDVarName:      methodIDVarName(svc.Name, name),
				MethodID:             methodID,
				Kind:                 streamKind(method),
				StreamTypeName:       streamServerTypeName(svc.Name, name),
				ReqStructName:        methodArgType,
				RespStructName:       methodRetType,
			})
			continue
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
