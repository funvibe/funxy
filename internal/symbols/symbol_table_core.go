package symbols

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/typesystem"
)

type SymbolKind int

type ScopeType int

const (
	ScopePrelude ScopeType = iota // Built-in symbols (types, functions, traits)
	ScopeGlobal                   // User code top-level
	ScopeFunction
	ScopeBlock
)

const (
	VariableSymbol SymbolKind = iota
	TypeSymbol
	ConstructorSymbol
	TraitSymbol  // New: Symbol for a Trait (Type Class)
	ModuleSymbol // New: Symbol for a Module
)

type Symbol struct {
	Name           string
	Type           typesystem.Type
	Kind           SymbolKind
	IsPending      bool            // Flag for forward declarations/cyclic dependencies
	IsConstant     bool            // True if defined with :- (immutable)
	UnderlyingType typesystem.Type // For type aliases: the underlying type (e.g., TRecord for type Vector = {...})
	OriginModule   string          // Module where symbol was originally defined (for re-export conflict detection)
	DefinitionNode ast.Node        // The AST node where this symbol was defined (for scoped lookups)
	DefinitionFile string          // The file path where this symbol was defined
	IsTraitMethod  bool            // True if this symbol represents a trait method
}

// GetTypeForUnification returns the underlying type for unification/field access.
// For type aliases, returns UnderlyingType; otherwise returns Type.
func (s Symbol) GetTypeForUnification() typesystem.Type {
	if s.UnderlyingType != nil {
		return s.UnderlyingType
	}
	return s.Type
}

// IsTypeAlias returns true if this symbol is a type alias with an underlying type.
func (s Symbol) IsTypeAlias() bool {
	return s.Kind == TypeSymbol && s.UnderlyingType != nil
}

// InstanceDef represents a registered trait instance
type InstanceDef struct {
	TraitName       string
	TargetTypes     []typesystem.Type
	ConstructorName string                  // Name of the dictionary constructor or global instance
	Requirements    []typesystem.Constraint // Constraints for generic instances
}

type Constraint struct {
	TypeVar string
	Trait   string
}

// RenameTypeVars renames type variables to avoid collisions during Unify checks
func RenameTypeVars(t typesystem.Type, suffix string) typesystem.Type {
	// We can use Apply with a substitution that maps every TVar to TVar + suffix
	vars := t.FreeTypeVariables()
	subst := make(typesystem.Subst)
	for _, v := range vars {
		subst[v.Name] = typesystem.TVar{Name: v.Name + "_" + suffix}
	}
	return t.Apply(subst)
}

// getTypeConstructorName extracts the constructor name from a type
func getTypeConstructorName(t typesystem.Type) string {
	switch tt := t.(type) {
	case typesystem.TCon:
		return tt.Name
	case typesystem.TApp:
		return getTypeConstructorName(tt.Constructor)
	default:
		return ""
	}
}

// recordsEqual checks if two TRecord types are structurally equal
func recordsEqual(a, b typesystem.TRecord) bool {
	if len(a.Fields) != len(b.Fields) {
		return false
	}
	for k, v := range a.Fields {
		bv, ok := b.Fields[k]
		if !ok {
			return false
		}
		// Use Unify to check type equality
		_, err := typesystem.Unify(v, bv)
		if err != nil {
			return false
		}
	}
	return true
}
