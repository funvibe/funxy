package evaluator

import "sync"

// extBuiltinsRegistry is a global registry for ext/* virtual module builtins.
// These are registered at startup by generated code from funxy.yaml bindings.
//
// Thread-safe: registration happens once at startup; reads happen from multiple VMs.
var extBuiltinsRegistry = struct {
	mu       sync.RWMutex
	registry *StringMap
}{
	registry: EmptyStringMap(),
}

// RegisterExtBuiltins registers runtime builtins for an ext/* virtual module.
// The name should be the module name without the "ext/" prefix (e.g. "redis").
//
// This function is called by generated binding code at startup.
// It is safe to call from init() functions.
func RegisterExtBuiltins(name string, builtins *StringMap) {
	extBuiltinsRegistry.mu.Lock()
	defer extBuiltinsRegistry.mu.Unlock()
	extBuiltinsRegistry.registry = extBuiltinsRegistry.registry.Put(name, builtins)
}

// GetExtBuiltins returns the registered builtins for an ext/* module.
// Returns nil if no builtins are registered for the given name.
func GetExtBuiltins(name string) *StringMap {
	extBuiltinsRegistry.mu.RLock()
	defer extBuiltinsRegistry.mu.RUnlock()
	obj := extBuiltinsRegistry.registry.Get(name)
	if obj == nil {
		return nil
	}
	return obj.(*StringMap)
}

// GetAllExtModules returns the names of all registered ext modules.
func GetAllExtModules() []string {
	extBuiltinsRegistry.mu.RLock()
	defer extBuiltinsRegistry.mu.RUnlock()
	names := make([]string, 0, extBuiltinsRegistry.registry.Len())
	extBuiltinsRegistry.registry.Range(func(name string, _ Object) bool {
		names = append(names, name)
		return true
	})
	return names
}

// IsExtModuleRegistered checks if an ext module is registered.
func IsExtModuleRegistered(name string) bool {
	extBuiltinsRegistry.mu.RLock()
	defer extBuiltinsRegistry.mu.RUnlock()
	return extBuiltinsRegistry.registry.Get(name) != nil
}

// ClearExtBuiltins removes all registered ext builtins.
// Used for testing.
func ClearExtBuiltins() {
	extBuiltinsRegistry.mu.Lock()
	defer extBuiltinsRegistry.mu.Unlock()
	extBuiltinsRegistry.registry = EmptyStringMap()
}
