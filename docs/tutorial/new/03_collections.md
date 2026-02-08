# 3. Collections

[← Back to Index](./00_index.md)

## Lists

### Creation

```rust
numbers = [1, 2, 3, 4, 5]
empty: List<Int> = []
mixed = [1, 2, 3,]  // trailing comma OK
```

### Access

```rust
numbers[0]   // 1 (first)
numbers[-1]  // 5 (last)
numbers[2]   // 3
```

### Operations

```rust
[1, 2] ++ [3, 4]   // [1, 2, 3, 4]
0 :: [1, 2, 3]     // [0, 1, 2, 3]
len([1, 2, 3])     // 3
```

### List Comprehensions

Declarative list construction.

```rust
// Basic
[x * 2 | x <- [1, 2, 3]]  // [2, 4, 6]

// With filter
[x | x <- 1..10, x % 2 == 0]  // [2, 4, 6, 8, 10]

// Multiple generators
[(x, y) | x <- [1, 2], y <- ['a', 'b']]
// [(1, 'a'), (1, 'b'), (2, 'a'), (2, 'b')]

// Destructuring
[a + b | (a, b) <- [(1, 2), (3, 4)]]  // [3, 7]
```

### Spread

```rust
xs = [1, 2, 3]
[0, ...xs, 4]      // [0, 1, 2, 3, 4]
[...xs, ...xs]     // [1, 2, 3, 1, 2, 3]
```

### Modification

Lists are **immutable**. You cannot assign to an index directly (e.g., `list[0] = 1`). Instead, use `insert` or `update` to create a new list.

```rust
import "lib/list" (insert, update)

list = [1, 2, 3]

// Insert at index
insert(list, 1, 99)      // [1, 99, 2, 3]

// Update at index
update(list, 0, 10)      // [10, 2, 3]
```

### Common Functions (lib/list)

```rust
import "lib/list" (*)

head([1, 2, 3])              // 1 (panic if empty)
tail([1, 2, 3])              // [2, 3]
take([1, 2, 3, 4], 2)        // [1, 2]
drop([1, 2, 3, 4], 2)        // [3, 4]

map(fun(x) -> x * 2, [1, 2, 3])           // [2, 4, 6]
filter(fun(x) -> x > 2, [1, 2, 3, 4])     // [3, 4]
foldl((+), 0, [1, 2, 3])                  // 6

find(fun(x) -> x > 2, [1, 2, 3, 4])       // Some(3)
any(fun(x) -> x > 3, [1, 2, 3, 4])        // true
all(fun(x) -> x > 0, [1, 2, 3])           // true

sort([3, 1, 4, 1, 5])                     // [1, 1, 3, 4, 5]
reverse([1, 2, 3])                        // [3, 2, 1]
unique([1, 2, 2, 3, 3, 3])                // [1, 2, 3]

zip([1, 2], ['a', 'b'])                   // [(1, 'a'), (2, 'b')]
partition(fun(x) -> x % 2 == 0, [1, 2, 3, 4])  // ([2, 4], [1, 3])
```

## Tuples

Fixed-size, heterogeneous collections.

```rust
pair = (1, "hello")
triple = (true, 42, "test")

// Access
pair[0]   // 1
pair[1]   // "hello"

// Destructuring
(a, b) = pair
print(a)  // 1

// Nested
((x, y), z) = ((1, 2), 3)
```

### Common Functions (lib/tuple)

```rust
import "lib/tuple" (*)

fst((1, "a"))           // 1
snd((1, "a"))           // "a"
tupleSwap((1, "a"))     // ("a", 1)
mapFst(fun(x) -> x * 2, (3, "a"))  // (6, "a")
```

## Records

Named fields.

### Creation and Access

```rust
user = { name: "Alice", age: 30 }
print(user.name)  // Alice
print(user.age)   // 30

// With type
type alias User = { name: String, age: Int }
alice: User = { name: "Alice", age: 30 }
```

### Mutation

```rust
user = { name: "Alice", age: 30 }
user.age = 31  // OK
print(user.age)  // 31
```

### Spread (Update)

Creates a new record with updated fields.

```rust
base = { x: 1, y: 2, z: 3 }
updated = { ...base, x: 10 }  // { x: 10, y: 2, z: 3 }
```

### Destructuring

```rust
user = { name: "Alice", age: 30 }
{ name: n, age: a } = user
print(n)  // Alice

// Partial
{ name: userName } = user

// Nested
data = { user: { name: "Bob", role: "admin" }, count: 5 }
{ user: { name: nestedName }, count: c } = data
```

### Row Polymorphism

Functions can accept records with *at least* specific fields.

```rust
fun getName(r: { name: String }) -> String {
    r.name
}

user = { name: "Alice", age: 30, role: "admin" }
print(getName(user))  // Alice - extra fields ignored
```

## Maps

Key-value pairs.

```rust
import "lib/map" (*)

// Creation
scores = %{ "Alice" => 100, "Bob" => 85 }
empty: Map<String, Int> = %{}

// Access
mapGet(scores, "Alice")           // Some(100)
mapGetOr(scores, "Unknown", 0)    // 0
mapContains(scores, "Alice")      // true

// Modification (returns new Map)
scores2 = mapPut(scores, "Charlie", 92)
scores3 = mapRemove(scores, "Bob")

// Iteration
mapKeys(scores)    // ["Alice", "Bob"]
mapValues(scores)  // [100, 85]
mapItems(scores)   // [("Alice", 100), ("Bob", 85)]
```
