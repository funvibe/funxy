# UUID (lib/uuid)

The `lib/uuid` module provides functions for generating and working with UUIDs.

```rust
import "lib/uuid" (*)
```

## What is UUID?

UUID (Universally Unique Identifier) — a 128-bit identifier that guarantees uniqueness without centralized coordination.

Format: `550e8400-e29b-41d4-a716-446655440000` (36 characters)

## UUID Versions

### v4 — Random (default)

Generated from random numbers. The most popular option.

```rust
import "lib/uuid" (uuidNew, uuidToString)

myUuid = uuidNew()
print(uuidToString(myUuid))  // "550e8400-e29b-41d4-a716-446655440000"
```

### v7 — Time-ordered

Contains a timestamp. Ideal for primary keys in databases — maintains chronological order.

```rust
import "lib/uuid" (uuidV7, uuidToString)

// Each next UUID is "greater" than the previous one
id1 = uuidV7()
id2 = uuidV7()
// id2 > id1 by creation time
```

### v5 — Deterministic

Generated from namespace + name via SHA-1. The same input always gives the same UUID.

```rust
import "lib/uuid" (uuidV5, uuidNamespaceDNS, uuidToString)

ns = uuidNamespaceDNS()

// Same input data = same result
id1 = uuidV5(ns, "example.com")
id2 = uuidV5(ns, "example.com")
print(uuidToString(id1) == uuidToString(id2))  // true
```

#### Standard Namespaces

```rust
uuidNamespaceDNS()   // for domain names
uuidNamespaceURL()   // for URLs
uuidNamespaceOID()   // for OIDs
uuidNamespaceX500()  // for X.500 DNs
```

## Special UUIDs

```rust
import "lib/uuid" (uuidNil, uuidMax, uuidToString)

// Nil UUID (all zeros)
print(uuidToString(uuidNil()))  // "00000000-0000-0000-0000-000000000000"

// Max UUID (all ones)
print(uuidToString(uuidMax()))  // "ffffffff-ffff-ffff-ffff-ffffffffffff"
```

## Parsing

```rust
import "lib/uuid" (uuidParse, uuidToString, uuidVersion)

// Different formats are supported
match uuidParse("550e8400-e29b-41d4-a716-446655440000") {
    Ok(u) -> {
        print("Version: " ++ show(uuidVersion(u)))
        print("UUID: " ++ uuidToString(u))
    }
    Fail(err) -> print("Error: " ++ err)
}

// These also work:
// "550e8400e29b41d4a716446655440000"     (without hyphens)
// "{550e8400-e29b-41d4-a716-446655440000}" (with curly braces)
// "urn:uuid:550e8400-e29b-41d4-a716-446655440000" (URN)
// "550E8400-E29B-41D4-A716-446655440000"  (uppercase)
```

## Output Formats

```rust
import "lib/uuid" (*)

u = uuidNew()

// Standard (8-4-4-4-12)
print(uuidToString(u))        // "550e8400-e29b-41d4-a716-446655440000"

// Compact (without hyphens)
print(uuidToStringCompact(u)) // "550e8400e29b41d4a716446655440000"

// URN
print(uuidToStringUrn(u))     // "urn:uuid:550e8400-e29b-41d4-a716-446655440000"

// With curly braces
print(uuidToStringBraces(u))  // "{550e8400-e29b-41d4-a716-446655440000}"

// Uppercase
print(uuidToStringUpper(u))   // "550E8400-E29B-41D4-A716-446655440000"
```

## Conversion to Bytes

```rust
import "lib/uuid" (uuidNew, uuidToBytes, uuidFromBytes, uuidToString)

original = uuidNew()

// UUID -> Bytes (16 bytes)
bytes = uuidToBytes(original)
print(len(bytes))  // 16

// Bytes -> UUID
match uuidFromBytes(bytes) {
    Ok(restored) -> {
        print(uuidToString(original) == uuidToString(restored))  // true
    }
    Fail(err) -> print("Error: " ++ err)
}
```

## UUID Information

```rust
import "lib/uuid" (uuidNew, uuidV7, uuidNil, uuidVersion, uuidIsNil)

// UUID version
print(uuidVersion(uuidNew()))  // 4
print(uuidVersion(uuidV7()))   // 7

// Nil check
print(uuidIsNil(uuidNil()))    // true
print(uuidIsNil(uuidNew()))    // false
```

## Comparison

UUIDs support equality comparison:

```rust
import "lib/uuid" (uuidNew, uuidNil)

// Equality
a = uuidNil()
b = uuidNil()
print(a == b)  // true

// Inequality (different random UUIDs)
c = uuidNew()
d = uuidNew()
print(c != d)  // true
```

## Practical Examples

### Generating ID for Entity

```rust
import "lib/uuid" (uuidNew, uuidToString)

type User = {
    id: String,
    name: String,
    email: String
}

fun createUser(name: String, email: String) -> User {
    {
        id: uuidToString(uuidNew()),
        name: name,
        email: email
    }
}
```

### ID for Database (v7 for sorting)

```rust
import "lib/uuid" (uuidV7, uuidToString)
import "lib/sql" (*)

fun insertRecord(db, name: String) -> Result<String, Int> {
    recordId = uuidToString(uuidV7())
    sqlExec(db, "INSERT INTO records (id, name) VALUES ($1, $2)", [
        SqlString(recordId),
        SqlString(name)
    ])
}
```

### Deterministic ID by Email

```rust
import "lib/uuid" (uuidV5, uuidNamespaceDNS, uuidToString)

fun userIdFromEmail(email: String) -> String {
    ns = uuidNamespaceDNS()
    uuidToString(uuidV5(ns, email))
}

// Always the same ID for the same email
id1 = userIdFromEmail("user@example.com")
id2 = userIdFromEmail("user@example.com")
print(id1 == id2)  // true
```

## When to Use Which Version?

| Version | When to Use |
|--------|-------------------|
| v4 | General purpose, when you just need uniqueness |
| v7 | Primary keys in databases (sortable by time) |
| v5 | When you need a deterministic ID from known data |

## Summary

| Function | Type | Description |
|---------|-----|----------|
| `uuidNew` | `() -> Uuid` | Random UUID (v4) |
| `uuidV4` | `() -> Uuid` | Alias for uuidNew |
| `uuidV5` | `(Uuid, String) -> Uuid` | Deterministic UUID |
| `uuidV7` | `() -> Uuid` | Time-ordered |
| `uuidNil` | `() -> Uuid` | Nil UUID |
| `uuidMax` | `() -> Uuid` | Max UUID |
| `uuidNamespaceDNS` | `() -> Uuid` | DNS namespace |
| `uuidNamespaceURL` | `() -> Uuid` | URL namespace |
| `uuidNamespaceOID` | `() -> Uuid` | OID namespace |
| `uuidNamespaceX500` | `() -> Uuid` | X.500 namespace |
| `uuidParse` | `(String) -> Result<String, Uuid>` | Parse string |
| `uuidFromBytes` | `(Bytes) -> Result<String, Uuid>` | From 16 bytes |
| `uuidToString` | `(Uuid) -> String` | Standard format |
| `uuidToStringCompact` | `(Uuid) -> String` | Without hyphens |
| `uuidToStringUrn` | `(Uuid) -> String` | URN format |
| `uuidToStringBraces` | `(Uuid) -> String` | With curly braces |
| `uuidToStringUpper` | `(Uuid) -> String` | Uppercase |
| `uuidToBytes` | `(Uuid) -> Bytes` | To 16 bytes |
| `uuidVersion` | `(Uuid) -> Int` | UUID version |
| `uuidIsNil` | `(Uuid) -> Bool` | Nil check |
