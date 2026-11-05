package parse

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kbirk/scg/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression tests for parser / resolver bugs found in review. Each test
// failed (or panicked) before its fix.

func parseFiles(t *testing.T, files map[string]string) (*Parse, error) {
	t.Helper()
	var p *Parse
	var err error
	require.NotPanics(t, func() {
		p, err = NewParseFromFiles("./test", files)
	})
	return p, err
}

func TestMissingPackageIsAnErrorNotAPanic(t *testing.T) {
	_, err := parseFiles(t, map[string]string{
		"./test/a.scg": "message M {\n\tint32 a = 0;\n}\n",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing package declaration")
}

func TestErrorPrinterDoesNotPanicAtEndOfInput(t *testing.T) {
	// an error on the last line, with no trailing newline
	_, err := parseFiles(t, map[string]string{
		"./test/a.scg": "package z;\nmessage M {\n\tint32 a = 0;\n}\nbogus",
	})
	require.Error(t, err)

	// an error inside a string const that is never terminated
	_, err = parseFiles(t, map[string]string{
		"./test/a.scg": "package z;\nconst string S = \"abc",
	})
	require.Error(t, err)

	// out of range positions are clamped rather than panicking
	require.NotPanics(t, func() {
		_ = getContentForError("one\ntwo", 10, 100)
		_ = getContentForError("", -1, -1)
	})
}

func TestErrorLineNumbersAreOneBased(t *testing.T) {
	_, err := parseFiles(t, map[string]string{
		"./test/a.scg": "package e;\n\nmessage M {\n\tint32 a = 0;\n\tint32 b c = 1;\n}\n",
	})
	require.Error(t, err)
	// `int32 b c = 1;` is on the 5th line
	assert.Contains(t, err.Error(), "line: 5,")
	assert.Contains(t, err.Error(), "5 | ")
}

func TestCommentDelimiterInsideStringConst(t *testing.T) {
	p, err := parseFiles(t, map[string]string{
		"./test/a.scg": "package c;\nconst string S = \"a#b\"; # a real comment\nconst string T = \"x;y\";\n",
	})
	require.NoError(t, err)
	s, err := ConstStringValue(p.Packages["c"].Consts["S"].Value)
	require.NoError(t, err)
	assert.Equal(t, "a#b", s)
	s, err = ConstStringValue(p.Packages["c"].Consts["T"].Value)
	require.NoError(t, err)
	assert.Equal(t, "x;y", s)
}

func TestEscapedQuotesInStringConst(t *testing.T) {
	p, err := parseFiles(t, map[string]string{
		// \\ is an escaped backslash, so the quote after it closes the string;
		// \\\" is an escaped backslash followed by an escaped quote.
		"./test/a.scg": `package c;
const string A = "a\\";
const string B = "b\\\"c";
`,
	})
	require.NoError(t, err)
	a, err := ConstStringValue(p.Packages["c"].Consts["A"].Value)
	require.NoError(t, err)
	assert.Equal(t, `a\`, a)
	b, err := ConstStringValue(p.Packages["c"].Consts["B"].Value)
	require.NoError(t, err)
	assert.Equal(t, `b\"c`, b)
}

func TestNonASCIIStringConst(t *testing.T) {
	p, err := parseFiles(t, map[string]string{
		"./test/a.scg": "package c;\nconst string S = \"héllö 世界\";\nmessage M {\n\tint32 a = 0;\n}\n",
	})
	require.NoError(t, err)
	s, err := ConstStringValue(p.Packages["c"].Consts["S"].Value)
	require.NoError(t, err)
	assert.Equal(t, "héllö 世界", s)
}

func TestIndexOverflowIsRejected(t *testing.T) {
	for _, content := range []string{
		"package y;\nmessage M {\n\tint32 a = 4294967296;\n}\n",
		"package y;\nmessage M {\n\tint32 a = -1;\n}\n",
		"package y;\nenum E {\n\tA = 4294967296;\n}\n",
	} {
		_, err := parseFiles(t, map[string]string{"./test/a.scg": content})
		require.Error(t, err, content)
		assert.Contains(t, err.Error(), "index", content)
	}
}

func TestDuplicateEnumStringValuesAreRejected(t *testing.T) {
	_, err := parseFiles(t, map[string]string{
		"./test/a.scg": "package d;\nenum E {\n\tA \"same\" = 0;\n\tB \"same\" = 1;\n}\n",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate enum string value")

	// an explicit string value equal to another value's default name
	_, err = parseFiles(t, map[string]string{
		"./test/a.scg": "package d;\nenum E {\n\tA \"B\" = 0;\n\tB = 1;\n}\n",
	})
	require.Error(t, err)
}

func TestConstValueValidation(t *testing.T) {
	valid := []string{
		"const uint32 A = 10;",
		"const uint32 A = 0;",
		"const uint32 A = 0xFF;",
		"const int32 A = -5;",
		"const int32 A = +5;",
		"const uint64 A = 18446744073709551615;",
		"const int64 A = -9223372036854775808;",
		"const float32 A = 1.5;",
		"const float64 A = -2.5e-3;",
		"const float64 A = .5;",
		"const float32 A = 10;",
		"const float32 A = 0;",
		"const float64 A = 010.5;",
		"const float64 A = 010e2;",
		`const string A = "tab\tquote\" unicode é";`,
		`const uuid A = "123e4567-e89b-12d3-a456-426614174000";`,
		`const string A = "it\'s";`,
		`const string A = "what\?";`,
	}
	for _, c := range valid {
		_, err := parseFiles(t, map[string]string{"./test/a.scg": "package c;\n" + c + "\n"})
		assert.NoError(t, err, c)
	}

	invalid := map[string]string{
		"const uint32 A = 010;":                                    "leading zeros",
		"const uint32 A = 08;":                                     "leading zeros",
		"const float32 A = 010;":                                   "leading zeros",
		"const float64 A = -07;":                                   "leading zeros",
		"const float32 A = NaN;":                                   "NaN",
		"const float64 A = Inf;":                                   "NaN",
		"const float64 A = 0x1p-2;":                                "invalid float",
		"const uint8 A = 256;":                                     "out of range",
		"const uuid A = 12345678-1234-5678-1234-567812345678;":     "quoted",
		`const uuid A = "{12345678-1234-5678-1234-567812345678}";`: "quoted",
		`const string A = 'x';`:                                    "double-quoted",
		`const string A = "\q";`:                                   "invalid quoted string",
	}
	for c, msg := range invalid {
		_, err := parseFiles(t, map[string]string{"./test/a.scg": "package c;\n" + c + "\n"})
		require.Error(t, err, c)
		assert.Contains(t, err.Error(), msg, c)
	}
}

func TestDependenciesAreKeyedByPackageQualifiedName(t *testing.T) {
	// a file referencing pa.Foo and pb.Foo depends on both packages
	p, err := parseFiles(t, map[string]string{
		"./test/a/a.scg": "package pa;\nmessage Foo {\n\tint32 x = 0;\n}\n",
		"./test/b/b.scg": "package pb;\nmessage Foo {\n\tstring y = 0;\n}\n",
		"./test/c/c.scg": "package pc;\nmessage Bar {\n\tpa.Foo a = 0;\n\tpb.Foo b = 1;\n}\n",
	})
	require.NoError(t, err)
	file := p.Files["c/c.scg"]
	var pkgs []string
	for _, dep := range file.GetPackageDependencies() {
		pkgs = append(pkgs, dep.PackageName)
	}
	assert.Equal(t, []string{"pa", "pb"}, pkgs)
	assert.Len(t, file.GetFileDependencies(), 2)

	// a local Foo does not hide the dependency on pa.Foo
	p, err = parseFiles(t, map[string]string{
		"./test/a/a.scg": "package pa;\nmessage Foo {\n\tint32 x = 0;\n}\n",
		"./test/c/c.scg": "package pc;\nmessage Foo {\n\tstring z = 0;\n}\nmessage Bar {\n\tpa.Foo a = 0;\n\tFoo b = 1;\n}\n",
	})
	require.NoError(t, err)
	deps := p.Files["c/c.scg"].GetPackageDependencies()
	require.Len(t, deps, 1)
	assert.Equal(t, "pa", deps[0].PackageName)
}

func TestServiceArgumentFromMissingPackageIsAnErrorNotAPanic(t *testing.T) {
	_, err := parseFiles(t, map[string]string{
		"./test/c/c.scg": "package pc;\nmessage Foo {\n\tstring z = 0;\n}\nservice S {\n\trpc Do (nosuchpkg.Foo) returns (Foo);\n}\n",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nosuchpkg")
}

func TestSameBaseNameInDifferentDirectoriesIsNotACycle(t *testing.T) {
	_, err := parseFiles(t, map[string]string{
		"./test/a/types.scg": "package s;\nmessage A {\n\tint32 x = 0;\n}\n",
		"./test/b/types.scg": "package s2;\nmessage B {\n\ts.A a = 0;\n}\n",
	})
	require.NoError(t, err)
}

func TestSameMessageNameInDifferentPackagesIsNotACycle(t *testing.T) {
	_, err := parseFiles(t, map[string]string{
		"./test/a/a.scg": "package pa;\nmessage Foo {\n\tint32 x = 0;\n}\n",
		"./test/b/b.scg": "package pb;\nmessage Foo {\n\tpa.Foo inner = 0;\n}\n",
	})
	require.NoError(t, err)
}

func TestTrueCyclesAreStillDetected(t *testing.T) {
	_, err := parseFiles(t, map[string]string{
		"./test/a.scg": "package c;\nmessage A {\n\tB b = 0;\n}\nmessage B {\n\tlist<A> a = 0;\n}\n",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular")
}

func TestResolverIsLinearOnDiamondGraphs(t *testing.T) {
	// M_i has two fields of type M_{i+1}: re-walking shared sub-graphs makes
	// resolution 2^depth (depth 22 took 16s); memoized it is instant.
	const depth = 40
	var sb strings.Builder
	sb.WriteString("package r;\n")
	for i := 0; i < depth; i++ {
		fmt.Fprintf(&sb, "message M%d {\n\tM%d a = 0;\n\tM%d b = 1;\n}\n", i, i+1, i+1)
	}
	fmt.Fprintf(&sb, "message M%d {\n\tint32 x = 0;\n}\n", depth)

	start := time.Now()
	_, err := parseFiles(t, map[string]string{"./test/r.scg": sb.String()})
	require.NoError(t, err)
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestListsOfZeroBitMessagesAreRejected(t *testing.T) {
	base := "package z;\nmessage Empty {}\nmessage NestedEmpty {\n\tEmpty e = 0;\n\tlist<int32> unused_by_check = 1;\n}\nmessage OnlyEmpty {\n\tEmpty e = 0;\n\tEmpty f = 1;\n}\nmessage NonEmpty {\n\tint32 x = 0;\n}\n"

	rejected := []string{
		"list<Empty>",
		"list<OnlyEmpty>",
		"list<list<Empty>>",
		"map<string, list<Empty>>",
	}
	for _, typ := range rejected {
		_, err := parseFiles(t, map[string]string{
			"./test/a.scg": base + "message M {\n\t" + typ + " f = 0;\n}\n",
		})
		require.Error(t, err, typ)
		assert.Contains(t, err.Error(), "zero", typ)
	}

	accepted := []string{
		"list<NonEmpty>",
		"list<NestedEmpty>",  // has a list field, so at least one bit
		"map<string, Empty>", // keys are always at least one bit
		"map<int32, OnlyEmpty>",
		"Empty",
	}
	for _, typ := range accepted {
		_, err := parseFiles(t, map[string]string{
			"./test/a.scg": base + "message M {\n\t" + typ + " f = 0;\n}\n",
		})
		require.NoError(t, err, typ)
	}
}

func TestRPCIDsArePackageQualified(t *testing.T) {
	p, err := parseFiles(t, map[string]string{
		"./test/a/a.scg": "package v1;\nmessage Req {\n\tint32 x = 0;\n}\nservice Auth {\n\trpc Login (Req) returns (Req);\n}\n",
		"./test/b/b.scg": "package v2;\nmessage Req {\n\tint32 x = 0;\n}\nservice Auth {\n\trpc Login (Req) returns (Req);\n}\n",
	})
	require.NoError(t, err)

	a := p.Packages["v1"].ServiceDefinitions["Auth"]
	b := p.Packages["v2"].ServiceDefinitions["Auth"]
	assert.NotEqual(t, a.ID, b.ID)
	assert.Equal(t, util.HashStringToUInt64("v1.Auth"), a.ID)
	assert.Equal(t, util.HashStringToUInt64("v1.Auth/Login"), a.Methods["Login"].ID)
	assert.Equal(t, util.HashStringToUInt64("v2.Auth/Login"), b.Methods["Login"].ID)

	id, err := p.Packages["v1"].HashStringToServiceID("Auth")
	require.NoError(t, err)
	assert.Equal(t, a.ID, id)
	id, err = p.Packages["v2"].HashStringToMethodID("Auth", "Login")
	require.NoError(t, err)
	assert.Equal(t, b.Methods["Login"].ID, id)
}

func TestRPCIDCollisionsAreDetected(t *testing.T) {
	orig := hashRPCName
	defer func() { hashRPCName = orig }()

	files := map[string]string{
		"./test/a/a.scg": "package v1;\nmessage Req {\n\tint32 x = 0;\n}\nservice Auth {\n\trpc Login (Req) returns (Req);\n\trpc Logout (Req) returns (Req);\n}\n",
		"./test/b/b.scg": "package v2;\nmessage Req {\n\tint32 x = 0;\n}\nservice Other {\n\trpc Login (Req) returns (Req);\n}\n",
	}

	// two methods of one service colliding
	hashRPCName = func(s string) uint64 {
		if strings.Contains(s, "/") {
			return 42
		}
		return orig(s)
	}
	_, err := parseFiles(t, files)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "method ID collision")

	// services in different packages colliding
	hashRPCName = func(s string) uint64 {
		if !strings.Contains(s, "/") {
			return 7
		}
		return orig(s)
	}
	_, err = parseFiles(t, files)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service ID collision")
	assert.Contains(t, err.Error(), "v1.Auth")
	assert.Contains(t, err.Error(), "v2.Other")
}

func TestDependencyAndMethodOrderIsDeterministic(t *testing.T) {
	files := map[string]string{
		"./test/pa/pa.scg": "package pa;\nmessage X {\n\tint32 x = 0;\n}\n",
		"./test/pb/pb.scg": "package pb;\nmessage Y {\n\tint32 y = 0;\n}\n",
		"./test/pc/pc.scg": "package pc;\nmessage Z {\n\tint32 z = 0;\n}\n",
		"./test/svc/svc.scg": "package svc;\nmessage Both {\n\tpc.Z z = 0;\n\tpa.X x = 1;\n\tpb.Y y = 2;\n}\n" +
			"service S {\n\trpc Zeta (Both) returns (Both);\n\trpc Alpha (Both) returns (Both);\n\trpc Mid (Both) returns (Both);\n}\n",
	}
	for i := 0; i < 20; i++ {
		p, err := parseFiles(t, files)
		require.NoError(t, err)
		file := p.Files["svc/svc.scg"]

		var pkgs []string
		for _, dep := range file.GetPackageDependencies() {
			pkgs = append(pkgs, dep.PackageName)
		}
		assert.Equal(t, []string{"pa", "pb", "pc"}, pkgs)

		var deps []string
		for _, dep := range file.GetFileDependencies() {
			deps = append(deps, dep.File.Name)
		}
		assert.Equal(t, []string{"pa.scg", "pb.scg", "pc.scg"}, deps)

		var methods []string
		for _, m := range file.ServiceDefinitions["S"].MethodsSortedByKey() {
			methods = append(methods, m.Name)
		}
		assert.Equal(t, []string{"Alpha", "Mid", "Zeta"}, methods)
	}
}

func TestConstTypedefFromAnotherPackage(t *testing.T) {
	p, err := parseFiles(t, map[string]string{
		"./test/a/a.scg": "package pa;\ntypedef MyInt = int32;\n",
		"./test/b/b.scg": "package pb;\nconst pa.MyInt X = 5;\n",
	})
	require.NoError(t, err)
	c := p.Packages["pb"].Consts["X"]
	require.NotNil(t, c.UnderlyingDataTypeDefinition)
	assert.Equal(t, DataTypeComparableInt32, c.UnderlyingDataTypeDefinition.Type)
}

func TestMinBitSize(t *testing.T) {
	p, err := parseFiles(t, map[string]string{
		"./test/a.scg": `package m;
enum E {
	A = 0;
}
typedef T = uint64;
message Empty {}
message Big {
	bool b = 0;
	int32 i = 1;
	uint32 u = 2;
	byte by = 3;
	float32 f = 4;
	float64 d = 5;
	string s = 6;
	timestamp ts = 7;
	uuid id = 8;
	E e = 9;
	T t = 10;
	list<int32> l = 11;
	map<string, int32> mp = 12;
	Empty empty = 13;
}
`,
	})
	require.NoError(t, err)
	big := &DataTypeDefinition{Type: DataTypeCustom, CustomType: "Big", CustomTypePackage: "m"}
	// 1 + 2 + 1 + 8 + 32 + 64 + 1 + 2 + 128 + 1 + 1 + 1 + 1 + 0
	assert.Equal(t, 243, p.MinBitSize(big))
	empty := &DataTypeDefinition{Type: DataTypeCustom, CustomType: "Empty", CustomTypePackage: "m"}
	assert.Equal(t, 0, p.MinBitSize(empty))
	assert.Equal(t, 1, p.MinBitSizeComparable(&DataTypeComparableDefinition{Type: DataTypeComparableCustom, CustomType: "E", CustomTypePackage: "m"}))
	assert.Equal(t, 8, p.MinBitSizeComparable(&DataTypeComparableDefinition{Type: DataTypeComparableUInt8}))
}

// `\'` and `\?` are valid C++ escapes that generated working C++ before; they
// are accepted and decoded, while an escaped backslash before them is not an
// escape of the quote / question mark.
func TestCppOnlyStringEscapesAreAccepted(t *testing.T) {
	cases := map[string]string{
		`"it\'s"`:      "it's",
		`"what\?"`:     "what?",
		`"a\\'b"`:      `a\'b`,
		`"\\\'"`:       `\'`,
		`"tab\t\'q\'"`: "tab\t'q'",
	}
	for raw, want := range cases {
		got, err := ConstStringValue(raw)
		require.NoError(t, err, raw)
		assert.Equal(t, want, got, raw)
	}
	_, err := ConstStringValue(`"\q"`)
	require.Error(t, err)
}

// The package cycle error must point at the same file and line every run (it
// used to depend on map iteration order), and name the file.
func TestPackageCycleErrorLocationIsDeterministic(t *testing.T) {
	// a1 -> b1 -> a2 has no file cycle, but packages pa -> pb -> pa do
	files := map[string]string{
		"./test/pa/a1.scg": "package pa;\nmessage A1 {\n\tpb.B b = 0;\n}\n",
		"./test/pa/a2.scg": "package pa;\nmessage A2 {\n\tint32 x = 0;\n}\nmessage A3 {\n\tint32 y = 0;\n}\n",
		"./test/pb/b1.scg": "package pb;\nmessage B {\n\tint32 pad = 0;\n\tpa.A3 c = 1;\n\tpa.A2 a = 2;\n\tpa.A2 again = 3;\n}\n",
	}
	var first string
	for i := 0; i < 25; i++ {
		_, err := parseFiles(t, files)
		require.Error(t, err)
		msg := err.Error()
		assert.Contains(t, msg, "circular dependency detected between packages")
		assert.Contains(t, msg, "file: b1.scg")
		if first == "" {
			first = msg
			continue
		}
		require.Equal(t, first, msg, "run %d", i)
	}
	// the first reference to package pa in sorted order is pa.A2 (field a)
	assert.Contains(t, first, "line: 5,")
}
