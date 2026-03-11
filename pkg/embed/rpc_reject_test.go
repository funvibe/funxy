package funxy

import (
	"fmt"
	"os"
	"github.com/funvibe/funxy/internal/evaluator"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRPCCallFastMutableReject(t *testing.T) {
	// target VM code
	code1 := `
import "lib/vmm"

fun myMethod(arg) {
    return "ok"
}
`

	hyp := NewHypervisor()
	hyp.RegisterCapabilityProvider(func(cap string, vmInstance *VM) error {
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

	// Spawn target VM
	_, err := hyp.SpawnVM(path1, map[string]interface{}{
		"name":         "target",
		"capabilities": []interface{}{"lib/vmm", "lib/rpc"},
	})
	if err != nil {
		t.Fatalf("SpawnVM failed for target: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// We can test RPC using an evaluator directly. We use an untrusted caller ID.
	eval := evaluator.New()
	eval.SupervisorHandler = hyp.SupervisorHandlerFor("untrusted_caller")

	// 1. Try to send HostObject
	hostObj := &evaluator.HostObject{Value: make(map[string]interface{})}

	// Check builtins directly (which has the CheckSerializable logic now)
	rpcBuiltins := evaluator.RpcBuiltins()
	resObj := rpcBuiltins["callWait"].Fn(eval, evaluator.StringToList("target"), evaluator.StringToList("myMethod"), hostObj)

	// In Funxy, builtins often return Result<String, T> objects for errors (which evaluate to an ADT Instance),
	// or they can return a primitive *Error.
	isExpectedErr := false
	if resObj.Type() == evaluator.ERROR_OBJ {
		isExpectedErr = strings.Contains(resObj.(*evaluator.Error).Message, "cannot pass mutable or non-serializable object via RPC")
	} else if resObj.Type() == evaluator.DATA_INSTANCE_OBJ {
		inst := resObj.(*evaluator.DataInstance)
		if inst.Name == "Fail" && len(inst.Fields) > 0 {
			if strObj, ok := inst.Fields[0].(*evaluator.List); ok && evaluator.IsStringList(strObj) {
				isExpectedErr = strings.Contains(evaluator.ListToString(strObj), "cannot pass mutable or non-serializable object via RPC")
			}
		}
	}

	if !isExpectedErr {
		t.Fatalf("Expected 'cannot pass mutable...' error, got: %s", resObj.Inspect())
	}
}
