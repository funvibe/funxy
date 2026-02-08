# System Functions (lib/sys)

The `lib/sys` module provides access to system functions: command line arguments, environment variables, program termination, and executing external commands.

```rust
import "lib/sys" (*)
```

## Functions

### sysArgs

```rust
sysArgs() -> List<String>
```

Returns command line arguments (without the program name).

```rust
import "lib/sys" (sysArgs)

// Run: ./funxy script.lang hello world
arguments = sysArgs()
// arguments = ["script.lang", "hello", "world"]

print(len(arguments))  // number of arguments
```

### sysEnv

```rust
sysEnv(name: String) -> Option<String>
```

Returns the value of an environment variable or `None` if not set.

```rust
import "lib/sys" (sysEnv)

// PATH exists on all systems
pathOpt = sysEnv("PATH")
hasPath = match pathOpt {
    Some(_) -> true
    None -> false
}
print(hasPath)  // true

// Non-existent variable
noVar = sysEnv("NONEXISTENT_VAR")
match noVar {
    Some(val) -> print("Value: " ++ val)
    None -> print("Not set")
}
// Not set
```

### sysExit

```rust
sysExit(code: Int) -> Nil
```

Terminates the program with the specified return code.

- `0` — successful termination
- `1` and above — error

```rust
import "lib/sys" (sysExit)

// Successful termination
sysExit(0)

// Error termination
sysExit(1)
```

**Important:** code after `sysExit()` is not executed.

### sysExec

```rust
sysExec(cmd: String, args: List<String>) -> { code: Int, stdout: String, stderr: String }
```

Executes an external command and returns the result as a record with fields:
- `code` — return code (0 = success, -1 = command not found)
- `stdout` — standard output
- `stderr` — standard error output

```rust
import "lib/sys" (sysExec)

// Simple command
echoResult = sysExec("echo", ["hello"])
print(echoResult.code == 0)  // true
print(len(echoResult.stdout) > 0)  // true

// Command with multiple arguments
lsResult = sysExec("ls", ["-la", "/"])
print(lsResult.code == 0)  // true on Unix systems

// Non-existent command
badResult = sysExec("nonexistent_command", [])
print(badResult.code == -1)  // true - command not found

// Check field types
print(getType(echoResult.code))    // Int
print(getType(echoResult.stdout))  // String
print(getType(echoResult.stderr))  // String
```

## Practical Examples

### CLI with Arguments

```rust
import "lib/sys" (sysArgs)
import "lib/list" (length, headOr, tail)

arguments = sysArgs()

// Check number of arguments
if length(arguments) < 2 {
    print("Usage: program <command>")
} else {
    // Skip script name, take command
    cmd = headOr(tail(arguments), "help")
    match cmd {
        "help" -> print("Commands: help, version")
        "version" -> print("v1.0.0")
        _ -> print("Unknown: " ++ cmd)
    }
}
```

### Environment Variable Check

```rust
import "lib/sys" (sysEnv)

// Get port from environment or use 8080
port = match sysEnv("PORT") {
    Some(p) -> p
    None -> "8080"
}
print("Port: " ++ port)

// Check debug mode
isDebug = match sysEnv("DEBUG") {
    Some(_) -> true
    None -> false
}
print(isDebug)
```

### Getting Command Output

```rust
import "lib/sys" (sysExec)

result = sysExec("git", ["status", "--short"])
if result.code == 0 {
    print("Git status:")
    print(result.stdout)
} else {
    print("Git error: " ++ result.stderr)
}
```

### Example: Simple grep

```rust
import "lib/sys" (sysArgs)
import "lib/io" (fileRead)
import "lib/list" (length, headOr, tail)
import "lib/string" (stringLines)
import "lib/regex" (regexMatch)

arguments = sysArgs()

if length(arguments) < 3 {
    print("Usage: grep <pattern> <file>")
} else {
    // arguments[0] = script, [1] = pattern, [2] = file
    pattern = headOr(tail(arguments), "")
    filename = headOr(tail(tail(arguments)), "")

    match fileRead(filename) {
        Ok(content) -> {
            allLines = stringLines(content)
            // Simple search with regex
            for line in allLines {
                if regexMatch(pattern, line) {
                    print(line)
                }
            }
        }
        Fail(err) -> print("Error: " ++ err)
    }
}
```

## Summary

| Function | Type | Description |
|---------|-----|----------|
| `sysArgs` | `() -> List<String>` | Command line arguments |
| `sysEnv` | `(String) -> Option<String>` | Environment variable |
| `sysExit` | `(Int) -> Nil` | Terminate program |
| `sysExec` | `(String, List<String>) -> { code: Int, stdout: String, stderr: String }` | Execute command |
