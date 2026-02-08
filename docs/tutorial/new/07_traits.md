# 7. Traits

[← Back to Index](./00_index.md)

Traits define shared behavior for different types. They are similar to interfaces in other languages or type classes in Haskell.

## Declaring a Trait

```rust
trait Printable<t> {
    fun render(val: t) -> String
}
```

## Implementing a Trait

```rust
trait Printable<t> {
    fun render(val: t) -> String
}

instance Printable Int {
    fun render(val: Int) -> String {
        "Int: " ++ show(val)
    }
}

instance Printable String {
    fun render(val: String) -> String {
        "String: " ++ val
    }
}
```

## Default Methods

Traits can provide default implementations.

```rust
trait Comparable<t> {
    fun eq(a: t, b: t) -> Bool

    // Default implementation
    fun neq(a: t, b: t) -> Bool {
        if eq(a, b) { false } else { true }
    }
}

instance Comparable Int {
    fun eq(a: Int, b: Int) -> Bool {
        a == b
    }
    // neq is available automatically
}
```

## Trait Inheritance

One trait can require another.

```rust
trait Eq<t> {
    fun eq(a: t, b: t) -> Bool
}

trait Ord<t> : Eq<t> {
    fun compare(a: t, b: t) -> Int  // -1, 0, 1
}

// Must implement both
type alias Wrapper = { val: Int }
instance Eq Wrapper { fun eq(a, b) { a.val == b.val } }
instance Ord Wrapper { fun compare(a, b) { if a.val < b.val { -1 } else if a.val > b.val { 1 } else { 0 } } }
```

## Constraints

Restrict generic parameters to types that implement specific traits.

```rust
trait Printable<t> { fun render(x: t) -> String }
trait Comparable<t> { fun eq(a: t, b: t) -> Bool }

fun printAll<t: Printable>(items: List<t>) {
    for item in items {
        print(render(item))
    }
}

// Multiple constraints
fun process<t: Printable, t: Comparable>(x: t, y: t) -> String {
    if eq(x, y) {
        render(x)
    } else {
        render(x) ++ " != " ++ render(y)
    }
}
```

## Operator Overloading

Operators are defined via traits (e.g., `Numeric`).

```rust
type alias Vec2 = { x: Float, y: Float }

instance Numeric Vec2 {
    operator (+)(a: Vec2, b: Vec2) -> Vec2 {
        { x: a.x + b.x, y: a.y + b.y }
    }

    operator (*)(a: Vec2, b: Vec2) -> Vec2 {
        { x: a.x * b.x, y: a.y * b.y }
    }

    // ... must implement other numeric operators
}

v1 = { x: 1.0, y: 2.0 }
v2 = { x: 3.0, y: 4.0 }
v3: Vec2 = v1 + v2  // { x: 4.0, y: 6.0 }
```

## Higher-Kinded Types (HKT)

Traits can abstract over type constructors.

```rust
trait MyFunctor<f: * -> *> {
    fun fmap<a, b>(func: (a) -> b, val: f<a>) -> f<b>
}

type Maybe<t> = Just(t) | Nothing

instance MyFunctor Maybe {
    fun fmap<a, b>(f: (a) -> b, fa: Maybe<a>) -> Maybe<b> {
        match fa {
            Just(x) -> Just(f(x))
            Nothing -> Nothing
        }
    }
}

instance MyFunctor List {
    fun fmap<a, b>(f: (a) -> b, fa: List<a>) -> List<b> {
        [f(x) | x <- fa]
    }
}

// Usage
fmap(fun(x) -> x * 2, Just(21))   // Just(42)
fmap(fun(x) -> x * 2, [1, 2, 3])  // [2, 4, 6]
```

## Built-in Traits

| Trait | Methods/Operators | Description |
|-------|-------------------|-------------|
| `Equal<t>` | `==`, `!=` | Equality comparison |
| `Order<t>` | `<`, `>`, `<=`, `>=` | Ordering |
| `Numeric<t>` | `+`, `-`, `*`, `/`, `%`, `**` | Arithmetic |
| `Concat<t>` | `++` | Concatenation |
| `Show<t>` | `show` | String representation |
| `Default<t>` | `default` | Default value |
| `Functor<f>` | `fmap` | Mapping over structure |
| `Semigroup<t>` | `<>` | Associative operation |
| `Monoid<t>` | `mempty` | Semigroup with identity |

## Do Notation

Syntactic sugar for monadic operations (requires `Monad` trait).

```text
// result = Just(30)
result = do {
    x <- Just(10)
    y <- Just(20)
    Just(x + y)
}

// With let binding
result = do {
    x <- Just(5)
    k :- 2  // let
    Just(x * k)
}
// result = Just(10)

// Short-circuiting
result = do {
    x <- Nothing   // stops here
    y <- Just(20)  // not executed
    Just(x + y)
}
// result = Nothing
```
