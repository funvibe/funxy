# Variables and Flow Control

Variables, blocks, and conditional expressions.

## Variables

Variables are created using `=`:

```rust
x = 10
x = x + 1    // Update
print(x)     // 11
```

## Scopes

Blocks `{ }` create a new scope:

```rust
x = 1
{
    y = 2
    x = 3    // Updates outer x
}
print(x)     // 3
// y is not available here
```

## Conditional Expressions (if/else)

`if` is an expression, returns a value:

```rust
x = 5
if x > 0 {
    print("Positive")
} else {
    print("Non-positive")
}
```

### if as expression

```rust
x = 5
val = if x > 0 { 1 } else { -1 }
print(val)  // 1

// In arguments
print(if true { "Yes" } else { "No" })
```

Types in `if` and `else` branches must match.

## Built-in Literals

| Literal | Type | Description |
|---------|-----|----------|
| `true` | `Bool` | True |
| `false` | `Bool` | False |
| `nil` | `Nil` | Absence of value |

```rust
x = true
y = false
z = nil

if z == nil {
    print("z is nil")
}
```

## Tests

See `tests/flow.lang`
