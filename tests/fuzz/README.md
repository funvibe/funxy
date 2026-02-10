# Fuzz Testing Infrastructure for Funxy

This directory contains the infrastructure for fuzz testing the Funxy language toolchain.
It uses Go's native fuzzing support (available in Go 1.18+).

## Directory Structure

- `targets/`: Contains the fuzz targets (entry points for the fuzzer).
  - `parser_fuzz.go`: Fuzzes the Parser.
  - `typechecker_fuzz.go`: Fuzzes the Type Checker (Analyzer).
  - `compiler_fuzz.go`: Fuzzes the Compiler.
- `generators/`: Contains logic for generating random Funxy code (for structure-aware fuzzing).
- `mutator/`: Contains logic for mutating ASTs (for mutation-based fuzzing).
- `corpus/`: Directory where the fuzzer stores interesting inputs (automatically managed).

## Version Control

You **should commit** the `tests/fuzz/targets/testdata/` directory to git.

This directory contains the fuzzing corpus (inputs that cover interesting code paths or caused crashes).
Go's testing framework automatically runs these inputs as regression tests when you run `go test ./tests/fuzz/targets`.
Committing them ensures that:
1.  Bugs found by the fuzzer (like the ones fixed in `analyzer` and `evaluator`) are permanently regression-tested.
2.  Future fuzzing runs start with a rich corpus, making them more effective.

## How to Run Fuzz Tests

You can run fuzz tests using the standard `go test` command with the `-fuzz` flag.

### Fuzzing the Parser

```bash
go test -fuzz=FuzzParser ./tests/fuzz/targets
```

### Fuzzing the Type Checker

```bash
go test -fuzz=FuzzTypeChecker ./tests/fuzz/targets
```

### Fuzzing the Compiler

```bash
go test -fuzz=FuzzCompiler ./tests/fuzz/targets
```

### Fuzzing the Tree Walk Interpreter

To fuzz the Tree Walk backend instead of the VM compiler, use the `-tree` flag:

```bash
go test -fuzz=FuzzCompiler ./tests/fuzz/targets -args -tree
```

### Differential Fuzzing (VM vs TreeWalk)

To compare the execution results of the VM and TreeWalk backends (finding semantic bugs):

```bash
go test -fuzz=FuzzDifferential ./tests/fuzz/targets
```

### Round-Trip Fuzzing (Parser -> Printer -> Parser)

To verify that the pretty printer produces valid code that can be re-parsed (idempotency check):

```bash
go test -fuzz=FuzzRoundTrip ./tests/fuzz/targets
```

### Stress Fuzzing (Resource Exhaustion)

To test the system's resilience to resource exhaustion (deep recursion, large data structures, infinite loops):

```bash
go test -fuzz=FuzzStress ./tests/fuzz/targets
```

### Kind Checker Fuzzing (Complex Types and Traits)

To test the kind checker with complex type and trait declarations, including higher-kinded types and nested generics:

```bash
go test -fuzz=FuzzKindChecker ./tests/fuzz/targets
```

### Row Polymorphism Fuzzing

To test the row polymorphism implementation (record unification, extension, and subtyping) against chained operations, recursive structures, and conflicting types:

```bash
go test -fuzz=FuzzRowPolymorphism ./tests/fuzz/targets
```

### Mutation-based Fuzzing

To test the parser and printer robustness against mutated (potentially invalid) ASTs. This uses a custom mutator to modify valid programs from the corpus.

```bash
go test -fuzz=FuzzMutation ./tests/fuzz/targets
```

### Standard Library Fuzzing

To test the parser's handling of standard library calls (List, Map, String operations) with random arguments.

```bash
go test -fuzz=FuzzStdLib ./tests/fuzz/targets
```

### Module System Fuzzing

To stress test the module loader by generating random directory structures with imports, cycles, and re-exports.

```bash
go test -fuzz=FuzzModules ./tests/fuzz/targets
```

### Formatter Fuzzing (Idempotency)

To verify that the pretty printer is idempotent: `print(parse(print(ast))) == print(ast)`.

```bash
go test -fuzz=FuzzFormatter ./tests/fuzz/targets
```

### VM Bytecode Fuzzing
To fuzz the VM directly with random bytecode (valid and invalid) to ensure robustness against crashes (e.g., truncated bytecode, infinite loops).

```bash
go test -fuzz=FuzzVM ./tests/fuzz/targets
```

### LSP Fuzzing
To fuzz the Language Server Protocol implementation by spawning the server process and sending random JSON-RPC messages.
```bash
go test -fuzz=FuzzLSP ./tests/fuzz/targets
```

### Async & Concurrency Fuzzing
To stress test the async/await machinery, task scheduling, and concurrency primitives.
```bash
go test -fuzz=FuzzAsync ./tests/fuzz/targets
```

## Finding Edge Cases

To effectively find edge cases in the Funxy compiler, use these strategies:

### 1. Run Multiple Fuzz Targets (Recommended: use the run script)

Use the provided script that runs tests in controlled batches:

```bash
# Run all fuzz targets (default: 180s each)
./tests/fuzz/run_all.sh

# Quick smoke test (60s each)
./tests/fuzz/run_all.sh 60s

# Overnight run (1 hour each)
./tests/fuzz/run_all.sh 1h
```

The script splits tests into 5 batches and limits `GOMAXPROCS` per test so the
total worker count stays close to available CPU cores. This prevents:
- Baseline coverage gathering from stalling (tests not starting)
- Tests far exceeding their `-fuzztime` budget
- Subprocess-based tests (FuzzLSP) hanging under CPU contention

**⚠️ Do NOT run all fuzz tests as bare background jobs:**

```bash
# BAD: 14 tests × GOMAXPROCS workers = massive CPU contention
go test -fuzz=FuzzParser -fuzztime=180s ./tests/fuzz/targets &
go test -fuzz=FuzzTypeChecker -fuzztime=180s ./tests/fuzz/targets &
go test -fuzz=FuzzCompiler -fuzztime=180s ./tests/fuzz/targets &
go test -fuzz=FuzzRowPolymorphism -fuzztime=180s ./tests/fuzz/targets &
go test -fuzz=FuzzKindChecker -fuzztime=180s ./tests/fuzz/targets &
go test -fuzz=FuzzDifferential -fuzztime=180s ./tests/fuzz/targets &
go test -fuzz=FuzzRoundTrip -fuzztime=180s ./tests/fuzz/targets &
go test -fuzz=FuzzStress -fuzztime=180s ./tests/fuzz/targets &
go test -fuzz=FuzzMutation -fuzztime=180s ./tests/fuzz/targets &
go test -fuzz=FuzzStdLib -fuzztime=180s ./tests/fuzz/targets &
go test -fuzz=FuzzModules -fuzztime=180s ./tests/fuzz/targets &
go test -fuzz=FuzzFormatter -fuzztime=180s ./tests/fuzz/targets &
go test -fuzz=FuzzVM -fuzztime=180s ./tests/fuzz/targets &
go test -fuzz=FuzzAsync -fuzztime=180s ./tests/fuzz/targets &
go test -fuzz=FuzzLSP -fuzztime=180s ./tests/fuzz/targets &
wait
```

If you must run individual tests in parallel, limit `GOMAXPROCS`:

```bash
# OK: Controlled parallelism (e.g., 3 tests on 12-core machine)
GOMAXPROCS=4 go test -fuzz=FuzzParser -fuzztime=180s ./tests/fuzz/targets &
GOMAXPROCS=4 go test -fuzz=FuzzTypeChecker -fuzztime=180s ./tests/fuzz/targets &
GOMAXPROCS=4 go test -fuzz=FuzzCompiler -fuzztime=180s ./tests/fuzz/targets &
wait
```

### 2. Focus on Specific Areas

If you're working on a particular component, focus on the relevant fuzz target:

- **Parser changes**: Use `FuzzParser` and `FuzzRoundTrip`
- **Type system changes**: Use `FuzzTypeChecker`, `FuzzKindChecker`, and `FuzzRowPolymorphism`
- **Backend changes**: Use `FuzzDifferential`, `FuzzCompiler`, and `FuzzVM`
- **Performance changes**: Use `FuzzStress`

### 3. Use Extended Fuzzing Sessions

For thorough edge case discovery, run longer fuzzing sessions:

```bash
# Run for 1 hour (good for overnight testing)
go test -fuzz=FuzzDifferential -fuzztime=1h ./tests/fuzz/targets

# Run indefinitely (stop manually with Ctrl+C)
go test -fuzz=FuzzKindChecker ./tests/fuzz/targets
```

### 4. Analyze Crash Reports

When a fuzzer finds a crash, it saves the input to `testdata/fuzz/FuzzName/`:

```bash
# View the crashing input
cat tests/fuzz/targets/testdata/fuzz/FuzzDifferential/xxxxxx

# Reproduce the crash with the specific input
go test -run=FuzzDifferential/xxxxxx ./tests/fuzz/targets
```

### 5. Use Custom Seed Corpus

Add specific edge cases to the seed corpus to guide the fuzzer:

```bash
# Add interesting inputs to the corpus
echo "type alias Complex<A, B> : * -> * -> * = Map<A, List<Option<B>>>" >> tests/fuzz/targets/testdata/fuzz/FuzzKindChecker/seed_corpus
```

**Note:** Fuzz tests run indefinitely by default. Use `-fuzztime` to limit the duration.

## Options

- `-fuzztime`: Duration to run the fuzzer (e.g., `-fuzztime=10s`, `-fuzztime=1h`).
- `-parallel`: Number of parallel workers (defaults to GOMAXPROCS).

## CI Integration

To integrate with CI, you can run the fuzzer for a limited time to catch regressions:

```bash
go test -fuzz=FuzzParser -fuzztime=30s ./tests/fuzz/targets
```

## Extending

To add a new fuzz target, create a new function in `tests/fuzz/targets/` with the signature `func FuzzName(f *testing.F)`.
Add seed corpus using `f.Add(...)` and then call `f.Fuzz(...)`.

## Unit Tests for Fuzzing Infrastructure

The fuzzing infrastructure itself (generators, mutators) has unit tests to ensure correctness.

```bash
# Run unit tests for generators
go test -v ./tests/fuzz/generators

# Run unit tests for mutators
go test -v ./tests/fuzz/mutator
```
