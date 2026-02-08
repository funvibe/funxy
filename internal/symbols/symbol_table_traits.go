package symbols

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/typesystem"
)

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
	// Mark as trait method
	if sym, ok := s.store[methodName]; ok {
		sym.IsTraitMethod = true
		s.store[methodName] = sym
	}
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

func (s *SymbolTable) RegisterTraitFunctionalDependencies(traitName string, deps []ast.FunctionalDependency) {
	s.traitFunctionalDependencies[traitName] = deps
}

func (s *SymbolTable) GetTraitFunctionalDependencies(traitName string) ([]ast.FunctionalDependency, bool) {
	deps, ok := s.traitFunctionalDependencies[traitName]
	if !ok && s.outer != nil {
		return s.outer.GetTraitFunctionalDependencies(traitName)
	}
	return deps, ok
}
