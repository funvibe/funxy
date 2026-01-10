package analyzer

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

func inferRangeExpression(ctx *InferenceContext, n *ast.RangeExpression, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	// 1. Infer start
	startType, s1, err := inferFn(n.Start, table)
	if err != nil {
		return nil, nil, err
	}
	totalSubst := s1

	// 2. Infer next (if present)
	if n.Next != nil {
		nextType, s2, err := inferFn(n.Next, table)
		if err != nil {
			return nil, nil, err
		}
		totalSubst = s2.Compose(totalSubst)

		// Start and Next must be same type
		unifySubst, err := typesystem.Unify(startType.Apply(totalSubst), nextType)
		if err != nil {
			return nil, nil, inferErrorf(n.Next, "range step type mismatch: %s vs %s", startType, nextType)
		}
		totalSubst = unifySubst.Compose(totalSubst)
	}

	// 3. Infer end
	endType, s3, err := inferFn(n.End, table)
	if err != nil {
		return nil, nil, err
	}
	totalSubst = s3.Compose(totalSubst)

	// Start and End must be same type
	unifySubst, err := typesystem.Unify(startType.Apply(totalSubst), endType)
	if err != nil {
		return nil, nil, inferErrorf(n.End, "range end type mismatch: %s vs %s", startType, endType)
	}
	totalSubst = unifySubst.Compose(totalSubst)

	// Result is Range<T>
	// We use "Range" as the type name, which should be registered in builtins
	elementType := startType.Apply(totalSubst)
	rangeType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Range"},
		Args:        []typesystem.Type{elementType},
	}

	return rangeType, totalSubst, nil
}
