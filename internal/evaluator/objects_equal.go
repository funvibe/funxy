package evaluator

import (
	"bytes"
)

// ObjectsEqual performs a deep equality check between two Funxy objects.
func ObjectsEqual(a, b Object) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Use RuntimeType for comparison, but optimize for common types
	if a.Type() != b.Type() {
		return false
	}

	switch aVal := a.(type) {
	case *Integer:
		if bVal, ok := b.(*Integer); ok {
			return aVal.Value == bVal.Value
		}
	case *Float:
		if bVal, ok := b.(*Float); ok {
			return aVal.Value == bVal.Value
		}
	case *Boolean:
		if bVal, ok := b.(*Boolean); ok {
			return aVal.Value == bVal.Value
		}
	case *Char:
		if bVal, ok := b.(*Char); ok {
			return aVal.Value == bVal.Value
		}
	case *Nil:
		_, ok := b.(*Nil)
		return ok
	case *List:
		if bVal, ok := b.(*List); ok {
			if aVal.Len() != bVal.Len() {
				return false
			}
			aSlice := aVal.ToSlice()
			bSlice := bVal.ToSlice()
			for i := range aSlice {
				if !ObjectsEqual(aSlice[i], bSlice[i]) {
					return false
				}
			}
			return true
		}
	case *Tuple:
		if bVal, ok := b.(*Tuple); ok {
			if len(aVal.Elements) != len(bVal.Elements) {
				return false
			}
			for i := range aVal.Elements {
				if !ObjectsEqual(aVal.Elements[i], bVal.Elements[i]) {
					return false
				}
			}
			return true
		}
	case *RecordInstance:
		if bVal, ok := b.(*RecordInstance); ok {
			if len(aVal.Fields) != len(bVal.Fields) {
				return false
			}
			// Records are sorted by key, so we can iterate in lockstep
			for i := range aVal.Fields {
				if aVal.Fields[i].Key != bVal.Fields[i].Key {
					return false
				}
				if !ObjectsEqual(aVal.Fields[i].Value, bVal.Fields[i].Value) {
					return false
				}
			}
			return true
		}
	case *Map:
		if bVal, ok := b.(*Map); ok {
			if aVal.Len() != bVal.Len() {
				return false
			}
			// Map equality requires iterating items
			for _, item := range aVal.hamt.Items() {
				v2 := bVal.hamt.Get(item.Key)
				if v2 == nil || !ObjectsEqual(item.Value, v2) {
					return false
				}
			}
			return true
		}
	case *DataInstance:
		if bVal, ok := b.(*DataInstance); ok {
			if aVal.Name != bVal.Name || len(aVal.Fields) != len(bVal.Fields) {
				return false
			}
			for i := range aVal.Fields {
				if !ObjectsEqual(aVal.Fields[i], bVal.Fields[i]) {
					return false
				}
			}
			return true
		}
	case *Bytes:
		if bVal, ok := b.(*Bytes); ok {
			return bytes.Equal(aVal.ToSlice(), bVal.ToSlice())
		}
	case *Bits:
		if bVal, ok := b.(*Bits); ok {
			return aVal.length == bVal.length && bytes.Equal(aVal.data, bVal.data)
		}
	case *BigInt:
		if bVal, ok := b.(*BigInt); ok {
			return aVal.Value.Cmp(bVal.Value) == 0
		}
	case *Rational:
		if bVal, ok := b.(*Rational); ok {
			return aVal.Value.Cmp(bVal.Value) == 0
		}
	case *Uuid:
		if bVal, ok := b.(*Uuid); ok {
			return aVal.Value == bVal.Value
		}
	case *TypeObject:
		if bVal, ok := b.(*TypeObject); ok {
			return aVal.TypeVal.String() == bVal.TypeVal.String()
		}
	}

	return false
}
