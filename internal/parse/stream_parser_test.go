package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamParser(t *testing.T) {

	tokens, err := tokenizeFile(`
		stream TestStream {
			client SendFromClient (ClientMessage) returns (ServerResponse);
			server SendFromServer (ServerMessage) returns (ClientResponse);
		}
	`)
	require.Nil(t, err)

	streams, err := parseStreamDefinitions(tokens)
	require.Nil(t, err)

	assert.Equal(t, 1, len(streams))

	stream := streams["TestStream"]

	assert.Equal(t, "TestStream", stream.Name)
	assert.Equal(t, 2, len(stream.Methods))

	assert.Equal(t, "SendFromClient", stream.Methods["SendFromClient"].Name)
	assert.Equal(t, StreamMethodDirectionClient, stream.Methods["SendFromClient"].Direction)
	assert.Equal(t, DataTypeCustom, stream.Methods["SendFromClient"].Argument.Type)
	assert.Equal(t, "ClientMessage", stream.Methods["SendFromClient"].Argument.CustomType)
	assert.Equal(t, DataTypeCustom, stream.Methods["SendFromClient"].Return.Type)
	assert.Equal(t, "ServerResponse", stream.Methods["SendFromClient"].Return.CustomType)

	assert.Equal(t, "SendFromServer", stream.Methods["SendFromServer"].Name)
	assert.Equal(t, StreamMethodDirectionServer, stream.Methods["SendFromServer"].Direction)
	assert.Equal(t, DataTypeCustom, stream.Methods["SendFromServer"].Argument.Type)
	assert.Equal(t, "ServerMessage", stream.Methods["SendFromServer"].Argument.CustomType)
	assert.Equal(t, DataTypeCustom, stream.Methods["SendFromServer"].Return.Type)
	assert.Equal(t, "ClientResponse", stream.Methods["SendFromServer"].Return.CustomType)
}

func TestServiceWithStreamReturn(t *testing.T) {

	tokens, err := tokenizeFile(`
		service TestService {
			rpc OpenStream (OpenStreamRequest) returns (stream TestStream);
			rpc RegularMethod (Request) returns (Response);
		}
	`)
	require.Nil(t, err)

	services, err := parseServiceDefinitions(tokens)
	require.Nil(t, err)

	assert.Equal(t, 1, len(services))

	service := services["TestService"]

	assert.Equal(t, "TestService", service.Name)
	assert.Equal(t, 2, len(service.Methods))

	assert.Equal(t, "OpenStream", service.Methods["OpenStream"].Name)
	assert.True(t, service.Methods["OpenStream"].ReturnsStream)
	assert.Equal(t, "TestStream", service.Methods["OpenStream"].StreamName)
	assert.Equal(t, DataTypeCustom, service.Methods["OpenStream"].Argument.Type)
	assert.Equal(t, "OpenStreamRequest", service.Methods["OpenStream"].Argument.CustomType)

	assert.Equal(t, "RegularMethod", service.Methods["RegularMethod"].Name)
	assert.False(t, service.Methods["RegularMethod"].ReturnsStream)
	assert.Equal(t, "", service.Methods["RegularMethod"].StreamName)
	assert.Equal(t, DataTypeCustom, service.Methods["RegularMethod"].Return.Type)
	assert.Equal(t, "Response", service.Methods["RegularMethod"].Return.CustomType)
}
