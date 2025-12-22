# Type System Deep Dive

This document explains how Funxy's type system works, including type inference, type erasure, and runtime type safety.

## Static Type Inference

Funxy uses **Hindley-Milner type inference** — the compiler deduces types from context without requiring explicit annotations everywhere.

### Basic Inference

```rust
x = 42           // x: Int (inferred from literal)
s = "hello"      // s: String (inferred)
list = [1, 2, 3] // list: List<Int> (inferred from elements)
```

### Polymorphic Values

Some values are **polymorphic** — they can have multiple types depending on context.

```rust
// Zero can be Option<T> for any T
z = Zero         // z: Option<T> (polymorphic)

// Static type is determined by context (at analysis time):
x: Option<Int> = Zero      // Analyzer sees: Zero: Option<Int>
y: Option<String> = Zero   // Analyzer sees: Zero: Option<String>

// But at runtime, type parameter is erased:
print(getType(x))  // type(Option) — not type(Option<Int>)!
print(getType(y))  // type(Option) — same as x
```

### Context-Based Inference

The compiler infers types from:

#### 1. Type Annotations

```rust
// Annotation determines the type parameter
opt: Option<Int> = Zero
print(opt)  // Zero is Option<Int> here
```

#### 2. Function Parameters

```rust
fun process(opt: Option<String>) -> String {
    match opt {
        Some(s) -> s
        Zero -> "empty"
    }
}

process(Zero)  // Zero is Option<String> here
```

#### 3. Return Types

```rust
fun getEmpty() -> Option<Float> {
    Zero  // Zero is Option<Float> here
}
```

#### 4. Usage in Expressions

```rust
// Type inferred from operations
fun example() {
    opt = Some(42)  // opt: Option<Int>

    // Zero must be Option<Int> to compare with opt
    if opt == Zero {
        print("empty")
    }
}
```

### Generic Function Inference

```rust
fun id<T>(x: T) -> T { x }

// T is inferred at each call site:
id(42)       // T = Int
id("hello")  // T = String
id([1, 2])   // T = List<Int>
```

## Type Erasure at Runtime

Funxy uses **type erasure** for generic types — type parameters exist only at **analysis time** and are "erased" at runtime.

**Key distinction:**
- **Analysis time**: Full types like `Option<Int>`, `List<String>` — used for type checking
- **Runtime**: Base types like `Option`, `List` — type parameters gone

### What Type Erasure Means

```rust
// At analysis time:
x: Option<Int> = Some(42)    // Full type: Option<Int>
y: Option<String> = Some("hi")  // Full type: Option<String>

// At runtime:
getType(x)  // type(Option) — parameter erased
getType(y)  // type(Option) — same base type
```

### Why This Matters

```rust
// These are the SAME at runtime:
zero1: Option<Int> = Zero
zero2: Option<String> = Zero

getType(zero1) == getType(zero2)  // true!
```

### Comparison with Other Languages

| Language | Generics |
|----------|----------|
| Funxy, Java, Kotlin | Type Erasure |
| C#, Rust | Reified (preserved at runtime) |

**Type Erasure** means:
- `List<Int>` and `List<String>` are the same type at runtime
- Cannot check `typeOf(list, List(Int))` — only `typeOf(list, List)`
- Simpler implementation, less memory usage

**Reified Generics** means:
- `List<Int>` and `List<String>` are different types at runtime
- Can check full generic type at runtime
- More runtime information, but more complex

## typeOf and getType Functions

### typeOf(value, Type) -> Bool

Checks if value matches a type. For generic types, checks only the **base type**:

```rust
list = [1, 2, 3]

typeOf(list, List)       // true — is it a List?
typeOf(list, Int)        // false — not an Int

opt = Some(42)
typeOf(opt, Option)      // true
```

### getType(value) -> Type

Returns the runtime type representation:

```rust
getType(42)         // type(Int)
getType("hello")    // type((List Char))
getType([1, 2, 3])  // type((List Int))
getType(Some(42))   // type(Option)  — parameter erased!
getType(Zero)       // type(Option)  — same
```

### Comparing Types

```rust
// Same base type:
getType(Some(42)) == getType(Zero)         // true
getType(Some(42)) == getType(Some("hi"))   // true

// Different base types:
getType(Some(42)) == getType(Ok(42))       // false
getType([1]) == getType(%{})               // false

// Nominal Types:
type Point = { x: Int, y: Int }
p: Point = { x: 1, y: 2 }
anon = { x: 1, y: 2 }

getType(p) == getType(anon)                // false (Point vs Record)
```

## Runtime Type Safety

### When Code is Type-Safe

All **statically typed code** is safe — type errors are caught at analysis time:

```rust
fun add(a: Int, b: Int) -> Int { a + b }

add(1, 2)      // ✓ OK
// add("x", "y")  // ✗ Would cause error at ANALYSIS time, not runtime
print("Type-safe code: OK")
```

The analyzer ensures:
- Function arguments match parameter types
- Return values match declared return type
- Pattern matching is exhaustive
- No type mismatches in expressions

### When Runtime Errors Can Occur

Runtime type errors are possible with **dynamic data** — values whose type is not known at compile time:

#### 1. JSON Parsing

The key issue is that `jsonDecode` can return **different types** depending on JSON content:

```rust
import "lib/json" (jsonDecode)

// Same function, different result types:
r1 = jsonDecode("42")           // Ok(42) — Int
r2 = jsonDecode("\"hello\"")    // Ok("hello") — String
r3 = jsonDecode("[1,2,3]")      // Ok([1,2,3]) — List
r4 = jsonDecode("{\"x\":1}")    // Ok({x:1}) — Record

// Analyzer sees: jsonDecode returns Result<String, ?>
// The ? is resolved only at RUNTIME based on JSON content
print(r1)
print(r2)
print(r3)
print(r4)
```

If you try arithmetic on decoded data without checking:

```rust
import "lib/json" (jsonDecode)

fun riskyAdd(json: String) -> Int {
    match jsonDecode(json) {
        Ok(data) -> data + 1  // Risky! data type unknown
        Fail(_) -> 0
    }
}

print(riskyAdd("42"))       // 43 — works
// print(riskyAdd("\"hi\""))  // Would CRASH: type mismatch at runtime
```

#### 2. HTTP Responses

```rust
import "lib/http" (httpGet)

response = httpGet("https://api.example.com/data")
// response body type is unknown
```

#### 3. File Content

```rust
import "lib/io" (fileRead)

content = fileRead("data.txt")?
// content is String, but parsed value type is unknown
```

### Protecting Against Runtime Errors

#### 1. Use typeOf for Runtime Checks

`typeOf` checks the **base type** (Int, String, List, etc.) — this is exactly what you need when `jsonDecode` can return any type:

```rust
import "lib/json" (jsonDecode)

fun safeProcess(json: String) {
    match jsonDecode(json) {
        Ok(data) -> {
            // Check actual runtime type before using
            if typeOf(data, Int) {
                result = data + 1  // Safe — verified it's Int
                print("Number + 1 = " ++ show(result))
            } else if typeOf(data, List) {
                print("List with " ++ show(len(data)) ++ " items")
            } else if typeOf(data, String) {
                print("String: " ++ data)
            } else {
                print("Other type: " ++ show(getType(data)))
            }
        }
        Fail(e) -> print("JSON error: " ++ e)
    }
}

safeProcess("42")          // Number + 1 = 43
safeProcess("[1,2,3]")     // List with 3 items
safeProcess("\"hello\"")   // String: hello
```

#### 2. Use Pattern Matching with Type Patterns

```rust
import "lib/json" (jsonDecode)

input = "[1, 2, 3]"  // JSON array
match jsonDecode(input) {
    Ok(data) -> {
        match data {
            n: Int -> print("Number: " ++ show(n))
            s: String -> print("String: " ++ s)
            xs: List -> print("List of " ++ show(len(xs)) ++ " items")
            _ -> print("Other type")
        }
    }
    Fail(e) -> print(e)
}
```

#### 3. Use Json ADT for Structured Access

```rust
import "lib/json" (Json, jsonParse, jsonGet)

input = "{\"name\":\"Alice\",\"age\":30}"
match jsonParse(input) {
    Ok(obj) -> {
        match jsonGet(obj, "age") {
            Some(JNum(age)) -> print("Age: " ++ show(age))
            Some(_) -> print("age is not a number")
            Zero -> print("no age field")
        }
    }
    Fail(e) -> print(e)
}
```

#### 4. Define Expected Types

```rust
import "lib/json" (jsonDecode)

type User = { name: String, age: Int }

fun parseUser(json: String) -> Result<String, User> {
    data = jsonDecode(json)?

    // Validate structure
    if !typeOf(data.name, String) {
        Fail("name must be string")
    } else if !typeOf(data.age, Int) {
        Fail("age must be int")
    } else {
        Ok(data)
    }
}
```

## Type Inference for ADTs

### Polymorphic Constructors

ADT constructors without data are polymorphic:

```rust
// Option is built-in: type Option<T> = Some T | Zero

// Zero has no data, so T is determined by context
z1: Option<Int> = Zero     // T = Int
z2: Option<String> = Zero  // T = String

// Some has data, so T is inferred from it
s1 = Some(42)      // T = Int (from 42)
s2 = Some("hi")    // T = String (from "hi")

print("z1: " ++ show(z1))  // Zero
print("s1: " ++ show(s1))  // Some(42)
```

### Result Type Inference

```rust
// Result is built-in: type Result<E, A> = Ok A | Fail E

// E and A inferred from data:
ok1 = Ok(42)           // Result<E, Int>
ok2 = Ok("success")    // Result<E, String>

fail1 = Fail("error")  // Result<String, A>
fail2 = Fail(404)      // Result<Int, A>

print("ok1: " ++ show(ok1))      // Ok(42)
print("fail1: " ++ show(fail1))  // Fail("error")
```

### Full Type from Context

```rust
fun divide(a: Int, b: Int) -> Result<String, Int> {
    if b == 0 {
        Fail("division by zero")  // Result<String, Int>
    } else {
        Ok(a / b)                 // Result<String, Int>
    }
}
```

## Summary

| Aspect | Static (Analysis) | Runtime |
|--------|-------------------|---------|
| Type Parameters | Full type `Option<Int>` | Erased to `Option` |
| Type Checking | All code verified | Only dynamic data needs checks |
| Polymorphic Values | Resolved from context | Single representation |
| Type Errors | Compile-time errors | Only with dynamic data |

### Best Practices

1. **Trust static types** — if code passes analysis, typed operations are safe

2. **Always validate dynamic data** — JSON returns different types based on content

3. **Use typeOf for runtime checks** — checks base type (Int, String, List), useful when `jsonDecode` can return any type

4. **Prefer Json ADT** — for structured access to unknown JSON

5. **Define validation functions** — encapsulate type checking logic

6. **Use Result for errors** — don't rely on type errors at runtime

