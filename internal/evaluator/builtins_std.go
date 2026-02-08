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
		// Add concrete types that were missing in analyzer/builtins.go but good to have
		"Uuid", "Reader", "Identity", "State", "Writer", "OptionT", "ResultT",
	}

	// Also register for specific DataInstance types if needed,
	// but usually they fall back to their ADT type name.
	// For example, "Option" covers Some/None if RuntimeType() returns Option.

	for _, typeName := range types {
		e.ClassImplementations[traitName][typeName] = &MethodTable{
			Methods: map[string]Object{
				methodName: genericShow,
			},
		}
	}

	// Register Dictionary Evidence for Show
	// We need to register globals like $impl_Show_Int, $ctor_Show_List, etc.
	// These are used by the new dictionary passing system.

	// registerDictionaryEvidence registers a global dictionary for a type
	registerDictionaryEvidence := func(trait, typeName string, methods []Object, isGeneric bool) {
		var name string
		if isGeneric {
			name = "$ctor_" + trait + "_" + typeName
		} else {
			name = "$impl_" + trait + "_" + typeName
		}

		if isGeneric {
			// For generics (List, Option...), we need a constructor function
			// $ctor_Show_List(dict_a_Show) -> Dictionary
			ctor := &Builtin{
				Name: name,
				Fn: func(eval *Evaluator, args ...Object) Object {
					// We construct a new dictionary that uses the inner dictionary (args[0])
					// Ideally, we should compose the methods.
					// For Show List<T>, we need show(xs) which uses show(x).

					// For generic types like List<T>, we return a dictionary where the 'show' method
					// relies on objectToString. Since objectToString is implemented to recursively
					// handle lists and other collections (calling String() on elements),
					// it correctly handles nested types without needing explicit composition
					// of inner dictionaries here.

					return &Dictionary{
						TraitName: trait,
						Methods:   methods,
						// We don't attach supers for Show currently
					}
				},
			}
			env.Set(name, ctor)
		} else {
			// For concrete types (Int, Bool...), we register a constant Dictionary
			dict := &Dictionary{
				TraitName: trait,
				Methods:   methods,
			}
			env.Set(name, dict)
		}
	}

	for _, typeName := range types {
		// Determine if generic
		isGeneric := false
		switch typeName {
		case "List", "Map", "Option", "Result", "Tuple", "Task":
			isGeneric = true
		}

		registerDictionaryEvidence(traitName, typeName, []Object{genericShow}, isGeneric)
	}
}
