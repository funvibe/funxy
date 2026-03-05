package funxy_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/funvibe/funxy/internal/evaluator"
	funxy "github.com/funvibe/funxy/pkg/embed"
)

func TestSleepCancellation(t *testing.T) {
	// Test that sleep properly handles context cancellation
	code := `
import "lib/time" (sleep)

fun testSleep() {
    sleep(10)  // Sleep for 10 seconds
    return "completed"
}
`

	hyp := funxy.NewHypervisor()
	hyp.RegisterCapabilityProvider(func(cap string, vmInstance *funxy.VM) error {
		if cap == "lib/time" {
			return nil
		}
		return nil
	})

	// Create temp file
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.lang")
	os.WriteFile(path, []byte(code), 0644)

	// Spawn VM
	_, err := hyp.SpawnVM(path, map[string]interface{}{
		"name":         "test_vm",
		"capabilities": []interface{}{"lib/time"},
	})
	if err != nil {
		t.Fatalf("SpawnVM failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond) // Let VM initialize

	// Create evaluator with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	eval := evaluator.New()
	eval.Context = ctx
	eval.GlobalEnv = evaluator.NewEnvironment()
	eval.SupervisorHandler = hyp.SupervisorHandler()

	// Initialize evaluator with basic functions and traits
	evaluator.RegisterBasicTraits(eval, eval.GlobalEnv)
	evaluator.RegisterFPTraits(eval, eval.GlobalEnv)
	evaluator.RegisterDictionaryGlobals(eval, eval.GlobalEnv)

	// Register time builtins
	timeBuiltins := evaluator.TimeBuiltins()
	for name, builtin := range timeBuiltins {
		eval.GlobalEnv.Set(name, builtin)
	}

	// Create a simple sleep call using ApplyFunction
	sleepFn, ok := eval.GlobalEnv.Get("sleep")
	if !ok || sleepFn == nil {
		t.Fatal("sleep function not found in global environment")
	}

	// Call sleep(10) - it should be cancelled due to context timeout
	tenSeconds := &evaluator.Integer{Value: 10}
	result := eval.ApplyFunction(sleepFn, []evaluator.Object{tenSeconds})
	if result.Type() != evaluator.ERROR_OBJ {
		t.Fatal("Expected cancellation error, but function completed")
	}

	errorObj := result.(*evaluator.Error)
	if !contains(errorObj.Message, "cancelled") {
		t.Fatalf("Expected cancellation error, got: %v", errorObj.Message)
	}
}

func TestSleepMsCancellation(t *testing.T) {
	// Test that sleepMs properly handles context cancellation
	code := `
import "lib/time" (sleepMs)

fun testSleepMs() {
    sleepMs(10000)  // Sleep for 10 seconds
    return "completed"
}
`

	hyp := funxy.NewHypervisor()
	hyp.RegisterCapabilityProvider(func(cap string, vmInstance *funxy.VM) error {
		if cap == "lib/time" {
			return nil
		}
		return nil
	})

	// Create temp file
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.lang")
	os.WriteFile(path, []byte(code), 0644)

	// Spawn VM
	_, err := hyp.SpawnVM(path, map[string]interface{}{
		"name":         "test_vm",
		"capabilities": []interface{}{"lib/time"},
	})
	if err != nil {
		t.Fatalf("SpawnVM failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond) // Let VM initialize

	// Create evaluator with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	eval := evaluator.New()
	eval.Context = ctx
	eval.GlobalEnv = evaluator.NewEnvironment()
	eval.SupervisorHandler = hyp.SupervisorHandler()

	// Initialize evaluator with basic functions and traits
	evaluator.RegisterBasicTraits(eval, eval.GlobalEnv)
	evaluator.RegisterFPTraits(eval, eval.GlobalEnv)
	evaluator.RegisterDictionaryGlobals(eval, eval.GlobalEnv)

	// Register time builtins
	timeBuiltins := evaluator.TimeBuiltins()
	for name, builtin := range timeBuiltins {
		eval.GlobalEnv.Set(name, builtin)
	}

	// Create a simple sleepMs call using ApplyFunction
	sleepMsFn, ok := eval.GlobalEnv.Get("sleepMs")
	if !ok || sleepMsFn == nil {
		t.Fatal("sleepMs function not found in global environment")
	}

	// Call sleepMs(10000) - it should be cancelled due to context timeout
	tenSeconds := &evaluator.Integer{Value: 10000}
	result := eval.ApplyFunction(sleepMsFn, []evaluator.Object{tenSeconds})
	if result.Type() != evaluator.ERROR_OBJ {
		t.Fatal("Expected cancellation error, but function completed")
	}

	errorObj := result.(*evaluator.Error)
	if !contains(errorObj.Message, "cancelled") {
		t.Fatalf("Expected cancellation error, got: %v", errorObj.Message)
	}
}

func TestReadKeyCancellation(t *testing.T) {
	// Test that readKey checks context before blocking
	// This test verifies that readKey checks context BEFORE starting the blocking operation
	// When context is already cancelled, readKey should fail immediately without blocking

	hyp := funxy.NewHypervisor()
	hyp.RegisterCapabilityProvider(func(cap string, vmInstance *funxy.VM) error {
		if cap == "lib/term" {
			return nil
		}
		return nil
	})

	// Create temp file
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.lang")
	code := `
import "lib/term" (readKey)

fun testReadKey() {
    return readKey(5000)
}
`
	os.WriteFile(path, []byte(code), 0644)

	// Spawn VM
	_, err := hyp.SpawnVM(path, map[string]interface{}{
		"name":         "test_vm",
		"capabilities": []interface{}{"lib/term"},
	})
	if err != nil {
		t.Fatalf("SpawnVM failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond) // Let VM initialize

	// Create evaluator with ALREADY cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	eval := evaluator.New()
	eval.Context = ctx
	eval.GlobalEnv = evaluator.NewEnvironment()
	eval.SupervisorHandler = hyp.SupervisorHandler()

	// Initialize evaluator with basic functions and traits
	evaluator.RegisterBasicTraits(eval, eval.GlobalEnv)
	evaluator.RegisterFPTraits(eval, eval.GlobalEnv)
	evaluator.RegisterDictionaryGlobals(eval, eval.GlobalEnv)

	// Register term builtins
	termBuiltins := evaluator.TermBuiltins()
	for name, builtin := range termBuiltins {
		eval.GlobalEnv.Set(name, builtin)
	}

	// Create a simple readKey call using ApplyFunction
	readKeyFn, ok := eval.GlobalEnv.Get("readKey")
	if !ok || readKeyFn == nil {
		t.Fatal("readKey function not found in global environment")
	}

	// Call readKey with already cancelled context - should fail immediately
	fiveSeconds := &evaluator.Integer{Value: 5000}
	result := eval.ApplyFunction(readKeyFn, []evaluator.Object{fiveSeconds})
	if result.Type() != evaluator.ERROR_OBJ {
		t.Fatal("Expected cancellation error for already cancelled context, but function proceeded")
	}

	errorObj := result.(*evaluator.Error)
	if !contains(errorObj.Message, "cancelled") {
		t.Fatalf("Expected cancellation error, got: %v", errorObj.Message)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && s[len(s)-len(substr):] == substr ||
		containsMiddle(s, substr)
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
