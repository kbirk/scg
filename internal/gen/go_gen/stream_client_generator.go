package go_gen

import (
	"bytes"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

type StreamClientMethodArgs struct {
	MethodNamePascalCase     string
	MethodNameCamelCase      string
	MethodIDVarName          string
	MethodID                 uint64
	MethodRequestStructName  string
	MethodResponseStructName string
}

type StreamClientArgs struct {
	StreamNamePascalCase   string
	StreamNameCamelCase    string
	ServiceIDVarName       string
	ServiceID              uint64
	StreamIDVarName        string
	StreamID               uint64
	StreamClientStructName string
	ClientMethods          []StreamClientMethodArgs
	ServerMethods          []StreamClientMethodArgs
	HasClientMethods       bool
	HasServerMethods       bool
}

const streamClientTemplateStr = `
{{if .HasServerMethods}}
type {{.StreamNamePascalCase}}StreamServerHandler interface { {{- range .ServerMethods}}
	{{.MethodNamePascalCase}}(context.Context, *{{.MethodRequestStructName}}) (*{{.MethodResponseStructName}}, error){{end}}
}
{{end}}

type {{.StreamClientStructName}} struct {
	stream *rpc.Stream{{if .HasServerMethods}}
	handler {{.StreamNamePascalCase}}StreamServerHandler{{end}}
}

func New{{.StreamNamePascalCase}}StreamClient(stream *rpc.Stream{{if .HasServerMethods}}, handler {{.StreamNamePascalCase}}StreamServerHandler{{end}}) *{{.StreamClientStructName}} {
	return &{{.StreamClientStructName}}{
		stream: stream,{{if .HasServerMethods}}
		handler: handler,{{end}}
	}
}
{{if .HasClientMethods}}
// Client-side methods (client sends to server)
{{range .ClientMethods}}
func (s *{{$.StreamClientStructName}}) {{.MethodNamePascalCase}}(ctx context.Context, req *{{.MethodRequestStructName}}) (*{{.MethodResponseStructName}}, error) {
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
{{if .HasServerMethods}}
// Server-side methods (receive from server)

// ProcessMessage implements the rpc.MessageProcessor interface
func (s *{{$.StreamClientStructName}}) ProcessMessage(methodID uint64, reader *serialize.Reader) (rpc.Message, error) {
	switch methodID { {{- range .ServerMethods}}
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

func (s *{{$.StreamClientStructName}}) Close() error {
	return s.stream.Close()
}

func (s *{{$.StreamClientStructName}}) Wait() <-chan struct{} {
	return s.stream.Wait()
}
`

var (
	streamClientTemplate = template.Must(template.New("streamClientTemplateGo").Parse(streamClientTemplateStr))
)

func streamIDVarName(streamName string) string {
	return util.EnsureCamelCase(streamName) + "StreamID"
}

func streamMethodIDVarName(streamName string, methodName string) string {
	return util.EnsureCamelCase(streamName) + "Stream_" + util.EnsurePascalCase(methodName) + "ID"
}

func getStreamClientStructName(streamName string) string {
	return util.EnsurePascalCase(streamName) + "StreamClient"
}

func generateStreamClientGoCode(pkg *parse.Package, stream *parse.StreamDefinition) (string, error) {
	streamID, err := pkg.HashStringToStreamID(stream.Name)
	if err != nil {
		return "", err
	}

	args := StreamClientArgs{
		StreamNamePascalCase:   util.EnsurePascalCase(stream.Name),
		StreamNameCamelCase:    util.EnsureCamelCase(stream.Name),
		ServiceIDVarName:       "0", // Streams are not tied to a service ID directly
		ServiceID:              0,
		StreamIDVarName:        streamIDVarName(stream.Name),
		StreamID:               streamID,
		StreamClientStructName: getStreamClientStructName(stream.Name),
		ClientMethods:          []StreamClientMethodArgs{},
		ServerMethods:          []StreamClientMethodArgs{},
		HasClientMethods:       false,
		HasServerMethods:       false,
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

		methodArgs := StreamClientMethodArgs{
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
	err = streamClientTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
