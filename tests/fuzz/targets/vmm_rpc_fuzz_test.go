package targets

import (
	"fmt"
	"os"
	"github.com/funvibe/funxy/internal/evaluator"
	funxy "github.com/funvibe/funxy/pkg/embed"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// FuzzHypervisor — VMM lifecycle with random configs
// =============================================================================

// FuzzHypervisor spawns VMs with random configs and exercises spawnVM, killVM, stopVM, listVMs.
// Named FuzzHypervisor (not FuzzVMM) to avoid -fuzz=FuzzVM matching both FuzzVM and FuzzVMM.
// Invariant: must never panic (compile/runtime errors are fine).
func FuzzHypervisor(f *testing.F) {
	capFuzzProcs()

	// Seed corpus
	f.Add("vm1", []byte("lib/vmm"))
	f.Add("worker_a", []byte("lib/vmm,lib/time"))
	f.Add("x", []byte(""))

	f.Fuzz(func(t *testing.T, name string, capsBytes []byte) {
		if len(name) > 64 || strings.ContainsAny(name, "/\\\x00") {
			return
		}
		if name == "" {
			name = "vm"
		}

		tmpDir := t.TempDir()
		workerPath := filepath.Join(tmpDir, "worker.lang")

		// Minimal worker: blocking loop so VM stays alive
		workerCode := `
import "lib/vmm"
import "lib/time" (sleepMs)
while true { sleepMs(100) }
`
		if err := os.WriteFile(workerPath, []byte(workerCode), 0644); err != nil {
			t.Fatalf("write worker: %v", err)
		}

		// Build capabilities from fuzz bytes
		capSet := make(map[string]bool)
		for _, b := range capsBytes {
			if b < 3 {
				capSet["lib/vmm"] = true
			} else if b < 6 {
				capSet["lib/time"] = true
			}
		}
		capSet["lib/vmm"] = true // always need vmm for worker
		capSet["lib/time"] = true
		caps := make([]interface{}, 0, len(capSet))
		for c := range capSet {
			caps = append(caps, c)
		}

		hyp := funxy.NewHypervisor()
		hyp.RegisterCapabilityProvider(func(cap string, vm *funxy.VM) error {
			if cap == "lib/vmm" {
				vm.RegisterSupervisor(hyp)
				return nil
			}
			if cap == "lib/time" {
				return nil
			}
			return fmt.Errorf("unknown cap: %s", cap)
		})

		// Spawn with timeout
		done := make(chan struct{})
		var spawnErr error
		go func() {
			_, spawnErr = hyp.SpawnVM(workerPath, map[string]interface{}{
				"name":         name,
				"capabilities": caps,
			})
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			return
		}
		if spawnErr != nil {
			return
		}

		// Brief wait for VM to init
		time.Sleep(50 * time.Millisecond)

		// Exercise listVMs
		_ = hyp.ListVMs()

		// Kill (no state save) with short timeout
		_, _ = hyp.KillVM(name, false, 500)
	})
}

// =============================================================================
// FuzzRPC — Cross-VM RPC with random args
// =============================================================================

// FuzzRPC spawns a target VM with a known function and fuzzes RPCCall with random serialized args.
// Invariant: must never panic.
func FuzzRPC(f *testing.F) {
	capFuzzProcs()

	// Seed corpus: method name + arg bytes
	f.Add("ping", []byte("hello"))
	f.Add("id", []byte{})
	f.Add("echo", []byte{1, 2, 3, 4, 5})

	f.Fuzz(func(t *testing.T, method string, argBytes []byte) {
		if len(method) > 64 || strings.ContainsAny(method, " \t\n\x00") {
			return
		}
		if method == "" {
			method = "id"
		}

		tmpDir := t.TempDir()
		targetPath := filepath.Join(tmpDir, "target.lang")

		// Target VM: simple identity + echo functions
		targetCode := `
import "lib/vmm"
import "lib/time" (sleepMs)

fun id(x) { x }
fun echo(x) { x }
fun ping(msg) { "pong: " ++ show(msg) }

while true { sleepMs(100) }
`
		if err := os.WriteFile(targetPath, []byte(targetCode), 0644); err != nil {
			t.Fatalf("write target: %v", err)
		}

		hyp := funxy.NewHypervisor()
		hyp.RegisterCapabilityProvider(func(cap string, vm *funxy.VM) error {
			if cap == "lib/vmm" {
				vm.RegisterSupervisor(hyp)
				return nil
			}
			if cap == "lib/time" {
				return nil
			}
			return fmt.Errorf("unknown cap: %s", cap)
		})

		_, err := hyp.SpawnVM(targetPath, map[string]interface{}{
			"name":         "target",
			"capabilities": []interface{}{"lib/vmm", "lib/time"},
		})
		if err != nil {
			return
		}

		time.Sleep(100 * time.Millisecond)

		// Build args: valid Funxy value or raw bytes (fuzz deserializer)
		argsData := fuzzRpcArgs(argBytes)

		_, _ = hyp.RPCCall("target", method, argsData, 5000)

		hyp.KillVM("target", false, 500)
	})
}

// fuzzRpcArgs produces bytes for RPC args. Either serializes a valid value
// (int/string) or passes raw bytes to fuzz the deserializer.
func fuzzRpcArgs(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	// 50%: valid serialized value; 50%: raw fuzz bytes (stress deserializer)
	if data[0]%2 == 0 {
		var obj evaluator.Object
		if data[0]%4 == 0 {
			obj = &evaluator.Integer{Value: int64(int8(data[0]))}
		} else {
			obj = evaluator.StringToList(string(data))
		}
		enc, err := evaluator.SerializeValue(obj, "ephemeral")
		if err != nil {
			return data
		}
		return enc
	}
	return data
}
