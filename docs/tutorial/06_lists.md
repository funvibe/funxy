# Lists

## List Representation

Lists in Funxy are immutable sequences of elements of the same type. A **hybrid ("Smart List")** implementation is used, combining the advantages of vectors and linked lists:

1. **Vector (Vector-backed)**: Lists created via literals `[...]` or ranges converted to lists. Provide **O(1)** append to end, **O(1)** indexed access, and **O(1)** length.
2. **Linked List (Cons-backed)**: Lists created with the `::` (cons) operator. Provide **O(1)** prepend.

Read operations are automatically optimized: if a vector segment is encountered while traversing a linked list, access switches to **O(1)**. This allows efficient use of both functional patterns (recursion with `head :: tail`) and imperative patterns (indexed access).

```rust
xs = [1, 2, 3, 4, 5]    // Type: List<Int> Vector-backed: O(1) random access
list = 0 :: xs          // Cons-backed: O(1) prepend
names = ["Alice", "Bob"] // Type: List<String>
empty: List<Int> = []    // Empty list with type
```

## Basic Operations

### Indexed Access

```rust
xs = [10, 20, 30, 40]
xs[0]      // 10
xs[2]      // 30
xs[-1]     // 40 (from end)
```

### Concatenation and Cons

```rust
// List concatenation
[1, 2] ++ [3, 4]        // [1, 2, 3, 4]

// Cons — prepend (::)
0 :: [1, 2, 3]          // [0, 1, 2, 3]
```

### Length

```rust
len([1, 2, 3])          // 3
length([1, 2, 3])       // 3 (from lib/list)
```

## Multiline Lists

Multiline syntax with trailing comma is supported:

```rust
numbers = [
    1,
    2,
    3,
    4,
    5,
]
```

## lib/list Module

```rust
import "lib/list" (*)
```

### Element Access

```rust
import "lib/list" (*)

xs = [10, 20, 30, 40, 50]

// First/last
head(xs)                // Some(10)
headOr(xs, 0)           // 10 (or default)
last(xs)                // Some(50)
lastOr(xs, 0)           // 50

// By index
nth(xs, 2)              // Some(30)
nthOr(xs, 10, -1)       // -1 (default)
```

### Slices

```rust
import "lib/list" (*)

xs = [1, 2, 3, 4, 5]

tail(xs)                // Some([2, 3, 4, 5]) — everything except first
init(xs)                // Some([1, 2, 3, 4]) — everything except last

take(xs, 3)             // [1, 2, 3]
drop(xs, 2)             // [3, 4, 5]
slice(xs, 1, 4)         // [2, 3, 4]

takeWhile(fun(x) -> x < 4, xs)  // [1, 2, 3]
dropWhile(fun(x) -> x < 3, xs)  // [3, 4, 5]
```

### Checks

```rust
import "lib/list" (*)

xs = [1, 2, 3, 4, 5]

isEmpty([])             // true
isEmpty(xs)             // false
length(xs)              // 5
contains(xs, 3)         // true
any(fun(x) -> x > 4, xs)   // true
all(fun(x) -> x > 0, xs)   // true
```

### Search

```rust
import "lib/list" (*)

xs = [10, 20, 30, 40, 50]

indexOf(xs, 30)                   // Some(2)
indexOf(xs, 99)                   // None

find(fun(x) -> x > 25, xs)        // Some(30)
findIndex(fun(x) -> x > 25, xs)   // Some(2)
```

### Transformations

```rust
import "lib/list" (*)

xs = [1, 2, 3, 4, 5]

// Map
map(fun(x) -> x * 2, xs)          // [2, 4, 6, 8, 10]

// Filter
filter(fun(x) -> x % 2 == 0, xs)  // [2, 4]

// Reverse
reverse(xs)                       // [5, 4, 3, 2, 1]

// Unique (remove duplicates)
unique([1, 2, 2, 3, 1])           // [1, 2, 3]

// Sort
sort([3, 1, 4, 1, 5])             // [1, 1, 3, 4, 5]
words = ["hello", "a", "world"]
sortBy(words, fun(a, b) -> length(a) - length(b))
```

### Folds

```rust
import "lib/list" (*)

xs = [1, 2, 3, 4, 5]

// Left fold: ((((0 + 1) + 2) + 3) + 4) + 5 = 15
foldl((+), 0, xs)                 // 15

// Right fold: 1 + (2 + (3 + (4 + (5 + 0)))) = 15
foldr((+), 0, xs)                 // 15

// Example: string concatenation
foldl((++), "", ["a", "b", "c"])  // "abc"
```

### Combining

```rust
import "lib/list" (*)

// Zip — combine two lists into pairs
zip([1, 2, 3], ["a", "b", "c"])   // [(1, "a"), (2, "b"), (3, "c")]

// Unzip — separate pairs
pairs: List<(Int, String)> = [(1, "a"), (2, "b")]
unzip(pairs)                      // ([1, 2], ["a", "b"])

// Append (adds to end)
append([1, 2], 3)           // [1, 2, 3]

// Insert (adds at index)
insert([1, 3], 1, 2)        // [1, 2, 3]

// Update (replaces at index)
update([1, 2, 3], 1, 5)     // [1, 5, 3]

// Concat — combine lists
concat([[1, 2], [3], [4, 5]])     // [1, 2, 3, 4, 5]

// Flatten — same as concat
flatten([[1, 2], [3, 4]])         // [1, 2, 3, 4]

// Partition — split by condition
partition(fun(x) -> x % 2 == 0, [1, 2, 3, 4, 5])
// ([2, 4], [1, 3, 5])

// forEach — execute function for each element (side effects)
[1, 2, 3] |> forEach(fun(x) -> print(x))
// prints: 1, 2, 3
// returns: Nil
```

### Generation

```rust
// Range — [start, end)
r = 1..4                            // Range<Int> inclusive
list = [x | x <- 1..4]              // [1, 2, 3, 4]
```

## Pipe Operator

List functions work great with pipe:

```rust
import "lib/list" (*)

result = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
    |> filter(fun(x) -> x % 2 == 0)   // [2, 4, 6, 8, 10]
    |> map(fun(x) -> x * x)           // [4, 16, 36, 64, 100]
    |> fun(xs) -> take(xs, 3)         // [4, 16, 36]
    |> foldl((+), 0)                   // 56
```

## Pattern Matching

### Destructuring

```rust
fun sum(xs: List<Int>) -> Int {
    match xs {
        [] -> 0
        [x, ...rest] -> x + sum(rest)
    }
}

sum([1, 2, 3, 4, 5])  // 15
```

### Spread Patterns

```rust
fun first3(xs: List<Int>) -> List<Int> {
    match xs {
        [a, b, c, ...rest] -> [a, b, c]
        _ -> xs
    }
}

first3([1, 2, 3, 4, 5])  // [1, 2, 3]
first3([1, 2])           // [1, 2]
```

### Specific Patterns

```rust
import "lib/list" (length)

fun describe(xs: List<Int>) -> String {
    match xs {
        [] -> "empty"
        [x] -> "single: ${x}"
        [x, y] -> "pair: ${x}, ${y}"
        [x, ...rest] -> "starts with ${x}, has ${length(rest)} more"
    }
}
```

## Spread in Literals

```rust
xs = [1, 2, 3]
ys = [4, 5]

// Spread at beginning
[0, ...xs]              // [0, 1, 2, 3]

// Spread at end
[...xs, 4]              // [1, 2, 3, 4]

// Multiple spreads
[...xs, ...ys]          // [1, 2, 3, 4, 5]
```

## List Comprehensions

List comprehensions provide a concise, declarative way to create lists. The syntax is inspired by Haskell and mathematical set notation.

### Basic Syntax

```rust
[output | clause, clause, ...]
```

Where clauses can be:
- **Generators:** `pattern <- iterable` — iterate over elements
- **Filters:** `boolean_expression` — filter elements

### Simple Examples

```rust
// Double each element
doubled = [x * 2 | x <- [1, 2, 3]]
// [2, 4, 6]

// Filter even numbers
evens = [x | x <- [1, 2, 3, 4, 5, 6], x % 2 == 0]
// [2, 4, 6]

// Combined: filter and transform
result = [x * 2 | x <- [1, 2, 3, 4, 5], x > 2]
// [6, 8, 10]
```

### Multiple Generators

Multiple generators create a cartesian product:

```rust
pairs = [(x, y) | x <- [1, 2], y <- [3, 4]]
// [(1, 3), (1, 4), (2, 3), (2, 4)]

// Dependent generators (y depends on x)
triangular = [(x, y) | x <- [1, 2, 3], y <- [1, 2, 3], y <= x]
// [(1, 1), (2, 1), (2, 2), (3, 1), (3, 2), (3, 3)]
```

### Pattern Destructuring

Generators support pattern matching for tuples and records:

```rust
// Tuple pattern
sums = [a + b | (a, b) <- [(1, 2), (3, 4), (5, 6)]]
// [3, 7, 11]

// Nested lists (flattening)
matrix = [[1, 2, 3], [4, 5, 6], [7, 8, 9]]
flattened = [x | row <- matrix, x <- row]
// [1, 2, 3, 4, 5, 6, 7, 8, 9]
```

### Multiple Filters

```rust
filtered = [x | x <- [1, 2, 3, 4, 5, 6, 7, 8, 9, 10], x > 3, x < 8]
// [4, 5, 6, 7]
```

### Using Outer Variables

```rust
multiplier = 3
scaled = [x * multiplier | x <- [1, 2, 3]]
// [3, 6, 9]
```

### Strings (List<Char>)

Since strings are `List<Char>`, comprehensions work with them:

```rust
chars = [c | c <- "abc"]
// "abc" (as List<Char>)
```

## Practical Examples

### Sum and Average

```rust
import "lib/list" (length, foldl)

sum = fun(xs: List<Int>) -> Int {
    foldl((+), 0, xs)
}

average = fun(xs: List<Float>) -> Float {
    foldl((+), 0.0, xs) / intToFloat(length(xs))
}

sum([1, 2, 3, 4, 5])      // 15
average([1.0, 2.0, 3.0])  // 2.0
```

### Maximum and Minimum

```rust
import "lib/list" (foldl, head, tail)

maximum = fun(xs: List<Int>) -> Int {
    foldl(fun(a, b) -> if a > b { a } else { b }, head(xs), tail(xs))
}

maximum([3, 1, 4, 1, 5, 9])  // 9
```

### Grouping

```rust
import "lib/list" (filter, unique, map)

fun groupBy<k, v>(keyFn: (v) -> k, xs: List<v>) -> List<(k, List<v>)> {
    keys = unique(map(keyFn, xs))
    map(fun(k) -> (k, filter(fun(x) -> keyFn(x) == k, xs)), keys)
}

// Group by parity
nums = [1, 2, 3, 4, 5, 6]
groupBy(fun(x) -> x % 2, nums)
// [(1, [1, 3, 5]), (0, [2, 4, 6])]
```

### Data Processing

```rust
import "lib/list" (*)
import "lib/string" (*)

// Count words in text
fun countWords(text: String) -> Int {
    length(stringWords(text))
}

// Find longest word
fun longestWord(text: String) -> Option<String> {
    words = stringWords(text)
    if isEmpty(words) {
        None
    } else {
        sorted = sortBy(words, fun(a, b) -> length(b) - length(a))
        Some(head(sorted))
    }
}

// Character frequency
fun charFreq(s: String) -> List<(Char, Int)> {
    chars = unique(s)
    map(fun(c) -> (c, length(filter(fun(x) -> x == c, s))), chars)
}
```

## lib/list Summary

| Function | Description |
|---------|----------|
| `head`, `headOr` | First element |
| `last`, `lastOr` | Last element |
| `nth`, `nthOr` | Element by index |
| `tail`, `init` | Without first/last |
| `take`, `drop` | Take/skip N elements |
| `slice` | Slice by indices |
| `takeWhile`, `dropWhile` | By condition |
| `isEmpty`, `length` | Check and length |
| `contains`, `indexOf` | Search element |
| `find`, `findIndex` | Search by condition |
| `any`, `all` | Checks by condition |
| `map`, `filter` | Transformations |
| `foldl`, `foldr` | Folds |
| `reverse`, `sort`, `sortBy` | Ordering |
| `unique` | Remove duplicates |
| `append` | Append element |
| `insert` | Insert element |
| `update` | Update element |
| `concat`, `flatten` | Combine |
| `zip`, `unzip` | Combine |
| `partition` | Split |
| `forEach` | Side effects |
| `range` | Generate |
