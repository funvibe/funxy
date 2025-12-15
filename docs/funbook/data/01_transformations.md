# 01. Data Transformations

## Task
Transform data elegantly and efficiently.

---

## Map: apply function to each element

```rust
import "lib/list" (map)

numbers = [1, 2, 3, 4, 5]

// Double each
doubled = map(fun(x) -> x * 2, numbers)
print(doubled)  // [2, 4, 6, 8, 10]

// Convert to strings
strings = map(fun(x) -> show(x), numbers)
print(strings)  // ["1", "2", "3", "4", "5"]
```

---

## Filter: keep only needed ones

```rust
import "lib/list" (filter)

numbers = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]

// Only even
evens = filter(fun(x) -> x % 2 == 0, numbers)
print(evens)  // [2, 4, 6, 8, 10]

// Greater than 5
big = filter(fun(x) -> x > 5, numbers)
print(big)  // [6, 7, 8, 9, 10]
```

---

## Fold/Reduce: fold into one value

```rust
import "lib/list" (foldl)

numbers = [1, 2, 3, 4, 5]

// Sum
sum = foldl(fun(acc, x) -> acc + x, 0, numbers)
print(sum)  // 15

// Product
product = foldl(fun(acc, x) -> acc * x, 1, numbers)
print(product)  // 120

// Maximum
maxVal = foldl(fun(acc, x) -> if x > acc { x } else { acc }, numbers[0], numbers)
print(maxVal)  // 5
```

---

## Flatten: unfold nested lists

```rust
import "lib/list" (flatten)

nested = [[1, 2], [3, 4], [5]]
flat = flatten(nested)
print(flat)  // [1, 2, 3, 4, 5]
```

---

## Combining transformations

```rust
import "lib/list" (filter, map)

users = [
    { name: "Alice", age: 30, active: true },
    { name: "Bob", age: 25, active: false },
    { name: "Carol", age: 35, active: true },
    { name: "David", age: 28, active: true }
]

// Get names of active users over 27
result = users
    |> filter(fun(u) -> u.active)
    |> filter(fun(u) -> u.age > 27)
    |> map(fun(u) -> u.name)

print(result)  // ["Alice", "Carol", "David"]
```

---

## Grouping

```rust
import "lib/list" (foldl)
import "lib/map" (mapGetOr, mapPut)

fun groupBy(items, keyFn) {
    foldl(fun(acc, item) -> {
        key = keyFn(item)
        existing = mapGetOr(acc, key, [])
        mapPut(acc, key, existing ++ [item])
    }, %{}, items)
}

people = [
    { name: "Alice", dept: "Engineering" },
    { name: "Bob", dept: "Sales" },
    { name: "Carol", dept: "Engineering" },
    { name: "David", dept: "Sales" }
]

byDept = groupBy(people, fun(p) -> p.dept)
print(byDept)
// %{"Engineering" => [...], "Sales" => [...]}
```

---

## Sorting

```rust
import "lib/list" (sort, sortBy)

numbers = [3, 1, 4, 1, 5, 9, 2, 6]

// Standard sorting
sorted = sort(numbers)
print(sorted)  // [1, 1, 2, 3, 4, 5, 6, 9]

// With custom comparator (descending)
descending = sortBy(numbers, fun(a, b) -> b - a)
print(descending)  // [9, 6, 5, 4, 3, 2, 1, 1]
```

---

## Sorting records

```rust
import "lib/list" (sortBy, map)

users = [
    { name: "Carol", score: 85 },
    { name: "Alice", score: 92 },
    { name: "Bob", score: 78 }
]

// By score (descending)
byScore = sortBy(users, fun(a, b) -> b.score - a.score)
names = map(fun(u) -> u.name, byScore)
print(names)  // ["Alice", "Carol", "Bob"]
```

---

## Unique values

```rust
import "lib/list" (unique)

nums = [1, 2, 2, 3, 3, 3, 4]
print(unique(nums))  // [1, 2, 3, 4]
```

---

## Zip: combine two lists

```rust
import "lib/list" (zip)

names = ["Alice", "Bob", "Carol"]
ages = [30, 25, 35]
pairs = zip(names, ages)
print(pairs)  // [("Alice", 30), ("Bob", 25), ("Carol", 35)]
```

---

## Partition: split into two groups

```rust
import "lib/list" (partition)

numbers = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
(evens, odds) = partition(fun(x) -> x % 2 == 0, numbers)
print(evens)  // [2, 4, 6, 8, 10]
print(odds)   // [1, 3, 5, 7, 9]
```

---

## Take and Drop

```rust
import "lib/list" (take, drop)

nums = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]

first5 = take(nums, 5)
print(first5)  // [1, 2, 3, 4, 5]

rest = drop(nums, 5)
print(rest)  // [6, 7, 8, 9, 10]
```

---

## Practical example: data processing

```rust
import "lib/list" (filter, map, foldl, sortBy, take)

// Sales data
sales = [
    { product: "Laptop", amount: 999.0, region: "EU" },
    { product: "Phone", amount: 699.0, region: "US" },
    { product: "Tablet", amount: 449.0, region: "EU" },
    { product: "Watch", amount: 299.0, region: "US" },
    { product: "Laptop", amount: 1099.0, region: "US" }
]

// Top 3 sales in US by amount
topUS = sales
    |> filter(fun(s) -> s.region == "US")
    |> fun(xs) -> sortBy(xs, fun(a, b) -> floatToInt(b.amount - a.amount))
    |> fun(xs) -> take(xs, 3)
    |> map(fun(s) -> s.product ++ ": $" ++ show(s.amount))

print(topUS)
// ["Laptop: $1099", "Phone: $699", "Watch: $299"]

// Total amount by regions
euTotal = sales
    |> filter(fun(s) -> s.region == "EU")
    |> foldl(fun(acc, s) -> acc + s.amount, 0.0)

print("EU Total: $" ++ show(euTotal))  // EU Total: $1448
```

---

## Comparison: imperative vs functional

```rust
import "lib/list" (filter, sortBy, take, map)

users = [
    { name: "Alice", score: 85, active: true },
    { name: "Bob", score: 92, active: false },
    { name: "Carol", score: 78, active: true },
    { name: "David", score: 95, active: true },
    { name: "Eve", score: 88, active: true }
]

// Task: get top 3 active users by score

// Imperative
filtered = []
for u in users {
    if u.active {
        filtered = filtered ++ [u]
    }
}
// ... need manual sorting and slicing

// Functional (one chain!)
top3 = users
    |> filter(fun(u) -> u.active)
    |> sortBy(fun(a, b) -> b.score - a.score)
    |> fun(xs) -> take(xs, 3)
    |> map(fun(u) -> u.name)

print(top3)  // ["David", "Eve", "Alice"]
