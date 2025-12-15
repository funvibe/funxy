package evaluator

// RegisterFPTraits registers all built-in FP traits and their instances.
// FP traits (Semigroup, Monoid, Functor, Applicative, Monad, Fallback) are always available.
// This includes trait methods: (<>), mempty, fmap, pure, (<*>), (>>=), (??)
func RegisterFPTraits(e *Evaluator, env *Environment) {
	// Initialize ClassImplementations maps
	for _, traitName := range []string{"Empty", "Semigroup", "Monoid", "Functor", "Applicative", "Monad", "Optional"} {
		if _, ok := e.ClassImplementations[traitName]; !ok {
			e.ClassImplementations[traitName] = make(map[string]Object)
		}
	}

	// Register trait methods as ClassMethod dispatchers
	// Arity: 0 = nullary (auto-call in type context), 1+ = needs explicit call

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

	// Register instances for built-in types
	registerEmptyInstances(e)
	registerSemigroupInstances(e)
	registerMonoidInstances(e)
	registerFunctorInstances(e)
	registerApplicativeInstances(e)
	registerMonadInstances(e)
	registerOptionalInstances(e)
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
					if len(args) != 1 {
						return newError("pure expects 1 argument, got %d", len(args))
					}
					return newList([]Object{args[0]})
				},
			},
			"(<*>)": &Builtin{
				Name: "(<*>)",
				Fn: func(eval *Evaluator, args ...Object) Object {
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
					if len(args) != 1 {
						return newError("pure expects 1 argument, got %d", len(args))
					}
					return makeSome(args[0])
				},
			},
			"(<*>)": &Builtin{
				Name: "(<*>)",
				Fn: func(eval *Evaluator, args ...Object) Object {
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
					if len(args) != 1 {
						return newError("pure expects 1 argument, got %d", len(args))
					}
					return makeOk(args[0])
				},
			},
			"(<*>)": &Builtin{
				Name: "(<*>)",
				Fn: func(eval *Evaluator, args ...Object) Object {
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

// SetFPBuiltinTypes sets type info for FP builtins (currently none)
func SetFPBuiltinTypes(builtins map[string]*Builtin) {
	// FP module uses traits, not standalone functions
}
