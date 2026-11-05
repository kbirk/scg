// Command wirecheck is the Go half of the cross-language wire compatibility
// check (run by run-serialize-tests.sh, together with wire_check.cpp):
//
//	wirecheck encode <file>   writes the Go encodings of every case to <file>
//	wirecheck verify <file>   decodes the C++ encodings in <file> and checks
//	                          them against the expected values (and, for
//	                          deterministic cases, against the Go bytes)
//
// The C++ program builds the same expected values, decodes the Go bytes,
// re-encodes them and compares. Each line of a case file is `<case> <hex>`.
//
// The json-encode / json-verify modes do the same for JSON (see json.go):
// Go ToJSON -> C++ fromJSON -> C++ toJSON -> Go FromJSON, and the reverse
// starting from C++'s own encoding.
package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kbirk/scg/test/scg/generated/wire"
)

type message interface {
	ToBytes() []byte
	FromBytes([]byte) error
}

type wireCase struct {
	name string
	// deterministic cases have a unique encoding (no multi-entry maps), so
	// both languages must produce identical bytes
	deterministic bool
	value         message
	empty         func() message
}

func ts(sec int64, nsec int64) time.Time {
	return time.Unix(sec, nsec).UTC()
}

// The values below must match wire_check.cpp exactly.
func cases() []wireCase {
	return []wireCase{
		{
			name:          "scalars",
			deterministic: true,
			value: &wire.Scalars{
				B0: true, By: 0xAB, U8: 200, U16: 60000, U32: 4000000000, U64: 18446744073709551615,
				I8: -5, I16: -300, I32: 123456, I64: -9000000000, F32: 1.5, F64: -2.25,
				S: "héllo wörld", Color: wire.Color_Blue, Label: "label", Count: 77,
				Ident: wire.Ident(uuid.MustParse("123e4567-e89b-42d3-a456-426614174000")),
				ID:    uuid.MustParse("12345678-1234-4678-9234-567812345678"),
				Ts:    ts(1700000000, 123456789), B1: true,
			},
			empty: func() message { return &wire.Scalars{} },
		},
		{
			name:          "scalars_zero",
			deterministic: true,
			value:         &wire.Scalars{Ts: ts(0, 0)},
			empty:         func() message { return &wire.Scalars{} },
		},
		{
			name:          "containers",
			deterministic: true,
			value: &wire.Containers{
				Inners:     []wire.Inner{{Flag: true, Bits: []bool{true, false, true}, Note: "n"}, {}},
				ByteLists:  [][]byte{{1, 2, 3}, {}},
				Bytes:      []byte{0, 1, 2, 254, 255},
				EmptyList:  []string{},
				ByColor:    map[wire.Color]wire.Count{wire.Color_Green: 9},
				Nested:     map[string]map[int32][]wire.Color{"k": {-1: {wire.Color_Red, wire.Color_Blue}}},
				ListOfMaps: []map[string]int32{{"a": 1}, {}},
				ByID:       map[uuid.UUID][]float64{uuid.MustParse("00000000-0000-4000-8000-000000000001"): {0.5, -1}},
				Empties:    map[string]wire.Empty{"e": {}},
				Stamps:     []time.Time{ts(1, 2), ts(1700000000, 999999999)},
			},
			empty: func() message { return &wire.Containers{} },
		},
		{
			// maps with several entries: Go encodes them in random order, so
			// only the decoded values are compared
			name:          "multi_entry_maps",
			deterministic: false,
			value: &wire.Containers{
				ByColor: map[wire.Color]wire.Count{wire.Color_Red: 1, wire.Color_Green: 2, wire.Color_Blue: 3},
				Nested: map[string]map[int32][]wire.Color{
					"a": {1: {wire.Color_Red}, 2: {wire.Color_Green, wire.Color_Blue}},
					"b": {},
				},
				Empties: map[string]wire.Empty{"x": {}, "y": {}, "z": {}},
			},
			empty: func() message { return &wire.Containers{} },
		},
		{
			// non-v4 UUIDs (v1, v3, v5, v7, nil, NCS variant) must be accepted
			// by both decoders
			name:          "uuid_versions",
			deterministic: true,
			value: &wire.UUIDs{
				IDs: []uuid.UUID{
					uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
					uuid.MustParse("6fa459ea-ee8a-3ca4-894e-db77e160355e"),
					uuid.MustParse("886313e1-3b8a-5372-9b90-0c9aee199e5d"),
					uuid.MustParse("018f3c5e-7b2a-7cde-8f00-123456789abc"),
					uuid.MustParse("00000000-0000-0000-0000-000000000000"),
					uuid.MustParse("12345678-1234-5678-1234-567812345678"),
				},
				Typed: wire.Ident(uuid.MustParse("6ba7b811-9dad-11d1-80b4-00c04fd430c8")),
			},
			empty: func() message { return &wire.UUIDs{} },
		},
		{
			// timestamps before 1970 must use floor(seconds) and nanoseconds in
			// [0, 1e9) on both sides
			name:          "pre_epoch_timestamps",
			deterministic: true,
			value: &wire.Times{Stamps: []time.Time{
				ts(-100, 5), ts(-1, 999999999), ts(-1, 0), ts(0, 0), ts(-2208988800, 0), ts(1, 1),
			}},
			empty: func() message { return &wire.Times{} },
		},
		{
			// Go's zero time.Time (year 1) is outside the C++ range: C++ decodes
			// it to its minimum and must encode that back as the zero time, so
			// a Go "unset" time survives a trip through C++
			name:          "go_zero_time",
			deterministic: true,
			value:         &wire.Times{Stamps: []time.Time{{}, ts(0, 0)}},
			empty:         func() message { return &wire.Times{} },
		},
	}
}

func equal(a, b reflect.Value) bool {
	if a.Type() == reflect.TypeOf(time.Time{}) {
		return a.Interface().(time.Time).Equal(b.Interface().(time.Time))
	}
	switch a.Kind() {
	case reflect.Ptr:
		return equal(a.Elem(), b.Elem())
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			if !equal(a.Field(i), b.Field(i)) {
				return false
			}
		}
		return true
	case reflect.Slice, reflect.Array:
		if a.Len() != b.Len() {
			return false
		}
		for i := 0; i < a.Len(); i++ {
			if !equal(a.Index(i), b.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Float32, reflect.Float64:
		// sign aware, so negative zero must round-trip
		return a.Float() == b.Float() && math.Signbit(a.Float()) == math.Signbit(b.Float())
	case reflect.Map:
		if a.Len() != b.Len() {
			return false
		}
		for _, k := range a.MapKeys() {
			bv := b.MapIndex(k)
			if !bv.IsValid() || !equal(a.MapIndex(k), bv) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(a.Interface(), b.Interface())
}

func readCases(path string) (map[string][]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	res := map[string][]byte{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 16<<20)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		var bs []byte
		if len(fields) > 1 {
			bs, err = hex.DecodeString(fields[1])
			if err != nil {
				return nil, fmt.Errorf("case %s: %v", fields[0], err)
			}
		}
		res[fields[0]] = bs
	}
	return res, scanner.Err()
}

func encode(path string) error {
	var sb strings.Builder
	for _, c := range cases() {
		fmt.Fprintf(&sb, "%s %s\n", c.name, hex.EncodeToString(c.value.ToBytes()))
	}
	return os.WriteFile(path, []byte(sb.String()), 0644)
}

func verify(path string) error {
	cppCases, err := readCases(path)
	if err != nil {
		return err
	}
	failed := 0
	for _, c := range cases() {
		bs, ok := cppCases[c.name]
		if !ok {
			fmt.Printf("FAIL %s: missing from the C++ output\n", c.name)
			failed++
			continue
		}
		decoded := c.empty()
		if err := decoded.FromBytes(bs); err != nil {
			fmt.Printf("FAIL %s: Go could not decode the C++ bytes: %v\n", c.name, err)
			failed++
			continue
		}
		if !equal(reflect.ValueOf(c.value), reflect.ValueOf(decoded)) {
			fmt.Printf("FAIL %s: Go decoded different values from the C++ bytes\n  got:  %+v\n  want: %+v\n", c.name, decoded, c.value)
			failed++
			continue
		}
		if c.deterministic {
			goBytes := c.value.ToBytes()
			if hex.EncodeToString(goBytes) != hex.EncodeToString(bs) {
				fmt.Printf("FAIL %s: C++ and Go encodings differ\n  go:  %x\n  c++: %x\n", c.name, goBytes, bs)
				failed++
				continue
			}
		}
		fmt.Printf("PASS %s (Go decode of C++ bytes)\n", c.name)
	}
	if failed > 0 {
		return fmt.Errorf("%d wire case(s) failed", failed)
	}
	return nil
}

func main() {
	usage := func() {
		fmt.Fprintln(os.Stderr, "usage: wirecheck encode|verify <file> | json-encode <go_json> | json-verify <cpp_rt> <cpp_native> <go_rt>")
		os.Exit(2)
	}
	if len(os.Args) < 3 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "encode":
		err = encode(os.Args[2])
	case "verify":
		err = verify(os.Args[2])
	case "json-encode":
		err = jsonEncode(os.Args[2])
	case "json-verify":
		if len(os.Args) != 5 {
			usage()
		}
		err = jsonVerify(os.Args[2], os.Args[3], os.Args[4])
	default:
		err = fmt.Errorf("unknown mode %s", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}
