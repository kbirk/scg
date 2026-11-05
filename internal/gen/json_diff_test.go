package gen_test

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

// TestCppJSONDifferential compares include/scg/json.h with Go's encoding/json
// (the reference) on generated inputs, via testdata/cpp/json_diff.cpp:
// sanitizing invalid UTF-8 in strings and map keys, decoding float32 / float64
// text (negative zero, the float32 overflow boundary), and decoding uint16
// enums and enum map keys. Results must be identical.
func TestCppJSONDifferential(t *testing.T) {
	cxx, includes := cppCompiler(t)
	src, err := filepath.Abs(filepath.Join("testdata", "cpp", "json_diff.cpp"))
	require.NoError(t, err)
	bin := filepath.Join(t.TempDir(), "json_diff")
	args := append([]string{"-std=c++17", "-Wall", "-Wextra", "-Werror", "-O1", "-o", bin, src}, includes...)
	out, err := exec.Command(cxx, args...).CombinedOutput()
	require.NoError(t, err, "compile failed:\n%s", firstLines(string(out), 30))

	type input struct {
		kind string
		text string
	}
	var inputs []input
	for _, s := range diffStrings() {
		inputs = append(inputs, input{"str", s}, input{"strkey", s})
	}
	for _, s := range diffFloat32Texts() {
		inputs = append(inputs, input{"f32", s})
	}
	for _, s := range diffFloat64Texts() {
		inputs = append(inputs, input{"f64", s})
	}
	for _, s := range []string{"0", "1", "65535", "65536", "-0", "-1", "1.0", "1e2", `"1"`, "true", "[]"} {
		inputs = append(inputs, input{"enum", s})
	}
	for _, s := range []string{"0", "1", "+1", "-0", "-1", "01", "65535", "65536", "", " 1", "1 ", "0x1", "1e2"} {
		inputs = append(inputs, input{"enumkey", s})
	}

	var stdin bytes.Buffer
	for _, in := range inputs {
		fmt.Fprintf(&stdin, "%s %s\n", in.kind, hex.EncodeToString([]byte(in.text)))
	}
	cmd := exec.Command(bin)
	cmd.Stdin = &stdin
	stdout, err := cmd.Output()
	require.NoError(t, err)
	var results []string
	scanner := bufio.NewScanner(bytes.NewReader(stdout))
	for scanner.Scan() {
		results = append(results, scanner.Text())
	}
	require.Len(t, results, len(inputs))

	mismatches := 0
	for i, in := range inputs {
		want := goJSONResult(in.kind, in.text)
		if results[i] != want {
			mismatches++
			if mismatches <= 20 {
				t.Errorf("%s %q: C++ %s, Go %s", in.kind, in.text, results[i], want)
			}
		}
	}
	require.Zero(t, mismatches, "%d of %d inputs differ from Go", mismatches, len(inputs))
}

// goJSONResult computes the reference result for one input.
func goJSONResult(kind string, text string) string {
	switch kind {
	case "str":
		bs, err := json.Marshal(text)
		if err != nil {
			return "ERR"
		}
		var s string
		if err := json.Unmarshal(bs, &s); err != nil {
			return "ERR"
		}
		return hex.EncodeToString([]byte(s))
	case "strkey":
		bs, err := json.Marshal(map[string]int{text: 1})
		if err != nil {
			return "ERR"
		}
		var m map[string]int
		if err := json.Unmarshal(bs, &m); err != nil {
			return "ERR"
		}
		for k := range m {
			return hex.EncodeToString([]byte(k))
		}
		return "ERR"
	case "f32":
		var f float32
		if err := json.Unmarshal([]byte(text), &f); err != nil {
			return "ERR"
		}
		return fmt.Sprintf("%08x", math.Float32bits(f))
	case "f64":
		var f float64
		if err := json.Unmarshal([]byte(text), &f); err != nil {
			return "ERR"
		}
		return fmt.Sprintf("%016x", math.Float64bits(f))
	case "enum":
		var v uint16
		if err := json.Unmarshal([]byte(text), &v); err != nil {
			return "ERR"
		}
		return strconv.Itoa(int(v))
	case "enumkey":
		var m map[uint16]int
		if err := json.Unmarshal([]byte(`{`+strconv.Quote(text)+`:1}`), &m); err != nil {
			return "ERR"
		}
		for k := range m {
			return strconv.Itoa(int(k))
		}
		return "ERR"
	}
	return "UNKNOWN"
}

// diffStrings returns curated and random byte strings, biased towards
// malformed UTF-8: lone continuation bytes, overlong forms, surrogates, code
// points above U+10FFFF and truncated sequences.
func diffStrings() []string {
	res := []string{
		"", "plain", "héllo 世界 😀", "�", "\x80", "\xC0\x80", "\xC1\xBF", "\xC2\x80", "\xDF\xBF",
		"\xE0\x80\x80", "\xE0\xA0\x80", "\xED\x9F\xBF", "\xED\xA0\x80", "\xED\xBF\xBF", "\xEE\x80\x80",
		"\xF0\x8F\xBF\xBF", "\xF0\x90\x80\x80", "\xF4\x8F\xBF\xBF", "\xF4\x90\x80\x80", "\xF5\x80\x80\x80",
		"\xFF", "\xFE\xFF", "\xE2\x82", "\xE2\x82\xAC", "a\xE2\x82\xACb\xE2", "\xF0\x9F\x98", "\xF0\x9F\x98\x80\x80",
		"<>&  \"\\\x00\x1f\x7f",
	}
	r := rand.New(rand.NewSource(20260411))
	pieces := []func() []byte{
		func() []byte { return []byte{byte(r.Intn(0x80))} },
		func() []byte { return []byte{byte(0x80 + r.Intn(0x40))} },
		func() []byte { return []byte{byte(0xC0 + r.Intn(0x40))} },
		func() []byte { return []byte{0xED, byte(0x80 + r.Intn(0x40)), byte(0x80 + r.Intn(0x40))} },
		func() []byte {
			return []byte{byte(0xF0 + r.Intn(0x10)), byte(0x80 + r.Intn(0x40)), byte(0x80 + r.Intn(0x40))}
		},
		func() []byte {
			buf := make([]byte, 4)
			n := utf8.EncodeRune(buf, rune(r.Intn(0x110000)))
			return buf[:n]
		},
	}
	for i := 0; i < 3000; i++ {
		var b []byte
		for n := r.Intn(8); n >= 0; n-- {
			b = append(b, pieces[r.Intn(len(pieces))]()...)
		}
		if r.Intn(4) == 0 && len(b) > 0 {
			b = b[:r.Intn(len(b))] // truncate mid-sequence
		}
		res = append(res, string(b))
	}
	return res
}

// diffFloat32Texts returns float32 inputs: curated boundaries plus random
// float32 values written the ways Go and C++ write them (shortest float32
// decimal, and the float64 decimal of the float32 value).
func diffFloat32Texts() []string {
	res := []string{
		"0", "-0", "-0.0", "0.0", "1.1", "-1.1", "0.1", "16777216", "16777217", "1e38", "-1e38",
		"3.4028235e38", "3.4028235e+38", "-3.4028235e+38", "3.4028235677973366e38", "-3.4028235677973366e38",
		"3.402823567797337e38", "3.4028236e38", "1e39", "1.401298464324817e-45", "1e-45", "-1e-45",
		"7e-46", "1e-46", "1e-50", "-1e-50", "1.1754943508222875e-38", "1.1754942e-38",
	}
	r := rand.New(rand.NewSource(7))
	for i := 0; i < 2000; i++ {
		f := math.Float32frombits(r.Uint32())
		if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
			continue
		}
		res = append(res, strconv.FormatFloat(float64(f), 'g', -1, 32), strconv.FormatFloat(float64(f), 'g', -1, 64))
	}
	return res
}

func diffFloat64Texts() []string {
	res := []string{
		"0", "-0", "-0.0", "1", "-1", "0.1", "1e308", "1.7976931348623157e308", "-1.7976931348623157e308",
		"1e309", "-1e309", "5e-324", "4.9406564584124654e-324", "2e-324", "1e-400", "-1e-400",
		"2.2250738585072014e-308", "123456789012345678901234567890",
	}
	r := rand.New(rand.NewSource(11))
	for i := 0; i < 2000; i++ {
		f := math.Float64frombits(r.Uint64())
		if math.IsNaN(f) || math.IsInf(f, 0) {
			continue
		}
		res = append(res, strconv.FormatFloat(f, 'g', -1, 64), strings.ToUpper(strconv.FormatFloat(f, 'e', 17, 64)))
	}
	return res
}
