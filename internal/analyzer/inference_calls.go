package analyzer

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

func inferCallExpression(ctx *InferenceContext, n *ast.CallExpression, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	// Infer function type
	fnType, s1, err := inferFn(n.Function, table)
	if err != nil {
		return nil, nil, err
	}
	totalSubst := s1
	fnType = fnType.Apply(totalSubst)

	// Resolve type aliases (e.g., type Observer = (Int) -> Nil)
	fnType = typesystem.UnwrapUnderlying(fnType)

	// Handle Type Application (e.g. List(Int))
	if tType, ok := fnType.(typesystem.TType); ok {
		var typeArgs []typesystem.Type
		for _, arg := range n.Arguments {
			argType, sArg, err := inferFn(arg, table)
			if err != nil {
				return nil, nil, err
			}
			totalSubst = sArg.Compose(totalSubst)

			if tArg, ok := argType.(typesystem.TType); ok {
				typeArgs = append(typeArgs, tArg.Type)
			} else {
				return nil, nil, inferErrorf(n, "type application expects types as arguments, got %s", argType)
			}
		}
		return typesystem.TType{Type: typesystem.TApp{Constructor: tType.Type, Args: typeArgs}}, totalSubst, nil
	} else if tFunc, ok := fnType.(typesystem.TFunc); ok {
		// Handle TFunc
		paramIdx := 0


		// Note: Function types from inferIdentifier are already instantiated.
		// We don't instantiate again here to keep TypeMap entries consistent.

		for i, arg := range n.Arguments {
			isSpread := false
			if _, ok := arg.(*ast.SpreadExpression); ok {
				isSpread = true
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
									subst, err := typesystem.Unify(varType, elType)
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
							subst, err := typesystem.UnifyAllowExtra(paramType, elType)
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

					subst, err := typesystem.Unify(varType, listElemType)
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
							subst, err := typesystem.Unify(varType, argType)
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
					subst, err := typesystem.UnifyAllowExtra(paramType, argType)
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

		// Check trait constraints from TFunc
		for _, c := range tFunc.Constraints {
			// Apply substitution to get the concrete type for the type variable
			tvar := typesystem.TVar{Name: c.TypeVar}
			concreteType := tvar.Apply(totalSubst)

			// Skip if type variable is not yet resolved (still a TVar)
			if _, stillVar := concreteType.(typesystem.TVar); stillVar {
				continue
			}

			// Check if the concrete type implements the required trait
			// Also check if it's a constrained type param (TCon like "T" in recursive calls)
			if !table.IsImplementationExists(c.Trait, concreteType) && !typeHasConstraint(ctx, concreteType, c.Trait) {
				return nil, nil, inferErrorf(n, "type %s does not implement trait %s", concreteType, c.Trait)
			}
		}

		return tFunc.ReturnType.Apply(totalSubst), totalSubst, nil
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

		subst, err := typesystem.Unify(tVar, expectedFnType)
		if err != nil {
			return nil, nil, inferErrorf(n, "cannot call %s as a function with arguments %v", fnType, paramTypes)
		}
		totalSubst = subst.Compose(totalSubst)

		return resultVar.Apply(totalSubst), totalSubst, nil
	} else {
		return nil, nil, inferErrorf(n, "cannot call non-function type: %s", fnType)
	}
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
