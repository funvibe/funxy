# Bytes - Binary Data

The `Bytes` type represents immutable sequences of bytes. It's essential for working with binary data, network protocols, file I/O, and encoding/decoding operations.

## Literals

Funxy provides three literal syntaxes for bytes:

```rust
// UTF-8 encoded string
hello = @"Hello, World!"

// Hexadecimal string
magic = @x"DEADBEEF"

// Binary string (8 bits per byte)
byte = @b"01001000"  // = @"H"
```

## Creation

```rust
import "lib/bytes" (*)

// Empty bytes
empty = bytesNew()

// From string (UTF-8 encoded)
fromStr = bytesFromString("hello")

// From list of byte values (0-255)
fromList = bytesFromList([72, 101, 108, 108, 111])  // "Hello"

// Parse hex string
match bytesFromHex("48656C6C6F") {
    Ok(b) -> print(b)   // @"Hello"
    Fail(e) -> print(e)
}

// Parse binary string
match bytesFromBin("01001000") {
    Ok(b) -> print(b)   // @"H"
    Fail(e) -> print(e)
}
```

## Indexing and Slicing

```rust
import "lib/bytes" (bytesSlice)

data = @"Hello"

// Indexing returns Option<Int>
data[0]   // Some(72) - 'H'
data[4]   // Some(111) - 'o'
data[10]  // Zero - out of bounds
data[-1]  // Some(111) - last byte

// Slicing
bytesSlice(data, 0, 3)  // @"Hel"
bytesSlice(data, 2, 5)  // @"llo"
```

## Concatenation

```rust
import "lib/bytes" (bytesConcat)

a = @"foo"
b = @"bar"

// Using operator
result1 = a ++ b  // @"foobar"

// Using function
result2 = bytesConcat(a, b)  // @"foobar"
```

## Comparison

Bytes support equality and lexicographic ordering:

```rust
@"abc" == @"abc"  // true
@"abc" != @"xyz"  // true
@"abc" < @"abd"   // true
@"a" < @"aa"      // true (shorter comes first)
```

## Conversion

```rust
import "lib/bytes" (bytesToHex, bytesToBin, bytesToList, bytesToString)

data = @"Hello"

// To hex string
bytesToHex(data)  // "48656c6c6f"

// To binary string
bytesToBin(@"H")  // "01001000"

// To list of ints
bytesToList(data)  // [72, 101, 108, 108, 111]

// To string (UTF-8 decode)
match bytesToString(data) {
    Ok(s) -> print(s)     // "Hello"
    Fail(e) -> print(e)   // Invalid UTF-8
}
```

## Search Operations

```rust
import "lib/bytes" (bytesContains, bytesIndexOf, bytesStartsWith, bytesEndsWith)

data = @"Hello World"

// Check presence
bytesContains(data, @"World")  // true
bytesContains(data, @"Foo")    // false

// Find position
bytesIndexOf(data, @"World")   // Some(6)
bytesIndexOf(data, @"Foo")     // Zero

// Prefix/suffix
bytesStartsWith(data, @"Hello")  // true
bytesEndsWith(data, @"World")    // true
```

## Split and Join

```rust
import "lib/bytes" (bytesSplit, bytesJoin)

// Split by separator
csv = @"a,b,c"
parts = bytesSplit(csv, @",")  // [@"a", @"b", @"c"]

// Join with separator
bytesJoin(parts, @"-")  // @"a-b-c"
```

## Numeric Encoding

Encode and decode numeric values:

```rust
import "lib/bytes" (bytesEncodeInt, bytesDecodeInt, bytesEncodeFloat, bytesDecodeFloat)

// Encode integer to bytes (requires endianness)
encoded = bytesEncodeInt(0x1234, 2, "big")
encoded[0]  // Some(0x12)
encoded[1]  // Some(0x34)

// Different endianness
bytesEncodeInt(0x1234, 2, "big")     // Big-endian (network byte order)
bytesEncodeInt(0x1234, 2, "little")  // Little-endian
bytesEncodeInt(0x1234, 2, "native")  // System-native endianness

// With signedness
bytesEncodeInt(-1, 4, "big-signed")
bytesEncodeInt(-1, 4, "little-signed")
bytesEncodeInt(-1, 4, "native-signed")

// Decode bytes to integer
data = @x"1234"
decoded = bytesDecodeInt(data, "big")          // 0x1234
bytesDecodeInt(data, "little")                 // Little-endian
bytesDecodeInt(data, "native")                 // System-native
bytesDecodeInt(data, "little-signed")          // With signedness

// Float encoding (4 bytes for float32, 8 for float64)
floatBytes = bytesEncodeFloat(3.14, 4)
decoded2 = bytesDecodeFloat(floatBytes, 4)
print(decoded2)  // 3.14...
```

### Endianness Specifiers

| Specifier | Description |
|-----------|-------------|
| `"big"` | Big-endian (default, network byte order) |
| `"little"` | Little-endian |
| `"native"` | System-native endianness |
| `"big-signed"` | Big-endian, signed interpretation |
| `"little-signed"` | Little-endian, signed interpretation |
| `"native-signed"` | Native endianness, signed interpretation |

## Practical Examples

### Parsing Binary Header

```rust
import "lib/bytes" (bytesSlice, bytesDecodeInt)

fun parseHeader(data: Bytes) -> { magic: Bytes, version: Int, length: Int } {
    // Read magic number (4 bytes)
    magic = bytesSlice(data, 0, 4)
    
    // Read version (2 bytes, big-endian)
    version = bytesDecodeInt(bytesSlice(data, 4, 6), "big")
    
    // Read length (4 bytes, big-endian)  
    length = bytesDecodeInt(bytesSlice(data, 6, 10), "big")
    
    { magic: magic, version: version, length: length }
}

// Example usage
header = @x"CAFEBABE00010000001A"
parsed = parseHeader(header)
print(parsed)
```

### Building Binary Message

```rust
import "lib/bytes" (bytesEncodeInt)

fun buildMessage(msgType: Int, payload: Bytes) -> Bytes {
    // Header: type (1 byte) + length (4 bytes)
    typeBytes = bytesEncodeInt(msgType, 1, "big")
    sizeBytes = bytesEncodeInt(len(payload), 4, "big")
    
    // Concatenate header and payload
    typeBytes ++ sizeBytes ++ payload
}

// Example
msg = buildMessage(1, @"Hello")
print(msg)
```

### Hex Dump Utility

```rust
import "lib/bytes" (bytesToHex, bytesFromList)
import "lib/list" (range)

fun hexDump(data: Bytes) -> Nil {
    for i in range(0, len(data)) {
        match data[i] {
            Some(b) -> {
                write(bytesToHex(bytesFromList([b])))
                write(" ")
                if (i + 1) % 16 == 0 {
                    print("")
                }
            }
            Zero -> Nil
        }
    }
}

// Example
hexDump(@"Hello World!")
```

## Notes

- `Bytes` is immutable - all operations return new byte sequences
- Indexing returns `Option<Int>` to handle out-of-bounds safely
- UTF-8 conversion can fail if bytes are not valid UTF-8
- Numeric encoding defaults to big-endian (network byte order)
- Use `"native"` for platform-specific data (e.g., shared memory, FFI)
- Use compound specifiers like `"big-signed"` or `"native-signed"` for clarity

