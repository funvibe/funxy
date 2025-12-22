package analyzer

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

func inferIfExpression(ctx *InferenceContext, n *ast.IfExpression, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	condType, s1, err := inferFn(n.Condition, table)
	if err != nil {
		return nil, nil, err
	}
	totalSubst := s1

	subst, err := typesystem.Unify(condType.Apply(totalSubst), typesystem.Bool)
	if err != nil {
		return nil, nil, inferErrorf(n.Condition, "condition in if-expression must be Bool, got %s", condType.Apply(totalSubst))
	}
	totalSubst = subst.Compose(totalSubst)

	conseqType, s2, err := inferFn(n.Consequence, table)
	if err != nil {
		return nil, nil, err
	}
	totalSubst = s2.Compose(totalSubst)
	conseqType = conseqType.Apply(totalSubst)

	if n.Alternative != nil {
		altType, s3, err := inferFn(n.Alternative, table)
		if err != nil {
			return nil, nil, err
		}
		totalSubst = s3.Compose(totalSubst)
		altType = altType.Apply(totalSubst)

		// Try to unify the branch types
		subst, err := typesystem.Unify(conseqType, altType)
		if err != nil {
			// If unification fails, create a union type
			// This allows: if b { 42 } else { Nil } -> Int | Nil
			unionType := typesystem.NormalizeUnion([]typesystem.Type{conseqType, altType})
			return unionType, totalSubst, nil
		}
		totalSubst = subst.Compose(totalSubst)

		return conseqType.Apply(totalSubst), totalSubst, nil
	} else {
		// No else clause: if consequence returns Nil, that's fine
		// Otherwise, result is T | Nil where T is consequence type
		if _, err := typesystem.Unify(conseqType, typesystem.Nil); err != nil {
			// Consequence returns non-Nil, so the if without else returns T | Nil
			unionType := typesystem.NormalizeUnion([]typesystem.Type{conseqType, typesystem.Nil})
			return unionType, totalSubst, nil
		}
		return typesystem.Nil, totalSubst, nil
	}
}

func cloneSubst(s typesystem.Subst) typesystem.Subst {
	newS := make(typesystem.Subst, len(s))
	for k, v := range s {
		newS[k] = v
	}
	return newS
}

func inferMatchExpression(ctx *InferenceContext, n *ast.MatchExpression, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error), typeMap map[ast.Node]typesystem.Type) (typesystem.Type, typesystem.Subst, error) {
	scrutineeType, s1, err := inferFn(n.Expression, table)
	if err != nil {
		return nil, nil, err
	}
	// globalSubst tracks constraints that apply to the entire match expression (e.g. result type consistency)
	globalSubst := s1

	var resType typesystem.Type
	var firstError error // Collect first error but continue analysis

	for _, arm := range n.Arms {
		armTable := symbols.NewEnclosedSymbolTable(table)

		// armSubst accumulates constraints specific to this arm (pattern matches)
		// We clone globalSubst to isolate this arm's refinements from others
		armSubst := cloneSubst(globalSubst)

		// Apply current global constraints to scrutinee type
		currentScrutinee := scrutineeType.Apply(armSubst)

		// inferPattern now returns Subst
		patSubst, err := inferPattern(ctx, arm.Pattern, currentScrutinee, armTable)
		if err != nil {
			if firstError == nil {
				firstError = err // Keep first error, don't wrap it
			}
			// Continue to check other arms for exhaustiveness analysis
			continue
		}
		armSubst = patSubst.Compose(armSubst)

		// Type-check guard expression if present (must be Bool)
		if arm.Guard != nil {
			guardType, sGuard, err := inferFn(arm.Guard, armTable)
			if err != nil {
				if firstError == nil {
					firstError = err
				}
				continue
			}
			armSubst = sGuard.Compose(armSubst)
			guardType = guardType.Apply(armSubst)

			// Guard must be Bool
			if _, err := typesystem.Unify(guardType, typesystem.TCon{Name: "Bool"}); err != nil {
				if firstError == nil {
					firstError = fmt.Errorf("guard expression must be Bool, got %s", guardType)
				}
				continue
			}
		}

		armType, sArm, err := inferFn(arm.Expression, armTable)
		if err != nil {
			if firstError == nil {
				firstError = err
			}
			continue
		}
		armSubst = sArm.Compose(armSubst)
		armType = armType.Apply(armSubst)

		if resType == nil {
			resType = armType
		} else {
			// Unify result type with previous arms
			// Note: We unify against resType.Apply(globalSubst) to respect global constraints,
			// but we unify WITH armType which has arm-local constraints applied.
			// The resulting substitution is added to globalSubst.
			subst, err := typesystem.Unify(resType.Apply(globalSubst), armType)
			if err != nil {
				// If unification fails, create a union type
				// This allows match arms to return different types: Int | Nil
				resType = typesystem.NormalizeUnion([]typesystem.Type{resType.Apply(globalSubst), armType})
				continue
			}
			globalSubst = subst.Compose(globalSubst)
			// resType might not change, but we apply new globalSubst next time
		}
	}

	// Always check exhaustiveness, even if there were pattern errors
	exhaustErr := CheckExhaustiveness(n, scrutineeType.Apply(globalSubst), table)

	// Return errors - prioritize pattern errors, but also report exhaustiveness
	if firstError != nil {
		if exhaustErr != nil {
			// Combine errors: show pattern error first, then exhaustiveness
			return nil, nil, &combinedError{errors: []error{firstError, exhaustErr}}
		}
		return nil, nil, firstError
	}
	if exhaustErr != nil {
		return nil, nil, exhaustErr
	}

	// Ensure resType is fully applied with final global constraints
	var finalResType typesystem.Type = typesystem.Nil // Default if no arms
	if resType != nil {
		finalResType = resType.Apply(globalSubst)
	}

	return finalResType, globalSubst, nil
}

func inferBlockStatement(ctx *InferenceContext, n *ast.BlockStatement, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	var lastType typesystem.Type = typesystem.Nil
	totalSubst := typesystem.Subst{}

	enclosedTable := symbols.NewEnclosedSymbolTable(table)

	// Infer statements
	for _, stmt := range n.Statements {
		if es, ok := stmt.(*ast.ExpressionStatement); ok {
			t, s, err := inferFn(es.Expression, enclosedTable)
			if err != nil {
				return nil, nil, err
			}
			totalSubst = s.Compose(totalSubst)
			lastType = t
		} else if tds, ok := stmt.(*ast.TypeDeclarationStatement); ok {
			RegisterTypeDeclaration(tds, enclosedTable, "")
			lastType = typesystem.Nil
		} else if id, ok := stmt.(*ast.InstanceDeclaration); ok {
			// Register local instance
			traitName := id.TraitName.Value
			if id.ModuleName != nil {
				traitName = id.ModuleName.Value + "." + traitName
			}

			// Validate trait exists
			if !enclosedTable.TraitExists(traitName) {
				// Try short name if qualified name failed (e.g. qualified_pkg.core.Validator -> Validator)
				shortName := id.TraitName.Value
				if enclosedTable.TraitExists(shortName) {
					traitName = shortName
				} else {
					return nil, nil, fmt.Errorf("trait %s not defined", traitName)
				}
			}

			// Build target type
			var errs []*diagnostics.DiagnosticError
			targetType := BuildType(id.Target, enclosedTable, &errs)
			if len(errs) > 0 {
				return nil, nil, errs[0]
			}

			// Register implementation
			if err := enclosedTable.RegisterImplementation(traitName, targetType); err != nil {
				return nil, nil, err
			}
			lastType = typesystem.Nil
		} else if fs, ok := stmt.(*ast.FunctionStatement); ok {
			// Register function in the block scope for type inference
			// We build a simplified signature here for inference purposes

			// 1. Create type variables for type parameters
			sigScope := symbols.NewEnclosedSymbolTable(enclosedTable)
			for _, tp := range fs.TypeParams {
				sigScope.DefineType(tp.Value, typesystem.TVar{Name: tp.Value}, "")
			}

			// 2. Build params
			var params []typesystem.Type
			for _, p := range fs.Parameters {
				if p.Type != nil {
					// Need BuildType equivalent accessible here?
					// BuildType is in declarations.go and depends on walker.
					// We can't use it easily here.
					// But for inference, we might rely on what's already resolved?
					// Or just use fresh vars if type building is too complex?
					// Using fresh vars is safer for pure inference pass.
					// If we really need strict types, we assume walker already validated them.
					params = append(params, ctx.FreshVar())
				} else {
					params = append(params, ctx.FreshVar())
				}
			}

			// 3. Return type
			retType := ctx.FreshVar()

			// 4. Constraints
			var fnConstraints []typesystem.Constraint
			for _, c := range fs.Constraints {
				fnConstraints = append(fnConstraints, typesystem.Constraint{TypeVar: c.TypeVar, Trait: c.Trait})
			}

			fnType := typesystem.TFunc{
				Params:      params,
				ReturnType:  retType,
				Constraints: fnConstraints,
			}

			enclosedTable.DefineConstant(fs.Name.Value, fnType, "")

			lastType = typesystem.Nil
		} else if bs, ok := stmt.(*ast.BreakStatement); ok {
			t, s, err := inferBreakStatement(ctx, bs, enclosedTable, inferFn)
			if err != nil {
				return nil, nil, err
			}
			totalSubst = s.Compose(totalSubst)
			lastType = t
		} else if cs, ok := stmt.(*ast.ContinueStatement); ok {
			t, s, err := inferContinueStatement(ctx, cs)
			if err != nil {
				return nil, nil, err
			}
			totalSubst = s.Compose(totalSubst)
			lastType = t
		} else if cd, ok := stmt.(*ast.ConstantDeclaration); ok {
			// Infer value to capture substitutions (e.g. from function calls)
			t, s, err := inferFn(cd.Value, enclosedTable)
			if err != nil {
				return nil, nil, err
			}
			totalSubst = s.Compose(totalSubst)

			// We should also check type annotation if present, but for now we prioritize
			// capturing the substitution and defining the symbol for subsequent statements.
			enclosedTable.Define(cd.Name.Value, t.Apply(totalSubst), "")

			lastType = typesystem.Nil
		}
	}
	return lastType.Apply(totalSubst), totalSubst, nil
}

func inferForExpression(ctx *InferenceContext, n *ast.ForExpression, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	loopScope := symbols.NewEnclosedSymbolTable(table)
	totalSubst := typesystem.Subst{}

	if n.Iterable != nil {
		iterableType, s1, err := inferFn(n.Iterable, table)
		if err != nil {
			return nil, nil, err
		}
		totalSubst = s1.Compose(totalSubst)
		iterableType = iterableType.Apply(totalSubst)

		var itemType typesystem.Type

		// Direct support for List<T>
		if tApp, ok := iterableType.(typesystem.TApp); ok {
			if tCon, ok := tApp.Constructor.(typesystem.TCon); ok && tCon.Name == config.ListTypeName && len(tApp.Args) == 1 {
				itemType = tApp.Args[0]
			}
		}

		if itemType == nil {
			// Check for iter method via Iter trait protocol
			// We look for an iter function that can handle this type.
			if iterSym, ok := table.Find(config.IterMethodName); ok {
				iterType := InstantiateWithContext(ctx, iterSym.Type)
				if tFunc, ok := iterType.(typesystem.TFunc); ok && len(tFunc.Params) > 0 {
					subst, err := typesystem.Unify(tFunc.Params[0], iterableType)
					if err == nil {
						totalSubst = subst.Compose(totalSubst)
						retType := tFunc.ReturnType.Apply(totalSubst)

						if iteratorFunc, ok := retType.(typesystem.TFunc); ok {
							iteratorRet := iteratorFunc.ReturnType
							if tApp, ok := iteratorRet.(typesystem.TApp); ok {
								if tCon, ok := tApp.Constructor.(typesystem.TCon); ok && tCon.Name == config.OptionTypeName && len(tApp.Args) >= 1 {
									itemType = tApp.Args[0]
								}
							}
						}
					}
				}
			}
		}

		if itemType == nil {
			return nil, nil, inferErrorf(n.Iterable, "iterable in for-loop must be List or implement Iter trait, got %s", iterableType)
		}

		loopScope.Define(n.ItemName.Value, itemType, "")

	} else {
		condType, s1, err := inferFn(n.Condition, table)
		if err != nil {
			return nil, nil, err
		}
		totalSubst = s1.Compose(totalSubst)

		subst, err := typesystem.Unify(typesystem.Bool, condType.Apply(totalSubst))
		if err != nil {
			return nil, nil, inferErrorf(n.Condition, "for-loop condition must be Bool, got %s", condType.Apply(totalSubst))
		}
		totalSubst = subst.Compose(totalSubst)
	}

	loopReturnType := ctx.FreshVar()
	loopScope.Define("__loop_return", loopReturnType, "")

	bodyType, sBody, err := inferFn(n.Body, loopScope)
	if err != nil {
		return nil, nil, err
	}
	totalSubst = sBody.Compose(totalSubst)
	bodyType = bodyType.Apply(totalSubst)

	subst, err := typesystem.Unify(loopReturnType.Apply(totalSubst), bodyType)
	if err != nil {
		return nil, nil, inferErrorf(n, "loop body type mismatch with break values: %v", err)
	}
	totalSubst = subst.Compose(totalSubst)

	return loopReturnType.Apply(totalSubst), totalSubst, nil
}

func inferBreakStatement(ctx *InferenceContext, n *ast.BreakStatement, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	var valType typesystem.Type = typesystem.Nil
	totalSubst := typesystem.Subst{}

	if n.Value != nil {
		t, s, err := inferFn(n.Value, table)
		if err != nil {
			return nil, nil, err
		}
		totalSubst = s.Compose(totalSubst)
		valType = t
	}

	if expectedType, ok := table.Find("__loop_return"); ok {
		subst, err := typesystem.Unify(expectedType.Type.Apply(totalSubst), valType.Apply(totalSubst))
		if err != nil {
			return nil, nil, inferErrorf(n, "break value type mismatch: expected %s, got %s", expectedType.Type, valType)
		}
		totalSubst = subst.Compose(totalSubst)
	} else {
		return nil, nil, inferError(n, "break statement outside of loop")
	}

	return typesystem.Nil, totalSubst, nil
}

func inferContinueStatement(ctx *InferenceContext, n *ast.ContinueStatement) (typesystem.Type, typesystem.Subst, error) {
	return typesystem.Nil, typesystem.Subst{}, nil
}
