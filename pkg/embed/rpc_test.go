package funxy_test

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

func TestRPCCall(t *testing.T) {
	// target VM code
	code1 := `
import "lib/vmm"

fun myMethod(arg) {
    if arg == "ping" {
        return "pong"
    }
    return "unknown"
}
`

	// caller VM code
	code2 := `
import "lib/rpc"
import "lib/vmm"

fun mainLoop() {
    let res = rpc.callWait("target", "myMethod", "ping")
    if res.isOk() {
        return res.unwrapResult()
    }
    return "error"
}
`

	hyp := funxy.NewHypervisor()
	hyp.RegisterCapabilityProvider(func(cap string, vmInstance *funxy.VM) error {
		if cap == "lib/vmm" {
			vmInstance.RegisterSupervisor(hyp)
			return nil
		}
		if cap == "lib/rpc" {
			return nil
		}
		return fmt.Errorf("unknown capability")
	})

	tmpDir := t.TempDir()
	path1 := filepath.Join(tmpDir, "target.lang")
	os.WriteFile(path1, []byte(code1), 0644)
	path2 := filepath.Join(tmpDir, "caller.lang")
	os.WriteFile(path2, []byte(code2), 0644)

	// Spawn target VM
	_, err := hyp.SpawnVM(path1, map[string]interface{}{
		"name":         "target",
		"capabilities": []interface{}{"lib/vmm", "lib/rpc"},
	})
	if err != nil {
		t.Fatalf("SpawnVM failed for target: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// We can test RPC using an evaluator directly
	eval := evaluator.New()
	eval.SupervisorHandler = hyp.SupervisorHandler()

	// Create "ping" string argument natively as Funxy List<Char>
	chars := make([]evaluator.Object, 0, 4)
	for _, r := range "ping" {
		chars = append(chars, &evaluator.Char{Value: int64(r)})
	}
	stringArg := evaluator.NewListWithType(chars, "Char")
	argsData, _ := evaluator.SerializeValue(stringArg, "ephemeral")

	// Perform the RPC call from the hypervisor
	resData, err := hyp.RPCCall("target", "myMethod", argsData, 5000)
	if err != nil {
		t.Fatalf("RPCCall failed: %v", err)
	}

	resObj, err := evaluator.DeserializeValue(resData)
	if err != nil {
		t.Fatalf("Failed to deserialize RPC result: %v", err)
	}

	// Convert result List<Char> back to string for comparison
	if resList, ok := resObj.(*evaluator.List); ok {
		var s string
		for _, el := range resList.ToSlice() {
			if c, ok := el.(*evaluator.Char); ok {
				s += string(rune(c.Value))
			}
		}
		if s != "pong" {
			t.Fatalf("Expected 'pong', got: %s", s)
		}
	} else {
		t.Fatalf("Expected List, got: %s", resObj.Type())
	}
}

func TestRPCCallTimeout(t *testing.T) {
	// target VM code with a slow function
	code1 := `
import "lib/vmm"
import "lib/time"

fun slowMethod(arg) {
    time.sleepMs(100)  // Sleep for 100ms
    return "done"
}
`

	// caller VM code
	code2 := `
import "lib/rpc"
import "lib/vmm"

fun mainLoop() {
    // This should timeout with 50ms timeout
    let res = rpc.callWait("target", "slowMethod", "test", 50)
    return res
}
`

	hyp := funxy.NewHypervisor()
	hyp.RegisterCapabilityProvider(func(cap string, vmInstance *funxy.VM) error {
		if cap == "lib/vmm" {
			vmInstance.RegisterSupervisor(hyp)
			return nil
		}
		if cap == "lib/rpc" {
			return nil
		}
		if cap == "lib/time" {
			return nil
		}
		return fmt.Errorf("unknown capability")
	})

	tmpDir := t.TempDir()
	path1 := filepath.Join(tmpDir, "target.lang")
	os.WriteFile(path1, []byte(code1), 0644)
	path2 := filepath.Join(tmpDir, "caller.lang")
	os.WriteFile(path2, []byte(code2), 0644)

	// Spawn target VM
	_, err := hyp.SpawnVM(path1, map[string]interface{}{
		"name":         "target",
		"capabilities": []interface{}{"lib/vmm", "lib/rpc", "lib/time"},
	})
	if err != nil {
		t.Fatalf("SpawnVM failed for target: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Test RPC call with timeout directly using the hypervisor
	// Serialize empty arguments
	argsData, _ := evaluator.SerializeValue(evaluator.NewListWithType([]evaluator.Object{}, "Char"), "ephemeral")

	// This should timeout with 10ms timeout (function sleeps 100ms)
	_, err = hyp.RPCCall("target", "slowMethod", argsData, 10)
	if err == nil {
		t.Fatal("Expected timeout error, but call succeeded")
	}

	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("Expected timeout error, got: %v", err)
	}
}

func TestRPCCallGroup(t *testing.T) {
	// target VM code
	code1 := `
import "lib/vmm"

fun myMethod(arg) {
    if arg == "ping" {
        return "pong_from_1"
    }
    return "unknown"
}
`
	code2 := `
import "lib/vmm"

fun myMethod(arg) {
    if arg == "ping" {
        return "pong_from_2"
    }
    return "unknown"
}
`

	hyp := funxy.NewHypervisor()
	hyp.RegisterCapabilityProvider(func(cap string, vmInstance *funxy.VM) error {
		if cap == "lib/vmm" || cap == "lib/rpc" {
			vmInstance.RegisterSupervisor(hyp)
			return nil
		}
		return fmt.Errorf("unknown capability")
	})

	tmpDir := t.TempDir()
	path1 := filepath.Join(tmpDir, "target1.lang")
	os.WriteFile(path1, []byte(code1), 0644)
	path2 := filepath.Join(tmpDir, "target2.lang")
	os.WriteFile(path2, []byte(code2), 0644)

	// Spawn target VMs in the same group
	_, err := hyp.SpawnVM(path1, map[string]interface{}{
		"name":         "target1",
		"group":        "workers",
		"capabilities": []interface{}{"lib/vmm", "lib/rpc"},
	})
	if err != nil {
		t.Fatalf("SpawnVM failed for target1: %v", err)
	}

	_, err = hyp.SpawnVM(path2, map[string]interface{}{
		"name":         "target2",
		"group":        "workers",
		"capabilities": []interface{}{"lib/vmm", "lib/rpc"},
	})
	if err != nil {
		t.Fatalf("SpawnVM failed for target2: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Create "ping" string argument natively as Funxy List<Char>
	chars := make([]evaluator.Object, 0, 4)
	for _, r := range "ping" {
		chars = append(chars, &evaluator.Char{Value: int64(r)})
	}
	stringArg := evaluator.NewListWithType(chars, "Char")
	argsData, _ := evaluator.SerializeValue(stringArg, "ephemeral")

	// Perform the RPC call from the hypervisor to the group
	resData1, err := hyp.RPCCallGroup("workers", "myMethod", argsData, 5000)
	if err != nil {
		t.Fatalf("RPCCallGroup failed: %v", err)
	}

	resData2, err := hyp.RPCCallGroup("workers", "myMethod", argsData, 5000)
	if err != nil {
		t.Fatalf("RPCCallGroup failed: %v", err)
	}

	resObj1, _ := evaluator.DeserializeValue(resData1)
	resObj2, _ := evaluator.DeserializeValue(resData2)

	var s1, s2 string
	if resList, ok := resObj1.(*evaluator.List); ok {
		for _, el := range resList.ToSlice() {
			if c, ok := el.(*evaluator.Char); ok {
				s1 += string(rune(c.Value))
			}
		}
	}
	if resList, ok := resObj2.(*evaluator.List); ok {
		for _, el := range resList.ToSlice() {
			if c, ok := el.(*evaluator.Char); ok {
				s2 += string(rune(c.Value))
			}
		}
	}

	if (s1 == "pong_from_1" && s2 == "pong_from_2") || (s1 == "pong_from_2" && s2 == "pong_from_1") {
		// Round robin works
	} else {
		t.Fatalf("Expected round robin 'pong_from_1' and 'pong_from_2', got %v and %v", s1, s2)
	}
}

func TestRPCCircuitBreakerOpenFastFail(t *testing.T) {
	code := `
import "lib/time"
fun slowMethod(arg) {
    time.sleepMs(120)
    return "done"
}
`

	hyp := funxy.NewHypervisor()
	hyp.RegisterCapabilityProvider(func(cap string, vmInstance *funxy.VM) error {
		if cap == "lib/time" || cap == "lib/rpc" {
			return nil
		}
		return fmt.Errorf("unknown capability")
	})

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target.lang")
	_ = os.WriteFile(targetPath, []byte(code), 0644)

	_, err := hyp.SpawnVM(targetPath, map[string]interface{}{
		"name":         "target",
		"capabilities": []interface{}{"lib/rpc", "lib/time"},
	})
	if err != nil {
		t.Fatalf("SpawnVM failed for target: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	argsData, _ := evaluator.SerializeValue(evaluator.NewListWithType([]evaluator.Object{}, "Char"), "ephemeral")

	sawCircuitOpen := false
	for i := 0; i < 10; i++ {
		start := time.Now()
		_, callErr := hyp.RPCCall("target", "slowMethod", argsData, 10)
		elapsed := time.Since(start)
		if callErr == nil {
			t.Fatal("expected RPC error, got nil")
		}
		if strings.Contains(callErr.Error(), "CircuitOpen") {
			sawCircuitOpen = true
			if elapsed > 50*time.Millisecond {
				t.Fatalf("CircuitOpen should fail fast, elapsed=%v", elapsed)
			}
			break
		}
	}
	if !sawCircuitOpen {
		t.Fatal("expected breaker to open and return CircuitOpen")
	}
}

func TestRPCCallGroupSkipsOpenCircuitNodes(t *testing.T) {
	slowCode := `
import "lib/time"
fun myMethod(arg) {
    time.sleepMs(120)
    return "slow"
}
`
	fastCode := `
fun myMethod(arg) {
    return "fast"
}
`

	hyp := funxy.NewHypervisor()
	hyp.RegisterCapabilityProvider(func(cap string, vmInstance *funxy.VM) error {
		if cap == "lib/time" || cap == "lib/rpc" {
			return nil
		}
		return fmt.Errorf("unknown capability")
	})

	tmpDir := t.TempDir()
	slowPath := filepath.Join(tmpDir, "slow.lang")
	fastPath := filepath.Join(tmpDir, "fast.lang")
	_ = os.WriteFile(slowPath, []byte(slowCode), 0644)
	_ = os.WriteFile(fastPath, []byte(fastCode), 0644)

	_, err := hyp.SpawnVM(slowPath, map[string]interface{}{
		"name":         "slow_worker",
		"group":        "workers",
		"capabilities": []interface{}{"lib/rpc", "lib/time"},
	})
	if err != nil {
		t.Fatalf("SpawnVM failed for slow worker: %v", err)
	}
	_, err = hyp.SpawnVM(fastPath, map[string]interface{}{
		"name":         "fast_worker",
		"group":        "workers",
		"capabilities": []interface{}{"lib/rpc"},
	})
	if err != nil {
		t.Fatalf("SpawnVM failed for fast worker: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	argsData, _ := evaluator.SerializeValue(evaluator.NewListWithType([]evaluator.Object{}, "Char"), "ephemeral")

	// Trip breaker on slow worker.
	for i := 0; i < 6; i++ {
		_, _ = hyp.RPCCall("slow_worker", "myMethod", argsData, 10)
	}

	resData, err := hyp.RPCCallGroup("workers", "myMethod", argsData, 30)
	if err != nil {
		t.Fatalf("RPCCallGroup should fallback to healthy node, got error: %v", err)
	}
	got := decodeFunxyString(t, resData)
	if got != "fast" {
		t.Fatalf("expected fallback response from fast node, got %q", got)
	}
}

func TestRPCCircuitBreakerGlobalConfigOverride(t *testing.T) {
	code := `
import "lib/time"
fun slowMethod(arg) {
    time.sleepMs(120)
    return "done"
}
`

	hyp := funxy.NewHypervisorWithRPCCircuitConfig(funxy.RPCCircuitConfig{
		FailureThreshold: 50,
		FailureWindowMs:  5000,
		OpenTimeoutMs:    2000,
	})
	hyp.RegisterCapabilityProvider(func(cap string, vmInstance *funxy.VM) error {
		if cap == "lib/time" || cap == "lib/rpc" {
			return nil
		}
		return fmt.Errorf("unknown capability")
	})

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target_global_cfg.lang")
	_ = os.WriteFile(targetPath, []byte(code), 0644)

	_, err := hyp.SpawnVM(targetPath, map[string]interface{}{
		"name":         "target_global_cfg",
		"capabilities": []interface{}{"lib/rpc", "lib/time"},
	})
	if err != nil {
		t.Fatalf("SpawnVM failed for target: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	argsData, _ := evaluator.SerializeValue(evaluator.NewListWithType([]evaluator.Object{}, "Char"), "ephemeral")

	for i := 0; i < 8; i++ {
		_, callErr := hyp.RPCCall("target_global_cfg", "slowMethod", argsData, 10)
		if callErr == nil {
			t.Fatal("expected RPC error, got nil")
		}
		if strings.Contains(callErr.Error(), "CircuitOpen") {
			t.Fatalf("did not expect CircuitOpen with elevated global threshold, iteration=%d", i)
		}
	}
}

func TestRPCCircuitBreakerPerVMOverride(t *testing.T) {
	code := `
import "lib/time"
fun slowMethod(arg) {
    time.sleepMs(120)
    return "done"
}
`

	hyp := funxy.NewHypervisorWithRPCCircuitConfig(funxy.RPCCircuitConfig{
		FailureThreshold: 50,
		FailureWindowMs:  5000,
		OpenTimeoutMs:    2000,
	})
	hyp.RegisterCapabilityProvider(func(cap string, vmInstance *funxy.VM) error {
		if cap == "lib/time" || cap == "lib/rpc" {
			return nil
		}
		return fmt.Errorf("unknown capability")
	})

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target_vm_cfg.lang")
	_ = os.WriteFile(targetPath, []byte(code), 0644)

	_, err := hyp.SpawnVM(targetPath, map[string]interface{}{
		"name":         "target_vm_cfg",
		"capabilities": []interface{}{"lib/rpc", "lib/time"},
		"rpcCircuit": map[string]interface{}{
			"failureThreshold": 1,
			"failureWindowMs":  5000,
			"openTimeoutMs":    5000,
		},
	})
	if err != nil {
		t.Fatalf("SpawnVM failed for target: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	argsData, _ := evaluator.SerializeValue(evaluator.NewListWithType([]evaluator.Object{}, "Char"), "ephemeral")

	_, callErr := hyp.RPCCall("target_vm_cfg", "slowMethod", argsData, 10)
	if callErr == nil {
		t.Fatal("expected first call to fail by timeout")
	}

	start := time.Now()
	_, callErr = hyp.RPCCall("target_vm_cfg", "slowMethod", argsData, 10)
	elapsed := time.Since(start)
	if callErr == nil {
		t.Fatal("expected second call to fail fast with CircuitOpen")
	}
	if !strings.Contains(callErr.Error(), "CircuitOpen") {
		t.Fatalf("expected CircuitOpen on per-VM override, got: %v", callErr)
	}
	if elapsed > 50*time.Millisecond {
		t.Fatalf("CircuitOpen should fail fast, elapsed=%v", elapsed)
	}
}

func decodeFunxyString(t *testing.T, data []byte) string {
	t.Helper()
	obj, err := evaluator.DeserializeValue(data)
	if err != nil {
		t.Fatalf("failed to deserialize value: %v", err)
	}
	resList, ok := obj.(*evaluator.List)
	if !ok {
		t.Fatalf("expected List<Char>, got %T", obj)
	}
	var s string
	for _, el := range resList.ToSlice() {
		if c, ok := el.(*evaluator.Char); ok {
			s += string(rune(c.Value))
		}
	}
	return s
}

func TestRPCTraceStreamWithTraceOn(t *testing.T) {
	code := `
import "lib/time"

fun ping(arg) {
    return "pong"
}

while true {
    time.sleepMs(50)
}
`
	hyp := funxy.NewHypervisor()
	hyp.RegisterCapabilityProvider(func(cap string, vmInstance *funxy.VM) error { return nil })

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "trace_target.lang")
	_ = os.WriteFile(targetPath, []byte(code), 0644)

	_, err := hyp.SpawnVM(targetPath, map[string]interface{}{
		"name":         "trace_target",
		"capabilities": []interface{}{"lib/time"},
	})
	if err != nil {
		t.Fatalf("SpawnVM failed for target: %v", err)
	}
	if err := hyp.TraceOn("trace_target"); err != nil {
		t.Fatalf("TraceOn failed: %v", err)
	}
	traceCh, unsubscribe := hyp.SubscribeRPCTrace()
	defer unsubscribe()
	time.Sleep(100 * time.Millisecond)

	argsData, _ := evaluator.SerializeValue(&evaluator.Nil{}, "ephemeral")
	_, err = hyp.RPCCall("trace_target", "ping", argsData, 500)
	if err != nil {
		t.Fatalf("RPCCall failed: %v", err)
	}

	select {
	case evt := <-traceCh:
		if evt.ToVM != "trace_target" {
			t.Fatalf("expected trace to target vm, got to=%q", evt.ToVM)
		}
		if evt.Method != "ping" {
			t.Fatalf("expected method ping, got %q", evt.Method)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected trace event, got timeout")
	}
}
