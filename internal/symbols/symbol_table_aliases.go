package symbols

import (
	"github.com/funvibe/funxy/internal/typesystem"
)

// DefineTypeAlias defines a type alias with both the nominal type (TCon) and underlying type.
// Type field stores TCon for trait/module lookup, UnderlyingType stores the resolved type for unification.
func (s *SymbolTable) DefineTypeAlias(name string, nominalType, underlyingType typesystem.Type, origin string) {
	s.types[name] = underlyingType // For ResolveType to get underlying

	// Crucial fix: Update the TCon itself to include the UnderlyingType.
	// This ensures that when this TCon is exported (as Symbol.Type) or passed around,
	// it carries the alias information needed for unwrapping.
	if tCon, ok := nominalType.(typesystem.TCon); ok {
		tCon.UnderlyingType = underlyingType
		nominalType = tCon // Assign back since Go structs are value types
	}

	s.store[name] = Symbol{
		Name:           name,
		Type:           nominalType, // TCon for trait lookup (with UnderlyingType set)
		Kind:           TypeSymbol,
		UnderlyingType: underlyingType, // TRecord for field access
		OriginModule:   origin,
	}
	// Also register in typeAliases for lookup
	s.typeAliases[name] = underlyingType
}

// RegisterTypeAlias stores the underlying type for a type alias.
// This keeps TCon in store for proper module tagging, while allowing
// type resolution to use the underlying type.
func (s *SymbolTable) RegisterTypeAlias(name string, underlyingType typesystem.Type) {
	s.typeAliases[name] = underlyingType
}

// GetTypeAlias returns the underlying type for a type alias.
func (s *SymbolTable) GetTypeAlias(name string) (typesystem.Type, bool) {
	t, ok := s.typeAliases[name]
	if !ok && s.outer != nil {
		return s.outer.GetTypeAlias(name)
	}
	return t, ok
}

// ResolveTypeAlias recursively resolves type aliases to their underlying types.
// For TCon types that are aliases, returns the underlying type.
// For TApp types, resolves the constructor and args recursively.
// For other types, returns them unchanged.
func (s *SymbolTable) ResolveTypeAlias(t typesystem.Type) typesystem.Type {
	return s.resolveTypeAliasWithCycleCheck(t, make(map[string]bool))
}

// ResolveTCon retrieves the canonical TCon definition from the symbol table.
// This is used to refresh stale TCon values (passed by value) that may be missing
// UnderlyingType or TypeParams fields.
func (s *SymbolTable) ResolveTCon(name string) (typesystem.TCon, bool) {
	sym, ok := s.Find(name)
	if !ok {
		return typesystem.TCon{}, false
	}
	if tCon, ok := sym.Type.(typesystem.TCon); ok {
		return tCon, true
	}
	return typesystem.TCon{}, false
}

func (s *SymbolTable) resolveTypeAliasWithCycleCheck(t typesystem.Type, visited map[string]bool) typesystem.Type {
	switch ty := t.(type) {
	case typesystem.TCon:
		// For qualified types (e.g., dbStorage.DbConfig), look up with qualified name
		lookupName := ty.Name
		if ty.Module != "" {
			lookupName = ty.Module + "." + ty.Name
		}

		// Cycle detection: if we've already visited this alias in the current path, stop expansion
		if visited[lookupName] {
			// Return the canonical type from symbol table if available,
			// so it has UnderlyingType set correctly for future Unify calls (UnwrapUnderlying).
			if sym, ok := s.Find(lookupName); ok && sym.Type != nil {
				return sym.Type
			}
			return t
		}

		// Optimization: if the TCon already has UnderlyingType set (e.g. from BuildType), use it.
		if ty.UnderlyingType != nil {
			visited[lookupName] = true
			defer delete(visited, lookupName)
			return s.resolveTypeAliasWithCycleCheck(ty.UnderlyingType, visited)
		}

		// Check if this TCon is a type alias
		if underlying, ok := s.GetTypeAlias(lookupName); ok {
			visited[lookupName] = true
			defer delete(visited, lookupName) // Backtrack: remove from visited path when returning

			// Recursively resolve the underlying type.
			// If recursion hits a cycle, it will return a TCon (or type containing TCon).
			return s.resolveTypeAliasWithCycleCheck(underlying, visited)
		} else if ty.Module != "" {
			// Try without module prefix (local alias)
			if underlying, ok := s.GetTypeAlias(ty.Name); ok {
				visited[ty.Name] = true
				defer delete(visited, ty.Name)
				return s.resolveTypeAliasWithCycleCheck(underlying, visited)
			}
		}

		// Fallback: Check if it's a Nominal Type with UnderlyingType (e.g. from RegisterTypeDeclaration fallback)
		// We treat it as an alias for resolution purposes to support structural unification (freshness).
		if sym, ok := s.Find(lookupName); ok && sym.Kind == TypeSymbol {
			if tCon, ok := sym.Type.(typesystem.TCon); ok && tCon.UnderlyingType != nil {
				visited[lookupName] = true
				defer delete(visited, lookupName)
				return s.resolveTypeAliasWithCycleCheck(tCon.UnderlyingType, visited)
			}
		}

		// EXTRA FALLBACK for qualified types (e.g. dbStorage.DbConfig) imported via module alias
		if ty.Module != "" {
			// 1. Try resolving via module symbol directly (if 'ty.Module' is a valid symbol in scope)
			// ty.Module is likely "dbStorage" (alias)
			if modSym, ok := s.Find(ty.Module); ok && modSym.Kind == ModuleSymbol {
				if modType, ok := modSym.Type.(typesystem.TRecord); ok {
					if fieldType, ok := modType.Fields[ty.Name]; ok {
						// Found the exported type!
						// Mark as visited to prevent re-entering this lookup for the same symbol
						visited[ty.Module+"."+ty.Name] = true
						defer delete(visited, ty.Module+"."+ty.Name)

						// If it's a TCon, check if it has underlying
						if tCon, ok := fieldType.(typesystem.TCon); ok {
							if tCon.UnderlyingType != nil {
								// Unwrap!
								return s.resolveTypeAliasWithCycleCheck(tCon.UnderlyingType, visited)
							}
							// Try to look up using the module alias logic below if this fails?
							if tCon.Name == ty.Name {
								// If we found a TCon with same name and no underlying type,
								// and we are trying to resolve it, maybe we should try the package name lookup?
								// Because maybe the TCon in the module record is just a reference, but the alias
								// registration has the underlying type.
							}
						} else {
							// Not a TCon (e.g. TRecord), return it directly
							return fieldType
						}
					}
				}
			}

			// 2. Try to resolve the module alias (if ty.Module is an alias like "dbStorage")
			if packageName, ok := s.GetPackageNameByAlias(ty.Module); ok {
				// Use the ORIGINAL package name for lookup in TypeAliases

				// Try fully qualified original name: "db.DbConfig"
				originalFullName := packageName + "." + ty.Name
				if underlying, ok := s.GetTypeAlias(originalFullName); ok {
					visited[originalFullName] = true
					defer delete(visited, originalFullName)
					return s.resolveTypeAliasWithCycleCheck(underlying, visited)
				}
			}
		}

		return t
	case typesystem.TApp:
		// For parameterized type aliases, check if the constructor is an alias with type params
		// If so, we need to substitute the args into the underlying type, not just recursively resolve
		if tCon, ok := ty.Constructor.(typesystem.TCon); ok {
			// Get the type params for this TCon
			typeParams, hasParams := s.GetTypeParams(tCon.Name)

			// Check if this is a type alias with an underlying type
			lookupName := tCon.Name
			if tCon.Module != "" {
				lookupName = tCon.Module + "." + tCon.Name
			}

			var underlying typesystem.Type
			var hasUnderlying bool

			// Try GetTypeAlias first
			if u, ok := s.GetTypeAlias(lookupName); ok {
				underlying = u
				hasUnderlying = true
			} else if tCon.Module != "" {
				// Try without module prefix
				if u, ok := s.GetTypeAlias(tCon.Name); ok {
					underlying = u
					hasUnderlying = true
				}
			}

			// If we have both type params and an underlying type, perform substitution
			if hasUnderlying && hasParams && len(typeParams) == len(ty.Args) {
				// Cycle detection
				if visited[lookupName] {
					// Return TApp as-is to avoid infinite recursion
					return t
				}
				visited[lookupName] = true
				defer delete(visited, lookupName)

				// Build substitution map: T -> Int, etc.
				subst := make(typesystem.Subst)
				for i, param := range typeParams {
					// Resolve the argument type recursively first
					resolvedArg := s.resolveTypeAliasWithCycleCheck(ty.Args[i], visited)
					subst[param] = resolvedArg
				}

				// Apply substitution to the underlying type
				substituted := underlying.Apply(subst)

				// Recursively resolve the substituted type (in case it contains more aliases)
				return s.resolveTypeAliasWithCycleCheck(substituted, visited)
			}
		}

		// Fallback: Resolve constructor and args recursively (for non-alias TApp)
		resolvedCon := s.resolveTypeAliasWithCycleCheck(ty.Constructor, visited)
		resolvedArgs := make([]typesystem.Type, len(ty.Args))
		for i, arg := range ty.Args {
			resolvedArgs[i] = s.resolveTypeAliasWithCycleCheck(arg, visited)
		}
		return typesystem.TApp{Constructor: resolvedCon, Args: resolvedArgs}
	case typesystem.TFunc:
		// Resolve params and return type
		resolvedParams := make([]typesystem.Type, len(ty.Params))
		for i, p := range ty.Params {
			resolvedParams[i] = s.resolveTypeAliasWithCycleCheck(p, visited)
		}
		return typesystem.TFunc{
			Params:       resolvedParams,
			ReturnType:   s.resolveTypeAliasWithCycleCheck(ty.ReturnType, visited),
			IsVariadic:   ty.IsVariadic,
			DefaultCount: ty.DefaultCount,
			Constraints:  ty.Constraints,
		}
	case typesystem.TTuple:
		resolvedElems := make([]typesystem.Type, len(ty.Elements))
		for i, e := range ty.Elements {
			resolvedElems[i] = s.resolveTypeAliasWithCycleCheck(e, visited)
		}
		return typesystem.TTuple{Elements: resolvedElems}
	case typesystem.TRecord:
		resolvedFields := make(map[string]typesystem.Type)
		for k, v := range ty.Fields {
			resolvedFields[k] = s.resolveTypeAliasWithCycleCheck(v, visited)
		}
		return typesystem.TRecord{Fields: resolvedFields, IsOpen: ty.IsOpen}
	default:
		return t
	}
}

// FindTypeAliasForRecord finds a type alias name that matches the given record structure.
// Returns the alias name and true if found, empty string and false otherwise.
func (s *SymbolTable) FindTypeAliasForRecord(rec typesystem.TRecord) (string, bool) {
	for name, aliasType := range s.typeAliases {
		if aliasRec, ok := aliasType.(typesystem.TRecord); ok {
			if recordsEqual(rec, aliasRec) {
				return name, true
			}
		}
	}
	if s.outer != nil {
		return s.outer.FindTypeAliasForRecord(rec)
	}
	return "", false
}

// FindMatchingRecordAlias searches for a type alias whose underlying type matches the given record structure.
// This enables nominal typing for record type aliases (e.g., type Response = { status: Int, ... }).
// Returns the nominal type (TCon) if found, nil otherwise.
func (s *SymbolTable) FindMatchingRecordAlias(recordType typesystem.TRecord) typesystem.Type {
	// Search in current scope
	for _, sym := range s.store {
		if sym.Kind == TypeSymbol && sym.IsTypeAlias() {
			// Check if underlying type matches the record structure
			if underlying, ok := sym.UnderlyingType.(typesystem.TRecord); ok {
				if recordsEqual(recordType, underlying) {
					// Return the nominal type (TCon)
					return sym.Type
				}
			}
		}
	}

	// Search in outer scope
	if s.outer != nil {
		return s.outer.FindMatchingRecordAlias(recordType)
	}

	return nil
}
