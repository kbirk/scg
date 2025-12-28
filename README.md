# scg - Simple Code Generator

This is a toy code generator for generating messages and RPC client / server boilerplate.

Similar to protobuf + gRPC, but worse in every conceivable way.

Message code is generated for both golang and C++ with JSON and binary serialization.

RPCs are implemented over pluggable transports (WebSocket and TCP currently supported). Client and server code is generated for both golang and C++.

Serialization uses bitpacked variable-length integer encoding with zigzag encoding for signed integers.

## Installation:

```sh
go install github.com/kbirk/scg/cmd/scg-go@latest
go install github.com/kbirk/scg/cmd/scg-cpp@latest
```

## Dependencies:

### Golang:

-   Websockets: [gorilla/websocket](https://github.com/gorilla/websocket)
-   TCP

### C++:

-   JSON serialization: [nlohmann/json](https://github.com/nlohmann/json)
-   Websockets: [websocketpp](https://github.com/zaphoyd/websocketpp) and [Asio](https://think-async.com/Asio/AsioStandalone)
-   SSL: [openssl](https://github.com/openssl/openssl)

## Syntax:

Shameless rip-off of protobuf / gRPC with a few simplifications and modifications.

```
package pingpong;

service PingPong {
	rpc Ping (PingRequest) returns (PongResponse);
}

message Ping {
	int32 count = 0;
}

message Pong {
	int32 count = 0;
}

message PingRequest {
	Ping ping = 0;
}

message PongResponse {
	Pong pong = 0;
}
```

Containers such as maps and lists use `<T>` syntax and can be nested:

```
message OtherStuff {
	map<string, float64> map_field = 0;
	list<uint64> list_field = 1;
	map<int32, list<map<string, list<uint8>>>> what_have_i_done = 2;
}
```

## Generating Go Code:

```sh
scg-go --input="./src/dir"  --output="./output/dir" --base-package="github.com/yourname/repo"
```

## Generating C++ Code:

```sh
scg-cpp --input="./src/dir"  --output="./output/dir"
```

## JSON Serialization

JSON serialization for C++ uses [nlohmann/json](https://github.com/nlohmann/json).

```cpp
#include "pingpong.h"

pingpong::PingRequest src;
src.ping.count = 42;

auto bs = req.toJSON();

pingpong::PingRequest dst;

auto err = dst.fromJSON(bs);
assert(!err && "deserialization failed");
```

JSON serialization for golang uses `encoding/json`.

```go
src := pingpong.PingRequest{
	Ping: {
		Count: 42,
	}
}

bs := src.ToJSON()

dst := pingpong.PingRequest{}

err := dst.FromJSON(bs)
if err != nil {
	panic(err)
}
```

## Binary Serialization

Binary serialization encodes the data in a portable payload using a single allocation for the destination buffer.

```cpp
#include "pingpong.h"

pingpong::PingRequest src;
src.ping.count = 42;

auto bs = req.toBytes();

pingpong::PingRequest dst;

auto err = dst.fromBytes(bs);
assert(!err && "deserialization failed");
```

```go
src := pingpong.PingRequest{
	Ping: {
		Count: 42,
	}
}

bs := src.ToBytes()

dst := pingpong.PingRequest{}

err := dst.FromBytes(bs)
if err != nil {
	panic(err)
}
```

## RPCs

The RPC system supports pluggable transports through the `Transport` interface. Both WebSocket and TCP transports are provided.

### Transport Interface

The transport layer is defined by three main interfaces:

```go
// Connection represents a bidirectional communication channel
type Connection interface {
	Send(data []byte) error
	Receive() ([]byte, error)
	Close() error
}

// ServerTransport handles incoming connections for the server
type ServerTransport interface {
	Listen() error
	Accept() (Connection, error)
	Close() error
}

// ClientTransport handles outgoing connections for the client
type ClientTransport interface {
	Connect() (Connection, error)
}
```

### Go Server

Both client and server code is generated for golang. The server uses a transport-based configuration:

```go
import (
	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/websocket"
	"github.com/yourname/repo/pingpong"
)

// Create server with WebSocket transport
server := rpc.NewServer(rpc.ServerConfig{
	Transport: websocket.NewServerTransport(
	websocket.ServerTransportConfig{
	Port: 8080,
	// Optional: for TLS
	// CertFile: "server.crt",
	// KeyFile: "server.key",
	}),
	ErrHandler: func(err error) {
	log.Printf("Server error: %v", err)
	},
})

// Register your service implementation
pingpong.RegisterPingPongServer(server, &pingpongServer{})

// Start the server
server.ListenAndServe()
```

### C++ Server

C++ server code is available for WebSocket and TCP transports:

```cpp
#include "scg/server.h"
#include "scg/tcp/transport_server.h"
#include "pingpong/pingpong.h"
#include <thread>
#include <chrono>

// Implement the service interface
class PingPongServerImpl : public pingpong::PingPongServer {
public:
	std::pair<pingpong::PongResponse, scg::error::Error> ping(
	const scg::context::Context& ctx,
	const pingpong::PingRequest& req) override {

	pingpong::PongResponse response;
	response.pong.count = req.ping.count + 1;
	response.pong.payload = req.ping.payload;

	return std::make_pair(response, nullptr);
	}
};

int main() {
	// Configure TCP transport
	scg::tcp::ServerTransportConfig transportConfig;
	transportConfig.port = 8080;

	// Configure server
	scg::rpc::ServerConfig config;
	config.transport = std::make_shared<scg::tcp::ServerTransportTCP>(transportConfig);
	config.errorHandler = [](const scg::error::Error& err) {
	printf("Server error: %s\n", err.message.c_str());
	};

	// Create server
	auto server = std::make_shared<scg::rpc::Server>(config);

	// Register service implementation
	auto impl = std::make_shared<PingPongServerImpl>();
	pingpong::registerPingPongServer(server.get(), impl);

	// Start server (starts background threads)
	server->start();

	// Keep main thread alive
	while (true) {
	std::this_thread::sleep_for(std::chrono::seconds(1));
	}

	return 0;
}
```

For TLS connections, use `scg::tcp::ServerTransportTCPTLS`:

```cpp
#include "scg/tcp/transport_server_tls.h"

scg::tcp::ServerTransportTLSConfig transportConfig;
transportConfig.port = 443;
transportConfig.certFile = "server.crt";
transportConfig.keyFile = "server.key";

config.transport = std::make_shared<scg::tcp::ServerTransportTCPTLS>(transportConfig);
```

For WebSocket connections, use `scg::ws::ServerTransportWS` or `scg::ws::ServerTransportWSTLS`:

```cpp
#include "scg/ws/transport_server.h"

scg::tcp::ServerTransportConfig transportConfig;
transportConfig.port = 8080;
transportConfig.logging = logging;

config.transport = std::make_shared<scg::tcp::ServerTransportNoTLS>(transportConfig);
```

### Go Client

The client also uses transport-based configuration:

```go
import (
	"context"
	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/websocket"
	"github.com/yourname/repo/pingpong"
)

// Create client with WebSocket transport
client := rpc.NewClient(rpc.ClientConfig{
	Transport: websocket.NewClientTransport(
	websocket.ClientTransportConfig{
	Host: "localhost",
	Port: 8080,
	// Optional: for TLS
	// TLSConfig: &tls.Config{...},
	}),
	ErrHandler: func(err error) {
	log.Printf("Client error: %v", err)
	},
})

c := pingpong.NewPingPongClient(client)

resp, err := c.Ping(context.Background(), &pingpong.PingRequest{
	Ping: pingpong.Ping{
	Count: 0,
	},
})
if err != nil {
	panic(err)
}
fmt.Println(resp.Pong.Count)
```

**Note:** A single `rpc.Client` can be used with multiple services. Service routing is handled automatically by the generated client code:

```go
// Single client for multiple services
client := rpc.NewClient(rpc.ClientConfig{
	Transport: websocket.NewClientTransport(
	websocket.ClientTransportConfig{
	Host: "localhost",
	Port: 8080,
	}),
})

// Create clients for different services using the same transport
serviceAClient := servicea.NewServiceAClient(client)
serviceBClient := serviceb.NewServiceBClient(client)

// Each service client automatically routes to the correct service
respA, _ := serviceAClient.MethodA(ctx, &servicea.RequestA{})
respB, _ := serviceBClient.MethodB(ctx, &serviceb.RequestB{})
```

### Middleware

Both client and server support middleware for cross-cutting concerns:

```go
// Server middleware
server.Middleware(func(ctx context.Context, next rpc.Handler) rpc.Handler {
	return func(ctx context.Context, req interface{}) (interface{}, error) {
	// Pre-processing
	log.Printf("Handling request...")

	resp, err := next(ctx, req)

	// Post-processing
	log.Printf("Request complete")

	return resp, err
	}
})

// Client middleware
client.Middleware(func(ctx context.Context, next rpc.Handler) rpc.Handler {
	return func(ctx context.Context, req interface{}) (interface{}, error) {
	// Pre-processing
	log.Printf("Sending request...")

	resp, err := next(ctx, req)

	// Post-processing
	log.Printf("Received response")

	return resp, err
	}
})
```

### C++ Client

C++ client example using WebSocket transport:

```cpp
#include <scg/ws/transport_client.h>

#include "pingpong.h"

scg::ws::ClientTransportConfigNoTLS config;
config.host = "localhost";
config.port = 8080;

auto client = std::make_shared<scg::ws::ClientTransportNoTLS>(config);

pingpong::PingPongClient pingPongClient(client);

pingpong::PingRequest req;
req.ping.count = 0;

auto [res, err] = pingPongClient.ping(scg::context::background(), req);
if (err) {
	std::cerr << "Request failed: " << err.message() << std::endl;
} else {
	std::cout << res.pong.count << std::endl;
}
```

For TLS connections:

```cpp
#include <scg/ws/transport_tls.h>

#include "pingpong.h"

scg::ws::ClientTransportConfigTLS config;
config.host = "localhost";
config.port = 443;

auto client = std::make_shared<scg::ws::ClientTransportTLS>(config);

pingpong::PingPongClient pingPongClient(client);

pingpong::PingRequest req;
req.ping.count = 0;

auto [res, err] = pingPongClient.ping(scg::context::background(), req);
if (err) {
	std::cerr << "Request failed: " << err.message() << std::endl;
} else {
	std::cout << res.pong.count << std::endl;
}
```

## SCG C++ Serialization Macros

The C++ `include/scg/macro.h` provides some macros for building serialization overrides for types that are _not_ generated with scg.

There are four macros:

-   `SCG_SERIALIZABLE_PUBLIC`: declare public fields as serializable.
-   `SCG_SERIALIZABLE_PRIVATE`: declare public _and_ private fields as serializable.
-   `SCG_SERIALIZABLE_DERIVED_PUBLIC`: declare a type as derived from another, include any base class serialization logic, along with new public fields.
-   `SCG_SERIALIZABLE_DERIVED_PRIVATE`: declare a type as derived from another, and include any base class serialization logic, along with new public _and_ private fields.

```cpp
// Declare public fields as serializable, note the macro is called _outside_ the struct.
struct MyStruct {
	uint32_t a = 0;
	float64_t b = 0;
	std::vector<std::string> c;
};
SCG_SERIALIZABLE_PUBLIC(MyStruct, a, b, c);

// Declare declare private fields as serializable, note the macro is called _inside_ the class.
class MyClass {
public:
	MyClass() = default;
	MyClass(uint32_t a, float64_t b) : a_(a), b_(b)
	{
	}
	SCG_SERIALIZABLE_PRIVATE(MyClass, a_, b_);
private:
	uint32_t a_ = 0;
	uint64_t b_ = 0;
};

// Declare the base class to derive serialization logic from, note the macro is called _outside_ the struct.
struct DerivedStruct : MyStruct{
	bool d = false;
};
SCG_SERIALIZABLE_DERIVED_PUBLIC(DerivedStruct, MyStruct, d);

// Declare the base class to derive serialization logic from, note the macro is called _inside_ the class.
class MyDerivedClass : public MyClass {
public:
	MyDerivedClass() = default;
	MyDerivedClass(uint32_t a, float64_t b, bool c) : MyClass(a, b), c_(c)
	{
	}
	SCG_SERIALIZABLE_DERIVED_PRIVATE(MyDerivedClass, MyClass, c_);
private:
	bool c_ = false;
};
```

Individual serialization overrides can be provided using ADL as follows, for example, here is how to extend it to serialize `glm` types:

```cpp
namespace glm {

template <typename WriterType>
inline void serialize(WriterType& writer, const glm::vec2& value)
{
	writer.write(value.x);
	writer.write(value.y);
}

template <typename ReaderType>
inline scg::error::Error deserialize(glm::vec2& value, ReaderType& reader)
{
	auto err = reader.read(value.x);
	if (err) {
		return err;
	}
	return reader.read(value.y);
}

}
```

## Development / Testing:

Generate test files:

```sh
./gen-test-code.sh
```

Generate SSL keys for test server:

```sh
./gen-ssl-keys.sh
```

Download and vendor the third party header files:

```sh
cd ./third_party && ./install-deps.sh &&  cd ..
```

Run the tests:

```sh
./run-tests.sh
```

## TODO:

-   Implement context cancellations and deadlines
-   Opentracing hooks and context serialization
-   Add stream support
-   Add C++ server code
