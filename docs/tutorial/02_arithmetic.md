# Arithmetic and Numbers

Numeric types and operations in Funxy.

## Data Types

| Type | Description |
|-----|----------|
| `Int` | 64-bit integer |
| `Float` | Floating point number |
| `BigInt` | Arbitrary precision integer |
| `Rational` | Rational number |
| `Bool` | Boolean value |

## Numeric Literals

| Format | Example | Type |
|--------|--------|-----|
| Decimal | `123`, `-42` | `Int` |
| Hexadecimal | `0xFF` | `Int` |
| Octal | `0o777` | `Int` |
| Binary | `0b101` | `Int` |
| Float | `1.5`, `3.14` | `Float` |
| BigInt | `100n`, `0xFFn` | `BigInt` |
| Rational | `1.5r`, `10r` | `Rational` |

## Operators

### Arithmetic

| Operation | Syntax | Note |
|----------|-----------|------------|
| Addition | `a + b` | |
| Subtraction | `a - b` | |
| Multiplication | `a * b` | |
| Division | `a / b` | Int - integer |
| Remainder | `a % b` | |
| Power | `a ** b` | |

### Comparisons

| Operation | Syntax |
|----------|-----------|
| Equal | `a == b` |
| Not equal | `a != b` |
| Less than | `a < b` |
| Greater than | `a > b` |
| Less than or equal | `a <= b` |
| Greater than or equal | `a >= b` |

### Bitwise Operations (Int only)

| Operation | Syntax |
|----------|-----------|
| AND | `a & b` |
| OR | `a | b` |
| XOR | `a ^ b` |
| NOT | `~a` |
| Left shift | `a << b` |
| Right shift | `a >> b` |

## Examples

```rust
print(1 + 2)      // 3
print(1.5 + 2.5)  // 4.0
print(2 ** 10)    // 1024
print(~0b101)     // -6
print(7 % 3)      // 1
```

## Built-in Functions

| Function | Description |
|---------|----------|
| `print(args...)` | Output to stdout |
| `len(collection)` | Length of list or tuple |
| `getType(val)` | Type of value |
| `typeOf(val, Type)` | Type check |
| `panic(msg)` | Abort with error |

```rust
print(len([1, 2, 3]))     // 3
print(getType(42))        // type(Int)

x = 10
if typeOf(x, Int) {
    print("x is Int")
}
```

## Tests

See `tests/arithmetic.lang`
