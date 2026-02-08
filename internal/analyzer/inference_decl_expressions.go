package analyzer

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

func inferAnnotatedExpression(ctx *InferenceContext, n *ast.AnnotatedExpression, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	if n.TypeAnnotation == nil {
		return nil, nil, inferError(n, "missing type annotation")
	}

	// Convert AST type to Internal type
	var errs []*diagnostics.DiagnosticError
	annotatedType := BuildType(n.TypeAnnotation, table, &errs)
	if err := wrapBuildTypeError(errs); err != nil {
		return nil, nil, err
	}

	// Kind Check: Annotation must be a proper type (Kind *)
	if k, err := typesystem.KindCheck(annotatedType); err != nil {
		return nil, nil, err
	} else if !k.Equal(typesystem.Star) {
		return nil, nil, inferError(n, "type annotation must be type (kind *), got kind "+k.String())
	}

	// Contextual Type Propagation: Push expected type to inner expression to guide inference (e.g. MPTC resolution)
	if ctx.ExpectedReturnTypes == nil {
		ctx.ExpectedReturnTypes = make(map[ast.Node]typesystem.Type)
	}
	ctx.ExpectedReturnTypes[n.Expression] = annotatedType

	// Infer type of inner expression
	exprType, s1, err := inferFn(n.Expression, table)
	if err != nil {
		return nil, nil, err
	}
	totalSubst := s1
	exprType = exprType.Apply(ctx.GlobalSubst).Apply(totalSubst)

	// Unify them (Check if exprType is a subtype of annotatedType)
	// Swap args: Expected, Actual
	subst, err := typesystem.UnifyAllowExtraWithResolver(annotatedType, exprType, table)
	if err != nil {
		// Auto-call logic for nullary functions (e.g. mempty)
		if tFunc, ok := exprType.(typesystem.TFunc); ok && len(tFunc.Params) == 0 {
			substCall, errCall := typesystem.UnifyAllowExtraWithResolver(annotatedType, tFunc.ReturnType, table)
			if errCall == nil {
				// Rewrite AST to CallExpression
				n.Expression = &ast.CallExpression{
					Function:  n.Expression,
					Arguments: []ast.Expression{},
				}
				// Update types
				subst = substCall
				exprType = tFunc.ReturnType
				err = nil
			}
		}
	}

	if err != nil {
		return nil, nil, typeMismatch(n, annotatedType.String(), exprType.String())
	}
	totalSubst = subst.Compose(totalSubst)
	finalType := exprType.Apply(ctx.GlobalSubst).Apply(totalSubst)

	// Witness Resolution for Call Expressions: Legacy manual setting removed
	// inferCallExpression handles this via ExpectedReturnTypes and PendingWitnesses.

	return finalType, totalSubst, nil
}

func inferIdentifier(ctx *InferenceContext, n *ast.Identifier, table *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error) {
	sym, ok := table.Find(n.Value)
	if !ok {
		// Find similar names for suggestion
		suggestions := findSimilarNames(n.Value, table, 2)
		if len(suggestions) > 0 {
			return nil, nil, undefinedWithHint(n, n.Value, "did you mean: "+suggestions[0]+"?")
		}
		return nil, nil, undefinedSymbol(n, n.Value)
	}

	// Store resolved symbol in ResolutionMap
	if ctx.ResolutionMap != nil {
		ctx.ResolutionMap[n] = sym
	}

	if sym.Type == nil {
		return nil, nil, inferErrorf(n, "symbol %s has no type", n.Value)
	}

	// If it is a TypeSymbol, return TType wrapping the type
	if sym.Kind == symbols.TypeSymbol {
		// For type aliases, use underlying type for unification
		if sym.IsTypeAlias() {
			return typesystem.TType{Type: sym.UnderlyingType}, typesystem.Subst{}, nil
		}
		return typesystem.TType{Type: sym.Type}, typesystem.Subst{}, nil
	} else {
		// Check context for Rank-N expectation
		// If the context expects a polytype (Forall), and the symbol is a polytype (or implicitly generic),
		// do NOT instantiate it. Pass it as is (lifted to TForall).
		if expectedType, ok := ctx.ExpectedTypes[n]; ok {
			// Resolve alias if any
			resolved := table.ResolveTypeAlias(expectedType)
			if _, isExpectedForall := resolved.(typesystem.TForall); isExpectedForall {
				// If symbol is already explicit Forall, return it directly.
				if _, isSymForall := sym.Type.(typesystem.TForall); isSymForall {
					return sym.Type, typesystem.Subst{}, nil
				}

				// If symbol is TFunc (implicit generic), lift it to TForall
				// We check if it has free variables that would normally be instantiated.
				// Important: Only do this for definitions (Kind == Variable/Constant), not for locally bound variables if we tracked them differently.
				// But generally, free vars in symbol table entries imply generics.
				freeVars := sym.Type.FreeTypeVariables()
				if len(freeVars) > 0 {
					// Sort vars to ensure deterministic order (important for Unify which might rely on order)
					// typesystem.FreeTypeVariables usually returns sorted list?
					// Let's assume yes or that Unify skolemization handles it.
					// Actually, FreeTypeVariables returns []TVar.

					// Construct TForall on the fly
					polyType := typesystem.TForall{
						Vars: freeVars,
						Type: sym.Type,
					}
					return polyType, typesystem.Subst{}, nil
				}
			}
		}

		// Instantiate generic types to avoid collisions and support polymorphism
		// Only instantiate GENERIC parameters (from previous scopes), not local inference variables.
		instType, mapping := InstantiateGenericsWithSubst(ctx, sym.Type)
		n.TypeVarMapping = mapping // Store the mapping for monomorphization

		if instType != nil {
			if ctx.TypeMap != nil {
				// Always update TypeMap with the latest instantiated type
				ctx.TypeMap[n] = instType
			}
		}
		return instType, typesystem.Subst{}, nil
	}
}

func inferSpreadExpression(ctx *InferenceContext, n *ast.SpreadExpression, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	// SpreadExpression unwraps a tuple.
	return inferFn(n.Expression, table)
}
