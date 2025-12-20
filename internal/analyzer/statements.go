package analyzer

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

func (w *walker) VisitProgram(program *ast.Program) {
	// Detect package name
	for _, stmt := range program.Statements {
		if pkg, ok := stmt.(*ast.PackageDeclaration); ok {
			w.currentModuleName = pkg.Name.Value
			// Set in inference context as well
			if w.inferCtx != nil {
				w.inferCtx.CurrentModuleName = pkg.Name.Value
			}
			break
		}
	}

	if w.mode == ModeHeaders {
		// Pass 1: Headers (Imports, Declarations)
		// Strategy for Cyclic Dependencies:
		// 1. Register Declarations (Names) FIRST.
		// 2. Process Imports (which might recurse and see our names).
		// 3. Resolve Signature Details (which might use imported types).

		// Phase 1: Pre-register Names
		for _, stmt := range program.Statements {
			if stmt == nil {
				continue
			}
			switch s := stmt.(type) {
			case *ast.TypeDeclarationStatement:
				// Register TCon (skip if parsing failed)
				if s == nil || s.Name == nil {
					continue
				}
				w.symbolTable.DefineTypePending(s.Name.Value, typesystem.TCon{Name: s.Name.Value}, w.currentModuleName)
				// Register Kind
				kind := typesystem.Star
				if len(s.TypeParameters) > 0 {
					kinds := make([]typesystem.Kind, len(s.TypeParameters)+1)
					for i := range s.TypeParameters {
						kinds[i] = typesystem.Star
					}
					kinds[len(s.TypeParameters)] = typesystem.Star
					kind = typesystem.MakeArrow(kinds...)
				}
				w.symbolTable.RegisterKind(s.Name.Value, kind)
			case *ast.FunctionStatement:
				// Register Function Name with placeholder (skip if parsing failed)
				if s == nil || s.Name == nil {
					continue
				}
				w.symbolTable.DefinePending(s.Name.Value, typesystem.TCon{Name: "PendingFunction"}, w.currentModuleName)
			// NOTE: ConstantDeclarations are NOT pre-registered in Phase 1
			// They are processed in order during body analysis
			// This ensures that `a = 1; a :- 2` correctly errors on the redefinition
			}
		}

		// Phase 2: Imports
		for _, stmt := range program.Statements {
			if s, ok := stmt.(*ast.ImportStatement); ok {
				s.Accept(w)
			}
		}

		// Phase 3: Full Declaration Analysis (Resolving Signatures)
		// Process statements in order to handle interleaved = and :- correctly
		for _, stmt := range program.Statements {
			switch s := stmt.(type) {
			case *ast.ImportStatement:
				// Already done
			case *ast.TypeDeclarationStatement:
				errs := RegisterTypeDeclaration(s, w.symbolTable, w.currentModuleName)
				if len(errs) > 0 {
					w.addErrors(errs)
				}
			case *ast.TraitDeclaration:
				s.Accept(w)
			case *ast.InstanceDeclaration:
				s.Accept(w)
			case *ast.FunctionStatement:
				errs := RegisterFunctionDeclaration(s, w.symbolTable, w.freshVarName, w.currentModuleName)
				if len(errs) > 0 {
					w.addErrors(errs)
				}
			case *ast.ConstantDeclaration:
				// Process constants in order with other statements
				s.Accept(w)
			case *ast.ExpressionStatement:
				// Process expression statements (including assignments) in order
				s.Accept(w)
			}
		}
		return
	}

	if w.mode == ModeBodies {
		// Pass 2: Bodies (only function bodies need secondary pass)
		for _, stmt := range program.Statements {
			switch s := stmt.(type) {
			case *ast.FunctionStatement:
				w.analyzeFunctionBody(s)
			case *ast.ImportStatement:
				s.Accept(w) // Ensure dependency bodies are analyzed
			}
		}
		return
	}

	// Pass 1: Register all top-level declarations
	for _, stmt := range program.Statements {
	switch s := stmt.(type) {
	case *ast.FunctionStatement:
		errs := RegisterFunctionDeclaration(s, w.symbolTable, w.freshVarName, w.currentModuleName)
			if len(errs) > 0 {
				for _, e := range errs {
					w.addError(e)
				}
			}
		case *ast.TypeDeclarationStatement:
			errs := RegisterTypeDeclaration(s, w.symbolTable, w.currentModuleName)
			if len(errs) > 0 {
				for _, e := range errs {
					w.addError(e)
				}
			}
		case *ast.TraitDeclaration:
			// Register Trait
		case *ast.InstanceDeclaration:
			// Register Instance
		case *ast.ImportStatement:
			// Legacy support for single-pass import handling (if needed)
			s.Accept(w)
		case *ast.ConstantDeclaration:
			s.Accept(w)
		}
	}

	// Pass 2: Analyze bodies and other statements
	for _, stmt := range program.Statements {
		switch s := stmt.(type) {
		case *ast.FunctionStatement:
			// Analyze function body manually to avoid duplicate registration
			w.analyzeFunctionBody(s)

		case *ast.TypeDeclarationStatement:
			// Already registered.
			continue

		case *ast.TraitDeclaration:
			s.Accept(w)
		case *ast.InstanceDeclaration:
			s.Accept(w)
		case *ast.ImportStatement:
			// Already visited in Pass 1
			continue
		case *ast.ConstantDeclaration:
			// Already visited
			continue
		default:
			stmt.Accept(w)
		}
	}
}

func (w *walker) analyzeFunctionBody(n *ast.FunctionStatement) {
	// Create scope for parameters
	outer := w.symbolTable
	w.symbolTable = symbols.NewEnclosedSymbolTable(outer)
	defer func() { w.symbolTable = outer }()

	// Register Generic Constraints / Type Params from FunctionStatement if needed
	// (Already done in Pass 1 for signature, but for Body analysis we need them in scope)
	for _, tp := range n.TypeParams {
		// Use TCon (Rigid Type Constant) for body analysis to prevent instantiation
		w.symbolTable.DefineType(tp.Value, typesystem.TCon{Name: tp.Value}, "")
	}

	// Register constraints in the inference context
	for _, c := range n.Constraints {
		w.inferCtx.AddConstraint(c.TypeVar, c.Trait)
	}

	// Define parameters
	if n.Receiver != nil {
		if n.Receiver.Type != nil {
			recvType := BuildType(n.Receiver.Type, w.symbolTable, &w.errors)
			w.symbolTable.Define(n.Receiver.Name.Value, recvType, "")
		} else {
			w.symbolTable.Define(n.Receiver.Name.Value, w.freshVar(), "")
		}
	}

	for _, param := range n.Parameters {
		var paramType typesystem.Type
		if param.Type != nil {
			paramType = BuildType(param.Type, w.symbolTable, &w.errors)
		} else {
			paramType = w.freshVar()
		}

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

	// Look up expected return type from Outer Scope (where signature was registered)
	var expectedRetType typesystem.Type

	// Try to find the function symbol
	var fnType typesystem.Type
	if n.Receiver != nil {
		recvTypeName := resolveReceiverTypeName(n.Receiver.Type, outer)
		if recvTypeName != "" {
			if t, ok := outer.GetExtensionMethod(recvTypeName, n.Name.Value); ok {
				fnType = t
			}
		}
	} else {
		if sym, ok := outer.Find(n.Name.Value); ok {
			fnType = sym.Type
		}
	}

	if tFunc, ok := fnType.(typesystem.TFunc); ok {
		expectedRetType = tFunc.ReturnType
		// Re-resolve qualified type names that might have been placeholders during cyclic imports
		if tCon, ok := expectedRetType.(typesystem.TCon); ok {
			typeName := tCon.Name
			if resolved, ok := w.symbolTable.ResolveType(typeName); ok {
				if _, isTCon := resolved.(typesystem.TCon); !isTCon {
					expectedRetType = resolved
				}
			} else if tCon.Module != "" {
				// Try with module prefix for cross-module types
				qualifiedName := tCon.Module + "." + tCon.Name
				if resolved, ok := w.symbolTable.ResolveType(qualifiedName); ok {
					if _, isTCon := resolved.(typesystem.TCon); !isTCon {
						expectedRetType = resolved
					}
				}
			}
		}
	} else {
		// Fallback: Should not happen if registered correctly.
		// If implicit, assume Nil as default (legacy behavior) or new TVar?
		// But if we didn't find it, we can't verify against signature.
		if n.ReturnType != nil {
			expectedRetType = BuildType(n.ReturnType, w.symbolTable, &w.errors)
		} else {
			// Implicit return type: Use Nil as placeholder if we can't find the signature TVar?
			// No, if we can't find it, we can't update the inference.
			expectedRetType = typesystem.Nil
		}
	}

	// Analyze body
	if n.Body != nil {
		prevInLoop := w.inLoop
		w.inLoop = false
		n.Body.Accept(w)
		w.markTailCalls(n.Body)
		w.inLoop = prevInLoop

		// Infer body type
		bodyType, sBody, err := InferWithContext(w.inferCtx, n.Body, w.symbolTable)
		if err != nil {
			w.appendError(n.Body, err)
		} else {
			// Apply accumulated substitution from body to return type before unification
			expectedRetType = expectedRetType.Apply(sBody)

			subst, err := typesystem.Unify(expectedRetType, bodyType)
			if err != nil {
				w.addError(diagnostics.NewError(diagnostics.ErrA003, n.Body.GetToken(),
					"function body type "+bodyType.String()+" does not match return type "+expectedRetType.String()))
			} else {
				// Success! Update TypeMap and SymbolTable with resolved types
				finalSubst := subst.Compose(sBody)

				// FIX: Remove bindings for generic type params to avoid replacing TVars with Rigid TCons in the signature
				for _, tp := range n.TypeParams {
					delete(finalSubst, tp.Value)
				}

				// Resolve fnType (which contains params and return type)
				if tFunc, ok := fnType.(typesystem.TFunc); ok {
					resolvedFnType := tFunc.Apply(finalSubst)

					// Update TypeMap for the function definition
					w.TypeMap[n] = resolvedFnType

					// Update SymbolTable so callers see the resolved type
					// Note: We need to be careful about overwriting if it was already defined.
					// Since we are analyzing the body of the definition, we are the source of truth.
					if n.Receiver != nil {
						// Extension method update
						recvTypeName := resolveReceiverTypeName(n.Receiver.Type, outer)
						if recvTypeName != "" {
							// We can't easily update extension method registry without a specific method.
							// But RegisterExtensionMethod allows overwriting?
							outer.RegisterExtensionMethod(recvTypeName, n.Name.Value, resolvedFnType)
						}
					} else {
						// Global function type update after inference
						// Use Update instead of Define to preserve IsConstant flag
						err := outer.Update(n.Name.Value, resolvedFnType)
						if err != nil {
							// If Update fails, it means function wasn't registered - shouldn't happen
							// Fall back to DefineConstant for safety
							outer.DefineConstant(n.Name.Value, resolvedFnType, w.currentModuleName)
						}
					}
				}
			}
		}
	}
}

func (w *walker) VisitExpressionStatement(stmt *ast.ExpressionStatement) {
	if stmt.Expression != nil {
		stmt.Expression.Accept(w)
		// Run inference to check types and exhaustiveness (for scripts/top-level expressions)
		t, s, err := InferWithContext(w.inferCtx, stmt.Expression, w.symbolTable)
		if err != nil {
			w.appendError(stmt.Expression, err)
		} else {
			w.TypeMap[stmt.Expression] = t.Apply(s)
			// Apply substitution to all sub-expressions so type variables are resolved
			w.applySubstToNode(stmt.Expression, s)
		}
	}
}

func (w *walker) VisitBlockStatement(block *ast.BlockStatement) {
	// Create a new scope for the block
	outer := w.symbolTable
	w.symbolTable = symbols.NewEnclosedSymbolTable(outer)
	defer func() { w.symbolTable = outer }()

	for _, stmt := range block.Statements {
		stmt.Accept(w)
	}
}

func (w *walker) VisitIfExpression(expr *ast.IfExpression) {
	if expr.Condition != nil {
		expr.Condition.Accept(w)
	}
	if expr.Consequence != nil {
		expr.Consequence.Accept(w)
	}
	if expr.Alternative != nil {
		expr.Alternative.Accept(w)
	}
}

func (w *walker) VisitForExpression(n *ast.ForExpression) {
	// Create loop scope
	outer := w.symbolTable
	w.symbolTable = symbols.NewEnclosedSymbolTable(outer)
	defer func() { w.symbolTable = outer }()

	if n.Iterable != nil {
		// Iteration loop
		n.Iterable.Accept(w)

		// Define loop variable
		// We infer the iterable type to determine item type.
		iterableType, s1, err := InferWithContext(w.inferCtx, n.Iterable, outer) // Use outer scope for inference
		if err != nil {
			w.appendError(n.Iterable, err)
			// Use Any/Unknown type for item to continue analysis
			w.symbolTable.Define(n.ItemName.Value, w.freshVar(), "")
		} else {
			iterableType = iterableType.Apply(s1)
			w.TypeMap[n.Iterable] = iterableType

			// Check if iterable is List (direct support)
			var itemType typesystem.Type

			if tApp, ok := iterableType.(typesystem.TApp); ok {
				if tCon, ok := tApp.Constructor.(typesystem.TCon); ok && tCon.Name == config.ListTypeName && len(tApp.Args) == 1 {
					itemType = tApp.Args[0]
				}
			}

			if itemType == nil {
				// Check for iter method via Iter trait protocol
				// We look for an iter function that can handle this type.
				// The function exists if the type (or a compatible type) implements Iter.
				if iterSym, ok := w.symbolTable.Find(config.IterMethodName); ok {
					iterType := InstantiateWithContext(w.inferCtx, iterSym.Type)
					if tFunc, ok := iterType.(typesystem.TFunc); ok && len(tFunc.Params) > 0 {
						subst, err := typesystem.Unify(tFunc.Params[0], iterableType)
						if err == nil {
							retType := tFunc.ReturnType.Apply(subst)
							if iteratorFunc, ok := retType.(typesystem.TFunc); ok {
								iteratorRet := iteratorFunc.ReturnType
								if tApp, ok := iteratorRet.(typesystem.TApp); ok {
									if tCon, ok := tApp.Constructor.(typesystem.TCon); ok && tCon.Name == config.OptionTypeName && len(tApp.Args) >= 1 {
										itemType = tApp.Args[0]
									}
								}
							}
						}
					}
				}
			}

			if itemType == nil {
				w.addError(diagnostics.NewError(diagnostics.ErrA003, n.Iterable.GetToken(), "iterable must be List or implement Iter trait, got "+iterableType.String()))
				itemType = w.freshVar()
			}

			w.symbolTable.Define(n.ItemName.Value, itemType, "")
		}
	} else {
		// Condition loop
		n.Condition.Accept(w)
	}

	// Define __loop_return in scope to support break inference within the loop body
	// This matches the logic in inferForExpression
	loopReturnType := w.freshVar()
	w.symbolTable.Define("__loop_return", loopReturnType, "")

	// Analyze body
	prevInLoop := w.inLoop
	w.inLoop = true
	n.Body.Accept(w)
	w.inLoop = prevInLoop
}

func (w *walker) VisitBreakStatement(n *ast.BreakStatement) {
	if !w.inLoop {
		w.addError(diagnostics.NewError(diagnostics.ErrA003, n.Token, "break statement outside of loop"))
	}
	if n.Value != nil {
		n.Value.Accept(w)
		t, s, err := InferWithContext(w.inferCtx, n.Value, w.symbolTable)
		if err != nil {
			w.appendError(n.Value, err)
		} else {
			w.TypeMap[n.Value] = t.Apply(s)
		}
	}
}

func (w *walker) VisitContinueStatement(n *ast.ContinueStatement) {
	if !w.inLoop {
		w.addError(diagnostics.NewError(diagnostics.ErrA003, n.Token, "continue statement outside of loop"))
	}
}

func (w *walker) VisitMatchExpression(n *ast.MatchExpression) {
	if n == nil {
		return
	}
	// Analyze scrutinee first
	if n.Expression != nil {
		n.Expression.Accept(w)
	}

	// The full match expression analysis (including patterns, exhaustiveness)
	// is done by InferWithContext. We just need to traverse the arm bodies
	// to continue the walk and populate symbol tables for nested expressions.

	// Infer scrutinee type for pattern binding
	var scrutineeType typesystem.Type
	if n.Expression != nil {
		var s1 typesystem.Subst
		var err error
		scrutineeType, s1, err = InferWithContext(w.inferCtx, n.Expression, w.symbolTable)
		if err != nil {
			// Error already reported by inference
			scrutineeType = w.freshVar()
		} else {
			scrutineeType = scrutineeType.Apply(s1)
			w.TypeMap[n.Expression] = scrutineeType
		}
	} else {
		scrutineeType = w.freshVar()
	}

	for _, arm := range n.Arms {
		// Create scope for arm
		outer := w.symbolTable
		w.symbolTable = symbols.NewEnclosedSymbolTable(outer)

		// Bind pattern variables (ignore errors - they're reported by inference)
		if patSubst, err := inferPattern(w.inferCtx, arm.Pattern, scrutineeType, w.symbolTable); err == nil {
			_ = patSubst
			// Continue body analysis with bound variables
			if arm.Expression != nil {
				arm.Expression.Accept(w)
			}
		}
		// If pattern fails, skip body to avoid cascading errors

		w.symbolTable = outer
	}
}
