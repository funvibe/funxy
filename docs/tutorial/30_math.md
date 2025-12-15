# Mathematical Functions (lib/math)

The `lib/math` module provides mathematical functions and constants.

```rust
import "lib/math" (*)
```

## Basic Operations

```rust
// abs: (Float) -> Float — absolute value
abs(-5.5)         // 5.5
abs(3.14)         // 3.14

// absInt: (Int) -> Int — absolute value of integer
absInt(-10)       // 10

// sign: (Float) -> Int — sign of number (-1, 0, 1)
sign(-3.14)       // -1
sign(0.0)         // 0
sign(42.0)        // 1

// min, max: (Float, Float) -> Float
min(3.0, 7.0)     // 3
max(3.0, 7.0)     // 7

// minInt, maxInt: (Int, Int) -> Int
minInt(3, 7)      // 3
maxInt(3, 7)      // 7

// clamp: (Float, Float, Float) -> Float — limit to range
clamp(5.0, 0.0, 10.0)   // 5 (in range)
clamp(-5.0, 0.0, 10.0)  // 0 (less than min)
clamp(15.0, 0.0, 10.0)  // 10 (greater than max)
```

## Rounding

```rust
// floor: (Float) -> Int — round down
floor(3.7)        // 3
floor(-3.7)       // -4

// ceil: (Float) -> Int — round up
ceil(3.2)         // 4
ceil(-3.2)        // -3

// round: (Float) -> Int — round to nearest
round(3.4)        // 3
round(3.5)        // 4

// trunc: (Float) -> Int — truncate fractional part
trunc(3.7)        // 3
trunc(-3.7)       // -3
```

## Powers and Roots

```rust
// sqrt: (Float) -> Float — square root
sqrt(16.0)        // 4
sqrt(2.0)         // 1.414...

// cbrt: (Float) -> Float — cube root
cbrt(27.0)        // 3
cbrt(-8.0)        // -2

// pow: (Float, Float) -> Float — power
pow(2.0, 10.0)    // 1024
pow(4.0, 0.5)     // 2 (square root)

// exp: (Float) -> Float — e^x
exp(0.0)          // 1
exp(1.0)          // e (~2.718)
```

## Logarithms

```rust
// log: (Float) -> Float — natural logarithm (ln)
log(1.0)          // 0
log(e())          // 1

// log10: (Float) -> Float — decimal logarithm
log10(100.0)      // 2
log10(1000.0)     // 3

// log2: (Float) -> Float — binary logarithm
log2(8.0)         // 3
log2(1024.0)      // 10
```

## Trigonometry

All functions work with radians.

```rust
// sin, cos, tan: (Float) -> Float
sin(0.0)          // 0
cos(0.0)          // 1
tan(0.0)          // 0

sin(pi() / 2.0)   // 1
cos(pi())         // -1

// asin, acos, atan: (Float) -> Float — inverse functions
asin(0.0)         // 0
acos(1.0)         // 0
atan(1.0)         // pi/4 (~0.785)

// atan2: (Float, Float) -> Float — angle of point (y, x)
atan2(1.0, 1.0)   // pi/4
```

## Hyperbolic Functions

```rust
sinh(0.0)         // 0
cosh(0.0)         // 1
tanh(0.0)         // 0
```

## Constants

```rust
// pi: () -> Float
pi()              // 3.141592653589793

// e: () -> Float — Euler's number
e()               // 2.718281828459045
```

## Conversion

```rust
// Functions intToFloat and floatToInt are now in prelude (don't require lib/math import)

// intToFloat: (Int) -> Float
intToFloat(42)       // 42.0

// floatToInt: (Float) -> Int — discards fractional part
floatToInt(3.99)       // 3
floatToInt(-3.99)      // -3
```

## Practical Examples

### Distance Between Points

```rust
import "lib/math" (sqrt)

fun distance(x1: Float, y1: Float, x2: Float, y2: Float) -> Float {
    dx = x2 - x1
    dy = y2 - y1
    sqrt(dx * dx + dy * dy)
}

distance(0.0, 0.0, 3.0, 4.0)  // 5
```

### Degrees to Radians

```rust
import "lib/math" (pi, sin, cos)

fun degToRad(deg: Float) -> Float {
    deg * pi() / 180.0
}

fun radToDeg(rad: Float) -> Float {
    rad * 180.0 / pi()
}

sin(degToRad(90.0))   // 1
cos(degToRad(180.0))  // -1
```

### Circle Area

```rust
import "lib/math" (pi)

fun circleArea(r: Float) -> Float {
    pi() * r * r
}

circleArea(1.0)   // pi
circleArea(2.0)   // 4*pi
```

### Normalize Value

```rust
import "lib/math" (clamp)

fun normalize(value: Float, min: Float, max: Float) -> Float {
    (clamp(value, min, max) - min) / (max - min)
}

normalize(5.0, 0.0, 10.0)   // 0.5
normalize(-5.0, 0.0, 10.0)  // 0
normalize(15.0, 0.0, 10.0)  // 1
```

### Quadratic Equation

```rust
import "lib/math" (sqrt, abs)

fun solveQuadratic(a: Float, b: Float, c: Float) -> Option<(Float, Float)> {
    discriminant = b * b - 4.0 * a * c
    if discriminant < 0.0 {
        Zero
    } else {
        sqrtD = sqrt(discriminant)
        x1 = (-b + sqrtD) / (2.0 * a)
        x2 = (-b - sqrtD) / (2.0 * a)
        Some((x1, x2))
    }
}

// x² - 5x + 6 = 0 → x = 2, x = 3
solveQuadratic(1.0, -5.0, 6.0)  // Some((3, 2))
```

## Summary

| Function | Type | Description |
|---------|-----|----------|
| `abs` | `(Float) -> Float` | Absolute value |
| `absInt` | `(Int) -> Int` | Absolute value of integer |
| `sign` | `(Float) -> Int` | Sign (-1, 0, 1) |
| `min` | `(Float, Float) -> Float` | Minimum |
| `max` | `(Float, Float) -> Float` | Maximum |
| `minInt` | `(Int, Int) -> Int` | Minimum of integers |
| `maxInt` | `(Int, Int) -> Int` | Maximum of integers |
| `clamp` | `(Float, Float, Float) -> Float` | Limit to range |
| `floor` | `(Float) -> Int` | Round down |
| `ceil` | `(Float) -> Int` | Round up |
| `round` | `(Float) -> Int` | Round to nearest |
| `trunc` | `(Float) -> Int` | Truncate fraction |
| `sqrt` | `(Float) -> Float` | Square root |
| `cbrt` | `(Float) -> Float` | Cube root |
| `pow` | `(Float, Float) -> Float` | Power |
| `exp` | `(Float) -> Float` | e^x |
| `log` | `(Float) -> Float` | Natural logarithm |
| `log10` | `(Float) -> Float` | Decimal logarithm |
| `log2` | `(Float) -> Float` | Binary logarithm |
| `sin` | `(Float) -> Float` | Sine |
| `cos` | `(Float) -> Float` | Cosine |
| `tan` | `(Float) -> Float` | Tangent |
| `asin` | `(Float) -> Float` | Arcsine |
| `acos` | `(Float) -> Float` | Arccosine |
| `atan` | `(Float) -> Float` | Arctangent |
| `atan2` | `(Float, Float) -> Float` | Angle of point (y, x) |
| `sinh` | `(Float) -> Float` | Hyperbolic sine |
| `cosh` | `(Float) -> Float` | Hyperbolic cosine |
| `tanh` | `(Float) -> Float` | Hyperbolic tangent |
| `pi` | `() -> Float` | Number π |
| `e` | `() -> Float` | Number e |

**Note:** Functions `intToFloat` and `floatToInt` are moved to prelude (available without import).
