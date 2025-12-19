# Modules and Packages

As your program grows, you'll want to organize your code into multiple files and namespaces. Our language provides a simple module system based on packages and imports.

## File Extensions

Funxy supports three file extensions: `.lang`, `.funxy`, and `.fx`.

**Important rule:** All files in a package must use the **same extension**. The extension is determined by the entry file (the file named after the directory).

```
mylib/
├── mylib.funxy    ← Entry file determines extension (.funxy)
├── helpers.funxy  ← ✓ Same extension, will be loaded
└── utils.lang     ← ✗ Different extension, will be IGNORED
```

This prevents accidental mixing of files with different extensions in the same package.

## Package Structure

A package is a directory containing source files. Every package must have an **entry file** with the same name as the directory:

```rust
math/
└── math.lang    ← Entry file (required)
```

Multi-file packages:

```rust
math/
├── math.lang    ← Entry file, controls exports
├── vector.lang  ← Internal file
└── matrix.lang  ← Internal file
```

### Entry File Rules

1. The entry file (`pkgname/pkgname.lang`) controls what the package exports
2. Internal files declare `package pkgname` without export list
3. All files in the package can access each other's symbols

**math/math.lang** (entry file):
```rust
package math (Vector, add)  // Only Vector and add are exported

type Vector = { x: Int, y: Int }

fun add(a: Int, b: Int) -> Int { a + b }
```

**math/internal.lang** (internal file):
```rust
package math  // No export list — cannot control exports

fun helper() -> Int { 42 }  // Available within package only
```

## Export Syntax

| Syntax | Meaning |
|--------|---------|
| `package math (A, B)` | Export only A and B |
| `package math (*)` | Export everything |
| `package math !(A, B)` | Export everything except A and B |
| `package math` | Export nothing (internal package or private file) |

### Multiline Export Lists

For packages with many exports:

```rust
package mylib (
    Vector, Matrix,
    add, subtract, multiply,
    Transform, rotate, scale
)
```

Trailing commas are allowed.

## Imports

```rust
import "lib/list"           // Import as module object
import "lib/list" as l      // Import with alias
import "lib/list" (map)     // Import specific symbols
import "lib/list" (*)       // Import all symbols
import "lib/list" !(map)    // Import all except specified
```

### Accessing Imported Symbols

**With module object:**
```rust
import "lib/list"

nums = list.map(fun(x) -> x * 2, [1, 2, 3])
```

**With selective import:**
```rust
import "lib/list" (map, filter)

nums = map(fun(x) -> x * 2, [1, 2, 3])
evens = filter(fun(x) -> x % 2 == 0, nums)
```

### Aliasing vs Selective Import

These are **mutually exclusive**:

```rust
import "lib/list" as l     // ✓ Valid
l.map(fun(x) -> x * 2, [1, 2, 3])
```

```rust
import "lib/list" (map)    // ✓ Valid
map(fun(x) -> x * 2, [1, 2, 3])
```

```rust
import "lib/list" as l (map)  // ✗ Invalid: cannot combine alias with selective import
```

### ADT Constructor Auto-Import

When you import an ADT type, all its constructors are automatically imported:

```rust
import "lib/sql" (SqlValue)

// SqlInt, SqlString, SqlNull, etc. are auto-imported
v1 = SqlInt(42)
v2 = SqlString("hello")

match v1 {
    SqlInt(n) -> print(n)
    SqlString(s) -> print(s)
    _ -> print("other")
}
```

This works with both selective and wildcard imports.

### Module Name Collisions and Auto-Aliasing

When two modules have the same package name but different paths, the system automatically generates unique aliases to avoid conflicts.

#### Problem: Same Package Name

```rust
import "kit/sql"       // package name: sql
import "lib/sql" (SqlValue)  // package name: sql - CONFLICT!
```

Both modules are named `sql`, but they're from different paths.

#### Solution 1: Manual Aliases

Explicitly specify different aliases:

```rust
import "kit/sql" as orm
import "lib/sql" as types

// Now use qualified names:
instance orm.Model User { ... }
v = types.SqlValue(42)
```

#### Solution 2: Auto-Aliasing

When no explicit alias is given for non-selective imports, the system can use the last segment of the path as the default alias:

```rust
import "kit/sql"        // Default alias: sql
import "lib/types"      // Default alias: types
```

**For qualified trait names**, you can use the full path:

```rust
import "kit/sql"        // Accessible as: kit.sql
import "lib/sql" (SqlValue)

// Multi-level qualification:
instance kit.sql.Model User { ... }

// Selectively imported symbols work directly:
v = SqlValue(...)
```

#### Best Practices

1. **Use explicit aliases** when you know there will be collisions:
   ```rust
   import "kit/sql" as orm
   import "lib/sql" as types
   ```

2. **Use selective imports** to avoid bringing in module objects:
   ```rust
   import "kit/sql" (Model, toRow, tableName)
   import "lib/sql" (SqlValue, SqlInt, SqlString)

   // All symbols available directly:
   instance Model User {
       fun tableName(u) { "users" }
       fun toRow(u) { ... }
   }

   v = SqlValue(42)
   ```

3. **OR use module aliases** for namespace control:
   ```rust
   import "kit/sql" as orm
   import "lib/sql" as types

   // Access via qualified names:
   instance orm.Model User {
       fun tableName(u) { "users" }
       fun toRow(u) { ... }
   }

   v = types.SqlValue(42)
   ```

   **Note**: Don't import the same module twice (with alias AND selective) - choose one approach.

4. **For qualified traits**, use full path qualification:
   ```rust
   // No conflict even with same package name:
   instance qualified_pkg.core.Validator User { ... }
   instance qualified_pkg.db.Storage Product { ... }
   ```

## Trait Instances

**Trait instances follow their types.** When you export a type, all its trait instances are automatically exported.

**shapes/shapes.lang** (example package structure):
```
package shapes (Shape, Circle, area)

trait Shape {
    fun area(self: Self) -> Float
}

type Circle = Circle { radius: Float }

instance Shape Circle {
    fun area(c) {
        match c { Circle(r) -> 3.14159 * r.radius * r.radius }
    }
}

instance Semigroup Circle {
    fun operator (<>)(a, b) {
        match (a, b) {
            (Circle(r1), Circle(r2)) -> Circle { radius: r1.radius + r2.radius }
        }
    }
}

fun area(s: Shape) { Shape.area(s) }
```

**main.lang** (using the package):
```
import "./shapes" (*)

c1 = Circle { radius: 5.0 }
c2 = Circle { radius: 3.0 }

print(area(c1))      // Works — Shape instance exported with Circle
print(c1 <> c2)      // Works — Semigroup instance exported with Circle
```

### Qualified Trait Names

Traits can be referenced using qualified names, particularly useful when:
- Implementing traits from other modules
- Disambiguating traits with the same name from different modules

**Multi-level qualification:**
```rust
import "kit/sql" (toRow, tableName)
import "lib/sql" (SqlValue)

type User = MkUser { id: Int, name: String }

// Using fully qualified trait name
instance kit.sql.Model User {
    fun tableName(u) { "users" }
    fun toRow(u) {
        match u {
            MkUser(r) -> {
                // build row map
            }
        }
    }
}
```

**Trait method auto-import:**

When you selectively import trait methods, the trait itself is automatically imported:

```rust
import "kit/sql" (toRow, tableName)
// The Model trait is now available for implementing

instance kit.sql.Model User { ... }
```

**With module aliases:**
```rust
import "kit/sql" as orm

type Product = MkProduct { id: Int, name: String }

// Using aliased module name in trait qualification
instance orm.Model Product {
    fun tableName(p) { "products" }
    fun toRow(p) { ... }
}
```

### Orphan Rule

Trait instances can only be defined where:
- The **type** is defined, OR
- The **trait** is defined

```
// ✓ OK: defining instance where type is defined
type MyNum = MyNum { value: Int }
instance Semigroup MyNum { ... }

// ✓ OK: defining instance where trait is defined
trait MyTrait { ... }
instance MyTrait Int { ... }

// ✗ Error: orphan instance (neither type nor trait defined here)
instance Semigroup Int { ... }  // Int and Semigroup are built-in
```

## Extension Methods

Extension methods follow the same rule — they are exported with their type:

**geometry/geometry.lang** (example package structure):
```
package geometry (Point)

type Point = { x: Int, y: Int }

extend Point {
    fun distance(self) -> Float {
        sqrt(intToFloat(self.x * self.x + self.y * self.y))
    }
}
```

**main.lang** (using the package):
```
import "./geometry" (Point)

p: Point = { x: 3, y: 4 }
d = p.distance()  // Works — extension method exported with Point
```

## Package Groups

Import all sub-packages from a directory:

```
mylib/
├── utils/
│   └── utils.lang      # package utils (*)
└── helpers/
    └── helpers.lang    # package helpers (*)
```

```
import "./mylib" (*)

double(5)       // from mylib/utils
greet("World")  // from mylib/helpers
```

## Virtual Packages

Built-in packages use paths without `./`:

```rust
import "lib/list" (map, filter)
import "lib/string" (stringToUpper, stringTrim)
import "lib/map" (*)
```

## Cyclic Dependencies

Supported via multi-pass analysis. Module A can import B while B imports A.

## Summary

| Feature | Behavior |
|---------|----------|
| File extensions | `.lang`, `.funxy`, `.fx` |
| Extension rule | All files in package must use same extension |
| Entry file | `pkgname/pkgname.{lang,funxy,fx}` required |
| Export control | Only in entry file |
| Internal files | `package pkgname` (no export list) |
| Trait instances | Exported with their type |
| Extension methods | Exported with their type |
| ADT constructors | Auto-imported with parent type |
| Trait methods | Auto-imported when methods are imported |
| Module aliases | Explicit: `as alias` or default from path |
| Name collisions | Use explicit aliases or qualified names |
| Qualified traits | Multi-level: `module.submodule.Trait` |
| Orphan instances | Forbidden (must define where type or trait lives) |
