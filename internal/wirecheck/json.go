package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kbirk/scg/test/scg/generated/wire"
)

// JSON compatibility check (with wire_check.cpp's json-* modes):
//
//	json-encode <go_json>                  Go ToJSON of every case
//	(C++ json-from-go: decode, compare, re-encode; plus C++'s own encoding)
//	json-verify <cpp_rt> <cpp_native> <go_rt>
//	                                       Go FromJSON of both C++ outputs,
//	                                       compared with the original values and,
//	                                       as JSON values, with Go's own text;
//	                                       re-encodes C++'s own values to <go_rt>
//	(C++ json-from-roundtrip <go_rt>: decode and compare)

type jsonMessage interface {
	ToJSON() ([]byte, error)
	FromJSON([]byte) error
}

type jsonCase struct {
	name  string
	value jsonMessage
	empty func() jsonMessage
	// roundTrip is the value Go expects back after a trip through C++, when it
	// differs from value (e.g. timestamps C++ saturates); nil means value
	roundTrip jsonMessage
	// normalize computes roundTrip as Go's own JSON round trip of value (e.g.
	// invalid UTF-8, which both languages replace with U+FFFD)
	normalize bool
	// mutate edits Go's JSON before C++ decodes it (e.g. dropping keys, as JSON
	// from an older schema version would)
	mutate func(map[string]json.RawMessage)
}

// expected returns the value Go expects to decode from C++'s JSON.
func (c jsonCase) expected() (jsonMessage, error) {
	if c.roundTrip != nil {
		return c.roundTrip, nil
	}
	if c.normalize {
		text, err := c.value.ToJSON()
		if err != nil {
			return nil, err
		}
		normalized := c.empty()
		if err := normalized.FromJSON(text); err != nil {
			return nil, err
		}
		return normalized, nil
	}
	return c.value, nil
}

var est = time.FixedZone("EST", -5*3600)

func jsonByteList(n int) []byte {
	bs := make([]byte, n)
	for i := range bs {
		bs[i] = byte(i * 7)
	}
	return bs
}

// The values below must match wire_check.cpp exactly.
func jsonCases() []jsonCase {
	id := uuid.MustParse("123e4567-e89b-42d3-a456-426614174000")
	nonV4 := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	typedefs := wire.JsonTypedefs{
		Tu8: 200, Tu16: 65535, Count: 7, Tu64: math.MaxUint64,
		Ti8: -128, Ti16: -2, Ti32: math.MinInt32, Ti64: math.MinInt64,
		Tf32: -1.5, Tf64: 1e300, Label: "lbl", Ident: wire.Ident(nonV4),
	}
	maxScalars := wire.JsonScalars{
		B: true, By: 255, U8: 255, U16: 65535, U32: math.MaxUint32, U64: math.MaxUint64,
		I8: 127, I16: 32767, I32: math.MaxInt32, I64: math.MaxInt64,
		F32: math.MaxFloat32, F64: math.MaxFloat64,
		S:     "héllo \"quoted\" \\ back\nslash\ttab \u0001 <>& 世界 \U0001F600  ",
		ID:    id,
		Ts:    time.Date(2023, 11, 14, 17, 13, 20, 123456789, est),
		Color: wire.Color_Blue,
	}
	return []jsonCase{
		{
			name:  "json_scalars_max",
			value: &maxScalars,
			empty: func() jsonMessage { return &wire.JsonScalars{} },
		},
		{
			name: "json_scalars_min",
			value: &wire.JsonScalars{
				B: false, I8: -128, I16: -32768, I32: math.MinInt32, I64: math.MinInt64,
				F32: -math.SmallestNonzeroFloat32, F64: math.SmallestNonzeroFloat64,
				S: "", ID: uuid.Nil, Ts: time.Unix(-100, 5), Color: wire.Color_Red,
			},
			empty: func() jsonMessage { return &wire.JsonScalars{} },
		},
		{
			name: "json_scalars_mixed",
			value: &wire.JsonScalars{
				U16: 1, U32: 1, U64: 1, I16: -1, I32: -1, I64: -1, F32: 1.1, F64: 0.1,
				S: "plain", ID: nonV4, Ts: time.Unix(1, 1).UTC(), Color: wire.Color_Green,
			},
			empty: func() jsonMessage { return &wire.JsonScalars{} },
		},
		{
			// Go's zero time.Time maps to the minimum C++ timestamp and back
			name:  "json_go_zero_scalars",
			value: &wire.JsonScalars{},
			empty: func() jsonMessage { return &wire.JsonScalars{} },
		},
		{
			name:  "json_typedefs",
			value: &typedefs,
			empty: func() jsonMessage { return &wire.JsonTypedefs{} },
		},
		{
			name: "json_containers",
			value: &wire.JsonContainers{
				Bytes:     jsonByteList(5000),
				U8s:       []uint8{0, 255},
				Tu8s:      []wire.Tu8{1, 2, 3},
				I8s:       []int8{-128, 0, 127},
				ByteLists: [][]byte{{}, {1, 2, 3}, jsonByteList(64)},
				Inners:    []wire.Inner{{Flag: true, Bits: []bool{true, false}, Note: "n"}, {}},
				Stamps:    []time.Time{time.Unix(-1, 999999999), time.Unix(0, 0), time.Unix(1700000000, 100000000).In(est)},
				IDs:       []uuid.UUID{id, uuid.Nil},
				Idents:    []wire.Ident{wire.Ident(nonV4)},
				Colors:    []wire.Color{wire.Color_Red, wire.Color_Blue},
				F32s:      []float32{1.1, -0.5, math.MaxFloat32},
				Strings:   []string{"", "a"},
				ByString:  map[string][]wire.Inner{"k": {{Note: "x"}}, "empty": {}},
				ByInt:     map[int32]map[string]int64{-1: {"a": math.MinInt64}, 2: {}},
				ByColor:   map[wire.Color]wire.Label{wire.Color_Green: "g", wire.Color_Blue: "b"},
				ByUUID:    map[uuid.UUID][]time.Time{id: {time.Unix(5, 6)}},
				ByLabel:   map[wire.Label]wire.Count{"l": 3},
				ByCount:   map[wire.Count]string{math.MaxUint32: "max", 0: "zero"},
				ByIdent:   map[wire.Ident]wire.Inner{wire.Ident(nonV4): {Flag: true}},
				ByTi8:     map[wire.Ti8]wire.Tu8{-128: 255, 127: 0},
				ByU64:     map[uint64]bool{math.MaxUint64: true, 0: false},
				ByI64:     map[int64]wire.JsonTypedefs{math.MinInt64: typedefs},
				ByU8:      map[uint8][]byte{255: {1}, 0: {}},
				Inner:     wire.Inner{Note: "inner"},
				Scalars:   maxScalars,
			},
			empty: func() jsonMessage { return &wire.JsonContainers{} },
		},
		{
			// nil slices and maps encode as null in Go and decode as empty
			name:  "json_nil_containers",
			value: &wire.JsonContainers{},
			empty: func() jsonMessage { return &wire.JsonContainers{} },
		},
		{
			// Go writes negative zero as -0
			name:  "json_negative_zero",
			value: &wire.JsonScalars{F32: float32(math.Copysign(0, -1)), F64: math.Copysign(0, -1), S: "z", Ts: time.Unix(0, 0)},
			empty: func() jsonMessage { return &wire.JsonScalars{} },
		},
		{
			// nil slices nested in a list or as map values are written as null
			name: "json_null_elements",
			value: &wire.JsonContainers{
				ByteLists: [][]byte{nil, {1}},
				ByString:  map[string][]wire.Inner{"nil": nil},
				ByUUID:    map[uuid.UUID][]time.Time{id: nil},
				Scalars:   wire.JsonScalars{Ts: time.Unix(0, 0)},
			},
			empty: func() jsonMessage { return &wire.JsonContainers{} },
		},
		{
			// JSON from an older schema version without some fields
			name:  "json_missing_keys",
			value: &wire.JsonScalars{B: true, I32: 7, Ts: time.Unix(10, 0)},
			empty: func() jsonMessage { return &wire.JsonScalars{} },
			mutate: func(m map[string]json.RawMessage) {
				delete(m, "f64")
				delete(m, "color")
				delete(m, "s")
			},
		},
		{
			// earlier Go code wrote uuid typedefs as an array of 16 numbers
			name:  "json_legacy_uuid_typedef",
			value: &wire.JsonTypedefs{Ident: wire.Ident(nonV4), Label: "x"},
			empty: func() jsonMessage { return &wire.JsonTypedefs{} },
			mutate: func(m map[string]json.RawMessage) {
				legacy, _ := json.Marshal([16]byte(nonV4))
				m["ident"] = legacy
			},
		},
		{
			// instants outside C++'s int64 nanosecond range saturate (the
			// earliest maps to Go's zero time), e.g. Go's "never" sentinel
			name: "json_time_saturation",
			value: &wire.JsonContainers{
				Stamps:  []time.Time{time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC), time.Date(1500, 1, 1, 0, 0, 0, 0, time.UTC), time.Unix(0, 0)},
				Scalars: wire.JsonScalars{Ts: time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)},
			},
			roundTrip: &wire.JsonContainers{
				Stamps:  []time.Time{time.Unix(0, math.MaxInt64), {}, time.Unix(0, 0)},
				Scalars: wire.JsonScalars{Ts: time.Unix(0, math.MaxInt64)},
			},
			empty: func() jsonMessage { return &wire.JsonContainers{} },
		},
		{
			// invalid UTF-8 is replaced with U+FFFD, byte by byte, by both
			name: "json_invalid_utf8",
			value: &wire.JsonContainers{
				Strings:  []string{"\xff", "a\xE2\x82\xACb\xE2", "\xED\xA0\x80", "\xC0\x80ok"},
				ByLabel:  map[wire.Label]wire.Count{"\xC0\x80": 1, "\xF4\x90\x80\x80": 2},
				ByString: map[string][]wire.Inner{"k\xF5": nil},
				Scalars:  wire.JsonScalars{S: "\xF0\x9F\x98", Ts: time.Unix(0, 0)},
			},
			normalize: true,
			empty:     func() jsonMessage { return &wire.JsonContainers{} },
		},
		{
			name: "json_empty_containers",
			value: &wire.JsonContainers{
				Bytes: []byte{}, U8s: []uint8{}, Tu8s: []wire.Tu8{}, I8s: []int8{}, ByteLists: [][]byte{},
				Inners: []wire.Inner{}, Stamps: []time.Time{}, IDs: []uuid.UUID{}, Idents: []wire.Ident{},
				Colors: []wire.Color{}, F32s: []float32{}, Strings: []string{},
				ByString: map[string][]wire.Inner{}, ByInt: map[int32]map[string]int64{},
				ByColor: map[wire.Color]wire.Label{}, ByUUID: map[uuid.UUID][]time.Time{},
				ByLabel: map[wire.Label]wire.Count{}, ByCount: map[wire.Count]string{},
				ByIdent: map[wire.Ident]wire.Inner{}, ByTi8: map[wire.Ti8]wire.Tu8{},
				ByU64: map[uint64]bool{}, ByI64: map[int64]wire.JsonTypedefs{}, ByU8: map[uint8][]byte{},
				Scalars: wire.JsonScalars{Ts: time.Unix(0, 0)},
			},
			empty: func() jsonMessage { return &wire.JsonContainers{} },
		},
	}
}

func abbreviate(bs []byte) string {
	if len(bs) > 400 {
		return string(bs[:400]) + "..."
	}
	return string(bs)
}

func writeJSONCases(path string, texts map[string][]byte) error {
	var names []string
	for name := range texts {
		names = append(names, name)
	}
	sort.Strings(names)
	var sb strings.Builder
	for _, name := range names {
		fmt.Fprintf(&sb, "%s %s\n", name, hex.EncodeToString(texts[name]))
	}
	return os.WriteFile(path, []byte(sb.String()), 0644)
}

func jsonEncode(path string) error {
	texts := map[string][]byte{}
	for _, c := range jsonCases() {
		bs, err := c.value.ToJSON()
		if err != nil {
			return fmt.Errorf("%s: ToJSON: %v", c.name, err)
		}
		if c.mutate != nil {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(bs, &fields); err != nil {
				return err
			}
			c.mutate(fields)
			if bs, err = json.Marshal(fields); err != nil {
				return err
			}
		}
		texts[c.name] = bs
	}
	return writeJSONCases(path, texts)
}

// decodeJSONValue parses JSON text keeping numbers exact.
func decodeJSONValue(text []byte) (interface{}, error) {
	dec := json.NewDecoder(bytes.NewReader(text))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func isEmptyJSON(v interface{}) bool {
	switch t := v.(type) {
	case nil:
		return true
	case []interface{}:
		return len(t) == 0
	case map[string]interface{}:
		return len(t) == 0
	case string:
		// an empty base64 byte list
		return t == ""
	}
	return false
}

func numbersEqual(a, b json.Number) bool {
	as, bs := a.String(), b.String()
	if as == bs {
		return true
	}
	isFloat := func(s string) bool { return strings.ContainsAny(s, ".eE") }
	if !isFloat(as) && !isFloat(bs) {
		ai, aerr := strconv.ParseInt(as, 10, 64)
		bi, berr := strconv.ParseInt(bs, 10, 64)
		if aerr == nil && berr == nil {
			return ai == bi
		}
		au, aerr := strconv.ParseUint(as, 10, 64)
		bu, berr := strconv.ParseUint(bs, 10, 64)
		return aerr == nil && berr == nil && au == bu
	}
	af, aerr := strconv.ParseFloat(as, 64)
	bf, berr := strconv.ParseFloat(bs, 64)
	return aerr == nil && berr == nil && af == bf
}

// jsonValuesEqual compares two parsed JSON documents as values: null equals an
// empty array / object, numbers compare numerically, and strings that are both
// RFC 3339 times compare as instants.
func jsonValuesEqual(a, b interface{}, path string) error {
	if isEmptyJSON(a) && isEmptyJSON(b) {
		return nil
	}
	switch av := a.(type) {
	case map[string]interface{}:
		bv, ok := b.(map[string]interface{})
		if !ok || len(av) != len(bv) {
			return fmt.Errorf("%s: %v vs %v", path, a, b)
		}
		for k, ae := range av {
			be, ok := bv[k]
			if !ok {
				return fmt.Errorf("%s: key %q missing", path, k)
			}
			if err := jsonValuesEqual(ae, be, path+"."+k); err != nil {
				return err
			}
		}
		return nil
	case []interface{}:
		bv, ok := b.([]interface{})
		if !ok || len(av) != len(bv) {
			return fmt.Errorf("%s: %v vs %v", path, a, b)
		}
		for i := range av {
			if err := jsonValuesEqual(av[i], bv[i], fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
		return nil
	case json.Number:
		bv, ok := b.(json.Number)
		if !ok || !numbersEqual(av, bv) {
			return fmt.Errorf("%s: %v vs %v", path, a, b)
		}
		return nil
	case string:
		bv, ok := b.(string)
		if !ok {
			return fmt.Errorf("%s: %v vs %v", path, a, b)
		}
		if av == bv {
			return nil
		}
		at, aerr := time.Parse(time.RFC3339Nano, av)
		bt, berr := time.Parse(time.RFC3339Nano, bv)
		if aerr == nil && berr == nil && at.Equal(bt) {
			return nil
		}
		return fmt.Errorf("%s: %q vs %q", path, av, bv)
	}
	if !reflect.DeepEqual(a, b) {
		return fmt.Errorf("%s: %v vs %v", path, a, b)
	}
	return nil
}

func compareJSONTexts(a, b []byte) error {
	av, err := decodeJSONValue(a)
	if err != nil {
		return err
	}
	bv, err := decodeJSONValue(b)
	if err != nil {
		return err
	}
	return jsonValuesEqual(av, bv, "$")
}

func jsonVerify(cppRoundTripPath, cppNativePath, goRoundTripPath string) error {
	cppRoundTrip, err := readCases(cppRoundTripPath)
	if err != nil {
		return err
	}
	cppNative, err := readCases(cppNativePath)
	if err != nil {
		return err
	}
	goRoundTrip := map[string][]byte{}
	failed := 0
	for _, c := range jsonCases() {
		want, err := c.expected()
		if err != nil {
			return err
		}
		goText, err := want.ToJSON()
		if err != nil {
			return err
		}
		for _, src := range []struct {
			label  string
			texts  map[string][]byte
			native bool
		}{
			{"C++ re-encoding of the Go JSON", cppRoundTrip, false},
			{"C++ encoding of its own value", cppNative, true},
		} {
			text, ok := src.texts[c.name]
			if !ok {
				fmt.Printf("FAIL %s: %s missing\n", c.name, src.label)
				failed++
				continue
			}
			decoded := c.empty()
			if err := decoded.FromJSON(text); err != nil {
				fmt.Printf("FAIL %s: Go could not decode the %s: %v\n  json: %s\n", c.name, src.label, err, abbreviate(text))
				failed++
				continue
			}
			if !equal(reflect.ValueOf(want), reflect.ValueOf(decoded)) {
				fmt.Printf("FAIL %s: Go decoded different values from the %s\n  json: %s\n", c.name, src.label, abbreviate(text))
				failed++
				continue
			}
			if err := compareJSONTexts(goText, text); err != nil {
				fmt.Printf("FAIL %s: the %s differs from Go's JSON: %v\n  go:  %s\n  c++: %s\n", c.name, src.label, err, abbreviate(goText), abbreviate(text))
				failed++
				continue
			}
			if src.native {
				reencoded, err := decoded.ToJSON()
				if err != nil {
					return err
				}
				goRoundTrip[c.name] = reencoded
			}
			fmt.Printf("PASS %s (Go decode of the %s)\n", c.name, src.label)
		}
	}
	if err := writeJSONCases(goRoundTripPath, goRoundTrip); err != nil {
		return err
	}
	if failed > 0 {
		return fmt.Errorf("%d JSON case(s) failed", failed)
	}
	return nil
}
