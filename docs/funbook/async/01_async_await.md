# 01. Async/Await

## Task
Perform asynchronous operations (I/O, HTTP requests) without blocking.

---

## Basic Syntax

```rust
import "lib/task" (async, await)

// Create an asynchronous task
task = async(fun() -> {
    // long operation
    42
})

// Wait for result
result = await(task)
print(result)
```

---

## Simple Example

```rust
import "lib/task" (async, await)
import "lib/time" (sleepMs)

fun slowAdd(a: Int, b: Int) -> Int {
    sleepMs(1000)  // 1 second
    a + b
}

task = async(fun() -> slowAdd(2, 3))
print("Task started...")
result = await(task)
print("Result: " ++ show(result))  // Result: 5
```

---

## Parallel Execution

```rust
import "lib/task" (async, await)
import "lib/time" (sleepMs)

fun fetchUser(id: Int) {
    sleepMs(500)
    { id: id, name: "User " ++ show(id) }
}

// Run three requests IN PARALLEL
task1 = async(fun() -> fetchUser(1))
task2 = async(fun() -> fetchUser(2))
task3 = async(fun() -> fetchUser(3))

// Wait for all results
user1 = await(task1)
user2 = await(task2)
user3 = await(task3)

// Total time: ~500ms instead of ~1500ms!
print(user1)
print(user2)
print(user3)
```

---

## HTTP Requests in Parallel

```rust
import "lib/task" (async, await)
import "lib/http" (httpGet)
import "lib/json" (jsonDecode)

fun fetchData(url: String) {
    match httpGet(url) {
        Ok(resp) -> match jsonDecode(resp.body) {
            Ok(data) -> data
            Fail(e) -> { error: e }
        }
        Fail(e) -> { error: e }
    }
}

// Parallel requests to different APIs
usersTask = async(fun() -> fetchData("https://api.example.com/users"))
postsTask = async(fun() -> fetchData("https://api.example.com/posts"))

users = await(usersTask)
posts = await(postsTask)

print("Got users and posts")
```

---

## Task Pool

```rust
import "lib/task" (async, await)
import "lib/list" (map)

fun processInParallel(items, process) {
    tasks = map(fun(item) -> async(fun() -> process(item)), items)
    map(fun(t) -> await(t), tasks)
}

urls = [
    "https://api.example.com/1",
    "https://api.example.com/2",
    "https://api.example.com/3"
]

results = processInParallel(urls, fun(url) -> { url: url, status: "fetched" })
print(results)
```

---

## Sequential vs Parallel

```rust
import "lib/task" (async, await)
import "lib/time" (sleepMs, timeNow)

fun slowTask(ms: Int) -> Int {
    sleepMs(ms)
    ms
}

// Sequential: 300ms
start1 = timeNow()
slowTask(100)
slowTask(100)
slowTask(100)
elapsed1 = timeNow() - start1
print("Sequential: " ++ show(elapsed1) ++ "ms")

// Parallel: ~100ms
start2 = timeNow()
t1 = async(fun() -> slowTask(100))
t2 = async(fun() -> slowTask(100))
t3 = async(fun() -> slowTask(100))
results = [await(t1), await(t2), await(t3)]
elapsed2 = timeNow() - start2
print("Parallel: " ++ show(elapsed2) ++ "ms")
```

---

## Practical Example: Web Scraper

```rust
import "lib/task" (async, await)
import "lib/http" (httpGet)
import "lib/list" (map)

fun scrapeUrl(url: String) {
    match httpGet(url) {
        Ok(resp) -> {
            url: url,
            status: resp.status,
            size: len(resp.body)
        }
        Fail(e) -> {
            url: url,
            status: 0,
            size: 0
        }
    }
}

urls = [
    "https://example.com",
    "https://github.com"
]

// Scrape all URLs in parallel
tasks = map(fun(url) -> async(fun() -> scrapeUrl(url)), urls)
results = map(fun(t) -> await(t), tasks)

for result in results {
    match result {
        Ok(r) -> print(r.url ++ " - status " ++ show(r.status) ++ " - " ++ show(r.size) ++ " bytes")
        Fail(e) -> print("Error: " ++ e)
    }
}
```

---

## When to Use Async

- I/O operations (files, network)
- HTTP requests
- Databases
- Any "waiting" operations

Async allows not blocking program execution while waiting for results.
