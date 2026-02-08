# 1. Quick Start

[← Back to Index](./00_index.md)

## Installation

```bash
# macOS
brew install funxy

# Linux
curl -sSL https://funxy.dev/install.sh | sh

# From Source
go install github.com/funxy-lang/funxy@latest
```

## First Program

Create a file named `hello.lang`:

```rust
print("Hello, Funxy!")
```

Run it:

```bash
funxy hello.lang
```

## A Taste of Funxy in 2 Minutes

```rust
import "lib/list" (*)

// Variables and Constants
name = "Alice"
const max_score = 100  // constant

// Functions
fun greet(name: String) -> String {
    "Hello, " ++ name ++ "!"
}

// Lists and Pipelines
numbers = [1, 2, 3, 4, 5]
sum_of_squares = numbers
    |> filter(fun(x) -> x % 2 == 0)
    |> map(fun(x) -> x * x)
    |> foldl((+), 0)

print(sum_of_squares)  // 20

// Pattern Matching
fun describe(x: Int) -> String {
    match x {
        0 -> "zero"
        n if n < 0 -> "negative"
        _ -> "positive"
    }
}
```

// Error Handling
```rust
import "lib/io" (fileRead)

match fileRead("config.txt") {
    Ok(content) -> print(content)
    Fail(err) -> print("Error: " ++ err)
}
```
