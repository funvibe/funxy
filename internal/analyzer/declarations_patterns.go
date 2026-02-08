package analyzer

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/token"
	"github.com/funvibe/funxy/internal/typesystem"
)

func (w *walker) VisitConstantDeclaration(n *ast.ConstantDeclaration) {
	// Skip if not properly parsed
	if n == nil {
		return
	}

	// Mode Checks: Only process in ModeBodies or ModeFull
	if w.mode == ModeNaming || w.mode == ModeHeaders || w.mode == ModeInstances {
		return
	}

	// Handle pattern destructuring: (a, b) :- pair
	if n.Pattern != nil {
		w.visitPatternDeclaration(n)
		return
	}

	// Simple binding: x :- expr
	if n.Name == nil {
		return
	}

	// Special case: Wildcard binding (_ :- expr)
	// We infer the type to check for errors in the expression, but we don't bind it.
	if n.Name.Value == "_" {
		// 2. Infer Value Type (just for checking)
		_, _, err := InferWithContext(w.inferCtx, n.Value, w.symbolTable)
		if err != nil {
			w.appendError(n.Value, err)
		}
		return
	}

	// 1. Check for redefinition - constants can NEVER be redefined
	if w.symbolTable.IsDefined(n.Name.Value) {
		sym, ok := w.symbolTable.Find(n.Name.Value)
		// Only allow if it's a pending symbol (forward declaration)
		if ok && !sym.IsPending {
			// In ModeInstances (where we inject global constants), we might visit multiple times?
			// No, VisitInstanceDeclaration manually injects.
			// But AnalyzeInstances visits Statements.
			// If we modify Program.Statements, we might visit the new nodes.

			// Allow redefinition for compiler-generated constants (starting with $)
			if len(n.Name.Value) > 0 && n.Name.Value[0] == '$' {
				// Implicitly skip redefinition check for internal symbols
				return
			}

			w.addError(diagnostics.NewError(diagnostics.ErrA004, n.Name.GetToken(), n.Name.Value))
			return
		}
	}

	// 2. Infer Value Type
	var explicitType typesystem.Type
	if n.TypeAnnotation != nil {
		var errs []*diagnostics.DiagnosticError
		explicitType = BuildType(n.TypeAnnotation, w.symbolTable, &errs)
		if len(errs) > 0 {
			w.appendError(n.Value, errs[0])
		} else {
			// Kind Check: Annotation must be a proper type (Kind *)
			if k, err := typesystem.KindCheck(explicitType); err != nil {
				w.appendError(n.Value, err)
			} else if !k.Equal(typesystem.Star) {
				w.appendError(n.Value, diagnostics.NewError(
					diagnostics.ErrA003,
					n.TypeAnnotation.GetToken(),
					"type annotation must be type (kind *), got kind "+k.String(),
				))
			} else {
				// Propagate expected type to value inference (for context-sensitive inference like pure())
				if w.inferCtx.ExpectedReturnTypes == nil {
					w.inferCtx.ExpectedReturnTypes = make(map[ast.Node]typesystem.Type)
				}
				w.inferCtx.ExpectedReturnTypes[n.Value] = explicitType
			}
		}
	}

	valType, s1, err := InferWithContext(w.inferCtx, n.Value, w.symbolTable)
	if err != nil {
		w.appendError(n.Value, err)
		valType = w.freshVar() // Recovery
	}
	valType = valType.Apply(s1)

	// 3. Check Type Annotation (if present) and Unify
	if explicitType != nil {
		// Unify explicit type with inferred type using AllowExtraWithResolver for HKT
		subst, err := typesystem.UnifyAllowExtraWithResolver(explicitType, valType, w.symbolTable)
		if err != nil {
			w.addError(diagnostics.NewError(
				diagnostics.ErrA003,
				n.Value.GetToken(),
				"constant type mismatch: expected "+explicitType.String()+", got "+valType.String(),
			))
		} else {
			// Use the annotated type as the source of truth, refined by unification
			valType = explicitType.Apply(subst)

			// Apply substitution to the value AST so TypeMap is updated with resolved types
			w.applySubstToNode(n.Value, subst)

			// Update global substitution in context
			if w.inferCtx != nil {
				w.inferCtx.GlobalSubst = subst.Compose(w.inferCtx.GlobalSubst)
			}
		}
	}

	// 4. Register in Symbol Table as Constant (immutable)
	w.symbolTable.DefineConstant(n.Name.Value, valType, w.currentModuleName)
}

// visitPatternDeclaration handles pattern destructuring in constant bindings (:-)
func (w *walker) visitPatternDeclaration(n *ast.ConstantDeclaration) {
	// 1. Infer Value Type
	valType, s1, err := InferWithContext(w.inferCtx, n.Value, w.symbolTable)
	if err != nil {
		w.appendError(n.Value, err)
		return
	}
	valType = valType.Apply(s1)

	// 2. Bind pattern variables with inferred types as CONSTANTS
	w.bindPatternVariablesAsConstant(n.Pattern, valType, n.Token)
}

// bindPatternVariables binds variables from a pattern to their types (mutable)
func (w *walker) bindPatternVariables(pat ast.Pattern, valType typesystem.Type, tok token.Token) {
	w.bindPatternVariablesWithConstFlag(pat, valType, tok, false)
}

// bindPatternVariablesAsConstant binds variables from a pattern to their types (immutable)
func (w *walker) bindPatternVariablesAsConstant(pat ast.Pattern, valType typesystem.Type, tok token.Token) {
	w.bindPatternVariablesWithConstFlag(pat, valType, tok, true)
}

// bindPatternVariablesLoose binds identifiers in a pattern without requiring
// type information. This is useful for list comprehensions where full type
// inference happens later, but we still need symbols in scope for filters/output.
func (w *walker) bindPatternVariablesLoose(pat ast.Pattern, tok token.Token) {
	w.bindPatternVariablesLooseWithConstFlag(pat, tok, false)
}

func (w *walker) bindPatternVariablesLooseWithConstFlag(pat ast.Pattern, tok token.Token, isConstant bool) {
	switch p := pat.(type) {
	case *ast.IdentifierPattern:
		if p.Value == "_" {
			return
		}
		// Check naming convention (variable must start with lowercase)
		if !checkValueName(p.Value, p.Token, &w.errors) {
			return
		}
		// Check for redefinition
		if w.symbolTable.IsDefined(p.Value) {
			sym, ok := w.symbolTable.Find(p.Value)
			if ok && !sym.IsPending {
				w.addError(diagnostics.NewError(diagnostics.ErrA004, p.Token, p.Value))
				return
			}
		}
		if isConstant {
			w.symbolTable.DefineConstant(p.Value, w.freshVar(), "")
		} else {
			w.symbolTable.Define(p.Value, w.freshVar(), "")
		}

	case *ast.TuplePattern:
		for _, elem := range p.Elements {
			w.bindPatternVariablesLooseWithConstFlag(elem, tok, isConstant)
		}

	case *ast.ListPattern:
		for _, elem := range p.Elements {
			w.bindPatternVariablesLooseWithConstFlag(elem, tok, isConstant)
		}

	case *ast.RecordPattern:
		for _, fieldPat := range p.Fields {
			w.bindPatternVariablesLooseWithConstFlag(fieldPat, tok, isConstant)
		}

	case *ast.SpreadPattern:
		if p.Pattern != nil {
			w.bindPatternVariablesLooseWithConstFlag(p.Pattern, tok, isConstant)
		}

	case *ast.ConstructorPattern:
		for _, elem := range p.Elements {
			w.bindPatternVariablesLooseWithConstFlag(elem, tok, isConstant)
		}

	case *ast.TypePattern:
		if p.Name == "_" {
			return
		}
		if !checkValueName(p.Name, p.Token, &w.errors) {
			return
		}
		if w.symbolTable.IsDefined(p.Name) {
			sym, ok := w.symbolTable.Find(p.Name)
			if ok && !sym.IsPending {
				w.addError(diagnostics.NewError(diagnostics.ErrA004, p.Token, p.Name))
				return
			}
		}
		if isConstant {
			w.symbolTable.DefineConstant(p.Name, w.freshVar(), "")
		} else {
			w.symbolTable.Define(p.Name, w.freshVar(), "")
		}

	case *ast.StringPattern:
		for _, part := range p.Parts {
			if !part.IsCapture {
				continue
			}
			if part.Value == "_" {
				continue
			}
			if !checkValueName(part.Value, tok, &w.errors) {
				continue
			}
			if w.symbolTable.IsDefined(part.Value) {
				sym, ok := w.symbolTable.Find(part.Value)
				if ok && !sym.IsPending {
					w.addError(diagnostics.NewError(diagnostics.ErrA004, tok, part.Value))
					continue
				}
			}
			if isConstant {
				w.symbolTable.DefineConstant(part.Value, w.freshVar(), "")
			} else {
				w.symbolTable.Define(part.Value, w.freshVar(), "")
			}
		}

	case *ast.PinPattern, *ast.WildcardPattern, *ast.LiteralPattern:
		// No new bindings.

	default:
		// Skip unsupported patterns here; full validation happens in inference.
	}
}

// bindPatternVariablesWithConstFlag binds variables from a pattern to their types
func (w *walker) bindPatternVariablesWithConstFlag(pat ast.Pattern, valType typesystem.Type, tok token.Token, isConstant bool) {
	switch p := pat.(type) {
	case *ast.IdentifierPattern:
		// Check naming convention (variable must start with lowercase)
		if !checkValueName(p.Value, p.Token, &w.errors) {
			return
		}
		// Check for redefinition
		if w.symbolTable.IsDefined(p.Value) {
			sym, ok := w.symbolTable.Find(p.Value)
			if ok && !sym.IsPending {
				w.addError(diagnostics.NewError(diagnostics.ErrA004, p.Token, p.Value))
				return
			}
		}
		if isConstant {
			w.symbolTable.DefineConstant(p.Value, valType, "")
		} else {
			w.symbolTable.Define(p.Value, valType, "")
		}

	case *ast.TuplePattern:
		// valType should be TTuple
		if tuple, ok := valType.(typesystem.TTuple); ok {
			if len(tuple.Elements) != len(p.Elements) {
				w.addError(diagnostics.NewError(
					diagnostics.ErrA003,
					tok,
					fmt.Sprintf("tuple pattern has %d elements but value has %d", len(p.Elements), len(tuple.Elements)),
				))
				return
			}
			for i, elem := range p.Elements {
				w.bindPatternVariablesWithConstFlag(elem, tuple.Elements[i], tok, isConstant)
			}
		} else {
			w.addError(diagnostics.NewError(
				diagnostics.ErrA003,
				tok,
				"cannot destructure non-tuple value with tuple pattern",
			))
		}

	case *ast.ListPattern:
		// valType should be TApp List<T>
		if app, ok := valType.(typesystem.TApp); ok {
			if con, ok := app.Constructor.(typesystem.TCon); ok && con.Name == "List" && len(app.Args) > 0 {
				elemType := app.Args[0]
				for _, elem := range p.Elements {
					w.bindPatternVariablesWithConstFlag(elem, elemType, tok, isConstant)
				}
			} else {
				w.addError(diagnostics.NewError(
					diagnostics.ErrA003,
					tok,
					"cannot destructure non-list value with list pattern",
				))
			}
		} else {
			w.addError(diagnostics.NewError(
				diagnostics.ErrA003,
				tok,
				"cannot destructure non-list value with list pattern",
			))
		}

	case *ast.WildcardPattern:
		// Ignore - don't bind anything

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
			w.addError(diagnostics.NewError(
				diagnostics.ErrA003,
				tok,
				"cannot destructure non-record value with record pattern",
			))
			return
		}

		for fieldName, fieldPat := range p.Fields {
			fieldType, ok := fields[fieldName]
			if !ok {
				w.addError(diagnostics.NewError(
					diagnostics.ErrA003,
					tok,
					fmt.Sprintf("record does not have field '%s'", fieldName),
				))
				return
			}
			w.bindPatternVariablesWithConstFlag(fieldPat, fieldType, tok, isConstant)
		}

	default:
		w.addError(diagnostics.NewError(
			diagnostics.ErrA003,
			tok,
			"unsupported pattern in destructuring",
		))
	}
}
