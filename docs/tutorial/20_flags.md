### `lib/flag` (Command Line Flags)

The `lib/flag` package provides a robust way to parse command-line arguments, inspired by Go's `flag` package but adapted for Funxy.

Supported argument formats:
- `-flag value`
- `--flag value`
- `-flag=value`
- `--flag=value`
- `-bool` (sets boolean flag to true)
- `-bool=true` / `-bool=false`

**Core Functions:**

- `flagSet(name: String, default: T, usage: String) -> Nil`
  - Registers a flag with a name, default value, and description.
  - The type of `default` determines the flag's type (Int, Float, Bool, String).
- `flagParse(args: List<String>?) -> Nil`
  - Parses the provided arguments (or `sysArgs()` if omitted).
  - Should be called after all `flagSet` calls and before accessing values.
- `flagGet(name: String) -> T`
  - Retrieves the value of a flag. Returns the default value if the flag was not set.
- `flagArgs() -> List<String>`
  - Returns the list of remaining non-flag arguments (positional arguments).
- `flagUsage() -> Nil`
  - Prints usage information to stderr.

**Example:**

```rust
import "lib/flag" (*)

// 1. Define flags
flagSet("port", 8080, "Server port")
flagSet("host", "localhost", "Server host")
flagSet("verbose", false, "Enable verbose logging")

// 2. Parse arguments
flagParse()

// 3. Use flags
port = flagGet("port")
host = flagGet("host")

if flagGet("verbose") {
    print("Starting server on ${host}:${port}")
}

// 4. Handle positional arguments
files = flagArgs()
```
