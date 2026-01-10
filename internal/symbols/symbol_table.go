package symbols

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/typesystem"
	"strings"
	"sync"
)

type SymbolKind int

type ScopeType int

const (
	ScopePrelude ScopeType = iota // Built-in symbols (types, functions, traits)
	ScopeGlobal                   // User code top-level
	ScopeFunction
	ScopeBlock
)

// Singleton prelude table containing all built-in symbols
var (
	preludeTable *SymbolTable
	preludeOnce  sync.Once
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

	// EvidenceTable maps a lookup key (e.g. "Show<Int>") to the name of the global variable
	// holding the Dictionary instance.
	EvidenceTable map[string]string

	// StrictMode enabled via directive
	StrictMode bool
}

type Constraint struct {
	TypeVar string
	Trait   string
}

func NewEmptySymbolTable() *SymbolTable {
	return &SymbolTable{
		store:               make(map[string]Symbol),
		types:               make(map[string]typesystem.Type),
		scopeType:           ScopeGlobal, // Default to global
		traitMethods:        make(map[string]string),
		traitTypeParams:     make(map[string][]string),
		traitTypeParamKinds: make(map[string]map[string]typesystem.Kind),
		traitSuperTraits:    make(map[string][]string),
		traitDefaultMethods: make(map[string]map[string]bool),
		traitAllMethods:     make(map[string][]string),
		operatorTraits:      make(map[string]string),
		implementations:     make(map[string][]InstanceDef),
		instanceMethods:     make(map[string]map[string]map[string]typesystem.Type),
		extensionMethods:    make(map[string]map[string]typesystem.Type),
		genericTypeParams:   make(map[string][]string),
		funcConstraints:     make(map[string][]Constraint),
		variants:            make(map[string][]string),
		kinds:               make(map[string]typesystem.Kind),
		moduleAliases:       make(map[string]string),
		typeAliases:         make(map[string]typesystem.Type),
		TraitMethodIndices:  make(map[string]map[string]int),
		EvidenceTable:       make(map[string]string),
	}
}

// GetPrelude returns the singleton prelude SymbolTable containing all built-in symbols.
// This table is shared across all compilation units.
func GetPrelude() *SymbolTable {
	preludeOnce.Do(func() {
		preludeTable = NewEmptySymbolTable()
		preludeTable.scopeType = ScopePrelude
		preludeTable.InitBuiltins()
	})
	return preludeTable
}

// NewSymbolTable creates a new user-level SymbolTable with prelude as parent scope.
// Built-in symbols are accessible via scope chain but cannot be redefined.
func NewSymbolTable() *SymbolTable {
	prelude := GetPrelude()
	st := NewEmptySymbolTable()
	st.outer = prelude
	st.scopeType = ScopeGlobal
	return st
}

// ResetPrelude resets the prelude singleton (for testing only).
func ResetPrelude() {
	preludeOnce = sync.Once{}
	preludeTable = nil
}

func (st *SymbolTable) InitBuiltins() {
	const prelude = "prelude" // Origin for built-in symbols

	// Define built-in types
	st.DefineType("Int", typesystem.TCon{Name: "Int"}, prelude)
	st.RegisterKind("Int", typesystem.Star)
	st.DefineType("Bool", typesystem.TCon{Name: "Bool"}, prelude)
	st.RegisterKind("Bool", typesystem.Star)
	st.DefineType("Float", typesystem.TCon{Name: "Float"}, prelude)
	st.RegisterKind("Float", typesystem.Star)
	st.DefineType("Char", typesystem.TCon{Name: "Char"}, prelude)
	st.RegisterKind("Char", typesystem.Star)
	st.DefineType(config.ListTypeName, typesystem.TCon{Name: config.ListTypeName}, prelude)
	st.RegisterKind(config.ListTypeName, typesystem.KArrow{Left: typesystem.Star, Right: typesystem.Star})
	// Map :: * -> * -> *
	st.DefineType(config.MapTypeName, typesystem.TCon{Name: config.MapTypeName}, prelude)
	st.RegisterKind(config.MapTypeName, typesystem.MakeArrow(typesystem.Star, typesystem.Star, typesystem.Star))
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{typesystem.TCon{Name: "Char"}},
	}
	st.DefineType("String", stringType, prelude)
	// Register as alias so matchTypeWithSubst can resolve it
	st.RegisterTypeAlias("String", stringType)
	st.RegisterKind("String", typesystem.Star)

	// Bytes and Bits are built-in types (available in prelude)
	st.DefineType("Bytes", typesystem.TCon{Name: "Bytes"}, prelude)
	st.RegisterKind("Bytes", typesystem.Star)
	st.DefineType("Bits", typesystem.TCon{Name: "Bits"}, prelude)
	st.RegisterKind("Bits", typesystem.Star)

	// Built-in ADTs for error handling
	// type Result e t = Ok t | Fail e  (like Haskell's Either e a)
	// E is error type (first), T is success type (last) - so Functor/Monad operate on T
	st.DefineType(config.ResultTypeName, typesystem.TCon{Name: config.ResultTypeName}, prelude)
	// Result :: * -> * -> *
	st.RegisterKind(config.ResultTypeName, typesystem.MakeArrow(typesystem.Star, typesystem.Star, typesystem.Star))
	st.DefineConstructor(config.OkCtorName, typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.TVar{Name: "t"}},
		ReturnType: typesystem.TApp{Constructor: typesystem.TCon{Name: config.ResultTypeName}, Args: []typesystem.Type{typesystem.TVar{Name: "e"}, typesystem.TVar{Name: "t"}}},
	}, prelude)
	st.DefineConstructor(config.FailCtorName, typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.TVar{Name: "e"}},
		ReturnType: typesystem.TApp{Constructor: typesystem.TCon{Name: config.ResultTypeName}, Args: []typesystem.Type{typesystem.TVar{Name: "e"}, typesystem.TVar{Name: "t"}}},
	}, prelude)
	st.RegisterVariant(config.ResultTypeName, config.OkCtorName)
	st.RegisterVariant(config.ResultTypeName, config.FailCtorName)

	// type Option t = Some t | Zero
	st.DefineType(config.OptionTypeName, typesystem.TCon{Name: config.OptionTypeName}, prelude)
	// Option :: * -> *
	st.RegisterKind(config.OptionTypeName, typesystem.KArrow{Left: typesystem.Star, Right: typesystem.Star})
	st.DefineConstructor(config.SomeCtorName, typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.TVar{Name: "t"}},
		ReturnType: typesystem.TApp{Constructor: typesystem.TCon{Name: config.OptionTypeName}, Args: []typesystem.Type{typesystem.TVar{Name: "t"}}},
	}, prelude)
	st.DefineConstructor(config.ZeroCtorName, typesystem.TApp{Constructor: typesystem.TCon{Name: config.OptionTypeName}, Args: []typesystem.Type{typesystem.TVar{Name: "t"}}}, prelude)
	st.RegisterVariant(config.OptionTypeName, config.SomeCtorName)
	st.RegisterVariant(config.OptionTypeName, config.ZeroCtorName)

	// Reader :: * -> * -> *
	st.DefineType("Reader", typesystem.TCon{Name: "Reader"}, prelude)
	st.RegisterKind("Reader", typesystem.MakeArrow(typesystem.Star, typesystem.Star, typesystem.Star))

	// Identity :: * -> *
	st.DefineType("Identity", typesystem.TCon{Name: "Identity"}, prelude)
	st.RegisterKind("Identity", typesystem.KArrow{Left: typesystem.Star, Right: typesystem.Star})

	// State :: * -> * -> *
	st.DefineType("State", typesystem.TCon{Name: "State"}, prelude)
	st.RegisterKind("State", typesystem.MakeArrow(typesystem.Star, typesystem.Star, typesystem.Star))

	// Writer :: * -> * -> *
	st.DefineType("Writer", typesystem.TCon{Name: "Writer"}, prelude)
	st.RegisterKind("Writer", typesystem.MakeArrow(typesystem.Star, typesystem.Star, typesystem.Star))

	// Note: SqlValue, Uuid, Logger, Task, Date types are registered via virtual packages on import
	// They are NOT available without importing the corresponding lib/* package
}

func NewEnclosedSymbolTable(outer *SymbolTable, scopeType ScopeType) *SymbolTable {
	st := NewEmptySymbolTable()
	st.outer = outer
	st.scopeType = scopeType
	// Inherit registries references or copy?
	// Trait definitions are global, so we can lookup in outer.
	// But defining new ones? Usually global.
	return st
}

// Outer returns the outer scope symbol table
func (s *SymbolTable) Outer() *SymbolTable {
	return s.outer
}

// IsFunctionScope returns true if this symbol table corresponds to a function scope.
func (s *SymbolTable) IsFunctionScope() bool {
	return s.scopeType == ScopeFunction
}

// IsGlobalScope returns true if this symbol table is the root (global) scope.
func (s *SymbolTable) IsGlobalScope() bool {
	return s.scopeType == ScopeGlobal
}

// Parent returns the outer symbol table (if any)
func (s *SymbolTable) Parent() *SymbolTable {
	return s.outer
}

func (s *SymbolTable) DefineModule(name string, moduleType typesystem.Type) {
	s.store[name] = Symbol{Name: name, Type: moduleType, Kind: ModuleSymbol}
}

// RegisterModuleAlias stores mapping from alias to package name
// Used for looking up extension methods in source modules
func (s *SymbolTable) RegisterModuleAlias(alias, packageName string) {
	s.moduleAliases[alias] = packageName
}

// GetPackageNameByAlias returns the package name for a given module alias
func (s *SymbolTable) GetPackageNameByAlias(alias string) (string, bool) {
	name, ok := s.moduleAliases[alias]
	if !ok && s.outer != nil {
		return s.outer.GetPackageNameByAlias(alias)
	}
	return name, ok
}

func (s *SymbolTable) DefinePending(name string, t typesystem.Type, origin string) {
	s.store[name] = Symbol{Name: name, Type: t, Kind: VariableSymbol, IsPending: true, IsConstant: false, OriginModule: origin}
}

func (s *SymbolTable) DefinePendingConstant(name string, t typesystem.Type, origin string) {
	s.store[name] = Symbol{Name: name, Type: t, Kind: VariableSymbol, IsPending: true, IsConstant: true, OriginModule: origin}
}

func (s *SymbolTable) DefineTypePending(name string, t typesystem.Type, origin string) {
	s.types[name] = t
	s.store[name] = Symbol{Name: name, Type: t, Kind: TypeSymbol, IsPending: true, OriginModule: origin}
}

func (s *SymbolTable) DefinePendingTrait(name string, origin string) {
	s.store[name] = Symbol{Name: name, Type: nil, Kind: TraitSymbol, IsPending: true, OriginModule: origin}
	s.implementations[name] = []InstanceDef{}
}

func (s *SymbolTable) Define(name string, t typesystem.Type, origin string) {
	s.store[name] = Symbol{Name: name, Type: t, Kind: VariableSymbol, IsConstant: false, OriginModule: origin}
}

// SetDefinitionNode updates the DefinitionNode for an existing symbol in the current scope
func (s *SymbolTable) SetDefinitionNode(name string, node ast.Node) {
	if sym, ok := s.store[name]; ok {
		sym.DefinitionNode = node
		s.store[name] = sym
	}
}

func (s *SymbolTable) DefineConstant(name string, t typesystem.Type, origin string) {
	s.store[name] = Symbol{Name: name, Type: t, Kind: VariableSymbol, IsConstant: true, OriginModule: origin}
}

func (s *SymbolTable) DefineType(name string, t typesystem.Type, origin string) {
	s.types[name] = t
	s.store[name] = Symbol{Name: name, Type: t, Kind: TypeSymbol, OriginModule: origin}
}

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

func (s *SymbolTable) DefineConstructor(name string, t typesystem.Type, origin string) {
	s.store[name] = Symbol{Name: name, Type: t, Kind: ConstructorSymbol, OriginModule: origin}
}

func (s *SymbolTable) DefineTrait(name string, typeParams []string, superTraits []string, origin string) {
	s.store[name] = Symbol{Name: name, Type: nil, Kind: TraitSymbol, OriginModule: origin}
	s.traitTypeParams[name] = typeParams
	s.traitSuperTraits[name] = superTraits
	// Only initialize implementations list if it doesn't exist yet
	// This prevents losing implementations when trait is re-defined during multi-pass analysis
	if _, exists := s.implementations[name]; !exists {
		s.implementations[name] = []InstanceDef{}
	}
}

func (s *SymbolTable) RegisterTraitTypeParamKind(traitName, paramName string, k typesystem.Kind) {
	if s.traitTypeParamKinds[traitName] == nil {
		s.traitTypeParamKinds[traitName] = make(map[string]typesystem.Kind)
	}
	s.traitTypeParamKinds[traitName][paramName] = k
}

func (s *SymbolTable) GetTraitTypeParamKind(traitName, paramName string) (typesystem.Kind, bool) {
	if kinds, ok := s.traitTypeParamKinds[traitName]; ok {
		if k, ok := kinds[paramName]; ok {
			return k, true
		}
	}
	if s.outer != nil {
		return s.outer.GetTraitTypeParamKind(traitName, paramName)
	}
	return nil, false
}

func (s *SymbolTable) GetTraitSuperTraits(name string) ([]string, bool) {
	t, ok := s.traitSuperTraits[name]
	if !ok && s.outer != nil {
		return s.outer.GetTraitSuperTraits(name)
	}
	return t, ok
}

// TraitExists checks if a trait is defined in this scope or any outer scope
func (s *SymbolTable) TraitExists(name string) bool {
	if _, ok := s.implementations[name]; ok {
		return true
	}
	if s.outer != nil {
		return s.outer.TraitExists(name)
	}
	return false
}

func (s *SymbolTable) RegisterTraitMethod(methodName, traitName string, t typesystem.Type, origin string) {
	s.traitMethods[methodName] = traitName
	// Define method as a constant function in the scope so it can be called but not reassigned
	s.DefineConstant(methodName, t, origin)
}

func (s *SymbolTable) RegisterTraitDefaultMethod(traitName, methodName string) {
	if s.traitDefaultMethods[traitName] == nil {
		s.traitDefaultMethods[traitName] = make(map[string]bool)
	}
	s.traitDefaultMethods[traitName][methodName] = true
}

func (s *SymbolTable) HasTraitDefaultMethod(traitName, methodName string) bool {
	if methods, ok := s.traitDefaultMethods[traitName]; ok {
		return methods[methodName]
	}
	if s.outer != nil {
		return s.outer.HasTraitDefaultMethod(traitName, methodName)
	}
	return false
}

func (s *SymbolTable) RegisterTraitMethod2(traitName, methodName string) {
	s.traitAllMethods[traitName] = append(s.traitAllMethods[traitName], methodName)
}

func (s *SymbolTable) GetTraitAllMethods(traitName string) []string {
	if methods, ok := s.traitAllMethods[traitName]; ok {
		return methods
	}
	if s.outer != nil {
		return s.outer.GetTraitAllMethods(traitName)
	}
	return nil
}

func (s *SymbolTable) GetTraitRequiredMethods(traitName string) []string {
	// Returns methods that DON'T have default implementations
	allMethods := s.GetTraitAllMethods(traitName)
	var required []string
	for _, method := range allMethods {
		if !s.HasTraitDefaultMethod(traitName, method) {
			required = append(required, method)
		}
	}
	return required
}

// GetTraitMethodType returns the generic type signature of a trait method
func (s *SymbolTable) GetTraitMethodType(methodName string) (typesystem.Type, bool) {
	if sym, ok := s.store[methodName]; ok && sym.Type != nil {
		return sym.Type, true
	}
	if s.outer != nil {
		return s.outer.GetTraitMethodType(methodName)
	}
	return nil, false
}

// GetOptionalUnwrapReturnType returns the return type of unwrap for a specific type.
// For a type F<A, ...> implementing Optional, this returns A (the "inner" type).
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

// RegisterOperatorTrait associates an operator with a trait
// e.g. RegisterOperatorTrait("+", "Add")
func (s *SymbolTable) RegisterOperatorTrait(operator, traitName string) {
	s.operatorTraits[operator] = traitName
}

// GetTraitForOperator returns the trait name for an operator
// e.g. GetTraitForOperator("+") returns "Add"
func (s *SymbolTable) GetTraitForOperator(operator string) (string, bool) {
	t, ok := s.operatorTraits[operator]
	if !ok && s.outer != nil {
		return s.outer.GetTraitForOperator(operator)
	}
	return t, ok
}

// GetAllOperatorTraits returns a copy of all operator -> trait mappings
func (s *SymbolTable) GetAllOperatorTraits() map[string]string {
	result := make(map[string]string)

	// Get from outer first (so inner scope overrides)
	if s.outer != nil {
		for k, v := range s.outer.GetAllOperatorTraits() {
			result[k] = v
		}
	}

	// Overlay current scope
	for k, v := range s.operatorTraits {
		result[k] = v
	}

	return result
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

// RegisterInstanceMethod stores a specialized method signature for a trait instance.
// For example, for instance Optional<Result<T, E>>, we store:
//
//	unwrap: Result<T, E> -> T
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

// FindWithScope returns the symbol and the scope where it was defined
func (s *SymbolTable) FindWithScope(name string) (Symbol, *SymbolTable, bool) {
	sym, ok := s.store[name]
	if ok {
		return sym, s, true
	}
	if s.outer != nil {
		return s.outer.FindWithScope(name)
	}
	return Symbol{}, nil, false
}

func (s *SymbolTable) Find(name string) (Symbol, bool) {
	sym, _, ok := s.FindWithScope(name)
	return sym, ok
}

// All returns all symbols in the current scope (not including outer scopes).
// Used for iterating over symbols, e.g., for re-export resolution.
func (s *SymbolTable) All() map[string]Symbol {
	return s.store
}

func (s *SymbolTable) ResolveType(name string) (typesystem.Type, bool) {
	// Handle Qualified Types (e.g. math.Vector)
	if strings.Contains(name, ".") {
		parts := strings.SplitN(name, ".", 2)
		if len(parts) == 2 {
			moduleName := parts[0]
			typeName := parts[1]

			// Look up module symbol
			if sym, ok := s.Find(moduleName); ok {
				// Check if it's a module and has exported type
				// TRecord is a value type, not pointer
				if rec, ok := sym.Type.(typesystem.TRecord); ok {
					// For Module exports, we stored both Values and Types in TRecord Fields.
					// If the field is a Type (e.g. TCon), it represents an exported Type.
					if exportedType, ok := rec.Fields[typeName]; ok {
						// If the exported type is a TCon (placeholder), try to resolve it further
						if tCon, isTCon := exportedType.(typesystem.TCon); isTCon {
							// Try to find the actual type definition (recursively via ResolveType)
							if actualType, ok := s.ResolveType(tCon.Name); ok {
								// Avoid infinite recursion - only use if different
								if _, stillTCon := actualType.(typesystem.TCon); !stillTCon {
									return actualType, true
								}
							}
							// Check if there's a type alias for this type
							if aliasType, ok := s.GetTypeAlias(typeName); ok {
								return aliasType, true
							}
							// Also check with module-qualified name
							if aliasType, ok := s.GetTypeAlias(moduleName + "." + typeName); ok {
								return aliasType, true
							}
						}
						return exportedType, true
					}
				}
			}
		}
	}

	t, ok := s.types[name]
	if !ok && s.outer != nil {
		return s.outer.ResolveType(name)
	}
	return t, ok
}

func (s *SymbolTable) IsDefined(name string) bool {
	_, ok := s.store[name]
	if !ok && s.outer != nil {
		return s.outer.IsDefined(name)
	}
	return ok
}

// IsDefinedLocally checks if a symbol is defined in the current scope (shallow check)
func (s *SymbolTable) IsDefinedLocally(name string) bool {
	_, ok := s.store[name]
	return ok
}

// Update updates the type of an existing symbol.
// It searches up the scope chain and updates the symbol where it is defined.
// Returns error if symbol not found.
func (s *SymbolTable) Update(name string, t typesystem.Type) error {
	if sym, ok := s.store[name]; ok {
		sym.Type = t
		s.store[name] = sym
		return nil
	}
	if s.outer != nil {
		return s.outer.Update(name, t)
	}
	return typesystem.NewSymbolNotFoundError(name)
}

func (s *SymbolTable) GetTraitForMethod(methodName string) (string, bool) {
	t, ok := s.traitMethods[methodName]
	if !ok && s.outer != nil {
		return s.outer.GetTraitForMethod(methodName)
	}
	return t, ok
}

func (s *SymbolTable) GetTraitTypeParams(traitName string) ([]string, bool) {
	t, ok := s.traitTypeParams[traitName]
	if !ok && s.outer != nil {
		return s.outer.GetTraitTypeParams(traitName)
	}
	return t, ok
}

// IsHKTTrait checks if a trait requires higher-kinded types.
// A trait is HKT if its type parameter is applied to arguments in method signatures.
// Example: Functor<F> with fmap(f, F<A>) -> F<B> is HKT because F is used as F<A>.
func (s *SymbolTable) IsHKTTrait(traitName string) bool {
	// Get trait's type parameters (e.g., ["F"] for Functor)
	typeParams, ok := s.GetTraitTypeParams(traitName)
	if !ok || len(typeParams) == 0 {
		return false
	}

	// Get trait's method names
	methodNames := s.GetTraitAllMethods(traitName)
	if len(methodNames) == 0 {
		return false
	}

	// Check each method's type signature for HKT pattern
	for _, methodName := range methodNames {
		if sym, ok := s.Find(methodName); ok {
			if containsAppliedTypeParam(sym.Type, typeParams) {
				return true
			}
		}
	}

	return false
}

// containsAppliedTypeParam checks if a type contains a type parameter applied to arguments.
// e.g., F<A> where F is in typeParams returns true.
func containsAppliedTypeParam(t typesystem.Type, typeParams []string) bool {
	if t == nil {
		return false
	}

	switch typ := t.(type) {
	case typesystem.TApp:
		// Check if constructor is one of the type params
		if tvar, ok := typ.Constructor.(typesystem.TVar); ok {
			for _, tp := range typeParams {
				if tvar.Name == tp {
					return true // Found F<...> pattern
				}
			}
		}
		if tcon, ok := typ.Constructor.(typesystem.TCon); ok {
			for _, tp := range typeParams {
				if tcon.Name == tp {
					return true // Found F<...> pattern (rigid type param)
				}
			}
		}
		// Recursively check args
		for _, arg := range typ.Args {
			if containsAppliedTypeParam(arg, typeParams) {
				return true
			}
		}
		return containsAppliedTypeParam(typ.Constructor, typeParams)

	case typesystem.TFunc:
		// Check params and return type
		for _, param := range typ.Params {
			if containsAppliedTypeParam(param, typeParams) {
				return true
			}
		}
		return containsAppliedTypeParam(typ.ReturnType, typeParams)

	case typesystem.TTuple:
		for _, elem := range typ.Elements {
			if containsAppliedTypeParam(elem, typeParams) {
				return true
			}
		}
		return false

	default:
		return false
	}
}

func (s *SymbolTable) RegisterExtensionMethod(typeName, methodName string, t typesystem.Type) {
	if _, ok := s.extensionMethods[typeName]; !ok {
		s.extensionMethods[typeName] = make(map[string]typesystem.Type)
	}
	s.extensionMethods[typeName][methodName] = t
}

func (s *SymbolTable) GetExtensionMethod(typeName, methodName string) (typesystem.Type, bool) {
	if methods, ok := s.extensionMethods[typeName]; ok {
		if t, ok := methods[methodName]; ok {
			return t, true
		}
	}
	if s.outer != nil {
		return s.outer.GetExtensionMethod(typeName, methodName)
	}
	return nil, false
}

// GetAllExtensionMethods returns all extension methods from this scope
// Returns map[typeName]map[methodName]Type
func (s *SymbolTable) GetAllExtensionMethods() map[string]map[string]typesystem.Type {
	result := make(map[string]map[string]typesystem.Type)

	// Get from outer first
	if s.outer != nil {
		for typeName, methods := range s.outer.GetAllExtensionMethods() {
			result[typeName] = make(map[string]typesystem.Type)
			for methodName, t := range methods {
				result[typeName][methodName] = t
			}
		}
	}

	// Overlay current level
	for typeName, methods := range s.extensionMethods {
		if result[typeName] == nil {
			result[typeName] = make(map[string]typesystem.Type)
		}
		for methodName, t := range methods {
			result[typeName][methodName] = t
		}
	}

	return result
}

func (s *SymbolTable) RegisterTypeParams(typeName string, params []string) {
	s.genericTypeParams[typeName] = params
}

func (s *SymbolTable) GetTypeParams(typeName string) ([]string, bool) {
	params, ok := s.genericTypeParams[typeName]
	if !ok && s.outer != nil {
		return s.outer.GetTypeParams(typeName)
	}
	return params, ok
}

func (s *SymbolTable) RegisterFuncConstraints(funcName string, constraints []Constraint) {
	s.funcConstraints[funcName] = constraints
}

func (s *SymbolTable) GetFuncConstraints(funcName string) ([]Constraint, bool) {
	c, ok := s.funcConstraints[funcName]
	if !ok && s.outer != nil {
		return s.outer.GetFuncConstraints(funcName)
	}
	return c, ok
}

func (s *SymbolTable) RegisterKind(typeName string, k typesystem.Kind) {
	s.kinds[typeName] = k
}

func (s *SymbolTable) GetKind(typeName string) (typesystem.Kind, bool) {
	k, ok := s.kinds[typeName]
	if !ok && s.outer != nil {
		return s.outer.GetKind(typeName)
	}
	return k, ok
}

func (s *SymbolTable) RegisterVariant(typeName, constructorName string) {
	s.variants[typeName] = append(s.variants[typeName], constructorName)
}

func (s *SymbolTable) GetVariants(typeName string) ([]string, bool) {
	v, ok := s.variants[typeName]
	if !ok && s.outer != nil {
		return s.outer.GetVariants(typeName)
	}
	return v, ok
}

// GetAllNames returns all symbol names in scope (for error suggestions)
func (s *SymbolTable) GetAllNames() []string {
	seen := make(map[string]bool)
	var names []string

	for name := range s.store {
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}

	if s.outer != nil {
		for _, name := range s.outer.GetAllNames() {
			if !seen[name] {
				names = append(names, name)
				seen[name] = true
			}
		}
	}
	return names
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

func (s *SymbolTable) RegisterTraitMethodIndex(traitName, methodName string, index int) {
	if s.TraitMethodIndices[traitName] == nil {
		s.TraitMethodIndices[traitName] = make(map[string]int)
	}
	s.TraitMethodIndices[traitName][methodName] = index
}

func (s *SymbolTable) GetTraitMethodIndex(traitName, methodName string) (int, bool) {
	if indices, ok := s.TraitMethodIndices[traitName]; ok {
		if idx, ok := indices[methodName]; ok {
			return idx, true
		}
	}
	if s.outer != nil {
		return s.outer.GetTraitMethodIndex(traitName, methodName)
	}
	return -1, false
}

func (s *SymbolTable) RegisterEvidence(key string, varName string) {
	s.EvidenceTable[key] = varName
}

func (s *SymbolTable) GetEvidence(key string) (string, bool) {
	if name, ok := s.EvidenceTable[key]; ok {
		return name, true
	}
	if s.outer != nil {
		return s.outer.GetEvidence(key)
	}
	return "", false
}

// Ensure Dictionary type is available
func (s *SymbolTable) InitDictionaryType() {
	s.DefineType("Dictionary", typesystem.TCon{Name: "Dictionary"}, "prelude")
	s.RegisterKind("Dictionary", typesystem.Star)
}

// FreeTypeVariables returns a set of all type variables free in the symbol table (and parents)
func (s *SymbolTable) FreeTypeVariables() map[typesystem.TVar]bool {
	free := make(map[typesystem.TVar]bool)
	s.collectFreeTypeVariables(free)
	return free
}

func (s *SymbolTable) collectFreeTypeVariables(free map[typesystem.TVar]bool) {
	for _, sym := range s.store {
		if sym.Type != nil {
			for _, tv := range sym.Type.FreeTypeVariables() {
				free[tv] = true
			}
		}
	}
	if s.outer != nil {
		s.outer.collectFreeTypeVariables(free)
	}
}

func (s *SymbolTable) SetStrictMode(enabled bool) {
	s.StrictMode = enabled
}

func (s *SymbolTable) IsStrictMode() bool {
	if s.StrictMode {
		return true
	}
	if s.outer != nil {
		return s.outer.IsStrictMode()
	}
	return false
}
