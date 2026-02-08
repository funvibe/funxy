package evaluator

import (
	"fmt"
	"github.com/funvibe/funxy/internal/typesystem"
	"math"
	"math/big"
)

// Boolean
type Boolean struct {
	Value bool
}

func (b *Boolean) Type() ObjectType { return BOOLEAN_OBJ }
func (b *Boolean) Inspect() string  { return fmt.Sprintf("%t", b.Value) }
func (b *Boolean) RuntimeType() typesystem.Type {
	if b == nil {
		return typesystem.TCon{Name: "Bool"}
	}
	return typesystem.TCon{Name: "Bool"}
}
func (b *Boolean) Hash() uint32 {
	if b.Value {
		return 1
	}
	return 0
}

// Integer
type Integer struct {
	Value int64
}

func (i *Integer) Type() ObjectType { return INTEGER_OBJ }
func (i *Integer) Inspect() string  { return fmt.Sprintf("%d", i.Value) }
func (i *Integer) RuntimeType() typesystem.Type {
	if i == nil {
		return typesystem.TCon{Name: "Int"}
	}
	return typesystem.TCon{Name: "Int"}
}
func (i *Integer) Hash() uint32 {
	return uint32(i.Value ^ (i.Value >> 32))
}

// Float
type Float struct {
	Value float64
}

func (f *Float) Type() ObjectType { return FLOAT_OBJ }
func (f *Float) Inspect() string  { return fmt.Sprintf("%g", f.Value) }
func (f *Float) RuntimeType() typesystem.Type {
	if f == nil {
		return typesystem.TCon{Name: "Float"}
	}
	return typesystem.TCon{Name: "Float"}
}
func (f *Float) Hash() uint32 {
	bits := math.Float64bits(f.Value)
	return uint32(bits ^ (bits >> 32))
}

// BigInt
type BigInt struct {
	Value *big.Int
}

func (bi *BigInt) Type() ObjectType { return BIG_INT_OBJ }
func (bi *BigInt) Inspect() string  { return bi.Value.String() }
func (bi *BigInt) RuntimeType() typesystem.Type {
	if bi == nil {
		return typesystem.TCon{Name: "BigInt"}
	}
	return typesystem.TCon{Name: "BigInt"}
}
func (bi *BigInt) Hash() uint32 {
	return hashString(bi.Value.String())
}

// Rational
type Rational struct {
	Value *big.Rat
}

func (r *Rational) Type() ObjectType { return RATIONAL_OBJ }
func (r *Rational) Inspect() string  { return r.Value.FloatString(10) }
func (r *Rational) RuntimeType() typesystem.Type {
	if r == nil {
		return typesystem.TCon{Name: "Rational"}
	}
	return typesystem.TCon{Name: "Rational"}
}
func (r *Rational) Hash() uint32 {
	return hashString(r.Value.String())
}

// Nil (e.g. for statements that don't return a value, or print)
type Nil struct{}

func (n *Nil) Type() ObjectType { return NIL_OBJ }
func (n *Nil) Inspect() string  { return "Nil" }
func (n *Nil) RuntimeType() typesystem.Type {
	if n == nil {
		return typesystem.TCon{Name: "Nil"}
	}
	return typesystem.TCon{Name: "Nil"}
}
func (n *Nil) Hash() uint32 { return 0 }

// Char represents a character.
type Char struct {
	Value int64
}

func (c *Char) Type() ObjectType { return CHAR_OBJ }
func (c *Char) Inspect() string  { return fmt.Sprintf("'%c'", c.Value) }
func (c *Char) RuntimeType() typesystem.Type {
	if c == nil {
		return typesystem.TCon{Name: "Char"}
	}
	return typesystem.TCon{Name: "Char"}
}
func (c *Char) Hash() uint32 { return uint32(c.Value) }
