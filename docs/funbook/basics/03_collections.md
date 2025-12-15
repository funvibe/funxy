# 03. Collections: Lists and Maps

## Task
Work with data collections: create, transform, search, aggregate.

---

## Lists

### Creation

```rust
import "lib/list" (range)

// Literal
nums = [1, 2, 3, 4, 5]
empty = []

// Through range (start inclusive, end exclusive)
r = range(1, 10)   // [1, 2, 3, ..., 9]
r2 = range(1, 11)  // [1, 2, 3, ..., 10]
print(r)
print(r2)
```

### Strings are Lists of Characters

```rust
import "lib/list" (head, tail, take, drop, reverse, map, filter, length)
import "lib/char" (charToUpper)

s = "hello"

// Indexing
print(s[0])        // 'h'
print(s[4])        // 'o'

// List functions work with strings
print(head(s))     // 'h'
print(tail(s))     // "ello"
print(take(s, 3))  // "hel"
print(drop(s, 2))  // "llo"
print(reverse(s))  // "olleh"
print(length(s))   // 5

// map — transform each character
upper = map(fun(c) -> charToUpper(c), s)
print(upper)       // "HELLO"

// filter — keep characters by condition
vowels = filter(fun(c) -> c == 'a' || c == 'e' || c == 'i' || c == 'o' || c == 'u', s)
print(vowels)      // "eo"
```

### Element Access

```rust
import "lib/list" (head, tail, last, nth, headOr, nthOr)

nums = [1, 2, 3, 4, 5]

// By index
print(nums[0])   // 1
print(nums[2])   // 3

// Safe access
print(head(nums))        // 1 (first element)
print(last(nums))        // 5 (last)
print(nth(nums, 2))      // 3 (by index)

// With default value
print(headOr([], 0))      // 0
print(nthOr(nums, 10, 0)) // 0
```

### Sublists

```rust
import "lib/list" (tail, init, take, drop, slice)

nums = [1, 2, 3, 4, 5]

print(tail(nums))        // [2, 3, 4, 5] (without first)
print(init(nums))        // [1, 2, 3, 4] (without last)
print(take(nums, 3))     // [1, 2, 3]
print(drop(nums, 2))     // [3, 4, 5]
print(slice(nums, 1, 4)) // [2, 3, 4]
```

### Transformations

```rust
import "lib/list" (map, filter, reverse, unique, flatten)

nums = [1, 2, 3, 4, 5]

// map — apply function to all
doubled = map(fun(x) -> x * 2, nums)
print(doubled)  // [2, 4, 6, 8, 10]

// filter — keep by condition
evens = filter(fun(x) -> x % 2 == 0, nums)
print(evens)  // [2, 4]

// reverse
print(reverse(nums))  // [5, 4, 3, 2, 1]

// unique — remove duplicates
print(unique([1, 2, 2, 3, 3, 3]))  // [1, 2, 3]

// flatten — unfold nested lists
print(flatten([[1, 2], [3, 4], [5]]))  // [1, 2, 3, 4, 5]
```

### Pipes — Elegant Chains

```rust
import "lib/list" (map, filter, take)

result = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
    |> filter(fun(x) -> x % 2 == 0)    // [2, 4, 6, 8, 10]
    |> map(fun(x) -> x * x)             // [4, 16, 36, 64, 100]
    |> fun(xs) -> take(xs, 3)           // [4, 16, 36]

print(result)  // [4, 16, 36]
```

For functions with multiple arguments in pipes, use a lambda wrapper.

### Fold (Reduce)

```rust
import "lib/list" (foldl, foldr)

nums = [1, 2, 3, 4, 5]

// Sum
sum = foldl(fun(acc, x) -> acc + x, 0, nums)
print(sum)  // 15

// Product
product = foldl(fun(acc, x) -> acc * x, 1, nums)
print(product)  // 120

// Maximum
max = foldl(fun(acc, x) -> if x > acc { x } else { acc }, nums[0], nums)
print(max)  // 5

// String concatenation
words = ["Hello", " ", "World"]
sentence = foldl(fun(acc, w) -> acc ++ w, "", words)
print(sentence)  // "Hello World"
```

### Search

```rust
import "lib/list" (find, findIndex, indexOf, contains, any, all)

nums = [1, 2, 3, 4, 5]

// contains — check if element exists
print(contains(nums, 3))  // true

// find — find by condition
print(find(fun(x) -> x > 3, nums))       // Some(4)
print(findIndex(fun(x) -> x > 3, nums))  // Some(3)
print(indexOf(nums, 3))                  // Some(2)

// any/all — predicates
print(any(fun(x) -> x > 4, nums))   // true (at least one > 4)
print(all(fun(x) -> x > 0, nums))   // true (all > 0)
```

### Conditional Operations

```rust
import "lib/list" (takeWhile, dropWhile, partition)

nums = [1, 2, 3, 4, 5, 4, 3, 2, 1]

// takeWhile — take while condition is true
print(takeWhile(fun(x) -> x < 4, nums))  // [1, 2, 3]

// dropWhile — skip while condition is true
print(dropWhile(fun(x) -> x < 4, nums))  // [4, 5, 4, 3, 2, 1]

// partition — split into two lists
(small, big) = partition(fun(x) -> x < 3, nums)
print(small)  // [1, 2, 2, 1]
print(big)    // [3, 4, 5, 4, 3]
```

### Combining

```rust
import "lib/list" (zip, unzip, concat)

// concat — join two lists
print(concat([1, 2], [3, 4]))  // [1, 2, 3, 4]

// or via ++
print([1, 2] ++ [3, 4])  // [1, 2, 3, 4]

// zip — join pairwise
names = ["Alice", "Bob"]
ages = [30, 25]
pairs = zip(names, ages)
print(pairs)  // [("Alice", 30), ("Bob", 25)]

// unzip — reverse
(ns, as) = unzip(pairs)
print(ns)  // ["Alice", "Bob"]
print(as)  // [30, 25]
```

### Sorting

```rust
import "lib/list" (sort, sortBy)

nums = [3, 1, 4, 1, 5, 9, 2, 6]

// sort (requires Order trait)
print(sort(nums))  // [1, 1, 2, 3, 4, 5, 6, 9]

// sortBy — with custom comparator
desc = sortBy(nums, fun(a, b) -> b - a)
print(desc)  // [9, 6, 5, 4, 3, 2, 1, 1]

// Sorting records
users = [
    { name: "Alice", age: 30 },
    { name: "Bob", age: 25 },
    { name: "Carol", age: 35 }
]

byAge = sortBy(users, fun(a, b) -> a.age - b.age)
for u in byAge {
    print(u.name)  // Bob, Alice, Carol
}
```

### Iteration

```rust
import "lib/list" (forEach, zip, range)

// for loop
for x in [1, 2, 3] {
    print(x)
}

// for with index (via zip + range)
for pair in zip(range(0, 100), ["a", "b", "c"]) {
    (i, x) = pair
    print("Index " ++ show(i) ++ ": " ++ x)
}

// forEach — for side effects, returns Nil
forEach(fun(x) -> print(x), [10, 20, 30])
```

---

## Map (Associative Array)

Map is an immutable associative array with keys and values of the same type.

### Creation

```rust
import "lib/map" (mapSize)

// Map literal: %{ key => value }
scores = %{ "Alice" => 100, "Bob" => 85, "Carol" => 92 }
print(mapSize(scores))  // 3

// Empty map (requires type annotation)
empty: Map<String, Int> = %{}

// Different key types
intKeys = %{ 1 => "one", 2 => "two", 3 => "three" }
```

All values in a Map must be of the same type.

### Access

```rust
import "lib/map" (mapGet, mapGetOr, mapContains)

scores = %{ "Alice" => 100, "Bob" => 85 }

// mapGet — returns Option
print(mapGet(scores, "Alice"))    // Some(100)
print(mapGet(scores, "Unknown"))  // Zero

// mapGetOr — with default value
print(mapGetOr(scores, "Alice", 0))    // 100
print(mapGetOr(scores, "Unknown", 0))  // 0

// mapContains — check existence
print(mapContains(scores, "Alice"))  // true
print(mapContains(scores, "Dave"))   // false
```

### Modification (Immutable)

All operations return a new Map, original is unchanged:

```rust
import "lib/map" (mapPut, mapRemove, mapMerge, mapGet, mapSize, mapContains)

scores = %{ "Alice" => 100, "Bob" => 85 }

// Add/update (returns new map)
updated = mapPut(scores, "Charlie", 92)
print(mapSize(scores))   // 2 (original unchanged)
print(mapSize(updated))  // 3

// Remove key
smaller = mapRemove(scores, "Bob")
print(mapContains(smaller, "Bob"))  // false

// Merge (second wins on conflict)
m1 = %{ "a" => 1, "b" => 2 }
m2 = %{ "b" => 20, "c" => 3 }
merged = mapMerge(m1, m2)
print(mapGet(merged, "b"))  // Some(20) — from m2
```

### Iteration

```rust
import "lib/map" (mapKeys, mapValues, mapItems)

scores = %{ "Alice" => 100, "Bob" => 85, "Carol" => 92 }

// Keys
print(mapKeys(scores))    // ["Bob", "Carol", "Alice"]

// Values
print(mapValues(scores))  // [85, 92, 100]

// Pairs (key, value)
for pair in mapItems(scores) {
    (k, v) = pair
    print(k ++ ": " ++ show(v))
}
```

### Size

```rust
import "lib/map" (mapSize)

scores = %{ "Alice" => 100, "Bob" => 85 }
print(mapSize(scores))  // 2
```

---

## Practical Examples

### Frequency Dictionary

```rust
import "lib/list" (foldl)
import "lib/map" (mapGetOr, mapPut)

fun frequency(items) {
    foldl(fun(acc, item) -> {
        count = mapGetOr(acc, item, 0)
        mapPut(acc, item, count + 1)
    }, %{}, items)
}

words = ["apple", "banana", "apple", "cherry", "banana", "apple"]
freq = frequency(words)
print(freq)
// %{"apple" => 3, "banana" => 2, "cherry" => 1}
```

### Grouping

```rust
import "lib/list" (foldl, length)
import "lib/map" (mapGetOr, mapPut)

fun groupByLength(words) {
    foldl(fun(acc, word) -> {
        key = length(word)
        existing = mapGetOr(acc, key, [])
        mapPut(acc, key, existing ++ [word])
    }, %{}, words)
}

words = ["hi", "hello", "hey", "world", "ok"]
byLen = groupByLength(words)
print(byLen)
// %{2 => ["hi", "ok"], 5 => ["hello", "world"], 3 => ["hey"]}
```

### Top-N Elements

```rust
import "lib/list" (sortBy, take)

fun topN(items, n, scoreFn) {
    sorted = sortBy(items, fun(a, b) -> scoreFn(b) - scoreFn(a))
    take(sorted, n)
}

products = [
    { name: "A", sales: 100 },
    { name: "B", sales: 500 },
    { name: "C", sales: 250 }
]

top2 = topN(products, 2, fun(p) -> p.sales)
for p in top2 {
    print(p.name ++ ": " ++ show(p.sales))
}
// B: 500
// C: 250
```

---

## lib/list Function Summary

| Category | Functions |
|-----------|---------|
| Access | head, headOr, last, lastOr, nth, nthOr |
| Sublists | tail, init, take, drop, slice, takeWhile, dropWhile |
| Transformations | map, filter, reverse, unique, flatten, sort, sortBy |
| Fold | foldl, foldr |
| Search | find, findIndex, indexOf, contains, any, all |
| Combining | concat, zip, unzip, partition |
| Generation | range |
| Iteration | forEach |
| Size | length |

## lib/map Function Summary

| Function | Description |
|---------|----------|
| mapGet | Get value (Option) |
| mapGetOr | Get or default |
| mapContains | Check key existence |
| mapSize | Number of entries |
| mapPut | Add/update (new Map) |
| mapRemove | Remove key (new Map) |
| mapMerge | Merge two Maps |
| mapKeys | List of keys |
| mapValues | List of values |
| mapItems | List of (key, value) pairs |
