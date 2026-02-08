# 6. Type System & ADTs

[← Back to Index](./00_index.md)

## Basic Types

| Type | Description | Example |
|------|-------------|---------|
| `Int` | 64-bit integer | `42` |
| `Float` | Floating point | `3.14` |
| `Bool` | Boolean | `true`, `false` |
| `Char` | Unicode character | `'a'` |
| `String` | String (List<Char>) | `"hello"` |
| `Nil` | Absence of value | `nil` |
| `BigInt` | Arbitrary precision | `100n` |
| `Rational` | Rational number | `1r/3r` |

## Collection Types

```rust
List<Int>           // List of integers
Map<String, Int>    // Key-value map
(Int, String)       // Tuple
{ x: Int, y: Int }  // Record
```

## Type Aliases

```rust
type alias Request = { path: String }
type alias Response = { status: Int }

type alias UserId = Int
type alias Point = { x: Float, y: Float }
type alias Handler = (Request) -> Response
```

## Union Types

A value can be one of several types.

```rust
x: Int | String = 42
x = "hello"  // OK

// Nullable - shorthand for T | Nil
name: String? = "Alice"
name = nil  // OK

// Handling
fun process(x: Int | String) -> String {
    match x {
        n: Int -> "number: " ++ show(n)
        s: String -> "text: " ++ s
    }
}
```

## Algebraic Data Types (ADTs)

Define complex data structures with variants.

```rust
// Enum
type Color = Red | Green | Blue

// With Data
type Shape =
    | Circle(Float)
    | Rectangle(Float, Float)
    | Point

// Usage
c = Circle(5.0)
r = Rectangle(3.0, 4.0)

fun area(s: Shape) -> Float {
    match s {
        Circle(radius) -> 3.14159 * radius * radius
        Rectangle(w, h) -> w * h
        Point -> 0.0
    }
}
```

## Parameterized Types (Generics)

```rust
// Generic Type
type Maybe<t> = Just(t) | Nothing

// Usage
x: Maybe<Int> = Just(42)
y: Maybe<String> = Nothing

// Multiple parameters
type Either<a, b> = Left(a) | Right(b)

// Recursive Types
type Tree<t> = Leaf(t) | Branch(Tree<t>, Tree<t>)
```

## Generic Functions

```rust
import "lib/tuple" (*)

fun myIdentity<t>(x: t) -> t {
    x
}

fun swap<a, b>(pair: (a, b)) -> (b, a) {
    (snd(pair), fst(pair))
}

fun map<a, b>(f: (a) -> b, list: List<a>) -> List<b> {
    [f(x) | x <- list]
}
```

## Runtime Type Checking

```rust
typeOf(42, Int)        // true
typeOf("hi", String)   // true
typeOf(42, String)     // false

getType(42)            // type(Int)

// For parameterized types
typeOf(Just(1), Maybe)       // true (without param)
typeOf(Just(1), Maybe(Int))  // true (with param)
```

## Flow-Sensitive Typing

The compiler narrows types inside `if` blocks.

```rust
fun process(val: Int | String) {
    if typeOf(val, Int) {
        // val is narrowed to Int here
        print(val * 2)
    } else {
        // val is String here
        print("Length: " ++ show(len(val)))
    }
}
```

## Kinds (Types of Types)

Just as values have types (e.g., `42` has type `Int`), types have "kinds".

*   `*` (Star): A proper type that has values. Examples: `Int`, `String`, `List<Int>`.
*   `* -> *`: A type constructor taking one argument. Example: `List`, `Maybe`.
*   `* -> * -> *`: A type constructor taking two arguments. Example: `Map`, `Either`.

### Kind Annotations

You can explicitly annotate kinds in type parameters. This is useful for Higher-Kinded Types (HKT).

```rust
// f must be a type constructor (* -> *)
trait Mappable<f: * -> *> {
    fun fmap<a, b>(f: (a) -> b, fa: f<a>) -> f<b>
}
```

### Kind Inference

Funxy usually infers kinds automatically:
*   `List<t>` -> `List` is `* -> *`
*   `f<a>` -> `f` is `* -> *`
