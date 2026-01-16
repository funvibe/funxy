# 41. Embedding Funxy in Go

Funxy is designed to be easily embedded into Go applications. This allows you to use Funxy as a flexible configuration language, a rule engine, or a scripting layer for your Go programs.

The `pkg/embed` package provides a high-level API for this purpose.

## Basic Setup

First, import the embed package and create a new VM instance.

```go
package main

import (
    "fmt"
    "log"
    "github.com/funvibe/funxy/pkg/embed"
)

func main() {
    // Create a new Funxy VM
    vm := funxy.New()

    // Execute simple code
    result, err := vm.Eval("10 + 20")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Result: %v\n", result) // Result: 30
}
```

## Binding Go Functions

You can expose Go functions to Funxy scripts using `vm.Bind`. The API automatically converts arguments and return values between Go and Funxy types.

```go
// Bind a simple function
vm.Bind("greet", func(name string) string {
    return fmt.Sprintf("Hello, %s!", name)
})

// Use it in Funxy
vm.Eval(`greet("Alice")`) // returns "Hello, Alice!"
```

### Supported Types

The Marshaller supports automatic conversion for:
- Integers (`int`, `int64`, etc.) -> `Int`
- Floats (`float64`, `float32`) -> `Float`
- Booleans (`bool`) -> `Bool`
- Strings (`string`) -> `String`
- Slices (`[]T`) -> `List<T>`
- Maps (`map[string]T`) -> `Map<String, T>` or Record
- Functions -> Callable objects
- Structs -> Host Objects (see below)

## Host Objects (Binding Structs)

You can bind Go structs to the VM. In Funxy, these appear as **Host Objects**. Scripts can access their exported fields and call their exported methods.

```go
type User struct {
    Name  string
    Score int
}

// Method callable from Funxy
func (u *User) AddScore(points int) {
    u.Score += points
}

func main() {
    vm := funxy.New()

    // Create a Go struct
    user := &User{Name: "Bob", Score: 100}

    // Bind it to the VM
    vm.Bind("player", user)

    // Run a script that uses the object
    code := `
    // Access field
    current = player.Score

    // Call method
    player.AddScore(50)

    // Return new score
    player.Score
    `

    res, _ := vm.Eval(code)
    fmt.Println(res) // 150

    // The Go object is modified!
    fmt.Println(user.Score) // 150
}
```

## Loading Scripts

For larger scripts, use `vm.LoadFile`. This method supports full module resolution and imports.

```go
err := vm.LoadFile("scripts/main.lang")
if err != nil {
    log.Fatal("Script error:", err)
}
```

The script can import modules just like any other Funxy program:

```rust
// scripts/main.lang
import "lib/math" (*)

fun calculate() {
    // ...
}
```

## Calling Funxy from Go

You can call functions defined in Funxy scripts from your Go code using `vm.Call`.

**script.lang**:
```rust
fun process(data) {
    "Processed: " ++ data
}
```

**main.go**:
```go
vm.LoadFile("script.lang")

// Call the 'process' function defined in the script
result, err := vm.Call("process", "some data")
// result is "Processed: some data"
```

## Error Handling

Errors from Funxy scripts (compilation or runtime) are returned as Go `error` objects. Runtime panics in Funxy are caught and returned as errors.

```go
_, err := vm.Eval("panic(\"oops\")")
if err != nil {
    fmt.Println("Caught error:", err) // Caught error: runtime error: oops
}
```

## Concurrency

The `funxy.VM` instance is **not safe for concurrent use** by multiple goroutines. If you need to execute scripts concurrently, create a separate `VM` instance for each goroutine.

However, bound Go functions are called directly. If your Go function is thread-safe, it can be called from multiple Funxy contexts (if you have multiple VMs).

## Example Project

See `examples/embed_demo` for a complete working example including:
- Binding functions and structs
- Modifying Go state from Funxy
- Calling Funxy functions from Go
- Module imports
