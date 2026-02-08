package evaluator

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
					// If either is None, result is None
					if isNoneValue(fnOpt) || isNoneValue(valOpt) {
						return makeNone()
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

	// Result<E, A>
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
