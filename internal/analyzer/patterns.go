package analyzer

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
	"sort"
)

// inferPattern infers types in a pattern and binds variables in the symbol table.
func inferPattern(ctx *InferenceContext, pat ast.Pattern, expectedType typesystem.Type, table *symbols.SymbolTable) (typesystem.Subst, error) {
	switch p := pat.(type) {
	case *ast.WildcardPattern:
		return typesystem.Subst{}, nil // Matches anything

	case *ast.IdentifierPattern:
		// Bind variable
		table.Define(p.Value, expectedType, "")

		// Map AST node to Symbol for LSP
		if ctx.ResolutionMap != nil {
			if sym, found := table.Find(p.Value); found {
				ctx.ResolutionMap[p] = sym
			}
		}

		// Also register in TypeMap
		if ctx.TypeMap != nil {
			ctx.TypeMap[p] = expectedType
		}
		return typesystem.Subst{}, nil

	case *ast.PinPattern:
		// Pin pattern: ^variable (compare with existing value, no new binding)
		sym, ok := table.Find(p.Name)
		if !ok {
			return nil, inferErrorf(p, "undefined variable in pin pattern: %s", p.Name)
		}
		// Unify the pinned variable's type with expected type
		return typesystem.Unify(expectedType, sym.Type)

	case *ast.TypePattern:
		// Type pattern: n: Int
		// Build the type from AST
		var errs []*diagnostics.DiagnosticError
		patternType := BuildType(p.Type, table, &errs)
		if len(errs) > 0 {
			return nil, errs[0]
		}

		// Check if expected type can contain this pattern type
		// For union types, check if patternType is a member
		// For non-union types, unify directly
		if union, ok := expectedType.(typesystem.TUnion); ok {
			// Check if patternType is a member of the union
			found := false
			for _, member := range union.Types {
				if _, err := typesystem.Unify(member, patternType); err == nil {
					found = true
					break
				}
			}
			if !found {
				return nil, inferErrorf(p, "type pattern %s is not a member of union %s", patternType, expectedType)
			}
		} else {
			// Non-union: try to unify
			if _, err := typesystem.Unify(expectedType, patternType); err != nil {
				return nil, inferErrorf(p, "type pattern %s does not match expected type %s", patternType, expectedType)
			}
		}

		// Bind the variable with the narrowed type
		if p.Name != "_" {
			table.Define(p.Name, patternType, "")
			// Important: Map AST node to Symbol for LSP hover/goto definition
			if ctx.ResolutionMap != nil {
				if sym, found := table.Find(p.Name); found {
					ctx.ResolutionMap[p] = sym
				}
			}
		}
		return typesystem.Subst{}, nil

	case *ast.LiteralPattern:
		var litType typesystem.Type
		switch p.Value.(type) {
		case int64:
			litType = typesystem.Int
		case bool:
			litType = typesystem.Bool
		case string:
			litType = typesystem.TApp{
				Constructor: typesystem.TCon{Name: config.ListTypeName},
				Args:        []typesystem.Type{typesystem.Char},
			}
		case rune:
			litType = typesystem.Char
		default:
			return nil, inferErrorf(p, "unknown literal type in pattern: %T", p.Value)
		}
		return typesystem.Unify(expectedType, litType)

	case *ast.StringPattern:
		// String pattern matches String (List<Char>) and binds captured variables
		stringType := typesystem.TApp{
			Constructor: typesystem.TCon{Name: config.ListTypeName},
			Args:        []typesystem.Type{typesystem.Char},
		}
		// Define captured variables as String
		for _, part := range p.Parts {
			if part.IsCapture && part.Value != "_" {
				table.Define(part.Value, stringType, "")
			}
		}
		return typesystem.Unify(expectedType, stringType)

	case *ast.ConstructorPattern:
		sym, ok := table.Find(p.Name.Value)
		if !ok {
			return nil, inferErrorf(p, "undefined symbol: %s", p.Name.Value)
		}

		// Allow matching against Type names (String, Int, etc.)
		// 1. If Kind is TypeSymbol (defined via type alias)
		// 2. If Kind is VariableSymbol (defined via DefineConstant) AND Type is TType (runtime Type Object)
		isTypeSymbol := sym.Kind == symbols.TypeSymbol
		isTypeConstant := sym.Kind == symbols.VariableSymbol && isTType(sym.Type)

		if (isTypeSymbol || isTypeConstant) && len(p.Elements) == 0 {
			// The pattern is a Type Object.
			// If it's TypeSymbol, its type is Type<T> where T is the type it represents.
			// If it's VariableSymbol (Type Constant), its type is already Type<T>.
			var typeObjType typesystem.Type
			if isTypeSymbol {
				typeObjType = typesystem.TType{Type: sym.Type}
			} else {
				typeObjType = sym.Type
			}

			subst, err := typesystem.Unify(expectedType, typeObjType)
			if err != nil {
				return nil, inferErrorf(p, "pattern type mismatch: expected %s, got %s (%s)", expectedType, typeObjType, p.Name.Value)
			}
			return subst, nil
		}

		if sym.Kind != symbols.ConstructorSymbol {
			return nil, inferErrorf(p, "undefined constructor: %s", p.Name.Value)
		}

		freshCtorType := InstantiateWithContext(ctx, sym.Type)
		totalSubst := typesystem.Subst{}

		if tFunc, ok := freshCtorType.(typesystem.TFunc); ok {
			subst, err := typesystem.Unify(expectedType, tFunc.ReturnType)
			if err != nil {
				return nil, inferErrorf(p, "pattern type mismatch: expected %s, got %s (%s)", expectedType, tFunc.ReturnType, p.Name.Value)
			}
			totalSubst = subst.Compose(totalSubst)

			if len(p.Elements) != len(tFunc.Params) {
				return nil, inferErrorf(p, "constructor %s expects %d arguments, got %d", p.Name.Value, len(tFunc.Params), len(p.Elements))
			}

			for i, el := range p.Elements {
				// Apply current substitution to parameter type
				paramType := tFunc.Params[i].Apply(totalSubst)
				s, err := inferPattern(ctx, el, paramType, table)
				if err != nil {
					return nil, err
				}
				totalSubst = s.Compose(totalSubst)
			}
			return totalSubst, nil
		} else {
			if len(p.Elements) > 0 {
				return nil, inferErrorf(p, "constructor %s takes no arguments, got %d", p.Name.Value, len(p.Elements))
			}

			subst, err := typesystem.Unify(expectedType, freshCtorType)
			if err != nil {
				return nil, inferErrorf(p, "pattern type mismatch: expected %s, got %s (%s)", expectedType, freshCtorType, p.Name.Value)
			}
			return subst, nil
		}

	case *ast.ListPattern:
		var elemType typesystem.Type = ctx.FreshVar()
		var listType typesystem.Type = typesystem.TApp{
			Constructor: typesystem.TCon{Name: config.ListTypeName},
			Args:        []typesystem.Type{elemType},
		}

		subst, err := typesystem.Unify(expectedType, listType)
		if err != nil {
			return nil, inferErrorf(p, "list pattern expects List, got %s", expectedType)
		}
		totalSubst := subst

		elemType = elemType.Apply(totalSubst)
		listType = listType.Apply(totalSubst)

		fixedCount := len(p.Elements)
		hasSpread := false
		if fixedCount > 0 {
			if _, ok := p.Elements[fixedCount-1].(*ast.SpreadPattern); ok {
				hasSpread = true
				fixedCount--
			}
		}

		for i := 0; i < fixedCount; i++ {
			s, err := inferPattern(ctx, p.Elements[i], elemType.Apply(totalSubst), table)
			if err != nil {
				return nil, err
			}
			totalSubst = s.Compose(totalSubst)
		}

		if hasSpread {
			spreadPat := p.Elements[fixedCount].(*ast.SpreadPattern)
			if spreadPat.Pattern != nil {
				s, err := inferPattern(ctx, spreadPat.Pattern, listType.Apply(totalSubst), table)
				if err != nil {
					return nil, err
				}
				totalSubst = s.Compose(totalSubst)
			}
		}

		return totalSubst, nil

	case *ast.RecordPattern:
		totalSubst := typesystem.Subst{}

		// Sort keys for deterministic type variable naming
		patternKeys := make([]string, 0, len(p.Fields))
		for k := range p.Fields {
			patternKeys = append(patternKeys, k)
		}
		sort.Strings(patternKeys)

		// Unwrap expected type if it's a TCon (alias or nominal record) to check fields
		checkType := typesystem.UnwrapUnderlying(expectedType)
		// Resolve alias if not unwrapped
		if _, ok := checkType.(typesystem.TRecord); !ok {
			checkType = table.ResolveTypeAlias(expectedType)
		}

		if tRec, ok := checkType.(typesystem.TRecord); ok {
			for _, key := range patternKeys {
				pat := p.Fields[key]
				if fieldType, ok := tRec.Fields[key]; ok {
					s, err := inferPattern(ctx, pat, fieldType.Apply(totalSubst), table)
					if err != nil {
						return nil, err
					}
					totalSubst = s.Compose(totalSubst)
				} else {
					if tRec.IsOpen {
						ft := ctx.FreshVar()
						s, err := inferPattern(ctx, pat, ft, table)
						if err != nil {
							return nil, err
						}
						totalSubst = s.Compose(totalSubst)
					} else {
						return nil, inferErrorf(p, "record pattern field '%s' not found in type %s", key, tRec)
					}
				}
			}
			return totalSubst, nil
		} else if _, ok := expectedType.(typesystem.TVar); ok {
			fields := make(map[string]typesystem.Type)
			for _, key := range patternKeys {
				pat := p.Fields[key]
				ft := ctx.FreshVar()
				s, err := inferPattern(ctx, pat, ft, table)
				if err != nil {
					return nil, err
				}
				totalSubst = s.Compose(totalSubst)
				fields[key] = ft.Apply(totalSubst) // Apply accumulated subst
			}
			recType := typesystem.TRecord{Fields: fields, IsOpen: true}

			subst, err := typesystem.Unify(expectedType.Apply(totalSubst), recType)
			if err != nil {
				return nil, err
			}
			totalSubst = subst.Compose(totalSubst)
			return totalSubst, nil
		}

		return nil, inferErrorf(p, "record pattern expects Record, got %s", expectedType)

	case *ast.TuplePattern:
		totalSubst := typesystem.Subst{}

		isList := false
		var listElemType typesystem.Type

		checkType := expectedType.Apply(totalSubst)

		// Resolve TCon with UnderlyingType (type aliases)
		checkType = typesystem.UnwrapUnderlying(checkType)

		if tApp, ok := checkType.(typesystem.TApp); ok {
			if tCon, ok := tApp.Constructor.(typesystem.TCon); ok && tCon.Name == config.ListTypeName {
				isList = true
				if len(tApp.Args) > 0 {
					listElemType = tApp.Args[0]
				} else {
					listElemType = ctx.FreshVar()
				}
			} else if tCon, ok := tApp.Constructor.(*typesystem.TCon); ok && tCon.Name == config.ListTypeName {
				isList = true
				if len(tApp.Args) > 0 {
					listElemType = tApp.Args[0]
				} else {
					listElemType = ctx.FreshVar()
				}
			}
		}

		if isList {
			fixedCount := len(p.Elements)
			hasSpread := false
			if fixedCount > 0 {
				if _, ok := p.Elements[fixedCount-1].(*ast.SpreadPattern); ok {
					hasSpread = true
					fixedCount--
				}
			}

			for i := 0; i < fixedCount; i++ {
				s, err := inferPattern(ctx, p.Elements[i], listElemType.Apply(totalSubst), table)
				if err != nil {
					return nil, err
				}
				totalSubst = s.Compose(totalSubst)
			}

			if hasSpread {
				spreadPat := p.Elements[fixedCount].(*ast.SpreadPattern)
				if spreadPat.Pattern != nil {
					s, err := inferPattern(ctx, spreadPat.Pattern, checkType.Apply(totalSubst), table)
					if err != nil {
						return nil, err
					}
					totalSubst = s.Compose(totalSubst)
				}
			}
			return totalSubst, nil
		}

		if tTuple, ok := checkType.(typesystem.TTuple); ok {
			hasSpread := false
			if len(p.Elements) > 0 {
				if _, ok := p.Elements[len(p.Elements)-1].(*ast.SpreadPattern); ok {
					hasSpread = true
				}
			}

			if hasSpread {
				fixedCount := len(p.Elements) - 1
				if len(tTuple.Elements) < fixedCount {
					return nil, inferErrorf(p, "tuple pattern length mismatch (variadic): expected at least %d, got %d", fixedCount, len(tTuple.Elements))
				}

				for i := 0; i < fixedCount; i++ {
					s, err := inferPattern(ctx, p.Elements[i], tTuple.Elements[i].Apply(totalSubst), table)
					if err != nil {
						return nil, err
					}
					totalSubst = s.Compose(totalSubst)
				}

				spreadPat := p.Elements[fixedCount].(*ast.SpreadPattern)
				restElements := tTuple.Elements[fixedCount:]
				// We need to apply current subst to rest elements too
				var finalRest []typesystem.Type
				for _, r := range restElements {
					finalRest = append(finalRest, r.Apply(totalSubst))
				}
				restType := typesystem.TTuple{Elements: finalRest}

				if spreadPat.Pattern != nil {
					s, err := inferPattern(ctx, spreadPat.Pattern, restType, table)
					if err != nil {
						return nil, err
					}
					totalSubst = s.Compose(totalSubst)
				}
				return totalSubst, nil
			}

			if len(tTuple.Elements) != len(p.Elements) {
				return nil, inferErrorf(p, "tuple pattern length mismatch: expected %d, got %d", len(tTuple.Elements), len(p.Elements))
			}
			for i, el := range p.Elements {
				s, err := inferPattern(ctx, el, tTuple.Elements[i].Apply(totalSubst), table)
				if err != nil {
					return nil, err
				}
				totalSubst = s.Compose(totalSubst)
			}
			return totalSubst, nil
		} else {
			if _, ok := checkType.(typesystem.TVar); ok {
				hasSpread := false
				if len(p.Elements) > 0 {
					if _, ok := p.Elements[len(p.Elements)-1].(*ast.SpreadPattern); ok {
						hasSpread = true
					}
				}

				var elemTypes []typesystem.Type

				if hasSpread {
					fixedCount := len(p.Elements) - 1
					for i := 0; i < fixedCount; i++ {
						elemTypes = append(elemTypes, ctx.FreshVar())
					}

					for i := 0; i < fixedCount; i++ {
						s, err := inferPattern(ctx, p.Elements[i], ctx.FreshVar(), table)
						if err != nil {
							return nil, err
						}
						totalSubst = s.Compose(totalSubst)
					}

					spreadPat := p.Elements[fixedCount].(*ast.SpreadPattern)
					if spreadPat.Pattern != nil {
						s, err := inferPattern(ctx, spreadPat.Pattern, ctx.FreshVar(), table)
						if err != nil {
							return nil, err
						}
						totalSubst = s.Compose(totalSubst)
					}

					return totalSubst, nil

				} else {
					for range p.Elements {
						elemTypes = append(elemTypes, ctx.FreshVar())
					}
					tupleType := typesystem.TTuple{Elements: elemTypes}

					subst, err := typesystem.Unify(checkType, tupleType)
					if err != nil {
						return nil, err
					}
					totalSubst = subst.Compose(totalSubst)

					for i, el := range p.Elements {
						// Use the now unified TVar from elemTypes
						s, err := inferPattern(ctx, el, elemTypes[i].Apply(totalSubst), table)
						if err != nil {
							return nil, err
						}
						totalSubst = s.Compose(totalSubst)
					}
					return totalSubst, nil
				}
			}

			return nil, inferErrorf(p, "expected tuple type, got %s", checkType)
		}
	}
	return nil, inferErrorf(pat, "unknown pattern type")
}

func isTType(t typesystem.Type) bool {
	_, ok := t.(typesystem.TType)
	return ok
}

func (w *walker) VisitWildcardPattern(n *ast.WildcardPattern) {}

func (w *walker) VisitLiteralPattern(n *ast.LiteralPattern) {}

func (w *walker) VisitIdentifierPattern(n *ast.IdentifierPattern) {
	w.symbolTable.Define(n.Value, nil, "")
}

func (w *walker) VisitConstructorPattern(n *ast.ConstructorPattern) {
	if !w.symbolTable.IsDefined(n.Name.Value) {
		w.addError(diagnostics.NewError(
			diagnostics.ErrA001,
			n.GetToken(),
			n.Name.Value,
		))
	}

	for _, el := range n.Elements {
		el.Accept(w)
	}
}

func (w *walker) VisitListPattern(n *ast.ListPattern) {
	for _, el := range n.Elements {
		el.Accept(w)
	}
}

func (w *walker) VisitTuplePattern(n *ast.TuplePattern) {
	for _, el := range n.Elements {
		el.Accept(w)
	}
}

func (w *walker) VisitRecordPattern(n *ast.RecordPattern) {
	for _, el := range n.Fields {
		el.Accept(w)
	}
}

func (w *walker) VisitSpreadPattern(n *ast.SpreadPattern) {
	if n.Pattern != nil {
		n.Pattern.Accept(w)
	}
}

func (w *walker) VisitTypePattern(n *ast.TypePattern) {
	// Define the variable with the pattern type
	if n.Name != "_" {
		patternType := BuildType(n.Type, w.symbolTable, &w.errors)
		w.symbolTable.Define(n.Name, patternType, "")
	}
}

func (w *walker) VisitStringPattern(n *ast.StringPattern) {
	// Define captured variables as String type
	for _, part := range n.Parts {
		if part.IsCapture && part.Value != "_" {
			// String type is List<Char>
			stringType := typesystem.TApp{
				Constructor: typesystem.TCon{Name: "List"},
				Args:        []typesystem.Type{typesystem.Char},
			}
			w.symbolTable.Define(part.Value, stringType, "")
		}
	}
}

func (w *walker) VisitPinPattern(n *ast.PinPattern) {
	// Pin pattern requires the variable to already exist
	if !w.symbolTable.IsDefined(n.Name) {
		w.addError(diagnostics.NewError(
			diagnostics.ErrA006,
			n.GetToken(),
			n.Name,
		))
	}
}
