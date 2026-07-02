package cpp_gen

import (
	"bytes"
	"fmt"
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
	MethodNameCamelCase string
	MethodIDVarName     string
	MethodID            uint64
	Kind                string // "bidi" | "server" | "client"
	StreamTypeName      string
	ReqStructName       string // request (argument) message
	RespStructName      string // response (return) message
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
{{- define "cppClientRecv"}}
	inline scg::rpc::StreamRecv<{{.RespStructName}}> tryRecv()
	{
		scg::rpc::StreamRecv<{{.RespStructName}}> res;
		scg::serialize::Reader reader({});
		res.state = stream_->tryRecv(reader, res.error);
		if (res.state == scg::rpc::StreamRecvState::Message) {
			auto err = reader.read(res.message);
			if (err) {
				res.state = scg::rpc::StreamRecvState::Closed;
				res.error = err;
			}
		}
		return res;
	}

	inline scg::rpc::StreamRecv<{{.RespStructName}}> recv()
	{
		scg::rpc::StreamRecv<{{.RespStructName}}> res;
		scg::serialize::Reader reader({});
		res.state = stream_->recv(reader, res.error);
		if (res.state == scg::rpc::StreamRecvState::Message) {
			auto err = reader.read(res.message);
			if (err) {
				res.state = scg::rpc::StreamRecvState::Closed;
				res.error = err;
			}
		}
		return res;
	}
{{- end}}
static constexpr uint64_t {{.ServiceIDVarName}} = {{.ServiceID}}UL;{{- range .ClientMethods}}
static constexpr uint64_t {{.MethodIDVarName}} = {{.MethodID}}UL;{{end}}{{range .ClientStreamMethods}}
static constexpr uint64_t {{.MethodIDVarName}} = {{.MethodID}}UL;{{end}}
{{range .ClientStreamMethods}}
// {{.StreamTypeName}} is the client handle for the {{.MethodNameCamelCase}} stream.
// Receiving is non-blocking via tryRecv() (drain from a game loop); a blocking
// recv() is also provided. Sending is non-blocking.
class {{.StreamTypeName}} {
public:
	explicit {{.StreamTypeName}}(std::shared_ptr<scg::rpc::ClientStream> stream) : stream_(stream) {}
{{if eq .Kind "bidi"}}
	inline scg::error::Error send(const {{.ReqStructName}}& msg)
	{
		return stream_->send(msg);
	}
{{template "cppClientRecv" .}}
	inline scg::error::Error closeSend()
	{
		return stream_->closeSend();
	}
{{else if eq .Kind "server"}}
{{template "cppClientRecv" .}}{{else}}
	inline scg::error::Error send(const {{.ReqStructName}}& msg)
	{
		return stream_->send(msg);
	}

	// closeAndRecv half-closes the send direction and blocks for the single response.
	inline std::pair<{{.RespStructName}}, scg::error::Error> closeAndRecv()
	{
		auto err = stream_->closeSend();
		if (err) {
			return std::make_pair({{.RespStructName}}{}, err);
		}
		scg::serialize::Reader reader({});
		scg::error::Error recvErr;
		auto state = stream_->recv(reader, recvErr);
		if (state != scg::rpc::StreamRecvState::Message) {
			if (recvErr) {
				return std::make_pair({{.RespStructName}}{}, recvErr);
			}
			return std::make_pair({{.RespStructName}}{}, scg::error::Error("stream closed before response"));
		}
		{{.RespStructName}} resp;
		auto derr = reader.read(resp);
		if (derr) {
			return std::make_pair({{.RespStructName}}{}, derr);
		}
		return std::make_pair(resp, nullptr);
	}
{{end}}
	inline const scg::context::Context& context() const
	{
		return stream_->context();
	}

	// cancel terminates the stream from the client side, notifying the server so
	// its handler is torn down and failing a blocked recv() with a cancelled error.
	inline scg::error::Error cancel()
	{
		return stream_->cancel();
	}

private:
	std::shared_ptr<scg::rpc::ClientStream> stream_;
};
{{end}}
// {{.ClientNamePascalCase}}Api is the abstract call surface of the {{.ClientNamePascalCase}} service,
// mirroring the client call shape one-to-one. {{.ClientNamePascalCase}}Client is the RPC-backed
// implementation; tests (or alternative transports) substitute their own. Symmetric to the
// server-side {{.ClientNamePascalCase}}Server interface. Only unary rpcs are part of the interface;
// streaming rpcs stay on the concrete client.
class {{.ClientNamePascalCase}}Api {
public:
	virtual ~{{.ClientNamePascalCase}}Api() = default;
	{{range .ClientMethods}}
	virtual std::pair<{{.MethodResponseStructName}}, scg::error::Error> {{.MethodNameCamelCase}}(scg::context::Context& ctx, const {{.MethodRequestStructName}}& req) = 0;
	{{end}}
};

class {{.ClientNamePascalCase}}Client final : public {{.ClientNamePascalCase}}Api {
public:
	inline explicit
	{{.ClientNamePascalCase}}Client(scg::rpc::Client* client) : client_(client) {}

	inline explicit
	{{.ClientNamePascalCase}}Client(std::shared_ptr<scg::rpc::Client> client) : client_(client) {}
	{{range .ClientMethods}}
	inline std::pair<{{.MethodResponseStructName}}, scg::error::Error> {{.MethodNameCamelCase}}(scg::context::Context& ctx, const {{.MethodRequestStructName}}& req) override
	{
		{{.MethodResponseStructName}} resp;
		auto err = {{.MethodNameCamelCase}}(&resp, ctx, req);
		return std::pair(resp, err);
	}

	inline scg::error::Error {{.MethodNameCamelCase}}({{.MethodResponseStructName}}* resp, scg::context::Context& c, const {{.MethodRequestStructName}}& req) const
	{
		auto handler = [this, req, resp](scg::context::Context& ctx, const scg::type::Message& r) -> std::pair<std::shared_ptr<scg::type::Message>, scg::error::Error> {
			auto [reader, err] = client_->call(ctx, {{$.ServiceIDVarName}}, {{.MethodIDVarName}}, req);
			if (err) {
				return std::make_pair(nullptr, err);
			}

			err = reader.read(*resp);
			if (err) {
				return std::make_pair(nullptr, err);
			}

			return std::make_pair(std::shared_ptr<scg::type::Message>(resp, [](scg::type::Message*){}), nullptr);
		};

		auto& middleware = client_->middleware();
		return scg::middleware::applyHandlerChain(c, req, middleware, handler).second;
	}
	{{end}}{{range .ClientStreamMethods}}
	{{if eq .Kind "server"}}inline std::pair<std::shared_ptr<{{.StreamTypeName}}>, scg::error::Error> {{.MethodNameCamelCase}}(const scg::context::Context& ctx, const {{.ReqStructName}}& req) const
	{
		auto [stream, err] = client_->openStream(ctx, {{$.ServiceIDVarName}}, {{.MethodIDVarName}});
		if (err) {
			return std::make_pair(nullptr, err);
		}
		auto handle = std::make_shared<{{.StreamTypeName}}>(stream);
		err = stream->send(req);
		if (err) {
			return std::make_pair(nullptr, err);
		}
		err = stream->closeSend();
		if (err) {
			return std::make_pair(nullptr, err);
		}
		return std::make_pair(handle, nullptr);
	}{{else}}inline std::pair<std::shared_ptr<{{.StreamTypeName}}>, scg::error::Error> {{.MethodNameCamelCase}}(const scg::context::Context& ctx) const
	{
		auto [stream, err] = client_->openStream(ctx, {{$.ServiceIDVarName}}, {{.MethodIDVarName}});
		if (err) {
			return std::make_pair(nullptr, err);
		}
		return std::make_pair(std::make_shared<{{.StreamTypeName}}>(stream), nullptr);
	}{{end}}
	{{end}}

private:
	std::shared_ptr<scg::rpc::Client> client_;
};

// {{.ClientNamePascalCase}}Fake is a test double for {{.ClientNamePascalCase}}Api: each method invokes
// its hook when set, otherwise returns a default-constructed response with no error. Deliberately
// minimal — no built-in call recording or thread-safety; layer those per the test's own needs.
class {{.ClientNamePascalCase}}Fake : public {{.ClientNamePascalCase}}Api {
public:
	{{range .ClientMethods}}std::function<std::pair<{{.MethodResponseStructName}}, scg::error::Error>(scg::context::Context&, const {{.MethodRequestStructName}}&)> on{{.MethodNamePascalCase}};
	{{end}}{{range .ClientMethods}}
	std::pair<{{.MethodResponseStructName}}, scg::error::Error> {{.MethodNameCamelCase}}(scg::context::Context& ctx, const {{.MethodRequestStructName}}& req) override
	{
		if (on{{.MethodNamePascalCase}}) {
			return on{{.MethodNamePascalCase}}(ctx, req);
		}
		return { {{.MethodResponseStructName}}{}, nullptr };
	}
	{{end}}
};
`

var (
	clientTemplate = template.Must(template.New("clientTemplateCpp").Parse(clientTemplateStr))
)

func serviceIDVarName(serviceName string) string {
	return fmt.Sprintf("%sID", util.EnsureCamelCase(serviceName))
}

func methodIDVarName(serviceName string, methodName string) string {
	return fmt.Sprintf("%s_%sID", util.EnsureCamelCase(serviceName), util.EnsurePascalCase(methodName))
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

	argType, err := mapDataTypeDefinitionToCppType(method.Argument)
	if err != nil {
		return "", "", err
	}
	retType, err := mapDataTypeDefinitionToCppType(method.Return)
	if err != nil {
		return "", "", err
	}
	return argType, retType, nil
}

func generateClientCppCode(pkg *parse.Package, svc *parse.ServiceDefinition) (string, error) {

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
				MethodNameCamelCase: util.EnsureCamelCase(name),
				MethodIDVarName:     methodIDVarName(svc.Name, name),
				MethodID:            methodID,
				Kind:                streamKind(method),
				StreamTypeName:      streamClientTypeName(svc.Name, name),
				ReqStructName:       methodArgType,
				RespStructName:      methodRetType,
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
