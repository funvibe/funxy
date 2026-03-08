package evaluator

import (
	"github.com/funvibe/funxy/internal/symbols"
	"sync"
)

func NewEnvironment() *Environment {
	return &Environment{store: make(map[string]Object)}
}

func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	if outer != nil {
		env.SymbolTable = outer.SymbolTable
	}
	return env
}

type Environment struct {
	mu          sync.RWMutex
	store       map[string]Object
	outer       *Environment
	SymbolTable *symbols.SymbolTable
}

func (e *Environment) Get(name string) (Object, bool) {
	e.mu.RLock()
	obj, ok := e.store[name]
	e.mu.RUnlock()
	if !ok && e.outer != nil {
		obj, ok = e.outer.Get(name)
	}
	return obj, ok
}

func (e *Environment) Set(name string, val Object) Object {
	e.mu.Lock()
	e.store[name] = val
	e.mu.Unlock()
	return val
}

func (e *Environment) Update(name string, val Object) bool {
	e.mu.Lock()
	_, ok := e.store[name]
	if ok {
		e.store[name] = val
		e.mu.Unlock()
		return true
	}
	e.mu.Unlock()
	if e.outer != nil {
		return e.outer.Update(name, val)
	}
	return false
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
	clone := &Environment{store: storeCopy}
	if e.outer != nil {
		clone.outer = e.outer.Clone()
	}
	clone.SymbolTable = e.SymbolTable
	return clone
}
