# Funxy Extensions (pkg/*)

Funxy allows you to embed compiled Funxy libraries directly into a binary. This is useful for distributing self-contained applications with bundled dependencies, or for creating custom Funxy interpreters with pre-loaded libraries.

## Quick Start

### 1. Create a library

```rust
// lib/math.lang
package math (getPi, add)

fun getPi() { 3.14159 }
fun add(a, b) { a + b }
```

### 2. Embed it into a binary

```bash
# Create a custom binary 'myfunxy' with 'lib/math.lang' embedded as 'pkg/math'
funxy pkg build lib/math.lang@math -o myfunxy
```

### 3. Use it in a script

```rust
// script.lang
import "pkg/math" (getPi, add)

print(getPi())
print(add(10, 20))
```

### 4. Run the script

```bash
./myfunxy script.lang
# Output:
# 3.14159
# 30
```

## How It Works

The `funxy pkg build` command:
1.  **Compiles** the specified library source files into Funxy bytecode.
2.  **Embeds** this bytecode into the binary (either a copy of the current `funxy` executable or an existing custom binary).
3.  **Registers** the library under the `pkg/` namespace (e.g., `pkg/math`).

At runtime, when a script imports `pkg/math`, the interpreter loads the pre-compiled bytecode directly from the binary, skipping disk I/O and compilation for that library.

## Managing Embedded Libraries

### Embedding Multiple Libraries

You can embed multiple libraries in a single command:

```bash
funxy pkg build lib/math.lang@math lib/utils.lang@utils -o myfunxy
```

### Updating Libraries

To update an embedded library, use the `-force` flag to overwrite the existing version:

```bash
funxy pkg build lib/math_v2.lang@math -force -o myfunxy
```

### Removing Libraries

To remove a library from the binary, use the `-delete` flag:

```bash
funxy pkg build pkg/math -delete -o myfunxy
```

### Listing Embedded Libraries

To see what libraries are embedded in a binary:

```bash
funxy pkg list myfunxy
# Output:
# Embedded packages in myfunxy:
# - pkg/math (exports: 2)
```

### Checking Libraries

To validate the integrity of embedded libraries (e.g., check for exports):

```bash
funxy pkg check myfunxy
```

### Generating Stubs (IDE Support)

To get autocomplete and type checking in your editor for embedded libraries, generate `.d.lang` stub files:

```bash
funxy pkg stubs myfunxy
# Generated stubs in .funxy/pkg
#   .funxy/pkg/math.d.lang
```

The `.funxy/` directory is automatically checked by the Funxy language server.

## Use Cases

### 1. Distributing Self-Contained Tools

If you're building a CLI tool that depends on several Funxy libraries, you can embed them all into a single binary. Users can then run your tool without needing to install dependencies or manage `FUNXY_PATH`.

### 2. Custom Interpreters

You can create specialized versions of the Funxy interpreter for your team or project. For example, a `data-funxy` binary could come pre-loaded with data analysis libraries (`pkg/dataframe`, `pkg/plot`), ready for use in scripts or REPL.

### 3. Performance

Embedded libraries are pre-compiled to bytecode. Importing them is instant, as it skips parsing and analysis. This can significantly improve startup time for scripts with many dependencies.

## Naming Conventions

*   **Namespace**: All embedded libraries live in the `pkg/` namespace.
*   **Aliases**: When embedding, you provide an alias (e.g., `@math`). The full import path becomes `pkg/math`.
*   **Automatic Prefix**: If you omit the `pkg/` prefix in the alias, it is added automatically. `funxy pkg build lib@mylib` creates `pkg/mylib`.

## Comparison with `ext/*` (Go Extensions)

| Feature | `pkg/*` (Funxy Extensions) | `ext/*` (Go Extensions) |
| :--- | :--- | :--- |
| **Source Language** | Funxy (`.lang`) | Go (`.go`) |
| **Compilation** | Compiles to Funxy bytecode | Compiles to native machine code |
| **Use Case** | Reusing Funxy code, bundling dependencies | High performance, system access, existing Go libs |
| **Build Tool** | `funxy pkg build` | `funxy ext build` (via `funxy.yaml`) |
| **Import** | `import "pkg/name"` | `import "ext/name"` |

You can combine both! A binary can have both embedded Funxy libraries (`pkg/*`) and compiled Go extensions (`ext/*`).

## CLI Reference

```bash
# Build/Embed
funxy pkg build <path>[@alias] [-force] [-delete] -o <binary>

# List
funxy pkg list <binary>

# Check
funxy pkg check <binary>

# Stubs
funxy pkg stubs <binary>