# 5. Functions

[← Back to Index](./00_index.md)

## Declarations

Functions are declared with `fun`. Return types are inferred if omitted.

```rust
fun add(a: Int, b: Int) -> Int {
    a + b
}

// Inferred return type
fun square(x: Int) {
    x * x
}

// No parameters
fun sayHello() {
    print("Hello!")
}
```

## Default Parameters

```text
fun greet(name = "World", times = 1) {
    for _ in 1..times {
        print("Hello, " ++ name ++ "!")
    }
}

greet()              // Hello, World!
greet("Alice")       // Hello, Alice!
greet("Bob", 3)      // Hello, Bob! (3 times)
```

## Anonymous Functions (Lambdas)

```rust
// Full syntax
double = fun(x: Int) -> Int { x * 2 }

// Short syntax
double = fun(x) -> x * 2

// Haskell-style sugar
double = \x -> x * 2
add = \x, y -> x + y
lazy = \ -> print("evaluated!")
```

## Higher-Order Functions

Functions that accept or return other functions.

```rust
fun apply(f: (Int) -> Int, x: Int) -> Int {
    f(x)
}

result = apply(fun(x) -> x * 2, 21)  // 42

// Returning a function
fun multiplier(factor: Int) -> (Int) -> Int {
    fun(x) -> x * factor
}

double = multiplier(2)
triple = multiplier(3)
print(double(5))  // 10
print(triple(5))  // 15
```

## Partial Application

Invoke a function with fewer arguments to get a new function.

```rust
fun add(a: Int, b: Int) -> Int { a + b }

add5 = add(5)       // (Int) -> Int
print(add5(3))      // 8

// Chaining
fun add3(a: Int, b: Int, c: Int) -> Int { a + b + c }
print(add3(1)(2)(3))  // 6
```

## Variadic Functions

Accept variable number of arguments.

```rust
fun sum(args: ...Int) -> Int {
    acc = 0
    for x in args {
        acc = acc + x
    }
    acc
}

sum(1, 2, 3)      // 6
sum(1, 2, 3, 4, 5)  // 15

// Spread arguments
nums = [2, 3, 4]
sum(1, ...nums)   // 10
```

## Closures

Inner functions capture variables from outer scopes.

```rust
fun makeCounter() {
    count = 0
    fun() {
        count = count + 1  // captures and mutates count
        count
    }
}

counter = makeCounter()
print(counter())  // 1
print(counter())  // 2
print(counter())  // 3
```

## Pipelines (`|>`)

Pass the result of one function as the first argument to the next.

```rust
import "lib/list" (*)

fun isEven(n) { n % 2 == 0 }
fun double(n) { n * 2 }
fun sum(xs) { foldl((+), 0, xs) }
numbers = [1, 2, 3, 4]

// Nested calls (hard to read)
result = sum(filter(isEven, map(double, numbers)))

// Pipeline (left-to-right flow)
result = numbers
    |> map(double)
    |> filter(isEven)
    |> sum

// Argument Placeholders
fun subtract(a, b) { a - b }
fun divide(a, b) { a / b }

// Use _ to specify where the piped value goes if not the first argument
10 |> subtract(20, _)   // subtract(20, 10)
10 |> divide(_, 2)      // divide(10, 2)
```

## Function Composition (`,,`)

Combine two functions into one.

```rust
fun double(x: Int) -> Int { x * 2 }
fun increment(x: Int) -> Int { x + 1 }

// Compose: double, then increment
// Note: ,, is right-to-left composition (like Haskell's .)
// f ,, g means f(g(x))
// increment(double(5)) = 10 + 1 = 11
incrementThenDouble = increment ,, double
print(incrementThenDouble(5))  // 11

// If you want double(increment(5)) = (5+1)*2 = 12
doubleThenIncrement = double ,, increment
print(doubleThenIncrement(5))  // 12
```

## Application Operator (`$`)

Low-precedence application operator to avoid parentheses.

```rust
fun add(a, b) { a + b }
fun multiply(a, b) { a * b }

// Without $
print(show(add(1, multiply(2, 3))))

// With $
print $ show $ add(1, multiply(2, 3))
```

## Operators as Functions

Wrap operators in parentheses to use them as functions.

```rust
import "lib/list" (foldl)

add = (+)
print(add(1, 2))  // 3

sum = foldl((+), 0, [1, 2, 3, 4, 5])  // 15
```

## Extension Methods

Add methods to types without modifying them.

```rust
import "lib/math" (sqrt)

type alias Point = { x: Int, y: Int }

fun (p: Point) magnitude() -> Float {
    sqrt(intToFloat(p.x * p.x + p.y * p.y))
}

p = { x: 3, y: 4 }
print(p.magnitude())  // 5.0
```

## Tail Call Optimization

Recursive functions in tail position are optimized to loops (no stack overflow).

```rust
// Optimized
fun factorial(n: Int, acc: Int) -> Int {
    if n <= 1 {
        acc
    } else {
        factorial(n - 1, acc * n)  // tail position
    }
}

// NOT Optimized
fun factorialBad(n: Int) -> Int {
    if n <= 1 {
        1
    } else {
        n * factorialBad(n - 1)  // result used in multiplication
    }
}
```

## Ignored Parameters

Use `_` to ignore unused parameters.

```rust
import "lib/list" (foldl)

items = [1, 2, 3]
count = foldl(fun(acc, _) -> acc + 1, 0, items)
third = fun(_, _, x) -> x
```
