# Variables and Flow Control

Variables, blocks, and conditional expressions.

## Variables

Variables are created using `=`:

```rust
x = 10
x = x + 1    // Update
print(x)     // 11
```

### Mutation and Shadowing

The `=` operator defines a new variable if it doesn't exist in the current or outer scopes. If the variable *does* exist, `=` acts as an assignment (mutation) to the existing variable.

This means you **cannot shadow** a variable from an outer scope by declaring a new one with the same name. You will implicitly mutate the outer variable instead.

```rust
x = 1
{
    // This is NOT a new local variable 'x'
    // This MUTATES the outer 'x'
    x = 2
}
print(x)  // 2
```

### Global Immutability

There is one important exception to the mutation rule: **Global (Module-level) variables cannot be mutated from within functions.**

```rust
globalCounter = 0

fun increment() {
    // Error: cannot mutate global variable 'globalCounter' from within a function
    globalCounter = globalCounter + 1
}
```

However, you **can** mutate variables from an enclosing *function* scope (closures):

```rust
fun makeCounter() {
    count = 0  // Local to makeCounter

    fun inc() {
        count = count + 1  // OK: mutating closure variable
        count
    }

    inc
}
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
