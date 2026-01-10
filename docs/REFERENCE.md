# Funxy Language Reference Manual

## Table of Contents

1. [Variables and Constants](#1-variables-and-constants)
2. [Data Types](#2-data-types)
3. [Literals](#3-literals)
4. [Operators](#4-operators)
5. [Control Flow](#5-control-flow)
6. [Functions](#6-functions)
7. [Collections](#7-collections)
8. [Pattern Matching](#8-pattern-matching)
9. [Types and Generics](#9-types-and-generics)
10. [Traits](#10-traits)
11. [Modules and Imports](#11-modules-and-imports)
12. [Error Handling](#12-error-handling)
13. [Built-in Functions](#13-built-in-functions)
14. [Standard Library](#14-standard-library)
15. [Asynchronous Programming](#15-asynchronous-programming)
16. [Command Line Interface](#16-command-line-interface)
17. [Bytecode Compilation](#17-bytecode-compilation)
18. [Advanced Functional Programming](#18-advanced-functional-programming)
19. [Compiler Directives](#19-compiler-directives)
20. [Tools and Debugging](#20-tools-and-debugging)
21. [Summary Tables](#21-summary-tables)

---

## 1. Variables and Constants

### Comments
```rust
// Single line comment
/*
   Multi-line
   comment
*/
```

### Variables (Mutable)
```rust
x = 42
x = 45  // mutation
print(x)  // 45
```

### Constants (Immutable)
```rust
y :- 1  // immutable
// y = 2  // Error: cannot reassign constant 'y'

pi :- 3.14159
max_retries: Int :- 5
```

### Tuple Unpacking with Constants
```rust
pair :- (1, "hello")
(a, b) :- pair  // a = 1, b = "hello"

// Nested unpacking
nested :- ((1, 2), 3)
((x, y), z) :- nested

// Wildcard for unused parts
(first, _) :- pair
```

### Discard Variable (_)
The underscore `_` can be used to ignore values in assignments.

```rust
_ = doSomething()      // Ignore result
(x, _) = (1, 2)        // Ignore part of tuple
```

Note: `_` is also used as a [Pipe Placeholder](#pipe-and-composition) and for [Ignored Parameters](#ignored-parameters).

### Scoping Rules
- Variables can be mutated if they exist in current or outer scopes
- Global (module-level) variables cannot be mutated from within functions
- Closure variables can be mutated from inner functions

```rust
x = 1
{
    x = 2  // Mutates outer x
}
print(x)  // 2

// Global immutability
globalCounter = 0
fun increment() {
    // globalCounter = globalCounter + 1  // Error: cannot mutate global
}
```

---

## 2. Data Types

### Primitive Types
- `Int` - 64-bit integer
- `Float` - Floating point number
- `BigInt` - Arbitrary precision integer (suffix `n`)
- `Rational` - Rational number (suffix `r`)
- `Bool` - Boolean (`true`, `false`)
- `Char` - Character
- `String` - String (alias for `List<Char>`)
- `Range<t>` - Range of values (e.g. `1..10`)
- `Bytes` - Immutable byte sequence
- `Bits` - Bit sequence (immutable)
- `Nil` - Absence of value (`nil`)

### Collection Types
- `List<t>` - Immutable list
- `Map<k, v>` - Immutable associative array
- `(t1, t2, ...)` - Tuple (fixed-size, heterogeneous)

### Record Types
```rust
type Point = { x: Int, y: Int }
p: Point = { x: 10, y: 20 }
```

### Algebraic Data Types (ADTs)
```rust
type Option<t> = Some t | Zero
type Result<e, a> = Ok a | Fail e
type Tree<t> = Leaf t | Branch((Tree<t>, Tree<t>))
```

### Union Types
```rust
x: Int | String = 42
x = "hello"  // OK

// Nullable shorthand
name: String? = "Alice"  // Equivalent to String | Nil
name = nil
```

### Type Aliases
```rust
type alias Money = Float
type alias UserId = Int

// Union Type Aliases (Required for named unions)
type alias ID = Int | String
```

---

## 3. Literals

### Numeric Literals
```rust
123        // Int (decimal)
-42        // Int (negative)
0xFF       // Int (hexadecimal)
0o777      // Int (octal)
0b101      // Int (binary)
1.5        // Float
3.14       // Float
100n       // BigInt
0xFFn      // BigInt (hex)
1.5r       // Rational
10r        // Rational
```

### String Literals
```rust
"Hello, World!"  // Regular string

// String Interpolation
name = "Alice"
age = 30
"Hello, ${name}!"                    // Hello, Alice!
"${x} + ${y} = ${x + y}"            // 5 + 3 = 8
"${person.name} is ${person.age}"   // Bob is 25
"Double: ${double(10)}"             // Double: 20

// Raw Strings (Multi-line)
text = `This is
a multi-line
string`

json = `{"name": "test", "value": 42}`

// Format String Literals
%"%.2f"(3.14159)                    // "3.14"
%"%s, %s!"("Hello", "World")        // "Hello, World!"
%"Name: %s, Age: %d"("Alice", 25)   // "Name: Alice, Age: 25"
formatter = %"%05d"
formatter(42)                       // "00042"

// String Indexing (String is List<Char>)
"hello"[1]         // 'e' (Char)
"hello"[-1]        // 'o'
// "hello"[10]     // Error: index out of bounds
```

### List Literals
```rust
[1, 2, 3]              // List<Int>
["a", "b", "c"]        // List<String>
[]                     // Empty list
[1, 2, 3, 4, 5,]      // Trailing comma allowed

// Multiline
numbers = [
    1,
    2,
    3,
]
```

### Tuple Literals
```rust
(1, "hello")           // (Int, String)
(true, 42, "test")     // (Bool, Int, String)
((1, 2), (3, 4))       // Nested tuples
()                     // Unit tuple
```

### Record Literals
```rust
{ x: 10, y: 20 }       // Anonymous record
p: Point = { x: 10, y: 20 }  // Named type
{}                     // Empty record
```

### Map Literals
```rust
%{ "key1" => 1, "key2" => 2 }  // Map<String, Int>
%{ 1 => "one", 2 => "two" }    // Map<Int, String>
%{}                              // Empty map
```

### Boolean and Nil Literals
```rust
true
false
nil
```

### Character Literals
```rust
'a'
'\n'
'\\'
```

### Bytes Literals
```rust
@"Hello"      // UTF-8 bytes
@x"DEADBEEF"  // Hex bytes
@b"10101010"  // Binary bytes
```

### Bits Literals
```rust
#b"101010"    // Binary (bits)
#x"FF"        // Hexadecimal (4 bits per digit)
#o"755"       // Octal (3 bits per digit)
```

---

## 4. Operators

### Arithmetic Operators
```rust
a + b    // Addition
a - b    // Subtraction
a * b    // Multiplication
a / b    // Division (integer for Int)
a % b    // Remainder
a ** b   // Power
```

### Comparison Operators
```rust
a == b   // Equal
a != b   // Not equal
a < b    // Less than
a > b    // Greater than
a <= b   // Less than or equal
a >= b   // Greater than or equal
```

### Range Operator
```rust
1..10          // Range from 1 to 10 (inclusive)
(1, 3)..10     // Range with step (1, 3, 5, 7, 9)
'a'..'z'       // Character range
```

### Logical Operators
```rust
a && b   // Logical AND (short-circuit)
a || b   // Logical OR (short-circuit)
a ?? b   // Null Coalescing (returns a if not nil, else b)
```

### Bitwise Operators (Int only)
```rust
a & b    // AND
a | b    // OR
a ^ b    // XOR
~a       // NOT
a << b   // Left shift
a >> b   // Right shift
```

### Implicit Conversions
Numeric operations support implicit conversion from `Int` to `Float`:
```rust
1 + 2.5      // 3.5 (Float)
3.14 * 2     // 6.28 (Float)
```

### String/List Operators
```rust
a ++ b   // Concatenation
a :: b   // Cons (prepend element, right-associative)
```

### Pipe and Composition
```rust
x |> f              // Pipe: f(x)
x |> f |> g         // Chain: g(f(x))
f ,, g              // Composition: f(g(x)) (right-to-left)

// Pipe Placeholders
x |> f(_, y)        // f(x, y)
x |> f(y, _)        // f(y, x)
```

### Function Application Operator
The `$` operator has the lowest precedence, allowing you to avoid parentheses.

```rust
f $ x       // Equivalent to f(x)
print $ 1 + 2 * 3   // print(7)

// Useful with complex expressions
print $ [(x, y) | x <- 'a'..'z', y <- 1..13]
```

### User-Definable Operators
```rust
a <> b    // UserOpCombine
a <|> b   // UserOpChoose
a <*> b   // UserOpApply
a >>= b   // UserOpBind
a <$> b   // UserOpMap
a <:> b   // UserOpCons
a <~> b   // UserOpSwap
a => b    // UserOpImply
```

### Operators as Functions
```rust
add = (+)
print(add(1, 2))  // 3

sum = foldl((+), 0, [1, 2, 3])  // 6
```

### Error Propagation Operator
```rust
value = result?  // Unwraps Ok/Some, propagates Fail/Zero
```

---

## 5. Control Flow

### If Expression
```rust
if x > 0 {
    print("Positive")
} else {
    print("Non-positive")
}

// If as expression
val = if x > 0 { 1 } else { -1 }
print(if true { "Yes" } else { "No" })
```

### While Loop
```rust
i = 0
while i < 5 {
    print(i)
    i = i + 1
}
```

### For Loop (Conditional)
```rust
i = 0
for i < 5 {
    print(i)
    i = i + 1
}
```

### For-In Loop
```rust
for item in [1, 2, 3] {
    print(item)
}

// With range
for i in 0..4 {
    print(i)  // 0, 1, 2, 3, 4
}
```

### Loop Control
```rust
for x in [1, 2, 3, 4, 5] {
    if x == 3 {
        continue  // Skip iteration
    }
    if x == 5 {
        break  // Stop loop
    }
    print(x)
}
```

### Loop Return Values
```rust
// Returns last iteration value
res = for x in [1, 2, 3] {
    x * 2
}
print(res)  // 6

// break with value
found = for x in [1, 3, 4, 5] {
    if x % 2 == 0 {
        break x  // returns 4
    }
    x
}
```

### Match Expression
```rust
match x {
    1 -> print("One")
    2 -> print("Two")
    _ -> print("Other")
}
```

---

## 6. Functions

### Function Declaration
```rust
fun add(a: Int, b: Int) -> Int {
    a + b
}

// Return type inferred
fun square(x: Int) {
    x * x  // Returns Int
}
```

### Default Parameters
```rust
fun greet(name = "World") {
    print("Hello, ${name}!")
}

greet()         // Hello, World!
greet("Alice")  // Hello, Alice!

fun format(value, prefix = "[", suffix = "]") {
    prefix ++ value ++ suffix
}
```

### Anonymous Functions (Lambdas)
```rust
// Full syntax
fun(x: Int) { x * 2 }

// Short syntax
fun(x) -> x * 2

// Lambda sugar (Haskell-style)
\x -> x * 2
\x, y -> x + y
\ -> print("Lazy!")
```

### Variadic Functions
```rust
fun sum(args: ...Int) -> Int {
    acc = 0
    for x in args {
        acc = acc + x
    }
    acc
}

s = sum(1, 2, 3)  // 6
```

### Partial Application
```rust
fun add(a: Int, b: Int) -> Int { a + b }

add5 = add(5)       // Returns (Int) -> Int
print(add5(3))      // 8

// Chain partial applications
fun add3(a: Int, b: Int, c: Int) -> Int { a + b + c }
print(add3(1)(2)(3))  // 6

// Calling with Spread
nums = [2, 3]
sum(1, ...nums)  // 6
```

### Closures
```rust
fun makeCounter() {
    count = 0
    fun inc() {
        count = count + 1  // OK: mutates closure variable
        count
    }
    inc
}
```

### Extension Methods
```rust
type Point = { x: Int, y: Int }

fun (p: Point) distanceFromOrigin() -> Int {
    p.x * p.x + p.y * p.y
}

p: Point = { x: 3, y: 4 }
print(p.distanceFromOrigin())  // 25

// Argument Shorthand (for last record argument)
fun connect(config: { host: String, port: Int }) { ... }
connect(host: "localhost", port: 8080)
```

### Block Syntax (Trailing Block)
```rust
// Traditional
myFunc(arg1, arg2, { expr1, expr2 })

// Block syntax (lowercase identifiers only)
myFunc(arg1, arg2) {
    expr1
    expr2
}

// UI DSL example
div {
    span { text("Hello") }
    span { text("World") }
}
```

### Ignored Parameters
```rust
count = foldl(fun(acc, _) -> acc + 1, 0, [1, 2, 3])
zeroed = map(fun(_) -> 0, [1, 2, 3])
third = fun(_, _, x) -> x
```

### Tail Call Optimization
```rust
// Tail-recursive (optimized)
fun factorial(n: Int, acc: Int) -> Int {
    if n <= 1 {
        acc
    } else {
        factorial(n - 1, acc * n)  // Tail position
    }
}

// NOT tail-recursive
fun factorial_bad(n: Int) -> Int {
    if n <= 1 {
        1
    } else {
        n * factorial_bad(n - 1)  // Result used in operation
    }
}
```

### Argument Shorthand (Record Arguments)

```rust
fun connect(config: { host: String, port: Int }) { ... }

// Last argument can use shorthand
connect(host: "localhost", port: 8080)
// Equivalent to:
connect({ host: "localhost", port: 8080 })
```

### Spread in Function Calls

```rust
// Calling with Spread
nums = [2, 3]
sum(1, ...nums)  // 6
```

---

## 7. Collections

### Lists

#### Creation
```rust
xs = [1, 2, 3, 4, 5]
list = 0 :: xs          // Cons: prepend
names = ["Alice", "Bob"]
empty: List<Int> = []
```

#### Access
```rust
xs[0]      // First element
xs[2]      // Third element
xs[-1]     // Last element (negative index)
```

#### Operations
```rust
[1, 2] ++ [3, 4]        // Concatenation
0 :: [1, 2, 3]          // Cons
len([1, 2, 3])          // Length
```

#### List Comprehensions
```rust
// Basic
[x * 2 | x <- [1, 2, 3]]  // [2, 4, 6]

// With filter
[x | x <- [1, 2, 3, 4, 5], x % 2 == 0]  // [2, 4]

// Multiple generators
[(x, y) | x <- [1, 2], y <- [3, 4]]  // [(1, 3), (1, 4), (2, 3), (2, 4)]

// Pattern destructuring
[a + b | (a, b) <- [(1, 2), (3, 4)]]  // [3, 7]

// Multiple filters
[x | x <- 1..10, x > 3, x < 8]  // [4, 5, 6, 7]

// Flatten nested lists
[x | row <- matrix, x <- row]
```

#### Spread in Lists
```rust
xs = [1, 2, 3]
ys = [4, 5]
[0, ...xs]              // [0, 1, 2, 3]
[...xs, 4]              // [1, 2, 3, 4]
[...xs, ...ys]          // [1, 2, 3, 4, 5]
```

### Tuples

#### Creation and Access
```rust
pair = (1, "hello")
pair[0]   // 1
pair[1]   // "hello"
pair[-1]  // "hello"
```

#### Destructuring
```rust
(a, b) = pair
print(a)  // 1
print(b)  // "hello"

// In function parameters
fun printPair(p: (Int, String)) -> Nil {
    (x, y) = p
    print(x)
    print(y)
}
```

### Records

#### Creation
```rust
type Point = { x: Int, y: Int }
p = { x: 10, y: 20 }
p: Point = { x: 10, y: 20 }
```

#### Field Access and Modification
```rust
print(p.x)    // 10
p.x = 100     // Modify field
print(p.x)    // 100
```

#### Record Update (Spread)
```rust
base: Point = { x: 1, y: 2 }
updated = { ...base, x: 10 }  // { x: 10, y: 2 }
```

#### Destructuring
```rust
{ x: a, y: b } = p
print(a)  // 10

// Partial destructuring
{ name: n } = person

// Nested destructuring
{ user: { name: userName, role: r }, count: c } = data
```

#### Row Polymorphism
```rust
fun getX(r: { x: Int }) -> Int {
    r.x
}

point = { x: 10, y: 20 }
print(getX(point))  // 10 (additional fields ignored)
```

### Maps

#### Creation
```rust
import "lib/map" (*)

scores = %{ "Alice" => 100, "Bob" => 85 }
empty: Map<String, Int> = %{}
```

#### Access
```rust
scores["Alice"]              // Some(100)
mapGet(scores, "Alice")      // Some(100)
mapGetOr(scores, "Unknown", 0)  // 0
mapContains(scores, "Alice")  // true
```

#### Modification (Immutable)
```rust
scores2 = mapPut(scores, "Charlie", 92)
scores3 = mapRemove(scores, "Bob")
merged = mapMerge(m1, m2)
```

#### Iteration
```rust
keys = mapKeys(scores)       // ["Alice", "Bob"]
vals = mapValues(scores)     // [100, 85]
items = mapItems(scores)     // [("Alice", 100), ("Bob", 85)]
```

---

## 8. Pattern Matching

### Literal Patterns
```rust
match x {
    1 -> "One"
    2 -> "Two"
    true -> "True"
    "hello" -> "Greeting"
    _ -> "Other"
}
```

### Variable Patterns
```rust
match 42 {
    val -> print(val)  // val is 42
}
```

### Tuple Patterns
```rust
pair = (10, 20)
match pair {
    (0, 0) -> "Origin"
    (x, 0) -> "X axis"
    (0, y) -> "Y axis"
    (x, y) -> "Point"
}

// Spread in tuples
match (1, 2, 3, 4) {
    (head, ...tail) -> {
        print(head)  // 1
        print(tail)  // (2, 3, 4)
    }
}
```

### List Patterns
```rust
match [1, 2, 3] {
    [] -> "Empty"
    [head, ...tail] -> {
        print(head)  // 1
        print(tail)  // [2, 3]
    }
}

// Fixed size matching
match [1, 2] {
    [a, b] -> print(a + b)
    _ -> print("Not a pair")
}
```

### Record Patterns
```rust
r = { x: 10, y: 20, z: 30 }
match r {
    { x: 0, y: 0 } -> "Origin"  // Partial match
    { x: val } -> print(val)    // Extract x
}
```

### Constructor Patterns (ADTs)
```rust
opt = Some(42)
match opt {
    Some(val) -> print(val)
    Zero -> print("Nothing")
}
```

### Type Patterns
```rust
fun process(x: Int | String | Nil) -> String {
    match x {
        n: Int -> "Got Int: " ++ show(n)
        s: String -> "Got string: " ++ s
        _: Nil -> "Got nil"
    }
}
```

### Guard Patterns
```rust
fun classify(n: Int) -> String {
    match n {
        x if x > 0 -> "positive"
        x if x < 0 -> "negative"
        _ -> "zero"
    }
}

// Guards with destructuring
fun comparePair(pair: (Int, Int)) -> String {
    match pair {
        (a, b) if a == b -> "equal"
        (a, b) if a > b -> "first is greater"
        (a, b) -> "second is greater"
    }
}
```

### String Patterns with Captures
```rust
path = "/hello/world"
match path {
    "/hello/{name}" -> print("Hello " ++ name)  // Hello world
    _ -> print("Not found")
}

// Multiple captures
match "/users/42/posts/123" {
    "/users/{userId}/posts/{postId}" -> {
        print("User: " ++ userId)    // User: 42
        print("Post: " ++ postId)    // Post: 123
    }
}

// Greedy capture
match "/static/css/main/style.css" {
    "/static/{file...}" -> print("Serving: " ++ file)  // css/main/style.css
}

// Escaping braces
match "value {x}" {
    "value {{x}}" -> "matched literal {x}"
    "value {captured}" -> "captured: " ++ captured
}
```

### Pin Operator (^)
```rust
someAge = 18
user = { name: "Alice", age: 25 }

match user {
    { age: ^someAge } -> "Exact match!"  // Compares: 25 == 18? No
    _ -> "No match"
}

// Pin in tuples
x = 1
y = 2
match (1, 2) {
    (^x, ^y) -> "exact match"
    _ -> "no"
}
```

### Nested Patterns
```rust
match (1, [2, 3]) {
    (x, [y, ...z]) -> {
        print(x)  // 1
        print(y)  // 2
        print(z)  // [3]
    }
}
```

---

## 9. Types and Generics

### Generic Functions
```rust
fun id<t>(x: t) -> t {
    x
}

n = id(42)       // t is Int
s = id("hello")  // t is String
```

### Generic Types
```rust
type Box<t> = { value: t }
type Pair<a, b> = Pair(a, b)
```

### Type Parameters Convention
- **Lowercase**: type parameters (`t`, `u`, `a`, `b`)
- **Uppercase**: types, constructors, traits (`Int`, `Some`, `Order`)

### Type Annotations
```rust
x: Int = 42
pair: (Int, String) = (1, "hello")
list: List<Int> = [1, 2, 3]
opt: Option<Int> = Some(42)
```

### Runtime Type Checking
```rust
typeOf(x, Int)      // true
typeOf(x, String)   // false
getType(x)          // type(Int)
```

### Parameterized Type Checking
```rust
typeOf(o, MyOption)        // Check without parameter
typeOf(o, MyOption(Int))   // Check with parameter (use parentheses!)
```

### Rank-N Types
Funxy supports Rank-N types (universally quantified types) using the `forall` keyword. This allows passing polymorphic functions as arguments.

```rust
// A function that takes a polymorphic function
fun run(f: forall a. (a) -> a) {
    f(1)
    f("hello")
}

run(fun(x) -> x)
```

### Flow-Sensitive Typing
Funxy supports type narrowing within `if` blocks using `typeOf`. This allows working with Union types safely without pattern matching.

```rust
fun process(val: Int | String) {
    if typeOf(val, Int) {
        // val is narrowed to Int here
        print("Square: " ++ show(val * val))
    } else {
        // val is String here
        print("Length: " ++ show(len(val)))
    }
}
```

### Strict Mode
Funxy provides an optional strict mode that enforces rigorous type checks, disabling some implicit behaviors (like unsafe union narrowing).

```rust
directive "strict_types"  // Enable strict mode

// Default behavior (Loose Mode) allows implicit union handling:
// x: Int | String = 10
// print(x + 5)  // Works (runtime check)

// Strict Mode requires explicit type narrowing:
x: Int | String = 10
// print(x + 5)  // Error: Type mismatch (Int | String vs Int)

if typeOf(x, Int) {
    print(x + 5) // OK: Type narrowed to Int
}
```

---

## 10. Traits

### Trait Declaration
```rust
trait MyShow<t> {
    fun show(val: t) -> String
}
```

### Instance Implementation
```rust
instance MyShow Int {
    fun show(val: Int) -> String {
        "Int"
    }
}
```

### Default Implementations
```rust
trait MyCmp<t> {
    fun eq(a: t, b: t) -> Bool
    fun neq(a: t, b: t) -> Bool {
        if eq(a, b) { false } else { true }
    }
}

instance MyCmp Int {
    fun eq(a: Int, b: Int) -> Bool {
        a == b
    }
    // neq automatically available via default
}
```

### Trait Inheritance
```rust
trait MyCmp<t> {
    fun eq(a: t, b: t) -> Bool
}

trait MyOrder<t> : MyCmp<t> {
    fun compare(a: t, b: t) -> Ordering
}

// Must implement Cmp first
instance MyCmp Int { fun eq(a, b) -> Bool { a == b } }
instance MyOrder Int { fun compare(a, b) -> Ordering { ... } }
```

### Operator Overloading
```rust
instance Numeric MyInt {
    operator (+)(a: MyInt, b: MyInt) -> MyInt {
        MkMyInt(unbox(a) + unbox(b))
    }
    operator (-)(a: MyInt, b: MyInt) -> MyInt { ... }
    operator (*)(a: MyInt, b: MyInt) -> MyInt { ... }
    operator (/)(a: MyInt, b: MyInt) -> MyInt { ... }
    operator (%)(a: MyInt, b: MyInt) -> MyInt { ... }
    operator (**)(a: MyInt, b: MyInt) -> MyInt { ... }
}
```

### Constraints
```rust
fun display<t: MyShow>(x: t) -> String {
    show(x)
}

// Multiple constraints
fun process<t: MyShow, t: MyCmp>(x: t, y: t) -> String {
    if eq(x, y) { show(x) } else { "different" }
}
```

### Higher-Kinded Types (HKT)
```rust
instance Functor<Option> {
    fun fmap<a, b>(f: (a) -> b, fa: Option<a>) -> Option<b> {
        match fa {
            Some(x) -> Some(f(x))
            Zero -> Zero
        }
    }
}

// Partial type application for multi-parameter types
instance Functor Result<e> {
    fun fmap<e, a, b>(f: (a) -> b, fa: Result<e, a>) -> Result<e, b> {
        match fa {
            Ok(x) -> Ok(f(x))
            Fail(e) -> Fail(e)
        }
    }
}
```

### Multi-Parameter Type Classes (MPTC)

Funxy supports traits with multiple type parameters, enabling relationships between different types:

```rust
// Trait with two parameters defining type conversions
trait Convert<a, b> {
    fun convert(val: a) -> b
}

// Implement for specific type pairs
instance Convert<Int, String> {
    fun convert(val: Int) -> String { show(val) }
}

instance Convert<Bool, Int> {
    fun convert(val: Bool) -> Int { if val { 1 } else { 0 } }
}

// Usage - dispatch based on runtime types
s: String = convert(42)    // Int -> String
i: Int = convert(true)     // Bool -> Int
```

MPTC dispatch works through runtime type checking, allowing the same function name to work with different type combinations.

### Built-in Traits
- `Equal<t>` - `==`, `!=`
- `Order<t>` - `<`, `>`, `<=`, `>=` (inherits Equal)
- `Numeric<t>` - `+`, `-`, `*`, `/`, `%`, `**`
- `Bitwise<t>` - `&`, `|`, `^`, `<<`, `>>`
- `Concat<t>` - `++`
- `Default<t>` - `default(Type)`
- `Iter<c, t>` - `iter` method for `for` loops
- `Functor<f>` - `fmap` (HKT)

---

## 11. Modules and Imports

### Package Structure
```
math/
├── math.lang    ← Entry file (controls exports)
├── vector.lang  ← Internal file
└── matrix.lang  ← Internal file
```

### Export Syntax
```rust
package math (Vector, add)  // Export only Vector and add
package math (*)             // Export everything
package math !(A, B)        // Export everything except A and B
package math                // Export nothing (internal)
```

### Import Syntax
```rust
import "lib/list"           // Import as module object
import "lib/list" as l      // Import with alias
import "lib/list" (map)     // Import specific symbols
import "lib/list" (*)       // Import all symbols
```

### Single Import Rule
- Each module can be imported only once per file
- Choose one import style per module

### ADT Constructor Auto-Import
```rust
import "lib/sql" (SqlValue)

// Constructors auto-imported
v1 = SqlInt(42)
v2 = SqlString("hello")
```

### Qualified Trait Names
```rust
import "kit/sql" as orm

instance orm.Model User {
    fun tableName(u) { "users" }
    fun toRow(u) { }
}
```

### File Extensions
- Supported: `.lang`, `.funxy`, `.fx`
- All files in a package must use the same extension

---

## 12. Error Handling

### Panic (Unrecoverable)
```rust
fun myHead<t>(xs: List<t>) -> t {
    match xs {
        [x, ..._] -> x
        [] -> panic("myHead: empty list")
    }
}
```

### Result Type
```rust
type Result<e, a> = Ok a | Fail e

fun divide(a: Int, b: Int) -> Result<String, Int> {
    if b == 0 {
        Fail("division by zero")
    } else {
        Ok(a / b)
    }
}

match divide(10, 2) {
    Ok(value) -> print("Result: " ++ show(value))
    Fail(err) -> print("Error: " ++ err)
}
```

### Error Propagation (?)
```rust
import "lib/io" (fileRead, fileWrite)

fun copyFile(src: String, dst: String) -> Result<String, Int> {
    content = fileRead(src)?     // Fail → early return
    bytes = fileWrite(dst, content)?
    Ok(bytes)
}
```

### Option Type
```rust
type Option<t> = Some t | Zero

x = Some(42)
y = Zero

match x {
    Some(value) -> print(value)
    Zero -> print("Nothing")
}

// ? operator works with Option too
first = find(fun(x) -> x > 0, xs)?
Some(first * 2)
```

### Nullable Types (T?)
```rust
age: Int? = 25
name: String? = nil

// Optional Chaining (works with T?, Option, Result)
emp?.address?.city  // Returns nil if any part is nil

// Option Chaining
optUser?.name       // Returns Some(name) or Zero

// Result Chaining
userResult?.email   // Returns Ok(email) or propagates Fail

fun describe(x: Int?) -> String {
    match x {
        n: Int -> "Got number: " ++ show(n)
        _: Nil -> "Got nil"
    }
}
```

---

## 13. Built-in Functions

### Output
```rust
print("Hello")              // With newline
print(1, 2, 3)              // Multiple arguments
write("Hello")              // Without newline
```

### Type Conversion
```rust
floatToInt(3.7)      // 3
intToFloat(42)        // 42.0
show(42)              // "42"
sprintf("%d, %.2f", 42, 3.14159)  // "42, 3.14"
```

### Format String Literals
```rust
formatter = %".2f"
3.14159 |> formatter  // "3.14"

%"%s, %s!"("Hello", "World")  // "Hello, World!"
%"Name: %s, Age: %d"("Alice", 25)  // "Name: Alice, Age: 25"
```

### Parsing
```rust
read("42", Int)       // Some(42)
read("abc", Int)      // Zero
read("3.14", Float)   // Some(3.14)
read("true", Bool)    // Some(true)
```

### Introspection
```rust
getType(42)           // type(Int)
typeOf(42, Int)       // true
show([1, 2, 3])       // "[1, 2, 3]"
```

### Default Values
```rust
default(Int)          // 0
default(Float)        // 0.0
default(Bool)         // false
default(String)       // ""
```

### Functional Helpers
```rust
id(42)                // 42
const(1, 2)           // 1
len([1, 2, 3])        // 3
len("Hello")          // 5 (characters)
lenBytes("Привет")    // 12 (UTF-8 bytes)
```

---

## 14. Standard Library

### lib/list
```rust
import "lib/list" (*)

head(xs)              // Some(first)
tail(xs)              // Some(rest)
append(xs, x)         // xs ++ [x]
map(fun(x) -> x * 2, xs)
filter(fun(x) -> x > 0, xs)
foldl((+), 0, xs)
foldr((+), 0, xs)
reverse(xs)
sort(xs)
unique(xs)
zip(xs, ys)
unzip(pairs)
partition(pred, xs)
range(1, 5)           // [1, 2, 3, 4]
```

### lib/string
```rust
import "lib/string" (*)

stringSplit("a,b,c", ",")           // ["a", "b", "c"]
stringJoin(["a", "b"], ",")         // "a,b"
stringTrim("  hello  ")              // "hello"
stringToUpper("hello")               // "HELLO"
stringToLower("HELLO")               // "hello"
stringCapitalize("hello")            // "Hello"
stringReplace("hello", "l", "L")    // "heLlo"
stringReplaceAll("hello", "l", "L") // "heLLo"
stringStartsWith("hello", "hel")     // true
stringEndsWith("hello", "lo")       // true
stringIndexOf("hello", "ll")        // Some(2)
stringRepeat("ab", 3)                // "ababab"
stringPadLeft("42", 5, '0')          // "00042"
stringPadRight("42", 5, '-')        // "42---"
```

### lib/map
```rust
import "lib/map" (*)

mapGet(m, key)        // Option<v>
mapGetOr(m, key, def)  // v
mapContains(m, key)   // Bool
mapSize(m)             // Int
mapPut(m, key, val)    // Map<k, v>
mapRemove(m, key)      // Map<k, v>
mapMerge(m1, m2)       // Map<k, v>
mapKeys(m)             // List<k>
mapValues(m)           // List<v>
mapItems(m)            // List<(k, v)>
```

### lib/tuple
```rust
import "lib/tuple" (*)

fst(pair)              // First element
snd(pair)              // Second element
tupleGet(triple, 0)    // Element by index
tupleSwap((1, "a"))    // ("a", 1)
tupleDup(42)           // (42, 42)
mapFst(f, pair)
mapSnd(f, pair)
mapPair(f, g, pair)
curry(fn)
uncurry(fn)
```

### lib/io
```rust
import "lib/io" (*)

readLine()                          // Option<String>
fileRead("path.txt")                // Result<String, String>
fileWrite("path.txt", "content")    // Result<Int, String>
fileAppend("path.txt", "content")   // Result<Int, String>
fileExists("path.txt")              // Bool
fileSize("path.txt")                // Result<Int, String>
fileDelete("path.txt")              // Result<Nil, String>
```

### lib/time
```rust
import "lib/time" (*)

timeNow()      // Unix timestamp (seconds)
clockNs()      // Monotonic nanoseconds
clockMs()      // Monotonic milliseconds
sleep(2)       // Sleep seconds
sleepMs(500)   // Sleep milliseconds
```

### lib/bytes
```rust
import "lib/bytes" (*)

b = @"Hello"
bytesSlice(b, 0, 3)     // @"Hel"
bytesConcat(b1, b2)     // b1 ++ b2
bytesToHex(b)           // "48..."
bytesFromHex("FF")      // Result<Bytes, String>
bytesEncodeInt(42, 4, "big") // Encode int
bytesDecodeInt(b, "big")     // Decode int
```

### lib/bits
```rust
import "lib/bits" (*)

b = #b"1010"
bitsGet(b, 0)           // Some(1)
bitsSet(b, 0, 0)        // #b"0010"
bitsToBinary(b)         // "1010"
bitsToHex(b)            // "a"
bitsFromHex("FF")       // Result<Bits, String>
bitsConcat(b1, b2)
```

### lib/regex
```rust
import "lib/regex" (*)

regexMatch(pattern, str)              // Bool
regexFind(pattern, str)               // Option<String>
regexFindAll(pattern, str)            // List<String>
regexCapture(pattern, str)            // Option<List<String>>
regexReplace(pattern, repl, str)      // String
regexReplaceAll(pattern, repl, str)   // String
regexSplit(pattern, str)              // List<String>
```

### lib/sql
```rust
import "lib/sql" (*)

// Connection
db = sqlOpen("sqlite", ":memory:")?

// Execution
sqlExec(db, "CREATE TABLE users (id INT, name TEXT)", [])?
sqlExec(db, "INSERT INTO users VALUES ($1, $2)", [SqlInt(1), SqlString("Alice")])?

// Query
rows = sqlQuery(db, "SELECT * FROM users", [])?
row = sqlQueryRow(db, "SELECT * FROM users WHERE id=$1", [SqlInt(1)])?

// Types: SqlInt, SqlString, SqlFloat, SqlBool, SqlBytes, SqlNull
```

### lib/http
```rust
import "lib/http" (*)

// Client
resp = httpGet("https://example.com")?
print(resp.body)
print(resp.status)

httpPost("https://api.com", "data")
httpPostJson("https://api.com", { name: "Alice" })

// Server
fun handler(req) {
    print(req.method ++ " " ++ req.path)
    { status: 200, body: "Hello", headers: [] }
}
httpServe(8080, handler)
```

### lib/ws
```rust
import "lib/ws" (*)

// Client
conn = wsConnect("ws://echo.websocket.org")?
wsSend(conn, "Hello")
msg = wsRecv(conn)?
wsClose(conn)

// Server
wsServe(8080, fun(conn, msg) -> {
    "Echo: " ++ msg
})
```

### lib/flag
```rust
import "lib/flag" (*)

flagSet("port", 8080, "Port number")
flagParse()
port = flagGet("port")
args = flagArgs() // Non-flag arguments
```

### lib/task
See [15. Asynchronous Programming](#15-asynchronous-programming) for details.

---

## 15. Asynchronous Programming

Funxy supports asynchronous programming using Tasks for non-blocking I/O operations and concurrent execution.

### Basic Async/Await

```rust
import "lib/task" (async, await)

// Create an asynchronous task
task = async(fun() -> {
    // Long-running operation
    expensiveComputation()
    42
})

// Wait for completion
result = await(task)
print(result)  // 42
```

### Parallel Execution

```rust
import "lib/task" (async, await)

// Run multiple operations in parallel
task1 = async(fun() -> fetchUser(1))
task2 = async(fun() -> fetchUser(2))
task3 = async(fun() -> fetchUser(3))

// Wait for all results
user1 = await(task1)
user2 = await(task2)
user3 = await(task3)
```

### HTTP Requests

```rust
import "lib/task" (async, await)
import "lib/http" (httpGet)

fun fetchData(url: String) {
    match httpGet(url) {
        Ok(resp) -> resp.body
        Fail(e) -> "Error: " ++ e
    }
}

// Parallel HTTP requests
urls = ["https://api1.com", "https://api2.com"]
tasks = map(fun(url) -> async(fun() -> fetchData(url)), urls)
results = map(fun(t) -> await(t), tasks)
```

### Task Combinators

```rust
import "lib/task" (awaitAll, awaitAny)

// Wait for all tasks to complete
allResults = awaitAll([task1, task2, task3])

// Wait for first task to complete
firstResult = awaitAny([task1, task2, task3])
```

### Error Handling

Tasks can fail, and errors are propagated:

```rust
task = async(fun() -> {
    match riskyOperation() {
        Ok(result) -> result
        Fail(error) -> panic("Task failed: " ++ error)
    }
})

match await(task) {
    Ok(result) -> print("Success: " ++ result)
    Fail(error) -> print("Error: " ++ error)
}
```

### When to Use Async

- I/O operations (files, network)
- HTTP requests and API calls
- Database operations
- Any blocking operations

Async allows multiple operations to run concurrently without blocking the main thread.

---

## 16. Command Line Interface

### Basic Usage

```bash
# Run a Funxy program
funxy program.lang

# Compile to bytecode
funxy -c program.lang -o output.fbc

# Run bytecode
funxy -r output.fbc

# Run tests
funxy test .
funxy test ./tests/my_test.lang

# Show help
funxy --help
```

### Compilation Flags

- `-c, --compile`: Compile to bytecode
- `-o, --output`: Output file for compilation
- `-r, --run`: Run bytecode file
- `--debug`: Enable debug mode (VM backend only)
- `--help`: Show help information

### Library Usage

Command-line argument parsing is available through `lib/flag`:

```rust
import "lib/flag" (*)

// Define flags
flagSet("port", 8080, "Server port")
flagSet("verbose", false, "Verbose output")

// Parse arguments
flagParse()

// Use values
port = flagGet("port")
if flagGet("verbose") { print("Verbose mode") }
```

---


## 17. Bytecode Compilation

Funxy supports compilation to bytecode for improved performance and distribution.

### Command Line Usage

```bash
# Compile source to bytecode
funxy -c source.lang -o output.fbc

# Run compiled bytecode
funxy -r output.fbc

# Compile and run in one step
funxy source.lang
```

### Bytecode Format

- **Extension**: `.fbc` (Funxy Bytecode)
- **Magic**: `FXYB` header with version
- **Encoding**: Gob-encoded chunks with metadata
- **Features**: Preserves imports, operator functions, and complex data structures

### Compilation Process

1. **Parse**: Source code → AST
2. **Analyze**: Type checking and inference
3. **Compile**: AST → Bytecode instructions
4. **Serialize**: Bytecode → Binary format

### Limitations

- Module packages require tree-walk mode
- Some runtime features may differ from interpreter
- Import resolution happens at runtime

### Performance Benefits

- Faster startup (no parsing/analysis)
- Optimized instruction dispatch
- Reduced memory usage for repeated execution

---

## 18. Advanced Functional Programming

### Function Composition

Combine functions using the `,,` operator:

```rust
fun double(x: Int) -> Int { x * 2 }
fun increment(x: Int) -> Int { x + 1 }

// Compose: increment after double
composed = double ,, increment
result = composed(5)  // (5 * 2) + 1 = 11
```

### Partial Application

Apply some arguments now, others later:

```rust
fun add(a: Int, b: Int) -> Int { a + b }

// Partial application
add5 = add(5)  // Partially apply first argument
result = add5(3)  // 5 + 3 = 8
```

### Function Pipelines

Chain operations using the pipe operator:

```rust
import "lib/list" (filter, map, foldl)

numbers = [1, 2, 3, 4, 5, 6]

// Functional pipeline
sumOfSquares = numbers
    |> filter(fun(x) -> x % 2 == 0)  // [2, 4, 6]
    |> map(fun(x) -> x * x)           // [4, 16, 36]
    |> foldl(fun(acc, x) -> acc + x, 0)  // 56

// Pipe Placeholders
// Use _ to specify where the piped value goes
result = 10
    |> sub(20, _)    // sub(20, 10) = 10
    |> div(_, 2)     // div(10, 2) = 5
```

### Higher-Order Functions

Functions that take or return other functions:

```rust
// Function that returns a function
fun multiplier(factor: Int) -> ((Int) -> Int) {
    fun(x) { x * factor }
}

double = multiplier(2)
triple = multiplier(3)

print(double(5))  // 10
print(triple(5))  // 15
```

### Functor and Monad Operations

```rust
import "lib/list" (fmap)

// Functor: apply function inside container
numbers = [1, 2, 3]
doubled = fmap(fun(x) -> x * 2, numbers)  // [2, 4, 6]

// Monad: chain operations
import "lib/list" (flatMap)

result = [1, 2, 3]
    |> flatMap(fun(x) -> [x, x * 10])  // [1, 10, 2, 20, 3, 30]
```

### Currying and Uncurrying

```rust
import "lib/tuple" (curry, uncurry)

fun add(pair: (Int, Int)) -> Int { (a, b) = pair, a + b }

// Currying: (Int, Int) -> Int becomes Int -> Int -> Int
curriedAdd = curry(add)
result = curriedAdd(5)(3)  // 8

// Uncurrying: reverse the process
uncurried = uncurry(curriedAdd)
result = uncurried((5, 3))  // 8
```

### Do Notation

```rust
result = do {
    x <- Some(10)
    y <- Some(20)
    Some(x + y)
}

// With let binding
result = do {
    x <- Some(5)
    k :- 2  // let binding
    Some(x * k)
}

// Short-circuiting
result = do {
    x <- Zero  // Stops here
    y <- Some(20)  // Not executed
    Some(x + y)
}
```

---

## 19. Compiler Directives

### Directives
Directives control compiler behavior.

```rust
directive "strict_types"  // Enable stricter type checking
```

---

## 20. Tools and Debugging

### Debugger
Funxy includes a built-in debugger for the VM backend.

```bash
funxy -debug program.lang
```

Commands:
- `break`, `b <line>`: Set breakpoint
- `continue`, `c`: Continue execution
- `step`, `s`: Step into
- `next`, `n`: Step over
- `print`, `p <expr>`: Evaluate expression
- `locals`: Show local variables

See `docs/DEBUGGER.md` for full documentation.

---

## 21. Summary Tables

### Operators Precedence
1. Function application (`$`) - Lowest
2. Pipe (`|>`) - Low
3. Logical (`&&`, `||`) - Low
4. Comparison (`==`, `!=`, `<`, `>`, `<=`, `>=`) - Medium
5. Range (`..`) - Medium
6. Bitwise (`&`, `|`, `^`, `<<`, `>>`) - Medium
7. Arithmetic (`+`, `-`, `*`, `/`, `%`, `**`) - High
8. Cons (`::`) - High, Right-associative
9. Concatenation (`++`) - High

### Type System Summary
- **Structural types**: Anonymous records, tuples
- **Nominal types**: Named record types, ADTs
- **Union types**: `T | U`, `T?` (nullable)
- **Generic types**: `List<t>`, `Map<k, v>`
- **Higher-kinded types**: `Functor<f>` where `f` is `* -> *`

### Error Handling Summary
| Mechanism | Type | Success | Failure | Use Case |
|-----------|------|---------|---------|----------|
| `panic` | - | - | Stops execution | Programming errors |
| `Result<e, a>` | ADT | `Ok(value)` | `Fail(error)` | Recoverable errors |
| `Option<t>` | ADT | `Some(value)` | `Zero` | Absent values |
| `T?` | Union | `value` | `nil` | Nullable values |

---

*This reference manual covers the core features of the Funxy language. For more examples and tutorials, see the `tests/` and `docs/tutorial/` directories.*
