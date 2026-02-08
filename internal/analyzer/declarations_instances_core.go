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

	// Check if Trait exists
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

	// Check that super traits are implemented for the target type
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

		// If BuildType fails (returns nil), it has already reported an error.
		// We skip this argument to avoid further nil pointer dereferences.
		if t == nil {
			continue
		}

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
		if arg == nil {
			continue
		}
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

	// Track initial error count to only abort if THIS instance failed kind checking
	initialErrorCount := len(w.errorSet)

	// Iterate over all arguments and check their kinds against the trait's parameter kinds
	typeParamNames, _ := w.symbolTable.GetTraitTypeParams(traitName)
	for i, arg := range instanceArgs {
		if i >= len(typeParamNames) {
			break
		}

		if arg == nil {
			continue
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
		if _, err := typesystem.UnifyKinds(argKind, expectedKind); err != nil {
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

	// Stop analysis if kinds don't match (for this instance)
	if len(w.errorSet) > initialErrorCount {
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

	// Extract type name from target
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

	// Add recovered requirements from parsed arguments
	requirements = append(requirements, recoveredRequirements...)

	// Extract constraints from inline declarations in Args (e.g. Pair<a: Eq>)
	for _, argNode := range n.Args {
		CollectConstraintsFromType(argNode, &requirements)
	}

	// Collect constraints from Type Parameters (inline: <T: Show>)
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

	// Collect constraints from Where Clause
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

	// Check Functional Dependencies consistency
	if deps, ok := w.symbolTable.GetTraitFunctionalDependencies(traitName); ok && len(deps) > 0 {
		allImpls := w.symbolTable.GetAllImplementations()[traitName]
		typeParamNames, _ := w.symbolTable.GetTraitTypeParams(traitName)

		paramIndices := make(map[string]int)
		for i, name := range typeParamNames {
			paramIndices[name] = i
		}

		for _, dep := range deps {
			// Coverage Condition Check: Variables in To must be subset of Variables in From
			fromVars := make(map[string]bool)
			for _, fromVar := range dep.From {
				idx, ok := paramIndices[fromVar]
				if !ok {
					continue
				}
				// Collect vars from instanceArgs[idx]
				vars := instanceArgs[idx].FreeTypeVariables()
				for _, v := range vars {
					fromVars[v.Name] = true
				}
			}

			for _, toVar := range dep.To {
				idx, ok := paramIndices[toVar]
				if !ok {
					continue
				}
				vars := instanceArgs[idx].FreeTypeVariables()
				for _, v := range vars {
					if !fromVars[v.Name] {
						w.addError(diagnostics.NewError(
							diagnostics.ErrA003,
							n.Token,
							fmt.Sprintf("instance violates functional dependency %v -> %v: variable(s) [%s] in output are not determined by input",
								strings.Join(dep.From, ", "), strings.Join(dep.To, ", "), v.Name),
						))
						return
					}
				}
			}

			for _, existingImpl := range allImpls {
				if len(existingImpl.TargetTypes) != len(instanceArgs) {
					continue
				}

				unifyFrom := true
				subst := make(typesystem.Subst)

				// Check if inputs unify
				for _, fromVar := range dep.From {
					idx, ok := paramIndices[fromVar]
					if !ok {
						continue
					}
					newArg := instanceArgs[idx]
					existingArg := existingImpl.TargetTypes[idx]

					existingArgRenamed := symbols.RenameTypeVars(existingArg, "exist")

					// Accumulate substitutions across all "From" arguments to ensure consistency.
					// We verify that the new instance's input types unify with the existing instance's
					// input types. For example, if we have dependency (a, b -> c) and existing instance
					// (List<x>, x, ...), a new instance (List<Int>, Int, ...) must unify `x` with `Int`
					// across both positions.
					//
					// If input unification succeeds, it implies the instances overlap on the domain of the dependency.
					// We must then verify that the codomain (To variables) also unifies.

					s, err := typesystem.Unify(newArg.Apply(subst), existingArgRenamed.Apply(subst))
					if err != nil {
						unifyFrom = false
						break
					}
					subst = subst.Compose(s)
				}

				if unifyFrom {
					// Inputs match, so outputs MUST match
					for _, toVar := range dep.To {
						idx, ok := paramIndices[toVar]
						if !ok {
							continue
						}
						newArg := instanceArgs[idx]
						existingArg := existingImpl.TargetTypes[idx]
						existingArgRenamed := symbols.RenameTypeVars(existingArg, "exist")

						newArgSubst := newArg.Apply(subst)
						existingArgRenamedSubst := existingArgRenamed.Apply(subst)

						// Collect free variables from new instance that were NOT fixed by From
						// We must not bind these variables during To unification.
						//
						// Coverage Condition & Unification Check:
						// 1. Coverage Condition: All variables in the "To" position (codomain) must be
						//    determined by the variables in the "From" position (domain). This ensures
						//    that the dependency is valid in isolation.
						// 2. Unification Check: Verify that the codomain (output types) is consistent
						//    with the existing instance. Since the inputs unify (domains overlap),
						//    the Functional Dependency `From -> To` requires the outputs to also unify.
						//    If they do not, it indicates a conflict where the same input maps to
						//    different outputs.

						s, err := typesystem.Unify(newArgSubst, existingArgRenamedSubst)
						if err != nil {
							w.addError(diagnostics.NewError(
								diagnostics.ErrA004,
								n.Token,
								fmt.Sprintf("Conflicting instances for trait %s. Dependency %v -> %v violated. Input matches existing instance (%s), but output maps to different type (%s vs %s)",
									traitName, dep.From, dep.To, existingImpl.TargetTypes, newArg, existingArg),
							))
							// Stop processing this instance after finding a conflict.
							// A single functional dependency violation is sufficient to invalidate the instance declaration.
							return
						}
						// Apply substitution from codomain check too
						subst = subst.Compose(s)
					}
				}
			}
		}
	}

	// Register Implementation
	// Register if in ModeInstances (Global) OR if in function body (Local)
	// We must avoid re-registering top-level instances during ModeBodies (Pass 4)
	shouldRegister := (w.mode == ModeInstances) || (w.mode == ModeBodies && w.inFunctionBody)
	if shouldRegister {
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

	// Check that all required methods are implemented
	// We can do this check when registering (Pass 3) or always.
	// Doing it always is fine.
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

	// Analyze methods
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
	// Must be done in targetScope every pass (as it's a fresh scope)
	// Do this BEFORE method analysis so methods can use them
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

	// Continue to method verification and dictionary generation
	// Use targetScope as 'w.symbolTable' for method processing to ensure visibility of type params
	// visitInstanceMethods expects w.symbolTable to be the inner scope.
	w.visitInstanceMethods(n, traitName, typeName, instanceArgs, requirements, subst, outer)
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
