# 10. Audience Tracks

[← Back to Index](./00_index.md)

- [Funxy Key Features](#funxy-key-features)
- [Python Developers](#python-developers)
- [JS/TS Developers](#jsts-developers)
- [Go Developers](#go-developers)
- [Haskell Developers](#haskell-developers)
- [Backend Developers](#backend-developers)

## Funxy Key Features

### Naming Rules

Funxy enforces identifier casing at the syntax level:

**Lowercase** (required): variables, functions, type parameters
```rust
userName = "Alice"
fun processData(x) { x }
fun myIdentity<t>(x: t) -> t { x }

// UserName = "Alice"  // Error: expected lowercase identifier
// fun Process(x) { }  // Error: expected lowercase identifier
```

**Uppercase** (required): types, traits, ADT constructors
```rust
type alias User = { name: String, age: Int }
type MyOption<t> = MySome(t) | MyNone
trait MyShow<t> { fun show(x: t) -> String }

// type user = { ... }     // Error: expected uppercase identifier
// type option<t> = ...    // Error: expected uppercase identifier
```

### Expression-Oriented

Most constructs are expressions and return values. The last expression in a block is the result, but you can also use an explicit `return` for early exit.

```rust
// Functions return last expression
fun calculateDiscount(price, userTier) {
    discount = match userTier {
        "gold" -> 0.2
        "silver" -> 0.1
        _ -> 0.0
    }
    price * discount
}

// if/else is an expression
x = 10
result = if x > 0 { "positive" } else { "non-positive" }

// match is an expression
value = 0
description = match value {
    0 -> "zero"
    n if n < 0 -> "negative"
    _ -> "positive"
}

// Blocks return last expression
fun compute() { 1 }
fun transform(x) { x }

value = {
    temp = compute()
    transform(temp)  // this is returned
}

// Loops can return values too
fun predicate(x) { x > 0 }
list = [1, 2, 3]

found = for item in list {
    if predicate(item) {
        break Some(item)  // break with value
    }
    None // fallback value if loop continues
}
```

### Type System

**Gradual Typing with Inference**
```rust
// Types are inferred automatically
numbers = [1, 2, 3]           // List<Int>
user = { name: "Alice", age: 30 }  // { name: String, age: Int }

// Add annotations when needed
fun process(x: Int | String) -> String { "ok" }
```

**Union Types and Nullable Shorthand**
```rust
x: Int | String = 42
x = "hello"  // OK

name: String? = "Alice"  // Equivalent to String | Nil
name = nil  // OK
```

**Algebraic Data Types (ADT)**
```rust
type Shape =
    | Circle(Float)
    | Rectangle(Float, Float)
    | Point

type Tree<t> = Leaf(t) | Branch(Tree<t>, Tree<t>)
```

**Type Aliases vs ADT**
```rust
type alias Money = Float           // Alias (same type)
type alias UserId = Int

type Color = Red | Green | Blue    // ADT (new type with constructors)
```

**Row Polymorphism**
```rust
fun getName(r: { name: String }) -> String {
    r.name
}

// Works with any record that has 'name' field
getName({ name: "Alice", age: 30, role: "admin" })  // OK
```

### Immutability Model

**Immutable Collections, Mutable Bindings**
```rust
import "lib/list" (update)

// Variables can be reassigned
x = 1
x = 2  // OK

// But collections are immutable
list = [1, 2, 3]
// list[0] = 10  // Error!
// Create new collection instead
list2 = update(list, 0, 10) // [10, 2, 3]
```

**Constants**
```rust
const pi = 3.14159
// or
// pi :- 3.14159
// pi = 3.0  // Error: cannot reassign constant
```

### Pattern Matching

**Comprehensive Destructuring**
```rust
type Shape = Circle(Float) | Rectangle(Float, Float) | Point

// Lists
items = [1, 2]
match items {
    [] -> "empty"
    [x] -> "single"
    [head, ...tail] -> "multiple"
}

// Records
userRec = { name: "Bob", age: 20, role: "user" }
match userRec {
    { role: "admin" } -> "administrator"
    { name: n, age: a } if a >= 18 -> "adult: " ++ n
    _ -> "other"
}

// ADT
shape = Circle(5.0)
match shape {
    Circle(r) -> 3.14 * r * r
    Rectangle(w, h) -> w * h
    Point -> 0.0
}
```

**String Patterns with Captures**
```rust
fun handleUser(id) { "User: " ++ id }
fun handlePost(u, p) { "Post: " ++ u ++ "/" ++ p }
fun serveFile(f) { "File: " ++ f }
fun notFound() { "404" }

path = "/users/123"

match path {
    "/users/{id}" -> handleUser(id)
    "/users/{userId}/posts/{postId}" -> handlePost(userId, postId)
    "/static/{...file}" -> serveFile(file)  // Greedy capture
    _ -> notFound()
}
```

**Guards and Pin Operator**
```rust
expected = 42
value = 150
match value {
    x if x > 100 -> "large"
    ^expected -> "exact match"  // Pin: compare with variable
    _ -> "other"
}
```

### Functions

**Multiple Syntaxes**
```rust
// Named function
fun add(a: Int, b: Int) -> Int { a + b }

// Lambda
double = fun(x) -> x * 2

// Haskell-style
triple = \x -> x * 3
```

**Partial Application**
```rust
fun add(a: Int, b: Int) -> Int { a + b }

add5 = add(5)      // (Int) -> Int
add5(3)            // 8

// Chain
add(1)(2)       // With curried function
```

**Pipelines and Composition**
```rust
fun parse(x) { x }
fun validate(x) { x }
fun transform(x) { x }
fun divide(a, b) { a / b }
data = 100

// Pipeline: left to right
result = data
    |> parse
    |> validate
    |> transform

// With placeholder
10 |> divide(100, _)  // divide(100, 10)

// Composition: right to left
process = parse ,, validate ,, transform
```

**Extension Methods**
```rust
import "lib/math" (sqrt)

type alias Point = { x: Int, y: Int }

fun (p: Point) magnitude() -> Float {
    sqrt(intToFloat(p.x * p.x + p.y * p.y))
}

p = { x: 3, y: 4 }
p.magnitude()  // 5.0
```

### Error Handling

**Result and Option Types**
```rust
// Option<t> = Some(t) | None
// Result<e, t> = Ok(t) | Fail(e)

fun divide(a: Int, b: Int) -> Result<String, Int> {
    if b == 0 { Fail("division by zero") }
    else { Ok(a / b) }
}
```

**Error Propagation with ?**
```rust
type alias Data = Map<String, String>

fun readFile(p: String) -> Result<String, String> { Ok("{}") }
fun parseJson(c: String) -> Result<String, Data> { Ok(%{}) }
fun transform(d: Data) -> String { "done" }

fun process() -> Result<String, String> {
    content = readFile("data.txt")?   // Early return on Fail
    parsed = parseJson(content)?
    Ok(transform(parsed))
}
```

**Optional Chaining**
```rust
type alias Addr = { city: String }
type alias User = { address: Addr }
user: Option<User> = Some({ address: { city: "NY" } })

val: Option<Int> = Some(1)
defaultValue = 2

user?.address?.city  // Some("NY") or None
val ?? defaultValue  // 1
```

### Traits and Generics

**Trait Definition and Implementation**
```rust
trait MyStringify<t: Show> {
    fun myFormat(val: t) -> String
}

instance MyStringify Int {
    fun myFormat(val: Int) -> String {
        "Int(" ++ show(val) ++ ")"
    }
}
```

**Higher-Kinded Types**
```rust
trait MyFunctor<f: * -> *> {
    fun fmap<a, b>(func: (a) -> b, val: f<a>) -> f<b>
}

instance MyFunctor List {
    fun fmap<a, b>(f: (a) -> b, list: List<a>) -> List<b> {
        [f(x) | x <- list]
    }
}
```

**Operator Overloading**
```rust
type alias Vec2 = { x: Float, y: Float }

instance Numeric Vec2 {
    operator (+)(a: Vec2, b: Vec2) -> Vec2 {
        { x: a.x + b.x, y: a.y + b.y }
    }
}
```

**Do-Notation**
```rust
result = do {
    x <- Some(10)
    y <- Some(20)
    k :- 2  // let binding
    Some((x + y) * k)
}
```

### Modules

**Export Control**
```rust
// package mylib (publicFunc, PublicType)  // Export specific
// package mylib (*)                        // Export all
// package mylib !(internal, Private)       // Export all except
```

**Import Styles**
```rust
import "lib/list"              // As module object
// import "lib/list" as l         // With alias
// import "lib/list" (map, filter) // Specific symbols
// import "lib/list" (*)          // All symbols
```

### Standard Library Highlights

| Module | Purpose |
|--------|---------|
| `lib/http` | HTTP client and server |
| `lib/sql` | SQLite database |
| `lib/ws` | WebSocket client and server |
| `lib/json` | JSON encode/decode |
| `lib/csv` | CSV parsing |
| `lib/task` | Async/await |
| `lib/regex` | Regular expressions |
| `lib/crypto` | Hashing, encoding |

### Go Embedding

```golang
vm := funxy.New()
vm.Bind("user", &User{Name: "Alice"})
result, _ := vm.Eval(`"Hello, " ++ user.Name`)
```

---

## Python Developers

**Key Differences:**

| Python | Funxy |
|--------|-------|
| `x = 42` | `x = 42` (same) |
| `PI = 3.14` (convention only) | `const pi = 3.14` or `pi :- 3.14` (enforced) |
| `x: int = 42` | `x: Int = 42` |
| `def f(x):` | `fun f(x) { }` |
| `lambda x: x*2` | `fun(x) -> x * 2` or `\x -> x * 2` |
| `[x*2 for x in lst]` | `[x*2 | x <- lst]` |
| `[x for x in lst if x > 0]` | `[x | x <- lst, x > 0]` |
| `lst[0]`, `lst[-1]` | `lst[0]`, `lst[-1]` (same) |
| `{"a": 1}` | `%{ "a" => 1 }` (Map) or `{ a: 1 }` (Record) |
| `None` | `nil` |
| `Optional[int]` | `Int?` or `Option<Int>` |
| `try/except` | `Result` + `match` or `?` operator |
| `f(g(h(x)))` | `x |> h |> g |> f` |

**Naming:** Unlike Python where `UPPER_CASE` for constants is just a convention, Funxy enforces casing at syntax level:
```rust
userName = "Alice"      // OK
// UserName = "Alice"      // Error: uppercase is for types

type alias User = { name: String }  // OK
// type user = { name: String }  // Error: types must be uppercase
```

**You'll Like:**
- List comprehensions with familiar syntax
- Type inference — types work without annotations
- String interpolation: `"Hello, ${name}!"`
- Pipelines `|>` for readable data transformations
- Negative indexing works the same way

**What's Different:**
- **Immutable collections**: Lists and Maps cannot be modified in place
```rust
import "lib/list" (update)

list = [1, 2, 3]
// list[0] = 10  // Error!
list2 = update(list, 0, 10) // [10, 2, 3]
```
- **Pattern matching** for complex destructuring and control flow:
```rust
x = 1
// Works alongside if/else
if x > 0 {
    "positive"
} else if x == 0 {
    "zero"
} else {
    "non-positive"
}

// Pattern matching shines with destructuring
value = (1, 1)
match value {
    (x, y) if x == y -> "equal pair"
    (0, y) -> "starts with zero"
    _ -> "other"
}
```
- **No classes**: Use Records for data, Traits for behavior
- **Explicit Option type**: `Some(value)` or `None` instead of null checks

**Example — Python to Funxy:**

```python
# Python
def process_users(users):
    return [u["name"].upper() for u in users if u["age"] >= 18]
```

```rust
// Funxy
import "lib/string" (stringToUpper)

fun processUsers(users) {
    [stringToUpper(u.name) | u <- users, u.age >= 18]
}
```

---

## JS/TS Developers

**Key Differences:**

| JS/TS | Funxy |
|-------|-------|
| `const x = 42` | `const x = 42` (constant) |
| `let x = 42` | `x = 42` (mutable in functions) |
| `function f(x) {}` | `fun f(x) {}` |
| `(x) => x * 2` | `fun(x) -> x * 2` |
| `array.map(x => x*2)` | `list |> map(fun(x) -> x * 2)` |
| `...spread` | `...spread` (similar for lists/records) |
| `null` / `undefined` | `nil` or `Option<T>` (`Some`/`None`) |
| `interface User { x: number }` | `type alias User = { x: Int }` |
| `type ID = string` | `type alias ID = String` |

**You'll Like:**
*   First-class functions.
*   Destructuring (Pattern Matching is essentially destructuring on steroids).
*   Expression-oriented syntax (everything returns a value).
*   Strict types without TypeScript's complex configuration.

**What's New:**
*   **Immutability:** Lists and Maps are immutable (use `update` or `mapPut`). Records are mutable (like JS objects).
*   **Pipelines:** Use `|>` for chaining instead of method chaining.
*   **Result Type:** No `try/catch` for logic flow, use `Result<E, T>`.

---

## Go Developers

**Embedding Funxy in Go:**

```golang
package main

import "github.com/funvibe/funxy/pkg/embed"

func main() {
    vm := funxy.New()

    // Bind Go function
    vm.Bind("multiply", func(a, b int) int {
        return a * b
    })

    // Bind struct
    type User struct {
        Name  string
        Score int
    }
    vm.Bind("user", &User{Name: "Alice", Score: 100})

    // Execute
    result, _ := vm.Eval(`
        newScore = multiply(user.Score, 2)
        "User " ++ user.Name ++ " has score " ++ show(newScore)
    `)

    fmt.Println(result) // User Alice has score 200
}
```

**Calling Funxy from Go:**

```golang
vm.LoadFile("rules.lang")
result, err := vm.Call("validateUser", userData)
```

**Type Mapping:**

| Go | Funxy |
|----|-------|
| `int`, `int64` | `Int` |
| `float64` | `Float` |
| `bool` | `Bool` |
| `string` | `String` |
| `[]T` | `List<T>` |
| `map[string]T` | `Map<String, T>` |
| `struct` | Record / HostObject |
| `nil` | `Nil` |

---

## Haskell Developers

**Familiar Concepts:**

```rust
// ADT
type Maybe<t> = Just(t) | Nothing

// Functor (Built-in trait)
// trait Functor<f: * -> *> {
//     fun fmap<a, b>(f: (a) -> b, fa: f<a>) -> f<b>
// }

instance Functor Maybe {
    fun fmap(f, m) {
        match m {
            Just(x) -> Just(f(x))
            Nothing -> Nothing
        }
    }
}

// Applicative (Built-in trait)
instance Applicative Maybe {
    fun pure(x) { Just(x) }
    operator (<*>)(f, m) {
        match f {
            Just(func) -> fmap(func, m)
            Nothing -> Nothing
        }
    }
}

// Monad (Built-in trait)
instance Monad Maybe {
    operator (>>=)(m, f) {
        match m {
            Just(x) -> f(x)
            Nothing -> Nothing
        }
    }
}

// Do Notation
result = do {
    x <- Just(10)
    Just(x + 1)
}
```

**Differences:**

| Haskell | Funxy |
|---------|-------|
| Lazy | Strict |
| `data Maybe a = Just a | Nothing` | `type Maybe<t> = Just(t) | Nothing` |
| `class Functor f` | `trait Functor<f: * -> *>` |
| `f . g` | `f ,, g` |
| `x >>= f` | `x >>= f` (same) |
| Pure everywhere | Local mutation allowed |

---

## Backend Developers

**HTTP Server Example:**

```rust
import "lib/http" (*)
import "lib/json" (jsonEncode)

// Mock DB
type alias DB = { getUsers: () -> List<Int> }
db: DB = { getUsers: fun() -> [] }

fun handler(req) {
    match (req.method, req.path) {
        ("GET", "/users") -> {
            users = db.getUsers()
            { status: 200, body: jsonEncode(users), headers: [] }
        }
        _ -> { status: 404, body: "Not found", headers: [] }
    }
}

// httpServe(8080, handler)
```
