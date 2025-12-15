package evaluator

import "sync"

func NewEnvironment() *Environment {
	return &Environment{store: make(map[string]Object)}
}

func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	return env
}

type Environment struct {
	mu    sync.RWMutex
	store map[string]Object
	outer *Environment
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

// GetStore returns a copy of the store (exported for VM)
func (e *Environment) GetStore() map[string]Object {
	e.mu.RLock()
	defer e.mu.RUnlock()
	copy := make(map[string]Object)
	for k, v := range e.store {
		copy[k] = v
	}
	return copy
}
