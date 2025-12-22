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
	ReturnsStream            bool
	StreamStructName         string
	StreamInterfaceName      string
	StreamNamePascalCase     string
	HasClientMethods         bool
	HasServerMethods         bool
}

type ServerArgs struct {
	ServerNamePascalCase string
	ServerNameCamelCase  string
	ServiceName          string
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

type {{.ServerNamePascalCase}} interface { {{- range .ServiceMethods}}{{if .ReturnsStream}}
	{{.MethodNamePascalCase}}(context.Context, *{{.MethodRequestStructName}}) (*{{.StreamStructName}}, error){{else}}
	{{.MethodNamePascalCase}}(context.Context, *{{.MethodRequestStructName}}) (*{{.MethodResponseStructName}}, error){{end}}{{end}}
}

func Register{{.ServerNamePascalCase}}(server *rpc.Server, {{.ServerNameCamelCase}} {{.ServerNamePascalCase}}) {
	stub := &{{.ServerStubStructName}}{ server, {{.ServerNameCamelCase}} }
	server.RegisterServer({{.ServiceIDVarName}}, "{{.ServiceName}}", stub)
	{{range .ServiceMethods}}{{if .ReturnsStream}}
	// Register stream handler for {{.MethodNamePascalCase}}
	server.RegisterStream({{$.ServiceIDVarName}}, {{.MethodIDVarName}}, stub.streamHandler{{.MethodNamePascalCase}}){{end}}{{end}}
}

type {{.ServerStubStructName}} struct {
	server *rpc.Server
	impl {{.ServerNamePascalCase}}
}

{{range .ServiceMethods}}{{if .ReturnsStream}}
// Stream handler for {{.MethodNamePascalCase}}
func (s *{{$.ServerStubStructName}}) streamHandler{{.MethodNamePascalCase}}(stream *rpc.Stream, reader *serialize.Reader) error {
	// Deserialize the request message
	req := &{{.MethodRequestStructName}}{}
	err := req.Deserialize(reader)
	if err != nil {
		return err
	}

	// Call the user's implementation to get the stream with handler
	streamWrapper, err := s.impl.{{.MethodNamePascalCase}}(stream.Context(), req)
	if err != nil {
		return err
	}

	// Set the stream's internal rpc.Stream
	streamWrapper.setStream(stream)

	// If the stream has client methods, register the message processor
	{{if .HasClientMethods}}stream.SetProcessor(streamWrapper){{end}}

	// Wait for the stream to close (user's goroutine will manage lifecycle)
	<-stream.Wait()

	return nil
}
{{end}}{{end}}

{{range .ServiceMethods}}{{if not .ReturnsStream}}
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
{{end}}{{end}}
func (s *{{$.ServerStubStructName}}) HandleWrapper(ctx context.Context, middleware []rpc.Middleware, requestID uint64, reader *serialize.Reader) []byte {
	var methodID uint64
	err := serialize.DeserializeUInt64(&methodID, reader)
	if err != nil {
		return rpc.RespondWithError(requestID, err)
	}

	switch methodID { {{- range .ServiceMethods}}{{if not .ReturnsStream}}
	case {{.MethodIDVarName}}:
		return s.handle{{.MethodNamePascalCase}}(ctx, middleware, requestID, reader){{end}}{{end}}
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
		ServiceName:          svc.Name,
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

		var streamStructName, streamInterfaceName string
		var methodArgType, methodRetType string
		var hasClientMethods, hasServerMethods bool

		if method.ReturnsStream {
			// For stream returns, use the stream server struct and interface names
			streamStructName = getStreamServerStructName(method.StreamName)
			streamInterfaceName = getStreamServerInterfaceName(method.StreamName)
			methodArgType, err = mapDataTypeDefinitionToGoType(method.Argument)
			if err != nil {
				return "", err
			}
			// Return type is not used for stream methods
			methodRetType = ""

			// Get stream definition to check for client/server methods
			stream, ok := pkg.StreamDefinitions[method.StreamName]
			if ok {
				for _, streamMethod := range stream.Methods {
					if streamMethod.Direction == parse.StreamMethodDirectionClient {
						hasClientMethods = true
					} else if streamMethod.Direction == parse.StreamMethodDirectionServer {
						hasServerMethods = true
					}
				}
			}
		} else {
			methodArgType, methodRetType, err = generateServiceMethodParams(method)
			if err != nil {
				return "", err
			}
		}

		args.ServiceMethods = append(args.ServiceMethods, ServiceMethodArgs{
			MethodNamePascalCase:     util.EnsurePascalCase(name),
			MethodNameCamelCase:      util.EnsureCamelCase(name),
			MethodIDVarName:          methodIDVarName(svc.Name, name),
			MethodID:                 methodID,
			MethodRequestStructName:  methodArgType,
			MethodResponseStructName: methodRetType,
			ReturnsStream:            method.ReturnsStream,
			StreamStructName:         streamStructName,
			StreamInterfaceName:      streamInterfaceName,
			StreamNamePascalCase:     util.EnsurePascalCase(method.StreamName),
			HasClientMethods:         hasClientMethods,
			HasServerMethods:         hasServerMethods,
		})
	}

	buf := &bytes.Buffer{}
	err = serverTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
