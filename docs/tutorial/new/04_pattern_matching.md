# 4. Pattern Matching

[← Back to Index](./00_index.md)

Pattern matching is a powerful mechanism for checking values against patterns and decomposing data.

## Basic Match

```rust
fun describe(x: Int) -> String {
    match x {
        0 -> "zero"
        1 -> "one"
        2 -> "two"
        _ -> "many"
    }
}
```

## Literal Patterns

Match exact values.

```rust
match value {
    42 -> "the answer"
    true -> "yes"
    "hello" -> "greeting"
    'x' -> "letter x"
    nil -> "nothing"
    _ -> "something else"
}
```

## Variable Patterns

Bind the matched value to a variable.

```rust
match x {
    0 -> "zero"
    n -> "got " ++ show(n)  // n binds to the value
}
```

## Guard Patterns

Add arbitrary conditions to patterns.

```rust
fun classify(n: Int) -> String {
    match n {
        x if x > 0 -> "positive"
        x if x < 0 -> "negative"
        _ -> "zero"
    }
}

// Multiple guards
pair = (1, 1)
match pair {
    (a, b) if a == b -> "equal"
    (a, b) if a > b -> "first is greater"
    (a, b) -> "second is greater"
}
```

## Tuple Patterns

Destructure tuples.

```rust
point = (0, 0)
match point {
    (0, 0) -> "origin"
    (x, 0) -> "on X axis"
    (0, y) -> "on Y axis"
    (x, y) -> "point at " ++ show(x) ++ ", " ++ show(y)
}

// Spread in tuples
match (1, 2, 3, 4) {
    (first, ...rest) -> {
        print(first)  // 1
        print(rest)   // (2, 3, 4)
    }
}
```

## List Patterns

Destructure lists.

```rust
list = [1, 2, 3]
match list {
    [] -> "empty"
    [x] -> "single: " ++ show(x)
    [x, y] -> "pair"
    [head, ...tail] -> "head: " ++ show(head)
}

// Fixed size
numbers = [1, 2, 3]
match numbers {
    [a, b, c] -> a + b + c
    _ -> 0
}
```

## Record Patterns

Destructure records.

```rust
user = { name: "admin", age: 30 }
match user {
    { name: "admin" } -> "administrator"
    { age: 0 } -> "newborn"
    { name: n, age: a } -> n ++ " is " ++ show(a)
}

// Partial match (only checks specified fields)
config = { debug: true }
fun enableDebug() {}

match config {
    { debug: true } -> enableDebug()
    _ -> ()
}
```

## Constructor Patterns (ADTs)

Match algebraic data types.

```rust
type Shape =
    | Circle(Float)
    | Rectangle(Float, Float)
    | Point

fun area(s: Shape) -> Float {
    match s {
        Circle(r) -> 3.14159 * r * r
        Rectangle(w, h) -> w * h
        Point -> 0.0
    }
}
```

## Type Patterns

Match based on runtime type (useful for Union types).

```rust
fun process(x: Int | String | Nil) -> String {
    match x {
        n: Int -> "number: " ++ show(n)
        s: String -> "text: " ++ s
        _: Nil -> "nothing"
    }
}
```

## String Patterns with Captures

Match string structure and extract parts.

```rust
path = "/users/42/posts/123"

match path {
    "/users/{id}" -> print("User: " ++ id)
    "/users/{userId}/posts/{postId}" -> {
        print("User: " ++ userId)
        print("Post: " ++ postId)
    }
    "/static/{...file}" -> print("File: " ++ file)  // greedy capture
    _ -> print("Not found")
}
```

## Pin Operator (^)

Match against an existing variable's value instead of binding a new one.

```rust
expected = 42
value = 42

match value {
    ^expected -> "matched 42"
    other -> "got " ++ show(other)
}

// In tuples
x = 1
y = 2
match (1, 2) {
    (^x, ^y) -> "exact match"
    _ -> "no match"
}
```

## Nested Patterns

Combine patterns arbitrarily.

```rust
data = (1, [2, 3])
match data {
    (1, [a, b, ...rest]) -> {
        print(a)
        print(b)
        print(rest)
    }
    _ -> ()
}

// Matching complex structures
// import "lib/option" (Some)

type alias User = { name: String }
complexData = ({ name: "Alice" }, Some(42))

match complexData {
    ({ name: n }, Some(value)) -> {
        print(n)
        print(value)
    }
    _ -> ()
}
```
