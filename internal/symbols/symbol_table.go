package symbols

import (
	"fmt"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/typesystem"
	"strings"
)

type SymbolKind int

const (
	VariableSymbol SymbolKind = iota
	TypeSymbol
	ConstructorSymbol
	TraitSymbol // New: Symbol for a Trait (Type Class)
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

type SymbolTable struct {
	store    map[string]Symbol
	types    map[string]typesystem.Type
	outer    *SymbolTable

	// Trait methods registry: MethodName -> TraitName
	// e.g. "show" -> "Show"
	traitMethods map[string]string

	// Trait type parameter registry: TraitName -> TypeParamNames
	// e.g. "Show" -> ["a"]
	traitTypeParams map[string][]string

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

	// Implementations registry: TraitName -> [Type]
	implementations map[string][]typesystem.Type

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
}

type Constraint struct {
	TypeVar string
	Trait   string
}

func NewEmptySymbolTable() *SymbolTable {
	return &SymbolTable{
		store:               make(map[string]Symbol),
		types:               make(map[string]typesystem.Type),
		traitMethods:        make(map[string]string),
		traitTypeParams:     make(map[string][]string),
		traitSuperTraits:    make(map[string][]string),
		traitDefaultMethods: make(map[string]map[string]bool),
		traitAllMethods:     make(map[string][]string),
		operatorTraits:      make(map[string]string),
		implementations:     make(map[string][]typesystem.Type),
		instanceMethods:     make(map[string]map[string]map[string]typesystem.Type),
		extensionMethods:    make(map[string]map[string]typesystem.Type),
		genericTypeParams:   make(map[string][]string),
		funcConstraints:     make(map[string][]Constraint),
		variants:            make(map[string][]string),
		kinds:               make(map[string]typesystem.Kind),
		moduleAliases:       make(map[string]string),
		typeAliases:         make(map[string]typesystem.Type),
	}
}

func NewSymbolTable() *SymbolTable {
	st := NewEmptySymbolTable()
	st.InitBuiltins()
	return st
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
	st.RegisterKind("String", typesystem.Star)

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

	// Note: SqlValue, Uuid, Logger, Task, Date types are registered via virtual packages on import
	// They are NOT available without importing the corresponding lib/* package
}

func NewEnclosedSymbolTable(outer *SymbolTable) *SymbolTable {
	st := NewEmptySymbolTable()
	st.outer = outer
	// Inherit registries references or copy?
	// Trait definitions are global, so we can lookup in outer.
	// But defining new ones? Usually global.
	return st
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

func (s *SymbolTable) Define(name string, t typesystem.Type, origin string) {
	s.store[name] = Symbol{Name: name, Type: t, Kind: VariableSymbol, IsConstant: false, OriginModule: origin}
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
	s.store[name] = Symbol{
		Name:           name,
		Type:           nominalType,      // TCon for trait lookup
		Kind:           TypeSymbol,
		UnderlyingType: underlyingType,   // TRecord for field access
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
	switch ty := t.(type) {
	case typesystem.TCon:
		// Check if this TCon is a type alias
		if underlying, ok := s.GetTypeAlias(ty.Name); ok {
			// Recursively resolve the underlying type
			return s.ResolveTypeAlias(underlying)
		}
		return t
	case typesystem.TApp:
		// Resolve constructor and args recursively
		resolvedCon := s.ResolveTypeAlias(ty.Constructor)
		resolvedArgs := make([]typesystem.Type, len(ty.Args))
		for i, arg := range ty.Args {
			resolvedArgs[i] = s.ResolveTypeAlias(arg)
		}
		return typesystem.TApp{Constructor: resolvedCon, Args: resolvedArgs}
	case typesystem.TFunc:
		// Resolve params and return type
		resolvedParams := make([]typesystem.Type, len(ty.Params))
		for i, p := range ty.Params {
			resolvedParams[i] = s.ResolveTypeAlias(p)
		}
		return typesystem.TFunc{
			Params:     resolvedParams,
			ReturnType: s.ResolveTypeAlias(ty.ReturnType),
			IsVariadic: ty.IsVariadic,
		}
	case typesystem.TTuple:
		resolvedElems := make([]typesystem.Type, len(ty.Elements))
		for i, e := range ty.Elements {
			resolvedElems[i] = s.ResolveTypeAlias(e)
		}
		return typesystem.TTuple{Elements: resolvedElems}
	case typesystem.TRecord:
		resolvedFields := make(map[string]typesystem.Type)
		for k, v := range ty.Fields {
			resolvedFields[k] = s.ResolveTypeAlias(v)
		}
		return typesystem.TRecord{Fields: resolvedFields}
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
	s.implementations[name] = []typesystem.Type{}
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
		// Found instance-specific signature, unify to get concrete return type
		funcType, ok := unwrapType.(typesystem.TFunc)
		if ok && len(funcType.Params) == 1 {
			// Rename type vars to avoid conflicts
			renamedParam := renameTypeVars(funcType.Params[0], "inst")
			renamedReturn := renameTypeVars(funcType.ReturnType, "inst")

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
		funcType, ok := genericUnwrap.(typesystem.TFunc)
		if ok && len(funcType.Params) == 1 {
			renamedParam := renameTypeVars(funcType.Params[0], "gen")
			renamedReturn := renameTypeVars(funcType.ReturnType, "gen")

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
func (s *SymbolTable) GetAllImplementations() map[string][]typesystem.Type {
	result := make(map[string][]typesystem.Type)

	// Get from outer first
	if s.outer != nil {
		for k, v := range s.outer.GetAllImplementations() {
			result[k] = append([]typesystem.Type(nil), v...) // copy slice
		}
	}

	// Merge current level
	for trait, impls := range s.implementations {
		result[trait] = append(result[trait], impls...)
	}

	return result
}

func (s *SymbolTable) RegisterImplementation(traitName string, t typesystem.Type) error {
	// Validate that trait exists
	if !s.TraitExists(traitName) {
		panic(fmt.Sprintf("RegisterImplementation: trait %q does not exist", traitName))
	}

	// Check for overlap
	if impls, ok := s.implementations[traitName]; ok {
		for _, existing := range impls {
			// Check if t overlaps with existing
			// We use UnifyAllowExtra or just Unify.
			// If Unify succeeds, it means there is a substitution that makes them equal.
			// Which implies overlap.
			// We need fresh copies? typesystem.Unify modifies substitutions if passed?
			// Unify returns substitution. It doesn't modify input types usually (unless Apply is called).
			// But it might rely on type variable names.
			// Instance types usually have different type variable names (or same names but different scopes).
			// To be safe, we should rename variables in one of them to be disjoint.
			// e.g. "List a" vs "List a".
			// renameVariables(t)

			tRenamed := renameTypeVars(t, "new")
			_, err := typesystem.Unify(existing, tRenamed)
			if err == nil {
				return fmt.Errorf("overlapping instances for trait %s: %s and %s", traitName, existing, t)
			}
		}
		s.implementations[traitName] = append(s.implementations[traitName], t)
		return nil
	} else if s.outer != nil {
		return s.outer.RegisterImplementation(traitName, t)
	}
	// Trait not found?
	// Should have been defined. But if we can't find it, we can't register.
	// Maybe Define it implicitly? No.
	return fmt.Errorf("trait %s not defined", traitName)
}

func (s *SymbolTable) IsImplementationExists(traitName string, t typesystem.Type) bool {
	// Collect types to check: original type + resolved alias if applicable
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

	if impls, ok := s.implementations[traitName]; ok {
		for _, impl := range impls {
			implRenamed := renameTypeVars(impl, "exist")

			// Try to match against all collected types
			for _, typeToCheck := range typesToCheck {
				_, err := typesystem.Unify(implRenamed, typeToCheck)
				if err == nil {
					return true
				}
			}

			// HKT: For type constructors like Result, check if t's constructor matches impl
			if implCon, ok := impl.(typesystem.TCon); ok {
				for _, typeToCheck := range typesToCheck {
					if tApp, ok := typeToCheck.(typesystem.TApp); ok {
						if tAppCon, ok := tApp.Constructor.(typesystem.TCon); ok {
							if implCon.Name == tAppCon.Name {
								return true
							}
						}
					}
				}
			}
		}
		return false
	}
	if s.outer != nil {
		return s.outer.IsImplementationExists(traitName, t)
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

// Helper to rename type variables to avoid collisions during Unify checks
func renameTypeVars(t typesystem.Type, suffix string) typesystem.Type {
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
//   unwrap: Result<T, E> -> T
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

func (s *SymbolTable) Find(name string) (Symbol, bool) {
	sym, ok := s.store[name]
	if !ok && s.outer != nil {
		return s.outer.Find(name)
	}
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
