package vm

import "github.com/funvibe/funxy/internal/evaluator"

// PersistentMap is an alias for evaluator.StringMap — string-keyed immutable HAMT.
type PersistentMap = evaluator.StringMap

// NewModuleScope creates a new module scope
func NewModuleScope() *ModuleScope {
	scope := moduleScopePool.Get().(*ModuleScope)
	scope.Globals = EmptyMap()
	return scope
}

// EmptyMap returns an empty persistent map
func EmptyMap() *PersistentMap {
	return evaluator.EmptyStringMap()
}
