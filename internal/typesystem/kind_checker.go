package typesystem

import "fmt"

// UnifyKinds attempts to unify two kinds.
// Currently performs strict equality check as we don't have Kind Variables yet.
// In the future, this handles KVar unification.
func UnifyKinds(k1, k2 Kind) error {
	if k1.Equal(k2) {
		return nil
	}
	return fmt.Errorf("kind mismatch: expected %s, got %s", k1, k2)
}

// KindCheck validates that a type is well-kinded and returns its kind.
// It also populates KindVal fields in TApp if they are nil.
func KindCheck(t Type) (Kind, error) {
	if t == nil {
		return nil, fmt.Errorf("cannot check kind of nil type")
	}

	switch typ := t.(type) {
	case TCon:
		return typ.Kind(), nil
	case TVar:
		return typ.Kind(), nil
	case *TApp:
		return checkTAppKind(*typ)
	case TApp:
		return checkTAppKind(typ)
	case TTuple:
		for _, elem := range typ.Elements {
			k, err := KindCheck(elem)
			if err != nil {
				return nil, err
			}
			if !k.Equal(Star) {
				return nil, fmt.Errorf("tuple element must be type (kind *), got kind %s", k)
			}
		}
		return Star, nil
	case TRecord:
		for _, field := range typ.Fields {
			k, err := KindCheck(field)
			if err != nil {
				return nil, err
			}
			if !k.Equal(Star) {
				return nil, fmt.Errorf("record field must be type (kind *), got kind %s", k)
			}
		}
		return Star, nil
	case TFunc:
		for _, p := range typ.Params {
			k, err := KindCheck(p)
			if err != nil {
				return nil, err
			}
			if !k.Equal(Star) {
				return nil, fmt.Errorf("function parameter must be type (kind *), got kind %s", k)
			}
		}
		k, err := KindCheck(typ.ReturnType)
		if err != nil {
			return nil, err
		}
		if !k.Equal(Star) {
			return nil, fmt.Errorf("function return type must be type (kind *), got kind %s", k)
		}
		return Star, nil
	case TForall:
		k, err := KindCheck(typ.Type)
		if err != nil {
			return nil, err
		}
		if !k.Equal(Star) {
			return nil, fmt.Errorf("polymorphic type must be type (kind *), got kind %s", k)
		}
		return Star, nil
	case TUnion:
		for _, t := range typ.Types {
			k, err := KindCheck(t)
			if err != nil {
				return nil, err
			}
			if !k.Equal(Star) {
				return nil, fmt.Errorf("union variant must be type (kind *), got kind %s", k)
			}
		}
		return Star, nil
	case TType:
		if _, err := KindCheck(typ.Type); err != nil {
			return nil, err
		}
		return Star, nil
	default:
		return Star, nil
	}
}

func checkTAppKind(t TApp) (Kind, error) {
	kCtor, err := KindCheck(t.Constructor)
	if err != nil {
		return nil, err
	}

	currKind := kCtor
	isTVar := false
	if _, ok := t.Constructor.(TVar); ok {
		isTVar = true
	}

	for _, arg := range t.Args {
		kArg, err := KindCheck(arg)
		if err != nil {
			return nil, err
		}

		if arrow, ok := currKind.(KArrow); ok {
			// Check argument kind matches expected
			if !arrow.Left.Equal(kArg) && !arrow.Left.Equal(AnyKind) {
				return nil, fmt.Errorf("kind mismatch in application: expected argument of kind %s, got %s", arrow.Left, kArg)
			}
			currKind = arrow.Right
		} else {
			// Relaxed check for TVars: if we are applying arguments to a Type Variable,
			// we assume it has the appropriate Higher Kind (inference fallback).
			if isTVar {
				currKind = Star
				continue
			}
			return nil, fmt.Errorf("cannot apply type argument to non-function kind %s", currKind)
		}
	}
	return currKind, nil
}
