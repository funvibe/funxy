# Iteration 10: Loops

The language supports loops for repeating code execution.

## While Loop (Standard For)

The `for` loop with a condition works as a conditional loop.

```rust
i = 0
for i < 5 {
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
- `Zero` — iteration complete

**Important:** The `for` loop **automatically unwraps** the `Option`. You get the inner value directly:

```rust
// Iterator returns: Some(1), Some(2), Some(3), Zero
// Loop variable gets: 1, 2, 3 (unwrapped values)
for x in [1, 2, 3] {
    print(x)  // prints 1, 2, 3 — not Some(1), Some(2), Some(3)
}
```

`List<T>` implements `Iter` by default. For custom iteration, use `range()` or build a list:

```rust
import "lib/list" (range)

// Using range for numeric iteration
for i in range(0, 5) {
    print(i)  // 0, 1, 2, 3, 4
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

