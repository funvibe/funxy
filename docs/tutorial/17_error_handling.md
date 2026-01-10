# Error Handling

The language provides three approaches to error handling:

1. **`panic`** — unrecoverable errors, stops execution immediately
2. **`Result<E, A>`** — recoverable errors with error information (error type first)
3. **`Option<T>`** — handling absent values (not really an error)

## 1. Panic (Unrecoverable Errors)

Use `panic` when an error is **unrecoverable** — the program cannot continue.

```rust
fun myHead<t>(xs: List<t>) -> t {
    match xs {
        [x, ..._] -> x
        [] -> panic("myHead: empty list")
    }
}

print(myHead([1, 2, 3]))  // 1
// print(myHead([]))      // PANIC: myHead: empty list
// Program would stop here, no further code runs
```

### When to Use Panic

- **Programming errors**: bugs that should never happen in correct code
- **Invariant violations**: impossible states
- **Unrecoverable situations**: missing critical resources

```rust
fun factorial(n: Int) -> Int {
    if n < 0 {
        panic("factorial: negative input")  // Bug in caller's code
    } else if n == 0 {
        1
    } else {
        n * factorial(n - 1)
    }
}
```

### Panic vs Result

| Use `panic` | Use `Result` |
|-------------|--------------|
| Bug in code | Expected failure |
| Impossible state | File not found |
| Assertion failed | Network error |
| Critical invariant violated | User input invalid |

## 2. Result Type (Recoverable Errors)

`Result<E, A>` represents an operation that may fail (error type first):

```
type Result<E, A> = Ok A | Fail E
```

- `Ok(value)` — success, contains result of type `A`
- `Fail(error)` — failure, contains error of type `E`

### Creating Results

```rust
// Success
x = Ok(42)               // Result<E, Int>
y = Ok("hello")          // Result<E, String>

// Failure
e = Fail("not found")    // Result<String, A>
e2 = Fail(404)           // Result<Int, A>
```

### Pattern Matching on Result

```rust
fun divide(a: Int, b: Int) -> Result<String, Int> {
    if b == 0 {
        Fail("division by zero")
    } else {
        Ok(a / b)
    }
}

match divide(10, 2) {
    Ok(value) -> print("Result: " ++ show(value))
    Fail(err) -> print("Error: " ++ err)
}
```

### The `?` Operator (Error Propagation)

The `?` operator provides automatic unwrapping with early return:

- **`Ok(value)?`** → returns `value`
- **`Fail(error)?`** → immediately returns `Fail(error)` from current function

```rust
import "lib/io" (fileRead, fileWrite)

fun copyFile(src: String, dst: String) -> Result<String, Int> {
    content = fileRead(src)?     // Fail → early return, Ok → unwrap
    bytes = fileWrite(dst, content)?
    Ok(bytes)
}
```

This is equivalent to:

```rust
import "lib/io" (fileRead, fileWrite)

fun copyFile(src: String, dst: String) -> Result<String, Int> {
    match fileRead(src) {
        Fail(e) -> Fail(e)       // Early return
        Ok(content) -> {
            match fileWrite(dst, content) {
                Fail(e) -> Fail(e)
                Ok(bytes) -> Ok(bytes)
            }
        }
    }
}
```

### Chaining Multiple `?` Operations

```rust
import "lib/io" (fileRead)

fun validate(n: Int) -> Result<String, Int> {
    if n > 0 { Ok(n) } else { Fail("must be positive") }
}

fun processData(path: String) -> Result<String, Int> {
    content = fileRead(path)?        // Step 1
    number = read(content, Int)?     // Step 2
    validated = validate(number)?    // Step 3
    Ok(validated * 2)
}
// If ANY step fails, function returns Fail immediately
```

### Important: Return Type Constraint

`?` only works inside functions with compatible return type:

```rust
import "lib/io" (fileRead)

// ✅ Works: returns Result<..., String>
fun loadFile(path: String) -> Result<String, String> {
    content = fileRead(path)?
    Ok(content)
}

// badExample would not compile: returns Int, not Result
// fun badExample(path: String) -> Int {
//     content = fileRead(path)?   // Where would Fail go?
//     len(content)
// }
```

## 3. Option Type (Absent Values)

`Option<T>` represents a value that may or may not exist:

```
type Option<T> = Some T | Zero
```

- `Some(value)` — contains a value
- `Zero` — no value (like `null`/`None`, but type-safe)

### Creating Options

```rust
x = Some(42)     // Option<Int>
y = Some("hi")   // Option<String>
z = Zero         // Option<T>
```

## 4. Nullable Types (`T?` syntax)

As a convenient shorthand, you can use `T?` to represent a **nullable type** (union of `T` and `Nil`):

```rust
// T? is syntactic sugar for T | Nil
age: Int? = 25
name: String? = nil
```

This is **different from `Option<T>`**:

| Feature | `Option<T>` | `T?` (Nullable) |
|---------|-------------|-----------------|
| Type | Sum type (ADT) | Union type |
| Values | `Some(x)` / `Zero` | `x` / `nil` |
| Pattern | `Some(v) -> ...` | `v: T -> ...` |
| Use case | Explicit optionality | Lightweight nullable |

### Creating Nullable Values

```rust
// Direct values or nil
a: Int? = 42
b: Int? = nil

print(a)   // 42
print(b)   // Nil
```

### Pattern Matching on Nullable Types

Use type annotations to distinguish between the value and `nil`:

```rust
fun describe(x: Int?) -> String {
    match x {
        n: Int -> "Got number: " ++ show(n)
        _: Nil -> "Got nil"
    }
}

print(describe(42))   // Got number: 42
print(describe(nil))  // Got nil
```

### Nullable Functions

```rust
fun safeDivide(a: Int, b: Int) -> Int? {
    if b == 0 { nil } else { a / b }
}

match safeDivide(10, 2) {
    result: Int -> print("Result: " ++ show(result))
    _: Nil -> print("Division by zero")
}
// Output: Result: 5
```

### When to Use `T?` vs `Option<T>`

| Use `T?` | Use `Option<T>` |
|----------|-----------------|
| Simple nullability | Explicit presence/absence semantics |
| Interop with nullable data | Functional programming patterns |
| Lightweight syntax | `fmap`, Monad operations |

### Pattern Matching on Option

```rust
fun findFirst<t>(xs: List<t>, pred: (t) -> Bool) -> Option<t> {
    match xs {
        [] -> Zero
        [x, ...rest] -> if pred(x) { Some(x) } else { findFirst(rest, pred) }
    }
}

match findFirst([1, 2, 3], fun(x) -> x > 2) {
    Some(value) -> print("Found: " ++ show(value))
    Zero -> print("Not found")
}
```

### The `?` Operator with Option

Works the same as with Result:

- **`Some(value)?`** → returns `value`
- **`Zero?`** → immediately returns `Zero` from current function

```rust
import "lib/list" (find)

fun getFirstPositive(xs: List<Int>) -> Option<Int> {
    first = find(fun(x) -> x > 0, xs)?
    Some(first * 2)
}

print(getFirstPositive([-1, 2, 3]))  // Some(4)
print(getFirstPositive([-1, -2]))    // Zero
```

### Option vs Result

| Use `Option` | Use `Result` |
|--------------|--------------|
| Value might not exist | Operation can fail |
| No error info needed | Need error details |
| `find`, `head`, lookup | File I/O, parsing, network |

## Practical Examples

### Example 1: Safe Division with Result

```rust
fun safeDiv(a: Int, b: Int) -> Result<String, Int> {
    if b == 0 { Fail("division by zero") } else { Ok(a / b) }
}

fun calculate(x: Int, y: Int, z: Int) -> Result<String, Int> {
    r1 = safeDiv(x, y)?
    r2 = safeDiv(r1, z)?
    Ok(r2 + 1)
}

print(calculate(100, 5, 2))   // Ok(11)
print(calculate(100, 0, 2))   // Fail("division by zero")
```

### Example 2: File Processing Pipeline

```rust
import "lib/io" (fileRead, fileWrite)
import "lib/string" (stringToUpper)

fun processFile(input: String, output: String) -> Result<String, Int> {
    content = fileRead(input)?
    processed = stringToUpper(content)
    bytes = fileWrite(output, processed)?
    Ok(bytes)
}

match processFile("in.txt", "out.txt") {
    Ok(bytes) -> print("Wrote ${bytes} bytes")
    Fail(err) -> print("Error: ${err}")
}
```

### Example 3: Combining Option and Panic

```rust
import "lib/list" (head, find)

// Safe: returns Option
maybeFirst = find(fun(x) -> x > 10, [1, 2, 3])
match maybeFirst {
    Some(x) -> print(x)
    Zero -> print("not found")
}

// Unsafe: panics on empty list
first = head([1, 2, 3])   // Some(1)
// head([])               // Would PANIC!
```

## Summary

| Mechanism | When to Use | Recovery |
|-----------|-------------|----------|
| `panic(msg)` | Bugs, impossible states | None (program stops) |
| `Result<E, A>` | Expected failures | Yes (handle `Fail`) |
| `Option<T>` | Absent values (explicit) | Yes (handle `Zero`) |
| `T?` | Nullable values (lightweight) | Yes (handle `nil`) |

| Type | Success | Failure | `?` on Success | `?` on Failure |
|------|---------|---------|----------------|----------------|
| `Result<E, A>` | `Ok(value)` | `Fail(error)` | Returns `value` | Returns `Fail(error)` |
| `Option<T>` | `Some(value)` | `Zero` | Returns `value` | Returns `Zero` |

**Guidelines:**
- Use `panic` for programming errors that should never happen
- Use `Result` when you need error information and recovery
- Use `Option<T>` when explicit optionality with ADT semantics is needed
- Use `T?` for simple nullable values (lightweight syntax)
- Use `?` to propagate errors cleanly without nested `match`
