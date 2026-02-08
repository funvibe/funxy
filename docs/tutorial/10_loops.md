# Iteration 10: Loops

The language supports loops for repeating code execution. The `for` and `while` cycles are synonymous.

## While Loop

The `for` loop with a condition works as a conditional loop.

```rust
i = 0
for i < 5 {
    print(i)
    i = i + 1
}

i = 0
while i < 5 {
    print(i)
    i = i + 1
}
```

## For-In Loop

The `for...in` syntax allows iterating over Lists and types implementing the `Iter` trait.

```rust
list = [1, 2, 3]
for item in list {
    print(item)
}
```

### The Iter Trait

`Iter` is a built-in trait that defines how a type can be iterated:

```rust
// Built-in trait definition (you don't need to define this):
// trait Iter<C, T> {
//     fun iter(collection: C) -> () -> Option<T>
// }
```

The iterator returns `Option<T>`:
- `Some(value)` — next element exists
- `None` — iteration complete

**Important:** The `for` loop **automatically unwraps** the `Option`. You get the inner value directly:

```rust
// Iterator returns: Some(1), Some(2), Some(3), None
// Loop variable gets: 1, 2, 3 (unwrapped values)
for x in [1, 2, 3] {
    print(x)  // prints 1, 2, 3 — not Some(1), Some(2), Some(3)
}
```

`List<T>` implements `Iter` by default. `Range<T>` also implements `Iter`.

```rust
// Using range for numeric iteration
for i in 0..4 {
    print(i)  // 0, 1, 2, 3, 4
}

// Range with step
for i in (0, 2)..10 {
    print(i) // 0, 2, 4, 6, 8
}

// Custom iteration via list generation
fun makeRange(start: Int, end: Int) -> List<Int> {
    if start >= end { [] }
    else { [start] ++ makeRange(start + 1, end) }
}

for i in makeRange(0, 5) {
    print(i)  // 0, 1, 2, 3, 4
}
```

## Loop Control

You can control the flow of loops using `break` and `continue`.

```rust
for i in [1, 2, 3, 4, 5] {
    if i == 3 {
        continue // Skip 3
    }
    if i == 5 {
        break // Stop loop
    }
    print(i)
}
```

## Return Values

Loops in this language are expressions. A `for` loop returns a value:

### Normal Completion

Returns the value of the last iteration:

```rust
res = for x in [1, 2, 3] {
    x * 2
}
print(res)  // 6 (last iteration: 3 * 2)
```

### break with Value

```rust
found = for x in [1, 3, 4, 5] {
    if x % 2 == 0 {
        break x  // returns 4
    }
    x
}
print(found)  // 4
```

### break without Value

Returns `Nil`:

```rust
res = for x in [1, 2, 3] {
    if x == 2 {
        break  // without value
    }
}
print(res)  // Nil
```

### continue

Skips the current iteration, doesn't affect the return value:

```rust
sum = 0
for x in [1, 2, 3, 4, 5] {
    if x == 3 {
        continue  // skip 3
    }
    sum = sum + x
}
print(sum)  // 1 + 2 + 4 + 5 = 12
```

### Type Consistency

If `break` returns a value of a specific type, the loop body must return the same type:

```rust
// Correct: break and body return Option<Int>
res = for x in [1, 2, 3] {
    if x == 2 {
        break Some(x)
    }
    Some(0)  // same type
}
```

## List Comprehensions

For many common iteration patterns, list comprehensions provide a more concise and declarative alternative to loops.

### Basic Syntax

```
[output | clause, clause, ...]
```

### Comparison with Loops

**Using a loop:**
```rust
result = []
for x in [1, 2, 3, 4, 5] {
    if x % 2 == 0 {
        result = result ++ [x * 2]
    }
}
// result = [4, 8]
```

**Using a list comprehension:**
```rust
result = [x * 2 | x <- [1, 2, 3, 4, 5], x % 2 == 0]
// result = [4, 8]
```

### Generators and Filters

- **Generator:** `pattern <- iterable` — iterates over elements
- **Filter:** `boolean_expression` — filters elements

```rust
// Double each element
doubled = [x * 2 | x <- [1, 2, 3]]
// [2, 4, 6]

// Filter and transform
evens = [x | x <- [1, 2, 3, 4, 5, 6], x % 2 == 0]
// [2, 4, 6]

// Multiple filters
filtered = [x | x <- 1..10, x > 3, x < 8]
// [4, 5, 6, 7]
```

### Multiple Generators (Nested Loops)

Multiple generators are equivalent to nested loops:

**Using nested loops:**
```rust
pairs = []
for x in [1, 2] {
    for y in [3, 4] {
        pairs = pairs ++ [(x, y)]
    }
}
// [(1, 3), (1, 4), (2, 3), (2, 4)]
```

**Using a list comprehension:**
```rust
pairs = [(x, y) | x <- [1, 2], y <- [3, 4]]
// [(1, 3), (1, 4), (2, 3), (2, 4)]
```

### Pattern Destructuring

Generators support pattern matching:

```rust
// Tuple destructuring
sums = [a + b | (a, b) <- [(1, 2), (3, 4), (5, 6)]]
// [3, 7, 11]

// Flattening nested lists
matrix = [[1, 2, 3], [4, 5, 6], [7, 8, 9]]
flat = [x | row <- matrix, x <- row]
// [1, 2, 3, 4, 5, 6, 7, 8, 9]
```

### When to Use Each

| Use Case | Recommended |
|----------|-------------|
| Transform/filter a list | List comprehension |
| Side effects (print, IO) | `for` loop |
| Early exit with `break` | `for` loop |
| Accumulating state | `for` loop or `foldl` |
| Cartesian products | List comprehension |
| Simple iteration | Either |

See [Lists](06_lists.md) for more details on list comprehensions.

