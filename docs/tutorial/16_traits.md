# Iteration 8: Traits (Type Classes)

Traits define shared behavior for different types.

## Trait Declaration

A `trait` defines a set of function signatures that a type must implement. Generic type parameters are specified in angle brackets `<t>`.

```rust
trait MyShow<t> {
    fun show(val: t) -> String
}
```

## Instance (Implementation)

The `instance` keyword implements a trait for a specific type.

```rust
trait MyShow<t> {
    fun show(val: t) -> String
}

instance MyShow Int {
    fun show(val: Int) -> String {
        "Int"
    }
}
```

## Default Implementations

Methods in a trait can have **default implementations**. If an instance doesn't override a method, the default is used.

```rust
trait MyCmp<t> {
    // Required method - must be implemented
    fun eq(a: t, b: t) -> Bool

    // Default implementation - uses eq()
    fun neq(a: t, b: t) -> Bool {
        if eq(a, b) { false } else { true }
    }
}

// Instance only needs to implement required methods
instance MyCmp Int {
    fun eq(a: Int, b: Int) -> Bool {
        a == b
    }
}
// neq is automatically available using the default!

print(eq(1, 1))      // true
print(neq(1, 2))     // true (default impl)
```

### Missing Required Methods

If you forget to implement a required method (one without a default), you get a **compile-time error**:

```rust
// This block is expected to fail compilation
// instance MyCmp Int {}
// ERROR: instance MyCmp for Int is missing required method 'eq'
```

### Overriding Defaults

You can override a default implementation if needed:

```rust
import "lib/math" (abs)

trait MyCmp<t> {
    fun eq(a: t, b: t) -> Bool
    fun neq(a: t, b: t) -> Bool {
        if eq(a, b) { false } else { true }
    }
}

instance MyCmp Float {
    fun eq(a: Float, b: Float) -> Bool {
        // Custom epsilon comparison
        abs(a - b) < 0.0001
    }

    // Override default with custom implementation
    fun neq(a: Float, b: Float) -> Bool {
        abs(a - b) >= 0.0001
    }
}
```

### Empty Instance Body

If a trait has **all methods with defaults**, the instance body can be empty:

```rust
trait SomeFullyDefaultTrait<t> {
    fun method1(x: t) -> Int { 42 }
    fun method2(x: t) -> Bool { true }
}

instance SomeFullyDefaultTrait Int {}
```

## Operator Methods (Operator Overloading)

Traits can define **operator methods** using the `operator` keyword. In Funxy, standard operators like `+` and `==` are associated with built-in traits (`Numeric`, `Equal`). To use them with your types, you implement these traits.

### Implementing Standard Operators

```rust
// Custom type wrapper around Int
type MyInt = MkMyInt Int

fun unbox(m: MyInt) -> Int {
    match m { MkMyInt x -> x }
}

// Implement Numeric to enable +, -, *, /
instance Numeric MyInt {
    operator (+)(a: MyInt, b: MyInt) -> MyInt {
        MkMyInt(unbox(a) + unbox(b))
    }

    operator (-)(a: MyInt, b: MyInt) -> MyInt {
        MkMyInt(unbox(a) - unbox(b))
    }

    operator (*)(a: MyInt, b: MyInt) -> MyInt {
        MkMyInt(unbox(a) * unbox(b))
    }

    operator (/)(a: MyInt, b: MyInt) -> MyInt {
        MkMyInt(unbox(a) / unbox(b))
    }

    operator (%)(a: MyInt, b: MyInt) -> MyInt {
        MkMyInt(unbox(a) % unbox(b))
    }

    operator (**)(a: MyInt, b: MyInt) -> MyInt {
        MkMyInt(unbox(a) ** unbox(b))
    }
}

// Now arithmetic works for MyInt!
v1 = MkMyInt(10)
v2 = MkMyInt(20)
v3 = v1 + v2        // MkMyInt(30)
print(unbox(v3))    // 30
```

### Supported Operators

| Operator | Description | Typical Trait |
|----------|-------------|---------------|
| `+` | Addition | `Numeric<t>` |
| `-` | Subtraction | `Numeric<t>` |
| `*` | Multiplication | `Numeric<t>` |
| `/` | Division | `Numeric<t>` |
| `%` | Modulo | `Numeric<t>` |
| `**` | Power | `Numeric<t>` |
| `==` | Equality | `Equal<t>` |
| `!=` | Inequality | `Equal<t>` (default) |
| `<`, `>`, `<=`, `>=` | Comparison | `Order<t>` |
| `&`, `\|`, `^` | Bitwise AND/OR/XOR | `Bitwise<t>` |
| `<<`, `>>` | Bit shift | `Bitwise<t>` |

### Built-in Operators (No Trait Needed)

These operators work on built-in types without requiring trait implementations:

| Operator | Types | Description |
|----------|-------|-------------|
| `&&` | `Bool` | Logical AND (short-circuit) |
| `\|\|` | `Bool` | Logical OR (short-circuit) |
| `++` | `List<t>`, `String` | Concatenation |
| `::` | `t`, `List<t>` | Cons (prepend element, right-associative) |
| `\|>` | `t`, `(t) -> r` | Pipe (apply function) |

```rust
// Logical operators (short-circuit)
true && false   // false - second not evaluated if first is false
false || true   // true - second not evaluated if first is true

// Concatenation
[1, 2] ++ [3, 4]   // [1, 2, 3, 4]
"Hello" ++ " World" // "Hello World"

// Cons (right-associative)
1 :: [2, 3]        // [1, 2, 3]
1 :: 2 :: 3 :: []  // [1, 2, 3] - no parens needed!

// Pipe operator (lowest precedence, left-associative)
// x |> f  is equivalent to  f(x)
fun double(x) { x * 2 }
fun inc(x) { x + 1 }

5 |> double          // 10
3 |> inc |> double   // 8 (double(inc(3)))
2 + 3 |> double      // 10 (double(2 + 3))
```

### Operators as Functions

Any operator can be used as a function by wrapping it in parentheses:

```rust
// Assign operator to variable
add = (+)
print(add(1, 2))  // 3

// Pass to higher-order functions
fun fold<t, r>(f: (r, t) -> r, init: r, list: List<t>) -> r {
    match list {
        [] -> init
        [head, ...tail] -> fold(f, f(init, head), tail)
    }
}

sum = fold((+), 0, [1, 2, 3, 4, 5])     // 15
product = fold((*), 1, [1, 2, 3, 4])    // 24

// With zipWith
fun zipWith<a, b, c>(f: (a, b) -> c, xs: List<a>, ys: List<b>) -> List<c> {
    match (xs, ys) {
        ([], _) -> []
        (_, []) -> []
        ([x, ...xs2], [y, ...ys2]) -> f(x, y) :: zipWith(f, xs2, ys2)
    }
}

sums = zipWith((+), [1, 2, 3], [10, 20, 30])  // [11, 22, 33]
```

Operator function types include constraints:

| Operator | Type |
|----------|------|
| `(+)`, `(-)`, `(*)`, `(/)`, `(%)`, `(**)` | `<t: Numeric>(t, t) -> t` |
| `(==)`, `(!=)` | `<t: Equal>(t, t) -> Bool` |
| `(<)`, `(>)`, `(<=)`, `(>=)` | `<t: Order>(t, t) -> Bool` |
| `(&)`, `(\|)`, `(^)`, `(<<)`, `(>>)` | `<t: Bitwise>(t, t) -> t` |
| `(&&)`, `(\|\|)` | `(Bool, Bool) -> Bool` |
| `(++)` | `(t, t) -> t` |
| `(::)` | `<t>(t, List<t>) -> List<t>` |

### How It Works

1. When the analyzer sees `a + b` where `a` has type `T`:
   - First, check if there's an `Add` trait with `+` operator
   - If `instance Numeric T` exists, allow the operation
   - Otherwise, fall back to built-in numeric types

2. At runtime, the evaluator dispatches `+` through the trait system

## Custom Traits with User Operators

To define a custom trait that uses a user-defined operator (like `<|>`), you inherit from the corresponding `UserOp` trait.

### Pattern: Trait Inheritance for Operators

1.  **Define your trait** inheriting from the relevant UserOp trait (e.g., `UserOpChoose` for `<|>`).
2.  **Implement** the UserOp trait for your type to define the operator's logic.
3.  **Implement** your custom trait for your type.

```rust
// 1. Define custom trait inheriting the operator requirement
trait MyAlternative<t> : UserOpChoose<t> {
    // Add custom methods if needed
    fun empty() -> t
}

type Box = MkBox Int
fun unbox(b) { match b { MkBox x -> x } }

// 2. Implement the operator logic (Required by UserOpChoose)
instance UserOpChoose Box {
    operator (<|>)(a: Box, b: Box) -> Box {
        // Example: choose the larger value
        match (a, b) {
            (MkBox x, MkBox y) -> if x > y { a } else { b }
        }
    }
}

// 3. Implement the custom trait
instance MyAlternative Box {
    fun empty() -> Box { MkBox(0) }
}

// Usage
b1 = MkBox(10)
b2 = MkBox(20)

// Uses UserOpChoose.<|>
print(unbox(b1 <|> b2))  // 20
```

## Trait Inheritance

Traits can inherit from other traits using the `:` syntax. A derived trait requires all super traits to be implemented first.

```rust
type Ordering = Lt | Eq | Gt

// Base trait for equality comparison
trait MyCmp<t> {
    fun eq(a: t, b: t) -> Bool
}

// Order inherits from Cmp - any type that implements Order
// must also implement Cmp
trait MyOrder<t> : MyCmp<t> {
    fun compare(a: t, b: t) -> Ordering
}
```

### Implementing Inherited Traits

When implementing a trait with super traits, **you must implement the super traits first**:

```rust
type Ordering = Lt | Eq | Gt

trait MyCmp<t> {
    fun eq(a: t, b: t) -> Bool
}

trait MyOrder<t> : MyCmp<t> {
    fun compare(a: t, b: t) -> Ordering
}

// First: implement Cmp for Int
instance MyCmp Int {
    fun eq(a: Int, b: Int) -> Bool {
        a == b
    }
}

// Then: implement Order for Int (requires Cmp Int)
instance MyOrder Int {
    fun compare(a: Int, b: Int) -> Ordering {
        if a < b { Lt }
        else if a > b { Gt }
        else { Eq }
    }
}
```

If you try to implement `Order` without implementing `Cmp` first, you get an error:

```rust
// This example is intentionally commented out to pass doc checks
// ERROR: cannot implement Order for Int: missing implementation of super trait Cmp
// instance MyOrder Int {
//     fun compare(a: Int, b: Int) -> Ordering { Lt }
// }
```

### Multiple Super Traits

A trait can inherit from multiple traits, separated by commas:

```rust
trait MyShow<t> {
    fun show(val: t) -> String
}

trait MyCmp<t> {
    fun eq(a: t, b: t) -> Bool
}

// MyPrintable requires BOTH MyShow AND MyCmp
trait MyPrintable<t> : MyShow<t>, MyCmp<t> {
    fun pretty(val: t) -> String
}
```

To implement `Printable`, you must first implement **all** super traits:

```rust
trait MyShow<t> {
    fun show(val: t) -> String
}

trait MyCmp<t> {
    fun eq(a: t, b: t) -> Bool
}

trait MyPrintable<t> : MyShow<t>, MyCmp<t> {
    fun pretty(val: t) -> String
}

// Step 1: Implement Show
instance MyShow Int {
    fun show(val: Int) -> String {
        "Int"
    }
}

// Step 2: Implement Cmp
instance MyCmp Int {
    fun eq(a: Int, b: Int) -> Bool {
        a == b
    }
}

// Step 3: Now Printable can be implemented
instance MyPrintable Int {
    fun pretty(val: Int) -> String {
        "Formatted: " ++ show(val)
    }
}

// Usage
print(show(42))        // Int
print(eq(1, 1))        // true
print(pretty(100))     // Formatted: Int
```

If any super trait is missing, you get an error:

```rust
// This example is intentionally commented out to pass doc checks
// Only Show is implemented, Cmp is NOT
// instance MyShow Int { ... }

// ERROR: cannot implement Printable for Int: missing implementation of super trait Cmp
// instance MyPrintable Int { ... }
```

## Usage (Constraints)

You can constrain generic parameters to types that implement a specific trait using `<t: TraitName>` syntax.

```rust
trait MyShow<t> {
    fun show(val: t) -> String
}

instance MyShow Int {
    fun show(val: Int) -> String { "Int" }
}

instance MyShow Bool {
    fun show(val: Bool) -> String {
        if val { "true" } else { "false" }
    }
}

// Constrained function - t must implement MyShow
fun display<t: MyShow>(x: t) -> String {
    show(x)
}

// Works - Int implements MyShow
print(display(42))      // Int

// Works - Bool implements MyShow
print(display(true))    // true

// ERROR: String does not implement MyShow
// print(display("hello"))
// Compile error: type (List Char) does not implement trait MyShow
```

### Multiple Constraints

A type parameter can have multiple constraints. Each constraint is specified separately:

```rust
trait MyShow<t> {
    fun show(val: t) -> String
}

trait MyCmp<t> {
    fun eq(a: t, b: t) -> Bool
}

// t must implement BOTH MyShow AND MyCmp
fun process<t: MyShow, t: MyCmp>(x: t, y: t) -> String {
    if eq(x, y) { show(x) } else { "different" }
}

// Int implements both
instance MyShow Int { fun show(val: Int) -> String { "Int" } }
instance MyCmp Int { fun eq(a: Int, b: Int) -> Bool { a == b } }

print(process(5, 5))   // Int
print(process(1, 2))   // different

// Bool only implements MyShow, NOT MyCmp
// instance MyShow Bool { ... }

// ERROR: type Bool does not implement trait MyCmp
// print(process(true, false))
```

## Multi-Parameter Traits

Traits can have multiple type parameters:

```rust
trait MyIter<c, t> {
    fun iter(collection: c) -> () -> Option<t>
}
```

This is used for iteration — `c` is the collection type, `t` is the element type.

## Functional Dependencies

When using multi-parameter traits, type inference can sometimes be ambiguous. Functional dependencies allow you to specify that one type parameter determines another, aiding the compiler in type inference.

Syntax: `trait Name<a, b> | a -> b`

This means "a determines b". For every unique `a`, there can be only one `b`.

### Example: Type Conversion

Consider a conversion trait:

```rust
trait Convert<from, to> | from -> to {
    fun convert(val: from) -> to
}

instance Convert<Int, String> {
    fun convert(val: Int) -> String {
        "Int: " ++ show(val)
    }
}
```

With the dependency `from -> to`, if the compiler knows `from` is `Int`, it knows `to` must be `String` (based on the visible instances). This avoids ambiguity if `convert(42)` is called where the return type isn't explicitly known.

## Built-in Traits

The language provides several built-in traits that are automatically implemented for primitive types.

### Core Traits

| Trait | Kind | Operators/Methods | Description |
|-------|------|-------------------|-------------|
| `Equal<t>` | `*` | `==`, `!=` | Equality comparison |
| `Order<t> : MyEqual<t>` | `*` | `<`, `>`, `<=`, `>=` | Ordering (inherits Equal) |
| `Numeric<t>` | `*` | `+`, `-`, `*`, `/`, `%`, `**` | Numeric operations |
| `Bitwise<t>` | `*` | `&`, `\|`, `^`, `<<`, `>>` | Bitwise operations |
| `Concat<t>` | `*` | `++` | Concatenation |
| `Default<t>` | `*` | `default(Type)` | Default value for type |
| `Iter<c, t>` | `*` | `iter` method | Make type iterable in `for` loops |
| `Functor<f>` | `* -> *` | `fmap` | Mappable containers (HKT) |

### Primitive Type Implementations

| Type | Equal | Order | Numeric | Bitwise | Default |
|------|-------|-------|--------|---------|---------|
| `Int` | ✓ | ✓ | ✓ | ✓ | `0` |
| `Float` | ✓ | ✓ | ✓ | — | `0.0` |
| `BigInt` | ✓ | ✓ | ✓ | ✓ | `0n` |
| `Rational` | ✓ | ✓ | ✓ | — | `0r` |
| `Bool` | ✓ | ✓ (`false < true`) | — | — | `false` |
| `Char` | ✓ | ✓ | — | — | `'\0'` |
| `String` | ✓ | ✓ (lexicographic) | — | — | `""` |
| `List<T>` | — | — | — | — | `[]` |
| `Nil` | — | — | — | — | `nil` |

Note: `String` and `List<t>` implement `Concat` for the `++` operator.

### Using Constraints with Primitives

Because primitives implement these traits, you can write generic functions that work with them:

```rust
// Works with Int, Float, BigInt, Rational
fun max<t: Order>(a: t, b: t) -> t {
    if a > b { a } else { b }
}

print(max(10, 20))      // 20
print(max(3.5, 2.1))    // 3.5

// Works with any Numeric type
fun sum<t: Numeric>(a: t, b: t, c: t) -> t {
    a + b + c
}

print(sum(1, 2, 3))        // 6
print(sum(1.0, 2.0, 3.0))  // 6.0

// Works with any Equal type
fun allEqual<t: Equal>(a: t, b: t, c: t) -> Bool {
    a == b && b == c
}

print(allEqual(5, 5, 5))   // true
```

### The `default` Function

The `default(Type)` function returns the default value for types that implement `Default`:

```rust
print(default(Int))     // 0
print(default(Float))   // 0.0
print(default(Bool))    // false
print(default(String))  // ""
```

This is useful for initializing values or providing fallbacks:

```rust
fun getOrDefault<t: Default>(opt: Option<t>) -> t {
    match opt {
        Some(x) -> x
        None -> default(t)
    }
}
```

## Higher-Kinded Types (HKT)

Higher-Kinded Types allow traits to work with type constructors (like `Option`, `List`, `Result`) rather than just concrete types (like `Int`, `String`).

### What are Kinds?

Every type has a **kind** that describes its arity:

| Type | Kind | Description |
|------|------|-------------|
| `Int`, `Bool`, `String` | `*` | Concrete types (0 type parameters) |
| `Option<t>`, `List<t>` | `* -> *` | Type constructors (1 type parameter) |
| `Result<e, a>` | `* -> * -> *` | Type constructors (2 type parameters, error first) |

### The Functor Trait

`Functor` is a built-in HKT trait for types that can be "mapped over":

```rust
// Functor is built-in - no need to define it!
// Its signature is:
// trait Functor<f> {
//     fun fmap<a, b>(f: (a) -> b, fa: f<a>) -> f<b>
// }
```

Note: `f` here is a type constructor (kind `* -> *`), not a concrete type.

### Implementing Functor

```
// Option has kind * -> *
instance Functor<Option> {
    fun fmap<a, b>(f: (a) -> b, fa: Option<a>) -> Option<b> {
        match fa {
            Some(x) -> Some(f(x))
            None -> None
        }
    }
}

// List has kind * -> *
instance Functor<List> {
    fun fmap<a, b>(f: (a) -> b, fa: List<a>) -> List<b> {
        match fa {
            [] -> []
            [x, ...xs] -> f(x) :: fmap(f, xs)
        }
    }
}

// Usage
print(fmap(fun(x) -> x * 2, Some(21)))   // Some(42)
print(fmap(fun(x) -> x * 2, [1, 2, 3]))  // [2, 4, 6]
```

### Partial Type Application for Multi-Parameter Types

`Result<e, a>` has kind `* -> * -> *` (two type parameters), but `Functor` expects kind `* -> *`. Use partial type application:

```
// Result<e, _> fixes e, leaving one parameter - kind becomes * -> *
instance Functor<Result, e> {
    fun fmap<e, a, b>(f: (a) -> b, fa: Result<e, a>) -> Result<e, b> {
        match fa {
            Ok(x) -> Ok(f(x))
            Fail(e) -> Fail(e)  // Error is preserved
        }
    }
}

print(fmap(fun(x) -> x * 2, Ok(50)))       // Ok(100)
print(fmap(fun(x) -> x * 2, Fail("err")))  // Fail("err")
```

### Kind Checking

The compiler automatically detects HKT traits and enforces correct kinds:

```
// ERROR: Int has kind *, but Functor requires kind * -> *
instance Functor<Int> {
    fun fmap<a, b>(f: (a) -> b, fa: Int) -> Int { fa }
}
// Compile error: type Int has kind *, but trait Functor requires kind * -> * (type constructor)
```

### Constraints with HKT

Use constraints to write generic functions that work with any Functor:

```rust
// Inline constraint syntax
fun double<f: Functor>(fa: f<Int>) -> f<Int> {
    fmap(fun(x) -> x * 2, fa)
}

// Works with any Functor
print(double(Some(21)))     // Some(42)
print(double([1, 2, 3]))    // [2, 4, 6]
print(double(Ok(50)))       // Ok(100)
```

### Custom HKT Traits

You can define your own HKT traits:

```rust
trait Bifunctor<b> {
    fun bimap<a, c, d, e>(f: (a) -> c, g: (d) -> e, x: b<a, d>) -> b<c, e>
}

instance Bifunctor<Result> {
    fun bimap<a, c, d, e>(f: (a) -> c, g: (d) -> e, x: Result<a, d>) -> Result<c, e> {
        match x {
            Ok(d) -> Ok(g(d))
            Fail(a) -> Fail(f(a))
        }
    }
}

// Map both success and error values
res = bimap(fun(e) -> e ++ "!", fun(x) -> x * 2, Fail("err"))
print(res)  // Fail("err!")
```

### How HKT Detection Works

The compiler automatically determines if a trait is HKT by analyzing its method signatures:

- If a type parameter `f` is used as `f<a>` (applied to arguments), the trait is HKT
- If a type parameter `t` is used directly as `t`, it's not HKT

```
// HKT trait - f is applied to a
trait Functor<f> {
    fun fmap<a, b>(f: (a) -> b, fa: f<a>) -> f<b>
}

// NOT HKT - t is used directly
trait MyShow<t> {
    fun show(x: t) -> String
}
```

## Functional Operators

The language provides special operators for functional programming patterns.

### Pipe Operator (`|>`)

The pipe operator passes a value to a function, allowing left-to-right data flow:

```rust
import "lib/list" (map, filter)

// Without pipe - nested calls
print(show(1 + 2))

// With pipe - left-to-right flow
1 + 2 |> show |> print

// Works with any function
[1, 2, 3] |> map(fun(x) -> x * 2) |> filter(fun(x) -> x > 2) |> print
// [4, 6]
```

The pipe operator has the **lowest precedence**, so it applies last.

### Composition Operator (`,,`)

The composition operator creates a new function by composing two functions (mathematical style):

```rust
// (f ,, g)(x) = f(g(x)) — g is applied first, then f
inc = fun(x) -> x + 1
double = fun(x) -> x * 2

// Compose: first increment, then double
incThenDouble = double ,, inc  // double(inc(x))

print(incThenDouble(5))  // 12 = (5 + 1) * 2

// Compare with reversed order
doubleThenInc = inc ,, double  // inc(double(x))
print(doubleThenInc(5))  // 11 = (5 * 2) + 1
```

**Note**: Composition is right-to-left like in mathematics: `(f ,, g)(x) = f(g(x))`.

This is the opposite of the pipe operator:
- `x |> f |> g` = `g(f(x))` — left-to-right data flow
- `(f ,, g)(x)` = `f(g(x))` — right-to-left composition

To get left-to-right composition, reverse the order: `g ,, f`.

## User-Definable Operators

Beyond the built-in operators, the language provides **fixed slots** for custom operators that can be used with user-defined types through the trait system.

### Available Operator Slots

| Operator | Trait | Precedence | Associativity | Typical Use |
|----------|-------|------------|---------------|-------------|
| `<>` | `UserOpCombine` | Low | Right | Semigroup combine |
| `<\|>` | `UserOpChoose` | Low | Left | Alternative choice |
| `<*>` | `UserOpApply` | Medium | Left | Applicative apply |
| `>>=` | `UserOpBind` | Low | Left | Monad bind |
| `<$>` | `UserOpMap` | High | Left | Functor map |
| `<:>` | `UserOpCons` | Medium | Right | Cons-like prepend |
| `<~>` | `UserOpSwap` | Medium | Left | Swap/Exchange |
| `=>` | `UserOpImply` | Low | Right | Implication |
| `$` | (built-in) | Lowest | Right | Function application |

### Example: Semigroup-like Combine

```rust
// Define a custom type
type Text = MkText String

fun getText(t: Text) -> String {
    match t { MkText s -> s }
}

// Implement <> operator via Semigroup
instance Semigroup Text {
    operator (<>)(a: Text, b: Text) -> Text {
        match (a, b) {
            (MkText x, MkText y) -> MkText(x ++ y)
        }
    }
}

// Use the operator
t1 = MkText("Hello")
t2 = MkText(" World")
result = t1 <> t2
print(getText(result))  // Hello World
```

### Example: Low-Precedence Application ($)

The `$` operator is built-in for function application with lowest precedence (right-associative):

```rust
fun double(x: Int) -> Int { x * 2 }
fun add10(x: Int) -> Int { x + 10 }

// f $ x = f(x)
double $ 21  // 42

// Right-associative: f $ g $ x = f(g(x))
add10 $ double $ 5  // add10(double(5)) = 20

// Works with function composition
composed = add10 ,, double
composed $ 5  // add10(double(5)) = 20
```

### Using User Operators as Functions

User-defined operators can be used as functions by wrapping them in parentheses:

```rust
type Text = MkText String

fun getText(t: Text) -> String {
    match t { MkText s -> s }
}

instance Semigroup Text {
    operator (<>)(a: Text, b: Text) -> Text {
        match (a, b) {
            (MkText x, MkText y) -> MkText(x ++ y)
        }
    }
}

combine = (<>)
result = combine(MkText("A"), MkText("B"))
print(getText(result))  // AB
```

## Compile-Time Checking

Constraints are checked at **compile time**. If you call a constrained function with a type that doesn't implement the required trait, you get a clear error message:

```
// If String doesn't implement MyShow:
display("hello")
// Error: type (List Char) does not implement trait MyShow
```

This catches errors early, before your program runs.

## Summary

| Feature | Syntax | Example |
|---------|--------|---------|
| Declare trait | `trait Name<t> { ... }` | `trait MyShow<t> { fun show(val: t) -> String }` |
| Inherit trait | `trait Name<t> : Super<t>` | `trait MyOrder<t> : MyEqual<t> { ... }` |
| Implement | `instance Name Type { ... }` | `instance MyShow Int { ... }` |
| Constrain | `<t: Trait>` | `fun f<t: Show>(x: t)` |
| Operator method | `operator (+)(a: t, b: t) -> t` | `instance Numeric t { operator (+)(...) }` |
| Default impl | Body in trait | `fun notEqual(...) { ... }` |
| User operator | `instance UserOpXxx Type` | `instance UserOpChoose Box { operator (<>)... }` |
