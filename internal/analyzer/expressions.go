package analyzer

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
	"sort"
)

func (w *walker) VisitAssignExpression(expr *ast.AssignExpression) {
	// First, analyze the value being assigned to ensure it's valid.
	if expr.Value != nil {
		expr.Value.Accept(w)
	}

	// Check if assignment is to a variable (definition or update) or member access (update)

	if ident, ok := expr.Left.(*ast.Identifier); ok {
		// Now, define the variable in the current scope.
		var varType typesystem.Type
		if expr.Value != nil {
			t, s, err := InferWithContext(w.inferCtx, expr.Value, w.symbolTable)
			if err != nil {
				// Don't add error here - it's already added by Accept(w) above
				// Just skip type inference for this variable
			} else {
				varType = t.Apply(s)
				// Apply substitution to all sub-expressions so type variables are resolved
				w.applySubstToNode(expr.Value, s)
			}
		}

		// Handle Annotation
		if expr.AnnotatedType != nil {
			annotType := BuildType(expr.AnnotatedType, w.symbolTable, &w.errors)
			if varType != nil {
				// Check if varType is compatible with annotType (subtyping)
				subst, err := typesystem.UnifyAllowExtraWithResolver(annotType, varType, w.symbolTable)
				if err != nil {
					w.addError(diagnostics.NewError(
						diagnostics.ErrA003,
						expr.Value.GetToken(),
						"type mismatch in assignment: expected "+annotType.String()+", got "+varType.String(),
					))
				} else {
					// Update value's type in TypeMap with unified type
					w.TypeMap[expr.Value] = varType.Apply(subst)
				}
			}
			// Use the annotation type for the variable (not the inferred type)
			varType = annotType
		}

		// Check if it exists in scope chain (Update)
		if w.symbolTable.IsDefined(ident.Value) {
			// Checks for constants, types, traits, modules, constructors, and imported symbols
			// are now done in inference (inference_decl.go) to avoid duplication
			// and ensure correct error positioning.
			// Type compatibility is also checked in inference.

			// Mutable variable - reassignment allowed (checked in inference)
		} else {
			// New variable definition
			w.symbolTable.Define(ident.Value, varType, "")
		}
	} else if ma, ok := expr.Left.(*ast.MemberExpression); ok {
		// Update member
		ma.Accept(w) // Check if left exists
	} else {
		w.addError(diagnostics.NewError(
			diagnostics.ErrA003,
			expr.GetToken(),
			"invalid assignment target",
		))
	}
}

func (w *walker) VisitPatternAssignExpression(expr *ast.PatternAssignExpression) {
	// Analyze the value being assigned
	if expr.Value != nil {
		expr.Value.Accept(w)
	}

	// Infer the value type
	valType, s1, err := InferWithContext(w.inferCtx, expr.Value, w.symbolTable)
	if err != nil {
		w.appendError(expr.Value, err)
		return
	}
	valType = valType.Apply(s1)

	// Bind pattern variables with inferred types
	w.bindPatternVariables(expr.Pattern, valType, expr.Token)
}

func (w *walker) VisitPrefixExpression(expr *ast.PrefixExpression) {
	if expr.Right != nil {
		expr.Right.Accept(w)
	}
}

func (w *walker) VisitInfixExpression(expr *ast.InfixExpression) {
	if expr.Left != nil {
		expr.Left.Accept(w)
	}
	if expr.Right != nil {
		expr.Right.Accept(w)
	}
}

func (w *walker) VisitOperatorAsFunction(expr *ast.OperatorAsFunction) {
	// Operator-as-function is handled in inference, nothing to visit
}

func (w *walker) VisitPostfixExpression(expr *ast.PostfixExpression) {
	expr.Left.Accept(w)
}

func (w *walker) VisitCallExpression(expr *ast.CallExpression) {
	// Visit function and arguments - inference handles undefined checks
	if expr.Function != nil {
		expr.Function.Accept(w)
	}
	for _, arg := range expr.Arguments {
		if arg != nil {
			arg.Accept(w)
		}
	}
}

func (w *walker) VisitMemberExpression(n *ast.MemberExpression) {
	n.Left.Accept(w)
}

func (w *walker) VisitIndexExpression(n *ast.IndexExpression) {
	n.Left.Accept(w)
	n.Index.Accept(w)
}

func (w *walker) VisitAnnotatedExpression(expr *ast.AnnotatedExpression) {
	// Validating type annotations would happen during inference
	expr.Expression.Accept(w)
}

func (w *walker) VisitTypeApplicationExpression(n *ast.TypeApplicationExpression) {
	// 1. Analyze the base expression (e.g., the identifier/function being applied)
	n.Expression.Accept(w)

	// 2. Validate Type Arguments
	for _, t := range n.TypeArguments {
		// We could use BuildType to verify they are valid types in current scope
		// (e.g., defined type names)
		// Since BuildType returns typesystem.Type and we don't have a place to store them
		// here (except TypeMap), we just call it for side-effects (errors).
		_ = BuildType(t, w.symbolTable, &w.errors)
	}

	// Note: Full type checking of the application happens in `Infer` which calls `inferTypeApplicationExpression`.
}

func (w *walker) VisitSpreadExpression(n *ast.SpreadExpression) {
	if n == nil || n.Expression == nil {
		return
	}
	n.Expression.Accept(w)
}

func (w *walker) VisitFunctionLiteral(n *ast.FunctionLiteral) {
	// Similar to FunctionStatement but no name registration in outer scope

	// 1. Create new scope for function body
	outer := w.symbolTable
	w.symbolTable = symbols.NewEnclosedSymbolTable(outer)
	defer func() { w.symbolTable = outer }()

	// 2. Register parameters
	for _, param := range n.Parameters {
		var paramType typesystem.Type
		if param.Type != nil {
			paramType = BuildType(param.Type, w.symbolTable, &w.errors)
		} else {
			paramType = w.freshVar()
		}

		// For variadic parameters, wrap in List
		if param.IsVariadic {
			paramType = typesystem.TApp{
				Constructor: typesystem.TCon{Name: config.ListTypeName},
				Args:        []typesystem.Type{paramType},
			}
		}

		// Don't define ignored parameters (_) in scope
		if !param.IsIgnored {
			w.symbolTable.Define(param.Name.Value, paramType, "")
		}
	}

	// 3. Analyze body
	prevInLoop := w.inLoop
	w.inLoop = false

	n.Body.Accept(w)

	w.markTailCalls(n.Body) // Mark tail calls in lambda body
	w.inLoop = prevInLoop

	// 4. Check return type if explicit
	if n.ReturnType != nil {
		declaredRet := BuildType(n.ReturnType, w.symbolTable, &w.errors)
		bodyType, sBody, err := InferWithContext(w.inferCtx, n.Body, w.symbolTable)
		if err != nil {
			w.addError(diagnostics.NewError(
				diagnostics.ErrA003,
				n.Body.GetToken(),
				err.Error(),
			))
		} else {
			// Apply body subst to declared type?
			declaredRet = declaredRet.Apply(sBody)

			_, err := typesystem.Unify(declaredRet, bodyType)
			if err != nil {
				w.addError(diagnostics.NewError(
					diagnostics.ErrA003,
					n.Body.GetToken(),
					"lambda return type mismatch: declared "+declaredRet.String()+", got "+bodyType.String(),
				))
			}
		}
	}
}

func (w *walker) VisitIdentifier(ident *ast.Identifier) {
	// Inference handles undefined checks
}

func (w *walker) VisitIntegerLiteral(lit *ast.IntegerLiteral) {}
func (w *walker) VisitFloatLiteral(lit *ast.FloatLiteral)     {}
func (w *walker) VisitBigIntLiteral(lit *ast.BigIntLiteral)   {}
func (w *walker) VisitRationalLiteral(lit *ast.RationalLiteral) {}
func (w *walker) VisitBooleanLiteral(lit *ast.BooleanLiteral) {}
func (w *walker) VisitNilLiteral(lit *ast.NilLiteral)         {}
func (w *walker) VisitStringLiteral(n *ast.StringLiteral) {}
func (w *walker) VisitFormatStringLiteral(n *ast.FormatStringLiteral) {}
func (w *walker) VisitInterpolatedString(n *ast.InterpolatedString) {
	for _, part := range n.Parts {
		part.Accept(w)
	}
}
func (w *walker) VisitCharLiteral(n *ast.CharLiteral) {}

func (w *walker) VisitBytesLiteral(n *ast.BytesLiteral) {}

func (w *walker) VisitBitsLiteral(n *ast.BitsLiteral) {}

func (w *walker) VisitTupleLiteral(lit *ast.TupleLiteral) {
	for _, el := range lit.Elements {
		el.Accept(w)
	}
}

func (w *walker) VisitListLiteral(n *ast.ListLiteral) {
	for _, el := range n.Elements {
		el.Accept(w)
	}
}

func (w *walker) VisitRecordLiteral(n *ast.RecordLiteral) {
	// Visit spread expression first if present
	if n.Spread != nil {
		n.Spread.Accept(w)
	}

	// Sort keys for deterministic traversal order
	keys := make([]string, 0, len(n.Fields))
	for k := range n.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		n.Fields[k].Accept(w)
	}
}

func (w *walker) VisitMapLiteral(n *ast.MapLiteral) {
	for _, pair := range n.Pairs {
		pair.Key.Accept(w)
		pair.Value.Accept(w)
	}
}
