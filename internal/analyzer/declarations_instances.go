package analyzer

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
	"strings"
)

func resolveReceiverTypeName(t ast.Type, table *symbols.SymbolTable) string {
	switch t := t.(type) {
	case *ast.NamedType:
		// Always return the named type - even if it resolves to a record (type alias)
		return t.Name.Value
	case *ast.TupleType:
		return "TUPLE"
	case *ast.RecordType:
		return "RECORD"
	default:
		return ""
	}
}

func (w *walker) VisitInstanceDeclaration(n *ast.InstanceDeclaration) {
	// Skip if not properly parsed
	if n == nil || n.TraitName == nil {
		return
	}

	// Mode Checks
	if w.mode == ModeNaming || w.mode == ModeHeaders {
		return
	}

	// 1. Check if Trait exists
	var sym *symbols.Symbol
	var ok bool
	var traitName string // Full trait name (qualified if module is specified)

	if n.ModuleName != nil {
		// Qualified trait name: sql.Model or kit.sql.Model
		// The ModuleName can be:
		// 1. Simple module alias: "sql" (from import "lib/sql")
		// 2. Qualified path: "kit.sql" (user wrote kit.sql.Model, but module was imported as "sql")

		// Try to find the trait using different strategies:
		// Strategy 1: Try the full qualified name (moduleName.traitName)
		fullQualifiedName := n.ModuleName.Value + "." + n.TraitName.Value

		// Strategy 2: Check if ModuleName contains dots (multi-level qualification)
		// If yes, try to find module by the last part only
		var candidateNames []string
		candidateNames = append(candidateNames, fullQualifiedName)

		if strings.Contains(n.ModuleName.Value, ".") {
			// kit.sql.Model -> try "sql.Model" as well
			parts := strings.Split(n.ModuleName.Value, ".")
			lastPart := parts[len(parts)-1]
			candidateNames = append(candidateNames, lastPart+"."+n.TraitName.Value)
		}

		// Strategy 3: Try just the trait name (for selective imports)
		candidateNames = append(candidateNames, n.TraitName.Value)

		// Try to find the trait using candidate names
		for _, candidateName := range candidateNames {
			symVal, found := w.symbolTable.Find(candidateName)
			if found && symVal.Kind == symbols.TraitSymbol {
				traitName = candidateName
				sym = &symVal
				ok = true
				break
			}
		}

		if !ok {
			w.addError(diagnostics.NewError(
				diagnostics.ErrA001,
				n.TraitName.GetToken(),
				fmt.Sprintf("trait %s not found in module %s", n.TraitName.Value, n.ModuleName.Value),
			))
			return
		}
	} else {
		// Unqualified trait name
		traitName = n.TraitName.Value
		symVal, found := w.symbolTable.Find(traitName)
		if found {
			sym = &symVal
			ok = true
		}
	}

	if !ok {
		w.addError(diagnostics.NewError(
			diagnostics.ErrA001, // Undeclared identifier
			n.TraitName.GetToken(),
			traitName,
		))
		return
	}
	if sym.Kind != symbols.TraitSymbol {
		w.addError(diagnostics.NewError(
			diagnostics.ErrA003, // Type error (or kind error)
			n.TraitName.GetToken(),
			traitName+" is not a trait",
		))
		return
	}

	// 1b. Check that super traits are implemented for the target type
	// We need to build target type first to check implementations
	if len(n.Args) == 0 {
		w.addError(diagnostics.NewError(
			diagnostics.ErrA003,
			n.Token,
			"missing target type/args for instance",
		))
		return
	}

	// Create a temporary scope for target type analysis to capture implicit generics
	// This ensures we capture 't' from 'List<t>'
	targetScope := symbols.NewEnclosedSymbolTable(w.symbolTable, symbols.ScopeFunction) // Instance head scope behaves like function scope for type params

	// Register explicit type params first (if any)
	existingParams := make(map[string]bool)
	for _, tp := range n.TypeParams {
		targetScope.DefineType(tp.Value, typesystem.TVar{Name: tp.Value}, "")
		existingParams[tp.Value] = true
	}

	// Scan for implicit generics in arguments (e.g. Result<e>)
	// We must do this BEFORE BuildType, so that 'e' is defined when BuildType runs.
	for _, argNode := range n.Args {
		implicits := CollectImplicitGenerics(argNode, targetScope)
		for _, name := range implicits {
			if !existingParams[name] {
				// Add to TypeParams
				n.TypeParams = append(n.TypeParams, &ast.Identifier{
					Token: n.TraitName.Token, // Best guess for token
					Value: name,
				})
				// Define in scope
				targetScope.DefineType(name, typesystem.TVar{Name: name}, "")
				existingParams[name] = true
			}
		}
	}

	// Build instance arguments (MPTC support) and recover constraints from leaked parser args
	var instanceArgs []typesystem.Type
	var recoveredRequirements []typesystem.Constraint
	var currentArgType typesystem.Type

	for _, argNode := range n.Args {
		t := BuildType(argNode, targetScope, &w.errors)

		// Check if t is a Trait (misparsed constraint)
		isTrait := false
		if tCon, ok := t.(typesystem.TCon); ok {
			if w.symbolTable.TraitExists(tCon.Name) {
				isTrait = true
				// It is a constraint on currentArgType
				if currentArgType != nil {
					// Extract var name
					if tv, ok := currentArgType.(typesystem.TVar); ok {
						recoveredRequirements = append(recoveredRequirements, typesystem.Constraint{
							TypeVar: tv.Name,
							Trait:   tCon.Name,
						})
					}
				}
			}
		}

		if !isTrait {
			instanceArgs = append(instanceArgs, t)
			currentArgType = t
		}
	}

	// Sanitize instanceArgs: fixes issue where parser includes constraints as type arguments
	// e.g. Entity<t: Show, Equal> being parsed as Entity<t, Equal> instead of Entity<t>
	for i, arg := range instanceArgs {
		if tApp, ok := arg.(typesystem.TApp); ok {
			if tCon, ok := tApp.Constructor.(typesystem.TCon); ok {
				// Check arity against definition
				if params, ok := w.symbolTable.GetTypeParams(tCon.Name); ok {
					if len(tApp.Args) > len(params) {
						// Truncate extra arguments (likely leaked constraints)
						newArgs := tApp.Args[:len(params)]
						instanceArgs[i] = typesystem.TApp{
							Constructor: tApp.Constructor,
							Args:        newArgs,
							KindVal:     tApp.KindVal,
						}
					}
				}
			}
		}
	}

	// For backward compatibility / legacy parts, set targetType to first arg
	var targetType typesystem.Type
	if len(instanceArgs) > 0 {
		targetType = instanceArgs[0]
	}

	// Use registered kinds (from symbol table) to verify
	// Automatically detect HKT traits by checking if type param is applied in method signatures

	// Iterate over all arguments and check their kinds against the trait's parameter kinds
	typeParamNames, _ := w.symbolTable.GetTraitTypeParams(traitName)
	for i, arg := range instanceArgs {
		if i >= len(typeParamNames) {
			break
		}

		paramName := typeParamNames[i]
		expectedKind, ok := w.symbolTable.GetTraitTypeParamKind(traitName, paramName)

		if !ok {
			// Should not happen for well-defined traits
			w.addError(diagnostics.NewError(
				diagnostics.ErrA001,
				n.TraitName.GetToken(),
				fmt.Sprintf("unknown kind for trait parameter %s of %s", paramName, traitName),
			))
			continue
		}

		argKind := arg.Kind()
		if err := typesystem.UnifyKinds(argKind, expectedKind); err != nil {
			// Use different error messages based on expected kind for clarity
			expectedDesc := ""
			if _, isArrow := expectedKind.(typesystem.KArrow); isArrow {
				expectedDesc = " (type constructor)"
			} else {
				expectedDesc = " (concrete type)"
			}

			w.addError(diagnostics.NewError(
				diagnostics.ErrA003,
				n.Args[i].GetToken(), // Use specific argument token
				fmt.Sprintf("type %s has kind %s, but trait %s requires kind %s for parameter %s%s",
					arg, argKind, traitName, expectedKind, paramName, expectedDesc),
			))
		}
	}

	// Stop analysis if kinds don't match to avoid generating broken dictionary signatures
	if len(w.getErrors()) > 0 {
		return
	}

	superTraits, _ := w.symbolTable.GetTraitSuperTraits(traitName)
	for _, superTrait := range superTraits {
		if !w.symbolTable.IsImplementationExists(superTrait, instanceArgs) {
			w.addError(diagnostics.NewError(
				diagnostics.ErrA003,
				n.Token,
				"cannot implement "+traitName+" for "+targetType.String()+": missing implementation of super trait "+superTrait,
			))
			return
		}
	}

	// 2. Extract type name from target
	var typeName string
	if tCon, ok := targetType.(typesystem.TCon); ok {
		typeName = tCon.Name
	} else if tApp, ok := targetType.(typesystem.TApp); ok {
		// Extract constructor name from app
		if tCon, ok := tApp.Constructor.(typesystem.TCon); ok {
			typeName = tCon.Name
		}
	}

	if typeName == "" && len(n.Args) > 0 {
		// Try to get from AST directly if BuildType resolves to something else (like Int which is built-in)
		if nt, ok := n.Args[0].(*ast.NamedType); ok {
			typeName = nt.Name.Value
		} else if _, ok := n.Args[0].(*ast.TupleType); ok {
			// Tuple support: use standardized "TUPLE" name
			typeName = "TUPLE"
		} else if _, ok := n.Args[0].(*ast.RecordType); ok {
			// Record support
			typeName = "RECORD"
		} else if _, ok := n.Args[0].(*ast.FunctionType); ok {
			// Function support
			typeName = "FUNCTION"
		}
	}

	if typeName == "" {
		// Fallback or error
		token := n.Token
		if len(n.Args) > 0 {
			token = n.Args[0].GetToken()
		}
		w.addError(diagnostics.NewError(
			diagnostics.ErrA003,
			token,
			"invalid target type for instance",
		))
		return
	}

	// Prepare requirements for generic instance constraints
	var requirements []typesystem.Constraint

	// 0. Add recovered requirements from parsed arguments
	requirements = append(requirements, recoveredRequirements...)

	// 0b. Extract constraints from inline declarations in Args (e.g. Pair<a: Eq>)
	for _, argNode := range n.Args {
		CollectConstraintsFromType(argNode, &requirements)
	}

	// 1. Collect constraints from Type Parameters (inline: <T: Show>)
	for _, tp := range n.TypeParams {
		for _, c := range tp.Constraints {
			var constraintArgs []typesystem.Type
			for _, argNode := range c.Args {
				t := BuildType(argNode, targetScope, &w.errors)
				constraintArgs = append(constraintArgs, t)
			}
			// Use tp.Value as TypeVar if c.TypeVar is empty (inline constraint usually refers to the param)
			typeVar := c.TypeVar
			if typeVar == "" {
				typeVar = tp.Value
			}

			requirements = append(requirements, typesystem.Constraint{
				TypeVar: typeVar,
				Trait:   c.Trait,
				Args:    constraintArgs,
			})
		}
	}

	// 2. Collect constraints from Where Clause
	for _, c := range n.Constraints {
		var constraintArgs []typesystem.Type
		for _, argNode := range c.Args {
			t := BuildType(argNode, targetScope, &w.errors)
			constraintArgs = append(constraintArgs, t)
		}

		requirements = append(requirements, typesystem.Constraint{
			TypeVar: c.TypeVar,
			Trait:   c.Trait,
			Args:    constraintArgs,
		})
	}

	// 3. Register Implementation
	// Register if in header scan (Global) OR if in function body (Local)
	if w.mode != ModeBodies || w.inFunctionBody {
		// Populate AST for Evaluator
		n.AnalyzedRequirements = requirements

		// Generate evidence name for dynamic lookup (SolveWitness fallback)
		var evidenceName string
		if len(n.TypeParams) > 0 {
			evidenceName = GetDictionaryConstructorName(traitName, typeName)
		} else {
			evidenceName = GetDictionaryName(traitName, typeName)
		}

		err := w.symbolTable.RegisterImplementation(traitName, instanceArgs, requirements, evidenceName)
		if err != nil {
			w.addError(diagnostics.NewError(
				diagnostics.ErrA004, // Redefinition/Overlap
				n.TraitName.GetToken(),
				err.Error(),
			))
			return
		}
	}

	// 3b. Check that all required methods are implemented
	requiredMethods := w.symbolTable.GetTraitRequiredMethods(traitName)
	implementedMethods := make(map[string]bool)
	for _, method := range n.Methods {
		if method == nil {
			continue
		}
		// For operator methods, Name is nil and Operator contains the operator symbol
		if method.Name != nil {
			implementedMethods[method.Name.Value] = true
		} else if method.Operator != "" {
			implementedMethods["("+method.Operator+")"] = true
		}
	}
	for _, required := range requiredMethods {
		if !implementedMethods[required] {
			w.addError(diagnostics.NewError(
				diagnostics.ErrA003,
				n.Token,
				"instance "+traitName+" for "+typeName+" is missing required method '"+required+"'",
			))
		}
	}

	// 4. Analyze methods
	// Reuse targetScope which already contains the instance type parameters (including implicit ones)
	// and has the correct parent (module scope).
	outer := w.symbolTable
	w.symbolTable = targetScope
	defer func() { w.symbolTable = outer }()

	// Track instance context
	prevInInstance := w.inInstance
	w.inInstance = true
	defer func() { w.inInstance = prevInInstance }()

	// Verify signatures
	typeParamNames, ok = w.symbolTable.GetTraitTypeParams(traitName)
	if !ok {
		w.addError(diagnostics.NewError(
			diagnostics.ErrA001,
			n.TraitName.GetToken(),
			"unknown trait type param for "+traitName,
		))
		return
	}

	if len(typeParamNames) == 0 {
		w.addError(diagnostics.NewError(
			diagnostics.ErrA003,
			n.TraitName.GetToken(),
			"trait "+traitName+" has no type parameters",
		))
		return
	}

	// Create substitution: TraitTypeParams -> InstanceArgs
	subst := make(typesystem.Subst)
	for i, paramName := range typeParamNames {
		if i < len(instanceArgs) {
			subst[paramName] = instanceArgs[i]
		}
	}

	// Register requirements in inference context so they are available in method bodies
	// This ensures that constraints like 'a: Eq' are available when analyzing 'eq(x1, x2)'
	for _, req := range requirements {
		if len(req.Args) > 0 {
			w.inferCtx.AddMPTCConstraint(req.TypeVar, req.Trait, req.Args)
		} else {
			w.inferCtx.AddConstraint(req.TypeVar, req.Trait)
		}
	}

	// Redefine type parameters as rigid TCons for method body analysis
	// This ensures that 'a' is treated as a constant, preventing unification from binding it to fresh variables
	// which causes "ambiguous type" errors in pattern matching.
	for _, tp := range n.TypeParams {
		if sym, ok := targetScope.Find(tp.Value); ok && sym.Kind == symbols.TypeSymbol {
			kind := sym.Type.Kind()
			tCon := typesystem.TCon{Name: tp.Value, KindVal: kind}
			targetScope.DefineType(tp.Value, tCon, "")
		}
	}

	// Define witness parameters for constraints (so SolveWitness can find them)
	for _, req := range requirements {
		witnessName := GetWitnessParamName(req.TypeVar, req.Trait)
		if len(req.Args) > 0 {
			// Append MPTC args to match witness param naming convention ($dict_T_Trait_Arg1...)
			// This must match SolveWitness Case A logic
			for _, arg := range req.Args {
				witnessName += "_" + arg.String()
			}
		}
		targetScope.DefineConstant(witnessName, typesystem.TCon{Name: "Dictionary"}, "")
	}

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
		// 1. Find generic signature
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

		// 2. Create Expected Type (Generic signature with substitution)
		expectedType := genericSymbol.Type.Apply(subst)

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

		// 3. Get Actual Type from the method definition in CURRENT scope
		// VisitFunctionStatement (called by method.Accept(w)) defined it in w.symbolTable (the inner scope).
		actualSymbol, ok := w.symbolTable.Find(mangledName)
		if !ok {
			// Should not happen if VisitFunctionStatement works
			continue
		}
		actualType := actualSymbol.Type

		// 4. Unify
		// If the actual method type is polymorphic (Rank-N), it means the method implementation
		// is generic. We need to check if this generic implementation satisfies the required
		// instance signature. This corresponds to subsumption check: actual <= expected.
		// Since Unify checks equality, we instantiate the actual type with fresh variables
		// to allow it to unify with the specific instance types.
		actualToCheck := actualType
		if poly, ok := actualType.(typesystem.TForall); ok {
			actualToCheck = InstantiateForall(w.inferCtx, poly)
		}

		_, err := typesystem.Unify(expectedType, actualToCheck)
		if err != nil {
			w.addError(diagnostics.NewError(
				diagnostics.ErrA003,
				method.Name.GetToken(),
				"method signature mismatch: expected "+expectedType.String()+", got "+actualType.String(),
			))
		} else {
			// 5. Register instance method signature for use in type inference
			// This allows traits like Optional to correctly extract inner types
			// for any user-defined type, not just built-in types.
			// We flatten the type to ensure TApp(TApp(C, args1), args2) becomes TApp(C, args1+args2)
			// This is critical for Unify to work correctly with instance signatures like Validated<e, a>.
			flattenedExpected := flattenType(expectedType)
			outer.RegisterInstanceMethod(traitName, typeName, method.Name.Value, flattenedExpected)
		}
	}

	// 2.2 Dictionary Generation (Static & Constructors)
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

// CollectConstraintsFromType recursively finds constraints in AST Type nodes
// (e.g. constraints attached to NamedType identifiers during parsing)
func CollectConstraintsFromType(t ast.Type, requirements *[]typesystem.Constraint) {
	if t == nil {
		return
	}
	switch n := t.(type) {
	case *ast.NamedType:
		if n.Name != nil {
			for _, c := range n.Name.Constraints {
				*requirements = append(*requirements, typesystem.Constraint{
					TypeVar: n.Name.Value,
					Trait:   c.Trait,
					Args:    nil, // Inline syntax only supports single trait for now
				})
			}
		}
		for _, arg := range n.Args {
			CollectConstraintsFromType(arg, requirements)
		}
	case *ast.TupleType:
		for _, el := range n.Types {
			CollectConstraintsFromType(el, requirements)
		}
	case *ast.RecordType:
		for _, v := range n.Fields {
			CollectConstraintsFromType(v, requirements)
		}
	case *ast.FunctionType:
		for _, p := range n.Parameters {
			CollectConstraintsFromType(p, requirements)
		}
		CollectConstraintsFromType(n.ReturnType, requirements)
	}
}
