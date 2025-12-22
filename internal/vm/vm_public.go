package vm

import "github.com/funvibe/funxy/internal/evaluator"

// SetGlobal sets a global variable
func (vm *VM) SetGlobal(name string, value evaluator.Object) {
	vm.globals.Globals = vm.globals.Globals.Put(name, value)
}
