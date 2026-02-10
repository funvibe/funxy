# Funxy

A pragmatic, statically typed scripting language for automation, services, and data tooling.

## Example

```rust
import "lib/csv" (csvEncode)
import "lib/json" (jsonDecode)
import "lib/io" (readFile, writeFile)
import "lib/list" (map)

users = jsonDecode(readFile("users.json"))
rows = map(\u -> [u.id, u.email, u.role], users)
writeFile("users.csv", csvEncode(rows))
```

## Why Funxy

- Script-first ergonomics with static types and strong inference
- Batteries-included stdlib: HTTP/gRPC, JSON/protobuf, SQL, files, async/await, bytes/bits
- Safe data modeling with records, unions, ADTs, and pattern matching
- Productive pipelines (pipes, comprehensions) and fast iteration for ops/ML workflows
- Easy embedding in Go for config, rules, and automation

## Installation

### Download Binary

Download from [Releases](https://github.com/funvibe/funxy/releases):
- macOS: `funxy-darwin-amd64` or `funxy-darwin-arm64`
- Linux: `funxy-linux-amd64` or `funxy-linux-arm64`
- Windows: `funxy-windows-amd64.exe`

Each release also includes a `-tree` variant (e.g., `funxy-darwin-arm64-tree`) which uses the legacy Tree-Walk interpreter. This is slower but provides more detailed stack traces for debugging compiler issues.

```bash
mv funxy-darwin-arm64 funxy
chmod +x funxy
./funxy hello.lang
```

If you downloaded the binary on macOS/Linux and see a permission error, mark it executable:

```bash
chmod +x funxy
```

### Build from Source

```bash
git clone https://github.com/funvibe/funxy
cd funxy
make build
./funxy hello.lang
```

**Running Tests:**

```bash
# Run all tests (VM backend)
go test -v ./tests/...

# Run all tests (Tree-Walk backend)
go test -v ./tests/... -tree
```

Requires Go 1.25+

## Quick Start

```bash
# Run a program
./funxy hello.lang

# Run from stdin
echo 'print("Hello!")' | ./funxy

# Web playground
./funxy playground/playground.lang
# Open http://localhost:8080

# Show help
./funxy -help
./funxy -help lib/http
```

Hello world in one line:

```rust
print("Hello, Funxy!")
```

Save as `hello.lang` and run: `./funxy hello.lang`.

## Key Features

### Scripting and Ops Utilities

```rust
import "lib/sys"
import "lib/http"
import "lib/json"

url = sys.args()[1]
status = json.decode(http.get(url).body).status
print(status)
```

### Static Typing with Inference

Most scripts need no type annotations, but you can add them when it matters.

```rust
import "lib/list" (map)

directive "strict_types" // Enforce stricter type checking

numbers = [1, 2, 3] // Type inferred: List<Int>

// Compact lambdas and implicit conversions (Int -> Float)
doubled = map(\x -> x * 2.0, numbers)

print(doubled) // [2.0, 4.0, 6.0]

// Explicit types when needed
fun add(a: Int, b: Int) -> Int { a + b }
```

Strict mode affects:
- Use of union values without narrowing (require `match`/`typeOf` before use)
- Operations that previously relied on implicit union handling (e.g. `Int | String`)

### Pattern Matching

```rust
fun describe(n) {
    match n {
        0 -> "zero"
        n if n < 0 -> "negative"
        _ -> "positive"
    }
}

// Record destructuring
user = { name: "admin", role: "superuser" }
match user {
    { name: "admin", role: r } -> print("Admin: ${r}")
    _ -> print("Guest")
}
```

### String Patterns for Routing

```rust
fun route(method, path) {
    match (method, path) {
        ("GET", "/users/{id}") -> "User: ${id}"
        ("GET", "/files/{...path}") -> "File: ${path}"
        _ -> "Not found"
    }
}

print(route("GET", "/users/42"))           // User: 42
print(route("GET", "/files/css/main.css")) // File: css/main.css
```

### Pipes

```rust
import "lib/list" (filter, map, foldl)

result = [1, 2, 3, 4, 5]
    |> filter(\x -> x % 2 == 0) // Lambda syntax
    |> map(\x -> x * x)
    |> foldl(\acc, x -> acc + x, 0)
    |> %"Result: %.2f" // Pipe formatting

// Result: 20.00
```

### Algebraic Data Types

```rust
type Shape = Circle Float | Rectangle Float Float

fun area(s: Shape) -> Float {
    match s {
        Circle r -> 3.14 * r * r
        Rectangle w h -> w * h
    }
}

print(area(Circle(5.0)))         // 78.5
print(area(Rectangle(3.0, 4.0))) // 12.0
```

### Argument Shorthand Sugar

Convenient syntax for record arguments in function calls:

```rust
type alias Config = { host: String, port: Int }

fun connect(config: Config) {
    print("Connecting to ${config.host}:${config.port}")
}

// Call with shorthand - no braces needed!
connect(host: "localhost", port: 8080)

// Equivalent to: connect({ host: "localhost", port: 8080 })
```

The record argument must be the last parameter, and you can mix regular arguments with named record fields.

```rust
fun createUser(name, options: { age: Int, active: Bool }) {
    // ...
}

// Usage
createUser("Alice", age: 25, active: true)
```

### Block Syntax (Trailing Block)

For cleaner DSL-style code, functions (lowercase identifiers) can be called with a block argument without parentheses:

```rust
import "kit/ui" (div, span, text, render)

// Clean syntax - no parentheses needed!
page = div {
    span { text("Header") }
    div {
        text("Content line 1")
        text("Content line 2")
    }
    span { text("Footer") }
}

html = render(page)
// <div><span>Header</span><div>Content line 1Content line 2</div><span>Footer</span></div>
```

### Do Notation

Monadic operations simplified:

```rust
fun maybeAdd(mx, my) {
    do {
        x <- mx
        y <- my
        pure(x + y)
    }
}
```

### Ranges and List Comprehensions

Expressive list generation:

```rust
chars = 'a'..'z'
squares = [x * x | x <- 1..10, x % 2 == 0]
```

### Tail Call Optimization

```rust
fun countdown(n, acc) {
    if n == 0 { acc }
    else { countdown(n - 1, acc + 1) }
}

print(countdown(1000000, 0))  // Works, no stack overflow
```

### Debugging

Funxy includes a built-in debugger.

```bash
./funxy -debug script.lang
```

## Cyclic Dependencies

Modules can import each other — the analyzer resolves cycles automatically:

```rust
// a/a.lang
package a (getB)
import "../b" as b
fun getB() -> b.BType { b.makeB() }

// b/b.lang
package b (BType, makeB)
import "../a" as a
type alias BType = { val: Int }
fun makeB() -> BType { { val: 1 } }
```

## Standard Library

Funxy ships with a batteries-included standard library for everyday scripting, backend utilities, and data workflows:

| Module | Description |
|--------|-------------|
| `lib/bignum` | BigInt, Rational |
| `lib/bits` | Bit-level parsing ([funbit](https://github.com/funvibe/funbit)) |
| `lib/bytes` | Byte manipulation |
| `lib/char` | Character functions |
| `lib/crypto` | sha256, md5, base64, hmac |
| `lib/csv` | CSV parsing and encoding |
| `lib/date` | Date and time |
| `lib/flag` | Command line flags |
| `lib/grpc` | gRPC client/server |
| `lib/http` | HTTP client and server |
| `lib/io` | Files and directories |
| `lib/json` | jsonEncode, jsonDecode |
| `lib/list` | map, filter, foldl, sort, zip, range, insert, update |
| `lib/log` | Structured logging |
| `lib/map` | Key-value operations |
| `lib/math` | Math functions |
| `lib/path` | File path manipulation |
| `lib/proto` | Protocol Buffers encoding/decoding |
| `lib/rand` | Random number generation |
| `lib/regex` | Regular expressions |
| `lib/sql` | SQLite (built-in, no drivers needed) |
| `lib/string` | split, trim, replace, contains |
| `lib/sys` | Args, env, exec |
| `lib/task` | async/await |
| `lib/test` | Unit testing |
| `lib/time` | Time and timing |
| `lib/tuple` | Tuple manipulation |
| `lib/url` | URL parsing and encoding |
| `lib/uuid` | UUID generation |
| `lib/ws` | WebSocket client and server |

Run `./funxy -help lib/<name>` for documentation.

## Documentation

- [reference](REFERENCE.md) — Reference Manual
- [tutorial](docs/tutorial) — Step-by-step tutorials
- [playground](playground) — Run code in a browser

## Editor Support

- [VS Code/Cursor extension](editors/vscode/)
- [Sublime Text syntax](editors/sublime/)

### LSP Installation

To use the new Language Server Protocol features:

1.  **Install the Language Server:**
    *   Download `funxy-lsp` from [Releases](https://github.com/funvibe/funxy/releases).
        *   macOS: `funxy-lsp-darwin-amd64` (rename to `funxy-lsp`)
        *   Linux: `funxy-lsp-linux-amd64` (rename to `funxy-lsp`)
        *   Windows: `funxy-lsp-windows-amd64.exe` (rename to `funxy-lsp.exe`)
    *   On macOS/Linux, you may need to make the binary executable:
        ```bash
        chmod +x funxy-lsp
        ```
    *   Place the binary in a directory included in your system's `PATH`.
    *   Install the updated extension from `editors/vscode`.
    *   It will automatically try to find `funxy-lsp` in your PATH.
    *   See `editors/vscode/README.md` for detailed configuration.

## File Extensions

Supported: `.lang`, `.funxy`, `.fx`

All files in a package must use the same extension (determined by the main file).

## Examples

### JSON API

```rust
import "lib/json" (jsonEncode)

users = [
    { id: 1, name: "Alice" },
    { id: 2, name: "Bob" }
]

fun handler(method, path) {
    match (method, path) {
        ("GET", "/api/users") -> {
            status: 200,
            body: jsonEncode(users)
        }
        ("GET", "/api/users/{id}") -> {
            status: 200,
            body: jsonEncode({ userId: id })
        }
        _ -> { status: 404, body: "Not found" }
    }
}
```

### QuickSort

```rust
import "lib/list" (filter)

fun qsort(xs) {
    match xs {
        [] -> []
        [pivot, ...rest] -> {
            less = filter(fun(x) { x < pivot }, rest)
            greater = filter(fun(x) { x >= pivot }, rest)
            qsort(less) ++ [pivot] ++ qsort(greater)
        }
    }
}

print(qsort([3, 1, 4, 1, 5, 9, 2, 6])) // [1, 1, 2, 3, 4, 5, 6, 9]
```

### Binary Parsing

```rust
import "lib/bits" (bitsExtract, bitsInt)

// TCP flags: 6 bits
packet = #b"010010"  // SYN + ACK

specs = [
    bitsInt("urg", 1), bitsInt("ack", 1),
    bitsInt("psh", 1), bitsInt("rst", 1),
    bitsInt("syn", 1), bitsInt("fin", 1)
]

match bitsExtract(packet, specs) {
    Ok(flags) -> print(flags) // %{"ack" => 1, "syn" => 1, ...}
    Fail(e) -> print(e)
}
```

## License

[LICENSE.md](LICENSE.md)
