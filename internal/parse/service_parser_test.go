package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceMethodTokenizer(t *testing.T) {

	content := `
		service TestServiceA {
			# comment
			rpc Authenticate ( []AuthRequest ) returns (AuthResponse );
			rpc DoThing (string, map [ string] [  ] AuthRequest) returns ( string); #comment
			rpc # comment
			YaYa (YayaRequest, int8) # comment again
			returns( string , # uh oh

				int8); # lots of comments
		}

		service TestServiceB {
			rpc Authenticate ( []AuthRequest ) returns (AuthResponse );
			rpc DoThing (string, map [ string] [  ] AuthRequest) returns ( string);
			rpc YaYa (YayaRequest, int8)returns( string , int8);
		}
	`

	tokens, err := tokenizeFile(content)
	require.Nil(t, err)

	require.Equal(t, 2, len(tokens))

	for _, token := range tokens {
		match, err := FindOneMatch(serviceRegex, token)
		require.Nil(t, err)

		methods, err := tokenizeServiceMethods(match.Captures[1])
		require.Nil(t, err)

		for _, method := range methods {
			match := getContentByTokenRange(content, method.LineStart, method.LineEnd, method.LineStartCharacterPosition, method.LineEndCharacterPosition)
			assert.Equal(t, method.Content, match)
		}

	}
}

func TestServiceParser(t *testing.T) {
	tokens, err := tokenizeFile(`
		service Test {
			rpc Authenticate (AuthRequest ) returns (AuthResponse );
			rpc DoThing (ThingRequest) returns ( ThingResponse);
			rpc YaYa (
				YayaRequest
				)returns(
				YayaResponse
				);
		}
	`)
	require.Nil(t, err)

	services, err := parseServiceDefinitions(tokens)
	require.Nil(t, err)

	assert.Equal(t, 1, len(services))

	service := services["Test"]

	assert.Equal(t, "Test", service.Name)
	assert.Equal(t, 3, len(service.Methods))

	assert.Equal(t, "Authenticate", service.Methods["Authenticate"].Name)
	assert.Equal(t, DataTypeCustom, service.Methods["Authenticate"].Argument.Type)
	assert.Equal(t, "AuthRequest", service.Methods["Authenticate"].Argument.CustomType)
	assert.Equal(t, DataTypeCustom, service.Methods["Authenticate"].Return.Type)
	assert.Equal(t, "AuthResponse", service.Methods["Authenticate"].Return.CustomType)

	assert.Equal(t, "DoThing", service.Methods["DoThing"].Name)
	assert.Equal(t, DataTypeCustom, service.Methods["DoThing"].Argument.Type)
	assert.Equal(t, "ThingRequest", service.Methods["DoThing"].Argument.CustomType)
	assert.Equal(t, DataTypeCustom, service.Methods["DoThing"].Return.Type)
	assert.Equal(t, "ThingResponse", service.Methods["DoThing"].Return.CustomType)

	assert.Equal(t, "YaYa", service.Methods["YaYa"].Name)
	assert.Equal(t, DataTypeCustom, service.Methods["YaYa"].Argument.Type)
	assert.Equal(t, "YayaRequest", service.Methods["YaYa"].Argument.CustomType)
	assert.Equal(t, DataTypeCustom, service.Methods["YaYa"].Return.Type)
	assert.Equal(t, "YayaResponse", service.Methods["YaYa"].Return.CustomType)
}

func TestMultipleService(t *testing.T) {

	tokens, err := tokenizeFile(`
		service TestServiceA {
			rpc MethodA (TypeA) returns (TypeA);
		}

		service TestServiceB {
			rpc MethodB (TypeB) returns (TypeB);
		}
	`)
	require.Nil(t, err)

	services, err := parseServiceDefinitions(tokens)
	require.Nil(t, err)

	assert.Equal(t, 2, len(services))

	serviceA := services["TestServiceA"]
	serviceB := services["TestServiceB"]

	assert.Equal(t, "TestServiceA", serviceA.Name)
	assert.Equal(t, 1, len(serviceA.Methods))

	assert.Equal(t, "TestServiceB", serviceB.Name)
	assert.Equal(t, 1, len(serviceB.Methods))
}

func TestServiceWithPackage(t *testing.T) {

	tokens, err := tokenizeFile(`
		service TestServiceA {
			rpc MethodA (some.package.TypeA) returns (big_package.TypeB);
			rpc MethodB (big_package.TypeB) returns (third.one_here.TypeA);
		}
	`)
	require.Nil(t, err)

	services, err := parseServiceDefinitions(tokens)
	require.Nil(t, err)

	assert.Equal(t, 1, len(services))

	serviceA := services["TestServiceA"]

	assert.Equal(t, "TestServiceA", serviceA.Name)
	assert.Equal(t, 2, len(serviceA.Methods))

	assert.Equal(t, "TypeA", serviceA.Methods["MethodA"].Argument.CustomType)
	assert.Equal(t, "some.package", serviceA.Methods["MethodA"].Argument.CustomTypePackage)
	assert.Equal(t, "TypeB", serviceA.Methods["MethodA"].Return.CustomType)
	assert.Equal(t, "big_package", serviceA.Methods["MethodA"].Return.CustomTypePackage)

	assert.Equal(t, "TypeB", serviceA.Methods["MethodB"].Argument.CustomType)
	assert.Equal(t, "big_package", serviceA.Methods["MethodB"].Argument.CustomTypePackage)
	assert.Equal(t, "TypeA", serviceA.Methods["MethodB"].Return.CustomType)
	assert.Equal(t, "third.one_here", serviceA.Methods["MethodB"].Return.CustomTypePackage)
}
