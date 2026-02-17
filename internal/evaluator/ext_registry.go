package evaluator

import "sync"

// extBuiltinsRegistry is a global registry for ext/* virtual module builtins.
// These are registered at startup by generated code from funxy.yaml bindings.
//
// Thread-safe: registration happens once at startup; reads happen from multiple VMs.
var extBuiltinsRegistry = struct {
	mu       sync.RWMutex
	registry map[string]map[string]Object
}{
	registry: make(map[string]map[string]Object),
}

// RegisterExtBuiltins registers runtime builtins for an ext/* virtual module.
// The name should be the module name without the "ext/" prefix (e.g. "redis").
// The builtins map is name → Object (typically *Builtin functions).
//
// This function is called by generated binding code at startup.
// It is safe to call from init() functions.
func RegisterExtBuiltins(name string, builtins map[string]Object) {
	extBuiltinsRegistry.mu.Lock()
	defer extBuiltinsRegistry.mu.Unlock()
	extBuiltinsRegistry.registry[name] = builtins
}

// GetExtBuiltins returns the registered builtins for an ext/* module.
// Returns nil if no builtins are registered for the given name.
func GetExtBuiltins(name string) map[string]Object {
	extBuiltinsRegistry.mu.RLock()
	defer extBuiltinsRegistry.mu.RUnlock()
	return extBuiltinsRegistry.registry[name]
}

// GetAllExtModules returns the names of all registered ext modules.
func GetAllExtModules() []string {
	extBuiltinsRegistry.mu.RLock()
	defer extBuiltinsRegistry.mu.RUnlock()
	names := make([]string, 0, len(extBuiltinsRegistry.registry))
	for name := range extBuiltinsRegistry.registry {
		names = append(names, name)
	}
	return names
}

// IsExtModuleRegistered checks if an ext module is registered.
func IsExtModuleRegistered(name string) bool {
	extBuiltinsRegistry.mu.RLock()
	defer extBuiltinsRegistry.mu.RUnlock()
	_, ok := extBuiltinsRegistry.registry[name]
	return ok
}

// ClearExtBuiltins removes all registered ext builtins.
// Used for testing.
func ClearExtBuiltins() {
	extBuiltinsRegistry.mu.Lock()
	defer extBuiltinsRegistry.mu.Unlock()
	extBuiltinsRegistry.registry = make(map[string]map[string]Object)
}
