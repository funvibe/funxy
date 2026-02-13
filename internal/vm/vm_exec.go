package vm

import (
	"fmt"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/typesystem"
	"math/big"
	"strings"
)

func (vm *VM) executeOneOp(op Opcode) error {
	switch op {
	case OP_CONST:
		// TODO: Pre-convert constants to Value in Chunk to avoid runtime conversion
		vm.push(ObjectToValue(vm.readConstant()))

	case OP_NIL:
		vm.push(NilVal())

	case OP_TRUE:
		vm.push(BoolVal(true))

	case OP_FALSE:
		vm.push(BoolVal(false))

	case OP_POP:
		vm.pop()

	case OP_POP_BELOW:
		keepCount := int(vm.readByte())
		targetIdx := vm.sp - 1 - keepCount
		if targetIdx < 0 {
			return fmt.Errorf("stack underflow in OP_POP_BELOW: sp=%d, keep=%d", vm.sp, keepCount)
		}
		// Shift elements down
		copy(vm.stack[targetIdx:], vm.stack[targetIdx+1:vm.sp])
		vm.sp--
		// None out the old top (good hygiene)
		vm.stack[vm.sp] = NilVal()

	case OP_DUP:
		vm.push(vm.peek(0))

	case OP_ADD, OP_SUB, OP_MUL, OP_DIV, OP_MOD, OP_POW:
		if err := vm.binaryOp(op); err != nil {
			return err
		}

	case OP_BAND, OP_BOR, OP_BXOR, OP_LSHIFT, OP_RSHIFT:
		if err := vm.bitwiseOp(op); err != nil {
			return err
		}

	case OP_CONCAT:
		if err := vm.concatOp(); err != nil {
			return err
		}

	case OP_CONS:
		if err := vm.consOp(); err != nil {
			return err
		}

	case OP_NEG:
		val := vm.pop()
		if val.IsInt() {
			vm.push(IntVal(-val.AsInt()))
		} else if val.IsFloat() {
			vm.push(FloatVal(-val.AsFloat()))
		} else {
			// Slow path for BigInt/Rational
			obj := val.AsObject()
			switch v := obj.(type) {
			case *evaluator.BigInt:
				res := new(big.Int).Neg(v.Value)
				vm.push(ObjVal(&evaluator.BigInt{Value: res}))
			case *evaluator.Rational:
				res := new(big.Rat).Neg(v.Value)
				vm.push(ObjVal(&evaluator.Rational{Value: res}))
			default:
				return fmt.Errorf("unknown operator: -%s", val.RuntimeType().String())
			}
		}

	case OP_EQ:
		b := vm.pop()
		a := vm.pop()
		vm.push(BoolVal(a.Equals(b)))

	case OP_NE:
		b := vm.pop()
		a := vm.pop()
		vm.push(BoolVal(!a.Equals(b)))

	case OP_LT, OP_LE, OP_GT, OP_GE:
		if err := vm.comparisonOp(op); err != nil {
			return err
		}

	case OP_NOT:
		val := vm.pop()
		if val.IsBool() {
			vm.push(BoolVal(!val.AsBool()))
		} else if val.IsObj() {
			if b, ok := val.Obj.(*evaluator.Boolean); ok {
				vm.push(BoolVal(!b.Value))
			} else {
				return fmt.Errorf("operator ! not supported for %s", val.Obj.Type())
			}
		} else {
			return fmt.Errorf("operator ! not supported for %s", val.AsObject().Type())
		}

	case OP_BNOT:
		val := vm.pop()
		if val.IsInt() {
			vm.push(IntVal(^val.AsInt()))
		} else {
			return fmt.Errorf("~ operator expects integer, got %s", val.RuntimeType().String())
		}

	case OP_SET_TYPE_NAME:
		// Set TypeName on top of stack (for type-annotated records)
		// Clean version
		typeName := vm.readConstant().Inspect()
		if val := vm.peek(0); val.IsObj() {
			if rec, ok := val.AsObject().(*evaluator.RecordInstance); ok {
				rec.TypeName = typeName
			}
		}
		// Don't pop - value stays on stack

	case OP_SET_LIST_ELEM_TYPE:
		// Set ElementType on top of stack (for type-annotated lists like List<Int>)
		elemType := vm.readConstant().Inspect()
		// Remove quotes if present
		if len(elemType) >= 2 && elemType[0] == '"' && elemType[len(elemType)-1] == '"' {
			elemType = elemType[1 : len(elemType)-1]
		}
		if val := vm.peek(0); val.IsObj() {
			if list, ok := val.AsObject().(*evaluator.List); ok {
				list.ElementType = elemType
			}
		}
		// Don't pop - value stays on stack

	case OP_ITER_NEXT:
		// Get next item from iterator
		// Args: iterableSlot, lenSlot, indexSlot
		// Pushes: (item, continue_flag)
		iterableSlot := int(vm.readByte())
		lenSlot := int(vm.readByte())
		indexSlot := int(vm.readByte())

		iterable := vm.stack[vm.frame.base+iterableSlot]
		lenObj := vm.stack[vm.frame.base+lenSlot]
		var lenVal int64 = -1
		if lenObj.IsInt() {
			lenVal = lenObj.AsInt()
		}
		var indexVal int64 = 0
		if idx := vm.stack[vm.frame.base+indexSlot]; idx.IsInt() {
			indexVal = idx.AsInt()
		}

		if lenVal >= 0 {
			// Index-based iteration (List/Tuple)
			if indexVal >= lenVal {
				// End of iteration
				vm.push(NilVal())
				vm.push(BoolVal(false))
			} else {
				// Get item by index
				var item Value
				if iterable.IsObj() {
					switch v := iterable.AsObject().(type) {
					case *evaluator.List:
						item = ObjectToValue(v.Get(int(indexVal)))
					case *evaluator.Tuple:
						item = ObjectToValue(v.Elements[indexVal])
					}
				}
				// Increment index
				vm.stack[vm.frame.base+indexSlot] = IntVal(indexVal + 1)
				vm.push(item)
				vm.push(BoolVal(true))
			}
		} else {
			// Lazy iteration (iterator function)
			// iterable is the iterator function (Value)
			itemVal, err := vm.callNoArgs(iterable)
			if err != nil {
				return err
			}
			// Check if None (end of iteration)
			// itemVal is Value. We need to check if it's DataInstance "None" or "Some"
			if itemVal.IsObj() {
				if di, ok := itemVal.AsObject().(*evaluator.DataInstance); ok && di.Name == "None" {
					vm.push(NilVal())
					vm.push(BoolVal(false))
				} else if di, ok := itemVal.AsObject().(*evaluator.DataInstance); ok && di.Name == "Some" {
					// Extract value from Some(x)
					if len(di.Fields) > 0 {
						vm.push(ObjectToValue(di.Fields[0]))
					} else {
						vm.push(NilVal())
					}
					vm.push(BoolVal(true))
				} else {
					// Not Option - treat as end? Or just return the value?
					// Usually iterator returns Option<T>. If it returns something else, maybe it's not an iterator?
					// Assuming iterator contract returns Option.
					vm.push(NilVal())
					vm.push(BoolVal(false))
				}
			} else if itemVal.IsNil() {
				// Nil is often used as end of iteration in some languages, but here we use Option
				vm.push(NilVal())
				vm.push(BoolVal(false))
			} else {
				// Should not happen if iterator follows contract
				vm.push(NilVal())
				vm.push(BoolVal(false))
			}
		}

	case OP_RANGE:
		end := vm.pop()
		next := vm.pop()
		start := vm.pop()
		vm.push(ObjVal(&ObjRange{Start: start, Next: next, End: end}))

	case OP_MAKE_ITER:
		// Convert iterable to (iterable_or_iterator, length_or_minus1)
		// For List/Tuple: push list, push length (index-based)
		// For Iter types: push iterator function, push -1 (lazy iteration)
		val := vm.pop()
		if val.IsObj() {
			switch v := val.AsObject().(type) {
			case *evaluator.List:
				vm.push(val)
				vm.push(IntVal(int64(v.Len())))
			case *evaluator.Tuple:
				vm.push(val)
				vm.push(IntVal(int64(len(v.Elements))))
			case *ObjRange:
				// Range iterator
				// Check for Int/Char optimization
				start := v.Start
				end := v.End
				next := v.Next

				var current int64
				var endVal int64
				var step int64 = 1
				isOptimized := false
				isChar := false

				if start.IsInt() && end.IsInt() {
					isOptimized = true
					current = start.AsInt()
					endVal = end.AsInt()
					if next.Type != ValNil && next.IsInt() {
						step = next.AsInt() - current
					}
				} else if start.IsObj() && end.IsObj() {
					if sChar, ok := start.Obj.(*evaluator.Char); ok {
						if eChar, ok := end.Obj.(*evaluator.Char); ok {
							isOptimized = true
							isChar = true
							current = sChar.Value
							endVal = eChar.Value
							if next.Type != ValNil {
								if nChar, ok := next.Obj.(*evaluator.Char); ok {
									step = nChar.Value - current
								}
							}
						}
					}
				}

				if isOptimized {
					// Create stateful closure
					currPtr := new(int64)
					*currPtr = current

					iterFn := func(args []evaluator.Object) evaluator.Object {
						c := *currPtr
						if (step > 0 && c > endVal) || (step < 0 && c < endVal) {
							return &evaluator.DataInstance{Name: config.NoneCtorName, TypeName: config.OptionTypeName}
						}
						*currPtr += step

						var valObj evaluator.Object
						if isChar {
							valObj = &evaluator.Char{Value: c}
						} else {
							valObj = &evaluator.Integer{Value: c}
						}

						return &evaluator.DataInstance{
							Name:     config.SomeCtorName,
							TypeName: config.OptionTypeName,
							Fields:   []evaluator.Object{valObj},
						}
					}

					vm.push(ObjVal(&BuiltinClosure{Name: "range_iter", Fn: iterFn}))
					vm.push(IntVal(-1)) // Lazy
				} else {
					// Fallback to trait lookup
					typeName := vm.getTypeName(val)
					iterMethod := vm.LookupTraitMethodAny("Iter", typeName, "iter")
					if iterMethod != nil {
						iterFn, err := vm.callAndGetResult(ObjectToValue(iterMethod), val)
						if err != nil {
							return err
						}
						vm.push(iterFn)
						vm.push(IntVal(-1))
					} else {
						return fmt.Errorf("type %s is not iterable (no Iter instance)", typeName)
					}
				}
			default:
				// Try Iter trait - use lazy iteration
				typeName := vm.getTypeName(val)
				iterMethod := vm.LookupTraitMethodAny("Iter", typeName, "iter")
				if iterMethod != nil {
					// Call iter() to get iterator function
					iterFn, err := vm.callAndGetResult(ObjectToValue(iterMethod), val)
					if err != nil {
						return err
					}
					vm.push(iterFn)
					vm.push(IntVal(-1)) // -1 signals lazy iteration
				} else {
					return fmt.Errorf("type %s is not iterable (no Iter instance)", typeName)
				}
			}
		} else {
			return fmt.Errorf("type %s is not iterable", val.RuntimeType().String())
		}

	case OP_LEN:
		val := vm.pop()
		if val.IsObj() {
			switch v := val.AsObject().(type) {
			case *evaluator.List:
				vm.push(IntVal(int64(v.Len())))
			case *evaluator.Tuple:
				vm.push(IntVal(int64(len(v.Elements))))
			case *evaluator.Map:
				vm.push(IntVal(int64(v.Len())))
			case *evaluator.RecordInstance:
				// Check if this type has Iter trait - if so, it's not directly indexable
				typeName := vm.getTypeName(val)
				if vm.LookupTraitMethodAny("Iter", typeName, "iter") != nil {
					// Has Iter trait - needs to be converted to iterator first
					// For now, error - iterators don't have a fixed length
					return fmt.Errorf("type %s implements Iter and must be iterated via iterator, not indexed", typeName)
				}
				vm.push(IntVal(int64(len(v.Fields))))
			default:
				// Try to get length via Inspect (for strings stored as Lists)
				str := v.Inspect()
				vm.push(IntVal(int64(len(str))))
			}
		} else {
			vm.push(IntVal(0)) // Primitives have length 0 or error?
		}

	case OP_INTERP_CONCAT:
		// String interpolation concatenation - converts values to strings
		b := vm.pop()
		a := vm.pop()
		aStr := vm.objectToString(a.AsObject())
		bStr := vm.objectToString(b.AsObject())
		vm.push(ObjectToValue(evaluator.StringToList(aStr + bStr)))

	case OP_JUMP:
		offset := vm.readJumpOffset()
		newIP := vm.frame.ip + offset
		if newIP < 0 || newIP > len(vm.frame.chunk.Code) {
			return fmt.Errorf("jump out of bounds: ip=%d, offset=%d, len=%d", vm.frame.ip, offset, len(vm.frame.chunk.Code))
		}
		vm.frame.ip = newIP

	case OP_JUMP_IF_FALSE:
		offset := vm.readJumpOffset()
		if !vm.isTruthy(vm.peek(0)) {
			newIP := vm.frame.ip + offset
			if newIP < 0 || newIP > len(vm.frame.chunk.Code) {
				return fmt.Errorf("jump out of bounds: ip=%d, offset=%d, len=%d", vm.frame.ip, offset, len(vm.frame.chunk.Code))
			}
			vm.frame.ip = newIP
		}

	case OP_LOOP:
		offset := vm.readJumpOffset()
		newIP := vm.frame.ip - offset
		if newIP < 0 || newIP > len(vm.frame.chunk.Code) {
			return fmt.Errorf("loop jump out of bounds: ip=%d, offset=%d, len=%d", vm.frame.ip, offset, len(vm.frame.chunk.Code))
		}
		vm.frame.ip = newIP

	case OP_GET_LOCAL:
		slot := int(vm.readByte())
		idx := vm.frame.base + slot
		if idx >= vm.sp {
			return fmt.Errorf("local slot %d out of bounds (sp=%d)", idx, vm.sp)
		}
		vm.push(vm.stack[idx])

	case OP_SET_LOCAL:
		slot := int(vm.readByte())
		idx := vm.frame.base + slot
		if idx >= vm.sp {
			return fmt.Errorf("local slot %d out of bounds (sp=%d)", idx, vm.sp)
		}
		vm.stack[idx] = vm.peek(0)

	case OP_GET_GLOBAL:
		name := vm.readConstant().Inspect()

		// Priority:
		// 1. Module-specific globals (if closure has them)
		// 2. VM globals

		var val evaluator.Object
		var ok bool

		if vm.frame.closure != nil && vm.frame.closure.Globals != nil {
			// ModuleScope lookup
			val = vm.frame.closure.Globals.Globals.Get(name)
			ok = val != nil
		}

		if !ok {
			val = vm.globals.Globals.Get(name)
			ok = val != nil
		}

		if !ok {
			return fmt.Errorf("undefined variable: %s", name)
		}
		vm.push(ObjectToValue(val))

	case OP_SET_GLOBAL:
		name := vm.readConstant().Inspect()
		val := vm.peek(0).AsObject()

		if vm.frame.closure != nil && vm.frame.closure.Globals != nil {
			// Update shared module scope
			vm.frame.closure.Globals.Globals = vm.frame.closure.Globals.Globals.Put(name, val)

			// If we are in the top-level script, update the VM's main global map too.
			// This ensures that subsequent OP_CLOSURE instructions capture the updated globals.
			if vm.frame.closure.Function.Name == "<script>" {
				vm.globals = vm.frame.closure.Globals
			}
		} else {
			vm.globals.Globals = vm.globals.Globals.Put(name, val)
		}

	case OP_CLOSE_SCOPE:
		n := int(vm.readByte())
		if vm.sp < n+1 {
			return fmt.Errorf("stack underflow in OP_CLOSE_SCOPE: sp=%d, n=%d", vm.sp, n)
		}
		// Close upvalues for locals being removed (under the result)
		// Locals start at sp - 1 - n.
		// Note: closeUpvalues closes everything >= start, so it might close result too
		// if result was captured (unlikely for temporary).
		vm.closeUpvalues(vm.sp - 1 - n)

		result := vm.pop()
		vm.sp -= n
		vm.push(result)

	case OP_SET_TYPE_CONTEXT:
		typeNameConst, ok := vm.readConstant().(*stringConstant)
		if !ok {
			return fmt.Errorf("expected string constant for type context")
		}
		typeName := typeNameConst.Value
		vm.typeContextStack = append(vm.typeContextStack, typeName)

	case OP_CLEAR_TYPE_CONTEXT:
		if len(vm.typeContextStack) > 0 {
			vm.typeContextStack = vm.typeContextStack[:len(vm.typeContextStack)-1]
		}

	case OP_CLOSURE:
		constVal := vm.readConstant()
		fn, ok := constVal.(*CompiledFunction)
		if !ok {
			return fmt.Errorf("expected CompiledFunction constant, got %T", constVal)
		}
		closure := &ObjClosure{
			Function: fn,
			Upvalues: make([]*ObjUpvalue, fn.UpvalueCount),
		}
		// Inherit module globals from parent closure (for nested lambdas in module functions)
		if vm.frame.closure != nil && vm.frame.closure.Globals != nil {
			closure.Globals = vm.frame.closure.Globals
		}
		for i := 0; i < fn.UpvalueCount; i++ {
			isLocal := vm.readByte()
			index := int(vm.readByte())
			if isLocal == 1 {
				loc := vm.frame.base + index
				// Allow capturing the immediate next slot (vm.sp) for recursive closures
				// where the closure itself will occupy that slot.
				if loc > vm.sp {
					return fmt.Errorf("capture upvalue index %d out of bounds (sp=%d)", loc, vm.sp)
				}
				closure.Upvalues[i] = vm.captureUpvalue(loc)
			} else {
				if index < 0 || index >= len(vm.frame.closure.Upvalues) {
					return fmt.Errorf("upvalue index %d out of bounds (len=%d)", index, len(vm.frame.closure.Upvalues))
				}
				closure.Upvalues[i] = vm.frame.closure.Upvalues[index]
			}
		}
		vm.push(ObjVal(closure))

	case OP_GET_UPVALUE:
		slot := int(vm.readByte())
		if slot < 0 || slot >= len(vm.frame.closure.Upvalues) {
			return fmt.Errorf("upvalue slot %d out of bounds (len=%d)", slot, len(vm.frame.closure.Upvalues))
		}
		upvalue := vm.frame.closure.Upvalues[slot]
		if upvalue.Location >= 0 {
			vm.push(vm.stack[upvalue.Location])
		} else {
			vm.push(ObjectToValue(upvalue.Closed))
		}

	case OP_SET_UPVALUE:
		slot := int(vm.readByte())
		if slot < 0 || slot >= len(vm.frame.closure.Upvalues) {
			return fmt.Errorf("upvalue slot %d out of bounds (len=%d)", slot, len(vm.frame.closure.Upvalues))
		}
		upvalue := vm.frame.closure.Upvalues[slot]
		if upvalue.Location >= 0 {
			vm.stack[upvalue.Location] = vm.peek(0)
		} else {
			upvalue.Closed = vm.peek(0).AsObject()
		}

	case OP_CLOSE_UPVALUE:
		vm.closeUpvalues(vm.sp - 1)
		vm.pop()

	case OP_CALL:
		argCount := int(vm.readByte())
		if err := vm.callValue(vm.peek(argCount), argCount); err != nil {
			return err
		}

	case OP_SPREAD_ARG:
		// Wrap value in SpreadArg marker
		val := vm.pop()
		vm.push(ObjVal(&spreadArg{Value: val.AsObject()}))

	case OP_CALL_SPREAD:
		// Call with spread arguments - unpack spread args into individual args
		argCount := int(vm.readByte())

		vm.checkStack(argCount + 1)

		// Collect and unpack arguments
		var args []evaluator.Object
		for i := 0; i < argCount; i++ {
			arg := vm.stack[vm.sp-argCount+i]
			if arg.IsObj() {
				argObj := arg.AsObject()
				if spread, ok := argObj.(*spreadArg); ok {
					// Unpack tuple or list
					switch v := spread.Value.(type) {
					case *evaluator.Tuple:
						args = append(args, v.Elements...)
					case *evaluator.List:
						args = append(args, v.ToSlice()...)
					default:
						return fmt.Errorf("cannot spread non-sequence type: %s", v.Type())
					}
				} else {
					args = append(args, argObj)
				}
			} else {
				args = append(args, arg.AsObject())
			}
		}
		// Remove original args from stack
		vm.sp -= argCount
		// Push unpacked args
		for _, arg := range args {
			vm.push(ObjectToValue(arg))
		}
		// Call with actual arg count
		if err := vm.callValue(vm.peek(len(args)), len(args)); err != nil {
			return err
		}

	case OP_COMPOSE:
		// Function composition: f ,, g creates a VM composed function
		g := vm.pop().AsObject()
		f := vm.pop().AsObject()
		composed := &VMComposedFunction{F: f, G: g}
		vm.push(ObjVal(composed))

	case OP_REGISTER_TRAIT:
		// Register trait method: [closure] traitIdx typeIdx methodIdx
		traitNameConst, ok1 := vm.readConstant().(*stringConstant)
		typeNameConst, ok2 := vm.readConstant().(*stringConstant)
		methodNameConst, ok3 := vm.readConstant().(*stringConstant)

		if !ok1 || !ok2 || !ok3 {
			return fmt.Errorf("expected string constants for trait registration")
		}

		traitName := traitNameConst.Value
		typeName := typeNameConst.Value
		methodName := methodNameConst.Value

		closureObj := vm.pop().AsObject()
		closure, ok := closureObj.(*ObjClosure)
		if !ok {
			return fmt.Errorf("expected closure for trait method, got %s", closureObj.Type())
		}
		vm.RegisterTraitMethod(traitName, typeName, methodName, closure)

	case OP_DEFAULT:
		// Get default value for type using Default trait
		typeObj := vm.pop().AsObject()
		tObj, ok := typeObj.(*evaluator.TypeObject)
		if !ok {
			return fmt.Errorf("default expects a Type, got %s", typeObj.Type())
		}

		// Get type name and resolve aliases
		actualType := tObj.TypeVal
		typeName := ""
		switch t := actualType.(type) {
		case typesystem.TCon:
			typeName = t.Name
			// Check if this is a type alias and resolve it
			if resolvedObj := vm.typeAliases.Get(typeName); resolvedObj != nil {
				if typeObj, ok := resolvedObj.(*evaluator.TypeObject); ok {
					resolved := typeObj.TypeVal
					actualType = resolved
					// Update typeName based on resolved type
					if con, ok := resolved.(typesystem.TCon); ok {
						typeName = con.Name
					} else if app, ok := resolved.(typesystem.TApp); ok {
						if con, ok := app.Constructor.(typesystem.TCon); ok {
							typeName = con.Name
						}
					}
				}
			}
		case typesystem.TApp:
			if con, ok := t.Constructor.(typesystem.TCon); ok {
				typeName = con.Name
			}
		default:
			typeName = actualType.String()
		}

		// Look for Default.getDefault method in VM trait registry
		closure := vm.LookupTraitMethod("Default", typeName, "getDefault")
		if closure != nil {
			// Call getDefault with a dummy argument (it's nullary but takes self)
			vm.push(ObjVal(closure))
			vm.push(NilVal()) // dummy arg
			return vm.callClosure(closure, 1)
		}

		// Check if type has underlying record type in typeAliases
		if underlyingObj := vm.typeAliases.Get(typeName); underlyingObj != nil {
			if typeObj, ok := underlyingObj.(*evaluator.TypeObject); ok {
				underlyingType := typeObj.TypeVal
				result := vm.getDefaultForRecord(underlyingType, typeName)
				if result != nil {
					vm.push(ObjectToValue(result))
					break
				}
			}
		}

		// Fallback to evaluator for built-in defaults (using resolved type)
		eval := vm.getEvaluator()
		result := eval.GetDefaultForType(actualType)
		if err, ok := result.(*evaluator.Error); ok {
			return fmt.Errorf("%s", err.Message)
		}
		vm.push(ObjectToValue(result))

	case OP_TAIL_CALL:
		argCount := int(vm.readByte())
		callee := vm.peek(argCount)
		if err := vm.tailCallValue(callee, argCount); err != nil {
			return err
		}

	case OP_MAKE_LIST:
		// OP_MAKE_LIST takes a 16-bit operand representing the number of elements.
		// Although readConstantIndex() is named for reading constant pool indices,
		// it reads a uint16 which is used here directly as the element count.
		count := vm.readConstantIndex()
		elements := make([]evaluator.Object, count)
		for i := count - 1; i >= 0; i-- {
			elements[i] = vm.pop().AsObject()
		}
		vm.push(ObjVal(evaluator.NewList(elements)))

	case OP_MAKE_TUPLE:
		count := int(vm.readByte())
		elements := make([]evaluator.Object, count)
		for i := count - 1; i >= 0; i-- {
			elements[i] = vm.pop().AsObject()
		}
		vm.push(ObjVal(&evaluator.Tuple{Elements: elements}))

	case OP_MAKE_RECORD:
		fieldCount := int(vm.readByte())
		fields := make(map[string]evaluator.Object)
		for i := 0; i < fieldCount; i++ {
			value := vm.pop().AsObject()
			nameObj := vm.pop().AsObject()
			name := nameObj.Inspect()
			// Remove quotes if present
			if len(name) >= 2 && name[0] == '"' && name[len(name)-1] == '"' {
				name = name[1 : len(name)-1]
			}
			fields[name] = value
		}

		// Read type name index (2 bytes)
		typeNameIdx := vm.readConstantIndex()
		record := evaluator.NewRecord(fields)

		// Set nominal type name if provided
		if typeNameIdx != 0xFFFF {
			// Here we need manual check because 0xFFFF is a sentinel, not a valid index
			if typeNameIdx >= len(vm.frame.chunk.Constants) {
				panic(errInvalidConstantIndex)
			}
			typeNameConst := vm.frame.chunk.Constants[typeNameIdx]
			if strConst, ok := typeNameConst.(*stringConstant); ok {
				record.TypeName = strConst.Value
			}
		}

		vm.push(ObjVal(record))

	case OP_EXTEND_RECORD:
		// Stack: [base, name1, val1, name2, val2...] -> [new_record]
		fieldCount := int(vm.readByte())

		// 1. Extract new fields (reverse order because stack)
		newFields := make(map[string]evaluator.Object)
		for i := 0; i < fieldCount; i++ {
			value := vm.pop().AsObject()
			nameObj := vm.pop().AsObject()
			name := nameObj.Inspect()
			if len(name) >= 2 && name[0] == '"' && name[len(name)-1] == '"' {
				name = name[1 : len(name)-1]
			}
			newFields[name] = value
		}

		// 2. Get base record
		baseObj := vm.pop().AsObject()
		baseRec, ok := baseObj.(*evaluator.RecordInstance)
		if !ok {
			return vm.runtimeError("spread operator expects a record, got %s", baseObj.Type())
		}

		// 3. Clone base fields
		finalFields := make(map[string]evaluator.Object)
		for _, field := range baseRec.Fields {
			finalFields[field.Key] = field.Value
		}

		// 4. Overwrite with new fields
		for k, v := range newFields {
			finalFields[k] = v
		}

		// 5. Create new record
		newRec := evaluator.NewRecord(finalFields)

		// Row Polymorphism: If we added new fields (not just overwriting existing ones),
		// mark as extended to use structural typing instead of nominal
		if len(newFields) > 0 {
			addedNewField := false
			for k := range newFields {
				found := false
				for _, f := range baseRec.Fields {
					if f.Key == k {
						found = true
						break
					}
				}
				if !found {
					addedNewField = true
					break
				}
			}
			if addedNewField {
				newRec.RowPolyExtended = true
			} else {
				// Only preserve TypeName if we didn't add new fields
				newRec.TypeName = baseRec.TypeName
			}
		} else {
			// No new fields, just spread - preserve TypeName
			newRec.TypeName = baseRec.TypeName
		}

		vm.push(ObjVal(newRec))

	case OP_MAKE_MAP:
		pairCount := int(vm.readByte())
		m := evaluator.NewMap()
		for i := 0; i < pairCount; i++ {
			value := vm.pop().AsObject()
			key := vm.pop().AsObject()
			m = m.Put(key, value)
		}
		vm.push(ObjVal(m))

	case OP_GET_INDEX:
		index := vm.pop()
		obj := vm.pop()
		result, err := vm.getIndex(obj, index)
		if err != nil {
			return err
		}
		vm.push(result)

	case OP_GET_FIELD:
		// Inspect is safe, but let's check if it's a string constant for correctness
		constVal := vm.readConstant()
		fieldName := constVal.Inspect()
		if strConst, ok := constVal.(*stringConstant); ok {
			fieldName = strConst.Value
		} else if len(fieldName) >= 2 && fieldName[0] == '"' && fieldName[len(fieldName)-1] == '"' {
			fieldName = fieldName[1 : len(fieldName)-1]
		}
		obj := vm.pop()
		result, err := vm.getField(obj, fieldName)
		if err != nil {
			return err
		}
		vm.push(result)

	case OP_OPTIONAL_CHAIN_FIELD:
		// Optional chaining: obj?.field
		// For Some/Ok: extract inner, get field, wrap back
		// For None/Fail: return unchanged
		fieldName := vm.readConstant().Inspect()
		if len(fieldName) >= 2 && fieldName[0] == '"' && fieldName[len(fieldName)-1] == '"' {
			fieldName = fieldName[1 : len(fieldName)-1]
		}
		obj := vm.pop()

		if obj.IsObj() {
			if data, ok := obj.AsObject().(*evaluator.DataInstance); ok {
				if vm.isEmptyDataInstance(data) {
					// None/Fail - return unchanged
					vm.push(obj)
				} else if vm.isWrapperDataInstance(data) || len(data.Fields) == 1 {
					// Some/Ok or user-defined wrapper with 1 field - extract inner, get field, wrap back
					if len(data.Fields) == 0 {
						vm.push(obj)
					} else {
						inner := ObjectToValue(data.Fields[0])
						result, err := vm.getField(inner, fieldName)
						if err != nil {
							return err
						}
						// Wrap result in same wrapper type
						vm.push(ObjVal(&evaluator.DataInstance{
							Name:     data.Name,
							Fields:   []evaluator.Object{result.AsObject()},
							TypeName: data.TypeName,
						}))
					}
				} else {
					// For other DataInstances with 0 or 2+ fields, try to get field directly
					result, err := vm.getField(obj, fieldName)
					if err != nil {
						return err
					}
					vm.push(result)
				}
			} else if _, ok := obj.AsObject().(*evaluator.Nil); ok {
				vm.push(obj)
			} else {
				// Regular object - just get field
				result, err := vm.getField(obj, fieldName)
				if err != nil {
					return err
				}
				vm.push(result)
			}
		} else if obj.IsNil() {
			vm.push(obj)
		} else {
			// Regular value - just get field
			result, err := vm.getField(obj, fieldName)
			if err != nil {
				return err
			}
			vm.push(result)
		}

	// Pattern matching opcodes
	case OP_CHECK_TAG:
		// Check if top of stack is DataInstance with given tag name
		// Inspect() is safe on any Object, so this was actually safe, but let's be consistent
		tagName := vm.readConstant().Inspect()
		// Remove quotes if it's a string constant (Inspect adds quotes)
		if len(tagName) >= 2 && tagName[0] == '"' && tagName[len(tagName)-1] == '"' {
			tagName = tagName[1 : len(tagName)-1]
		}
		val := vm.peek(0)
		if val.IsObj() {
			if data, ok := val.AsObject().(*evaluator.DataInstance); ok && data.Name == tagName {
				vm.push(BoolVal(true))
			} else {
				vm.push(BoolVal(false))
			}
		} else {
			vm.push(BoolVal(false))
		}

	case OP_GET_DATA_FIELD:
		// Get field from DataInstance by index
		fieldIdx := int(vm.readByte())
		val := vm.pop()
		if !val.IsObj() {
			return fmt.Errorf("expected DataInstance, got %s", val.RuntimeType().String())
		}
		data, ok := val.AsObject().(*evaluator.DataInstance)
		if !ok {
			return fmt.Errorf("expected DataInstance, got %s", val.RuntimeType().String())
		}
		if fieldIdx >= len(data.Fields) {
			return fmt.Errorf("field index %d out of bounds (len=%d)", fieldIdx, len(data.Fields))
		}
		vm.push(ObjectToValue(data.Fields[fieldIdx]))

	case OP_CHECK_LIST_LEN:
		// Check list length: operand encodes (op, length) where op: 0=exact, 1=at_least
		op := vm.readByte()
		// readConstantIndex returns int, not accessing constants pool here.
		// It's used as an immediate value (length).
		length := int(vm.readConstantIndex())
		val := vm.peek(0)
		if val.IsObj() {
			list, ok := val.AsObject().(*evaluator.List)
			if !ok {
				vm.push(BoolVal(false))
			} else {
				listLen := list.Len()
				var match bool
				if op == 0 {
					match = listLen == length
				} else {
					match = listLen >= length
				}
				vm.push(BoolVal(match))
			}
		} else {
			vm.push(BoolVal(false))
		}

	case OP_GET_LIST_ELEM:
		// Get list element by index (index on stack)
		idxObj := vm.pop()
		val := vm.pop()
		if !val.IsObj() {
			return fmt.Errorf("expected List, got %s", val.RuntimeType().String())
		}
		list, ok := val.AsObject().(*evaluator.List)
		if !ok {
			return fmt.Errorf("expected List, got %s", val.RuntimeType().String())
		}
		if !idxObj.IsInt() {
			return fmt.Errorf("list index must be Integer, got %s", idxObj.RuntimeType().String())
		}
		i := int(idxObj.AsInt())
		// Support negative indexing
		if i < 0 {
			i = list.Len() + i
		}
		if i >= list.Len() || i < 0 {
			return fmt.Errorf("list index %d out of bounds (len=%d)", int(idxObj.AsInt()), list.Len())
		}
		vm.push(ObjectToValue(list.Get(i)))

	case OP_GET_LIST_REST:
		// Get rest of list from index
		fromIdx := int(vm.readByte())
		val := vm.pop()
		if !val.IsObj() {
			return fmt.Errorf("expected List, got %s", val.RuntimeType().String())
		}
		list, ok := val.AsObject().(*evaluator.List)
		if !ok {
			return fmt.Errorf("expected List, got %s", val.RuntimeType().String())
		}
		// Create new list with elements from fromIdx onwards
		listLen := list.Len()
		if fromIdx >= listLen {
			vm.push(ObjVal(evaluator.NewList([]evaluator.Object{})))
		} else {
			restElems := make([]evaluator.Object, listLen-fromIdx)
			for i := fromIdx; i < listLen; i++ {
				restElems[i-fromIdx] = list.Get(i)
			}
			vm.push(ObjVal(evaluator.NewList(restElems)))
		}

	case OP_CHECK_TYPE:
		// Check if value is of given type
		expectedTypeConst, ok := vm.readConstant().(*stringConstant)
		if !ok {
			return fmt.Errorf("expected string constant for type check")
		}
		expectedType := expectedTypeConst.Value
		val := vm.peek(0)
		actualType := vm.getTypeName(val)
		// Also check against Object.Type()
		match := actualType == expectedType || (val.IsObj() && string(val.AsObject().Type()) == expectedType)

		// Special cases
		if !match && val.IsObj() {
			obj := val.AsObject()
			switch expectedType {
			case "Int":
				_, match = obj.(*evaluator.Integer)
			case "Float":
				_, match = obj.(*evaluator.Float)
			case "Bool":
				_, match = obj.(*evaluator.Boolean)
			case "String":
				_, match = obj.(*evaluator.List) // Strings are lists
			case "Nil":
				_, match = obj.(*evaluator.Nil)
			}
		}

		// Check aliases if not matched yet
		if !match && vm.typeAliases != nil {
			if aliasObj := vm.typeAliases.Get(expectedType); aliasObj != nil {
				if typeObj, ok := aliasObj.(*evaluator.TypeObject); ok {
					switch t := typeObj.TypeVal.(type) {
					case typesystem.TFunc:
						if val.IsObj() {
							switch val.AsObject().(type) {
							case *evaluator.Function, *evaluator.Builtin, *evaluator.ClassMethod, *evaluator.BoundMethod, *evaluator.OperatorFunction, *evaluator.ComposedFunction, *evaluator.PartialApplication, *ObjClosure, *VMComposedFunction:
								match = true
							}
						}
					case typesystem.TRecord:
						if val.IsObj() {
							if rec, ok := val.AsObject().(*evaluator.RecordInstance); ok {
								// Check fields
								match = true
								for fieldName := range t.Fields {
									if rec.Get(fieldName) == nil {
										match = false
										break
									}
								}
							}
						}
					case typesystem.TTuple:
						if val.IsObj() {
							if tuple, ok := val.AsObject().(*evaluator.Tuple); ok {
								if len(tuple.Elements) == len(t.Elements) {
									match = true
								}
							}
						}
					}

					// Check named alias (e.g. type A = B)
					if !match {
						aliasName := evaluator.ExtractTypeConstructorName(typeObj.TypeVal)
						if aliasName != "" && aliasName == actualType {
							match = true
						}
					}
				}
			}
		}

		vm.push(BoolVal(match))

	case OP_SET_FIELD:
		// Set field in record: [record, value] -> [record] (mutates in-place for reference semantics)
		fieldNameConst, ok := vm.readConstant().(*stringConstant)
		if !ok {
			return fmt.Errorf("expected string constant for field name")
		}
		fieldName := fieldNameConst.Value
		value := vm.pop().AsObject()
		obj := vm.pop()
		if !obj.IsObj() {
			return fmt.Errorf("cannot set field on %s", obj.RuntimeType().String())
		}
		record, ok := obj.AsObject().(*evaluator.RecordInstance)
		if !ok {
			return fmt.Errorf("cannot set field on %s", obj.RuntimeType().String())
		}
		// Mutate record in-place for reference semantics
		record.Set(fieldName, value)
		vm.push(obj) // Push original Value (record pointer)

	case OP_SET_INDEX:
		// Set element in list/map: [collection, index, value] -> [new_collection]
		value := vm.pop().AsObject()
		index := vm.pop().AsObject()
		obj := vm.pop()
		if !obj.IsObj() {
			return fmt.Errorf("cannot set index on %s", obj.RuntimeType().String())
		}
		switch coll := obj.AsObject().(type) {
		case *evaluator.List:
			idx, ok := index.(*evaluator.Integer)
			if !ok {
				return fmt.Errorf("list index must be integer")
			}
			// Create new list with updated element
			newList := coll.Set(int(idx.Value), value)
			vm.push(ObjVal(newList))
		case *evaluator.Map:
			newMap := coll.Put(index, value)
			vm.push(ObjVal(newMap))
		default:
			return fmt.Errorf("cannot set index on %s", obj.RuntimeType().String())
		}

	case OP_COALESCE:
		// Null coalescing: check if value isEmpty
		// Stack: [value] -> [unwrapped_or_value, bool]
		// bool = true if not empty (value was unwrapped)
		// bool = false if empty (default should be used)
		val := vm.peek(0)

		// Check for Option/Result types
		if val.IsObj() {
			valObj := val.AsObject()
			if di, ok := valObj.(*evaluator.DataInstance); ok {
				if vm.isEmptyDataInstance(di) {
					// Empty - push false (keep on stack for pop later)
					vm.push(BoolVal(false))
				} else if vm.isWrapperDataInstance(di) || len(di.Fields) == 1 {
					// Not empty wrapper - unwrap and push true
					if len(di.Fields) > 0 {
						vm.pop()
						vm.push(ObjectToValue(di.Fields[0]))
						vm.push(BoolVal(true))
					} else {
						vm.push(BoolVal(false))
					}
				} else {
					// Unknown DataInstance with 0 or 2+ fields - treat as non-empty
					vm.push(BoolVal(true))
				}
			} else if _, ok := valObj.(*evaluator.Nil); ok {
				// Nil is empty
				vm.push(BoolVal(false))
			} else {
				// Other values are not empty, keep as-is
				vm.push(BoolVal(true))
			}
		} else {
			// Primitive values (Int/Float/Bool/NilVal)
			if val.IsNil() {
				vm.push(BoolVal(false))
			} else {
				vm.push(BoolVal(true))
			}
		}

	case OP_REGISTER_EXTENSION:
		typeNameConst, ok1 := vm.readConstant().(*stringConstant)
		methodNameConst, ok2 := vm.readConstant().(*stringConstant)

		if !ok1 || !ok2 {
			return fmt.Errorf("expected string constants for extension registration")
		}

		typeName := typeNameConst.Value
		methodName := methodNameConst.Value

		closureObj := vm.pop().AsObject()
		closure, ok := closureObj.(*ObjClosure)
		if !ok {
			return fmt.Errorf("expected closure for extension method, got %s", closureObj.Type())
		}

		// extensionMethods[typeName]
		var methodMap *PersistentMap
		if val := vm.extensionMethods.Get(typeName); val != nil {
			methodMap = val.(*PersistentMap)
		} else {
			methodMap = EmptyMap()
		}

		// extensionMethods[typeName][methodName] = closure
		methodMap = methodMap.Put(methodName, closure)
		vm.extensionMethods = vm.extensionMethods.Put(typeName, methodMap)

	case OP_REGISTER_TYPE_ALIAS:
		nameConst, ok := vm.readConstant().(*stringConstant)
		if !ok {
			return fmt.Errorf("expected string constant for type alias name")
		}
		name := nameConst.Value
		typeObj := vm.pop().AsObject()

		if vm.typeAliases == nil {
			vm.typeAliases = EmptyMap()
		}
		vm.typeAliases = vm.typeAliases.Put(name, typeObj)

	case OP_CALL_METHOD:
		// Call method on object: [receiver, arg1, arg2, ...] -> [result]
		// Stack layout: receiver, args... (receiver at bottom)
		nameIdx := vm.readConstantIndex()
		if nameIdx >= len(vm.frame.chunk.Constants) {
			panic(errInvalidConstantIndex)
		}
		methodNameConst, ok := vm.frame.chunk.Constants[nameIdx].(*stringConstant)
		if !ok {
			return fmt.Errorf("expected string constant for method name")
		}
		methodName := methodNameConst.Value
		argCount := int(vm.readByte())

		// Get receiver (it's below the arguments on the stack)
		vm.checkStack(argCount + 1)
		receiverIdx := vm.sp - argCount - 1
		receiver := vm.stack[receiverIdx]

		// 1. Special case: Check if receiver is RecordInstance and has field
		// This prioritizes record fields (e.g. module functions) over global extensions
		// Fixes issue where module.func() was treated as extension method call
		if receiver.IsObj() {
			if rec, ok := receiver.AsObject().(*evaluator.RecordInstance); ok {
				if val := rec.Get(methodName); val != nil {
					vm.stack[receiverIdx] = ObjectToValue(val)
					return vm.callValue(ObjectToValue(val), argCount)
				}
			}
		}

		// 2. Try to find an extension method
		// Priority:
		// a. Registered extension methods (namespaced imports support this)
		// b. Global functions (legacy support for scripts)

		typeName := vm.getTypeName(receiver)
		var extFn evaluator.Object

		// Check registered extensions
		if typeMapObj := vm.extensionMethods.Get(typeName); typeMapObj != nil {
			typeMap := typeMapObj.(*PersistentMap)
			if fnObj := typeMap.Get(methodName); fnObj != nil {
				extFn = fnObj
			}
		}

		// Fallback to globals if not found
		if extFn == nil {
			if fn := vm.globals.Globals.Get(methodName); fn != nil {
				extFn = fn
			}
		}

		// Also check closure context globals if present
		if extFn == nil && vm.frame.closure != nil && vm.frame.closure.Globals != nil {
			if fn := vm.frame.closure.Globals.Globals.Get(methodName); fn != nil {
				extFn = fn
			}
		}

		if extFn != nil {
			// Extension method found - rearrange stack to: fn, receiver, args
			// Currently: receiver, args
			// Need: fn, receiver, args
			// Shift receiver and args up by 1, then insert fn
			// Only if we have space... check stack size?
			// vm.push checks size, but we are shifting.
			if vm.sp >= len(vm.stack) {
				// Grow stack logic repeated (should extract to method)
				growBy := StackGrowthIncrement
				if len(vm.stack) > growBy {
					growBy = len(vm.stack)
				}
				newStack := make([]Value, len(vm.stack)+growBy)
				copy(newStack, vm.stack)
				vm.stack = newStack
			}

			copy(vm.stack[receiverIdx+1:vm.sp+1], vm.stack[receiverIdx:vm.sp])
			vm.stack[receiverIdx] = ObjectToValue(extFn)
			vm.sp++

			// Now call with argCount + 1 (receiver is an argument)
			return vm.callValue(ObjectToValue(extFn), argCount+1)
		}

		// 3. No extension method - try getting field from receiver and calling it
		fieldVal, err := vm.getField(receiver, methodName)
		if err != nil {
			return vm.runtimeError("cannot call method '%s' on %s: %v", methodName, receiver.RuntimeType().String(), err)
		}

		// Replace receiver with the callable, keep args
		vm.stack[receiverIdx] = fieldVal
		return vm.callValue(fieldVal, argCount)

	case OP_CHECK_TUPLE_LEN:
		// Check tuple length (also supports List for variadic args compatibility)
		length := int(vm.readByte())
		val := vm.peek(0)
		if val.IsObj() {
			switch v := val.AsObject().(type) {
			case *evaluator.Tuple:
				vm.push(BoolVal(len(v.Elements) == length))
			case *evaluator.List:
				vm.push(BoolVal(v.Len() == length))
			default:
				vm.push(BoolVal(false))
			}
		} else {
			vm.push(BoolVal(false))
		}

	case OP_CHECK_TUPLE_LEN_GE:
		// Check tuple length >= N (for spread patterns, also supports List)
		length := int(vm.readByte())
		val := vm.peek(0)
		if val.IsObj() {
			switch v := val.AsObject().(type) {
			case *evaluator.Tuple:
				vm.push(BoolVal(len(v.Elements) >= length))
			case *evaluator.List:
				vm.push(BoolVal(v.Len() >= length))
			default:
				vm.push(BoolVal(false))
			}
		} else {
			vm.push(BoolVal(false))
		}

	case OP_MATCH_STRING_PATTERN:
		// Match string against pattern with captures (legacy - returns Map or Nil)
		constVal := vm.readConstant()
		patternParts, ok := constVal.(*StringPatternParts)
		if !ok {
			return fmt.Errorf("expected StringPatternParts constant, got %T", constVal)
		}

		val := vm.pop()

		// Convert List<Char> to string
		var str string
		if val.IsObj() {
			if listVal, ok := val.AsObject().(*evaluator.List); ok {
				str = evaluator.ListToString(listVal)
			} else {
				vm.push(NilVal())
				return nil
			}
		} else {
			vm.push(NilVal())
			return nil
		}

		// Match pattern
		matched, captures := evaluator.MatchStringPattern(patternParts.Parts, str)
		if !matched {
			vm.push(NilVal())
			return nil
		}

		// Create Map with captures
		result := evaluator.NewMap()
		for name, value := range captures {
			result = result.Put(evaluator.StringToList(name), evaluator.StringToList(value))
		}
		vm.push(ObjVal(result))

	case OP_MATCH_STRING_EXTRACT:
		// Match string, pop input, push captures then bool.
		// Format: OP_MATCH_STRING_EXTRACT <pattern_idx:uint16> <capture_count:uint8>
		// We manually read the operands to ensure correct parsing order and bounds checking.
		partsIdx := vm.readConstantIndex()
		captureCount := int(vm.readByte())

		if partsIdx >= len(vm.frame.chunk.Constants) {
			panic(errInvalidConstantIndex)
		}
		constVal := vm.frame.chunk.Constants[partsIdx]

		patternParts, ok := constVal.(*StringPatternParts)
		if !ok {
			return fmt.Errorf("expected StringPatternParts constant, got %T", constVal)
		}

		val := vm.pop() // pop input string

		// Convert List<Char> to string
		var str string
		isString := false
		if val.IsObj() {
			if listVal, ok := val.AsObject().(*evaluator.List); ok {
				str = evaluator.ListToString(listVal)
				isString = true
			}
		}

		if !isString {
			// Not a string - push dummy values for captures then false
			for i := 0; i < captureCount; i++ {
				vm.push(NilVal())
			}
			vm.push(BoolVal(false))
			return nil
		}

		// Match pattern
		matched, captures := evaluator.MatchStringPattern(patternParts.Parts, str)
		if !matched {
			// No match - push dummy values for captures then false
			for i := 0; i < captureCount; i++ {
				vm.push(NilVal())
			}
			vm.push(BoolVal(false))
			return nil
		}

		// Match succeeded - push captures in pattern order, then true
		for _, part := range patternParts.Parts {
			if part.IsCapture {
				value := captures[part.Value]
				vm.push(ObjVal(evaluator.StringToList(value)))
			}
		}
		vm.push(BoolVal(true))

	case OP_TRAIT_OP:
		// Trait-based operator dispatch (e.g., <>, <*>, >>=, ??, ?.)
		opStrConst, ok := vm.readConstant().(*stringConstant)
		if !ok {
			return fmt.Errorf("expected string constant for trait operator")
		}
		opStr := opStrConst.Value
		right := vm.pop()
		left := vm.pop()

		// Native trait lookup first
		typeName := vm.getTypeName(left)

		// Strategy for MPTC lookup:
		// For Multi-Parameter Type Classes (like Convert<From, To>), implementations are registered
		// with composite keys (e.g. "Int_String").
		// If the standard lookup by single type name fails ("Int"), we attempt to construct
		// a composite key using the current type context (which often represents the expected 'To' type).

		lookupTypeName := typeName
		if context := vm.getTypeContext(); context != "" {
			// If context is distinct from typeName, try combining
			// e.g. typeName="Int", context="String" -> "Int_String"
			if context != typeName {
				composite := typeName + "_" + context
				// Check if we have methods registered for this composite key
				// This is heuristic and assumes strict naming convention "Arg1_Arg2"
				// But traitMethods access is cheap.
				if vm.traitMethods.Get("Convert") != nil {
					// We only do this for Convert trait? Or generically?
					// OP_TRAIT_OP doesn't know it's Convert. It knows opStr (e.g. "convert").
					// We need to find WHICH trait defines this operator.
					// LookupOperator scans ALL traits.

					// Let's modify LookupOperator to support trying the composite key if provided.
					// But LookupOperator is generic.
				}
				lookupTypeName = composite
			}
		}

		if closure := vm.LookupOperator(lookupTypeName, opStr); closure != nil {
			// Found with composite key!
			vm.nextImplicitContext = lookupTypeName
			vm.push(ObjVal(closure))
			vm.push(left)
			vm.push(right)
			if err := vm.callClosure(closure, 2); err != nil {
				return err
			}
		} else if closure := vm.LookupOperator(typeName, opStr); closure != nil {
			// Standard lookup
			vm.nextImplicitContext = typeName
			vm.push(ObjVal(closure))
			vm.push(left)
			vm.push(right)
			if err := vm.callClosure(closure, 2); err != nil {
				return err
			}
		} else if bc := vm.LookupBuiltinOperator(typeName, opStr); bc != nil {
			// Call builtin trait operator

			// Push dummy frame for context so VMCallHandler can inherit it
			if vm.frameCount >= len(vm.frames) {
				// Grow frames
				growBy := FrameGrowthIncrement
				if len(vm.frames) > growBy {
					growBy = len(vm.frames)
				}
				newFrames := make([]CallFrame, len(vm.frames)+growBy)
				copy(newFrames, vm.frames[:vm.frameCount])
				vm.frames = newFrames
			}

			// Use a dummy closure/chunk to avoid nil dereferences
			vm.frames[vm.frameCount] = CallFrame{
				ImplicitTypeContext:      typeName,
				ExplicitTypeContextDepth: len(vm.typeContextStack),
				base:                     vm.sp,
				closure:                  &ObjClosure{Function: &CompiledFunction{Name: "<builtin_op_context>"}},
				chunk:                    &Chunk{},
			}
			vm.frameCount++
			originalFrame := vm.frame
			vm.frame = &vm.frames[vm.frameCount-1]

			result := bc.Fn([]evaluator.Object{left.AsObject(), right.AsObject()})

			// Pop dummy frame
			vm.frameCount--
			vm.frame = originalFrame

			if err, ok := result.(*evaluator.Error); ok {
				return fmt.Errorf("%s", err.Message)
			}
			vm.push(ObjectToValue(result))
		} else {
			// Fallback to evaluator for builtin trait operators
			eval := vm.getEvaluator()
			// Evaluator handles its own context
			result := eval.EvalInfixExpression(opStr, left.AsObject(), right.AsObject())
			if err, ok := result.(*evaluator.Error); ok {
				return fmt.Errorf("%s", err.Message)
			}
			vm.push(ObjectToValue(result))
		}

	case OP_TUPLE_SLICE:
		// Get slice of tuple from start (also supports List for variadic args)
		startObj := vm.pop()
		obj := vm.pop().AsObject()

		if !startObj.IsInt() {
			if startObj.AsObject() == nil {
				return fmt.Errorf("tuple slice index must be Integer, got nil object")
			}
			return fmt.Errorf("tuple slice index must be Integer, got %s", startObj.RuntimeType().String())
		}
		start := int(startObj.AsInt())

		switch v := obj.(type) {
		case *evaluator.Tuple:
			if start < 0 || start > len(v.Elements) {
				return fmt.Errorf("tuple slice index out of bounds: %d (len=%d)", start, len(v.Elements))
			}
			rest := &evaluator.Tuple{Elements: v.Elements[start:]}
			vm.push(ObjVal(rest))
		case *evaluator.List:
			if start < 0 || start > v.Len() {
				return fmt.Errorf("list slice index out of bounds: %d (len=%d)", start, v.Len())
			}
			rest := v.Slice(start, v.Len())
			vm.push(ObjVal(rest))
		default:
			if obj == nil {
				return fmt.Errorf("expected Tuple or List, got nil")
			}
			return fmt.Errorf("expected Tuple or List, got %s", obj.Type())
		}

	case OP_LIST_SLICE:
		// Get slice of list from start to end
		startObj := vm.pop()
		list := vm.pop().AsObject()

		if !startObj.IsInt() {
			if startObj.AsObject() == nil {
				return fmt.Errorf("list slice index must be Integer, got nil object")
			}
			return fmt.Errorf("list slice index must be Integer, got %s", startObj.RuntimeType().String())
		}
		start := int(startObj.AsInt())

		listVal, ok := list.(*evaluator.List)
		if !ok {
			if list == nil {
				return fmt.Errorf("expected List for slice operation, got nil")
			}
			return fmt.Errorf("expected List for slice operation, got %s", list.Type())
		}

		length := listVal.Len()
		if start < 0 || start > length {
			return fmt.Errorf("slice bounds out of range: start=%d, length=%d", start, length)
		}

		rest := listVal.Slice(start, length)
		vm.push(ObjVal(rest))

	case OP_UNWRAP_OR_RETURN:
		// Unwrap Option/Result or early return
		val := vm.pop()
		if val.IsObj() {
			if data, ok := val.AsObject().(*evaluator.DataInstance); ok {
				// Check by constructor name (more reliable than TypeName which may be empty)
				switch data.Name {
				case config.SomeCtorName: // Some - unwrap
					if len(data.Fields) > 0 {
						vm.push(ObjectToValue(data.Fields[0]))
					} else {
						vm.push(NilVal())
					}
				case config.NoneCtorName: // None - early return
					vm.push(val)
					return errEarlyReturn
				case config.OkCtorName: // Ok - unwrap
					if len(data.Fields) > 0 {
						vm.push(ObjectToValue(data.Fields[0]))
					} else {
						vm.push(NilVal())
					}
				case config.FailCtorName: // Fail - early return
					vm.push(val)
					return errEarlyReturn
				default:
					return fmt.Errorf("? operator requires Option or Result, got %s(%s)", data.TypeName, data.Name)
				}
			} else {
				// Handle nil gracefully (e.g. popping from empty stack or invalid object)
				if val.AsObject() == nil {
					return fmt.Errorf("? operator called on nil object")
				}
				return fmt.Errorf("? operator requires Option or Result, got %s", val.RuntimeType().String())
			}
		} else {
			if val.AsObject() == nil {
				return fmt.Errorf("? operator called on nil object")
			}
			return fmt.Errorf("? operator requires Option or Result, got %s", val.RuntimeType().String())
		}

	case OP_UNWRAP_OR_PANIC:
		// Unwrap Option/Result or panic (for |>> operator)
		// Ok(v) -> v, Some(v) -> v, Fail(e) -> error, None -> error, other -> pass through
		val := vm.pop()
		if val.IsObj() {
			if data, ok := val.AsObject().(*evaluator.DataInstance); ok {
				switch data.Name {
				case config.OkCtorName:
					if len(data.Fields) > 0 {
						vm.push(ObjectToValue(data.Fields[0]))
					} else {
						vm.push(NilVal())
					}
				case config.FailCtorName:
					if len(data.Fields) > 0 {
						return fmt.Errorf("|>> unwrap failed: Fail(%s)", data.Fields[0].Inspect())
					}
					return fmt.Errorf("|>> unwrap failed: Fail")
				case config.SomeCtorName:
					if len(data.Fields) > 0 {
						vm.push(ObjectToValue(data.Fields[0]))
					} else {
						vm.push(NilVal())
					}
				case config.NoneCtorName:
					return fmt.Errorf("|>> unwrap failed: None")
				default:
					// Not Option/Result — pass through
					vm.push(val)
				}
			} else {
				// Not a DataInstance — pass through
				vm.push(val)
			}
		} else {
			// Primitive value — pass through
			vm.push(val)
		}

	case OP_GET_TUPLE_ELEM:
		// Get tuple element by index (also supports List for variadic args)
		idxObj := vm.pop()
		val := vm.pop().AsObject()

		if !idxObj.IsInt() {
			// Check if idxObj corresponds to nil/empty object which may cause RuntimeType to panic
			if idxObj.AsObject() == nil {
				return fmt.Errorf("tuple/list index must be Integer, got nil object")
			}
			return fmt.Errorf("tuple/list index must be Integer, got %s", idxObj.RuntimeType().String())
		}
		i := int(idxObj.AsInt())

		switch v := val.(type) {
		case *evaluator.Tuple:
			// Support negative indexing
			if i < 0 {
				i = len(v.Elements) + i
			}
			if i >= len(v.Elements) || i < 0 {
				return fmt.Errorf("tuple index %d out of bounds (len=%d)", i, len(v.Elements))
			}
			vm.push(ObjectToValue(v.Elements[i]))
		case *evaluator.List:
			// Support negative indexing
			listLen := v.Len()
			if i < 0 {
				i = listLen + i
			}
			if i >= listLen || i < 0 {
				return fmt.Errorf("list index %d out of bounds (len=%d)", i, listLen)
			}

			elem := v.Get(i)
			if elem == nil {
				return fmt.Errorf("list index %d out of bounds (nil element)", i)
			}
			vm.push(ObjectToValue(elem))
		default:
			if val == nil {
				return fmt.Errorf("expected Tuple or List, got nil")
			}
			return fmt.Errorf("expected Tuple or List, got %s", val.Type())
		}

	case OP_FORMATTER:
		// Create format string function closure (variadic)
		// Args: [constant_index] (format string)
		// Returns: [closure] - a variadic function that calls format
		fmtStr := vm.readConstant().Inspect()
		if len(fmtStr) >= 2 && fmtStr[0] == '"' && fmtStr[len(fmtStr)-1] == '"' {
			fmtStr = fmtStr[1 : len(fmtStr)-1]
		}

		// For short form (no % in string), prepend % to make it a valid format specifier
		// For full form (contains %), use as-is
		if !strings.Contains(fmtStr, "%") {
			// Only prepend % if it results in a valid format string
			if _, err := evaluator.CountFormatVerbs("%" + fmtStr); err == nil {
				fmtStr = "%" + fmtStr
			}
		}

		// Create a variadic Native Function that captures the format string
		formatter := &evaluator.Builtin{
			Name: "formatter",
			TypeInfo: typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.TVar{Name: "a"}},
				ReturnType: typesystem.TCon{Name: "String"},
				IsVariadic: true,
			},
			Fn: func(e *evaluator.Evaluator, args ...evaluator.Object) evaluator.Object {
				// Call internal format with all arguments
				format, ok := evaluator.Builtins["format"]
				if !ok {
					return &evaluator.Error{Message: "internal error: format not found"}
				}
				// Prepend format string to args
				allArgs := make([]evaluator.Object, 0, len(args)+1)
				allArgs = append(allArgs, evaluator.StringToList(fmtStr))
				allArgs = append(allArgs, args...)
				return format.Fn(e, allArgs...)
			},
		}
		vm.push(ObjVal(formatter))

	case OP_HALT:
		return nil // Should be handled by step()

	case OP_AUTO_CALL:
		val := vm.peek(0)
		// If it's a nullary class method and we have type context, auto-call it
		if val.IsObj() {
			if cm, ok := val.AsObject().(*evaluator.ClassMethod); ok && cm.Arity == 0 {
				if vm.getTypeContext() != "" {
					// Try to call it.
					// vm.callClassMethod handles stack manipulation (pops cm).
					err := vm.callClassMethod(cm, 0)
					if err != nil {
						return err
					}
				}
			}
		}

	default:
		return fmt.Errorf("unknown opcode: %d", op)
	}

	return nil
}

// Run executes the bytecode and returns the result
