# scg - Simple Code Generator

This is a toy code generator for generating messages and RPC client / server boilerplate.

Similar to protobuf + gRPC, but worse in every conceivable way.

Message code is generated for both golang and C++ with JSON and binary serialization.

RPCs are implemented over websockets. Client and server code is generated for golang. Only client code is generated for C++.

Serialization uses bitpacked variable-length integer encoding with zigzag encoding for signed integers.

## Installation:

```sh
go install github.com/kbirk/scg/cmd/scg-go@latest
go install github.com/kbirk/scg/cmd/scg-cpp@latest
```

## Dependencies:

### Golang:

- Websockets: [gorilla/websocket](https://github.com/gorilla/websocket)

### C++:

- JSON serialization: [nlohmann/json](https://github.com/nlohmann/json)
- Websockets: [websocketpp](https://github.com/zaphoyd/websocketpp) and [Asio](https://think-async.com/Asio/AsioStandalone)
- SSL: [openssl](https://github.com/openssl/openssl)

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

Both client and server code is generated for golang:

```go
// server

server := rpc.NewServer(rpc.ServerConfig{
	Port: 8080,
	ErrHandler: func(err error) {
		require.NoError(t, err)
	},
})
pingpong.RegisterPingPongServer(server, &pingpongServer{})

server.ListenAndServe()

// client
client := rpc.NewClient(rpc.ClientConfig{
	Host: "localhost",
	Port: 8080,
	ErrHandler: func(err error) {
		require.NoError(t, err)
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

Only client code is generated for C++:

```cpp
#include <scg/client_no_tls.h>

#include "pingpong.h"

scg::rpc::ClientConfig config;
config.uri = "localhost:8080";

auto client = std::make_shared<scg::rpc::ClientNoTLS>(config);

pingpong::PingPongClient pingPongClient(client);

pingpong::PingRequest req;
req.ping.count = 0;

auto [res, err] = pingPongClient.ping(scg::context::background(), req);
assert(err && "request failed");

std::cout << res.pong.count << std::endl;
```

## SCG C++ Serialization Macros

The C++ `include/scg/macro.h` provides some macros for building serialization overrides for types that are _not_ generated with scg.

There are four macros:

- `SCG_SERIALIZABLE_PUBLIC`: declare public fields as serializable.
- `SCG_SERIALIZABLE_PRIVATE`: declare public _and_ private fields as serializable.
- `SCG_SERIALIZABLE_DERIVED_PUBLIC`: declare a type as derived from another, include any base class serialization logic, along with new public fields.
- `SCG_SERIALIZABLE_DERIVED_PRIVATE`: declare a type as derived from another, and include any base class serialization logic, along with new public _and_ private fields.

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
````

## TODO:

- Implement context cancellations and deadlines
- Opentracing hooks and context serialization
- Add stream support
- Add C++ server code
