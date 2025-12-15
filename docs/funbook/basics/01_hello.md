# 01. Hello World and Output

## Task
Display text on screen.

## Solution

```rust
// Simple output
print("Hello, Funxy!")

// Output with variables
name = "World"
print("Hello, " ++ name ++ "!")

// String interpolation
print("Hello, ${name}!")

// Multi-line output
print("Line 1\nLine 2\nLine 3")
```

## Explanation

- `print()` — outputs with newline
- `write()` — outputs without newline
- `++` — string concatenation
- `${...}` — interpolation inside strings
- `\n` — newline

## Variations

```rust
// Output without newline
write("Loading")
write(".")
write(".")
write(".\n")

// Output value type
x = 42
print(getType(x))  // Int

// Output list
print([1, 2, 3])  // [1, 2, 3]
```

## Number Formatting

```rust
import "lib/string" (stringPadLeft)

price = 42
print("Price: $" ++ stringPadLeft(show(price), 5, '0'))
// Price: $00042
