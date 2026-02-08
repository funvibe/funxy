# Big Numbers (lib/bignum)

The `lib/bignum` module provides functions for working with arbitrary precision numbers.

```rust
import "lib/bignum" (*)
```

## Types

- **BigInt** — arbitrary precision integers (literal with suffix `n`)
- **Rational** — arbitrary precision rational numbers (fractions) (literal with suffix `r`)

## BigInt

### Literals

```rust
a = 123456789012345678901234567890n
b = -42n
c = 0n
```

### Creation

```rust
import "lib/bignum" (*)

// bigIntNew: (String) -> BigInt
x = bigIntNew("999999999999999999999999")

// bigIntFromInt: (Int) -> BigInt
y = bigIntFromInt(42)
```

### Conversion

```rust
import "lib/bignum" (*)

// bigIntToString: (BigInt) -> String
x = bigIntNew("999999999999999999999999")
s = bigIntToString(x)  // "999999999999999999999999"

// bigIntToInt: (BigInt) -> Option<Int>
// Returns Some if value fits in Int, otherwise None
bigIntToInt(bigIntFromInt(100))  // Some(100)
bigIntToInt(bigIntNew("99999999999999999999"))  // None (too large)
```

### Operators

BigInt supports all arithmetic operators through the `Numeric` trait:

```rust
a = 1000000000000000000n
b = 2000000000000000000n

a + b   // 3000000000000000000
b - a   // 1000000000000000000
a * b   // 2000000000000000000000000000000000000
b / a   // 2
b % a   // 0
2n ** 100n  // 1267650600228229401496703205376
```

Comparison through the `Order` trait:

```rust
a = 1000000000000000000n
b = 2000000000000000000n

a < b   // true
a == a  // true
a > b   // false
```

## Rational

Rational numbers store exact fractions without loss of precision.

### Literals

```rust
half = 0.5r
third = 1.0r / 3.0r  // exactly 1/3, not approximation
```

### Creation

```rust
import "lib/bignum" (*)

// ratFromInt: (Int, Int) -> Rational
// Automatically simplifies the fraction
r1 = ratFromInt(1, 3)   // 1/3
r2 = ratFromInt(2, 4)   // 1/2 (simplified)
r3 = ratFromInt(-6, 8)  // -3/4 (simplified)

// ratNew: (BigInt, BigInt) -> Rational
num = bigIntNew("22")
denom = bigIntNew("7")
pi_approx = ratNew(num, denom)  // 22/7
```

### Accessing Numerator and Denominator

```rust
import "lib/bignum" (*)

// ratNumer: (Rational) -> BigInt
// ratDenom: (Rational) -> BigInt
frac = ratFromInt(3, 4)
bigIntToString(ratNumer(frac))    // "3"
bigIntToString(ratDenom(frac))  // "4"
```

### Conversion

```rust
// ratToFloat: (Rational) -> Option<Float>
// Returns None if result doesn't fit in Float (infinity/NaN)
ratToFloat(ratFromInt(1, 4))  // Some(0.25)
ratToFloat(ratFromInt(1, 3))  // Some(0.333...)

// ratToString: (Rational) -> String
// Format "numerator/denominator"
ratToString(ratFromInt(3, 4))  // "3/4"
ratToString(ratFromInt(7, 1))  // "7/1" or "7"
```

### Operators

Rational supports arithmetic through the `Numeric` trait:

```rust
import "lib/bignum" (*)

half = ratFromInt(1, 2)
third = ratFromInt(1, 3)

ratToString(half + third)  // "5/6"
ratToString(half - third)  // "1/6"
ratToString(half * third)  // "1/6"
ratToString(half / third)  // "3/2"
```

Comparison:

```rust
import "lib/bignum" (*)

half = ratFromInt(1, 2)
third = ratFromInt(1, 3)

half > third   // true
half == half   // true
```

## Practical Examples

### Exact Arithmetic

Floating point has rounding errors:

```rust
import "lib/bignum" (*)

// In regular float: 0.1 + 0.2 = 0.30000000000000004
// In Rational: exactly 3/10
sum = ratFromInt(1, 10) + ratFromInt(2, 10)
ratToString(sum)   // "3/10"
ratToFloat(sum)    // Some(0.3)
```

### Large Number Factorial

```rust
import "lib/bignum" (*)

fun factorial(n: BigInt) -> BigInt {
    if bigIntToInt(n) == Some(0) { 1n }
    else { n * factorial(n - 1n) }
}

factorial(20n)   // 2432902008176640000
factorial(100n)  // huge number (~158 digits)
```

### Fibonacci Numbers

```rust
import "lib/bignum" (*)
import "lib/list" (range)

fun fib(n: BigInt) -> BigInt {
    if bigIntToInt(n) == Some(0) { 0n }
    else if bigIntToInt(n) == Some(1) { 1n }
    else { fib(n - 1n) + fib(n - 2n) }
}

// Or iteratively for large n
fun fibIter(n: Int) -> BigInt {
    a = 0n
    b = 1n
    for i in range(0, n) {
        temp = a + b
        a = b
        b = temp
    }
    a
}

fibIter(100)  // 354224848179261915075
```

### Exact Financial Calculations

```rust
import "lib/bignum" (*)

// Money as cents in Rational
dollars = fun(d: Int, c: Int) -> Rational {
    ratFromInt(d * 100 + c, 100)
}

price = dollars(19, 99)    // $19.99
tax = ratFromInt(8, 100)  // 8%
total = price + price * tax

ratToString(total)  // exact amount
```

## Summary

### BigInt

| Function | Type | Description |
|---------|-----|----------|
| `bigIntNew` | `(String) -> BigInt` | Parse from string |
| `bigIntFromInt` | `(Int) -> BigInt` | Int → BigInt |
| `bigIntToString` | `(BigInt) -> String` | BigInt → String |
| `bigIntToInt` | `(BigInt) -> Option<Int>` | BigInt → Int (if fits) |

### Rational

| Function | Type | Description |
|---------|-----|----------|
| `ratFromInt` | `(Int, Int) -> Rational` | Create from numerator/denominator |
| `ratNew` | `(BigInt, BigInt) -> Rational` | Create from BigInt |
| `ratNumer` | `(Rational) -> BigInt` | Get numerator |
| `ratDenom` | `(Rational) -> BigInt` | Get denominator |
| `ratToFloat` | `(Rational) -> Option<Float>` | Rational → Float |
| `ratToString` | `(Rational) -> String` | Rational → "a/b" |

### Operators (work automatically)

- Arithmetic: `+`, `-`, `*`, `/`, `%`, `**`
- Comparison: `<`, `<=`, `>`, `>=`, `==`, `!=`
