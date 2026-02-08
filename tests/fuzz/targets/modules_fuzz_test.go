package targets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/tests/fuzz/generators"
)

func FuzzModules(f *testing.F) {
	f.Add(int64(12345))
	f.Add(int64(67890))

	f.Fuzz(func(t *testing.T, seed int64) {
		// Create a temporary directory for the modules
		tempDir, err := os.MkdirTemp("", "fuzz_modules_*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Generate modules
		gen := generators.NewModuleGenerator(seed, tempDir)
		if err := gen.GenerateModules(5); err != nil {
			t.Fatalf("failed to generate modules: %v", err)
		}

		// Try to load the main module
		loader := modules.NewLoader()
		mainPath := filepath.Join(tempDir, "main")

		// We expect loading to potentially fail (e.g. cycles, invalid imports), but not panic
		_, err = loader.Load(mainPath)
		if err != nil {
			// Expected errors: cycles, missing files, etc.
			// t.Logf("Load error: %v", err)
		}
	})
}
