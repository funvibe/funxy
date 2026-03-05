# kit/vmm

Supervisor utilities for resilient VMM orchestration in Funxy.

## Features

- Restart intensity and exponential backoff guard (`supervise`)
- State validation fallback for hot-reload (`spawnWithStateValidation`)

## API

- `defaultRestartPolicy() -> Record`
- `restartBackoffMs(restartsInWindow: Int, initialBackoffMs: Int, maxBackoffMs: Int) -> Int`
- `recordRestartAt(nowMs: Int, restartTimestamps: List<Int>, windowMs: Int) -> List<Int>`
- `isLifecycleEventForVm(evt: Record, vmId: String) -> Bool`
- `consumeEventWithGap(lastSeq: Int, timeoutMs: Int) -> Record` // returns `{ event, lastSeq, gap, gapCount }`
- `consumeAndReconcile(lastSeq: Int, timeoutMs: Int, reconcileFn: (Int) -> Nil) -> Record`
- `waitLifecycleEventForVmWithReconcile(vmId: String, timeoutMs: Int, lastSeq: Int, reconcileFn: (Int) -> Nil) -> Record` // returns `{ event, lastSeq }`
- `waitLifecycleEventForVm(vmId: String, timeoutMs: Int) -> Record`
- `spawnWithStateValidation(path: String, config: Record, stateOpt: Option<Record>, probeTimeoutMs: Int) -> Result<String, String>`
- `supervise(path: String, config: Record, initialStateOpt: Option<Record>, policy: Record) -> Nil`
- `superviseWithReconcile(path: String, config: Record, initialStateOpt: Option<Record>, policy: Record, reconcileFn: (Int) -> Nil) -> Nil`

`consumeEventWithGap` is a helper around `receiveEventWait` for watermark-based loss detection:
- reads one event;
- updates `lastSeq` from `event.seq` when present;
- sets `gap = true` and `gapCount > 0` when `event.seq > previousLastSeq + 1`.

`consumeAndReconcile` wraps `consumeEventWithGap` and invokes `reconcileFn(gapCount)` only when a gap is detected.

`superviseWithReconcile` is `supervise` with built-in watermark handling:
- tracks `lastSeq` across supervisor loop iterations;
- detects gaps while waiting for worker lifecycle events;
- invokes `reconcileFn(gapCount)` immediately when a gap is observed.

## Example

```rust
import "kit/vmm" (supervise, defaultRestartPolicy)

policy = {
    maxRestarts: 3,
    windowMs: 10000,
    initialBackoffMs: 300,
    maxBackoffMs: 5000,
    stateProbeMs: 500,
    eventStepTimeoutMs: 5000
}

config = {
    name: "billing_worker",
    capabilities: ["lib/rpc", "lib/time"]
}

// Runs until worker exits cleanly, or escalates with panic on crash-loop.
supervise("./workers/billing.lang", config, None, policy)
```
