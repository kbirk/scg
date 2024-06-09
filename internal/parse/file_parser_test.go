package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	path        = "./test/test.scg"
	relativeDir = "test"
)

func TestFileParser(t *testing.T) {

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

	file, err := parseFileContent(path, relativeDir, content)
	require.Nil(t, err)

	assert.Equal(t, 2, len(file.MessageDefinitions))
	assert.Equal(t, 1, len(file.ServiceDefinitions))

	type0 := file.MessageDefinitions["CustomType"]
	type1 := file.MessageDefinitions["OtherType"]

	assert.Equal(t, "CustomType", type0.Name)
	assert.Equal(t, 3, len(type0.Fields))

	assert.Equal(t, "a", type0.Fields["a"].Name)
	assert.Equal(t, uint(0), type0.Fields["a"].Index)
	assert.Equal(t, DataTypeByte, type0.Fields["a"].DataTypeDefinition.Type)

	assert.Equal(t, "b", type0.Fields["b"].Name)
	assert.Equal(t, uint(1), type0.Fields["b"].Index)
	assert.Equal(t, DataTypeUInt8, type0.Fields["b"].DataTypeDefinition.Type)

	assert.Equal(t, "c", type0.Fields["c"].Name)
	assert.Equal(t, uint(2), type0.Fields["c"].Index)
	assert.Equal(t, DataTypeUInt16, type0.Fields["c"].DataTypeDefinition.Type)

	assert.Equal(t, "OtherType", type1.Name)
	assert.Equal(t, 5, len(type1.Fields))

	assert.Equal(t, "m", type1.Fields["m"].Name)
	assert.Equal(t, uint(0), type1.Fields["m"].Index)
	assert.Equal(t, DataTypeList, type1.Fields["m"].DataTypeDefinition.Type)
	assert.Equal(t, DataTypeString, type1.Fields["m"].DataTypeDefinition.SubType.Type)

	assert.Equal(t, "n", type1.Fields["n"].Name)
	assert.Equal(t, uint(1), type1.Fields["n"].Index)
	assert.Equal(t, DataTypeMap, type1.Fields["n"].DataTypeDefinition.Type)
	assert.Equal(t, DataComparableTypeString, type1.Fields["n"].DataTypeDefinition.Key)
	assert.Equal(t, DataTypeString, type1.Fields["n"].DataTypeDefinition.SubType.Type)

	assert.Equal(t, "o", type1.Fields["o"].Name)
	assert.Equal(t, uint(2), type1.Fields["o"].Index)
	assert.Equal(t, DataTypeList, type1.Fields["o"].DataTypeDefinition.Type)
	assert.Equal(t, DataTypeMap, type1.Fields["o"].DataTypeDefinition.SubType.Type)
	assert.Equal(t, DataComparableTypeString, type1.Fields["o"].DataTypeDefinition.SubType.Key)
	assert.Equal(t, DataTypeList, type1.Fields["o"].DataTypeDefinition.SubType.SubType.Type)
	assert.Equal(t, DataTypeString, type1.Fields["o"].DataTypeDefinition.SubType.SubType.SubType.Type)

	assert.Equal(t, "p", type1.Fields["p"].Name)
	assert.Equal(t, uint(3), type1.Fields["p"].Index)
	assert.Equal(t, DataTypeCustom, type1.Fields["p"].DataTypeDefinition.Type)
	assert.Equal(t, "CustomType", type1.Fields["p"].DataTypeDefinition.CustomType)
	assert.Equal(t, "test.custom", type1.Fields["p"].DataTypeDefinition.CustomTypePackage)

	assert.Equal(t, "q", type1.Fields["q"].Name)
	assert.Equal(t, uint(4), type1.Fields["q"].Index)
	assert.Equal(t, DataTypeList, type1.Fields["q"].DataTypeDefinition.Type)
	assert.Equal(t, DataTypeCustom, type1.Fields["q"].DataTypeDefinition.SubType.Type)
	assert.Equal(t, "CustomType", type1.Fields["q"].DataTypeDefinition.SubType.CustomType)
	assert.Equal(t, "test.custom", type1.Fields["q"].DataTypeDefinition.SubType.CustomTypePackage)

	service := file.ServiceDefinitions["Test"]

	assert.Equal(t, "Test", service.Name)
	assert.Equal(t, 2, len(service.Methods))

	assert.Equal(t, "DoThingA", service.Methods["DoThingA"].Name)
	assert.Equal(t, DataTypeCustom, service.Methods["DoThingA"].Argument.Type)
	assert.Equal(t, "CustomType", service.Methods["DoThingA"].Argument.CustomType)
	assert.Equal(t, DataTypeCustom, service.Methods["DoThingA"].Return.Type)
	assert.Equal(t, "OtherType", service.Methods["DoThingA"].Return.CustomType)

	assert.Equal(t, "DoThingB", service.Methods["DoThingB"].Name)
	assert.Equal(t, DataTypeCustom, service.Methods["DoThingB"].Argument.Type)
	assert.Equal(t, "OtherType", service.Methods["DoThingB"].Argument.CustomType)
	assert.Equal(t, DataTypeCustom, service.Methods["DoThingB"].Return.Type)
	assert.Equal(t, "CustomType", service.Methods["DoThingB"].Return.CustomType)

}

func TestPackageParsingFailures(t *testing.T) {

	_, err := parseFileContent(path, relativeDir, `
		package _invalidpackage;
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package 0invalidpackage;
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package invalid-package;
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package invalid package;
	`)
	assert.NotNil(t, err)
}

func TestMessageParsingFailures(t *testing.T) {

	_, err := parseFileContent(path, relativeDir, `
		package test;

		message 0invalidMessage {}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		message _invalidMessage {}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		message invalid-Message {}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		message invalid Message {}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		message ValidMessage {
			int32 _invalidField = 0;
		}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		message ValidMessage {
			int32 0invalidField = 0;
		}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		message ValidMessage {
			int32 invalid-Field = 0;
		}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		message ValidMessage {
			int32 .invalidField = 0;
		}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		message ValidMessage {
			0InvalidType ValidField = 0;
		}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		message ValidMessage {
			_invalidType ValidField = 0;
		}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		message ValidMessage {
			invalid-type ValidField = 0;
		}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		message ValidMessage {
			.invalidType ValidField = wer0;
		}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		message ValidMessage {
			int32 ValidField = wer0;
		}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		message ValidMessage {
			int32 ValidField = 1dfgsd;
		}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		message ValidMessage {
			int32 ValidFieldBadIndex = 123;
		}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		message ValidMessage {
			int32 ValidFieldMissingSemiColon = 1
		}
	`)
	assert.NotNil(t, err)
}

func TestServiceParsingFailures(t *testing.T) {

	_, err := parseFileContent(path, relativeDir, `
		package test;

		service _invalidService {}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		service 0invalidService {}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		service invalid-Service {}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		service ValidService {
			rpc InvalidReturnType (SomeType) returns (int32);
		}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		service ValidService {
			rpc ValidName (.InvalidType) returns (SomeType);
		}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		service ValidService {
			rpc ValidName (ValidType) returns (.InvalidType);
		}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		service ValidService {
			rpc InvalidArgumentType (int32) returns (SomeType);
		}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		service ValidService {
			rpc _invalidMethod (SomeType) returns (SomeType);
		}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		service ValidService {
			rpc 0invalidMethod (SomeType) returns (SomeType);
		}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		service ValidService {
			rpc invalid-Method (SomeType) returns (SomeType);
		}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		service ValidService {
			rpcsdf ValidMethod(SomeType) returns (SomeType);
		}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		service ValidService {
			dfsgrpc ValidMethod(SomeType) returns (SomeType);
		}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		service ValidService {
			rpc ValidMethod(SomeType) returnsfdghdfgh (SomeType);
		}
	`)
	assert.NotNil(t, err)

	_, err = parseFileContent(path, relativeDir, `
		package test;

		service ValidService {
			rpc ValidMethodMissingSemiColon(SomeType) returns (SomeType)
		}
	`)
	assert.NotNil(t, err)
}
