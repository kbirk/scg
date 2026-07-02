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

type ClientStreamMethodArgs struct {
	MethodNamePascalCase string
	MethodIDVarName      string
	Kind                 string // "bidi" | "server" | "client"
	StreamTypeName       string
	ReqStructName        string // request (argument) message
	RespStructName       string // response (return) message
}

type ClientArgs struct {
	ClientNamePascalCase string
	ClientNameCamelCase  string
	ServiceIDVarName     string
	ServiceID            uint64
	ClientMethods        []ClientMethodArgs
	ClientStreamMethods  []ClientStreamMethodArgs
}

const clientTemplateStr = `
// {{.ClientNamePascalCase}}Api is the abstract call surface of the {{.ClientNamePascalCase}} service,
// mirroring the client call shape one-to-one. {{.ClientNamePascalCase}}Client is the RPC-backed
// implementation; tests (or alternative transports) substitute their own. Only unary rpcs are part
// of the interface; streaming rpcs stay on the concrete client.
type {{.ClientNamePascalCase}}Api interface { {{- range .ClientMethods}}
	{{.MethodNamePascalCase}}(ctx context.Context, req *{{.MethodRequestStructName}}) (*{{.MethodResponseStructName}}, error){{end}}
}

type {{.ClientNamePascalCase}}Client struct {
	client *rpc.Client
}

var _ {{.ClientNamePascalCase}}Api = (*{{.ClientNamePascalCase}}Client)(nil) // compile-time conformance

func New{{.ClientNamePascalCase}}Client(client *rpc.Client) *{{.ClientNamePascalCase}}Client {
	return &{{.ClientNamePascalCase}}Client{
		client: client,
	}
}

{{range .ClientMethods}}
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
{{end}}
{{range .ClientStreamMethods}}
// {{.StreamTypeName}} is the client handle for the {{.MethodNamePascalCase}} stream.
type {{.StreamTypeName}} struct {
	stream *rpc.ClientStream
}
{{if eq .Kind "bidi"}}
// Send writes a message to the server. It does not block on the peer.
func (s *{{.StreamTypeName}}) Send(req *{{.ReqStructName}}) error {
	return s.stream.Send(req)
}

// Recv blocks for the next server message; returns io.EOF on a clean close.
func (s *{{.StreamTypeName}}) Recv() (*{{.RespStructName}}, error) {
	reader, err := s.stream.Recv()
	if err != nil {
		return nil, err
	}
	resp := &{{.RespStructName}}{}
	if err := resp.Deserialize(reader); err != nil {
		return nil, err
	}
	return resp, nil
}

// CloseSend signals that the client is done sending; it may still receive.
func (s *{{.StreamTypeName}}) CloseSend() error {
	return s.stream.CloseSend()
}
{{else if eq .Kind "server"}}
// Recv blocks for the next server message; returns io.EOF on a clean close.
func (s *{{.StreamTypeName}}) Recv() (*{{.RespStructName}}, error) {
	reader, err := s.stream.Recv()
	if err != nil {
		return nil, err
	}
	resp := &{{.RespStructName}}{}
	if err := resp.Deserialize(reader); err != nil {
		return nil, err
	}
	return resp, nil
}
{{else}}
// Send writes a message to the server. It does not block on the peer.
func (s *{{.StreamTypeName}}) Send(req *{{.ReqStructName}}) error {
	return s.stream.Send(req)
}

// CloseAndRecv half-closes the send direction and blocks for the single response.
func (s *{{.StreamTypeName}}) CloseAndRecv() (*{{.RespStructName}}, error) {
	if err := s.stream.CloseSend(); err != nil {
		return nil, err
	}
	reader, err := s.stream.Recv()
	if err != nil {
		return nil, err
	}
	resp := &{{.RespStructName}}{}
	if err := resp.Deserialize(reader); err != nil {
		return nil, err
	}
	return resp, nil
}
{{end}}
// Context returns the context the stream was opened with.
func (s *{{.StreamTypeName}}) Context() context.Context {
	return s.stream.Context()
}
{{if eq .Kind "server"}}
func (c *{{$.ClientNamePascalCase}}Client) {{.MethodNamePascalCase}}(ctx context.Context, req *{{.ReqStructName}}) (*{{.StreamTypeName}}, error) {
	stream, err := c.client.OpenStream(ctx, {{$.ServiceIDVarName}}, {{.MethodIDVarName}})
	if err != nil {
		return nil, err
	}
	if err := stream.Send(req); err != nil {
		return nil, err
	}
	if err := stream.CloseSend(); err != nil {
		return nil, err
	}
	return &{{.StreamTypeName}}{stream: stream}, nil
}
{{else}}
func (c *{{$.ClientNamePascalCase}}Client) {{.MethodNamePascalCase}}(ctx context.Context) (*{{.StreamTypeName}}, error) {
	stream, err := c.client.OpenStream(ctx, {{$.ServiceIDVarName}}, {{.MethodIDVarName}})
	if err != nil {
		return nil, err
	}
	return &{{.StreamTypeName}}{stream: stream}, nil
}
{{end}}
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
		ClientStreamMethods:  []ClientStreamMethodArgs{},
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
			args.ClientStreamMethods = append(args.ClientStreamMethods, ClientStreamMethodArgs{
				MethodNamePascalCase: util.EnsurePascalCase(name),
				MethodIDVarName:      methodIDVarName(svc.Name, name),
				Kind:                 streamKind(method),
				StreamTypeName:       streamClientTypeName(svc.Name, name),
				ReqStructName:        methodArgType,
				RespStructName:       methodRetType,
			})
			continue
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
