# Time (lib/time)

The `lib/time` module provides functions for working with time.

```rust
import "lib/time" (*)
```

## Functions

### time

```rust
timeNow() -> Int
```

Returns Unix timestamp in seconds (time since January 1, 1970).

```rust
import "lib/time" (timeNow)

now = timeNow()
print(now)  // 1701234567
```

### clockNs and clockMs

```rust
clockNs() -> Int   // nanoseconds
clockMs() -> Int   // milliseconds
```

Monotonic clocks for measuring time. Not affected by system time changes.

```rust
import "lib/time" (clockMs)

start = clockMs()
// ... code ...
end = clockMs()
elapsed = end - start
print("Elapsed: ${elapsed} ms")
```

### sleep and sleepMs

```rust
sleep(seconds: Int) -> Nil
sleepMs(ms: Int) -> Nil
```

Pauses program execution.

```rust
import "lib/time" (sleep, sleepMs)

sleepMs(100)  // pause 100 ms
sleep(1)      // pause 1 second
```

## Practical Examples

### Function Benchmark

```rust
import "lib/list" (range)
import "lib/time" (clockMs)

fun benchmark(f: () -> Nil, iterations: Int) -> Int {
    start = clockMs()
    for i in range(0, iterations) {
        f()
    }
    end = clockMs()
    end - start
}

fun heavyWork() {
    // ... some work ...
}

elapsed = benchmark(heavyWork, 1000)
print("1000 iterations: ${elapsed} ms")
```

### Timeout

```rust
import "lib/time" (clockMs, sleepMs)

fun waitFor(condition: () -> Bool, timeoutMs: Int) -> Bool {
    start = clockMs()
    for !condition() {
        if clockMs() - start > timeoutMs {
            break false
        }
        sleepMs(10)
        true
    }
}
```

### Simple Profiler

```rust
import "lib/time" (clockNs)

fun timed<t>(label: String, f: () -> t) -> t {
    start = clockNs()
    result = f()
    elapsed = clockNs() - start
    print("${label}: ${elapsed / 1000000} ms")
    result
}

// Usage
result = timed("computation", fun() -> {
    // ... heavy computations ...
    42
})
```

## Difference between timeNow() and clockMs()/clockNs()

| Function | Type | Changes when system time changes |
|---------|-----|----------------------------------------|
| `timeNow()` | Wall clock | Yes |
| `clockMs()` | Monotonic | No |
| `clockNs()` | Monotonic | No |

**Rule**: use `clockMs()`/`clockNs()` for measuring intervals, `timeNow()` for working with dates.

## Summary

| Function | Type | Description |
|---------|-----|----------|
| `time` | `() -> Int` | Unix timestamp (seconds) |
| `clockNs` | `() -> Int` | Monotonic nanoseconds |
| `clockMs` | `() -> Int` | Monotonic milliseconds |
| `sleep` | `(Int) -> Nil` | Pause in seconds |
| `sleepMs` | `(Int) -> Nil` | Pause in milliseconds |
