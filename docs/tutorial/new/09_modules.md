# 9. Modules & Imports

[← Back to Index](./00_index.md)

## Package Structure

```
mypackage/
├── mypackage.lang    ← Entry file (controls exports)
├── utils.lang        ← Internal file
└── helpers.lang      ← Internal file
```

## Exporting

In `mypackage.lang`:

```text
// Export specific symbols
package mypackage (MyType, myFunction, helperFunc)

// Export everything
package mypackage (*)

// Export everything except specific symbols
package mypackage !(internalFunc, PrivateType)

// Export nothing (internal module)
package mypackage
```

## Importing

Imports are package-wide. A value, type, ADT constructor, or module alias
imported by any package file is available in every other file. All imports are
collected before declarations and top-level code are processed, so visibility
does not depend on file order.

Import as module object:

```rust
import "lib/list"
// list.map(f, xs)
```

Import with alias:

```rust
import "lib/list" as l
// l.map(f, xs)
```

Import specific symbols:

```rust
import "lib/list" (map, filter, foldl)
// map(f, xs)
```

Import all symbols:

```rust
import "lib/list" (*)
// map(f, xs)
```

## Single Import Rule

Each module can be imported only once per file.

Different files may import the same module path; their selected symbols are
merged in the package scope. Conflicting local names remain an error.

```text
// Wrong
import "lib/list" (map)
import "lib/list" (filter)

// Correct
import "lib/list" (map, filter)
```

## Auto-Import of ADT Constructors

When you import an Algebraic Data Type, its constructors are automatically available.

```rust
import "lib/json" (Json)

// Constructors are available
x = JNull
y = JBool(true)
z = JStr("hello")
```

## Qualified Trait Names

Traits can be accessed via their module alias.

```text
import "mylib/orm" as orm

instance orm.Model User {
    fun tableName(u) { "users" }
}
```

## File Extensions

Supported extensions: `.lang`, `.funxy`, `.fx`.
All files in a package must use the same extension.
