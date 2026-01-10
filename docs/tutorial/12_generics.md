# Iteration 5: Generics

Generics allow you to write code that works with different types, increasing flexibility and type safety.

## Naming Convention

**Important**: Type parameters must start with a **lowercase letter**.

```rust
// ✓ t, u are lowercase - correct
fun swap<t, u>(pair: (t, u)) -> (u, t) {
    (a, b) = pair
    (b, a)
}

// ✗ Uppercase type parameters would be an error:
// fun swap<T, U>(pair: (T, U)) -> (U, T) { ... }
print(swap((1, "hello")))  // ("hello", 1)
```

This follows the language-wide convention:
- **Uppercase**: types, constructors, traits (`Int`, `Some`, `Order`)
- **Lowercase**: values, functions, variables, type parameters (`myVar`, `calculate`, `x`, `t`)

## Generic Functions

You can define functions that accept type parameters using angle brackets `<t>`.

```rust
// Identity function working on any type t
fun id<t>(x: t) -> t {
    x
}

n = id(42)       // t is Int
s = id("hello")  // t is String
print(n)         // 42
print(s)         // "hello"
```

## Generic Types

Type declarations can also be generic. Type parameters are listed in angle brackets after the type name.

```rust
// A simple wrapper type
type Box<t> = { value: t }

b = { value: 10 }  // Box<Int>
print(b.value)     // 10
```

## Type Inference

The type system infers concrete types at call sites:

```rust
fun id<t>(x: t) -> t { x }

id(42)      // t inferred as Int
id("hello") // t inferred as String
```

