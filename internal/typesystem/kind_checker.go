package typesystem

import "fmt"

// KindSubst maps KVar names to Kinds
type KindSubst map[string]Kind

// ApplyKindSubst applies kind substitution to a kind
func ApplyKindSubst(s KindSubst, k Kind) Kind {
	if k == nil {
		return nil
	}
	switch k := k.(type) {
	case KVar:
		if replacement, ok := s[k.Name]; ok {
			return ApplyKindSubst(s, replacement)
		}
		return k
	case KArrow:
		return KArrow{
			Left:  ApplyKindSubst(s, k.Left),
			Right: ApplyKindSubst(s, k.Right),
		}
	default:
		return k
	}
}

// UnifyKinds attempts to unify two kinds and returns a substitution.
// It supports KVar unification.
func UnifyKinds(k1, k2 Kind) (KindSubst, error) {
	if k1 == nil || k2 == nil {
		return nil, fmt.Errorf("cannot unify nil kinds")
	}
	s := make(KindSubst)
	if err := unifyKinds(s, k1, k2); err != nil {
		return nil, err
	}
	return s, nil
}

func unifyKinds(s KindSubst, k1, k2 Kind) error {
	k1 = ApplyKindSubst(s, k1)
	k2 = ApplyKindSubst(s, k2)

	if k1 == nil || k2 == nil {
		return fmt.Errorf("cannot unify nil kinds: %v ~ %v", k1, k2)
	}

	if k1.Equal(k2) {
		return nil
	}

	if v, ok := k1.(KVar); ok {
		return bindKind(s, v.Name, k2)
	}
	if v, ok := k2.(KVar); ok {
		return bindKind(s, v.Name, k1)
	}

	if arrow1, ok1 := k1.(KArrow); ok1 {
		if arrow2, ok2 := k2.(KArrow); ok2 {
			if err := unifyKinds(s, arrow1.Left, arrow2.Left); err != nil {
				return err
			}
			return unifyKinds(s, arrow1.Right, arrow2.Right)
		}
	}

	return fmt.Errorf("kind mismatch: expected %s, got %s", k1, k2)
}

func bindKind(s KindSubst, name string, k Kind) error {
	if v, ok := k.(KVar); ok && v.Name == name {
		return nil
	}
	// Occurs check to prevent infinite recursion (e.g. k1 ~ k1 -> *)
	if kindOccurs(name, k) {
		return fmt.Errorf("recursive kind unification: %s occurs in %s", name, k)
	}
	s[name] = k
	return nil
}

// kindOccurs checks if a KVar with the given name appears in the kind k
func kindOccurs(name string, k Kind) bool {
	switch k := k.(type) {
	case KVar:
		return k.Name == name
	case KArrow:
		return kindOccurs(name, k.Left) || kindOccurs(name, k.Right)
	default:
		return false
	}
}

// KindContext holds the mapping from TVar names to their Kinds/KVars
type KindContext struct {
	KindVars map[string]Kind
	Counter  int
}

func NewKindContext() *KindContext {
	return &KindContext{
		KindVars: make(map[string]Kind),
	}
}

func (kc *KindContext) FreshKVar() KVar {
	kc.Counter++
	return KVar{Name: fmt.Sprintf("k%d", kc.Counter)}
}

// InferKind infers the kind of a type and returns it along with a substitution.
// It handles TVars by assigning/retrieving KVars and TApps by unifying kinds.
func InferKind(t Type, ctx *KindContext) (Kind, KindSubst, error) {
	if t == nil {
		return nil, nil, fmt.Errorf("cannot check kind of nil type")
	}

	subst := make(KindSubst)

	switch typ := t.(type) {
	case TCon:
		return typ.Kind(), subst, nil

	case TVar:
		if k, ok := ctx.KindVars[typ.Name]; ok {
			return k, subst, nil
		}
		// If TVar has explicit kind, trust it (for annotated types)
		if typ.KindVal != nil {
			ctx.KindVars[typ.Name] = typ.KindVal
			return typ.KindVal, subst, nil
		}
		// Otherwise create fresh KVar
		kv := ctx.FreshKVar()
		ctx.KindVars[typ.Name] = kv
		return kv, subst, nil

	case *TApp:
		return inferTAppKind(*typ, ctx)
	case TApp:
		return inferTAppKind(typ, ctx)

	case TTuple:
		for _, elem := range typ.Elements {
			k, s, err := InferKind(elem, ctx)
			if err != nil {
				return nil, nil, err
			}
			subst = mergeKindSubst(subst, s)
			if err := unifyKinds(subst, k, Star); err != nil {
				return nil, nil, fmt.Errorf("tuple element must be type (kind *), got kind %s", ApplyKindSubst(subst, k))
			}
		}
		return Star, subst, nil

	case TRecord:
		for _, field := range typ.Fields {
			k, s, err := InferKind(field, ctx)
			if err != nil {
				return nil, nil, err
			}
			subst = mergeKindSubst(subst, s)
			if err := unifyKinds(subst, k, Star); err != nil {
				return nil, nil, fmt.Errorf("record field must be type (kind *), got kind %s", ApplyKindSubst(subst, k))
			}
		}
		return Star, subst, nil

	case TFunc:
		for _, p := range typ.Params {
			k, s, err := InferKind(p, ctx)
			if err != nil {
				return nil, nil, err
			}
			subst = mergeKindSubst(subst, s)
			if err := unifyKinds(subst, k, Star); err != nil {
				return nil, nil, fmt.Errorf("function parameter must be type (kind *), got kind %s", ApplyKindSubst(subst, k))
			}
		}
		k, s, err := InferKind(typ.ReturnType, ctx)
		if err != nil {
			return nil, nil, err
		}
		subst = mergeKindSubst(subst, s)
		if err := unifyKinds(subst, k, Star); err != nil {
			return nil, nil, fmt.Errorf("function return type must be type (kind *), got kind %s", ApplyKindSubst(subst, k))
		}
		return Star, subst, nil

	case TForall:
		// TForall introduces new type variables which are quantified over the body.
		// We register them in the context to infer their kinds based on usage in the body.
		// e.g. forall f. f Int -> Int implies f: * -> *
		k, s, err := InferKind(typ.Type, ctx)
		if err != nil {
			return nil, nil, err
		}
		subst = mergeKindSubst(subst, s)
		if err := unifyKinds(subst, k, Star); err != nil {
			return nil, nil, fmt.Errorf("polymorphic type must be type (kind *), got kind %s", ApplyKindSubst(subst, k))
		}
		return Star, subst, nil

	case TUnion:
		for _, t := range typ.Types {
			k, s, err := InferKind(t, ctx)
			if err != nil {
				return nil, nil, err
			}
			subst = mergeKindSubst(subst, s)
			if err := unifyKinds(subst, k, Star); err != nil {
				return nil, nil, fmt.Errorf("union variant must be type (kind *), got kind %s", ApplyKindSubst(subst, k))
			}
		}
		return Star, subst, nil

	case TType:
		// TType wraps a type representation. Its kind is * (it's a value).
		// We verify the inner type is well-kinded.
		_, s, err := InferKind(typ.Type, ctx)
		if err != nil {
			return nil, nil, err
		}
		return Star, mergeKindSubst(subst, s), nil

	default:
		return Star, subst, nil
	}
}

func inferTAppKind(t TApp, ctx *KindContext) (Kind, KindSubst, error) {
	kCtor, subst, err := InferKind(t.Constructor, ctx)
	if err != nil {
		return nil, nil, err
	}

	for _, arg := range t.Args {
		kArg, sArg, err := InferKind(arg, ctx)
		if err != nil {
			return nil, nil, err
		}
		subst = mergeKindSubst(subst, sArg)

		// Create a fresh KVar for result kind
		kRet := ctx.FreshKVar()

		// Ctor kind must be kArg -> kRet
		expectedCtorKind := KArrow{Left: ApplyKindSubst(subst, kArg), Right: kRet}

		lhs := ApplyKindSubst(subst, kCtor)

		if err := unifyKinds(subst, lhs, expectedCtorKind); err != nil {
			return nil, nil, fmt.Errorf("kind mismatch in application: %v", err)
		}

		// Update kCtor for next iteration (it is now kRet)
		kCtor = kRet
	}

	return ApplyKindSubst(subst, kCtor), subst, nil
}

func mergeKindSubst(s1, s2 KindSubst) KindSubst {
	res := make(KindSubst)
	for k, v := range s1 {
		res[k] = v
	}
	for k, v := range s2 {
		res[k] = v
	}
	return res
}

// KindCheck validates that a type is well-kinded and returns its kind.
// It initializes a fresh KindContext and runs inference.
func KindCheck(t Type) (Kind, error) {
	ctx := NewKindContext()
	k, _, err := InferKind(t, ctx)
	return k, err
}
