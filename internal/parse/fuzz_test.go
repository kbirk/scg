package parse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FuzzNewParseFromFiles feeds arbitrary schema text through the tokenizer,
// parsers and resolver: it must return an error or a parse, never panic, and
// rendering any error (which prints the offending source lines) must not
// panic either. `go test` runs the seed corpus; fuzz with:
//
//	go test ./internal/parse -run '^$' -fuzz FuzzNewParseFromFiles
func FuzzNewParseFromFiles(f *testing.F) {
	for _, root := range []string{"../../test/scg", "../../benchmark/scg/benchmark", "../gen/testdata/corpus"} {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && strings.HasSuffix(path, ".scg") {
				if bs, err := os.ReadFile(path); err == nil {
					f.Add(string(bs))
				}
			}
			return nil
		})
	}
	for _, seed := range []string{
		"",
		"package",
		"package a",
		"package a;\nmessage",
		"package a;\nmessage M {",
		"package a;\nconst string S = \"",
		"package a;\nconst string S = \"a#b\";",
		"package a;\nconst string S = \"\\\\\\\"\";",
		"package a;\nenum E { A \"x\" = 0; }",
		"package a;\nmessage M { map<string, list<map<int32, M>>> m = 0; }",
		"package a;\nservice S { rpc A (stream M) returns (stream M); }",
		"package é;\nmessage M { int32 a = 0; }\n# comment é",
		"\n\n\n}",
		"package a;\nmessage M {\n\tint32 a = 99999999999999999999;\n}",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, content string) {
		p, err := NewParseFromFiles("./fuzz", map[string]string{
			"./fuzz/a.scg": content,
		})
		if err != nil {
			_ = err.Error()
			return
		}
		// exercise the accessors generators rely on
		for _, file := range p.Files {
			_ = file.GetPackageDependencies()
			_ = file.GetFileDependencies()
			_ = file.MessagesSortedByDependenciesAndKeys()
			for _, msg := range file.MessagesSortedByKey() {
				for _, field := range msg.FieldsByIndex() {
					_ = p.MinBitSize(field.DataTypeDefinition)
				}
			}
			for _, svc := range file.ServicesSortedByKey() {
				_ = svc.MethodsSortedByKey()
			}
		}
	})
}
