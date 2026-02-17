# Strings

## String Representation

Strings in our language are represented as `List<Char>` - a list of characters. This allows using all list operations for working with strings.

```rust
s = "hello"        // Type: String (alias for List<Char>)
print(s[0])        // 'h' - access to character
print(len(s))      // 5
```

## String Literal Types

### Regular Strings

```rust
s = "Hello, World!"
```

### Raw Strings (multi-line)

Use backticks and preserve formatting:

```rust
text = `This is
a multi-line
string`
```

### Interpolated Strings

Allow embedding expressions directly in the string using `${...}`:

```rust
name = "Alice"
age = 30

// Simple interpolation
print("Hello, ${name}!")  // Hello, Alice!

// Expressions in interpolation
x = 5
y = 3
print("${x} + ${y} = ${x + y}")  // 5 + 3 = 8

// Field access
person = { name: "Bob", age: 25 }
print("${person.name} is ${person.age}")  // Bob is 25

// Function calls
fun double(n) { n * 2 }
print("Double: ${double(10)}")  // Double: 20
```

### What can be used in `${...}`

- Variables: `${name}`
- Arithmetic expressions: `${a + b * 2}`
- Field access: `${person.name}`
- Indexing: `${list[0]}`
- Function calls: `${func(arg)}`
- Nested strings: `${"inner"}`

## String Concatenation

The `++` operator (preferred):

```rust
greeting = "Hello"
name = "World"
message = greeting ++ ", " ++ name ++ "!"
print(message)  // Hello, World!
```

But interpolation is usually more convenient and readable.

## Unicode

Strings work correctly with Unicode:

```rust
import "lib/list" (reverse)

s = "Привет"
print(len(s))      // 6 (characters, not bytes)
print(reverse(s))  // "тевирП"

for c in "日本語" {
    print(c)       // Prints each character
}
```

## lib/list functions on strings

Since `String = List<Char>`, all list functions work:

```rust
import "lib/list" (head, last, tail, init, take, drop, reverse, filter, foldl, find)

s = "hello"

// Length
print(len(s))                     // 5

// Access
print(head(s))                    // Some('h')
print(last(s))                    // Some('o')
print(tail(s))                    // "ello"
print(init(s))                    // "hell"

// Slices
print(take(s, 3))                 // "hel"
print(drop(s, 2))                 // "llo"

// Search
print(find(fun(c) -> c == 'e', s)) // Some('e')

// Transformation
print(reverse(s))                 // "olleh"
print(filter(fun(c) -> c != 'l', s)) // "heo"

// Fold
print(foldl(fun(acc, c) -> acc + 1, 0, s))  // 5
```

## lib/string module

The `lib/string` module provides specialized string operations:

```rust
import "lib/string" (*)
```

### Split and Join

```rust
import "lib/string" (stringSplit, stringJoin, stringLines, stringWords)

// stringSplit: (String, String) -> List<String>
print(stringSplit("a,b,c", ","))           // ["a", "b", "c"]
print(stringSplit("one::two", "::"))       // ["one", "two"]

// stringJoin: (List<String>, String) -> String
print(stringJoin(["a", "b", "c"], ","))    // "a,b,c"
print(stringJoin(["a", "b", "c"], ""))     // "abc"

// stringWords: (String) -> List<String>
// Splits by spaces
print(stringWords("hello   world  test"))  // ["hello", "world", "test"]
```

### Whitespace Trimming

```rust
// stringTrim: (String) -> String
stringTrim("  hello  ")             // "hello"

// stringTrimStart: (String) -> String
stringTrimStart("  hello")          // "hello"

// stringTrimEnd: (String) -> String
stringTrimEnd("hello  ")            // "hello"
```

### Case

```rust
// stringToUpper: (String) -> String
stringToUpper("hello")              // "HELLO"
stringToUpper("Привет")             // "ПРИВЕТ"

// stringToLower: (String) -> String
stringToLower("HELLO")              // "hello"

// stringCapitalize: (String) -> String
stringCapitalize("hello")           // "Hello"
```

### Search and Replace

```rust
// stringReplace: (String, String, String) -> String
// Replaces first occurrence
stringReplace("hello", "l", "L")    // "heLlo"

// stringReplaceAll: (String, String, String) -> String
// Replaces all occurrences
stringReplaceAll("hello", "l", "L") // "heLLo"
stringReplaceAll("banana", "a", "o") // "bonono"

// stringStartsWith: (String, String) -> Bool
stringStartsWith("hello", "hel")    // true
stringStartsWith("hello", "ell")    // false

// stringEndsWith: (String, String) -> Bool
stringEndsWith("hello", "lo")       // true
stringEndsWith("hello", "ll")       // false

// stringIndexOf: (String, String) -> Option<Int>
stringIndexOf("hello", "ll")      // Some(2)
stringIndexOf("hello", "x")       // None
```

### Repeat and Padding

```rust
import "lib/string" (stringRepeat, stringPadLeft, stringPadRight)

// stringRepeat: (String, Int) -> String
print(stringRepeat("ab", 3))               // "ababab"
print(stringRepeat("x", 5))                // "xxxxx"

// stringPadLeft: (String, Int, Char) -> String
print(stringPadLeft("42", 5, '0'))         // "00042"
print(stringPadLeft("hello", 3, '-'))      // "hello" (unchanged if >= length)

// stringPadRight: (String, Int, Char) -> String
print(stringPadRight("42", 5, '-'))        // "42---"
```

## Pipe Operator

String functions work great with pipe:

```rust
import "lib/string" (*)

result = "  HELLO WORLD  "
    |> stringTrim
    |> stringToLower
    |> stringCapitalize
print(result)  // "Hello world"

// CSV processing
csv = "a,b,c"
parts = stringSplit(csv, ",")
formatted = stringJoin(parts, " | ")
print(formatted)  // "a | b | c"
```

## Practical Examples

### Word Count

```rust
import "lib/string" (stringWords)

fun wordCount(text: String) -> Int {
    len(stringWords(text))
}

print(wordCount("hello world test"))  // 3
```

### Title Case

```rust
import "lib/string" (stringWords, stringJoin, stringCapitalize)
import "lib/list" (map)

fun titleCase(s: String) -> String {
    words = stringWords(s)
    capitalized = map(stringCapitalize, words)
    stringJoin(capitalized, " ")
}

print(titleCase("hello world"))  // "Hello World"
```

### Number Formatting

```rust
import "lib/string" (stringPadLeft)

fun formatNumber(n: Int, width: Int) -> String {
    stringPadLeft(show(n), width, '0')
}

print(formatNumber(42, 5))  // "00042"
```

### CSV Parsing

```rust
import "lib/string" (stringSplit, stringTrim)
import "lib/list" (map)

parseCSVLine = fun(line: String) -> List<String> {
    stringSplit(line, ",") |> map(stringTrim)
}

parseCSVLine("  a , b , c  ") // ["a", "b", "c"]
```

### Debug Output

```rust
fun debugLog(label, value) {
    print("[DEBUG] ${label} = ${value}")
}

x = 100
debugLog("x", x)  // [DEBUG] x = 100
```

## lib/string Summary

| Function | Type | Description |
|---------|-----|----------|
| `stringSplit` | `(String, String) -> List<String>` | Split by delimiter |
| `stringJoin` | `(List<String>, String) -> String` | Join with delimiter |
| `stringLines` | `(String) -> List<String>` | Split by lines |
| `stringWords` | `(String) -> List<String>` | Split by spaces |
| `stringTrim` | `(String) -> String` | Trim whitespace |
| `stringTrimStart` | `(String) -> String` | Trim left |
| `stringTrimEnd` | `(String) -> String` | Trim right |
| `stringToUpper` | `(String) -> String` | Convert to uppercase |
| `stringToLower` | `(String) -> String` | Convert to lowercase |
| `stringCapitalize` | `(String) -> String` | Capitalize first letter |
| `stringReplace` | `(String, String, String) -> String` | Replace first |
| `stringReplaceAll` | `(String, String, String) -> String` | Replace all |
| `stringStartsWith` | `(String, String) -> Bool` | Check prefix |
| `stringEndsWith` | `(String, String) -> Bool` | Check suffix |
| `stringIndexOf` | `(String, String) -> Option<Int>` | Find substring |
| `stringRepeat` | `(String, Int) -> String` | Repeat N times |
| `stringPadLeft` | `(String, Int, Char) -> String` | Pad left |
| `stringPadRight` | `(String, Int, Char) -> String` | Pad right |
