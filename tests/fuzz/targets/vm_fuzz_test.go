package targets

import (
	"context"
	"github.com/funvibe/funxy/internal/vm"
	"github.com/funvibe/funxy/tests/fuzz/generators"
	"runtime/debug"
	"testing"
	"time"
)

// FuzzVM tests the VM with random bytecode.
func FuzzVM(f *testing.F) {
	// Add seed corpus
	f.Add([]byte("seed"))

	f.Fuzz(func(t *testing.T, data []byte) {
		gen := generators.NewBytecodeGenerator(data)
		chunk := gen.GenerateChunk()

		// Context timeout ensures the VM goroutine actually stops on infinite loops,
		// not just the select — preventing goroutine accumulation over long fuzz runs.
		vmCtx, vmCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)

		v := vm.New()
		v.SetContext(vmCtx)

		// We expect it to either succeed or return an error, but NOT panic.
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("VM panic: %v", r)
			}
		}()

		// Buffered channel prevents goroutine leak when timeout fires.
		done := make(chan bool, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("VM panic: %v\n%s", r, string(debug.Stack()))
				}
				done <- true
			}()
			_, _ = v.Run(chunk)
		}()

		select {
		case <-done:
			vmCancel()
		case <-vmCtx.Done():
			vmCancel()
		}
	})
}
