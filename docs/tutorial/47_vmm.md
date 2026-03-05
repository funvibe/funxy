# 47. Virtual Machine Manager (VMM)

Funxy provides a powerful Virtual Machine Manager (VMM) architecture inspired by Erlang/OTP. It allows you to run multiple isolated microservice VMs inside a single Go process, orchestrating them via a master VM (the Supervisor).

This is particularly useful for building resilient systems, actor-based architectures, or safely running untrusted code.

This tutorial is the canonical practical reference for `lib/vmm`, `lib/rpc`, and `lib/mailbox`.

## The Architecture

1.  **The Go Hypervisor:** The foundation. It manages the physical execution, limits resources, routes messages, and handles actual hardware access.
2.  **The Supervisor VM:** A privileged Funxy script that acts as the control plane. It decides which VMs to start, restart, or kill.
3.  **Worker VMs:** Isolated Funxy scripts. They do the actual work, only accessing capabilities explicitly granted to them by the Supervisor.

## 1. Running via CLI (`funxy vmm`)

The easiest way to start the VMM is using the built-in CLI command. It initializes the Go Hypervisor with default capabilities and runs your script as the Supervisor VM.

```bash
# Starts the VMM with supervisor.lang as the root orchestrator
funxy vmm supervisor.lang

# You can also customize the launch with optional flags:
funxy vmm supervisor.lang \
  --pidfile /path/to/vmm.pid \
  --socket /path/to/vmm.sock \
  --metrics-port 9090 \
  --rpc-serialization auto
```

SQLite state persistence example (using `kit/sql` from supervisor):

```bash
funxy vmm examples/vmm/sql_state_supervisor.lang
```

This example stores worker state snapshots in `examples/vmm/state.sqlite3`, then respawns the worker with the saved state.

Long-running daemon variant (keeps VMM alive, periodic sqlite checkpoints):

```bash
funxy vmm examples/vmm/sql_state_supervisor_daemon.lang
```

- `--pidfile`: Where to write the host process ID (default: `.vmm.pid`).
- `--socket`: Path to the UNIX socket for admin commands (default: `/tmp/funxy_vmm.sock`).
- `--metrics-port`: Port for Prometheus metrics (default: `9090`).
- `--rpc-serialization`: Slow-path RPC serialization mode: `auto|fdf|ephemeral` (default: `auto`).

For RPC serialization mode:
- `auto` (default): use fast object path where available; for byte fallback, use stable `fdf`.
- `fdf`: always use stable `fdf` on slow byte RPC path.
- `ephemeral`: use `encoding/gob` on slow byte RPC path (fastest, but version-coupled).

While running, you can send a `SIGUSR1` signal to the host process to trigger a graceful hot-reload:
```bash
kill -SIGUSR1 $(cat .vmm.pid)
```
The Hypervisor will catch this signal and broadcast a `{ type: "hot_reload" }` event to the Supervisor script.

To gracefully shut down the entire VMM cluster (for example, via systemd or CI/CD):
```bash
kill -SIGTERM $(cat .vmm.pid)
```

### Keep VMM running after terminal closes (Linux `systemd`)

For production-like usage on Linux, run VMM as a `systemd` service.

Create `~/.config/systemd/user/funxy-vmm.service`:

```ini
[Unit]
Description=Funxy VMM Cluster
After=network.target

[Service]
Type=simple
WorkingDirectory=/home/YOU
ExecStart=/home/YOU/funxy vmm /home/YOU/examples/vmm/supervisor.lang --pidfile /home/YOU/.vmm.pid --socket /tmp/funxy_vmm.sock
Restart=always
RestartSec=2
KillSignal=SIGTERM
TimeoutStopSec=10

[Install]
WantedBy=default.target
```

Enable and start:

```bash
systemctl --user daemon-reload
systemctl --user enable --now funxy-vmm
```

Inspect status/logs:

```bash
systemctl --user status funxy-vmm
journalctl --user -u funxy-vmm -f
```

Optional (keep running after logout):

```bash
loginctl enable-linger $USER
```

### Administrator Commands
When the VMM is running, you can interact with it via the CLI using the admin socket. This allows operations like Docker/PM2 directly from your terminal:

```bash
# List all running VMs (including the supervisor)
funxy vmm ps

# See metrics (memory allocations, instructions) for a specific VM
funxy vmm stats worker_1

# Inspect a specific VM (shows metrics AND live stack trace - invaluable for debugging stuck VMs)
funxy vmm inspect worker_1

# Inspect RPC circuit breaker state/counters for a specific VM
funxy vmm circuit worker_1

# Stream live RPC trace for a VM (Ctrl+C to stop)
funxy vmm trace worker_1

# Stream live RPC trace for all VMs
funxy vmm trace
funxy vmm trace all
funxy vmm trace --all

# Show VM uptime
funxy vmm uptime worker_1

# Gracefully stop a worker (triggers onTerminate hook, 5s timeout)
funxy vmm stop worker_1

# Force kill a stuck worker immediately
funxy vmm kill worker_1

# Trigger hot-reload event (similar to SIGUSR1, but can target specific VMs)
funxy vmm reload           # Broadcast to all VMs
funxy vmm reload worker_1  # Target a specific VM
```
*(Note: If you started the VMM with a custom `--socket`, you must provide the same `--socket` argument to the admin commands).*

### VMM Dashboard (kit/vmmui)

For an interactive TUI to manage VMM clusters, use the built-in dashboard:

```bash
# Scan current directory for .vmm*.pid files
./funxy kit/vmmui .

# Scan a specific directory
./funxy kit/vmmui /path/to/project

# Custom socket path
./funxy kit/vmmui --socket /var/run/funxy_vmm.sock
./funxy kit/vmmui /var/run/funxy_vmm.sock
```

The dashboard lets you inspect VMs, view stats, and gracefully stop or kill workers.

### Monitoring & Metrics

Funxy VMM provides built-in observability features to monitor the health and performance of your VMs.

#### CLI Stats
You can view real-time metrics for a specific VM using the `stats` command:

```bash
funxy vmm stats worker_1
```

Output:
```text
Stats for worker_1:
- instructions: 4523194
- allocations: 15024
```

#### Prometheus Endpoint
For production monitoring, you can expose a Prometheus-compatible metrics endpoint using the `--metrics-port` flag:

```bash
funxy vmm supervisor.lang --metrics-port 9090
```

The VMM will start an HTTP server at `http://localhost:9090/metrics` exposing the following metrics:
*   `funxy_vm_instructions_total{vm_id="..."}`: Total instructions executed.
*   `funxy_vm_allocations_total{vm_id="..."}`: Total bytes allocated.
*   `funxy_vmm_rpc_circuit_state{vm_id="...",state="closed|open|half_open"}`: One-hot current breaker state.
*   `funxy_vmm_rpc_circuit_failures_window{vm_id="..."}`: Current failures in breaker window.
*   `funxy_vmm_rpc_circuit_fast_fail_total{vm_id="..."}`: Total fast-fail rejections due to open/half-open.
*   `funxy_vmm_rpc_circuit_transitions_total{vm_id="...",to="open|half_open|closed"}`: State transition counters.

You can scrape this endpoint with Prometheus and visualize it in Grafana to track CPU usage (rate of instructions) and memory pressure per VM.

Additionally, you can query JSON metrics for a specific VM via HTTP:
```bash
curl "http://localhost:9090/stats?id=worker_1"
```
Output:
```json
{"allocations":15024,"instructions":4523194}
```

### Accessing the Admin API from Funxy

The admin socket speaks HTTP. Use `lib/http` with the `http+unix://` URL scheme to call it from Funxy scripts:

```rust
import "lib/http" (httpGet)
import "lib/json" (jsonDecode)

socketPath = "/tmp/funxy_vmm.sock"

// List running VMs
match httpGet("http+unix://" ++ socketPath ++ ":/ps") {
    Ok(resp) -> match jsonDecode(resp.body) {
        Ok(ids) -> print("VMs: " ++ show(ids))
        Fail(_) -> ()
    }
    Fail(e) -> print("Error: " ++ e)
}

// Get stats for a VM
match httpGet("http+unix://" ++ socketPath ++ ":/stats?id=worker_1") {
    Ok(resp) -> print(resp.body)
    Fail(e) -> print("Error: " ++ e)
}

// Get RPC circuit diagnostics for a VM
match httpGet("http+unix://" ++ socketPath ++ ":/circuit?id=worker_1") {
    Ok(resp) -> print(resp.body)
    Fail(e) -> print("Error: " ++ e)
}

// Stream RPC trace for a VM (line-by-line text stream)
match httpGet("http+unix://" ++ socketPath ++ ":/trace?id=worker_1") {
    Ok(resp) -> print(resp.body)
    Fail(e) -> print("Error: " ++ e)
}
```

## 2. Setting up a Custom Host (Go)

To use the VMM, you need a Go host that registers the capabilities you want to expose to the child VMs.

```go
package main

import (
	"fmt"
	"parser/pkg/embed" // Assuming funxy is aliased here
)

func main() {
	// 1. Create a Hypervisor
	hypervisor := embed.NewHypervisor()

	// 2. Register a Capability Provider
	// This defines what happens when a VM asks for a specific capability (e.g., "db:read")
	hypervisor.RegisterCapabilityProvider(func(cap string, vm *embed.VM) error {
		if cap == "db:read" {
			// In a real app, you would inject a safe wrapper around a database connection
			// vm.Bind("db", NewSafeDbProxy(rawDb))
			fmt.Println("Capability 'db:read' granted!")
			return nil
		}
		return fmt.Errorf("unknown capability: %s", cap)
	})

	// 3. Create the Supervisor VM
	supervisorVM := embed.New()

	// 4. Inject the Hypervisor into the Supervisor VM
	// This enables the `lib/vmm` built-ins
	supervisorVM.RegisterSupervisor(hypervisor)

	// 5. Run the Supervisor script
	_, err := supervisorVM.LoadFile("supervisor.lang")
	if err != nil {
		fmt.Printf("Supervisor failed: %v\n", err)
	}
}
```

## 3. The Supervisor Script (Funxy)

The Supervisor script imports `lib/vmm` and manages the worker VMs.

```rust
// supervisor.lang
import "lib/vmm" (spawnVM, spawnVMGroup, listVMs, receiveEventWait)

print("Supervisor started.")

// Define the configuration for our worker
config = {
    name: "worker_1",          // The ID of the VM (ignored for groups)
    capabilities: ["db:read", "lib/mailbox"], // Capabilities
    limits: {                  // Optional limits
        maxMemoryMB: 128,      // Hard limit on memory allocations
        maxInstructions: 5000000, // Budget per tick / execution limit
        maxStackDepth: 100     // Prevent deep recursion
    },
    mailbox: {                 // Optional mailbox config
        capacity: 1000,
        strategy: "dropOld"
    },
    rpcCircuit: {              // Optional per-VM RPC circuit override
        failureThreshold: 3,   // default: 3
        failureWindowMs: 5000, // default: 5000
        openTimeoutMs: 2000    // default: 2000
    }
}

// Spawn a single worker VM. It will run in isolation.
match spawnVM("worker.lang", config) {
    Ok(id) -> print("Successfully spawned VM: " ++ id)
    Fail(e) -> print("Failed to spawn VM: " ++ e)
}

// Or spawn a group of identical VMs (e.g. for a web server cluster)
groupConfig = {
    group: "web_workers",
    capabilities: ["lib/http"]
}
match spawnVMGroup("web_worker.lang", groupConfig, 3) {
    Ok(ids) -> print("Spawned group: " ++ show(ids))
    Fail(e) -> print("Failed to spawn group: " ++ e)
}

print("Running VMs: " ++ show(listVMs()))

// The supervisor can block and wait for events (like crashes or normal exits)
// receiveEventWait() uses default timeout 5000ms.
print("Waiting for events...")
event = receiveEventWait()
print("Received event: " ++ show(event))
// Worker lifecycle events use event.vmId as canonical worker id.
```

## 4. The Worker Script (Funxy)

The worker is just a normal Funxy script. It runs in its own memory space and global environment.

```rust
// worker.lang
import "lib/time" (sleep)

print("[Worker] Hello from isolated VM!")

// Simulate some work
sleep(2)

print("[Worker] Work done. Exiting.")
// When this script finishes, the VM exits gracefully.
```

## Inter-VM Communication (RPC)

VMs can communicate with each other synchronously using Remote Procedure Calls (RPC). The Go Hypervisor handles the routing, pausing the caller, executing the function in the target VM, and returning the serialized result.

### 1. The Target VM (Service)

The target VM must define a function that will be called via RPC.

```rust
// service.lang
import "lib/time" (sleep)

print("[Service] Started.")

// This function will be called via RPC
fun processOrder(data) {
    print("[Service] Processing order: " ++ show(data))
    sleep(1) // Simulate work

    // Check for errors
    if mapGetOr(data, "amount", 0) <= 0 {
        return { status: "error", message: "Invalid amount" }
    }

    return { status: "success", orderId: 12345 }
}

// Prevent the VM from exiting immediately so it can receive calls
// In a real app, this might be a server loop or the supervisor handles it differently
sleep(100)
```

### 2. The Calling VM (Client)

Another VM (or the Supervisor) can call the function using `lib/rpc`.

```rust
// client.lang
import "lib/rpc" (callWait, callWaitGroup)

print("[Client] Started.")

// Call the 'processOrder' function on the VM named "billing_service"
orderData = { amount: 100, item: "Laptop" }

match callWait("billing_service", "processOrder", orderData) {  // timeoutMs defaults to 5000
    Ok(result) -> {
        print("[Client] RPC Success!")
        print("[Client] Result: " ++ show(result))
    }
    Fail(error) -> {
        print("[Client] RPC Failed: " ++ error)
    }
}

// Or call any worker in a specific group (Round Robin)
match callWaitGroup("billing_workers", "processOrder", orderData) {
    Ok(result) -> print("Group RPC Success: " ++ show(result))
    Fail(error) -> print("Group RPC Failed: " ++ error)
}
```

### Important rules for RPC:
*   **Serialization:** Arguments and return values are serialized when crossing VM boundaries.
*   **No Functions or Host Objects:** You cannot pass closures, functions, or `HostObject`s (raw Go pointers) via RPC. Only plain data (Records, Lists, Maps, Strings, Ints, etc.) is allowed.
*   **Synchronous:** `callWait` / `callWaitGroup` block until the target VM returns a result or fails.
*   **Circuit Breaker:** The hypervisor tracks per-target RPC failures. On repeated failures/timeouts, the circuit opens and calls fail fast with `Fail("CircuitOpen")` instead of waiting full timeout.
*   **Group Safety:** `callWaitGroup` skips workers with open circuit and routes to healthy workers in the same group when available.

## Supervisor Resilience Helpers (`kit/vmm`)

Funxy now provides `kit/vmm` with reusable supervisor-side resilience primitives, so you don't have to hand-roll crash-loop and poisoned-state handling.

```rust
import "kit/vmm" (supervise, defaultRestartPolicy)

policy = defaultRestartPolicy()  // maxRestarts/window/backoff/probe defaults
config = {
    name: "billing_worker",
    capabilities: ["lib/rpc", "lib/time"]
}

// Handles restart intensity/backoff and state-validation fallback internally.
supervise("./workers/billing.lang", config, None, policy)
```

Included behavior:
*   **Restart intensity/backoff:** escalates with `panic` when restart rate exceeds policy.
*   **State validation fallback:** when boot with `Some(state)` crashes immediately, retries once with `None`.

## Asynchronous Messaging (Mailbox)

For non-blocking communication or actor-model patterns, use `lib/mailbox`.

### 1. Sending Messages
```rust
import "lib/mailbox" (send)

// Fire and forget
send("worker_2", { type: "job", payload: "data" })

// Send with importance (Low, Info, Warn, Crit, System)
send("worker_2", { payload: "alert", importance: Crit })
```

### 2. Receiving Messages
```rust
import "lib/mailbox" (receiveWait)

// Block until message arrives (with timeout)
match receiveWait(5000) {
    Ok(msg) -> print("Got: " ++ show(msg.payload))
    Fail(e) -> print("Timeout or error: " ++ e)
}
```

### 3. Request-Response
```rust
import "lib/mailbox" (requestWait, reply)

// Requester
resp = requestWait("worker_2", { cmd: "ping" }, 1000)

// Responder
msg = receiveWait(5000)?
reply(msg, { result: "pong" })
```

## Persistent State (FDF format)

While the default state handoff serialization (ephemeral) is fast and perfect for zero-downtime hot-reloads within the *same* process, it is not safe to save to a database. If the Go host binary is upgraded, the internal memory representation might change.

For persisting state across host upgrades, the VMM provides the Funxy Data Format (FDF). FDF is version-agnostic and retains 100% of Funxy's type semantics (differentiating between `Tuple` and `List`, ADT variant tags, etc.).

```rust
import "lib/vmm" (serialize, deserialize)

// Save state safely to disk or database
persistentStateBytes = serialize(currentState, "fdf")
saveToDatabase(persistentStateBytes)

// Later (even after a host binary upgrade)
loadedBytes = readFromDatabase()
match deserialize(loadedBytes) {
    Ok(state) -> print("Loaded state: " ++ show(state))
    Fail(e) -> print("Failed to load: " ++ e)
}
```

## State Handoff (Hot Reloading)

Funxy supports state handoff, allowing you to hot-reload a service without losing its internal state.

The worker VM uses `getState` and `setState` from `lib/vmm` to manage its hot-reloadable state. The Supervisor coordinates the process using the `stopVM` and `spawnVM` functions.

### 1. The Worker (Stateful Service)

The worker updates its state during operation and uses `setState`. It can also retrieve it on startup using `getState`.

```rust
// stateful_worker.lang
import "lib/vmm" (getState, setState)
import "lib/time" (sleep)

// Get initial state (None on first run) or use default
initialState = getState() ?? { count: 0 }

print("[Worker] Starting with count: " ++ show(initialState.count))

// The worker does its job and updates state
state = initialState
for i in 1..5 {
    sleep(1)
    state = state.count = state.count + 1
    setState(state) // Save current state for potential handoff
    print("[Worker] Count is now: " ++ show(state.count))
}
```

### 2. The Supervisor (Performing Handoff)

The Supervisor can stop a VM gracefully, requesting its saved state, and then spawn a new version of the VM with that state.

```rust
// supervisor.lang
import "lib/vmm" (spawnVM, stopVM, receiveEventWait)

// Start v1
spawnVM("stateful_worker.lang", { name: "worker" })

// Wait for a bit, then decide to upgrade
import "lib/time" (sleep)
sleep(2)

print("[Supervisor] Upgrading worker to v2...")

// Stop the VM gracefully, requesting its state.
// We specify a timeout of 2000ms. If the VM doesn't stop, it's hard-killed.
stopResult = stopVM("worker", Some({ saveState: true, timeoutMs: 2000 }))

match stopResult {
    Ok(savedState) -> {
        print("[Supervisor] Captured state: " ++ show(savedState))

        // Spawn the new version (v2), passing the captured state
        spawnVM("stateful_worker_v2.lang", { name: "worker" }, savedState)
        print("[Supervisor] Upgrade complete!")
    }
    Fail(e) -> print("[Supervisor] Failed to stop VM: " ++ e)
}
```

This ensures zero-downtime upgrades and resilient service management without losing in-flight data.

## VM Lifecycle Hooks

Worker VMs can define special functions (hooks) in their global scope that the Hypervisor will call automatically at specific lifecycle events.

| Hook | When it is called | Signature | Purpose |
|------|-------------------|-----------|---------|
| **`onInit`** | After the main script completes execution (on first boot or hot-reload) | `onInit(state: Option<a>) -> a` | Initialization. On first boot, receives `None`. On Hot-Reload, receives `Some(oldState)`. Returns the initial state to be kept by the hypervisor. |
| **`onTerminate`** | During graceful shutdown (`stopVM` with `saveState: true`) | `onTerminate(currentState: a) -> a` | Graceful shutdown. Receives the current state, can finalize it, and returns the state to be serialized and passed back to the supervisor. |
| **`onError`** | On panic, resource limit exceeded, or forceful `killVM` | `onError(error: String)` | Last breath before crash. Useful for sending crash reports to the supervisor or a logging service. Executes in a fresh context with a 5-second timeout. |

### Example with Lifecycle Hooks

```rust
import "lib/vmm" (setState)
import "lib/mailbox" (send)

fun onInit(stateOpt: Option<Int>) -> Int {
    match stateOpt {
        Some(s) -> {
            print("Resuming from state: " ++ show(s))
            s
        }
        None -> {
            print("Starting fresh")
            0
        }
    }
}

fun onTerminate(currentState: Int) -> Int {
    print("Gracefully shutting down. Final state: " ++ show(currentState))
    currentState
}

fun onError(error: String) {
    print("Worker crashed! Reason: " ++ error)
    // Send crash report to the supervisor or a logging VM
    send("logger_vm", { event: "crash", error: error })
}

// Main execution finishes, hooks take over
```

## Web Server Clusters (Shared Listeners)

Funxy provides built-in support for the "Shared Listeners" pattern, commonly used in Node.js clusters. When multiple worker VMs in the same group try to start an HTTP server (`httpServe` or `httpServeAsync`) on the **same port**, Funxy automatically shares a single underlying Go `net.Listener`.

This means you can trivially spawn a pool of workers to handle high web traffic, without running into "address already in use" errors:

```rust
// supervisor.lang
import "lib/vmm" (spawnVMGroup)

// Spawn 4 identical web servers sharing port 8080
match spawnVMGroup("web_worker.lang", {
    group: "web_servers",
    capabilities: ["lib/http"]
}, 4) {
    Ok(ids) -> print("Web cluster started: " ++ show(ids))
    Fail(e) -> panic(e)
}
```

```rust
// web_worker.lang
import "lib/http" (httpServe)

print("Starting server on port 8080...")

// If another VM in this cluster is already listening on 8080,
// this VM will share the same port and handle incoming requests concurrently.
httpServe(8080, fun(req) {
    return {
        status: 200,
        body: "Hello from worker!",
        headers: [("Content-Type", "text/plain")]
    }
})
```

The Go runtime automatically load-balances incoming HTTP requests across all available workers.

## Canonical API Reference (`lib/vmm`, `lib/rpc`, `lib/mailbox`)

This section is the complete operator/developer reference. Use it as the source of truth when writing supervisor and worker code.

### `lib/vmm` (lifecycle, orchestration, state)

#### Full API

| Function | Signature | Purpose |
|---|---|---|
| `spawnVM` | `(path: String, config: Record, state?: Option<Record>) -> Result<String, String>` | Spawn one worker VM from `.lang` or `.fbc` |
| `spawnVMGroup` | `(path: String, config: Record, size: Int, state?: Option<Record>) -> Result<String, List<String>>` | Spawn N workers in one group |
| `killVM` | `(vmId: String) -> Nil` | Hard-stop VM immediately |
| `stopVM` | `(vmId: String, opts?: Option<Record>) -> Result<String, a>` | Graceful stop; optional state handoff |
| `traceOn` | `(vmId?: String) -> Nil` | Enable live RPC trace (`traceOn()` for all VMs, `traceOn("vm")` for one VM) |
| `traceOff` | `(vmId?: String) -> Nil` | Disable live RPC trace (`traceOff()` disables global/all, `traceOff("vm")` disables one VM) |
| `listVMs` | `() -> List<String>` | List currently running VM ids |
| `vmStats` | `(vmId: String) -> Map<String, Int>` | Per-VM low-level counters |
| `rpcCircuitStats` | `(vmId: String) -> Record` | Per-VM RPC circuit diagnostics (`state`, counters, thresholds) |
| `receiveEventWait` | `(timeoutMs?: Int = 5000) -> Record` | Blocking hypervisor event stream read |
| `serialize` | `(value: a, mode?: String = "ephemeral") -> Bytes` | Encode state/value to bytes |
| `deserialize` | `(bytes: Bytes) -> Result<String, a>` | Decode bytes back to value |
| `getState` | `() -> Option<a>` | Read current VM state snapshot |
| `setState` | `(value: a) -> Nil` | Update current VM state snapshot |

#### `spawnVM` config schema (recommended)

```rust
config = {
    name: "billing_worker",                   // optional explicit vm id
    group: "billing_workers",                 // optional group name
    capabilities: ["lib/rpc", "lib/time"],    // capability whitelist
    limits: {                                 // optional resource limits
        maxMemoryMB: 64,
        maxInstructions: 10000000,
        maxInstructionsPerSec: 5000000,
        maxAllocationsPerSecond: 2000000,
        maxStackDepth: 64
    },
    mailbox: {                                // optional mailbox tuning
        capacity: 1024,
        strategy: "dropOld"                   // host policy-dependent
    },
    rpcCircuit: {                             // optional per-VM RPC breaker override
        failureThreshold: 3,
        failureWindowMs: 5000,
        openTimeoutMs: 2000
    }
}
```

#### `receiveEventWait` event model

`receiveEventWait` returns records from hypervisor event stream.

- Lifecycle event shape (examples):
  - `{ type: "exit", vmId: "...", status: "stopped", seq: 123 }`
  - `{ type: "crash", vmId: "...", status: "crashed", error: "...", seq: 124 }`
  - `{ type: "vm_exit", vmId: "...", reason: "limit_exceeded", detail: "...", status: "...", seq: 125 }`
- Timeout shape:
  - `{ type: "timeout", timed_out: true }`

Notes:
- `vmId` is canonical worker identifier for lifecycle events.
- `seq` is monotonic watermark for gap detection. If `evt.seq > lastSeq + 1`, events were dropped in between.
- Timeout events are synthetic and do not carry `seq`.

#### RPC circuit diagnostics (`rpcCircuitStats`)

Use `rpcCircuitStats(vmId)` from supervisor code when you need live breaker diagnostics:
- `state`: `"closed" | "open" | "half_open"`
- `failureCount`: current failures inside window
- `fastFailTotal`: total immediate rejections due to open/half-open
- `transitionsOpenTotal` / `transitionsHalfOpenTotal` / `transitionsClosedTotal`
- `failureThreshold` / `failureWindowMs` / `openTimeoutMs` (effective config)

#### `stopVM` semantics

- `stopVM(vmId, Some({ saveState: true, timeoutMs: 2000 }))`:
  - tries graceful termination (`onTerminate`) and returns captured state in `Ok(state)` when successful.
- `saveState: false`:
  - no state capture path; still an intentional stop and lifecycle event is `type: "exit", status: "stopped"`.
- on timeout:
  - VM is forcefully terminated and `Fail(error)` is returned.

#### `serialize` modes

- `"ephemeral"` (default): optimized for same-host/same-runtime handoff.
- `"fdf"`: durable format for persistence across host upgrades.

---

### `lib/rpc` (synchronous request/response between VMs)

#### Full API

| Function | Signature | Purpose |
|---|---|---|
| `callWait` | `<a,b>(targetVmId: String, method: String, payload: a, timeoutMs?: Int = 5000) -> Result<String, b>` | Call method on specific VM |
| `callWaitGroup` | `<a,b>(group: String, method: String, payload: a, timeoutMs?: Int = 5000) -> Result<String, b>` | Call method on VM selected by round-robin in group |

#### Behavior guarantees

- Synchronous call: caller blocks until reply/error/timeout.
- VM boundary crossing serializes data (plain data only).
- Group routing is round-robin.
- Hypervisor circuit breaker is enforced per target VM.

#### Circuit breaker behavior

- States: `Closed -> Open -> HalfOpen -> Closed/Open`.
- Open state fast-fails with `Fail("CircuitOpen")`.
- `callWaitGroup` skips open-circuit workers and tries healthy workers in same group.
- If all group candidates are open/unhealthy, group call returns failure (typically `CircuitOpen`).

#### Common failure modes in `Fail(error)`

- `CircuitOpen`
- target VM not found
- method not found on target
- call timeout (`... timed out after Nms`)
- serialization/deserialization errors

#### Data contract advice

- Keep RPC payload/reply DTO-style: `Record`, `List`, `Map`, primitives.
- Avoid passing function values, closures, or host objects.
- Prefer explicit error records in business layer (`{ ok: false, code: "...", ... }`) in addition to transport `Result`.

---

### `lib/mailbox` (asynchronous actor-style messaging)

#### Full API

| Function | Signature | Purpose |
|---|---|---|
| `send` | `(targetId: String, msg: a) -> Result<String, Nil>` | Fire-and-forget send |
| `sendWait` | `(targetId: String, msg: a, timeoutMs?: Int = 5000) -> Result<String, Nil>` | Send with waiting/backpressure handling |
| `requestWait` | `(targetId: String, payload: a, timeoutMs?: Int = 5000) -> Result<String, Record>` | RPC-like request over mailbox |
| `reply` | `(originalMsg: Record, payload: a) -> Result<String, Nil>` | Reply to request message |
| `replyWait` | `(originalMsg: Record, payload: a, timeoutMs?: Int = 5000) -> Result<String, Nil>` | Reply with waiting/backpressure handling |
| `receive` | `() -> Result<String, Record>` | Receive immediately or fail if empty/closed |
| `receiveWait` | `(timeoutMs?: Int = 5000) -> Result<String, Record>` | Blocking receive with timeout |
| `receiveBy` | `(predicate: (Record) -> Bool) -> Result<String, Record>` | Selective receive |
| `receiveByWait` | `(predicate: (Record) -> Bool, timeoutMs?: Int = 5000) -> Result<String, Record>` | Selective receive with timeout |
| `peek` | `() -> Result<String, Record>` | Peek without consuming |
| `peekBy` | `(predicate: (Record) -> Bool) -> Result<String, Record>` | Selective peek without consuming |

#### Message envelope conventions (recommended)

Use explicit envelope fields in user payloads:

```rust
{
    type: "order.created",
    correlationId: "...",
    ts: 1710000000000,
    payload: { ... }
}
```

For request/reply patterns, rely on `requestWait`/`reply` metadata from original message rather than crafting your own return channel.

#### Mailbox vs RPC: when to choose

- Use `lib/rpc` when you need immediate response and strict call flow.
- Use `lib/mailbox` when you need decoupling, buffering, selective receive, and actor patterns.
- Mix both in one system: mailbox for command/event flow, RPC for critical read/compute calls.

---

### Production patterns (recommended)

#### 1) Supervisor event loop with watermark (`seq`)

```rust
lastSeq = 0

while true {
    evt = receiveEventWait(5000)
    if evt.type == "timeout" {
        ()
    } else {
        if evt.seq > lastSeq + 1 {
            // gap detected: reconcile your local model
            // e.g. compare internal state with listVMs()
            ()
        } else {
            ()
        }
        lastSeq = evt.seq
    }
}
```

#### 2) Prefer `kit/vmm` for resilient supervisors

- `supervise(...)` for restart intensity + backoff + state validation fallback.
- `superviseWithReconcile(...)` when you want built-in watermark gap callback.

#### 3) Set explicit timeouts

- Do not rely on infinite waits.
- Keep timeout values close to SLOs and expected worker latency.
- Handle `Fail("CircuitOpen")` separately from generic timeout for clearer recovery logic.

---

### Quick troubleshooting matrix

| Symptom | Likely reason | What to do |
|---|---|---|
| `spawnVM` returns `capability denied` | worker imports module not whitelisted in `capabilities` | add required capability explicitly |
| `callWait` fails with `CircuitOpen` | target repeatedly timing out/failing | inspect target health, reduce timeout pressure, rely on group fallback |
| `receiveEventWait` seems to "miss" lifecycle events | ring buffer overflow under burst | detect `seq` gaps and run reconcile |
| `stopVM(...saveState=true)` fails | `onTerminate` timeout or terminate error | increase timeout, harden `onTerminate`, keep shutdown idempotent |
| mailbox send fails intermittently | queue full/backpressure strategy | tune mailbox capacity/strategy, use `sendWait` where needed |

Treat this section as the operational contract for `lib/vmm`, `lib/rpc`, and `lib/mailbox`.

