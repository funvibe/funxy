package analyzer

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
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

	// Witness Resolution for Call Expressions: If inner expression is a CallExpression (e.g., pure(10)),
	// set witness based on annotated type to enable runtime dispatch
	if callExpr, ok := n.Expression.(*ast.CallExpression); ok {
		// Extract trait constraints from annotated type
		// For Writer<MySum, Int>, we need to set Applicative witness to Writer<MySum, Int>
		// For OptionT<Identity, Int>, we need Applicative witness to OptionT<Identity, Int>
		// Check if this type implements Applicative (for pure) or other traits
		// Unwrap type aliases before checking to handle cases like Writer<IntList, Int>
		// where IntList = List<Int> is a type alias
		shouldSetWitness := false

		// Check implementation by walking up TApp chain (e.g. OptionT<Id, Int> -> OptionT<Id>)
		checkType := finalType

		// Kind-directed Smart Witness Resolution
		// Expected kind for Applicative is * -> *
		expectedKind := typesystem.TCon{Name: "Applicative"}.Kind()

		for {
			// Check kind match
			currentKind, err := typesystem.KindCheck(checkType)
			if err == nil && currentKind.Equal(expectedKind) {
				if table.IsImplementationExists("Applicative", []typesystem.Type{checkType}) {
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
				// Use original finalType (before unwrapping) to preserve type aliases
				witnesses["Applicative"] = []typesystem.Type{finalType}
			}
		}
	}

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

// isNonExpansive checks if the node is syntactically a value (safe to generalize)
// This implements the Value Restriction to prevent unsound generalization of expansive expressions
func isNonExpansive(n ast.Node) bool {
	switch n.(type) {
	case *ast.FunctionLiteral, *ast.IntegerLiteral, *ast.FloatLiteral, *ast.StringLiteral, *ast.BooleanLiteral, *ast.CharLiteral:
		return true
	case *ast.Identifier, *ast.OperatorAsFunction:
		return true
	// ListLiteral and RecordLiteral are safe only if their elements are safe,
	// but for simplicity we can treat them as safe if they don't contain calls?
	// Conservatively, only functions and primitives are fully safe.
	default:
		return false
	}
}

// inferAssignExpression checks assignment compatibility and handles symbol definition/update.
// It also ensures that mutations respect scope rules (preventing mutation of global variables from function scopes).
func inferAssignExpression(ctx *InferenceContext, n *ast.AssignExpression, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error), typeMap map[ast.Node]typesystem.Type) (typesystem.Type, typesystem.Subst, error) {
	// Track the declared type (annotation, expected type, or inferred)
	var declaredType typesystem.Type
	var expectedType typesystem.Type
	var valType typesystem.Type
	var totalSubst typesystem.Subst

	// Check for expected type from look-ahead pass
	// Only use expected type if variable is not yet defined (new assignment)
	// But if the variable is defined but has type variables (e.g. from an earlier weak inference),
	// we might want to allow refinement.

	if n.AnnotatedType != nil {
		// Use annotation as expected type for context propagation
		var errs []*diagnostics.DiagnosticError
		explicitType := BuildType(n.AnnotatedType, table, &errs)
		if err := wrapBuildTypeError(errs); err != nil {
			return nil, nil, err
		}

		// Kind Check: Annotation must be a proper type (Kind *)
		if k, err := typesystem.KindCheck(explicitType); err != nil {
			return nil, nil, err
		} else if !k.Equal(typesystem.Star) {
			return nil, nil, inferError(n, "type annotation must be type (kind *), got kind "+k.String())
		}

		expectedType = explicitType
	} else if ident, ok := n.Left.(*ast.Identifier); ok {
		// Check if variable is already defined
		sym, found := table.Find(ident.Value)

		// If variable is defined as a TVar (weak inference), treat as new assignment for refinement purposes
		isWeak := false
		if found && !sym.IsPending && sym.Type != nil {
			if _, isTVar := sym.Type.(typesystem.TVar); isTVar {
				isWeak = true
			}
		}

		if !found || isWeak {
			// Variable not yet defined or weakly defined - can use expected type
			if expType, hasExpected := ctx.ExpectedTypes[ident]; hasExpected {
				// Check if expected type is a bare type variable (TVar)
				if _, isTVar := expType.(typesystem.TVar); !isTVar {
					expectedType = expType
				}
			}
		} else if sym.IsPending {
			// Variable is pending (forward declared) - can use expected type
			if expType, hasExpected := ctx.ExpectedTypes[ident]; hasExpected {
				expectedType = expType
			}
		}
	}

	// Setup expectation if applicable
	if expectedType != nil {
		if ctx.ExpectedReturnTypes == nil {
			ctx.ExpectedReturnTypes = make(map[ast.Node]typesystem.Type)
		}
		// Push expected type to the value expression so recursive inference can use it
		// This is CRITICAL for InfixExpressions (like >>=) and CallExpressions (like pure)
		// that rely on ExpectedReturnTypes for context-sensitive inference.
		ctx.ExpectedReturnTypes[n.Value] = expectedType
	}

	// Infer value type (ONCE)
	var err error
	valType, totalSubst, err = inferFn(n.Value, table)
	if err != nil {
		return nil, nil, err
	}
	valType = valType.Apply(ctx.GlobalSubst).Apply(totalSubst)

	// If expectation exists, try to unify (refine)
	if expectedType != nil && n.AnnotatedType == nil {
		// Unify expected type with inferred type
		// This will resolve type variables in valType based on expectedType
		subst, err := typesystem.UnifyAllowExtraWithResolver(expectedType, valType, table)
		if err == nil {
			// Successfully unified - use expected type as declared type
			totalSubst = subst.Compose(totalSubst)
			valType = valType.Apply(ctx.GlobalSubst).Apply(totalSubst)
			declaredType = expectedType.Apply(ctx.GlobalSubst).Apply(totalSubst)

			// Update typeMap with resolved type for the value expression
			if typeMap != nil && n.Value != nil {
				typeMap[n.Value] = valType
			}
		} else {
			// Unification failed.
			// Proceed with inferred valType, ignoring expectation conflict here.
			// This allows later assignment checks to validate strict correctness,
			// or allows cases where expectation was too specific/incorrect.
			declaredType = valType
		}
	}

	if declaredType == nil {
		declaredType = valType
	}

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
			// Auto-call logic for nullary functions (e.g. mempty)
			if tFunc, ok := valType.(typesystem.TFunc); ok && len(tFunc.Params) == 0 {
				substCall, errCall := typesystem.UnifyAllowExtraWithResolver(explicitType, tFunc.ReturnType, table)
				if errCall == nil {
					// Rewrite AST to CallExpression
					n.Value = &ast.CallExpression{
						Function:  n.Value,
						Arguments: []ast.Expression{},
					}
					// Update types
					subst = substCall
					valType = tFunc.ReturnType
					err = nil
				}
			}
		}

		if err != nil {
			name := "?"
			if id, ok := n.Left.(*ast.Identifier); ok {
				name = id.Value
			}
			return nil, nil, inferErrorf(n, "type mismatch in assignment to %s: expected %s, got %s", name, explicitType, valType)
		}
		totalSubst = subst.Compose(totalSubst)
		valType = valType.Apply(ctx.GlobalSubst).Apply(totalSubst)

		// Use explicit type (nominal TCon) for variable declaration
		// This preserves TCon{Name, Module} for extension method lookup
		declaredType = explicitType.Apply(ctx.GlobalSubst).Apply(totalSubst)

		if typeMap != nil && n.Value != nil {
			typeMap[n.Value] = valType
		}

		// Witness Resolution for Call Expressions: If value is a CallExpression (e.g., pure(10)),
		// set witness based on annotated type to enable runtime dispatch
		if callExpr, ok := n.Value.(*ast.CallExpression); ok {
			// Extract trait constraints from annotated type
			// Check if this type implements Applicative (for pure) or other traits
			// Unwrap type aliases before checking to handle cases like Writer<IntList, Int>
			// where IntList = List<Int> is a type alias
			shouldSetWitness := false

			// First try with original type
			// Check implementation by walking up TApp chain
			checkType := declaredType

			// Kind-directed Smart Witness Resolution
			expectedKind := typesystem.TCon{Name: "Applicative"}.Kind()

			for {
				// Check kind match
				currentKind, err := typesystem.KindCheck(checkType)
				if err == nil && currentKind.Equal(expectedKind) {
					if table.IsImplementationExists("Applicative", []typesystem.Type{checkType}) {
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
					// Use original declaredType (before unwrapping) to preserve type aliases
					witnesses["Applicative"] = []typesystem.Type{declaredType}
				}
			}
		}
	}

	// Handle assignment target
	if ident, ok := n.Left.(*ast.Identifier); ok {
		// If identifier is "_", do not define or check symbol.
		// Just evaluate right side and return.
		// This treats "_" as a discard / wildcard assignment.
		if ident.Value == "_" {
			// If n.AnnotatedType is present, verify against it?
			// The logic above already inferred valType and checked annotation.
			// valType already has substitutions applied.
			// We just return Nil (Unit) to signify discard.
			// And technically we should not define "_" in the table.
			return typesystem.Nil, totalSubst, nil
		}

		// Check if variable exists in scope chain (Update)
		if sym, defScope, ok := table.FindWithScope(ident.Value); ok {
			// If it's a Pending symbol (forward declared in Naming pass),
			// treat this as the initial definition and overwrite it.
			if sym.IsPending {
				// No Local Let Generalization: Only generalize if at global scope
				// UPDATE: Enable local let generalization for non-expansive values (Rank-N support)
				if n.AnnotatedType == nil && isNonExpansive(n.Value) {
					declaredType = ctx.Generalize(declaredType, table, ident.Value)
				}
				table.Define(ident.Value, declaredType, ctx.CurrentModuleName)
				return valType, totalSubst, nil
			}

			// Create token for error reporting
			errorTok := ident.GetToken()
			if len(ident.Value) > 0 && errorTok.Column > len(ident.Value) {
				errorTok.Column = errorTok.Column - len(ident.Value)
			}

			// Check if it's a constant
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

			// Check for Module-Scope mutation from Function Scope
			// If we are in a function (or nested in one), we should NOT be able to mutate
			// variables defined in the Global/Module scope.
			//
			// Logic:
			// - Is 'defScope' the Global Scope?
			// - Is 'table' (current scope) a Function or Block scope nested within a Function?
			//
			// We iterate up from 'table' to check if we cross a Function boundary before hitting 'defScope'.
			if defScope.IsGlobalScope() {
				// Check if we are currently inside a function
				isInsideFunction := false
				curr := table
				for curr != nil && curr != defScope {
					if curr.IsFunctionScope() {
						isInsideFunction = true
						break
					}
					curr = curr.Parent()
				}

				if isInsideFunction {
					return nil, nil, diagnostics.NewError(diagnostics.ErrA003, errorTok, "cannot mutate global variable '"+ident.Value+"' from within a function")
				}
			}

			// It exists. Unify types.
			if sym.Type != nil {
				subst, err := typesystem.UnifyAllowExtraWithResolver(sym.Type, valType, table)
				if err != nil {
					return nil, nil, inferErrorf(n, "cannot assign %s to variable %s of type %s", valType, ident.Value, sym.Type)
				}
				totalSubst = subst.Compose(totalSubst)
				return valType.Apply(ctx.GlobalSubst).Apply(totalSubst), totalSubst, nil
			}
		} else {
			// Define variable in the current scope if not found
			// Use the declared type (annotation type if present, else inferred type)
			// No Local Let Generalization: Only generalize if at global scope
			// UPDATE: Enable local let generalization for non-expansive values (Rank-N support)
			if n.AnnotatedType == nil && isNonExpansive(n.Value) {
				declaredType = ctx.Generalize(declaredType, table, ident.Value)
			}
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
		objType = objType.Apply(ctx.GlobalSubst).Apply(totalSubst)

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
				return valType.Apply(ctx.GlobalSubst).Apply(totalSubst), totalSubst, nil
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
	// Create scope
	enclosedTable := symbols.NewEnclosedSymbolTable(table, symbols.ScopeFunction)
	totalSubst := typesystem.Subst{}

	// Save/Restore pending witnesses to handle local resolution
	oldPending := ctx.PendingWitnesses
	ctx.PendingWitnesses = make([]PendingWitness, 0)

	// Save/Restore constraints to separate local from outer
	oldConstraints := ctx.Constraints
	ctx.Constraints = make([]Constraint, 0)

	// Check for expected type to guide inference (Contextual Typing)
	var expectedFuncType *typesystem.TFunc
	var expectedForall *typesystem.TForall

	if expectedType, ok := ctx.ExpectedTypes[n]; ok {
		// Resolve alias if any
		resolved := table.ResolveTypeAlias(expectedType)
		if tFunc, ok := resolved.(typesystem.TFunc); ok {
			expectedFuncType = &tFunc
		} else if tForall, ok := resolved.(typesystem.TForall); ok {
			expectedForall = &tForall
			// Skolemize the expected polytype
			subst := make(typesystem.Subst)
			for _, v := range tForall.Vars {
				// Use TCon with special name as Skolem (rigid type constant)
				skolemName := fmt.Sprintf("$skolem_%s_%d", v.Name, ctx.counter)
				ctx.counter++
				skolem := typesystem.TCon{Name: skolemName, KindVal: v.Kind()}
				subst[v.Name] = skolem
			}
			instantiated := tForall.Type.Apply(subst)
			if tFunc, ok := instantiated.(typesystem.TFunc); ok {
				expectedFuncType = &tFunc
			}
		}
	}

	// Define params
	var paramTypes []typesystem.Type
	isVariadic := false
	defaultCount := 0
	for i, p := range n.Parameters {
		var pt typesystem.Type
		if p.Type != nil {
			var errs []*diagnostics.DiagnosticError
			pt = BuildType(p.Type, enclosedTable, &errs)
			if err := wrapBuildTypeError(errs); err != nil {
				return nil, nil, err
			}
			pt = OpenRecords(pt, ctx.FreshVar)
		} else {
			// Try to use expected type for parameter
			if expectedFuncType != nil && i < len(expectedFuncType.Params) {
				// Use the expected parameter type
				// We need to instantiate it if it's generic?
				// Usually expected type is already instantiated in the call site.
				pt = expectedFuncType.Params[i]
			} else {
				pt = ctx.FreshVar()
			}
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

	// Propagate expected return type to the body (for context-sensitive inference in the last statement)
	if expectedFuncType != nil {
		if ctx.ExpectedReturnTypes == nil {
			ctx.ExpectedReturnTypes = make(map[ast.Node]typesystem.Type)
		}
		ctx.ExpectedReturnTypes[n.Body] = expectedFuncType.ReturnType
	}

	// Infer body
	bodyType, sBody, err := inferFn(n.Body, enclosedTable)
	if err != nil {
		return nil, nil, err
	}
	totalSubst = sBody.Compose(totalSubst)
	bodyType = bodyType.Apply(ctx.GlobalSubst).Apply(totalSubst)

	// Unify body type with expected return type if available
	// This ensures that even if the body ends with an Identifier (which ignores expectations),
	// we still capture the type information (e.g. t13 -> OptionT)
	if expectedFuncType != nil {
		expectedRet := expectedFuncType.ReturnType.Apply(ctx.GlobalSubst).Apply(totalSubst)
		subst, err := typesystem.UnifyAllowExtraWithResolver(expectedRet, bodyType, table)
		if err == nil {
			totalSubst = subst.Compose(totalSubst)
			// Explicitly update GlobalSubst to ensure deferred constraints can see the resolution
			ctx.GlobalSubst = subst.Compose(ctx.GlobalSubst)
			bodyType = bodyType.Apply(ctx.GlobalSubst).Apply(totalSubst)
		}
	}

	// Update params with subst derived from body
	for i := range paramTypes {
		paramTypes[i] = paramTypes[i].Apply(ctx.GlobalSubst).Apply(totalSubst)
	}

	// Check explicit return type if present
	if n.ReturnType != nil {
		var errs []*diagnostics.DiagnosticError
		retType := BuildType(n.ReturnType, enclosedTable, &errs)
		if err := wrapBuildTypeError(errs); err != nil {
			return nil, nil, err
		}
		retType = OpenRecords(retType, ctx.FreshVar)
		// Allow returning a subtype (e.g. record with more fields)
		subst, err := typesystem.UnifyAllowExtraWithResolver(retType, bodyType, table)
		if err != nil {
			return nil, nil, inferErrorf(n, "lambda return type mismatch: expected %s, got %s", retType, bodyType)
		}
		totalSubst = subst.Compose(totalSubst)

		bodyType = bodyType.Apply(ctx.GlobalSubst).Apply(totalSubst)
		for i := range paramTypes {
			paramTypes[i] = paramTypes[i].Apply(ctx.GlobalSubst).Apply(totalSubst)
		}
	}

	// Mark tail calls since walker skips inner functions/expressions
	MarkTailCalls(n.Body)

	// Rank-2/Rank-N Polymorphism: If a parameter is polymorphic (TForall),
	// and the return type depends on calling that parameter, generalize the return type
	// This enables Rank-2: forall T. (forall U. U -> U) -> T
	returnType := bodyType
	for _, paramType := range paramTypes {
		if tForall, ok := paramType.(typesystem.TForall); ok {
			// Parameter is polymorphic - check if return type depends on calling it
			// For Rank-2, if parameter is (forall U. U -> U), and we call it,
			// the return type should be polymorphic: forall T. T
			// We detect this by checking if the parameter is a function type inside forall
			if _, ok := tForall.Type.(typesystem.TFunc); ok {
				// Parameter is polymorphic function - return type should be generalized
				// Check if bodyType is concrete (no free type variables)
				freeVars := bodyType.FreeTypeVariables()
				if len(freeVars) == 0 {
					// Return type is concrete - generalize it for Rank-2
					// Create a fresh type variable for the return type
					retVar := ctx.FreshVar()
					returnType = retVar
					// Note: In full Rank-2 implementation, we'd need to track that retVar
					// unifies with bodyType when the polymorphic parameter is instantiated
					// For now, this enables Rank-2 type checking
					break // Only generalize once for the first polymorphic parameter
				}
			}
		}
	}

	tFunc := typesystem.TFunc{
		Params:       paramTypes,
		ReturnType:   returnType,
		IsVariadic:   isVariadic,
		DefaultCount: defaultCount,
	}

	// Kind Check: Ensure function signature is valid (components are proper types)
	if _, err := typesystem.KindCheck(tFunc); err != nil {
		return nil, nil, inferError(n, err.Error())
	}

	// Generalize / Capture Constraints
	// Identify constraints that depend ONLY on local type variables (params/body) and not outer scope.
	// This makes the lambda generic over these constraints (e.g. fun(m) { m >>= ... } becomes Monad m => m -> m).

	var captured []Constraint
	var remaining []Constraint

	// Apply substitutions to current constraints before checking
	for _, c := range ctx.Constraints {
		// Apply global and local subst
		if c.Kind == ConstraintImplements {
			c.Left = c.Left.Apply(ctx.GlobalSubst).Apply(totalSubst)
			for i, arg := range c.Args {
				c.Args[i] = arg.Apply(ctx.GlobalSubst).Apply(totalSubst)
			}
		}

		if c.Kind != ConstraintImplements {
			remaining = append(remaining, c)
			continue
		}

		cVars := c.Left.FreeTypeVariables()
		for _, arg := range c.Args {
			for _, v := range arg.FreeTypeVariables() {
				cVars = append(cVars, v)
			}
		}

		if len(cVars) == 0 {
			remaining = append(remaining, c)
			continue
		}

		// Fix: Lambdas (FunctionLiteral) do not have explicit type params,
		// so they should NOT capture constraints. Force bubble up.
		remaining = append(remaining, c)
	}

	// Restore outer constraints and append remaining
	ctx.Constraints = append(oldConstraints, remaining...)

	// Update tFunc constraints and FunctionLiteral WitnessParams
	n.WitnessParams = nil
	for _, c := range captured {
		tv, ok := c.Left.(typesystem.TVar)
		if !ok {
			// Skip constraints on non-variables for signature (simplification)
			continue
		}

		// Convert to typesystem.Constraint
		// Note: c.Args in InferenceContext includes the self-type as the first argument.
		// typesystem.Constraint expects only the additional arguments.
		var constraintArgs []typesystem.Type
		if len(c.Args) > 1 {
			constraintArgs = c.Args[1:]
		}

		tc := typesystem.Constraint{
			TypeVar: tv.Name,
			Trait:   c.Trait,
			Args:    constraintArgs,
		}
		tFunc.Constraints = append(tFunc.Constraints, tc)

		// Generate witness param name
		witnessName := GetWitnessParamName(tv.Name, c.Trait)
		// For MPTC, name might need args? GetWitnessParamName only takes Trait and TVar.
		// If we have collisions (same Trait/Var but different args), we need better naming.
		// For now assume standard type classes.

		n.WitnessParams = append(n.WitnessParams, witnessName)

		// Define witness parameter in enclosed scope for local resolution
		// We use Dictionary type (though exact type doesn't matter for SolveWitness which just checks definition)
		enclosedTable.Define(witnessName, typesystem.TCon{Name: "Dictionary"}, "")
	}

	// Resolve pending witnesses generated in the body against captured constraints
	unresolvedWitnesses := resolveLocalWitnesses(ctx, enclosedTable)

	// Append unresolved witnesses back to oldPending (to be handled by outer scope)
	ctx.PendingWitnesses = append(oldPending, unresolvedWitnesses...)

	if expectedForall != nil {
		// If we successfully checked against a polytype, return the polytype
		return *expectedForall, totalSubst, nil
	}

	return tFunc, totalSubst, nil
}

// resolveLocalWitnesses attempts to resolve pending witnesses using local constraints (witness params)
// Returns list of witnesses that could not be resolved locally.
func resolveLocalWitnesses(ctx *InferenceContext, table *symbols.SymbolTable) []PendingWitness {
	var unresolved []PendingWitness

	for _, pw := range ctx.PendingWitnesses {
		// Prepare args for SolveWitness
		args := pw.Args
		// Apply substitutions? They should be mostly applied by now?
		// We should apply GlobalSubst just in case.
		resolvedArgs := make([]typesystem.Type, len(args))
		for i, arg := range args {
			resolvedArgs[i] = arg.Apply(ctx.GlobalSubst)
		}

		// Attempt to solve
		witnessExpr, err := ctx.SolveWitness(pw.Node, pw.Trait, resolvedArgs, table)

		if err == nil {
			// Success! Update the AST node.
			// The node is pw.Node (CallExpression).
			// We need to put witnessExpr into Witnesses slice at the correct index.
			if pw.Index >= 0 && pw.Index < len(pw.Node.Witnesses) {
				pw.Node.Witnesses[pw.Index] = witnessExpr
			} else {
				// Index out of bounds? Should not happen if registered correctly.
				// Fallback: append if index is not valid? No, that breaks alignment.
				// If index is -1 (legacy), maybe append?
				pw.Node.Witnesses = append(pw.Node.Witnesses, witnessExpr)
			}
		} else {
			// Failed to resolve locally.
			// If the witness depends on variables local to the function (that were generalized),
			// then this is a genuine error (missing constraint).
			// If it depends on outer variables, we pass it up.

			// How to check dependency?
			// The variables in resolvedArgs.
			// If any variable is NOT in table.FreeTypeVariables (of the OUTER table), it's local.
			// But here 'table' is the ENCLOSED table.
			// We need to know which variables are "local generic parameters" of this function.
			// These are the ones we just generalized (in 'captured').

			// Simplification: If SolveWitness failed, and we have generalized variables involved,
			// it usually means we are missing a constraint on the generalized variable.
			// But if we just bubble it up, the outer scope won't know about inner generalized vars.
			// So we MUST check if it depends on inner vars.

			// Check if any arg is a Type Variable that matches one of our captured/generalized vars.
			// We don't have the list of generalized vars explicitly here, but they are in 'captured'.
			// Or we can check if the variable is defined in 'table' (as a param) or generated as fresh var.

			// Actually, just append to unresolved for now.
			// If it depends on a local generalized var, the outer scope won't find it either,
			// and ResolvePendingWitnesses (global) will report "unresolved type variable" error.
			// This matches the current error message we see!
			// "GLOBAL RESOLVE: implementation ... for types [t15] not found (unresolved type variable)"
			// So bubbling up works for error reporting.

			unresolved = append(unresolved, pw)
		}
	}

	return unresolved
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
