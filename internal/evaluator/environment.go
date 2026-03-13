package evaluator

import (
	"github.com/funvibe/funxy/internal/symbols"
	"sync"
)

var (
	// Pool for reusing Environment structs
	environmentPool = sync.Pool{
		New: func() interface{} {
			return &Environment{store: make(map[string]Object)}
		},
	}
)

// NewEnvironment creates a new environment
func NewEnvironment() *Environment {
	env := environmentPool.Get().(*Environment)
	env.outer = nil
	env.SymbolTable = nil
	env.isIsolated = false
	// store map is reused (cleared on Release)
	return env
}

// NewEnclosedEnvironment creates a new enclosed environment
func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := environmentPool.Get().(*Environment)
	// store map is reused (cleared on Release), but we need to ensure it's empty
	env.outer = outer
	if outer != nil {
		env.SymbolTable = outer.SymbolTable
	}
	// Important: Reset isolated flag if reusing struct
	env.isIsolated = false
	return env
}

// NewIsolatedEnvironment creates a new environment that treats the outer environment
// as read-only. Updates to variables existing in outer will be written to local store,
// effectively shadowing them (Copy-On-Write for variables).
func NewIsolatedEnvironment(outer *Environment) *Environment {
	env := environmentPool.Get().(*Environment)
	env.outer = outer
	env.isIsolated = true
	if outer != nil {
		env.SymbolTable = outer.SymbolTable
	}
	return env
}

// Release returns the environment to the pool
func (e *Environment) Release() {
	if e == nil {
		return
	}
	e.mu.Lock()
	// Clear store
	for k := range e.store {
		delete(e.store, k)
	}
	e.outer = nil
	e.SymbolTable = nil
	e.isIsolated = false
	e.mu.Unlock()
	environmentPool.Put(e)
}

type Environment struct {
	mu          sync.RWMutex
	store       map[string]Object
	outer       *Environment
	SymbolTable *symbols.SymbolTable
	isIsolated  bool // If true, Update() writes to local store instead of propagating to outer
	readOnly    bool // If true, Get() skips locking (assumes immutable)
}

func (e *Environment) Get(name string) (Object, bool) {
	// Optimization: Skip lock if ReadOnly
	if e.readOnly {
		obj, ok := e.store[name]
		if !ok && e.outer != nil {
			obj, ok = e.outer.Get(name)
		}
		return obj, ok
	}

	e.mu.RLock()
	obj, ok := e.store[name]
	e.mu.RUnlock()
	if !ok && e.outer != nil {
		obj, ok = e.outer.Get(name)
	}
	return obj, ok
}

func (e *Environment) Set(name string, val Object) Object {
	if e.readOnly {
		// Modifying a read-only environment is a bug.
		panic("attempted to modify read-only environment")
	}
	e.mu.Lock()
	e.store[name] = val
	e.mu.Unlock()
	return val
}

func (e *Environment) Update(name string, val Object) bool {
	if e.readOnly {
		panic("attempted to update read-only environment")
	}
	e.mu.Lock()
	_, ok := e.store[name]
	if ok {
		e.store[name] = val
		e.mu.Unlock()
		return true
	}
	e.mu.Unlock()

	// If isolated, we treat update as set for globals (COW behavior)
	// We check if it exists in outer to confirm it's a valid variable,
	// but we write to local store.
	if e.isIsolated && e.outer != nil {
		if _, existsInOuter := e.outer.Get(name); existsInOuter {
			e.Set(name, val)
			return true
		}
	}

	if e.outer != nil && !e.isIsolated {
		return e.outer.Update(name, val)
	}
	return false
}

// SetReadOnly marks the environment as read-only, optimizing reads
func (e *Environment) SetReadOnly() {
	e.mu.Lock()
	e.readOnly = true
	e.mu.Unlock()
}

// GetStore returns the environment bindings as a StringMap.
func (e *Environment) GetStore() *StringMap {
	e.mu.RLock()
	defer e.mu.RUnlock()
	sm := EmptyStringMap()
	for k, v := range e.store {
		sm = sm.Put(k, v)
	}
	return sm
}

// Clone creates a thread-safe copy of the environment.
func (e *Environment) Clone() *Environment {
	if e == nil {
		return nil
	}
	e.mu.RLock()
	storeCopy := make(map[string]Object, len(e.store))
	for k, v := range e.store {
		storeCopy[k] = v
	}
	e.mu.RUnlock()

	// Clone should also use pool or manually allocate, but Clone is usually deep copy.
	// For performance, we should avoid Clone if possible.
	// But here we return a NEW Environment.
	// We can use the pool if we are careful about Release().
	// Typically Clone()ed environments are owned by the new evaluator and released by it.

	clone := environmentPool.Get().(*Environment)
	clone.store = storeCopy
	if e.outer != nil {
		clone.outer = e.outer.Clone()
	}
	clone.SymbolTable = e.SymbolTable
	return clone
}
