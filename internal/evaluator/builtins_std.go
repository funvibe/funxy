package evaluator

import (
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/typesystem"
)

// RegisterStandardTraits registers standard library traits like Show, Eq, etc.
func RegisterStandardTraits(e *Evaluator, env *Environment) {
	registerShowTrait(e, env)
}

// registerShowTrait registers Show trait and its instances
func registerShowTrait(e *Evaluator, env *Environment) {
	traitName := "Show"
	methodName := "show"

	// Ensure trait exists
	if _, ok := e.ClassImplementations[traitName]; !ok {
		e.ClassImplementations[traitName] = make(map[string]Object)
	}

	// Register trait method dispatcher in environment
	// This makes 'show' a ClassMethod, enabling dynamic dispatch
	// Note: This shadows the 'show' builtin function if it exists
	env.Set(methodName, &ClassMethod{
		Name:      methodName,
		ClassName: traitName,
		Arity:     1,
	})

	// Generic Show implementation using objectToString
	genericShow := &Builtin{
		Name: methodName,
		TypeInfo: typesystem.TFunc{
			Params:     []typesystem.Type{typesystem.TVar{Name: "a"}},
			ReturnType: typesystem.TApp{Constructor: typesystem.TCon{Name: config.ListTypeName}, Args: []typesystem.Type{typesystem.Char}},
		},
		Fn: func(eval *Evaluator, args ...Object) Object {
			if len(args) != 1 {
				return newError("show expects 1 argument, got %d", len(args))
			}
			return stringToList(objectToString(args[0]))
		},
	}

	// Register instances for all standard types
	types := []string{
		"Int", "Float", "Bool", "Char", "String", 
		"List", "Map", "Option", "Result", 
		"Type", "Nil", "Bytes", "Bits", 
		"BigInt", "Rational", "Function",
		"Tuple", "Task",
	}

	// Also register for specific DataInstance types if needed, 
	// but usually they fall back to their ADT type name.
	// For example, "Option" covers Some/Zero if RuntimeType() returns Option.

	for _, typeName := range types {
		e.ClassImplementations[traitName][typeName] = &MethodTable{
			Methods: map[string]Object{
				methodName: genericShow,
			},
		}
	}
	
	// Register for "Tuple" (special handling might be needed if RuntimeType uses TTuple)
	// RuntimeType for Tuple returns TTuple which has no name?
	// TTuple.String() returns (T1, T2).
	// vm.getTypeName calls t.Name.
	// If it's TTuple, Name is empty?
	// Let's check getTypeName in VM or evaluator.
}

