# 04. Pattern Matching

## Task
Elegantly handle different cases without if-else chains.

## Basic Solution

```rust
fun describe(x: Int) -> String {
    match x {
        0 -> "zero"
        1 -> "one"
        n if n < 0 -> "negative"
        n if n > 100 -> "big"
        _ -> "some number"
    }
}

print(describe(0))    // zero
print(describe(-5))   // negative
print(describe(150))  // big
print(describe(42))   // some number
```

## Explanation

- `match` checks value in order
- `_` — wildcard, catches everything else
- `n if condition` — guard pattern with condition
- Returns value (it's an expression!)

## Tuple Destructuring

```rust
fun process(pair: (Int, Int)) -> String {
    match pair {
        (0, 0) -> "origin"
        (0, y) -> "on Y axis at " ++ show(y)
        (x, 0) -> "on X axis at " ++ show(x)
        (x, y) -> "point (" ++ show(x) ++ ", " ++ show(y) ++ ")"
    }
}

print(process((0, 0)))   // origin
print(process((0, 5)))   // on Y axis at 5
print(process((3, 4)))   // point (3, 4)
```

## List Destructuring

```rust
import "lib/list" (length)

fun listInfo(xs) {
    match xs {
        [] -> "empty"
        [x] -> "single element"
        [x, y] -> "two elements"
        [head, tail...] -> "starts with something, " ++ show(length(tail)) ++ " more"
    }
}

print(listInfo([]))           // empty
print(listInfo([1]))          // single element
print(listInfo([1, 2]))       // two elements
print(listInfo([1, 2, 3, 4])) // starts with something, 3 more
```

Spread syntax: `tail...` (after variable).

## ADT (Algebraic Data Types)

```rust
// Constructor with one argument: Circle(Float)
// Constructor with multiple: Rectangle((Float, Float)) — tuple
type Shape = Circle(Float) | Rectangle((Float, Float))

fun area(s: Shape) -> Float {
    match s {
        Circle(r) -> 3.14159 * r * r
        Rectangle((w, h)) -> w * h
    }
}

print(area(Circle(5.0)))             // 78.53975
print(area(Rectangle((4.0, 3.0))))   // 12
```

## Option (Built-in)

`Option<T>` — built-in type: `Some T | Zero`.

```rust
fun safeDivide(a: Int, b: Int) -> Option<Int> {
    if b == 0 { Zero } else { Some(a / b) }
}

fun showResult(opt: Option<Int>) -> String {
    match opt {
        Some(x) -> "Result: " ++ show(x)
        Zero -> "Cannot divide by zero"
    }
}

print(showResult(safeDivide(10, 2)))  // Result: 5
print(showResult(safeDivide(10, 0)))  // Cannot divide by zero
```

## FizzBuzz

```rust
import "lib/list" (range)

fun fizzbuzz(n: Int) -> String {
    match (n % 3, n % 5) {
        (0, 0) -> "FizzBuzz"
        (0, _) -> "Fizz"
        (_, 0) -> "Buzz"
        _ -> show(n)
    }
}

for i in range(1, 16) {
    print(fizzbuzz(i))
}
```

## Nested Matching

`Result<E, A>` — built-in type: `Ok(A) | Fail(E)`.

```rust
// Result<E, A> = Ok(A) | Fail(E) — built-in

fun processResult(r: Result<String, Option<Int>>) -> String {
    match r {
        Ok(Some(x)) -> "Got value: " ++ show(x)
        Ok(Zero) -> "Ok but empty"
        Fail(e) -> "Error: " ++ e
    }
}

print(processResult(Ok(Some(42))))   // Got value: 42
print(processResult(Ok(Zero)))        // Ok but empty
print(processResult(Fail("oops")))    // Error: oops
```

## Guard Patterns (Conditions)

```rust
fun grade(score: Int) -> String {
    match score {
        s if s >= 90 -> "A"
        s if s >= 80 -> "B"
        s if s >= 70 -> "C"
        s if s >= 60 -> "D"
        _ -> "F"
    }
}

print(grade(95))  // A
print(grade(72))  // C
print(grade(55))  // F
```

## Condition Combinations

```rust
fun classify(x: Int) -> String {
    match x {
        n if n > 0 && n % 2 == 0 -> "positive even"
        n if n > 0 -> "positive odd"
        n if n < 0 -> "negative"
        _ -> "zero"
    }
}

print(classify(4))   // positive even
print(classify(3))   // positive odd
print(classify(-5))  // negative
print(classify(0))   // zero
```

## Syntax Summary

| Pattern | Example | Description |
|---------|--------|----------|
| Literal | `0 ->` | Exact match |
| Variable | `x ->` | Bind to variable |
| Wildcard | `_ ->` | Any value |
| Tuple | `(x, y) ->` | Destructuring |
| List | `[x, y] ->` | Fixed length |
| List spread | `[h, t...] ->` | Head and tail |
| ADT | `Circle(r) ->` | ADT destructuring |
| Guard | `x if x > 0 ->` | With condition |
