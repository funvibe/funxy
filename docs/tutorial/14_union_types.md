# Union Types

Union types allow a variable to hold values of multiple different types. They are useful for representing values that can be one of several possibilities.

## Basic Syntax

Use `|` to create a union of types:

```rust
// Variable can hold Int or String
x: Int | String = 42
print(x)    // 42

x = "hello"
print(x)    // hello
```

## Nullable Types

The `T?` shorthand creates a union of `T | Nil`:

```rust
// name can be String or Nil
name: String? = "Alice"
print(name)   // Alice

name = Nil
print(name)   // Nil
```

This is equivalent to:

```rust
name: String | Nil = "Alice"
```

## Union Type Functions

### As Parameters

```rust
fun describe(val: Int | String) -> String {
    match val {
        n: Int -> "Integer: " ++ show(n)
        s: String -> "String: " ++ s
    }
}

print(describe(42))      // Integer: 42
print(describe("test"))  // String: test
```

### As Return Type

```rust
fun maybeInt(b: Bool) -> Int | Nil {
    if b { 100 } else { Nil }
}

print(maybeInt(true))   // 100
print(maybeInt(false))  // Nil
```

## Type Inference from Branches

The compiler automatically infers union types when `if` or `match` branches return different types:

```rust
fun autoInfer(b: Bool) {
    // result has type Int | String
    result = if b { 42 } else { "forty-two" }
    print(result)
}
```

## Pattern Matching with Type Patterns

Use type patterns (`name: Type`) in match expressions to match and bind values by their type:

```rust
fun process(x: Int | String | Nil) -> String {
    match x {
        n: Int -> "Got Int: " ++ show(n)
        s: String -> "Got string: " ++ s
        _: Nil -> "Got nil"
    }
}

print(process(100))   // Got Int: 100
print(process("abc")) // Got string: abc
print(process(Nil))   // Got nil
```

### Type Pattern Syntax

- `n: Int` - Matches if value is Int, binds to `n`
- `s: String` - Matches if value is String, binds to `s`
- `_: Nil` - Matches if value is Nil, doesn't bind (ignored)

## Exhaustiveness Checking

The compiler ensures all union members are covered in match expressions:

```rust
// This would cause a compile error:
// fun incomplete(x: Int | String) -> String {
//     match x {
//         n: Int -> show(n)
//         // Error: missing case for String!
//     }
// }

// Correct version - all cases covered:
fun complete(x: Int | String) -> String {
    match x {
        n: Int -> show(n)
        s: String -> s
    }
}

print(complete(42))      // "42"
print(complete("hello")) // "hello"
```

## Runtime Type Checking

Use `typeOf` to check the runtime type:

```rust
fun typeCheck(x: Int | String) -> String {
    if typeOf(x, Int) {
        "It's an integer"
    } else {
        "It's a string"
    }
}
```

## Union with Complex Types

Unions work with any types, including generics:

```rust
// List or Nil
listOrNil: List<Int> | Nil = [1, 2, 3]
print(listOrNil)  // [1, 2, 3]

listOrNil = Nil
print(listOrNil)  // Nil
```

## Normalization

Union types are automatically normalized:
- Duplicates are removed: `Int | Int` becomes `Int`
- Nested unions are flattened: `(Int | String) | Bool` becomes `Int | String | Bool`
- Members are sorted alphabetically for consistency

## Comparison with Option

Union types and `Option<T>` are different:

```rust
// Option<T> is an ADT with Some(T) and Zero constructors
opt: Option<Int> = Some(42)

// T? is a union type (T | Nil)
nullable: Int? = 42
```

They are not compatible with each other:

```rust
// This will NOT compile:
// opt: Option<Int> = Nil  // Error: Nil is not Zero

// This works:
opt: Option<Int> = Zero
nullable: Int? = Nil
```

## Strict Mode and Safety

By default, Funxy allows "unsafe" downcasting from a union type to a member type for convenience:

```rust
type alias NumOrStr = Int | String

fun takeInt(i: Int) { ... }

val: NumOrStr = 42
takeInt(val)  // Allowed by default (unsafe)
```

However, this can lead to runtime errors if `val` happens to be a `String`.

To prevent this, you can enable **Strict Mode** by adding `directive "strict_types"` at the top of your file. In strict mode, implicit downcasting is disallowed, and you must use pattern matching to safely extract the value.

```rust
directive "strict_types"

type alias NumOrStr = Int | String

fun takeInt(i: Int) { ... }

val: NumOrStr = 42
// takeInt(val)  // Compile Error: cannot implicitly downcast

// Must use match:
match val {
    i: Int -> takeInt(i)
    _ -> print("Not an int")
}
```

## Best Practices

1. **Use `T?` for nullable values**: It's cleaner than `T | Nil`

2. **Prefer ADTs for complex unions**: If you have more than 2-3 types in a union, consider creating an ADT

3. **Always handle all cases**: The exhaustiveness checker ensures you don't miss any type

4. **Use type patterns in match**: They make code clearer than runtime checks

