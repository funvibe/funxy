# Funxy

A statically typed scripting language that compiles to native binaries. For automation, services, and data tooling.

- Write scripts, ship native binaries — `funxy build` creates standalone executables with embedded resources
- Static types with strong inference — most code needs no annotations
- Batteries-included stdlib: HTTP/gRPC, JSON/protobuf, SQL, TUI, async/await, bytes/bits
- Use Go packages from scripts — declare in `funxy.yaml`, import as `ext/*`
- Command-line eval mode (`-pe`, `-lpe`) for one-liners and shell pipelines
- Safe data modeling with records, unions, ADTs, and pattern matching
- Easy embedding in Go for config, rules, and automation

```bash
funxy build server.lang -o myserver && scp myserver user@prod:~/
```

```bash
echo '{"name":"Alice"}' | funxy -pe '|>> jsonDecode |> \x -> x.name'   # Alice
```

```rust
import "lib/csv"  (csvEncode)
import "lib/io"   (fileRead, fileWrite)
import "lib/json" (jsonDecode)

users = "users.json" |>> fileRead |>> jsonDecode
fileWrite("users.csv", csvEncode(users))
```

## Install

```bash
curl -sSL https://raw.githubusercontent.com/funvibe/funxy/main/install.sh | bash
```

Or download manually from [releases](https://github.com/funvibe/funxy/releases), or build from source: `git clone ... && cd funxy && make build` (Go 1.25+).

## Build & Distribution

Compile scripts into self-contained native binaries. All dependencies, traits, and optional static resources are bundled inside.

```bash
funxy build server.lang -o myserver                       # standalone binary
funxy build app.lang --embed templates,static -o app      # with embedded resources
funxy build app.lang --host bin/funxy-linux-amd64 -o app  # cross-compile
```

Bundle multiple scripts into one binary (BusyBox-style). Each becomes a subcommand:

```bash
funxy build api.lang worker.lang cron.lang -o myserver

./myserver api --port 8080    # runs api.lang
./myserver worker             # runs worker.lang
./myserver                    # prints available commands
```

Symlink dispatch — create links, each acts as a standalone tool:

```bash
ln -s myserver api && ./api --port 8080   # dispatched by argv[0]
```

Built binaries are also full Funxy interpreters — pass `$` to switch:

```bash
./myserver                    # runs embedded app (or shows commands)
./myserver --port 8080        # flags go to your app via sysArgs
./myserver $ script.lang      # interpreter mode
./myserver $ -pe '1 + 2'     # eval mode
```

## Go Ecosystem Access

Use any Go package directly from Funxy. Declare dependencies in `funxy.yaml` — functions and type methods are auto-generated:

```yaml
# funxy.yaml
deps:
  - pkg: github.com/slack-go/slack
    version: v0.15.0
    bind:
      - func: New
        as: slackNew
      - type: Client
        as: slack
        methods: [PostMessage, GetUserInfo]
        error_to_result: true
```

```rust
import "ext/slack" (slackNew, slackPostMessage)

client = slackNew("xoxb-your-token")

match slackPostMessage(client, "#general", "Deploy complete!") {
  Ok(_)   -> print("Sent!")
  Fail(e) -> print("Error: " ++ show(e))
}
```

```bash
funxy build notify.lang -o notify    # auto-detects funxy.yaml, builds with Go bindings
```

Features: `error_to_result` maps Go `(T, error)` to `Result<String, T>`, `skip_context` auto-injects `context.Background()`, type bindings expose struct methods as functions. See [docs/tutorial/44_go_extensions.md](docs/tutorial/44_go_extensions.md).

## One-Liners

`-e` evaluate, `-p` auto-print, `-l` line-by-line. Piped input available as `stdin`. Stdlib functions are auto-imported.

```bash
funxy -pe '1 + 2 * 3'                                             # 7
echo '{"name":"Alice","age":30}' | funxy -pe 'stdin |>> jsonDecode'   # full object
cat data.txt | funxy -lpe 'stringToUpper(stdin)'                   # per line
curl -s api.com/users | funxy -pe 'stdin |>> jsonDecode |> filter(\x -> x.active) |> map(\x -> x.name)'
```

## Language

Multi-paradigm: imperative loops and mutable variables work alongside pattern matching, pipes, and ADTs. Write in the style that fits the task.

```rust
// Imperative
results = []
for user in users {
    if user.active {
        results = results ++ [user.name]
    }
}

// Functional
results = users |> filter(\u -> u.active) |> map(\u -> u.name)
```

### Types and Inference

Most code needs no annotations. Add types when it matters.

```rust
numbers = [1, 2, 3]
doubled = map(\x -> x * 2.5, numbers)  // [2.5, 5, 7.5]

fun add(a: Int, b: Int) -> Int { a + b }
```

### Pattern Matching

```rust
match user {
    { name: "admin", role: r } -> print("Admin: ${r}")
    _ -> print("Guest")
}

// String patterns
match (method, path) {
    ("GET", "/users/{id}") -> getUser(id)
    ("GET", "/files/{...path}") -> serveFile(path)
    _ -> notFound()
}
```

### ADTs and Unions

```rust
type Shape = Circle Float | Rectangle Float Float

fun area(s: Shape) -> Float {
    match s {
        Circle r -> 3.14 * r * r
        Rectangle w h -> w * h
    }
}

x: Int | String = 42
x = "hello"  // OK

// Nullable shorthand
name: String? = "Alice"  // Equivalent to String | Nil
name = nil
```

### More

Ranges and comprehensions, pipes, error propagation, tail call optimization, argument shorthand, block syntax, cyclic module imports, debugger (`funxy -debug`)... See [Reference](REFERENCE.md).

## Standard Library

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
| `lib/io` | Files, directories, stdin |
| `lib/json` | jsonEncode, jsonDecode |
| `lib/list` | map, filter, foldl, sort, zip |
| `lib/log` | Structured logging |
| `lib/map` | Key-value operations |
| `lib/math` | Math functions |
| `lib/path` | File path manipulation |
| `lib/proto` | Protocol Buffers |
| `lib/rand` | Random number generation |
| `lib/regex` | Regular expressions |
| `lib/sql` | SQLite (built-in, no drivers needed) |
| `lib/string` | split, trim, replace, contains |
| `lib/sys` | Args, env, exec, exePath, scriptDir |
| `lib/task` | async/await |
| `lib/term` | Colors, prompts, spinners, progress bars, tables |
| `lib/test` | Unit testing |
| `lib/time` | Time and timing |
| `lib/tuple` | Tuple manipulation |
| `lib/url` | URL parsing and encoding |
| `lib/uuid` | UUID generation |
| `lib/ws` | WebSocket client and server |

Run `funxy -help lib/<name>` for documentation.

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

print $ qsort([3, 1, 4, 1, 5, 9, 2, 6]) // [1, 1, 2, 3, 4, 5, 6, 9]
```

## Documentation

- [Reference](REFERENCE.md)
- [Tutorial](docs/tutorial)
- [Go Extensions](docs/tutorial/44_go_extensions.md)
- [Playground](playground) — run code in a browser

## Editor Support

- [VS Code / Cursor](editors/vscode/) — syntax highlighting + LSP
- [Sublime Text](editors/sublime/)

## File Extensions

`.lang`, `.funxy`, `.fx`

## License

[LICENSE.md](LICENSE.md)
