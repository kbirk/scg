package cpp_gen

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

type ClientMethodArgs struct {
	MethodNameCamelCase      string
	MethodIDVarName          string
	MethodID                 uint64
	MethodRequestStructName  string
	MethodResponseStructName string
}

type ClientArgs struct {
	ClientNamePascalCase string
	ClientNameCamelCase  string
	ServiceIDVarName     string
	ServiceID            uint64
	ClientMethods        []ClientMethodArgs
}

const clientTemplateStr = `
static constexpr uint64_t {{.ServiceIDVarName}} = {{.ServiceID}}UL;{{- range .ClientMethods}}
static constexpr uint64_t {{.MethodIDVarName}} = {{.MethodID}}UL;{{end}}

class {{.ClientNamePascalCase}}Client {
public:
	inline explicit
	{{.ClientNamePascalCase}}Client(scg::rpc::Client* client) : client_(client) {}

	inline explicit
	{{.ClientNamePascalCase}}Client(std::shared_ptr<scg::rpc::Client> client) : client_(client) {}
	{{range .ClientMethods}}
	inline std::pair<{{.MethodResponseStructName}}, scg::error::Error> {{.MethodNameCamelCase}}(scg::context::Context& ctx, const {{.MethodRequestStructName}}& req) const
	{
		{{.MethodResponseStructName}} resp;
		auto err = {{.MethodNameCamelCase}}(&resp, ctx, req);
		return std::pair(resp, err);
	}

	inline scg::error::Error {{.MethodNameCamelCase}}({{.MethodResponseStructName}}* resp, scg::context::Context& c, const {{.MethodRequestStructName}}& req) const
	{
		auto handler = [this, req, resp](scg::context::Context& ctx, const scg::type::Message& r) -> std::pair<scg::type::Message*, scg::error::Error> {
			auto [reader, err] = client_->call(ctx, {{$.ServiceIDVarName}}, {{.MethodIDVarName}}, req);
			if (err) {
				return std::pair(nullptr, err);
			}

			err = reader.read(*resp);
			if (err) {
				return std::pair(nullptr, err);
			}

			return std::pair(resp, nullptr);
		};

		auto& middleware = client_->middleware();
		return scg::middleware::applyHandlerChain(c, req, middleware, handler).second;
	}
	{{end}}

private:
	std::shared_ptr<scg::rpc::Client> client_;
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

func getServerStubStructName(serviceName string) string {
	return fmt.Sprintf("%s_Stub", util.EnsureCamelCase(serviceName))
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
		args.ClientMethods = append(args.ClientMethods, ClientMethodArgs{
			MethodNameCamelCase:      util.EnsureCamelCase(name),
			MethodIDVarName:          methodIDVarName(util.EnsureCamelCase(name), util.EnsurePascalCase(name)),
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
