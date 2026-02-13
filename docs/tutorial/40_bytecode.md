# Bytecode Compilation & Self-Contained Binaries

Funxy supports compilation to bytecode and building self-contained native binaries.

## Self-Contained Binaries

The `build` command creates a single executable that includes your script and the Funxy runtime:

```bash
# Build a standalone binary
funxy build script.lang              # creates ./script
funxy build script.lang -o myapp     # custom output name

# Run without Funxy installed
./myapp
```

### Dual-Mode Operation

Self-contained binaries also work as full Funxy interpreters. Pass `$` as the first argument to switch — the `$` is stripped and the rest is processed normally:

```bash
./myapp                    # runs embedded bundle
./myapp --port 8080        # runs embedded bundle (flags go to your app via sysArgs)
./myapp $ other.lang       # acts as Funxy interpreter
./myapp $ -e 'print(42)'  # eval mode
./myapp $ -pe '1 + 2'     # eval with auto-print
./myapp $ --help           # shows help
```

This lets tools like the Playground invoke themselves to run user code. Use `sysExePath()` from `lib/sys` to get the executable path, and `sysScriptDir()` to resolve paths relative to the script:

```funxy
import "lib/sys" (sysExePath, sysExec, sysScriptDir)

result = sysExec(sysExePath(), ["$", userScript])  // invoke self as interpreter
dir = sysScriptDir()                               // directory of the running script
```

### Embedding Static Files (`--embed`)

Bundle static files (HTML templates, configs, images, data files) into the binary:

```bash
# Embed a directory of templates
funxy build server.lang --embed templates -o server

# Multiple directories (two equivalent forms)
funxy build app.lang --embed static --embed config -o app
funxy build app.lang --embed static,config -o app

# Glob patterns
funxy build app.lang --embed "templates/*.html" -o app
funxy build app.lang --embed "assets/*.png,config/*.toml" -o app

# Embed a single file
funxy build tool.lang --embed data/schema.json -o tool
```

Multiple paths can be comma-separated within a single `--embed`, and glob patterns (`*`, `?`, `[...]`) are supported.

Embedded files are accessible via the standard `fileRead`, `fileReadBytes`, `fileExists`, and `fileSize` functions — the binary checks embedded resources first, then falls back to the filesystem. No code changes needed:

```funxy
import "lib/io" (fileRead)

// Works the same whether interpreted or built as binary
html = fileRead("templates/index.html") |>> \x -> x
```

Paths are stored relative to the source file directory.

### Cross-Compilation (`--host`)

To build for a different OS or architecture, provide a pre-built Funxy binary for the target platform via `--host`:

```bash
# Build for Linux (from macOS or any other OS)
funxy build script.lang --host release-bin/funxy-linux-amd64 -o myapp

# Build for Windows
funxy build script.lang --host release-bin/funxy-windows-amd64.exe -o myapp.exe

# Build for macOS Intel (from ARM Mac)
funxy build script.lang --host release-bin/funxy-darwin-amd64 -o myapp-intel

# Build for FreeBSD
funxy build script.lang --host release-bin/funxy-freebsd-amd64 -o myapp-bsd
```

The bytecode is platform-independent — only the host binary determines the target. The `--host` flag requires an explicit path; there are no default targets.

### How it works

1. Your script and all user module dependencies are compiled to bytecode
2. The bytecode is serialized into a Bundle (v2 format), including any `--embed` resources
3. The Bundle is appended to the host binary (own executable or `--host`)
4. On startup, the binary detects the embedded bundle and executes it

The resulting binary includes:
- The full Funxy VM runtime
- Your script's bytecode
- All user module dependencies (pre-compiled)
- Pre-compiled trait default methods
- Embedded static files (if `--embed` was used)

Virtual modules (`lib/*`) are resolved at runtime from the built-in standard library.

## Bytecode Compilation

For pre-compiling without creating a full binary:

```bash
# Compile to bytecode bundle
funxy -c script.lang          # creates script.fbc

# Run compiled bytecode
funxy -r script.fbc
```

## Bundle Format (v2)

The v2 bundle format replaces the legacy single-chunk v1 format:

- **Magic**: `FXYB` (4 bytes)
- **Version**: `0x02` (1 byte)
- **Payload**: Gob-encoded `Bundle` struct containing:
  - `MainChunk`: compiled bytecode for the entry script
  - `Modules`: map of absolute path → pre-compiled `BundledModule`
  - `TraitDefaults`: pre-compiled trait default methods

Each `BundledModule` contains:
- `Chunk`: compiled bytecode
- `PendingImports`: the module's own import dependencies
- `Exports`: list of exported symbol names
- `TraitDefaults`: module-level trait defaults

The v1 format (single `Chunk` with `FXYB` + version `0x01`) is still supported for backwards compatibility.

## Self-Contained Binary Format

```
[Host Binary][Bundle Data][8-byte size LE][4-byte "FXYS" magic]
```

- The host binary is the Funxy runtime itself
- Bundle data is a serialized v2 Bundle
- The 12-byte footer contains the bundle size and magic marker
- On macOS, the binary is re-signed with ad-hoc signature after creation

## Performance Benefits

- **Faster startup**: No parsing or semantic analysis needed
- **Bundled dependencies**: All user modules pre-compiled
- **Zero-dependency distribution**: Single binary, no Funxy installation needed
- **UPX compatible**: Output binaries can be compressed with UPX for smaller size
