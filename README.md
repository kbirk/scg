# scg - Simple Code Generator

A lightweight, high-performance code generator for messages and RPC client/server boilerplate.

SCG provides efficient serialization and RPC performance while maintaining a significantly smaller footprint and simpler codebase than heavyweight alternatives.

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
-   TCP: [net](https://pkg.go.dev/net)

### C++:

-   JSON serialization: [nlohmann/json](https://github.com/nlohmann/json)
-   Websockets: [websocketpp](https://github.com/zaphoyd/websocketpp) and [asio](https://think-async.com/Asio/AsioStandalone)
-   TCP: [asio](https://think-async.com/Asio/AsioStandalone)
-   SSL: [openssl](https://github.com/openssl/openssl)

The headers require C++17 and build with GCC and MSVC 2022 (C++17 or C++20, `/permissive-`), with or without `NOMINMAX` / `WIN32_LEAN_AND_MEAN`. scg's own headers and the code it generates produce no MSVC warnings at `/W4`. websocketpp still triggers a few inside the standard library when the WebSocket transport is used.

## Syntax:

Familiar protobuf-inspired syntax with practical simplifications and enhancements.

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

Go and C++ produce and accept the same JSON values, so a message serialized in one language can be parsed in the other. The format follows Go's `encoding/json` defaults:

| `.scg` type                                                  | JSON                                                                                                     |
| ------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------- |
| message                                                      | object keyed by the `.scg` field names; every field is written                                           |
| `bool`, integers (all widths), floats, enums                 | `true`/`false` and exact JSON numbers (NaN and ±Inf cannot be encoded)                                   |
| `string`                                                     | string                                                                                                   |
| `uuid`                                                       | lowercase canonical string, e.g. `"123e4567-e89b-12d3-a456-426614174000"`                                |
| `timestamp`                                                  | RFC 3339 with nanoseconds, e.g. `"2024-05-01T12:30:00.5Z"`; any UTC offset is accepted when parsing      |
| typedef                                                      | encoded as its underlying type (a numeric typedef is a number)                                           |
| `list<byte>`, `list<uint8>` (and lists of byte/uint8 typedefs) | standard base64 string                                                                                   |
| other `list<T>`                                              | array                                                                                                    |
| `map<K, V>`                                                  | object; integer and enum keys are written in base 10, uuid keys as canonical strings                     |

When parsing, both languages behave like Go's `encoding/json`:
-   A missing key leaves the field unchanged, and unknown keys are ignored, so JSON from an older or newer schema still loads.
-   A JSON `null` list or map decodes as empty (Go writes nil slices and maps as `null`), and a `null` list element decodes as that type's zero value.
-   `-0` keeps its sign in float fields, as in Go (C++: when the text is parsed by the generated `fromJSON` or `scg::json::parse`, not by `nlohmann::json::parse`).
-   Invalid UTF-8 is replaced with U+FFFD, byte for byte as Go does.

Known differences:
-   The two languages write equal values, not identical text. C++ spells some numbers differently (`16777216.0` and `1e+20` where Go writes `16777216` and `100000000000000000000`), sorts object keys instead of using field order, and does not escape `<`, `>` and `&`. Compare decoded messages, never JSON text.
-   Maps with float keys can't be exchanged, because Go's `encoding/json` cannot encode them.
-   C++ matches keys exactly; Go also falls back to case-insensitive matching.
-   C++ timestamps cover roughly the years 1678 to 2262. Instants outside that range are clamped to the nearest end, the same as binary decoding, and Go's zero `time.Time` maps to the earliest C++ timestamp and back.
-   A float32 value written with more precision than float32 holds (by a non-Go producer) can round differently by one unit in the last place. Values written by Go or C++ always round-trip exactly.
-   Parsing into a message that already holds data replaces its lists and maps in C++ (as binary decoding does). Go merges into an existing map and reuses existing list elements.

JSON serialization for C++ uses [nlohmann/json](https://github.com/nlohmann/json). The generated code converts fields through `scg/json.h`. The plain nlohmann conversions for `scg::type::uuid`, `scg::type::timestamp` and `SCG_TYPEDEF` types are unchanged, so your own nlohmann-based JSON that uses those types is not affected.

```cpp
#include "pingpong.h"

pingpong::PingRequest src;
src.ping.count = 42;

auto bs = src.toJSON();

pingpong::PingRequest dst;

// throws nlohmann::json::exception on malformed or mistyped JSON
dst.fromJSON(bs);
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
	Send(data []byte, serviceID uint64) error
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

// ContextClientTransport is optional. When a transport implements it, the
// call's context deadline also bounds the dial and any TLS/websocket handshake.
type ContextClientTransport interface {
	ClientTransport
	ConnectContext(ctx context.Context) (Connection, error)
}
```

Transports should return (or wrap) `rpc.ErrConnectionClosed` when the peer closes cleanly, so the client and server can tell a normal close from an error.

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
	printf("Server error: %s\n", err.message().c_str());
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

### Client Interfaces and Fakes

Alongside each `XxxClient`, scg generates an abstract `XxxApi` interface that mirrors the
client's call surface one-to-one, plus an `XxxFake` test double. This gives orchestration
logic a mockable seam without hand-writing gateway/forwarding boilerplate — it is the
symmetric counterpart to the server-side `XxxServer` interface.

Only unary rpcs are part of the interface; streaming rpcs stay on the concrete client.

**C++** — `XxxClient` is `final` and derives from `XxxApi`; `XxxFake` exposes one
`std::function` hook per rpc (`onMethodName`) and returns a default-constructed response
with no error when a hook is unset:

```cpp
// Production code depends on the interface, not the concrete client:
void runCheckpoint(pingpong::PingPongApi& api) {
	pingpong::PingRequest req;
	auto [res, err] = api.ping(scg::context::background(), req);
	// ...
}

// Wire the real client in production:
pingpong::PingPongClient client(transport);
runCheckpoint(client);

// Substitute a scripted fake in tests:
pingpong::PingPongFake fake;
fake.onPing = [](scg::context::Context& ctx, const pingpong::PingRequest& req) {
	pingpong::PongResponse resp;
	resp.pong.count = req.ping.count + 1;
	return std::make_pair(resp, scg::error::Error(nullptr));
};
runCheckpoint(fake);
```

The fake is deliberately minimal — hooks only, no built-in call recording or thread-safety;
tests layer those per their own needs.

**Go** — `XxxApi` is an interface satisfied by `*XxxClient` (enforced with a compile-time
`var _ XxxApi = (*XxxClient)(nil)` assertion). Depend on `XxxApi` in code you want to fake;
since the generated `XxxServer` interface has the same unary signatures, an in-process test
can even hand a server implementation directly to client-side code.

```go
type PingPongApi interface {
	Ping(ctx context.Context, req *PingRequest) (*PongResponse, error)
}
```

### Streaming RPCs

Mark a request and/or response type with the `stream` qualifier to make an RPC a
stream. A stream is opened once and both sides read from and write to it until
either side closes it (à la gRPC). Unary RPCs are unaffected and share the same
connection and wire format.

```
service Chat {
	# bidirectional: client sends events, server pushes events, until either side closes
	rpc Connect (stream ChatMessage) returns (stream ChatMessage);
}

message ChatMessage {
	string text = 0;
	int32 seq = 1;
}
```

**Go — client:**

```go
chat := pingpong.NewChatClient(client)

stream, err := chat.Connect(context.Background()) // opens the stream; no message sent yet
_ = stream.Send(&pingpong.ChatMessage{Text: "hello"})
msg, err := stream.Recv()                          // blocks; err == io.EOF on clean close
_ = stream.CloseSend()                             // half-close: done sending, still receiving
```

**Go — server:**

```go
func (s *chatServer) Connect(stream *pingpong.Chat_ConnectStreamServer) error {
	for {
		msg, err := stream.Recv()        // io.EOF when the client half-closes
		if err == io.EOF { return nil }
		if err != nil { return err }
		_ = stream.Send(&pingpong.ChatMessage{Text: "echo:" + msg.Text})
	}
}
```

**C++ — client (no exceptions; non-blocking drain suited to a game loop):**

```cpp
pingpong::ChatClient chat(client);

auto [stream, err] = chat.connect(scg::context::background());
err = stream->send(ev);                  // non-blocking

// Per-frame, drain without blocking (recommended for a single-threaded game loop):
for (;;) {
	auto r = stream->tryRecv();          // never blocks
	if (r.state == scg::rpc::StreamRecvState::Empty)  break;   // nothing this frame
	if (r.state == scg::rpc::StreamRecvState::Closed) break;   // r.error is nil on clean close
	handle(r.message);
}

err = stream->closeSend();
// auto blocking = stream->recv();       // also available for non-frame-locked consumers
```

The C++ client receives on the transport's existing background I/O thread and
delivers into a bounded per-stream queue; `tryRecv()`/`send()` never block, so
all stream handling stays on the caller's thread. See `streaming.md` for the
full threading-model rationale and wire protocol.

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

## Compatibility and Runtime Behaviour

### Wire compatibility

-   Service and method IDs are hashed from package-qualified names (`pkg.Service` and `pkg.Service/Method`), so services with the same name in different packages no longer collide. **Code generated by earlier versions of scg cannot call, or be called by, code generated by this version.** Regenerate every peer, Go and C++, and deploy them together.
-   Generated C++ JSON now uses the shared Go-compatible format described in [JSON Serialization](#json-serialization). Keys are the `.scg` field names (snake_case). Timestamps are RFC 3339 strings, numeric typedefs are numbers, byte lists are base64, and non-string-keyed maps are objects. Don't parse JSON written by older C++ code with the new code: its camelCase keys are unknown keys now, so they are ignored and those fields silently keep their current values.
-   Generated Go `uuid` typedefs now encode in JSON as canonical strings, like `uuid.UUID` itself (they were arrays of 16 numbers) and can be used as map keys. The old array form is still accepted when parsing, but Go code generated by earlier versions cannot parse the new string form.
-   The binary encoding of messages is otherwise unchanged, including data persisted with the C++ `Writer`/`Reader`.

### Schema rules

The generator rejects, with an error that gives the file and line:

-   `list<T>` where `T` encodes to zero bits (a message with no fields, or only zero-bit fields). A hostile length prefix on such a list would otherwise cost unbounded decode time.
-   Identifiers that collide with a language keyword, a generated method, or another identifier after case conversion (e.g. `foo_bar` and `fooBar`).
-   Duplicate enum string values, integer constants (and float constants written as integers) with leading zeros, `NaN`/`Inf` floats, and unquoted or non-canonical UUID constants.

Generated output is deterministic. `./run-codegen-checks.sh` verifies this, compiles every generated header, and runs `go vet`.

### Decoding limits

Generated decoders reject a container whose declared length cannot fit in the remaining bytes, and cap preallocation, so a small frame cannot force a large allocation. The per-frame receive cap (`MaxRecvMessageSize`, default 32 MiB) still bounds total input. Messages that decode to many small elements can still expand by roughly the in-memory size of each element relative to its wire size. Lower the receive cap if that matters for your deployment.

### Go runtime

-   **TCP:** each frame is sent with a single write, and `TCP_NODELAY` is always left enabled (Go's default). The `NoDelay` config field is kept for compatibility; setting it to `false` no longer turns Nagle's algorithm on.
-   **The server always replies.** A request for an unknown service, a request arriving during shutdown, or a response the server cannot send (for example, one over `MaxSendMessageSize`) produces an error response instead of leaving the caller waiting.
-   **`Server.Shutdown(ctx)`** stops accepting connections, rejects new requests and streams with `rpc.ErrServerShuttingDown`, waits for in-flight unary calls to finish, then closes every connection (cancelling open streams) and waits for their handlers to exit. If `ctx` expires first, it force-closes the remaining connections (without waiting on stuck peers) and returns `ctx.Err()`. Transports can support this by implementing the optional `rpc.ForceCloser` interface. A server cannot be restarted after `Shutdown`.
-   **A failed write closes the connection.** If a send fails partway (for example on a `WriteTimeout`), the connection is torn down rather than reused with a truncated frame, and in-flight calls on it fail.
-   **Sentinel errors:** `rpc.ErrConnectionClosed`, `rpc.ErrTransportClosed`, `rpc.ErrServerShuttingDown`, `rpc.ErrServerBusy` and `rpc.ErrMessageTooLarge`, checked with `errors.Is` or `rpc.IsConnectionClosed(err)`. `Client.Close()` does not invoke `ErrHandler`.
-   **The client never holds its lock across network I/O.** Concurrent calls share a single dial, and each caller's context deadline bounds how long it waits. A connection dropped mid-call fails that call promptly.
-   **Unary handler contexts** are cancelled when the client's connection drops.
-   **Optional configuration:**
    -   `ServerConfig.MaxConcurrentRequests`: running unary requests per connection (0 = unlimited). Requests beyond the limit wait in a first-in-first-out queue of the same size. If that queue is also full, the caller immediately gets `rpc.ErrServerBusy`. The connection keeps reading throughout, so keepalives and streams are unaffected.
    -   `WriteTimeout` on the TCP, TLS and WebSocket transports. 0 means no timeout.
    -   `HandshakeTimeout` on the TLS transports. The default is 10 s, applied to server and client handshakes.
-   **WebSocket:** `Listen` returns bind and certificate errors directly. The client handshake times out after `websocket.DefaultHandshakeTimeout` (45 s), and the server applies a 30 s read-header timeout.
-   **Keepalive:** a `KeepaliveTimeout` that is not greater than `KeepaliveInterval` is raised to twice the interval.
-   **Metadata:** `rpc.AppendMetadataToContext` merges into a copy of the existing metadata instead of replacing it, and never mutates the parent context's metadata.

### C++ runtime

-   **TLS:** `tcp::ClientTransportTLSConfig::verifyPeer` defaults to `true`, and the TLS clients verify the server hostname. Set `verifyPeer = false` explicitly for self-signed development certificates.
-   **Timeouts and limits:**
    -   `connectTimeout`: client transports bound connect (including the TLS handshake) by the call's deadline, or by this value (default 10 s).
    -   `handshakeTimeout`: TLS servers drop connections that don't complete the handshake within this time (default 10 s).
    -   `maxWriteQueueBytes`: each connection's outbound queue is capped (default 64 MiB; 0 disables the cap).
-   **Server shutdown** closes accepted connections and waits for in-flight unary and stream handlers. The server always replies, as described for Go. Exceptions thrown by stream handlers or middleware close the stream with an error instead of terminating the process.
-   **Client:** errors carry their real cause (for example "Connection closed" or "keepalive timeout") instead of "Unknown error". `disconnect()` followed by a new call reconnects. Dropping the last handle to a live `ClientStream` cancels it on the server.
-   **Serialization:**
    -   UUIDs of any version are accepted.
    -   Deserializing into a `std::map`, `std::unordered_map`, `std::set` or `std::unordered_set` replaces its contents, as it already did for `std::vector`.
    -   `Writer::sizeBytes()` and `WriterView::sizeBytes()` return the number of bytes written so far. `bytes()` returns the whole buffer, which can be pre-sized larger than that.
    -   Timestamps before 1970 are written in the same canonical form Go uses; the older form still decodes.
    -   Timestamps outside the representable range (roughly 1678 to 2262) decode to the nearest end instead of failing the message. Go's zero `time.Time` decodes to the earliest timestamp, which encodes back as Go's zero time, so an unset Go time survives a round trip through C++.
-   **Generated clients:** the raw-pointer constructor `XClient(scg::rpc::Client*)`, which takes ownership, is deprecated. Construct from a `std::shared_ptr<scg::rpc::Client>`, or use `XClient(scg::rpc::Client&)` for a non-owning reference.
-   **Generated enum string constants** are scoped per enum as `<ENUM>_<VALUE>_STRING` (e.g. `JOB_ACOLYTE_STRING`). The unscoped names remain as deprecated aliases where they are unambiguous. `<ENUM>_ENUM_TO_STRING` maps are unchanged.
-   **Includes:** define `SCG_MINIMAL_INCLUDES` before including scg headers to skip the broad legacy includes (such as all of `<asio.hpp>`) and reduce compile time. Your code must then include what it uses itself.

## Testing:

Generate test files:

```sh
./gen-test-code.sh
```

Generate SSL keys for test server:

```sh
./gen-ssl-keys.sh
```

The C++ TLS tests that check hostname verification generate their own throwaway `localhost` certificate when they run. No keys are committed to the repository.

Download and vendor the third party header files:

```sh
cd ./third_party && ./install-deps.sh &&  cd ..
```

Run the tests:

```sh
./run-codegen-checks.sh
./run-serialize-tests.sh
./run-tcp-tests.sh
./run-websocket-tests.sh
```

The `*ExternalServer` Go tests run only when their external C++ server is reachable, or when `SCG_EXTERNAL_SERVER=1` is set. The run scripts start those servers themselves.

The C++ tests can be built under sanitizers with, for example, `cmake -DSCG_SANITIZE=address,undefined` or `cmake -DSCG_SANITIZE=thread`.

## TODO:

-   Opentracing hooks and context serialization
