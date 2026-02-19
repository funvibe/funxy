# Go Extensions (ext/*)

Funxy can use any Go package directly from scripts. You describe what you need in `funxy.yaml`, and `funxy build` handles the rest — introspection, code generation, compilation.

No Go code to write. No FFI. Just a config file and an import.

## Quick Start

### 1. Create `funxy.yaml`

```yaml
deps:
  - pkg: github.com/slack-go/slack
    version: v0.15.0
    bind:
      - func: New
        as: slackNew
      - func: MsgOptionText
        as: slackMsgOptionText
      - type: Client
        as: slack
        methods: [PostMessage, GetUserInfo]
        skip_context: true
        error_to_result: true
```

### 2. Write your script

```rust
// notify.lang
import "ext/slack" (slackNew, slackPostMessage, slackMsgOptionText)

client = slackNew("xoxb-your-token")

// Use MsgOptionText to create the message option
msg = slackMsgOptionText("Hello from Funxy!", false)

match slackPostMessage(client, "#general", msg) {
  Ok(result) -> print("Sent to: " ++ show(result))
  Fail(err)  -> print("Slack error: " ++ show(err))
}
```

### 3. Build and run

```bash
funxy build notify.lang -o notify
./notify
# Sent to: #general
```

`funxy build` detects `funxy.yaml` automatically. You can also use `--config path/to/funxy.yaml` explicitly.

## How It Works

When `funxy build` finds a `funxy.yaml`:

1. **Parse** — reads the config, validates bindings
2. **Inspect** — downloads Go packages, extracts type info via `go/packages`
3. **Generate** — creates Go wrapper code for each binding
4. **Compile** — builds a custom Funxy binary with the Go bindings compiled in
5. **Bundle** — appends your script's bytecode to this binary

The result is a single self-contained binary that includes both the Funxy runtime and the Go library code.

## Binding Functions

The simplest binding — expose a Go function as a Funxy function:

```yaml
deps:
  - pkg: github.com/slack-go/slack
    version: v0.15.0
    bind:
      - func: New          # Go function name
        as: slackNew       # Funxy function name
```

```rust
import "ext/slack" (slackNew)
client = slackNew("xoxb-your-token")
```

### Error Handling (`error_to_result`)

Go functions often return `(T, error)`. With `error_to_result: true`, this becomes `Result<String, T>` in Funxy:

```yaml
- type: Client
  as: slack
  methods: [GetUserInfo]
  error_to_result: true
```

```rust
import "ext/slack" (slackGetUserInfo)

match slackGetUserInfo(client, "U12345") {
  Ok(user) -> print("Got user: " ++ show(user))
  Fail(msg)  -> print("Error: " ++ show(msg))
}
```

Without `error_to_result`, Go errors cause a runtime panic. With it — they become values you can match on.

### Context Injection (`skip_context`)

Many Go APIs (gRPC clients, database drivers) require `context.Context` as the first argument. With `skip_context: true`, Funxy auto-injects `context.Background()`:

```yaml
deps:
  - pkg: github.com/redis/go-redis/v9
    version: v9.7.0
    bind:
      - type: Client
        as: redis
        methods: [Get]
        skip_context: true
        error_to_result: true
```

```rust
import "ext/redis" (redisGet)

// Instead of redisGet(ctx, client, "key"), just:
match redisGet(client, "key") {
  Ok(val) -> print("Value: " ++ val)
  Fail(err) -> print("Redis error: " ++ show(err))
}
```

## Binding Types (Struct Methods)

To call methods on Go structs, use `type` binding:

```yaml
deps:
  - pkg: github.com/redis/go-redis/v9
    version: v9.7.0
    bind:
      - func: ParseURL
        as: redisParseURL
        error_to_result: true
      - func: NewClient
        as: redisNew
      - type: Client
        as: redis
        methods: [Set, Get, Del, Close]
        skip_context: true
        error_to_result: true
```

This generates functions named `<prefix><Method>`:
- `Client.Set()` → `redisSet(client, key, val, ttl)`
- `Client.Get()` → `redisGet(client, key)`
- `Client.Close()` → `redisClose(client)`

The first argument is always the object itself.

### Initialization

Go constructors that take struct arguments (like `redis.NewClient(&redis.Options{...})`) require a `HostObject`. There are two ways to create Go structs in Funxy:

1. **`constructor: true`** — generate a constructor function that creates a struct from a Funxy record (see [Struct creation via `constructor`](#struct-creation-via-constructor) below).
2. **Use a Go function** that returns the struct — many libraries provide a string-based constructor or builder.

For Redis, `ParseURL` creates options from a connection string:

```rust
import "ext/redis" (redisParseURL, redisNew, redisSet, redisGet, redisClose)

// Create client from a connection URL
client = match redisParseURL("redis://localhost:6379/0") {
  Ok(opts) -> redisNew(opts)
  Fail(e)  -> { print("Bad Redis URL: " ++ show(e)); exit(1) }
}

match redisSet(client, "mykey", "hello", 0) {
  Ok(_)   -> print("Set OK")
  Fail(e) -> print("Error: " ++ show(e))
}

match redisGet(client, "mykey") {
  Ok(val) -> print("Got: " ++ show(val))
  Fail(e) -> print("Error: " ++ show(e))
}

redisClose(client)
```

### Controlling Methods

```yaml
# Only these methods:
- type: Client
  as: redis
  methods: [Set, Get, Del, Close]

# All methods except these:
- type: Client
  as: redis
  exclude_methods: [Options, PoolStats, Pipeline]
```

### Method Chaining (`chain_result`)

Many Go APIs use a fluent pattern where methods return command objects with a `.Result()` method (e.g. go-redis `*StatusCmd`, `*StringCmd`). When `error_to_result: true` is set, Funxy **auto-detects** these patterns: if a method returns a single non-error type that has a `.Result() → (T, error)` method, chaining is applied automatically.

```yaml
- type: Client
  as: redis
  methods: [Set, Get, Del, Close]
  skip_context: true
  error_to_result: true
  # chain_result is auto-detected for Set/Get/Del (return *StatusCmd/*StringCmd)
  # Close() returns error directly — no chaining needed, handled as-is
```

You can also set `chain_result` explicitly to override auto-detection or chain a different method:

```yaml
- type: DynamoDB
  as: dynamo
  chain_result: "Result"
  error_to_result: true
```

Funxy collapses `obj.PutItem(input).Result()` into a single call, handling the error automatically. Methods that already return `error` or `(T, error)` are not chained — they are handled directly.

## Callbacks (Function Parameters)

Funxy closures are automatically wrapped into Go `func(...)` types when passed to bound functions. This enables APIs that accept callbacks:

```yaml
deps:
  - pkg: sort
    bind:
      - func: Search
        as: sortSearch
```

```rust
import "ext/sort" (sortSearch)

// Find the smallest index i in [0, 10) such that i >= 6
idx = sortSearch(10, fun(i) { i >= 6 })
print(idx) // 6
```

The wrapping is transparent: Funxy closures are converted to Go functions at call time, with arguments and return values automatically marshalled between Funxy and Go types.

If the Go callback returns `error` and the Funxy function returns an error, it's propagated as a Go error. For callbacks without error returns, a Funxy error causes a panic.

## Struct Field Access

Exported struct fields are automatically bound as getter functions. For a type binding with `as: "redis"`, each field generates `<prefix><FieldName>(obj)`:

```yaml
- type: Options
  as: opts
```

```rust
import "ext/redis" (optsAddr, optsPassword, optsDB)

print("Connecting to: " ++ optsAddr(options))
```

Field getters are read-only. To modify fields, use methods.

Passing a nil pointer to a field getter produces a clear error instead of a panic:

```rust
// If opts is nil:
optsAddr(opts)
// Error: optsAddr: nil pointer dereference on *redis.Options
```

## Binding Constants

Package-level constants can be bound with `const:`:

```yaml
deps:
  - pkg: net/http
    bind:
      - const: StatusOK
        as: httpStatusOK
      - const: StatusNotFound
        as: httpStatusNotFound
      - const: MethodGet
        as: httpMethodGet
```

```rust
import "ext/http" (httpStatusOK, httpMethodGet)

if status == httpStatusOK {
  print("Request succeeded")
}
```

Constants are registered as values (not functions) — use them directly without calling.

With `bind_all: true`, all exported constants are bound automatically.

## Automatic Binding (`bind_all`)

Instead of listing every function and type manually, you can bind everything exported by a package using `bind_all: true`.

```yaml
deps:
  - pkg: github.com/google/uuid
    version: v1.3.0
    as: uuid
    bind_all: true
```

This automatically generates bindings for:
- All exported functions: `uuidNew()`, `uuidNewString()`, etc.
- All exported types and their methods: `uuidUUID`, `uuidUUID.String()`, etc.
- All exported constants.

The naming convention uses the `as` alias (or package name) as a prefix:
- `uuid.New()` → `uuidNew()`
- `uuid.Must(u, err)` → `uuidMust(u, err)`

### Comparison

| Feature | `bind: [...]` | `bind_all: true` |
|---------|--------------|------------------|
| **Control** | Precise (only what you need) | Everything (convenient but larger binary) |
| **Naming** | Custom (`as: myName`) | Auto (`<alias><Name>`) |
| **Options** | Per-binding (`error_to_result`) | Global defaults applied |

When using `bind_all`, you can't customize individual bindings (like `error_to_result` for just one function).

## Module Names

The ext module name is derived from the Go package path:

| Go package | Module name |
|-----------|------------|
| `github.com/slack-go/slack` | `ext/slack` |
| `github.com/google/uuid` | `ext/uuid` |
| `github.com/redis/go-redis/v9` | `ext/go-redis` |

Override with `as` at the dep level:

```yaml
deps:
  - pkg: github.com/slack-go/slack
    version: v0.15.0
    as: notifier     # → import "ext/notifier"
    bind:
      - func: New
        as: notifierNew
```

### Naming Rules & Sanitization

Funxy automatically sanitizes package names to generate valid internal Go identifiers:
- **Hyphens, dots, and spaces are removed** (`go-redis` → `goredis`).
- **Version tags are stripped** (`v9` is ignored if it's the last segment).
- **Reserved keywords are prefixed** (`go` → `pkgGo`).

This ensures compatibility with packages that use non-standard naming. You can always override the module name using `as`.

## `funxy.yaml` Reference

```yaml
deps:
  - pkg: <go_import_path>       # required — Go package path
    version: <semver>            # required — version tag (ignored for local deps)
    module: <go_module_path>     # optional — Go module path for go.mod (when differs from pkg)
    local: <path>                # optional — path to local Go package (relative to funxy.yaml)
    as: <alias>                  # optional — override module name

    bind_all: true               # bind all exported types/functions
    # OR
    bind:                        # explicit bindings
      - func: <GoName>          # function binding
        as: <funxyName>         # required — name in Funxy
        error_to_result: true   # (T, error) → Result<String, T>
        skip_context: true      # auto-inject context.Background()

      - type: <GoTypeName>     # type binding
        as: <prefix>           # required — method name prefix
        methods: [A, B]        # optional whitelist
        exclude_methods: [X]   # optional blacklist
        constructor: true      # generate <prefix>({ ... }) constructor
        type_args: [string]    # Go type args for generics (e.g. Set[string])
        chain_result: "Result" # optional method chaining
        error_to_result: true
        skip_context: true

      - const: <GoConstName>  # constant binding
        as: <funxyName>       # required — name in Funxy
```

Rules:
- `bind` and `bind_all` are mutually exclusive
- Each binding is `func`, `type`, or `const` — mutually exclusive
- `as` is required everywhere
- `methods`, `exclude_methods`, `chain_result`, `constructor`, `type_args` — only for `type` (except `type_args` also works with `func`)
- `const` bindings only support `as`
- Aliases must be unique across the entire config
- `local` and `version` are mutually exclusive — local deps don't need a version
- `module` is for monorepo packages where Go module path differs from the import path (e.g. AWS SDK v2)

## Local Go Packages

You don't have to publish your Go code to GitHub. Funxy supports local Go packages — just point `local:` to a directory on disk:

```yaml
deps:
  - pkg: mycompany.dev/internal/auth
    local: ./golib/auth
    bind:
      - func: Verify
        as: authVerify
        error_to_result: true
      - func: NewToken
        as: authNewToken
```

The `local:` path is relative to the `funxy.yaml` file. The directory must be a valid Go module (with its own `go.mod`) or be part of one.

### How it works

When `local:` is specified, Funxy:

1. Skips downloading the package from the network
2. Adds a `replace` directive to the generated `go.mod`, pointing the import path to your local directory
3. Runs `go mod tidy` which resolves the package from disk

The `version:` field is ignored for local deps — you don't need to specify it.

### Example: local helper library

Project structure:

```
myproject/
├── funxy.yaml
├── app.lang
└── golib/
    └── utils/
        ├── go.mod      # module mycompany.dev/internal/utils
        ├── hash.go     # func Hash(s string) string
        └── random.go   # func RandomID() string
```

```yaml
# funxy.yaml
deps:
  - pkg: mycompany.dev/internal/utils
    local: ./golib/utils
    bind:
      - func: Hash
        as: utilsHash
      - func: RandomID
        as: utilsRandomID
```

```rust
// app.lang
import "ext/utils" (utilsHash, utilsRandomID)

id = utilsRandomID()
print("ID: " ++ id ++ ", hash: " ++ utilsHash(id))
```

### Mixing remote and local deps

You can freely mix remote (GitHub) and local packages:

```yaml
deps:
  - pkg: github.com/slack-go/slack
    version: v0.15.0
    bind:
      - func: New
        as: slackNew

  - pkg: mycompany.dev/internal/auth
    local: ./golib/auth
    bind:
      - func: Verify
        as: authVerify
```

### Requirements for local packages

- The directory must exist and contain Go source files
- It should be a Go module (have `go.mod`) or be in a subdirectory of a Go module
- The `pkg:` value must match the module path in the local `go.mod`

## CLI Commands

### `funxy ext build` (Custom Interpreter)

Creates a custom interpreter binary containing your Go extensions.

```bash
$ funxy ext build -o myfunxy
Building custom Funxy with 2 ext deps...
  github.com/slack-go/slack v0.15.0 → ext/slack
  github.com/redis/go-redis/v9 v9.7.0 → ext/redis
Built: myfunxy (12.3 MB)
```

**Development Workflow:**
1. Build interpreter once: `funxy ext build -o myfunxy`
2. Run scripts instantly: `./myfunxy app.lang`
3. Edit `app.lang` and run again

This acts exactly like the standard `funxy` command, but with your `ext/*` modules built-in. You can also use it as a host for bundling: `funxy build app.lang --host myfunxy`.

### `funxy ext check`

Validates `funxy.yaml`, downloads Go packages, inspects types, reports binding count:

```bash
$ funxy ext check
Config: funxy.yaml ✓
Dependencies: 2
  github.com/slack-go/slack v0.15.0 → ext/slack
  github.com/redis/go-redis/v9 v9.7.0 → ext/redis

Inspecting Go packages...
Resolved 6 bindings:
  func slack.New → slackNew
  type slack.Client → 2 methods
  type redis.Client → 4 methods

All checks passed ✓
```

### `funxy ext list`

Shows what's declared in `funxy.yaml` (no Go toolchain needed):

```bash
$ funxy ext list
ext/slack (github.com/slack-go/slack v0.15.0)
  func New as slackNew
  type Client as slack [PostMessage, GetUserInfo]
ext/redis (github.com/redis/go-redis/v9 v9.7.0)
  type Client as redis [Set, Get, Del, Close]
```

### `funxy ext stubs`

Generates `.d.lang` files for editor support (LSP autocomplete):

```bash
$ funxy ext stubs
Generated stubs in .funxy/ext
  slack.d.lang
  redis.d.lang
```

The `.funxy/` directory is auto-added to `.gitignore`.

## Build Flags

| Flag | Description |
|------|-----------|
| `--config <path>` | Path to `funxy.yaml` (default: auto-detect in script directory) |
| `--ext-verbose` | Show detailed ext build output |

```bash
funxy build app.lang --config funxy.yaml --ext-verbose -o myapp
```

## Caching

The compiled ext host binary is cached in `.funxy/ext-cache/`. The cache key is a hash of:
- `funxy.yaml` contents
- Target OS and architecture
- Codegen version

If `funxy.yaml` hasn't changed, rebuilds reuse the cached binary — nearly instant.

```bash
# Clear the cache
rm -rf .funxy/ext-cache/
```

## Type Mapping

| Go type | Funxy type |
|---------|-----------|
| `int`, `int64`, `int32`, `int16`, `int8` | `Int` |
| `uint`, `uint64`, `uint32`, `uint16`, `uint8` | `Int` |
| `float64`, `float32` | `Float` |
| `bool` | `Bool` |
| `string` | `String` |
| `[]byte` | `Bytes` |
| `[]T` | `List<T>` |
| `map[K]V` | `Map<K, V>` |
| `func(...)` | Funxy closure (auto-wrapped) |
| `error` | `String` (inside `Result`) |
| `context.Context` | auto-injected |
| struct, interface, pointer | `HostObject` |

Go maps are converted to/from the Funxy `Map` type. Go `func(...)` parameters accept Funxy closures — they are automatically wrapped into the matching Go function type.

Go values without a direct Funxy equivalent are wrapped in `HostObject` — an opaque container. Methods on these objects are called via generated wrapper functions (the type bindings).

## Example: Slack + Redis

```yaml
deps:
  - pkg: github.com/slack-go/slack
    version: v0.15.0
    bind:
      - func: New
        as: slackNew
      - func: MsgOptionText
        as: slackMsgOptionText
      - type: Client
        as: slack
        methods: [PostMessage]
        error_to_result: true

  - pkg: github.com/redis/go-redis/v9
    version: v9.7.0
    bind:
      - func: ParseURL
        as: redisParseURL
        error_to_result: true
      - func: NewClient
        as: redisNew
      - type: Client
        as: redis
        methods: [Set, Get, Del, Close]
        skip_context: true
        error_to_result: true
```

```rust
import "ext/slack" (slackNew, slackPostMessage, slackMsgOptionText)
import "ext/redis" (redisParseURL, redisNew, redisGet)

// Initialize Redis client
redis = match redisParseURL("redis://localhost:6379/0") {
  Ok(opts) -> redisNew(opts)
  Fail(e)  -> { print("Redis init error: " ++ show(e)); exit(1) }
}

// Read a config value from Redis
match redisGet(redis, "slack-channel") {
  Ok(channel) -> {
    client = slackNew("xoxb-your-token")
    msg = slackMsgOptionText("Deploy complete!", false)
    match slackPostMessage(client, channel, msg) {
      Ok(_)    -> print("Notification sent")
      Fail(e)  -> print("Slack error: " ++ show(e))
    }
  }
  Fail(e) -> print("Redis error: " ++ show(e))
}
```

## ext/* vs lib/*

| | `lib/*` | `ext/*` |
|--|--------|--------|
| Where | Built into Funxy | Defined in `funxy.yaml` |
| Examples | `lib/json`, `lib/http`, `lib/sql` | `ext/slack`, `ext/redis` |
| Needs Go? | No | Yes (for initial build) |
| Import | `import "lib/list"` | `import "ext/slack"` |
| Binary size | Always included | Only when used |

If Funxy has a built-in module for something (e.g. `lib/uuid`, `lib/http`), prefer it. Use `ext/*` for Go packages that aren't in the stdlib.

## Requirements

- Go toolchain installed (for the initial ext build)
- Network access (to download Go packages on first build) — not needed for local deps
- Subsequent builds use cached binaries — no network needed

You do **not** need the Funxy source code locally. The builder automatically fetches the correct version of the Funxy runtime as a Go module (`github.com/funvibe/funxy`).

If you *do* have the source (e.g. developing Funxy itself), you can set `FUNXY_HOME` to point to it, and the builder will use your local source instead of the remote module.

## Limitations

The ext mechanism covers the most common Go integration patterns, but has boundaries:

### Struct creation via `constructor`

You can't write `redis.Options{Addr: "..."}` directly in Funxy — but you can create Go structs from Funxy records using `constructor: true`:

```yaml
- type: Options
  as: opts
  constructor: true   # → opts({ addr: "localhost:6379", db: 0 })
```

```rust
import "ext/redis" (opts, redisNew)

options = opts({ addr: "localhost:6379", password: "", db: 0 })
client = redisNew(options)
```

The constructor takes a Funxy record and maps camelCase field names to Go PascalCase fields (`addr` → `Addr`, `db` → `DB`). Omitted fields use Go zero values. The result is a `HostObject` wrapping a pointer to the struct (`*Options`).

**Supported field types:**
- Basic types: `string`, `int`, `int32`, `int64`, `float64`, `bool`
- Pointers to basic types: `*string`, `*int32`, `*bool` (common in APIs like AWS SDK)
- Nested structs and pointers to structs

**Skipped fields (silently ignored):**
- Function-typed fields (`func() Retryer`, `func(*Options)`)
- Interface-typed fields
- Channel-typed fields

Only enable `constructor` for struct types you actually need to create (like `Options`, `Config`, `Input`). Types returned from functions (like `Client`, `Response`) don't need it.

### Monorepo packages (`module:`)

Some Go libraries (notably AWS SDK v2) are structured as monorepos: the Go module path in `go.mod` differs from the package import path. For example:

- Module: `github.com/aws/aws-sdk-go-v2` (in `go.mod`)
- Package: `github.com/aws/aws-sdk-go-v2/aws` (in Go `import`)

Use the `module:` field to handle this:

```yaml
deps:
  - pkg: github.com/aws/aws-sdk-go-v2/aws
    module: github.com/aws/aws-sdk-go-v2
    version: v1.36.3
    bind:
      - type: Config
        as: awsConfig
        constructor: true

  - pkg: github.com/aws/aws-sdk-go-v2/service/s3
    version: v1.79.2
    bind:
      - func: NewFromConfig
        as: s3New
```

The `module:` value is used for the `require` directive in the generated `go.mod`, while `pkg:` is used as the Go import path for code generation and inspection. When `module:` is not specified, `pkg:` is used for both.

### No pointer out-parameters

Go functions that write results via pointer arguments (`Scan(&dest)`, `Unmarshal(data, &obj)`) cannot be called directly from Funxy. There is no `&` operator in Funxy — you can't create a pointer to a Funxy value and pass it to Go. Even if you wrap a pointer in a `HostObject`, the modified value won't flow back to Funxy variables. **Workaround**: write a Go wrapper that calls the pointer-based API internally and returns the result as a value:

```go
// Go wrapper
func ScanRow(rows *sql.Rows) (string, int, error) {
    var name string
    var age int
    err := rows.Scan(&name, &age)
    return name, age, err
}
```

### Struct field access is read-only

Exported struct fields are auto-generated as getter functions: `<prefix><FieldName>(obj)`. For example, binding `type: Options` with `as: redis` generates `redisAddr(opts)`, `redisPassword(opts)`, etc. Writing fields is not supported — use methods for mutation.

### No channels

`chan T` is mapped to `HostObject`. You can receive a channel from Go and pass it back, but there's no way to send (`ch <- val`) or receive (`<-ch`) from Funxy. **Workaround**: bind Go helper functions like `ChanSend(ch, val)` / `ChanRecv(ch)`.

### No goroutines or concurrency

There's no `go func()`, no `sync.Mutex`, no `sync.WaitGroup`. All ext calls are synchronous. **Workaround**: bind Go helper functions that manage concurrency internally.

### Go generics

Go generic functions and types are supported. For unconstrained type parameters (`any`), no configuration is needed — they are auto-instantiated:

```yaml
- func: Map          # func Map[T, U any]([]T, func(T) U) []U
  as: colMap          # → colMap[any, any](...)
```

For constrained type parameters (`comparable`, custom constraints), specify `type_args`:

```yaml
- func: Contains      # func Contains[T comparable]([]T, T) bool
  as: colContains
  type_args: [string]  # → Contains[string](...)

- type: Set            # type Set[T comparable] struct{...}
  as: colSet
  type_args: [string]  # methods operate on Set[string]
  methods: [Add, Has, Len, Values]
```

Supported type args: `string`, `int`, `int8`–`int64`, `uint`–`uint64`, `float32`, `float64`, `bool`, `any`.

### Can't implement Go interfaces from Funxy

You can call methods on a Go object that implements an interface. But you can't write a Funxy type that satisfies `io.Reader`, `http.Handler`, or any other Go interface and pass it to Go code. **Workaround**: write a Go adapter struct that implements the interface and delegates to Funxy via callbacks.

### Multiple return values → Tuple

The `(T, error)` pattern is well supported via `error_to_result: true` → `Result<String, T>`. But functions returning 3+ values (e.g. `(int, string, bool)`) are wrapped in a `Tuple`, which is less ergonomic — you need to destructure by index.

## Summary

| What | How |
|------|-----|
| Declare dependency | `funxy.yaml` with `deps` |
| Bind a function | `func: GoName`, `as: funxyName` |
| Bind type methods | `type: GoType`, `as: prefix`, `methods: [...]` |
| Bind a constant | `const: GoConst`, `as: funxyName` |
| Create Go structs | `constructor: true` → `<prefix>({ field: val, ... })` |
| Read struct fields | Auto-generated: `<prefix><FieldName>(obj)` |
| Pass callbacks | Funxy closures auto-wrap into Go `func(...)` |
| Handle Go errors | `error_to_result: true` → `Result<String, T>` |
| Skip context.Context | `skip_context: true` |
| Use local Go code | `local: ./path/to/pkg` (no network, no version) |
| Monorepo packages | `module: <go_module_path>` (when module ≠ package path) |
| Bind Go generics | `type_args: [string]` (auto for `any`, explicit for `comparable`) |
| Build | `funxy build app.lang` (auto-detects `funxy.yaml`) |
| Build interpreter | `funxy ext build -o myfunxy` (full interpreter + ext) |
| Validate | `funxy ext check` |
| LSP stubs | `funxy ext stubs` |
