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

### Modification (Immutable)

Lists are **immutable**. When you assign to an index directly, it evaluates to a **new** list with the updated element. The original list remains unchanged.

```rust
list = [1, 2, 3]

// Index assignment creates a new list
list2 = list[0] = 10
print(list)   // [1, 2, 3]
print(list2)  // [10, 2, 3]

// To "mutate" a variable, reassign it
list = list[0] = 99
```

> **Warning:** Discarding the result of an immutable update expression (e.g. `list[0] = 10` as a standalone statement without assignment) will result in a compilation error: `type error: pure expression result discarded`. However, it is perfectly legal to use it as a return value:

```rust
fun updateFirst(lst: List<Int>, val: Int) -> List<Int> {
    // Valid: implicitly returns the new list
    lst[0] = val
}
```

You can also use functions from `lib/list`:

```rust
import "lib/list" (insert, update)

list = [1, 2, 3]

// Insert at index
insert(list, 1, 99)      // [1, 99, 2, 3]

// Update at index (alternative to list[0] = 10)
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

### Modification (Immutable)

Records are immutable. When you assign to a field, it evaluates to a **new** record with the updated field. The original record remains unchanged.

```rust
user = { name: "Alice", age: 30 }

// Field assignment creates a new record
user2 = user.age = 31

print(user.age)   // 30 (original is unchanged)
print(user2.age)  // 31

// To "mutate" a variable, reassign it
user = user.age = 31
```

> **Warning:** Discarding the result of an immutable update expression (e.g. `user.age = 31` as a standalone statement without assignment) will result in a compilation error: `type error: pure expression result discarded`. However, it is perfectly legal to use it as a return value:

```rust
fun updateAge(u: User, newAge: Int) -> User {
    // Valid: implicitly returns the new record
    u.age = newAge
}
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
scores["Alice"]                   // Some(100)
mapGet(scores, "Alice")           // Some(100)
mapGetOr(scores, "Unknown", 0)    // 0
mapContains(scores, "Alice")      // true

// Modification (Immutable - returns new Map)
scores2 = scores["Charlie"] = 92
scores3 = mapPut(scores, "Charlie", 92)
scores4 = mapRemove(scores, "Bob")

// To "mutate" a variable, reassign it
scores = scores["Charlie"] = 92
```

> **Warning:** Discarding the result of an immutable update expression (e.g. `scores["Alice"] = 100` as a standalone statement without assignment) will result in a compilation error: `type error: pure expression result discarded`. However, it is perfectly legal to use it as a return value:

```rust
fun addScore(scores: Map<String, Int>, name: String, val: Int) -> Map<String, Int> {
    // Valid: implicitly returns the new map
    scores[name] = val
}
```

```rust
// Iteration
mapKeys(scores)    // ["Alice", "Bob"]
mapValues(scores)  // [100, 85]
mapItems(scores)   // [("Alice", 100), ("Bob", 85)]
```

### Map Comprehensions

Similar to list comprehensions, you can concisely generate maps without intermediate allocations:

```rust
// Basic map comprehension
m1 = %{ "key_" ++ show(i) => i * 2 | i <- 1..5 }

// With filter
m2 = %{ "key_" ++ show(i) => i | i <- 1..10, i % 2 == 0 }

// From map items
m3 = %{ k => v * 10 | (k, v) <- mapItems(scores) }
```
