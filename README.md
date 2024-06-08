# scg - Simple Code Generator

This is a toy code generator for generating messages and RPC client / server boilerplate. Similar to protobuf + gRPC, but worse in every conceivable way.

Message code is generated for both golang and C++ with JSON and binary serialization.

RPCs are implemented over websockets. Client and server code is generated for golang. Only client code is generated for C++.

## Dependencies:

### Golang:

- Websockets: [gorilla/websocket](https://github.com/gorilla/websocket)

### C++:

- JSON serialization: [nlohmann/json](https://github.com/nlohmann/json)
- Websockets: [websocketpp](https://github.com/zaphoyd/websocketpp) and [Asio](https://think-async.com/Asio/AsioStandalone)
- SSL: [openssl](https://github.com/openssl/openssl)

## Syntax:

Shameless rip-off of protobuf / gRPC, minus the braces on rpc method declarations.

```proto
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

Containers such as maps and lists use golang syntax and can be nested:

```proto
message OtherStuff {
	map[string]float64 map_field = 0;
	[]uint64 list_field = 1;
	map[int32][]map[string][]uint8 what_have_i_done = 2;
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

- Implement context cancellations and deadlines.
- Add stream support
- Add [vscode syntax highlighting](https://code.visualstudio.com/api/language-extensions/syntax-highlight-guide)
- Add C++ server code?
