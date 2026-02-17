package analyzer

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

func RegisterTypeDeclaration(stmt *ast.TypeDeclarationStatement, table *symbols.SymbolTable, origin string) []*diagnostics.DiagnosticError {
	var errors []*diagnostics.DiagnosticError
	if stmt == nil || stmt.Name == nil {
		return errors
	}

	// Check for shadowing, but allow overwriting Pending symbols (forward declarations)
	if table.IsDefined(stmt.Name.Value) {
		sym, ok := table.Find(stmt.Name.Value)
		if ok && !sym.IsPending {
			errors = append(errors, diagnostics.NewError(
				diagnostics.ErrA004,
				stmt.Name.GetToken(),
				stmt.Name.Value,
			))
			return errors
		}
		// If it is pending, we proceed to overwrite it
	}

	// Discover Implicit Generics (Case Sensitivity Rule)
	// If no explicit type parameters are provided, scan the definition for lowercase identifiers
	// that are not defined in the scope. These are implicit type variables.
	if len(stmt.TypeParameters) == 0 {
		implicitParams := CollectImplicitGenerics(stmt, table)
		if len(implicitParams) > 0 {
			for _, name := range implicitParams {
				stmt.TypeParameters = append(stmt.TypeParameters, &ast.Identifier{
					Token: stmt.Name.Token, // Fallback token
					Value: name,
				})
			}
		}
	}

	// Infer Kinds for Type Parameters using explicit annotations or fresh KVars
	// This replaces hardcoded Star logic
	kindContext := typesystem.NewKindContext()
	var paramKinds []typesystem.Kind

	for _, tp := range stmt.TypeParameters {
		// If AST has explicit kind, use it.
		// Otherwise create fresh KVar.
		var kVar typesystem.Kind
		if tp.Kind != nil {
			kVar = tp.Kind
		} else {
			kVar = kindContext.FreshKVar()
		}
		kindContext.KindVars[tp.Value] = kVar
		paramKinds = append(paramKinds, kVar)
	}

	// Construct result kind: kParam1 -> kParam2 -> ... -> KRet
	// KRet is usually Star for type constructors, but HKT definitions (like type Functor f = ...) are tricky.
	// For ADT/Alias definitions `type T<Params> = Body`, the result kind is Star if it constructs a concrete type.
	resultKind := typesystem.Star

	// Infer Kind from Body to resolve KVars
	var bodyType typesystem.Type
	// Create temporary scope for type parameters to build body type
	typeScope := symbols.NewEnclosedSymbolTable(table, symbols.ScopeFunction)
	for i, tp := range stmt.TypeParameters {
		// Define TVar with the KVar we assigned
		typeScope.DefineType(tp.Value, typesystem.TVar{Name: tp.Value, KindVal: paramKinds[i]}, "")
		typeScope.RegisterKind(tp.Value, paramKinds[i])
	}

	// Register the type itself in the temporary scope with a provisional kind (assuming result is Star).
	// This is crucial for recursive types (e.g. HFix) to refer to themselves with the correct kind structure.
	// Note: For aliases, the result kind might eventually be different, but for recursive references,
	// assuming the structure (Params -> Star) is usually sufficient/required for ADTs.
	provisionalSelfKind := typesystem.MakeArrow(append(paramKinds, typesystem.Star)...)
	tConSelf := typesystem.TCon{Name: stmt.Name.Value, KindVal: provisionalSelfKind}
	typeScope.DefineType(stmt.Name.Value, tConSelf, origin)
	typeScope.RegisterKind(stmt.Name.Value, provisionalSelfKind)

	if stmt.IsAlias {
		if stmt.TargetType != nil {
			bodyType = BuildType(stmt.TargetType, typeScope, &errors)
		}
	} else {
		// For ADTs, we analyze constructors to infer kinds of parameters
		// E.g. type Box<f> = MkBox(f<Int>) implies f: * -> *
		// We construct a synthetic tuple type of all constructor arguments to infer kinds
		var allParams []typesystem.Type
		for _, c := range stmt.Constructors {
			for _, p := range c.Parameters {
				allParams = append(allParams, BuildType(p, typeScope, &errors))
			}
		}
		if len(allParams) > 0 {
			bodyType = typesystem.TTuple{Elements: allParams}
		}
	}

	// Run Kind Inference on Body
	var subst typesystem.KindSubst
	if bodyType != nil {
		kBody, s, err := typesystem.InferKind(bodyType, kindContext)
		if err != nil {
			errors = append(errors, diagnostics.NewError(
				diagnostics.ErrA003,
				stmt.Name.GetToken(),
				"Kind inference error: "+err.Error(),
			))
		} else {
			// For aliases, update resultKind if it differs from Star
			if stmt.IsAlias {
				resultKind = kBody
			}
		}
		subst = s
	} else {
		subst = make(typesystem.KindSubst)
	}

	// Register the type name immediately with correct Kind
	// If resultKind was updated, we need to update the kind.
	// But 'kind' variable was calculated using old resultKind (Star).
	// Recalculate kind using resultKind (which might be updated kBody).

	// Apply substitution to paramKinds first
	finalKinds := make([]typesystem.Kind, len(paramKinds))
	for i, k := range paramKinds {
		resolved := typesystem.ApplyKindSubst(subst, k)
		if _, isVar := resolved.(typesystem.KVar); isVar {
			resolved = typesystem.Star
		}
		finalKinds[i] = resolved
	}

	// Update resultKind with substitution too
	resultKind = typesystem.ApplyKindSubst(subst, resultKind)

	kind := typesystem.MakeArrow(append(finalKinds, resultKind)...)

	table.RegisterKind(stmt.Name.Value, kind)

	tCon := typesystem.TCon{Name: stmt.Name.Value, KindVal: kind}
	table.DefineType(stmt.Name.Value, tCon, origin)
	table.SetDefinitionNode(stmt.Name.Value, stmt.Name)

	// Update constructors with finalized TCon (to propagate correct KindVal)
	if !stmt.IsAlias {
		for _, c := range stmt.Constructors {
			if sym, ok := table.Find(c.Name.Value); ok && sym.Kind == symbols.ConstructorSymbol {
				// Reconstruct result type with new TCon
				var resultType typesystem.Type = tCon
				if len(stmt.TypeParameters) > 0 {
					args := []typesystem.Type{}
					for _, tp := range stmt.TypeParameters {
						// We need TVars with finalized kinds
						k, _ := typeScope.GetKind(tp.Value)
						args = append(args, typesystem.TVar{Name: tp.Value, KindVal: k})
					}
					resultType = typesystem.TApp{Constructor: resultType, Args: args}
				}

				// Reconstruct constructor type
				var constructorType typesystem.Type
				if len(c.Parameters) > 0 {
					// Update the constructor's return type to use the finalized Type Constructor (TCon)
					// which contains the correct Kind information. We also update the parameter types to ensure consistency.
					if tFunc, ok := sym.Type.(typesystem.TFunc); ok {
						tFunc.ReturnType = resultType
						// Also replace old TCon in parameters
						newParams := make([]typesystem.Type, len(tFunc.Params))
						for i, p := range tFunc.Params {
							newParams[i] = typesystem.ReplaceTCon(p, stmt.Name.Value, tCon)
						}
						tFunc.Params = newParams
						constructorType = tFunc
					} else {
						// Not a function? Should not happen if params > 0
						constructorType = resultType
					}
				} else {
					constructorType = resultType
				}

				// Update symbol table
				table.DefineConstructor(c.Name.Value, constructorType, origin)
			}
		}
	}

	// Register type parameters
	if len(stmt.TypeParameters) > 0 {
		params := make([]string, len(stmt.TypeParameters))
		for i, p := range stmt.TypeParameters {
			params[i] = p.Value
		}
		table.RegisterTypeParams(stmt.Name.Value, params)
		// Update local tCon so subsequent DefineTypeAlias uses one with params
		tCon.TypeParams = &params
	}

	// Ensure typeScope has updated resolved kinds for body processing
	for i, tp := range stmt.TypeParameters {
		typeScope.DefineType(tp.Value, typesystem.TVar{Name: tp.Value, KindVal: finalKinds[i]}, "")
		typeScope.RegisterKind(tp.Value, finalKinds[i])
	}
	typeScope.DefineType(stmt.Name.Value, tCon, origin)

	if stmt.IsAlias {
		if stmt.TargetType == nil {
			return errors
		}
		// Validate target type
		if nt, ok := stmt.TargetType.(*ast.NamedType); ok {
			if !table.IsDefined(nt.Name.Value) && !typeScope.IsDefined(nt.Name.Value) {
				errors = append(errors, diagnostics.NewError(
					diagnostics.ErrA002,
					nt.GetToken(),
					nt.Name.Value,
				))
			}
		}

		// Use typeScope to build the type
		realType := BuildType(stmt.TargetType, typeScope, &errors)
		// Use DefineTypeAlias: TCon for trait lookup, realType for field access/unification
		table.DefineTypeAlias(stmt.Name.Value, tCon, realType, origin)
		table.SetDefinitionNode(stmt.Name.Value, stmt.Name) // Re-set definition node after alias redefinition
	} else {
		// ADT: Register constructors
		for _, c := range stmt.Constructors {
			var resultType typesystem.Type = typesystem.TCon{Name: stmt.Name.Value, KindVal: kind}
			if len(stmt.TypeParameters) > 0 {
				args := []typesystem.Type{}
				for _, tp := range stmt.TypeParameters {
					k, _ := typeScope.GetKind(tp.Value)
					args = append(args, typesystem.TVar{Name: tp.Value, KindVal: k})
				}
				resultType = typesystem.TApp{Constructor: resultType, Args: args}
			}

			var constructorType typesystem.Type

			if len(c.Parameters) > 0 {
				var params []typesystem.Type
				for _, p := range c.Parameters {
					// Use typeScope to resolve type parameters
					params = append(params, BuildType(p, typeScope, &errors))
				}
				constructorType = typesystem.TFunc{
					Params:     params,
					ReturnType: resultType,
				}
			} else {
				constructorType = resultType
			}

			if table.IsDefined(c.Name.Value) {
				sym, _ := table.Find(c.Name.Value)
				// Allow constructor to have same name as the type (common in ADTs)
				// This relies on the SymbolTable handling the definition (e.g. upgrading to TypeAndConstructor or separate namespaces)
				// or the language treating them as separate based on context.
				isSelfType := c.Name.Value == stmt.Name.Value && sym.Kind == symbols.TypeSymbol

				if !isSelfType {
					errors = append(errors, diagnostics.NewError(
						diagnostics.ErrA004,
						c.Name.GetToken(),
						c.Name.Value,
					))
					continue
				}
			}

			// Kind Check: Constructor must be well-kinded (arguments must be types)
			if _, _, err := typesystem.InferKind(constructorType, kindContext); err != nil {
				errors = append(errors, diagnostics.NewError(
					diagnostics.ErrA003,
					c.Name.GetToken(),
					"invalid constructor signature: "+err.Error(),
				))
			}

			table.DefineConstructor(c.Name.Value, constructorType, origin)
			table.SetDefinitionNode(c.Name.Value, c.Name)
			table.RegisterVariant(stmt.Name.Value, c.Name.Value)

			for _, p := range c.Parameters {
				if nt, ok := p.(*ast.NamedType); ok {
					name := nt.Name.Value
					if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
						// Check if defined in typeScope (which includes global via outer)
						if !typeScope.IsDefined(name) {
							errors = append(errors, diagnostics.NewError(
								diagnostics.ErrA002,
								nt.GetToken(),
								name,
							))
						}
					}
				}
			}
		}

		// Fallback for Nominal Records (type Node = { ... } where IsAlias=false)
		// Ensure UnderlyingType is set so Unify can verify structural compatibility
		if len(stmt.Constructors) == 0 && stmt.TargetType != nil {
			realType := BuildType(stmt.TargetType, typeScope, &errors)

			// Kind Check for nominal types
			if _, _, err := typesystem.InferKind(realType, kindContext); err != nil {
				errors = append(errors, diagnostics.NewError(
					diagnostics.ErrA003,
					stmt.Name.GetToken(),
					"invalid type definition: "+err.Error(),
				))
			}

			if sym, ok := table.Find(stmt.Name.Value); ok && sym.Kind == symbols.TypeSymbol {
				if tCon, ok := sym.Type.(typesystem.TCon); ok {
					tCon.UnderlyingType = realType
					table.DefineType(stmt.Name.Value, tCon, origin)
				}
			}
		}
	}
	return errors
}

func (w *walker) VisitTypeDeclarationStatement(stmt *ast.TypeDeclarationStatement) {
	if w.mode == ModeNaming || w.mode == ModeInstances {
		return
	}

	// Handle local type declarations vs top-level
	// In ModeHeaders: Process ONLY global types (top-level)
	// In ModeBodies: Process ONLY local types (inside functions)
	// In ModeFull: Process ALL
	isGlobal := w.symbolTable.IsGlobalScope()

	if w.mode == ModeHeaders && !isGlobal {
		return // Should not happen usually as Headers pass doesn't enter bodies
	}
	if w.mode == ModeBodies && isGlobal {
		return // Skip top-level types in Bodies pass (already done in Headers)
	}

	// Use RegisterTypeDeclaration to register and get errors
	errs := RegisterTypeDeclaration(stmt, w.symbolTable, w.currentModuleName)
	w.addErrors(errs)
}

func (w *walker) VisitTraitDeclaration(n *ast.TraitDeclaration) {
	// Skip if trait was not properly parsed
	if n == nil || n.Name == nil {
		return
	}

	// Mode Checks: Only process in ModeHeaders or ModeFull
	if w.mode == ModeNaming || w.mode == ModeInstances || w.mode == ModeBodies {
		return
	}

	// Check for redefinition of existing trait (including built-ins)
	if sym, ok := w.symbolTable.Find(n.Name.Value); ok && sym.Kind == symbols.TraitSymbol {
		if !sym.IsPending {
			w.addError(diagnostics.NewError(
				diagnostics.ErrA004,
				n.Token,
				n.Name.Value,
			))
			return
		}
	}

	// Extract super trait names and verify they exist
	superTraitNames := make([]string, 0, len(n.SuperTraits))
	for _, st := range n.SuperTraits {
		var superName string
		if nt, ok := st.(*ast.NamedType); ok {
			superName = nt.Name.Value
		}
		if superName != "" {
			// Check that super trait exists
			if !w.symbolTable.IsDefined(superName) {
				w.addError(diagnostics.NewError(
					diagnostics.ErrA006,
					n.Token,
					superName,
				))
			} else {
				superTraitNames = append(superTraitNames, superName)
			}
		}
	}

	// Register Trait with type params and super traits
	typeParamNames := make([]string, len(n.TypeParams))
	for i, tp := range n.TypeParams {
		typeParamNames[i] = tp.Value
	}
	w.symbolTable.DefineTrait(n.Name.Value, typeParamNames, superTraitNames, w.currentModuleName)

	// Register Functional Dependencies
	if len(n.Dependencies) > 0 {
		for _, dep := range n.Dependencies {
			// Validate From vars
			for _, from := range dep.From {
				found := false
				for _, tp := range typeParamNames {
					if tp == from {
						found = true
						break
					}
				}
				if !found {
					w.addError(diagnostics.NewError(
						diagnostics.ErrA003,
						n.Token,
						fmt.Sprintf("unknown type variable '%s' in functional dependency", from),
					))
				}
			}
			// Validate To vars
			for _, to := range dep.To {
				found := false
				for _, tp := range typeParamNames {
					if tp == to {
						found = true
						break
					}
				}
				if !found {
					w.addError(diagnostics.NewError(
						diagnostics.ErrA003,
						n.Token,
						fmt.Sprintf("unknown type variable '%s' in functional dependency", to),
					))
				}
			}
		}
		w.symbolTable.RegisterTraitFunctionalDependencies(n.Name.Value, n.Dependencies)
	}

	// Register methods
	// Methods are generic functions where the TypeParam is the trait variable.
	// e.g. show(val: a) -> String. 'a' is bound to the trait param.

	// We need a scope for the trait definition to define 'a'
	outer := w.symbolTable
	w.symbolTable = symbols.NewEnclosedSymbolTable(outer, symbols.ScopeFunction) // Trait definition scope behaves like function scope for type params
	defer func() { w.symbolTable = outer }()

	// Define the type variables with inferred Kinds
	var traitKind typesystem.Kind
	var traitTypeVars []typesystem.TVar
	for i, tp := range n.TypeParams {
		var kind typesystem.Kind

		// 1. Use explicit kind annotation if available
		if tp.Kind != nil {
			kind = tp.Kind
		} else {
			// 2. Infer Kind from usage in signatures
			maxArgs := 0
			for _, sig := range n.Signatures {
				if c := findMaxTypeArgs(tp.Value, sig.ReturnType); c > maxArgs {
					maxArgs = c
				}
				for _, param := range sig.Parameters {
					if c := findMaxTypeArgs(tp.Value, param.Type); c > maxArgs {
						maxArgs = c
					}
				}
			}

			if maxArgs > 0 {
				kinds := make([]typesystem.Kind, maxArgs+1)
				for i := 0; i <= maxArgs; i++ {
					kinds[i] = typesystem.Star
				}
				kind = typesystem.MakeArrow(kinds...)
			} else {
				kind = typesystem.Star
			}
		}

		tv := typesystem.TVar{Name: tp.Value, KindVal: kind}
		traitTypeVars = append(traitTypeVars, tv)

		w.symbolTable.DefineType(tp.Value, tv, "")
		// Also register Kind in table for GetKind lookups
		w.symbolTable.RegisterKind(tp.Value, kind)

		// Register kind specifically for this trait parameter (for MPTC checks)
		outer.RegisterTraitTypeParamKind(n.Name.Value, tp.Value, kind)

		if i == 0 {
			traitKind = kind
		}
	}

	// Register the Kind of the Trait itself (based on its first type parameter)
	// This allows instance checks to verify the kind of the type being instantiated.
	if traitKind != nil {
		outer.RegisterKind(n.Name.Value, traitKind)
	}

	for _, method := range n.Signatures {
		// Create a temporary scope for method type parameters
		methodScope := symbols.NewEnclosedSymbolTable(w.symbolTable, symbols.ScopeFunction)

		// Register explicit type parameters of the method
		var methodTypeVars []typesystem.TVar
		if len(method.TypeParams) > 0 {
			for _, tp := range method.TypeParams {
				var kind typesystem.Kind = typesystem.Star
				if tp.Kind != nil {
					kind = tp.Kind
				}
				tv := typesystem.TVar{Name: tp.Value, KindVal: kind}
				methodScope.DefineType(tp.Value, tv, "")
				methodScope.RegisterKind(tp.Value, kind)
				methodTypeVars = append(methodTypeVars, tv)
			}
		}

		// Construct function type using the method scope
		var retType typesystem.Type
		if method.ReturnType != nil {
			retType = BuildType(method.ReturnType, methodScope, &w.errors)
		} else {
			retType = typesystem.Nil
		}

		var params []typesystem.Type
		for _, p := range method.Parameters {
			if p.Type != nil {
				params = append(params, BuildType(p.Type, methodScope, &w.errors))
			} else {
				// Error: method signature usually requires types
				params = append(params, w.freshVar())
			}
		}

		methodType := typesystem.TFunc{
			Params:     params,
			ReturnType: retType,
		}

		// Add Implicit Trait Constraint to the method signature
		// This ensures that calling the method requires satisfying the trait
		if len(typeParamNames) > 0 {
			var constraintArgs []typesystem.Type
			for i := 1; i < len(typeParamNames); i++ {
				constraintArgs = append(constraintArgs, typesystem.TVar{Name: typeParamNames[i]})
			}
			methodType.Constraints = append(methodType.Constraints, typesystem.Constraint{
				TypeVar: typeParamNames[0],
				Trait:   n.Name.Value,
				Args:    constraintArgs,
			})
		}

		// Collect Implicit Generics from methodScope
		// BuildType populates methodScope with any new TVars encountered (that weren't explicit)
		// We need to add these to the methodTypeVars list so they are quantified in TForall.
		existingVars := make(map[string]bool)
		for _, v := range methodTypeVars {
			existingVars[v.Name] = true
		}

		// Get all symbols from methodScope, check if they are TVars and new
		for name, sym := range methodScope.All() {
			if sym.Kind == symbols.TypeSymbol {
				if tv, ok := sym.Type.(typesystem.TVar); ok {
					if !existingVars[name] {
						// Found implicit generic!
						methodTypeVars = append(methodTypeVars, tv)
						existingVars[name] = true
					}
				}
			}
		}

		// Wrap in TForall if method has generics
		var finalMethodType typesystem.Type = methodType
		if len(methodTypeVars) > 0 || len(traitTypeVars) > 0 {
			// Combine trait vars and method vars
			var allVars []typesystem.TVar
			allVars = append(allVars, traitTypeVars...)
			allVars = append(allVars, methodTypeVars...)

			// Collect constraints from method type params (e.g. <T: Show>)
			var constraints []typesystem.Constraint
			for _, tp := range method.TypeParams {
				for _, c := range tp.Constraints {
					args := []typesystem.Type{}
					for _, arg := range c.Args {
						args = append(args, BuildType(arg, methodScope, &w.errors))
					}
					constraints = append(constraints, typesystem.Constraint{
						TypeVar: tp.Value,
						Trait:   c.Trait,
						Args:    args,
					})
				}
			}

			finalMethodType = typesystem.TForall{
				Vars:        allVars,
				Constraints: constraints,
				Type:        methodType,
			}
		}

		// Check for collision with existing symbols in the outer scope
		// Trait methods become global functions, so they must not conflict with existing globals or prelude
		if sym, ok := outer.Find(method.Name.Value); ok {
			// Check if the existing symbol is a trait method (allow overriding)
			_, isTraitMethod := outer.GetTraitForMethod(method.Name.Value)

			// If it's not pending (forward decl) and NOT a trait method, it's a conflict
			if !sym.IsPending && !isTraitMethod {
				w.addError(diagnostics.NewError(
					diagnostics.ErrA004,
					method.Token,
					fmt.Sprintf("redefinition of symbol: '%s'", method.Name.Value),
				))
				continue
			}
		}

		// Calculate Dispatch Strategy
		// Determine where to find each type parameter (A, B, ...) in the method signature
		var dispatchSources []typesystem.DispatchSource
		for _, typeParam := range typeParamNames {
			source := typesystem.DispatchSource{Kind: typesystem.DispatchHint, Index: -1}

			// 1. Check arguments
			for i, paramType := range methodType.Params {
				if containsTypeVar(paramType, typeParam) {
					source = typesystem.DispatchSource{Kind: typesystem.DispatchArg, Index: i}
					break
				}
			}

			// 2. Check return type if not found in args
			if source.Kind == typesystem.DispatchHint {
				if containsTypeVar(methodType.ReturnType, typeParam) {
					source = typesystem.DispatchSource{Kind: typesystem.DispatchReturn, Index: -1}
				}
			}

			dispatchSources = append(dispatchSources, source)
		}

		// Register in Global Scope (outer) so it can be called
		// And associate with Trait
		outer.RegisterTraitMethod(method.Name.Value, n.Name.Value, finalMethodType, w.currentModuleName)

		// Register Dispatch Strategy in SymbolTable (to be used by Evaluator/VM)
		outer.RegisterTraitMethodDispatch(n.Name.Value, method.Name.Value, dispatchSources)

		// Trait Method Indexing
		// Assign unique index to method for O(1) vtable lookup
		outer.RegisterTraitMethodIndex(n.Name.Value, method.Name.Value, len(outer.TraitMethodIndices[n.Name.Value]))

		// Register method name in trait's method list
		outer.RegisterTraitMethod2(n.Name.Value, method.Name.Value)

		// If this is an operator method, register the operator -> trait mapping
		if method.Operator != "" {
			// Check if operator is already defined in another trait
			if existingTrait, exists := outer.GetTraitForOperator(method.Operator); exists {
				w.addError(diagnostics.NewError(
					diagnostics.ErrA004,
					method.Token,
					"operator "+method.Operator+" (already defined in trait "+existingTrait+")",
				))
			} else {
				outer.RegisterOperatorTrait(method.Operator, n.Name.Value)
			}
		}

		// If method has a body, it's a default implementation
		if method.Body != nil {
			outer.RegisterTraitDefaultMethod(n.Name.Value, method.Name.Value)
			// Store the function for evaluator
			key := n.Name.Value + "." + method.Name.Value
			w.TraitDefaults[key] = method
		}
	}
}

func (w *walker) VisitForallType(n *ast.ForallType) {
	n.Type.Accept(w)
}
