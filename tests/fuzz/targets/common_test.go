package targets

import (
	"flag"
	"os"
	"runtime"
	"strconv"
	"sync"

	"github.com/funvibe/funxy/internal/analyzer"
	"github.com/funvibe/funxy/internal/modules"
)

// useTreeWalk flag controls whether to use the tree-walk backend for fuzzing.
// It matches the flag used in other tests.
var useTreeWalk = flag.Bool("tree", false, "run fuzz tests with tree-walk backend")

var capFuzzProcsOnce sync.Once

func capFuzzProcs() {
	capFuzzProcsOnce.Do(func() {
		// Cap fuzz worker parallelism unless explicitly disabled.
		if os.Getenv("FUZZ_NO_GOMAXPROCS_CAP") == "" {
			max := 2
			if raw := os.Getenv("FUZZ_MAX_GOMAXPROCS"); raw != "" {
				if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
					max = parsed
				}
			}
			if max > runtime.NumCPU() {
				max = runtime.NumCPU()
			}
			if runtime.GOMAXPROCS(0) > max {
				runtime.GOMAXPROCS(max)
			}
		}
	})
}

func init() {
	// Preload shared global state before fuzzing starts to avoid concurrent writes.
	analyzer.RegisterBuiltins(nil)
	modules.InitVirtualPackages()
	capFuzzProcs()
}
