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
	ReturnsStream            bool
	StreamClientClassName    string
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
	{{range .ClientMethods}}{{if .ReturnsStream}}
	inline std::pair<std::shared_ptr<{{.StreamClientClassName}}>, scg::error::Error> {{.MethodNameCamelCase}}(scg::context::Context& ctx, const {{.MethodRequestStructName}}& req) const
	{
		auto [stream, err] = client_->openStream<{{.MethodRequestStructName}}>(ctx, {{$.ServiceIDVarName}}, {{.MethodIDVarName}}, req);
		if (err) {
			return std::make_pair(nullptr, err);
		}
		return std::make_pair(std::make_shared<{{.StreamClientClassName}}>(stream), nullptr);
	}
	{{else}}
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
	{{end}}{{end}}

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

		methodArgs := ClientMethodArgs{
			MethodNameCamelCase: util.EnsureCamelCase(name),
			MethodIDVarName:     methodIDVarName(svc.Name, name),
			MethodID:            methodID,
			ReturnsStream:       method.ReturnsStream,
		}

		if method.ReturnsStream {
			// For stream returns, we only need the request type
			argType, err := mapDataTypeDefinitionToCppType(method.Argument)
			if err != nil {
				return "", err
			}
			methodArgs.MethodRequestStructName = argType
			methodArgs.StreamClientClassName = getStreamClientClassName(method.StreamName)
		} else {
			// For normal returns, we need both request and response types
			methodArgType, methodRetType, err := generateServiceMethodParams(method)
			if err != nil {
				return "", err
			}
			methodArgs.MethodRequestStructName = methodArgType
			methodArgs.MethodResponseStructName = methodRetType
		}

		args.ClientMethods = append(args.ClientMethods, methodArgs)
	}

	buf := &bytes.Buffer{}
	err = clientTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
