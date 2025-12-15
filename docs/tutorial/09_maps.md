# Maps

## Representation

Map in Funxy is an immutable associative array (hash table). Internally implemented as HAMT (Hash Array Mapped Trie) for efficient operations on immutable data.

```rust
import "lib/map" (*)

scores = %{ "Alice" => 100, "Bob" => 85, "Charlie" => 92 }
// Type: Map<String, Int>
```

## Syntax

### Creating Map

```rust
// Map literal
m = %{ "key1" => 1, "key2" => 2 }

// Empty map (requires type annotation)
empty: Map<String, Int> = %{}

// Multiline with trailing comma
config = %{
    "host" => "localhost",
    "port" => "8080",
    "debug" => "1",
}
```

### Different Key Types

```rust
// String keys
stringMap = %{ "a" => 1, "b" => 2 }

// Int keys  
intMap = %{ 1 => "one", 2 => "two", 3 => "three" }

// Any type with Eq
tupleMap = %{ (0, 0) => "origin", (1, 0) => "x-axis" }
```

## lib/map Module

```rust
import "lib/map" (*)
```

### Accessing Values

```rust
import "lib/map" (*)

scores = %{ "Alice" => 100, "Bob" => 85 }

// Index access — returns Option<V>
scores["Alice"]              // Some(100)
scores["Unknown"]            // Zero

// mapGet — same thing
mapGet(scores, "Alice")      // Some(100)
mapGet(scores, "Unknown")    // Zero

// mapGetOr — with default value
mapGetOr(scores, "Alice", 0)   // 100
mapGetOr(scores, "Unknown", 0) // 0

// mapContains — check presence
mapContains(scores, "Alice")   // true
mapContains(scores, "Dave")    // false
```

### Size

```rust
import "lib/map" (mapSize)

scores = %{ "Alice" => 100, "Bob" => 85 }
mapSize(scores)              // 2
len(scores)                  // 2 (built-in len also works)
```

### Modification (Immutable)

All modification operations return a **new** Map, original is unchanged:

```rust
import "lib/map" (*)

scores = %{ "Alice" => 100, "Bob" => 85 }

// Add or update
scores2 = mapPut(scores, "Charlie", 92)
mapSize(scores)              // 2 (original unchanged)
mapSize(scores2)             // 3

// Update existing
scores3 = mapPut(scores, "Alice", 110)
mapGet(scores, "Alice")      // Some(100)  — original
mapGet(scores3, "Alice")     // Some(110)  — new

// Remove key
scores4 = mapRemove(scores, "Bob")
mapSize(scores4)             // 1
mapContains(scores4, "Bob")  // false
```

### Merging

```rust
import "lib/map" (*)

m1 = %{ "a" => 1, "b" => 2 }
m2 = %{ "b" => 20, "c" => 3 }

// Merge — second map "wins" on conflict
merged = mapMerge(m1, m2)
mapGet(merged, "a")          // Some(1)   — from m1
mapGet(merged, "b")          // Some(20)  — from m2 (overwrote)
mapGet(merged, "c")          // Some(3)   — from m2
```

### Iteration

```rust
import "lib/map" (*)

scores = %{ "Alice" => 100, "Bob" => 85, "Charlie" => 92 }

// Get all keys
keys = mapKeys(scores)       // ["Alice", "Bob", "Charlie"]

// Get all values
vals = mapValues(scores)     // [100, 85, 92]

// Get pairs (key, value)
items = mapItems(scores)     // [("Alice", 100), ("Bob", 85), ...]
```

## Pattern Matching with Option

Since mapGet returns Option<V>, use pattern matching:

```rust
import "lib/map" (*)

fun getScore(scores: Map<String, Int>, name: String) -> String {
    match mapGet(scores, name) {
        Some(s) -> "Score: " ++ show(s)
        Zero -> "Not found"
    }
}

// Usage example
scores = %{ "Alice" => 100, "Bob" => 85 }
print(getScore(scores, "Alice"))     // Score: 100
print(getScore(scores, "Unknown"))   // Not found

// Or with mapGetOr
score = mapGetOr(scores, "Alice", 0)
```

## Practical Examples

### Frequency Count

```rust
import "lib/map" (*)
import "lib/list" (foldl)

fun countFreq(xs: List<Char>) -> Map<Char, Int> {
    foldl(fun(m, x) -> {
        count = mapGetOr(m, x, 0)
        mapPut(m, x, count + 1)
    }, %{}, xs)
}

freq = countFreq(['a', 'b', 'a', 'c', 'a', 'b'])
mapGet(freq, 'a')            // Some(3)
mapGet(freq, 'b')            // Some(2)
mapGet(freq, 'c')            // Some(1)
```

### Grouping by Key

```rust
import "lib/map" (*)
import "lib/list" (foldl, length)

fun groupByLen(xs: List<String>) -> Map<Int, List<String>> {
    foldl(fun(m, x) -> {
        k = length(x)
        existing = mapGetOr(m, k, [])
        mapPut(m, k, existing ++ [x])
    }, %{}, xs)
}

words = ["hi", "hello", "hey", "world", "ok"]
byLen = groupByLen(words)
mapGet(byLen, 2)             // Some(["hi", "ok"])
mapGet(byLen, 5)             // Some(["hello", "world"])
```

### Configuration

```rust
import "lib/map" (*)

// Loading configuration
defaultConfig = %{
    "host" => "localhost",
    "port" => "8080",
    "timeout" => "30",
}

userConfig = %{
    "port" => "3000",
    "debug" => "true",
}

// Merge: user overrides default
config = mapMerge(defaultConfig, userConfig)
mapGet(config, "host")       // Some("localhost") — default
mapGet(config, "port")       // Some("3000")      — overridden
mapGet(config, "debug")      // Some("true")      — user only
```

### Map Inversion

```rust
import "lib/map" (*)
import "lib/list" (foldl)
import "lib/tuple" (fst, snd)

// Swap keys and values
fun invert(m: Map<String, Int>) -> Map<Int, String> {
    foldl(fun(acc, kv) -> {
        mapPut(acc, snd(kv), fst(kv))
    }, %{}, mapItems(m))
}

m = %{ "a" => 1, "b" => 2, "c" => 3 }
inv = invert(m)
mapGet(inv, 1)               // Some("a")
mapGet(inv, 2)               // Some("b")
```

## When to Use Map

**Use Map when:**
- Need fast key lookup
- Data is often read but rarely modified
- Need immutability (thread safety)
- Keys are heterogeneous or dynamic

**Use Record when:**
- Fixed set of fields known in advance
- Need typing for each field
- Structure is defined at compile time

```rust
import "lib/map" (mapGet)

// Record — static structure
type User = { name: String, age: Int, email: String }
user: User = { name: "Alice", age: 30, email: "a@b.com" }
user.name                    // Typed access

// Map — dynamic structure
userData = %{ "name" => "Alice", "age" => "30" }
mapGet(userData, "name")     // Option<String>
```

## lib/map Summary

| Function | Type | Description |
|---------|-----|----------|
| mapGet | (Map K V, K) -> Option V | Get value |
| mapGetOr | (Map K V, K, V) -> V | Get or default |
| mapContains | (Map K V, K) -> Bool | Check presence |
| mapSize | (Map K V) -> Int | Number of entries |
| mapPut | (Map K V, K, V) -> Map K V | Add/update |
| mapRemove | (Map K V, K) -> Map K V | Remove key |
| mapMerge | (Map K V, Map K V) -> Map K V | Merge |
| mapKeys | (Map K V) -> List K | All keys |
| mapValues | (Map K V) -> List V | All values |
| mapItems | (Map K V) -> List (K, V) | All pairs |

Built-in len(m) also works for size and m[key] for access.
