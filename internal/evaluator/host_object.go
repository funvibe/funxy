package evaluator

import (
	"fmt"
	"github.com/funvibe/funxy/internal/typesystem"
	"reflect"
)

// HostObject wraps a Go interface{} for use in Funxy.
// It allows calling methods and accessing fields via reflection.
type HostObject struct {
	Value interface{}
}

func (h *HostObject) Type() ObjectType { return HOST_OBJ }

func (h *HostObject) Inspect() string {
	return fmt.Sprintf("<HostObject: %T %+v>", h.Value, h.Value)
}

func (h *HostObject) RuntimeType() typesystem.Type {
	// We can try to map Go type to Funxy type, or just return a generic HostObject type
	// For now, let's return a named type "HostObject"
	return typesystem.TCon{Name: "HostObject"}
}

func (h *HostObject) Hash() uint32 {
	// Best effort hash
	if h.Value == nil {
		return 0
	}
	// Use address if possible
	val := reflect.ValueOf(h.Value)
	switch val.Kind() {
	case reflect.Ptr, reflect.UnsafePointer, reflect.Chan, reflect.Func, reflect.Map, reflect.Slice:
		return uint32(val.Pointer())
	default:
		// Fallback to string representation hash
		return hashString(fmt.Sprintf("%v", h.Value))
	}
}
