# Constants

Constants are named values that cannot be changed after they are defined. They are declared using the `const` keyword (or the `:-` operator).

## Syntax & Examples

```rust
// Inferred type (Float)
const pi = 3.14159

// Explicit type
const max_retries: Int = 5

// String constant
const app_name = "My Application"

// Using alternate syntax
version :- "1.0.0"

fun area(r: Float) -> Float {
    pi * r * r
}

print(area(2.0))
print(app_name)
```

## Semantics

*   **Immutability**: You cannot assign a new value to a constant.
*   **Single Definition**: A constant can only be defined once. You cannot redefine an existing variable as constant.
*   **Scope**: Constants obey standard scoping rules. Top-level constants are visible throughout the package.
*   **Evaluation**: Constants are evaluated once when the module is loaded.

## Difference from Variables

```rust
// Variable (mutable) — can be reassigned
x = 10
x = 20       // OK
print(x)     // 20

// Constant (immutable) — cannot be reassigned
const y = 10
print(y)     // 10
// y = 20   // Would cause: Error: cannot reassign constant 'y'

// Cannot redefine variable as constant
z = 10
// const z = 20  // Would cause: Error: redefinition of symbol 'z'
```

## Tuple Unpacking

Constants support tuple unpacking:

```rust
const pair = (1, "hello")
const (a, b) = pair      // a = 1, b = "hello"

// Nested unpacking
const nested = ((1, 2), 3)
const ((x, y), z) = nested

// Wildcard for unused parts
const (first, _) = pair  // only binds first
```

## Note on Naming
This is a language restriction, not just a convention: lowercase identifiers are required for variables and constants. Capitalized identifiers are reserved for Types and Constructors.
