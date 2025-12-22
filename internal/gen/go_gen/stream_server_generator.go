package go_gen

import (
	"bytes"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

type StreamServerMethodArgs struct {
	MethodNamePascalCase     string
	MethodNameCamelCase      string
	MethodIDVarName          string
	MethodID                 uint64
	MethodRequestStructName  string
	MethodResponseStructName string
}

type StreamServerArgs struct {
	StreamNamePascalCase      string
	StreamNameCamelCase       string
	StreamIDVarName           string
	StreamID                  uint64
	StreamServerInterfaceName string
	StreamServerStructName    string
	ClientMethods             []StreamServerMethodArgs
	ServerMethods             []StreamServerMethodArgs
	HasClientMethods          bool
	HasServerMethods          bool
}

const streamServerTemplateStr = `
const (
	{{.StreamIDVarName}} uint64 = {{.StreamID}} {{- range .ClientMethods}}
	{{.MethodIDVarName}} uint64 = {{.MethodID}}{{end}}{{range .ServerMethods}}
	{{.MethodIDVarName}} uint64 = {{.MethodID}}{{end}}
)
{{if .HasClientMethods}}
type {{.StreamNamePascalCase}}StreamClientHandler interface { {{- range .ClientMethods}}
	{{.MethodNamePascalCase}}(context.Context, *{{.MethodRequestStructName}}) (*{{.MethodResponseStructName}}, error){{end}}
}
{{end}}
{{if .HasServerMethods}}
type {{.StreamServerInterfaceName}} interface { {{- range .ServerMethods}}
	{{.MethodNamePascalCase}}(context.Context, *{{.MethodRequestStructName}}) (*{{.MethodResponseStructName}}, error){{end}}
}
{{end}}

type {{.StreamServerStructName}} struct {
	stream *rpc.Stream{{if .HasClientMethods}}
	handler {{.StreamNamePascalCase}}StreamClientHandler{{end}}
}

func New{{.StreamNamePascalCase}}StreamServer({{if .HasClientMethods}}handler {{.StreamNamePascalCase}}StreamClientHandler{{end}}) *{{.StreamServerStructName}} {
	return &{{.StreamServerStructName}}{
		stream: nil,{{if .HasClientMethods}}
		handler: handler,{{end}}
	}
}

// setStream sets the internal RPC stream (called by generated code)
func (s *{{.StreamServerStructName}}) setStream(stream *rpc.Stream) {
	s.stream = stream
}
{{if .HasClientMethods}}
// Client-side methods (receive from client)

// ProcessMessage implements the rpc.MessageProcessor interface
func (s *{{$.StreamServerStructName}}) ProcessMessage(methodID uint64, reader *serialize.Reader) (rpc.Message, error) {
	switch methodID { {{- range .ClientMethods}}
	case {{.MethodIDVarName}}:
		req := &{{.MethodRequestStructName}}{}
		err := req.Deserialize(reader)
		if err != nil {
			return nil, err
		}
		return s.handler.{{.MethodNamePascalCase}}(context.Background(), req){{end}}
	default:
		return nil, fmt.Errorf("unrecognized stream method ID: %d", methodID)
	}
}
{{end}}
{{if .HasServerMethods}}
// Server-side methods (server sends to client)
{{range .ServerMethods}}
func (s *{{$.StreamServerStructName}}) {{.MethodNamePascalCase}}(ctx context.Context, req *{{.MethodRequestStructName}}) (*{{.MethodResponseStructName}}, error) {
	reader, err := s.stream.SendMessage(ctx, {{.MethodIDVarName}}, req)
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
{{end}}

func (s *{{$.StreamServerStructName}}) Close() error {
	return s.stream.Close()
}

func (s *{{$.StreamServerStructName}}) Wait() <-chan struct{} {
	return s.stream.Wait()
}
`

var (
	streamServerTemplate = template.Must(template.New("streamServerTemplateGo").Parse(streamServerTemplateStr))
)

func getStreamServerInterfaceName(streamName string) string {
	return util.EnsurePascalCase(streamName) + "StreamHandler"
}

func getStreamServerStructName(streamName string) string {
	return util.EnsurePascalCase(streamName) + "StreamServer"
}

func generateStreamServerGoCode(pkg *parse.Package, stream *parse.StreamDefinition) (string, error) {
	streamID, err := pkg.HashStringToStreamID(stream.Name)
	if err != nil {
		return "", err
	}

	args := StreamServerArgs{
		StreamNamePascalCase:      util.EnsurePascalCase(stream.Name),
		StreamNameCamelCase:       util.EnsureCamelCase(stream.Name),
		StreamIDVarName:           streamIDVarName(stream.Name),
		StreamID:                  streamID,
		StreamServerInterfaceName: getStreamServerInterfaceName(stream.Name),
		StreamServerStructName:    getStreamServerStructName(stream.Name),
		ClientMethods:             []StreamServerMethodArgs{},
		ServerMethods:             []StreamServerMethodArgs{},
		HasClientMethods:          false,
		HasServerMethods:          false,
	}

	for name, method := range stream.Methods {
		methodID, err := pkg.HashStringToStreamMethodID(stream.Name, name)
		if err != nil {
			return "", err
		}

		argType, err := mapDataTypeDefinitionToGoType(method.Argument)
		if err != nil {
			return "", err
		}
		retType, err := mapDataTypeDefinitionToGoType(method.Return)
		if err != nil {
			return "", err
		}

		methodArgs := StreamServerMethodArgs{
			MethodNamePascalCase:     util.EnsurePascalCase(name),
			MethodNameCamelCase:      util.EnsureCamelCase(name),
			MethodIDVarName:          streamMethodIDVarName(stream.Name, name),
			MethodID:                 methodID,
			MethodRequestStructName:  argType,
			MethodResponseStructName: retType,
		}

		if method.Direction == parse.StreamMethodDirectionClient {
			args.ClientMethods = append(args.ClientMethods, methodArgs)
			args.HasClientMethods = true
		} else if method.Direction == parse.StreamMethodDirectionServer {
			args.ServerMethods = append(args.ServerMethods, methodArgs)
			args.HasServerMethods = true
		}
	}

	buf := &bytes.Buffer{}
	err = streamServerTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
