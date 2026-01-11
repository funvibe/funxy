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
	// Propagate expected return type to branches
	if expectedType, ok := ctx.ExpectedReturnTypes[n]; ok {
		if ctx.ExpectedReturnTypes == nil {
			ctx.ExpectedReturnTypes = make(map[ast.Node]typesystem.Type)
		}
		// If consequence/alternative are blocks, they will handle it.
		// If expressions, they might need it (e.g. pure()).
		ctx.ExpectedReturnTypes[n.Consequence] = expectedType
		if n.Alternative != nil {
			ctx.ExpectedReturnTypes[n.Alternative] = expectedType
		}
	}

	// Analyze Condition for Flow-Sensitive Typing
	var guardVar *ast.Identifier
	var guardType typesystem.Type

	// Check if condition is a call to typeOf
	if call, ok := n.Condition.(*ast.CallExpression); ok {
		if ident, ok := call.Function.(*ast.Identifier); ok && ident.Value == config.TypeOfFuncName {
			if len(call.Arguments) == 2 {
				// First arg must be an identifier (variable)
				if v, ok := call.Arguments[0].(*ast.Identifier); ok {
					// Second arg is the type to check against.
					// We support Identifier (SimpleType) and MemberExpression (Module.Type)
					// and simple Generic instantiation if parsed as CallExpression (List(Int)) or similar,
					// but for now let's stick to Identifier which covers the 'List' case.

					var astType ast.Type
					typeArg := call.Arguments[1]

					if tIdent, ok := typeArg.(*ast.Identifier); ok {
						astType = &ast.NamedType{Name: tIdent}
					}

					if astType != nil {
						var errs []*diagnostics.DiagnosticError
						gType := BuildType(astType, table, &errs)
						if len(errs) == 0 {
							// Instantiate if it's a constructor (e.g. List -> List<t1>)
							// This ensures that using the variable in the block works as expected (expecting * kind)
							kind := gType.Kind()
							var newArgs []typesystem.Type

							for {
								if kArrow, ok := kind.(typesystem.KArrow); ok {
									fresh := ctx.FreshVarWithKind(kArrow.Left)
									newArgs = append(newArgs, fresh)
									kind = kArrow.Right
								} else {
									break
								}
							}

							if len(newArgs) > 0 {
								gType = typesystem.TApp{Constructor: gType, Args: newArgs}
							}

							guardVar = v
							guardType = gType
						}
					}
				}
			}
		}
	}

	condType, s1, err := inferFn(n.Condition, table)
	if err != nil {
		return nil, nil, err
	}
	totalSubst := s1

	subst, err := typesystem.Unify(condType.Apply(ctx.GlobalSubst).Apply(totalSubst), typesystem.Bool)
	if err != nil {
		return nil, nil, inferErrorf(n.Condition, "condition in if-expression must be Bool, got %s", condType.Apply(ctx.GlobalSubst).Apply(totalSubst))
	}
	totalSubst = subst.Compose(totalSubst)

	// Consequence with Narrowing
	conseqTable := symbols.NewEnclosedSymbolTable(table, symbols.ScopeBlock)
	if guardVar != nil && guardType != nil {
		// Narrow type in consequence
		// We overwrite the variable definition in the new scope
		conseqTable.Define(guardVar.Value, guardType, "")
	}

	conseqType, s2, err := inferFn(n.Consequence, conseqTable)
	if err != nil {
		return nil, nil, err
	}
	totalSubst = s2.Compose(totalSubst)
	conseqType = conseqType.Apply(ctx.GlobalSubst).Apply(totalSubst)

	if n.Alternative != nil {
		// Alternative with Negative Narrowing (Union Types)
		altTable := symbols.NewEnclosedSymbolTable(table, symbols.ScopeBlock)

		if guardVar != nil && guardType != nil {
			// Get original type
			if sym, ok := table.Find(guardVar.Value); ok {
				originalType := sym.Type.Apply(ctx.GlobalSubst).Apply(totalSubst)

				// Only if original is Union, we can subtract
				if union, ok := originalType.(typesystem.TUnion); ok {
					var remainingTypes []typesystem.Type
					for _, t := range union.Types {
						// Check if t matches guardType. If NOT, keep it.
						// We use Unify to check compatibility.
						// If Unify succeeds, it means 't' COULD be 'guardType', so we remove it (or refine it).
						// For strict removal: if t is subtype of guardType.
						// For now, if they Unify, we assume it's the branch taken by 'if', so we exclude it from 'else'.
						// NOTE: This is slightly aggressive for overlapping types, but correct for Disjoint Unions.

						_, err := typesystem.Unify(t, guardType)
						if err != nil {
							// Did not unify -> Include in else branch
							remainingTypes = append(remainingTypes, t)
						}
					}

					if len(remainingTypes) > 0 {
						narrowedType := typesystem.NormalizeUnion(remainingTypes)
						altTable.Define(guardVar.Value, narrowedType, "")
					}
				}
			}
		}

		altType, s3, err := inferFn(n.Alternative, altTable)
		if err != nil {
			return nil, nil, err
		}
		totalSubst = s3.Compose(totalSubst)
		altType = altType.Apply(ctx.GlobalSubst).Apply(totalSubst)

		// Try to unify the branch types
		subst, err := typesystem.Unify(conseqType, altType)
		if err != nil {
			// If unification fails, create a union type
			// This allows: if b { 42 } else { Nil } -> Int | Nil
			unionType := typesystem.NormalizeUnion([]typesystem.Type{conseqType, altType})
			return unionType, totalSubst, nil
		}
		totalSubst = subst.Compose(totalSubst)

		return conseqType.Apply(ctx.GlobalSubst).Apply(totalSubst), totalSubst, nil
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

	// Propagate expected return type to arms
	if expectedType, ok := ctx.ExpectedReturnTypes[n]; ok {
		if ctx.ExpectedReturnTypes == nil {
			ctx.ExpectedReturnTypes = make(map[ast.Node]typesystem.Type)
		}
		for _, arm := range n.Arms {
			ctx.ExpectedReturnTypes[arm.Expression] = expectedType
		}
	}

	var resType typesystem.Type
	var firstError error // Collect first error but continue analysis

	for _, arm := range n.Arms {
		armTable := symbols.NewEnclosedSymbolTable(table, symbols.ScopeBlock)

		// armSubst accumulates constraints specific to this arm (pattern matches)
		// We clone globalSubst to isolate this arm's refinements from others
		armSubst := cloneSubst(globalSubst)

		// Apply current global constraints to scrutinee type
		currentScrutinee := scrutineeType.Apply(ctx.GlobalSubst).Apply(armSubst)

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
		// Propagate pattern constraints to global substitution (e.g. refining scrutinee type)
		globalSubst = patSubst.Compose(globalSubst)

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
			guardType = guardType.Apply(ctx.GlobalSubst).Apply(armSubst)

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
		// Accumulate arm constraints into global substitution
		globalSubst = sArm.Compose(globalSubst)

		armType = armType.Apply(ctx.GlobalSubst).Apply(armSubst)

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
		finalResType = resType.Apply(ctx.GlobalSubst).Apply(globalSubst)
	}

	return finalResType, globalSubst, nil
}

func inferBlockStatement(ctx *InferenceContext, n *ast.BlockStatement, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	var lastType typesystem.Type = typesystem.Nil
	totalSubst := typesystem.Subst{}

	enclosedTable := symbols.NewEnclosedSymbolTable(table, symbols.ScopeBlock)

	// Check for expected return type for the block (propagated from function literal or control flow)
	var expectedReturnType typesystem.Type
	if t, ok := ctx.ExpectedReturnTypes[n]; ok {
		// Recovery for potential TCon -> TVar mutation in expected type propagation
		// If we expect a TVar that resolves to a TCon in the current table, prefer TCon.
		// This is crucial for MPTC rigid type variable matching.
		if tv, ok := t.(typesystem.TVar); ok {
			if resolved, ok := table.ResolveType(tv.Name); ok {
				if tCon, ok := resolved.(typesystem.TCon); ok {
					t = tCon
				}
			}
		}
		expectedReturnType = t
	}

	// Infer statements
	for i, stmt := range n.Statements {
		if es, ok := stmt.(*ast.ExpressionStatement); ok {
			// If it's the last statement and we have an expected type, propagate it to the expression
			// This allows recursive context propagation (e.g. lambda -> block -> pure())
			if i == len(n.Statements)-1 && expectedReturnType != nil {
				if ctx.ExpectedReturnTypes == nil {
					ctx.ExpectedReturnTypes = make(map[ast.Node]typesystem.Type)
				}
				ctx.ExpectedReturnTypes[es.Expression] = expectedReturnType
			}

			t, s, err := inferFn(es.Expression, enclosedTable)
			if err != nil {
				return nil, nil, err
			}
			totalSubst = s.Compose(totalSubst)
			// Apply substitution to GlobalSubst to allow subsequent statements
			// to see type refinements (e.g. inference of local variables).
			ctx.GlobalSubst = s.Compose(ctx.GlobalSubst)
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
			var targetType typesystem.Type
			if len(id.Args) > 0 {
				targetType = BuildType(id.Args[0], enclosedTable, &errs)
			}
			if len(errs) > 0 {
				return nil, nil, errs[0]
			}

			// Prepare requirements for generic instance constraints
			var requirements []typesystem.Constraint
			for _, c := range id.Constraints {
				var constraintArgs []typesystem.Type
				for _, argNode := range c.Args {
					t := BuildType(argNode, enclosedTable, &errs)
					constraintArgs = append(constraintArgs, t)
				}
				if len(errs) > 0 {
					return nil, nil, errs[0]
				}

				requirements = append(requirements, typesystem.Constraint{
					TypeVar: c.TypeVar,
					Trait:   c.Trait,
					Args:    constraintArgs,
				})
			}

			// Generate evidence name
			var typeName string
			if tCon, ok := targetType.(typesystem.TCon); ok {
				typeName = tCon.Name
			} else if tApp, ok := targetType.(typesystem.TApp); ok {
				if tCon, ok := tApp.Constructor.(typesystem.TCon); ok {
					typeName = tCon.Name
				}
			}
			if typeName == "" {
				typeName = "Local"
			}

			var evidenceName string
			if len(id.TypeParams) > 0 {
				evidenceName = GetDictionaryConstructorName(traitName, typeName)
			} else {
				evidenceName = GetDictionaryName(traitName, typeName)
			}

			// Register implementation
			if err := enclosedTable.RegisterImplementation(traitName, []typesystem.Type{targetType}, requirements, evidenceName); err != nil {
				return nil, nil, err
			}

			// Local Dictionary Definition
			// Required for SolveWitness to find the evidence symbol
			evidenceKey := GetEvidenceKey(traitName, []typesystem.Type{targetType})
			enclosedTable.RegisterEvidence(evidenceKey, evidenceName)
			enclosedTable.DefineConstant(evidenceName, typesystem.TCon{Name: "Dictionary"}, "")

			lastType = typesystem.Nil
		} else if fs, ok := stmt.(*ast.FunctionStatement); ok {
			// Register function in the block scope for type inference
			// We build a simplified signature here for inference purposes

			// Analyze Type Parameters and Kinds
			typeParamVars := make([]typesystem.TVar, len(fs.TypeParams))
			typeParamNames := make([]string, len(fs.TypeParams))

			// Temporary scope for building the generic signature (uses TVars)
			sigScope := symbols.NewEnclosedSymbolTable(enclosedTable, symbols.ScopeFunction)

			for i, tp := range fs.TypeParams {
				kind := inferKindFromFunction(fs, tp.Value, enclosedTable)
				tv := typesystem.TVar{Name: tp.Value, KindVal: kind}
				typeParamVars[i] = tv
				typeParamNames[i] = tp.Value
				sigScope.DefineType(tp.Value, tv, "")
			}

			// Build params (using TVars)
			var params []typesystem.Type
			for _, p := range fs.Parameters {
				var pt typesystem.Type
				if p.Type != nil {
					var errs []*diagnostics.DiagnosticError
					pt = BuildType(p.Type, sigScope, &errs)
					if err := wrapBuildTypeError(errs); err != nil {
						return nil, nil, err
					}
					pt = OpenRecords(pt, ctx.FreshVar)
				} else {
					pt = ctx.FreshVar()
				}
				params = append(params, pt)
			}

			// Return type (using TVars)
			var retType typesystem.Type
			if fs.ReturnType != nil {
				var errs []*diagnostics.DiagnosticError
				retType = BuildType(fs.ReturnType, sigScope, &errs)
				if err := wrapBuildTypeError(errs); err != nil {
					return nil, nil, err
				}
				retType = OpenRecords(retType, ctx.FreshVar)
			} else {
				retType = ctx.FreshVar()
			}

			// Constraints (using TVars)
			var fnConstraints []typesystem.Constraint
			for _, c := range fs.Constraints {
				var cArgs []typesystem.Type
				for _, argAst := range c.Args {
					var errs []*diagnostics.DiagnosticError
					argType := BuildType(argAst, sigScope, &errs)
					if err := wrapBuildTypeError(errs); err != nil {
						return nil, nil, err
					}
					cArgs = append(cArgs, argType)
				}

				fnConstraints = append(fnConstraints, typesystem.Constraint{
					TypeVar: c.TypeVar,
					Trait:   c.Trait,
					Args:    cArgs,
				})
			}

			fnType := typesystem.TFunc{
				Params:      params,
				ReturnType:  retType,
				Constraints: fnConstraints,
			}

			// Define Generic Function (TForall) in outer scope
			var definedType typesystem.Type = fnType
			if len(typeParamVars) > 0 {
				definedType = typesystem.TForall{
					Vars:        typeParamVars,
					Constraints: fnConstraints,
					Type:        fnType,
				}
			}
			enclosedTable.DefineConstant(fs.Name.Value, definedType, "")

			// Analyze Body (Recursively)
			// Create scope for body
			fnScope := symbols.NewEnclosedSymbolTable(enclosedTable, symbols.ScopeFunction)

			// Prepare Skolemization (Rigid TCons) for body inference
			skolemSubst := typesystem.Subst{}

			// Register Rigid Type Constants in body scope
			for i, tv := range typeParamVars {
				// Name matches original param name
				name := typeParamNames[i]
				// Create Rigid TCon
				tCon := typesystem.TCon{Name: name, KindVal: tv.Kind()}
				fnScope.DefineType(name, tCon, "")
				// Map TVar -> TCon for skolemization
				skolemSubst[tv.Name] = tCon
			}

			// Apply Skolem substitution to signature for body checking
			skolemParams := make([]typesystem.Type, len(params))
			for i, p := range params {
				skolemParams[i] = p.Apply(skolemSubst)
			}
			skolemRetType := retType.Apply(skolemSubst)

			// Register params in body scope (using Skolemized types)
			for i, p := range fs.Parameters {
				pType := skolemParams[i]
				if p.IsVariadic {
					pType = typesystem.TApp{
						Constructor: typesystem.TCon{Name: config.ListTypeName},
						Args:        []typesystem.Type{pType},
					}
				}
				fnScope.Define(p.Name.Value, pType, "")
			}

			// Infer body
			if ctx.ExpectedReturnTypes == nil {
				ctx.ExpectedReturnTypes = make(map[ast.Node]typesystem.Type)
			}
			ctx.ExpectedReturnTypes[fs.Body] = skolemRetType

			// Isolate deferred constraints to handle local satisfaction
			oldConstraints := ctx.Constraints
			ctx.Constraints = make([]Constraint, 0)

			// Register constraints in active context for body analysis
			// Save current active constraints to restore later
			oldActiveConstraints := make(map[string][]Constraint)
			for k, v := range ctx.ActiveConstraints {
				oldActiveConstraints[k] = append([]Constraint(nil), v...)
			}

			// Add function constraints to context (using Skolems)
			for _, c := range fnConstraints {
				// Apply skolem subst to args
				skolemArgs := make([]typesystem.Type, len(c.Args))
				for i, arg := range c.Args {
					skolemArgs[i] = arg.Apply(skolemSubst)
				}

				// TypeVar is string name. In body, that name maps to TCon.
				// AddConstraint expects name.
				if len(skolemArgs) > 0 {
					ctx.AddMPTCConstraint(c.TypeVar, c.Trait, skolemArgs)
				} else {
					ctx.AddConstraint(c.TypeVar, c.Trait)
				}
			}

			bodyType, sBody, err := inferFn(fs.Body, fnScope)

			// Filter out constraints satisfied by local context (ActiveConstraints)
			// This prevents them from bubbling up to outer scopes where the local type params are unknown
			var remainingConstraints []Constraint
			for _, c := range ctx.Constraints {
				if c.Kind == ConstraintImplements {
					// Resolve args with latest substitution
					satisfied := false
					if len(c.Args) > 0 {
						// Simple check: first arg has constraint?
						firstArg := c.Args[0].Apply(ctx.GlobalSubst).Apply(sBody)

						// Use typeHasConstraint
						if typeHasConstraint(ctx, firstArg, c.Trait, fnScope) {
							satisfied = true
						} else {
							// Check MPTC
							// Apply to all args
							fullArgs := make([]typesystem.Type, len(c.Args))
							for i, arg := range c.Args {
								fullArgs[i] = arg.Apply(ctx.GlobalSubst).Apply(sBody)
							}
							if ctx.HasMPTCConstraint(c.Trait, fullArgs) {
								satisfied = true
							}
						}
					}

					if !satisfied {
						remainingConstraints = append(remainingConstraints, c)
					}
				} else {
					remainingConstraints = append(remainingConstraints, c)
				}
			}

			// Restore active constraints
			ctx.ActiveConstraints = oldActiveConstraints
			// Restore deferred constraints (append remaining)
			ctx.Constraints = append(oldConstraints, remainingConstraints...)

			if err != nil {
				return nil, nil, err
			}
			totalSubst = sBody.Compose(totalSubst)

			// Apply subst to types
			skolemRetType = skolemRetType.Apply(ctx.GlobalSubst).Apply(totalSubst)
			bodyType = bodyType.Apply(ctx.GlobalSubst).Apply(totalSubst)

			// Unify body type with Skolemized return type
			subst, err := typesystem.UnifyAllowExtraWithResolver(skolemRetType, bodyType, enclosedTable)
			if err != nil {
				return nil, nil, inferErrorf(fs, "return type mismatch in local function %s: expected %s, got %s", fs.Name.Value, skolemRetType, bodyType)
			}
			totalSubst = subst.Compose(totalSubst)

			// Mark tail calls since walker skips inner functions
			MarkTailCalls(fs.Body)

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
			var explicitType typesystem.Type
			if cd.TypeAnnotation != nil {
				var errs []*diagnostics.DiagnosticError
				explicitType = BuildType(cd.TypeAnnotation, enclosedTable, &errs)
				if err := wrapBuildTypeError(errs); err != nil {
					return nil, nil, err
				}

				// Propagate expected type to value inference
				if ctx.ExpectedReturnTypes == nil {
					ctx.ExpectedReturnTypes = make(map[ast.Node]typesystem.Type)
				}
				ctx.ExpectedReturnTypes[cd.Value] = explicitType
			}

			// Infer value to capture substitutions (e.g. from function calls)
			t, s, err := inferFn(cd.Value, enclosedTable)
			if err != nil {
				return nil, nil, err
			}
			totalSubst = s.Compose(totalSubst)
			t = t.Apply(ctx.GlobalSubst).Apply(totalSubst)

			// If there is an AnnotatedType, unify it with the inferred type to drive inference (e.g. for pure)
			if explicitType != nil {
				// Unify explicit type with inferred type
				// Swap args: Expected (Left), Actual (Right)
				// Use UnifyAllowExtraWithResolver to handle HKT (e.g. Writer<IntList> binding to T<IntList>)
				subst, err := typesystem.UnifyAllowExtraWithResolver(explicitType, t, enclosedTable)
				if err != nil {
					return nil, nil, inferErrorf(cd, "type mismatch in constant declaration %s: expected %s, got %s", cd.Name.Value, explicitType, t)
				}
				totalSubst = subst.Compose(totalSubst)
				ctx.GlobalSubst = subst.Compose(ctx.GlobalSubst) // CRITICAL: Update global context
				t = t.Apply(ctx.GlobalSubst).Apply(totalSubst)

				// Witness Resolution for Call Expressions: If value is a CallExpression (e.g., pure(10)),
				// set witness based on annotated type to enable runtime dispatch
				if callExpr, ok := cd.Value.(*ast.CallExpression); ok {
					declaredType := explicitType.Apply(ctx.GlobalSubst).Apply(totalSubst)
					shouldSetWitness := false

					// Check implementation by walking up TApp chain (e.g. OptionT<Id, Int> -> OptionT<Id>)
					checkType := declaredType

					// Get expected kind for Applicative (which is * -> *)
					// We use TCon to look it up from builtins, as we don't have trait kind lookup yet
					expectedKind := typesystem.TCon{Name: "Applicative"}.Kind()

					for {
						// Kind-directed check
						currentKind, err := typesystem.KindCheck(checkType)
						if err == nil && currentKind.Equal(expectedKind) {
							if enclosedTable.IsImplementationExists("Applicative", []typesystem.Type{checkType}) {
								shouldSetWitness = true
								break
							}
						}

						// Try unwrapping alias
						if tCon, ok := checkType.(typesystem.TCon); ok && tCon.UnderlyingType != nil {
							checkType = typesystem.UnwrapUnderlying(checkType)
							continue
						}

						// Try stripping last argument (HKT)
						// We only strip if we haven't found a match yet
						if tApp, ok := checkType.(typesystem.TApp); ok && len(tApp.Args) > 0 {
							if len(tApp.Args) == 1 {
								checkType = tApp.Constructor
							} else {
								checkType = typesystem.TApp{
									Constructor: tApp.Constructor,
									Args:        tApp.Args[:len(tApp.Args)-1],
								}
							}
							continue
						}
						break
					}
					if shouldSetWitness {
						if callExpr.Witness == nil {
							callExpr.Witness = make(map[string][]typesystem.Type)
						}
						if witnesses, ok := callExpr.Witness.(map[string][]typesystem.Type); ok {
							witnesses["Applicative"] = []typesystem.Type{declaredType}
						}
					}
				}
			}

			// Handle pattern destructuring or simple name binding
			if cd.Pattern != nil {
				// Use inferPattern to bind variables
				patSubst, err := inferPattern(ctx, cd.Pattern, t.Apply(totalSubst), enclosedTable)
				if err != nil {
					return nil, nil, err
				}
				totalSubst = patSubst.Compose(totalSubst)
			} else if cd.Name != nil {
				// We should also check type annotation if present, but for now we prioritize
				// capturing the substitution and defining the symbol for subsequent statements.
				enclosedTable.Define(cd.Name.Value, t.Apply(ctx.GlobalSubst).Apply(totalSubst), "")
			}

			lastType = typesystem.Nil
		}
	}
	return lastType.Apply(ctx.GlobalSubst).Apply(totalSubst), totalSubst, nil
}

func inferForExpression(ctx *InferenceContext, n *ast.ForExpression, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	loopScope := symbols.NewEnclosedSymbolTable(table, symbols.ScopeBlock)
	totalSubst := typesystem.Subst{}

	if n.Iterable != nil {
		iterableType, s1, err := inferFn(n.Iterable, table)
		if err != nil {
			return nil, nil, err
		}
		totalSubst = s1.Compose(totalSubst)
		iterableType = iterableType.Apply(ctx.GlobalSubst).Apply(totalSubst)

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

		subst, err := typesystem.Unify(typesystem.Bool, condType.Apply(ctx.GlobalSubst).Apply(totalSubst))
		if err != nil {
			return nil, nil, inferErrorf(n.Condition, "for-loop condition must be Bool, got %s", condType.Apply(ctx.GlobalSubst).Apply(totalSubst))
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
	bodyType = bodyType.Apply(ctx.GlobalSubst).Apply(totalSubst)

	subst, err := typesystem.Unify(loopReturnType.Apply(ctx.GlobalSubst).Apply(totalSubst), bodyType)
	if err != nil {
		return nil, nil, inferErrorf(n, "loop body type mismatch with break values: %v", err)
	}
	totalSubst = subst.Compose(totalSubst)

	return loopReturnType.Apply(ctx.GlobalSubst).Apply(totalSubst), totalSubst, nil
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
		subst, err := typesystem.Unify(expectedType.Type.Apply(ctx.GlobalSubst).Apply(totalSubst), valType.Apply(ctx.GlobalSubst).Apply(totalSubst))
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
