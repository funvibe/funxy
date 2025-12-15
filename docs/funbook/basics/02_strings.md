# 02. Working with Strings

## Task
Manipulate text data: split, join, search, format.

## Basics

```rust
// Strings are List<Char>
s = "Hello, World!"

// Length
print(len(s))  // 13

// Index access
print(s[0])    // 'H'
print(s[7])    // 'W'

// Concatenation
full = "Hello" ++ ", " ++ "World!"
```

## Interpolation

```rust
name = "Alice"
age = 30

// ${expression} inside string
print("Name: ${name}, Age: ${age}")
// Name: Alice, Age: 30

// Calculations inside
print("In 10 years: ${age + 10}")
// In 10 years: 40
```

## Multi-line Strings

```rust
poem = "Roses are red,
Violets are blue,
Funxy is awesome,
And so are you!"

print(poem)
```

## Escape Sequences

```rust
print("Line 1\nLine 2")     // newline
print("Tab\there")          // tab
print("Quote: \"Hi\"")      // quotes
print("Backslash: \\")      // backslash
print("Dollar: \$")         // dollar (for interpolation)
print("Null: \0")           // null byte
```

## lib/string — Powerful Functions

```rust
import "lib/string" (*)

s = "  Hello, World!  "

// Trimming
print(stringTrim(s))       // "Hello, World!"
print(stringTrimStart(s))  // "Hello, World!  "
print(stringTrimEnd(s))    // "  Hello, World!"

// Case
print(stringToUpper("hello"))      // "HELLO"
print(stringToLower("HELLO"))      // "hello"
print(stringCapitalize("hello"))   // "Hello"

// Split/Join
parts = stringSplit("a,b,c", ",")  // ["a", "b", "c"]
joined = stringJoin(parts, "-")    // "a-b-c"

// Convenient splits
lines = stringLines("a\nb\nc")     // ["a", "b", "c"]
words = stringWords("hello world") // ["hello", "world"]

// Search
print(stringStartsWith("hello", "he"))   // true
print(stringEndsWith("hello", "lo"))     // true
print(stringIndexOf("hello", "ll"))      // Some(2)

// Replacement
print(stringReplace("hello", "l", "L"))     // "heLlo" (first)
print(stringReplaceAll("hello", "l", "L"))  // "heLLo" (all)

// Repeat and padding
print(stringRepeat("ab", 3))              // "ababab"
print(stringPadLeft("42", 5, '0'))        // "00042"
print(stringPadRight("hi", 5, ' '))       // "hi   "
```

## lib/char — Working with Characters

```rust
import "lib/char" (*)

// Character code
print(charToCode('A'))  // 65
print(charFromCode(65)) // 'A'

// Checks
print(charIsUpper('A'))  // true
print(charIsLower('a'))  // true

// Conversion
print(charToUpper('a'))  // 'A'
print(charToLower('Z'))  // 'z'
```

## Type Conversion

```rust
// To string
print(show(42))       // "42"
print(show(3.14))     // "3.14"
print(show(true))     // "true"
print(show([1,2,3]))  // "[1, 2, 3]"

// From string (returns Option)
print(read("42", Int))        // Some(42)
print(read("3.14", Float))    // Some(3.14)
print(read("abc", Int))       // Zero
```

## Strings as Lists

```rust
import "lib/list" (map, filter, foldl, reverse, take, drop, contains)
import "lib/char" (charToUpper, charToCode)

// String is List<Char>, so all list functions work!

s = "hello"

// map
upper = map(charToUpper, s)
print(upper)  // "HELLO"

// filter
noVowels = filter(fun(c) -> !contains("aeiou", c), s)
print(noVowels)

// foldl
codes = foldl(fun(acc, c) -> acc + charToCode(c), 0, s)
print(codes)

// reverse
rev = reverse(s)
print(rev)  // "olleh"

// take/drop
print(take(s, 3))  // "hel"
print(drop(s, 2))  // "llo"
```

## Number Formatting

```rust
import "lib/string" (stringPadLeft)

fun formatPrice(cents: Int) -> String {
    dollars = cents / 100
    remainder = cents % 100
    "$" ++ show(dollars) ++ "." ++ stringPadLeft(show(remainder), 2, '0')
}

print(formatPrice(1299))  // "$12.99"
print(formatPrice(500))   // "$5.00"
```

## Data Parsing

```rust
import "lib/string" (stringSplit, stringTrim)
import "lib/list" (map)

// Parse CSV string
csv = "Alice, 30, Engineer"
parts = stringSplit(csv, ",") |> map(fun(s) -> stringTrim(s))
print(parts)  // ["Alice", "30", "Engineer"]

// Parse key=value
fun parseKV(s: String) -> (String, String) {
    parts = stringSplit(s, "=")
    (parts[0], parts[1])
}

(key, value) = parseKV("name=Alice")
print(key)    // name
print(value)  // Alice
```

## Regex (lib/regex)

```rust
import "lib/regex" (*)

text = "Email: alice@example.com, Phone: 123-456-7890"

// Check pattern
print(regexMatch("[a-z]+@[a-z]+\\.[a-z]+", text))  // true

// Find first match
match regexFind("[0-9-]+", text) {
    Some phone -> print(phone)  // "123-456-7890"
    Zero -> print("Not found")
}

// Find all matches
numbers = regexFindAll("[0-9]+", text)  // ["123", "456", "7890"]

// Capture groups
match regexCapture("([a-z]+)@([a-z]+)", text) {
    Some groups -> {
        print(groups[1])  // "alice"
        print(groups[2])  // "example"
    }
    Zero -> print("No match")
}

// Replacement
censored = regexReplaceAll("[0-9]", "X", text)
print(censored)
// "Email: alice@example.com, Phone: XXX-XXX-XXXX"

// Split by pattern
parts = regexSplit("[,;]\\s*", "a, b; c")  // ["a", "b", "c"]
```

## Practical Example: URL Query String Parser

```rust
import "lib/string" (stringSplit)
import "lib/list" (foldl)
import "lib/map" (mapPut, mapGet)

fun parseQuery(query: String) -> Map<String, String> {
    pairs = stringSplit(query, "&")
    foldl(fun(acc, pair) {
        parts = stringSplit(pair, "=")
        key = parts[0]
        value = if len(parts) > 1 { parts[1] } else { "" }
        mapPut(acc, key, value)
    }, %{}, pairs)
}

params = parseQuery("name=Alice&age=30")
match mapGet(params, "name") {
    Some(n) -> print(n)  // "Alice"
    Zero -> print("not found")
}
```

## Practical Example: Slug Generator

```rust
import "lib/string" (stringToLower, stringTrim)
import "lib/regex" (regexReplaceAll)

fun slugify(title: String) -> String {
    s1 = stringTrim(title)
    s2 = stringToLower(s1)
    s3 = regexReplaceAll("[^a-z0-9\\s-]", "", s2)  // remove special chars
    s4 = regexReplaceAll("\\s+", "-", s3)          // spaces -> hyphens
    regexReplaceAll("-+", "-", s4)                  // remove duplicate hyphens
}

print(slugify("Hello, World! This is Funxy"))
// "hello-world-this-is-funxy"
