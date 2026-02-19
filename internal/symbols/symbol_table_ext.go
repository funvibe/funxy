package symbols

import "github.com/funvibe/funxy/internal/typesystem"

// FindMatchingInstances finds all instance definitions that partially match the given arguments.
// It matches resolved (concrete) arguments against the instance definition.
// Unresolved variables in 'args' are ignored (wildcards).
func (s *SymbolTable) FindMatchingInstances(traitName string, args []typesystem.Type, isVar func(typesystem.Type) bool) []InstanceDef {
	var matches []InstanceDef

	// Check local implementations
	if impls, ok := s.implementations[traitName]; ok {
		for _, implDef := range impls {
			if len(implDef.TargetTypes) != len(args) {
				continue
			}

			match := true
			// Rename instance vars
			renamedTargetTypes := make([]typesystem.Type, len(implDef.TargetTypes))
			for j, t := range implDef.TargetTypes {
				renamedTargetTypes[j] = RenameTypeVars(t, "inst")
			}

			for i, arg := range args {
				// If arg is a variable, it's a wildcard, matches anything.
				if isVar(arg) {
					continue
				}

				// Arg is concrete. It must match the instance type.
				// Instance type might be generic (e.g. List<a>).
				// We need to check if they unify.
				_, err := typesystem.Unify(renamedTargetTypes[i], arg)
				if err != nil {
					// Try alias expansion similar to matchesTypeOrAlias
					if !s.matchesTypeOrAlias(renamedTargetTypes[i], arg) {
						match = false
						break
					}
				}
			}

			if match {
				matches = append(matches, implDef)
			}
		}
	}

	// Check outer scope
	if s.outer != nil {
		outerMatches := s.outer.FindMatchingInstances(traitName, args, isVar)
		matches = append(matches, outerMatches...)
	}

	return matches
}

// GetAllExtensionMethods returns all extension methods in the symbol table (and outer scopes)
func (s *SymbolTable) GetAllExtensionMethods() map[string]map[string]typesystem.Type {
	allMethods := make(map[string]map[string]typesystem.Type)

	// Copy from outer scope first
	if s.outer != nil {
		outerMethods := s.outer.GetAllExtensionMethods()
		for typeName, methods := range outerMethods {
			allMethods[typeName] = make(map[string]typesystem.Type)
			for methodName, methodType := range methods {
				allMethods[typeName][methodName] = methodType
			}
		}
	}

	// Merge local methods
	for typeName, methods := range s.extensionMethods {
		if _, ok := allMethods[typeName]; !ok {
			allMethods[typeName] = make(map[string]typesystem.Type)
		}
		for methodName, methodType := range methods {
			allMethods[typeName][methodName] = methodType
		}
	}

	return allMethods
}

// GetExtensionMethodsByName returns all types that have an extension method with the given name.
// This uses the optimized index if available, or falls back to scanning.
func (s *SymbolTable) GetExtensionMethodsByName(methodName string) []string {
	var types []string
	seen := make(map[string]bool)

	// Check local index
	if s.extensionMethodsByName != nil {
		if typeNames, ok := s.extensionMethodsByName[methodName]; ok {
			for _, t := range typeNames {
				if !seen[t] {
					types = append(types, t)
					seen[t] = true
				}
			}
		}
	} else {
		// Fallback to scanning local map if index not built (should not happen if RegisterExtensionMethod is used)
		for typeName, methods := range s.extensionMethods {
			if _, ok := methods[methodName]; ok {
				if !seen[typeName] {
					types = append(types, typeName)
					seen[typeName] = true
				}
			}
		}
	}

	// Check outer scope
	if s.outer != nil {
		outerTypes := s.outer.GetExtensionMethodsByName(methodName)
		for _, t := range outerTypes {
			if !seen[t] {
				types = append(types, t)
				seen[t] = true
			}
		}
	}

	return types
}
