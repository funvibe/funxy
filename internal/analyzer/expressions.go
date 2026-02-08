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
	// Inference and validation is handled by InferWithContext (called from VisitExpressionStatement)
	// We just need to traverse children to ensure any nested structures are visited
	// (though InferWithContext also traverses, so this might be redundant, but safe)
	if expr.Left != nil {
		expr.Left.Accept(w)
	}
	if expr.Value != nil {
		expr.Value.Accept(w)
	}
}

func (w *walker) VisitPatternAssignExpression(expr *ast.PatternAssignExpression) {
	// Inference handled by InferWithContext
	if expr.Value != nil {
		expr.Value.Accept(w)
	}
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
	if expr.Left != nil {
		expr.Left.Accept(w)
	}
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
	if n.Left != nil {
		n.Left.Accept(w)
	}
}

func (w *walker) VisitIndexExpression(n *ast.IndexExpression) {
	if n.Left != nil {
		n.Left.Accept(w)
	}
	if n.Index != nil {
		n.Index.Accept(w)
	}
}

func (w *walker) VisitAnnotatedExpression(expr *ast.AnnotatedExpression) {
	// Validating type annotations would happen during inference
	if expr.Expression != nil {
		expr.Expression.Accept(w)
	}
}

func (w *walker) VisitTypeApplicationExpression(n *ast.TypeApplicationExpression) {
	// Analyze the base expression (e.g., the identifier/function being applied)
	if n.Expression != nil {
		n.Expression.Accept(w)
	}

	// Validate Type Arguments
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

	// Create new scope for function body
	outer := w.symbolTable
	w.symbolTable = symbols.NewEnclosedSymbolTable(outer, symbols.ScopeFunction)
	defer func() { w.symbolTable = outer }()

	// Pre-calculate declared return type (to define implicit generics BEFORE params overwrite them)
	var declaredRet typesystem.Type
	if n.ReturnType != nil {
		declaredRet = BuildType(n.ReturnType, w.symbolTable, &w.errors)

		// Refresh TVars that are TCons in scope (Rigid Type Variables from outer context)
		freeVars := declaredRet.FreeTypeVariables()
		rigidSubst := make(typesystem.Subst)
		for _, v := range freeVars {
			if resolved, ok := w.symbolTable.ResolveType(v.Name); ok {
				if tCon, ok := resolved.(typesystem.TCon); ok {
					rigidSubst[v.Name] = tCon
				}
			}
		}
		if len(rigidSubst) > 0 {
			declaredRet = declaredRet.Apply(rigidSubst)
		}
	}

	// Register parameters
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

	// Analyze body
	prevInLoop := w.inLoop
	w.inLoop = false

	// Set inFunctionBody flag to skip redundant expression inference during walk
	// because the whole body will be inferred together when the function/lambda is inferred
	prevInFn := w.inFunctionBody
	w.inFunctionBody = true

	w.pushReturnType(declaredRet)
	defer w.popReturnType()

	if n.Body != nil {
		n.Body.Accept(w)
	}

	w.inFunctionBody = prevInFn

	if n.Body != nil {
		w.markTailCalls(n.Body) // Mark tail calls in lambda body
	}
	w.inLoop = prevInLoop

	// Check return type if explicit
	// Only run explicit inference if we are NOT inside another function body
	// (because nested functions are already inferred by the outer function's inference pass)
	if n.ReturnType != nil && !prevInFn {
		// Clear pending witnesses and constraints from the walk phase (Accept)
		// because we are about to re-infer the whole body and we want fresh witnesses/constraints
		w.inferCtx.PendingWitnesses = nil
		w.inferCtx.Constraints = nil

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

			subst, err := typesystem.Unify(declaredRet, bodyType)
			if err != nil {
				w.addError(diagnostics.NewError(
					diagnostics.ErrA003,
					n.Body.GetToken(),
					"lambda return type mismatch: declared "+declaredRet.String()+", got "+bodyType.String(),
				))
			} else {
				// Update GlobalSubst with the unification result!
				w.inferCtx.GlobalSubst = subst.Compose(w.inferCtx.GlobalSubst)
			}
		}
	}
}

func (w *walker) VisitIdentifier(ident *ast.Identifier) {
	// Inference handles undefined checks
}

func (w *walker) VisitIntegerLiteral(lit *ast.IntegerLiteral)         {}
func (w *walker) VisitFloatLiteral(lit *ast.FloatLiteral)             {}
func (w *walker) VisitBigIntLiteral(lit *ast.BigIntLiteral)           {}
func (w *walker) VisitRationalLiteral(lit *ast.RationalLiteral)       {}
func (w *walker) VisitBooleanLiteral(lit *ast.BooleanLiteral)         {}
func (w *walker) VisitNilLiteral(lit *ast.NilLiteral)                 {}
func (w *walker) VisitStringLiteral(n *ast.StringLiteral)             {}
func (w *walker) VisitFormatStringLiteral(n *ast.FormatStringLiteral) {}
func (w *walker) VisitInterpolatedString(n *ast.InterpolatedString) {
	for _, part := range n.Parts {
		if part != nil {
			part.Accept(w)
		}
	}
}
func (w *walker) VisitCharLiteral(n *ast.CharLiteral) {}

func (w *walker) VisitBytesLiteral(n *ast.BytesLiteral) {}

func (w *walker) VisitBitsLiteral(n *ast.BitsLiteral) {}

func (w *walker) VisitTupleLiteral(lit *ast.TupleLiteral) {
	for _, el := range lit.Elements {
		if el != nil {
			el.Accept(w)
		}
	}
}

func (w *walker) VisitListLiteral(n *ast.ListLiteral) {
	if n == nil {
		return
	}
	for _, el := range n.Elements {
		if el != nil {
			el.Accept(w)
		}
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
		if n.Fields[k] != nil {
			n.Fields[k].Accept(w)
		}
	}
}

func (w *walker) VisitMapLiteral(n *ast.MapLiteral) {
	for _, pair := range n.Pairs {
		if pair.Key != nil {
			pair.Key.Accept(w)
		}
		if pair.Value != nil {
			pair.Value.Accept(w)
		}
	}
}

func (w *walker) VisitListComprehension(n *ast.ListComprehension) {
	// Create a new scope for the comprehension
	outer := w.symbolTable
	w.symbolTable = symbols.NewEnclosedSymbolTable(outer, symbols.ScopeBlock)
	defer func() { w.symbolTable = outer }()

	// Process clauses in order - generators introduce bindings, filters use them
	for _, clause := range n.Clauses {
		switch c := clause.(type) {
		case *ast.CompGenerator:
			// Visit iterable expression to traverse nested nodes.
			if c.Iterable != nil {
				c.Iterable.Accept(w)
			}

			// Bind pattern variables without inferring iterable types here.
			// Full type validation happens in inferListComprehension during inference.
			w.bindPatternVariablesLoose(c.Pattern, c.Token)
		case *ast.CompFilter:
			// Filters use variables from generators
			if c.Condition != nil {
				c.Condition.Accept(w)
			}
		}
	}

	// Visit the output expression (uses all bound variables)
	if n.Output != nil {
		n.Output.Accept(w)
	}
}

func (w *walker) VisitRangeExpression(n *ast.RangeExpression) {
	if n.Start != nil {
		n.Start.Accept(w)
	}
	if n.Next != nil {
		n.Next.Accept(w)
	}
	if n.End != nil {
		n.End.Accept(w)
	}
}
