# 8. Error Handling

[← Back to Index](./00_index.md)

## Panic (Unrecoverable Errors)

Use `panic` for errors that should stop execution immediately.

```rust
fun divide(a: Int, b: Int) -> Int {
    if b == 0 {
        panic("division by zero")
    }
    a / b
}
```

## Option (Absence of Value)

Used when a value might not exist.

```rust
// Option<t> = Some(t) | None

import "lib/list" (*)

fun findItem(predicate: (Int) -> Bool, list: List<Int>) -> Option<Int> {
    match list {
        [] -> None
        [x, ...xs] -> if predicate(x) { Some(x) } else { findItem(predicate, xs) }
    }
}

// Usage
numbers = [1, 2, 3, 4, 11]
match findItem(fun(x) -> x > 10, numbers) {
    Some(n) -> print("Found: " ++ show(n))
    None -> print("Not found")
}
```

## Result (Recoverable Errors)

Used when an operation can fail with an error message.

```rust
// Result<e, t> = Ok(t) | Fail(e)

fun parseNumber(s: String) -> Result<String, Int> {
    match read(s, Int) {
        Some(n) -> Ok(n)
        None -> Fail("Invalid number: " ++ s)
    }
}

// Usage
match parseNumber("42") {
    Ok(n) -> print("Parsed: " ++ show(n))
    Fail(err) -> print("Error: " ++ err)
}
```

## Error Propagation Operator (`?`)

The `?` operator unwraps `Ok` values or returns early on `Fail`.

```rust
import "lib/io" (fileRead, fileWrite)
import "lib/list" (head)

fun copyFile(src: String, dst: String) -> Result<String, Int> {
    content = fileRead(src)?
    fileWrite(dst, content) // returns Result<String, Int>
}

// Works with Option too
fun processFirst(list: List<Int>) -> Option<Int> {
    match list {
        [x, ..._] -> Some(x * 2)
        [] -> None
    }
}
```

## Nullable Types (`T?`)

Types ending in `?` are shorthand for `T | Nil`.

```rust
type alias User = { address: Option<{ city: String }> }
user: User = { address: None }

name: Option<String> = Some("Alice")
name = None

// Optional chaining (works on types implementing Optional trait)
// user?.address?.city

// Null coalescing
// name ?? "Anonymous"
```

## Helper Functions

### Option helpers

```text
// Built-in functions
isSome(Some(1))           // true
isNone(None)              // true
unwrap(Some(42))          // 42 (panics if None)
unwrapOr(None, 0)         // 0
unwrapOrElse(None, fun() -> computeDefault())
```

### Result helpers

```text
// Built-in functions
isOk(Ok(1))               // true
isFail(Fail("err"))       // true
unwrapResult(Ok(42))      // 42 (panics if Fail)
unwrapResultOr(Fail("x"), 0)  // 0
unwrapError(Fail("err"))  // "err" (panics if Ok)
```

## Combining with Do Notation

```rust
type alias User = { id: Int, profileId: Int, profile: String }
fun fetchUser(id: Int) -> Result<String, User> { Ok({ id: id, profileId: 1, profile: "" }) }
fun fetchProfile(id: Int) -> Result<String, String> { Ok("Admin") }

fun processUser(id: Int) -> Result<String, User> {
    match fetchUser(id) {
        Ok(user) -> match fetchProfile(user.profileId) {
            Ok(profile) -> Ok({ ...user, profile: profile })
            Fail(e) -> Fail(e)
        }
        Fail(e) -> Fail(e)
    }
}
```
