# Bits - Bit Sequences

`Bits` is an immutable sequence of bits that can have any length (not necessarily a multiple of 8). This makes it ideal for working with binary protocols, bit fields, and other low-level data manipulation.

## Literals

```rust
// Binary literals (1 bit per digit)
b1 = #b"10101010"    // 8 bits
b2 = #b"101"         // 3 bits (not byte-aligned!)
b3 = #b""            // empty

// Hex literals (4 bits per digit)
b4 = #x"FF"          // 8 bits: 11111111
b5 = #x"A5"          // 8 bits: 10100101

// Octal literals (3 bits per digit)
b6 = #o"7"           // 3 bits: 111
b7 = #o"377"         // 9 bits: 011111111
```

## Creating Bits

```rust
import "lib/bits" (*)

// Empty bits
empty = bitsNew()

// From Bytes
b1 = bitsFromBytes(@"Hello")

// From binary string
match bitsFromBinary("10101010") {
    Ok(b) -> print(b)
    Fail(e) -> print(e)
}

// From hex string
match bitsFromHex("FF") {
    Ok(b) -> print(b)
    Fail(e) -> print(e)
}
```

## Conversion

```rust
import "lib/bits" (*)

b = #b"10101010"

// To binary string
binary = bitsToBinary(b)  // "10101010"

// To hex string
hex = bitsToHex(b)        // "aa"

// To Bytes (with optional padding)
// "low" (default) - pad zeros at end
// "high" - pad zeros at beginning
bytes = bitsToBytes(b)          // default "low"
bytes = bitsToBytes(b, "high")  // explicit padding
```

## Operations

### Length and Access

```rust
import "lib/bits" (*)

b = #b"10101010"

// Length
print(len(b))            // 8

// Check if empty
print(len(b) == 0)       // false

// Get bit at index
match bitsGet(b, 0) {
    Some(bit) -> print(bit)  // 1
    None -> print("out of bounds")
}

// Slice [start, end)
part = bitsSlice(b, 0, 4)  // #b"1010"
```

### Modification

```rust
import "lib/bits" (*)

b1 = #b"1111"
b2 = #b"0000"

// Concatenation
concat1 = b1 ++ b2           // #b"11110000"
concat2 = bitsConcat(b1, b2) // #b"11110000"

// Set bit at index
b3 = bitsSet(#b"0000", 0, 1)  // #b"1000"

// Padding
padded1 = bitsPadLeft(#b"101", 8)   // #b"00000101"
padded2 = bitsPadRight(#b"101", 8)  // #b"10100000"
```

### Adding Numeric Values

```rust
import "lib/bits" (*)

b = bitsNew()

// Add integer (value, size in bits, endianness)
// Endianness is optional, defaults to "big"
b = bitsAddInt(b, 255, 8)              // default big endian
b = bitsAddInt(b, 255, 8, "big")       // explicit big endian
b = bitsAddInt(b, 1, 16, "little")     // little-endian 16-bit
b = bitsAddInt(b, 1, 16, "native")     // system-native endianness

// Signed integers
b = bitsAddInt(b, -1, 8, "big-signed")
b = bitsAddInt(b, -1, 16, "little-signed")

// Add float (value, size: 32 or 64)
b = bitsAddFloat(bitsNew(), 3.14, 32)  // IEEE 754 float
```

## Pattern Matching (Binary Parsing)

The pattern matching API allows you to extract structured data from bit sequences:

```rust
import "lib/bits" (*)
import "lib/map" (mapGet)

// Create a binary packet:
// - version: 8 bits
// - flags: 4 bits
// - reserved: 4 bits
// - length: 16 bits
packet = #b"00000001010100000000000100000000"

// Define extraction specs
specs = [
    bitsInt("version", 8, "big"),
    bitsInt("flags", 4, "big"),
    bitsInt("reserved", 4, "big"),
    bitsInt("length", 16, "big")
]

// Extract fields
match bitsExtract(packet, specs) {
    Ok(fields) -> {
        version = mapGet(fields, "version")  // Some(1)
        flags = mapGet(fields, "flags")      // Some(5)
        length = mapGet(fields, "length")    // Some(256)
        print("Version: ${show(version)}")
    }
    Fail(err) -> print("Parse error: ${err}")
}
```

### Spec Functions

```rust
// Integer field: bitsInt(name, size, endianness)
// Endianness: "big", "little", "big-signed", "little-signed"
bitsInt("count", 16, "big")
bitsInt("offset", 32, "little-signed")

// Bytes field: bitsBytes(name, size_in_bytes)
bitsBytes("payload", 16)

// Dynamic size from another field
bitsBytes("data", "length")  // uses value of "length" field

// Rest of bits: bitsRest(name)
bitsRest("tail")
```

## Comparison

```rust
// Equality
#b"101" == #b"101"  // true
#b"101" != #b"110"  // true
```

## Traits

`Bits` implements:
- `Eq` - equality comparison (`==`, `!=`)
- `Concat` - concatenation (`++`)

## Use Cases

- Binary protocol parsing (TCP/IP headers, file formats)
- Bit field manipulation
- Compression algorithms
- Cryptography
- Network packet inspection
- Embedded systems communication

