package typesystem

import (
	"fmt"
	"reflect"
	"sort"
)

// Unify attempts to find a substitution that makes t1 and t2 equal.
// It enforces strict equality (invariant).
func Unify(t1, t2 Type) (Subst, error) {
	return unifyInternal(t1, t2, false)
}

// UnifyAllowExtra attempts to unify t1 and t2, allowing t2 to have extra fields if t1 is a Record.
// This implements width subtyping (t2 is a subtype of t1).
// t1 is the Expected type (Supertype), t2 is the Actual type (Subtype).
func UnifyAllowExtra(t1, t2 Type) (Subst, error) {
	return unifyInternal(t1, t2, true)
}

func unifyInternal(t1, t2 Type, allowExtra bool) (Subst, error) {
	// If types are strictly equal
	if reflect.DeepEqual(t1, t2) {
		return Subst{}, nil
	}

	// Special case: t2 is a union type, t1 is not
	// Check if t1 is a member of the union (subtyping: T <: T | U)
	if _, ok := t1.(TUnion); !ok {
		if union, ok := t2.(TUnion); ok {
			for _, member := range union.Types {
				if s, err := unifyInternal(t1, member, allowExtra); err == nil {
					return s, nil
				}
			}
			return nil, errUnifyMsg(t1, t2, "type is not a member of union")
		}
	}

	switch t1 := t1.(type) {
	case TVar:
		return Bind(t1, t2)
	case TApp:
		switch t2 := t2.(type) {
		case TVar:
			return Bind(t2, t1)
		case TApp:
			// HKT: Handle higher-kinded type unification
			// Case 1: F<A> (TVar constructor) unified with Result<String, E> (concrete)
			// We need to bind F to a partially applied type
			if t1Var, ok := t1.Constructor.(TVar); ok {
				// t1 = F<A1, A2, ...Am>  (m args)
				// t2 = C<B1, B2, ...Bn>  (n args)
				// If m <= n, we can unify by:
				//   F = C<B1, ..., B(n-m)>  (partial application)
				//   A1 = B(n-m+1), ..., Am = Bn
				if len(t1.Args) <= len(t2.Args) {
					numExtra := len(t2.Args) - len(t1.Args)

					// Build the partial type for F
					var partialType Type
					if numExtra == 0 {
						// Same arity - F binds directly to constructor
						partialType = t2.Constructor
					} else {
						// F binds to partially applied type: C<B1, ..., B(n-m)>
						partialType = TApp{
							Constructor: t2.Constructor,
							Args:        t2.Args[:numExtra],
						}
					}

					// Bind F to the partial type
					s1, err := Bind(t1Var, partialType)
					if err != nil {
						return nil, err
					}

					// Unify remaining arguments: A1..Am with B(n-m+1)..Bn
					for i := 0; i < len(t1.Args); i++ {
						arg1 := t1.Args[i].Apply(s1)
						arg2 := t2.Args[numExtra+i].Apply(s1)
						s2, err := unifyInternal(arg1, arg2, false)
						if err != nil {
							return nil, err
						}
						s1 = s1.Compose(s2)
					}
					return s1, nil
				}
			}

			// Case 2: Concrete<A> unified with F<B> (TVar constructor in t2)
			if t2Var, ok := t2.Constructor.(TVar); ok {
				if len(t2.Args) <= len(t1.Args) {
					numExtra := len(t1.Args) - len(t2.Args)

					var partialType Type
					if numExtra == 0 {
						partialType = t1.Constructor
					} else {
						partialType = TApp{
							Constructor: t1.Constructor,
							Args:        t1.Args[:numExtra],
						}
					}

					s1, err := Bind(t2Var, partialType)
					if err != nil {
						return nil, err
					}

					for i := 0; i < len(t2.Args); i++ {
						arg1 := t1.Args[numExtra+i].Apply(s1)
						arg2 := t2.Args[i].Apply(s1)
						s2, err := unifyInternal(arg1, arg2, false)
						if err != nil {
							return nil, err
						}
						s1 = s1.Compose(s2)
					}
					return s1, nil
				}
			}

			// Case 3: Standard unification (same constructor, same arity)
			// Unify constructors
			s1, err := unifyInternal(t1.Constructor, t2.Constructor, false)
			if err != nil {
				return nil, err
			}

			// Unify arguments length
			if len(t1.Args) != len(t2.Args) {
				return nil, errMismatch(fmt.Sprintf("type arguments length mismatch: %d vs %d", len(t1.Args), len(t2.Args)))
			}

			// Unify arguments
			for i := 0; i < len(t1.Args); i++ {
				arg1 := t1.Args[i].Apply(s1)
				arg2 := t2.Args[i].Apply(s1)
				s2, err := unifyInternal(arg1, arg2, false)
				if err != nil {
					return nil, err
				}
				s1 = s1.Compose(s2)
			}
			return s1, nil
		default:
			return nil, errUnify(t1, t2)
		}
	case TCon:
		switch t2 := t2.(type) {
		case TVar:
			return Bind(t2, t1)
		case TCon:
			// If both have same name (ignoring module), they're the same type
			if t1.Name == t2.Name {
				return Subst{}, nil
			}
			// Unwrap nested TCons and unify underlying types
			u1 := UnwrapUnderlying(t1)
			u2 := UnwrapUnderlying(t2)
			// If both unwrapped to non-TCon, unify them
			if u1 != t1 || u2 != t2 {
				return unifyInternal(u1, u2, allowExtra)
			}
			return nil, errUnifyMsg(t1, t2, "type constant mismatch")
		default:
			// Unwrap and try to unify with underlying type
			u1 := UnwrapUnderlying(t1)
			if u1 != t1 {
				return unifyInternal(u1, t2, allowExtra)
			}
			return nil, errUnify(t1, t2)
		}
	case TTuple:
		switch t2 := t2.(type) {
		case TVar:
			return Bind(t2, t1)
		case TTuple:
			if len(t1.Elements) != len(t2.Elements) {
				return nil, errMismatch(fmt.Sprintf("tuple length mismatch: %d vs %d", len(t1.Elements), len(t2.Elements)))
			}
			s1 := Subst{}
			for i := 0; i < len(t1.Elements); i++ {
				arg1 := t1.Elements[i].Apply(s1)
				arg2 := t2.Elements[i].Apply(s1)
				// Tuple elements use same strictness as parent?
				// Tuples are immutable structural types, so they can be covariant?
				// If (Int, {x}) vs (Int, {x,y}).
				// If we read tuple, it's safe.
				// So we pass allowExtra.
				s2, err := unifyInternal(arg1, arg2, allowExtra)
				if err != nil {
					return nil, err
				}
				s1 = s1.Compose(s2)
			}
			return s1, nil
		default:
			return nil, errUnifyMsg(t1, t2, "cannot unify tuple")
		}
	case TRecord:
		// If t2 is TCon with underlying type, unwrap it first
		if tCon, ok := t2.(TCon); ok && tCon.UnderlyingType != nil {
			return unifyInternal(t1, UnwrapUnderlying(tCon), allowExtra)
		}
		switch t2 := t2.(type) {
		case TVar:
			return Bind(t2, t1)
		case TRecord:
			if allowExtra || t1.IsOpen { // Allow extra if requested OR if t1 is Open
				// Width Subtyping: t1 fields must be subset of t2 fields
				if len(t1.Fields) > len(t2.Fields) {
					return nil, errMismatch(fmt.Sprintf("record fields mismatch: expected at most %d fields, got %d", len(t1.Fields), len(t2.Fields)))
				}
			} else {
				// Strict Equality
				if len(t1.Fields) != len(t2.Fields) {
					return nil, errMismatch(fmt.Sprintf("record fields count mismatch: %d vs %d", len(t1.Fields), len(t2.Fields)))
				}
			}

			s1 := Subst{}
			// Sort keys for deterministic unification order
			keys := make([]string, 0, len(t1.Fields))
			for k := range t1.Fields {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, k := range keys {
				v1 := t1.Fields[k]
				v2, ok := t2.Fields[k]
				if !ok {
					return nil, errMismatch(fmt.Sprintf("record missing field: %s", k))
				}

				// Apply current substitution
				v1Applied := v1.Apply(s1)
				v2Applied := v2.Apply(s1)

				// Recursively unify fields
				// If we allow extra at top, do we allow extra deep?
				// NO. Records are mutable. Depth subtyping is unsafe for mutable fields.
				// Example: { a: {x, y} } passed to { a: {x} }.
				// If function writes {x} to a, the original record loses y.
				// So fields must be Invariant (Strict equality).
				s2, err := unifyInternal(v1Applied, v2Applied, false) // false = Strict
				if err != nil {
					return nil, errUnifyContext(fmt.Sprintf("record field '%s'", k), err)
				}
				s1 = s1.Compose(s2)
			}
			return s1, nil
		default:
			return nil, errUnifyMsg(t1, t2, "cannot unify record")
		}
	case TUnion:
		switch t2 := t2.(type) {
		case TVar:
			return Bind(t2, t1)
		case TUnion:
			// Union types must have the same members (after normalization)
			if len(t1.Types) != len(t2.Types) {
				return nil, errMismatch(fmt.Sprintf("union type mismatch: %d vs %d members", len(t1.Types), len(t2.Types)))
			}
			// Since types are normalized (sorted), we can compare pairwise
			s := Subst{}
			for i := range t1.Types {
				s2, err := unifyInternal(t1.Types[i].Apply(s), t2.Types[i].Apply(s), allowExtra)
				if err != nil {
					return nil, errUnifyContext("union member", err)
				}
				s = s.Compose(s2)
			}
			return s, nil
		default:
			// Check if t2 is a member of the union t1 (subtyping: T <: T | U)
			for _, member := range t1.Types {
				if s, err := unifyInternal(member, t2, allowExtra); err == nil {
					return s, nil
				}
			}
			return nil, errUnifyMsg(t1, t2, "cannot unify union")
		}
	case TFunc:
		switch t2 := t2.(type) {
		case TVar:
			return Bind(t2, t1)
		case TFunc:
			if t1.IsVariadic != t2.IsVariadic {
				return nil, errMismatch("cannot unify variadic function with non-variadic")
			}
			if len(t1.Params) != len(t2.Params) {
				return nil, errMismatch(fmt.Sprintf("function parameter count mismatch: %d vs %d", len(t1.Params), len(t2.Params)))
			}
			s1 := Subst{}
			for i := 0; i < len(t1.Params); i++ {
				p1 := t1.Params[i].Apply(s1)
				p2 := t2.Params[i].Apply(s1)
				// Function params are invariant for now (simplification)
				// Or Contravariant?
				// Using strict equality for simplicity.
				s2, err := unifyInternal(p1, p2, false)
				if err != nil {
					return nil, err
				}
				s1 = s1.Compose(s2)
			}

			ret1 := t1.ReturnType.Apply(s1)
			ret2 := t2.ReturnType.Apply(s1)
			// Return type is Covariant.
			s3, err := unifyInternal(ret1, ret2, allowExtra)
			if err != nil {
				return nil, err
			}
			return s1.Compose(s3), nil
		default:
			return nil, errUnifyMsg(t1, t2, "cannot unify function type")
		}
	case TType:
		switch t2 := t2.(type) {
		case TVar:
			return Bind(t2, t1)
		case TType:
			// Types of Types should be strict?
			return unifyInternal(t1.Type, t2.Type, false)
		default:
			return nil, errUnifyMsg(t1, t2, "cannot unify TType")
		}
	default:
		return nil, errMismatch(fmt.Sprintf("unknown type kind: %T", t1))
	}
}

// Bind binds a type variable to a type, performing the occurs check.
func Bind(tv TVar, t Type) (Subst, error) {
	// If t is the same variable, return empty substitution
	if tVal, ok := t.(TVar); ok && tVal.Name == tv.Name {
		return Subst{}, nil
	}

	// Occurs check: ensure tv does not appear in t (to avoid infinite types like a = List a)
	if OccursCheck(tv, t) {
		return nil, errMismatch(fmt.Sprintf("infinite type detected: %s in %s", tv, t))
	}

	return Subst{tv.Name: t}, nil
}

// OccursCheck returns true if tv appears free in t.
func OccursCheck(tv TVar, t Type) bool {
	for _, v := range t.FreeTypeVariables() {
		if v.Name == tv.Name {
			return true
		}
	}
	return false
}
