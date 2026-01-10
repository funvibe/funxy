package modules

import (
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

// VirtualPackage represents a built-in package with pre-defined symbols
type VirtualPackage struct {
	Name    string
	Symbols map[string]typesystem.Type

	// Types exported by this package (registered on import)
	Types map[string]typesystem.Type

	// ADT constructors exported by this package
	Constructors map[string]typesystem.Type

	// ADT variants for pattern matching
	Variants map[string][]string // TypeName -> [VariantName, ...]

	// Extended support for full packages (traits, instances, etc.)
	SymbolTable *symbols.SymbolTable // Full symbol table (optional, for complex packages)

	// Traits defined in this package: TraitName -> {TypeParams, SuperTraits, Methods}
	Traits map[string]*VirtualTrait

	// Operator -> Trait mappings for this package
	OperatorTraits map[string]string
}

// VirtualTrait represents a trait definition in a virtual package
type VirtualTrait struct {
	TypeParams  []string                   // e.g., ["F"] for Functor<F>
	SuperTraits []string                   // e.g., ["Functor"] for Applicative
	Methods     map[string]typesystem.Type // method name -> type signature
	Kind        typesystem.Kind            // e.g., * -> * for Functor
}

// virtualPackages maps package paths to their definitions
var virtualPackages = map[string]*VirtualPackage{}

// RegisterVirtualPackage registers a virtual package
func RegisterVirtualPackage(path string, pkg *VirtualPackage) {
	virtualPackages[path] = pkg
}

// GetVirtualPackage returns a virtual package by path, or nil if not found
func GetVirtualPackage(path string) *VirtualPackage {
	return virtualPackages[path]
}

// IsVirtualPackage checks if a path is a virtual package
func IsVirtualPackage(path string) bool {
	_, ok := virtualPackages[path]
	return ok
}

// CreateVirtualModule creates a Module from a VirtualPackage
func (vp *VirtualPackage) CreateVirtualModule() *Module {
	mod := &Module{
		Name:        vp.Name,
		Dir:         "virtual:" + vp.Name,
		Exports:     make(map[string]bool),
		SymbolTable: symbols.NewSymbolTable(),
		IsVirtual:   true,
	}

	// If package has its own SymbolTable, use it as base
	if vp.SymbolTable != nil {
		mod.SymbolTable = vp.SymbolTable
	}

	origin := vp.Name // Origin module for all symbols in this package

	// Register types exported by this package
	for name, typ := range vp.Types {
		mod.Exports[name] = true
		mod.SymbolTable.DefineType(name, typ, origin)
	}

	// Register constructors exported by this package
	for name, typ := range vp.Constructors {
		mod.Exports[name] = true
		mod.SymbolTable.DefineConstructor(name, typ, origin)
	}

	// Register variants for ADTs
	for typeName, variants := range vp.Variants {
		for _, variant := range variants {
			mod.SymbolTable.RegisterVariant(typeName, variant)
		}
	}

	// Register all simple symbols as exported
	for name, typ := range vp.Symbols {
		mod.Exports[name] = true
		mod.SymbolTable.Define(name, typ, origin)
	}

	// Register traits
	for traitName, trait := range vp.Traits {
		mod.Exports[traitName] = true
		mod.SymbolTable.DefineTrait(traitName, trait.TypeParams, trait.SuperTraits, origin)

		// Register kind if specified
		if trait.Kind != nil {
			mod.SymbolTable.RegisterKind(traitName, trait.Kind)
		}

		// Register trait methods
		for methodName, methodType := range trait.Methods {
			mod.SymbolTable.RegisterTraitMethod(methodName, traitName, methodType, origin)
			mod.SymbolTable.RegisterTraitMethod2(traitName, methodName)
			mod.Exports[methodName] = true
		}
	}

	// Register operator -> trait mappings
	for op, trait := range vp.OperatorTraits {
		mod.SymbolTable.RegisterOperatorTrait(op, trait)
	}

	return mod
}
