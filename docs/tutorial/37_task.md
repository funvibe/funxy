# Asynchronous Computations (lib/task)

`lib/task` provides Task — an abstraction for asynchronous computations (Futures/Promises).

## Import

```rust
import "lib/task" (*)
```

## Task Type

`Task<T>` represents an asynchronous computation that:
- Can complete successfully with a value of type T
- Can complete with an error (String)
- Can be cancelled

## Creating Tasks

### async — start asynchronous computation

```rust
import "lib/task" (async, await)

// Runs function in a separate goroutine
task = async(fun() -> Int {
    // heavy computations
    42 * 42
})

result = await(task)
print(result)  // Ok(1764)
```

### taskResolve / taskReject — ready tasks

```rust
import "lib/task" (taskResolve, taskReject, await)

// Already completed task with value
resolved = taskResolve(42)
print(await(resolved))  // Ok(42)

// Already completed task with error
fun getRejected() -> Result<String, Int> {
    rejected = taskReject("something went wrong")
    await(rejected)
}
print(getRejected())  // Fail("something went wrong")
```

## Waiting for Result

### await — basic waiting

```rust
import "lib/task" (async, await)

task = async(fun() -> String { "fetched data" })

match await(task) {
    Ok(data) -> print("Got: ${data}")
    Fail(err) -> print("Error: ${err}")
}
```

### awaitTimeout — waiting with timeout

```rust
import "lib/task" (async, awaitTimeout)

task = async(fun() -> Int { 42 })

// Timeout in milliseconds
match awaitTimeout(task, 5000) {
    Ok(result) -> print("Got result: ${result}")
    Fail("timeout") -> print("Task took too long!")
    Fail(err) -> print("Error: ${err}")
}
```

## Multiple Tasks

### awaitAll — wait for all

```rust
import "lib/task" (async, awaitAll)

tasks = [
    async(fun() -> Int { 1 }),
    async(fun() -> Int { 2 }),
    async(fun() -> Int { 3 })
]

match awaitAll(tasks) {
    Ok(results) -> {
        // results = [1, 2, 3]
        print(results)
    }
    Fail(err) -> print("One task failed: ${err}")
}
```

### awaitAny — first successful

```rust
import "lib/task" (async, awaitAny)

// Returns first successful result, ignoring errors
tasks = [
    async(fun() -> String { "server1" }),
    async(fun() -> String { "server2" }),
    async(fun() -> String { "server3" })
]

match awaitAny(tasks) {
    Ok(data) -> print("Got data: ${data}")
    Fail(_) -> print("All servers failed")
}
```

### awaitFirst — first completed

```rust
import "lib/task" (async, awaitFirst)

tasks = [
    async(fun() -> Int { 1 }),
    async(fun() -> Int { 2 })
]

// Returns first completed result (success or error)
match awaitFirst(tasks) {
    Ok(v) -> print("First completed with: ${v}")
    Fail(e) -> print("First completed with error: ${e}")
}
```

### Variants with Timeout

```rust
import "lib/task" (async, awaitAllTimeout, awaitAnyTimeout, awaitFirstTimeout)

tasks = [
    async(fun() -> Int { 1 }),
    async(fun() -> Int { 2 })
]

// awaitAllTimeout — all with timeout
print(awaitAllTimeout(tasks, 10000))

// awaitAnyTimeout — first successful with timeout
// print(awaitAnyTimeout(tasks, 5000))

// awaitFirstTimeout — first completed with timeout
// print(awaitFirstTimeout(tasks, 5000))
```

## Task Management

### taskIsDone — check completion

```rust
import "lib/task" (async, taskIsDone)

task = async(fun() -> Int { 42 })

// Non-blocking check
if taskIsDone(task) { print("Task finished") }
else { print("Still running...") }
```

### taskCancel — cancellation

```rust
import "lib/task" (async, taskCancel, taskIsCancelled)

task = async(fun() -> Int {
    // long operation
    42
})

// Cancel (if task hasn't started yet)
taskCancel(task)

if taskIsCancelled(task) { print("Task was cancelled") }
```

## Goroutine Pool

By default, maximum 1000 parallel tasks. Can be configured:

```rust
import "lib/task" (taskGetGlobalPool, taskSetGlobalPool)

// Get current limit
poolLimit = taskGetGlobalPool()
print("Current limit: ${poolLimit}")

// Set new limit
taskSetGlobalPool(2000)
```

## Combinators

### taskMap — result transformation

```rust
import "lib/task" (async, await, taskMap)

task = async(fun() -> Int { 10 })
doubled = taskMap(task, fun(x) -> Int { x * 2 })

match await(doubled) {
    Ok(v) -> print(v)  // 20
    Fail(_) -> print("failed")
}
```

### taskFlatMap — task chain

```rust
import "lib/task" (async, await, taskFlatMap)

type User = { id: Int, name: String }
type Post = { title: String }

fun fetchUser(userId: Int) {
    async(fun() -> User { { id: userId, name: "Alice" } })
}

fun fetchPosts(user: User) {
    async(fun() -> List<Post> { [{ title: "Post 1" }] })
}

// Chain: first user, then their posts
result = taskFlatMap(fetchUser(1), fetchPosts)

match await(result) {
    Ok(posts) -> print(posts)
    Fail(err) -> print("Error: ${err}")
}
```

### taskCatch — error handling

```rust
import "lib/task" (async, await, taskCatch)

riskyTask = async(fun() -> Int {
    if true { panic("failure") } else { 42 }
})

// Recovery on error
safeTask = taskCatch(riskyTask, fun(err) -> Int {
    0  // default value
})

match await(safeTask) {
    Ok(v) -> print("Got: " ++ show(v))
    Fail(e) -> print("Handler failed: " ++ e)
}
```

**Important:** If the handler function inside `taskCatch` itself calls `panic`, the result will be `Fail`.

## Usage Patterns

### Parallel Processing

```rust
import "lib/task" (async, awaitAll)
import "lib/list" (map)

// Parallel processing of list
items = [1, 2, 3]
tasks = map(fun(item) -> async(fun() -> Int { item * 2 }), items)
result = awaitAll(tasks)
print(result)  // Ok([2, 4, 6])
```

### Worker Pool

```rust
import "lib/task" (async, awaitAll, taskSetGlobalPool)
import "lib/list" (map)

// Processing task queue with parallelism limit
queue = [1, 2, 3, 4, 5]
taskSetGlobalPool(2)  // Maximum 2 parallel tasks

tasks = map(fun(job) -> async(fun() -> Int { job * 10 }), queue)
result = awaitAll(tasks)
print(result)  // Ok([10, 20, 30, 40, 50])
```

### Race with Timeout

```rust
import "lib/task" (async, await, awaitTimeout)

// Fetch with fallback
fun fetchWithFallback() -> Result<String, String> {
    primary = async(fun() -> String { "primary data" })
    
    match awaitTimeout(primary, 1000) {
        Ok(data) -> Ok(data)
        Fail(_) -> {
            // Primary failed or timed out, try fallback
            fallback = async(fun() -> String { "fallback data" })
            await(fallback)
        }
    }
}

print(fetchWithFallback())  // Ok("primary data")
```

### Pipeline with Combinators

```rust
import "lib/task" (taskResolve, taskMap, taskFlatMap, taskCatch, async, await)

// Pipeline with combinators
t1 = taskResolve(1)
t2 = taskMap(t1, fun(x) -> Int { x + 1 })      // 2
t3 = taskMap(t2, fun(x) -> Int { x * 3 })      // 6
t4 = taskFlatMap(t3, fun(x) { async(fun() -> Int { x + 10 }) })  // 16
t5 = taskCatch(t4, fun(err) -> Int { 0 })       // fallback
result = await(t5)

match result {
    Ok(v) -> print("Result: ${v}")  // 16
    Fail(e) -> print("Error: ${e}")
}
```

## Important Notes

1. **await returns Result** — always check result through match or ?? operator

2. **panic is caught** — if function inside async calls panic, it will be Fail in Result

3. **taskCancel** — cancellation works only for tasks that haven't started executing yet

4. **Global pool** — protects against creating too many goroutines (taskSetGlobalPool/taskGetGlobalPool)

5. **Combinators don't block** — taskMap, taskFlatMap, taskCatch return new Task immediately
