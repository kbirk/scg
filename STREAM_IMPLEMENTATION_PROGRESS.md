# Stream Implementation Progress

## Design Overview

### API Design Examples

```scg
service PingPong {
    rpc OpenPingPongStream (OpenPingPongStream) returns (stream PingPongStream);
}

stream PingPongStream {
    client SendPingFromClient (ClientPingMessage) returns (ServerPongResponse);
    server SendPingFromServer (ServerPingMessage) returns (ClientPongResponse);
}
```

#### Go Client Example

```go
// Client will need to define it's stream handlers
type pingpongStreamClient struct {
}

func (s *pingpongStreamClient) SendPingFromServer(ctx context.Context, req *pingpong.ServerPingMessage) (*pingpong.ClientPongResponse, error) {
    return &pingpong.ClientPongResponse{}, nil
}

func main() {
    // Create the rpc client
    client := rpc.NewClient(rpc.ClientConfig{...})

    // Create the ping pong client
    c := pingpong.NewPingPongClient(client)

    // Open the stream, providing the handler implementations (to ensure they exist before the server sends anything)
    stream, err := c.OpenPingPongStream(context.Background(), &pingpongStreamClient{}, &pingpong.OpenPingPongStream{})
    if err != nil {
        // handle
    }

    resp, err := stream.SendPingFromClient(context.Background(), &pingpong.ClientPingMessage{})
    if err != nil {
        // handle
    }

    // close the stream
    stream.Close()
}
```

#### Go Server Example

```go
type pingpongStreamServer struct {
}

func (s *pingpongStreamServer) SendPingFromClient(ctx context.Context, req *pingpong.ClientPingMessage) (*pingpong.ServerPongResponse, error) {
    return &pingpong.ServerPongResponse{}, nil
}

type pingpongServer struct {
}

func (s *pingpongServer) OpenPingPongStream(ctx context.Context, req *pingpong.OpenPingPongStream) (*pingpong.PingPongStream, error) {

    stream := pingpong.NewPingPongStream(&pingpongStreamServer{})

    // capture the stream if you want...
    streamsContainer.Add(stream)

    // Spawn background goroutine for lifecycle
    go func() {
        defer stream.Close()

        for i:=0; i<10; i++ {
            resp, err := stream.SendPingFromServer(context.Background(), &pingpong.ServerPingMessage{})
            if err != nil {
                return
            }
            time.Sleep(100 * time.Millisecond)
        }

        <-stream.Wait()

        // remove from capture...
        streamsContainer.Remove(stream)
    }()

    return stream, nil
}

func main() {
    // Create the rpc server
    server = rpc.NewServer(rpc.ServerConfig{...})

    // Register the server
    pingpong.RegisterPingPongServer(server, &pingpongServer{})

    // Listen
    err := server.ListenAndServe()
    if err != nil {
        fmt.Println(err)
    }
}
```

#### C++ Client Example

```cpp
#include "pingpong/pingpong.h"
#include "scg/client.h"
#include "scg/tcp/transport_client.h"

// Client-side stream handler implementation
class PingPongStreamClientHandler : public pingpong::PingPongStreamClientHandler {
public:
    // Handle server->client RPC calls
    std::pair<pingpong::ClientPongResponse, scg::error::Error>
    sendPingFromServer(const scg::context::Context& ctx, const pingpong::ServerPingMessage& req) override {
        printf("Received ping from server: %s\n", req.message.c_str());

        pingpong::ClientPongResponse response;
        response.timestamp = scg::type::timestamp();
        return std::make_pair(response, nullptr);
    }
};

int main() {
    // ... client setup ...

    auto streamHandler = std::make_shared<PingPongStreamClientHandler>();

    pingpong::OpenPingPongStream openReq;
    openReq.initialMessage = "Hello from client";

    scg::context::Context ctx;
    auto [stream, openErr] = pingPongClient.openPingPongStream(ctx, streamHandler.get(), openReq);
    if (openErr) {
        return 1;
    }

    // Main game loop
    bool running = true;
    int frameCount = 0;

    while (running) {
        // Client also just calls process() - no manual stream processing needed
        client->process();  // This dispatches incoming stream messages

        // Send messages to server
        if (frameCount % 60 == 0) {
            pingpong::ClientPingMessage ping;
            ping.count = frameCount / 60;

            auto [response, err] = stream->sendPingFromClient(scg::context::Context(), ping);
            if (err) {
                running = false;
            }
        }

        if (stream->isClosed()) {
            running = false;
        }

        frameCount++;
        std::this_thread::sleep_for(std::chrono::milliseconds(16));
    }

    stream->close();

    return 0;
}
```

#### C++ Server Example

```cpp
#include "pingpong/pingpong.h"
#include "scg/server.h"
#include "scg/tcp/transport_server.h"
#include <atomic>
#include <csignal>

std::atomic<bool> running(true);

void signalHandler(int signum) {
    running = false;
}

// Server-side stream handler implementation
class PingPongStreamServerHandler : public pingpong::PingPongStreamServerHandler {
public:
    // Handle client->server RPC calls within the stream
    std::pair<pingpong::ServerPongResponse, scg::error::Error>
    sendPingFromClient(const scg::context::Context& ctx, const pingpong::ClientPingMessage& req) override {
        printf("Received ping from client, count: %d\n", req.count);

        pingpong::ServerPongResponse response;
        response.count = req.count + 1;
        return std::make_pair(response, nullptr);
    }
};

// Server implementation
class PingPongServerImpl : public pingpong::PingPongServer {
public:

    // Handle stream open requests
    std::pair<pingpong::PingPongStream*, scg::error::Error> openPingPongStream(
        const scg::context::Context& ctx,
        const pingpong::OpenPingPongStream& req) override
    {
        auto stream = pingpong::registerPingPongStream(std::make_shared<PingPongStreamServerHandler>())
        streams_.add(s);
        return std::make_pair(stream, nullptr);
    }


private:
    StreamContainer streams_; // Your global stream storage
};

int main() {
    signal(SIGINT, signalHandler);
    signal(SIGTERM, signalHandler);

    // Configure transport
    scg::tcp::ServerTransportConfig transportConfig;
    transportConfig.port = 9001;

    // Configure server
    scg::rpc::ServerConfig config;
    config.transport = std::make_shared<scg::tcp::ServerTransportTCP>(transportConfig);
    config.errorHandler = [](const scg::error::Error& err) {
        printf("Server error: %s\n", err.message.c_str());
    };

    // Create server
    auto server = std::make_shared<scg::rpc::Server>(config);

    // Create stream handler factory
    auto streamHandlerFactory = []() -> pingpong::PingPongStreamServerHandler* {
        return new PingPongStreamServerHandler();
    };

    // Create and register service implementation
    auto impl = std::make_shared<PingPongServerImpl>();
    pingpong::registerPingPongServer(server.get(), impl.get(), streamHandlerFactory);

    // Start server
    auto err = server->start();
    if (err) {
        printf("Failed to start server: %s\n", err.message.c_str());
        return 1;
    }

    printf("Server started on port 9001\n");

    int frameCount = 0;

    // Main game loop (poll-based, 60 FPS)
    while (running) {
        // Process any pending messages/connections AND active streams
        // This will:
        // 1. Accept new connections
        // 2. Process incoming RPC requests (including OpenPingPongStream)
        // 3. Process incoming stream messages
        // 4. Call update callbacks for active streams
        server->process();

        frameCount++;

        // Frame timing
        std::this_thread::sleep_for(std::chrono::milliseconds(16));
    }

    // Stop server (closes all streams)
    server->stop();

    return 0;
}
```

### Key Design Points

1. **Client-initiated streams only** - Server cannot initiate streams
2. **Handler-based message processing** - Both client and server provide handler implementations
3. **Go: Async handlers** - Client handlers run in goroutines automatically
4. **C++: Poll-based** - `client->process()` / `server->process()` dispatch stream messages
5. **Server lifecycle** - User returns stream from `OpenPingPongStream`, can spawn goroutines for background work
6. **Client receives stream** - `OpenPingPongStream` returns the stream object immediately

## Phase 1: Parser & AST Updates ✅ COMPLETE

### Completed:

1. ✅ Added `StreamTokenType` and `StreamMethodTokenType` to token.go
2. ✅ Created `stream_parser.go` with:
    - `StreamDefinition` struct
    - `StreamMethodDefinition` struct
    - `StreamMethodDirection` enum (Client/Server)
    - `parseStreamDefinitions()` function
    - `parseStreamMethodDefinition()` function
3. ✅ Updated `service_parser.go` to support `returns (stream StreamName)`:
    - Added `ReturnsStream` bool field
    - Added `StreamName` string field
    - Updated regex to capture `stream` keyword
4. ✅ Updated `parser.go` to recognize `stream` keyword
5. ✅ Updated `file_parser.go` to parse and store StreamDefinitions
6. ✅ Updated `processor.go` Package struct:
    - Added `StreamDefinitions` map
    - Added `HashStringToStreamID()` method
    - Added `HashStringToStreamMethodID()` method
7. ✅ Created comprehensive tests in `stream_parser_test.go`
8. ✅ All tests passing

## Phase 2: Protocol Constants ✅ COMPLETE

### Completed:

1. ✅ Updated `pkg/rpc/const.go` - Added new prefixes:
    - `StreamOpenPrefix`
    - `StreamMessagePrefix`
    - `StreamResponsePrefix`
    - `StreamClosePrefix`
2. ⬜ Add new response types for streams (deferred - existing types sufficient)
3. ⬜ Update C++ `include/scg/const.h` with same constants (Phase 5)

## Phase 3: Core Stream Infrastructure (Go) ✅ COMPLETE

### Completed:

1. ✅ Created `pkg/rpc/stream.go`:
    - `Stream` struct (base stream functionality)
    - `StreamID` management
    - Message routing
    - Lifecycle management (`Wait()`, `Close()`)
    - Request/response tracking per stream
    - Context management
2. ✅ Updated `pkg/rpc/client.go`:
    - Added `streams` map tracking active streams
    - Added `streamID` field for unique stream identification
    - Added stream message handling in receive loop (RPC, StreamResponse, StreamClose)
    - Added `handleRPCResponse()` for regular RPC responses
    - Added `handleStreamResponse()` for stream message responses
    - Added `handleStreamClose()` for stream close messages
    - Added `OpenStream()` helper method
3. ✅ Updated `pkg/rpc/server.go`:
    - Added `streamHandlers` map for stream handler registration
    - Added `StreamHandler` type definition
    - Added `RegisterStream()` method
    - Per-connection stream tracking in `handleConnection()`
    - Message routing (RPC, StreamOpen, StreamMessage, StreamClose)
    - Added `handleRPCRequest()` for regular RPC requests
    - Added `handleStreamOpen()` for creating stream and invoking handler
    - Added `handleStreamMessage()` for routing stream messages
    - Added `handleStreamClose()` for handling stream close
    - Automatic cleanup on connection close

**Build Status:** ✅ All code compiles successfully

## Phase 4: Go Code Generation ✅ COMPLETE

### Completed:

1. ✅ Created `internal/gen/go_gen/stream_client_generator.go`:

    - Generates stream client wrapper structs
    - Client-side methods for sending messages to server
    - Server-side method stubs for receiving messages from server
    - Message routing in `processServerMessage()`
    - `Close()` and `Wait()` lifecycle methods

2. ✅ Created `internal/gen/go_gen/stream_server_generator.go`:

    - Generates stream server wrapper structs and interfaces
    - Server-side methods for sending messages to client
    - Client-side method stubs for receiving messages from client
    - Message routing in `processClientMessage()`
    - Stream handler interface for user implementation
    - `Close()` and `Wait()` lifecycle methods

3. ✅ Updated `internal/gen/go_gen/client_generator.go`:

    - Detects when service methods return streams
    - Generates `OpenStream()` call for stream methods
    - Returns stream client wrapper struct
    - Regular RPC methods unchanged

4. ✅ Updated `internal/gen/go_gen/server_generator.go`:

    - Detects when service methods return streams
    - Server interface includes stream handler parameter
    - Wraps stream with generated server struct
    - Regular RPC methods unchanged

5. ✅ Updated `internal/gen/go_gen/file_generator.go`:

    - Added `StreamServers` and `StreamClients` sections
    - Integrated stream generation into file output

6. ✅ Updated `internal/parse/file_parser.go`:
    - Added `StreamsSortedByKey()` helper method

**Build Status:** ✅ All code compiles successfully

**Next Step:** Test code generation with an example `.scg` file containing stream definitions ✅ COMPLETE

### Test Results:

-   ✅ Created test file: `test/files/input/streaming/streaming.scg`
-   ✅ Generated Go code compiles without errors
-   ✅ Stream client and server stubs generated correctly
-   ✅ Bidirectional methods properly separated (client vs server)
-   ✅ Service methods returning streams work correctly

**Generated code includes:**

-   Message definitions (ChatMessage, ChatResponse, ServerNotification, Empty)
-   Stream client wrapper (`chatStreamStreamClient`) with:
    -   `SendMessage()` - client sends to server
    -   `handleSendNotification()` - receives from server
    -   `processServerMessage()` - routes server messages
    -   `Close()` and `Wait()` lifecycle methods
-   Stream server wrapper (`chatStreamStreamServer`) with:
    -   `SendNotification()` - server sends to client
    -   `handleSendMessage()` - receives from client
    -   `processClientMessage()` - routes client messages
    -   Stream handler interface for user implementation
    -   `Close()` and `Wait()` lifecycle methods
-   Service client (`ChatServiceClient`) with:
    -   `OpenChat()` returns stream client wrapper
-   Service server interface (`ChatServiceServer`) with:
    -   `OpenChat()` receives request and stream server wrapper

## Phase 5: Core Stream Infrastructure (C++) ✅ COMPLETE

### Completed:

1. ✅ Updated `include/scg/const.h`:

    - Added `STREAM_OPEN_PREFIX`
    - Added `STREAM_MESSAGE_PREFIX`
    - Added `STREAM_RESPONSE_PREFIX`
    - Added `STREAM_CLOSE_PREFIX`

2. ✅ Created `include/scg/stream.h`:

    - `Stream` class with full lifecycle management
    - `sendMessage()` template method for type-safe messaging
    - `handleMessage()` for routing incoming stream messages
    - `handleClose()` for remote stream closure
    - `close()` for local stream closure
    - `wait()` returns future for async waiting
    - Request/response tracking per stream
    - Thread-safe with mutex protection

3. ✅ Updated `include/scg/client.h`:
    - Added `streams_` map to track active streams
    - Added `streamID_` field for unique stream identification
    - Updated `onMessage()` to route different message types
    - Added `handleRPCResponse()` for regular RPC responses
    - Added `handleStreamResponse()` for stream message responses
    - Added `handleStreamClose()` for stream close messages
    - Added `closeAllStreamsUnsafe()` for cleanup
    - Added `openStream()` template method for initiating streams

**Build Status:** ✅ All C++ tests compile successfully (100% pass)

**Architecture Notes:**

-   C++ uses same async pattern as RPC calls (futures/promises)
-   Thread-safe with mutex protection on all shared state
-   Automatic cleanup on disconnect
-   Compatible with existing transport layer
-   No breaking changes to existing client API

## Phase 6: C++ Code Generation ✅ COMPLETE

### Completed:

1. ✅ Created `internal/gen/cpp_gen/stream_client_generator.go`
    - Generates `ChatStreamStreamClient` class
    - Client methods for sending to server
    - Virtual server methods for receiving from server
2. ✅ Created `internal/gen/cpp_gen/stream_server_generator.go`
    - Generates `ChatStreamStreamHandler` interface
    - Generates `ChatStreamStreamServer` class
    - Server methods for sending to client
3. ✅ Updated `internal/gen/cpp_gen/client_generator.go`
    - Added `ReturnsStream` and `StreamClientClassName` fields
    - Template handles stream-returning methods
    - Calls `client_->openStream<>()` for streams
4. ✅ Updated `internal/gen/cpp_gen/server_generator.go`
    - Skips methods that return streams (placeholder for Phase 7)
5. ✅ Updated `internal/gen/cpp_gen/file_generator.go`
    - Added `StreamClients` and `StreamServers` to file template
    - Integrated stream generation into file output
6. ✅ Generated C++ code compiles successfully
7. ✅ All existing C++ tests pass (100%)

## Phase 7: Testing & Validation ✅ COMPLETE

### Completed:

1. ✅ Fixed stream type export issue
    - Changed `getStreamClientStructName` from camelCase to PascalCase
    - Changed `getStreamServerStructName` from camelCase to PascalCase
    - Types now properly exported: `ChatStreamStreamClient`, `ChatStreamStreamServer`
2. ✅ Created comprehensive Go tests (`test/test_go/streaming_test.go`)
    - `TestStreamingCodeGeneration`: Basic struct creation and serialization
    - `TestStreamingMessageSerialization`: All message types (ChatMessage, ChatResponse, ServerNotification, Empty)
    - `TestStreamingTypesExist`: Verifies all stream types are exported
    - All tests passing ✅
3. ✅ Verified parser tests
    - `TestStreamParser`: Stream definition parsing
    - `TestServiceWithStreamReturn`: Service method returning stream
    - All parser tests passing ✅
4. ✅ Generated test file: `test/files/output/streaming/streaming.go` (440 lines)
    - Contains all message structs
    - Contains `ChatStreamStreamClient` and `ChatStreamStreamServer`
    - Contains `ChatStreamStreamHandler` and `ChatStreamStreamClientHandler` interfaces
    - Contains `ChatServiceClient` with `OpenChat()` method
    - Contains `ChatServiceServer` interface with stream-returning method
5. ✅ Generated test file: `test/files/output/streaming/streaming.h` (581 lines)
    - Contains all message structs with serialization
    - Contains `ChatStreamStreamClient` class
    - Contains `ChatStreamStreamHandler` interface and `ChatStreamStreamServer` class
    - Contains `ChatServiceClient` with `openChat()` method
6. ✅ Verified C++ compilation
    - `streaming.h` compiles without errors
    - All existing C++ tests pass
7. ✅ **Updated to handler-based design**
    - Server methods return `(*ChatStreamStreamServer, error)` instead of taking stream parameter
    - Client methods accept `ChatStreamStreamHandler` parameter for server→client messages
    - Stream server accepts `ChatStreamStreamClientHandler` for client→server messages
    - Stream client accepts `ChatStreamStreamServerHandler` for server→client messages
    - `ProcessMessage` implemented on both client and server to delegate to handlers
    - `setStream()` method on server stream to set internal stream after creation
8. ✅ **Protocol handling updated**
    - Split `handleStreamMessage` into unsolicited messages (with methodID) and responses (with requestID)
    - `StreamMessagePrefix` routes to `HandleIncomingMessage(methodID, reader)`
    - `StreamResponsePrefix` routes to `HandleMessageResponse(requestID, reader)`
    - Server and client both handle bidirectional message flow correctly
9. ✅ **Test server implementation**
    - `test/streaming_server_tcp/main.go` implements new handler-based design
    - Server returns stream from `OpenChat()`
    - Server spawns goroutine for lifecycle management
    - Stream handler processes client messages
    - All code compiles successfully

### Known Status:

1. ✅ **Server-side stream opening logic COMPLETE**

    - Server receives stream open request
    - Creates `Stream` object on server side
    - Calls user implementation with empty request
    - User returns stream server wrapper
    - Generated code sets internal stream and registers processor
    - Stream ready for bidirectional communication

2. ⚠️ **Integration testing needed**
    - Full end-to-end streaming tests not yet run
    - Need to test: client opens stream → server receives → bidirectional messages → close
    - Manual test with `streaming_server_tcp` possible

## Implementation Notes

### Stream Protocol Design:

```
Opening a stream:
  Client → Server: [RequestPrefix][Context][RequestID][ServiceID][MethodID][OpenMessage]
  Server → Client: [ResponsePrefix][RequestID][StreamID][MessageResponse]

Stream messages:
  Either → Other: [StreamMessagePrefix][StreamID][MethodID][Message]
  Other → Either: [StreamResponsePrefix][StreamID][MessageResponse/Error]

Closing stream:
  Either → Other: [StreamClosePrefix][StreamID]
```

### Key Design Decisions:

-   Go: Goroutine-based lifecycle with defer cleanup
-   C++: Poll-based, application-managed lifecycle
-   C++: Automatic message routing during process()
-   Both: Thread-safe stream method calls
-   Both: Same API surface (`NewStream`, `send*`, `close`, `wait`)

## Implementation Status Summary

### ✅ Fully Completed Phases:

1. **Phase 1: Parser & AST Updates** - Stream definitions, method directions, service integration
2. **Phase 2: Protocol Constants** - Stream message prefixes for Go and C++
3. **Phase 3: Core Stream Infrastructure (Go)** - Stream class, client/server integration, message routing
4. **Phase 4: Go Code Generation** - Client/server wrappers, handler interfaces, bidirectional messaging
5. **Phase 5: Core Stream Infrastructure (C++)** - Stream class, client integration (server TBD)
6. **Phase 6: C++ Code Generation** - Client wrappers, handler interfaces
7. **Phase 7: Testing & Validation** - Code generation tests, handler-based design implementation

### 🔄 In Progress:

-   **Integration Testing** - Need end-to-end tests with running client/server
-   **C++ Server Support** - Server-side stream handling not yet implemented in C++

### ⬜ Future Work:

1. **Full Integration Tests**

    - Create automated test that starts server, connects client, exchanges messages
    - Test stream lifecycle (open, bidirectional messages, close)
    - Test error conditions (network failure, invalid messages, etc.)

2. **C++ Server Implementation**

    - Add stream management to C++ Server class
    - Generate C++ server-side stream handler code
    - Support bidirectional streaming from C++ server

3. **Advanced Features**
    - Stream multiplexing optimizations

### Current Capabilities:

✅ **Working:**

-   Parse stream definitions from `.scg` files
-   Generate Go client and server code with handler-based design
-   Generate C++ client code with handler-based design
-   Open streams from Go client to Go server
-   Send bidirectional messages over streams
-   Process incoming stream messages via handlers
-   Close streams gracefully
-   Compile and run streaming server

✅ **Ready to Test:**

-   Go client → Go server streaming
-   Full bidirectional message flow
-   Stream lifecycle management

⚠️ **Not Yet Implemented:**

-   C++ server-side streaming
-   Comprehensive integration tests
-   Error recovery scenarios

## Latest Progress (December 21, 2025)

### ✅ Bidirectional Streaming COMPLETE

Full bidirectional streaming implementation is now working end-to-end! Integration test passing with all features functional.

### Final Implementation Details:

1. ✅ **Stream Handler Lifecycle**

    - Problem: Generated stream handler returned immediately, causing stream cleanup before use
    - Fix: Added `<-stream.Wait()` to block until stream closes

2. ✅ **Race Condition in Stream Registration**

    - Problem: Server handled all messages in parallel goroutines, causing race between StreamOpenPrefix registration and StreamMessagePrefix lookup
    - Fix: Handle StreamOpenPrefix synchronously to ensure stream is registered before subsequent messages

3. ✅ **Context Deserialization Missing**

    - Problem: Server `handleStreamMessage` wasn't deserializing context, read context bytes as stream ID
    - Symptom: "unrecognized stream id: 0" errors
    - Fix: Added `DeserializeContext(&ctx, reader)` before deserializing stream ID

4. ✅ **Request ID Deserialization Missing**

    - Problem: Server/client weren't deserializing request ID between stream ID and method ID
    - Symptom: "unrecognized stream method ID" with garbage values
    - Fix: Added request ID deserialization in correct position

5. ✅ **Stream Message Response Mechanism**

    - Problem: `HandleIncomingMessage` couldn't send responses back to sender
    - Symptom: Client `SendMessage` blocks indefinitely waiting for response
    - Fix: Changed signature to include `requestID uint64` parameter, implemented full StreamResponsePrefix serialization with ErrorResponse/MessageResponse types

6. ✅ **Debug Output Cleanup**
    - Removed all `fmt.Printf` debug statements from stream.go, server.go, and client.go
    - Removed unused `sync/atomic` import

### Test Results:

-   ✅ Parser tests: PASSING
-   ✅ Code generation tests: PASSING
-   ✅ Serialization tests: PASSING
-   ✅ **Integration test: PASSING** (2.51s)
    -   Client opens stream successfully
    -   Client sends 5 messages, receives 5 responses
    -   Server sends 3 unsolicited notifications, client receives all 3
    -   Stream lifecycle works correctly (open → bidirectional messages → close)

### Next Steps (Future Work):

1. ⬜ **C++ Server Support**

    - Port stream handling to C++ server implementation
    - Generate C++ server stream handlers
    - Create C++ integration tests

2. ⬜ **Additional Integration Tests**

    - Error handling scenarios
    - Stream closure edge cases
    - Network failure recovery
    - Extremely large message handling

3. ⬜ **Performance Testing**

    - Benchmark message throughput
    - Test with high concurrent stream counts
    - Memory usage profiling

4. ⬜ **Documentation**
    - User guide for streaming API
    - Example applications
    - Best practices guide

### Transport Support Status:

-   ✅ TCP (with and without TLS)
-   ✅ Unix Sockets
-   ✅ WebSocket (with and without TLS)
-   ✅ NATS (**Fixed!** - required special handling for request/reply architecture)

**NATS Streaming Implementation Details:**

-   NATS uses request/reply pattern (one request → one reply), but streaming needs persistent bidirectional communication
-   **Solution**: Track active connections by client's reply inbox
    -   Server maintains `map[string]*natsServerConnection` indexed by inbox subject
    -   First message from client creates connection, subsequent messages route to same connection via `responseCh`
    -   Added `serviceID` field to `Stream` struct to ensure messages sent to correct NATS subject
    -   Connection closes when stream closes, triggering cleanup of inbox mapping

All streaming tests are now integrated into the `service_test_suite.go` and run automatically for all transports.
**All 90 streaming integration tests passing** (15 tests × 6 transports).

```

```
