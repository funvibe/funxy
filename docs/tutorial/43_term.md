# Terminal UI (lib/term)

`lib/term` provides a comprehensive terminal UI toolkit: ANSI colors and styles, cursor control, interactive prompts, spinners, progress bars, and formatted tables.

## Import

```rust
import "lib/term" (*)
```

Or selectively:

```rust
import "lib/term" (red, green, bold, confirm, select, table)
```

## Colors & Styles

All style/color functions have the signature `String -> String`. They wrap text with ANSI escape codes. When stdout is not a TTY or the `$NO_COLOR` environment variable is set, they become identity functions (return text unchanged).

### Text Styles

```rust
import "lib/term" (bold, dim, italic, underline, strikethrough)

print(bold("Important"))
print(dim("Muted text"))
print(italic("Emphasized"))
print(underline("Link-like"))
print(strikethrough("Deprecated"))
```

### Foreground Colors

```rust
import "lib/term" (red, green, yellow, blue, magenta, cyan, white, gray)

print(red("Error: something failed"))
print(green("Success!"))
print(yellow("Warning: check this"))
print(blue("Info message"))
print(gray("Debug output"))
```

### Background Colors

```rust
import "lib/term" (bgRed, bgGreen, bgYellow, bgBlue, bgCyan, bgMagenta)

print(bgRed("  CRITICAL  "))
print(bgGreen("  PASSED  "))
```

### Composition via Pipe

Styles compose naturally using the `|>` pipeline:

```rust
import "lib/term" (red, bold, underline, green, italic)

// Bold red error
"ERROR: disk full" |> red |> bold |> print

// Green underlined success
"All tests passed" |> green |> underline |> print

// Italic bold warning
"Deprecation notice" |> italic |> bold |> print
```

### Extended Colors (RGB, Hex)

For terminals with truecolor support (16M colors):

```rust
import "lib/term" (rgb, bgRgb, hex, bgHex)

// RGB foreground
print(rgb(255, 128, 0, "Orange text"))

// Hex foreground
print(hex("#FF8000", "Also orange"))

// RGB background
print(bgRgb(0, 100, 200, "Custom background"))

// Hex background
print(bgHex("#006400", "Dark green bg"))
```

### Removing ANSI Codes

`stripAnsi` removes all ANSI escape sequences from a string — useful for logging to files or computing visible text length:

```rust
import "lib/term" (red, bold, stripAnsi)

styled = bold(red("hello"))
plain = stripAnsi(styled)
print(plain)  // "hello"
```

### Color Level Detection

```rust
import "lib/term" (termColors)

level = termColors()
// 0        — no color (not a TTY, NO_COLOR, or TERM=dumb)
// 1        — basic 16 colors
// 256      — 256-color terminal
// 16777216 — truecolor (24-bit)
```

### cprint — Print with Style

`cprint` applies a style function to its arguments and prints them:

```rust
import "lib/term" (cprint, red, green, bold)

cprint(red, "Error:", "file not found")
cprint(green, "OK:", "all checks passed")
cprint(bold, "Total:", "42 items")
```

## Terminal Control

### Terminal Size

```rust
import "lib/term" (termSize)

(cols, rows) = termSize()
print("Terminal: ${show(cols)}x${show(rows)}")
```

### TTY Detection

```rust
import "lib/term" (termIsTTY)

if termIsTTY() {
    print("Running interactively")
} else {
    print("Output is piped/redirected")
}
```

### Screen and Line Clearing

```rust
import "lib/term" (termClear, termClearLine)

termClear()       // Clear entire screen
termClearLine()   // Clear current line
```

### Cursor Control

```rust
import "lib/term" (cursorUp, cursorDown, cursorLeft, cursorRight,
                   cursorTo, cursorHide, cursorShow)

cursorUp(3)         // Move cursor up 3 lines
cursorDown(1)       // Move down 1 line
cursorLeft(5)       // Move left 5 columns
cursorRight(10)     // Move right 10 columns
cursorTo(0, 0)      // Move to column 0, row 0 (top-left)

cursorHide()        // Hide cursor (useful during animations)
// ... render something ...
cursorShow()        // Restore cursor
```

## Interactive Prompts

### Text Input

```rust
import "lib/term" (prompt)

name = prompt("Your name", "Anonymous")   // with default
email = prompt("Email", "")              // empty default
```

### Confirmation (Yes/No)

```rust
import "lib/term" (confirm)

if confirm("Continue?") {            // default: true
    print("Proceeding...")
}

if confirm("Delete all data?", false) {  // default: false
    print("Deleting...")
}
```

### Password Input

Characters are hidden during input:

```rust
import "lib/term" (password)

pass = password("Enter password")
// User types, nothing is echoed
```

### Single Selection Menu

Arrow keys (↑↓) to navigate, Enter to confirm:

```rust
import "lib/term" (select)

lang = select("Favorite language", ["Funxy", "Go", "Rust", "Haskell", "Python"])
print("You chose: " ++ lang)
```

Terminal output:

```
? Favorite language
  ▸ Funxy
    Go
    Rust
    Haskell
    Python
```

### Multi-Selection Menu

Space to toggle, Enter to confirm:

```rust
import "lib/term" (multiSelect)

features = multiSelect("Enable features", [
    "Colors",
    "Logging",
    "Metrics",
    "Tracing",
    "Profiling"
])
print("Enabled: " ++ show(features))
```

Terminal output:

```
? Enable features
  ▸ ◻ Colors
    ◻ Logging
    ◻ Metrics
    ◻ Tracing
    ◻ Profiling
```

## Spinners & Progress Bars

### Spinner

An animated indicator for long-running operations:

```rust
import "lib/term" (spinnerStart, spinnerUpdate, spinnerStop, green)

s = spinnerStart("Loading data...")

// ... do work ...
spinnerUpdate(s, "Processing records...")

// ... more work ...
spinnerStop(s, green("✓") ++ " Done!")
```

The spinner animates through `⠋ ⠙ ⠹ ⠸ ⠼ ⠴ ⠦ ⠧ ⠇ ⠏` characters with the message.

### Progress Bar

A visual progress indicator with percentage:

```rust
import "lib/term" (progressNew, progressTick, progressSet, progressDone, green)

bar = progressNew(100, "Downloading")

for i in 0..99 {
    // ... do work ...
    progressTick(bar)          // Increment by 1
}

progressDone(bar)

// Or set to a specific value:
bar2 = progressNew(50, "Processing")
progressSet(bar2, 25)          // Jump to 50%
progressDone(bar2)
```

Terminal output:

```
Downloading [████████████░░░░░░░░] 60%
```

## Tables

Print formatted tables with Unicode box-drawing characters and auto-aligned columns:

```rust
import "lib/term" (table)

table(
    ["Name", "Language", "Stars"],
    [
        ["Funxy", "Go", "1000"],
        ["Haskell", "Haskell", "999"],
        ["Rust", "Rust", "500"]
    ]
)
```

Output:

```
┌─────────┬──────────┬───────┐
│ Name    │ Language │ Stars │
├─────────┼──────────┼───────┤
│ Funxy   │ Go       │ 1000  │
│ Haskell │ Haskell  │ 999   │
│ Rust    │ Rust     │ 500   │
└─────────┴──────────┴───────┘
```

You can combine tables with styles:

```rust
import "lib/term" (table, bold, red, green)

table(
    [bold("Status"), bold("Service"), bold("Uptime")],
    [
        [green("●"), "api-server",  "99.9%"],
        [green("●"), "db-primary",  "99.8%"],
        [red("●"),   "cache-node3", "—"]
    ]
)
```

## `$NO_COLOR` Convention

Funxy respects the [`NO_COLOR`](https://no-color.org/) convention. When the `$NO_COLOR` environment variable is set (to any value), all color and style functions return text unchanged:

```bash
NO_COLOR=1 funxy script.lang   # No ANSI codes in output
```

This also happens automatically when stdout is piped or redirected.

## Practical Examples

### Colored CLI Output

```rust
import "lib/term" (red, green, yellow, bold, dim)

fun logError(msg) { print(bold(red("ERROR")) ++ " " ++ msg) }
fun logWarn(msg)  { print(bold(yellow("WARN")) ++ "  " ++ msg) }
fun logOk(msg)    { print(bold(green("OK")) ++ "    " ++ msg) }
fun logDebug(msg) { print(dim("DEBUG") ++ " " ++ msg) }

logOk("Server started on :8080")
logWarn("Config file not found, using defaults")
logError("Connection refused")
logDebug("Request: GET /api/users")
```

### Interactive Installer

```rust
import "lib/term" (bold, green, red, confirm, select, multiSelect,
                   spinnerStart, spinnerStop, table)
import "lib/time" (sleep)

print(bold("Welcome to MyApp Installer"))
print("")

env = select("Target environment", ["Development", "Staging", "Production"])
features = multiSelect("Select features", [
    "Core",
    "Analytics",
    "Notifications",
    "Admin Panel"
])

if confirm("Install " ++ show(len(features)) ++ " features to " ++ env ++ "?") {
    s = spinnerStart("Installing...")
    sleep(2000)
    spinnerStop(s, green("✓") ++ " Installed!")

    print("")
    table(
        ["Feature", "Status"],
        [["f", green("✓ Installed")] | f <- features]
    )
} else {
    print(red("Installation cancelled."))
}
```

### Deploy Script

```rust
import "lib/term" (bold, green, yellow, red, confirm, select,
                   spinnerStart, spinnerUpdate, spinnerStop)
import "lib/sys" (sysExec)
import "lib/time" (sleep)

env = select("Deploy to", ["dev", "staging", "prod"])

if env == "prod" && !confirm(bold(yellow("⚠ Deploy to PRODUCTION?")), false) {
    print("Aborted.")
} else {
    s = spinnerStart("Building...")
    sleep(1000)
    spinnerUpdate(s, "Running tests...")
    sleep(1000)
    spinnerUpdate(s, "Deploying to " ++ env ++ "...")
    sleep(1000)
    spinnerStop(s, green("✓") ++ " Deployed to " ++ bold(env))
}
```

## Summary

| Function | Signature | Description |
|----------|-----------|-------------|
| `bold`, `dim`, `italic`, `underline`, `strikethrough` | `String -> String` | Text styles |
| `red`, `green`, `yellow`, `blue`, `magenta`, `cyan`, `white`, `gray` | `String -> String` | Foreground colors |
| `bgRed`, `bgGreen`, `bgYellow`, `bgBlue`, `bgCyan`, `bgMagenta` | `String -> String` | Background colors |
| `rgb` | `Int, Int, Int, String -> String` | RGB foreground |
| `bgRgb` | `Int, Int, Int, String -> String` | RGB background |
| `hex` | `String, String -> String` | Hex foreground |
| `bgHex` | `String, String -> String` | Hex background |
| `stripAnsi` | `String -> String` | Remove ANSI codes |
| `termColors` | `() -> Int` | Detect color level |
| `cprint` | `(String -> String), ...String -> Nil` | Print with style |
| `termSize` | `() -> (Int, Int)` | Terminal dimensions |
| `termIsTTY` | `() -> Bool` | Is stdout a TTY? |
| `termClear` | `() -> Nil` | Clear screen |
| `termClearLine` | `() -> Nil` | Clear line |
| `cursorUp`, `cursorDown`, `cursorLeft`, `cursorRight` | `Int -> Nil` | Move cursor |
| `cursorTo` | `Int, Int -> Nil` | Move to position |
| `cursorHide`, `cursorShow` | `() -> Nil` | Toggle cursor |
| `prompt` | `String, String? -> String` | Text input |
| `confirm` | `String, Bool? -> Bool` | Yes/no |
| `password` | `String -> String` | Hidden input |
| `select` | `String, List<String> -> String` | Single choice |
| `multiSelect` | `String, List<String> -> List<String>` | Multi choice |
| `spinnerStart` | `String -> Handle` | Start spinner |
| `spinnerUpdate` | `Handle, String -> Nil` | Update message |
| `spinnerStop` | `Handle, String -> Nil` | Stop spinner |
| `progressNew` | `Int, String -> Handle` | New progress bar |
| `progressTick` | `Handle -> Nil` | Increment +1 |
| `progressSet` | `Handle, Int -> Nil` | Set value |
| `progressDone` | `Handle -> Nil` | Complete |
| `table` | `List<String>, List<List<String>> -> Nil` | Print table |
