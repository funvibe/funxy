# JSON

The `lib/json` package provides JSON encoding, decoding, and dynamic JSON manipulation via the `Json` ADT.

## Import

```rust
// Basic functions
import "lib/json" (jsonEncode, jsonDecode)

// With Json ADT
import "lib/json" (jsonEncode, jsonDecode, jsonParse, jsonFromValue, jsonGet, jsonKeys)
```

## Encoding

### `jsonEncode(value) -> String`

Converts any value to a JSON string.

```rust
import "lib/json" (jsonEncode)

// Basic types
print(jsonEncode(42))        // 42
print(jsonEncode(3.14))      // 3.14
print(jsonEncode(true))      // true
print(jsonEncode("hello"))   // "hello"

// Lists
print(jsonEncode([1, 2, 3]))     // [1,2,3]

// Records
user = { name: "Alice", age: 30 }
print(jsonEncode(user))  // {"age":30,"name":"Alice"}

// Tuples (encoded as arrays)
print(jsonEncode((1, "two")))  // [1,"two"]

// Option
print(jsonEncode(Some(42)))  // 42
print(jsonEncode(Zero))      // null
```

### Type Mapping (Encode)

| Type | JSON |
|------|------|
| `Int` | number |
| `Float` | number |
| `Bool` | true/false |
| `String` | string |
| `Nil` | null |
| `List<T>` | array |
| `Tuple` | array |
| `Record` | object |
| `Option<T>` | value or null |
| `BigInt` | string |
| `Rational` | string ("3/4") |

## Decoding

### `jsonDecode(json: String) -> Result<T, String>`

Parses a JSON string into a value. Returns `Ok(value)` on success, `Fail(error)` on failure.

```rust
import "lib/json" (jsonDecode)

// Decode number
match jsonDecode("42") {
    Ok(n) -> print(n)      // 42
    Fail(e) -> print(e)
}

// Decode array
match jsonDecode("[1, 2, 3]") {
    Ok(arr) -> print(arr)  // [1, 2, 3]
    Fail(e) -> print(e)
}

// Decode boolean
match jsonDecode("true") {
    Ok(b) -> print(b)      // true
    Fail(e) -> print(e)
}

// Decode null
match jsonDecode("null") {
    Ok(v) -> print(v)      // Nil
    Fail(e) -> print(e)
}

// Handle errors
match jsonDecode("not json") {
    Ok(_) -> print("success")
    Fail(e) -> print("Error: " ++ e)  // Error: invalid JSON: ...
}
```

### Type Inference

The decoder automatically infers types from JSON values:

| JSON | Inferred Type |
|------|---------------|
| number (integer) | `Int` |
| number (decimal) | `Float` |
| true/false | `Bool` |
| null | `Nil` |
| string | `String` |
| array | `List<?>` |
| object | `Record` |

## Working with Records

```rust
import "lib/json" (jsonEncode, jsonDecode)

// Encode record
config = { host: "localhost", port: 8080, debug: true }
jsonStr = jsonEncode(config)
print(jsonStr)  // {"debug":true,"host":"localhost","port":8080}

// Decode and access fields
match jsonDecode(jsonStr) {
    Ok(data) -> {
        print(data.host)   // localhost
        print(data.port)   // 8080
    }
    Fail(e) -> print(e)
}
```

## Nested Structures

```rust
import "lib/json" (jsonEncode)

// Nested records and arrays
data = {
    users: [
        { name: "Alice", active: true },
        { name: "Bob", active: false }
    ],
    count: 2
}

json = jsonEncode(data)
print(json)
// {"count":2,"users":[{"active":true,"name":"Alice"},{"active":false,"name":"Bob"}]}
```

## Error Handling

```rust
import "lib/json" (jsonDecode)

type Config = { port: Int, host: String }

// Use ? operator with Result
fun loadConfig(json: String) -> Result<Config, String> {
    config = jsonDecode(json)?
    Ok(config)
}

// Or use match for detailed handling
fun processJson(userInput: String) -> Config {
    match jsonDecode(userInput) {
        Ok(data) -> data
        Fail(e) -> {
            print("JSON error: " ++ e)
            { port: 8080, host: "localhost" }
        }
    }
}
```

## BigInt and Rational

Large numbers are encoded as strings to preserve precision:

```rust
import "lib/bignum" (bigIntNew, ratFromInt)
import "lib/json" (jsonEncode)

big = bigIntNew("123456789012345678901234567890")
print(jsonEncode(big))  // "123456789012345678901234567890"

r = ratFromInt(3, 4)
print(jsonEncode(r))    // "3/4"
```

## Json ADT

For dynamic JSON manipulation, use the `Json` type:

```
// Json ADT (built-in)
type Json =
    JNull                       // null
  | JBool Bool                  // true/false
  | JNum Float                  // numbers
  | JStr String                 // strings
  | JArr List<Json>             // arrays
  | JObj List<(String, Json)>   // objects (key-value pairs)
```

### parseJson

Parses a JSON string into the `Json` ADT:

```rust
import "lib/json" (Json, jsonParse)

match jsonParse("[1, 2, 3]") {
    Ok(JArr(items)) -> print("Array: " ++ show(len(items)))
    Ok(_) -> print("Not an array")
    Fail(e) -> print("Error: " ++ e)
}

match jsonParse("true") {
    Ok(JBool(b)) -> print("Boolean: " ++ show(b))
    Ok(JNum(n)) -> print("Number: " ++ show(n))
    Ok(JNull) -> print("Null")
    Ok(_) -> print("Other")
    Fail(e) -> print("Error")
}
```

### toJson

Converts any value to `Json`:

```rust
import "lib/json" (jsonFromValue)

json1 = jsonFromValue({ x: 10, y: 20 })
print(json1)  // JObj([("x", JNum(10)), ("y", JNum(20))])

json2 = jsonFromValue([1, 2, 3])
print(json2)  // JArr([JNum(1), JNum(2), JNum(3)])
```

### jsonGet

Gets a field from `JObj`:

```rust
import "lib/json" (Json, jsonParse, jsonGet)

jsonStr = "{\"name\":\"Alice\",\"age\":30}"
match jsonParse(jsonStr) {
    Ok(obj) -> {
        match jsonGet(obj, "name") {
            Some(JStr(name)) -> print("Name: " ++ name)
            Some(_) -> print("name is not a string")
            Zero -> print("no name field")
        }
    }
    Fail(e) -> print(e)
}
```

### jsonKeys

Gets all keys from `JObj`:

```rust
import "lib/json" (jsonParse, jsonKeys)

jsonStr = "{\"name\":\"Alice\",\"age\":30}"
match jsonParse(jsonStr) {
    Ok(obj) -> {
        keys = jsonKeys(obj)
        for key in keys {
            print(key)
        }
    }
    Fail(e) -> print(e)
}
```

### When to Use Json ADT

Use `Json` ADT when:
- Schema is unknown at compile time
- Need to iterate over object keys
- Building JSON transformers or validators
- Pretty-printing or formatting JSON

Use `decode` when:
- Schema is known
- Want type-safe field access
- Working with records

## Best Practices

1. **Always handle decode errors** - JSON from external sources may be invalid

2. **Use records for structured data** - They map naturally to JSON objects

3. **Prefer Option for nullable fields** - `Some(x)` → value, `Zero` → null

4. **BigInt for large numbers** - JSON numbers have precision limits

5. **Use Json ADT for dynamic access** - When you need to explore unknown JSON structures

