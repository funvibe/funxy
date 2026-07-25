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
		nominalFieldTypes := make(map[string]typesystem.Type)
		totalSubst := typesystem.Subst{}

		// Handle spread expression first: { ...base, key: val }
		if n.Spread != nil {
			spreadType, s, err := inferFn(n.Spread, table)
			if err != nil {
				return nil, nil, err
			}
			totalSubst = s.Compose(totalSubst)
			spreadType = spreadType.Apply(totalSubst)
			if nominalRecord, ok := resolveRecordShapePreservingAliases(spreadType, table); ok {
				for k, v := range nominalRecord.Fields {
					nominalFieldTypes[k] = v
				}
			}

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

		// Override/add fields from explicit field definitions.
		// When a spread base provides a field type, propagate it as the expected
		// type for the overriding value. This is critical for nominal type
		// preservation: e.g. `{...box, item: {target: ..., weight: ..., fails: ...}}`
		// where `item` has type `Item` (a type alias for a record). Without
		// propagating the expected field type, the inner record literal is
		// inferred as an anonymous TRecord and loses its `Item` nominal tag,
		// causing downstream pattern matches on `item: Item` to fail at runtime.
		for _, k := range keys {
			v := n.Fields[k]
			if existingType, hasField := fieldTypes[k]; hasField && ctx.ExpectedTypes != nil {
				if nominalType, hasNominalField := nominalFieldTypes[k]; hasNominalField {
					existingType = nominalType
				}
				ctx.ExpectedTypes[v] = existingType.Apply(totalSubst)
			}
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

		// Return anonymous record type by default.
		// Nominal typing is handled via explicit type annotations or unification.
		// Empty record literal {} is treated as Open to allow it to unify with any record (as a base/default)
		isOpen := len(finalFields) == 0
		resultRecord := typesystem.TRecord{Fields: finalFields, IsOpen: isOpen}

		// Nominal type preservation: if there is an expected type for this
		// record literal that is a nominal type (TCon) whose underlying type
		// (via alias resolution) is a record, return the nominal TCon instead
		// of the anonymous TRecord. This ensures the TypeMap stores the nominal
		// type name so the VM compiler can emit it as the record's TypeName,
		// preserving nominal identity for downstream pattern matching and
		// trait dispatch (e.g. `match box.item { item: Item -> ... }`).
		if expectedType, ok := ctx.ExpectedTypes[n]; ok {
			nominalTypes := resolveNominalRecordTypes(expectedType, table)
			if len(nominalTypes) > 0 {
				// Structural validation before adopting the nominal tag.
				// Returning the nominal type (TCon or TApp) bypasses the unification the
				// caller would normally perform against the anonymous TRecord,
				// so we must verify here that the literal actually matches the
				// underlying record structure. Without this, a literal with a
				// wrong field type or a missing field would silently be stamped
				// with the nominal type and fail at runtime (e.g. `record has no
				// field 'fails'`).
				//
				// Validation is strict (no width subtyping): a literal with
				// extra fields is rejected, consistent with how record function
				// returns are checked (statements.go). Otherwise the nominal
				// tag would swallow the extra fields and hide them from the
				// outer strict return check, creating an inconsistency between
				// direct (`{item: Item}`) and union (`{item: Item | ...}`)
				// contexts. The sole exception is the "any record" idiom
				// (`type alias AnyRecord = {}`), a closed empty record used to
				// accept any record shape; there we must allow width subtyping.
				resolver := &ResolverWrapper{Table: table, Ctx: ctx}
				var firstErr error
				for _, nominalType := range nominalTypes {
					underlying := table.ResolveTypeAlias(nominalType)

					allowExtra := false
					if rec, isRec := underlying.(typesystem.TRecord); isRec && len(rec.Fields) == 0 {
						allowExtra = true
					}

					var s typesystem.Subst
					var err error
					if allowExtra {
						s, err = typesystem.UnifyAllowExtraWithResolver(underlying, resultRecord, resolver)
					} else {
						s, err = typesystem.UnifyWithResolver(underlying, resultRecord, resolver)
					}
					if err != nil {
						if firstErr == nil {
							firstErr = err
						}
						continue
					}
					totalSubst = s.Compose(totalSubst)
					ctx.GlobalSubst = s.Compose(ctx.GlobalSubst)
					return nominalType, totalSubst, nil
				}
				return nil, nil, inferErrorf(n, "record literal does not match any nominal record in expected type %s: %s", expectedType, firstErr)
			}
		}

		return resultRecord, totalSubst, nil

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

// inferMapComprehension infers the type of a map comprehension
// %{ key => value | clause, clause, ... }
// The result type is Map<K, V> where K and V are the types of the key and value expressions
func inferMapComprehension(ctx *InferenceContext, n *ast.MapComprehension, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
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

			// Resolve type alias
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

	// Infer the type of the key expression
	keyType, s1, err := inferFn(n.Key, compScope)
	if err != nil {
		return nil, nil, err
	}
	totalSubst = s1.Compose(totalSubst)
	keyType = keyType.Apply(totalSubst)

	// Infer the type of the value expression
	valType, s2, err := inferFn(n.Value, compScope)
	if err != nil {
		return nil, nil, err
	}
	totalSubst = s2.Compose(totalSubst)
	valType = valType.Apply(totalSubst)

	// Result is Map<keyType, valType>
	return typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Map"},
		Args:        []typesystem.Type{keyType, valType},
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

// resolveNominalRecordTypes returns nominal record candidates contained in an
// expected type. Candidates are validated against the literal by the caller;
// this is important for unions such as A | B where both members are records.
// The complete TApp is preserved so generic arguments remain available during
// structural validation and in the TypeMap.
func resolveNominalRecordTypes(expectedType typesystem.Type, table *symbols.SymbolTable) []typesystem.Type {
	if expectedType == nil {
		return nil
	}
	if union, ok := expectedType.(typesystem.TUnion); ok {
		var candidates []typesystem.Type
		for _, member := range union.Types {
			candidates = append(candidates, resolveNominalRecordTypes(member, table)...)
		}
		return candidates
	}
	resolved := table.ResolveTypeAlias(expectedType)
	if _, isRecord := resolved.(typesystem.TRecord); !isRecord {
		return nil
	}
	switch expectedType.(type) {
	case typesystem.TCon, typesystem.TApp:
		return []typesystem.Type{expectedType}
	default:
		return nil
	}
}

// resolveRecordShapePreservingAliases expands only the outer record alias.
// Unlike SymbolTable.ResolveTypeAlias, it intentionally leaves field aliases
// nominal so an overridden spread field can pass Item or Entry<Int> as the
// expected type of its inline record literal.
func resolveRecordShapePreservingAliases(t typesystem.Type, table *symbols.SymbolTable) (typesystem.TRecord, bool) {
	return resolveRecordShapePreservingAliasesDepth(t, table, 0)
}

func resolveRecordShapePreservingAliasesDepth(t typesystem.Type, table *symbols.SymbolTable, depth int) (typesystem.TRecord, bool) {
	if t == nil || depth > 64 {
		return typesystem.TRecord{}, false
	}
	switch ty := t.(type) {
	case typesystem.TRecord:
		return ty, true
	case typesystem.TCon:
		underlying := ty.UnderlyingType
		if underlying == nil {
			lookupName := ty.Name
			if ty.Module != "" {
				lookupName = ty.Module + "." + ty.Name
			}
			if alias, ok := table.GetTypeAlias(lookupName); ok {
				underlying = alias
			}
		}
		if underlying == nil {
			return typesystem.TRecord{}, false
		}
		return resolveRecordShapePreservingAliasesDepth(underlying, table, depth+1)
	case typesystem.TApp:
		constructor, ok := ty.Constructor.(typesystem.TCon)
		if !ok {
			return typesystem.TRecord{}, false
		}
		underlying := constructor.UnderlyingType
		lookupName := constructor.Name
		if constructor.Module != "" {
			lookupName = constructor.Module + "." + constructor.Name
		}
		if underlying == nil {
			if alias, found := table.GetTypeAlias(lookupName); found {
				underlying = alias
			}
		}
		if underlying == nil {
			return typesystem.TRecord{}, false
		}
		var params []string
		if constructor.TypeParams != nil {
			params = *constructor.TypeParams
		} else if registered, found := table.GetTypeParams(lookupName); found {
			params = registered
		} else if registered, found := table.GetTypeParams(constructor.Name); found {
			params = registered
		}
		if len(params) == len(ty.Args) {
			subst := make(typesystem.Subst, len(params))
			for i, param := range params {
				subst[param] = ty.Args[i]
			}
			underlying = underlying.Apply(subst)
		}
		return resolveRecordShapePreservingAliasesDepth(underlying, table, depth+1)
	default:
		return typesystem.TRecord{}, false
	}
}
