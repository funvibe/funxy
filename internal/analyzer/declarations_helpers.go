package analyzer

import (
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

// renameConflictingTypeVars renames type variables in `t` that conflict with `conflictNames`.
// This is needed when creating substitutions for trait instances where the target type
// might have type variables with the same name as the trait's type parameters.
// For example: `instance UserOpChoose Box<T>` where trait UserOpChoose<T> - the T in Box<T>
// should not be confused with the trait's T parameter.
func renameConflictingTypeVars(t typesystem.Type, conflictNames []string, ctx *InferenceContext) typesystem.Type {
	if t == nil || ctx == nil {
		return t
	}

	// Find conflicting type variables in t
	freeVars := t.FreeTypeVariables()
	conflictSet := make(map[string]bool)
	for _, name := range conflictNames {
		conflictSet[name] = true
	}

	// Create renaming substitution for conflicting vars
	renameSubst := typesystem.Subst{}
	for _, tv := range freeVars {
		if conflictSet[tv.Name] {
			// Rename to a fresh variable
			fresh := ctx.FreshVar()
			renameSubst[tv.Name] = fresh
		}
	}

	if len(renameSubst) == 0 {
		return t // No conflicts
	}

	return t.Apply(renameSubst)
}

// importTraitImplementations copies trait implementations for exported types
// from imported module's symbol table to current symbol table
func (w *walker) importTraitImplementations(importedTable *symbols.SymbolTable, exportedTypes map[string]bool, moduleName string) {
	// Get all implementations from imported module
	allImpls := importedTable.GetAllImplementations()

	// For each trait, copy implementations for exported types
	for traitName, impls := range allImpls {
		for _, def := range impls {
			// Check if this implementation should be imported
			// We import implementations for types that are either:
			// 1. Exported types (by name)
			// 2. Types that match exported type structures (for aliases)

			shouldImport := true

			if shouldImport {
				// Tag the types with module name if they are named types
				taggedArgs := make([]typesystem.Type, len(def.TargetTypes))
				for i, arg := range def.TargetTypes {
					taggedArgs[i] = tagModule(arg, moduleName, exportedTypes)
				}
				// Check if trait is defined in current scope (either imported or qualified)
				actualTraitName := traitName
				if !w.symbolTable.IsDefined(actualTraitName) {
					// Try qualified name
					qualifiedName := moduleName + "." + traitName
					if w.symbolTable.IsDefined(qualifiedName) {
						actualTraitName = qualifiedName
					} else {
						// Trait not found in current scope - cannot register implementation
						// This can happen if the trait itself wasn't imported
						continue
					}
				}

				// Register the implementation in current table
				_ = w.symbolTable.RegisterImplementation(actualTraitName, taggedArgs, def.Requirements, def.ConstructorName)

				// Import the evidence constant symbol so SolveWitness can find it
				// We register it as an imported symbol mapping local name to remote name
				w.symbolTable.DefineConstant(def.ConstructorName, typesystem.TCon{Name: "Dictionary"}, moduleName)
			}
		}
	}
}

// shouldImportTraitImplementation determines if a trait implementation should be imported
func (w *walker) shouldImportTraitImplementation(implType typesystem.Type, exportedTypes map[string]bool, moduleName string) bool {
	switch t := implType.(type) {
	case typesystem.TCon:
		// Import if the type name is exported
		return exportedTypes[t.Name]
	case typesystem.TRecord:
		// Import record types (for type aliases that resolve to records)
		return true
	case typesystem.TApp:
		// For type applications (like List<T>), check the constructor
		if tCon, ok := t.Constructor.(typesystem.TCon); ok {
			return exportedTypes[tCon.Name]
		}
	}

	// Default: don't import
	return false
}

// importExtensionMethods copies extension methods for exported types
// from imported module's symbol table to current symbol table
func (w *walker) importExtensionMethods(importedTable *symbols.SymbolTable, exportedTypes map[string]bool) {
	allExtMethods := importedTable.GetAllExtensionMethods()

	for typeName, methods := range allExtMethods {
		// Only import extension methods for exported types
		if !exportedTypes[typeName] {
			continue
		}

		for methodName, methodType := range methods {
			w.symbolTable.RegisterExtensionMethod(typeName, methodName, methodType)
		}
	}
}
