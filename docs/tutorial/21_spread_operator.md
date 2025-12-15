# Iteration 9: Spread Operator

The spread operator `...` allows you to unpack lists or tuples into individual elements.

## List Literal Spread

You can spread a list into another list literal.

```rust
l1 = [1, 2]
l2 = [3, 4]
l3 = [0, ...l1, ...l2, 5]
// l3 is [0, 1, 2, 3, 4, 5]
```

## Variadic Functions

You can define functions that accept a variable number of arguments using `...`. The arguments are collected into a List.

```rust
fun sum(args: Int...) -> Int {
    acc = 0
    for x in args {
        acc = acc + x
    }
    acc
}

s = sum(1, 2, 3) // 6
```

## Variadic Lambdas

Anonymous functions (lambdas) also support variadic parameters.

```rust
// Variadic lambda
variadicSum = fun (args: Int...) Int {
    total = 0
    for x in args {
        total = total + x
    }
    total
}

result = variadicSum(1, 2, 3, 4)  // 10

// Using spread inside lambda
appendAll = fun (items: Int...) List<Int> {
    base = [0]
    [base..., items...]
}

appendAll(1, 2, 3)  // [0, 1, 2, 3]
```

## Spreading Arguments in Function Calls

You can pass a list (or tuple) to a variadic function using the spread operator.

```rust
fun sum(args: Int...) -> Int {
    acc = 0
    for x in args { acc = acc + x }
    acc
}

nums = [10, 20]
s = sum(5, ...nums, 30)
print(s) // 65
```

This works with lambdas too:

```rust
variadicSum(...[4, 5, 6])  // 15
```

## Built-in Print

The `print` function supports spread arguments natively.

```rust
// print with spread
values = ["a", "b", "c"]
print(...values)  // prints: a b c
```

## Limitations

### Variadic Type Annotations

You **cannot** write variadic types in type annotations. The syntax `(Int...) -> Int` is not supported.

```rust
// ❌ These do NOT work (syntax errors):
// f: (Int...) -> Int = variadicSum
// fun applyVariadic(f: (Int...) -> Int) -> Int { f(1, 2, 3) }

// Variadic type annotations are not supported
print("See workaround below")
```

**Reason**: Variadic is a calling convention, not a type. At the type level, a variadic parameter is just `List<T>`.

### Workaround: Use List<T>

If you need to pass variadic functions to higher-order functions, use `List<T>` explicitly:

```rust
// Instead of trying to type a variadic function,
// create a wrapper that takes List<T>:

fun sumList(args: List<Int>) -> Int {
    acc = 0
    for x in args {
        acc = acc + x
    }
    acc
}

// Now you can pass it with explicit typing:
fun applyToNumbers(f: (List<Int>) -> Int, nums: List<Int>) -> Int {
    f(nums)
}

result = applyToNumbers(sumList, [1, 2, 3, 4])  // 10
```

### Why This Design?

1. **Simplicity**: Variadic is syntactic sugar for convenient calling, not a separate type.
2. **Type inference works**: You can still use variadic lambdas without explicit type annotations:
   ```rust
   // Type is inferred automatically
   mySum = fun (args: Int...) Int { ... }
   mySum(1, 2, 3)  // works!
   ```
