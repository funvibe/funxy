# 01. Unit Tests

## Task
Write and run tests to verify code correctness.

---

## Basic syntax

```rust
import "lib/test" (testRun, assertEquals)

// Test definition
testRun("addition works", fun() -> {
    assertEquals(2 + 2, 4)
})

testRun("strings concatenate", fun() -> {
    assertEquals("Hello" ++ " World", "Hello World")
})
```

---

## Running tests

```bash
# Run all tests in file
lang tests/my_tests.lang

# Output:
#  PASS addition works
#  PASS strings concatenate
# 2 tests passed, 0 failed
```

---

## assertEquals

```rust
import "lib/test" (testRun, assertEquals)

testRun("basic equality", fun() -> {
    assertEquals(1 + 1, 2)
    assertEquals("hello", "hello")
    assertEquals([1, 2, 3], [1, 2, 3])
    assertEquals({ a: 1 }, { a: 1 })
})
```

---

## assert

```rust
import "lib/test" (testRun, assert)
import "lib/list" (contains)

testRun("custom conditions", fun() -> {
    assert(5 > 3)
    assert(len("hello") == 5)
    assert(contains([1, 2, 3], 2))
    
    // With custom message
    assert(10 > 0, "10 should be positive")
})
```

---

## Testing Result

```rust
import "lib/test" (testRun, assertOk, assertFail)
import "lib/io" (fileRead)

testRun("file exists", fun() -> {
    result = fileRead("README.md")
    assertOk(result)
})

testRun("file not found", fun() -> {
    result = fileRead("nonexistent.txt")
    assertFail(result)
})

```

---

## Testing Option

```rust
import "lib/test" (testRun, assertSome, assertZero)
import "lib/list" (find)

testRun("element found", fun() -> {
    result = find(fun(x) -> x > 3, [1, 2, 3, 4, 5])
    assertSome(result)
})

testRun("element not found", fun() -> {
    result = find(fun(x) -> x > 10, [1, 2, 3, 4, 5])
    assertZero(result)
})

```

---

## Grouping tests

```rust
import "lib/test" (testRun, assertEquals)

// Math operations
testRun("math: addition", fun() -> {
    assertEquals(2 + 3, 5)
})

testRun("math: subtraction", fun() -> {
    assertEquals(5 - 3, 2)
})

testRun("math: multiplication", fun() -> {
    assertEquals(3 * 4, 12)
})
```

---

## Testing functions

```rust
import "lib/test" (testRun, assertEquals)

// Function to test
fun factorial(n: Int) -> Int {
    if n <= 1 { 1 } else { n * factorial(n - 1) }
}

// Tests
testRun("factorial of 0", fun() -> {
    assertEquals(factorial(0), 1)
})

testRun("factorial of 5", fun() -> {
    assertEquals(factorial(5), 120)
})

testRun("factorial of 10", fun() -> {
    assertEquals(factorial(10), 3628800)
})
```

---

## Testing ADT

```rust
import "lib/test" (testRun, assertEquals)

type Tree = Leaf(Int) | Node((Tree, Tree))

fun treeSum(t: Tree) -> Int {
    match t {
        Leaf(n) -> n
        Node((l, r)) -> treeSum(l) + treeSum(r)
    }
}

testRun("leaf sum", fun() -> {
    assertEquals(treeSum(Leaf(5)), 5)
})

testRun("node sum", fun() -> {
    tree = Node((Leaf(1), Node((Leaf(2), Leaf(3)))))
    assertEquals(treeSum(tree), 6)
})
```

---

## Parameterized tests

```rust
import "lib/test" (testRun, assertEquals)
import "lib/list" (forEach)

fun testCases() {
    [
        (0, 1),
        (1, 1),
        (2, 2),
        (3, 6),
        (4, 24),
        (5, 120)
    ]
}

fun factorial(n: Int) -> Int {
    if n <= 1 { 1 } else { n * factorial(n - 1) }
}

// Run tests for each case
testRun("factorial(0) = 1", fun() -> { assertEquals(factorial(0), 1) })
testRun("factorial(1) = 1", fun() -> { assertEquals(factorial(1), 1) })
testRun("factorial(2) = 2", fun() -> { assertEquals(factorial(2), 2) })
testRun("factorial(3) = 6", fun() -> { assertEquals(factorial(3), 6) })
testRun("factorial(4) = 24", fun() -> { assertEquals(factorial(4), 24) })
testRun("factorial(5) = 120", fun() -> { assertEquals(factorial(5), 120) })
```

---

## Testing with records

```rust
import "lib/test" (testRun, assertEquals)
import "lib/list" (foldl)

type Item = { name: String, price: Float, quantity: Int }
type Order = { items: List<Item>, discount: Float }

fun calculateTotal(order: Order) -> Float {
    subtotal = foldl(fun(acc, i) -> acc + i.price * intToFloat(i.quantity), 0.0, order.items)
    subtotal * (1.0 - order.discount)
}

testRun("empty order", fun() -> {
    order = { items: [], discount: 0.0 }
    assertEquals(calculateTotal(order), 0.0)
})

testRun("order with items", fun() -> {
    order = {
        items: [
            { name: "Apple", price: 1.0, quantity: 3 },
            { name: "Banana", price: 0.5, quantity: 4 }
        ],
        discount: 0.0
    }
    assertEquals(calculateTotal(order), 5.0)
})

testRun("order with discount", fun() -> {
    order = {
        items: [{ name: "Item", price: 100.0, quantity: 1 }],
        discount: 0.1
    }
    assertEquals(calculateTotal(order), 90.0)
})
```

---

## Best Practices

1. One test - one check (when possible)
2. Clear and descriptive test names
3. Test edge cases (empty lists, zeros, boundary values)
4. Tests should be independent of each other
5. Use parameterized tests for similar checks
