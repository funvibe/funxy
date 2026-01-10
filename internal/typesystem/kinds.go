package typesystem

import "fmt"

// Kind represents the "type of a type".
// * (Star) is the kind of proper types (Int, Bool, List Int).
// * -> * is the kind of type constructors (List, Option).
type Kind interface {
	String() string
	Equal(Kind) bool
}

// KStar represents the kind of a value type (*).
type KStar struct{}

func (k KStar) String() string { return "*" }
func (k KStar) Equal(other Kind) bool {
	if _, ok := other.(KWildcard); ok {
		return true
	}
	_, ok := other.(KStar)
	return ok
}

// KWildcard represents a kind that matches any other kind.
// Used for built-ins like typeOf that accept any Type<T> regardless of T's kind.
type KWildcard struct{}

func (k KWildcard) String() string        { return "?" }
func (k KWildcard) Equal(other Kind) bool { return true }

// KArrow represents a higher-kinded type (k1 -> k2).
type KArrow struct {
	Left  Kind
	Right Kind
}

func (k KArrow) String() string {
	return fmt.Sprintf("(%s -> %s)", k.Left.String(), k.Right.String())
}

func (k KArrow) Equal(other Kind) bool {
	if _, ok := other.(KWildcard); ok {
		return true
	}
	o, ok := other.(KArrow)
	if !ok {
		return false
	}
	return k.Left.Equal(o.Left) && k.Right.Equal(o.Right)
}

var Star Kind = KStar{}
var AnyKind Kind = KWildcard{}

// Helper to create N-ary arrows
// e.g. List a -> * -> *
func MakeArrow(args ...Kind) Kind {
	if len(args) == 0 {
		return Star
	}
	if len(args) == 1 {
		return args[0]
	}
	return KArrow{Left: args[0], Right: MakeArrow(args[1:]...)}
}
