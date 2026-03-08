package vm

import "github.com/funvibe/funxy/internal/evaluator"

// PersistentMap is an alias for evaluator.StringMap — string-keyed immutable HAMT.
type PersistentMap = evaluator.StringMap

// ModuleScope wraps PersistentMap to provide shared mutable access to globals within a module
type ModuleScope struct {
	Globals *PersistentMap
}

// NewModuleScope creates a new module scope
func NewModuleScope() *ModuleScope {
	return &ModuleScope{
		Globals: EmptyMap(),
	}
}

// EmptyMap returns an empty persistent map
func EmptyMap() *PersistentMap {
	return evaluator.EmptyStringMap()
}
