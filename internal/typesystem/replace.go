package typesystem

// ReplaceTCon replaces all occurrences of TCon with the given name with the replacement type.
// This is useful for updating recursive type references when the TCon definition is finalized (e.g. kind inferred).
func ReplaceTCon(t Type, name string, replacement Type) Type {
	if t == nil {
		return nil
	}
	switch typ := t.(type) {
	case TCon:
		if typ.Name == name {
			return replacement
		}
		return typ
	case TApp:
		newCtor := ReplaceTCon(typ.Constructor, name, replacement)
		newArgs := make([]Type, len(typ.Args))
		for i, arg := range typ.Args {
			newArgs[i] = ReplaceTCon(arg, name, replacement)
		}
		return TApp{
			Constructor: newCtor,
			Args:        newArgs,
			KindVal:     typ.KindVal,
		}
	case TFunc:
		newParams := make([]Type, len(typ.Params))
		for i, p := range typ.Params {
			newParams[i] = ReplaceTCon(p, name, replacement)
		}
		newRet := ReplaceTCon(typ.ReturnType, name, replacement)
		return TFunc{
			Params:       newParams,
			ReturnType:   newRet,
			IsVariadic:   typ.IsVariadic,
			DefaultCount: typ.DefaultCount,
			Constraints:  typ.Constraints,
		}
	case TTuple:
		newElements := make([]Type, len(typ.Elements))
		for i, e := range typ.Elements {
			newElements[i] = ReplaceTCon(e, name, replacement)
		}
		return TTuple{Elements: newElements}
	case TRecord:
		newFields := make(map[string]Type)
		for k, v := range typ.Fields {
			newFields[k] = ReplaceTCon(v, name, replacement)
		}
		return TRecord{Fields: newFields}
	case TForall:
		newType := ReplaceTCon(typ.Type, name, replacement)
		return TForall{
			Vars:        typ.Vars,
			Constraints: typ.Constraints,
			Type:        newType,
		}
	default:
		return t
	}
}
