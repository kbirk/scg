package cpp_gen

import (
	"bytes"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

type StreamClientMethodArgs struct {
	MethodNameCamelCase      string
	MethodNamePascalCase     string
	MethodIDVarName          string
	MethodID                 uint64
	MethodRequestStructName  string
	MethodResponseStructName string
}

type StreamClientArgs struct {
	StreamNamePascalCase  string
	StreamNameCamelCase   string
	StreamIDVarName       string
	StreamID              uint64
	StreamClientClassName string
	ClientMethods         []StreamClientMethodArgs
	ServerMethods         []StreamClientMethodArgs
	HasClientMethods      bool
	HasServerMethods      bool
}

const streamClientTemplateStr = `
static constexpr uint64_t {{.StreamIDVarName}} = {{.StreamID}}UL;{{- range .ClientMethods}}
static constexpr uint64_t {{.MethodIDVarName}} = {{.MethodID}}UL;{{end}}{{range .ServerMethods}}
static constexpr uint64_t {{.MethodIDVarName}} = {{.MethodID}}UL;{{end}}

class {{.StreamClientClassName}}{{if .HasServerMethods}} : public scg::rpc::MessageProcessor{{end}} {
public:
	inline explicit
	{{.StreamClientClassName}}(std::shared_ptr<scg::rpc::Stream> stream) : stream_(stream) {
{{if .HasServerMethods}}		stream_->setProcessor(this);
{{end}}	}

	virtual ~{{.StreamClientClassName}}() = default;
{{if .HasServerMethods}}
	// MessageProcessor interface implementation
	inline std::pair<std::vector<uint8_t>, scg::error::Error> processMessage(uint64_t methodID, scg::serialize::Reader& reader) override
	{
		scg::context::Context ctx = stream_->context();
		switch (methodID) {
{{range .ServerMethods}}
		case {{.MethodIDVarName}}:
		{
			{{.MethodRequestStructName}} req;
			auto err = reader.read(req);
			if (err) {
				return std::make_pair(std::vector<uint8_t>{}, err);
			}
			auto [resp, handlerErr] = handle{{.MethodNamePascalCase}}(ctx, req);
			if (handlerErr) {
				return std::make_pair(std::vector<uint8_t>{}, handlerErr);
			}
			scg::serialize::Writer writer;
			writer.write(resp);
			return std::make_pair(writer.bytes(), nullptr);
		}
{{end}}
		default:
			return std::make_pair(std::vector<uint8_t>{}, scg::error::Error("Unknown method ID"));
		}
	}

{{end}}{{if .HasClientMethods}}
	// Client-side methods (client sends to server)
{{range .ClientMethods}}
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
{{if .HasServerMethods}}
	// Server-side methods (receive from server) - override these in derived class
{{range .ServerMethods}}
	virtual std::pair<{{.MethodResponseStructName}}, scg::error::Error> handle{{.MethodNamePascalCase}}(scg::context::Context& ctx, const {{.MethodRequestStructName}}& req)
	{
		(void)ctx;
		(void)req;
		return std::make_pair({{.MethodResponseStructName}}{}, scg::error::Error("{{.MethodNamePascalCase}} handler not implemented"));
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
	streamClientTemplate = template.Must(template.New("streamClientTemplateCpp").Parse(streamClientTemplateStr))
)

func streamIDVarName(streamName string) string {
	return util.EnsureCamelCase(streamName) + "StreamID"
}

func streamMethodIDVarName(streamName string, methodName string) string {
	return util.EnsureCamelCase(streamName) + "Stream_" + util.EnsurePascalCase(methodName) + "ID"
}

func getStreamClientClassName(streamName string) string {
	return util.EnsurePascalCase(streamName) + "StreamClient"
}

func generateStreamClientCppCode(pkg *parse.Package, stream *parse.StreamDefinition) (string, error) {
	streamID, err := pkg.HashStringToStreamID(stream.Name)
	if err != nil {
		return "", err
	}

	args := StreamClientArgs{
		StreamNamePascalCase:  util.EnsurePascalCase(stream.Name),
		StreamNameCamelCase:   util.EnsureCamelCase(stream.Name),
		StreamIDVarName:       streamIDVarName(stream.Name),
		StreamID:              streamID,
		StreamClientClassName: getStreamClientClassName(stream.Name),
		ClientMethods:         []StreamClientMethodArgs{},
		ServerMethods:         []StreamClientMethodArgs{},
		HasClientMethods:      false,
		HasServerMethods:      false,
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

		methodArgs := StreamClientMethodArgs{
			MethodNameCamelCase:      util.EnsureCamelCase(name),
			MethodNamePascalCase:     util.EnsurePascalCase(name),
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
