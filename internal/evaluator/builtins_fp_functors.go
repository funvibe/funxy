package evaluator

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

	// Option<T>: fmap over Some, None stays None
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
					if isNoneValue(opt) {
						return makeNone()
					}
					// Extract value from Some
					if di, ok := opt.(*DataInstance); ok && di.Name == "Some" && len(di.Fields) == 1 {
						mapped := eval.ApplyFunction(fn, []Object{di.Fields[0]})
						if isError(mapped) {
							return mapped
						}
						return makeSome(mapped)
					}
					return newError("fmap for Option: expected Some or None")
				},
			},
		},
	}

	// Result<E, A>: fmap over Ok, Fail stays Fail
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
