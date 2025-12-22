# Iteration 8: Traits (Type Classes)

Traits define shared behavior for different types.

## Trait Declaration

A `trait` defines a set of function signatures that a type must implement. Generic type parameters are specified in angle brackets `<T>`.

```rust
trait MyShow<T> {
    fun show(val: T) -> String
}
```

## Instance (Implementation)

The `instance` keyword implements a trait for a specific type.

```rust
trait MyShow<T> {
    fun show(val: T) -> String
}

instance MyShow Int {
    fun show(val: Int) -> String {
        "int"
    }
}
```

## Default Implementations

Methods in a trait can have **default implementations**. If an instance doesn't override a method, the default is used.

```rust
trait MyCmp<T> {
    // Required method - must be implemented
    fun eq(a: T, b: T) -> Bool

    // Default implementation - uses eq()
    fun neq(a: T, b: T) -> Bool {
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

trait MyCmp<T> {
    fun eq(a: T, b: T) -> Bool
    fun neq(a: T, b: T) -> Bool {
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
trait SomeFullyDefaultTrait<T> {
    fun method1(x: T) -> Int { 42 }
    fun method2(x: T) -> Bool { true }
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
| `+` | Addition | `Numeric<T>` |
| `-` | Subtraction | `Numeric<T>` |
| `*` | Multiplication | `Numeric<T>` |
| `/` | Division | `Numeric<T>` |
| `%` | Modulo | `Numeric<T>` |
| `**` | Power | `Numeric<T>` |
| `==` | Equality | `Equal<T>` |
| `!=` | Inequality | `Equal<T>` (default) |
| `<`, `>`, `<=`, `>=` | Comparison | `Order<T>` |
| `&`, `\|`, `^` | Bitwise AND/OR/XOR | `Bitwise<T>` |
| `<<`, `>>` | Bit shift | `Bitwise<T>` |

### Built-in Operators (No Trait Needed)

These operators work on built-in types without requiring trait implementations:

| Operator | Types | Description |
|----------|-------|-------------|
| `&&` | `Bool` | Logical AND (short-circuit) |
| `\|\|` | `Bool` | Logical OR (short-circuit) |
| `++` | `List<T>`, `String` | Concatenation |
| `::` | `T`, `List<T>` | Cons (prepend element, right-associative) |
| `\|>` | `T`, `(T) -> R` | Pipe (apply function) |

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
fun fold<T, R>(f: (R, T) -> R, init: R, list: List<T>) -> R {
    match list {
        [] -> init
        [head, tail...] -> fold(f, f(init, head), tail)
    }
}

sum = fold((+), 0, [1, 2, 3, 4, 5])     // 15
product = fold((*), 1, [1, 2, 3, 4])    // 24

// With zipWith
fun zipWith<A, B, C>(f: (A, B) -> C, xs: List<A>, ys: List<B>) -> List<C> {
    match (xs, ys) {
        ([], _) -> []
        (_, []) -> []
        ([x, xs2...], [y, ys2...]) -> f(x, y) :: zipWith(f, xs2, ys2)
    }
}

sums = zipWith((+), [1, 2, 3], [10, 20, 30])  // [11, 22, 33]
```

Operator function types include constraints:

| Operator | Type |
|----------|------|
| `(+)`, `(-)`, `(*)`, `(/)`, `(%)`, `(**)` | `<T: Numeric>(T, T) -> T` |
| `(==)`, `(!=)` | `<T: Equal>(T, T) -> Bool` |
| `(<)`, `(>)`, `(<=)`, `(>=)` | `<T: Order>(T, T) -> Bool` |
| `(&)`, `(\|)`, `(^)`, `(<<)`, `(>>)` | `<T: Bitwise>(T, T) -> T` |
| `(&&)`, `(\|\|)` | `(Bool, Bool) -> Bool` |
| `(++)` | `(T, T) -> T` |
| `(::)` | `<T>(T, List<T>) -> List<T>` |

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
trait MyAlternative<T> : UserOpChoose<T> {
    // Add custom methods if needed
    fun empty() -> T
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
trait MyCmp<T> {
    fun eq(a: T, b: T) -> Bool
}

// Order inherits from Cmp - any type that implements Order
// must also implement Cmp
trait MyOrder<T> : MyCmp<T> {
    fun compare(a: T, b: T) -> Ordering
}
```

### Implementing Inherited Traits

When implementing a trait with super traits, **you must implement the super traits first**:

```rust
type Ordering = Lt | Eq | Gt

trait MyCmp<T> {
    fun eq(a: T, b: T) -> Bool
}

trait MyOrder<T> : MyCmp<T> {
    fun compare(a: T, b: T) -> Ordering
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
trait MyShow<T> {
    fun show(val: T) -> String
}

trait MyCmp<T> {
    fun eq(a: T, b: T) -> Bool
}

// MyPrintable requires BOTH MyShow AND MyCmp
trait MyPrintable<T> : MyShow<T>, MyCmp<T> {
    fun format(val: T) -> String
}
```

To implement `Printable`, you must first implement **all** super traits:

```rust
trait MyShow<T> {
    fun show(val: T) -> String
}

trait MyCmp<T> {
    fun eq(a: T, b: T) -> Bool
}

trait MyPrintable<T> : MyShow<T>, MyCmp<T> {
    fun format(val: T) -> String
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
    fun format(val: Int) -> String {
        "Formatted: " ++ show(val)
    }
}

// Usage
print(show(42))        // Int
print(eq(1, 1))        // true
print(format(100))     // Formatted: Int
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

You can constrain generic parameters to types that implement a specific trait using `<T: TraitName>` syntax.

```rust
trait MyShow<T> {
    fun show(val: T) -> String
}

instance MyShow Int {
    fun show(val: Int) -> String { "int" }
}

instance MyShow Bool {
    fun show(val: Bool) -> String {
        if val { "true" } else { "false" }
    }
}

// Constrained function - T must implement MyShow
fun display<T: MyShow>(x: T) -> String {
    show(x)
}

// Works - Int implements MyShow
print(display(42))      // int

// Works - Bool implements MyShow
print(display(true))    // true

// ERROR: String does not implement MyShow
// print(display("hello"))
// Compile error: type (List Char) does not implement trait MyShow
```

### Multiple Constraints

A type parameter can have multiple constraints. Each constraint is specified separately:

```rust
trait MyShow<T> {
    fun show(val: T) -> String
}

trait MyCmp<T> {
    fun eq(a: T, b: T) -> Bool
}

// T must implement BOTH MyShow AND MyCmp
fun process<T: MyShow, T: MyCmp>(x: T, y: T) -> String {
    if eq(x, y) { show(x) } else { "different" }
}

// Int implements both
instance MyShow Int { fun show(val: Int) -> String { "int" } }
instance MyCmp Int { fun eq(a: Int, b: Int) -> Bool { a == b } }

print(process(5, 5))   // int
print(process(1, 2))   // different

// Bool only implements MyShow, NOT MyCmp
// instance MyShow Bool { ... }

// ERROR: type Bool does not implement trait MyCmp
// print(process(true, false))
```

## Multi-Parameter Traits

Traits can have multiple type parameters:

```rust
trait MyIter<C, T> {
    fun iter(collection: C) -> () -> Option<T>
}
```

This is used for iteration — `C` is the collection type, `T` is the element type.

## Built-in Traits

The language provides several built-in traits that are automatically implemented for primitive types.

### Core Traits

| Trait | Kind | Operators/Methods | Description |
|-------|------|-------------------|-------------|
| `Equal<T>` | `*` | `==`, `!=` | Equality comparison |
| `Order<T> : MyEqual<T>` | `*` | `<`, `>`, `<=`, `>=` | Ordering (inherits Equal) |
| `Numeric<T>` | `*` | `+`, `-`, `*`, `/`, `%`, `**` | Numeric operations |
| `Bitwise<T>` | `*` | `&`, `\|`, `^`, `<<`, `>>` | Bitwise operations |
| `Concat<T>` | `*` | `++` | Concatenation |
| `Default<T>` | `*` | `default(Type)` | Default value for type |
| `Iter<C, T>` | `*` | `iter` method | Make type iterable in `for` loops |
| `Functor<F>` | `* -> *` | `fmap` | Mappable containers (HKT) |

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

Note: `String` and `List<T>` implement `Concat` for the `++` operator.

### Using Constraints with Primitives

Because primitives implement these traits, you can write generic functions that work with them:

```rust
// Works with Int, Float, BigInt, Rational
fun max<T: Order>(a: T, b: T) -> T {
    if a > b { a } else { b }
}

print(max(10, 20))      // 20
print(max(3.5, 2.1))    // 3.5

// Works with any Numeric type
fun sum<T: Numeric>(a: T, b: T, c: T) -> T {
    a + b + c
}

print(sum(1, 2, 3))        // 6
print(sum(1.0, 2.0, 3.0))  // 6.0

// Works with any Equal type
fun allEqual<T: Equal>(a: T, b: T, c: T) -> Bool {
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
fun getOrDefault<T: Default>(opt: Option<T>) -> T {
    match opt {
        Some(x) -> x
        Zero -> default(T)
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
| `Option<T>`, `List<T>` | `* -> *` | Type constructors (1 type parameter) |
| `Result<E, A>` | `* -> * -> *` | Type constructors (2 type parameters, error first) |

### The Functor Trait

`Functor` is a built-in HKT trait for types that can be "mapped over":

```rust
// Functor is built-in - no need to define it!
// Its signature is:
// trait Functor<F> {
//     fun fmap<A, B>(f: (A) -> B, fa: F<A>) -> F<B>
// }
```

Note: `F` here is a type constructor (kind `* -> *`), not a concrete type.

### Implementing Functor

```
// Option has kind * -> *
instance Functor<Option> {
    fun fmap<A, B>(f: (A) -> B, fa: Option<A>) -> Option<B> {
        match fa {
            Some(x) -> Some(f(x))
            Zero -> Zero
        }
    }
}

// List has kind * -> *
instance Functor<List> {
    fun fmap<A, B>(f: (A) -> B, fa: List<A>) -> List<B> {
        match fa {
            [] -> []
            [x, xs...] -> f(x) :: fmap(f, xs)
        }
    }
}

// Usage
print(fmap(fun(x) -> x * 2, Some(21)))   // Some(42)
print(fmap(fun(x) -> x * 2, [1, 2, 3]))  // [2, 4, 6]
```

### Partial Type Application for Multi-Parameter Types

`Result<E, A>` has kind `* -> * -> *` (two type parameters), but `Functor` expects kind `* -> *`. Use partial type application:

```
// Result<E, _> fixes E, leaving one parameter - kind becomes * -> *
instance Functor<Result, E> {
    fun fmap<E, A, B>(f: (A) -> B, fa: Result<E, A>) -> Result<E, B> {
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
    fun fmap<A, B>(f: (A) -> B, fa: Int) -> Int { fa }
}
// Compile error: type Int has kind *, but trait Functor requires kind * -> * (type constructor)
```

### Constraints with HKT

Use constraints to write generic functions that work with any Functor:

```rust
// Inline constraint syntax
fun double<F: Functor>(fa: F<Int>) -> F<Int> {
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
trait Bifunctor<B> {
    fun bimap<A, C, D, E>(f: (A) -> C, g: (D) -> E, x: B<A, D>) -> B<C, E>
}

instance Bifunctor<Result> {
    fun bimap<A, C, D, E>(f: (A) -> C, g: (D) -> E, x: Result<D, A>) -> Result<E, C> {
        match x {
            Ok(a) -> Ok(f(a))
            Fail(d) -> Fail(g(d))
        }
    }
}

// Map both success and error values
res = bimap(fun(x) -> x * 2, fun(e) -> e ++ "!", Fail("err"))
print(res)  // Fail("err!")
```

### How HKT Detection Works

The compiler automatically determines if a trait is HKT by analyzing its method signatures:

- If a type parameter `F` is used as `F<A>` (applied to arguments), the trait is HKT
- If a type parameter `T` is used directly as `T`, it's not HKT

```
// HKT trait - F is applied to A
trait Functor<F> {
    fun fmap<A, B>(f: (A) -> B, fa: F<A>) -> F<B>
}

// NOT HKT - T is used directly
trait MyShow<T> {
    fun show(x: T) -> String
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
| Declare trait | `trait Name<T> { ... }` | `trait MyShow<T> { fun show(val: T) -> String }` |
| Inherit trait | `trait Name<T> : Super<T>` | `trait MyOrder<T> : MyEqual<T> { ... }` |
| Implement | `instance Name Type { ... }` | `instance MyShow Int { ... }` |
| Constrain | `<T: Trait>` | `fun f<T: Show>(x: T)` |
| Operator method | `operator (+)(a: T, b: T) -> T` | `instance Numeric T { operator (+)(...) }` |
| Default impl | Body in trait | `fun notEqual(...) { ... }` |
| User operator | `instance UserOpXxx Type` | `instance UserOpChoose Box { operator (<>)... }` |
