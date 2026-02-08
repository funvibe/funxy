# 2. Basics

[← Back to Index](./00_index.md)

## Comments

```rust
// Single-line comment

/*
   Multi-line
   comment
*/
```

## Variables (Mutable)

```rust
x = 42
x = 45  // OK: can be mutated
print(x)  // 45

name = "Alice"
name = "Bob"  // OK
```

## Constants (Immutable)

Use `const` (or `:-`) to declare constants. They cannot be reassigned.

```rust
const pi = 3.14159
const max_retries = 5
// or
e :- 2.718

// pi = 3.0  // Error: cannot reassign constant 'pi'
```

## Type Annotations

Types are inferred automatically, but you can specify them explicitly:

```rust
count: Int = 0
name: String = "Alice"
ratio: Float = 0.5
active: Bool = true
```

## Primitive Types

### Numbers

```rust
// Int - 64-bit integer
age = 30
hex = 0xFF        // 255
binary = 0b1010   // 10

// Float - Floating point
pi = 3.14159

// BigInt - Arbitrary precision
huge = 12345678901234567890n

// Rational - Exact rational numbers
half = 1r / 2r    // exactly 1/2
```

### Strings and Characters

```rust
// Strings
greeting = "Hello, World!"

// Interpolation
name = "Alice"
message = "Hello, ${name}!"  // Hello, Alice!

// Raw Strings (Multi-line)
text = `This is
a multi-line
string`

// Characters
letter = 'a'
newline = '\n'

// String is a List<Char>
"hello"[0]   // 'h'
"hello"[-1]  // 'o'
```

### Booleans and Nil

```rust
yes = true
no = false
nothing = nil
```

## Operators

### Arithmetic

```rust
a + b    // Addition
a - b    // Subtraction
a * b    // Multiplication
a / b    // Division
a % b    // Modulo
a ** b   // Exponentiation
```

### Comparison

```rust
a = 1
b = 2
a == b   // Equal
a != b   // Not equal
a < b    // Less than
a > b    // Greater than
a <= b   // Less than or equal
a >= b   // Greater than or equal
```

### Logical

```rust
a && b   // AND (short-circuit)
a || b   // OR (short-circuit)
a ?? b   // Null coalescing (a if not nil, else b)
```

### String and List

```rust
"Hello" ++ " World"  // Concatenation: "Hello World"
1 :: [2, 3]          // Cons: [1, 2, 3]
```

### Ranges

```rust
1..5           // [1, 2, 3, 4, 5]
'a'..'e'       // ['a', 'b', 'c', 'd', 'e']
(1, 3)..10     // [1, 3, 5, 7, 9] (with step)
```

## Scope Rules

Variables have lexical scope.

```rust
x = 1
{
    x = 2  // Mutates outer x
}
print(x)  // 2

// Global variables cannot be mutated from within functions
globalCounter = 0
fun increment() {
    // globalCounter = 1  // Error!
}
```
