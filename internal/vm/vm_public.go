package vm

import (
	"fmt"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/evaluator"
	"reflect"
	"strings"
)

// SetGlobal sets a global variable
func (vm *VM) SetGlobal(name string, value evaluator.Object) {
	vm.evalMu.Lock()
	defer vm.evalMu.Unlock()
	vm.globals.Globals = vm.globals.Globals.Put(name, value)
	// Sync back to evaluator
	if vm.eval != nil && vm.eval.GlobalEnv != nil {
		vm.eval.GlobalEnv.Set(name, value)
	}
}

// GetGlobals returns the current global map
func (vm *VM) GetGlobals() *PersistentMap {
	vm.evalMu.Lock()
	defer vm.evalMu.Unlock()
	return vm.globals.Globals
}

// SetGlobals sets the global map
func (vm *VM) SetGlobals(globals *PersistentMap) {
	vm.evalMu.Lock()
	defer vm.evalMu.Unlock()
	vm.globals.Globals = globals
}

// CallFunction calls a function with arguments
func (vm *VM) CallFunction(fn evaluator.Object, args []evaluator.Object) (evaluator.Object, error) {
	eval := vm.getEvaluator()
	res := eval.ApplyFunction(fn, args)
	if res.Type() == evaluator.ERROR_OBJ {
		return nil, fmt.Errorf("%s", res.(*evaluator.Error).Message)
	}
	return res, nil
}

// GetStackTrace returns the current call stack of the VM
func (vm *VM) GetStackTrace() string {
	// We don't lock here to avoid deadlocks if the VM is stuck in a blocking call
	// that holds a lock, but we copy the frame count to minimize race conditions.
	count := vm.frameCount
	if count <= 0 {
		return "<empty stack>"
	}

	// Cap the count to avoid out-of-bounds if slice shrinks (unlikely but possible in a race)
	if count > len(vm.frames) {
		count = len(vm.frames)
	}

	var stackTrace strings.Builder
	for i := count - 1; i >= 0; i-- {
		frame := &vm.frames[i]
		if frame == nil || frame.chunk == nil {
			continue
		}

		file := frame.chunk.File
		if file == "" {
			file = vm.currentFile
		}
		if file == "" {
			file = "<script>"
		}
		file = config.TrimSourceExt(file)

		line := 0
		if frame.ip > 0 && frame.ip-1 < len(frame.chunk.Lines) {
			line = frame.chunk.Lines[frame.ip-1]
		}

		fnName := file
		if frame.closure != nil && frame.closure.Function != nil {
			if frame.closure.Function.Name != "" && frame.closure.Function.Name != "<script>" {
				fnName = frame.closure.Function.Name
			}
		}

		displayName := fnName
		if i == 0 && fnName == file {
			fullPath := frame.chunk.File
			if fullPath == "" {
				fullPath = vm.currentFile
			}
			displayName = formatFilePath(fullPath)
			if displayName == "" {
				displayName = file
			}
		}

		stackTrace.WriteString(fmt.Sprintf("  at %s:%d\n", displayName, line))
	}

	return stackTrace.String()
}

// SetHostHandlers sets the handlers for host object interaction
func (vm *VM) SetHostHandlers(
	call func(reflect.Value, []evaluator.Object) (evaluator.Object, error),
	toVal func(interface{}) (evaluator.Object, error),
) {
	e := vm.getEvaluator()
	e.HostCallHandler = call
	e.HostToValueHandler = toVal
}
