package targets

import (
	"flag"
	"os"
	"runtime"

	"github.com/funvibe/funxy/internal/analyzer"
	"github.com/funvibe/funxy/internal/modules"
)

// useTreeWalk flag controls whether to use the tree-walk backend for fuzzing.
// It matches the flag used in other tests.
var useTreeWalk = flag.Bool("tree", false, "run fuzz tests with tree-walk backend")

func init() {
	// Preload shared global state before fuzzing starts to avoid concurrent writes.
	analyzer.RegisterBuiltins(nil)
	modules.InitVirtualPackages()

	// Cap fuzz worker parallelism unless the caller explicitly set GOMAXPROCS.
	if _, ok := os.LookupEnv("GOMAXPROCS"); !ok {
		max := runtime.NumCPU()
		if max > 4 {
			max = 4
		}
		if runtime.GOMAXPROCS(0) > max {
			runtime.GOMAXPROCS(max)
		}
	}
}
