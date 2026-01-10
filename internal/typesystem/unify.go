package typesystem

import (
	"fmt"
	"reflect"
)

// Resolver interface allows Unify to look up type definitions (e.g. from SymbolTable)
// to handle nominal types or aliases that are not locally resolved.
type Resolver interface {
	ResolveTypeAlias(Type) Type
	ResolveTCon(name string) (TCon, bool)
	IsStrictMode() bool
}

// Unify attempts to find a substitution that makes t1 and t2 equal.
// It enforces strict equality (invariant).
func Unify(t1, t2 Type) (Subst, error) {
	return unifyInternal(t1, t2, false, nil, nil)
}

// UnifyWithResolver attempts to find a substitution using a resolver for type aliases.
func UnifyWithResolver(t1, t2 Type, resolver Resolver) (Subst, error) {
	return unifyInternal(t1, t2, false, nil, resolver)
}

// UnifyAllowExtra attempts to unify t1 and t2, allowing t2 to have extra fields if t1 is a Record.
// This implements width subtyping (t2 is a subtype of t1).
// t1 is the Expected type (Supertype), t2 is the Actual type (Subtype).
func UnifyAllowExtra(t1, t2 Type) (Subst, error) {
	return unifyInternal(t1, t2, true, nil, nil)
}

// UnifyAllowExtraWithResolver allows extra fields and uses a resolver.
func UnifyAllowExtraWithResolver(t1, t2 Type, resolver Resolver) (Subst, error) {
	return unifyInternal(t1, t2, true, nil, resolver)
}

// typePair represents a pair of types being compared for co-induction
type typePair struct {
	t1 Type
	t2 Type
}

func unifyInternal(t1, t2 Type, allowExtra bool, visited []typePair, resolver Resolver) (Subst, error) {
	// Co-induction step: Check if we are already comparing these two types in the current stack
	for _, p := range visited {
		// Use reflect.DeepEqual for robust comparison including TCons
		if reflect.DeepEqual(p.t1, t1) && reflect.DeepEqual(p.t2, t2) {
			// Cycle detected, assume success (co-induction)
			return Subst{}, nil
		}
	}

	// Add current pair to visited
	visited = append(visited, typePair{t1: t1, t2: t2})

	// If types are strictly equal
	if reflect.DeepEqual(t1, t2) {
		return Subst{}, nil
	}

	// Unify directionality fix: If t2 is a TCon (alias) and t1 is a structural type (Record, Func, etc.),
	// we need to unwrap t2 to see if it matches t1.
	// We skip this if t1 is TCon (handled in switch) or TVar (handled in switch).
	_, t1IsTCon := t1.(TCon)
	_, t1IsTVar := t1.(TVar)
	if !t1IsTCon && !t1IsTVar {
		if t2Con, ok := t2.(TCon); ok {
			u2 := UnwrapUnderlying(t2Con)
			if t2Con.UnderlyingType != nil { // If unwrappable
				return unifyInternal(t1, u2, allowExtra, visited, resolver)
			}
			// Try resolver if UnwrapUnderlying failed
			if resolver != nil {
				r2 := resolver.ResolveTypeAlias(t2)
				// ResolveTypeAlias returns t2 if no change.
				// We need robust check if it actually resolved something relevant?
				// ResolveTypeAlias might return expanded type.
				if !reflect.DeepEqual(r2, t2) {
					return unifyInternal(t1, r2, allowExtra, visited, resolver)
				}

				// Extra check for qualified types:
				// If r2 is still TCon, but maybe it resolved to a different TCon that DOES have underlying?
				// ResolveTypeAlias might return TCon{Name: "DbConfig", Underlying: ...}
				if r2Con, ok := r2.(TCon); ok && r2Con.UnderlyingType != nil {
					return unifyInternal(t1, r2Con.UnderlyingType, allowExtra, visited, resolver)
				}
			}
		}
	}

	// Unify TApp aliases: If one is TApp (alias) and other is NOT TApp/TVar, try resolving.
	// This handles cases like Node<Int> (TApp) ~ { ... } (TRecord)
	if tApp, ok := t1.(TApp); ok {
		if _, isTVar := t2.(TVar); !isTVar {
			if _, isTApp := t2.(TApp); !isTApp {
				// Try manual expansion first (fast path avoiding heavy ResolveTypeAlias)
				var tCon TCon
				var isTCon bool
				if tCon, isTCon = tApp.Constructor.(TCon); isTCon {
					// If TCon is stale (nil Underlying), try to refresh from resolver
					if tCon.UnderlyingType == nil && resolver != nil {
						if updated, found := resolver.ResolveTCon(tCon.Name); found {
							tCon = updated
						}
					}
				}

				if isTCon && tCon.UnderlyingType != nil && tCon.TypeParams != nil && len(*tCon.TypeParams) == len(tApp.Args) {
					subst := make(Subst)
					for i, param := range *tCon.TypeParams {
						subst[param] = tApp.Args[i]
					}
					expanded := tCon.UnderlyingType.Apply(subst)
					return unifyInternal(expanded, t2, allowExtra, visited, resolver)
				}

				if resolver != nil {
					r1 := resolver.ResolveTypeAlias(t1)
					if !reflect.DeepEqual(r1, t1) {
						return unifyInternal(r1, t2, allowExtra, visited, resolver)
					}
				}
			}
		}
	}

	if tApp, ok := t2.(TApp); ok {
		if _, isTVar := t1.(TVar); !isTVar {
			if _, isTApp := t1.(TApp); !isTApp {
				// Try manual expansion first (fast path avoiding heavy ResolveTypeAlias)
				var tCon TCon
				var isTCon bool
				if tCon, isTCon = tApp.Constructor.(TCon); isTCon {
					// If TCon is stale (nil Underlying), try to refresh from resolver
					if tCon.UnderlyingType == nil && resolver != nil {
						if updated, found := resolver.ResolveTCon(tCon.Name); found {
							tCon = updated
						}
					}
				}

				if isTCon && tCon.UnderlyingType != nil && tCon.TypeParams != nil && len(*tCon.TypeParams) == len(tApp.Args) {
					subst := make(Subst)
					for i, param := range *tCon.TypeParams {
						subst[param] = tApp.Args[i]
					}
					expanded := tCon.UnderlyingType.Apply(subst)
					return unifyInternal(t1, expanded, allowExtra, visited, resolver)
				}

				if resolver != nil {
					r2 := resolver.ResolveTypeAlias(t2)
					if !reflect.DeepEqual(r2, t2) {
						return unifyInternal(t1, r2, allowExtra, visited, resolver)
					}
				}
			}
		}
	}

	// Special case: t2 is a union type, t1 is not
	// Check if t1 is a member of the union (subtyping: T <: T | U)
	if _, ok := t1.(TUnion); !ok {
		// Strict Mode Check: Disable unsafe Union -> Member implicit conversion
		isStrict := false
		if resolver != nil {
			isStrict = resolver.IsStrictMode()
		}

		if !isStrict {
			if union, ok := t2.(TUnion); ok {
				for _, member := range union.Types {
					if s, err := unifyInternal(t1, member, allowExtra, visited, resolver); err == nil {
						return s, nil
					}
				}
				return nil, errUnifyMsg(t1, t2, "type is not a member of union")
			}
		}
	}

	switch t1 := t1.(type) {
	case TVar:
		return Bind(t1, t2)
	case TApp:
		// Try to expand type aliases before unification
		// e.g., StringResult<Int> -> Result<Int, String>
		expanded1 := ExpandTypeAlias(t1)
		if expanded1 != nil && !reflect.DeepEqual(expanded1, t1) {
			return unifyInternal(expanded1, t2, allowExtra, visited, resolver)
		}

		switch t2 := t2.(type) {
		case TVar:
			return Bind(t2, t1)
		case TApp:
			// Try to expand t2 as well
			expanded2 := ExpandTypeAlias(t2)
			if expanded2 != nil && !reflect.DeepEqual(expanded2, t2) {
				return unifyInternal(t1, expanded2, allowExtra, visited, resolver)
			}

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
						s2, err := unifyInternal(arg1, arg2, false, visited, resolver)
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

					// Bind F to the partial type
					s1, err := Bind(t2Var, partialType)
					if err != nil {
						return nil, err
					}

					for i := 0; i < len(t2.Args); i++ {
						arg1 := t1.Args[numExtra+i].Apply(s1)
						arg2 := t2.Args[i].Apply(s1)
						s2, err := unifyInternal(arg1, arg2, false, visited, resolver)
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
			s1, err := unifyInternal(t1.Constructor, t2.Constructor, false, visited, resolver)
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
				s2, err := unifyInternal(arg1, arg2, false, visited, resolver)
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

			// Implicit Int -> Float conversion
			// If t1 is Float (Expected) and t2 is Int (Actual)
			if t1.Name == "Float" && t2.Name == "Int" {
				return Subst{}, nil
			}

			// Unwrap nested TCons and unify underlying types
			u1 := UnwrapUnderlying(t1)
			u2 := UnwrapUnderlying(t2)
			// If both unwrapped to non-TCon, unify them
			if t1.UnderlyingType != nil || t2.UnderlyingType != nil {
				return unifyInternal(u1, u2, allowExtra, visited, resolver)
			}
			// Use Resolver if available to expand types
			if resolver != nil {
				r1 := resolver.ResolveTypeAlias(t1)
				r2 := resolver.ResolveTypeAlias(t2)
				if !reflect.DeepEqual(r1, t1) || !reflect.DeepEqual(r2, t2) {
					return unifyInternal(r1, r2, allowExtra, visited, resolver)
				}
				// Check if resolved types have underlying types even if they are still TCons
				if r1Con, ok := r1.(TCon); ok && r1Con.UnderlyingType != nil {
					return unifyInternal(r1Con.UnderlyingType, r2, allowExtra, visited, resolver)
				}
				if r2Con, ok := r2.(TCon); ok && r2Con.UnderlyingType != nil {
					return unifyInternal(r1, r2Con.UnderlyingType, allowExtra, visited, resolver)
				}
			}
			return nil, errUnifyMsg(t1, t2, "type constant mismatch")
		default:
			// Unwrap and try to unify with underlying type
			u1 := UnwrapUnderlying(t1)
			if t1.UnderlyingType != nil {
				return unifyInternal(u1, t2, allowExtra, visited, resolver)
			}
			// Use Resolver
			if resolver != nil {
				r1 := resolver.ResolveTypeAlias(t1)
				if !reflect.DeepEqual(r1, t1) {
					return unifyInternal(r1, t2, allowExtra, visited, resolver)
				}
			}
			// If we are comparing two TCons and strict name match failed,
			// AND unwrapping failed (because nil underlying), maybe we can unwrap t2?
			if t2Con, ok := t2.(TCon); ok {
				u2 := UnwrapUnderlying(t2Con)
				if t2Con.UnderlyingType != nil {
					return unifyInternal(t1, u2, allowExtra, visited, resolver)
				}
				if resolver != nil {
					r2 := resolver.ResolveTypeAlias(t2)
					if !reflect.DeepEqual(r2, t2) {
						return unifyInternal(t1, r2, allowExtra, visited, resolver)
					}
				}
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
				s2, err := unifyInternal(arg1, arg2, allowExtra, visited, resolver)
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
			return unifyInternal(t1, UnwrapUnderlying(tCon), allowExtra, visited, resolver)
		}
		// Try resolver for t2
		if resolver != nil {
			if tCon, ok := t2.(TCon); ok {
				r2 := resolver.ResolveTypeAlias(tCon)
				if !reflect.DeepEqual(r2, t2) {
					return unifyInternal(t1, r2, allowExtra, visited, resolver)
				}
			}
		}

		switch t2 := t2.(type) {
		case TVar:
			return Bind(t2, t1)
		case TRecord:
			// Row Polymorphism Unification
			// 1. Unify common fields
			// 2. Identify mismatch fields
			// 3. Unify Row variables with mismatch fields

			s1 := Subst{}

			// 1. Common fields and missing from t2 (present in t1)
			for k, v1 := range t1.Fields {
				v1 = v1.Apply(s1)
				if v2, ok := t2.Fields[k]; ok {
					// Common field
					v2 = v2.Apply(s1)
					s2, err := unifyInternal(v1, v2, false, visited, resolver) // strict
					if err != nil {
						return nil, errUnifyContext(fmt.Sprintf("record field '%s'", k), err)
					}
					s1 = s1.Compose(s2)
				}
			}

			// 2. Collect extra fields
			extra1 := map[string]Type{} // Fields in t1 but not in t2
			for k, v := range t1.Fields {
				if _, ok := t2.Fields[k]; !ok {
					extra1[k] = v.Apply(s1)
				}
			}

			extra2 := map[string]Type{} // Fields in t2 but not in t1
			for k, v := range t2.Fields {
				if _, ok := t1.Fields[k]; !ok {
					extra2[k] = v.Apply(s1)
				}
			}

			// 3. Handle Row variables
			// If t1 has row variable r1, it must absorb extra2.
			// If t2 has row variable r2, it must absorb extra1.

			// Check t1 row
			if len(extra2) > 0 {
				if t1.Row != nil {
					// t1.Row ~ { extra2 | fresh? }
					// Since we don't have fresh vars, we only support unifying with CLOSED set of extras for now,
					// unless t2.Row is present.
					// Construct record type for extra2
					// If t2.Row is present, then t1.Row ~ { extra2 | t2.Row }

					var tail Type = nil
					if t2.Row != nil {
						tail = t2.Row.Apply(s1)
					}

					expectedTail := TRecord{Fields: extra2, Row: tail, IsOpen: tail != nil}

					// Apply current subst to t1.Row
					row1 := t1.Row.Apply(s1)

					// Unify row1 with expectedTail
					s2, err := unifyInternal(row1, expectedTail, allowExtra, visited, resolver)
					if err != nil {
						return nil, errUnifyContext("record row extension", err)
					}
					s1 = s1.Compose(s2)
				} else {
					// t1 is closed but t2 has extra fields.
					// If allowExtra (width subtyping), we ignore extra2.
					if !allowExtra && !t1.IsOpen {
						return nil, errMismatch(fmt.Sprintf("record has extra fields: %v", extra2))
					}
				}
			}

			// Check t2 row
			if len(extra1) > 0 {
				if t2.Row != nil {
					// t2.Row ~ { extra1 | t1.Row }

					var tail Type = nil
					if t1.Row != nil {
						tail = t1.Row.Apply(s1)
					}

					expectedTail := TRecord{Fields: extra1, Row: tail, IsOpen: tail != nil}

					row2 := t2.Row.Apply(s1)

					s2, err := unifyInternal(row2, expectedTail, allowExtra, visited, resolver)
					if err != nil {
						return nil, errUnifyContext("record row extension", err)
					}
					s1 = s1.Compose(s2)
				} else {
					// t2 is closed but t1 has extra fields.
					// t2 is Actual, t1 is Expected.
					// If t2 lacks fields required by t1, it's always an error.
					return nil, errMismatch(fmt.Sprintf("record missing fields: %v", extra1))
				}
			}

			// If both have rows and no extra fields, unify rows directly
			if len(extra1) == 0 && len(extra2) == 0 {
				if t1.Row != nil && t2.Row != nil {
					s2, err := unifyInternal(t1.Row.Apply(s1), t2.Row.Apply(s1), allowExtra, visited, resolver)
					if err != nil {
						return nil, err
					}
					s1 = s1.Compose(s2)
				}
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
				s2, err := unifyInternal(t1.Types[i].Apply(s), t2.Types[i].Apply(s), allowExtra, visited, resolver)
				if err != nil {
					return nil, errUnifyContext("union member", err)
				}
				s = s.Compose(s2)
			}
			return s, nil
		default:
			// Check if t2 is a member of the union t1 (subtyping: T <: T | U)
			for _, member := range t1.Types {
				if s, err := unifyInternal(member, t2, allowExtra, visited, resolver); err == nil {
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
				// Function params are Contravariant if subtyping is allowed.
				// If t2 <: t1 (t2 is subtype of t1), then t1 params must be subtype of t2 params.
				// i.e. p1 <: p2.
				// We check this by Unify(p2, p1) with allowExtra.
				// If strict equality, params are Invariant.
				var s2 Subst
				var err error
				if allowExtra {
					s2, err = unifyInternal(p2, p1, true, visited, resolver)
				} else {
					s2, err = unifyInternal(p1, p2, false, visited, resolver)
				}
				if err != nil {
					return nil, err
				}
				s1 = s1.Compose(s2)
			}

			ret1 := t1.ReturnType.Apply(s1)
			ret2 := t2.ReturnType.Apply(s1)
			// Return type is Covariant.
			s3, err := unifyInternal(ret1, ret2, allowExtra, visited, resolver)
			if err != nil {
				return nil, err
			}
			return s1.Compose(s3), nil
		default:
			return nil, errUnifyMsg(t1, t2, "cannot unify function type")
		}
	case TForall:
		switch t2 := t2.(type) {
		case TVar:
			return Bind(t2, t1)
		case TForall:
			// Unify two polytypes: forall a. T1 vs forall b. T2
			// Check alpha-equivalence by Skolemization
			if len(t1.Vars) != len(t2.Vars) {
				return nil, errMismatch("polytype variable count mismatch")
			}

			// Substitute variables with fresh Skolem constants (represented as rigid TCons)
			// to ensure body unification matches structure exactly without binding
			// the quantified variables to concrete types.
			subst := make(Subst)
			for i, v1 := range t1.Vars {
				// Use TCon with special name as Skolem
				skolemName := fmt.Sprintf("$skolem_%s", v1.Name)
				skolem := TCon{Name: skolemName, KindVal: v1.Kind()}
				subst[v1.Name] = skolem
				// Map corresponding var from t2 to SAME skolem
				subst[t2.Vars[i].Name] = skolem
			}

			t1Body := t1.Type.Apply(subst)
			t2Body := t2.Type.Apply(subst)

			// Unify bodies with rigid skolems
			return unifyInternal(t1Body, t2Body, allowExtra, visited, resolver)

		default:
			return nil, errUnifyMsg(t1, t2, "cannot unify polytype with monotype")
		}
	case TType:
		switch t2 := t2.(type) {
		case TVar:
			return Bind(t2, t1)
		case TType:
			// Types of Types should be strict?
			return unifyInternal(t1.Type, t2.Type, false, visited, resolver)
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

	// Kind Check: ensure tv and t have the same Kind
	// This is crucial for Higher-Order Unification to avoid binding * -> * variable to * type
	if !tv.Kind().Equal(t.Kind()) {
		return nil, errMismatch(fmt.Sprintf("kind mismatch: variable %s has kind %s, but type %s has kind %s",
			tv.Name, tv.Kind(), t, t.Kind()))
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

func errUnify(t1, t2 Type) error {
	return fmt.Errorf("cannot unify %s with %s", t1, t2)
}

func errUnifyMsg(t1, t2 Type, msg string) error {
	return fmt.Errorf("%s: %s vs %s", msg, t1, t2)
}

func errMismatch(msg string) error {
	return fmt.Errorf("type mismatch: %s", msg)
}

func errUnifyContext(ctx string, err error) error {
	return fmt.Errorf("in %s: %w", ctx, err)
}
