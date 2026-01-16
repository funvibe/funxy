package evaluator

import (
	"reflect"
)

// AccessHostMember accesses a field or method on a HostObject using reflection
func (e *Evaluator) AccessHostMember(hostObj *HostObject, member string) Object {
	val := reflect.ValueOf(hostObj.Value)

	// Dereference interface/pointer if needed to get to struct/value
	if val.Kind() == reflect.Interface {
		val = val.Elem()
	}

	// 1. Check for Method
	method := val.MethodByName(member)
	if method.IsValid() {
		// Return a Builtin that wraps this method call
		return &Builtin{
			Name: member,
			Fn: func(ev *Evaluator, args ...Object) Object {
				// Convert args to reflect.Value
				// We need to know expected types from method signature?
				// Or do best effort conversion?
				// The Marshaller is in pkg/embed/marshaller.go, which imports evaluator.
				// Evaluator CANNOT import pkg/embed (cycle).
				// So we need conversion logic here or injected.
				// Ideally, we should inject a "HostCallHandler" into Evaluator.

				// Quick fix: Use a simplified conversion for now, or require injection.
				// Since we are adding HostObject support to Evaluator, we should add HostCallHandler.
				if ev.HostCallHandler != nil {
					res, err := ev.HostCallHandler(method, args)
					if err != nil {
						return newError("host call error: %v", err)
					}
					return res
				}
				return newError("HostCallHandler not configured")
			},
		}
	}

	// 2. Check for Field (if struct)
	// We need to dereference pointer to struct to access fields
	indirectVal := val
	if val.Kind() == reflect.Ptr {
		indirectVal = val.Elem()
	}

	if indirectVal.Kind() == reflect.Struct {
		field := indirectVal.FieldByName(member)
		if field.IsValid() {
			// Convert field value to Object
			// Again, we need conversion logic.
			if e.HostToValueHandler != nil {
				res, err := e.HostToValueHandler(field.Interface())
				if err != nil {
					return newError("host conversion error: %v", err)
				}
				return res
			}
			return &HostObject{Value: field.Interface()}
		}
	}

	return newError("member '%s' not found on HostObject %s", member, hostObj.Inspect())
}
