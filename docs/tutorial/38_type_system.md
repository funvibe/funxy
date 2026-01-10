# Type System Deep Dive

This document explains how Funxy's type system works, including type inference, type erasure, runtime type safety, and advanced features like MPTC.

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
// Zero can be Option<t> for any t
z = Zero         // z: Option<t> (polymorphic)

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
fun id<t>(x: t) -> t { x }

// t is inferred at each call site:
id(42)       // t = Int
id("hello")  // t = String
id([1, 2])   // t = List<Int>
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

## Multi-Parameter Type Classes (MPTC)

Funxy supports type classes with multiple parameters, allowing you to define relationships between types.

### Definition and Instance

```rust
// Define a trait with two parameters
trait Convert<a, b> {
    fun convert(val: a) -> b
}

// Implement for specific pairs
instance Convert<Int, String> {
    fun convert(val: Int) -> String { show(val) }
}

instance Convert<Bool, Int> {
    fun convert(val: Bool) -> Int { if val { 1 } else { 0 } }
}
```

### Runtime Dispatch

Even with type erasure, MPTC dispatch works correctly in Funxy by checking the runtime types of all arguments.

```rust
// Dispatch based on argument type (Int -> String)
s: String = convert(42)

// Dispatch based on different argument type (Bool -> Int)
i: Int = convert(true)
```

In the VM backend, dispatch uses a fuzzy lookup strategy to find the correct instance even when partial type information is available, handling collisions correctly (e.g. `Convert<Int, String>` vs `Convert<Int, Bool>`).

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

### Strict Mode

By default, Funxy allows some "unsafe" operations for convenience, such as implicitly downcasting a Union type to one of its member types (e.g. passing `Int | String` to a function expecting `Int`).

To enforce stricter type safety, you can enable **Strict Mode** using a directive at the top of your file:

```rust
directive "strict_types"

type MyUnion = Int | String

fun takeInt(x: Int) { ... }

u: MyUnion = 10
// takeInt(u)  // Compile Error in Strict Mode!
```

In strict mode, you must explicitly match on the union or cast it before using it as a specific member type.

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
            xs: List<Int> -> print("List of " ++ show(len(xs)) ++ " items")
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
        Fail("age must be Int")
    } else {
        Ok(data)
    }
}
```

## Type Inference for ADTs

### Polymorphic Constructors

ADT constructors without data are polymorphic:

```rust
// Option is built-in: type Option<t> = Some t | Zero

// Zero has no data, so t is determined by context
z1: Option<Int> = Zero     // t = Int
z2: Option<String> = Zero  // t = String

// Some has data, so t is inferred from it
s1 = Some(42)      // t = Int (from 42)
s2 = Some("hi")    // t = String (from "hi")

print("z1: " ++ show(z1))  // Zero
print("s1: " ++ show(s1))  // Some(42)
```

### Result Type Inference

```rust
// Result is built-in: type Result<e, a> = Ok a | Fail e

// e and a inferred from data:
ok1 = Ok(42)           // Result<e, Int>
ok2 = Ok("success")    // Result<e, String>

fail1 = Fail("error")  // Result<String, a>
fail2 = Fail(404)      // Result<Int, a>

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

## Let Polymorphism & Value Restriction

Funxy implements **let-polymorphism** for local definitions, allowing them to be generalized to polymorphic types:

```rust
// Local definition can be polymorphic
fun example() {
    id = fun(x) { x }  // id: t -> t (polymorphic)

    id(42)      // t = Int
    id("hi")    // t = String
    id([1, 2])  // t = List<Int>
}
```

However, **value restriction** prevents generalization of non-expansive expressions to maintain type soundness:

```rust
// Cannot generalize mutable references
fun example() {
    // This would be unsafe if generalized:
    // ref = ref(5)  // Cannot be polymorphic
}
```

This ensures that side-effecting constructs maintain correct type constraints.

## Runtime Witness Passing

For Higher-Kinded Types (HKT) and generic trait methods, Funxy uses **witness passing** (dictionary passing) at runtime:

### How It Works

1. **Analysis Phase:** The analyzer ensures trait instances exist and records witness requirements.
2. **Runtime Phase:** The `Evaluator` maintains a `WitnessStack` that stores type dictionaries for generic calls.
3. **Dispatch:** When calling generic methods like `pure`, the evaluator pushes the appropriate witness (e.g., `Applicative -> OptionT`) onto the stack.
4. **Consumption:** Built-in trait methods (e.g., `OptionT.pure`) read from the witness stack to determine inner monad types.

### Example

```rust
// Annotation provides witness context
f = fun() -> OptionT<Identity, Int> {
    pure(200)  // Uses witness from return type annotation
}

val = runIdentity(runOptionT(f()))
```

### Context Isolation

Monad transformers (`OptionT`, `ResultT`) require careful context isolation to prevent witness pollution:

- Before calling inner monad operations, transformers save and clear `WitnessStack`, `CurrentCallNode`, and `ContainerContext`.
- This ensures inner calls (e.g., `fmap` on `Result`) use their own witness context, not the outer transformer's context.
- After the inner call returns, the saved context is restored.

This prevents outer transformer witnesses from incorrectly affecting inner monad dispatch.

## Summary

| Aspect | Static (Analysis) | Runtime |
|--------|-------------------|---------|
| Type Parameters | Full type `Option<Int>` | Erased to `Option` |
| Type Checking | All code verified | Only dynamic data needs checks |
| Polymorphic Values | Resolved from context | Single representation |
| Type Errors | Compile-time errors | Only with dynamic data |
| Let Polymorphism | Local definitions generalized | N/A (compile-time only) |
| Witness Passing | Witnesses prepared | `WitnessStack` passes dictionaries |
| MPTC Dispatch | Verified by solver | Fuzzy lookup by arg types |

### Best Practices

1. **Trust static types** — if code passes analysis, typed operations are safe

2. **Always validate dynamic data** — JSON returns different types based on content

3. **Use typeOf for runtime checks** — checks base type (Int, String, List), useful when `jsonDecode` can return any type

4. **Prefer Json ADT** — for structured access to unknown JSON

5. **Define validation functions** — encapsulate type checking logic

6. **Use Result for errors** — don't rely on type errors at runtime
