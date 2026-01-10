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
