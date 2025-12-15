# Regular Expressions (lib/regex)

The `lib/regex` module provides functions for working with regular expressions, based on RE2 (Go) syntax.

```rust
import "lib/regex" (*)
```

## Important: Escape Sequences

In this language's strings, backslash is passed as-is:

```rust
import "lib/regex" (regexMatch)

// Correct:
regexMatch("\d+", "abc123")  // \d = digit

// Incorrect (double backslash):
regexMatch("\\d+", "abc123")  // \\d = literal \d
```

## Functions

### matchRe

```rust
regexMatch(pattern: String, str: String) -> Bool
```

Checks if pattern matches anywhere in the string.

```rust
import "lib/regex" (regexMatch)

print(regexMatch("\d+", "abc123"))     // true
print(regexMatch("\d+", "abc"))        // false
print(regexMatch("^hello", "hello!"))  // true (starts with "hello")
print(regexMatch("world$", "hello world"))  // true (ends with "world")
```

### findRe

```rust
regexFind(pattern: String, str: String) -> Option<String>
```

Finds the first match of the pattern.

```rust
import "lib/regex" (regexFind)

result = regexFind("\d+", "abc123def456")
print(result)  // Some("123")

noMatch = regexFind("\d+", "abc")
print(noMatch)  // Zero
```

### findAllRe

```rust
regexFindAll(pattern: String, str: String) -> List<String>
```

Finds all matches of the pattern.

```rust
import "lib/regex" (regexFindAll)

matches = regexFindAll("\d+", "a1b22c333")
print(matches)  // ["1", "22", "333"]

noMatches = regexFindAll("\d+", "abc")
print(noMatches)  // []
```

### capture

```rust
regexCapture(pattern: String, str: String) -> Option<List<String>>
```

Extracts capture groups from the first match.

- Index 0: full match
- Index 1+: capture groups `(...)` in order of appearance

```rust
import "lib/regex" (regexCapture)

// Pattern with groups: (year)-(month)-(day)
result = regexCapture("(\d{4})-(\d{2})-(\d{2})", "Date: 2024-03-15")

match result {
    Some(groups) -> {
        print(groups[0])  // "2024-03-15" (full match)
        print(groups[1])  // "2024" (year)
        print(groups[2])  // "03" (month)
        print(groups[3])  // "15" (day)
    }
    Zero -> print("No match")
}
```

### replaceRe

```rust
regexReplace(pattern: String, replacement: String, str: String) -> String
```

Replaces the **first** match of the pattern.

```rust
import "lib/regex" (regexReplace)

result = regexReplace("\d+", "X", "a1b2c3")
print(result)  // "aXb2c3"
```

### replaceAllRe

```rust
regexReplaceAll(pattern: String, replacement: String, str: String) -> String
```

Replaces **all** matches of the pattern.

```rust
import "lib/regex" (regexReplaceAll)

// Replace all numbers
result = regexReplaceAll("\d+", "X", "a1b2c3")
print(result)  // "aXbXcX"

// Backreferences to groups ($1, $2, ...)
wrapped = regexReplaceAll("(\w+)", "[$1]", "hello world")
print(wrapped)  // "[hello] [world]"
```

### splitRe

```rust
regexSplit(pattern: String, str: String) -> List<String>
```

Splits string by pattern.

```rust
import "lib/regex" (regexSplit)

// By comma and spaces
parts = regexSplit(",\s*", "a, b,c,  d")
print(parts)  // ["a", "b", "c", "d"]

// By spaces
words = regexSplit("\s+", "hello   world   foo")
print(words)  // ["hello", "world", "foo"]
```

### validateRe

```rust
regexValidate(pattern: String) -> Result<Nil, String>
```

Validates regular expression syntax.

```rust
import "lib/regex" (regexValidate)

match regexValidate("\d+") {
    Ok(_) -> print("Valid")
    Fail(err) -> print("Error: ${err}")
}  // "Valid"

match regexValidate("[invalid") {
    Ok(_) -> print("Valid")
    Fail(err) -> print("Error: ${err}")
}  // "Error: ..."
```

## Pattern Syntax

RE2 (Go) syntax is supported:

| Pattern | Description |
|---------|----------|
| `.` | Any character (except newline) |
| `\d` | Digit [0-9] |
| `\D` | Not digit |
| `\w` | Word [a-zA-Z0-9_] |
| `\W` | Not word |
| `\s` | Whitespace character |
| `\S` | Not whitespace |
| `^` | Start of string |
| `$` | End of string |
| `*` | 0 or more |
| `+` | 1 or more |
| `?` | 0 or 1 |
| `{n}` | Exactly n times |
| `{n,m}` | From n to m times |
| `[abc]` | Any of characters |
| `[^abc]` | Any except |
| `(...)` | Capture group |
| `(?:...)` | Non-capture group |
| `a\|b` | a or b |

## Practical Examples

### Email Validation

```rust
import "lib/regex" (regexMatch)

fun isValidEmail(email: String) -> Bool {
    pattern = "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}"
    regexMatch(pattern, email)
}

print(isValidEmail("test@example.com"))  // true
print(isValidEmail("invalid"))           // false
```

### Data Extraction

```rust
import "lib/regex" (regexCapture, regexFindAll)

// Extract URLs from text
urls = regexFindAll("https?://[^\s]+", "Visit https://example.com or http://test.org")
print(urls)  // ["https://example.com", "http://test.org"]

// Extract URL parts
result = regexCapture("(https?)://([^/]+)(/.*)?", "https://example.com/path")
match result {
    Some(parts) -> {
        print(parts[1])  // "https" (protocol)
        print(parts[2])  // "example.com" (host)
        print(parts[3])  // "/path" (path)
    }
    Zero -> ()
}
```

### Data Cleaning

```rust
import "lib/regex" (regexReplaceAll, regexSplit)

// Remove HTML tags
clean = regexReplaceAll("<[^>]+>", "", "<p>Hello <b>world</b></p>")
print(clean)  // "Hello world"

// Normalize whitespace
normalized = regexReplaceAll("\s+", " ", "hello    world\nfoo")
print(normalized)  // "hello world foo"

// Parse CSV (simple case)
values = regexSplit(",", "a,b,c,d")
print(values)  // ["a", "b", "c", "d"]
```

## Summary

| Function | Type | Description |
|---------|-----|----------|
| `matchRe` | `(String, String) -> Bool` | Check match |
| `findRe` | `(String, String) -> Option<String>` | First match |
| `findAllRe` | `(String, String) -> List<String>` | All matches |
| `capture` | `(String, String) -> Option<List<String>>` | Capture groups |
| `replaceRe` | `(String, String, String) -> String` | Replace first |
| `replaceAllRe` | `(String, String, String) -> String` | Replace all |
| `splitRe` | `(String, String) -> List<String>` | Split |
| `validateRe` | `(String) -> Result<Nil, String>` | Validate pattern |

## Limitations

- Lookbehind assertions are not supported (RE2 limitation)
- Backreference in pattern (`\1`) is not supported
- Backreference in replacement (`$1`) is supported
