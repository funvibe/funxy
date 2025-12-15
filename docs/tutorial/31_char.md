# Characters (lib/char)

The `lib/char` module provides functions for working with individual characters (Char).

```rust
import "lib/char" (*)
```

## Conversion

### charToCode

```rust
charToCode(c: Char) -> Int
```

Returns the Unicode code of a character.

```rust
charToCode('A')   // 65
charToCode('a')   // 97
charToCode('0')   // 48
charToCode(' ')   // 32
charToCode('\n')  // 10
charToCode('\t')  // 9
charToCode('\0')  // 0
charToCode('\\')  // 92
```

### charFromCode

```rust
charFromCode(code: Int) -> Char
```

Creates a character from a Unicode code.

```rust
charFromCode(65)     // 'A'
charFromCode(97)     // 'a'
charFromCode(1071)   // 'Я' (Cyrillic)
charFromCode(12354)  // 'あ' (Japanese hiragana)
```

## Classification

### charIsUpper

```rust
charIsUpper(c: Char) -> Bool
```

Checks if a character is an uppercase letter.

```rust
charIsUpper('A')  // true
charIsUpper('Z')  // true
charIsUpper('a')  // false
charIsUpper('5')  // false

// Works with Unicode
charIsUpper(charFromCode(1071))  // true ('Я')
```

### charIsLower

```rust
charIsLower(c: Char) -> Bool
```

Checks if a character is a lowercase letter.

```rust
charIsLower('a')  // true
charIsLower('z')  // true
charIsLower('A')  // false
charIsLower('5')  // false

// Works with Unicode
charIsLower(charFromCode(1103))  // true ('я')
```

## Case Conversion

### charToUpper

```rust
charToUpper(c: Char) -> Char
```

Converts a character to uppercase.

```rust
charToUpper('a')  // 'A'
charToUpper('z')  // 'Z'
charToUpper('A')  // 'A' (unchanged)
charToUpper('5')  // '5' (unchanged)

// Unicode
charToUpper(charFromCode(1103))  // 'я' -> 'Я'
```

### charToLower

```rust
charToLower(c: Char) -> Char
```

Converts a character to lowercase.

```rust
charToLower('A')  // 'a'
charToLower('Z')  // 'z'
charToLower('a')  // 'a' (unchanged)
charToLower('5')  // '5' (unchanged)

// Unicode
charToLower(charFromCode(1071))  // 'Я' -> 'я'
```

## Practical Examples

### Checking First Letter

```rust
import "lib/char" (charIsUpper)

fun startsWithUpper(s: String) -> Bool {
    if len(s) == 0 { false }
    else { charIsUpper(s[0]) }
}

print(startsWithUpper("Hello"))  // true
print(startsWithUpper("world"))  // false
```

### Capitalize

```rust
import "lib/char" (charToUpper)
import "lib/list" (tail)

fun capitalize(s: String) -> String {
    if len(s) == 0 { "" }
    else { charToUpper(s[0]) :: tail(s) }
}

print(capitalize("hello"))  // "Hello"
```

### ASCII Check

```rust
import "lib/char" (charToCode)

fun isAscii(c: Char) -> Bool {
    charToCode(c) < 128
}

fun isDigit(c: Char) -> Bool {
    code = charToCode(c)
    code >= 48 && code <= 57
}

fun isLetter(c: Char) -> Bool {
    code = charToCode(c)
    (code >= 65 && code <= 90) || (code >= 97 && code <= 122)
}

isDigit('5')   // true
isLetter('a')  // true
```

### ROT13

```rust
import "lib/char" (charToCode, charFromCode, charIsUpper, charIsLower)
import "lib/list" (map)

fun rot13char(c: Char) -> Char {
    code = charToCode(c)
    if charIsUpper(c) {
        charFromCode(65 + (code - 65 + 13) % 26)
    } else if charIsLower(c) {
        charFromCode(97 + (code - 97 + 13) % 26)
    } else {
        c
    }
}

fun rot13(s: String) -> String {
    map(rot13char, s)
}

print(rot13("Hello"))  // "Uryyb"
print(rot13("Uryyb"))  // "Hello"
```

## Working with Strings

Since `String = List<Char>`, lib/char functions work well with lib/list:

```rust
import "lib/char" (charToUpper, charIsLower)
import "lib/list" (map, filter, length)

s = "Hello World"

// All to uppercase (without lib/string)
upper = map(charToUpper, s)  // "HELLO WORLD"

// Only lowercase
lowers = filter(charIsLower, s)  // "elloorld"

// Count lowercase
count = length(filter(charIsLower, s))  // 8
```

## Summary

| Function | Type | Description |
|---------|-----|----------|
| `charToCode` | `(Char) -> Int` | Character → Unicode code |
| `charFromCode` | `(Int) -> Char` | Unicode code → character |
| `charIsUpper` | `(Char) -> Bool` | Uppercase letter? |
| `charIsLower` | `(Char) -> Bool` | Lowercase letter? |
| `charToUpper` | `(Char) -> Char` | To uppercase |
| `charToLower` | `(Char) -> Char` | To lowercase |

## Escape Sequences

In char literals, the following are supported:

| Escape | Value |
|--------|----------|
| `'\n'` | Newline (10) |
| `'\t'` | Tab (9) |
| `'\r'` | Carriage return (13) |
| `'\0'` | Null (0) |
| `'\\'` | Backslash (92) |
| `'\''` | Single quote (39) |
