package go_gen

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

func generateGoTree(t *testing.T, basePackage string, p *parse.Parse) map[string]string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, GenerateGoCode(basePackage, dir, p))
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

func TestGeneratedGoIsDeterministic(t *testing.T) {
	for _, dir := range []string{"../../../test/scg/pingpong", "../../../test/scg/basic", "../../../test/scg/sample", "../../../benchmark/scg/benchmark"} {
		var first map[string]string
		for i := 0; i < 10; i++ {
			p, err := parse.NewParse(dir)
			require.NoError(t, err)
			out := generateGoTree(t, "github.com/test", p)
			if first == nil {
				first = out
				continue
			}
			require.Equal(t, first, out, "%s: generation %d differs from the first", dir, i)
		}
	}
}

func TestImportPathFollowsSourceDirectory(t *testing.T) {
	p, err := parse.NewParse("../../../test/scg/sample")
	require.NoError(t, err)
	out := generateGoTree(t, "github.com/test/gen", p)
	sample1 := out["sample1/sample1.go"]
	// package sample.name lives in sample0/, so it is imported from there,
	// aliased to the Go package name used in the generated code
	assert.Contains(t, sample1, `sample_name "github.com/test/gen/sample0"`)
	assert.NotContains(t, sample1, `"github.com/test/gen/sample_name"`)
	assert.Contains(t, sample1, "sample_name.AuthRequest")
}

func TestImportIsAliasedWhenItClashesWithGeneratedNames(t *testing.T) {
	p := mustParse(t, map[string]string{
		"./in/rpc/rpc.scg": "package rpc;\nmessage Req {\n\tint32 x = 0;\n}\n",
		"./in/svc/svc.scg": "package svc;\nservice S {\n\trpc Do (rpc.Req) returns (rpc.Req);\n}\n",
	})
	out := generateGoTree(t, "github.com/test", p)
	assert.Contains(t, out["svc/svc.go"], `rpc_pkg "github.com/test/rpc"`)
	assert.Contains(t, out["svc/svc.go"], "*rpc_pkg.Req")
	assert.Contains(t, out["svc/svc.go"], `"github.com/kbirk/scg/pkg/rpc"`)
}

func TestGoOutputFileName(t *testing.T) {
	cases := map[string]string{
		"a/thing.scg":         "thing.go",
		"thing_test.scg":      "thing_test_scg.go",
		"net_linux.scg":       "net_linux_scg.go",
		"net_amd64.scg":       "net_amd64_scg.go",
		"linux.scg":           "linux.go",
		"_private.scg":        "scg_private.go",
		"sample_other.scg":    "sample_other.go",
		"windows_service.scg": "windows_service.go",
	}
	for in, want := range cases {
		assert.Equal(t, want, goOutputFileName(in), in)
	}
}

func TestGoValidationRejectsUncompilableSchemas(t *testing.T) {
	cases := map[string]struct {
		files map[string]string
		msg   string
	}{
		"field collides with generated method": {
			files: map[string]string{"./in/a.scg": "package a;\nmessage M {\n\tint32 serialize = 0;\n}\n"},
			msg:   "collides with the generated method Serialize",
		},
		"bit_size field": {
			files: map[string]string{"./in/a.scg": "package a;\nmessage M {\n\tint32 bit_size = 0;\n}\n"},
			msg:   "BitSize",
		},
		"case folded duplicate fields": {
			files: map[string]string{"./in/a.scg": "package a;\nmessage M {\n\tint32 foo_bar = 0;\n\tint32 fooBar = 1;\n}\n"},
			msg:   "`FooBar`",
		},
		"message and enum share a name": {
			files: map[string]string{"./in/a.scg": "package a;\nenum Thing {\n\tA = 0;\n}\nmessage Thing {\n\tint32 x = 0;\n}\n"},
			msg:   "`Thing`",
		},
		"enum value collides with generated map": {
			files: map[string]string{"./in/a.scg": "package a;\nenum E {\n\tToString = 0;\n}\n"},
			msg:   "`E_ToString`",
		},
		"service and message clash": {
			files: map[string]string{"./in/a.scg": "package a;\nmessage AuthClient {\n\tint32 x = 0;\n}\nservice Auth {\n\trpc Do (AuthClient) returns (AuthClient);\n}\n"},
			msg:   "`AuthClient`",
		},
		"keyword package": {
			files: map[string]string{"./in/a.scg": "package default;\nmessage M {\n\tint32 x = 0;\n}\n"},
			msg:   "Go keyword",
		},
		"two packages in one directory": {
			files: map[string]string{
				"./in/a.scg": "package pa;\nmessage M {\n\tint32 x = 0;\n}\n",
				"./in/b.scg": "package pb;\nmessage N {\n\tint32 x = 0;\n}\n",
			},
			msg: "one package per directory",
		},
		"imported package split across directories": {
			files: map[string]string{
				"./in/x/a.scg": "package p;\nmessage A {\n\tint32 x = 0;\n}\n",
				"./in/y/b.scg": "package p;\nmessage B {\n\tint32 x = 0;\n}\n",
				"./in/z/c.scg": "package q;\nmessage C {\n\tp.A a = 0;\n}\n",
			},
			msg: "split across directories",
		},
		"output file collision": {
			files: map[string]string{
				"./in/thing_test.scg":     "package t;\nmessage A {\n\tint32 x = 0;\n}\n",
				"./in/thing_test_scg.scg": "package t;\nmessage B {\n\tint32 x = 0;\n}\n",
			},
			msg: "would both generate",
		},
		"two imports with the same go name": {
			files: map[string]string{
				"./in/x/a.scg": "package a.b;\nmessage A {\n\tint32 x = 0;\n}\n",
				"./in/y/b.scg": "package a_b;\nmessage B {\n\tint32 x = 0;\n}\n",
				"./in/z/c.scg": "package c;\nmessage C {\n\ta.b.A a = 0;\n\ta_b.B b = 1;\n}\n",
			},
			msg: "both map to the Go package name",
		},
	}
	for name, c := range cases {
		p := mustParse(t, c.files)
		err := validateGo(p)
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), c.msg, name)
	}
}

func TestGoValidationAcceptsRepositorySchemas(t *testing.T) {
	for _, dir := range []string{"../../../test/scg/basic", "../../../test/scg/pingpong", "../../../test/scg/sample", "../../../benchmark/scg/benchmark"} {
		p, err := parse.NewParse(dir)
		require.NoError(t, err, dir)
		require.NoError(t, validateGo(p), dir)
	}
}

func TestContainerHelperNamesAreUniqueByStructure(t *testing.T) {
	// a typedef named `Time` maps to the same concatenated name as the builtin
	// timestamp; both helpers must exist under distinct names
	p := mustParse(t, map[string]string{
		"./in/a.scg": "package a;\ntypedef Time = int64;\nmessage M {\n\tlist<Time> a = 0;\n\tlist<timestamp> b = 1;\n}\n",
	})
	code, err := generateMessageGoCode(p, p.Packages["a"].MessageDefinitions["M"])
	require.NoError(t, err)
	assert.Contains(t, code, "func m_BitSizeListTime(arg []Time) int")
	assert.Contains(t, code, "func m_BitSizeListTime_2(arg []time.Time) int")
	assert.Contains(t, code, "m_SerializeListTime(writer, m.A)")
	assert.Contains(t, code, "m_SerializeListTime_2(writer, m.B)")
	assert.Contains(t, code, "m_DeserializeListTime_2(&m.B, reader)")
}

func TestByteListsUseBulkHelpers(t *testing.T) {
	p := mustParse(t, map[string]string{
		"./in/a.scg": "package a;\nmessage M {\n\tlist<byte> a = 0;\n\tlist<uint8> b = 1;\n\tlist<list<byte>> c = 2;\n}\n",
	})
	code, err := generateMessageGoCode(p, p.Packages["a"].MessageDefinitions["M"])
	require.NoError(t, err)
	assert.Contains(t, code, "serialize.BitSizeByteList(m.A)")
	assert.Contains(t, code, "serialize.SerializeByteList(writer, m.B)")
	assert.Contains(t, code, "serialize.DeserializeByteList(&m.A, reader)")
	// nested: the outer list has a helper, the inner lists use the bulk path
	assert.Contains(t, code, "serialize.SerializeByteList(writer, v)")
	assert.NotContains(t, code, "SerializeListByte(")
}

func TestFixedSizeListBitSizeDoesNotIterate(t *testing.T) {
	p := mustParse(t, map[string]string{
		"./in/a.scg": "package a;\nmessage M {\n\tlist<float64> a = 0;\n\tlist<bool> b = 1;\n\tlist<int32> c = 2;\n}\n",
	})
	code, err := generateMessageGoCode(p, p.Packages["a"].MessageDefinitions["M"])
	require.NoError(t, err)
	assert.Contains(t, code, "return serialize.BitSizeUInt32(uint32(len(arg))) + len(arg)*64")
	assert.Contains(t, code, "return serialize.BitSizeUInt32(uint32(len(arg))) + len(arg)*1")
	// varints still iterate
	assert.Contains(t, code, "size += serialize.BitSizeInt32(v)")
}

func TestContainerDecodersPassMinimumElementBits(t *testing.T) {
	p := mustParse(t, map[string]string{
		"./in/a.scg": "package a;\nmessage Big {\n\tstring s = 0;\n\tuuid id = 1;\n}\nmessage M {\n\tlist<Big> a = 0;\n\tmap<uint8, Big> b = 1;\n}\n",
	})
	code, err := generateMessageGoCode(p, p.Packages["a"].MessageDefinitions["M"])
	require.NoError(t, err)
	// Big: string (1) + uuid (128)
	assert.Contains(t, code, "serialize.MakeSlice[Big](reader, length, 129)")
	// map entry: uint8 key (8) + Big (129)
	assert.Contains(t, code, "serialize.MakeMap[uint8, Big](reader, length, 137)")
}

func TestUUIDConstOnlyFileImportsUUID(t *testing.T) {
	p := mustParse(t, map[string]string{
		"./in/a.scg": "package a;\nconst uuid U = \"12345678-1234-5678-1234-567812345678\";\n",
	})
	out := generateGoTree(t, "github.com/test", p)
	assert.Contains(t, out["a.go"], `"github.com/google/uuid"`)
	assert.Contains(t, out["a.go"], `uuid.MustParse("12345678-1234-5678-1234-567812345678")`)
}

func TestConstStringsAreRequoted(t *testing.T) {
	p := mustParse(t, map[string]string{
		"./in/a.scg": "package a;\nconst string S = \"a#b \\\"q\\\" \\x41\";\n",
	})
	out := generateGoTree(t, "github.com/test", p)
	assert.Contains(t, out["a.go"], `S string = "a#b \"q\" A"`)
}

var goIDRegex = regexp.MustCompile(`(\w+ServerID|\w+Server_\w+ID)\s+uint64 = (\d+)`)

// GoServiceIDs extracts the service / method ID constants from generated Go.
func GoServiceIDs(code string) map[string]string {
	ids := map[string]string{}
	for _, m := range goIDRegex.FindAllStringSubmatch(code, -1) {
		ids[m[1]] = m[2]
	}
	return ids
}

func TestServiceIDsArePackageQualifiedInGeneratedCode(t *testing.T) {
	p := mustParse(t, map[string]string{
		"./in/a/a.scg": "package v1;\nmessage Req {\n\tint32 x = 0;\n}\nservice Auth {\n\trpc Login (Req) returns (Req);\n}\n",
		"./in/b/b.scg": "package v2;\nmessage Req {\n\tint32 x = 0;\n}\nservice Auth {\n\trpc Login (Req) returns (Req);\n}\n",
	})
	out := generateGoTree(t, "github.com/test", p)
	a := GoServiceIDs(out["a/a.go"])
	b := GoServiceIDs(out["b/b.go"])
	require.NotEmpty(t, a["authServerID"])
	assert.NotEqual(t, a["authServerID"], b["authServerID"])
	assert.NotEqual(t, a["authServer_LoginID"], b["authServer_LoginID"])
	assert.True(t, strings.HasPrefix(out["a/a.go"], "// Code generated by scg"))
}

func TestCppOnlyStringEscapesAreReEncodedForGo(t *testing.T) {
	p := mustParse(t, map[string]string{
		"./in/a.scg": "package a;\nconst string Apos = \"it\\'s\";\nconst string Tri = \"a\\?\\?=b\";\n",
	})
	out := generateGoTree(t, "github.com/test", p)
	assert.Contains(t, out["a.go"], `Apos string = "it's"`)
	assert.Contains(t, out["a.go"], `Tri string = "a??=b"`)
}
