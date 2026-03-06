# Funxy Actor Benchmarks (Savina-style)

This directory contains actor-oriented benchmark scenarios for Funxy VMM.
The suite is inspired by Savina-style workloads (message passing, RPC round-trips,
ring/token passing, and fan-out/fan-in patterns).

## Files

- `suite.lang` - benchmark runner and reporting
- `rpc_echo_worker.lang` - RPC echo worker
- `ring_worker.lang` - ring forwarding worker
- `mailbox_responder_worker.lang` - mailbox request/reply worker

## Run

```bash
funxy vmm examples/savina/suite.lang
```

## Flags

- `--scenario` (default: `all`)
  - `all`
  - `rpc_roundtrip`
  - `rpc_group_roundtrip`
  - `rpc_fanout_fanin`
  - `ring_token`
  - `mailbox_request_reply`
- `--messages` (default: `5000`) - request count for RPC/mailbox scenarios
- `--actors` (default: `sysCPUCount()`) - actor count for group/ring scenarios
- `--hops` (default: `2000`) - token hops for `ring_token`
- `--warmup` (default: `500`) - warmup iterations before measurement
- `--format` (default: `table`) - output format: `table` or `csv`
- `--spinner` (default: `true`) - show spinner while scenarios are running

## Output

By default the runner prints a formatted terminal table (`lib/term`).

Use CSV mode when needed:

```bash
funxy vmm examples/savina/suite.lang --format csv
```

CSV columns:

`scenario,ops,ok,errors,elapsed_ms,ops_per_sec,ok_ops_per_sec,error_pct`

Table headers use short labels:
- `err %`
- `t ms`
- `ops/s`
- `ok ops/s`

Notes:
- `ops_per_sec` is attempt throughput and can be numerically greater than `ops` when elapsed time is less than 1 second.
- `ok_ops_per_sec` is success-only throughput.
