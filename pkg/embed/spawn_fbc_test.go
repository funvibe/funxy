package funxy_test

import (
	"fmt"
	"os"
	funxy "github.com/funvibe/funxy/pkg/embed"
	"path/filepath"
	"testing"
	"time"

	"github.com/funvibe/funxy/internal/vm"
)

// TestSpawnVM_FBC verifies that spawnVM can load and run .fbc bytecode files directly.
func TestSpawnVM_FBC(t *testing.T) {
	workerCode := `
import "lib/vmm" (getState, setState)
import "lib/time" (sleepMs)

fun onInit(s: Option<a>) { s ?? { count: 0 } }
fun onTerminate(c) { return c }

st = getState() ?? { count: 0 }
print("Worker started from .fbc, count=" ++ show(st.count))

while true {
    sleepMs(200)
    st = { count: st.count + 1 }
    setState(st)
}
`

	tmpDir := t.TempDir()
	langPath := filepath.Join(tmpDir, "worker.lang")
	fbcPath := filepath.Join(tmpDir, "worker.fbc")

	if err := os.WriteFile(langPath, []byte(workerCode), 0644); err != nil {
		t.Fatalf("write lang: %v", err)
	}

	// Compile .lang to .fbc using embed VM
	compiler := funxy.New()
	chunk, err := compiler.CompileFile(langPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if chunk == nil {
		t.Fatal("chunk is nil")
	}

	bundle := &vm.Bundle{
		MainChunk: chunk,
		Modules:   make(map[string]*vm.BundledModule),
	}
	if chunk.File != "" {
		bundle.SourceFile = chunk.File
	}
	data, err := bundle.Serialize()
	if err != nil {
		t.Fatalf("serialize bundle: %v", err)
	}
	if err := os.WriteFile(fbcPath, data, 0644); err != nil {
		t.Fatalf("write fbc: %v", err)
	}

	// Spawn VM from .fbc
	hyp := funxy.NewHypervisor()
	hyp.RegisterCapabilityProvider(func(cap string, vmInstance *funxy.VM) error {
		if cap == "lib/vmm" || cap == "lib/time" {
			vmInstance.RegisterSupervisor(hyp)
			return nil
		}
		return fmt.Errorf("unknown capability: %s", cap)
	})

	id, err := hyp.SpawnVM(fbcPath, map[string]interface{}{
		"name":         "worker_fbc",
		"capabilities": []interface{}{"lib/vmm", "lib/time"},
	})
	if err != nil {
		t.Fatalf("SpawnVM(.fbc) failed: %v", err)
	}
	if id != "worker_fbc" {
		t.Errorf("expected id worker_fbc, got %s", id)
	}

	time.Sleep(300 * time.Millisecond)

	// Gracefully stop and capture state
	stateData, err := hyp.KillVM(id, true, 5000)
	if err != nil {
		t.Fatalf("KillVM failed: %v", err)
	}
	if len(stateData) == 0 {
		t.Fatal("expected non-empty state from worker")
	}

	// Spawn again from .fbc with restored state (tests cache + hot-reload path)
	id2, err := hyp.SpawnVM(fbcPath, map[string]interface{}{
		"name":           "worker_fbc",
		"_initial_state": stateData,
		"capabilities":   []interface{}{"lib/vmm", "lib/time"},
	})
	if err != nil {
		t.Fatalf("SpawnVM(.fbc) with state failed: %v", err)
	}
	if id2 != "worker_fbc" {
		t.Errorf("expected id worker_fbc, got %s", id2)
	}

	time.Sleep(100 * time.Millisecond)
	hyp.KillVM(id2, false, 5000)
}
