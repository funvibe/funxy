# Records

Records are structural types with named fields.

## Record Type Definition

You can define a record type using the `type` keyword:

```rust
// Named record type
type alias Point = { x: Int, y: Int }

// Type with multiple fields of different types
type alias Person = { name: String, age: Int, active: Bool }

// Nested records
type alias Rectangle = { topLeft: Point, bottomRight: Point }
```

## Creating Records

### Record Literals

```rust
type alias Point = { x: Int, y: Int }
type alias Person = { name: String, age: Int, active: Bool }

// Anonymous record (without type declaration)
point = { x: 10, y: 20 }

// Record with type annotation
p: Point = { x: 10, y: 20 }

// Field order doesn't matter
person: Person = { age: 30, name: "Alice", active: true }
```

### Empty Record

```rust
type alias EmptyRec = {}

e: EmptyRec = {}
```

## Field Access

Use dot notation:

```rust
p = { x: 10, y: 20 }

print(p.x)    // 10
print(p.y)    // 20

// Nested access
rect = { topLeft: { x: 0, y: 0 }, bottomRight: { x: 100, y: 50 } }
print(rect.topLeft.x)      // 0
print(rect.bottomRight.y)  // 50
```

## Field Modification

Funxy allows modifying record fields:

```rust
p = { x: 10, y: 20 }
p.x = 100
print(p.x)  // 100

// Nested modification
rect = { tl: { x: 0, y: 0 }, br: { x: 10, y: 10 } }
rect.tl.y = 50
print(rect.tl.y)  // 50
```

## Record Update (spread)

Creating a new record based on an existing one with some fields changed:

```rust
type alias Point = { x: Int, y: Int }

base: Point = { x: 1, y: 2 }

// Create new record, changing only x
updated = { ...base, x: 10 }
print(updated.x)  // 10
print(updated.y)  // 2 (from base)

// Original is unchanged
print(base.x)     // 1
```

## Argument Shorthand Sugar

Funxy supports a convenient shorthand for record arguments in functions, similar to named parameters.

If you have a function that accepts a record:

```rust
type alias Config = { host: String, port: Int }

fun connect(config: Config) {
    // ...
}
```

You can call it passing a record literal:

```rust
connect({ host: "localhost", port: 8080 })
```

### Implicit Record Construction

If the **last argument** of a function is expected to be a record, you can pass the fields directly as named arguments without the enclosing braces.

```rust
// Definition
fun createUser(name, options: { age: Int, active: Bool }) {}

// Call with shorthand
createUser("Bob", age: 25, active: true)

// Equivalent to:
createUser("Bob", { age: 25, active: true })
```

This works particularly well for configuration objects or optional parameters.

**Rules:**
1. The record argument must be the last one.
2. You use `key: value` syntax separated by commas.
3. The keys must match the fields of the expected record type.

Example with UI components:

```rust
// Modern UI DSL syntax - function with trailing block
import "kit/ui" (div, span, text)

// Clean syntax without parentheses when using trailing block
page = div {
    span { text("Hello") }
    span { text("World") }
}

// Or with traditional call syntax and named arguments for attributes
page2 = div(
    class: "container",
    id: "main"
) {
    text("Content")
}
```

## Record Destructuring

Records support destructuring to extract fields into variables:

```rust
// Basic destructuring
p = { x: 3, y: 4 }
{ x: a, y: b } = p
print(a)  // 3
print(b)  // 4
```

### With Named Types

```rust
type alias Point = { x: Int, y: Int }

p: Point = { x: 10, y: 20 }
{ x: x, y: y } = p
print(x)  // 10
print(y)  // 20
```

### Partial Destructuring

You can extract only the needed fields:

```rust
person = { name: "Alice", age: 30, city: "Moscow" }

// Only name
{ name: n } = person
print(n)  // Alice
```

### Nested Destructuring

```rust
data = {
    user: { name: "Bob", role: "admin" },
    count: 42
}

{ user: { name: userName, role: r }, count: c } = data
print(userName)  // Bob
print(r)         // admin
print(c)         // 42
```

### In Functions

```rust
import "lib/math" (sqrt)

type alias Point = { x: Int, y: Int }

fun magnitude(p: Point) -> Float {
    { x: x, y: y } = p
    sqrt(intToFloat(x * x + y * y))
}

p: Point = { x: 3, y: 4 }
print(magnitude(p))  // 5
```

## Nominal vs Structural Types

### Structural Types (Anonymous Records)

Without type annotation, a record has a structural type:

```rust
point1 = { x: 10, y: 20 }
print(getType(point1))  // { x: Int, y: Int } (represented as anonymous record type)
```

### Nominal Types (Record Type Definition)

When you define a record type and use annotation, the record has a nominal type:

```rust
// Record type definition
type alias Point = { x: Int, y: Int }

p: Point = { x: 10, y: 20 }
print(getType(p))  // Point
```

**Important:** `type alias Point = { ... }` creates a type alias for a record structure. It does not create a new nominal type identity, but it gives a name to the structure.

For assignment, Funxy allows assigning a structurally compatible anonymous record to a nominal type variable (implicit conversion):

```rust
type alias Point = { x: Int, y: Int }
p: Point = { x: 1, y: 2 } // OK, converted to Point
```

But at runtime, they are distinct:

```rust
type alias Point = { x: Int, y: Int }
anon = { x: 1, y: 2 }
named: Point = { x: 1, y: 2 }

print(typeOf(named, Point)) // true
print(typeOf(anon, Point))  // false (unless casted/assigned to Point)
```

### Why is this important?

Nominal types are needed for:

1. **Extension methods** — methods are bound to the type name:

```rust
type alias Point = { x: Int, y: Int }

fun (p: Point) length() -> Int {
    p.x + p.y  // simplified
}

// Works only with Point
p: Point = { x: 3, y: 4 }
print(p.length())  // 7

// DOESN'T work with anonymous record
anon = { x: 3, y: 4 }
// anon.length()  // Error! { x: Int, y: Int } has no method length
```

2. **Trait instances** — trait implementations are bound to the type name:

```rust
type alias Point = { x: Int, y: Int }

instance Default Point {}

p: Point = default(Point)  // { x: 0, y: 0 }
```

3. **Method Dispatch** — resolving overloaded methods or trait methods based on type.

## Extension Methods

Defining methods on records:

```rust
type alias Point = { x: Int, y: Int }

// Method without parameters
fun (p: Point) distanceFromOrigin() -> Int {
    p.x * p.x + p.y * p.y
}

// Method with parameters
fun (p: Point) add(other: Point) -> Point {
    { x: p.x + other.x, y: p.y + other.y }
}

// Usage
p1: Point = { x: 3, y: 4 }
p2: Point = { x: 1, y: 1 }

print(p1.distanceFromOrigin())  // 25
print(p1.add(p2).x)             // 4
```

## Pattern Matching

Anonymous records support pattern matching:

```rust
// Pattern matching works with anonymous records
fun describe(p: { x: Int, y: Int }) -> String {
    match p {
        { x: 0, y: 0 } -> "origin"
        { x: 0, y: _ } -> "on Y axis"
        { x: _, y: 0 } -> "on X axis"
        { x: x, y: y } -> "point at (" ++ show(x) ++ ", " ++ show(y) ++ ")"
    }
}

p = { x: 0, y: 5 }
print(describe(p))  // "on Y axis"
```

For named record types, use field access:

```rust
type alias Point = { x: Int, y: Int }

fun describePoint(p: Point) -> String {
    if p.x == 0 && p.y == 0 { "origin" }
    else if p.x == 0 { "on Y axis" }
    else if p.y == 0 { "on X axis" }
    else { "point at (" ++ show(p.x) ++ ", " ++ show(p.y) ++ ")" }
}

p: Point = { x: 0, y: 5 }
print(describePoint(p))  // "on Y axis"
```

### Partial Matching (Row Polymorphism)

You can match only part of the fields:

```rust
r = { x: 10, y: 20, z: 30 }

match r {
    { x: val } -> print(val)  // 10 — other fields are ignored
}
```

## Row Polymorphism

A function can accept records with "additional" fields. Unlike some languages, the `...` syntax is **not required** — row polymorphism works automatically:

```rust
// Function expects a record with field x (without "...")
fun getX(r: { x: Int }) -> Int {
    r.x
}

// You can pass a record with any number of additional fields
point = { x: 10, y: 20 }
print(getX(point))  // 10

extended = { x: 5, y: 6, z: 7, name: "test" }
print(getX(extended))  // 5

// Even completely different records, if they have the needed field
config = { x: 100, host: "localhost", port: 8080 }
print(getX(config))  // 100
```

> **Note**: The `{ x: Int, ... }` syntax for row polymorphism is **not needed** and not supported. The language automatically allows any records containing at least the specified fields.

## Records in Functions

### As Parameters

```rust
type alias Config = { host: String, port: Int, debug: Bool }

fun connect(cfg: Config) -> String {
    cfg.host ++ ":" ++ show(cfg.port)
}

config: Config = { host: "localhost", port: 8080, debug: true }
print(connect(config))  // "localhost:8080"
```

### As Return Value

```rust
type alias Point = { x: Int, y: Int }

fun makePoint(x: Int, y: Int) -> Point {
    { x: x, y: y }
}

p = makePoint(10, 20)
print(p.x)  // 10
```

## Generics with Records

```rust
type alias Box<t> = { value: t }

fun makeBox<t>(v: t) -> Box<t> {
    { value: v }
}

intBox = makeBox(42)
print(intBox.value)  // 42

strBox = makeBox("hello")
print(strBox.value)  // "hello"
```

## Practical Examples

### Configuration

```rust
type alias ServerConfig = {
    host: String,
    port: Int,
    timeout: Int,
    debug: Bool
}

defaultConfig: ServerConfig = {
    host: "localhost",
    port: 8080,
    timeout: 30,
    debug: false
}

// Create configuration for production
prodConfig = { ...defaultConfig, host: "api.example.com", debug: false }
```

### Game State

```rust
type alias Player = { name: String, health: Int, x: Int, y: Int }

fun damage(p: Player, amount: Int) -> Player {
    { ...p, health: p.health - amount }
}

fun moveTo(p: Player, newX: Int, newY: Int) -> Player {
    { ...p, x: newX, y: newY }
}

player: Player = { name: "Hero", health: 100, x: 0, y: 0 }
player = damage(player, 20)
player = moveTo(player, 10, 5)
print(player.health)  // 80
print(player.x)       // 10
```

### DTO for API

```rust
type alias User = { id: Int, name: String, email: String }
type alias CreateUserRequest = { name: String, email: String }

fun createUser(req: CreateUserRequest, id: Int) -> User {
    { id: id, name: req.name, email: req.email }
}
```

## When to Use Records

**Use Records when:**
- Data structure is fixed and known in advance
- Need typing for each field
- Want to use extension methods
- Want to implement traits for the type

**Use Map when:**
- Keys are dynamic or unknown in advance
- All values are of the same type
- Need fast lookup by arbitrary key

**Use Tuple when:**
- Need a fixed collection without field names
- Data is temporary (e.g., returning multiple values)

## Summary

| Operation | Syntax | Example |
|----------|-----------|--------|
| Type definition | `type alias Name = { field: Type }` | `type alias Point = { x: Int, y: Int }` |
| Creation | `{ field: value }` | `{ x: 10, y: 20 }` |
| With annotation | `name: Type = { ... }` | `p: Point = { x: 10, y: 20 }` |
| Field access | `record.field` | `p.x` |
| Field modification | `record.field = value` | `p.x = 100` |
| Spread/Update | `{ ...base, field: value }` | `{ ...p, x: 0 }` |
| Extension method | `fun (r: Type) name() { }` | `fun (p: Point) len() { }` |
| Pattern match | `match r { { f: v } -> ... }` | `match p { { x: 0 } -> "zero" }` |

## See Also

- [Custom Types](06_custom_types.md) — ADT and type aliases
- [Pattern Matching](07_pattern_matching.md) — record destructuring
- [Traits](08_traits.md) — trait implementation for records
- [Maps](24_maps.md) — dynamic associative arrays
