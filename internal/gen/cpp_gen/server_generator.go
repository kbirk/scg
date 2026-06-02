package cpp_gen

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

type ServerMethodArgs struct {
	MethodNamePascalCase     string
	MethodNameCamelCase      string
	MethodIDVarName          string
	MethodID                 uint64
	MethodRequestStructName  string
	MethodResponseStructName string
}

type ServerStreamMethodArgs struct {
	MethodNamePascalCase string
	MethodNameCamelCase  string
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
	ServerStubClassName  string
	ServiceID            uint64
	ServerMethods        []ServerMethodArgs
	ServerStreamMethods  []ServerStreamMethodArgs
}

const serverTemplateStr = `
{{- define "cppServerRecv"}}
	inline scg::rpc::StreamRecv<{{.ReqStructName}}> recv()
	{
		scg::rpc::StreamRecv<{{.ReqStructName}}> res;
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

	inline scg::rpc::StreamRecv<{{.ReqStructName}}> tryRecv()
	{
		scg::rpc::StreamRecv<{{.ReqStructName}}> res;
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
{{- end}}
static constexpr uint64_t {{.ServiceIDVarName}} = {{.ServiceID}}UL;{{- range .ServerMethods}}
static constexpr uint64_t {{.MethodIDVarName}} = {{.MethodID}}UL;{{end}}{{range .ServerStreamMethods}}
static constexpr uint64_t {{.MethodIDVarName}} = {{.MethodID}}UL;{{end}}
{{range .ServerStreamMethods}}
// {{.StreamTypeName}} is the server handle for the {{.MethodNameCamelCase}} stream.
class {{.StreamTypeName}} {
public:
	explicit {{.StreamTypeName}}(std::shared_ptr<scg::rpc::ServerStream> stream) : stream_(stream) {}
{{if eq .Kind "bidi"}}{{template "cppServerRecv" .}}
	inline scg::error::Error send(const {{.RespStructName}}& msg)
	{
		return stream_->send(msg);
	}
{{else if eq .Kind "server"}}
	inline scg::error::Error send(const {{.RespStructName}}& msg)
	{
		return stream_->send(msg);
	}
{{else}}{{template "cppServerRecv" .}}{{end}}
	inline const scg::context::Context& context() const
	{
		return stream_->context();
	}

private:
	std::shared_ptr<scg::rpc::ServerStream> stream_;
};
{{end}}
class {{.ServerNamePascalCase}} {
public:
	virtual ~{{.ServerNamePascalCase}}() = default;
	{{range .ServerMethods}}
	virtual std::pair<{{.MethodResponseStructName}}, scg::error::Error> {{.MethodNameCamelCase}}(const scg::context::Context& ctx, const {{.MethodRequestStructName}}& req) = 0;
	{{end}}{{range .ServerStreamMethods}}
	{{if eq .Kind "bidi"}}virtual scg::error::Error {{.MethodNameCamelCase}}(std::shared_ptr<{{.StreamTypeName}}> stream) = 0;
	{{else if eq .Kind "server"}}virtual scg::error::Error {{.MethodNameCamelCase}}(const {{.ReqStructName}}& req, std::shared_ptr<{{.StreamTypeName}}> stream) = 0;
	{{else}}virtual std::pair<{{.RespStructName}}, scg::error::Error> {{.MethodNameCamelCase}}(std::shared_ptr<{{.StreamTypeName}}> stream) = 0;
	{{end}}{{end}}
};

class {{.ServerStubClassName}} {
public:
	{{.ServerStubClassName}}(std::shared_ptr<{{.ServerNamePascalCase}}> impl) : impl_(impl) {}

	{{range .ServerMethods}}
	std::vector<uint8_t> handle{{.MethodNamePascalCase}}(const scg::context::Context& ctx, const std::vector<scg::middleware::Middleware>& middleware, uint64_t requestID, scg::serialize::Reader& reader) {
		{{.MethodRequestStructName}} req;
		auto err = reader.read(req);
		if (err) {
			return scg::rpc::respondWithError(requestID, err);
		}

		auto handler = [this, &req](scg::context::Context& ctx, const scg::type::Message& r) -> std::pair<std::shared_ptr<scg::type::Message>, scg::error::Error> {
			auto [resp, err] = impl_->{{.MethodNameCamelCase}}(ctx, req);
			if (err) {
				return std::make_pair(nullptr, err);
			}
			return std::make_pair(std::make_shared<{{.MethodResponseStructName}}>(resp), nullptr);
		};

		auto result = scg::middleware::applyHandlerChain(const_cast<scg::context::Context&>(ctx), req, middleware, handler);
		if (result.second) {
			return scg::rpc::respondWithError(requestID, result.second);
		}

		auto resp = std::dynamic_pointer_cast<{{.MethodResponseStructName}}>(result.first);
		if (!resp) {
			return scg::rpc::respondWithError(requestID, scg::error::Error("Invalid response type"));
		}

		auto response = scg::rpc::respondWithMessage(requestID, *resp);
		return response;
	}
	{{end}}
	std::vector<uint8_t> handleWrapper(const scg::context::Context& ctx, const std::vector<scg::middleware::Middleware>& middleware, uint64_t requestID, scg::serialize::Reader& reader) {
		uint64_t methodID = 0;
		auto err = reader.read(methodID);
		if (err) {
			return scg::rpc::respondWithError(requestID, err);
		}

		switch (methodID) { {{- range .ServerMethods}}
		case {{.MethodIDVarName}}:
			return handle{{.MethodNamePascalCase}}(ctx, middleware, requestID, reader);{{end}}
		default:
			return scg::rpc::respondWithError(requestID, scg::error::Error("Unrecognized method ID: " + std::to_string(methodID)));
		}
	}

	scg::error::Error handleStreamWrapper(const scg::context::Context& ctx, std::shared_ptr<scg::rpc::ServerStream> stream, uint64_t methodID) {
		switch (methodID) { {{- range .ServerStreamMethods}}
		case {{.MethodIDVarName}}: {
			{{if eq .Kind "bidi"}}return impl_->{{.MethodNameCamelCase}}(std::make_shared<{{.StreamTypeName}}>(stream));{{else if eq .Kind "server"}}scg::serialize::Reader reader({});
			scg::error::Error recvErr;
			auto state = stream->recv(reader, recvErr);
			if (state != scg::rpc::StreamRecvState::Message) {
				return recvErr ? recvErr : scg::error::Error("stream closed before request");
			}
			{{.ReqStructName}} req;
			auto derr = reader.read(req);
			if (derr) {
				return derr;
			}
			return impl_->{{.MethodNameCamelCase}}(req, std::make_shared<{{.StreamTypeName}}>(stream));{{else}}auto [resp, uerr] = impl_->{{.MethodNameCamelCase}}(std::make_shared<{{.StreamTypeName}}>(stream));
			if (uerr) {
				return uerr;
			}
			return stream->send(resp);{{end}}
		}{{end}}
		default:
			return scg::error::Error("Unrecognized stream method ID: " + std::to_string(methodID));
		}
	}

private:
	std::shared_ptr<{{.ServerNamePascalCase}}> impl_;
};

inline void register{{.ServerNamePascalCase}}(scg::rpc::Server* server, std::shared_ptr<{{.ServerNamePascalCase}}> impl) {
	auto stub = std::make_shared<{{.ServerStubClassName}}>(impl);

	auto handler = [stub](const scg::context::Context& ctx, const std::vector<scg::middleware::Middleware>& middleware, uint64_t requestID, scg::serialize::Reader& reader) -> std::vector<uint8_t> {
		return stub->handleWrapper(ctx, middleware, requestID, reader);
	};

	server->registerService({{.ServiceIDVarName}}, "{{.ServiceName}}", handler);

	auto streamHandler = [stub](const scg::context::Context& ctx, std::shared_ptr<scg::rpc::ServerStream> stream, uint64_t methodID) -> scg::error::Error {
		return stub->handleStreamWrapper(ctx, stream, methodID);
	};

	server->registerStreamService({{.ServiceIDVarName}}, streamHandler);
}
`

var (
	serverTemplate = template.Must(template.New("serverTemplateCpp").Parse(serverTemplateStr))
)

func serverServiceIDVarName(serviceName string) string {
	return fmt.Sprintf("%sServerID", util.EnsureCamelCase(serviceName))
}

func serverMethodIDVarName(serviceName string, methodName string) string {
	return fmt.Sprintf("%sServer_%sID", util.EnsureCamelCase(serviceName), util.EnsurePascalCase(methodName))
}

func getServerStubClassName(serviceName string) string {
	return fmt.Sprintf("%s_ServerStub", util.EnsurePascalCase(serviceName))
}

func getServerNamePascalCase(serviceName string) string {
	return fmt.Sprintf("%sServer", util.EnsurePascalCase(serviceName))
}

func getServerNameCamelCase(serviceName string) string {
	return fmt.Sprintf("%sServer", util.EnsureCamelCase(serviceName))
}

func generateServerCppCode(pkg *parse.Package, svc *parse.ServiceDefinition) (string, error) {

	serverID, err := pkg.HashStringToServiceID(svc.Name)
	if err != nil {
		return "", err
	}

	args := ServerArgs{
		ServerNamePascalCase: getServerNamePascalCase(svc.Name),
		ServerNameCamelCase:  getServerNameCamelCase(svc.Name),
		ServiceName:          svc.Name,
		ServiceIDVarName:     serverServiceIDVarName(svc.Name),
		ServiceID:            serverID,
		ServerMethods:        []ServerMethodArgs{},
		ServerStreamMethods:  []ServerStreamMethodArgs{},
		ServerStubClassName:  getServerStubClassName(svc.Name),
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
			args.ServerStreamMethods = append(args.ServerStreamMethods, ServerStreamMethodArgs{
				MethodNamePascalCase: util.EnsurePascalCase(name),
				MethodNameCamelCase:  util.EnsureCamelCase(name),
				MethodIDVarName:      serverMethodIDVarName(svc.Name, name),
				MethodID:             methodID,
				Kind:                 streamKind(method),
				StreamTypeName:       streamServerTypeName(svc.Name, name),
				ReqStructName:        methodArgType,
				RespStructName:       methodRetType,
			})
			continue
		}

		args.ServerMethods = append(args.ServerMethods, ServerMethodArgs{
			MethodNamePascalCase:     util.EnsurePascalCase(name),
			MethodNameCamelCase:      util.EnsureCamelCase(name),
			MethodIDVarName:          serverMethodIDVarName(svc.Name, name),
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
