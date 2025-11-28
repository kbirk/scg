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

type ServerArgs struct {
	ServerNamePascalCase string
	ServerNameCamelCase  string
	ServiceName          string
	ServiceIDVarName     string
	ServerStubClassName  string
	ServiceID            uint64
	ServerMethods        []ServerMethodArgs
}

const serverTemplateStr = `
static constexpr uint64_t {{.ServiceIDVarName}} = {{.ServiceID}}UL;{{- range .ServerMethods}}
static constexpr uint64_t {{.MethodIDVarName}} = {{.MethodID}}UL;{{end}}

class {{.ServerNamePascalCase}} {
public:
	virtual ~{{.ServerNamePascalCase}}() = default;
	{{range .ServerMethods}}
	virtual std::pair<{{.MethodResponseStructName}}, scg::error::Error> {{.MethodNameCamelCase}}(const scg::context::Context& ctx, const {{.MethodRequestStructName}}& req) = 0;
	{{end}}
};

class {{.ServerStubClassName}} {
public:
	{{.ServerStubClassName}}({{.ServerNamePascalCase}}* impl) : impl_(impl) {}

	{{range .ServerMethods}}
	std::vector<uint8_t> handle{{.MethodNamePascalCase}}(const scg::context::Context& ctx, const std::vector<scg::middleware::Middleware>& middleware, uint64_t requestID, scg::serialize::Reader& reader) {
		{{.MethodRequestStructName}} req;
		auto err = reader.read(req);
		if (err) {
			return scg::rpc::respondWithError(requestID, err);
		}

		auto handler = [this, &req](scg::context::Context& ctx, const scg::type::Message& r) -> std::pair<scg::type::Message*, scg::error::Error> {
			auto [resp, err] = impl_->{{.MethodNameCamelCase}}(ctx, req);
			if (err) {
				return std::make_pair(nullptr, err);
			}
			return std::make_pair(new {{.MethodResponseStructName}}(resp), nullptr);
		};

		auto result = scg::middleware::applyHandlerChain(const_cast<scg::context::Context&>(ctx), req, middleware, handler);
		if (result.second) {
			return scg::rpc::respondWithError(requestID, result.second);
		}

		auto* resp = dynamic_cast<{{.MethodResponseStructName}}*>(result.first);
		if (!resp) {
			delete result.first;
			return scg::rpc::respondWithError(requestID, scg::error::Error("Invalid response type"));
		}

		auto response = scg::rpc::respondWithMessage(requestID, *resp);
		delete resp;
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

private:
	{{.ServerNamePascalCase}}* impl_;
};

inline void register{{.ServerNamePascalCase}}(scg::rpc::Server* server, {{.ServerNamePascalCase}}* impl) {
	auto stub = new {{.ServerStubClassName}}(impl);

	auto handler = [stub](const scg::context::Context& ctx, const std::vector<scg::middleware::Middleware>& middleware, uint64_t requestID, scg::serialize::Reader& reader) -> std::vector<uint8_t> {
		return stub->handleWrapper(ctx, middleware, requestID, reader);
	};

	server->registerService({{.ServiceIDVarName}}, "{{.ServiceName}}", handler);
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
