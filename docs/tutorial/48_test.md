# Testing in Funxy (`lib/test`)

Funxy includes a built-in testing framework for unit testing (with mocking capabilities) and integration testing (with VM clustering and load generation).

The testing framework is available via the `lib/test` virtual package.

## Running Tests

To run tests, use the Funxy CLI:

```bash
funxy test ./tests
```

The CLI will recursively find and execute all files ending in `_test.lang`, `_test.funxy`, or `_test.fx`.

## Basic Structure and Assertions

A test is defined using the `testRun` function, which takes a name and a closure.

```rust
import "lib/test" (*)

testRun("basic arithmetic", \ -> {
    result = 2 + 2
    assertEquals(4, result)
})

testRun("string concatenation", \ -> {
    greeting = "Hello, " ++ "World!"
    assertEquals("Hello, World!", greeting)
    assertNotEquals("Hi", greeting)
})
```

### Available Assertions

*   `assert(condition: Bool)` / `assertTrue(condition: Bool)`
*   `assertFalse(condition: Bool)`
*   `assertEquals(expected: A, actual: A)`
*   `assertNotEquals(notExpected: A, actual: A)`
*   `assertOk(result: Result<E, A>)`
*   `assertFail(result: Result<E, A>)`
*   `assertSome(option: Option<A>)`
*   `assertNone(option: Option<A>)`

---

## Unit Testing & Mocking

`lib/test` provides built-in mocking for standard side effects.

### 1. HTTP Mocking

You can intercept HTTP requests made via `lib/http` using `mockHttp`. Mocks are matched by URL prefix or exact match.

```rust
import "lib/test" (*)
import "lib/http"

testRun("mock http request", \ -> {
    // Setup the mock
    mockHttp("https://api.example.com/data", {
        status: 200,
        body: "{\"mocked\": true}",
        headers: {"Content-Type": "application/json"}
    })

    // The actual code under test
    res = http.get("https://api.example.com/data")

    assertOk(res)
    match res {
        Ok(response) -> assertEquals(200, response.status)
        _ -> assert(false)
    }

    // Clean up (optional, mocks are automatically cleared between testRuns)
    mockHttpOff()
})
```

### 2. File System and Environment Mocking

You can mock file reads and environment variables:

```rust
import "lib/test" (*)
import "lib/io"
import "lib/os"

testRun("mock file and env", \ -> {
    // Setup mocks
    mockFile("/etc/config.json", Ok("{\"port\": 8080}"))
    mockEnv("API_KEY", "secret-test-key")

    // The function being tested:
    // fun loadConfig() -> Result<String, Record> {
    //     key = os.getEnv("API_KEY")
    //     if isFail(key) { return Fail("Missing API_KEY") }
    //     return io.readTextFile("/etc/config.json") >>= json.parse
    // }

    // Test the function
    res = loadConfig()

    assertOk(res)
    match res {
        Ok(cfg) -> assertEquals(8080, cfg.port)
        Fail(_) -> assert(false)
    }
})
```

### 3. Funxy VMM Mocking (RPC & Mailbox)

You can mock inter-VM communication to test a microservice in isolation.

**Mocking RPC (`lib/rpc`):**

```rust
import "lib/test" (*)
import "lib/rpc"

testRun("mock rpc call", \ -> {
    // Intercept RPC calls to "auth_service" calling the "verifyToken" method
    mockRpc("auth_service", "verifyToken", \args -> {
        match args[0] {
            "valid_token" -> Ok({ user_id: 123, role: "admin" })
            _ -> Fail("invalid token")
        }
    })

    // The function being tested:
    // fun processOrder(token: String, orderId: Int) {
    //     authRes = rpc.callWait("auth_service", "verifyToken", [token], 1000)
    //     match authRes {
    //         Ok(user) -> if user.role == "admin" { Ok("processed") } else { Fail("forbidden") }
    //         Fail(e) -> Fail(e)
    //     }
    // }

    // Test successful path
    assertOk(processOrder("valid_token", 999))

    // Test failure path
    assertFail(processOrder("bad_token", 999))
})
```

**Mocking Mailbox (`lib/mailbox`):**

```rust
import "lib/test" (*)
import "lib/mailbox"

testRun("mock mailbox send", \ -> {
    // Intercept mailbox messages sent to "logger_vm"
    mockMailboxSend("logger_vm", \msg -> {
        assertEquals("log_event", msg.type)
        assertEquals("user_signup", msg.payload.event_name)
        Ok(unit)
    })

    // The function being tested:
    // fun trackSignup(userId: Int) {
    //     mailbox.send("logger_vm", {
    //         type: "log_event",
    //         payload: { event_name: "user_signup", uid: userId }
    //     })
    //     // ... rest of business logic ...
    //     Ok(unit)
    // }

    res = trackSignup(42)
    assertOk(res)
})
```

**Mocking Supervisor Events (`lib/vmm.receiveEventWait`):**

```rust
import "lib/test" (*)
import "lib/vmm" (receiveEventWait)

testRun("mock supervisor event", \ -> {
    mockSupervisorEvent(\timeoutMs -> {
        { type: "exit", vmId: "worker_1", status: "stopped", seenTimeout: timeoutMs }
    })

    evt = receiveEventWait(500)
    assertEquals("worker_1", evt.vmId)
    assertEquals(500, evt.seenTimeout)

    // Optional cleanup inside the same test block
    mockSupervisorEventOff()
})
```

---

## Integration Testing (VMM Cluster)

For integration testing, you can spin up real background VMs directly from your test script.

### Spawning Real VMs (`testSpawnVM` / `testSpawnVMGroup`)

`testSpawnVM` works like `vmm.spawnVM`. The Test Runner automatically tracks and kills all spawned VMs when the test finishes.

`testSpawnVMGroup` allows you to spawn multiple identical instances of a VM simultaneously, returning a list of IDs.

```rust
import "lib/test" (*)
import "lib/rpc"

testRun("integration: real vm rpc", \ -> {
    // 1. Spawn a real worker VM from a file
    spawnRes = testSpawnVM("./tests/e2e/auth_service.lang", {
        capabilities: ["lib/mailbox", "lib/rpc"]
    })
    assertOk(spawnRes)

    // Alternatively, spawn multiple identical workers
    groupRes = testSpawnVMGroup("./tests/e2e/auth_service.lang", {
        group: "auth_workers",
        capabilities: ["lib/mailbox", "lib/rpc"]
    }, 3)
    assertOk(groupRes)

    match spawnRes {
        Ok(vm_id) -> {
            // 2. Make a real RPC call to the running VM
            // The auth_service.lang has a `fun login(user, pass)`
            res = rpc.callWait(vm_id, "login", ["admin", "secret123"], 1000)

            assertOk(res)
            match res {
                Ok(token) -> assertNotEquals("", token)
                _ -> assert(false)
            }
        }
        _ -> assert(false)
    }
    // 3. VM is automatically killed here
})
```

You can also spawn multiple VMs in a group and test group RPC (Round Robin):

```rust
import "lib/test" (*)
import "lib/rpc"

testRun("integration: group rpc", \ -> {
    // 1. Spawn multiple VMs in the same group
    testSpawnVM("./tests/e2e/worker.lang", { name: "w1", group: "backend", capabilities: ["lib/rpc"] })
    testSpawnVM("./tests/e2e/worker.lang", { name: "w2", group: "backend", capabilities: ["lib/rpc"] })

    // 2. Make RPC calls to the group. The hypervisor will round-robin between w1 and w2.
    res1 = rpc.callWaitGroup("backend", "process", "data1", 1000)
    res2 = rpc.callWaitGroup("backend", "process", "data2", 1000)

    assertOk(res1)
    assertOk(res2)
})
```

### Load Testing (`testSpawnLoad`)

You can use the built-in load generator `testSpawnLoad` to generate background traffic.

Supported load types:
*   `"cpu"`: Mathematical loops.
*   `"memory"`: High allocation rates.
*   `"mailbox"`: Asynchronous message spam to a target VM.

```rust
import "lib/test" (*)

testRun("stress test worker", \ -> {
    spawnRes = testSpawnVM("./tests/e2e/dummy_worker.lang", {
        capabilities: ["lib/mailbox", "lib/rpc"]
    })

    match spawnRes {
        Ok(vm_id) -> {
            // Generate 5000 messages per second across 4 threads for 2 seconds
            loadRes = testSpawnLoad({
                type: "mailbox",
                target_vm: vm_id,
                duration: 2000,
                rps: 5000,
                threads: 4
            })
            assertOk(loadRes)

            // The load runs asynchronously.
            // Spin up CPU and Memory load in parallel
            testSpawnLoad({
                type: "cpu",
                duration: 2000,
                threads: 2
            })
            
            testSpawnLoad({
                type: "memory",
                duration: 2000,
                threads: 1
            })
        }
        _ -> assert(false)
    }
})
```

Active load generators are automatically cancelled and cleaned up when the `testRun` block exits.
