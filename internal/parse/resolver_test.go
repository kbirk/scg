package parse

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolver(t *testing.T) {

	content := `
		package test.custom;

		message CustomType {
			byte a = 0;
			uint8 b = 1;
			uint16 c = 2;
		}

		message OtherType {
			list<string> m = 0;
			map<string, string> n = 1;
			list<map<string,list<string>>> o = 2;
			CustomType p = 3;
			list<test.custom.CustomType> q = 4;
		}

		typedef TestID = uint32;

		message FinalType {
			list<TestID> a = 0;
			map<string, TestID> b = 1;
			map<TestID, string> c = 2;
		}

		service Test {
			rpc DoThingA (CustomType) returns (OtherType);
			rpc DoThingB (OtherType) returns (CustomType);
		}
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/test.scg": content,
	})
	require.Nil(t, err)
}

func TestResolverCircularDep(t *testing.T) {

	content := `
		package test;

		message A {
			B b = 0;
		}

		message B {
			C c = 0;
		}

		message C {
			A a = 0;
		}
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/test.scg": content,
	})
	require.NotNil(t, err)
}

func TestResolverUndefinedType(t *testing.T) {

	content := `
		package test;

		message A {
			B b = 0;
		}

		message B {
			C c = 0;
		}
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/test.scg": content,
	})
	require.NotNil(t, err)
}

func TestResolverDependencyBetweenFiles(t *testing.T) {

	contentA := `
		package test;

		message A {
			B b = 0;
		}
	`

	contentB := `
		package test;

		message B {
			int32 v = 0;
		}
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/testA.scg": contentA,
		"./test/testB.scg": contentB,
	})
	require.Nil(t, err)
}

func TestResolverDependencyBetweenFiles2(t *testing.T) {

	contentA := `
		package basic;

		typedef BasicString = string;

		const string TestStringConst = "This string has an \"escaped quoted portion\", holy moly!";

		message BasicStruct {
			uint8 val_uint_8 = 0;
			int8 val_int_8 = 1;
			uint16 val_uint_16  = 2;
			int16 val_int_16  = 3;
			uint32 val_uint_32  = 4;
			int32 val_int_32  = 5;
			uint64 val_uint_64  = 6;
			int64 val_int_64  = 7;
			string val_string = 8;
			list<string> val_array_string = 9;
			map<string, float32> val_map_string_float_32 = 10;
		}

		message StructA {
			int8 val_int_8 = 0;
			float32 val_float_32 = 1;
			bool val_bool = 2;
			map<uint8,string> val_map_uint8_string = 3;
		}

		message StructB {
			list<int32> val_array_int = 0;
			map<string, int32> val_map_string_int = 1;
			map<int8, map<int16, string>> val_map_uint8_map_uint16_string = 2;
		}

		message StructC {
			BasicString str = 0;
		 	list<BasicString> strs = 1;
		 	map<BasicString, BasicString> str_map = 2;
		}

		message ComplicatedStruct {
			map<string, StructA> struct_a_map = 0;
			list<StructB> struct_b_array = 1;
		}
	`

	contentB := `
		package basic;

		service BenchMarker {
			rpc Benchy(BasicStruct) returns (ComplicatedStruct);
		}
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/testA.scg": contentA,
		"./test/testB.scg": contentB,
	})
	require.Nil(t, err)
}

func TestResolverListMessageDependency(t *testing.T) {

	contentA := `
		package test;

		message A {
			list<B> b = 0;
		}
	`

	contentB := `
		package test;

		message B {
			int32 v = 0;
		}
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/testA.scg": contentA,
		"./test/testB.scg": contentB,
	})
	require.Nil(t, err)
}

func TestResolverMapValueMessageDependency(t *testing.T) {

	contentA := `
		package test;

		message A {
			map<uint64, B> b = 0;
		}
	`

	contentB := `
		package test;

		message B {
			int32 v = 0;
		}
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/testA.scg": contentA,
		"./test/testB.scg": contentB,
	})
	require.Nil(t, err)
}

func TestResolverMapKeyMessageDependency(t *testing.T) {

	contentA := `
		package test;

		message A {
			map<SomeID, string> b = 0;
		}
	`

	contentB := `
		package test;

		typedef SomeID = uint64;
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/testA.scg": contentA,
		"./test/testB.scg": contentB,
	})
	require.Nil(t, err)
}

func TestResolverTypedefDependencyBetweenFiles(t *testing.T) {

	contentA := `
		package test;

		message A {
			SomeID id = 0;
		}
	`

	contentB := `
		package test;

		typedef SomeID = uint32;
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/testA.scg": contentA,
		"./test/testB.scg": contentB,
	})
	require.Nil(t, err)
}

func TestResolverDependencyBetweenFilesCircular(t *testing.T) {

	contentA := `
		package test;

		message A {
			B b = 0;
		}
	`

	contentB := `
		package test;

		message B {
			int32 a = 0;
		}

		message C {
			A a = 0;
		}
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/testA.scg": contentA,
		"./test/testB.scg": contentB,
	})
	require.NotNil(t, err)
}

func TestResolverTypedefDependencyBetweenFilesCircular(t *testing.T) {

	contentA := `
		package test;

		message A {
			B b = 0;
		}
	`

	contentB := `
		package test;

		typedef B = uint32;

		message C {
			A a = 0;
		}
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/testA.scg": contentA,
		"./test/testB.scg": contentB,
	})
	require.NotNil(t, err)
}

func TestResolverDependencyBetweenPackages(t *testing.T) {

	contentA := `
		package a;

		message A {
			b.B b = 0;
		}
	`

	contentB := `
		package b;

		message B {
			int32 v = 0;
		}
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/testA.scg": contentA,
		"./test/testB.scg": contentB,
	})
	require.Nil(t, err)
}

func TestResolverListDependencyBetweenPackages(t *testing.T) {

	contentA := `
		package a;

		message A {
			list<b.B> b = 0;
		}
	`

	contentB := `
		package b;

		message B {
			int32 v = 0;
		}
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/testA.scg": contentA,
		"./test/testB.scg": contentB,
	})
	require.Nil(t, err)
}

func TestResolverMapValueDependencyBetweenPackages(t *testing.T) {

	contentA := `
		package a;

		message A {
			map<string, b.B> b = 0;
		}
	`

	contentB := `
		package b;

		message B {
			int32 v = 0;
		}
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/testA.scg": contentA,
		"./test/testB.scg": contentB,
	})
	require.Nil(t, err)
}

func TestResolverMapKeyDependencyBetweenPackages(t *testing.T) {

	contentA := `
		package a;

		message A {
			map<b.MyID, float64> b = 0;
		}
	`

	contentB := `
		package b;

		typedef MyID = string;
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/testA.scg": contentA,
		"./test/testB.scg": contentB,
	})
	require.Nil(t, err)
}

func TestResolverTypedefDependencyBetweenPackages(t *testing.T) {

	contentA := `
		package a;

		message A {
			list<b.SomeID> b = 0;
		}
	`

	contentB := `
		package b;

		typedef SomeID = uint32;
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/testA.scg": contentA,
		"./test/testB.scg": contentB,
	})
	require.Nil(t, err)
}

func TestResolverDependencyBetweenPackagesCircular(t *testing.T) {

	contentA := `
		package a;

		message A {
			b.B b = 0;
		}
	`

	contentB := `
		package b;

		message B {
			int32 v = 0;
		}
	`

	contentC := `
		package b;

		message C {
			a.A a = 0;
		}
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/testA.scg": contentA,
		"./test/testB.scg": contentB,
		"./test/testC.scg": contentC,
	})
	require.NotNil(t, err)
}

func TestResolverTypedefDependencyBetweenPackagesCircular(t *testing.T) {

	contentA := `
		package a;

		message A {
			b.B b = 0;
		}
	`

	contentB := `
		package b;

		typedef B = string;
	`

	contentC := `
		package b;

		message C {
			a.A a = 0;
		}
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/testA.scg": contentA,
		"./test/testB.scg": contentB,
		"./test/testC.scg": contentC,
	})
	require.NotNil(t, err)
}

func TestResolverTransitivePackageMissing(t *testing.T) {

	contentA := `
		package a;

		message A1 {
			int32 v = 0;
		}

		message A2 {
			b.B1 b = 0;
		}
	`

	contentB := `
		package b;

		message B1 {
			int32 v = 0;
		}

		message B2 {
			c.C1 c = 0;
		}
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/testA.scg": contentA,
		"./test/testB.scg": contentB,
	})
	require.NotNil(t, err)
}

func TestResolverTransitivePackageCircular(t *testing.T) {

	contentA := `
		package a;

		message A1 {
			int32 v = 0;
		}

		message A2 {
			b.B1 b = 0;
		}
	`

	contentB := `
		package b;

		message B1 {
			int32 v = 0;
		}

		message B2 {
			c.C1 c = 0;
		}
	`

	contentC := `
		package c;

		message C1 {
			int32 v = 0;
		}

		message C2 {
			a.A1 a = 0;
		}
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/testA.scg": contentA,
		"./test/testB.scg": contentB,
		"./test/testC.scg": contentC,
	})
	require.NotNil(t, err)
}

func TestResolverTransitiveFileMissing(t *testing.T) {

	contentA := `
		package a;

		message A1 {
			int32 v = 0;
		}

		message A2 {
			B1 b = 0;
		}
	`

	contentB := `
		package a;

		message B1 {
			int32 v = 0;
		}

		message B2 {
			C1 c = 0;
		}
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/testA.scg": contentA,
		"./test/testB.scg": contentB,
	})
	require.NotNil(t, err)
}

func TestResolverTransitiveFileCircular(t *testing.T) {

	contentA := `
		package a;

		message A1 {
			int32 v = 0;
		}

		message A2 {
			B1 b = 0;
		}
	`

	contentB := `
		package a;

		message B1 {
			int32 v = 0;
		}

		message B2 {
			C1 c = 0;
		}
	`

	contentC := `
		package a;

		message C1 {
			int32 v = 0;
		}

		message C2 {
			A1 a = 0;
		}
	`

	_, err := NewParseFromFiles("./test", map[string]string{
		"./test/testA.scg": contentA,
		"./test/testB.scg": contentB,
		"./test/testC.scg": contentC,
	})
	require.NotNil(t, err)
}
