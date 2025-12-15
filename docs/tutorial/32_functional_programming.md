# Functional Programming Traits

This tutorial covers the built-in functional programming (FP) traits. These traits form a hierarchy of abstractions for composable data transformations.

> **Note**: FP traits are **built-in** and always available. No import required!

## The FP Trait Hierarchy

```
Semigroup
    ↓
  Monoid

Functor
    ↓
Applicative
    ↓
  Monad
```

Each level builds upon the previous, adding more capabilities.

## Semigroup: Combinable Values

A **Semigroup** provides an associative binary operation `<>` that combines two values of the same type.

### Trait Definition

The built-in trait is defined as:

```text
trait Semigroup<A> {
    operator (<>)(a: A, b: A) -> A
}
```

### Laws

The `<>` operation must be **associative**:
```
(a <> b) <> c == a <> (b <> c)
```

### Built-in Instances

**List**: Concatenation
```rust
[1, 2, 3] <> [4, 5, 6]  // [1, 2, 3, 4, 5, 6]
```

**Option**: First non-Zero wins
```rust
Some(10) <> Some(20)  // Some(10)
Some(10) <> Zero      // Some(10)
Zero <> Some(20)      // Some(20)
Zero <> Zero          // Zero
```

### Custom Instance Example

```rust
type Text = MkText String

instance Semigroup Text {
    operator (<>)(a: Text, b: Text) -> Text {
        match (a, b) {
            (MkText x, MkText y) -> MkText(x ++ y)
        }
    }
}

t1 = MkText("Hello")
t2 = MkText(" ")
t3 = MkText("World")
result = t1 <> t2 <> t3
// Note: We need a helper to extract the string because
// we can't pattern match on Text directly in print()
fun getText(t) { match t { MkText s -> s } }
print(getText(result)) // Hello World
```

## Monoid: Semigroup with Identity

A **Monoid** is a Semigroup with an identity element `mempty`.

### Trait Definition

```text
trait Monoid<A> : Semigroup<A> {
    fun mempty() -> A
}
```

### Laws

1. **Left identity**: `mempty <> x == x`
2. **Right identity**: `x <> mempty == x`

### Built-in Instances

**List**: Empty list is identity
```rust
mempty: List<Int>  // []
[] <> [1, 2, 3]    // [1, 2, 3]
[1, 2, 3] <> []    // [1, 2, 3]
```

**Option**: Zero is identity
```rust
mempty: Option<Int>  // Zero
Zero <> Some(42)     // Some(42)
```

### Custom Instance Example

```rust
type Text = MkText String
fun getText(t) { match t { MkText s -> s } }

instance Semigroup Text {
    operator (<>)(a: Text, b: Text) -> Text {
        match (a, b) {
            (MkText x, MkText y) -> MkText(x ++ y)
        }
    }
}

instance Monoid Text {
    fun mempty() -> Text { MkText("") }
}

// Type annotation needed for dispatch
t: Text = mempty()
print(getText(t)) // ""
```

## Functor: Mappable Containers

A **Functor** is a type constructor that can be mapped over with a function.

### Trait Definition

```text
trait Functor<F> {
    fun fmap<A, B>(f: (A) -> B, fa: F<A>) -> F<B>
}
```

### Laws

1. **Identity**: `fmap(id, x) == x`
2. **Composition**: `fmap(f ,, g, x) == fmap(f, fmap(g, x))`

### Built-in Instances

**List**: Map over each element
```rust
fmap(fun(x) -> x * 2, [1, 2, 3, 4, 5])  // [2, 4, 6, 8, 10]
```

**Option**: Map over Some, Zero stays Zero
```rust
fmap(fun(x) -> x * 2, Some(10))  // Some(20)
fmap(fun(x) -> x * 2, Zero)      // Zero
```

**Result**: Map over Ok, Fail stays Fail
```rust
fmap(fun(x) -> x + 100, Ok(5))           // Ok(105)
fmap(fun(x) -> x + 100, Fail("error"))   // Fail("error")
```

### Verifying Functor Laws

```rust
// Identity law
id = fun(x) -> x
print(fmap(id, [1, 2, 3]) == [1, 2, 3])  // true

// Composition law
inc = fun(x) -> x + 1
dbl = fun(x) -> x * 2
composed = dbl ,, inc  // dbl(inc(x))

opt = Some(5)
print(fmap(composed, opt) == fmap(dbl, fmap(inc, opt)))  // true
```

## Applicative: Functor with Application

An **Applicative** functor adds the ability to lift values and apply wrapped functions.

### Trait Definition

```text
trait Applicative<F> : Functor<F> {
    fun pure<A>(x: A) -> F<A>
    operator (<*>)<A, B>(ff: F<(A) -> B>, fa: F<A>) -> F<B>
}
```

### Laws

1. **Identity**: `pure(id) <*> v == v`
2. **Homomorphism**: `pure(f) <*> pure(x) == pure(f(x))`
3. **Interchange**: `u <*> pure(y) == pure(fun(f) -> f(y)) <*> u`
4. **Composition**: `pure(,,) <*> u <*> v <*> w == u <*> (v <*> w)`

### Built-in Instances

**List**: Cartesian product application
```rust
fns = [fun(x) -> x + 1, fun(x) -> x * 2]
vals = [10, 20]
fns <*> vals  // [11, 21, 20, 40] - applies each fn to each val
```

**Option**: Apply if both are Some
```rust
Some(fun(x) -> x + 1) <*> Some(10)  // Some(11)
Some(fun(x) -> x + 1) <*> Zero      // Zero
Zero <*> Some(10)                    // Zero
```

**Result**: Apply if both are Ok
```rust
Ok(fun(x) -> x * 2) <*> Ok(5)       // Ok(10)
Ok(fun(x) -> x * 2) <*> Fail("e")   // Fail("e")
Fail("e1") <*> Fail("e2")           // Fail("e1") - first error
```

### Using pure

The `pure` function lifts a value into an applicative context. To use it, you need to specify the target type:

```rust
opt: Option<Int> = pure(42)   // Some(42)
lst: List<Int> = pure(42)     // [42]
res: Result<String, Int> = pure(42)  // Ok(42)
```

## Monad: Sequencing Computations

A **Monad** extends Applicative with the ability to chain computations that depend on previous results.

### Trait Definition

```text
trait Monad<M> : Applicative<M> {
    operator (>>=)<A, B>(ma: M<A>, f: (A) -> M<B>) -> M<B>
}
```

### Laws

1. **Left identity**: `pure(a) >>= f == f(a)`
2. **Right identity**: `m >>= pure == m`
3. **Associativity**: `(m >>= f) >>= g == m >>= (fun(x) -> f(x) >>= g)`

### Built-in Instances

**List**: FlatMap (concatMap)
```rust
[1, 2, 3] >>= fun(x) -> [x, x * 10]
// [1, 10, 2, 20, 3, 30]
```

**Option**: Chain computations that may fail
```rust
Some(10) >>= fun(x) -> Some(x + 1)   // Some(11)
Some(10) >>= fun(_) -> Zero          // Zero
Zero >>= fun(x) -> Some(x + 1)       // Zero
```

**Result**: Chain computations with errors
```rust
Ok(10) >>= fun(x) -> Ok(x * 2)       // Ok(20)
Ok(10) >>= fun(_) -> Fail("error")   // Fail("error")
Fail("e") >>= fun(x) -> Ok(x * 2)    // Fail("e")
```

### Chaining Multiple Operations

```rust
// Safe division chain
fun safeDiv(x: Int, y: Int) -> Option<Int> {
    if y == 0 { Zero } else { Some(x / y) }
}

// 100 / 2 / 5 / 2 (chained on single line)
result = Some(100) >>= fun(x) -> safeDiv(x, 2) >>= fun(x) -> safeDiv(x, 5) >>= fun(x) -> safeDiv(x, 2)
print(result)  // Some(5)

// Division by zero — returns Zero immediately
result2 = Some(100) >>= fun(x) -> safeDiv(x, 0) >>= fun(x) -> safeDiv(x, 5)
print(result2)  // Zero
```

## Creating Custom Instances

You can implement FP traits for your own types:

```rust
type Box<T> = MkBox T

// Functor instance
instance Functor Box {
    fun fmap<A, B>(f: (A) -> B, fa: Box<A>) -> Box<B> {
        match fa {
            MkBox(x) -> MkBox(f(x))
        }
    }
}

// Now you can use fmap with Box
fmap(fun(x) -> x * 2, MkBox(21))  // MkBox(42)
```

## Practical Example: Validation

Using Applicative for parallel validation:

```rust
import "lib/list" (length)

type ValidationError = VErr String

// Validation functions returning Option
fun validateName(name: String) -> Option<String> {
    if length(name) > 0 { Some(name) } else { Zero }
}

fun validateAge(age: Int) -> Option<Int> {
    if age >= 0 && age < 150 { Some(age) } else { Zero }
}

// Combine validations
type Person = MkPerson((String, Int))

// Using Applicative to combine (curried function)
fun makePerson(name: String) -> (Int) -> Person {
    fun(age: Int) -> MkPerson((name, age))
}

// If both validations pass, we get Some(Person)
validatedPerson = fmap(makePerson, validateName("Alice")) <*> validateAge(30)
print(validatedPerson)  // Some(MkPerson("Alice", 30))

// If any fails, we get Zero
invalidPerson = fmap(makePerson, validateName("")) <*> validateAge(30)
print(invalidPerson)  // Zero
```

## Summary of Operators

| Operator | Trait       | Type Signature                    | Description          |
|----------|-------------|-----------------------------------|----------------------|
| `<>`     | Semigroup   | `A -> A -> A`                    | Combine two values   |
| `<*>`    | Applicative | `F<(A -> B)> -> F<A> -> F<B>`    | Apply wrapped fn     |
| `>>=`    | Monad       | `M<A> -> (A -> M<B>) -> M<B>`    | Bind/flatMap         |

## Summary of Functions

| Function | Trait       | Type Signature           | Description            |
|----------|-------------|--------------------------|------------------------|
| `mempty` | Monoid      | `() -> A`               | Identity element       |
| `fmap`   | Functor     | `(A -> B) -> F<A> -> F<B>` | Map over container  |
| `pure`   | Applicative | `A -> F<A>`             | Lift value into F      |

## Note on Type Inference

Due to Higher-Kinded Types (HKT), some operations require explicit type annotations:

```rust
// Type annotation needed for pure
opt: Option<Int> = pure(42)

// fmap and >>= can often infer types from context
fmap(fun(x) -> x + 1, Some(10))  // Works without annotation
Some(10) >>= fun(x) -> Some(x + 1)  // Works without annotation
```

## See Also

- [Traits](08_traits.md) - How traits work in general
- [Generics](05_generics.md) - Generic types and type parameters
- [Error Handling](15_error_handling.md) - Using Option and Result
