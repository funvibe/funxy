package evaluator

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
					if isNoneValue(opt) {
						return makeNone()
					}
					if di, ok := opt.(*DataInstance); ok && di.Name == "Some" && len(di.Fields) == 1 {
						// Set monad context from left operand type
						oldContainer := eval.ContainerContext
						eval.ContainerContext = getRuntimeTypeName(opt)
						defer func() { eval.ContainerContext = oldContainer }()
						return eval.ApplyFunction(fn, []Object{di.Fields[0]})
					}
					return newError("(>>=) for Option: expected Some or None")
				},
			},
		},
	}

	// Result<E, A>: (>>=) = flatMap
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
