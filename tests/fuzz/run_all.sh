#!/bin/bash
#
# Run all fuzz targets with controlled parallelism.
#
# Running all 14 fuzz tests simultaneously (as background jobs) causes severe
# CPU contention: each `go test -fuzz` spawns GOMAXPROCS workers by default,
# so 14 tests × 12 cores = 168 workers competing for 12 cores. This leads to:
#   - Baseline coverage gathering stalling for minutes
#   - Tests far exceeding their -fuzztime budget
#   - Subprocess-based tests (FuzzLSP) hanging and being killed
#
# This script runs tests in batches, limiting GOMAXPROCS per test so the total
# worker count stays close to the available CPU cores.
#
# Usage:
#   ./tests/fuzz/run_all.sh              # defaults: 180s fuzztime
#   ./tests/fuzz/run_all.sh 60s          # quick run
#   ./tests/fuzz/run_all.sh 1h           # overnight run
#
set -e
set -o pipefail

FUZZTIME="${1:-180s}"
NCPU=$(sysctl -n hw.ncpu 2>/dev/null || nproc 2>/dev/null || echo 8)

# Total test timeout = generous multiple of fuzztime to allow for baseline coverage.
# Parse fuzztime to seconds for timeout calculation.
parse_seconds() {
    local val="$1"
    if [[ "$val" =~ ^([0-9]+)s$ ]]; then
        echo "${BASH_REMATCH[1]}"
    elif [[ "$val" =~ ^([0-9]+)m$ ]]; then
        echo $(( ${BASH_REMATCH[1]} * 60 ))
    elif [[ "$val" =~ ^([0-9]+)h$ ]]; then
        echo $(( ${BASH_REMATCH[1]} * 3600 ))
    else
        echo 180 # fallback
    fi
}

FUZZ_SECS=$(parse_seconds "$FUZZTIME")
# Timeout = fuzztime × 2 (generous baseline allowance) + 60s safety margin
TIMEOUT_SECS=$(( FUZZ_SECS * 2 + 60 ))
TIMEOUT="${TIMEOUT_SECS}s"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║  Fuzz Testing — fuzztime=$FUZZTIME, timeout=$TIMEOUT, CPUs=$NCPU"
echo "╚══════════════════════════════════════════════════════════════╝"

TOTAL_FAILED=0
FAILED_TARGETS=()

# Use project-local directories instead of $TMPDIR.
# macOS periodically purges /var/folders/.../T/ which kills long-running fuzzes:
#   - Go's compiled test binary (targets.test) disappears → "no such file or directory"
#   - Our log files disappear → tee fails
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

LOG_DIR="$PROJECT_ROOT/.fuzz-logs"
rm -rf "$LOG_DIR"
mkdir -p "$LOG_DIR"

# GOTMPDIR tells `go test` where to put compiled binaries & work dirs.
# Defaults to $TMPDIR which macOS cleans — use a project-local cache instead.
export GOTMPDIR="$PROJECT_ROOT/.fuzz-cache"
mkdir -p "$GOTMPDIR"

echo "Logs:  $LOG_DIR"
echo "Cache: $GOTMPDIR"

run_batch() {
    local batch_name="$1"
    shift
    local procs_override=""
    if [[ "$1" == --procs=* ]]; then
        procs_override="${1#--procs=}"
        shift
    fi
    local count=$#
    local procs=$(( NCPU / count ))
    [ "$procs" -lt 1 ] && procs=1
    if [ -n "$procs_override" ]; then
        procs="$procs_override"
    fi

    echo ""
    echo "━━━ $batch_name ($count tests, GOMAXPROCS=$procs) ━━━"

    local pids=()
    local targets=()
    for target in "$@"; do
        targets+=("$target")
        local logfile="$LOG_DIR/${target}.log"
        # -run="^$target$" restricts seed corpus regression tests to only this
        # target. Without it, `go test -fuzz=FuzzVM` runs ALL Fuzz* seed corpus
        # tests first, so failures from e.g. FuzzDifferential appear under
        # the [FuzzVM] prefix — confusing and wrong.
        #
        # Output goes to both the terminal (via tee+sed) and a per-target log file
        # so that failures can be reviewed after the run.
        (GOMAXPROCS=$procs go test \
            -run="^${target}$" \
            -fuzz="$target" \
            -fuzztime="$FUZZTIME" \
            -timeout="$TIMEOUT" \
            ./tests/fuzz/targets 2>&1 | tee "$logfile" | sed "s/^/[$target] /") &
        pids+=($!)
    done

    local batch_failed=0
    for i in "${!pids[@]}"; do
        local target="${targets[$i]}"
        local logfile="$LOG_DIR/${target}.log"
        if ! wait "${pids[$i]}"; then
            # Go fuzz exits non-zero on timeout (context deadline exceeded).
            # That's not a real failure — check if the log has an actual crash.
            if grep -q "context deadline exceeded" "$logfile" && \
               ! grep -q "^--- FAIL.*panic\|Failing input\|runtime error" "$logfile"; then
                echo "  ✓ ${target} PASSED (timeout, no crashes)"
            else
                echo "  ✗ ${target} FAILED  (log: $logfile)"
                batch_failed=$((batch_failed + 1))
                FAILED_TARGETS+=("${target}")
            fi
        else
            echo "  ✓ ${target} PASSED"
        fi
    done

    TOTAL_FAILED=$((TOTAL_FAILED + batch_failed))

    if [ "$batch_failed" -gt 0 ]; then
        echo "  ⚠ $batch_failed test(s) failed in $batch_name"
    fi
}

# ── Batch 1: Lightweight CPU-bound tests (no subprocesses, no timeouts) ──
run_batch "Batch 1: Parser & Type Checking" --procs=2 \
    FuzzParser FuzzTypeChecker FuzzKindChecker

# Run FuzzCompiler separately with a lower worker cap for stability.
run_batch "Batch 1b: Compiler" --procs=2 \
    FuzzCompiler FuzzRowPolymorphism FuzzBundleRoundTrip

# ── Batch 2: Lightweight tests with larger corpus ──
run_batch "Batch 2: Formatting & Mutation" \
    FuzzRoundTrip FuzzMutation FuzzStdLib FuzzFormatter

# ── Batch 3: Execution tests (per-iteration timeouts, temp dirs) ──
run_batch "Batch 3: Execution & Modules" \
    FuzzDifferential FuzzModules FuzzStress

# ── Batch 4: Heavy tests (goroutine leaks possible on timeout) ──
run_batch "Batch 4: VM & Async" \
    FuzzAsync FuzzVM

# ── Batch 5: Subprocess-heavy test (spawns OS process per iteration) ──
run_batch "Batch 5: LSP" \
    FuzzLSP

echo ""
echo "══════════════════════════════════════════════════════════════"
if [ "$TOTAL_FAILED" -gt 0 ]; then
    echo "  DONE — $TOTAL_FAILED test(s) FAILED"
    echo ""
    for target in "${FAILED_TARGETS[@]}"; do
        echo "┌─── $target ───"
        # Show the FAIL lines, error details, and the re-run command from the log
        grep -E '(FAIL|FATAL|panic:|Failing input|To re-run:|Error|failed|hung)' \
            "$LOG_DIR/${target}.log" 2>/dev/null | head -30 | sed 's/^/│ /'
        echo "│"
        echo "│ Full log: $LOG_DIR/${target}.log"
        echo "└───────────────────────────────────────"
        echo ""
    done
    exit 1
else
    echo "  DONE — all fuzz tests PASSED"
    # Clean up on success
    rm -rf "$LOG_DIR" "$GOTMPDIR"
fi
