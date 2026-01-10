# Built-in Functions

The language provides a set of built-in functions for common operations.

## Output

### `print(args...)`

Prints values to standard output **with newline**. Accepts any number of arguments.

```rust
print("Hello")              // Hello
print(1, 2, 3)              // 1 2 3
print("Sum: " ++ show(1 + 2)) // Sum: 3
```

Returns `Nil`.

### `write(args...)`

Prints values to standard output **without newline**. Accepts any number of arguments.

```rust
import "lib/list" (range)
import "lib/time" (sleepMs)

write("Hello")
write(" ")
write("World")
print("!")                  // Hello World!

// Progress indicator
for i in range(1, 6) {
    write("\rProgress: " ++ show(i) ++ "/5")
    sleepMs(100)
}
print("")  // Final newline
```

Returns `Nil`.

## Type Conversion

### `floatToInt(f: Float)` → `Int`

Converts a Float to Int by truncating the decimal part:

```rust
print(floatToInt(3.7))      // 3
print(floatToInt(-3.7))     // -3
print(floatToInt(3.99))     // 3
```

### `intToFloat(n: Int)` → `Float`

Converts an Int to Float:

```rust
print(intToFloat(42))       // 42.0
print(intToFloat(-10))      // -10.0
```

### `show<T>(value: T)` → `String`

Converts a value to a string:

```rust
print(show(42))         // 42
print(show(true))       // true
print(show([1, 2]))     // [1, 2]
```

### `sprintf(format: String, ...args: Any)` → `String`

Formats a value according to a format string (like printf).

```rust
// Explicit call
s = sprintf("Values: %d, %.2f", 42, 3.14159)  // "Values: 42, 3.14"

// Literal syntax (creates a formatter function for single argument)
fmt = %".2f"
s2 = 3.14159 |> fmt           // "3.14"

// Inside interpolation
print("Pi is ${3.14159 |> %".2f"}")
```

Supported formats:
- `%f`: Floating point (e.g. `%.2f`)
- `%d`: Integer (e.g. `%05d` for zero padding)
- `%x`: Hexadecimal
- `%s`: String

### `charFromCode(Int)` → `Char`

Converts an integer to a character (by ASCII/Unicode code):

```rust
import "lib/char" (charFromCode)

print(charFromCode(65))     // A
print(charFromCode(97))     // a
```

### `charToCode(char)` → `Int`

Returns the ASCII/Unicode code of a character:

```rust
import "lib/char" (charToCode)

print(charToCode('A'))      // 65
print(charToCode('a'))      // 97
```

## Parsing

### `read<T>(string: String, type: Type<T>)` → `Option<T>`

Parses a string into the specified type. Returns `Some(value)` on success, `Zero` on failure.

```rust
// Parsing integers
x = read("42", Int)
print(match x { Some v -> v; Zero -> -1 })  // 42

bad = read("abc", Int)
print(match bad { Some v -> v; Zero -> -1 })  // -1

// Parsing floats
y = read("3.14", Float)
print(match y { Some v -> v; Zero -> 0.0 })  // 3.14

// Parsing booleans (only "true"/"false")
z = read("true", Bool)
print(match z { Some v -> v; Zero -> false })  // true
```

**Note**: `read` always requires an explicit type argument. Type inference from context is not supported.

## Introspection

### `getType<T>(value: T)` → `String`

Returns the type of a value as a string:

```rust
print(getType(42))          // Int
print(getType(3.14))        // Float
print(getType("hi"))        // String
print(getType([1, 2, 3]))   // List<Int>
print(getType(Some(1)))     // Option<Int>
```

### `typeOf<T>(value: T, type: Type)` → `Bool`

Checks if a value matches a given type:

```rust
print(typeOf(42, Int))          // true
print(typeOf(42, Float))        // false
print(typeOf("hi", String))     // true
print(typeOf(Some(1), Option))  // true
```

### `show<T>(value: T)` → `String`

Converts any value to its string representation:

```rust
print(show(42))             // 42
print(show([1, 2, 3]))      // [1, 2, 3]
print(show(Some("hi")))     // Some(hi)
```

## Default Values

### `default<T>(type: Type<T>)` → `T`

Returns the default value for a type:

```rust
print(default(Int))         // 0
print(default(Float))       // 0.0
print(default(Bool))        // false
print(default(String))      // (empty string)
print(default(Char))        // '\0'
```

Works with any type that implements the `Default` trait.

## Functional Helpers

### `id<T>(x: T) -> T`

Identity function - returns its argument unchanged:

```rust
print(id(42))               // 42
print(id("hello"))          // hello

// Useful as default transformer
needDouble = true
transform = if needDouble { fun(x) -> x * 2 } else { id }
print(transform(5))         // 10
```

### `const<A, B>(x: A, y: B) -> A`

Returns first argument, ignores second:

```rust
print(const(1, 2))          // 1
print(const("a", "b"))      // a
```

### `len<T>(collection: T) -> Int`

Returns length of list, tuple, or string (character count):

```rust
print(len([1, 2, 3]))       // 3
print(len("Hello"))         // 5
print(len("Привет"))        // 6 (characters, not bytes)
```

### `lenBytes(s: String) -> Int`

Returns byte length of UTF-8 string:

```rust
print(lenBytes("Hello"))    // 5
print(lenBytes("Привет"))   // 12 (UTF-8 bytes)
```

## Function Composition

### `,,` Operator

Right-to-left function composition:

```rust
double = fun(x) -> x * 2
inc = fun(x) -> x + 1

// (double ,, inc)(x) = double(inc(x))
doubleInc = double ,, inc
print(doubleInc(5))         // 12 (5+1=6, 6*2=12)

// Chain multiple functions
square = fun(x) -> x * x
f = square ,, double ,, inc
print(f(3))                 // 64 (3+1=4, 4*2=8, 8*8=64)

// Identity is neutral element
print((inc ,, id)(5))       // 6
print((id ,, inc)(5))       // 6
```

---

# `lib/list` — Standard List Library

Import with:
```rust
import "lib/list" (*)              // all functions
import "lib/list" (map, filter)    // specific functions
import "lib/list" !(sort)          // all except sort
```

## Access Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `head` | `List<T> -> T` | First element (panics if empty) |
| `headOr` | `(List<T>, T) -> T` | First element or default |
| `last` | `List<T> -> T` | Last element (panics if empty) |
| `lastOr` | `(List<T>, T) -> T` | Last element or default |
| `nth` | `(List<T>, Int) -> T` | Element at index (panics if out of bounds) |
| `nthOr` | `(List<T>, Int, T) -> T` | Element at index or default |

## Sublist Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `tail` | `List<T> -> List<T>` | All except first (panics if empty) |
| `init` | `List<T> -> List<T>` | All except last (panics if empty) |
| `take` | `(List<T>, Int) -> List<T>` | First n elements |
| `drop` | `(List<T>, Int) -> List<T>` | All except first n |
| `slice` | `(List<T>, Int, Int) -> List<T>` | Elements from..to |

## Higher-Order Functions

**Note:** Higher-order functions use **function-first** argument order for better composition and partial application.

### Partial Application

All `lib/list` functions support **partial application** — calling with fewer arguments returns a new function:

```rust
import "lib/list" (map, filter, foldl)

// Create reusable transformations
double = fun(x) -> x * 2
doubled = map(double)           // (List<Int>) -> List<Int>
print(doubled([1, 2, 3]))       // [2, 4, 6]

// Create reusable filters
isEven = fun(x) -> x % 2 == 0
evens = filter(isEven)          // (List<Int>) -> List<Int>
print(evens([1, 2, 3, 4, 5]))   // [2, 4]

// Create reusable reducers
sum = foldl((+), 0)             // (List<Int>) -> Int
print(sum([1, 2, 3, 4, 5]))     // 15

// Use with pipe operator
result = [1, 2, 3, 4, 5] |> filter(fun(x) -> x > 2) |> map(fun(x) -> x * 10)
print(result)                   // [30, 40, 50]
```

### `filter(pred, list) -> List<T>`

Keeps elements where predicate returns true:

```rust
import "lib/list" (filter)
evens = filter(fun(x) -> x % 2 == 0, [1,2,3,4,5])
print(evens)  // [2, 4]
```

### `map(fn, list) -> List<U>`

Applies function to each element:

```rust
import "lib/list" (map)
doubled = map(fun(x) -> x * 2, [1,2,3])
print(doubled)  // [2, 4, 6]
```

### `foldl(fn, init, list) -> U`

Left fold - processes left to right:

```rust
import "lib/list" (foldl)
// foldl((+), 0, [1,2,3]) = ((0+1)+2)+3 = 6
sum = foldl((+), 0, [1,2,3,4])
print(sum)  // 10

// foldl((-), 0, [1,2,3]) = ((0-1)-2)-3 = -6
print(foldl(fun(acc, x) -> acc - x, 0, [1,2,3]))  // -6
```

### `foldr(fn, init, list) -> U`

Right fold - processes right to left:

```rust
import "lib/list" (foldr)
// foldr((-), 0, [1,2,3]) = 1-(2-(3-0)) = 2
print(foldr(fun(x, acc) -> x - acc, 0, [1,2,3]))  // 2

// Build list in original order
copy = foldr(fun(x, acc) -> x :: acc, [], [1,2,3])
print(copy)  // [1, 2, 3]
```

## Search Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `indexOf` | `(List<T>, T) -> Option<Int>` | Find index by value |
| `find` | `((T)->Bool, List<T>) -> Option<T>` | Find first matching element |
| `findIndex` | `((T)->Bool, List<T>) -> Option<Int>` | Find index of first match |

## Predicate Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `any` | `((T)->Bool, List<T>) -> Bool` | True if any element matches |
| `all` | `((T)->Bool, List<T>) -> Bool` | True if all elements match |

## Conditional Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `takeWhile` | `((T)->Bool, List<T>) -> List<T>` | Take while predicate is true |
| `dropWhile` | `((T)->Bool, List<T>) -> List<T>` | Drop while predicate is true |

## Generation

| Function | Signature | Description |
|----------|-----------|-------------|
| `range` | `(Int, Int) -> List<Int>` | Generate `[start..end-1]` |

```rust
import "lib/list" (range)

print(range(1, 5))   // [1, 2, 3, 4]
print(range(0, 3))   // [0, 1, 2]
print(range(5, 5))   // [] (empty when start >= end)
```

## Other Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `isEmpty` | `List<T> -> Bool` | Check if empty |
| `length` | `List<T> -> Int` | Number of elements |
| `contains` | `(List<T>, T) -> Bool` | Check membership |
| `reverse` | `List<T> -> List<T>` | Reverse order |
| `concat` | `(List<T>, List<T>) -> List<T>` | Join lists |
| `flatten` | `List<List<T>> -> List<T>` | Flatten nested |
| `unique` | `List<T> -> List<T>` | Remove duplicates |
| `partition` | `(List<T>, (T)->Bool) -> (List<T>, List<T>)` | Split by predicate |
| `zip` | `(List<A>, List<B>) -> List<(A,B)>` | Pair elements |
| `unzip` | `List<(A,B)> -> (List<A>, List<B>)` | Split pairs |
| `sort` | `List<T> -> List<T>` | Sort (requires `Order`) |
| `sortBy` | `(List<T>, (T,T)->Int) -> List<T>` | Sort with comparator |

## Summary Table (Core Builtins)

| Function | Signature | Description |
|----------|-----------|-------------|
| `print` | `(args...) -> Nil` | Output to stdout with newline |
| `write` | `(args...) -> Nil` | Output to stdout without newline |
| `id` | `(T) -> T` | Identity function |
| `const` | `(A, B) -> A` | Return first argument |
| `len` | `(collection) -> Int` | Length (chars for strings) |
| `lenBytes` | `(String) -> Int` | Byte length |
| `read` | `(String, Type<T>) -> Option<T>` | Parse string |
| `getType` | `(value) -> Type<T>` | Runtime type |
| `typeOf` | `(value, Type) -> Bool` | Check type match |
| `show` | `(value) -> String` | String representation |
| `sprintf` | `(fmt, value) -> String` | Format string |
| `default` | `(Type<T>) -> T` | Default value |
| `panic` | `(String) -> a` | Abort with message |

---

# `lib/time` — Time and Benchmarking

Import with:
```rust
import "lib/time" (*)
import "lib/time" (clockMs, timeNow)
```

## Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `timeNow` | `() -> Int` | Unix timestamp in seconds |
| `clockNs` | `() -> Int` | Monotonic nanoseconds |
| `clockMs` | `() -> Int` | Monotonic milliseconds |
| `sleep` | `(Int) -> Nil` | Pause for seconds |
| `sleepMs` | `(Int) -> Nil` | Pause for milliseconds |

## Usage Examples

### Unix Timestamp
```rust
import "lib/time" (timeNow)

now = timeNow()
print("Unix: " ++ show(now))  // Unix: 1732945200
```

### Benchmarking
```rust
import "lib/time" (clockMs)

t1 = clockMs()
// ... expensive operation ...
t2 = clockMs()
print("Elapsed: " ++ show(t2 - t1) ++ " ms")
```

### High-Resolution Timing
```rust
import "lib/time" (clockNs)

start = clockNs()
// ... operation ...
end = clockNs()
print("Elapsed: " ++ show((end - start) / 1000) ++ " μs")
```

### Delays
```rust
import "lib/time" (sleep, sleepMs)

print("Wait 2 seconds...")
sleep(2)

print("Wait 500 ms...")
sleepMs(500)
```

## Notes

- `clockNs`/`clockMs` are **monotonic** — always increase, never jump back
- Use monotonic clocks for benchmarking (not affected by system time changes)
- `timeNow()` returns wall clock — can jump if system time is adjusted

---

# `lib/io` — Input/Output

Import with:
```rust
import "lib/io" (*)
import "lib/io" (fileRead, fileWrite)
```

## Console

| Function | Signature | Description |
|----------|-----------|-------------|
| `readLine` | `() -> Option<String>` | Read line from stdin, `Zero` at EOF |

## File Reading

| Function | Signature | Description |
|----------|-----------|-------------|
| `fileRead` | `(String) -> Result<String, String>` | Read entire file |
| `fileReadAt` | `(String, Int, Int) -> Result<String, String>` | Read `length` bytes from `offset` |

## File Writing

| Function | Signature | Description |
|----------|-----------|-------------|
| `fileWrite` | `(String, String) -> Result<Int, String>` | Write to file (overwrite), returns bytes written |
| `fileAppend` | `(String, String) -> Result<Int, String>` | Append to file, returns bytes written |

## File Info

| Function | Signature | Description |
|----------|-----------|-------------|
| `fileExists` | `(String) -> Bool` | Check if file exists |
| `fileSize` | `(String) -> Result<Int, String>` | Get file size in bytes |

## File Management

| Function | Signature | Description |
|----------|-----------|-------------|
| `deleteFile` | `(String) -> Result<Nil, String>` | Delete file |

## Usage Examples

### Read and Process File
```rust
import "lib/io" (fileRead)

match fileRead("data.txt") {
    Ok(content) -> print("Content: " ++ content)
    Fail(err) -> print("Error: " ++ err)
}
```

### Write to File
```rust
import "lib/io" (fileWrite)

match fileWrite("output.txt", "Hello, World!") {
    Ok(bytes) -> print("Wrote " ++ show(bytes) ++ " bytes")
    Fail(err) -> print("Error: " ++ err)
}
```

### Read Partial File
```rust
import "lib/io" (fileReadAt, fileSize)

// Read last 100 bytes of a file
match fileSize("large.bin") {
    Ok(size) -> {
        match fileReadAt("large.bin", size - 100, 100) {
            Ok(data) -> print("Last 100 bytes: " ++ data)
            Fail(e) -> print(e)
        }
    }
    Fail(e) -> print(e)
}
```

### Interactive Input
```rust
import "lib/io" (readLine)

print("Enter your name: ")
match readLine() {
    Some(name) -> print("Hello, " ++ name ++ "!")
    Zero -> print("No input received")
}
```

