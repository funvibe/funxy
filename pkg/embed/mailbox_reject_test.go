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

func TestMailboxFastMutableReject(t *testing.T) {
	code1 := `
import "lib/vmm"

fun loop() {
    return "ok"
}
`

	hyp := NewHypervisor()
	hyp.RegisterCapabilityProvider(func(cap string, vmInstance *VM) error {
		if cap == "lib/vmm" {
			vmInstance.RegisterSupervisor(hyp)
			return nil
		}
		if cap == "lib/mailbox" {
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
		"capabilities": []interface{}{"lib/vmm", "lib/mailbox"},
	})
	if err != nil {
		t.Fatalf("SpawnVM failed for target: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	eval := evaluator.New()
	eval.MailboxHandler = hyp.MailboxHandler(CallerIDHost)

	// 1. Try to send HostObject
	hostObj := &evaluator.HostObject{Value: make(map[string]interface{})}

	resObj := evaluator.MailboxBuiltins()["send"].Fn(eval, evaluator.StringToList("target"), hostObj)

	isExpectedErr := false
	if resObj.Type() == evaluator.ERROR_OBJ {
		isExpectedErr = strings.Contains(resObj.(*evaluator.Error).Message, "cannot pass mutable or non-serializable object via mailbox") || strings.Contains(resObj.(*evaluator.Error).Message, "cannot serialize HostObject")
	} else if resObj.Type() == evaluator.DATA_INSTANCE_OBJ {
		inst := resObj.(*evaluator.DataInstance)
		if inst.Name == "Fail" && len(inst.Fields) > 0 {
			if strObj, ok := inst.Fields[0].(*evaluator.List); ok && evaluator.IsStringList(strObj) {
				isExpectedErr = strings.Contains(evaluator.ListToString(strObj), "cannot serialize HostObject")
			}
		}
	}

	if !isExpectedErr {
		t.Fatalf("Expected 'cannot pass mutable...' error, got: %s", resObj.Inspect())
	}
}
