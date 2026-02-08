package evaluator

import (
	"github.com/funvibe/funxy/internal/typesystem"
)

// RegisterFPTraits registers all built-in FP traits and their instances.
// FP traits (Semigroup, Monoid, Functor, Applicative, Monad, Fallback) are always available.
// This includes trait methods: (<>), mempty, fmap, pure, (<*>), (>>=), (??)
func RegisterFPTraits(e *Evaluator, env *Environment) {
	// Initialize ClassImplementations maps
	for _, traitName := range []string{"Show", "Empty", "Semigroup", "Monoid", "Functor", "Applicative", "Monad", "Optional"} {
		if _, ok := e.ClassImplementations[traitName]; !ok {
			e.ClassImplementations[traitName] = make(map[string]Object)
		}
	}

	// Register trait methods as ClassMethod dispatchers
	// Arity: 0 = nullary (auto-call in type context), 1+ = needs explicit call

	// Show: show - unary (value to string)
	env.Set("show", &ClassMethod{
		Name:            "show",
		ClassName:       "Show",
		Arity:           1,
		DispatchSources: []typesystem.DispatchSource{{Kind: typesystem.DispatchArg, Index: 0}},
	})

	// Empty: isEmpty - unary (container)
	env.Set("isEmpty", &ClassMethod{
		Name:            "isEmpty",
		ClassName:       "Empty",
		Arity:           1,
		DispatchSources: []typesystem.DispatchSource{{Kind: typesystem.DispatchArg, Index: 0}},
	})

	// Semigroup: (<>) - binary operator
	env.Set("(<>)", &ClassMethod{
		Name:            "(<>)",
		ClassName:       "Semigroup",
		Arity:           2,
		DispatchSources: []typesystem.DispatchSource{{Kind: typesystem.DispatchArg, Index: 0}},
	})

	// Monoid: mempty - nullary, needs type context for dispatch
	env.Set("mempty", &ClassMethod{
		Name:            "mempty",
		ClassName:       "Monoid",
		Arity:           0,
		DispatchSources: []typesystem.DispatchSource{{Kind: typesystem.DispatchReturn, Index: -1}},
	})

	// Functor: fmap - binary (f, fa)
	env.Set("fmap", &ClassMethod{
		Name:            "fmap",
		ClassName:       "Functor",
		Arity:           2,
		DispatchSources: []typesystem.DispatchSource{{Kind: typesystem.DispatchArg, Index: 1}},
	})

	// Applicative: pure (unary), (<*>) (binary)
	env.Set("pure", &ClassMethod{
		Name:            "pure",
		ClassName:       "Applicative",
		Arity:           1,
		DispatchSources: []typesystem.DispatchSource{{Kind: typesystem.DispatchReturn, Index: -1}},
	})
	env.Set("(<*>)", &ClassMethod{
		Name:            "(<*>)",
		ClassName:       "Applicative",
		Arity:           2,
		DispatchSources: []typesystem.DispatchSource{{Kind: typesystem.DispatchArg, Index: 0}},
	})

	// Monad: (>>=) - binary
	env.Set("(>>=)", &ClassMethod{
		Name:            "(>>=)",
		ClassName:       "Monad",
		Arity:           2,
		DispatchSources: []typesystem.DispatchSource{{Kind: typesystem.DispatchArg, Index: 0}},
	})

	// Optional: (??) - binary, short-circuit semantics handled in evalInfixExpression
	env.Set("(??)", &ClassMethod{
		Name:            "(??)",
		ClassName:       "Optional",
		Arity:           2,
		DispatchSources: []typesystem.DispatchSource{{Kind: typesystem.DispatchArg, Index: 0}},
	})

	// Register operator -> trait mappings
	if e.OperatorTraits == nil {
		e.OperatorTraits = make(map[string]string)
	}
	e.OperatorTraits["<>"] = "Semigroup"
	e.OperatorTraits["<*>"] = "Applicative"
	e.OperatorTraits[">>="] = "Monad"
	e.OperatorTraits["??"] = "Optional"
	e.OperatorTraits["?."] = "Optional"

	// Register builtin trait defaults (for traits with default implementations)
	registerBuiltinTraitDefaults(e)

	// Register instances for built-in types
	registerShowInstances(e)
	registerEmptyInstances(e)
	registerSemigroupInstances(e)
	registerMonoidInstances(e)
	registerFunctorInstances(e)
	registerApplicativeInstances(e)
	registerMonadInstances(e)
	registerOptionalInstances(e)
	registerReaderInstances(e)
	registerIdentityInstances(e)
	registerStateInstances(e)
	registerWriterInstances(e)
	registerOptionTInstances(e)
	registerResultTInstances(e)

	// Finally, expose all registered instances as global dictionary objects
	RegisterDictionaryGlobals(e, env)
}

// ============================================================================
// Builtin Trait Defaults
// ============================================================================

func registerBuiltinTraitDefaults(e *Evaluator) {
	if e.BuiltinTraitDefaults == nil {
		e.BuiltinTraitDefaults = make(map[string]*Builtin)
	}

	// Show.show default: returns structural representation (Inspect)
	// This is used when no explicit instance Show T is defined
	e.BuiltinTraitDefaults["Show.show"] = &Builtin{
		Name: "show",
		Fn: func(eval *Evaluator, args ...Object) Object {
			if len(args) != 1 {
				return newError("show expects 1 argument, got %d", len(args))
			}
			return StringToList(args[0].Inspect())
		},
	}
}

// ============================================================================
// Show instances
// ============================================================================

func registerShowInstances(e *Evaluator) {
	// Default Show implementation for most types - uses Inspect()
	defaultShowMethod := &Builtin{
		Name: "show",
		Fn: func(eval *Evaluator, args ...Object) Object {
			if _, rest, found := extractWitnessMethod(args, "show"); found {
				args = rest
			}
			if len(args) != 1 {
				return newError("show expects 1 argument, got %d", len(args))
			}
			return StringToList(args[0].Inspect())
		},
	}

	// Special Show for String (List<Char>) - returns content without quotes
	stringShowMethod := &Builtin{
		Name: "show",
		Fn: func(eval *Evaluator, args ...Object) Object {
			if _, rest, found := extractWitnessMethod(args, "show"); found {
				args = rest
			}
			if len(args) != 1 {
				return newError("show expects 1 argument, got %d", len(args))
			}
			if list, ok := args[0].(*List); ok {
				// For strings (List<Char>), return the string content directly
				if IsStringList(list) {
					return list // Return as-is, it's already a string
				}
			}
			// For other Lists, use Inspect()
			return StringToList(args[0].Inspect())
		},
	}

	// Register for all built-in types
	// Note: Some types use UPPERCASE names (TUPLE, RECORD, FUNCTION)
	builtinTypes := []string{
		"Int", "Float", "Bool", "Char", RUNTIME_TYPE_TUPLE,
		RUNTIME_TYPE_RECORD, "Map", "Option", "Result", "BigInt", "Rational",
		"Bytes", "Bits", "Nil", RUNTIME_TYPE_FUNCTION, "Type",
	}

	for _, typeName := range builtinTypes {
		e.ClassImplementations["Show"][typeName] = &MethodTable{
			Methods: map[string]Object{
				"show": defaultShowMethod,
			},
		}
	}

	// String and List use the special show that returns content without quotes
	e.ClassImplementations["Show"]["String"] = &MethodTable{
		Methods: map[string]Object{
			"show": stringShowMethod,
		},
	}
	e.ClassImplementations["Show"]["List"] = &MethodTable{
		Methods: map[string]Object{
			"show": stringShowMethod,
		},
	}
}

// ============================================================================
// Empty instances: isEmpty :: F<A> -> Bool
// ============================================================================

func registerEmptyInstances(e *Evaluator) {
	// List<T>
	e.ClassImplementations["Empty"]["List"] = &MethodTable{
		Methods: map[string]Object{
			"isEmpty": &Builtin{
				Name: "isEmpty",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "isEmpty"); found {
						args = rest
					}
					if len(args) != 1 {
						return newError("isEmpty expects 1 argument, got %d", len(args))
					}
					if list, ok := args[0].(*List); ok {
						if list.len() == 0 {
							return TRUE
						}
						return FALSE
					}
					return newError("isEmpty: expected List, got %T", args[0])
				},
			},
		},
	}

	// Option<T>
	e.ClassImplementations["Empty"]["Option"] = &MethodTable{
		Methods: map[string]Object{
			"isEmpty": &Builtin{
				Name: "isEmpty",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "isEmpty"); found {
						args = rest
					}
					if len(args) != 1 {
						return newError("isEmpty expects 1 argument, got %d", len(args))
					}
					if isNoneValue(args[0]) {
						return TRUE
					}
					return FALSE
				},
			},
		},
	}

	// Result<E, A>
	e.ClassImplementations["Empty"]["Result"] = &MethodTable{
		Methods: map[string]Object{
			"isEmpty": &Builtin{
				Name: "isEmpty",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "isEmpty"); found {
						args = rest
					}
					if len(args) != 1 {
						return newError("isEmpty expects 1 argument, got %d", len(args))
					}
					if di, ok := args[0].(*DataInstance); ok && di.Name == "Fail" {
						return TRUE
					}
					return FALSE
				},
			},
		},
	}

}

// ============================================================================
// Semigroup instances: (<>) :: A -> A -> A
// ============================================================================

func registerSemigroupInstances(e *Evaluator) {
	// List<T>: (<>) = concat
	e.ClassImplementations["Semigroup"]["List"] = &MethodTable{
		Methods: map[string]Object{
			"(<>)": &Builtin{
				Name: "(<>)",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "(<>)"); found {
						args = rest
					}
					if len(args) != 2 {
						return newError("(<>) expects 2 arguments, got %d", len(args))
					}
					left, ok1 := args[0].(*List)
					right, ok2 := args[1].(*List)
					if !ok1 || !ok2 {
						return newError("(<>) for List expects two lists")
					}
					result := make([]Object, 0, left.len()+right.len())
					result = append(result, left.ToSlice()...)
					result = append(result, right.ToSlice()...)
					return newList(result)
				},
			},
		},
	}

	// Option<T>: (<>) = first Some wins (like First monoid)
	e.ClassImplementations["Semigroup"]["Option"] = &MethodTable{
		Methods: map[string]Object{
			"(<>)": &Builtin{
				Name: "(<>)",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "(<>)"); found {
						args = rest
					}
					if len(args) != 2 {
						return newError("(<>) expects 2 arguments, got %d", len(args))
					}
					// If left is Some, return left; otherwise return right
					if isNoneValue(args[0]) {
						return args[1]
					}
					return args[0]
				},
			},
		},
	}
}

// ============================================================================
// Monoid instances: mempty :: A
// ============================================================================

func registerMonoidInstances(e *Evaluator) {
	// List<T>: mempty = []
	e.ClassImplementations["Monoid"]["List"] = &MethodTable{
		Methods: map[string]Object{
			"mempty": &Builtin{
				Name: "mempty",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "mempty"); found {
						args = rest
					}
					return newList([]Object{})
				},
			},
		},
	}

	// Option<T>: mempty = None
	e.ClassImplementations["Monoid"]["Option"] = &MethodTable{
		Methods: map[string]Object{
			"mempty": &Builtin{
				Name: "mempty",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "mempty"); found {
						args = rest
					}
					return makeNone()
				},
			},
		},
	}
}

// Helper function for checking None
func isNoneValue(obj Object) bool {
	if di, ok := obj.(*DataInstance); ok {
		return di.Name == "None" && len(di.Fields) == 0
	}
	return false
}
