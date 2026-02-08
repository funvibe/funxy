package analyzer

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
	"sort"
)

// inferCallExpression infers the type of a call expression
func inferCallExpression(ctx *InferenceContext, n *ast.CallExpression, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	// Infer function type
	fnType, s1, err := inferFn(n.Function, table)
	if err != nil {
		return nil, nil, err
	}
	totalSubst := s1
	fnType = fnType.Apply(totalSubst)

	// Resolve type aliases (e.g., type Observer = (Int) -> Nil)
	// Use table to look up alias definitions
	fnType = table.ResolveTypeAlias(fnType)

	// Handle Type Application (e.g. List(Int)) or Constructor Call (e.g. Point({x:1}))
	if tType, ok := fnType.(typesystem.TType); ok {
		var typeArgs []typesystem.Type
		var valArgs []typesystem.Type
		isConstruction := false

		// Analyze arguments to determine if it's type application or construction
		for i, arg := range n.Arguments {
			argType, sArg, err := inferFn(arg, table)
			if err != nil {
				return nil, nil, err
			}
			totalSubst = sArg.Compose(totalSubst)

			// Resolve aliases to see if it's a type (TType)
			resolved := table.ResolveTypeAlias(argType)

			if tArg, ok := resolved.(typesystem.TType); ok && !isConstruction {
				typeArgs = append(typeArgs, tArg.Type)
			} else {
				// Encountered a value (or we already decided it's construction)
				if i == 0 {
					isConstruction = true
				} else if !isConstruction {
					// Mixed types and values? e.g. List(Int, 1) -> invalid Type App.
					// But could be construction if first arg was Type?
					// Usually Type<Arg> or Type(Arg).
					// If Type(Type), it's TypeApp.
					// If Type(Value), it's Construction.
					// If Type(Type, Value)? Invalid.
					return nil, nil, inferErrorf(arg, "mixed type and value arguments not supported in type application")
				}
				valArgs = append(valArgs, argType)
			}
		}

		if isConstruction {
			// Construction / Cast: Point({x:1}) -> Point
			targetType := tType.Type

			// If target is TCon (alias), resolving it might give TRecord.
			// But we want to return TCon (nominal type).

			// If single argument, unify directly
			if len(valArgs) == 1 {
				// Unify target type with argument type
				// e.g. Point vs {x:1}
				// Point (Alias) unwraps to {x:int}. Unify works.
				// Returns Point.

				// Resolve target alias just for unification check (UnifyWithResolver does this)
				// But we want to keep targetType as result.

				subst, err := typesystem.UnifyAllowExtraWithResolver(targetType, valArgs[0], table)
				if err != nil {
					return nil, nil, inferErrorf(n, "constructor type mismatch: expected %s, got %s", targetType, valArgs[0])
				}
				totalSubst = subst.Compose(totalSubst)
				return targetType.Apply(totalSubst), totalSubst, nil
			} else {
				// Multiple arguments: unify with Tuple?
				// e.g. Point(1, 2) vs Point={x:Int, y:Int} ? No, Point is Record.
				// Point(1, 2) only works if Point is Tuple alias.
				tupleArg := typesystem.TTuple{Elements: valArgs}
				subst, err := typesystem.UnifyWithResolver(targetType, tupleArg, table)
				if err != nil {
					return nil, nil, inferErrorf(n, "constructor arguments mismatch: %s vs %s", targetType, tupleArg)
				}
				totalSubst = subst.Compose(totalSubst)
				return targetType.Apply(totalSubst), totalSubst, nil
			}
		}

		return typesystem.TType{Type: typesystem.TApp{Constructor: tType.Type, Args: typeArgs}}, totalSubst, nil
	} else if tFunc, ok := fnType.(typesystem.TFunc); ok {
		// Apply global subst to ensure we see constraints from previous statements
		tFunc = tFunc.Apply(ctx.GlobalSubst).(typesystem.TFunc)
		return inferCallWithFuncType(ctx, n, tFunc, totalSubst, table, inferFn)
	} else if tVar, ok := fnType.(typesystem.TVar); ok {
		// If fnType is a type variable, unify it with a function type
		// This allows: fun wrapper(f) { fun(x) -> f(x) }
		var paramTypes []typesystem.Type
		for _, arg := range n.Arguments {
			argType, sArg, err := inferFn(arg, table)
			if err != nil {
				return nil, nil, err
			}
			totalSubst = sArg.Compose(totalSubst)
			paramTypes = append(paramTypes, argType.Apply(totalSubst))
		}

		resultVar := ctx.FreshVar()
		expectedFnType := typesystem.TFunc{
			Params:     paramTypes,
			ReturnType: resultVar,
		}

		subst, err := typesystem.UnifyWithResolver(tVar, expectedFnType, table)
		if err != nil {
			return nil, nil, inferErrorf(n, "cannot call %s as a function with arguments %v", fnType, paramTypes)
		}
		totalSubst = subst.Compose(totalSubst)

		return resultVar.Apply(totalSubst), totalSubst, nil
	} else if tForall, ok := fnType.(typesystem.TForall); ok {
		// Handle call to ForallType (Rank-N instantiation at call site)
		// Instantiate the forall to get a fresh monomorphic type (likely TFunc)
		instantiated := InstantiateForall(ctx, tForall)

		if tFunc, ok := instantiated.(typesystem.TFunc); ok {
			// Apply global subst to ensure we see constraints from previous statements
			tFunc = tFunc.Apply(ctx.GlobalSubst).(typesystem.TFunc)
			return inferCallWithFuncType(ctx, n, tFunc, totalSubst, table, inferFn)
		} else {
			return nil, nil, inferErrorf(n, "instantiated forall type is not a function: %s", instantiated)
		}
	} else if tUnion, ok := fnType.(typesystem.TUnion); ok {
		// Union Callability: Check if any member is callable
		var callableMembers []typesystem.TFunc
		for _, member := range tUnion.Types {
			// Resolve aliases to find underlying type
			resolved := table.ResolveTypeAlias(member)
			if tf, ok := resolved.(typesystem.TFunc); ok {
				callableMembers = append(callableMembers, tf)
			}
		}

		if len(callableMembers) == 1 {
			// Exactly one callable member, use it
			// This enables "Magic DSL" patterns like SmartResult = VNode | BuilderFunc
			return inferCallWithFuncType(ctx, n, callableMembers[0], totalSubst, table, inferFn)
		} else if len(callableMembers) > 1 {
			return nil, nil, inferErrorf(n, "ambiguous call on union type %s: multiple callable members found", fnType)
		} else {
			return nil, nil, inferErrorf(n, "cannot call union type %s: no callable members", fnType)
		}
	} else {
		return nil, nil, inferErrorf(n, "cannot call non-function type: %s", fnType)
	}
}

// inferCallWithFuncType handles the logic when the function type is known to be TFunc
func inferCallWithFuncType(
	ctx *InferenceContext,
	n *ast.CallExpression,
	tFunc typesystem.TFunc,
	totalSubst typesystem.Subst,
	table *symbols.SymbolTable,
	inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error),
) (typesystem.Type, typesystem.Subst, error) {
	paramIdx := 0
	// ... logic from original TFunc block ...

	// Note: Function types from inferIdentifier are already instantiated.
	// We don't instantiate again here to keep TypeMap entries consistent.

	for i, arg := range n.Arguments {
		isSpread := false
		if _, ok := arg.(*ast.SpreadExpression); ok {
			isSpread = true
		}

		// Attempt to predict parameter type to guide inference (Rank-N / Contextual Typing)
		if !isSpread {
			// Basic positional logic to determine expected type
			threshold := len(tFunc.Params)
			if tFunc.IsVariadic {
				threshold--
			}
			if paramIdx < threshold {
				expected := tFunc.Params[paramIdx].Apply(totalSubst)
				ctx.ExpectedTypes[arg] = expected
			} else if tFunc.IsVariadic {
				expected := tFunc.Params[len(tFunc.Params)-1].Apply(totalSubst)
				ctx.ExpectedTypes[arg] = expected
			}
		}

		argType, sArg, err := inferFn(arg, table)
		if err != nil {
			return nil, nil, err
		}
		totalSubst = sArg.Compose(totalSubst)
		argType = argType.Apply(totalSubst)
		// Resolve type aliases for proper unification with trait method signatures
		argType = table.ResolveTypeAlias(argType)

		if isSpread {
			if tTuple, ok := argType.(typesystem.TTuple); ok {
				for _, elType := range tTuple.Elements {
					// Determine if we are in the variadic part
					threshold := len(tFunc.Params)
					if tFunc.IsVariadic {
						threshold--
					}

					if paramIdx >= threshold {
						if tFunc.IsVariadic {
							varType := tFunc.Params[len(tFunc.Params)-1].Apply(totalSubst)

							// Special case: isolated type variable logic (same as above)
							isIsolated := false
							if tvar, ok := tFunc.Params[len(tFunc.Params)-1].(typesystem.TVar); ok {
								isUsed := false
								retVars := tFunc.ReturnType.FreeTypeVariables()
								for _, v := range retVars {
									if v.Name == tvar.Name {
										isUsed = true
										break
									}
								}
								if !isUsed {
									isIsolated = true
								}
							} else if tvar, ok := varType.(typesystem.TVar); ok {
								isUsed := false
								retVars := tFunc.ReturnType.Apply(totalSubst).FreeTypeVariables()
								for _, v := range retVars {
									if v.Name == tvar.Name {
										isUsed = true
										break
									}
								}
								if !isUsed {
									isIsolated = true
								}
							}

							if isIsolated {
								// Skip unification
							} else {
								subst, err := typesystem.UnifyAllowExtraWithResolver(varType, elType, table)
								if err != nil {
									return nil, nil, inferErrorf(arg, "argument type mismatch (variadic): %s vs %s", varType, elType)
								}
								totalSubst = subst.Compose(totalSubst)
							}
						} else {
							return nil, nil, inferError(n, "too many arguments")
						}
					} else {
						paramType := tFunc.Params[paramIdx].Apply(totalSubst)
						subst, err := typesystem.UnifyAllowExtraWithResolver(paramType, elType, table)
						if err != nil {
							return nil, nil, inferErrorf(arg, "argument %d type mismatch: %s vs %s", paramIdx+1, paramType, elType)
						}
						totalSubst = subst.Compose(totalSubst)
					}
					paramIdx++
				}
			} else if isList(argType) {
				if !tFunc.IsVariadic {
					return nil, nil, inferError(arg, "cannot spread List into non-variadic function")
				}
				if paramIdx < len(tFunc.Params)-1 {
					return nil, nil, inferError(arg, "cannot spread List into fixed parameters (ambiguous length)")
				}

				listElemType := getListElementType(argType)
				varType := tFunc.Params[len(tFunc.Params)-1].Apply(totalSubst)

				subst, err := typesystem.UnifyAllowExtraWithResolver(varType, listElemType, table)
				if err != nil {
					return nil, nil, inferErrorf(arg, "spread argument element type mismatch: %s vs %s", varType, listElemType)
				}
				totalSubst = subst.Compose(totalSubst)

				paramIdx = len(tFunc.Params)

			} else if _, ok := argType.(typesystem.TVar); ok {
				if tFunc.IsVariadic && i == len(n.Arguments)-1 {
					paramIdx = len(tFunc.Params)
				} else {
					return nil, nil, inferError(arg, "unknown spread only allowed as last argument to variadic function")
				}
			} else {
				return nil, nil, inferError(arg, "spread argument must be tuple or list")
			}
		} else {
			// Determine if we are in the variadic part of arguments
			// If IsVariadic is true, the last parameter (index len-1) handles all remaining arguments
			threshold := len(tFunc.Params)
			if tFunc.IsVariadic {
				threshold--
			}

			if paramIdx >= threshold {
				if tFunc.IsVariadic {
					varType := tFunc.Params[len(tFunc.Params)-1].Apply(totalSubst)

					// Special case: if the variadic parameter type is a Type Variable
					// that does NOT appear in the return type (and wasn't unified earlier),
					// we can treat it as heterogeneous (instantiate fresh for each arg).
					// This supports builtins like print(...) and format(fmt, ...).
					isIsolated := false
					// Prefer checking the base parameter type to avoid accidental unification
					// from earlier arguments in the same call.
					if tvar, ok := tFunc.Params[len(tFunc.Params)-1].(typesystem.TVar); ok {
						isUsed := false
						retVars := tFunc.ReturnType.FreeTypeVariables()
						for _, v := range retVars {
							if v.Name == tvar.Name {
								isUsed = true
								break
							}
						}
						if !isUsed {
							isIsolated = true
						}
					} else if tvar, ok := varType.(typesystem.TVar); ok {
						// Fallback: check after substitution
						isUsed := false
						retVars := tFunc.ReturnType.Apply(totalSubst).FreeTypeVariables()
						for _, v := range retVars {
							if v.Name == tvar.Name {
								isUsed = true
								break
							}
						}
						if !isUsed {
							isIsolated = true
						}
					}

					if isIsolated {
						// Heterogeneous: don't unify with varType, just ensure it's a valid type
						// But wait, we still need to infer the argument's type.
						// And we don't need to update totalSubst with a binding for 'a' that constrains future args.
						// Actually, we can just skip unification if it's isolated.
						// But we should verify constraints if any?
						// For now, simple skipping.
					} else {
						subst, err := typesystem.UnifyAllowExtraWithResolver(varType, argType, table)
						if err != nil {
							return nil, nil, inferErrorf(arg, "argument type mismatch (variadic): %s vs %s", varType, argType)
						}
						totalSubst = subst.Compose(totalSubst)
					}
				} else {
					return nil, nil, inferError(n, "too many arguments")
				}
			} else {
				paramType := tFunc.Params[paramIdx].Apply(totalSubst)
				// Resolve type aliases in parameter type for proper unification
				// This handles cases where parameter is a type alias (e.g., pkg.Handler)
				paramType = table.ResolveTypeAlias(paramType)

				subst, err := typesystem.UnifyAllowExtraWithResolver(paramType, argType, table)
				if err != nil {
					return nil, nil, inferErrorf(arg, "argument %d type mismatch: (%s) vs %s", paramIdx+1, paramType, argType)
				}
				totalSubst = subst.Compose(totalSubst)
			}
			paramIdx++
		}
	}

	fixedCount := len(tFunc.Params)
	if tFunc.IsVariadic {
		fixedCount--
	}
	// Account for default parameters
	requiredCount := fixedCount - tFunc.DefaultCount

	// Partial Application: if fewer arguments than required, return a function type
	// representing the remaining parameters
	if paramIdx < requiredCount {
		// Build remaining parameters (skip already applied)
		remainingParams := make([]typesystem.Type, 0, len(tFunc.Params)-paramIdx)
		for i := paramIdx; i < len(tFunc.Params); i++ {
			remainingParams = append(remainingParams, tFunc.Params[i].Apply(totalSubst))
		}

		// Return a new function type with remaining params
		partialFuncType := typesystem.TFunc{
			Params:       remainingParams,
			ReturnType:   tFunc.ReturnType.Apply(totalSubst),
			IsVariadic:   tFunc.IsVariadic,
			DefaultCount: max(0, tFunc.DefaultCount-(fixedCount-paramIdx)),
			Constraints:  tFunc.Constraints,
		}
		return partialFuncType, totalSubst, nil
	}

	resultType := tFunc.ReturnType.Apply(totalSubst)

	// Determine expected return type (from return context or argument context).
	expectedReturnType, hasExpectedReturn := ctx.ExpectedReturnTypes[n]
	if !hasExpectedReturn {
		if exp, ok := ctx.ExpectedTypes[n]; ok {
			expectedReturnType = exp
			hasExpectedReturn = true
		}
	}

	// If this is a trait method that dispatches only on return type, ensure we have context.
	if ident, ok := n.Function.(*ast.Identifier); ok {
		if traitName, isTraitMethod := table.GetTraitForMethod(ident.Value); isTraitMethod {
			if sources, ok := table.GetTraitMethodDispatch(traitName, ident.Value); ok {
				requiresReturn := true
				for _, src := range sources {
					if src.Kind != typesystem.DispatchReturn {
						requiresReturn = false
						break
					}
				}
				if requiresReturn {
					if !hasExpectedReturn {
						ctx.RegisterPendingReturnContext(n, ident.Value)
					}
				}
			}
		}
	}

	// If we have an expected return type from look-ahead pass, unify with it
	// This helps with trait methods like pure() where the return type depends on context
	if hasExpectedReturn {
		subst, err := typesystem.UnifyAllowExtraWithResolver(expectedReturnType, resultType, table)
		if err == nil {
			// Successfully unified - use expected type
			totalSubst = subst.Compose(totalSubst)
			resultType = expectedReturnType.Apply(totalSubst)
		}
		// If unification fails, fall back to inferred type
	}

	// Check trait constraints from TFunc and register PendingWitness for each
	// Moved AFTER unification with expected return type to capture context-dependent constraints
	for _, c := range tFunc.Constraints {
		// Calculate deferred arguments using current substitution.
		// These might still contain variables, which is fine for PendingWitness.
		// Solver will re-apply GlobalSubst later.

		tvar := typesystem.TVar{Name: c.TypeVar}
		concreteType := tvar.Apply(totalSubst)

		// Reconstruct args with current substitution
		// For Solver, Args must contain ALL MPTC arguments: [Left, ...Rest]
		deferredArgs := make([]typesystem.Type, 0, 1+len(c.Args))
		deferredArgs = append(deferredArgs, concreteType)
		for _, arg := range c.Args {
			deferredArgs = append(deferredArgs, arg.Apply(totalSubst))
		}

		// Append placeholder to Witnesses to maintain index alignment
		// We use a dummy Identifier for now, which will be replaced when resolved
		n.Witnesses = append(n.Witnesses, &ast.Identifier{Value: "$placeholder"})
		witnessIndex := len(n.Witnesses) - 1

		// Register Pending Witness
		// The solver will attempt to resolve this witness at the end of the function body
		// when all types are fully inferred.
		ctx.RegisterPendingWitness(n, c.Trait, c.TypeVar, deferredArgs, witnessIndex)
	}

	// Calculate Instantiation for Monomorphization
	if ident, ok := n.Function.(*ast.Identifier); ok && len(ident.TypeVarMapping) > 0 {
		instantiation := make(map[string]typesystem.Type)
		for genVar, freshVar := range ident.TypeVarMapping {
			// Apply substitutions to find the concrete type
			concrete := freshVar.Apply(ctx.GlobalSubst).Apply(totalSubst)
			instantiation[genVar] = concrete
		}
		n.Instantiation = instantiation
	}

	// Calculate TypeArgs for Data Constructors (Reified Generics)
	// Only for constructor calls that result in parameterized types
	if ident, ok := n.Function.(*ast.Identifier); ok {
		if sym, found := table.Find(ident.Value); found && sym.Kind == symbols.ConstructorSymbol {
			// This is a constructor call - add TypeArgs if result is parameterized
			if tApp, ok := resultType.(typesystem.TApp); ok {
				// Apply substitutions to resolve type variables
				resolvedArgs := make([]typesystem.Type, len(tApp.Args))
				for i, arg := range tApp.Args {
					resolvedArgs[i] = arg.Apply(totalSubst)
				}
				n.TypeArgs = resolvedArgs
			}
		}
	}

	return resultType, totalSubst, nil
}

func isList(t typesystem.Type) bool {
	if tApp, ok := t.(typesystem.TApp); ok {
		if tCon, ok := tApp.Constructor.(typesystem.TCon); ok && tCon.Name == config.ListTypeName {
			return true
		}
		if tCon, ok := tApp.Constructor.(*typesystem.TCon); ok && tCon.Name == config.ListTypeName {
			return true
		}
	}
	if tApp, ok := t.(*typesystem.TApp); ok {
		if tCon, ok := tApp.Constructor.(typesystem.TCon); ok && tCon.Name == config.ListTypeName {
			return true
		}
		if tCon, ok := tApp.Constructor.(*typesystem.TCon); ok && tCon.Name == config.ListTypeName {
			return true
		}
	}
	return false
}

func getListElementType(t typesystem.Type) typesystem.Type {
	if tApp, ok := t.(typesystem.TApp); ok {
		if len(tApp.Args) > 0 {
			return tApp.Args[0]
		}
	}
	if tApp, ok := t.(*typesystem.TApp); ok {
		if len(tApp.Args) > 0 {
			return tApp.Args[0]
		}
	}
	return typesystem.TVar{Name: "unknown"}
}

// inferTypeApplicationExpression infers the type of a type application (e.g. f<Int>)
func inferTypeApplicationExpression(ctx *InferenceContext, n *ast.TypeApplicationExpression, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	var baseType typesystem.Type
	var s1 typesystem.Subst = make(typesystem.Subst)
	var err error

	// Special handling for Identifiers to avoid implicit instantiation
	// We want the polymorphic type (TForall) so we can instantiate it with user-provided types
	if ident, ok := n.Expression.(*ast.Identifier); ok {
		sym, found := table.Find(ident.Value)
		if !found {
			return nil, nil, inferErrorf(ident, "undefined symbol: %s", ident.Value)
		}
		baseType = sym.Type
		if ctx.ResolutionMap != nil {
			ctx.ResolutionMap[ident] = sym
		}
	} else {
		// For other expressions, infer normally
		baseType, s1, err = inferFn(n.Expression, table)
		if err != nil {
			return nil, nil, err
		}
	}

	// 2. Resolve provided type arguments
	var typeArgs []typesystem.Type
	for _, tArg := range n.TypeArguments {
		var errors []*diagnostics.DiagnosticError
		t := BuildType(tArg, table, &errors)
		if len(errors) > 0 {
			return nil, nil, errors[0]
		}
		typeArgs = append(typeArgs, t)
	}

	// 3. Apply type arguments to base type
	if forall, ok := baseType.(typesystem.TForall); ok {
		if len(typeArgs) != len(forall.Vars) {
			return nil, nil, inferErrorf(n, "type argument count mismatch: expected %d, got %d", len(forall.Vars), len(typeArgs))
		}

		subst := make(typesystem.Subst)
		for i, v := range forall.Vars {
			subst[v.Name] = typeArgs[i]
		}

		// Instantiate body with provided types
		instantiated := forall.Type.Apply(subst)

		// 4. Resolve Witnesses for Constraints
		// Clear previous witnesses if re-analyzing
		n.Witnesses = nil

		for _, c := range forall.Constraints {
			// Apply subst to constraint args
			traitArgs := make([]typesystem.Type, len(c.Args))
			for i, arg := range c.Args {
				traitArgs[i] = arg.Apply(subst)
			}

			// Solve witness
			// Constraint.Args includes all types (e.g. [Int] for Show<Int>)
			witnessExpr, err := ctx.SolveWitness(n, c.Trait, traitArgs, table)
			if err != nil {
				return nil, nil, err
			}

			n.Witnesses = append(n.Witnesses, witnessExpr)
		}

		// If instantiated type is TFunc, we strip constraints because we handled them via witnesses
		if tFunc, ok := instantiated.(typesystem.TFunc); ok {
			tFunc.Constraints = nil
			instantiated = tFunc
		}

		return instantiated, s1.Compose(subst), nil
	} else {
		// Implicit generics: use free type variables sorted by name
		vars := baseType.FreeTypeVariables()
		// Sort to establish deterministic order (convention: alphabetical)
		sort.Slice(vars, func(i, j int) bool {
			return vars[i].Name < vars[j].Name
		})

		if len(vars) > 0 {
			if len(typeArgs) != len(vars) {
				return nil, nil, inferErrorf(n, "type argument count mismatch: expected %d, got %d (implicit generics)", len(vars), len(typeArgs))
			}

			subst := make(typesystem.Subst)
			for i, v := range vars {
				subst[v.Name] = typeArgs[i]
			}

			instantiated := baseType.Apply(subst)

			// Handle TFunc constraints if present (common for implicit generics)
			if tFunc, ok := instantiated.(typesystem.TFunc); ok {
				n.Witnesses = nil
				// Just checking syntax here, loop is below
				_ = tFunc
			}

			// Re-logic for implicit constraints:
			// We should use baseType's constraints and apply substitution manually.
			if tFuncBase, ok := baseType.(typesystem.TFunc); ok {
				n.Witnesses = nil
				for _, c := range tFuncBase.Constraints {
					// Resolve the TypeVar
					var concreteType typesystem.Type
					if t, ok := subst[c.TypeVar]; ok {
						concreteType = t
					} else {
						concreteType = typesystem.TVar{Name: c.TypeVar}
					}

					// Resolve Args
					concreteArgs := make([]typesystem.Type, len(c.Args))
					for i, arg := range c.Args {
						concreteArgs[i] = arg.Apply(subst)
					}

					// Full Args for Witness Lookup: [Type, Args...]
					fullArgs := append([]typesystem.Type{concreteType}, concreteArgs...)

					witnessExpr, err := ctx.SolveWitness(n, c.Trait, fullArgs, table)
					if err != nil {
						return nil, nil, err
					}
					n.Witnesses = append(n.Witnesses, witnessExpr)
				}

				// Strip constraints from instantiated type
				if tFuncInst, ok := instantiated.(typesystem.TFunc); ok {
					tFuncInst.Constraints = nil
					instantiated = tFuncInst
				}
			}

			return instantiated, s1.Compose(subst), nil
		}
	}

	return nil, nil, inferErrorf(n, "cannot apply types to non-generic type: %s", baseType)
}
