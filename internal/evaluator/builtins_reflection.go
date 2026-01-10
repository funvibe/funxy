package evaluator

import (
	"fmt"
	"github.com/funvibe/funxy/internal/typesystem"
)

// kindOf returns the string representation of the value's type kind.
// Usage: kindOf(List) -> "* -> *"
// Usage: kindOf([1]) -> "*"
func builtinKindOf(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("kindOf expects 1 argument")
	}

	val := args[0]
	var t typesystem.Type

	// If the argument is a TypeObject (e.g. kindOf(Int)), use its value
	if typeObj, ok := val.(*TypeObject); ok {
		t = typeObj.TypeVal
	} else {
		// Otherwise use the type of the value
		t = val.RuntimeType()
	}

	return stringToList(t.Kind().String())
}

// debugType returns the full string signature of the type
func builtinDebugType(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("debugType expects 1 argument")
	}
	return stringToList(args[0].RuntimeType().String())
}

// debugRepr returns the Go-struct dump of the value (useful for checking internal state)
func builtinDebugRepr(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("debugRepr expects 1 argument")
	}
	return stringToList(fmt.Sprintf("%#v", args[0]))
}
