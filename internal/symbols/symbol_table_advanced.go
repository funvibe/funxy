package symbols

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/typesystem"
)

// SymbolTable struct definition - moved from original file
type SymbolTable struct {
	store     map[string]Symbol
	types     map[string]typesystem.Type
	outer     *SymbolTable
	scopeType ScopeType // Type of this scope

	// Trait methods registry: MethodName -> TraitName
	// e.g. "show" -> "Show"
	traitMethods map[string]string

	// Trait type parameter registry: TraitName -> TypeParamNames
	// e.g. "Show" -> ["a"]
	traitTypeParams map[string][]string

	// Trait Functional Dependencies: TraitName -> [FunDeps]
	traitFunctionalDependencies map[string][]ast.FunctionalDependency

	// Trait type parameter kinds: TraitName -> ParamName -> Kind
	traitTypeParamKinds map[string]map[string]typesystem.Kind

	// Trait inheritance registry: TraitName -> [SuperTraitName]
	// e.g. "Order" -> ["Equal"]
	traitSuperTraits map[string][]string

	// Trait default implementations: TraitName -> MethodName -> has default
	// e.g. "Equal" -> "notEqual" -> true
	traitDefaultMethods map[string]map[string]bool

	// All methods of a trait: TraitName -> [MethodNames]
	// e.g. "Equal" -> ["equal", "notEqual"]
	traitAllMethods map[string][]string

	// Operator -> Trait registry: Operator -> TraitName
	// e.g. "+" -> "Add", "==" -> "Equal"
	operatorTraits map[string]string

	// Implementations registry: TraitName -> [InstanceDef]
	implementations map[string][]InstanceDef

	// Instance method signatures: TraitName -> TypeName -> MethodName -> Type
	// Stores specialized method signatures for each instance
	instanceMethods map[string]map[string]map[string]typesystem.Type

	// Extension Methods registry: TypeName -> MethodName -> FuncType
	extensionMethods map[string]map[string]typesystem.Type

	// Generic Type Parameters registry: TypeName -> ParamNames
	// Stores type parameters for generic types (aliases and ADTs) to allow correct instantiation.
	genericTypeParams map[string][]string

	// Function Constraints registry: FuncName -> Constraints
	funcConstraints map[string][]Constraint

	// ADT Variants: TypeName -> [ConstructorNames]
	variants map[string][]string

	// Kinds registry: TypeName -> Kind
	kinds map[string]typesystem.Kind

	// Module alias to package name mapping: alias -> packageName
	// Used for looking up extension methods in source modules
	moduleAliases map[string]string

	// Type aliases: TypeName -> underlying type
	// For type alias `type Vector = { x: Int, y: Int }`, stores Vector -> TRecord
	// The main types map stores TCon{Name: "Vector"} for proper module tagging
	typeAliases map[string]typesystem.Type

	// TraitMethodIndices maps TraitName -> MethodName -> Index in the Dictionary.
	// Used for O(1) method lookup from a Dictionary.
	TraitMethodIndices map[string]map[string]int

	// TraitMethodDispatch maps TraitName -> MethodName -> DispatchStrategy.
	// Stores the strategy for finding the type parameters for method dispatch.
	TraitMethodDispatch map[string]map[string][]typesystem.DispatchSource

	// EvidenceTable maps a lookup key (e.g. "Show<Int>") to the name of the global variable
	// holding the Dictionary instance.
	EvidenceTable map[string]string

	// StrictMode enabled via directive
	StrictMode bool
}
