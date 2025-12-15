# 02. Pipes and Composition

## Task
Create readable chains of data transformations.

## Pipe operator `|>`

```rust
import "lib/math" (absInt)

// Without pipe (nested calls, read from inside out)
result = show(absInt(-42))

// With pipe (linear, read left to right)
result = -42 |> absInt |> show
print(result)  // "42"
```

## Explanation

`x |> f` is equivalent to `f(x)`

Pipe passes the left operand as the first argument to the right function.

## List transformations

```rust
import "lib/list" (filter, map)

numbers = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]

// Imperatively (verbose)
result1 = []
for n in numbers {
    if n % 2 == 0 {
        result1 = result1 ++ [n * n]
    }
}

// Functionally with pipe (elegant!)
result2 = numbers
    |> filter(fun(n) -> n % 2 == 0)  // [2, 4, 6, 8, 10]
    |> map(fun(n) -> n * n)           // [4, 16, 36, 64, 100]

print(result2)  // [4, 16, 36, 64, 100]
```

## Aggregation

```rust
import "lib/list" (foldl)

sales = [100, 250, 180, 320, 90]

total = foldl(fun(acc, x) -> acc + x, 0, sales)
print(total)  // 940

average = total / len(sales)
print(average)  // 188
```

## Real example: data processing

```rust
import "lib/string" (stringSplit)
import "lib/list" (map, filter)

// Simple parser (without error handling for clarity)
fun parseAge(s: String) -> Int {
    match read(s, Int) {
        Some(n) -> n
        Zero -> 0
    }
}

csv = "Alice,30,Engineer\nBob,25,Designer\nCarol,35,Manager"

employees = stringSplit(csv, "\n")
    |> map(fun(line) -> {
        parts = stringSplit(line, ",")
        {
            name: parts[0],
            age: parseAge(parts[1]),
            role: parts[2]
        }
    })
    |> filter(fun(e) -> e.age >= 30)
    |> map(fun(e) -> e.name)

print(employees)  // ["Alice", "Carol"]
```

## Function composition

```rust
// Create new functions from existing ones
fun double(x: Int) -> Int { x * 2 }
fun increment(x: Int) -> Int { x + 1 }
fun square(x: Int) -> Int { x * x }

// Apply chain
result = 3 |> double |> increment |> square
print(result)  // ((3 * 2) + 1)² = 49
```

## Partial application with lambdas

```rust
import "lib/list" (map, filter, all)

numbers = [1, 2, 3, 4, 5]

// Multiply all by 10
tens = numbers |> map(fun(x) -> x * 10)
print(tens)  // [10, 20, 30, 40, 50]

// Filter greater than 3
big = numbers |> filter(fun(x) -> x > 3)
print(big)  // [4, 5]

// Check if all are positive
allPositive = all(fun(x) -> x > 0, numbers)
print(allPositive)  // true
```

## Getting fields from a list of objects

```rust
import "lib/list" (map)

users = [
    { name: "Alice", age: 30 },
    { name: "Bob", age: 25 }
]

// Extract fields
names = users |> map(fun(u) -> u.name)
print(names)  // ["Alice", "Bob"]

ages = users |> map(fun(u) -> u.age)
print(ages)  // [30, 25]
```

## Combining operations

```rust
import "lib/string" (stringToUpper)
import "lib/list" (filter, map, foldl)

words = ["hello", "world", "funxy", "rocks"]

result = words
    |> filter(fun(w) -> len(w) > 4)
    |> map(fun(w) -> stringToUpper(w))
    |> foldl(fun(acc, w) -> if acc == "" { w } else { acc ++ ", " ++ w }, "")

print(result)  // "HELLO, WORLD, FUNXY, ROCKS"
```

## Working with Option

```rust
// Option is built-in: Some T | Zero

fun mapOption(opt, f) {
    match opt {
        Some(x) -> Some(f(x))
        Zero -> Zero
    }
}

fun flatMapOption(opt, f) {
    match opt {
        Some(x) -> f(x)
        Zero -> Zero
    }
}

// Chain of safe operations
result = Some(10)
    |> fun(o) -> mapOption(o, fun(x) -> x * 2)
    |> fun(o) -> flatMapOption(o, fun(x) -> if x > 15 { Some(x) } else { Zero })
    |> fun(o) -> mapOption(o, fun(x) -> "Result: " ++ show(x))

print(result)  // Some("Result: 20")
```

## Style comparison

```rust
import "lib/list" (range, filter, map, foldl)

// Task: find sum of squares of even numbers from 1 to 10

// Imperatively
sum1 = 0
for i in range(1, 11) {
    if i % 2 == 0 {
        sum1 += i * i
    }
}
print(sum1)  // 220

// Functionally
sum2 = range(1, 11)
    |> filter(fun(x) -> x % 2 == 0)
    |> map(fun(x) -> x * x)
    |> foldl(fun(a, b) -> a + b, 0)
print(sum2)  // 220
