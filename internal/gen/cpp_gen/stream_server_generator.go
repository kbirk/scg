package cpp_gen

import (
	"bytes"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

type StreamServerMethodArgs struct {
	MethodNameCamelCase      string
	MethodNamePascalCase     string
	MethodIDVarName          string
	MethodID                 uint64
	MethodRequestStructName  string
	MethodResponseStructName string
}

type StreamServerArgs struct {
	StreamNamePascalCase   string
	StreamNameCamelCase    string
	StreamIDVarName        string
	StreamID               uint64
	StreamServerClassName  string
	StreamHandlerClassName string
	ServerMethods          []StreamServerMethodArgs
	ClientMethods          []StreamServerMethodArgs
	HasServerMethods       bool
	HasClientMethods       bool
}

const streamServerTemplateStr = `
class {{.StreamServerClassName}};

class {{.StreamHandlerClassName}} {
public:
	virtual ~{{.StreamHandlerClassName}}() = default;
{{if .HasServerMethods}}
	// Server-side methods (server sends to client)
{{range .ServerMethods}}
	virtual std::pair<{{.MethodResponseStructName}}, scg::error::Error> {{.MethodNameCamelCase}}(scg::context::Context& ctx, {{$.StreamServerClassName}}& stream, const {{.MethodRequestStructName}}& req) = 0;
{{end}}
{{end}}
{{if .HasClientMethods}}
	// Client-side methods (receive from client) - implement these
{{range .ClientMethods}}
	virtual std::pair<{{.MethodResponseStructName}}, scg::error::Error> handle{{.MethodNamePascalCase}}(scg::context::Context& ctx, {{$.StreamServerClassName}}& stream, const {{.MethodRequestStructName}}& req) = 0;
{{end}}
{{end}}
};

class {{.StreamServerClassName}} {
public:
	inline explicit
	{{.StreamServerClassName}}(std::shared_ptr<scg::rpc::Stream> stream) : stream_(stream) {}

	virtual ~{{.StreamServerClassName}}() = default;
{{if .HasServerMethods}}
	// Server-side methods (server sends to client)
{{range .ServerMethods}}
	inline std::pair<{{.MethodResponseStructName}}, scg::error::Error> {{.MethodNameCamelCase}}(scg::context::Context& ctx, const {{.MethodRequestStructName}}& req)
	{
		{{.MethodResponseStructName}} resp;
		auto [reader, err] = stream_->sendMessage(ctx, {{.MethodIDVarName}}, req);
		if (err) {
			return std::make_pair({{.MethodResponseStructName}}{}, err);
		}

		err = reader.read(resp);
		if (err) {
			return std::make_pair({{.MethodResponseStructName}}{}, err);
		}

		return std::make_pair(resp, nullptr);
	}
{{end}}
{{end}}
	inline scg::error::Error close()
	{
		return stream_->close();
	}

	inline std::future<void> wait()
	{
		return stream_->wait();
	}

protected:
	std::shared_ptr<scg::rpc::Stream> stream_;
};
`

var (
	streamServerTemplate = template.Must(template.New("streamServerTemplateCpp").Parse(streamServerTemplateStr))
)

func getStreamServerClassName(streamName string) string {
	return util.EnsurePascalCase(streamName) + "StreamServer"
}

func getStreamHandlerClassName(streamName string) string {
	return util.EnsurePascalCase(streamName) + "StreamHandler"
}

func generateStreamServerCppCode(pkg *parse.Package, stream *parse.StreamDefinition) (string, error) {
	streamID, err := pkg.HashStringToStreamID(stream.Name)
	if err != nil {
		return "", err
	}

	args := StreamServerArgs{
		StreamNamePascalCase:   util.EnsurePascalCase(stream.Name),
		StreamNameCamelCase:    util.EnsureCamelCase(stream.Name),
		StreamIDVarName:        streamIDVarName(stream.Name),
		StreamID:               streamID,
		StreamServerClassName:  getStreamServerClassName(stream.Name),
		StreamHandlerClassName: getStreamHandlerClassName(stream.Name),
		ServerMethods:          []StreamServerMethodArgs{},
		ClientMethods:          []StreamServerMethodArgs{},
		HasServerMethods:       false,
		HasClientMethods:       false,
	}

	for name, method := range stream.Methods {
		methodID, err := pkg.HashStringToStreamMethodID(stream.Name, name)
		if err != nil {
			return "", err
		}

		argType, err := mapDataTypeDefinitionToCppType(method.Argument)
		if err != nil {
			return "", err
		}
		retType, err := mapDataTypeDefinitionToCppType(method.Return)
		if err != nil {
			return "", err
		}

		methodArgs := StreamServerMethodArgs{
			MethodNameCamelCase:      util.EnsureCamelCase(name),
			MethodNamePascalCase:     util.EnsurePascalCase(name),
			MethodIDVarName:          streamMethodIDVarName(stream.Name, name),
			MethodID:                 methodID,
			MethodRequestStructName:  argType,
			MethodResponseStructName: retType,
		}

		if method.Direction == parse.StreamMethodDirectionServer {
			args.ServerMethods = append(args.ServerMethods, methodArgs)
			args.HasServerMethods = true
		} else if method.Direction == parse.StreamMethodDirectionClient {
			args.ClientMethods = append(args.ClientMethods, methodArgs)
			args.HasClientMethods = true
		}
	}

	buf := &bytes.Buffer{}
	err = streamServerTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
