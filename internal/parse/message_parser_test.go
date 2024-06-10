package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageFieldTokenizer(t *testing.T) {

	content := `
		message TestMessageA {
			# comment here
			byte a = 1;
			uint8 b = 2;
			uint16 c = 3;
		}

		message TestMessageB {
			uint32 d = 1; # comment here
			uint64 e = 2;

		}

		message TestMessageC {
			list< string
			> #comment
			 m #across
			 = 1; # lines
			map<string, string> n = 2;
			map< string, some.other_package.CustomType > r = 3;
			another.nested.CustomArg = 4;
		}
	`

	tokens, err := tokenizeFile(content)
	require.Nil(t, err)

	for _, token := range tokens {
		match, err := FindOneMatch(messageRegex, token)
		require.Nil(t, err)

		fields, err := tokenizeMessageFields(match.Captures[1])
		require.Nil(t, err)

		for _, field := range fields {
			match := getContentByTokenRange(content, field.LineStart, field.LineEnd, field.LineStartCharacterPosition, field.LineEndCharacterPosition)
			assert.Equal(t, field.Content, match)
		}

	}
}

func TestMessageParser(t *testing.T) {

	tokens, err := tokenizeFile(`
		# commment the message
		message TestMessage {
			bool z = 0;
			byte a = 1;
			uint8 b = 2;
			uint16 c = 3;
			uint32 d = 4;
			uint64 e = 5;
			int8 f = 6;
			int16 g = 7;
			int32 h = 8;
			int64 i = 9;
			float32 j = 10;
			float64 k = 11;
			string l = 12;
			list<string> m = 13;
			map<string, string> n = 14;
			list<map<string, list<string>>> o = 15;
			CustomType p = 16;
			list<custom.CustomType> q = 17;
			# commment here
			map<string, some.other_package.CustomType> r = 18;
		}
	`)
	require.Nil(t, err)

	messages, err := parseMessageDefinitions(tokens)
	require.Nil(t, err)

	assert.Equal(t, 1, len(messages))

	message := messages["TestMessage"]

	assert.Equal(t, "TestMessage", message.Name)
	assert.Equal(t, 19, len(message.Fields))

	assert.Equal(t, "z", message.Fields["z"].Name)
	assert.Equal(t, uint(0), message.Fields["z"].Index)
	assert.Equal(t, DataTypeBool, message.Fields["z"].DataTypeDefinition.Type)

	assert.Equal(t, "a", message.Fields["a"].Name)
	assert.Equal(t, uint(1), message.Fields["a"].Index)
	assert.Equal(t, DataTypeByte, message.Fields["a"].DataTypeDefinition.Type)

	assert.Equal(t, "b", message.Fields["b"].Name)
	assert.Equal(t, uint(2), message.Fields["b"].Index)
	assert.Equal(t, DataTypeUInt8, message.Fields["b"].DataTypeDefinition.Type)

	assert.Equal(t, "c", message.Fields["c"].Name)
	assert.Equal(t, uint(3), message.Fields["c"].Index)
	assert.Equal(t, DataTypeUInt16, message.Fields["c"].DataTypeDefinition.Type)

	assert.Equal(t, "d", message.Fields["d"].Name)
	assert.Equal(t, uint(4), message.Fields["d"].Index)
	assert.Equal(t, DataTypeUInt32, message.Fields["d"].DataTypeDefinition.Type)

	assert.Equal(t, "e", message.Fields["e"].Name)
	assert.Equal(t, uint(5), message.Fields["e"].Index)
	assert.Equal(t, DataTypeUInt64, message.Fields["e"].DataTypeDefinition.Type)

	assert.Equal(t, "f", message.Fields["f"].Name)
	assert.Equal(t, uint(6), message.Fields["f"].Index)
	assert.Equal(t, DataTypeInt8, message.Fields["f"].DataTypeDefinition.Type)

	assert.Equal(t, "g", message.Fields["g"].Name)
	assert.Equal(t, uint(7), message.Fields["g"].Index)
	assert.Equal(t, DataTypeInt16, message.Fields["g"].DataTypeDefinition.Type)

	assert.Equal(t, "h", message.Fields["h"].Name)
	assert.Equal(t, uint(8), message.Fields["h"].Index)
	assert.Equal(t, DataTypeInt32, message.Fields["h"].DataTypeDefinition.Type)

	assert.Equal(t, "i", message.Fields["i"].Name)
	assert.Equal(t, uint(9), message.Fields["i"].Index)
	assert.Equal(t, DataTypeInt64, message.Fields["i"].DataTypeDefinition.Type)

	assert.Equal(t, "j", message.Fields["j"].Name)
	assert.Equal(t, uint(10), message.Fields["j"].Index)
	assert.Equal(t, DataTypeFloat32, message.Fields["j"].DataTypeDefinition.Type)

	assert.Equal(t, "k", message.Fields["k"].Name)
	assert.Equal(t, uint(11), message.Fields["k"].Index)
	assert.Equal(t, DataTypeFloat64, message.Fields["k"].DataTypeDefinition.Type)

	assert.Equal(t, "l", message.Fields["l"].Name)
	assert.Equal(t, uint(12), message.Fields["l"].Index)
	assert.Equal(t, DataTypeString, message.Fields["l"].DataTypeDefinition.Type)

	assert.Equal(t, "m", message.Fields["m"].Name)
	assert.Equal(t, uint(13), message.Fields["m"].Index)
	assert.Equal(t, DataTypeList, message.Fields["m"].DataTypeDefinition.Type)
	assert.Equal(t, DataTypeString, message.Fields["m"].DataTypeDefinition.SubType.Type)

	assert.Equal(t, "n", message.Fields["n"].Name)
	assert.Equal(t, uint(14), message.Fields["n"].Index)
	assert.Equal(t, DataTypeMap, message.Fields["n"].DataTypeDefinition.Type)
	assert.Equal(t, DataComparableTypeString, message.Fields["n"].DataTypeDefinition.Key)
	assert.Equal(t, DataTypeString, message.Fields["n"].DataTypeDefinition.SubType.Type)

	assert.Equal(t, "o", message.Fields["o"].Name)
	assert.Equal(t, uint(15), message.Fields["o"].Index)
	assert.Equal(t, DataTypeList, message.Fields["o"].DataTypeDefinition.Type)
	assert.Equal(t, DataTypeMap, message.Fields["o"].DataTypeDefinition.SubType.Type)
	assert.Equal(t, DataComparableTypeString, message.Fields["o"].DataTypeDefinition.SubType.Key)
	assert.Equal(t, DataTypeList, message.Fields["o"].DataTypeDefinition.SubType.SubType.Type)
	assert.Equal(t, DataTypeString, message.Fields["o"].DataTypeDefinition.SubType.SubType.SubType.Type)

	assert.Equal(t, "p", message.Fields["p"].Name)
	assert.Equal(t, uint(16), message.Fields["p"].Index)
	assert.Equal(t, DataTypeCustom, message.Fields["p"].DataTypeDefinition.Type)
	assert.Equal(t, "CustomType", message.Fields["p"].DataTypeDefinition.CustomType)

	assert.Equal(t, "q", message.Fields["q"].Name)
	assert.Equal(t, uint(17), message.Fields["q"].Index)
	assert.Equal(t, DataTypeList, message.Fields["q"].DataTypeDefinition.Type)
	assert.Equal(t, DataTypeCustom, message.Fields["q"].DataTypeDefinition.SubType.Type)
	assert.Equal(t, "CustomType", message.Fields["q"].DataTypeDefinition.SubType.CustomType)
	assert.Equal(t, "custom", message.Fields["q"].DataTypeDefinition.SubType.CustomTypePackage)

	assert.Equal(t, "r", message.Fields["r"].Name)
	assert.Equal(t, uint(18), message.Fields["r"].Index)
	assert.Equal(t, DataTypeMap, message.Fields["r"].DataTypeDefinition.Type)
	assert.Equal(t, DataComparableTypeString, message.Fields["r"].DataTypeDefinition.Key)
	assert.Equal(t, DataTypeCustom, message.Fields["r"].DataTypeDefinition.SubType.Type)
	assert.Equal(t, "CustomType", message.Fields["r"].DataTypeDefinition.SubType.CustomType)
	assert.Equal(t, "some.other_package", message.Fields["r"].DataTypeDefinition.SubType.CustomTypePackage)
}

func TestMessageListParser(t *testing.T) {

	tokens, err := tokenizeFile(`
		# commment the message
		message TestMessage {
			list<string> list_field = 0;
		}
	`)
	require.Nil(t, err)

	_, err = parseMessageDefinitions(tokens)
	require.Nil(t, err)
}

func TestMessageMapParser(t *testing.T) {

	tokens, err := tokenizeFile(`
		# commment the message
		message TestMessage {
			map<string, string> map_field = 0;
		}
	`)
	require.Nil(t, err)

	_, err = parseMessageDefinitions(tokens)
	require.Nil(t, err)
}

func TestMessageNestedParser(t *testing.T) {

	tokens, err := tokenizeFile(`
		# commment the message
		message TestMessage {
			list<map<string, string>> list_of_maps_field = 0;
			map<string, map<string, string>> map_of_maps_field = 1;
		}
	`)
	require.Nil(t, err)

	_, err = parseMessageDefinitions(tokens)
	require.Nil(t, err)
}

func TestMultipleMessages(t *testing.T) {

	tokens, err := tokenizeFile(`
		# my message type
		message TestMessageA {
			byte a = 0;
			uint8 b = 1;
		}

		message TestMessageB {
			uint16 c = 0;
			# this guy here
			uint32 d = 1;
		}

		message TestMessageC {
			uint64 # this type
			e # this name
			= 0; # last comment
		}
	`)
	require.Nil(t, err)

	messages, err := parseMessageDefinitions(tokens)
	require.Nil(t, err)

	assert.Equal(t, 3, len(messages))

	messageA := messages["TestMessageA"]
	messageB := messages["TestMessageB"]
	messageC := messages["TestMessageC"]

	assert.Equal(t, "TestMessageA", messageA.Name)
	assert.Equal(t, 2, len(messageA.Fields))

	assert.Equal(t, "TestMessageB", messageB.Name)
	assert.Equal(t, 2, len(messageB.Fields))

	assert.Equal(t, "TestMessageC", messageC.Name)
	assert.Equal(t, 1, len(messageC.Fields))
}
