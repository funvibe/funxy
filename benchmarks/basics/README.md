# Funxy Architecture Proofs

This directory contains standalone, runnable examples that prove the architectural claims made in `docs/BASICS.md`. These aren't just unit tests; they are designed to be run by users to see the engine's behavior and performance characteristics firsthand.

## 1. Zero-Copy and Immutable Structural Sharing
**Proves:** Absence of GIL, safe concurrency, and zero-copy VM boundaries.
**File:** `01_zero_copy.lang`
**How to run:**
```bash
funxy vmm examples/basics/01_zero_copy.lang
```
**What happens:** The Supervisor generates a massive 500,000-element immutable `Map` and passes it to an intermediate Worker VMs (the caller) which then forwards it to another Worker VM (the target) via RPC. It measures the overhead of caller-to-target RPC with `callWait` (safe) and `callWaitFast` (unsafe).
**Why it matters:** The benchmark shows the overhead of the O(N) security check (`CheckSerializable`) in the hypervisor compared to raw zero-copy transfer. Because *no memory is copied*, the VMM simply passes a pointer across the isolation boundary. For 500,000 elements, skipping the check saves significant CPU cycles, leaving only the baseline Go runtime overhead (channels/dispatch). Thanks to structural sharing, the Worker's mutation creates a new map node without affecting the Supervisor's original map, proving thread-safety without locks.

## 2. Monomorphization
**Proves:** Generic functions have no runtime overhead in the hot path.
**File:** `02_monomorphization.lang`
**How to run:**
```bash
funxy examples/basics/02_monomorphization.lang
```
**What happens:** Runs 5,000,000 iterations of a math operation using a strictly typed function (`addInt`) and a highly abstract generic function with typeclasses (`addGen<t: Numeric>`).
**Why it matters:** Dynamic `addDyn` is actually the fastest here because it compiles to a raw `OP_ADD` instruction without any parameter type-checking overhead, leaning entirely on the Go runtime's fast type switch. Strictly typed `addInt` includes safety type assertions. Generic `addGen` is successfully monomorphized to `addInt`, but has the additional overhead of passing the implicit typeclass dictionary (like in Haskell), not from dynamic dispatch. This proves the generic function specializes at compile time to avoid method lookup.

## 3. Lightweight Async and VM Forking
**Proves:** Cheap interpreter cloning and highly concurrent `lib/task` limits.
**File:** `03_async_tasks.lang`
**How to run:**
```bash
funxy examples/basics/03_async_tasks.lang
```
**What happens:** Spawns 10,000 concurrent asynchronous tasks (`async`) that capture a shared lexical scope, and waits for all of them to complete via `awaitAll`.
**Why it matters:** The 10,000 tasks are created and completed in under a second with minimal memory overhead. Since the Funxy runtime state is immutable, creating a new execution context simply involves copying a few pointers rather than deep-copying memory.

## 4. Hot Reload and State Handoff
**Proves:** The Supervisor/Worker architecture enables zero-downtime upgrades without losing in-flight state.
**Directory:** `04_hot_reload/`
**How to run:**
```bash
funxy vmm examples/basics/04_hot_reload/supervisor.lang
```
**What happens:**
1. Supervisor spawns `worker_v1`, which starts counting up.
2. Supervisor intercepts and gracefully stops `v1`, calling its `onTerminate` hook to extract its internal state (the current count).
3. Supervisor instantly spawns `worker_v2`, passing the extracted state.
4. `v2` initializes, upgrades the state format, and continues counting seamlessly from where `v1` left off.
**Why it matters:** This demonstrates the Erlang/OTP-inspired VM lifecycle. State is safely decoupled from code logic, allowing binary updates or script hot-reloads on live production clusters without losing user data or dropping connections.

## 5. List Comprehension Performance
**Proves:** List comprehensions use optimized transient builders to avoid O(N^2) copying.
**File:** `05_list_comp.lang`
**How to run:**
```bash
funxy examples/basics/05_list_comp.lang
```
**What happens:** Compares building a 500,000-element list using a loop with repeated concatenation (`++`) versus a list comprehension (`[x | x <- 1..500000]`).
**Why it matters:** Immutable lists typically require O(N) copy for appending (or O(log N) for persistent vectors), making loop-based construction slow (O(N^2)). List comprehensions in Funxy are optimized to use mutable transient builders internally, freezing the result only at the end. This results in O(N) performance, matching the speed of mutable languages while preserving immutability for the user.

## 6. Map Comprehension Performance
**Proves:** Map comprehensions use optimized transient builders for fast bulk creation.
**File:** `06_map_comp.lang`
**How to run:**
```bash
funxy examples/basics/06_map_comp.lang
```
**What happens:** Compares building a 500,000-element map using a loop with `mapPut` versus a map comprehension.
**Why it matters:** Similar to lists, repeated updates to an immutable Hash Array Mapped Trie (HAMT) involve path copying overhead (O(log N) per insert). Map comprehensions use a transient mutable hash map during construction and convert it to a persistent HAMT in a single pass (or use transient HAMT nodes), significantly outperforming the loop-based approach.
