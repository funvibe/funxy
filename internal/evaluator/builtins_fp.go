package evaluator

import (
	"github.com/funvibe/funxy/internal/ast"
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
	env.Set("show", &ClassMethod{Name: "show", ClassName: "Show", Arity: 1})

	// Empty: isEmpty - unary (container)
	env.Set("isEmpty", &ClassMethod{Name: "isEmpty", ClassName: "Empty", Arity: 1})

	// Semigroup: (<>) - binary operator
	env.Set("(<>)", &ClassMethod{Name: "(<>)", ClassName: "Semigroup", Arity: 2})

	// Monoid: mempty - nullary, needs type context for dispatch
	env.Set("mempty", &ClassMethod{Name: "mempty", ClassName: "Monoid", Arity: 0})

	// Functor: fmap - binary (f, fa)
	env.Set("fmap", &ClassMethod{Name: "fmap", ClassName: "Functor", Arity: 2})

	// Applicative: pure (unary), (<*>) (binary)
	env.Set("pure", &ClassMethod{Name: "pure", ClassName: "Applicative", Arity: 1})
	env.Set("(<*>)", &ClassMethod{Name: "(<*>)", ClassName: "Applicative", Arity: 2})

	// Monad: (>>=) - binary
	env.Set("(>>=)", &ClassMethod{Name: "(>>=)", ClassName: "Monad", Arity: 2})

	// Optional: (??) - binary, short-circuit semantics handled in evalInfixExpression
	env.Set("(??)", &ClassMethod{Name: "(??)", ClassName: "Optional", Arity: 2})

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
// Reader instances
// ============================================================================

func registerReaderInstances(e *Evaluator) {
	// Functor: fmap f (Reader g) = Reader (f . g)
	e.ClassImplementations["Functor"]["Reader"] = &MethodTable{
		Methods: map[string]Object{
			"fmap": &Builtin{
				Name: "fmap",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "fmap"); found {
						args = rest
					}
					if len(args) != 2 {
						return newError("fmap expects 2 arguments, got %d", len(args))
					}
					f := args[0]
					r := args[1]
					di, ok := r.(*DataInstance)
					if !ok || di.TypeName != "Reader" || len(di.Fields) != 1 {
						return newError("fmap for Reader: expected Reader")
					}
					g := di.Fields[0]

					// Create composed function: \x -> f(g(x))
					composed := &Builtin{
						Name: "fmap_reader_composed",
						Fn: func(ev *Evaluator, callArgs ...Object) Object {
							if len(callArgs) != 1 {
								return newError("Reader function expects 1 argument")
							}
							// Run inner reader: g(env)
							val := ev.ApplyFunction(g, callArgs)
							if isError(val) {
								return val
							}
							// Apply f
							return ev.ApplyFunction(f, []Object{val})
						},
					}

					// Return new Reader(composed)
					return &DataInstance{
						Name:     "reader",
						TypeName: "Reader",
						Fields:   []Object{composed},
					}
				},
			},
		},
	}

	// Applicative
	e.ClassImplementations["Applicative"]["Reader"] = &MethodTable{
		Methods: map[string]Object{
			"pure": &Builtin{
				Name: "pure",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "pure"); found {
						args = rest
					}
					// Handle placeholder/implicit witness removal if needed
					for len(args) > 1 {
						if _, ok := args[0].(*Dictionary); ok {
							args = args[1:]
						} else {
							break
						}
					}
					if len(args) != 1 {
						return newError("pure expects 1 argument")
					}
					val := args[0]
					// Reader (\_ -> val)
					constFn := &Builtin{
						Name: "pure_reader",
						Fn: func(ev *Evaluator, callArgs ...Object) Object {
							return val
						},
					}
					return &DataInstance{Name: "reader", TypeName: "Reader", Fields: []Object{constFn}}
				},
			},
			"(<*>)": &Builtin{
				Name: "(<*>)",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "(<*>)"); found {
						args = rest
					}
					if len(args) != 2 {
						return newError("(<*>) expects 2 arguments")
					}
					// rf <*> rx = Reader (\e -> (run rf e) (run rx e))
					rf := args[0] // Reader (E -> A -> B)
					rx := args[1] // Reader (E -> A)

					diF, ok1 := rf.(*DataInstance)
					diX, ok2 := rx.(*DataInstance)
					if !ok1 || !ok2 || diF.TypeName != "Reader" || diX.TypeName != "Reader" {
						return newError("(<*>) for Reader: expected Reader")
					}
					runF := diF.Fields[0]
					runX := diX.Fields[0]

					applied := &Builtin{
						Name: "ap_reader",
						Fn: func(ev *Evaluator, callArgs ...Object) Object {
							if len(callArgs) != 1 {
								return newError("Reader function expects 1 argument")
							}
							envVal := callArgs[0]
							// f = runF(env)
							fVal := ev.ApplyFunction(runF, []Object{envVal})
							if isError(fVal) {
								return fVal
							}
							// x = runX(env)
							xVal := ev.ApplyFunction(runX, []Object{envVal})
							if isError(xVal) {
								return xVal
							}
							// f(x)
							return ev.ApplyFunction(fVal, []Object{xVal})
						},
					}
					return &DataInstance{Name: "reader", TypeName: "Reader", Fields: []Object{applied}}
				},
			},
		},
	}

	// Monad
	e.ClassImplementations["Monad"]["Reader"] = &MethodTable{
		Methods: map[string]Object{
			"(>>=)": &Builtin{
				Name: "(>>=)",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "(>>=)"); found {
						args = rest
					}
					if len(args) != 2 {
						return newError("(>>=) expects 2 arguments")
					}
					// r >>= f = Reader (\e -> run (f (run r e)) e)
					r := args[0]
					f := args[1] // A -> Reader<E, B>

					di, ok := r.(*DataInstance)
					if !ok || di.TypeName != "Reader" {
						return newError("(>>=) for Reader: expected Reader")
					}
					runR := di.Fields[0]

					bind := &Builtin{
						Name: "bind_reader",
						Fn: func(ev *Evaluator, callArgs ...Object) Object {
							if len(callArgs) != 1 {
								return newError("Reader function expects 1 argument")
							}
							envVal := callArgs[0]
							// a = runR(e)
							aVal := ev.ApplyFunction(runR, []Object{envVal})
							if isError(aVal) {
								return aVal
							}
							// Set monad context for the callback
							oldContainer := ev.ContainerContext
							ev.ContainerContext = "Reader"
							defer func() { ev.ContainerContext = oldContainer }()

							// mb = f(a) -> returns Reader<E, B>
							mb := ev.ApplyFunction(f, []Object{aVal})
							if isError(mb) {
								return mb
							}
							// run mb e
							diMB, ok := mb.(*DataInstance)
							if !ok || diMB.TypeName != "Reader" {
								return newError("(>>=) for Reader: function must return Reader")
							}
							return ev.ApplyFunction(diMB.Fields[0], []Object{envVal})
						},
					}
					return &DataInstance{Name: "reader", TypeName: "Reader", Fields: []Object{bind}}
				},
			},
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
					if isZeroValue(args[0]) {
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
					if isZeroValue(args[0]) {
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

	// Option<T>: mempty = Zero
	e.ClassImplementations["Monoid"]["Option"] = &MethodTable{
		Methods: map[string]Object{
			"mempty": &Builtin{
				Name: "mempty",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "mempty"); found {
						args = rest
					}
					return makeZero()
				},
			},
		},
	}
}

// ============================================================================
// Functor instances: fmap :: (A -> B) -> F<A> -> F<B>
// ============================================================================

func registerFunctorInstances(e *Evaluator) {
	// List<T>: fmap = map
	e.ClassImplementations["Functor"]["List"] = &MethodTable{
		Methods: map[string]Object{
			"fmap": &Builtin{
				Name: "fmap",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "fmap"); found {
						args = rest
					}
					if len(args) != 2 {
						return newError("fmap expects 2 arguments, got %d", len(args))
					}
					fn := args[0]
					list, ok := args[1].(*List)
					if !ok {
						return newError("fmap for List expects a list as second argument")
					}
					result := make([]Object, list.len())
					for i, elem := range list.ToSlice() {
						mapped := eval.ApplyFunction(fn, []Object{elem})
						if isError(mapped) {
							return mapped
						}
						result[i] = mapped
					}
					return newList(result)
				},
			},
		},
	}

	// Option<T>: fmap over Some, Zero stays Zero
	e.ClassImplementations["Functor"]["Option"] = &MethodTable{
		Methods: map[string]Object{
			"fmap": &Builtin{
				Name: "fmap",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "fmap"); found {
						args = rest
					}
					if len(args) != 2 {
						return newError("fmap expects 2 arguments, got %d", len(args))
					}
					fn := args[0]
					opt := args[1]
					if isZeroValue(opt) {
						return makeZero()
					}
					// Extract value from Some
					if di, ok := opt.(*DataInstance); ok && di.Name == "Some" && len(di.Fields) == 1 {
						mapped := eval.ApplyFunction(fn, []Object{di.Fields[0]})
						if isError(mapped) {
							return mapped
						}
						return makeSome(mapped)
					}
					return newError("fmap for Option: expected Some or Zero")
				},
			},
		},
	}

	// Result<A, E>: fmap over Ok, Fail stays Fail
	e.ClassImplementations["Functor"]["Result"] = &MethodTable{
		Methods: map[string]Object{
			"fmap": &Builtin{
				Name: "fmap",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "fmap"); found {
						args = rest
					}
					if len(args) != 2 {
						return newError("fmap expects 2 arguments, got %d", len(args))
					}
					fn := args[0]
					result := args[1]
					if di, ok := result.(*DataInstance); ok {
						if di.Name == "Ok" && len(di.Fields) == 1 {
							mapped := eval.ApplyFunction(fn, []Object{di.Fields[0]})
							if isError(mapped) {
								return mapped
							}
							return makeOk(mapped)
						}
						if di.Name == "Fail" {
							return result // Fail stays Fail
						}
					}
					return newError("fmap for Result: expected Ok or Fail")
				},
			},
		},
	}
}

// ============================================================================
// Applicative instances: pure :: A -> F<A>, (<*>) :: F<(A -> B)> -> F<A> -> F<B>
// ============================================================================

func registerApplicativeInstances(e *Evaluator) {
	// List<T>
	e.ClassImplementations["Applicative"]["List"] = &MethodTable{
		Methods: map[string]Object{
			"pure": &Builtin{
				Name: "pure",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "pure"); found {
						args = rest
					}
					if len(args) != 1 {
						return newError("pure expects 1 argument, got %d", len(args))
					}
					return newList([]Object{args[0]})
				},
			},
			"(<*>)": &Builtin{
				Name: "(<*>)",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "(<*>)"); found {
						args = rest
					}
					if len(args) != 2 {
						return newError("(<*>) expects 2 arguments, got %d", len(args))
					}
					fns, ok1 := args[0].(*List)
					vals, ok2 := args[1].(*List)
					if !ok1 || !ok2 {
						return newError("(<*>) for List expects two lists")
					}
					// Cartesian product: [f, g] <*> [x, y] = [f(x), f(y), g(x), g(y)]
					result := make([]Object, 0, fns.len()*vals.len())
					for _, fn := range fns.ToSlice() {
						for _, val := range vals.ToSlice() {
							applied := eval.ApplyFunction(fn, []Object{val})
							if isError(applied) {
								return applied
							}
							result = append(result, applied)
						}
					}
					return newList(result)
				},
			},
		},
	}

	// Option<T>
	e.ClassImplementations["Applicative"]["Option"] = &MethodTable{
		Methods: map[string]Object{
			"pure": &Builtin{
				Name: "pure",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "pure"); found {
						args = rest
					}
					// Handle placeholder/implicit witness removal if needed
					for len(args) > 1 {
						if _, ok := args[0].(*Dictionary); ok {
							args = args[1:]
						} else {
							break
						}
					}
					if len(args) != 1 {
						return newError("pure expects 1 argument, got %d", len(args))
					}
					return makeSome(args[0])
				},
			},
			"(<*>)": &Builtin{
				Name: "(<*>)",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "(<*>)"); found {
						args = rest
					}
					if len(args) != 2 {
						return newError("(<*>) expects 2 arguments, got %d", len(args))
					}
					fnOpt := args[0]
					valOpt := args[1]
					// If either is Zero, result is Zero
					if isZeroValue(fnOpt) || isZeroValue(valOpt) {
						return makeZero()
					}
					// Extract function and value
					fnDi, ok1 := fnOpt.(*DataInstance)
					valDi, ok2 := valOpt.(*DataInstance)
					if !ok1 || !ok2 || fnDi.Name != "Some" || valDi.Name != "Some" {
						return newError("(<*>) for Option: expected Some")
					}
					if len(fnDi.Fields) != 1 || len(valDi.Fields) != 1 {
						return newError("(<*>) for Option: malformed Some")
					}
					applied := eval.ApplyFunction(fnDi.Fields[0], []Object{valDi.Fields[0]})
					if isError(applied) {
						return applied
					}
					return makeSome(applied)
				},
			},
		},
	}

	// Result<A, E>
	e.ClassImplementations["Applicative"]["Result"] = &MethodTable{
		Methods: map[string]Object{
			"pure": &Builtin{
				Name: "pure",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "pure"); found {
						args = rest
					}
					// Handle placeholder/implicit witness removal if needed
					for len(args) > 1 {
						if _, ok := args[0].(*Dictionary); ok {
							args = args[1:]
						} else {
							break
						}
					}
					if len(args) != 1 {
						return newError("pure expects 1 argument, got %d", len(args))
					}
					return makeOk(args[0])
				},
			},
			"(<*>)": &Builtin{
				Name: "(<*>)",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "(<*>)"); found {
						args = rest
					}
					if len(args) != 2 {
						return newError("(<*>) expects 2 arguments, got %d", len(args))
					}
					fnRes := args[0]
					valRes := args[1]
					fnDi, ok1 := fnRes.(*DataInstance)
					valDi, ok2 := valRes.(*DataInstance)
					if !ok1 || !ok2 {
						return newError("(<*>) for Result: expected Result types")
					}
					// If fn is Fail, return Fail
					if fnDi.Name == "Fail" {
						return fnRes
					}
					// If val is Fail, return Fail
					if valDi.Name == "Fail" {
						return valRes
					}
					// Both Ok
					if fnDi.Name == "Ok" && valDi.Name == "Ok" && len(fnDi.Fields) == 1 && len(valDi.Fields) == 1 {
						applied := eval.ApplyFunction(fnDi.Fields[0], []Object{valDi.Fields[0]})
						if isError(applied) {
							return applied
						}
						return makeOk(applied)
					}
					return newError("(<*>) for Result: malformed Result")
				},
			},
		},
	}
}

// ============================================================================
// Monad instances: (>>=) :: M<A> -> (A -> M<B>) -> M<B>
// ============================================================================

func registerMonadInstances(e *Evaluator) {
	// List<T>: (>>=) = flatMap/concatMap
	e.ClassImplementations["Monad"]["List"] = &MethodTable{
		Methods: map[string]Object{
			"(>>=)": &Builtin{
				Name: "(>>=)",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "(>>=)"); found {
						args = rest
					}
					if len(args) != 2 {
						return newError("(>>=) expects 2 arguments, got %d", len(args))
					}
					list, ok := args[0].(*List)
					fn := args[1]
					if !ok {
						return newError("(>>=) for List expects a list as first argument")
					}
					result := make([]Object, 0)
					// Set monad context from left operand type
					oldContainer := eval.ContainerContext
					eval.ContainerContext = getRuntimeTypeName(args[0])
					defer func() { eval.ContainerContext = oldContainer }()

					for _, elem := range list.ToSlice() {
						mapped := eval.ApplyFunction(fn, []Object{elem})
						if isError(mapped) {
							return mapped
						}
						if mappedList, ok := mapped.(*List); ok {
							result = append(result, mappedList.ToSlice()...)
						} else {
							return newError("(>>=) for List: function must return a List")
						}
					}
					return newList(result)
				},
			},
		},
	}

	// Option<T>: (>>=) = flatMap
	e.ClassImplementations["Monad"]["Option"] = &MethodTable{
		Methods: map[string]Object{
			"(>>=)": &Builtin{
				Name: "(>>=)",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "(>>=)"); found {
						args = rest
					}
					if len(args) != 2 {
						return newError("(>>=) expects 2 arguments, got %d", len(args))
					}
					opt := args[0]
					fn := args[1]
					if isZeroValue(opt) {
						return makeZero()
					}
					if di, ok := opt.(*DataInstance); ok && di.Name == "Some" && len(di.Fields) == 1 {
						// Set monad context from left operand type
						oldContainer := eval.ContainerContext
						eval.ContainerContext = getRuntimeTypeName(opt)
						defer func() { eval.ContainerContext = oldContainer }()
						return eval.ApplyFunction(fn, []Object{di.Fields[0]})
					}
					return newError("(>>=) for Option: expected Some or Zero")
				},
			},
		},
	}

	// Result<A, E>: (>>=) = flatMap
	e.ClassImplementations["Monad"]["Result"] = &MethodTable{
		Methods: map[string]Object{
			"(>>=)": &Builtin{
				Name: "(>>=)",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "(>>=)"); found {
						args = rest
					}
					if len(args) != 2 {
						return newError("(>>=) expects 2 arguments, got %d", len(args))
					}
					result := args[0]
					fn := args[1]
					if di, ok := result.(*DataInstance); ok {
						if di.Name == "Fail" {
							return result // Fail propagates
						}
						if di.Name == "Ok" && len(di.Fields) == 1 {
							// Set monad context from left operand type
							oldContainer := eval.ContainerContext
							eval.ContainerContext = getRuntimeTypeName(result)
							defer func() { eval.ContainerContext = oldContainer }()
							return eval.ApplyFunction(fn, []Object{di.Fields[0]})
						}
					}
					return newError("(>>=) for Result: expected Ok or Fail")
				},
			},
		},
	}
}

// ============================================================================
// Optional instances: isEmpty, unwrap, wrap for ?? and ?.
// ============================================================================

func registerOptionalInstances(e *Evaluator) {
	// Option<T>
	e.ClassImplementations["Optional"]["Option"] = &MethodTable{
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
					if isZeroValue(args[0]) {
						return TRUE
					}
					return FALSE
				},
			},
			"unwrap": &Builtin{
				Name: "unwrap",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "unwrap"); found {
						args = rest
					}
					if len(args) != 1 {
						return newError("unwrap expects 1 argument, got %d", len(args))
					}
					if di, ok := args[0].(*DataInstance); ok && di.Name == "Some" && len(di.Fields) == 1 {
						return di.Fields[0]
					}
					return newError("unwrap: expected Some, got Zero")
				},
			},
			"wrap": &Builtin{
				Name: "wrap",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "wrap"); found {
						args = rest
					}
					if len(args) != 1 {
						return newError("wrap expects 1 argument, got %d", len(args))
					}
					return makeSome(args[0])
				},
			},
		},
	}

	// Result<A, E>
	e.ClassImplementations["Optional"]["Result"] = &MethodTable{
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
			"unwrap": &Builtin{
				Name: "unwrap",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "unwrap"); found {
						args = rest
					}
					if len(args) != 1 {
						return newError("unwrap expects 1 argument, got %d", len(args))
					}
					if di, ok := args[0].(*DataInstance); ok && di.Name == "Ok" && len(di.Fields) == 1 {
						return di.Fields[0]
					}
					return newError("unwrap: expected Ok, got Fail")
				},
			},
			"wrap": &Builtin{
				Name: "wrap",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "wrap"); found {
						args = rest
					}
					if len(args) != 1 {
						return newError("wrap expects 1 argument, got %d", len(args))
					}
					return makeOk(args[0])
				},
			},
		},
	}
}

// Helper function for checking Zero
func isZeroValue(obj Object) bool {
	if di, ok := obj.(*DataInstance); ok {
		return di.Name == "Zero" && len(di.Fields) == 0
	}
	return false
}

// ============================================================================
// Identity instances
// ============================================================================

func registerIdentityInstances(e *Evaluator) {
	// Functor
	e.ClassImplementations["Functor"]["Identity"] = &MethodTable{
		Methods: map[string]Object{
			"fmap": &Builtin{
				Name: "fmap",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "fmap"); found {
						args = rest
					}
					f := args[0]
					idVal := args[1]
					di, ok := idVal.(*DataInstance)
					if !ok {
						return newError("fmap Identity: expected Identity")
					}
					res := eval.ApplyFunction(f, []Object{di.Fields[0]})
					if isError(res) {
						return res
					}
					return &DataInstance{Name: "identity", TypeName: "Identity", Fields: []Object{res}}
				},
			},
		},
	}
	// Applicative
	e.ClassImplementations["Applicative"]["Identity"] = &MethodTable{
		Methods: map[string]Object{
			"pure": &Builtin{
				Name: "pure",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "pure"); found {
						args = rest
					}
					// Handle placeholder/implicit witness removal if needed
					for len(args) > 1 {
						if _, ok := args[0].(*Dictionary); ok {
							args = args[1:]
						} else {
							break
						}
					}
					if len(args) != 1 {
						return newError("pure expects 1 argument")
					}
					return &DataInstance{Name: "identity", TypeName: "Identity", Fields: []Object{args[0]}}
				},
			},
			"(<*>)": &Builtin{
				Name: "(<*>)",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "(<*>)"); found {
						args = rest
					}
					idF := args[0]
					idX := args[1]
					diF, ok1 := idF.(*DataInstance)
					diX, ok2 := idX.(*DataInstance)
					if !ok1 || !ok2 {
						return newError("(<*>) Identity: expected Identity")
					}
					f := diF.Fields[0]
					x := diX.Fields[0]
					res := eval.ApplyFunction(f, []Object{x})
					if isError(res) {
						return res
					}
					return &DataInstance{Name: "identity", TypeName: "Identity", Fields: []Object{res}}
				},
			},
		},
	}
	// Monad
	e.ClassImplementations["Monad"]["Identity"] = &MethodTable{
		Methods: map[string]Object{
			"(>>=)": &Builtin{
				Name: "(>>=)",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "(>>=)"); found {
						args = rest
					}
					idVal := args[0]
					f := args[1]
					di, ok := idVal.(*DataInstance)
					if !ok {
						return newError("(>>=) Identity: expected Identity")
					}
					return eval.ApplyFunction(f, []Object{di.Fields[0]})
				},
			},
		},
	}
}

// ============================================================================
// State instances
// ============================================================================

func registerStateInstances(e *Evaluator) {
	// Functor
	e.ClassImplementations["Functor"]["State"] = &MethodTable{
		Methods: map[string]Object{
			"fmap": &Builtin{
				Name: "fmap",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "fmap"); found {
						args = rest
					}
					f := args[0]
					s := args[1]
					di, ok := s.(*DataInstance)
					if !ok {
						return newError("expected State")
					}
					runS := di.Fields[0]

					composed := &Builtin{
						Name: "fmap_state",
						Fn: func(ev *Evaluator, callArgs ...Object) Object {
							// callArgs[0] is state
							res := ev.ApplyFunction(runS, callArgs)
							if isError(res) {
								return res
							}
							tuple, ok := res.(*Tuple)
							if !ok {
								return newError("State function must return tuple")
							}

							val := tuple.Elements[0]
							newState := tuple.Elements[1]

							mappedVal := ev.ApplyFunction(f, []Object{val})
							if isError(mappedVal) {
								return mappedVal
							}

							return &Tuple{Elements: []Object{mappedVal, newState}}
						},
					}
					return &DataInstance{Name: "state", TypeName: "State", Fields: []Object{composed}}
				},
			},
		},
	}
	// Applicative
	e.ClassImplementations["Applicative"]["State"] = &MethodTable{
		Methods: map[string]Object{
			"pure": &Builtin{
				Name: "pure",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "pure"); found {
						args = rest
					}
					// Handle placeholder/implicit witness removal if needed
					for len(args) > 1 {
						if _, ok := args[0].(*Dictionary); ok {
							args = args[1:]
						} else {
							break
						}
					}
					if len(args) != 1 {
						return newError("pure expects 1 argument")
					}
					val := args[0]
					pureFn := &Builtin{
						Name: "pure_state",
						Fn: func(ev *Evaluator, callArgs ...Object) Object {
							return &Tuple{Elements: []Object{val, callArgs[0]}}
						},
					}
					return &DataInstance{Name: "state", TypeName: "State", Fields: []Object{pureFn}}
				},
			},
			"(<*>)": &Builtin{
				Name: "(<*>)",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "(<*>)"); found {
						args = rest
					}
					sf := args[0]
					sx := args[1]
					diF, ok1 := sf.(*DataInstance)
					diX, ok2 := sx.(*DataInstance)
					if !ok1 || !ok2 {
						return newError("expected State")
					}
					runF := diF.Fields[0]
					runX := diX.Fields[0]

					apFn := &Builtin{
						Name: "ap_state",
						Fn: func(ev *Evaluator, callArgs ...Object) Object {
							// s -> (f, s1)
							resF := ev.ApplyFunction(runF, callArgs)
							if isError(resF) {
								return resF
							}
							tF, ok := resF.(*Tuple)
							if !ok {
								return newError("State return tuple")
							}
							f := tF.Elements[0]
							s1 := tF.Elements[1]

							// s1 -> (x, s2)
							resX := ev.ApplyFunction(runX, []Object{s1})
							if isError(resX) {
								return resX
							}
							tX, ok := resX.(*Tuple)
							if !ok {
								return newError("State return tuple")
							}
							x := tX.Elements[0]
							s2 := tX.Elements[1]

							// f(x)
							applied := ev.ApplyFunction(f, []Object{x})
							if isError(applied) {
								return applied
							}

							return &Tuple{Elements: []Object{applied, s2}}
						},
					}
					return &DataInstance{Name: "state", TypeName: "State", Fields: []Object{apFn}}
				},
			},
		},
	}
	// Monad
	e.ClassImplementations["Monad"]["State"] = &MethodTable{
		Methods: map[string]Object{
			"(>>=)": &Builtin{
				Name: "(>>=)",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "(>>=)"); found {
						args = rest
					}
					s := args[0]
					f := args[1]
					di, ok := s.(*DataInstance)
					if !ok {
						return newError("expected State")
					}
					runS := di.Fields[0]

					bindFn := &Builtin{
						Name: "bind_state",
						Fn: func(ev *Evaluator, callArgs ...Object) Object {
							res := ev.ApplyFunction(runS, callArgs)
							if isError(res) {
								return res
							}
							t, ok := res.(*Tuple)
							if !ok {
								return newError("State return tuple")
							}
							a := t.Elements[0]
							s1 := t.Elements[1]

							mb := ev.ApplyFunction(f, []Object{a})
							if isError(mb) {
								return mb
							}
							diMB, ok := mb.(*DataInstance)
							if !ok {
								return newError("bind callback must return State")
							}
							runMB := diMB.Fields[0]

							return ev.ApplyFunction(runMB, []Object{s1})
						},
					}
					return &DataInstance{Name: "state", TypeName: "State", Fields: []Object{bindFn}}
				},
			},
		},
	}
}

// ============================================================================
// Writer instances
// ============================================================================

func registerWriterInstances(e *Evaluator) {
	// Functor
	e.ClassImplementations["Functor"]["Writer"] = &MethodTable{
		Methods: map[string]Object{
			"fmap": &Builtin{
				Name: "fmap",
				Fn: func(eval *Evaluator, args ...Object) Object {
					if _, rest, found := extractWitnessMethod(args, "fmap"); found {
						args = rest
					}
					f := args[0]
					w := args[1]
					di, ok := w.(*DataInstance)
					if !ok {
						return newError("expected Writer")
					}
					val := di.Fields[0]
					log := di.Fields[1]

					mapped := eval.ApplyFunction(f, []Object{val})
					if isError(mapped) {
						return mapped
					}

					return &DataInstance{Name: "writer", TypeName: "Writer", Fields: []Object{mapped, log}}
				},
			},
		},
	}
	// Applicative - pure is skipped or errors
	e.ClassImplementations["Applicative"]["Writer"] = &MethodTable{
		Methods: map[string]Object{
			"pure": &Builtin{
				Name: "pure",
				Fn: func(eval *Evaluator, args ...Object) Object {
					// pure(x) -> Writer(x, mempty)
					var mEmpty Object
					if m, rest, found := extractWitnessMethod(args, "mempty"); found {
						mEmpty = m
						args = rest
					}

					// Handle placeholder/implicit witness removal if needed
					for len(args) > 1 {
						if _, ok := args[0].(*Dictionary); ok {
							args = args[1:]
						} else {
							break
						}
					}

					if len(args) != 1 {
						return newError("pure expects 1 argument")
					}

					if mEmpty != nil {
						logVal := eval.ApplyFunction(mEmpty, []Object{})
						if isError(logVal) {
							return logVal
						}
						return &DataInstance{Name: "writer", TypeName: "Writer", Fields: []Object{args[0], logVal}}
					}

					// Need to find mempty for the Log type W from Writer<W, A>

					var logType typesystem.Type

					// 1. Check Witness (Stack-based - Proposal 002)
					if ts := eval.GetWitness("Applicative"); len(ts) > 0 {
						t := ts[0]
						if tApp, ok := t.(typesystem.TApp); ok {
							if len(tApp.Args) >= 1 {
								logType = tApp.Args[0]
							}
						}
					}

					// Legacy AST annotation check (fallback)
					if logType == nil {
						if callNode, ok := eval.CurrentCallNode.(*ast.CallExpression); ok {
							if witnesses, ok := callNode.Witness.(map[string][]typesystem.Type); ok {
								if witnessTypes, ok := witnesses["Applicative"]; ok && len(witnessTypes) > 0 {
									witnessType := witnessTypes[0]
									if tApp, ok := witnessType.(typesystem.TApp); ok {
										if len(tApp.Args) >= 2 {
											logType = tApp.Args[0]
										}
									}
								}
							}
						}
					}

					// Proposal 002: TypeMap fallback (Analyzer-inferred type)
					if logType == nil && eval.TypeMap != nil && eval.CurrentCallNode != nil {
						if t := eval.TypeMap[eval.CurrentCallNode]; t != nil {
							if tApp, ok := t.(typesystem.TApp); ok {
								// Writer<W, A> has 2 type arguments
								if len(tApp.Args) >= 2 {
									logType = tApp.Args[0]
								}
							}
						}
					}

					if logType != nil {
						// Unwrap type aliases (e.g., IntList = List<Int>) to get the actual type
						// for trait method lookup

						// 1. Try typesystem unwrap (if UnderlyingType is set)
						unwrappedLogType := typesystem.UnwrapUnderlying(logType)
						if unwrappedLogType != nil {
							logType = unwrappedLogType
						}

						// 2. Try Evaluator.TypeAliases (if TypeMap didn't have full info)
						if tCon, ok := logType.(typesystem.TCon); ok {
							if alias, ok := eval.TypeAliases[tCon.Name]; ok {
								logType = alias
								// Recursive unwrap using typesystem util
								if unwrapped := typesystem.UnwrapUnderlying(logType); unwrapped != nil {
									logType = unwrapped
								}
							}
						}

						logTypeName := ExtractTypeConstructorName(logType)

						// 2. Find Monoid.mempty for W
						memptyMethod, found := eval.lookupTraitMethod("Monoid", "mempty", logTypeName)
						if found {
							// 3. Call mempty()
							// Push witness for recursive call if needed (W's mempty might depend on it)
							// Actually mempty is nullary, but if W is generic (e.g. List<T>), it might need info.
							// For simple types, no witness needed.
							// But we should push witness for W: Monoid constraint?
							// mempty usually doesn't take args or witnesses for simple types.

							// Just call it
							logVal := eval.ApplyFunction(memptyMethod, []Object{})
							if isError(logVal) {
								return logVal
							}

							return &DataInstance{Name: "writer", TypeName: "Writer", Fields: []Object{args[0], logVal}}
						}
					}

					// Explicit error instead of fallback (Proposal 002 recommendation)
					return newError("pure for Writer requires witness to determine Log Monoid type")
				},
			},
			"(<*>)": &Builtin{
				Name: "(<*>)",
				Fn: func(eval *Evaluator, args ...Object) Object {
					var mConcat Object
					if m, rest, found := extractWitnessMethod(args, "(<>)"); found {
						mConcat = m
						args = rest
					}

					if len(args) != 2 {
						return newError("(<*>) expects 2 arguments")
					}
					wf := args[0]
					wx := args[1]
					diF, ok1 := wf.(*DataInstance)
					diX, ok2 := wx.(*DataInstance)
					if !ok1 || !ok2 {
						return newError("expected Writer")
					}
					f := diF.Fields[0]
					log1 := diF.Fields[1]
					x := diX.Fields[0]
					log2 := diX.Fields[1]

					// Apply f(x)
					res := eval.ApplyFunction(f, []Object{x})
					if isError(res) {
						return res
					}

					// Combine logs: log1 <> log2
					var concatFn Object
					if mConcat != nil {
						concatFn = mConcat
					} else {
						var ok bool
						concatFn, ok = eval.GlobalEnv.Get("(<>)")
						if !ok {
							return newError("(<>) not found")
						}
					}

					newLog := eval.ApplyFunction(concatFn, []Object{log1, log2})
					if isError(newLog) {
						return newLog
					}

					return &DataInstance{Name: "writer", TypeName: "Writer", Fields: []Object{res, newLog}}
				},
			},
		},
	}
	// Monad
	e.ClassImplementations["Monad"]["Writer"] = &MethodTable{
		Methods: map[string]Object{
			"(>>=)": &Builtin{
				Name: "(>>=)",
				Fn: func(eval *Evaluator, args ...Object) Object {
					var mConcat Object
					if m, rest, found := extractWitnessMethod(args, "(<>)"); found {
						mConcat = m
						args = rest
					}

					if len(args) != 2 {
						return newError("expected Writer")
					}
					w := args[0]
					f := args[1]
					di, ok := w.(*DataInstance)
					if !ok {
						return newError("expected Writer")
					}
					val := di.Fields[0]
					log1 := di.Fields[1]

					// f(val) -> Writer(b, log2)
					res := eval.ApplyFunction(f, []Object{val})
					if isError(res) {
						return res
					}
					diRes, ok := res.(*DataInstance)
					if !ok || diRes.TypeName != "Writer" {
						return newError("bind callback must return Writer")
					}

					val2 := diRes.Fields[0]
					log2 := diRes.Fields[1]

					// Combine logs: log1 <> log2
					var concatFn Object
					if mConcat != nil {
						concatFn = mConcat
					} else {
						var ok bool
						concatFn, ok = eval.GlobalEnv.Get("(<>)")
						if !ok {
							return newError("(<>) not found")
						}
					}

					newLog := eval.ApplyFunction(concatFn, []Object{log1, log2})
					if isError(newLog) {
						return newLog
					}

					return &DataInstance{Name: "writer", TypeName: "Writer", Fields: []Object{val2, newLog}}
				},
			},
		},
	}
}

// ============================================================================
// OptionT instances
// Wraps M<Option<A>>
// ============================================================================

func registerOptionTInstances(e *Evaluator) {
	// Helper to extract inner monad and value
	getInner := func(obj Object) (Object, bool) {
		if di, ok := obj.(*DataInstance); ok {
			if di.TypeName == "OptionT" && len(di.Fields) == 1 {
				return di.Fields[0], true
			}
		}
		return nil, false
	}

	// Functor: fmap f (OptionT m)
	e.ClassImplementations["Functor"]["OptionT"] = &MethodTable{
		Methods: map[string]Object{
			"fmap": &Builtin{
				Name: "fmap",
				Fn: func(eval *Evaluator, args ...Object) Object {
					var mFmap Object
					if m, rest, found := extractWitnessMethod(args, "fmap"); found {
						mFmap = m
						args = rest
					}

					if len(args) != 2 {
						return newError("fmap expects 2 arguments (plus optional witness), got %d", len(args))
					}
					f := args[0]
					ot := args[1]
					m, ok := getInner(ot)
					if !ok {
						return newError("expected OptionT")
					}

					// Look up fmap for Option (to use inside)
					optFmap, found := eval.GlobalEnv.Get("fmap")
					if !found {
						return newError("fmap not found")
					}

					// Inner mapper: \opt -> fmap(f, opt)
					innerMapper := &Builtin{
						Name: "optiont_inner_map",
						Fn: func(ev *Evaluator, callArgs ...Object) Object {
							// callArgs[0] is Option<A>
							return ev.ApplyFunction(optFmap, []Object{f, callArgs[0]})
						},
					}

					// Outer mapper: fmap(innerMapper, m)
					var outerFmap Object
					if mFmap != nil {
						outerFmap = mFmap
					} else {
						mType := getRuntimeTypeName(m)
						var found bool
						outerFmap, found = eval.lookupTraitMethod("Functor", "fmap", mType)
						if !found {
							outerFmap, found = eval.GlobalEnv.Get("fmap")
							if !found {
								return newError("fmap not found for inner type %s", mType)
							}
						}
					}

					// Clear CurrentCallNode and ContainerContext for inner dispatch
					oldNode := eval.CurrentCallNode
					oldContainer := eval.ContainerContext
					oldWitnessStack := eval.WitnessStack
					eval.CurrentCallNode = nil
					eval.ContainerContext = ""
					eval.WitnessStack = nil
					defer func() {
						eval.CurrentCallNode = oldNode
						eval.ContainerContext = oldContainer
						eval.WitnessStack = oldWitnessStack
					}()

					newM := eval.ApplyFunction(outerFmap, []Object{innerMapper, m})
					if isError(newM) {
						return newM
					}

					return &DataInstance{Name: "OptionT", TypeName: "OptionT", Fields: []Object{newM}}
				},
			},
		},
	}

	// Applicative
	e.ClassImplementations["Applicative"]["OptionT"] = &MethodTable{
		Methods: map[string]Object{
			"pure": &Builtin{
				Name: "pure",
				Fn: func(eval *Evaluator, args ...Object) Object {
					val := args[0]

					// Helper to extract Monad type from Witness or Hint
					var mType typesystem.Type

					// 1. Check Witness (Stack-based - Proposal 002)
					if ts := eval.GetWitness("Applicative"); len(ts) > 0 {
						t := ts[0]
						if tApp, ok := t.(typesystem.TApp); ok {
							if len(tApp.Args) >= 1 {
								mType = tApp.Args[0]
							}
						}
					}

					// Legacy AST annotation check (fallback)
					if mType == nil {
						if callNode, ok := eval.CurrentCallNode.(*ast.CallExpression); ok {
							if witnesses, ok := callNode.Witness.(map[string][]typesystem.Type); ok {
								if witnessTypes, ok := witnesses["Applicative"]; ok && len(witnessTypes) > 0 {
									witnessType := witnessTypes[0]
									// witnessType is OptionT<M>
									if tApp, ok := witnessType.(typesystem.TApp); ok {
										if len(tApp.Args) >= 1 {
											mType = tApp.Args[0]
										}
									}
								}
							}
						}
					}

					// Proposal 002: TypeMap fallback (Analyzer-inferred type)
					if mType == nil && eval.TypeMap != nil && eval.CurrentCallNode != nil {
						if t := eval.TypeMap[eval.CurrentCallNode]; t != nil {
							if tApp, ok := t.(typesystem.TApp); ok {
								// Check if it matches OptionT constructor
								// We assume the type is OptionT because we are inside OptionT.pure
								if len(tApp.Args) >= 1 {
									mType = tApp.Args[0]
								}
							}
						}
					}

					if mType != nil {
						// Construct Some(val)
						someVal := makeSome(val)
						// Call pure for M
						mTypeName := ExtractTypeConstructorName(mType)
						pureMethod, found := eval.lookupTraitMethod("Applicative", "pure", mTypeName)
						if found {
							// Push witness for inner monad M if it's generic?
							// M itself is a type (e.g. Identity, or List).
							// If M is e.g. Writer<Log>, we need to push Applicative witness for Writer.
							// The mType IS the witness for M's Applicative instance.
							// So we should push { "Applicative": mType }?
							// Actually, if M is Writer<L>, mType is TApp(Writer, [L]).
							// That matches what Writer.pure expects.

							eval.PushWitness(map[string][]typesystem.Type{"Applicative": {mType}})
							defer eval.PopWitness()

							mVal := eval.ApplyFunction(pureMethod, []Object{someVal})
							if isError(mVal) {
								return mVal
							}

							return &DataInstance{Name: "OptionT", TypeName: "OptionT", Fields: []Object{mVal}}
						}
					}
					return newError("pure for OptionT requires witness to determine inner Monad")
				},
			},
			"(<*>)": &Builtin{
				Name: "(<*>)",
				Fn: func(eval *Evaluator, args ...Object) Object {
					// OptionT mf <*> OptionT mx
					// We need M to be Monad (or at least Applicative)
					// newM = mf >>= \fOpt -> mx >>= \xOpt -> pure(fOpt <*> xOpt)
					// Or if M is just Applicative: lift2 (<*>) mf mx

					// Simplified: assume M is Monad for easier implementation via bind?
					// Or use lift2 if we can dispatch it.
					// Let's assume M implements Applicative.

					mf, ok1 := getInner(args[0])
					mx, ok2 := getInner(args[1])
					if !ok1 || !ok2 {
						return newError("expected OptionT")
					}

					// We want to combine values inside M<Option>.
					// This requires lift2 over M with a function that combines Options.

					// OptCombiner: \fOpt xOpt -> fOpt <*> xOpt
					optAp, found := eval.GlobalEnv.Get("(<*>)")
					if !found {
						return newError("(<*>) not found")
					}

					combiner := &Builtin{
						Name: "option_combiner",
						Fn: func(ev *Evaluator, callArgs ...Object) Object {
							return ev.ApplyFunction(optAp, callArgs)
						},
					}

					// Lift2 over M: lift2(combiner, mf, mx)
					// lift2 f x y = fmap f x <*> y

					// 1. fmap combiner mf
					mType := getRuntimeTypeName(mf)
					mFmap, found := eval.lookupTraitMethod("Functor", "fmap", mType)
					if !found {
						// Fallback
						var foundGlobal bool
						mFmap, foundGlobal = eval.GlobalEnv.Get("fmap")
						if !foundGlobal {
							return newError("fmap not found for inner type %s", mType)
						}
					}

					// Clear context for inner dispatch
					oldNode := eval.CurrentCallNode
					oldContainer := eval.ContainerContext
					oldWitnessStack := eval.WitnessStack
					eval.CurrentCallNode = nil
					eval.ContainerContext = ""
					eval.WitnessStack = nil
					defer func() {
						eval.CurrentCallNode = oldNode
						eval.ContainerContext = oldContainer
						eval.WitnessStack = oldWitnessStack
					}()

					mPartial := eval.ApplyFunction(mFmap, []Object{combiner, mf})
					if isError(mPartial) {
						return mPartial
					}

					// 2. mPartial <*> mx
					mAp, found := eval.lookupTraitMethod("Applicative", "(<*>)", mType)
					if !found {
						// Fallback
						var foundGlobal bool
						mAp, foundGlobal = eval.GlobalEnv.Get("(<*>)")
						if !foundGlobal {
							return newError("(<*>) not found for inner type %s", mType)
						}
					}

					newM := eval.ApplyFunction(mAp, []Object{mPartial, mx})
					if isError(newM) {
						return newM
					}

					return &DataInstance{Name: "OptionT", TypeName: "OptionT", Fields: []Object{newM}}
				},
			},
		},
	}

	// Monad
	e.ClassImplementations["Monad"]["OptionT"] = &MethodTable{
		Methods: map[string]Object{
			"(>>=)": &Builtin{
				Name: "(>>=)",
				Fn: func(eval *Evaluator, args ...Object) Object {
					// Check for explicit dictionary witness (Proposal 002)
					// If passed explicitly, consume it.
					if len(args) > 0 {
						if _, ok := args[0].(*Dictionary); ok {
							args = args[1:]
						}
					}

					var witnessType typesystem.Type
					// Try to get witness from stack (pushed by CallExpression or InfixExpression)
					if ts := eval.GetWitness("Monad"); len(ts) > 0 {
						witnessType = ts[0]
					} else if ts := eval.GetWitness("Applicative"); len(ts) > 0 {
						witnessType = ts[0]
					}

					ot := args[0]
					f := args[1] // A -> OptionT M B
					m, ok := getInner(ot)
					if !ok {
						return newError("expected OptionT in bind, got %s", ot.Inspect())
					}

					// m >>= \opt -> match opt { Some a -> run (f a); Zero -> pure Zero }

					// We need (>>=) for M
					mType := getRuntimeTypeName(m)
					mBind, found := eval.lookupTraitMethod("Monad", "(>>=)", mType)
					if !found {
						// Fallback: try finding generic operator in global env if trait lookup fails
						// This might happen if M is not formally a Monad instance but has bind-like behavior?
						// But for OptionT, we require M to be Monad.
						var foundGlobal bool
						mBind, foundGlobal = eval.GlobalEnv.Get("(>>=)")
						if !foundGlobal {
							return newError("(>>=) not found for inner monad %s", mType)
						}
					}

					// We also need pure for M (to wrap Zero)
					mPure, found := eval.lookupTraitMethod("Applicative", "pure", mType)
					if !found {
						return newError("pure not found for inner monad %s", mType)
					}

					bindFn := &Builtin{
						Name: "optiont_bind",
						Fn: func(ev *Evaluator, callArgs ...Object) Object {
							// callArgs[0] is Option<A>
							opt := callArgs[0]

							if isZeroValue(opt) {
								// pure(Zero)
								return ev.ApplyFunction(mPure, []Object{makeZero()})
							}

							// Some(a) -> runOptionT(f(a))
							if di, ok := opt.(*DataInstance); ok && di.Name == "Some" && len(di.Fields) == 1 {
								a := di.Fields[0]

								// Push Witness if available (critical for nested/complex HKTs like pure inside lambda)
								if witnessType != nil {
									w := map[string][]typesystem.Type{
										"Applicative": {witnessType},
										"Monad":       {witnessType},
									}
									ev.PushWitness(w)
									defer ev.PopWitness()
								}

								res := ev.ApplyFunction(f, []Object{a}) // OptionT M B
								if isError(res) {
									return res
								}

								// Unwrap
								inner, ok := getInner(res)
								if !ok {
									return newError("callback must return OptionT")
								}
								return inner
							}

							return newError("invalid Option value")
						},
					}

					// Clear CurrentCallNode and ContainerContext for inner dispatch to avoid context pollution
					oldNode := eval.CurrentCallNode
					oldContainer := eval.ContainerContext
					oldWitnessStack := eval.WitnessStack
					eval.CurrentCallNode = nil
					eval.ContainerContext = ""
					eval.WitnessStack = nil
					defer func() {
						eval.CurrentCallNode = oldNode
						eval.ContainerContext = oldContainer
						eval.WitnessStack = oldWitnessStack
					}()

					newM := eval.ApplyFunction(mBind, []Object{m, bindFn})
					if isError(newM) {
						return newM
					}

					return &DataInstance{Name: "OptionT", TypeName: "OptionT", Fields: []Object{newM}}
				},
			},
		},
	}
}

// ============================================================================
// ResultT instances
// Wraps M<Result<A, E>>
// ============================================================================

func registerResultTInstances(e *Evaluator) {
	getInner := func(obj Object) (Object, bool) {
		if di, ok := obj.(*DataInstance); ok && di.TypeName == "ResultT" && len(di.Fields) == 1 {
			return di.Fields[0], true
		}
		return nil, false
	}

	// Functor
	e.ClassImplementations["Functor"]["ResultT"] = &MethodTable{
		Methods: map[string]Object{
			"fmap": &Builtin{
				Name: "fmap",
				Fn: func(eval *Evaluator, args ...Object) Object {
					var mFmap Object
					if m, rest, found := extractWitnessMethod(args, "fmap"); found {
						mFmap = m
						args = rest
					}

					if len(args) != 2 {
						return newError("fmap expects 2 arguments, got %d", len(args))
					}
					f := args[0]
					rt := args[1]
					m, ok := getInner(rt)
					if !ok {
						return newError("expected ResultT")
					}

					resFmap, found := eval.GlobalEnv.Get("fmap")
					if !found {
						return newError("fmap not found")
					}

					innerMapper := &Builtin{
						Name: "resultt_inner_map",
						Fn: func(ev *Evaluator, callArgs ...Object) Object {
							obj := callArgs[0]
							return ev.ApplyFunction(resFmap, []Object{f, obj})
						},
					}

					var outerFmap Object
					if mFmap != nil {
						outerFmap = mFmap
					} else {
						mType := getRuntimeTypeName(m)
						var found bool
						outerFmap, found = eval.lookupTraitMethod("Functor", "fmap", mType)
						if !found {
							outerFmap, found = eval.GlobalEnv.Get("fmap")
							if !found {
								return newError("fmap not found for inner type %s", mType)
							}
						}
					}

					// Clear CurrentCallNode and ContainerContext for inner dispatch
					oldNode := eval.CurrentCallNode
					oldContainer := eval.ContainerContext
					oldWitnessStack := eval.WitnessStack
					eval.CurrentCallNode = nil
					eval.ContainerContext = ""
					eval.WitnessStack = nil
					defer func() {
						eval.CurrentCallNode = oldNode
						eval.ContainerContext = oldContainer
						eval.WitnessStack = oldWitnessStack
					}()

					newM := eval.ApplyFunction(outerFmap, []Object{innerMapper, m})
					if isError(newM) {
						return newM
					}

					return &DataInstance{Name: "ResultT", TypeName: "ResultT", Fields: []Object{newM}}
				},
			},
		},
	}

	// Applicative (skipping pure logic for now as it needs type hint, similar to OptionT)
	e.ClassImplementations["Applicative"]["ResultT"] = &MethodTable{
		Methods: map[string]Object{
			"pure": &Builtin{
				Name: "pure",
				Fn: func(eval *Evaluator, args ...Object) Object {
					var mPure Object
					if m, rest, found := extractWitnessMethod(args, "pure"); found {
						mPure = m
						args = rest
					}

					if len(args) != 1 {
						return newError("pure expects 1 argument (plus optional witness), got %d", len(args))
					}

					if mPure != nil {
						okVal := makeOk(args[0])
						mVal := eval.ApplyFunction(mPure, []Object{okVal})
						if isError(mVal) {
							return mVal
						}
						return &DataInstance{Name: "ResultT", TypeName: "ResultT", Fields: []Object{mVal}}
					}

					var mType typesystem.Type

					// 1. Check Witness (Stack-based - Proposal 002)
					if ts := eval.GetWitness("Applicative"); len(ts) > 0 {
						t := ts[0]
						if tApp, ok := t.(typesystem.TApp); ok {
							if len(tApp.Args) >= 1 {
								mType = tApp.Args[0]
							}
						}
					}

					// Legacy AST annotation check (fallback)
					if mType == nil {
						if callNode, ok := eval.CurrentCallNode.(*ast.CallExpression); ok {
							if witnesses, ok := callNode.Witness.(map[string][]typesystem.Type); ok {
								if witnessTypes, ok := witnesses["Applicative"]; ok && len(witnessTypes) > 0 {
									witnessType := witnessTypes[0]
									// witnessType is ResultT<M, E>
									if tApp, ok := witnessType.(typesystem.TApp); ok {
										if len(tApp.Args) >= 1 {
											mType = tApp.Args[0]
										}
									}
								}
							}
						}
					}

					// Proposal 002: TypeMap fallback (Analyzer-inferred type)
					if mType == nil && eval.TypeMap != nil && eval.CurrentCallNode != nil {
						if t := eval.TypeMap[eval.CurrentCallNode]; t != nil {
							if tApp, ok := t.(typesystem.TApp); ok {
								// ResultT<M, E, A> has 3 args? Or 2 args + constructor?
								// ResultT is likely TApp(ResultT, [M, E, A])
								if len(tApp.Args) >= 1 {
									mType = tApp.Args[0]
								}
							}
						}
					}

					if mType != nil {
						okVal := makeOk(args[0])
						mTypeName := ExtractTypeConstructorName(mType)
						pureMethod, found := eval.lookupTraitMethod("Applicative", "pure", mTypeName)
						if found {
							eval.PushWitness(map[string][]typesystem.Type{"Applicative": {mType}})
							defer eval.PopWitness()

							mVal := eval.ApplyFunction(pureMethod, []Object{okVal})
							if isError(mVal) {
								return mVal
							}
							return &DataInstance{Name: "ResultT", TypeName: "ResultT", Fields: []Object{mVal}}
						}
					}
					return newError("pure for ResultT requires witness to determine inner Monad")
				},
			},
			"(<*>)": &Builtin{
				Name: "(<*>)",
				Fn: func(eval *Evaluator, args ...Object) Object {
					var mFmap, mAp Object
					if len(args) >= 1 {
						if dict, ok := args[0].(*Dictionary); ok {
							mFmap = FindMethodInDictionary(dict, "fmap")
							mAp = FindMethodInDictionary(dict, "(<*>)")
							if mFmap != nil || mAp != nil {
								args = args[1:]
							}
						}
					}

					if len(args) != 2 {
						return newError("expected 2 arguments")
					}

					mf, ok1 := getInner(args[0])
					mx, ok2 := getInner(args[1])
					if !ok1 || !ok2 {
						return newError("expected ResultT")
					}

					resAp, found := eval.GlobalEnv.Get("(<*>)")
					if !found {
						return newError("(<*>) not found")
					}

					combiner := &Builtin{
						Name: "result_combiner",
						Fn: func(ev *Evaluator, callArgs ...Object) Object {
							return ev.ApplyFunction(resAp, callArgs)
						},
					}

					if mFmap == nil {
						mType := getRuntimeTypeName(mf)
						var found bool
						mFmap, found = eval.lookupTraitMethod("Functor", "fmap", mType)
						if !found {
							mFmap, found = eval.GlobalEnv.Get("fmap")
							if !found {
								return newError("fmap not found for inner type %s", getRuntimeTypeName(mf))
							}
						}
					}

					// Clear context for inner dispatch
					oldNode := eval.CurrentCallNode
					oldContainer := eval.ContainerContext
					oldWitnessStack := eval.WitnessStack
					eval.CurrentCallNode = nil
					eval.ContainerContext = ""
					eval.WitnessStack = nil
					defer func() {
						eval.CurrentCallNode = oldNode
						eval.ContainerContext = oldContainer
						eval.WitnessStack = oldWitnessStack
					}()

					mPartial := eval.ApplyFunction(mFmap, []Object{combiner, mf})

					if mAp == nil {
						mType := getRuntimeTypeName(mf)
						var found bool
						mAp, found = eval.lookupTraitMethod("Applicative", "(<*>)", mType)
						if !found {
							mAp, found = eval.GlobalEnv.Get("(<*>)")
							if !found {
								return newError("(<*>) not found for inner type %s", mType)
							}
						}
					}

					newM := eval.ApplyFunction(mAp, []Object{mPartial, mx})

					if isError(newM) {
						return newM
					}
					return &DataInstance{Name: "ResultT", TypeName: "ResultT", Fields: []Object{newM}}
				},
			},
		},
	}

	// Monad
	e.ClassImplementations["Monad"]["ResultT"] = &MethodTable{
		Methods: map[string]Object{
			"(>>=)": &Builtin{
				Name: "(>>=)",
				Fn: func(eval *Evaluator, args ...Object) Object {
					var mBind, mPure Object
					if len(args) >= 1 {
						if dict, ok := args[0].(*Dictionary); ok {
							mBind = FindMethodInDictionary(dict, "(>>=)")
							mPure = FindMethodInDictionary(dict, "pure")
							if mBind != nil || mPure != nil {
								args = args[1:]
							}
						}
					}

					if len(args) != 2 {
						return newError("expected 2 arguments")
					}

					rt := args[0]
					f := args[1]
					m, ok := getInner(rt)
					if !ok {
						return newError("expected ResultT")
					}

					if mBind == nil {
						mType := getRuntimeTypeName(m)
						var found bool
						mBind, found = eval.lookupTraitMethod("Monad", "(>>=)", mType)
						if !found {
							mBind, found = eval.GlobalEnv.Get("(>>=)")
							if !found {
								return newError("(>>=) not found for inner monad %s", mType)
							}
						}
					}

					if mPure == nil {
						mType := getRuntimeTypeName(m)
						var found bool
						mPure, found = eval.lookupTraitMethod("Applicative", "pure", mType)
						if !found {
							return newError("inner monad operations not found")
						}
					}

					bindFn := &Builtin{
						Name: "resultt_bind",
						Fn: func(ev *Evaluator, callArgs ...Object) Object {
							res := callArgs[0]
							// Check Ok/Fail
							if di, ok := res.(*DataInstance); ok {
								if di.Name == "Fail" {
									// pure(Fail)
									return ev.ApplyFunction(mPure, []Object{res})
								}
								if di.Name == "Ok" && len(di.Fields) == 1 {
									val := di.Fields[0]
									next := ev.ApplyFunction(f, []Object{val})
									if isError(next) {
										return next
									}
									inner, ok := getInner(next)
									if !ok {
										return newError("callback must return ResultT")
									}
									return inner
								}
							}
							return newError("invalid Result value")
						},
					}

					// Clear CurrentCallNode and ContainerContext for inner dispatch
					oldNode := eval.CurrentCallNode
					oldContainer := eval.ContainerContext
					oldWitnessStack := eval.WitnessStack
					eval.CurrentCallNode = nil
					eval.ContainerContext = ""
					eval.WitnessStack = nil
					defer func() {
						eval.CurrentCallNode = oldNode
						eval.ContainerContext = oldContainer
						eval.WitnessStack = oldWitnessStack
					}()

					newM := eval.ApplyFunction(mBind, []Object{m, bindFn})
					if isError(newM) {
						return newM
					}
					return &DataInstance{Name: "ResultT", TypeName: "ResultT", Fields: []Object{newM}}
				},
			},
		},
	}
}
