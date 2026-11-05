package cpp_gen

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/kbirk/scg/internal/parse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParse(t *testing.T, files map[string]string) *parse.Parse {
	t.Helper()
	p, err := parse.NewParseFromFiles("./in", files)
	require.NoError(t, err)
	return p
}

func generateCppTree(t *testing.T, baseDir string, p *parse.Parse) map[string]string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, GenerateCppCode(baseDir, dir, p))
	out := map[string]string{}
	require.NoError(t, filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		bs, err := os.ReadFile(path)
		out[filepath.ToSlash(rel)] = string(bs)
		return err
	}))
	return out
}

func TestGeneratedCppIsDeterministic(t *testing.T) {
	for _, dir := range []string{"../../../test/scg/pingpong", "../../../test/scg/basic", "../../../test/scg/sample", "../../../benchmark/scg/benchmark"} {
		var first map[string]string
		for i := 0; i < 10; i++ {
			p, err := parse.NewParse(dir)
			require.NoError(t, err)
			out := generateCppTree(t, "", p)
			if first == nil {
				first = out
				continue
			}
			require.Equal(t, first, out, "%s: generation %d differs from the first", dir, i)
		}
	}
}

func TestIncludePathsFollowSourceDirectories(t *testing.T) {
	p, err := parse.NewParse("../../../test/scg/sample")
	require.NoError(t, err)

	out := generateCppTree(t, "", p)
	// relative to the including header
	assert.Contains(t, out["sample1/sample1.h"], `#include "../sample0/sample0.h"`)

	out = generateCppTree(t, "gen/sample", p)
	// relative to the given base dir, keeping the dependency's directory
	assert.Contains(t, out["sample1/sample1.h"], `#include "gen/sample/sample0/sample0.h"`)
}

func TestCrossPackageTypesAreFullyQualified(t *testing.T) {
	p, err := parse.NewParse("../../../test/scg/sample")
	require.NoError(t, err)
	out := generateCppTree(t, "", p)
	// inside namespace another::sample, `sample::name` would resolve to
	// another::sample::name
	assert.Contains(t, out["sample1/sample1.h"], "::sample::name::AuthRequest")
	assert.NotRegexp(t, regexp.MustCompile(`[^:]sample::name::AuthRequest`), out["sample1/sample1.h"])
}

func TestEnumStringConstantsAreScopedPerEnum(t *testing.T) {
	p := mustParse(t, map[string]string{
		"./in/a.scg": "package e;\nenum Color {\n\tUnknown = 0;\n\tRed = 1;\n}\nenum Shape {\n\tUnknown = 0;\n\tCircle = 1;\n}\n",
	})
	out := generateCppTree(t, "", p)
	h := out["a.h"]
	assert.Contains(t, h, `constexpr const char* COLOR_UNKNOWN_STRING = "Unknown";`)
	assert.Contains(t, h, `constexpr const char* SHAPE_UNKNOWN_STRING = "Unknown";`)
	// the legacy unscoped name is ambiguous here, so it is not emitted...
	assert.NotContains(t, h, "char* UNKNOWN_STRING")
	// ...but unique legacy names are kept, deprecated
	assert.Contains(t, h, `[[deprecated("use COLOR_RED_STRING")]] constexpr const char* RED_STRING = COLOR_RED_STRING;`)
	assert.Contains(t, h, `[[deprecated("use SHAPE_CIRCLE_STRING")]] constexpr const char* CIRCLE_STRING = SHAPE_CIRCLE_STRING;`)
	assert.Contains(t, h, "inline const std::unordered_map<std::string, Color> COLOR_STRING_TO_ENUM")
}

func TestEnumAndConstOnlyFilesIncludeTheirDependencies(t *testing.T) {
	p := mustParse(t, map[string]string{
		"./in/enums.scg":  "package e;\nenum Color {\n\tRed = 0;\n}\n",
		"./in/consts.scg": "package e;\nconst string S = \"x\";\nconst uuid U = \"12345678-1234-5678-1234-567812345678\";\n",
	})
	out := generateCppTree(t, "", p)
	for _, f := range []string{"enums.h", "consts.h"} {
		assert.Contains(t, out[f], "#include <scg/serialize.h>", f)
		assert.Contains(t, out[f], "#include <unordered_map>", f)
		assert.Contains(t, out[f], "#include <scg/uuid.h>", f)
	}
}

func TestEnumFieldsAreValueInitialized(t *testing.T) {
	p := mustParse(t, map[string]string{
		"./in/a.scg": "package e;\nenum Color {\n\tRed = 0;\n}\nmessage M {\n\tColor c = 0;\n\tint32 i = 1;\n}\n",
	})
	out := generateCppTree(t, "", p)
	assert.Contains(t, out["a.h"], "::e::Color c = {};")
	assert.Contains(t, out["a.h"], "int32_t i = 0;")
}

func TestZeroBitMessagesSerializeToNoBytes(t *testing.T) {
	p := mustParse(t, map[string]string{
		"./in/a.scg": "package e;\nmessage Empty {}\nmessage NestedEmpty {\n\tEmpty e = 0;\n}\n",
	})
	out := generateCppTree(t, "", p)
	assert.Contains(t, out["a.h"], "if (num_bits_ == 0) {")
}

func TestJSONKeysAreSnakeCase(t *testing.T) {
	p := mustParse(t, map[string]string{
		"./in/a.scg": "package e;\nmessage M {\n\tint32 val_uint_8 = 0;\n\tstring userName = 1;\n}\n",
	})
	out := generateCppTree(t, "", p)
	// the same keys as the Go json struct tags
	assert.Contains(t, out["a.h"], `j["val_uint_8"] = scg::json::encode(m.valUint8);`)
	assert.Contains(t, out["a.h"], `j["user_name"] = scg::json::encode(m.userName);`)
	assert.Contains(t, out["a.h"], `if (auto it = j.find("user_name"); it != j.end()) {`)
	assert.Contains(t, out["a.h"], `scg::json::decode(*it, m.userName);`)
	assert.Contains(t, out["a.h"], `if (!scg::json::expect_object(j)) {`)
	assert.NotContains(t, out["a.h"], "NLOHMANN_DEFINE_TYPE_NON_INTRUSIVE")
}

func TestMiddlewareRequestIsUsedAndRawPointerCtorIsDeprecated(t *testing.T) {
	p := mustParse(t, map[string]string{
		"./in/a.scg": "package e;\nmessage Req {\n\tint32 x = 0;\n}\nservice S {\n\trpc Fetch (Req) returns (Req);\n}\n",
	})
	h := generateCppTree(t, "", p)["a.h"]
	assert.Contains(t, h, "dynamic_cast<const ::e::Req*>(&r)")
	assert.Contains(t, h, "impl_->fetch(handlerCtx, *typedReq)")
	assert.Contains(t, h, "client_->call(ctx, sID, s_FetchID, *typedReq)")
	assert.NotContains(t, h, "[this, &req]")
	assert.NotContains(t, h, "[this, req, resp]")
	assert.Contains(t, h, "[[deprecated(\"takes ownership of the raw pointer")
	assert.Contains(t, h, "SClient(scg::rpc::Client& client) : client_(std::shared_ptr<scg::rpc::Client>(), &client) {}")
	assert.Contains(t, h, "std::make_shared<::e::Req>(std::move(resp))")
}

func TestConstLiterals(t *testing.T) {
	p := mustParse(t, map[string]string{
		"./in/a.scg": "package c;\ntypedef MyString = string;\n" +
			"const MyString TS = \"hi\";\n" +
			"const string S = \"a#b \\\"q\\\" é\";\n" +
			"const uint64 Big = 18446744073709551615;\n" +
			"const int64 Min = -9223372036854775808;\n" +
			"const uint32 Hex = 0xFF;\n",
	})
	h := generateCppTree(t, "", p)["a.h"]
	// a typedef of std::string is not a literal type
	assert.Contains(t, h, `inline const ::c::MyString TS = ::c::MyString("hi");`)
	// non-ASCII bytes as octal escapes, independent of the source charset
	assert.Contains(t, h, `constexpr const char* S = "a#b \"q\" \303\251";`)
	assert.Contains(t, h, "constexpr uint64_t BIG = 18446744073709551615ULL;")
	assert.Contains(t, h, "constexpr int64_t MIN = (-9223372036854775807LL - 1);")
	assert.Contains(t, h, "constexpr uint32_t HEX = 0xFF;")
}

func TestCppValidationRejectsUncompilableSchemas(t *testing.T) {
	cases := map[string]struct {
		files map[string]string
		msg   string
	}{
		"keyword field": {
			files: map[string]string{"./in/a.scg": "package a;\nmessage M {\n\tint32 class = 0;\n}\n"},
			msg:   "C++ keyword",
		},
		"keyword method": {
			files: map[string]string{"./in/a.scg": "package a;\nmessage M {\n\tint32 x = 0;\n}\nservice S {\n\trpc Delete (M) returns (M);\n}\n"},
			msg:   "C++ keyword",
		},
		"keyword namespace": {
			files: map[string]string{"./in/a.scg": "package my.class;\nmessage M {\n\tint32 x = 0;\n}\n"},
			msg:   "C++ keyword",
		},
		"reserved namespace": {
			files: map[string]string{"./in/a.scg": "package foo.scg;\nmessage M {\n\tint32 x = 0;\n}\n"},
			msg:   "shadow the top-level namespace `scg`",
		},
		"leading std namespace": {
			files: map[string]string{"./in/a.scg": "package std.app;\nmessage M {\n\tint32 x = 0;\n}\n"},
			msg:   "namespace `std`",
		},
		"non-leading nlohmann namespace": {
			files: map[string]string{"./in/a.scg": "package app.nlohmann;\nmessage M {\n\tint32 x = 0;\n}\n"},
			msg:   "shadow the top-level namespace `nlohmann`",
		},
		"runtime sub-namespace": {
			files: map[string]string{"./in/a.scg": "package scg.type;\nmessage M {\n\tint32 x = 0;\n}\n"},
			msg:   "belongs to the scg runtime",
		},
		"field collides with member function": {
			files: map[string]string{"./in/a.scg": "package a;\nmessage M {\n\tint32 to_bytes = 0;\n}\n"},
			msg:   "toBytes",
		},
		"standard macro enum value": {
			files: map[string]string{"./in/a.scg": "package a;\nenum E {\n\tNan = 0;\n}\n"},
			msg:   "standard library macro",
		},
		"standard macro field": {
			files: map[string]string{"./in/a.scg": "package a;\nmessage M {\n\tint32 errno = 0;\n}\n"},
			msg:   "standard library macro",
		},
		"case folded duplicate fields": {
			files: map[string]string{"./in/a.scg": "package a;\nmessage M {\n\tint32 foo_bar = 0;\n\tint32 fooBar = 1;\n}\n"},
			msg:   "`fooBar`",
		},
		"case folded duplicate enum values": {
			files: map[string]string{"./in/a.scg": "package a;\nenum E {\n\tfoo_bar = 0;\n\tfooBar = 1;\n}\n"},
			msg:   "`FOO_BAR`",
		},
		"message and enum share a name": {
			files: map[string]string{"./in/a.scg": "package a;\nenum Thing {\n\tA = 0;\n}\nmessage Thing {\n\tint32 x = 0;\n}\n"},
			msg:   "`Thing`",
		},
		"method collides with stub dispatcher": {
			files: map[string]string{"./in/a.scg": "package a;\nmessage M {\n\tint32 x = 0;\n}\nservice S {\n\trpc Wrapper (M) returns (M);\n}\n"},
			msg:   "handleWrapper",
		},
		"header file collision": {
			files: map[string]string{
				"./in/fooBar.scg":  "package a;\nmessage A {\n\tint32 x = 0;\n}\n",
				"./in/foo_bar.scg": "package a;\nmessage B {\n\tint32 x = 0;\n}\n",
			},
			msg: "would both generate",
		},
	}
	for name, c := range cases {
		p := mustParse(t, c.files)
		_, err := validateCpp(p)
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), c.msg, name)
	}
}

func TestCppValidationWarnsAboutPlatformMacros(t *testing.T) {
	var buf bytes.Buffer
	orig := WarningWriter
	WarningWriter = &buf
	defer func() { WarningWriter = orig }()

	p := mustParse(t, map[string]string{
		"./in/a.scg": "package a;\nenum Method {\n\tGet = 0;\n\tDelete = 1;\n}\n",
	})
	_, err := validateCpp(p)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "`DELETE`")
}

func TestCppValidationAcceptsRepositorySchemas(t *testing.T) {
	for _, dir := range []string{"../../../test/scg/basic", "../../../test/scg/pingpong", "../../../test/scg/sample", "../../../benchmark/scg/benchmark"} {
		p, err := parse.NewParse(dir)
		require.NoError(t, err, dir)
		_, err = validateCpp(p)
		require.NoError(t, err, dir)
	}
}

var cppIDRegex = regexp.MustCompile(`static constexpr uint64_t (\w+) = (\d+)UL;`)

func TestCppServiceIDsMatchParse(t *testing.T) {
	p := mustParse(t, map[string]string{
		"./in/a/a.scg": "package v1;\nmessage Req {\n\tint32 x = 0;\n}\nservice Auth {\n\trpc Login (Req) returns (Req);\n}\n",
	})
	h := generateCppTree(t, "", p)["a/a.h"]
	ids := map[string]string{}
	for _, m := range cppIDRegex.FindAllStringSubmatch(h, -1) {
		ids[m[1]] = m[2]
	}
	svc := p.Packages["v1"].ServiceDefinitions["Auth"]
	want := func(id uint64) string { return fmtUint(id) }
	assert.Equal(t, want(svc.ID), ids["authServerID"])
	assert.Equal(t, want(svc.ID), ids["authID"])
	assert.Equal(t, want(svc.Methods["Login"].ID), ids["authServer_LoginID"])
	assert.Equal(t, want(svc.Methods["Login"].ID), ids["auth_LoginID"])
}

func fmtUint(v uint64) string {
	return strconv.FormatUint(v, 10)
}

func captureWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := WarningWriter
	WarningWriter = &buf
	t.Cleanup(func() { WarningWriter = orig })
	return &buf
}

// stdin / stdout / stderr compile as members with glibc and macOS, and C++20
// keywords are ordinary identifiers in C++17: both generated working code
// before, so they warn instead of failing.
func TestStdioNamesAndCpp20KeywordsWarnInsteadOfFailing(t *testing.T) {
	warnings := captureWarnings(t)
	p := mustParse(t, map[string]string{
		"./in/a.scg": "package a;\nmessage ExecResult {\n\tstring stdout = 0;\n\tstring stderr = 1;\n}\n" +
			"message Manifest {\n\tlist<string> requires = 0;\n\tstring concept = 1;\n\tstring consteval = 2;\n}\n",
	})
	_, err := validateCpp(p)
	require.NoError(t, err)
	for _, name := range []string{"`stdout`", "`stderr`", "`requires`", "`concept`", "`consteval`"} {
		assert.Contains(t, warnings.String(), name)
	}

	// real C++17 keywords and errno are still errors
	for _, field := range []string{"errno", "class", "template"} {
		p := mustParse(t, map[string]string{
			"./in/a.scg": "package a;\nmessage M {\n\tstring " + field + " = 0;\n}\n",
		})
		_, err := validateCpp(p)
		require.Error(t, err, field)
	}
}

// Repro A: Foo.Bar's scoped name FOO_BAR_STRING is the legacy name of
// Baz.FooBar. The legacy name must keep its meaning ("FooBar"); the scoped name
// is skipped and Foo.Bar keeps its legacy name, undeprecated.
func TestScopedEnumStringNameNeverRebindsALegacyName(t *testing.T) {
	warnings := captureWarnings(t)
	p := mustParse(t, map[string]string{
		"./in/a.scg": "package e;\nenum Foo {\n\tBar = 0;\n}\nenum Baz {\n\tFooBar = 0;\n}\n",
	})
	h := generateCppTree(t, "", p)["a.h"]
	// the legacy name keeps its old value
	assert.Contains(t, h, `[[deprecated("use BAZ_FOO_BAR_STRING")]] constexpr const char* FOO_BAR_STRING = BAZ_FOO_BAR_STRING;`)
	assert.Contains(t, h, `constexpr const char* BAZ_FOO_BAR_STRING = "FooBar";`)
	assert.NotContains(t, h, `FOO_BAR_STRING = "Bar"`)
	// Foo.Bar: no scoped name, so its legacy name is its only name
	assert.Contains(t, h, `constexpr const char* BAR_STRING = "Bar";`)
	assert.NotContains(t, h, `deprecated("use FOO_BAR_STRING")`)
	assert.Contains(t, h, "{ BAR_STRING, Foo::BAR }")
	assert.Contains(t, warnings.String(), "Foo.Bar: not emitting `FOO_BAR_STRING`")
}

// Repro B: Foo.BarBaz and FooBar.Baz map to the same scoped name. This
// compiled before (its legacy names are distinct), so it must not be rejected:
// both scoped names are skipped and the legacy names stay as they were.
func TestCollidingScopedEnumStringNamesAreSkippedNotRejected(t *testing.T) {
	warnings := captureWarnings(t)
	p := mustParse(t, map[string]string{
		"./in/a.scg": "package e;\nenum Foo {\n\tBarBaz = 0;\n}\nenum FooBar {\n\tBaz = 0;\n}\n",
	})
	h := generateCppTree(t, "", p)["a.h"]
	assert.NotContains(t, h, "FOO_BAR_BAZ_STRING")
	assert.Contains(t, h, `constexpr const char* BAR_BAZ_STRING = "BarBaz";`)
	assert.Contains(t, h, `constexpr const char* BAZ_STRING = "Baz";`)
	assert.NotContains(t, h, "deprecated(\"use")
	assert.Contains(t, warnings.String(), "another enum value maps to the same name")
}

// A value whose legacy name is ambiguous and whose scoped name is skipped is
// still serialized correctly: the maps use the string literal.
func TestEnumValueWithoutAnyStringConstantUsesALiteral(t *testing.T) {
	captureWarnings(t)
	p := mustParse(t, map[string]string{
		// legacy X_STRING is shared by A.X and B.X; A.X's scoped name A_X_STRING
		// is also the legacy name of C.aX
		"./in/a.scg": "package e;\nenum A {\n\tX = 0;\n}\nenum B {\n\tX = 0;\n}\nenum C {\n\taX = 0;\n}\n",
	})
	h := generateCppTree(t, "", p)["a.h"]
	assert.NotContains(t, h, "char* X_STRING")
	assert.Contains(t, h, `{ "X", A::X }`)
	assert.Contains(t, h, `return os << "X";`)
	assert.Contains(t, h, `constexpr const char* B_X_STRING = "X";`)
}

func TestScgLeadingNamespaceIsAccepted(t *testing.T) {
	p := mustParse(t, map[string]string{
		"./in/a.scg": "package scg.app;\nmessage M {\n\tint32 x = 0;\n}\nenum E {\n\tA = 0;\n}\n",
	})
	_, err := validateCpp(p)
	require.NoError(t, err)
	h := generateCppTree(t, "", p)["a.h"]
	assert.Contains(t, h, "namespace scg {")
	assert.Contains(t, h, "namespace app {")
}

func TestCppOnlyStringEscapesAreReEncoded(t *testing.T) {
	p := mustParse(t, map[string]string{
		"./in/a.scg": "package c;\nconst string Apos = \"it\\'s\";\nconst string Tri = \"a\\?\\?=b\";\n",
	})
	h := generateCppTree(t, "", p)["a.h"]
	assert.Contains(t, h, `constexpr const char* APOS = "it's";`)
	// "??=" must not be able to form a trigraph
	assert.Contains(t, h, `constexpr const char* TRI = "a?\?=b";`)
}

// Generated C++ must compile warning-free with MSVC /W4 (C4065, C4100, C4305,
// C4458): no default-only switches, no unused named parameters, float32
// literals with an f suffix, and member function locals that cannot hide a
// field.
func TestGeneratedCppAvoidsMSVCWarnings(t *testing.T) {
	p := mustParse(t, map[string]string{
		"./in/a.scg": "package w;\ntypedef F = float32;\n" +
			"const float32 A = 123.456;\nconst float32 B = 5;\nconst F C = 1e3;\nconst float64 D = 1.5;\n" +
			"message Empty {}\nmessage Req {\n\tstring str = 0;\n\tstring text = 1;\n\tlist<byte> bytes = 2;\n}\n" +
			"service Unary {\n\trpc Fetch (Req) returns (Req);\n}\n" +
			"service Streaming {\n\trpc Watch (Req) returns (stream Req);\n}\n",
	})
	h := generateCppTree(t, "", p)["a.h"]
	assert.Contains(t, h, "constexpr float32_t A = 123.456f;")
	assert.Contains(t, h, "constexpr float32_t B = 5.0f;")
	assert.Contains(t, h, "constexpr ::w::F C = ::w::F(1e3f);")
	assert.Contains(t, h, "constexpr float64_t D = 1.5;")
	assert.Contains(t, h, "inline void serialize(WriterType&, const Empty&)")
	assert.Contains(t, h, "inline void from_json(const nlohmann::json& j, Empty&)")
	assert.Contains(t, h, "scg::json::expect_object(j);")
	assert.Contains(t, h, "auto text_ = json_.dump(")
	assert.NotContains(t, h, "auto str =")
	// a service without streaming methods has no switch in handleStreamWrapper,
	// and one without unary methods none in handleWrapper
	assert.Contains(t, h, "// no streaming methods")
	assert.Contains(t, h, "// no unary methods")
	assert.NotRegexp(t, regexp.MustCompile(`switch \(methodID\) \{\s*default:`), h)
}
