package cpp_gen

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"github.com/kbirk/scg/internal/parse"
	"github.com/kbirk/scg/internal/util"
)

type EnumValueArgs struct {
	ValueNameUpperCase string
	ValueString        string
	Index              int
	// ScopedStringName is the per-enum `<ENUM>_<VALUE>_STRING` constant, empty
	// when it is not emitted because it would be ambiguous.
	ScopedStringName string
	// LegacyStringName is the historical unscoped `<VALUE>_STRING` constant,
	// empty when it is not emitted (it is ambiguous within the namespace).
	// It is deprecated in favor of the scoped name when that is emitted.
	LegacyStringName string
	// StringRef is how the maps and operator<< refer to the value's string:
	// the scoped name, else the legacy name, else a string literal.
	StringRef string
}

type EnumArgs struct {
	EnumNamePascalCase string
	EnumNameUpperCase  string
	EnumValueArgs      []EnumValueArgs
}

// Enums of a package share one C++ namespace, so the historical unscoped
// `<VALUE>_STRING` constants of two enums can clash, and the scoped
// `<ENUM>_<VALUE>_STRING` constants can clash with each other or with a legacy
// name (enum Foo { Bar } vs enum Baz { FooBar }). Which names are emitted is
// decided by planEnumStringNames.
const enumTemplateStr = `
enum class {{.EnumNamePascalCase}} {
{{- range .EnumValueArgs}}
	{{.ValueNameUpperCase}} = {{.Index}},{{end}}
};

{{- range .EnumValueArgs}}{{if .ScopedStringName}}
constexpr const char* {{.ScopedStringName}} = "{{.ValueString}}";{{end}}{{end}}
{{- range .EnumValueArgs}}{{if .LegacyStringName}}{{if .ScopedStringName}}
[[deprecated("use {{.ScopedStringName}}")]] constexpr const char* {{.LegacyStringName}} = {{.ScopedStringName}};{{else}}
constexpr const char* {{.LegacyStringName}} = "{{.ValueString}}";{{end}}{{end}}{{end}}

inline const std::unordered_map<std::string, {{.EnumNamePascalCase}}> {{.EnumNameUpperCase}}_STRING_TO_ENUM = {
{{range .EnumValueArgs}}	{ {{.StringRef}}, {{$.EnumNamePascalCase}}::{{.ValueNameUpperCase}} },
{{end -}}
};
inline const std::unordered_map<{{.EnumNamePascalCase}}, std::string> {{.EnumNameUpperCase}}_ENUM_TO_STRING = {
{{range .EnumValueArgs}}	{ {{$.EnumNamePascalCase}}::{{.ValueNameUpperCase}}, {{.StringRef}} },
{{end -}}
};

inline std::ostream& operator<<(std::ostream& os, const {{.EnumNamePascalCase}}& value) {
	switch (value) {
{{- range .EnumValueArgs}}
	case {{$.EnumNamePascalCase}}::{{.ValueNameUpperCase}}:
		return os << {{.StringRef}};{{end}}
	}
	// a value outside the enum (e.g. decoded from a newer peer)
	return os << static_cast<uint32_t>(value);
}
`

var (
	enumTemplate = template.Must(template.New("enumTemplateCpp").Parse(enumTemplateStr))
)

func enumValueNameCpp(v *parse.EnumValueDefinition) string {
	return strings.ToUpper(util.EnsureSnakeCase(v.Name))
}

func enumNameUpperCpp(enum *parse.EnumDefinition) string {
	return strings.ToUpper(util.EnsureSnakeCase(enum.Name))
}

// scopedEnumStringName is the per-enum string constant name.
func scopedEnumStringName(enum *parse.EnumDefinition, v *parse.EnumValueDefinition) string {
	return enumNameUpperCpp(enum) + "_" + enumValueNameCpp(v) + "_STRING"
}

// legacyEnumStringName is the historical unscoped string constant name.
func legacyEnumStringName(v *parse.EnumValueDefinition) string {
	return enumValueNameCpp(v) + "_STRING"
}

func enumValueKey(enum *parse.EnumDefinition, v *parse.EnumValueDefinition) string {
	return enum.Name + "." + v.Name
}

// enumStringNames records which enum string constants are emitted for the
// enums of one package (one C++ namespace).
type enumStringNames struct {
	// scoped holds the enum values (enumValueKey) whose scoped name is emitted
	scoped map[string]bool
	// legacy holds the legacy names that are emitted
	legacy map[string]bool
}

// planEnumStringNames decides which enum string constants to emit for the
// enums of one namespace:
//
//   - a legacy `<VALUE>_STRING` name is emitted, with the same value as before,
//     whenever it is unique among the legacy names (i.e. whenever the previous
//     generator's output compiled) and not taken by another identifier;
//   - a scoped `<ENUM>_<VALUE>_STRING` name is emitted only if it collides with
//     no legacy name, no other scoped name and no other identifier; otherwise it
//     is skipped with a warning, so an existing name never changes meaning.
//
// taken reports whether a name is used by another generated identifier.
func planEnumStringNames(enums []*parse.EnumDefinition, taken func(string) bool, warn func(string)) *enumStringNames {
	legacyCounts := map[string]int{}
	scopedCounts := map[string]int{}
	for _, enum := range enums {
		for _, v := range enum.ValuesByIndex() {
			legacyCounts[legacyEnumStringName(v)]++
			scopedCounts[scopedEnumStringName(enum, v)]++
		}
	}

	names := &enumStringNames{
		scoped: map[string]bool{},
		legacy: map[string]bool{},
	}
	for _, enum := range enums {
		for _, v := range enum.ValuesByIndex() {
			legacy := legacyEnumStringName(v)
			if legacyCounts[legacy] == 1 && !taken(legacy) {
				names.legacy[legacy] = true
			}

			scoped := scopedEnumStringName(enum, v)
			switch {
			case legacyCounts[scoped] > 0:
				warn(fmt.Sprintf("enum value %s.%s: not emitting `%s`, it is already the name of another enum value's string constant", enum.Name, v.Name, scoped))
			case scopedCounts[scoped] > 1:
				warn(fmt.Sprintf("enum value %s.%s: not emitting `%s`, another enum value maps to the same name", enum.Name, v.Name, scoped))
			case taken(scoped):
				warn(fmt.Sprintf("enum value %s.%s: not emitting `%s`, it is the name of another generated identifier", enum.Name, v.Name, scoped))
			default:
				names.scoped[enumValueKey(enum, v)] = true
			}
		}
	}
	return names
}

func generateEnumCppCode(enum *parse.EnumDefinition) (string, error) {
	names := planEnumStringNames([]*parse.EnumDefinition{enum}, func(string) bool { return false }, func(string) {})
	return generateEnumCppCodeWithNames(enum, names)
}

func generateEnumCppCodeWithNames(enum *parse.EnumDefinition, names *enumStringNames) (string, error) {

	if len(enum.Values) >= 65536 {
		return "", fmt.Errorf("enum %s has too many values", enum.Name)
	}
	if names == nil {
		return generateEnumCppCode(enum)
	}

	var enumValueArgs []EnumValueArgs
	for i, v := range enum.ValuesByIndex() {
		args := EnumValueArgs{
			ValueNameUpperCase: enumValueNameCpp(v),
			ValueString:        v.Value,
			Index:              i,
		}
		if names.scoped[enumValueKey(enum, v)] {
			args.ScopedStringName = scopedEnumStringName(enum, v)
		}
		if names.legacy[legacyEnumStringName(v)] {
			args.LegacyStringName = legacyEnumStringName(v)
		}
		switch {
		case args.ScopedStringName != "":
			args.StringRef = args.ScopedStringName
		case args.LegacyStringName != "":
			args.StringRef = args.LegacyStringName
		default:
			args.StringRef = strconv.Quote(v.Value)
		}
		enumValueArgs = append(enumValueArgs, args)
	}

	args := EnumArgs{
		EnumNamePascalCase: util.EnsurePascalCase(enum.Name),
		EnumNameUpperCase:  enumNameUpperCpp(enum),
		EnumValueArgs:      enumValueArgs,
	}

	buf := &bytes.Buffer{}
	err := enumTemplate.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
