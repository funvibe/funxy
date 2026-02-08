package analyzer

import "github.com/funvibe/funxy/internal/typesystem"

// containsTypeVar checks if a type variable appears in a type
func containsTypeVar(t typesystem.Type, varName string) bool {
	switch typ := t.(type) {
	case typesystem.TVar:
		return typ.Name == varName
	case typesystem.TApp:
		if containsTypeVar(typ.Constructor, varName) {
			return true
		}
		for _, arg := range typ.Args {
			if containsTypeVar(arg, varName) {
				return true
			}
		}
		return false
	case typesystem.TFunc:
		for _, p := range typ.Params {
			if containsTypeVar(p, varName) {
				return true
			}
		}
		return containsTypeVar(typ.ReturnType, varName)
	case typesystem.TTuple:
		for _, el := range typ.Elements {
			if containsTypeVar(el, varName) {
				return true
			}
		}
		return false
	case typesystem.TRecord:
		for _, field := range typ.Fields {
			if containsTypeVar(field, varName) {
				return true
			}
		}
		return false
	case typesystem.TUnion:
		for _, el := range typ.Types {
			if containsTypeVar(el, varName) {
				return true
			}
		}
		return false
	}
	return false
}
