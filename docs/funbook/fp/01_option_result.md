# 01. Option and Result

## Task
Handle missing values and errors without null/exceptions.

## Option: may have a value, may not have

Option is a built-in type: `Some T | Zero`.

```rust
fun safeDivide(a: Int, b: Int) -> Option<Int> {
    if b == 0 { Zero } else { Some(a / b) }
}

print(safeDivide(10, 2))  // Some(5)
print(safeDivide(10, 0))  // Zero
```

## Handling Option

```rust
fun showResult(opt: Option<Int>) -> String {
    match opt {
        Some(x) -> "Got: " ++ show(x)
        Zero -> "Nothing"
    }
}

print(showResult(Some(42)))  // Got: 42
print(showResult(Zero))      // Nothing
```

## Quick check: `T?` syntax

```rust
// T? is equivalent to T | Nil
type User = { name: String, id: Int }

fun findUser(id: Int) -> User? {
    if id > 0 { { name: "Alice", id: id } } else { Nil }
}

user = findUser(1)
match user {
    _: Nil -> print("Not found")
    u: User -> print("Found: " ++ u.name)
}
```

## Result: success or error with information

Result is a built-in type: `Ok A | Fail E`.

```rust
fun parseNumber(s: String) -> Result<String, Int> {
    match read(s, Int) {
        Some(n) -> Ok(n)
        Zero -> Fail("Cannot parse: " ++ s)
    }
}

print(parseNumber("42"))     // Ok(42)
print(parseNumber("hello"))  // Fail("Cannot parse: hello")
```

## Handling Result

```rust
fun handleResult(r: Result<String, Int>) -> String {
    match r {
        Ok(value) -> "Success: " ++ show(value)
        Fail(error) -> "Error: " ++ error
    }
}

print(handleResult(Ok(100)))          // Success: 100
print(handleResult(Fail("oops")))     // Error: oops
```

## Functions for working with Option

```rust
// Apply function to value inside Option
fun mapOption(opt, f) {
    match opt {
        Some(x) -> Some(f(x))
        Zero -> Zero
    }
}

// Default value
fun getOrElse(opt, default) {
    match opt {
        Some(x) -> x
        Zero -> default
    }
}

// Examples
x = Some(10)
doubled = mapOption(x, fun(n) -> n * 2)  // Some(20)
print(doubled)

value = getOrElse(Zero, 42)  // 42
print(value)
```

## Chain of operations (flatMap)

```rust
// Chain of operations with Option
fun flatMapOption(opt, f) {
    match opt {
        Some(x) -> f(x)
        Zero -> Zero
    }
}

// Example: safe division with chain
fun safeDivide(a: Int, b: Int) -> Option<Int> {
    if b == 0 { Zero } else { Some(a / b) }
}

result = Some(100)
    |> fun(opt) -> flatMapOption(opt, fun(x) -> safeDivide(x, 2))
    |> fun(opt) -> flatMapOption(opt, fun(x) -> safeDivide(x, 5))

print(result)  // Some(10)
```

## Practical example: form validation

```rust
import "lib/list" (contains)

type ValidationError = EmptyField(String)
                     | TooShort(String)
                     | InvalidEmail(String)

type User = { name: String, email: String }

fun validateName(name: String) -> Result<ValidationError, String> {
    if len(name) == 0 { Fail(EmptyField("name")) }
    else if len(name) < 2 { Fail(TooShort("name")) }
    else { Ok(name) }
}

fun validateEmail(email: String) -> Result<ValidationError, String> {
    if !contains(email, '@') { Fail(InvalidEmail(email)) }
    else { Ok(email) }
}

fun validateUser(name: String, email: String) -> Result<ValidationError, User> {
    match (validateName(name), validateEmail(email)) {
        (Ok(n), Ok(e)) -> Ok({ name: n, email: e })
        (Fail(err), _) -> Fail(err)
        (_, Fail(err)) -> Fail(err)
    }
}

// Usage
match validateUser("Alice", "alice@example.com") {
    Ok(user) -> print("Valid: " ++ user.name)
    Fail(EmptyField(f)) -> print("Field " ++ f ++ " is empty")
    Fail(TooShort(f)) -> print(f ++ " is too short")
    Fail(InvalidEmail(e)) -> print("Invalid email: " ++ e)
}
```

## Combining multiple Results

```rust
fun sequence(results) {
    match results {
        [] -> Ok([])
        [Ok(x), rest...] -> match sequence(rest) {
            Ok(xs) -> Ok([x] ++ xs)
            Fail(e) -> Fail(e)
        }
        [Fail(e), _...] -> Fail(e)
    }
}

results = [Ok(1), Ok(2), Ok(3)]
print(sequence(results))  // Ok([1, 2, 3])

withError = [Ok(1), Fail("oops"), Ok(3)]
print(sequence(withError))  // Fail("oops")
```

## When to use what?

| Situation | Type |
|----------|-----|
| Value may be missing | `Option<T>` or `T?` |
| Need error information | `Result<E, A>` |
| Quick null-check | `T?` (T \| Nil) |
| Complex error handling | `Result<E, A>` |

## Converting between types

```rust
fun optionToResult(opt, error) {
    match opt {
        Some(x) -> Ok(x)
        Zero -> Fail(error)
    }
}

fun resultToOption(res) {
    match res {
        Ok(x) -> Some(x)
        Fail(_) -> Zero
    }
}

// Examples
print(optionToResult(Some(42), "missing"))  // Ok(42)
print(optionToResult(Zero, "missing"))      // Fail("missing")

print(resultToOption(Ok(42)))       // Some(42)
print(resultToOption(Fail("err")))  // Zero
```
