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

	// 0. Discover Implicit Generics (Case Sensitivity Rule)
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

	// Register Kind
	kind := typesystem.Star
	if len(stmt.TypeParameters) > 0 {
		kinds := make([]typesystem.Kind, len(stmt.TypeParameters)+1)
		for i := range stmt.TypeParameters {
			kinds[i] = typesystem.Star
		}
		kinds[len(stmt.TypeParameters)] = typesystem.Star
		kind = typesystem.MakeArrow(kinds...)
	}
	table.RegisterKind(stmt.Name.Value, kind)

	// Register the type name immediately with correct Kind
	tCon := typesystem.TCon{Name: stmt.Name.Value, KindVal: kind}
	table.DefineType(stmt.Name.Value, tCon, origin)

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

	// 1. Create temporary scope for type parameters
	typeScope := symbols.NewEnclosedSymbolTable(table, symbols.ScopeFunction) // Type definition scope behaves like function scope for type params
	for _, tp := range stmt.TypeParameters {
		typeScope.DefineType(tp.Value, typesystem.TVar{Name: tp.Value}, "")
	}

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
	} else {
		// ADT: Register constructors
		for _, c := range stmt.Constructors {
			var resultType typesystem.Type = typesystem.TCon{Name: stmt.Name.Value, KindVal: kind}
			if len(stmt.TypeParameters) > 0 {
				args := []typesystem.Type{}
				for _, tp := range stmt.TypeParameters {
					args = append(args, typesystem.TVar{Name: tp.Value})
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
			if _, err := typesystem.KindCheck(constructorType); err != nil {
				errors = append(errors, diagnostics.NewError(
					diagnostics.ErrA003,
					c.Name.GetToken(),
					"invalid constructor signature: "+err.Error(),
				))
			}

			table.DefineConstructor(c.Name.Value, constructorType, origin)
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

		// Special case: Single-constructor ADT wrapping a record
		// REMOVED: This optimization caused ADTs (like MyBox) to be treated as type aliases
		// by ResolveTypeAlias, breaking nominal typing in function signatures and pattern matching.
		// ADTs should be strict and require constructor usage.
		/*
			if len(stmt.Constructors) == 1 && len(stmt.TypeParameters) == 0 {
				c := stmt.Constructors[0]
				if len(c.Parameters) == 1 {
					// Build the parameter type
					paramType := BuildType(c.Parameters[0], typeScope, &errors)
					// If it's a record, update the TCon to include it as underlying type
					if _, ok := paramType.(typesystem.TRecord); ok {
						// Get the current symbol and update it
						if sym, ok := table.Find(stmt.Name.Value); ok && sym.Kind == symbols.TypeSymbol {
							if tCon, ok := sym.Type.(typesystem.TCon); ok {
								tCon.UnderlyingType = paramType
								// Re-define with updated TCon
								table.DefineType(stmt.Name.Value, tCon, origin)
							}
						}
					}
				}
			}
		*/

		// Fallback for Nominal Records (type Node = { ... } where IsAlias=false)
		// Ensure UnderlyingType is set so Unify can verify structural compatibility
		if len(stmt.Constructors) == 0 && stmt.TargetType != nil {
			realType := BuildType(stmt.TargetType, typeScope, &errors)

			// Kind Check for nominal types
			if _, err := typesystem.KindCheck(realType); err != nil {
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

	// 0.1. Check for redefinition of existing trait (including built-ins)
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

	// 1. Extract super trait names and verify they exist
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

	// 2. Register Trait with type params and super traits
	typeParamNames := make([]string, len(n.TypeParams))
	for i, tp := range n.TypeParams {
		typeParamNames[i] = tp.Value
	}
	w.symbolTable.DefineTrait(n.Name.Value, typeParamNames, superTraitNames, w.currentModuleName)

	// 3. Register methods
	// Methods are generic functions where the TypeParam is the trait variable.
	// e.g. show(val: a) -> String. 'a' is bound to the trait param.

	// We need a scope for the trait definition to define 'a'
	outer := w.symbolTable
	w.symbolTable = symbols.NewEnclosedSymbolTable(outer, symbols.ScopeFunction) // Trait definition scope behaves like function scope for type params
	defer func() { w.symbolTable = outer }()

	// Define the type variables with inferred Kinds
	var traitKind typesystem.Kind
	for i, tp := range n.TypeParams {
		// Infer Kind from usage in signatures
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

		kind := typesystem.Star
		if maxArgs > 0 {
			kinds := make([]typesystem.Kind, maxArgs+1)
			for i := 0; i <= maxArgs; i++ {
				kinds[i] = typesystem.Star
			}
			kind = typesystem.MakeArrow(kinds...)
		}

		w.symbolTable.DefineType(tp.Value, typesystem.TVar{Name: tp.Value, KindVal: kind}, "")
		// Also register Kind in table for GetKind lookups
		w.symbolTable.RegisterKind(tp.Value, kind)

		// Register kind specifically for this trait parameter (for MPTC checks)
		// NOTE: w.symbolTable is the inner scope, but we need to register it in the outer (trait declaration) scope
		// OR better: register it in the outer scope which is where DefineTrait was called.
		// However, the `outer` variable in this function refers to the scope where trait is defined.
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
		// Construct function type
		var retType typesystem.Type
		if method.ReturnType != nil {
			retType = BuildType(method.ReturnType, w.symbolTable, &w.errors)
		} else {
			retType = typesystem.Nil
		}

		var params []typesystem.Type
		for _, p := range method.Parameters {
			if p.Type != nil {
				params = append(params, BuildType(p.Type, w.symbolTable, &w.errors))
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

		// Register in Global Scope (outer) so it can be called
		// And associate with Trait
		outer.RegisterTraitMethod(method.Name.Value, n.Name.Value, methodType, w.currentModuleName)

		// 2.1 Trait Method Indexing
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
