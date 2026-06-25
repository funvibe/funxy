# Iteration 6: Custom Types

This iteration introduces powerful type system features: Type Aliases, Records, and Algebraic Data Types (ADTs).

## Type Aliases

Aliases create a new name for an existing type. They are interchangeable with the original type at compile time but help with readability.

> **Runtime note:** alias transparency is a *compile-time* property. At runtime, aliases of
> scalars and tuples are erased to their base type (`getType(money) == getType(0.0)`), but
> aliases of **records** (and ADTs) are **nominal** — they carry their name, so `getType`
> distinguishes them (`getType(pointA) != getType(pointB)` even with identical fields).
> See [Runtime Representation](#runtime-representation) below.

```rust
type alias Money = Float

fun formatMoney(m: Money) -> String {
    "$" ++ show(m)
}

print(formatMoney(19.99))  // "$19.99"
```

## Records

Records are structural types with named fields.

```rust
// Type definition
type alias Point = { x: Int, y: Int }

// Literal
p = { x: 10, y: 20 }

// Access
print(p.x)  // 10
```

## Methods

You can define methods on custom types (Records or ADTs) using the `fun (receiver: Type) name()` syntax.

```rust
type alias Point = { x: Int, y: Int }

// Define a method 'dist_sq' on Point
fun (p: Point) dist_sq() -> Int {
    p.x * p.x + p.y * p.y
}

p: Point = { x: 3, y: 4 }
print(p.dist_sq()) // 25
```

## Algebraic Data Types (ADTs)

ADTs allow you to define types that can be one of several variants.

Parameters for generic ADTs are listed in angle brackets after the type name.

```rust
// Option is built-in, but custom ADTs look like:
type MyOption<t> = MySome(t) | MyNone

x = MySome(42)
y = MyNone
print(x)  // MySome(42)
```

Recursive ADTs are supported (e.g., for Lists or Trees).

```rust
// List is built-in, but custom recursive ADTs look like:
type MyList<t> = MyCons((t, MyList<t>)) | MyEmpty

list = MyCons((1, MyCons((2, MyEmpty))))
print(list)
```

## Runtime Type Checking

### typeOf

The function `typeOf(value, Type) -> Bool` checks the type of a value:

```rust
x = 10
typeOf(x, Int)      // true
typeOf(x, String)   // false

name = "Alice"
typeOf(name, String)  // true
```

### Parameterized Types

For checking parameterized types, use **parentheses** (not angle brackets!):

```rust
type MyOption<t> = Yes t | NoVal

o = Yes(42)

// Check without parameter - any MyOption
typeOf(o, MyOption)       // true

// Check with parameter - specific MyOption<Int>
typeOf(o, MyOption(Int))  // true
typeOf(o, MyOption(String))  // false
```

**Important:** In expressions use `Type(Param)`, not `Type<Param>`:
```rust
// Correct:
typeOf(list, List)
typeOf(opt, Option(Int))

// Syntax error:
typeOf(list, List<Int>)  // angle brackets don't work!
```

### getType

The function `getType(value) -> Type` returns the type of a value:

```rust
x = 42
t1 = getType(x)
print(t1)  // type(Int)

f = fun(a: Int) -> a * 2
t2 = getType(f)
print(t2)  // type((Int) -> Int)
```

## Runtime Representation

*   **Scalar/tuple aliases** (e.g. `type alias Money = Int`): resolved to the underlying type and erased at runtime — `getType` returns the base type, and two such aliases are indistinguishable.
*   **Record aliases** (e.g. `type alias Point = { x: Int, y: Int }`): nominal at runtime. The `RecordInstance` carries the alias name, so `getType` reports `Point` (not `{ x: Int, y: Int }`) and distinguishes it from other aliases with identical fields.
*   **ADTs**: Represented as `DataInstance` with constructor name and field values (nominal).
