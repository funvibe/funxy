# 03. BigInt and Rational

## Task
Work with arbitrary precision numbers: large integers and exact fractions.

---

## BigInt — arbitrarily large integers

Regular `Int` is limited by machine word size. `BigInt` has no limit.

### Creation

```rust
import "lib/bignum" (bigIntNew, bigIntFromInt, bigIntToString, bigIntToInt)

// From string (for very large numbers)
huge = bigIntNew("123456789012345678901234567890")

// From regular Int
small = bigIntFromInt(42)

// Conversion back
print(bigIntToString(huge))  // "123456789012345678901234567890"

// To Int (might not fit!)
match bigIntToInt(small) {
    Some(n) -> print(n)       // 42
    Zero -> print("Too big!")
}
```

### Arithmetic

```rust
import "lib/bignum" (bigIntNew, bigIntToString)

a = bigIntNew("1000000000000000000")
b = bigIntNew("2000000000000000000")

// BigInt supports standard operators
sum = a + b   // 3000000000000000000
diff = b - a  // 1000000000000000000
prod = a * b  // 2000000000000000000000000000000000000
quot = b / a  // 2

print(bigIntToString(prod))
```

### Practical example: factorial

```rust
import "lib/bignum" (bigIntFromInt, bigIntToString)

fun factorial(n: Int) -> BigInt {
    fun go(i: Int, acc: BigInt) -> BigInt {
        if i <= 1 { acc } else { go(i - 1, acc * bigIntFromInt(i)) }
    }
    go(n, bigIntFromInt(1))
}

print(bigIntToString(factorial(50)))
// 30414093201713378043612608166064768844377641568960512000000000000
```

### Practical example: Fibonacci numbers

```rust
import "lib/bignum" (bigIntFromInt, bigIntToString)

fun fib(n: Int) -> BigInt {
    fun go(i: Int, a: BigInt, b: BigInt) -> BigInt {
        if i == 0 { a }
        else { go(i - 1, b, a + b) }
    }
    go(n, bigIntFromInt(0), bigIntFromInt(1))
}

print(bigIntToString(fib(100)))
// 354224848179261915075
```

---

## Rational — exact fractions

`Rational` stores numerator and denominator as `BigInt`. No precision loss!

### Creation

```rust
import "lib/bignum" (ratFromInt, ratNew, bigIntNew, ratNumer, ratDenom, bigIntToString)

// From two Int
half = ratFromInt(1, 2)       // 1/2
third = ratFromInt(1, 3)      // 1/3
twoThirds = ratFromInt(2, 3)  // 2/3

// From BigInt
huge = ratNew(bigIntNew("1"), bigIntNew("3"))

// Access to parts
print(bigIntToString(ratNumer(half)))  // "1"
print(bigIntToString(ratDenom(half)))  // "2"
```

### Arithmetic

```rust
import "lib/bignum" (ratFromInt, ratToString)

a = ratFromInt(1, 2)   // 1/2
b = ratFromInt(1, 3)   // 1/3

// Standard operators
sum = a + b    // 5/6
diff = a - b   // 1/6
prod = a * b   // 1/6
quot = a / b   // 3/2

print(ratToString(sum))   // "5/6"
print(ratToString(prod))  // "1/6"
print(ratToString(quot))  // "3/2"
```

### Compound operators

```rust
import "lib/bignum" (ratFromInt, ratToString)

r = ratFromInt(1, 2)

r += ratFromInt(1, 4)   // 3/4
r *= ratFromInt(2, 1)   // 3/2 (= 6/4 simplifies)
r -= ratFromInt(1, 2)   // 1/1 = 1

print(ratToString(r))  // "1/1"
```

### Automatic simplification

```rust
import "lib/bignum" (ratFromInt, ratToString)

// Fractions are automatically reduced to irreducible form
frac = ratFromInt(4, 8)  
print(ratToString(frac))  // "1/2" (not "4/8")

frac2 = ratFromInt(15, 25)
print(ratToString(frac2))  // "3/5"
```

### Conversion to Float

```rust
import "lib/bignum" (ratFromInt, ratToFloat)

r = ratFromInt(1, 3)

match ratToFloat(r) {
    Some(f) -> print(f)  // 0.3333333333333333
    Zero -> print("Cannot convert")
}
```

### Practical example: exact financial calculations

```rust
import "lib/bignum" (ratFromInt, ratToString)

// Float problem: 0.1 + 0.2 != 0.3
print(0.1 + 0.2)  // 0.30000000000000004

// Solution: Rational
a = ratFromInt(1, 10)   // 0.1
b = ratFromInt(2, 10)   // 0.2
c = ratFromInt(3, 10)   // 0.3

print(a + b == c)  // true

// Discount calculation
fun applyDiscount(price: Rational, discountPercent: Int) -> Rational {
    discount = ratFromInt(discountPercent, 100)
    price * (ratFromInt(1, 1) - discount)
}

price = ratFromInt(9999, 100)  // $99.99
discounted = applyDiscount(price, 15)  // 15% discount
print(ratToString(discounted))  // exact result!
```

### Practical example: compound interest

```rust
import "lib/bignum" (ratFromInt, ratToString)

// Compound interest: A = P * (1 + r/n)^(n*t)
// Exact calculation without precision loss

fun compoundInterest(
    principal: Rational,
    rate: Rational,
    times: Int,
    years: Int
) -> Rational {
    base = ratFromInt(1, 1) + rate / ratFromInt(times, 1)
    periods = times * years
    
    // Exponentiation
    fun pow(r: Rational, n: Int) -> Rational {
        if n == 0 { ratFromInt(1, 1) }
        else { r * pow(r, n - 1) }
    }
    
    principal * pow(base, periods)
}

// $1000 at 5% annual rate, monthly, 10 years
result = compoundInterest(
    ratFromInt(1000, 1),
    ratFromInt(5, 100),
    12,
    10
)
print(ratToString(result))
```

---

## Type comparison

| Type | Range | Precision | Speed |
|-----|----------|----------|----------|
| `Int` | ±2^63 | Exact | Fast |
| `Float` | ±10^308 | Approximate | Fast |
| `BigInt` | Infinite | Exact | Slower |
| `Rational` | Infinite | Exact | Slower |

---

## When to use

- Int — counters, indices, regular arithmetic
- Float — scientific calculations, graphics, where small error is acceptable
- BigInt — cryptography, factorials, Fibonacci numbers
- Rational — finance, exact proportions, where 0.1 + 0.2 = 0.3 is critical
