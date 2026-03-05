package funxy_test

import (
	"fmt"
	"os"
	funxy "github.com/funvibe/funxy/pkg/embed"
	"path/filepath"
	"testing"
	"time"
)

type MyHostObj struct {
	Name string
}

func TestStateHandoff(t *testing.T) {
	code1 := `
import "lib/vmm"

fun onInit(s: Option<a>) {
    s ?? { counter: 0 }
}

fun onTerminate(c) {
    return c
}

fun increment(s) {
    return s.put("counter", s.get("counter") + 1)
}

fun getCounter(s) {
    return s.get("counter")
}

// Emulate a blocking service
fun mainLoop() {
    // just wait for cancellation
    while true {
		// fake loop
    }
}
`

	// 1. Create Hypervisor
	hyp := funxy.NewHypervisor()
	hyp.RegisterCapabilityProvider(func(cap string, vmInstance *funxy.VM) error {
		if cap == "lib/vmm" {
			vmInstance.RegisterSupervisor(hyp)
			return nil
		}
		return fmt.Errorf("unknown capability")
	})

	// 2. We need to save the code to a file since SpawnVM takes a path.
	// But actually we can just compile it directly. Wait, SpawnVM expects a path.
	// For testing, let's write to a temp file.
	tmpDir := t.TempDir()
	path1 := filepath.Join(tmpDir, "service1.lang")
	os.WriteFile(path1, []byte(code1), 0644)

	// Spawn first VM
	id1, err := hyp.SpawnVM(path1, map[string]interface{}{
		"name":         "vm1",
		"capabilities": []interface{}{"lib/vmm"},
	})
	if err != nil {
		t.Fatalf("SpawnVM failed: %v", err)
	}

	// Give it a moment to run onInit and enter loop
	time.Sleep(100 * time.Millisecond)

	// Now gracefully stop the VM and capture state
	stateData, err := hyp.KillVM(id1, true, 5000)
	if err != nil {
		t.Fatalf("KillVM failed: %v", err)
	}

	if len(stateData) == 0 {
		t.Fatalf("stateData is empty")
	}

	// Spawn new VM with old state
	id2, err := hyp.SpawnVM(path1, map[string]interface{}{
		"name":           "vm2",
		"_initial_state": stateData,
		"capabilities":   []interface{}{"lib/vmm"},
	})
	if err != nil {
		t.Fatalf("SpawnVM failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	hyp.KillVM(id2, false, 5000)
}
