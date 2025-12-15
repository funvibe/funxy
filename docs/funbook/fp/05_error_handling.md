# 05. Error Handling

## Task
Handle errors and missing values in a type-safe way, without exceptions.

---

## Three approaches to errors

| Type | When to use | Example |
|-----|-------------------|--------|
| `Option<T>` | Value may be missing | `find(predicate, list)` |
| `T?` (Nullable) | Quick null check | `user.email?` |
| `Result<E, A>` | Need error information | `fileRead(path)` |

---

## Option<T>: exists or not

Option is a built-in type: `Some T | Zero`.

```rust
fun safeDivide(a: Int, b: Int) -> Option<Int> {
    if b == 0 { Zero } else { Some(a / b) }
}

print(safeDivide(10, 2))  // Some(5)
print(safeDivide(10, 0))  // Zero
```

### Pattern Matching with Option

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

### Functions for working with Option

```rust
// map - apply function to value inside
fun mapOption(opt, f) {
    match opt {
        Some(x) -> Some(f(x))
        Zero -> Zero
    }
}

// flatMap - for chains of operations
fun flatMapOption(opt, f) {
    match opt {
        Some(x) -> f(x)
        Zero -> Zero
    }
}

// getOrElse - default value
fun getOrElse(opt, default) {
    match opt {
        Some(x) -> x
        Zero -> default
    }
}

// orElse - alternative Option
fun orElse(opt, alternative) {
    match opt {
        Some(x) -> Some(x)
        Zero -> alternative
    }
}

// Usage examples
x = Some(10)
doubled = mapOption(x, fun(n) -> n * 2)
print(doubled)  // Some(20)

value = getOrElse(Zero, 42)
print(value)  // 42

backup = orElse(Zero, Some(100))
print(backup)  // Some(100)
```

---

## T? (Nullable): quick null-check

`T?` is syntactic sugar for `T | Nil`.

```rust
type User = { name: String, id: Int }

// Function can return Nil
fun findUser(id: Int) -> User? {
    if id > 0 { { name: "Alice", id: id } } else { Nil }
}

user = findUser(1)

// Pattern matching with types
match user {
    _: Nil -> print("Not found")
    u: User -> print("Found: " ++ u.name)
}
```

### Nullable fields in records

```rust
type Profile = {
    name: String,
    email: String?,
    phone: String?,
    age: Int
}

fun showProfile(p: Profile) {
    print("Name: " ++ p.name)
    
    match p.email {
        _: Nil -> print("Email: not provided")
        e: String -> print("Email: " ++ e)
    }
    
    match p.phone {
        _: Nil -> print("Phone: not provided")
        ph: String -> print("Phone: " ++ ph)
    }
}

profile = { name: "Alice", email: "alice@example.com", phone: Nil, age: 30 }
showProfile(profile)
```

---

## Result<E, A>: success or error with information

Result is a built-in type: `Ok A | Fail E`.

```rust
fun parseNumber(s: String) -> Result<String, Int> {
    match read(s, Int) {
        Some(n) -> Ok(n)
        Zero -> Fail("Cannot parse '" ++ s ++ "' as number")
    }
}

print(parseNumber("42"))     // Ok(42)
print(parseNumber("hello"))  // Fail("Cannot parse 'hello' as number")
```

### Pattern Matching with Result

```rust
fun handleResult(r: Result<String, Int>) -> String {
    match r {
        Ok(value) -> "Success: " ++ show(value)
        Fail(error) -> "Error: " ++ error
    }
}

print(handleResult(Ok(100)))        // Success: 100
print(handleResult(Fail("oops")))   // Error: oops
```

### Functions for working with Result

```rust
// map - apply function to successful value
fun mapResult(r, f) {
    match r {
        Ok(a) -> Ok(f(a))
        Fail(e) -> Fail(e)
    }
}

// flatMap - for chains
fun flatMapResult(r, f) {
    match r {
        Ok(a) -> f(a)
        Fail(e) -> Fail(e)
    }
}

// mapError - transform error
fun mapError(r, f) {
    match r {
        Ok(a) -> Ok(a)
        Fail(e) -> Fail(f(e))
    }
}

// getOrElse
fun getOrElseResult(r, default) {
    match r {
        Ok(a) -> a
        Fail(_) -> default
    }
}

// Examples
doubled = mapResult(Ok(5), fun(x) -> x * 2)
print(doubled)  // Ok(10)

withDefault = getOrElseResult(Fail("error"), 0)
print(withDefault)  // 0
```

---

## Real examples from lib/*

### Reading file

```rust
import "lib/io" (fileRead, fileWrite)

fun loadConfig(path: String) {
    match fileRead(path) {
        Ok(content) -> {
            print("Loaded " ++ show(len(content)) ++ " bytes")
            content
        }
        Fail(err) -> {
            print("Error reading " ++ path ++ ": " ++ err)
            "{}"  // default empty config
        }
    }
}

config = loadConfig("config.json")

```

### HTTP requests

```rust
import "lib/http" (httpGet)
import "lib/json" (jsonDecode)

fun fetchUser(id: Int) {
    url = "https://api.example.com/users/" ++ show(id)
    
    match httpGet(url) {
        Ok(response) -> {
            if response.status == 200 {
                Ok(jsonDecode(response.body))
            } else {
                Fail("HTTP " ++ show(response.status))
            }
        }
        Fail(err) -> Fail("Network error: " ++ err)
    }
}

```

---

## Chains of operations

### Problem: nested match

```rust
// Example: mock functions
fun getUser(id: Int) -> Result<String, { id: Int, name: String }> {
    if id > 0 { Ok({ id: id, name: "Alice" }) } else { Fail("User not found") }
}

fun getEmail(user) -> Result<String, String> {
    Ok(user.name ++ "@example.com")
}

fun validateEmail(email: String) -> Result<String, String> {
    Ok(email)
}

// Deep nesting - works, but hard to read
fun processUserBad(id: Int) {
    match getUser(id) {
        Fail(e) -> Fail(e)
        Ok(user) -> match getEmail(user) {
            Fail(e) -> Fail(e)
            Ok(email) -> match validateEmail(email) {
                Fail(e) -> Fail(e)
                Ok(valid) -> Ok({ user: user, email: valid })
            }
        }
    }
}

result = processUserBad(1)
print(result)
```

### Solution: flatMap chain

```rust
// Helper functions for Result
fun mapResult(r, f) {
    match r { Ok(v) -> Ok(f(v)), Fail(e) -> Fail(e) }
}

fun flatMapResult(r, f) {
    match r { Ok(v) -> f(v), Fail(e) -> Fail(e) }
}

// Mock functions
fun getUser(id: Int) { if id > 0 { Ok({ id: id, name: "Alice" }) } else { Fail("Not found") } }
fun getEmail(user) { Ok(user.name ++ "@example.com") }
fun validateEmail(email: String) { Ok(email) }

// Linear chain - cleaner
fun processUserGood(id: Int) {
    result = getUser(id)
    result = flatMapResult(result, fun(user) -> mapResult(getEmail(user), fun(email) -> { user: user, email: email }))
    result = flatMapResult(result, fun(data) -> mapResult(validateEmail(data.email), fun(valid) -> { user: data.user, email: valid }))
    result
}

result = processUserGood(1)
print(result)
```

---

## Custom error types

```rust
import "lib/list" (contains)

// ADT for validation errors
type ValidationError = EmptyField(String)
                     | TooShort(String)
                     | InvalidFormat(String)

// Validation functions
fun validateUsername(name: String) -> Result<ValidationError, String> {
    if len(name) == 0 { 
        Fail(EmptyField("username")) 
    } else if len(name) < 3 { 
        Fail(TooShort("username")) 
    } else { 
        Ok(name) 
    }
}

fun validateEmail(email: String) -> Result<ValidationError, String> {
    if len(email) == 0 {
        Fail(EmptyField("email"))
    } else if !contains(email, '@') {
        Fail(InvalidFormat("email"))
    } else {
        Ok(email)
    }
}

// Beautiful error output
fun showValidationError(e: ValidationError) -> String {
    match e {
        EmptyField(field) -> "Field '" ++ field ++ "' is required"
        TooShort(field) -> "Field '" ++ field ++ "' is too short"
        InvalidFormat(field) -> "Field '" ++ field ++ "' has invalid format"
    }
}

// Usage
match validateUsername("ab") {
    Ok(name) -> print("Valid username: " ++ name)
    Fail(err) -> print("Validation error: " ++ showValidationError(err))
}
// Validation error: Field 'username' is too short
```

---

## Combining multiple Results

```rust
// If all Ok - return list of values
// If at least one Fail - return first error
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

// Examples
allOk = [Ok(1), Ok(2), Ok(3)]
print(sequence(allOk))  // Ok([1, 2, 3])

withError = [Ok(1), Fail("oops"), Ok(3)]
print(sequence(withError))  // Fail("oops")
```

### Collect all errors (not just first)

```rust
import "lib/list" (foldl)

// Collects all Ok values or all Fail errors
fun sequenceAll(results) {
    foldl(fun(acc, r) -> {
        match (acc, r) {
            (Ok(xs), Ok(x)) -> Ok(xs ++ [x])
            (Fail(es), Fail(e)) -> Fail(es ++ [e])
            (Fail(es), Ok(_)) -> Fail(es)
            (Ok(_), Fail(e)) -> Fail([e])
        }
    }, Ok([]), results)
}

// Example
results = [Ok(1), Ok(2), Ok(3)]
match sequenceAll(results) {
    Ok(values) -> print("All valid: " ++ show(values))
    Fail(errors) -> print("Errors: " ++ show(errors))
}

resultsWithErrors = [Ok(1), Fail("error1"), Fail("error2")]
match sequenceAll(resultsWithErrors) {
    Ok(values) -> print("All valid: " ++ show(values))
    Fail(errors) -> print("Errors: " ++ show(errors))
}

```

---

## Converting between types

```rust
// Option -> Result
fun optionToResult(opt, errorMsg) {
    match opt {
        Some(x) -> Ok(x)
        Zero -> Fail(errorMsg)
    }
}

// Result -> Option (lose error information)
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

---

## When to use what?

| Situation | Use | Example |
|----------|------------|--------|
| Value may be missing, reason not important | `Option<T>` | `find()`, `head()` |
| Quick null-check for fields | `T?` | `user.middleName?` |
| Operation might fail, need reason | `Result<String, T>` | `fileRead()`, `httpGet()` |
| Validation with detailed errors | `Result<CustomError, T>` | Form validation |
| Might be multiple errors | `Result<List<E>, T>` | Batch validation |
