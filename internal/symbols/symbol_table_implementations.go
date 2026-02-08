package symbols

import (
	"fmt"
	"github.com/funvibe/funxy/internal/typesystem"
)

// GetOptionalUnwrapReturnType returns the return type of unwrap for a specific type.
// For a type F<A, ...> implementing Optional, this returns A (the "inner" type").
//
// Uses the instance method signature to derive this:
// 1. Find the unwrap signature for the type's constructor (e.g., Result, Option)
// 2. Unify the parameter type with the concrete type
// 3. Apply substitution to get the concrete return type
func (s *SymbolTable) GetOptionalUnwrapReturnType(t typesystem.Type) (typesystem.Type, bool) {
	// Get the type constructor name (e.g., "Option", "Result")
	typeName := getTypeConstructorName(t)
	if typeName == "" {
		return nil, false
	}

	// Look for instance-specific unwrap signature
	unwrapType, ok := s.GetInstanceMethodType("Optional", typeName, "unwrap")
	if ok {
		// Instantiate if Polytype
		if poly, ok := unwrapType.(typesystem.TForall); ok {
			subst := make(typesystem.Subst)
			for _, tv := range poly.Vars {
				subst[tv.Name] = typesystem.TVar{Name: tv.Name + "_inst"}
			}
			unwrapType = poly.Type.Apply(subst)
		}

		// Found instance-specific signature, unify to get concrete return type
		funcType, ok := unwrapType.(typesystem.TFunc)
		if ok && len(funcType.Params) == 1 {
			// Rename type vars to avoid conflicts
			renamedParam := RenameTypeVars(funcType.Params[0], "inst")
			renamedReturn := RenameTypeVars(funcType.ReturnType, "inst")

			// Unify concrete type with parameter
			subst, err := typesystem.Unify(t, renamedParam)
			if err == nil {
				return renamedReturn.Apply(subst), true
			}
		}
	}

	// Fallback to generic trait method
	genericUnwrap, ok := s.GetTraitMethodType("unwrap")
	if ok {
		// Instantiate if Polytype
		if poly, ok := genericUnwrap.(typesystem.TForall); ok {
			subst := make(typesystem.Subst)
			for _, tv := range poly.Vars {
				subst[tv.Name] = typesystem.TVar{Name: tv.Name + "_gen"}
			}
			genericUnwrap = poly.Type.Apply(subst)
		}

		funcType, ok := genericUnwrap.(typesystem.TFunc)
		if ok && len(funcType.Params) == 1 {
			renamedParam := RenameTypeVars(funcType.Params[0], "gen")
			renamedReturn := RenameTypeVars(funcType.ReturnType, "gen")

			subst, err := typesystem.Unify(t, renamedParam)
			if err == nil {
				return renamedReturn.Apply(subst), true
			}
		}
	}

	// Last fallback: Args[0] for common cases
	if tApp, ok := t.(typesystem.TApp); ok && len(tApp.Args) > 0 {
		return tApp.Args[0], true
	}

	return nil, false
}

func (s *SymbolTable) RegisterImplementation(traitName string, args []typesystem.Type, requirements []typesystem.Constraint, evidenceName string) error {
	// Validate that trait exists (search entire scope chain)
	if !s.TraitExists(traitName) {
		panic(fmt.Sprintf("RegisterImplementation: trait %q does not exist", traitName))
	}

	// Check for overlap across ALL scopes (local + parents)
	allImpls := s.GetAllImplementations()[traitName]
	for _, existingDef := range allImpls {
		if len(existingDef.TargetTypes) != len(args) {
			continue // Arity mismatch, shouldn't happen for same trait
		}

		// Check overlap for all args
		overlap := true
		for i, arg := range args {
			tRenamed := RenameTypeVars(arg, "new")
			_, err := typesystem.Unify(existingDef.TargetTypes[i], tRenamed)
			if err != nil {
				overlap = false
				break
			}
		}

		if overlap {
			// Format error message: if single arg, print it directly to match old tests
			var existStr, newStr string
			if len(existingDef.TargetTypes) == 1 {
				existStr = existingDef.TargetTypes[0].String()
				newStr = args[0].String()
			} else {
				existStr = fmt.Sprintf("%v", existingDef.TargetTypes)
				newStr = fmt.Sprintf("%v", args)
			}
			return fmt.Errorf("overlapping instances for trait %s: %s and %s", traitName, existStr, newStr)
		}
	}

	// Register in CURRENT scope (not outer) - instances are scoped to where they're declared
	// We create a new InstanceDef
	s.implementations[traitName] = append(s.implementations[traitName], InstanceDef{
		TraitName:       traitName,
		TargetTypes:     args,
		Requirements:    requirements,
		ConstructorName: evidenceName,
	})
	return nil
}

// FindMatchingImplementation finds an instance definition that matches the given arguments.
// It returns the InstanceDef and the substitution map derived from unification.
func (s *SymbolTable) FindMatchingImplementation(traitName string, args []typesystem.Type) (*InstanceDef, typesystem.Subst, error) {
	// Check local implementations
	if impls, ok := s.implementations[traitName]; ok {
		for i := range impls {
			implDef := &impls[i] // Pointer to avoid copy
			if len(implDef.TargetTypes) != len(args) {
				continue
			}

			// Try to unify all args
			totalSubst := make(typesystem.Subst)
			match := true

			// Rename instance vars to avoid collision with args vars
			// But args are concrete usually. Instance types are generic.
			// We rename instance types.
			renamedTargetTypes := make([]typesystem.Type, len(implDef.TargetTypes))
			for j, t := range implDef.TargetTypes {
				renamedTargetTypes[j] = RenameTypeVars(t, "inst")
			}

			for j, implArg := range renamedTargetTypes {
				// We want to match: implArg (generic) matches args[j] (concrete)
				// Unify(implArg, arg) -> subst that maps generic to concrete
				// Handle aliases: try to unify with alias expansion if direct unification fails
				subst, err := typesystem.Unify(implArg, args[j])
				if err != nil {
					// Retry with expanded aliases
					argExpanded := s.ResolveTypeAlias(args[j])
					implArgExpanded := s.ResolveTypeAlias(implArg)

					// Try combinations
					if argExpanded != args[j] {
						subst, err = typesystem.Unify(implArg, argExpanded)
					}
					if err != nil && implArgExpanded != implArg {
						subst, err = typesystem.Unify(implArgExpanded, args[j])
					}
					if err != nil && argExpanded != args[j] && implArgExpanded != implArg {
						subst, err = typesystem.Unify(implArgExpanded, argExpanded)
					}
				}

				if err != nil {
					match = false
					break
				}
				totalSubst = subst.Compose(totalSubst)
			}

			if match {
				return implDef, totalSubst, nil
			}
		}
	}

	// Check outer scope
	if s.outer != nil {
		return s.outer.FindMatchingImplementation(traitName, args)
	}

	return nil, nil, fmt.Errorf("implementation not found")
}

func (s *SymbolTable) IsImplementationExists(traitName string, args []typesystem.Type) bool {
	// Check local implementations
	if impls, ok := s.implementations[traitName]; ok {
		for _, implDef := range impls {
			if len(implDef.TargetTypes) != len(args) {
				continue
			}

			match := true
			for i, implArg := range implDef.TargetTypes {
				// Check arg[i] against implArg[i]
				if !s.matchesTypeOrAlias(implArg, args[i]) {
					match = false
					break
				}
			}

			if match {
				return true
			}
		}
	}

	// Always check outer scope (don't short-circuit if local didn't match)
	if s.outer != nil {
		return s.outer.IsImplementationExists(traitName, args)
	}
	return false
}

// matchesTypeOrAlias checks if `t` matches `impl` (considering aliases for t and impl)
// Refactored from original IsImplementationExists logic
func (s *SymbolTable) matchesTypeOrAlias(impl, t typesystem.Type) bool {
	typesToCheck := []typesystem.Type{t}

	// If t is a TCon, check if it's a type alias and add underlying type
	if tCon, ok := t.(typesystem.TCon); ok {
		if underlyingType, ok := s.GetTypeAlias(tCon.Name); ok {
			typesToCheck = append(typesToCheck, underlyingType)
		}
	}

	// If t is a TRecord, try to find a type alias name for it
	if tRec, ok := t.(typesystem.TRecord); ok {
		if aliasName, ok := s.FindTypeAliasForRecord(tRec); ok && aliasName != "" {
			typesToCheck = append(typesToCheck, typesystem.TCon{Name: aliasName})
		}
	}

	// Also check if impl is an alias
	implsToCheck := []typesystem.Type{impl}
	if tCon, ok := impl.(typesystem.TCon); ok {
		if underlying, ok := s.GetTypeAlias(tCon.Name); ok {
			implsToCheck = append(implsToCheck, underlying)
		}
	}

	for _, implCandidate := range implsToCheck {
		implRenamed := RenameTypeVars(implCandidate, "exist")

		for _, typeToCheck := range typesToCheck {
			_, err := typesystem.Unify(implRenamed, typeToCheck)
			if err == nil {
				return true
			}
		}
	}

	// HKT: For type constructors like Result, check if t's constructor matches impl
	if implCon, ok := impl.(typesystem.TCon); ok {
		for _, typeToCheck := range typesToCheck {
			// Direct check: Type IS the constructor (e.g. Writer)
			if tCon, ok := typeToCheck.(typesystem.TCon); ok {
				if implCon.Name == tCon.Name {
					return true
				}
			}
			// Partially applied check: Type is TApp (e.g. Writer<IntList>)
			if tApp, ok := typeToCheck.(typesystem.TApp); ok {
				// Check the immediate constructor first
				if tAppCon, ok := tApp.Constructor.(typesystem.TCon); ok {
					if implCon.Name == tAppCon.Name {
						return true
					}
				}
				// Unwrap all args to get to the base constructor
				base := tApp.Constructor
				for {
					if nestedApp, ok := base.(typesystem.TApp); ok {
						base = nestedApp.Constructor
					} else {
						break
					}
				}

				if tAppCon, ok := base.(typesystem.TCon); ok {
					if implCon.Name == tAppCon.Name {
						return true
					}
				}
			}
		}
	}

	return false
}

// RegisterInstanceMethod stores a specialized method signature for a trait instance.
// For example, for instance Optional<Result<E, T>>, we store:
//
//	unwrap: Result<E, T> -> T
//
// This allows correct inner type extraction for any type, not just Args[0].
func (s *SymbolTable) RegisterInstanceMethod(traitName, typeName, methodName string, methodType typesystem.Type) {
	if s.instanceMethods[traitName] == nil {
		s.instanceMethods[traitName] = make(map[string]map[string]typesystem.Type)
	}
	if s.instanceMethods[traitName][typeName] == nil {
		s.instanceMethods[traitName][typeName] = make(map[string]typesystem.Type)
	}
	s.instanceMethods[traitName][typeName][methodName] = methodType
}

// GetInstanceMethodType retrieves the specialized method signature for a trait instance.
// Returns the method type and true if found, nil and false otherwise.
func (s *SymbolTable) GetInstanceMethodType(traitName, typeName, methodName string) (typesystem.Type, bool) {
	if traitMethods, ok := s.instanceMethods[traitName]; ok {
		if typeMethods, ok := traitMethods[typeName]; ok {
			if methodType, ok := typeMethods[methodName]; ok {
				return methodType, true
			}
		}
	}
	if s.outer != nil {
		return s.outer.GetInstanceMethodType(traitName, typeName, methodName)
	}
	return nil, false
}

// GetAllImplementations returns a copy of all trait implementations
func (s *SymbolTable) GetAllImplementations() map[string][]InstanceDef {
	result := make(map[string][]InstanceDef)

	// Get from outer first
	if s.outer != nil {
		for k, v := range s.outer.GetAllImplementations() {
			result[k] = append([]InstanceDef(nil), v...) // copy slice
		}
	}

	// Merge current level
	for trait, impls := range s.implementations {
		result[trait] = append(result[trait], impls...)
	}

	return result
}
