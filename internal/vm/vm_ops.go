package vm

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/evaluator"
)

// binaryOp performs binary arithmetic operations
func (vm *VM) binaryOp(op Opcode) error {
	b := vm.pop()
	a := vm.pop()

	// Fast path for integers
	if a.IsInt() && b.IsInt() {
		var result int64
		aVal := a.AsInt()
		bVal := b.AsInt()

		switch op {
		case OP_ADD:
			result = aVal + bVal
		case OP_SUB:
			result = aVal - bVal
		case OP_MUL:
			result = aVal * bVal
		case OP_DIV:
			if bVal == 0 {
				return fmt.Errorf("division by zero")
			}
			result = aVal / bVal
		case OP_MOD:
			if bVal == 0 {
				return fmt.Errorf("modulo by zero")
			}
			result = aVal % bVal
		case OP_POW:
			result = intPow(aVal, bVal)
		}
		vm.push(IntVal(result))
		return nil
	}

	// Fast path for floats
	if a.IsFloat() && b.IsFloat() {
		var result float64
		aVal := a.AsFloat()
		bVal := b.AsFloat()

		switch op {
		case OP_ADD:
			result = aVal + bVal
		case OP_SUB:
			result = aVal - bVal
		case OP_MUL:
			result = aVal * bVal
		case OP_DIV:
			if bVal == 0 {
				return fmt.Errorf("division by zero")
			}
			result = aVal / bVal
		case OP_MOD:
			result = math.Mod(aVal, bVal)
		case OP_POW:
			result = math.Pow(aVal, bVal)
		}
		vm.push(FloatVal(result))
		return nil
	}

	// Slow path: convert to Objects for complex types (BigInt, etc.)
	aObj := a.AsObject()
	bObj := b.AsObject()

	// Handle BigInt
	aBigInt, aIsBigInt := aObj.(*evaluator.BigInt)
	bBigInt, bIsBigInt := bObj.(*evaluator.BigInt)
	if aIsBigInt && bIsBigInt {
		result := new(big.Int)
		switch op {
		case OP_ADD:
			result.Add(aBigInt.Value, bBigInt.Value)
		case OP_SUB:
			result.Sub(aBigInt.Value, bBigInt.Value)
		case OP_MUL:
			result.Mul(aBigInt.Value, bBigInt.Value)
		case OP_DIV:
			if bBigInt.Value.Sign() == 0 {
				return fmt.Errorf("division by zero")
			}
			result.Div(aBigInt.Value, bBigInt.Value)
		case OP_MOD:
			if bBigInt.Value.Sign() == 0 {
				return fmt.Errorf("modulo by zero")
			}
			result.Mod(aBigInt.Value, bBigInt.Value)
		case OP_POW:
			result.Exp(aBigInt.Value, bBigInt.Value, nil)
		}
		vm.push(ObjVal(&evaluator.BigInt{Value: result}))
		return nil
	}

	// Handle Rational
	aRat, aIsRat := aObj.(*evaluator.Rational)
	bRat, bIsRat := bObj.(*evaluator.Rational)
	if aIsRat && bIsRat {
		result := new(big.Rat)
		switch op {
		case OP_ADD:
			result.Add(aRat.Value, bRat.Value)
		case OP_SUB:
			result.Sub(aRat.Value, bRat.Value)
		case OP_MUL:
			result.Mul(aRat.Value, bRat.Value)
		case OP_DIV:
			if bRat.Value.Sign() == 0 {
				return fmt.Errorf("division by zero")
			}
			result.Quo(aRat.Value, bRat.Value)
		case OP_MOD:
			quo := new(big.Rat).Quo(aRat.Value, bRat.Value)
			floor := new(big.Int)
			floor.Div(quo.Num(), quo.Denom())
			floorRat := new(big.Rat).SetInt(floor)
			result.Sub(aRat.Value, new(big.Rat).Mul(floorRat, bRat.Value))
		}
		vm.push(ObjVal(&evaluator.Rational{Value: result}))
		return nil
	}

	// Try trait-based operator
	var opName string
	switch op {
	case OP_ADD:
		opName = "+"
	case OP_SUB:
		opName = "-"
	case OP_MUL:
		opName = "*"
	case OP_DIV:
		opName = "/"
	case OP_MOD:
		opName = "%"
	case OP_POW:
		opName = "**"
	}

	typeName := vm.getTypeName(a)
	if closure := vm.LookupOperator(typeName, opName); closure != nil {
		vm.push(ObjVal(closure))
		vm.push(a)
		vm.push(b)
		return vm.callClosure(closure, 2)
	}

	return fmt.Errorf("no operator %s for types %s and %s", opName, aObj.Type(), bObj.Type())
}

// intPow computes integer power
func intPow(base, exp int64) int64 {
	if exp < 0 {
		return 0
	}
	result := int64(1)
	for exp > 0 {
		if exp%2 == 1 {
			result *= base
		}
		base *= base
		exp /= 2
	}
	return result
}

// bitwiseOp performs bitwise operations
func (vm *VM) bitwiseOp(op Opcode) error {
	b := vm.pop()
	a := vm.pop()

	if a.IsInt() && b.IsInt() {
		var result int64
		aVal := a.AsInt()
		bVal := b.AsInt()

		switch op {
		case OP_BAND:
			result = aVal & bVal
		case OP_BOR:
			result = aVal | bVal
		case OP_BXOR:
			result = aVal ^ bVal
		case OP_LSHIFT:
			result = aVal << bVal
		case OP_RSHIFT:
			result = aVal >> bVal
		}
		vm.push(IntVal(result))
		return nil
	}

	var opName string
	switch op {
	case OP_BAND:
		opName = "&"
	case OP_BOR:
		opName = "|"
	case OP_BXOR:
		opName = "^"
	case OP_LSHIFT:
		opName = "<<"
	case OP_RSHIFT:
		opName = ">>"
	}

	typeName := vm.getTypeName(a)
	if closure := vm.LookupOperator(typeName, opName); closure != nil {
		vm.push(ObjVal(closure))
		vm.push(a)
		vm.push(b)
		return vm.callClosure(closure, 2)
	}

	return fmt.Errorf("no bitwise operator %s for type %s", opName, typeName)
}

// concatOp performs concatenation (++)
func (vm *VM) concatOp() error {
	b := vm.pop()
	a := vm.pop()

	// Slow path: boxing
	aObj := a.AsObject()
	bObj := b.AsObject()

	// List ++ List
	aList, aIsList := aObj.(*evaluator.List)
	bList, bIsList := bObj.(*evaluator.List)
	if aIsList && bIsList {
		vm.push(ObjVal(aList.Concat(bList)))
		return nil
	}

	// List ++ Tuple (variadic args are tuples)
	bTuple, bIsTuple := bObj.(*evaluator.Tuple)
	if aIsList && bIsTuple {
		bAsL := evaluator.NewList(bTuple.Elements)
		vm.push(ObjVal(aList.Concat(bAsL)))
		return nil
	}

	// Tuple ++ List
	aTuple, aIsTuple := aObj.(*evaluator.Tuple)
	if aIsTuple && bIsList {
		aAsL := evaluator.NewList(aTuple.Elements)
		vm.push(ObjVal(aAsL.Concat(bList)))
		return nil
	}

	// Bits ++ Bits
	aBits, aIsBits := aObj.(*evaluator.Bits)
	bBits, bIsBits := bObj.(*evaluator.Bits)
	if aIsBits && bIsBits {
		vm.push(ObjVal(aBits.Concat(bBits)))
		return nil
	}

	// Bytes ++ Bytes
	aBytes, aIsBytes := aObj.(*evaluator.Bytes)
	bBytes, bIsBytes := bObj.(*evaluator.Bytes)
	if aIsBytes && bIsBytes {
		vm.push(ObjVal(aBytes.Concat(bBytes)))
		return nil
	}

	typeName := vm.getTypeName(a)
	if closure := vm.LookupOperator(typeName, "++"); closure != nil {
		vm.push(ObjVal(closure))
		vm.push(a)
		vm.push(b)
		return vm.callClosure(closure, 2)
	}

	return fmt.Errorf("cannot concatenate %s and %s", aObj.Type(), bObj.Type())
}

// consOp performs cons operation (::)
func (vm *VM) consOp() error {
	b := vm.pop()
	a := vm.pop()

	bObj := b.AsObject()
	bList, ok := bObj.(*evaluator.List)
	if !ok {
		return fmt.Errorf(":: requires list on right side, got %s", bObj.Type())
	}

	vm.push(ObjVal(bList.Prepend(a.AsObject())))
	return nil
}

// getIndex gets element by index from list/map/tuple
func (vm *VM) getIndex(obj, index Value) (Value, error) {
	// Special fast path for List<Int> or Tuple<Int> if common?
	// For now, unbox
	objVal := obj.AsObject()
	idxVal := index.AsObject()

	switch o := objVal.(type) {
	case *evaluator.List:
		idx, ok := idxVal.(*evaluator.Integer)
		if !ok {
			return NilVal(), fmt.Errorf("list index must be integer, got %s", idxVal.Type())
		}
		i := int(idx.Value)
		if i < 0 {
			i = o.Len() + i
		}
		if i < 0 || i >= o.Len() {
			return NilVal(), fmt.Errorf("list index %d out of bounds (len=%d)", int(idx.Value), o.Len())
		}
		return ObjectToValue(o.Get(i)), nil

	case *evaluator.Tuple:
		idx, ok := idxVal.(*evaluator.Integer)
		if !ok {
			return NilVal(), fmt.Errorf("tuple index must be integer, got %s", idxVal.Type())
		}
		i := int(idx.Value)
		if i < 0 {
			i = len(o.Elements) + i
		}
		if i < 0 || i >= len(o.Elements) {
			return NilVal(), fmt.Errorf("tuple index %d out of bounds (len=%d)", int(idx.Value), len(o.Elements))
		}
		return ObjectToValue(o.Elements[i]), nil

	case *evaluator.Map:
		if val, ok := o.Get(idxVal); ok {
			// Return Some(value) for consistency with tree-walk
			return ObjVal(&evaluator.DataInstance{Name: "Some", Fields: []evaluator.Object{val}}), nil
		}
		// Return Zero for not found
		return ObjVal(&evaluator.DataInstance{Name: "Zero", Fields: nil}), nil

	case *evaluator.Bytes:
		idx, ok := idxVal.(*evaluator.Integer)
		if !ok {
			return NilVal(), fmt.Errorf("bytes index must be integer, got %s", idxVal.Type())
		}
		i := int(idx.Value)
		data := o.ToSlice()
		if i < 0 {
			i = len(data) + i
		}
		if i < 0 || i >= len(data) {
			// Return Zero for out of bounds
			return ObjVal(&evaluator.DataInstance{Name: "Zero", Fields: nil}), nil
		}
		// Return Some(byte)
		return ObjVal(&evaluator.DataInstance{Name: "Some", Fields: []evaluator.Object{&evaluator.Integer{Value: int64(data[i])}}}), nil

	case *evaluator.Bits:
		idx, ok := idxVal.(*evaluator.Integer)
		if !ok {
			return NilVal(), fmt.Errorf("bits index must be integer, got %s", idxVal.Type())
		}
		i := int(idx.Value)
		bitsLen := o.Len()
		if i < 0 {
			i = bitsLen + i
		}
		if i < 0 || i >= bitsLen {
			// Return Zero for out of bounds
			return ObjVal(&evaluator.DataInstance{Name: "Zero", Fields: nil}), nil
		}
		// Return Some(bit)
		return ObjVal(&evaluator.DataInstance{Name: "Some", Fields: []evaluator.Object{&evaluator.Integer{Value: int64(o.Get(i))}}}), nil

	default:
		return NilVal(), fmt.Errorf("cannot index %s", objVal.Type())
	}
}

// getField gets field from record
func (vm *VM) getField(obj Value, name string) (Value, error) {
	objVal := obj.AsObject()

	switch o := objVal.(type) {
	case *evaluator.RecordInstance:
		if val := o.Get(name); val != nil {
			return ObjectToValue(val), nil
		}
		return NilVal(), fmt.Errorf("record has no field '%s'", name)

	case *evaluator.DataInstance:
		if fn := vm.globals.Get(name); fn != nil {
			if function, ok := fn.(*evaluator.Function); ok {
				return ObjVal(&evaluator.BoundMethod{Receiver: objVal, Function: function}), nil
			}
			return ObjectToValue(fn), nil
		}
		return NilVal(), fmt.Errorf("DataInstance %s has no field or method '%s'", o.Name, name)

	case *evaluator.Map:
		key := evaluator.StringToList(name)
		if val, ok := o.Get(key); ok {
			return ObjectToValue(val), nil
		}
		return NilVal(), nil

	default:
		if fn := vm.globals.Get(name); fn != nil {
			if function, ok := fn.(*evaluator.Function); ok {
				return ObjVal(&evaluator.BoundMethod{Receiver: objVal, Function: function}), nil
			}
			return ObjectToValue(fn), nil
		}
		return NilVal(), fmt.Errorf("cannot get field from %s", objVal.Type())
	}
}

// comparisonOp performs comparison operations
func (vm *VM) comparisonOp(op Opcode) error {
	b := vm.pop()
	a := vm.pop()

	if a.IsInt() && b.IsInt() {
		var result bool
		aVal := a.AsInt()
		bVal := b.AsInt()
		switch op {
		case OP_LT:
			result = aVal < bVal
		case OP_LE:
			result = aVal <= bVal
		case OP_GT:
			result = aVal > bVal
		case OP_GE:
			result = aVal >= bVal
		}
		vm.push(BoolVal(result))
		return nil
	}

	if a.IsFloat() && b.IsFloat() {
		var result bool
		aVal := a.AsFloat()
		bVal := b.AsFloat()
		switch op {
		case OP_LT:
			result = aVal < bVal
		case OP_LE:
			result = aVal <= bVal
		case OP_GT:
			result = aVal > bVal
		case OP_GE:
			result = aVal >= bVal
		}
		vm.push(BoolVal(result))
		return nil
	}

	var opName string
	switch op {
	case OP_LT:
		opName = "<"
	case OP_LE:
		opName = "<="
	case OP_GT:
		opName = ">"
	case OP_GE:
		opName = ">="
	}

	// Slow path for objects
	aObj := a.AsObject()
	bObj := b.AsObject()

	typeName := vm.getTypeName(a)
	if closure := vm.LookupOperator(typeName, opName); closure != nil {
		vm.push(ObjVal(closure))
		vm.push(a)
		vm.push(b)
		if err := vm.callClosure(closure, 2); err != nil {
			return err
		}
		return nil
	}

	// Handle BigInt comparison
	aBigInt, aIsBigInt := aObj.(*evaluator.BigInt)
	bBigInt, bIsBigInt := bObj.(*evaluator.BigInt)
	if aIsBigInt && bIsBigInt {
		cmp := aBigInt.Value.Cmp(bBigInt.Value)
		var result bool
		switch op {
		case OP_LT:
			result = cmp < 0
		case OP_LE:
			result = cmp <= 0
		case OP_GT:
			result = cmp > 0
		case OP_GE:
			result = cmp >= 0
		}
		vm.push(BoolVal(result))
		return nil
	}

	// Handle Rational comparison
	aRat, aIsRat := aObj.(*evaluator.Rational)
	bRat, bIsRat := bObj.(*evaluator.Rational)
	if aIsRat && bIsRat {
		cmp := aRat.Value.Cmp(bRat.Value)
		var result bool
		switch op {
		case OP_LT:
			result = cmp < 0
		case OP_LE:
			result = cmp <= 0
		case OP_GT:
			result = cmp > 0
		case OP_GE:
			result = cmp >= 0
		}
		vm.push(BoolVal(result))
		return nil
	}

	// Handle Char comparison
	aChar, aIsChar := aObj.(*evaluator.Char)
	bChar, bIsChar := bObj.(*evaluator.Char)
	if aIsChar && bIsChar {
		var result bool
		switch op {
		case OP_LT:
			result = aChar.Value < bChar.Value
		case OP_LE:
			result = aChar.Value <= bChar.Value
		case OP_GT:
			result = aChar.Value > bChar.Value
		case OP_GE:
			result = aChar.Value >= bChar.Value
		}
		vm.push(BoolVal(result))
		return nil
	}

	// Handle Bytes comparison (lexicographic)
	aBytes, aIsBytes := aObj.(*evaluator.Bytes)
	bBytes, bIsBytes := bObj.(*evaluator.Bytes)
	if aIsBytes && bIsBytes {
		cmp := bytes.Compare(aBytes.ToSlice(), bBytes.ToSlice())
		var result bool
		switch op {
		case OP_LT:
			result = cmp < 0
		case OP_LE:
			result = cmp <= 0
		case OP_GT:
			result = cmp > 0
		case OP_GE:
			result = cmp >= 0
		}
		vm.push(BoolVal(result))
		return nil
	}

	// Handle Boolean comparison (false < true)
	if a.IsBool() && b.IsBool() {
		aVal := 0
		if a.AsBool() {
			aVal = 1
		}
		bVal := 0
		if b.AsBool() {
			bVal = 1
		}
		var result bool
		switch op {
		case OP_LT:
			result = aVal < bVal
		case OP_LE:
			result = aVal <= bVal
		case OP_GT:
			result = aVal > bVal
		case OP_GE:
			result = aVal >= bVal
		}
		vm.push(BoolVal(result))
		return nil
	}

	// Strings are List<Char> in this language, handled by List comparison below

	// Handle List comparison (lexicographic)
	aList, aIsList := aObj.(*evaluator.List)
	bList, bIsList := bObj.(*evaluator.List)
	if aIsList && bIsList {
		cmp := vm.compareLists(aList, bList)
		var result bool
		switch op {
		case OP_LT:
			result = cmp < 0
		case OP_LE:
			result = cmp <= 0
		case OP_GT:
			result = cmp > 0
		case OP_GE:
			result = cmp >= 0
		}
		vm.push(BoolVal(result))
		return nil
	}

	// Handle Tuple comparison (lexicographic)
	aTuple, aIsTuple := aObj.(*evaluator.Tuple)
	bTuple, bIsTuple := bObj.(*evaluator.Tuple)
	if aIsTuple && bIsTuple {
		cmp := vm.compareTuples(aTuple, bTuple)
		var result bool
		switch op {
		case OP_LT:
			result = cmp < 0
		case OP_LE:
			result = cmp <= 0
		case OP_GT:
			result = cmp > 0
		case OP_GE:
			result = cmp >= 0
		}
		vm.push(BoolVal(result))
		return nil
	}

	// Handle DataInstance comparison (for Option, Result, etc.)
	aData, aIsData := aObj.(*evaluator.DataInstance)
	bData, bIsData := bObj.(*evaluator.DataInstance)
	if aIsData && bIsData {
		cmp := vm.compareDataInstances(aData, bData)
		var result bool
		switch op {
		case OP_LT:
			result = cmp < 0
		case OP_LE:
			result = cmp <= 0
		case OP_GT:
			result = cmp > 0
		case OP_GE:
			result = cmp >= 0
		}
		vm.push(BoolVal(result))
		return nil
	}

	return vm.runtimeError("cannot compare %s and %s with %s", aObj.Type(), bObj.Type(), opName)
}

// compareLists returns -1, 0, or 1 for lexicographic comparison
func (vm *VM) compareLists(a, b *evaluator.List) int {
	aElems := a.ToSlice()
	bElems := b.ToSlice()
	minLen := len(aElems)
	if len(bElems) < minLen {
		minLen = len(bElems)
	}
	for i := 0; i < minLen; i++ {
		cmp := vm.compareObjects(ObjectToValue(aElems[i]), ObjectToValue(bElems[i]))
		if cmp != 0 {
			return cmp
		}
	}
	if len(aElems) < len(bElems) {
		return -1
	} else if len(aElems) > len(bElems) {
		return 1
	}
	return 0
}

// compareTuples returns -1, 0, or 1 for lexicographic comparison
func (vm *VM) compareTuples(a, b *evaluator.Tuple) int {
	minLen := len(a.Elements)
	if len(b.Elements) < minLen {
		minLen = len(b.Elements)
	}
	for i := 0; i < minLen; i++ {
		cmp := vm.compareObjects(ObjectToValue(a.Elements[i]), ObjectToValue(b.Elements[i]))
		if cmp != 0 {
			return cmp
		}
	}
	if len(a.Elements) < len(b.Elements) {
		return -1
	} else if len(a.Elements) > len(b.Elements) {
		return 1
	}
	return 0
}

// compareDataInstances compares DataInstances (for Option, Result ordering)
func (vm *VM) compareDataInstances(a, b *evaluator.DataInstance) int {
	// Compare by constructor name first (Zero < Some, Fail < Ok, etc.)
	if a.Name != b.Name {
		// Define ordering: Zero < Some, Fail < Ok
		aRank := vm.constructorRank(a.Name)
		bRank := vm.constructorRank(b.Name)
		if aRank < bRank {
			return -1
		}
		return 1
	}
	// Same constructor - compare fields
	minLen := len(a.Fields)
	if len(b.Fields) < minLen {
		minLen = len(b.Fields)
	}
	for i := 0; i < minLen; i++ {
		cmp := vm.compareObjects(ObjectToValue(a.Fields[i]), ObjectToValue(b.Fields[i]))
		if cmp != 0 {
			return cmp
		}
	}
	if len(a.Fields) < len(b.Fields) {
		return -1
	} else if len(a.Fields) > len(b.Fields) {
		return 1
	}
	return 0
}

// constructorRank returns ordering rank for Option/Result constructors
func (vm *VM) constructorRank(name string) int {
	switch name {
	case config.ZeroCtorName, config.FailCtorName:
		return 0
	case config.SomeCtorName, config.OkCtorName:
		return 1
	default:
		return 2
	}
}

// compareObjects returns -1, 0, or 1 for any two objects
func (vm *VM) compareObjects(a, b Value) int {
	if a.IsInt() && b.IsInt() {
		aVal := a.AsInt()
		bVal := b.AsInt()
		if aVal < bVal {
			return -1
		} else if aVal > bVal {
			return 1
		}
		return 0
	}

	if a.IsFloat() && b.IsFloat() {
		aVal := a.AsFloat()
		bVal := b.AsFloat()
		if aVal < bVal {
			return -1
		} else if aVal > bVal {
			return 1
		}
		return 0
	}

	if a.IsBool() && b.IsBool() {
		aVal := 0
		if a.AsBool() {
			aVal = 1
		}
		bVal := 0
		if b.AsBool() {
			bVal = 1
		}
		if aVal < bVal {
			return -1
		} else if aVal > bVal {
			return 1
		}
		return 0
	}

	// Fallback to object comparison
	aObj := a.AsObject()
	bObj := b.AsObject()

	// Integer comparison (already handled fast path, but kept for completeness if needed)
	if aInt, ok := aObj.(*evaluator.Integer); ok {
		if bInt, ok := bObj.(*evaluator.Integer); ok {
			if aInt.Value < bInt.Value {
				return -1
			} else if aInt.Value > bInt.Value {
				return 1
			}
			return 0
		}
	}
	// Float comparison (already handled fast path)
	if aFloat, ok := aObj.(*evaluator.Float); ok {
		if bFloat, ok := bObj.(*evaluator.Float); ok {
			if aFloat.Value < bFloat.Value {
				return -1
			} else if aFloat.Value > bFloat.Value {
				return 1
			}
			return 0
		}
	}
	// Boolean comparison (already handled fast path)
	if aBool, ok := aObj.(*evaluator.Boolean); ok {
		if bBool, ok := bObj.(*evaluator.Boolean); ok {
			aVal, bVal := 0, 0
			if aBool.Value {
				aVal = 1
			}
			if bBool.Value {
				bVal = 1
			}
			if aVal < bVal {
				return -1
			} else if aVal > bVal {
				return 1
			}
			return 0
		}
	}
	// Char comparison
	if aChar, ok := aObj.(*evaluator.Char); ok {
		if bChar, ok := bObj.(*evaluator.Char); ok {
			if aChar.Value < bChar.Value {
				return -1
			} else if aChar.Value > bChar.Value {
				return 1
			}
			return 0
		}
	}
	// List comparison
	if aList, ok := aObj.(*evaluator.List); ok {
		if bList, ok := bObj.(*evaluator.List); ok {
			return vm.compareLists(aList, bList)
		}
	}
	// Tuple comparison
	if aTuple, ok := aObj.(*evaluator.Tuple); ok {
		if bTuple, ok := bObj.(*evaluator.Tuple); ok {
			return vm.compareTuples(aTuple, bTuple)
		}
	}
	// DataInstance comparison
	if aData, ok := aObj.(*evaluator.DataInstance); ok {
		if bData, ok := bObj.(*evaluator.DataInstance); ok {
			return vm.compareDataInstances(aData, bData)
		}
	}
	// Fallback: compare by type name
	aType := string(aObj.Type())
	bType := string(bObj.Type())
	if aType < bType {
		return -1
	} else if aType > bType {
		return 1
	}
	return 0
}
