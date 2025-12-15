# 02. Bytes and Bits

## Task
Work with binary data: protocols, files, network packets.

---

## Bits — arbitrary bit sequences

Unique feature: length doesn't have to be a multiple of 8! Ideal for protocols with bit fields.

### Literals

```rust
// Binary literals (1 bit per character)
b1 = #b"10101010"    // 8 bits
b2 = #b"101"         // 3 bits (not byte-aligned!)
b3 = #b""            // empty

// Hex literals (4 bits per character)
b4 = #x"FF"          // 8 bits: 11111111
b5 = #x"A5"          // 8 bits: 10100101

// Octal literals (3 bits per character)
b6 = #o"7"           // 3 bits: 111
b7 = #o"377"         // 9 bits: 011111111

print(b1)
print(b4)
```

### Creation and conversion

```rust
import "lib/bits" (bitsFromBytes, bitsToBinary, bitsFromBinary, bitsToHex, bitsFromHex, bitsToBytes)

// From strings
match bitsFromBinary("10101010") {
    Ok(b) -> print(bitsToBinary(b))  // "10101010"
    Fail(e) -> print(e)
}

match bitsFromHex("DEADBEEF") {
    Ok(b) -> print(bitsToHex(b))  // "deadbeef"
    Fail(e) -> print(e)
}
```

### Operations

```rust
import "lib/bits" (bitsGet, bitsSlice, bitsSet, bitsPadLeft, bitsPadRight)

b = #b"10101010"

// Length
print(len(b))  // 8

// Index access
match bitsGet(b, 0) {
    Some(bit) -> print(bit)  // 1
    Zero -> print("out of bounds")
}

// Slice [start, end)
part = bitsSlice(b, 0, 4)
print(part)  // #b"1010"

// Concatenation
joined = #b"1111" ++ #b"0000"
print(joined)  // #b"11110000"

// Padding
padL = bitsPadLeft(#b"101", 8)
print(padL)  // #b"00000101"

padR = bitsPadRight(#b"101", 8)
print(padR)  // #b"10100000"
```

### Adding numeric values

```rust
import "lib/bits" (bitsNew, bitsAddInt, bitsAddFloat)

b = bitsNew()

// Integers (value, size in bits, endianness)
b = bitsAddInt(b, 255, 8)              // big endian (default)
b = bitsAddInt(b, 255, 8, "big")       // explicitly big endian
b = bitsAddInt(b, 1, 16, "little")     // little endian

// Float (IEEE 754)
bf = bitsAddFloat(bitsNew(), 3.14, 32)  // 32 bits
print(bf)
```

### Pattern Matching — parsing binary protocols

```rust
import "lib/bits" (bitsExtract, bitsInt)
import "lib/map" (mapGet)

// Packet with bit fields:
// - version: 8 bits
// - flags: 4 bits
// - reserved: 4 bits
// - length: 16 bits
packet = #b"00000001010100000000000100000000"

// Field specifications
specs = [
    bitsInt("version", 8, "big"),
    bitsInt("flags", 4, "big"),
    bitsInt("reserved", 4, "big"),
    bitsInt("length", 16, "big")
]

// Extraction!
match bitsExtract(packet, specs) {
    Ok(fields) -> {
        version = mapGet(fields, "version")
        flags = mapGet(fields, "flags")
        length = mapGet(fields, "length")
        print("Version: " ++ show(version))
        print("Flags: " ++ show(flags))
        print("Length: " ++ show(length))
    }
    Fail(err) -> print("Parse error: " ++ err)
}
```

### Spec functions for parsing

```rust
import "lib/bits" (bitsInt, bitsBytes, bitsRest)

// Integer: bitsInt(name, size_in_bits, endianness)
spec1 = bitsInt("count", 16, "big")
spec2 = bitsInt("offset", 32, "little")

// Bytes: bitsBytes(name, size_in_bytes)
spec3 = bitsBytes("payload", 16)

// Fixed size
spec4 = bitsBytes("data", 64)

// Rest of bits
spec5 = bitsRest("tail")

```

### Practical example: PNG check

```rust
import "lib/bits" (bitsExtract, bitsInt, bitsFromBytes)
import "lib/map" (mapGet)

fun isPNG(data) -> Bool {
    specs = [
        bitsInt("magic1", 32, "big"),
        bitsInt("magic2", 32, "big")
    ]
    match bitsExtract(data, specs) {
        Ok(fields) -> {
            m1 = mapGet(fields, "magic1")
            m2 = mapGet(fields, "magic2")
            match (m1, m2) {
                (Some(v1), Some(v2)) -> v1 == 0x89504E47 && v2 == 0x0D0A1A0A
                _ -> false
            }
        }
        Fail(_) -> false
    }
}

// PNG magic bytes: 89 50 4E 47 0D 0A 1A 0A
print(isPNG(#x"89504E470D0A1A0A"))  // true
print(isPNG(#x"FFD8FFE0"))          // false (this is JPEG)
```

---

## Bytes — byte sequences

### Creation

```rust
import "lib/bytes" (bytesFromString, bytesFromList, bytesFromHex, bytesToString)

// From string
b = bytesFromString("Hello")

// From list of bytes
b2 = bytesFromList([0x48, 0x65, 0x6C, 0x6C, 0x6F])

// From hex string
match bytesFromHex("48656C6C6F") {
    Ok(b) -> {
        match bytesToString(b) {
            Ok(s) -> print(s)  // "Hello"
            Fail(e) -> print(e)
        }
    }
    Fail(e) -> print(e)
}
```

### Conversion

```rust
import "lib/bytes" (bytesFromString, bytesToString, bytesToList, bytesToHex)

b = bytesFromString("Hello")

// To string
match bytesToString(b) {
    Ok(s) -> print(s)  // "Hello"
    Fail(e) -> print("Not valid UTF-8")
}

// To list of bytes
list = bytesToList(b)
print(list)  // [72, 101, 108, 108, 111]

// To hex
hex = bytesToHex(b)
print(hex)  // "48656c6c6f"
```

### Operations

```rust
import "lib/bytes" (bytesFromString, bytesConcat, bytesSlice, bytesContains, bytesStartsWith, bytesEndsWith, bytesIndexOf, bytesSplit, bytesJoin)

b1 = bytesFromString("Hello")
b2 = bytesFromString(" World")

// Concatenation
joined = bytesConcat(b1, b2)

// Slice
part = bytesSlice(b1, 0, 3)  // "Hel"

// Search
print(bytesContains(b1, bytesFromString("ell")))     // true
print(bytesStartsWith(b1, bytesFromString("He")))    // true
print(bytesEndsWith(b1, bytesFromString("lo")))      // true
print(bytesIndexOf(b1, bytesFromString("ll")))       // Some(2)

// Split/Join
parts = bytesSplit(joined, bytesFromString(" "))
back = bytesJoin(parts, bytesFromString("-"))
```

### Numeric encoding

```rust
import "lib/bytes" (bytesEncodeInt, bytesDecodeInt, bytesEncodeFloat, bytesDecodeFloat)

// Int -> Bytes
b = bytesEncodeInt(256, 2)            // 2 bytes, big endian (default)
b2 = bytesEncodeInt(256, 2, "little") // little endian

// Bytes -> Int
n = bytesDecodeInt(b)
print(n)  // 256

// Float
bf = bytesEncodeFloat(3.14, 4)    // 32-bit float
f = bytesDecodeFloat(bf, 4)
print(f)  // 3.14...
```

---

## Comparison: Bits vs Bytes

| Aspect | Bits | Bytes |
|--------|------|-------|
| Minimum unit | 1 bit | 8 bits |
| Literals | `#b"101"`, `#x"FF"`, `#o"7"` | `bytesFromString("...")` |
| Not multiple of 8 | Yes | No |
| Pattern matching | bitsExtract | — |
| Working with protocols | Excellent | Good |
| Working with files | Possible | Excellent |

---

## Use Cases

Bits:
- TCP/IP headers with bit flags
- Compression (Huffman codes)
- Video codecs (H.264 NAL units)
- Cryptography
- Embedded protocols

Bytes:
- File I/O
- HTTP bodies
- JSON/XML data
- Images
