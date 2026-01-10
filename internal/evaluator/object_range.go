package evaluator

import (
	"fmt"
	"github.com/funvibe/funxy/internal/typesystem"
)

// Range represents a range of values [Start, End] or [Start, Next, End]
type Range struct {
	Start Object
	Next  Object // Nil if not present
	End   Object
}

func (r *Range) Type() ObjectType { return RANGE_OBJ }
func (r *Range) Inspect() string {
	if _, ok := r.Next.(*Nil); !ok {
		return fmt.Sprintf("%s, %s..%s", r.Start.Inspect(), r.Next.Inspect(), r.End.Inspect())
	}
	return fmt.Sprintf("%s..%s", r.Start.Inspect(), r.End.Inspect())
}
func (r *Range) RuntimeType() typesystem.Type {
	return typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Range"},
		Args:        []typesystem.Type{r.Start.RuntimeType()},
	}
}
func (r *Range) Hash() uint32 {
	h := r.Start.Hash()
	if _, ok := r.Next.(*Nil); !ok {
		h ^= r.Next.Hash()
	}
	h ^= r.End.Hash()
	return h
}
