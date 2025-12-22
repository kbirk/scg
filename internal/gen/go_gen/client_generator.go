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
	ReturnsStream            bool
	StreamStructName         string
	StreamNamePascalCase     string
	StreamHandlerInterface   string
	HasServerMethods         bool
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

{{range .ClientMethods}}{{if .ReturnsStream}}
func (c *{{$.ClientNamePascalCase}}Client) {{.MethodNamePascalCase}}(ctx context.Context{{if .HasServerMethods}}, handler {{.StreamHandlerInterface}}{{end}}, req *{{.MethodRequestStructName}}) (*{{.StreamStructName}}, error) {
	stream, err := c.client.OpenStream(ctx, {{$.ServiceIDVarName}}, {{.MethodIDVarName}}, req)
	if err != nil {
		return nil, err
	}
	streamClient := New{{.StreamNamePascalCase}}StreamClient(stream{{if .HasServerMethods}}, handler{{end}})
	{{if .HasServerMethods}}stream.SetProcessor(streamClient){{end}}
	return streamClient, nil
}
{{else}}
func (c *{{$.ClientNamePascalCase}}Client) {{.MethodNamePascalCase}}(ctx context.Context, req *{{.MethodRequestStructName}}) (*{{.MethodResponseStructName}}, error) {

	handler := func (ctx context.Context, req rpc.Message) (rpc.Message, error) {
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

	middleware := c.client.GetMiddleware()
	resp, err := rpc.ApplyHandlerChain(ctx, req, middleware, handler)
	if err != nil {
		return nil, err
	}
	r, ok := resp.(*{{.MethodResponseStructName}})
	if !ok {
		return nil, fmt.Errorf("invalid response type %T", resp)
	}
	return r, nil
}
{{end}}{{end}}
`

var (
	clientTemplate = template.Must(template.New("clientTemplateGo").Parse(clientTemplateStr))
)

func getStreamClientHandlerInterfaceName(streamName string) string {
	return util.EnsurePascalCase(streamName) + "StreamHandler"
}

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

		var streamStructName, streamHandlerInterface string
		var methodArgType, methodRetType string
		var hasServerMethods bool

		if method.ReturnsStream {
			// For stream returns, use the stream client struct name
			streamStructName = getStreamClientStructName(method.StreamName)
			streamHandlerInterface = getStreamClientHandlerInterfaceName(method.StreamName)
			methodArgType, err = mapDataTypeDefinitionToGoType(method.Argument)
			if err != nil {
				return "", err
			}
			// Return type is not used for stream methods
			methodRetType = ""

			// Get stream definition to check for server methods (which client must handle)
			stream, ok := pkg.StreamDefinitions[method.StreamName]
			if ok {
				for _, streamMethod := range stream.Methods {
					if streamMethod.Direction == parse.StreamMethodDirectionServer {
						hasServerMethods = true
						break
					}
				}
			}
		} else {
			methodArgType, methodRetType, err = generateServiceMethodParams(method)
			if err != nil {
				return "", err
			}
		}

		args.ClientMethods = append(args.ClientMethods, ClientMethodArgs{
			MethodNamePascalCase:     util.EnsurePascalCase(name),
			MethodNameCamelCase:      util.EnsureCamelCase(name),
			MethodIDVarName:          methodIDVarName(svc.Name, name),
			MethodID:                 methodID,
			MethodRequestStructName:  methodArgType,
			MethodResponseStructName: methodRetType,
			ReturnsStream:            method.ReturnsStream,
			StreamStructName:         streamStructName,
			StreamNamePascalCase:     util.EnsurePascalCase(method.StreamName),
			StreamHandlerInterface:   streamHandlerInterface,
			HasServerMethods:         hasServerMethods,
		})
	}

	buf := &bytes.Buffer{}
	err = clientTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
