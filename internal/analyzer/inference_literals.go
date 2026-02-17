package analyzer

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
	"sort"
)

func inferLiteral(ctx *InferenceContext, node ast.Node, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	switch n := node.(type) {
	case *ast.IntegerLiteral:
		return typesystem.Int, typesystem.Subst{}, nil

	case *ast.FloatLiteral:
		return typesystem.Float, typesystem.Subst{}, nil

	case *ast.BigIntLiteral:
		return typesystem.BigInt, typesystem.Subst{}, nil

	case *ast.RationalLiteral:
		return typesystem.Rational, typesystem.Subst{}, nil

	case *ast.BooleanLiteral:
		return typesystem.Bool, typesystem.Subst{}, nil

	case *ast.NilLiteral:
		return typesystem.Nil, typesystem.Subst{}, nil

	case *ast.StringLiteral:
		return typesystem.TApp{
			Constructor: typesystem.TCon{Name: config.ListTypeName},
			Args:        []typesystem.Type{typesystem.TCon{Name: "Char"}},
		}, typesystem.Subst{}, nil

	case *ast.InterpolatedString:
		// Interpolated strings also return List<Char>
		// Analyze all parts to catch any errors
		for _, part := range n.Parts {
			_, _, err := inferFn(part, table)
			if err != nil {
				return nil, nil, err
			}
		}
		return typesystem.TApp{
			Constructor: typesystem.TCon{Name: config.ListTypeName},
			Args:        []typesystem.Type{typesystem.TCon{Name: "Char"}},
		}, typesystem.Subst{}, nil

	case *ast.FormatStringLiteral:
		// Format string literal creates a variadic formatter function: (...args) -> String
		// It can accept any number of arguments of any type
		paramType := ctx.FreshVar()
		return typesystem.TFunc{
			Params: []typesystem.Type{paramType},
			ReturnType: typesystem.TApp{
				Constructor: typesystem.TCon{Name: config.ListTypeName},
				Args:        []typesystem.Type{typesystem.TCon{Name: "Char"}},
			},
			IsVariadic: true, // Variadic function - accepts any number of arguments
		}, typesystem.Subst{}, nil

	case *ast.CharLiteral:
		return typesystem.TCon{Name: "Char"}, typesystem.Subst{}, nil

	case *ast.BytesLiteral:
		return typesystem.TCon{Name: config.BytesTypeName}, typesystem.Subst{}, nil

	case *ast.BitsLiteral:
		return typesystem.TCon{Name: config.BitsTypeName}, typesystem.Subst{}, nil

	case *ast.TupleLiteral:
		elementTypes := []typesystem.Type{}
		totalSubst := typesystem.Subst{}

		for _, el := range n.Elements {
			t, s, err := inferFn(el, table)
			if err != nil {
				return nil, nil, err
			}
			totalSubst = s.Compose(totalSubst)
			elementTypes = append(elementTypes, t)
		}
		// Apply accumulated substitution to all elements to ensure consistency?
		// Yes, if later elements refined type variables used in earlier elements.
		finalElements := []typesystem.Type{}
		for _, t := range elementTypes {
			finalElements = append(finalElements, t.Apply(totalSubst))
		}

		return typesystem.TTuple{Elements: finalElements}, totalSubst, nil

	case *ast.RecordLiteral:
		fieldTypes := make(map[string]typesystem.Type)
		totalSubst := typesystem.Subst{}

		// Handle spread expression first: { ...base, key: val }
		if n.Spread != nil {
			spreadType, s, err := inferFn(n.Spread, table)
			if err != nil {
				return nil, nil, err
			}
			totalSubst = s.Compose(totalSubst)
			spreadType = spreadType.Apply(totalSubst)

			// Resolve type alias to get underlying record type
			spreadType = table.ResolveTypeAlias(spreadType)

			// Spread type must be a record
			if rec, ok := spreadType.(typesystem.TRecord); ok {
				// Copy fields from spread base
				for k, v := range rec.Fields {
					fieldTypes[k] = v
				}
			} else {
				return nil, nil, inferErrorf(n.Spread, "spread expression must be a record type, got %s", spreadType)
			}
		}

		// Sort keys for deterministic type variable naming
		keys := make([]string, 0, len(n.Fields))
		for k := range n.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// Override/add fields from explicit field definitions
		for _, k := range keys {
			v := n.Fields[k]
			t, s, err := inferFn(v, table)
			if err != nil {
				return nil, nil, err
			}
			totalSubst = s.Compose(totalSubst)
			fieldTypes[k] = t
		}

		finalFields := make(map[string]typesystem.Type)
		for k, t := range fieldTypes {
			finalFields[k] = t.Apply(totalSubst)
		}

		// Return anonymous record type by default
		// Nominal typing is handled via explicit type annotations or unification
		// Empty record literal {} is treated as Open to allow it to unify with any record (as a base/default)
		isOpen := len(finalFields) == 0
		return typesystem.TRecord{Fields: finalFields, IsOpen: isOpen}, totalSubst, nil

	case *ast.ListLiteral:
		if n == nil {
			return nil, nil, fmt.Errorf("panic prevention: nil ListLiteral")
		}
		if len(n.Elements) == 0 {
			return typesystem.TApp{
				Constructor: typesystem.TCon{Name: config.ListTypeName},
				Args:        []typesystem.Type{ctx.FreshVar()},
			}, typesystem.Subst{}, nil
		} else {
			totalSubst := typesystem.Subst{}
			var elemType typesystem.Type

			// Infer first element
			firstNode := n.Elements[0]
			firstType, s1, err := inferFn(firstNode, table)
			if err != nil {
				return nil, nil, err
			}
			totalSubst = s1.Compose(totalSubst)

			if _, ok := firstNode.(*ast.SpreadExpression); ok {
				// Resolve alias (e.g. String -> List<Char>)
				if table != nil {
					firstType = table.ResolveTypeAlias(firstType)
				}

				// If spread, firstType is the List type (List<T>).
				if tApp, ok := firstType.(typesystem.TApp); ok {
					if tCon, ok := tApp.Constructor.(typesystem.TCon); ok && tCon.Name == config.ListTypeName && len(tApp.Args) == 1 {
						elemType = tApp.Args[0]
					} else {
						return nil, nil, inferError(firstNode, "spread element must be a List")
					}
				} else if _, ok := firstType.(typesystem.TVar); ok {
					elemType = ctx.FreshVar()
					listType := typesystem.TApp{
						Constructor: typesystem.TCon{Name: config.ListTypeName},
						Args:        []typesystem.Type{elemType},
					}
					subst, err := typesystem.Unify(firstType, listType)
					if err != nil {
						return nil, nil, inferErrorf(firstNode, "spread element expected List, got %s", firstType)
					}
					totalSubst = subst.Compose(totalSubst)
					elemType = elemType.Apply(totalSubst)
				} else {
					return nil, nil, inferErrorf(firstNode, "spread element must be a List, got %s", firstType)
				}
			} else {
				elemType = firstType
			}

			for i := 1; i < len(n.Elements); i++ {
				node := n.Elements[i]
				nextType, sNext, err := inferFn(node, table)
				if err != nil {
					return nil, nil, err
				}
				totalSubst = sNext.Compose(totalSubst)

				// Apply known substitution to current types before unification
				elemType = elemType.Apply(totalSubst)
				nextType = nextType.Apply(totalSubst)

				var itemType typesystem.Type
				if _, ok := node.(*ast.SpreadExpression); ok {
					if tApp, ok := nextType.(typesystem.TApp); ok {
						if tCon, ok := tApp.Constructor.(typesystem.TCon); ok && tCon.Name == config.ListTypeName && len(tApp.Args) == 1 {
							itemType = tApp.Args[0]
						} else {
							return nil, nil, inferError(firstNode, "spread element must be a List")
						}
					} else if _, ok := nextType.(typesystem.TVar); ok {
						listType := typesystem.TApp{
							Constructor: typesystem.TCon{Name: config.ListTypeName},
							Args:        []typesystem.Type{elemType},
						}
						subst, err := typesystem.Unify(nextType, listType)
						if err != nil {
							return nil, nil, inferErrorf(node, "spread element type mismatch: %s vs %s", nextType, listType)
						}
						totalSubst = subst.Compose(totalSubst)
						elemType = elemType.Apply(totalSubst)
						itemType = elemType // Resolved
					} else {
						return nil, nil, inferErrorf(node, "spread element must be a known List, got %s", nextType)
					}
				} else {
					itemType = nextType
				}

				subst, err := typesystem.Unify(elemType, itemType)
				if err != nil {
					return nil, nil, inferErrorf(node, "list element type mismatch: %s vs %s", elemType, itemType)
				}
				totalSubst = subst.Compose(totalSubst)
				elemType = elemType.Apply(totalSubst)
			}

			return typesystem.TApp{
				Constructor: typesystem.TCon{Name: config.ListTypeName},
				Args:        []typesystem.Type{elemType},
			}, totalSubst, nil
		}

	case *ast.MapLiteral:
		if len(n.Pairs) == 0 {
			// Empty map: Map<k, v> with fresh type variables
			return typesystem.TApp{
				Constructor: typesystem.TCon{Name: config.MapTypeName},
				Args:        []typesystem.Type{ctx.FreshVar(), ctx.FreshVar()},
			}, typesystem.Subst{}, nil
		}

		totalSubst := typesystem.Subst{}

		// Infer first pair
		keyType, s1, err := inferFn(n.Pairs[0].Key, table)
		if err != nil {
			return nil, nil, err
		}
		totalSubst = s1.Compose(totalSubst)

		valType, s2, err := inferFn(n.Pairs[0].Value, table)
		if err != nil {
			return nil, nil, err
		}
		totalSubst = s2.Compose(totalSubst)

		// Unify remaining pairs
		for i := 1; i < len(n.Pairs); i++ {
			pair := n.Pairs[i]

			nextKeyType, sk, err := inferFn(pair.Key, table)
			if err != nil {
				return nil, nil, err
			}
			totalSubst = sk.Compose(totalSubst)
			keyType = keyType.Apply(totalSubst)
			nextKeyType = nextKeyType.Apply(totalSubst)

			subst, err := typesystem.Unify(keyType, nextKeyType)
			if err != nil {
				return nil, nil, inferErrorf(pair.Key, "map key type mismatch: %s vs %s", keyType, nextKeyType)
			}
			totalSubst = subst.Compose(totalSubst)
			keyType = keyType.Apply(totalSubst)

			nextValType, sv, err := inferFn(pair.Value, table)
			if err != nil {
				return nil, nil, err
			}
			totalSubst = sv.Compose(totalSubst)
			valType = valType.Apply(totalSubst)
			nextValType = nextValType.Apply(totalSubst)

			subst, err = typesystem.Unify(valType, nextValType)
			if err != nil {
				return nil, nil, inferErrorf(pair.Value, "map value type mismatch: %s vs %s", valType, nextValType)
			}
			totalSubst = subst.Compose(totalSubst)
			valType = valType.Apply(totalSubst)
		}

		return typesystem.TApp{
			Constructor: typesystem.TCon{Name: config.MapTypeName},
			Args:        []typesystem.Type{keyType, valType},
		}, totalSubst, nil
	}
	return nil, nil, inferErrorf(node, "unknown literal type: %T", node)
}

// inferListComprehension infers the type of a list comprehension
// [output | clause, clause, ...]
// The result type is List<T> where T is the type of the output expression
func inferListComprehension(ctx *InferenceContext, n *ast.ListComprehension, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	totalSubst := typesystem.Subst{}

	// Create a new scope for the comprehension
	compScope := symbols.NewEnclosedSymbolTable(table, symbols.ScopeBlock)

	// Process each clause to bind variables and infer types
	for _, clause := range n.Clauses {
		switch c := clause.(type) {
		case *ast.CompGenerator:
			// Infer the type of the iterable
			iterType, s, err := inferFn(c.Iterable, compScope)
			if err != nil {
				return nil, nil, err
			}
			totalSubst = s.Compose(totalSubst)
			iterType = iterType.Apply(totalSubst)

			// Resolve type alias (e.g. String -> List<Char>)
			iterType = table.ResolveTypeAlias(iterType)

			// Extract element type from iterable
			var elemType typesystem.Type
			if tApp, ok := iterType.(typesystem.TApp); ok {
				if tCon, ok := tApp.Constructor.(typesystem.TCon); ok {
					if tCon.Name == config.ListTypeName && len(tApp.Args) == 1 {
						elemType = tApp.Args[0]
					} else if tCon.Name == "Range" && len(tApp.Args) == 1 {
						elemType = tApp.Args[0]
					} else {
						return nil, nil, inferErrorf(c.Iterable, "generator iterable must be a List or Range, got %s", iterType)
					}
				} else {
					return nil, nil, inferErrorf(c.Iterable, "generator iterable must be a List or Range, got %s", iterType)
				}
			} else if tVar, ok := iterType.(typesystem.TVar); ok {
				// Unknown type, create fresh element type and constrain
				elemType = ctx.FreshVar()
				listType := typesystem.TApp{
					Constructor: typesystem.TCon{Name: config.ListTypeName},
					Args:        []typesystem.Type{elemType},
				}
				subst, err := typesystem.Unify(tVar, listType)
				if err != nil {
					return nil, nil, inferErrorf(c.Iterable, "generator iterable must be a List, got %s", iterType)
				}
				totalSubst = subst.Compose(totalSubst)
				elemType = elemType.Apply(totalSubst)
			} else {
				return nil, nil, inferErrorf(c.Iterable, "generator iterable must be a List, got %s", iterType)
			}

			// Bind pattern variables with the element type
			bindPatternType(c.Pattern, elemType, compScope)

		case *ast.CompFilter:
			// Infer the type of the filter condition
			condType, s, err := inferFn(c.Condition, compScope)
			if err != nil {
				return nil, nil, err
			}
			totalSubst = s.Compose(totalSubst)
			condType = condType.Apply(totalSubst)

			// Filter condition must be Bool
			subst, err := typesystem.Unify(condType, typesystem.Bool)
			if err != nil {
				return nil, nil, inferErrorf(c.Condition, "filter condition must be Bool, got %s", condType)
			}
			totalSubst = subst.Compose(totalSubst)
		}
	}

	// Infer the type of the output expression
	outputType, s, err := inferFn(n.Output, compScope)
	if err != nil {
		return nil, nil, err
	}
	totalSubst = s.Compose(totalSubst)
	outputType = outputType.Apply(totalSubst)

	// Result is List<outputType>
	return typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{outputType},
	}, totalSubst, nil
}

// bindPatternType binds variables in a pattern to the given type in the symbol table
func bindPatternType(pattern ast.Pattern, t typesystem.Type, table *symbols.SymbolTable) {
	switch p := pattern.(type) {
	case *ast.IdentifierPattern:
		if p.Value != "_" {
			table.Define(p.Value, t, "comprehension")
		}
	case *ast.WildcardPattern:
		// Nothing to bind
	case *ast.TuplePattern:
		if tuple, ok := t.(typesystem.TTuple); ok && len(tuple.Elements) == len(p.Elements) {
			for i, elem := range p.Elements {
				bindPatternType(elem, tuple.Elements[i], table)
			}
		}
	case *ast.ListPattern:
		if tApp, ok := t.(typesystem.TApp); ok {
			if tCon, ok := tApp.Constructor.(typesystem.TCon); ok && tCon.Name == config.ListTypeName && len(tApp.Args) == 1 {
				elemType := tApp.Args[0]
				for _, elem := range p.Elements {
					bindPatternType(elem, elemType, table)
				}
			}
		}
	}
}
