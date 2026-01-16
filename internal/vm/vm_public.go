package vm

import (
	"fmt"
	"github.com/funvibe/funxy/internal/evaluator"
	"reflect"
)

// SetGlobal sets a global variable
func (vm *VM) SetGlobal(name string, value evaluator.Object) {
	vm.globals.Globals = vm.globals.Globals.Put(name, value)
}

// GetGlobals returns the current global map
func (vm *VM) GetGlobals() *PersistentMap {
	return vm.globals.Globals
}

// SetGlobals sets the global map
func (vm *VM) SetGlobals(globals *PersistentMap) {
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

// SetHostHandlers sets the handlers for host object interaction
func (vm *VM) SetHostHandlers(
	call func(reflect.Value, []evaluator.Object) (evaluator.Object, error),
	toVal func(interface{}) (evaluator.Object, error),
) {
	e := vm.getEvaluator()
	e.HostCallHandler = call
	e.HostToValueHandler = toVal
}
