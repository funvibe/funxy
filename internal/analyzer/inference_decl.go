package analyzer

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

func inferAnnotatedExpression(ctx *InferenceContext, n *ast.AnnotatedExpression, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	// Infer type of inner expression
	exprType, s1, err := inferFn(n.Expression, table)
	if err != nil {
		return nil, nil, err
	}
	totalSubst := s1
	exprType = exprType.Apply(totalSubst)

	if n.TypeAnnotation == nil {
		return nil, nil, inferError(n, "missing type annotation")
	}

	// Convert AST type to Internal type
	var errs []*diagnostics.DiagnosticError
	annotatedType := BuildType(n.TypeAnnotation, table, &errs)
	if err := wrapBuildTypeError(errs); err != nil {
		return nil, nil, err
	}

	// Unify them (Check if exprType is a subtype of annotatedType)
	// Swap args: Expected, Actual
	subst, err := typesystem.UnifyAllowExtraWithResolver(annotatedType, exprType, table)
	if err != nil {
		return nil, nil, typeMismatch(n, annotatedType.String(), exprType.String())
	}
	totalSubst = subst.Compose(totalSubst)

	return exprType.Apply(totalSubst), totalSubst, nil
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
		// Instantiate generic types to avoid collisions and support polymorphism
		instType := InstantiateWithContext(ctx, sym.Type)
		if instType != nil {
			if ctx.TypeMap != nil {
				// Only store if not already present - avoid overwriting resolved types
				if _, exists := ctx.TypeMap[n]; !exists {
					ctx.TypeMap[n] = instType
				}
			}
		}
		return instType, typesystem.Subst{}, nil
	}
}

func inferAssignExpression(ctx *InferenceContext, n *ast.AssignExpression, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error), typeMap map[ast.Node]typesystem.Type) (typesystem.Type, typesystem.Subst, error) {
	valType, s1, err := inferFn(n.Value, table)
	if err != nil {
		return nil, nil, err
	}
	totalSubst := s1
	valType = valType.Apply(totalSubst)

	// Track the declared type (annotation or inferred)
	declaredType := valType

	if n.AnnotatedType != nil {
		var errs []*diagnostics.DiagnosticError
		explicitType := BuildType(n.AnnotatedType, table, &errs)
		if err := wrapBuildTypeError(errs); err != nil {
			return nil, nil, err
		}

		// Unify: TCon with UnderlyingType will automatically unify with TRecord
		// Ensure types are fully resolved (handling stale references in recursive types)
		// Use UnifyAllowExtraWithResolver to handle stale TCons in recursive types
		subst, err := typesystem.UnifyAllowExtraWithResolver(explicitType, valType, table)
		if err != nil {
			name := "?"
			if id, ok := n.Left.(*ast.Identifier); ok {
				name = id.Value
			}
			return nil, nil, inferErrorf(n, "type mismatch in assignment to %s: expected %s, got %s", name, explicitType, valType)
		}
		totalSubst = subst.Compose(totalSubst)
		valType = valType.Apply(totalSubst)

		// Use explicit type (nominal TCon) for variable declaration
		// This preserves TCon{Name, Module} for extension method lookup
		declaredType = explicitType.Apply(totalSubst)

		if typeMap != nil && n.Value != nil {
			typeMap[n.Value] = valType
		}
	}

	// Handle assignment target
	if ident, ok := n.Left.(*ast.Identifier); ok {
		// Check if variable exists in scope chain (Update)
		if sym, ok := table.Find(ident.Value); ok {
			// If it's a Pending symbol (forward declared in Naming pass),
			// treat this as the initial definition and overwrite it.
			if sym.IsPending {
				table.Define(ident.Value, declaredType, ctx.CurrentModuleName)
				return valType, totalSubst, nil
			}

			// Create token for error reporting at the start of the assignment statement
			// Use the identifier token but adjust column to point to start of identifier
			errorTok := ident.GetToken()
			// The identifier token column points past the identifier
			// We need to adjust it to point to the start
			// Calculate: current column minus length of identifier
			if len(ident.Value) > 0 && errorTok.Column > len(ident.Value) {
				errorTok.Column = errorTok.Column - len(ident.Value)
			}

			// Check if it's a constant - cannot reassign constants
			if sym.IsConstant {
				return nil, nil, diagnostics.NewError(diagnostics.ErrA003, errorTok, "cannot reassign constant '"+ident.Value+"'")
			}

			// Cannot assign to types, traits, modules, or constructors
			switch sym.Kind {
			case symbols.TypeSymbol:
				return nil, nil, diagnostics.NewError(diagnostics.ErrA003, errorTok, "cannot assign to type '"+ident.Value+"'")
			case symbols.TraitSymbol:
				return nil, nil, diagnostics.NewError(diagnostics.ErrA003, errorTok, "cannot assign to trait '"+ident.Value+"'")
			case symbols.ModuleSymbol:
				return nil, nil, diagnostics.NewError(diagnostics.ErrA003, errorTok, "cannot assign to module '"+ident.Value+"'")
			case symbols.ConstructorSymbol:
				return nil, nil, diagnostics.NewError(diagnostics.ErrA003, errorTok, "cannot assign to constructor '"+ident.Value+"'")
			}

			// Cannot reassign imported symbols
			if ctx != nil && sym.OriginModule != "" && sym.OriginModule != ctx.CurrentModuleName {
				return nil, nil, diagnostics.NewError(diagnostics.ErrA003, errorTok, "cannot reassign imported symbol '"+ident.Value+"' from module '"+sym.OriginModule+"'")
			}

			// It exists. Unify types.
			// Note: If sym.Type is nil?
			if sym.Type != nil {
				// Allow subtype assignment
				subst, err := typesystem.UnifyAllowExtraWithResolver(sym.Type, valType, table)
				if err != nil {
					return nil, nil, inferErrorf(n, "cannot assign %s to variable %s of type %s", valType, ident.Value, sym.Type)
				}
				totalSubst = subst.Compose(totalSubst)
				return valType.Apply(totalSubst), totalSubst, nil
			}
		} else {
			// Define variable in the current scope if not found
			// Use the declared type (annotation type if present, else inferred type)
			table.Define(ident.Value, declaredType, "")
			return valType, totalSubst, nil
		}
	} else if ma, ok := n.Left.(*ast.MemberExpression); ok {
		// Assignment to member: obj.field = val
		// Infer obj type
		objType, s2, err := inferFn(ma.Left, table)
		if err != nil {
			return nil, nil, err
		}
		totalSubst = s2.Compose(totalSubst)
		objType = objType.Apply(totalSubst)

		// Resolve named record types (e.g., Counter -> { value: Int })
		resolvedType := typesystem.UnwrapUnderlying(objType)

		if tRec, ok := resolvedType.(typesystem.TRecord); ok {
			if fieldType, ok := tRec.Fields[ma.Member.Value]; ok {
				// Unify field type with value type
				// Allow subtype assignment to field
				subst, err := typesystem.UnifyAllowExtraWithResolver(fieldType, valType, table)
				if err != nil {
					return nil, nil, inferErrorf(n, "type mismatch in assignment to field %s: expected %s, got %s", ma.Member.Value, fieldType, valType)
				}
				totalSubst = subst.Compose(totalSubst)
				return valType.Apply(totalSubst), totalSubst, nil
			} else {
				return nil, nil, inferErrorf(n, "record %s has no field '%s'", tRec, ma.Member.Value)
			}
		} else {
			return nil, nil, inferErrorf(n, "assignment to member expects Record, got %s", objType)
		}
	}
	return nil, nil, inferError(n, "invalid assignment target")
}

func inferFunctionLiteral(ctx *InferenceContext, n *ast.FunctionLiteral, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	// 1. Create scope
	enclosedTable := symbols.NewEnclosedSymbolTable(table)
	totalSubst := typesystem.Subst{}

	// 2. Define params
	var paramTypes []typesystem.Type
	isVariadic := false
	defaultCount := 0
	for _, p := range n.Parameters {
		var pt typesystem.Type
		if p.Type != nil {
			var errs []*diagnostics.DiagnosticError
			pt = BuildType(p.Type, enclosedTable, &errs)
			if err := wrapBuildTypeError(errs); err != nil {
				return nil, nil, err
			}
		} else {
			pt = ctx.FreshVar()
		}

		// Store element type in signature
		paramTypes = append(paramTypes, pt)

		// Count defaults
		if p.Default != nil {
			defaultCount++
		}

		// For variadic, the local variable is wrapped in List
		localType := pt
		if p.IsVariadic {
			isVariadic = true
			localType = typesystem.TApp{
				Constructor: typesystem.TCon{Name: config.ListTypeName},
				Args:        []typesystem.Type{pt},
			}
		}

		enclosedTable.Define(p.Name.Value, localType, "")
	}

	// 3. Infer body
	bodyType, sBody, err := inferFn(n.Body, enclosedTable)
	if err != nil {
		return nil, nil, err
	}
	totalSubst = sBody.Compose(totalSubst)
	bodyType = bodyType.Apply(totalSubst)

	// Update params with subst derived from body
	for i := range paramTypes {
		paramTypes[i] = paramTypes[i].Apply(totalSubst)
	}

	// 4. Check explicit return type if present
	if n.ReturnType != nil {
		var errs []*diagnostics.DiagnosticError
		retType := BuildType(n.ReturnType, enclosedTable, &errs)
		if err := wrapBuildTypeError(errs); err != nil {
			return nil, nil, err
		}
		// Allow returning a subtype (e.g. record with more fields)
		subst, err := typesystem.UnifyAllowExtraWithResolver(retType, bodyType, table)
		if err != nil {
			return nil, nil, inferErrorf(n, "lambda return type mismatch: expected %s, got %s", retType, bodyType)
		}
		totalSubst = subst.Compose(totalSubst)

		bodyType = bodyType.Apply(totalSubst)
		for i := range paramTypes {
			paramTypes[i] = paramTypes[i].Apply(totalSubst)
		}
	}

	return typesystem.TFunc{
		Params:       paramTypes,
		ReturnType:   bodyType,
		IsVariadic:   isVariadic,
		DefaultCount: defaultCount,
	}, totalSubst, nil
}

func inferSpreadExpression(ctx *InferenceContext, n *ast.SpreadExpression, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	// SpreadExpression unwraps a tuple.
	return inferFn(n.Expression, table)
}

func inferPatternAssignExpression(ctx *InferenceContext, n *ast.PatternAssignExpression, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	// Infer the value type
	valType, subst, err := inferFn(n.Value, table)
	if err != nil {
		return nil, nil, err
	}

	// Bind pattern variables to the symbol table
	err = bindPatternToType(n.Pattern, valType, table, ctx)
	if err != nil {
		return nil, nil, err
	}

	// Pattern assignment expressions return unit/nil type
	return typesystem.TCon{Name: "Unit"}, subst, nil
}

// bindPatternToType binds pattern variables to types in the symbol table
func bindPatternToType(pat ast.Pattern, valType typesystem.Type, table *symbols.SymbolTable, ctx *InferenceContext) error {
	switch p := pat.(type) {
	case *ast.IdentifierPattern:
		table.Define(p.Value, valType, "")
		return nil

	case *ast.TuplePattern:
		tuple, ok := valType.(typesystem.TTuple)
		if !ok {
			return inferErrorf(pat, "cannot destructure non-tuple value with tuple pattern")
		}
		if len(tuple.Elements) != len(p.Elements) {
			return inferErrorf(pat, "tuple pattern has %d elements but value has %d", len(p.Elements), len(tuple.Elements))
		}
		for i, elem := range p.Elements {
			if err := bindPatternToType(elem, tuple.Elements[i], table, ctx); err != nil {
				return err
			}
		}
		return nil

	case *ast.ListPattern:
		// Extract element type from List<T>
		if app, ok := valType.(typesystem.TApp); ok {
			if con, ok := app.Constructor.(typesystem.TCon); ok && con.Name == "List" && len(app.Args) > 0 {
				elemType := app.Args[0]
				for _, elem := range p.Elements {
					if err := bindPatternToType(elem, elemType, table, ctx); err != nil {
						return err
					}
				}
				return nil
			}
		}
		return inferErrorf(pat, "cannot destructure non-list value with list pattern")

	case *ast.WildcardPattern:
		// Ignore - don't bind anything
		return nil

	case *ast.RecordPattern:
		// Handle both TRecord and named record types
		var fields map[string]typesystem.Type

		switch t := valType.(type) {
		case typesystem.TRecord:
			fields = t.Fields
		default:
			// Try to get underlying type if it's a named record type
			if underlying := typesystem.UnwrapUnderlying(valType); underlying != nil {
				if rec, ok := underlying.(typesystem.TRecord); ok {
					fields = rec.Fields
				}
			}
		}

		if fields == nil {
			return inferErrorf(pat, "cannot destructure non-record value with record pattern")
		}

		for fieldName, fieldPat := range p.Fields {
			fieldType, ok := fields[fieldName]
			if !ok {
				return inferErrorf(pat, "record does not have field '%s'", fieldName)
			}
			if err := bindPatternToType(fieldPat, fieldType, table, ctx); err != nil {
				return err
			}
		}
		return nil

	default:
		return inferErrorf(pat, "unsupported pattern in destructuring")
	}
}
