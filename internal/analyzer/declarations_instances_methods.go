package analyzer

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

func specializePolytype(t typesystem.Type, subst typesystem.Subst) typesystem.Type {
	if poly, ok := t.(typesystem.TForall); ok {
		// Identify which quantified variables are being substituted
		newVars := []typesystem.TVar{}
		for _, v := range poly.Vars {
			if _, present := subst[v.Name]; !present {
				newVars = append(newVars, v)
			}
		}

		// Instantiate the quantified type by applying the substitution to its body.
		// We manually handle the body and constraints to avoid TForall.Apply's shadowing logic,
		// as we are explicitly instantiating specific quantified variables.

		newType := poly.Type.Apply(subst)

		var newConstraints []typesystem.Constraint
		for _, c := range poly.Constraints {
			// Apply subst to constraint args
			newArgs := make([]typesystem.Type, len(c.Args))
			for i, arg := range c.Args {
				newArgs[i] = arg.Apply(subst)
			}
			// Apply to TypeVar
			newTv := c.TypeVar
			if s, ok := subst[c.TypeVar]; ok {
				if tv, ok := s.(typesystem.TVar); ok {
					newTv = tv.Name
				}
				// If a constrained type variable is substituted with a concrete type,
				// the constraint is satisfied by the concrete type's instance (verified elsewhere).
				// We remove the constraint from the signature as it no longer applies to a variable.
			}

			// Keep constraint only if it refers to remaining quantified vars
			// or if we want to propagate checks.
			// Usually, if we specialize `f` -> `Box`, we drop `f: UserFunctor` constraint from signature.
			if _, present := subst[c.TypeVar]; !present {
				newConstraints = append(newConstraints, typesystem.Constraint{
					TypeVar: newTv,
					Trait:   c.Trait,
					Args:    newArgs,
				})
			}
		}

		if len(newVars) == 0 {
			// Fully instantiated
			return newType
		}

		return typesystem.TForall{
			Vars:        newVars,
			Constraints: newConstraints,
			Type:        newType,
		}
	}
	return t.Apply(subst)
}

// visitInstanceMethods verifies method signatures and generates dictionary code
func (w *walker) visitInstanceMethods(n *ast.InstanceDeclaration, traitName, typeName string, instanceArgs []typesystem.Type, requirements []typesystem.Constraint, subst typesystem.Subst, outer *symbols.SymbolTable) {
	for _, method := range n.Methods {
		if method == nil {
			continue
		}

		// Save original name and mangle locally to avoid shadowing global trait method
		// inside the method body.
		var originalName string
		var mangledName string
		if method.Name != nil {
			originalName = method.Name.Value
			mangledName = "$local_" + originalName
			method.Name.Value = mangledName
		}

		// Verify signature matches Trait definition
		// Find generic signature
		genericSymbol, ok := w.symbolTable.Find(originalName)
		if !ok {
			// Method not in trait?
			w.addError(diagnostics.NewError(
				diagnostics.ErrA001,
				method.Name.GetToken(),
				"method "+originalName+" is not part of trait "+traitName,
			))
			continue
		}

		traitForMethod, _ := w.symbolTable.GetTraitForMethod(originalName)
		if traitForMethod != traitName {
			w.addError(diagnostics.NewError(
				diagnostics.ErrA003,
				method.Name.GetToken(),
				"method "+originalName+" belongs to trait "+traitForMethod+", not "+traitName,
			))
			continue
		}

		// Create Expected Type (Generic signature with substitution)
		// We need to specialize the polytype by applying the substitution to quantified variables if they match trait params.
		// e.g. forall f a. ... with subst {f: Box} -> forall a. ... (where f is replaced by Box)
		expectedType := specializePolytype(genericSymbol.Type, subst)

		// Inject expected types into AST parameters if missing
		// This helps VisitFunctionStatement to correctly infer parameter types
		// from the instance signature, avoiding "forall" generalization errors.
		// Handle TForall by stripping quantifier to get the underlying TFunc
		effectiveExpectedType := expectedType
		if poly, ok := expectedType.(typesystem.TForall); ok {
			effectiveExpectedType = poly.Type
		}

		if funcType, ok := effectiveExpectedType.(typesystem.TFunc); ok {
			for i, p := range method.Parameters {
				if p.Type == nil && i < len(funcType.Params) {
					// Use TypeToAST for full recursive type reconstruction
					p.Type = TypeToAST(funcType.Params[i])
					// Preserve token if possible
					if nt, ok := p.Type.(*ast.NamedType); ok && nt.Name != nil {
						nt.Name.Token = p.Token
					}
				}
			}
		}

		method.Accept(w)

		// Restore original name
		if method.Name != nil {
			method.Name.Value = originalName
		}

		// Skip operator methods (method.Name is nil for operators)
		if method.Name == nil {
			continue
		}

		// Get Actual Type from the method definition in CURRENT scope
		// VisitFunctionStatement (called by method.Accept(w)) defined it in w.symbolTable (the inner scope).
		actualSymbol, ok := w.symbolTable.Find(mangledName)
		if !ok {
			// Should not happen if VisitFunctionStatement works
			continue
		}
		actualType := actualSymbol.Type

		// Unify
		// If ModeInstances (Pass 3), skip verification if the actual type relies on inference (e.g. returns TVar/t?).
		// We will verify again in ModeBodies (Pass 4) after body analysis updates the type.
		if w.mode == ModeInstances && isIncompleteType(actualType) {
			// Skip check
		} else {
			// If the actual method type is polymorphic (Rank-N), it means the method implementation
			// is generic. We need to check if this generic implementation satisfies the required
			// instance signature.
			// If Expected is TForall, we compare them as polytypes (alpha-equivalence).
			// If Expected is Monotype, we instantiate Actual to see if it can specialize to Expected.
			actualToCheck := actualType
			if _, expectedIsPoly := expectedType.(typesystem.TForall); !expectedIsPoly {
				if poly, ok := actualType.(typesystem.TForall); ok {
					actualToCheck = InstantiateForall(w.inferCtx, poly)
				}
			} else {
				// If the expected type is polymorphic (TForall) but the actual type appears monomorphic,
				// it may be due to the method being defined in a context where its type variables are in scope but not quantified yet.
				// We attempt to generalize the actual type by quantifying over its free variables to match the expected polymorphism.
				if _, actualIsPoly := actualType.(typesystem.TForall); !actualIsPoly {
					// Construct TForall from actualType if it has free variables matching expected
					freeVars := actualType.FreeTypeVariables()
					if len(freeVars) > 0 {
						actualToCheck = typesystem.TForall{
							Vars: freeVars,
							Type: actualType,
						}
					}
				}
			}

			_, err := typesystem.Unify(expectedType, actualToCheck)
			if err != nil {
				w.addError(diagnostics.NewError(
					diagnostics.ErrA003,
					method.Name.GetToken(),
					"method signature mismatch: expected "+expectedType.String()+", got "+actualType.String(),
				))
			} else {
				// Register instance method signature for use in type inference
				// This allows traits like Optional to correctly extract inner types
				// for any user-defined type, not just built-in types.
				// We flatten the type to ensure TApp(TApp(C, args1), args2) becomes TApp(C, args1+args2)
				// This is critical for Unify to work correctly with instance signatures like Validated<e, a>.
				flattenedExpected := flattenType(expectedType)
				outer.RegisterInstanceMethod(traitName, typeName, method.Name.Value, flattenedExpected)
			}
		}
	}

	// Dictionary Generation (Static & Constructors)
	// Only generate dictionaries in ModeInstances or ModeFull to avoid duplication/premature generation
	// UNLESS it is a local instance (ModeBodies + inFunctionBody), which is only visited in ModeBodies.
	if (w.mode != ModeInstances && w.mode != ModeFull) && !w.inFunctionBody {
		return
	}

	// Check if this is a generic instance (has type params)
	isGenericInstance := len(n.TypeParams) > 0

	if isGenericInstance {
		// Generic Instance: instance Show List<T>
		// Generate Constructor Function: $ctor_Show_List
		// fun $ctor_Show_List(dict_T_Show) -> Dictionary { ... }

		ctorName := GetDictionaryConstructorName(traitName, typeName)

		// Create constructor parameters
		var ctorParams []*ast.Parameter
		var witnessParams []string

		// For each type param, we need evidence if it has constraints
		// Example: instance Show a => Show (List a)
		// We need a dictionary for Show a.
		for range n.TypeParams {
			// Find constraints for this type param in n.Constraints
			// OR implicitly from context.
			// Actually, n.Constraints contains them.
		}
		// Wait, n.Constraints is used for "instance Show a => Show (List a)"
		// We need to pass dictionaries for these constraints.
		// Use 'requirements' which includes explicit constraints, inline type param constraints, AND recovered constraints.
		for _, req := range requirements {
			// Generate param name: $dict_a_Show
			paramName := GetWitnessParamName(req.TypeVar, req.Trait)
			witnessParams = append(witnessParams, paramName)
			ctorParams = append(ctorParams, &ast.Parameter{
				Name: &ast.Identifier{Token: n.TraitName.Token, Value: paramName},
				Type: &ast.NamedType{Name: &ast.Identifier{Value: "Dictionary"}}, // Untyped Dictionary
			})
		}

		// Generate Body
		// return __make_dictionary("Show", [methods...], [supers...])
		// Methods act as closures, capturing the dictionary parameters from the constructor scope.

		// We register the evidence name in SymbolTable so Solver can find it.
		// Key: "Show<List>" -> "$ctor_Show_List"
		// Solver will see it's a function and call it.

		// Construct the Type Key for evidence lookup
		// Use canonical key generation
		evidenceKey := GetEvidenceKey(traitName, instanceArgs)
		outer.RegisterEvidence(evidenceKey, ctorName)

		// Define the constructor in symbol table
		// It returns a Dictionary
		ctorType := typesystem.TFunc{
			Params:     make([]typesystem.Type, len(ctorParams)), // Dictionaries
			ReturnType: typesystem.TCon{Name: "Dictionary"},
		}
		for i := range ctorParams {
			ctorType.Params[i] = typesystem.TCon{Name: "Dictionary"}
		}
		outer.DefineConstant(ctorName, ctorType, w.currentModuleName)

		// Register witness parameters in the current scope (instance body scope)
		// so that SolveWitness can find them when resolving super traits.
		for i, paramName := range witnessParams {
			// We define them as variables in the scope
			w.symbolTable.Define(paramName, typesystem.TCon{Name: "Dictionary"}, w.currentModuleName)
			_ = i
		}

		// Generate methods array for the dictionary
		var methodExprs []ast.Expression
		methodNames := outer.GetTraitAllMethods(traitName)

		for _, mName := range methodNames {
			// Find method in n.Methods
			var impl ast.Expression
			found := false
			for _, m := range n.Methods {
				if m.Name != nil && m.Name.Value == mName {
					// Use the function literal from the method
					impl = &ast.FunctionLiteral{
						Token:      m.Token,
						Parameters: m.Parameters,
						ReturnType: m.ReturnType,
						Body:       m.Body,
						// Witnesses will be filled by inference if this body uses other traits
					}
					found = true
					break
				}
			}
			if !found {
				impl = &ast.NilLiteral{} // Placeholder for default
			}
			methodExprs = append(methodExprs, impl)
		}

		// Generate super traits expressions
		// We need to provide dictionaries for all super traits of the instance's trait.
		// e.g. instance Monad m => Applicative m.
		// Applicative requires Functor.
		// So when constructing Applicative m, we need Functor m.
		// We use SolveWitness to find it (which might find it in $dict_m_Monad or construct it).
		var superExprs []ast.Expression
		superTraitNames, _ := outer.GetTraitSuperTraits(traitName)
		for _, superName := range superTraitNames {
			// Resolve witness for SuperTrait<InstanceArgs>
			// Use w.inferCtx.SolveWitness
			witness, err := w.inferCtx.SolveWitness(n, superName, instanceArgs, w.symbolTable)
			if err != nil {
				w.addError(diagnostics.NewError(
					diagnostics.ErrA003,
					n.Token,
					fmt.Sprintf("failed to resolve super trait %s for generic instance %s: %v", superName, traitName, err),
				))
				// Fallback to Nil to allow compilation to proceed
				witness = &ast.NilLiteral{}
			}
			superExprs = append(superExprs, witness)
		}

		// Create the constructor function declaration
		// fun $ctor_...(params) { __make_dictionary(...) }
		ctorFuncDecl := &ast.FunctionStatement{
			Token:      n.TraitName.Token, // Borrow token from trait reference for error reporting
			Name:       &ast.Identifier{Token: n.TraitName.Token, Value: ctorName},
			TypeParams: n.TypeParams,
			Parameters: ctorParams,
			ReturnType: &ast.NamedType{Name: &ast.Identifier{Value: "Dictionary"}},
			Body: &ast.BlockStatement{
				Statements: []ast.Statement{
					&ast.ExpressionStatement{
						Expression: generateDictionaryNode(traitName, methodExprs, superExprs),
					},
				},
			},
		}

		// Inject into Program
		if w.Program != nil {
			// Queue for injection (to be processed after the current pass)
			w.injectedStmts = append(w.injectedStmts, ctorFuncDecl)
		}
	} else {
		// Concrete Instance: instance Show Int
		// Generate Global Constant: $impl_Show_Int = Dictionary { ... }

		implName := GetDictionaryName(traitName, typeName)

		// evidenceKey should match SolveWitness logic
		evidenceKey := GetEvidenceKey(traitName, instanceArgs)
		outer.RegisterEvidence(evidenceKey, implName)

		// Define constant in symbol table
		outer.DefineConstant(implName, typesystem.TCon{Name: "Dictionary"}, w.currentModuleName)

		// We need to inject the AST for this constant assignment?
		// e.g. $impl_Show_Int = __make_dictionary(...)
		// Can we append to w.Program.Statements?
		if w.Program != nil {
			// Generate methods array
			var methodExprs []ast.Expression
			// Get all required methods in order (by index)
			methodNames := outer.GetTraitAllMethods(traitName)
			// Sort/Fill based on indices
			// Indices are 0..N
			// We need a dense array.

			// Fill with implementations
			// Find implementation for each method
			for _, mName := range methodNames {
				// Find method in n.Methods
				var impl ast.Expression
				found := false
				for _, m := range n.Methods {
					if m.Name != nil && m.Name.Value == mName {
						// Use the function literal from the method
						// We need to convert FunctionStatement to FunctionLiteral?
						// Or reference it by name if we named it?
						// We analyzed `n.Methods` which are FunctionStatements.
						// They are defined as constants in the inner scope?
						// Wait, VisitInstanceDeclaration defined them in `w.symbolTable` (instance scope).
						// But they are not accessible globally by default.
						// We need to lift them or create closures.

						// Simplest: Create FunctionLiteral from FunctionStatement body
						// and parameters.
						impl = &ast.FunctionLiteral{
							Token:      m.Token,
							Parameters: m.Parameters,
							ReturnType: m.ReturnType,
							Body:       m.Body,
						}
						found = true
						break
					}
				}
				if !found {
					// Use default implementation if available
					// Need to refer to Trait.method default?
					// Use Identifier "Trait.method"? No, evaluator has defaults map.
					// We can put Nil or a marker.
					// Or lookup the default function.
					impl = &ast.NilLiteral{} // Placeholder for default
				}
				methodExprs = append(methodExprs, impl)
			}

			// Generate AST node
			dictNode := generateDictionaryNode(traitName, methodExprs, []ast.Expression{})

			// Inject declaration: $impl_Show_Int :- __make_dictionary(...)
			decl := &ast.ConstantDeclaration{
				Token: n.TraitName.Token, // Borrow token
				Name:  &ast.Identifier{Token: n.TraitName.Token, Value: implName},
				Value: dictNode,
				// TypeAnnotation: &ast.NamedType{Name: &ast.Identifier{Value: "Dictionary"}}, // Optional but good for documentation
			}

			// Append to program statements
			// We append to the end of the program to ensure it's defined
			// Since it's a constant, order doesn't matter for scope definition (Analyzer handles it),
			// but for evaluation it must be evaluated. Global constants are evaluated in order?
			// Usually yes. But since it's a constant definition, it should be fine.
			w.injectedStmts = append(w.injectedStmts, decl)
		}
	}
}

// generateDictionaryNode creates an AST expression for creating a Dictionary at runtime.
func generateDictionaryNode(traitName string, methods []ast.Expression, supers []ast.Expression) ast.Expression {
	return &ast.CallExpression{
		Function: &ast.Identifier{Value: "__make_dictionary"},
		Arguments: []ast.Expression{
			&ast.StringLiteral{Value: traitName},
			&ast.TupleLiteral{Elements: methods}, // Use Tuple for heterogeneous methods
			&ast.ListLiteral{Elements: supers},
		},
	}
}

// flattenType flattens nested TApps to ensure canonical representation.
// e.g. TApp(TApp(C, args1), args2) -> TApp(C, args1+args2)
func flattenType(t typesystem.Type) typesystem.Type {
	switch typ := t.(type) {
	case typesystem.TApp:
		constructor := flattenType(typ.Constructor)
		args := make([]typesystem.Type, len(typ.Args))
		for i, arg := range typ.Args {
			args[i] = flattenType(arg)
		}

		if nestedApp, ok := constructor.(typesystem.TApp); ok {
			// Flatten: TApp(TApp(C, args1), args2) -> TApp(C, args1+args2)
			newArgs := append([]typesystem.Type{}, nestedApp.Args...)
			newArgs = append(newArgs, args...)
			return flattenType(typesystem.TApp{
				Constructor: nestedApp.Constructor,
				Args:        newArgs,
			})
		}
		return typesystem.TApp{
			Constructor: constructor,
			Args:        args,
		}
	case typesystem.TFunc:
		newParams := make([]typesystem.Type, len(typ.Params))
		for i, p := range typ.Params {
			newParams[i] = flattenType(p)
		}
		return typesystem.TFunc{
			Params:       newParams,
			ReturnType:   flattenType(typ.ReturnType),
			IsVariadic:   typ.IsVariadic,
			DefaultCount: typ.DefaultCount,
			Constraints:  typ.Constraints,
		}
	case typesystem.TTuple:
		newElements := make([]typesystem.Type, len(typ.Elements))
		for i, e := range typ.Elements {
			newElements[i] = flattenType(e)
		}
		return typesystem.TTuple{Elements: newElements}
	}
	return t
}

func isIncompleteType(t typesystem.Type) bool {
	// Checks if the type contains unresolved inference variables (TVars).
	// This is used to defer strict signature verification during the initial pass (ModeInstances)
	// until full type inference (ModeBodies) is complete.
	if fn, ok := t.(typesystem.TFunc); ok {
		if _, isTVar := fn.ReturnType.(typesystem.TVar); isTVar {
			return true
		}
	}
	if poly, ok := t.(typesystem.TForall); ok {
		return isIncompleteType(poly.Type)
	}
	return false
}
