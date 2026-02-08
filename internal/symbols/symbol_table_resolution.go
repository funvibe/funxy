package symbols

import (
	"github.com/funvibe/funxy/internal/typesystem"
	"strings"
)

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
