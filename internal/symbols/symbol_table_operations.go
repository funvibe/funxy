package symbols

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/typesystem"
)

func NewEmptySymbolTable() *SymbolTable {
	return &SymbolTable{
		store:                       make(map[string]Symbol),
		types:                       make(map[string]typesystem.Type),
		scopeType:                   ScopeGlobal, // Default to global
		traitMethods:                make(map[string]string),
		traitTypeParams:             make(map[string][]string),
		traitTypeParamKinds:         make(map[string]map[string]typesystem.Kind),
		traitSuperTraits:            make(map[string][]string),
		traitFunctionalDependencies: make(map[string][]ast.FunctionalDependency),
		traitDefaultMethods:         make(map[string]map[string]bool),
		traitAllMethods:             make(map[string][]string),
		operatorTraits:              make(map[string]string),
		implementations:             make(map[string][]InstanceDef),
		instanceMethods:             make(map[string]map[string]map[string]typesystem.Type),
		extensionMethods:            make(map[string]map[string]typesystem.Type),
		genericTypeParams:           make(map[string][]string),
		funcConstraints:             make(map[string][]Constraint),
		variants:                    make(map[string][]string),
		kinds:                       make(map[string]typesystem.Kind),
		moduleAliases:               make(map[string]string),
		typeAliases:                 make(map[string]typesystem.Type),
		TraitMethodIndices:          make(map[string]map[string]int),
		TraitMethodDispatch:         make(map[string]map[string][]typesystem.DispatchSource),
		EvidenceTable:               make(map[string]string),
	}
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

func (s *SymbolTable) DefinePendingTrait(name string, origin string) {
	s.store[name] = Symbol{Name: name, Type: nil, Kind: TraitSymbol, IsPending: true, OriginModule: origin}
	s.implementations[name] = []InstanceDef{}
}

func (s *SymbolTable) DefineTypePending(name string, t typesystem.Type, origin string) {
	s.types[name] = t
	s.store[name] = Symbol{Name: name, Type: t, Kind: TypeSymbol, IsPending: true, OriginModule: origin}
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

// SetDefinitionFile updates the DefinitionFile for an existing symbol in the current scope
func (s *SymbolTable) SetDefinitionFile(name string, file string) {
	if sym, ok := s.store[name]; ok {
		sym.DefinitionFile = file
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

func (s *SymbolTable) DefineConstructor(name string, t typesystem.Type, origin string) {
	s.store[name] = Symbol{Name: name, Type: t, Kind: ConstructorSymbol, OriginModule: origin}
}

// DefineWithNode defines a symbol and stores the AST node where it was defined.
func (s *SymbolTable) DefineWithNode(name string, t typesystem.Type, origin string, node ast.Node) {
	s.store[name] = Symbol{Name: name, Type: t, Kind: VariableSymbol, OriginModule: origin, DefinitionNode: node}
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
