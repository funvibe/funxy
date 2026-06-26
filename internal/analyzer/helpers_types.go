package analyzer

import (
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

// unfoldAliasType resolves a bare TCon type alias to its underlying structural
// type (e.g. `Scores` -> Map<String, Int>, `Entry` -> (String, Int), `Ints` ->
// List<Int>). Structural dispatch on collections/tuples needs the underlying form;
// without this, an alias TCon would not match the Map/Tuple/List branches.
// Non-alias TCons and already-structural types are returned unchanged.
func unfoldAliasType(t typesystem.Type, table *symbols.SymbolTable) typesystem.Type {
	tCon, ok := t.(typesystem.TCon)
	if !ok {
		return t
	}
	if tCon.UnderlyingType != nil {
		return typesystem.UnwrapUnderlying(tCon)
	}
	if table != nil {
		return table.ResolveTypeAlias(tCon)
	}
	return t
}

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
