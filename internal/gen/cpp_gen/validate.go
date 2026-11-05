package cpp_gen

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

// WarningWriter receives non-fatal generation warnings.
var WarningWriter io.Writer = os.Stderr

// cppKeywords are the C++17 keywords (and alternative operator tokens). An
// identifier with one of these names does not compile.
var cppKeywords = toSet(
	"alignas", "alignof", "and", "and_eq", "asm", "auto", "bitand", "bitor", "bool", "break",
	"case", "catch", "char", "char16_t", "char32_t", "class", "compl",
	"const", "constexpr", "const_cast", "continue",
	"decltype", "default", "delete", "do", "double", "dynamic_cast",
	"else", "enum", "explicit", "export", "extern", "false", "float", "for", "friend", "goto",
	"if", "inline", "int", "long", "mutable", "namespace", "new", "noexcept", "not", "not_eq",
	"nullptr", "operator", "or", "or_eq", "private", "protected", "public", "register",
	"reinterpret_cast", "return", "short", "signed", "sizeof", "static",
	"static_assert", "static_cast", "struct", "switch", "template", "this", "thread_local",
	"throw", "true", "try", "typedef", "typeid", "typename", "union", "unsigned", "using",
	"virtual", "void", "volatile", "wchar_t", "while", "xor", "xor_eq",
)

// cpp20Keywords are keywords added in C++20. Generated code targets C++17,
// where they are ordinary identifiers, so they only produce a warning.
var cpp20Keywords = toSet(
	"char8_t", "concept", "consteval", "constinit", "co_await", "co_return", "co_yield",
	"requires",
)

// cppStandardMacros are object-like macros defined by the C / C++ standard
// library headers (and POSIX errno.h) that generated headers include directly
// or transitively. An identifier with one of these names does not compile.
var cppStandardMacros = toSet(
	"NAN", "INFINITY", "HUGE_VAL", "HUGE_VALF", "HUGE_VALL", "EOF", "NULL", "BUFSIZ",
	"FILENAME_MAX", "FOPEN_MAX", "TMP_MAX", "SEEK_SET", "SEEK_CUR", "SEEK_END",
	"EXIT_SUCCESS", "EXIT_FAILURE", "RAND_MAX", "MB_CUR_MAX", "MB_LEN_MAX", "CHAR_BIT",
	"CHAR_MIN", "CHAR_MAX", "SCHAR_MIN", "SCHAR_MAX", "UCHAR_MAX", "SHRT_MIN", "SHRT_MAX",
	"USHRT_MAX", "INT_MIN", "INT_MAX", "UINT_MAX", "LONG_MIN", "LONG_MAX", "ULONG_MAX",
	"LLONG_MIN", "LLONG_MAX", "ULLONG_MAX", "SIZE_MAX", "INT8_MIN", "INT8_MAX", "UINT8_MAX",
	"INT16_MIN", "INT16_MAX", "UINT16_MAX", "INT32_MIN", "INT32_MAX", "UINT32_MAX",
	"INT64_MIN", "INT64_MAX", "UINT64_MAX", "CLOCKS_PER_SEC",
	"EDOM", "ERANGE", "EILSEQ", "EPERM", "ENOENT", "ESRCH", "EINTR", "EIO", "ENXIO", "E2BIG",
	"ENOEXEC", "EBADF", "ECHILD", "EAGAIN", "ENOMEM", "EACCES", "EFAULT", "EBUSY", "EEXIST",
	"EXDEV", "ENODEV", "ENOTDIR", "EISDIR", "EINVAL", "ENFILE", "EMFILE", "ENOTTY", "EFBIG",
	"ENOSPC", "ESPIPE", "EROFS", "EMLINK", "EPIPE", "ETIMEDOUT", "ECONNREFUSED",
	"ECONNRESET", "EWOULDBLOCK", "EINPROGRESS", "ENOTSUP", "ECANCELED",
	"errno",
)

// cppPlatformMacros are object-like macros defined by common platform headers
// (the Windows SDK, GCC in its default GNU mode, or C libraries whose stdio
// streams are macros that do not expand to themselves). They only break builds
// on those platforms, so they produce a warning rather than an error.
var cppPlatformMacros = toSet(
	"stdin", "stdout", "stderr",
	"ERROR", "DELETE", "IN", "OUT", "OPTIONAL", "CONST", "VOID", "TRUE", "FALSE", "NEAR",
	"FAR", "PASCAL", "CALLBACK", "WINAPI", "APIENTRY", "INFINITE", "IGNORE", "ABSOLUTE",
	"RELATIVE", "TRANSPARENT", "OPAQUE", "DIFFERENCE", "DOMAIN", "SING", "OVERFLOW",
	"UNDERFLOW", "TLOSS", "PLOSS", "NO_ERROR", "STRICT", "MAX_PATH",
	"interface", "small", "near", "far", "linux", "unix", "i386",
)

// cppMessageMembers are the member functions generated on every message.
var cppMessageMembers = toSet("toJSON", "fromJSON", "toBytes", "fromBytes")

// cppShadowedNamespaces are namespaces referenced (qualified, not with a
// leading ::) from inside generated code. A non-leading package segment with
// one of these names would shadow them, e.g. inside namespace foo::scg the
// name `scg::type` would resolve to foo::scg::type.
var cppShadowedNamespaces = toSet("scg", "std", "nlohmann")

// cppRuntimeSubNamespaces are the namespaces the runtime declares directly in
// namespace scg. A package `scg.<segment>` with one of them would merge the
// generated declarations into the runtime's namespace.
var cppRuntimeSubNamespaces = toSet(
	"rpc", "type", "serialize", "error", "context", "middleware", "tcp", "ws", "detail", "log",
)

func toSet(names ...string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
}

func sortedMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

type cppValidator struct {
	warnings []string
}

// checkName rejects C++17 keywords and standard macros, and warns about C++20
// keywords and platform macros.
func (v *cppValidator) checkName(name string, origin string) error {
	if cppKeywords[name] {
		return fmt.Errorf("%s generates the C++ identifier `%s`, which is a C++ keyword; rename it", origin, name)
	}
	if cpp20Keywords[name] {
		v.warnings = append(v.warnings, fmt.Sprintf("%s generates the C++ identifier `%s`, which is a keyword from C++20 on and will not compile there; consider renaming it", origin, name))
	}
	if cppStandardMacros[name] {
		return fmt.Errorf("%s generates the C++ identifier `%s`, which is a standard library macro; rename it", origin, name)
	}
	if cppPlatformMacros[name] {
		v.warnings = append(v.warnings, fmt.Sprintf("%s generates the C++ identifier `%s`, which is a macro on some platforms (Windows SDK / GNU mode) and may not compile there; consider renaming it", origin, name))
	}
	return nil
}

type cppIdentifiers struct {
	scope  string
	origin map[string]string
}

func newCppIdentifiers(scope string) *cppIdentifiers {
	return &cppIdentifiers{scope: scope, origin: map[string]string{}}
}

func (ids *cppIdentifiers) add(name string, origin string) error {
	if existing, ok := ids.origin[name]; ok {
		return fmt.Errorf("C++ identifier collision in %s: %s and %s both generate `%s`; rename one of them", ids.scope, existing, origin, name)
	}
	ids.origin[name] = origin
	return nil
}

// validateCpp rejects schemas whose generated C++ code would not compile and
// returns, per package, the enum string constant names to emit. Warnings are
// written to WarningWriter.
func validateCpp(p *parse.Parse) (map[string]*enumStringNames, error) {
	v := &cppValidator{}

	if err := validateCppOutputs(p); err != nil {
		return nil, err
	}

	enumNames := map[string]*enumStringNames{}
	for _, pkgName := range sortedMapKeys(p.Packages) {
		names, err := v.validatePackage(p.Packages[pkgName])
		if err != nil {
			return nil, err
		}
		enumNames[pkgName] = names
	}

	for _, w := range v.warnings {
		fmt.Fprintf(WarningWriter, "WARNING: %s\n", w)
	}
	return enumNames, nil
}

// validateCppOutputs rejects source files that map to the same header, e.g.
// fooBar.scg and foo_bar.scg (headers use snake_case file names).
func validateCppOutputs(p *parse.Parse) error {
	outputs := map[string]string{}
	for _, path := range sortedMapKeys(p.Files) {
		file := p.Files[path]
		out := filepath.Join(file.RelativePath, getOutputFileName(path))
		if existing, ok := outputs[out]; ok {
			return fmt.Errorf("files %s and %s would both generate %s; rename one of them", existing, path, out)
		}
		outputs[out] = path
	}
	return nil
}

func (v *cppValidator) validatePackage(pkg *parse.Package) (*enumStringNames, error) {

	if err := v.validateNamespace(pkg); err != nil {
		return nil, err
	}

	ids := newCppIdentifiers(fmt.Sprintf("namespace of package %s", pkg.Name))

	for _, name := range sortedMapKeys(pkg.MessageDefinitions) {
		msg := pkg.MessageDefinitions[name]
		origin := fmt.Sprintf("message %s", msg.Name)
		msgName := util.EnsurePascalCase(msg.Name)
		if err := v.checkName(msgName, origin); err != nil {
			return nil, err
		}
		if err := ids.add(msgName, origin); err != nil {
			return nil, err
		}
		if err := v.validateMessageFields(msg); err != nil {
			return nil, err
		}
	}

	for _, name := range sortedMapKeys(pkg.Typedefs) {
		typdef := pkg.Typedefs[name]
		origin := fmt.Sprintf("typedef %s", typdef.Name)
		typedefName := util.EnsurePascalCase(typdef.Name)
		if err := v.checkName(typedefName, origin); err != nil {
			return nil, err
		}
		if err := ids.add(typedefName, origin); err != nil {
			return nil, err
		}
		// SCG_TYPEDEF also declares a tag struct `<Name>_`
		if err := ids.add(typedefName+"_", origin); err != nil {
			return nil, err
		}
	}

	for _, name := range sortedMapKeys(pkg.Consts) {
		c := pkg.Consts[name]
		origin := fmt.Sprintf("const %s", c.Name)
		constName := strings.ToUpper(util.EnsureSnakeCase(c.Name))
		if err := v.checkName(constName, origin); err != nil {
			return nil, err
		}
		if err := ids.add(constName, origin); err != nil {
			return nil, err
		}
	}

	for _, name := range sortedMapKeys(pkg.Enums) {
		enum := pkg.Enums[name]
		origin := fmt.Sprintf("enum %s", enum.Name)
		enumName := util.EnsurePascalCase(enum.Name)
		if err := v.checkName(enumName, origin); err != nil {
			return nil, err
		}
		for _, id := range []string{enumName, enumNameUpperCpp(enum) + "_STRING_TO_ENUM", enumNameUpperCpp(enum) + "_ENUM_TO_STRING"} {
			if err := ids.add(id, origin); err != nil {
				return nil, err
			}
		}
		values := newCppIdentifiers(fmt.Sprintf("enum %s", enum.Name))
		for _, val := range enum.ValuesByIndex() {
			valueOrigin := fmt.Sprintf("enum value %s.%s", enum.Name, val.Name)
			valueName := enumValueNameCpp(val)
			if err := v.checkName(valueName, valueOrigin); err != nil {
				return nil, err
			}
			if err := values.add(valueName, valueOrigin); err != nil {
				return nil, err
			}
		}
	}

	for _, name := range sortedMapKeys(pkg.ServiceDefinitions) {
		svc := pkg.ServiceDefinitions[name]
		if err := v.validateService(svc, ids); err != nil {
			return nil, err
		}
	}

	var enums []*parse.EnumDefinition
	for _, name := range sortedMapKeys(pkg.Enums) {
		enums = append(enums, pkg.Enums[name])
	}
	names := planEnumStringNames(enums, func(name string) bool {
		_, taken := ids.origin[name]
		return taken
	}, func(warning string) {
		v.warnings = append(v.warnings, fmt.Sprintf("package %s: %s", pkg.Name, warning))
	})
	return names, nil
}

// validateNamespace rejects package names whose C++ namespace breaks name
// lookup in generated code: a leading `std` (generated declarations would be
// added to namespace std), a non-leading `scg` / `std` / `nlohmann` (it would
// shadow the namespace generated code refers to), or `scg.<x>` where <x> is
// one of the runtime's own namespaces.
func (v *cppValidator) validateNamespace(pkg *parse.Package) error {
	origin := fmt.Sprintf("package %s", pkg.Name)
	segments := convertPackageNameToCppNamespaces(pkg.Name)
	for i, segment := range segments {
		if i == 0 && segment == "std" {
			return fmt.Errorf("%s generates declarations in namespace `std`, which is reserved for the standard library; rename the package", origin)
		}
		if i > 0 && cppShadowedNamespaces[segment] {
			return fmt.Errorf("%s generates the nested C++ namespace `%s`, which would shadow the top-level namespace `%s` used by generated code; rename the package", origin, strings.Join(segments[:i+1], "::"), segment)
		}
		if i == 1 && segments[0] == "scg" && cppRuntimeSubNamespaces[segment] {
			return fmt.Errorf("%s generates the C++ namespace `scg::%s`, which belongs to the scg runtime; rename the package", origin, segment)
		}
		if err := v.checkName(segment, origin); err != nil {
			return err
		}
	}
	return nil
}

func (v *cppValidator) validateMessageFields(msg *parse.MessageDefinition) error {
	fields := newCppIdentifiers(fmt.Sprintf("message %s", msg.Name))
	jsonKeys := newCppIdentifiers(fmt.Sprintf("message %s JSON keys", msg.Name))
	for _, field := range msg.FieldsByIndex() {
		origin := fmt.Sprintf("field %s.%s", msg.Name, field.Name)
		name := util.EnsureCamelCase(field.Name)
		if err := v.checkName(name, origin); err != nil {
			return err
		}
		if cppMessageMembers[name] {
			return fmt.Errorf("%s generates the C++ member `%s`, which collides with the generated member function %s(); rename the field", origin, name, name)
		}
		if err := fields.add(name, origin); err != nil {
			return err
		}
		if err := jsonKeys.add(util.EnsureSnakeCase(field.Name), origin); err != nil {
			return err
		}
	}
	return nil
}

func (v *cppValidator) validateService(svc *parse.ServiceDefinition, ids *cppIdentifiers) error {
	origin := fmt.Sprintf("service %s", svc.Name)
	svcPascal := util.EnsurePascalCase(svc.Name)
	for _, id := range []string{
		getServerNamePascalCase(svc.Name),
		getServerStubClassName(svc.Name),
		"register" + getServerNamePascalCase(svc.Name),
		serverServiceIDVarName(svc.Name),
		serviceIDVarName(svc.Name),
		svcPascal + "Api",
		svcPascal + "Client",
		svcPascal + "Fake",
	} {
		if err := ids.add(id, origin); err != nil {
			return err
		}
	}

	methods := newCppIdentifiers(fmt.Sprintf("service %s", svc.Name))
	fakeMembers := map[string]bool{}
	for _, method := range svc.MethodsSortedByKey() {
		if !method.IsStreaming() {
			fakeMembers["on"+util.EnsurePascalCase(method.Name)] = true
		}
	}
	for _, method := range svc.MethodsSortedByKey() {
		methodOrigin := fmt.Sprintf("method %s.%s", svc.Name, method.Name)
		name := util.EnsureCamelCase(method.Name)
		if err := v.checkName(name, methodOrigin); err != nil {
			return err
		}
		if err := methods.add(name, methodOrigin); err != nil {
			return err
		}
		if fakeMembers[name] {
			return fmt.Errorf("%s generates the C++ method `%s`, which collides with the %sFake hook member of the same name; rename the method", methodOrigin, name, svcPascal)
		}
		if !method.IsStreaming() {
			handler := "handle" + util.EnsurePascalCase(method.Name)
			if handler == "handleWrapper" || handler == "handleStreamWrapper" {
				return fmt.Errorf("%s generates the C++ server stub method `%s`, which collides with the stub's dispatch method; rename the method", methodOrigin, handler)
			}
		}
		for _, id := range []string{serverMethodIDVarName(svc.Name, method.Name), methodIDVarName(svc.Name, method.Name)} {
			if err := ids.add(id, methodOrigin); err != nil {
				return err
			}
		}
		if method.IsStreaming() {
			for _, id := range []string{streamServerTypeName(svc.Name, method.Name), streamClientTypeName(svc.Name, method.Name)} {
				if err := ids.add(id, methodOrigin); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
