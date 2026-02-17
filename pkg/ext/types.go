package ext

import (
	"fmt"
	"github.com/funvibe/funxy/internal/evaluator"
)

// Object types aliases
type Object = evaluator.Object
type HostObject = evaluator.HostObject
type Builtin = evaluator.Builtin
type Evaluator = evaluator.Evaluator
type Error = evaluator.Error
type Tuple = evaluator.Tuple
type Nil = evaluator.Nil
type DataInstance = evaluator.DataInstance
type List = evaluator.List
type Map = evaluator.Map
type Integer = evaluator.Integer
type Float = evaluator.Float
type Boolean = evaluator.Boolean
type Bytes = evaluator.Bytes
type RecordInstance = evaluator.RecordInstance

// RegisterExtBuiltins registers a map of builtins for a given extension module.
func RegisterExtBuiltins(moduleName string, builtins map[string]Object) {
	evaluator.RegisterExtBuiltins(moduleName, builtins)
}

// Helpers for creating objects (used by codegen)

func NewError(format string, args ...interface{}) *Error {
	// We have to implement this here or expose NewError in evaluator
	// Since evaluator.newError is private, we can't call it.
	// But we can construct the struct directly as it's exported.
	msg := fmt.Sprintf(format, args...)
	return &Error{Message: msg}
}

func NewDataInstance(name string, fields []Object) *DataInstance {
	return &DataInstance{Name: name, Fields: fields}
}

func MakeResultOk(val Object) Object {
	return NewDataInstance("Ok", []Object{val})
}

func MakeResultFail(msg string) Object {
	return NewDataInstance("Fail", []Object{evaluator.StringToList(msg)})
}

// ListToString converts a list of Chars to a Go string
func ListToString(l *List) string {
	return evaluator.ListToString(l)
}

// IsStringList checks if a list contains only Char elements (is a string)
func IsStringList(l *List) bool {
	return evaluator.IsStringList(l)
}

// Re-export constants
var (
	TRUE  = evaluator.TRUE
	FALSE = evaluator.FALSE
)

// Helper functions for conversions (mirroring logic from builder templates, but in Go)
// This reduces the amount of code we need to generate.

func ToFunxy(val interface{}) Object {
	if val == nil {
		return &Nil{}
	}

	switch v := val.(type) {
	case Object:
		return v
	case int:
		return &Integer{Value: int64(v)}
	case int64:
		return &Integer{Value: v}
	case float64:
		return &Float{Value: v}
	case bool:
		if v {
			return TRUE
		}
		return FALSE
	case string:
		return evaluator.StringToList(v)
	case []byte:
		return evaluator.BytesFromSlice(v)
	case error:
		return NewError("%s", v.Error())
	}

	return &HostObject{Value: val}
}
