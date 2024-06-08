package parse

import (
	"fmt"
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
			[]string m = 0;
			map[string]string n = 1;
			[]map[string][]string o = 2;
			CustomType p = 3;
			[]test.custom.CustomType q = 4;
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

	fmt.Println(err.Error())
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

	fmt.Println(err.Error())
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
