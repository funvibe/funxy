package symbols

import (
	"github.com/funvibe/funxy/internal/typesystem"
)

// GetInstanceRequirements returns the requirements (constraints) for a matching instance.
// Returns nil, false if no instance matches.
func (s *SymbolTable) GetInstanceRequirements(traitName string, args []typesystem.Type) ([]typesystem.Constraint, bool) {
	implDef, _, found := s.FindInstance(traitName, args)
	if found {
		return implDef.Requirements, true
	}
	return nil, false
}

// GetInstanceArgs returns the target types defined in the matching instance.
func (s *SymbolTable) GetInstanceArgs(traitName string, evidenceName string) ([]typesystem.Type, bool) {
	// Check local implementations
	if impls, ok := s.implementations[traitName]; ok {
		for _, implDef := range impls {
			if implDef.ConstructorName == evidenceName {
				return implDef.TargetTypes, true
			}
		}
	}

	if s.outer != nil {
		return s.outer.GetInstanceArgs(traitName, evidenceName)
	}
	return nil, false
}

// FindInstance searches for an instance definition that matches the given arguments.
// It returns the InstanceDef, the Substitution (mapping instance vars to concrete args), and a boolean success flag.
func (s *SymbolTable) FindInstance(traitName string, args []typesystem.Type) (InstanceDef, typesystem.Subst, bool) {
	// Check local implementations
	if impls, ok := s.implementations[traitName]; ok {
		for _, implDef := range impls {
			if len(implDef.TargetTypes) != len(args) {
				continue
			}

			subst := make(typesystem.Subst)
			match := true
			for i, implArg := range implDef.TargetTypes {
				if !s.matchTypeWithSubst(implArg, args[i], subst) {
					match = false
					break
				}
			}

			if match {
				return implDef, subst, true
			}
		}
	}

	if s.outer != nil {
		return s.outer.FindInstance(traitName, args)
	}
	return InstanceDef{}, nil, false
}

// matchTypeWithSubst checks if pattern matches target, populating subst with variable mappings.
// pattern: Instance generic type (e.g. List<a>)
// target: Concrete type (e.g. List<Int>)
func (s *SymbolTable) matchTypeWithSubst(pattern, target typesystem.Type, subst typesystem.Subst) bool {
	// Unwrap aliases from target
	target = typesystem.UnwrapUnderlying(target)
	if target == nil {
		// Should not happen if UnwrapUnderlying handles non-alias types gracefully,
		// but if it returns nil for non-alias, we use original.
		// Assuming UnwrapUnderlying might return nil for non-alias types in some implementations?
		// Let's assume standard behavior: returns underlying or nil.
		// Safe way:
		// target is already target.
	}
	// Actually, matching logic should handle aliases by expanding if needed.
	// Reuse matchesTypeOrAlias logic but with capture?
	// Simplified structural matching:

	switch p := pattern.(type) {
	case typesystem.TVar:
		// Capture variable
		if existing, ok := subst[p.Name]; ok {
			// Check consistency
			// e.g. T(a, a) matching T(Int, String) -> Fail
			_, err := typesystem.Unify(existing, target)
			return err == nil
		}
		subst[p.Name] = target
		return true

	case typesystem.TCon:
		// Exact match (or alias match)
		if tCon, ok := target.(typesystem.TCon); ok {
			if p.Name == tCon.Name {
				return true
			}
			// Check if target is alias for p?
			if underlying, ok := s.GetTypeAlias(tCon.Name); ok {
				return s.matchTypeWithSubst(p, underlying, subst)
			}
			// Check if p is alias for target?
			if underlying, ok := s.GetTypeAlias(p.Name); ok {
				return s.matchTypeWithSubst(underlying, target, subst)
			}
			return false
		}
		// If target is App, maybe p is alias for App?
		if underlying, ok := s.GetTypeAlias(p.Name); ok {
			return s.matchTypeWithSubst(underlying, target, subst)
		}
		return false

	case typesystem.TApp:
		if tApp, ok := target.(typesystem.TApp); ok {
			// Match Constructor
			if !s.matchTypeWithSubst(p.Constructor, tApp.Constructor, subst) {
				return false
			}
			// Match Args
			if len(p.Args) != len(tApp.Args) {
				return false
			}
			for i := range p.Args {
				if !s.matchTypeWithSubst(p.Args[i], tApp.Args[i], subst) {
					return false
				}
			}
			return true
		}
		// Check alias on target
		if tCon, ok := target.(typesystem.TCon); ok {
			if underlying, ok := s.GetTypeAlias(tCon.Name); ok {
				return s.matchTypeWithSubst(p, underlying, subst)
			}
		}
		return false

		// Handle other types (Records, Tuples, Functions) similarly...
		// For now assume TCon/TApp/TVar covers most MPTC cases
	}

	// Fallback to strict equality or Unify?
	// Unify modifies subst, but we want pattern matching (one way).
	// But Unify is bidirectional.
	_, err := typesystem.Unify(pattern, target)
	return err == nil
}
