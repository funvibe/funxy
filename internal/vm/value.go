package vm

import (
	"fmt"
	"math"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/typesystem"
)

// ValueType identifies the type of value stored in the Value struct
type ValueType uint8

const (
	ValNil ValueType = iota
	ValInt
	ValFloat
	ValBool
	ValObj // Complex object (String, List, Map, etc.)
)

// Value is a stack-allocated tagged union.
// It avoids heap allocation for small primitives (Int, Float, Bool, Nil).
// Size: ~24 bytes on 64-bit systems (1 byte type + 7 padding + 8 data + 8 pointer).
type Value struct {
	Type ValueType
	Data uint64           // Stores int64 bits, float64 bits, or bool (0/1)
	Obj  evaluator.Object // Holds heap objects (pointers) to keep them alive for GC
}

// Constructors

func NilVal() Value {
	return Value{Type: ValNil}
}

func IntVal(v int64) Value {
	return Value{Type: ValInt, Data: uint64(v)}
}

func FloatVal(v float64) Value {
	return Value{Type: ValFloat, Data: math.Float64bits(v)}
}

func BoolVal(v bool) Value {
	var data uint64
	if v {
		data = 1
	}
	return Value{Type: ValBool, Data: data}
}

func ObjVal(o evaluator.Object) Value {
	return Value{Type: ValObj, Obj: o}
}

// Accessors

func (v Value) AsInt() int64 {
	return int64(v.Data)
}

func (v Value) AsFloat() float64 {
	return math.Float64frombits(v.Data)
}

func (v Value) AsBool() bool {
	return v.Data == 1
}

// Conversion to evaluator.Object (boxing) - for compatibility/return
func (v Value) AsObject() evaluator.Object {
	switch v.Type {
	case ValInt:
		return &evaluator.Integer{Value: int64(v.Data)}
	case ValFloat:
		return &evaluator.Float{Value: math.Float64frombits(v.Data)}
	case ValBool:
		return &evaluator.Boolean{Value: v.Data == 1}
	case ValNil:
		return &evaluator.Nil{}
	case ValObj:
		return v.Obj
	default:
		return &evaluator.Nil{}
	}
}

// Helper to convert Object to Value (unboxing)
func ObjectToValue(obj evaluator.Object) Value {
	switch o := obj.(type) {
	case *evaluator.Integer:
		return IntVal(o.Value)
	case *evaluator.Float:
		return FloatVal(o.Value)
	case *evaluator.Boolean:
		return BoolVal(o.Value)
	case *evaluator.Nil:
		return NilVal()
	default:
		return ObjVal(obj)
	}
}

// Type checking helpers

func (v Value) IsInt() bool   { return v.Type == ValInt }
func (v Value) IsFloat() bool { return v.Type == ValFloat }
func (v Value) IsBool() bool  { return v.Type == ValBool }
func (v Value) IsNil() bool   { return v.Type == ValNil }
func (v Value) IsObj() bool   { return v.Type == ValObj }

// Equality check
func (v Value) Equals(other Value) bool {
	if v.Type != other.Type {
		// Implicit Int -> Float conversion
		if v.Type == ValInt && other.Type == ValFloat {
			return float64(v.AsInt()) == other.AsFloat()
		}
		if v.Type == ValFloat && other.Type == ValInt {
			return v.AsFloat() == float64(other.AsInt())
		}
		return false
	}
	switch v.Type {
	case ValInt, ValBool:
		return v.Data == other.Data
	case ValFloat:
		return v.Data == other.Data // Bitwise comparison for equality (careful with NaN)
	case ValNil:
		return true
	case ValObj:
		// Fallback to object equality logic
		return evaluator.ObjectsEqual(v.Obj, other.Obj)
	default:
		return false
	}
}

// Inspect returns string representation
func (v Value) Inspect() string {
	switch v.Type {
	case ValInt:
		return fmt.Sprintf("%d", int64(v.Data))
	case ValFloat:
		return fmt.Sprintf("%g", math.Float64frombits(v.Data))
	case ValBool:
		return fmt.Sprintf("%t", v.Data == 1)
	case ValNil:
		return "Nil"
	case ValObj:
		if v.Obj != nil {
			return v.Obj.Inspect()
		}
		return "<nil obj>"
	default:
		return "<?>"
	}
}

// RuntimeType returns the type info
func (v Value) RuntimeType() typesystem.Type {
	switch v.Type {
	case ValInt:
		return typesystem.TCon{Name: "Int"}
	case ValFloat:
		return typesystem.TCon{Name: "Float"}
	case ValBool:
		return typesystem.TCon{Name: "Bool"}
	case ValNil:
		return typesystem.TCon{Name: "Nil"}
	case ValObj:
		if v.Obj != nil {
			return v.Obj.RuntimeType()
		}
		return typesystem.TCon{Name: "Nil"}
	default:
		return typesystem.TCon{Name: "Unknown"}
	}
}

// Hash returns hash code
func (v Value) Hash() uint32 {
	switch v.Type {
	case ValInt:
		return uint32(v.Data ^ (v.Data >> 32))
	case ValFloat:
		return uint32(v.Data ^ (v.Data >> 32))
	case ValBool:
		if v.Data == 1 {
			return 1
		}
		return 0
	case ValNil:
		return 0
	case ValObj:
		if v.Obj != nil {
			return v.Obj.Hash()
		}
		return 0
	default:
		return 0
	}
}
