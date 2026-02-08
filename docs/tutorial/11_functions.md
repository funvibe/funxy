# Iteration 4: Functions

This iteration introduces user-defined functions, enabling code reuse and better structure.

## Function Declaration

Functions are declared using the `fun` keyword.

```rust
fun add(a: Int, b: Int) -> Int {
    a + b
}
```

*   **Parameters**: Must have explicit types (`a: Int`).
*   **Return Type**: Specified after `->`. **Optional**. If omitted, it is inferred from the last expression in the body.
*   **Body**: A block of statements. The last expression in the block is implicitly returned unless you use `return`.

### Implicit Return & Type Inference

You can omit the return type if the function body is an expression block.

```rust
fun square(x: Int) {
    x * x  // Return type inferred as Int
}
```

If the function performs side effects and doesn't return a meaningful value, it returns `nil` (the only value of type `Nil`).

```rust
fun log(msg: String) {
    print(msg) // print returns its first argument
}

// log returns what print returns (the message)
result = log("hello")
print(result) // "hello"
```

### Explicit Return

You can use an explicit return to exit early:

```rust
fun classify(n: Int) -> Int {
    if n < 0 {
        return -1
    }
    if n == 0 {
        return 0
    }
    1 // implicit return still works
}
```

`return` without a value returns `nil`.

## Default Parameters

Parameters can have default values:

```rust
fun greet(name = "World") {
    print("Hello, ${name}!")
}

greet()         // Hello, World!
greet("Alice")  // Hello, Alice!

// Multiple defaults
fun wrap(value, prefix = "[", suffix = "]") {
    prefix ++ value ++ suffix
}

print(wrap("test"))           // [test]
print(wrap("test", "<"))      // <test]
print(wrap("test", "<", ">")) // <test>

// With type annotations
fun multiply(x: Int, y: Int = 2) -> Int {
    x * y
}
```

**Rules:**
- Parameters with defaults must come after required parameters
- Default expressions are evaluated at call time
- Defaults can reference other variables in scope

## Function Types and Rules

### 1. Global Functions
Declared at the top level of a file.
- **Parameter Types**: **Required**.
- **Return Type**: Optional (inferred).

### 2. Local Functions
Declared inside another function. They are visible only within the enclosing scope and can capture variables (closure).
- **Parameter Types**: **Required**.
- **Return Type**: Optional (inferred).

```rust
fun outer() {
    factor = 2

    fun inner(x: Int) {
        x * factor // Captures 'factor'
    }

    print(inner(5)) // 10
}
```

#### Mutation in Closures
Local functions can **mutate** variables captured from their enclosing function scope.

```rust
fun counter() {
    count = 0
    fun inc() {
        count = count + 1  // OK: mutates 'count' from 'counter' scope
        count
    }
    inc
}
```

**Note:** You **cannot** mutate variables defined at the **module level (global)** from inside a function. Global variables are read-only to functions.

### 3. Anonymous Functions (Lambdas)
Used as expressions, typically passed to other functions.
- **Parameter Types**: **Optional** (if context allows inference).
- **Return Type**: Optional (inferred).

Syntax: `fun(args) { body }` or `fun(args) -> expr`.
Short syntax: `\args -> expr` (Haskell-style).

```rust
import "lib/list" (map)

list = [1, 2, 3]

// Explicit types
doubled = map(fun(x: Int) { x * 2 }, list)

// Inferred types (recommended)
tripled = map(fun(x) -> x * 3, list)

// Short syntax (Lambda sugar)
quadrupled = map(\x -> x * 4, list)
multiArg = (\x, y -> x + y)(1, 2)

// No arguments
thunk = fun() { print("Lazy!") }
thunk2 = \ -> print("Lazy 2!")
thunk()
```

## Block Syntax (Trailing Block)

When calling a function, you can pass a block as the last argument without wrapping it in parentheses. This creates a clean DSL-like syntax, especially useful for UI builders, configuration, and nested structures.

**Important**: This syntax **only works with lowercase identifiers** (functions). Uppercase identifiers (constructors) use the standard syntax to avoid conflicts with constructor record syntax like `MkUser { field: value }`.

### Basic Syntax

```rust
// Traditional syntax
myFunc(arg1, arg2, { expr1, expr2, expr3 })

// Block syntax - omit the wrapping when block is last argument
myFunc(arg1, arg2) {
    expr1
    expr2
    expr3
}

// When function takes only a block, you can omit parentheses entirely
myFunc {
    expr1
    expr2
    expr3
}
```

### Case Sensitivity Rule

```rust
// ✓ Lowercase - block syntax works
div { text("Hello") }

// ✗ Uppercase - standard constructor syntax required
MkUser { userId: 1, userName: "Alice" }  // This is constructor record syntax
MyConstructor("value")  // Use parentheses for constructors
```

### UI DSL Example

```rust
import "kit/ui" (div, span, text, render)

// Clean nested structure without excessive parentheses
page = div {
    span { text("Header") }
    div {
        text("Content line 1")
        text("Content line 2")
    }
    span { text("Footer") }
}

html = render(page)
```

### With Attributes

```rust
// Combine named arguments with block syntax
div(class: "container", id: "main") {
    text("Hello")
    span { text("World") }
}
```

### Custom DSL Functions

The block syntax works with any variadic function that accepts a list as its last parameter:

```rust
// Define a function that accepts children
fun container(...items) {
    print("Container with ${len(items)} items")
    items
}

// Use it with block syntax
result = container {
    1
    2
    3
}
// Prints: Container with 1 items
// Result is [1, 2, 3]
```

**Note**: The block expressions are collected into a single list and passed as one argument.

## Ignored Parameters (`_`)

Use `_` as a parameter name when you need to accept an argument but don't use its value. This is common in callbacks, pattern matching, and functional programming.

```rust
import "lib/list" (foldl, map)

// Count elements (ignore values, just count)
count = foldl(fun(acc, _) -> acc + 1, 0, [1, 2, 3, 4, 5])
print(count)  // 5

// Fill with constant value
zeroed = map(fun(_) -> 0, [1, 2, 3])
print(zeroed)  // [0, 0, 0]

// Multiple ignored parameters
third = fun(_, _, x) -> x
print(third(1, 2, 3))  // 3

// Ignore first, use second
f = fun(_, y) -> y * 2
print(f(100, 5))  // 10

// Named function with ignored parameter
fun process(_, value: Int) -> Int {
    value + 10
}
print(process("ignored", 32))  // 42
```

### In Extension Methods

```rust
fun (x: Int) addIgnoring(_, y: Int) -> Int {
    x + y
}

print(10.addIgnoring("ignored", 5))  // 15
```

### Key Points

- Multiple `_` parameters are allowed in the same function
- `_` parameters are still type-checked but not bound to any name
- Works in lambdas, named functions, and extension methods
- Also works in pattern matching: `match pair { (x, _) -> x }`

## Recursion & Tail Call Optimization (TCO)

Recursive functions are supported. The interpreter implements **Tail Call Optimization (TCO)** using a **trampolining** mechanism. When a recursive call is in **tail position** (i.e., the result is returned directly without further operations), the interpreter avoids consuming stack frames. This allows for deep recursion without stack overflow.

### What is a Tail Call?

A call is in **tail position** if it is the **last action** of a function—its result is returned directly.

```rust
// TAIL CALL - result returned directly
fun factorial(n: Int, acc: Int) -> Int {
    if n <= 1 {
        acc
    } else {
        factorial(n - 1, acc * n)  // ✓ Tail position
    }
}

// NOT a tail call - result used in operation
fun factorial_bad(n: Int) -> Int {
    if n <= 1 {
        1
    } else {
        n * factorial_bad(n - 1)  // ✗ Result multiplied after call
    }
}
```

### Supported TCO Patterns

The analyzer marks calls as tail calls in these contexts:

1. **Last expression in a block**:
```rust
fun g(x) { x }  // helper

fun f(n) {
    temp = n + 1
    g(temp)  // ✓ Tail position
}
print(f(5))
```

2. **Both branches of `if`**:
```rust
fun g(x) { x }  // helper

fun f(n) {
    if n > 0 { f(n - 1) }  // ✓ Tail
    else { g(n) }          // ✓ Tail
}
print(f(5))
```

3. **All arms of `match`**:
```rust
fun g() { 0 }  // helper

fun f(n) {
    match n {
        0 -> g()    // ✓ Tail
        _ -> f(n-1) // ✓ Tail
    }
}
print(f(5))
```

4. **Mutual recursion**:
```rust
fun is_even(n) {
    if n == 0 { true } else { is_odd(n - 1) }  // ✓ Tail
}

fun is_odd(n) {
    if n == 0 { false } else { is_even(n - 1) }  // ✓ Tail
}

print(is_even(100000))  // Works! No stack overflow
```

### NOT Tail Calls (No Optimization)

These patterns are **not** in tail position:

```rust
// These are NOT tail calls (shown as comments):

// Result used in operation:
// n * f(n - 1)

// Assigned to variable before return:
// f(n - 1)

// Inside a data structure:
// [x, f(n - 1)...]

// Not the last expression:
// f(n - 1)
// other_call()  // ← This would be the last expression

print("Not tail call examples")
```

### Converting to Tail-Recursive Form

Use an **accumulator** parameter to convert regular recursion to tail recursion:

```rust
// Non-tail recursive (will overflow for large n)
fun sum(n) {
    if n == 0 { 0 } else { n + sum(n - 1) }
}

// Tail-recursive with accumulator (safe for any n)
fun sum_tail(n, acc) {
    if n == 0 { acc } else { sum_tail(n - 1, acc + n) }
}

print(sum_tail(100000, 0))  // Works!
```

### Implementation Details

The interpreter uses a **trampoline loop**:
1. When a tail call is detected, it returns a `TailCall` object instead of evaluating immediately
2. The trampoline loop catches this and continues execution with the new arguments
3. This converts recursion into iteration at runtime

This approach supports:
- Self-recursion (function calling itself)
- Mutual recursion (functions calling each other)
- Calls to different functions in tail position

## Partial Application

Functions can be called with fewer arguments than they require. This returns a new function that waits for the remaining arguments.

```rust
fun add(a: Int, b: Int) -> Int { a + b }

// Partial application - provide only first argument
add5 = add(5)       // Returns a function (Int) -> Int
print(add5(3))      // 8
print(add5(10))     // 15

// Chain partial applications
fun add3(a: Int, b: Int, c: Int) -> Int { a + b + c }
add10 = add3(10)           // (Int, Int) -> Int
add10and20 = add10(20)     // (Int) -> Int
print(add10and20(5))       // 35

// Direct chaining
print(add3(1)(2)(3))       // 6
```

### With lib/list Functions

Partial application is especially powerful with higher-order functions:

```rust
import "lib/list" (map, filter, foldl)

// Create reusable transformations
double = fun(x) -> x * 2
doubled = map(double)      // (List<Int>) -> List<Int>
print(doubled([1, 2, 3]))  // [2, 4, 6]

// Create reusable filters
isEven = fun(x) -> x % 2 == 0
evens = filter(isEven)
print(evens([1, 2, 3, 4, 5, 6]))  // [2, 4, 6]

// Create reusable reducers
sum = foldl((+), 0)
print(sum([1, 2, 3, 4, 5]))  // 15
```

### With Pipe Operator

Partial application works seamlessly with the pipe operator:

```rust
import "lib/list" (filter, map)

result = [1, 2, 3, 4, 5]
    |> filter(fun(x) -> x > 2)
    |> map(fun(x) -> x * 10)
print(result)  // [30, 40, 50]
```

### Constructor Partial Application

Type constructors also support partial application:

```rust
type Point = MkPoint Int Int
pointX5 = MkPoint(5)       // Waiting for second Int
print(pointX5(10))         // MkPoint(5, 10)

type Box<t> = MkBox t t t
boxWith1 = MkBox(1)
print(boxWith1(2)(3))      // MkBox(1, 2, 3)

// Built-in constructors too
wrapSome = Some
print(wrapSome(42))        // Some(42)
```

### Returning Partial Applications

Functions can return partial applications:

```rust
fun add(a: Int, b: Int) -> Int { a + b }

fun makeAdder(n: Int) -> (Int) -> Int {
    add(n)  // Returns partial application of add
}

add5 = makeAdder(5)
print(add5(3))  // 8
```

## Next Steps

Future iterations will cover:
*   Generic functions (`fun id<T>(x: T)`).
*   Extension methods (`fun (s: String) shout()`).
