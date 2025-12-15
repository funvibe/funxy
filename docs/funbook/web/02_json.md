# 02. JSON Processing

## Task
Parse, create and transform JSON data.

---

## Parsing JSON string

```rust
import "lib/json" (jsonDecode, jsonEncode)

// JSON string with escaping
jsonStr = "{\"name\": \"Alice\", \"age\": 30, \"active\": true}"

match jsonDecode(jsonStr) {
    Ok(data) -> {
        // Access fields through dot notation
        print(data.name)    // Alice
        print(data.age)     // 30
        print(data.active)  // true
    }
    Fail(err) -> print("Parse error: " ++ err)
}
```

---

## Creating JSON from records

```rust
import "lib/json" (jsonEncode)

// Records are automatically serialized
user = { name: "Bob", age: 25 }
print(jsonEncode(user))
// {"name":"Bob","age":25}

// Nested records
profile = {
    user: { name: "Alice", age: 30 },
    settings: { theme: "dark", notifications: true }
}
print(jsonEncode(profile))
```

---

## Creating JSON from Map

```rust
import "lib/json" (jsonEncode)
import "lib/map" (*)

// Map with dynamic keys
scores = %{ "Alice" => 100, "Bob" => 85, "Carol" => 92 }
print(jsonEncode(scores))
// {"Alice":100,"Bob":85,"Carol":92}
```

---

## Working with arrays

```rust
import "lib/json" (jsonEncode)
import "lib/list" (filter, map)

users = [
    { id: 1, name: "Alice", role: "admin" },
    { id: 2, name: "Bob", role: "user" },
    { id: 3, name: "Carol", role: "admin" }
]

// Filtering
admins = filter(fun(u) -> u.role == "admin", users)
print(jsonEncode(admins))
// [{"id":1,"name":"Alice","role":"admin"},{"id":3,"name":"Carol","role":"admin"}]

// Transformation
names = map(fun(u) -> u.name, users)
print(jsonEncode(names))
// ["Alice","Bob","Carol"]
```

---

## Nested structures

```rust
import "lib/json" (jsonEncode)

company = {
    name: "Acme Corp",
    departments: [
        {
            name: "Engineering",
            floor: 3,
            employees: [
                { name: "Alice", title: "Senior Dev" },
                { name: "Bob", title: "Junior Dev" }
            ]
        },
        {
            name: "Sales",
            floor: 1,
            employees: [
                { name: "Carol", title: "Manager" }
            ]
        }
    ]
}

print(jsonEncode(company))

// Access to nested data
for dept in company.departments {
    print(dept.name ++ " (floor " ++ show(dept.floor) ++ "):")
    for emp in dept.employees {
        print("  - " ++ emp.name ++ ", " ++ emp.title)
    }
}
```

---

## Reading JSON from file

```rust
import "lib/io" (fileRead)
import "lib/json" (jsonDecode)

// fileRead returns Result
match fileRead("config.json") {
    Ok(content) -> match jsonDecode(content) {
        Ok(config) -> {
            print("Host: " ++ config.host)
            print("Port: " ++ show(config.port))
        }
        Fail(e) -> print("Invalid JSON: " ++ e)
    }
    Fail(err) -> print("Cannot read file: " ++ err)
}
```

---

## Writing JSON to file

```rust
import "lib/io" (fileWrite)
import "lib/json" (jsonEncode)

config = {
    host: "localhost",
    port: 8080,
    debug: true,
    maxConnections: 100,
    allowedOrigins: ["http://localhost:3000", "https://example.com"]
}

match fileWrite("config.json", jsonEncode(config)) {
    Ok(_) -> print("Config saved successfully")
    Fail(err) -> print("Cannot write file: " ++ err)
}

```

---

## Data transformation

```rust
import "lib/json" (jsonEncode)
import "lib/list" (map)

// Input data
people = [
    { firstName: "Alice", lastName: "Smith", birthYear: 1990 },
    { firstName: "Bob", lastName: "Jones", birthYear: 1985 },
    { firstName: "Carol", lastName: "Wilson", birthYear: 1992 }
]

// Transformation: create new structure
transformed = map(fun(p) -> {
    fullName: p.firstName ++ " " ++ p.lastName,
    initial: p.firstName[0],
    age: 2024 - p.birthYear
}, people)

print(jsonEncode(transformed))
// [{"fullName":"Alice Smith","initial":"A","age":34},...]
```

---

## Filtering and aggregation

```rust
import "lib/json" (jsonEncode)
import "lib/list" (filter, map, foldl)

orders = [
    { id: 1, customer: "Alice", total: 150.0, status: "completed" },
    { id: 2, customer: "Bob", total: 89.50, status: "pending" },
    { id: 3, customer: "Alice", total: 220.0, status: "completed" },
    { id: 4, customer: "Carol", total: 45.0, status: "completed" }
]

// Only completed orders
completed = filter(fun(o) -> o.status == "completed", orders)

// Total sum
totalRevenue = foldl(fun(acc, o) -> acc + o.total, 0.0, completed)
print("Total revenue: $" ++ show(totalRevenue))  // $415.0

// Orders by customer
aliceOrders = filter(fun(o) -> o.customer == "Alice", orders)
aliceTotal = foldl(fun(acc, o) -> acc + o.total, 0.0, aliceOrders)
print("Alice total: $" ++ show(aliceTotal))  // $370.0
```

---

## API Response processing

```rust
import "lib/json" (jsonEncode)
import "lib/list" (map, filter, foldl)

// API response simulation
apiResponse = {
    status: "success",
    data: {
        users: [
            { id: 1, name: "Alice", email: "alice@example.com", active: true },
            { id: 2, name: "Bob", email: "bob@example.com", active: false },
            { id: 3, name: "Carol", email: "carol@example.com", active: true }
        ],
        pagination: { page: 1, totalPages: 5, perPage: 10 }
    }
}

// Extract only active users
activeUsers = filter(fun(u) -> u.active, apiResponse.data.users)

// Create response for frontend
frontendResponse = {
    users: map(fun(u) -> { id: u.id, name: u.name }, activeUsers),
    meta: {
        count: len(activeUsers),
        page: apiResponse.data.pagination.page
    }
}

print(jsonEncode(frontendResponse))
```

---

## Comparison: imperative vs functional

```rust
import "lib/list" (filter, map, foldl)

products = [
    { name: "Laptop", price: 999.0, category: "electronics" },
    { name: "Book", price: 15.0, category: "books" },
    { name: "Phone", price: 699.0, category: "electronics" },
    { name: "Desk", price: 200.0, category: "furniture" }
]

// Task: find average price of electronics

// Imperative style
electronics1 = []
for p in products {
    if p.category == "electronics" {
        electronics1 = electronics1 ++ [p]
    }
}
sum1 = 0.0
for e in electronics1 {
    sum1 += e.price
}
avg1 = sum1 / intToFloat(len(electronics1))
print("Imperative avg: " ++ show(avg1))

// Functional style (one chain!)
electronics2 = filter(fun(p) -> p.category == "electronics", products)
sum2 = foldl(fun(acc, p) -> acc + p.price, 0.0, electronics2)
avg2 = sum2 / intToFloat(len(electronics2))
print("Functional avg: " ++ show(avg2))
```

---

## Building JSON for API

```rust
import "lib/json" (jsonEncode)

// Function to create standard API response
fun apiSuccess(data) {
    {
        status: "success",
        data: data,
        error: Nil
    }
}

fun apiError(message: String, code: Int) {
    {
        status: "error",
        data: Nil,
        error: { message: message, code: code }
    }
}

// Usage
print(jsonEncode(apiSuccess({ user: { id: 1, name: "Alice" } })))
// {"status":"success","data":{"user":{"id":1,"name":"Alice"}},"error":null}

print(jsonEncode(apiError("Not found", 404)))
// {"status":"error","data":null,"error":{"message":"Not found","code":404}}
