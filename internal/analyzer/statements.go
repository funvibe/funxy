package analyzer

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

func (w *walker) VisitDirectiveStatement(stmt *ast.DirectiveStatement) {
	if stmt.Name == "strict_types" {
		w.symbolTable.SetStrictMode(true)
	}
}

func (w *walker) VisitProgram(program *ast.Program) {
	// Set current file context for error reporting
	if program.File != "" {
		w.currentFile = program.File
	}

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

	if w.mode == ModeNaming {
		// Pass 0: Naming (Discovery) - Register names only
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

				// Check for redefinition (including builtins from prelude)
				if sym, ok := w.symbolTable.Find(s.Name.Value); ok && !sym.IsPending {
					w.addError(diagnostics.NewError(diagnostics.ErrA004, s.Name.GetToken(), s.Name.Value))
					continue
				}

				// Calculate Kind first
				kind := typesystem.Star
				if len(s.TypeParameters) > 0 {
					kinds := make([]typesystem.Kind, len(s.TypeParameters)+1)
					for i := range s.TypeParameters {
						kinds[i] = typesystem.Star
					}
					kinds[len(s.TypeParameters)] = typesystem.Star
					kind = typesystem.MakeArrow(kinds...)
				}

				// Register TCon with correct Kind
				w.symbolTable.DefineTypePending(s.Name.Value, typesystem.TCon{Name: s.Name.Value, KindVal: kind}, w.currentModuleName)
				w.symbolTable.RegisterKind(s.Name.Value, kind)
				w.symbolTable.SetDefinitionFile(s.Name.Value, w.currentFile)
			case *ast.FunctionStatement:
				// Register Function Name with placeholder (skip if parsing failed)
				if s == nil || s.Name == nil {
					continue
				}
				// Check for redefinition (including builtins from prelude)
				// Allow shadowing trait methods - users can define their own functions with same name
				if sym, ok := w.symbolTable.Find(s.Name.Value); ok && !sym.IsPending {
					// Check if it's a trait method - those can be shadowed
					if _, isTraitMethod := w.symbolTable.GetTraitForMethod(s.Name.Value); !isTraitMethod {
						w.addError(diagnostics.NewError(diagnostics.ErrA004, s.Name.GetToken(), s.Name.Value))
						continue
					}
				}
				w.symbolTable.DefinePending(s.Name.Value, typesystem.TVar{Name: "_pending_fn_" + s.Name.Value}, w.currentModuleName)
				w.symbolTable.SetDefinitionFile(s.Name.Value, w.currentFile)
			case *ast.TraitDeclaration:
				if s == nil || s.Name == nil {
					continue
				}
				// Check for redefinition (including builtins from prelude)
				if sym, ok := w.symbolTable.Find(s.Name.Value); ok && !sym.IsPending {
					w.addError(diagnostics.NewError(diagnostics.ErrA004, s.Name.GetToken(), s.Name.Value))
					continue
				}
				w.symbolTable.DefinePendingTrait(s.Name.Value, w.currentModuleName)
				w.symbolTable.SetDefinitionFile(s.Name.Value, w.currentFile)
			case *ast.ConstantDeclaration:
				if s != nil && s.Name != nil {
					// Check for redefinition (including builtins from prelude)
					if sym, ok := w.symbolTable.Find(s.Name.Value); ok && !sym.IsPending {
						w.addError(diagnostics.NewError(diagnostics.ErrA004, s.Name.GetToken(), s.Name.Value))
						continue
					}
					w.symbolTable.DefinePendingConstant(s.Name.Value, typesystem.TVar{Name: "_pending_const_" + s.Name.Value}, w.currentModuleName)
					w.symbolTable.SetDefinitionFile(s.Name.Value, w.currentFile)
				}
			case *ast.ExpressionStatement:
				// Handle top-level assignments: x = expr
				if s != nil && s.Expression != nil {
					if assign, ok := s.Expression.(*ast.AssignExpression); ok {
						if ident, ok := assign.Left.(*ast.Identifier); ok {
							// Check for redefinition (including builtins from prelude)
							if sym, ok := w.symbolTable.Find(ident.Value); ok && !sym.IsPending {
								w.addError(diagnostics.NewError(diagnostics.ErrA004, ident.GetToken(), ident.Value))
								continue
							}
							w.symbolTable.DefinePending(ident.Value, typesystem.TVar{Name: "_pending_var_" + ident.Value}, w.currentModuleName)
							w.symbolTable.SetDefinitionFile(ident.Value, w.currentFile)
						}
					}
				}
			}
		}
		return
	}

	if w.mode == ModeHeaders {
		// Pass 1: Headers (Imports, Declarations)

		// Phase 1: Imports
		for _, stmt := range program.Statements {
			if s, ok := stmt.(*ast.ImportStatement); ok {
				s.Accept(w)
			}
		}

		// Phase 2: Declarations (Resolving Signatures)
		for _, stmt := range program.Statements {
			switch s := stmt.(type) {
			case *ast.ImportStatement:
				// Already done
			case *ast.TypeDeclarationStatement:
				s.Accept(w)
			case *ast.TraitDeclaration:
				s.Accept(w)
			case *ast.FunctionStatement:
				s.Accept(w)
			}
		}
		return
	}

	if w.mode == ModeBodies {
		// Pass 2: Bodies (only function bodies need secondary pass)
		for _, stmt := range program.Statements {
			if stmt == nil {
				continue
			}
			switch s := stmt.(type) {
			case *ast.FunctionStatement:
				if s != nil {
					w.analyzeFunctionBody(s)
				}
			case *ast.ImportStatement:
				if s != nil {
					s.Accept(w) // Ensure dependency bodies are analyzed
				}
			case *ast.ConstantDeclaration:
				if s != nil {
					s.Accept(w)
				}
			case *ast.ExpressionStatement:
				if s != nil {
					s.Accept(w)
				}
			case *ast.ReturnStatement:
				if s != nil {
					s.Accept(w)
				}
			case *ast.InstanceDeclaration:
				if s != nil {
					// We need to visit instance declarations in ModeBodies to process method bodies!
					// In ModeInstances (Pass 3), we only checked signatures.
					s.Accept(w)
				}
			case *ast.DirectiveStatement:
				if s != nil {
					s.Accept(w)
				}
			}
		}
		return
	}

	if w.mode == ModeInstances {
		// Pass 3: Instances (only InstanceDeclaration)
		for _, stmt := range program.Statements {
			if s, ok := stmt.(*ast.InstanceDeclaration); ok {
				s.Accept(w)
			}
		}

		// Flushing of injected statements is now handled by the caller (AnalyzeInstances)
		// to allow flexible ordering (prepending).
		return
	}

	// Pass 1: Register all top-level declarations
	for _, stmt := range program.Statements {
		switch s := stmt.(type) {
		case *ast.FunctionStatement:
			s.Accept(w)
		case *ast.TypeDeclarationStatement:
			s.Accept(w)
		case *ast.TraitDeclaration:
			// Register Trait
		case *ast.InstanceDeclaration:
			// Register Instance
		case *ast.ImportStatement:
			// Legacy support for single-pass import handling (if needed)
			s.Accept(w)
		case *ast.ConstantDeclaration:
			s.Accept(w)
		case *ast.DirectiveStatement:
			s.Accept(w)
		}
	}

	// Pass 2: Analyze bodies and other statements
	for _, stmt := range program.Statements {
		if stmt == nil {
			continue
		}
		switch s := stmt.(type) {
		case *ast.FunctionStatement:
			// Analyze function body manually to avoid duplicate registration
			if s != nil {
				w.analyzeFunctionBody(s)
			}

		case *ast.TypeDeclarationStatement:
			// Already registered.
			continue

		case *ast.TraitDeclaration:
			if s != nil {
				s.Accept(w)
			}
		case *ast.InstanceDeclaration:
			if s != nil {
				s.Accept(w)
			}
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
	w.symbolTable = symbols.NewEnclosedSymbolTable(outer, symbols.ScopeFunction)
	defer func() { w.symbolTable = outer }()

	// Register generic type parameters (from n.TypeParams)
	rigidSubst := make(typesystem.Subst)
	for _, tp := range n.TypeParams {
		// Use TVar for body analysis to allow unification (legacy behavior compatibility)
		// We previously used Rigid TCon, but it caused issues with implicit unification in some tests.
		kind := inferKindFromFunction(n, tp.Value, w.symbolTable)
		tVar := typesystem.TVar{Name: tp.Value, KindVal: kind}
		w.symbolTable.DefineType(tp.Value, tVar, "")
		w.symbolTable.RegisterKind(tp.Value, kind)
		rigidSubst[tp.Value] = tVar
	}

	// Register Witness Parameters (Dictionaries)
	// These were added to WitnessParams in VisitFunctionStatement
	for _, witnessName := range n.WitnessParams {
		// Define as Dictionary type
		w.symbolTable.DefineConstant(witnessName, typesystem.TCon{Name: "Dictionary"}, "")
	}

	// Register constraints in the inference context and collect them for TFunc
	// We need to build typesystem.Constraint list for TFunc
	// But TFunc is created in Pass 1 (VisitFunctionStatement). This logic here is analyzing body (Pass 2).
	// We need to update TFunc constraints if they were incomplete?
	// No, VisitFunctionStatement created TFunc.
	// But VisitFunctionStatement calls BuildType for signature.
	// Constraints are part of signature.
	// Where are constraints added to TFunc?

	// In VisitFunctionStatement (Pass 1):
	// stmt.Constraints are parsed.
	// We need to convert ast.Constraints to typesystem.Constraints and add to TFunc.

	// Check VisitFunctionStatement logic.

	// Register constraints in the inference context
	for _, c := range n.Constraints {
		if len(c.Args) > 0 {
			// Resolve arguments for MPTC
			args := make([]typesystem.Type, len(c.Args))
			for i, argNode := range c.Args {
				// We use w.symbolTable to resolve types (TCon/TVar)
				args[i] = BuildType(argNode, w.symbolTable, &w.errors)
			}
			w.inferCtx.AddMPTCConstraint(c.TypeVar, c.Trait, args)

			// Register evidence for MPTC
			// Key: Trait[Arg1, Arg2, ...]
			// The first arg is c.TypeVar.
			fullArgs := make([]typesystem.Type, 0, 1+len(args))
			fullArgs = append(fullArgs, typesystem.TCon{Name: c.TypeVar})
			fullArgs = append(fullArgs, args...)
			key := GetEvidenceKey(c.Trait, fullArgs)

			// Witness Name: $dict_TypeVar_Trait_Arg1_Arg2...
			// Must match VisitFunctionStatement generation
			witnessName := "$dict_" + c.TypeVar + "_" + c.Trait
			for _, arg := range args {
				witnessName += "_" + arg.String()
			}
			w.symbolTable.RegisterEvidence(key, witnessName)

		} else {
			w.inferCtx.AddConstraint(c.TypeVar, c.Trait)

			// Register evidence for Single Param
			// Key: Trait[TypeVar]
			fullArgs := []typesystem.Type{typesystem.TCon{Name: c.TypeVar}}
			key := GetEvidenceKey(c.Trait, fullArgs)
			witnessName := "$dict_" + c.TypeVar + "_" + c.Trait
			w.symbolTable.RegisterEvidence(key, witnessName)
		}
	}

	// Look up function signature from Outer Scope (where it was registered in Headers pass)
	// This is critical to reuse the same type variables (TVars) for parameters
	// so that body analysis refines the signature in the symbol table.
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

	// Unwrap TForall to access TFunc structure
	if fnType != nil {
		if poly, ok := fnType.(typesystem.TForall); ok {
			fnType = poly.Type
		}
	}

	// Propagate expected return type to body with Rigid Type Constants
	if fnType != nil {
		if tFunc, ok := fnType.(typesystem.TFunc); ok {
			if w.inferCtx.ExpectedReturnTypes == nil {
				w.inferCtx.ExpectedReturnTypes = make(map[ast.Node]typesystem.Type)
			}
			rigidRetType := tFunc.ReturnType.Apply(rigidSubst)
			w.inferCtx.ExpectedReturnTypes[n.Body] = rigidRetType
		}
	}

	// Define parameters
	if n.Receiver != nil {
		if n.Receiver.Type != nil {
			recvType := BuildType(n.Receiver.Type, w.symbolTable, &w.errors)
			w.symbolTable.Define(n.Receiver.Name.Value, recvType, "")
			w.symbolTable.SetDefinitionFile(n.Receiver.Name.Value, w.currentFile)
		} else {
			w.symbolTable.Define(n.Receiver.Name.Value, w.freshVar(), "")
			w.symbolTable.SetDefinitionFile(n.Receiver.Name.Value, w.currentFile)
		}
	}

	fnTypeFunc, _ := fnType.(typesystem.TFunc)
	// Calculate offset for parameters in TFunc (if receiver is present in params)
	paramOffset := 0
	if n.Receiver != nil && n.Receiver.Type != nil {
		paramOffset = 1
	}

	for i, param := range n.Parameters {
		var paramType typesystem.Type
		if param.Type != nil {
			paramType = BuildType(param.Type, w.symbolTable, &w.errors)
		} else {
			// Try to reuse type from signature
			if fnType != nil && (i+paramOffset) < len(fnTypeFunc.Params) {
				paramType = fnTypeFunc.Params[i+paramOffset]
			} else {
				paramType = w.freshVar()
			}
		}

		if param.IsVariadic {
			paramType = typesystem.TApp{
				Constructor: typesystem.TCon{Name: config.ListTypeName},
				Args:        []typesystem.Type{paramType},
			}
		}
		// Don't define ignored parameters (_) in scope
		if !param.IsIgnored {
			w.symbolTable.DefineWithNode(param.Name.Value, paramType, "", param.Name)
			w.symbolTable.SetDefinitionFile(param.Name.Value, w.currentFile)

			// Map parameter AST node to its symbol in ResolutionMap for LSP
			if w.ResolutionMap != nil {
				if sym, found := w.symbolTable.Find(param.Name.Value); found {
					w.ResolutionMap[param.Name] = sym
				}
			}
		}
	}

	// Look up expected return type from Outer Scope
	var expectedRetType typesystem.Type

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

		w.pushReturnType(expectedRetType)
		defer w.popReturnType()

		// Set inFunctionBody flag to skip redundant expression inference during walk
		prevInFn := w.inFunctionBody
		// We set inFunctionBody to true to prevent VisitExpressionStatement from re-inferring expressions
		// that InferWithContext (called below on n.Body) will infer.
		// However, we still need to visit statements to populate the symbol table with local definitions
		// and check for naming conventions.
		// So VisitExpressionStatement MUST run, but it should return early after AST traversal/checks
		// before calling InferWithContext again.
		w.inFunctionBody = true

		n.Body.Accept(w)

		w.inFunctionBody = prevInFn

		w.markTailCalls(n.Body)
		w.inLoop = prevInLoop

		// Infer body type
		// Save pending witnesses from the outer scope (e.g. previous top-level expressions)
		// because we are about to re-infer the whole body and we want fresh witnesses
		// that match the fresh type variables generated in this pass.
		oldPending := w.inferCtx.PendingWitnesses
		w.inferCtx.PendingWitnesses = make([]PendingWitness, 0)
		// Restore pending witnesses when done with this body
		defer func() {
			w.inferCtx.PendingWitnesses = oldPending
		}()

		// Propagate expected return type to the body block for context-sensitive inference
		if expectedRetType != nil {
			if w.inferCtx.ExpectedReturnTypes == nil {
				w.inferCtx.ExpectedReturnTypes = make(map[ast.Node]typesystem.Type)
			}
			// Apply rigid substitution to ensure type parameters are treated as Rigid TCons
			// Also refresh any TypeVars in expectedRetType that resolve to TCons in the current scope
			// (e.g. instance parameters used in method return type)
			freeVars := expectedRetType.FreeTypeVariables()
			for _, v := range freeVars {
				if _, present := rigidSubst[v.Name]; !present {
					if resolved, ok := w.symbolTable.ResolveType(v.Name); ok {
						if tCon, ok := resolved.(typesystem.TCon); ok {
							rigidSubst[v.Name] = tCon
						}
					}
				}
			}

			if len(rigidSubst) > 0 {
				expectedRetType = expectedRetType.Apply(rigidSubst)
			}
			w.inferCtx.ExpectedReturnTypes[n.Body] = expectedRetType
		}

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

				// Update function symbol in outer scope with resolved return type
				// This is critical for inferred return types (where the symbol initially has a TVar)
				// to be propagated to callers or instance verifiers.
				if sym, ok := outer.Find(n.Name.Value); ok {
					// We need to apply substitution to the WHOLE type (including TForall if present)
					resolvedType := sym.Type.Apply(finalSubst)

					// Only update if it actually changed (optimization)
					if resolvedType.String() != sym.Type.String() {
						err := outer.Update(n.Name.Value, resolvedType)
						if err != nil {
							// Should not happen
							w.addError(diagnostics.NewError(diagnostics.ErrA003, n.Name.GetToken(), "failed to update function return type: "+err.Error()))
						}
					}
				}

				// Resolve pending witnesses
				ResolvePendingWitnesses(w.inferCtx, finalSubst, w.symbolTable, func(n ast.Node, err error) {
					w.addError(diagnostics.NewError(diagnostics.ErrA003, getNodeToken(n), "LOCAL RESOLVE: "+err.Error()))
				})

				// Process Inferred Constraints (Contextual Inference for Generics)
				if len(w.inferCtx.InferredConstraints) > 0 {
					// Update AST and SymbolTable
					for _, c := range w.inferCtx.InferredConstraints {
						// 1. Construct AST TypeConstraint
						var cArgs []ast.Type
						typeVarName := ""

						if len(c.Args) > 0 {
							if tv, ok := c.Args[0].(typesystem.TVar); ok {
								typeVarName = tv.Name
							} else if tc, ok := c.Args[0].(typesystem.TCon); ok {
								typeVarName = tc.Name
							}

							for _, arg := range c.Args[1:] {
								cArgs = append(cArgs, TypeToAST(arg))
							}
						}

						if typeVarName != "" {
							// Update AST
							n.Constraints = append(n.Constraints, &ast.TypeConstraint{
								TypeVar: typeVarName,
								Trait:   c.Trait,
								Args:    cArgs,
							})

							// 2. Add Witness Parameter
							witnessName := GetWitnessParamName(typeVarName, c.Trait)
							if len(c.Args) > 1 {
								for _, arg := range c.Args[1:] {
									witnessName += "_" + arg.String()
								}
							}
							n.WitnessParams = append(n.WitnessParams, witnessName)

							// Define in inner scope (w.symbolTable) so it is available during codegen/check
							w.symbolTable.DefineConstant(witnessName, typesystem.TCon{Name: "Dictionary"}, "")
						}
					}

					// 3. Update Function Signature in Outer Scope
					if fnSym, ok := outer.Find(n.Name.Value); ok {
						if tFunc, ok := fnSym.Type.(typesystem.TFunc); ok {
							// Append constraints to TFunc
							for _, ic := range w.inferCtx.InferredConstraints {
								var typeVar string
								if len(ic.Args) > 0 {
									if tv, ok := ic.Args[0].(typesystem.TVar); ok {
										typeVar = tv.Name
									} else if tc, ok := ic.Args[0].(typesystem.TCon); ok {
										typeVar = tc.Name
									}
								}
								if typeVar != "" {
									tFunc.Constraints = append(tFunc.Constraints, typesystem.Constraint{
										TypeVar: typeVar,
										Trait:   ic.Trait,
										Args:    ic.Args[1:], // Store only args after the first one (receiver)
									})
								}
							}
							fnSym.Type = tFunc
							err := outer.Update(fnSym.Name, fnSym.Type)
							if err != nil {
								// Should not happen if symbol was found
								w.addError(diagnostics.NewError(diagnostics.ErrA003, n.Name.GetToken(), "failed to update function symbol: "+err.Error()))
							}
						}
					}
				}

				// Apply substitution to function body nodes
				w.applySubstToNode(n.Body, finalSubst)

				// FIX: Remove bindings for generic type params to avoid replacing TVars with Rigid TCons in the signature
				for _, tp := range n.TypeParams {
					delete(finalSubst, tp.Value)
					// Also remove from GlobalSubst to prevent Generalize from picking them up
					delete(w.inferCtx.GlobalSubst, tp.Value)
				}

				// Resolve fnType (which contains params and return type)
				if tFunc, ok := fnType.(typesystem.TFunc); ok {
					resolvedFnType := tFunc.Apply(finalSubst)

					// Generalize the function type (Let Polymorphism for top-level/nested functions)
					resolvedFnType = w.inferCtx.Generalize(resolvedFnType, outer, n.Name.Value)

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

		// If we are inside a function body, skip redundant inference of individual expressions
		// because the whole body will be inferred together in analyzeFunctionBody.
		if w.inFunctionBody {
			return
		}

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
	w.symbolTable = symbols.NewEnclosedSymbolTable(outer, symbols.ScopeBlock)
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
	w.symbolTable = symbols.NewEnclosedSymbolTable(outer, symbols.ScopeBlock)
	defer func() { w.symbolTable = outer }()

	if n.Iterable != nil {
		// Iteration loop - traverse iterable and bind loop variable loosely.
		// Full type validation happens in inferForExpression.
		n.Iterable.Accept(w)
		w.symbolTable.Define(n.ItemName.Value, w.freshVar(), "")
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

		// If we are inside a function body, skip redundant expression inference during walk
		// because the whole body will be inferred together in analyzeFunctionBody.
		if w.inFunctionBody {
			return
		}

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

func (w *walker) VisitReturnStatement(n *ast.ReturnStatement) {
	if len(w.returnTypeStack) == 0 {
		w.addError(diagnostics.NewError(diagnostics.ErrA003, n.Token, "return statement outside of function"))
		return
	}

	var expectedRetType typesystem.Type
	if len(w.returnTypeStack) > 0 {
		expectedRetType = w.returnTypeStack[len(w.returnTypeStack)-1]
	}

	if n.Value != nil {
		n.Value.Accept(w)
	}

	// If we are inside a function body, still infer return expression for checking
	if n.Value != nil {
		if expectedRetType != nil && w.inferCtx != nil {
			if w.inferCtx.ExpectedReturnTypes == nil {
				w.inferCtx.ExpectedReturnTypes = make(map[ast.Node]typesystem.Type)
			}
			w.inferCtx.ExpectedReturnTypes[n.Value] = expectedRetType
		}

		t, s, err := InferWithContext(w.inferCtx, n.Value, w.symbolTable)
		if err != nil {
			w.appendError(n.Value, err)
			return
		}
		t = t.Apply(s)
		w.TypeMap[n.Value] = t

		if expectedRetType != nil {
			if _, err := typesystem.Unify(expectedRetType, t); err != nil {
				w.addError(diagnostics.NewError(
					diagnostics.ErrA003,
					n.Value.GetToken(),
					"return type mismatch: expected "+expectedRetType.String()+", got "+t.String(),
				))
			}
		}
	} else if expectedRetType != nil {
		if _, err := typesystem.Unify(expectedRetType, typesystem.Nil); err != nil {
			w.addError(diagnostics.NewError(
				diagnostics.ErrA003,
				n.Token,
				"return type mismatch: expected "+expectedRetType.String()+", got Nil",
			))
		}
	}
}

func (w *walker) pushReturnType(t typesystem.Type) {
	w.returnTypeStack = append(w.returnTypeStack, t)
}

func (w *walker) popReturnType() {
	if len(w.returnTypeStack) == 0 {
		return
	}
	w.returnTypeStack = w.returnTypeStack[:len(w.returnTypeStack)-1]
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
		w.symbolTable = symbols.NewEnclosedSymbolTable(outer, symbols.ScopeBlock)

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
