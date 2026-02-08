# 42. gRPC and Protocol Buffers

Funxy provides built-in support for gRPC clients and Protocol Buffers serialization via the `lib/grpc` and `lib/proto` packages. This support is **dynamic**, meaning you don't need to compile your `.proto` files into Funxy code. Instead, you load them at runtime, and Funxy uses reflection to convert between Funxy `Record`/`Map` types and Protobuf messages.

## Setup

You need:
1. A running gRPC server.
2. The `.proto` files that define the service and messages.

## Loading Protobuf Definitions

Before you can use gRPC or Protobuf encoding, you must load the definitions from your `.proto` files.

```funxy
import "lib/grpc" (grpcLoadProto)
import "lib/io" (fileWrite)

proto = `syntax = "proto3";
package example;
service Greeter { rpc SayHello (HelloRequest) returns (HelloReply) {} }
message HelloRequest { string name = 1; }
message HelloReply { string message = 1; }`

fileWrite("/tmp/service.proto", proto)

// Load a single file (imports in the same directory work automatically)
match grpcLoadProto("/tmp/service.proto") {
    Ok(_) -> print("Protos loaded")
    Fail(e) -> print("Failed to load protos: " ++ e)
}
```

## gRPC Client

The `lib/grpc` package allows you to connect to a gRPC server and invoke methods.

### Connecting

```funxy
import "lib/grpc" (grpcConnect, grpcClose)

match grpcConnect("localhost:50051") {
    Ok(conn) -> {
        // Use connection...
        grpcClose(conn)
    }
    Fail(e) -> print("Connection failed: " ++ e)
}
```

### Invoking Methods

To invoke a method, you need:
1. The **Full Method Name**: usually `package.Service/Method`.
2. The **Request Data**: a `Record` or `Map` matching the request message structure.

The result is a `Result<String, Response>`, where `Response` is a `Map` representing the response message.

```funxy
import "lib/grpc" (grpcConnect, grpcInvoke, grpcClose)
import "lib/map" (mapGet)

// Assume service.proto defines:
// service Greeter { rpc SayHello (HelloRequest) returns (HelloReply) {} }
// message HelloRequest { string name = 1; }
// message HelloReply { string message = 1; }

match grpcConnect("localhost:50051") {
    Ok(conn) -> {
        request = { name: "Funxy" }

        match grpcInvoke(conn, "example.Greeter/SayHello", request) {
            Ok(response) -> {
                // response is a Map, e.g. %{ "message" => "Hello Funxy" }
                match mapGet(response, "message") {
                    Some(msg) -> print("Server replied: " ++ msg)
                    None -> print("No message received")
                }
            }
            Fail(err) -> print("RPC Error: " ++ err)
        }

        grpcClose(conn)
    }
    Fail(e) -> print("Connection failed: " ++ e)
}
```

## gRPC Server

Funxy can also act as a gRPC server. You can dynamically register handlers for services defined in your loaded `.proto` files.

### 1. Define Handlers

Handlers are defined as a `Record` or `Map` where keys match the RPC method names, and values are functions. The function receives the request (as a Map/Record) and must return the response (as a Map/Record).

```funxy
handler = {
    SayHello: fun(req) {
        print("Received request from: " ++ req.name)
        { message: "Hello " ++ req.name }
    }
}
```

### 2. Create and Register Server

```funxy
import "lib/grpc" (grpcServer, grpcRegister, grpcLoadProto)
import "lib/io" (fileWrite)

proto = `syntax = "proto3";
package example;
service Greeter { rpc SayHello (HelloRequest) returns (HelloReply) {} }
message HelloRequest { string name = 1; }
message HelloReply { string message = 1; }`

fileWrite("/tmp/service.proto", proto)
grpcLoadProto("/tmp/service.proto")

handler = {
    SayHello: fun(req) { { message: "Hello " ++ req.name } }
}

// Create server instance
server = grpcServer()

// Register the handler for the service "example.Greeter" (must match package.Service in proto)
match grpcRegister(server, "example.Greeter", handler) {
    Ok(_) -> print("Service registered")
    Fail(e) -> print("Registration failed: " ++ e)
}
```

### 3. Start Serving

You can start the server in blocking mode (`grpcServe`) or async mode (`grpcServeAsync`).

```funxy
import "lib/grpc" (grpcServer, grpcRegister, grpcServeAsync, grpcStop, grpcLoadProto)
import "lib/io" (fileWrite)
import "lib/time" (sleepMs)

proto = `syntax = "proto3";
package example;
service Greeter { rpc SayHello (HelloRequest) returns (HelloReply) {} }
message HelloRequest { string name = 1; }
message HelloReply { string message = 1; }`

fileWrite("/tmp/service.proto", proto)
grpcLoadProto("/tmp/service.proto")

handler = { SayHello: fun(req) { { message: "Hello " ++ req.name } } }
server = grpcServer()
grpcRegister(server, "example.Greeter", handler)

// Start and stop quickly so the example completes
grpcServeAsync(server, ":50051")
sleepMs(50)
grpcStop(server)
```

### Async Serving

For async execution (e.g. to run client and server in same script):

```funxy
import "lib/grpc" (grpcServer, grpcRegister, grpcServeAsync, grpcStop, grpcLoadProto)
import "lib/io" (fileWrite)
import "lib/time" (sleepMs)

proto = `syntax = "proto3";
package example;
service Greeter { rpc SayHello (HelloRequest) returns (HelloReply) {} }
message HelloRequest { string name = 1; }
message HelloReply { string message = 1; }`

fileWrite("/tmp/service.proto", proto)
grpcLoadProto("/tmp/service.proto")

handler = { SayHello: fun(req) { { message: "Hello " ++ req.name } } }
server = grpcServer()
grpcRegister(server, "example.Greeter", handler)

grpcServeAsync(server, ":50051")
sleepMs(100) // Give it a moment to start

// ... client code ...

grpcStop(server)
```

## Protocol Buffers Serialization

If you only need to encode/decode Protobuf messages (e.g. for saving to files or sending over other protocols), use `lib/proto`.

```funxy
import "lib/proto" (protoEncode, protoDecode)
import "lib/grpc" (grpcLoadProto) // Reuses the loader
import "lib/io" (fileWrite)
import "lib/map" (mapGet)

proto = `syntax = "proto3";
package myapp;
message User {
  int64 id = 1;
  string name = 2;
  repeated string roles = 3;
}`

fileWrite("/tmp/data.proto", proto)
grpcLoadProto("/tmp/data.proto")

// Data to encode
user = {
    id: 123,
    name: "Alice",
    roles: ["admin", "editor"] // Repeated fields map to Lists
}

// Encode to Bytes
match protoEncode("myapp.User", user) {
    Ok(bytes) -> {
        print("Encoded " ++ show(len(bytes)) ++ " bytes")

        // Decode back
        match protoDecode("myapp.User", bytes) {
            Ok(decodedUser) -> {
                // decodedUser is a Map
                match mapGet(decodedUser, "name") {
                    Some(name) -> print("Decoded: " ++ name)
                    None -> print("Name field missing")
                }
            }
            Fail(e) -> print("Decode error: " ++ e)
        }
    }
    Fail(e) -> print("Encode error: " ++ e)
}
```

## Type Mapping

Funxy maps Protobuf types to native types as follows:

| Protobuf Type | Funxy Type | Notes |
|---------------|------------|-------|
| `double`, `float` | `Float` | |
| `int32`, `int64`, etc. | `Int` | |
| `bool` | `Bool` | |
| `string` | `String` | |
| `bytes` | `Bytes` | |
| `repeated` | `List<T>` | |
| `map<K, V>` | `Map<K, V>` | |
| `message` | `Map<String, T>` or `Record` | Input can be Record, Output is always Map |
| `oneof` | `Map` entry | Only the set field is present |
| `enum` | `Int` | |

### Default Values

- **Encoding**: Missing fields in the input Record/Map are treated as default values (0, "", nil).
- **Decoding**: Proto3 default values are generally not emitted in the output Map, following Proto3 JSON mapping rules, but `lib/proto` attempts to provide a faithful representation.

## Advanced: Saving to File

You can combine `lib/proto` with `lib/io` to save binary data.

```funxy
import "lib/proto" (protoEncode)
import "lib/grpc" (grpcLoadProto)
import "lib/io" (fileWrite)
import "lib/bytes" (bytesToHex)

proto = `syntax = "proto3";
package myapp;
message User {
  int64 id = 1;
  string name = 2;
}`

fileWrite("/tmp/data.proto", proto)
grpcLoadProto("/tmp/data.proto")

data = { id: 1, name: "Bob" }
bytes = protoEncode("myapp.User", data)?

// Save bytes as hex string
fileWrite("data.hex", bytesToHex(bytes))
```
