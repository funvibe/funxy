package analyzer

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
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
		// Now check if it's a function type and recurse (or handle inline)
		// We can't easily recurse inferCallExpression because 'n' is already processed partially.
		// Instead, we call inferCallExpression again but with the instantiated type as 'fnType' context?
		// No, `inferCallExpression` derives fnType from n.Function.
		// We can just proceed with the instantiated type logic by falling through?
		// Or better: Unify the instantiated type with a new function type variable, similar to TVar case?
		// Actually, if we instantiate it, we get a Type (e.g. TFunc). We can just process it as TFunc.

		if tFunc, ok := instantiated.(typesystem.TFunc); ok {
			// Duplicate TFunc logic... refactor?
			// For now, let's recursively call inferCallExpression by mocking the function type?
			// No, that's messy.
			// Let's just swap fnType and loop? No, Go doesn't have goto like that easily here.
			// Let's copy the TFunc block logic or refactor.

			// Refactoring: extract TFunc handling
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
							if tvar, ok := varType.(typesystem.TVar); ok {
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
					// This supports builtins like print(...) and sprintf(fmt, ...).
					isIsolated := false
					if tvar, ok := varType.(typesystem.TVar); ok {
						// Check if this TVar is used in return type
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

	// If we have an expected return type from look-ahead pass, unify with it
	// This helps with trait methods like pure() where the return type depends on context
	if expectedReturnType, hasExpected := ctx.ExpectedReturnTypes[n]; hasExpected {
		subst, err := typesystem.UnifyAllowExtraWithResolver(expectedReturnType, resultType, table)
		if err == nil {
			// Successfully unified - use expected type
			totalSubst = subst.Compose(totalSubst)
			resultType = expectedReturnType.Apply(totalSubst)
		}
		// If unification fails, fall back to inferred type
	}

	// Check trait constraints from TFunc
	// Moved AFTER unification with expected return type to capture context-dependent constraints
	for _, c := range tFunc.Constraints {
		// Apply substitution to get the concrete type for the type variable
		tvar := typesystem.TVar{Name: c.TypeVar}
		concreteType := tvar.Apply(totalSubst)

		// Check if any MPTC arg is unresolved
		hasUnresolvedArg := false
		for _, arg := range c.Args {
			resolvedArg := arg.Apply(totalSubst)
			if _, ok := resolvedArg.(typesystem.TVar); ok {
				hasUnresolvedArg = true
				break
			}
		}

		// Skip if type variable or any MPTC arg is not yet resolved
		isRigid := false
		if tCon, ok := concreteType.(typesystem.TCon); ok && len(tCon.Name) > 0 && tCon.Name[0] >= 'a' && tCon.Name[0] <= 'z' {
			isRigid = true
		}
		if _, stillVar := concreteType.(typesystem.TVar); stillVar || isRigid || hasUnresolvedArg {
			// If type is still a variable, defer constraint check
			// We need to pass the *original* arguments (with potentially unresolved vars) to AddDeferredConstraint,
			// but wrapped in appropriate structure. AddDeferredConstraint takes Constraint struct.
			// We should use the constraints from TFunc but with substituted vars where possible.

			// Reconstruct args with current substitution
			// For Solver, Args must contain ALL MPTC arguments: [Left, ...Rest]
			deferredArgs := make([]typesystem.Type, 0, 1+len(c.Args))
			deferredArgs = append(deferredArgs, concreteType)
			for _, arg := range c.Args {
				deferredArgs = append(deferredArgs, arg.Apply(totalSubst))
			}

			ctx.AddDeferredConstraint(Constraint{
				Kind:  ConstraintImplements,
				Left:  concreteType,
				Trait: c.Trait,
				Args:  deferredArgs,
				Node:  n,
			})

			// Also register as PendingWitness to ensure dictionary passing at runtime
			// Even though we defer the check, we need to record that this call site
			// requires a witness for this trait and type variable.
			if n.Witness == nil {
				n.Witness = make(map[string][]typesystem.Type)
			}

			// Append placeholder to Witnesses to maintain index alignment
			// We use a dummy Identifier for now, which will be replaced when resolved
			n.Witnesses = append(n.Witnesses, &ast.Identifier{Value: "$placeholder"})
			witnessIndex := len(n.Witnesses) - 1

			ctx.RegisterPendingWitness(n, c.Trait, tvar.Name, deferredArgs, witnessIndex)

			continue
		}

		// Check if the concrete type implements the required trait
		// Also check if it's a constrained type param (TCon like "T" in recursive calls)
		// Unwrap type aliases before checking to handle cases like Writer<IntList, Int>
		checkType := typesystem.UnwrapUnderlying(concreteType)
		if checkType == nil {
			checkType = concreteType
		}

		// Build full args list for MPTC: [checkType, c.Args...]
		// We need to apply substitutions to c.Args first
		fullArgs := []typesystem.Type{checkType}
		for _, arg := range c.Args {
			fullArgs = append(fullArgs, arg.Apply(totalSubst))
		}

		if !table.IsImplementationExists(c.Trait, fullArgs) && !typeHasConstraint(ctx, concreteType, c.Trait, table) {
			// Format error message based on arity
			if len(fullArgs) == 1 {
				return nil, nil, inferErrorf(n, "type %s does not implement trait %s", fullArgs[0], c.Trait)
			}
			return nil, nil, inferErrorf(n, "types %v do not implement trait %s", fullArgs, c.Trait)
		}

		// 2.4 Call Site Solving & Rewriting
		// Solve witness for the constraint
		witnessExpr, err := ctx.SolveWitness(n, c.Trait, fullArgs, table)
		if err != nil {
			// If solve fails, check if we can defer it (if deferredArgs available)
			// But we already checked unresolved args above.
			// If we are here, args are resolved or we failed to solve concrete witness.

			// If we added a DeferredConstraint earlier (unresolved vars), we continued loop.
			// So we are here only if args are concrete-ish or resolved.

			// Report error
			return nil, nil, err
		}

		// Append to Witnesses list
		n.Witnesses = append(n.Witnesses, witnessExpr)

		// Legacy support: Store witness for runtime (optional, can be removed once Evaluator uses Witnesses)
		if n.Witness == nil {
			n.Witness = make(map[string][]typesystem.Type)
		}
		if witnesses, ok := n.Witness.(map[string][]typesystem.Type); ok {
			witnesses[c.Trait] = fullArgs
			witnesses[c.TypeVar] = fullArgs // For compatibility
		}
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
