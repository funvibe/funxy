# YAML

The `lib/yaml` module provides functions for encoding, decoding, reading, and writing YAML.

## Import

```rust
import "lib/yaml" (*)
```

## Decoding YAML

`yamlDecode` parses a YAML string into Funxy values. Maps become records, sequences become lists, scalars become their natural types (`Int`, `Float`, `Bool`, `String`, `Nil`):

```rust
import "lib/yaml" (yamlDecode)

yaml = "name: Alice
age: 30
tags:
  - go
  - funxy"

match yamlDecode(yaml) {
    Ok(data) -> {
        print(data.name)      // Alice
        print(data.age)       // 30
        print(data.tags[0])   // go
    }
    Fail(e) -> print("Error: " ++ e)
}
```

### Nested structures

```rust
import "lib/yaml" (yamlDecode)

yaml = "database:
  host: db.example.com
  port: 5432
  ssl: true
cache:
  ttl: 300"

match yamlDecode(yaml) {
    Ok(cfg) -> {
        print(cfg.database.host)  // db.example.com
        print(cfg.database.port)  // 5432
        print(cfg.cache.ttl)      // 300
    }
    Fail(e) -> print("Error: " ++ e)
}
```

### Lists

```rust
import "lib/yaml" (yamlDecode)

yaml = "- 1
- 2
- 3"

match yamlDecode(yaml) {
    Ok(items) -> print(show(items))  // [1, 2, 3]
    Fail(e) -> print("Error: " ++ e)
}
```

## Encoding to YAML

`yamlEncode` converts any Funxy value to a YAML string:

```rust
import "lib/yaml" (yamlEncode)

config = {
    server: { host: "localhost", port: 8080 },
    debug: true,
    tags: ["web", "api"]
}

print(yamlEncode(config))
// debug: true
// server:
//     host: localhost
//     port: 8080
// tags:
//     - web
//     - api
```

Works with any value:

```rust
import "lib/yaml" (yamlEncode)

print(yamlEncode(42))         // 42
print(yamlEncode("hello"))    // hello
print(yamlEncode([1, 2, 3]))  // - 1\n- 2\n- 3
```

## Reading from File

```rust
import "lib/yaml" (yamlRead)

match yamlRead("config.yaml") {
    Ok(cfg) -> {
        print(cfg.database.host)
        print(cfg.server.port)
    }
    Fail(e) -> print("Cannot read file: " ++ e)
}
```

## Writing to File

```rust
import "lib/yaml" (yamlWrite)

config = {
    database: { host: "db.example.com", port: 5432 },
    cache: { ttl: 300 }
}

match yamlWrite("config.yaml", config) {
    Ok(_) -> print("Saved!")
    Fail(e) -> print("Error: " ++ e)
}
```

## Practical example: config file

```rust
import "lib/yaml" (yamlRead)
import "lib/http" (httpServe)

match yamlRead("app.yaml") {
    Ok(cfg) -> {
        port = cfg.server.port
        print("Starting on port " ++ show(port))
        httpServe(port, fun(req) { { status: 200, body: "OK" } })
    }
    Fail(e) -> panic("Bad config: " ++ e)
}
```

## Function Reference

| Function | Signature | Description |
|---------|-----------|-------------|
| `yamlDecode(content)` | `(String) -> Result<String, T>` | Parse YAML string into Funxy values |
| `yamlEncode(value)` | `(a) -> String` | Encode any value to YAML string |
| `yamlRead(path)` | `(String) -> Result<String, T>` | Read and parse a YAML file |
| `yamlWrite(path, value)` | `(String, a) -> Result<String, Nil>` | Write a value to a YAML file |

### Type mapping

| YAML | Funxy |
|------|-------|
| Mapping (`key: value`) | Record `{ key: value }` |
| Sequence (`- item`) | List `[item]` |
| Integer (`42`) | Int |
| Float (`3.14`) | Float |
| Boolean (`true`/`false`) | Bool |
| String (`"hello"`) | String |
| Null (`null`, `~`) | Nil |
