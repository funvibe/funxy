package analyzer

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

// inferPipeExpression handles pipe operator with placeholder support
// x |> f(_, y)  ≡  f(x, y)
// x |> f(y, _)  ≡  f(y, x)
// x |> f        ≡  f(x)
// x |> f(a)     ≡  f(a, x)
func inferPipeExpression(ctx *InferenceContext, n *ast.InfixExpression, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	// Infer left operand (the value being piped)
	l, s1, err := inferFn(n.Left, table)
	if err != nil {
		return nil, nil, err
	}
	totalSubst := s1
	l = l.Apply(ctx.GlobalSubst).Apply(totalSubst)

	// Check if right side is a CallExpression
	if callExpr, ok := n.Right.(*ast.CallExpression); ok {
		// Infer function type
		fnType, s2, err := inferFn(callExpr.Function, table)
		if err != nil {
			return nil, nil, err
		}
		totalSubst = s2.Compose(totalSubst)
		fnType = fnType.Apply(totalSubst)

		// Collect argument types, handling placeholders
		argTypes := make([]typesystem.Type, 0, len(callExpr.Arguments)+1)
		placeholderPos := -1

		for i, argExpr := range callExpr.Arguments {
			// Check if this is a placeholder
			if ident, ok := argExpr.(*ast.Identifier); ok && ident.Value == "_" {
				if placeholderPos == -1 {
					placeholderPos = i
					argTypes = append(argTypes, l) // Use piped value type
				} else {
					// Multiple placeholders not supported
					return nil, nil, inferErrorf(argExpr, "multiple placeholders in pipe expression not supported")
				}
			} else {
				// Regular argument - infer its type
				argType, sArg, err := inferFn(argExpr, table)
				if err != nil {
					return nil, nil, err
				}
				totalSubst = sArg.Compose(totalSubst)
				argTypes = append(argTypes, argType.Apply(totalSubst))
			}
		}

		// If no placeholder, append piped value to end
		if placeholderPos == -1 {
			argTypes = append(argTypes, l)
		}

		// Check if fnType is a function type
		if tFunc, ok := fnType.(typesystem.TFunc); ok {
			// Check parameter count
			minParams := len(tFunc.Params)
			if tFunc.IsVariadic && minParams > 0 {
				minParams--
			}

			if len(argTypes) < minParams {
				return nil, nil, inferErrorf(n, "not enough arguments: expected at least %d, got %d", minParams, len(argTypes))
			}
			if !tFunc.IsVariadic && len(argTypes) > len(tFunc.Params) {
				return nil, nil, inferErrorf(n, "too many arguments: expected %d, got %d", len(tFunc.Params), len(argTypes))
			}

			// Unify each argument with corresponding parameter
			for i, argType := range argTypes {
				if i >= len(tFunc.Params) {
					break // Variadic args
				}
				param := tFunc.Params[i].Apply(totalSubst)
				subst, err := typesystem.UnifyWithResolver(argType, param, table)
				if err != nil {
					return nil, nil, inferErrorf(n, "argument %d type mismatch: %s vs %s", i+1, argType, param)
				}
				totalSubst = subst.Compose(totalSubst)
			}

			return tFunc.ReturnType.Apply(totalSubst), totalSubst, nil
		}

		// fnType is not a function - try to unify with expected function type
		resultVar := ctx.FreshVar()
		expectedFnType := typesystem.TFunc{
			Params:     argTypes,
			ReturnType: resultVar,
		}

		subst, err := typesystem.UnifyWithResolver(fnType, expectedFnType, table)
		if err != nil {
			return nil, nil, inferErrorf(callExpr.Function, "expected function, got %s", fnType)
		}
		totalSubst = subst.Compose(totalSubst)

		return resultVar.Apply(totalSubst), totalSubst, nil
	}

	// Right side is not a CallExpression - infer it normally
	r, s2, err := inferFn(n.Right, table)
	if err != nil {
		return nil, nil, err
	}
	totalSubst = s2.Compose(totalSubst)
	l = l.Apply(totalSubst)
	r = r.Apply(totalSubst)

	// Standard pipe: x |> f ≡ f(x)
	// Check if r is a function type
	if fnType, ok := r.(typesystem.TFunc); ok {
		if len(fnType.Params) >= 1 {
			// Unify left operand with first parameter
			subst, err := typesystem.UnifyWithResolver(l, fnType.Params[0], table)
			if err != nil {
				return nil, nil, inferErrorf(n.Left, "cannot pipe %s to function expecting %s", l, fnType.Params[0])
			}
			totalSubst = subst.Compose(totalSubst)
			return fnType.ReturnType.Apply(totalSubst), totalSubst, nil
		}
	}

	// General case: unify with expected function type
	resultVar := ctx.FreshVar()
	expectedFnType := typesystem.TFunc{
		Params:     []typesystem.Type{l},
		ReturnType: resultVar,
	}

	subst, err := typesystem.UnifyWithResolver(r, expectedFnType, table)
	if err != nil {
		return nil, nil, inferErrorf(n.Right, "right operand of |> must be a function (T) -> R, got %s", r)
	}
	totalSubst = subst.Compose(totalSubst)

	return resultVar.Apply(totalSubst), totalSubst, nil
}
