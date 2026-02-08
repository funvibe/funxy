package symbols

import "github.com/funvibe/funxy/internal/typesystem"

func (s *SymbolTable) RegisterTraitMethodDispatch(traitName, methodName string, sources []typesystem.DispatchSource) {
	if s.TraitMethodDispatch[traitName] == nil {
		s.TraitMethodDispatch[traitName] = make(map[string][]typesystem.DispatchSource)
	}
	s.TraitMethodDispatch[traitName][methodName] = sources
}

func (s *SymbolTable) GetTraitMethodDispatch(traitName, methodName string) ([]typesystem.DispatchSource, bool) {
	if methods, ok := s.TraitMethodDispatch[traitName]; ok {
		if sources, ok := methods[methodName]; ok {
			return sources, true
		}
	}
	if s.outer != nil {
		return s.outer.GetTraitMethodDispatch(traitName, methodName)
	}
	return nil, false
}
