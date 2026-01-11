package vm

import (
	"fmt"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/typesystem"
)

// callValue dispatches call based on callee type
func (vm *VM) callValue(callee Value, argCount int) error {
	calleeObj := callee.AsObject() // Unbox for type switching (slow path anyway)

	switch fn := calleeObj.(type) {
	case *ObjClosure:
		return vm.callClosure(fn, argCount)
	case *CompiledFunction:
		closure := &ObjClosure{Function: fn, Upvalues: nil}
		return vm.callClosure(closure, argCount)
	case *evaluator.Builtin:
		return vm.callBuiltin(fn, argCount)
	case *evaluator.Constructor:
		return vm.callConstructor(fn, argCount)
	case *evaluator.TypeObject:
		return vm.callTypeObject(fn, argCount)
	case *evaluator.OperatorFunction:
		return vm.callOperatorFunction(fn, argCount)
	case *VMComposedFunction:
		return vm.callVMComposedFunction(fn, argCount)
	case *evaluator.ClassMethod:
		return vm.callClassMethod(fn, argCount)
	case *evaluator.BoundMethod:
		return vm.callBoundMethod(fn, argCount)
	case *evaluator.PartialApplication:
		return vm.callPartialApplication(fn, argCount)
	case *BuiltinClosure:
		return vm.callBuiltinClosure(fn, argCount)
	default:
		return vm.runtimeError("can only call functions, got %s", callee.RuntimeType().String())
	}
}

// callBuiltinClosure calls a native Go function wrapped as BuiltinClosure
func (vm *VM) callBuiltinClosure(bc *BuiltinClosure, argCount int) error {
	// Clear nextImplicitContext if set, as builtins don't consume it
	if vm.nextImplicitContext != "" {
		vm.nextImplicitContext = ""
	}

	args := make([]evaluator.Object, argCount)
	for i := argCount - 1; i >= 0; i-- {
		args[i] = vm.pop().AsObject() // Unbox arguments for builtin
	}
	vm.pop() // pop the BuiltinClosure itself (Value wrapped Object)

	result := bc.Fn(args)
	if err, ok := result.(*evaluator.Error); ok {
		return fmt.Errorf("%s", err.Message)
	}
	vm.push(ObjectToValue(result))
	return nil
}

// executeDefaultChunk executes a compiled default expression and returns the result
func (vm *VM) executeDefaultChunk(chunk *Chunk, parentClosure *ObjClosure) (Value, error) {
	tempFn := &CompiledFunction{
		Arity:      0,
		Chunk:      chunk,
		Name:       "<default>",
		LocalCount: 0,
	}
	tempClosure := &ObjClosure{
		Function: tempFn,
		Upvalues: parentClosure.Upvalues,
	}

	// savedFrame := vm.frame
	savedFrameCount := vm.frameCount
	savedSp := vm.sp

	// Grow frames array if needed for default evaluation
	if vm.frameCount >= len(vm.frames) {
		// Use consistent growth strategy from vm.go
		growBy := FrameGrowthIncrement
		if len(vm.frames) > growBy {
			growBy = len(vm.frames)
		}
		newFrames := make([]CallFrame, len(vm.frames)+growBy)
		copy(newFrames, vm.frames[:vm.frameCount])
		vm.frames = newFrames
	}

	vm.frameCount++
	vm.frame = &vm.frames[vm.frameCount-1]
	vm.frame.closure = tempClosure
	vm.frame.ip = 0
	vm.frame.base = vm.sp
	vm.frame.chunk = chunk

	// Inherit implicit context
	if savedFrameCount > 0 {
		vm.frame.ImplicitTypeContext = vm.frames[savedFrameCount-1].ImplicitTypeContext
	} else {
		vm.frame.ImplicitTypeContext = ""
	}
	vm.frame.ExplicitTypeContextDepth = len(vm.typeContextStack)

	for {
		result, done, err := vm.step()
		if err != nil {
			// Restore frame safely
			if savedFrameCount > 0 {
				vm.frame = &vm.frames[savedFrameCount-1]
			} else {
				vm.frame = nil
			}
			vm.frameCount = savedFrameCount
			vm.sp = savedSp
			return NilVal(), err
		}
		if done {
			// Restore frame safely
			if savedFrameCount > 0 {
				vm.frame = &vm.frames[savedFrameCount-1]
			} else {
				vm.frame = nil
			}
			vm.frameCount = savedFrameCount
			vm.sp = savedSp
			return result, nil
		}
	}
}

// callClosure sets up a new call frame for a closure
func (vm *VM) callClosure(closure *ObjClosure, argCount int) error {
	fn := closure.Function

	if fn.IsVariadic {
		if argCount < fn.Arity {
			return vm.runtimeError("expected at least %d arguments but got %d", fn.Arity, argCount)
		}
		variadicCount := argCount - fn.Arity
		variadicArgs := make([]evaluator.Object, variadicCount)
		for i := 0; i < variadicCount; i++ {
			variadicArgs[i] = vm.stack[vm.sp-variadicCount+i].AsObject()
		}
		vm.sp -= variadicCount
		vm.push(ObjVal(evaluator.NewList(variadicArgs)))
		argCount = fn.Arity + 1
	} else {
		if argCount < fn.RequiredArity {
			// Calling with 0 arguments when expecting some is an error
			if argCount == 0 && fn.RequiredArity > 0 {
				fnName := fn.Name
				if fnName == "" {
					fnName = "<anonymous>"
				}
				return vm.runtimeErrorWithCallee(fnName, "wrong number of arguments: expected %d, got 0", fn.RequiredArity)
			}
			// Partial application with some args
			args := make([]evaluator.Object, argCount)
			for i := argCount - 1; i >= 0; i-- {
				args[i] = vm.pop().AsObject()
			}
			vm.pop()
			partial := &evaluator.PartialApplication{
				VMClosure:   closure,
				AppliedArgs: args,
			}
			vm.push(ObjVal(partial))
			return nil
		}
		if argCount < fn.Arity && len(fn.Defaults) > 0 {
			for i := argCount; i < fn.Arity; i++ {
				defaultIdx := i - fn.RequiredArity
				if defaultIdx >= 0 && defaultIdx < len(fn.Defaults) {
					constIdx := fn.Defaults[defaultIdx]
					if constIdx >= 0 {
						vm.push(ObjectToValue(closure.Function.Chunk.Constants[constIdx]))
						argCount++
					} else if fn.DefaultChunks != nil && defaultIdx < len(fn.DefaultChunks) && fn.DefaultChunks[defaultIdx] != nil {
						defaultChunk := fn.DefaultChunks[defaultIdx]
						defaultVal, err := vm.executeDefaultChunk(defaultChunk, closure)
						if err != nil {
							return err
						}
						vm.push(defaultVal)
						argCount++
					}
				}
			}
		}
		if argCount > fn.Arity && !fn.IsVariadic {
			return vm.runtimeError("expected %d arguments but got %d", fn.Arity, argCount)
		}
	}

	// Grow frames array if needed
	if vm.frameCount >= len(vm.frames) {
		// Use consistent growth strategy from vm.go
		growBy := FrameGrowthIncrement
		if len(vm.frames) > growBy {
			growBy = len(vm.frames)
		}
		newFrames := make([]CallFrame, len(vm.frames)+growBy)
		copy(newFrames, vm.frames[:vm.frameCount])
		vm.frames = newFrames
	}

	for i := 0; i < argCount; i++ {
		vm.stack[vm.sp-argCount-1+i] = vm.stack[vm.sp-argCount+i]
	}
	vm.sp--

	frame := &vm.frames[vm.frameCount]
	frame.closure = closure
	frame.chunk = fn.Chunk
	frame.ip = 0
	frame.base = vm.sp - argCount

	// Inherit implicit context from current frame (caller)
	if vm.frame != nil {
		frame.ImplicitTypeContext = vm.frame.ImplicitTypeContext
	} else {
		frame.ImplicitTypeContext = ""
	}
	// If nextImplicitContext is set (e.g. by OP_TRAIT_OP), override/use it
	if vm.nextImplicitContext != "" {
		frame.ImplicitTypeContext = vm.nextImplicitContext
		vm.nextImplicitContext = ""
	}

	frame.ExplicitTypeContextDepth = len(vm.typeContextStack)

	vm.frameCount++
	vm.frame = frame

	return nil
}

// tailCallValue dispatches tail call based on callee type
func (vm *VM) tailCallValue(callee Value, argCount int) error {
	calleeObj := callee.AsObject()

	switch fn := calleeObj.(type) {
	case *ObjClosure:
		return vm.tailCallClosure(fn, argCount)
	case *CompiledFunction:
		closure := &ObjClosure{Function: fn, Upvalues: nil}
		return vm.tailCallClosure(closure, argCount)
	default:
		return vm.callValue(callee, argCount)
	}
}

// tailCallClosure performs tail call optimization by reusing the current frame
func (vm *VM) tailCallClosure(closure *ObjClosure, argCount int) error {
	fn := closure.Function

	// Handle partial application - can't do TCO, fall back to regular call
	if argCount < fn.RequiredArity {
		// Can't TCO partial application, use regular call
		return vm.callClosure(closure, argCount)
	}

	// Consume nextImplicitContext if set (similar to callClosure)
	// Even though we are reusing the frame, we might need to update the context
	// if this tail call was triggered by a trait dispatch.
	if vm.nextImplicitContext != "" {
		vm.frame.ImplicitTypeContext = vm.nextImplicitContext
		vm.nextImplicitContext = ""
	}

	if argCount > fn.Arity && !fn.IsVariadic {
		return vm.runtimeError("expected %d arguments but got %d", fn.Arity, argCount)
	}

	vm.closeUpvalues(vm.frame.base)

	// Handle variadic arguments packing
	if fn.IsVariadic {
		if argCount < fn.Arity {
			return vm.runtimeError("expected at least %d arguments but got %d", fn.Arity, argCount)
		}
		variadicCount := argCount - fn.Arity
		variadicArgs := make([]evaluator.Object, variadicCount)
		for i := 0; i < variadicCount; i++ {
			variadicArgs[i] = vm.stack[vm.sp-variadicCount+i].AsObject()
		}

		// Copy fixed arguments
		for i := 0; i < fn.Arity; i++ {
			vm.stack[vm.frame.base+i] = vm.stack[vm.sp-argCount+i]
		}

		// Pack variadic arguments into List
		vm.stack[vm.frame.base+fn.Arity] = ObjVal(evaluator.NewList(variadicArgs))
		vm.sp = vm.frame.base + fn.Arity + 1
	} else {
		// Non-variadic: copy all arguments
		for i := 0; i < argCount; i++ {
			vm.stack[vm.frame.base+i] = vm.stack[vm.sp-argCount+i]
		}
		vm.sp = vm.frame.base + argCount
	}

	vm.frame.closure = closure
	vm.frame.chunk = fn.Chunk
	vm.frame.ip = 0

	return nil
}

// callBuiltin calls a built-in function
func (vm *VM) callBuiltin(builtin *evaluator.Builtin, argCount int) error {
	// Clear nextImplicitContext if set, as builtins don't consume it
	// and we don't want it to leak to subsequent calls (e.g. adjacent testRun calls)
	if vm.nextImplicitContext != "" {
		vm.nextImplicitContext = ""
	}
	// Also ensure we don't leak context OUT of the builtin (e.g. from inner calls)
	defer func() {
		vm.nextImplicitContext = ""
	}()

	args := make([]evaluator.Object, argCount)
	for i := 0; i < argCount; i++ {
		args[i] = vm.stack[vm.sp-argCount+i].AsObject()
	}

	// Check for partial application via TypeInfo
	if fnType, ok := builtin.TypeInfo.(typesystem.TFunc); ok && !fnType.IsVariadic {
		totalParams := len(fnType.Params)
		requiredParams := totalParams - fnType.DefaultCount

		// Only create partial application if we have FEWER args than required
		// If we have MORE, let the builtin function handle it (it might error, which is correct)
		if argCount < requiredParams && argCount > 0 {
			// Partial application for builtin
			partial := &evaluator.PartialApplication{
				Builtin:         builtin,
				AppliedArgs:     args,
				RemainingParams: requiredParams - argCount,
			}
			vm.sp -= argCount + 1
			vm.push(ObjVal(partial))
			return nil
		}
	}

	// Fill in default arguments if needed
	if len(builtin.DefaultArgs) > 0 {
		// Get total params from TypeInfo
		if fnType, ok := builtin.TypeInfo.(typesystem.TFunc); ok {
			totalParams := len(fnType.Params)
			// Only apply defaults if we have fewer arguments than total params
			// AND we are not in an "excess arguments" situation
			if argCount < totalParams {
				// Need to fill in some defaults
				fullArgs := make([]evaluator.Object, 0, totalParams)
				fullArgs = append(fullArgs, args...)
				// Defaults cover the last len(DefaultArgs) parameters
				defaultStart := totalParams - len(builtin.DefaultArgs)
				for i := argCount; i < totalParams; i++ {
					defaultIdx := i - defaultStart
					if defaultIdx >= 0 && defaultIdx < len(builtin.DefaultArgs) {
						fullArgs = append(fullArgs, builtin.DefaultArgs[defaultIdx])
					}
				}
				args = fullArgs
			}
		}
	}

	eval := vm.getEvaluator()
	// Populate call stack for debug/trace builtins
	if vm.frame != nil && vm.frame.chunk != nil && vm.frame.ip > 0 {
		ip := vm.frame.ip - 1
		line := 0
		if ip < len(vm.frame.chunk.Lines) {
			line = vm.frame.chunk.Lines[ip]
		}
		file := vm.frame.chunk.File
		if file == "" {
			file = vm.currentFile
		}
		eval.CallStack = []evaluator.CallFrame{{File: file, Line: line}}
	}
	result := builtin.Fn(eval, args...)

	if result != nil && result.Type() == evaluator.ERROR_OBJ {
		return vm.runtimeError("%s", result.Inspect())
	}

	vm.sp -= argCount + 1
	if result == nil {
		vm.push(NilVal())
	} else {
		vm.push(ObjectToValue(result))
	}

	return nil
}

// callConstructor handles ADT constructor calls
func (vm *VM) callConstructor(ctor *evaluator.Constructor, argCount int) error {
	// Extract TypeArgs from leading TypeObject arguments (Reified Generics)
	var typeArgs []typesystem.Type
	var valueArgs []evaluator.Object
	var actualArgCount int

	// Check leading arguments for TypeObjects
	for i := 0; i < argCount; i++ {
		arg := vm.stack[vm.sp-argCount+i].AsObject()
		if typeObj, ok := arg.(*evaluator.TypeObject); ok {
			typeArgs = append(typeArgs, typeObj.TypeVal)
		} else {
			valueArgs = append(valueArgs, arg)
			actualArgCount++
		}
	}

	if actualArgCount > ctor.Arity {
		return vm.runtimeError("constructor %s expects %d arguments, got %d", ctor.Name, ctor.Arity, actualArgCount)
	}

	// Partial application: fewer args than arity
	if actualArgCount < ctor.Arity {
		partial := &evaluator.PartialApplication{
			Constructor:     ctor,
			AppliedArgs:     valueArgs,
			RemainingParams: ctor.Arity - actualArgCount,
		}
		vm.sp -= argCount + 1
		vm.push(ObjVal(partial))
		return nil
	}

	result := &evaluator.DataInstance{
		Name:     ctor.Name,
		Fields:   valueArgs,
		TypeName: ctor.TypeName,
		TypeArgs: typeArgs,
	}

	vm.sp -= argCount + 1
	vm.push(ObjVal(result))

	return nil
}

// callTypeObject handles type application like List(Int) or value construction like Sum({x:1})
func (vm *VM) callTypeObject(typeObj *evaluator.TypeObject, argCount int) error {
	// Check for value construction/casting
	isConstruction := false
	if argCount > 0 {
		firstArg := vm.stack[vm.sp-argCount].AsObject()
		if _, ok := firstArg.(*evaluator.TypeObject); !ok {
			isConstruction = true
		}
	}

	if isConstruction {
		if argCount != 1 {
			return vm.runtimeError("type constructor expects 1 argument, got %d", argCount)
		}
		val := vm.stack[vm.sp-1].AsObject()

		if rec, ok := val.(*evaluator.RecordInstance); ok {
			newRec := &evaluator.RecordInstance{
				Fields:   rec.Fields,
				TypeName: evaluator.ExtractTypeConstructorName(typeObj.TypeVal),
			}
			vm.sp -= 2 // Pop func + arg
			vm.push(ObjectToValue(newRec))
			return nil
		}

		// Just return the value if not record (cast/alias)
		vm.sp -= 2
		vm.push(ObjectToValue(val))
		return nil
	}

	typeArgs := make([]typesystem.Type, argCount)
	for i := 0; i < argCount; i++ {
		arg := vm.stack[vm.sp-argCount+i].AsObject()
		tArg, ok := arg.(*evaluator.TypeObject)
		if !ok {
			return vm.runtimeError("type application expects types as arguments, got %s", arg.Type())
		}
		typeArgs[i] = tArg.TypeVal
	}

	result := &evaluator.TypeObject{
		TypeVal: typesystem.TApp{Constructor: typeObj.TypeVal, Args: typeArgs},
	}

	vm.sp -= argCount + 1
	vm.push(ObjVal(result))

	return nil
}

// callOperatorFunction calls an operator as a binary function
func (vm *VM) callOperatorFunction(opFn *evaluator.OperatorFunction, argCount int) error {
	if argCount != 2 {
		return vm.runtimeError("operator %s expects 2 arguments, got %d", opFn.Operator, argCount)
	}

	right := vm.pop()
	left := vm.pop()
	vm.pop()

	typeName := vm.getTypeName(left)
	if closure := vm.LookupOperator(typeName, opFn.Operator); closure != nil {
		vm.push(ObjVal(closure))
		vm.push(left)
		vm.push(right)
		return vm.callClosure(closure, 2)
	}

	return vm.callBuiltinOperator(opFn.Operator, left, right)
}

// callBuiltinOperator handles builtin operators for primitive types
func (vm *VM) callBuiltinOperator(op string, left, right Value) error {
	switch op {
	case "+":
		return vm.binaryOpWithArgs(OP_ADD, left, right)
	case "-":
		return vm.binaryOpWithArgs(OP_SUB, left, right)
	case "*":
		return vm.binaryOpWithArgs(OP_MUL, left, right)
	case "%":
		return vm.binaryOpWithArgs(OP_MOD, left, right)
	case "**":
		return vm.binaryOpWithArgs(OP_POW, left, right)
	case "/":
		return vm.binaryOpWithArgs(OP_DIV, left, right)
	case "<":
		return vm.comparisonOpWithArgs(OP_LT, left, right)
	case "<=":
		return vm.comparisonOpWithArgs(OP_LE, left, right)
	case ">":
		return vm.comparisonOpWithArgs(OP_GT, left, right)
	case ">=":
		return vm.comparisonOpWithArgs(OP_GE, left, right)
	case "&&":
		// Logical AND - both must be truthy
		leftBool := vm.isTruthy(left)
		rightBool := vm.isTruthy(right)
		vm.push(BoolVal(leftBool && rightBool))
		return nil
	case "||":
		// Logical OR - at least one must be truthy
		leftBool := vm.isTruthy(left)
		rightBool := vm.isTruthy(right)
		vm.push(BoolVal(leftBool || rightBool))
		return nil
	case "++":
		// Concatenation
		vm.push(left)
		vm.push(right)
		return vm.concatOp()
	case "::":
		// Cons
		vm.push(left)
		vm.push(right)
		return vm.consOp()
	case "==":
		// Equality
		vm.push(BoolVal(left.Equals(right))) // Use Value.Equals for fast path
		return nil
	case "!=":
		// Inequality
		vm.push(BoolVal(!left.Equals(right)))
		return nil
	case "&":
		// Bitwise AND
		vm.push(left)
		vm.push(right)
		return vm.bitwiseOp(OP_BAND)
	case "|":
		// Bitwise OR
		vm.push(left)
		vm.push(right)
		return vm.bitwiseOp(OP_BOR)
	case "^":
		// Bitwise XOR
		vm.push(left)
		vm.push(right)
		return vm.bitwiseOp(OP_BXOR)
	case "<<":
		// Left shift
		vm.push(left)
		vm.push(right)
		return vm.bitwiseOp(OP_LSHIFT)
	case ">>":
		// Right shift
		vm.push(left)
		vm.push(right)
		return vm.bitwiseOp(OP_RSHIFT)
	default:
		return fmt.Errorf("no implementation of operator %s for type %s", op, vm.getTypeName(left))
	}
}

func (vm *VM) binaryOpWithArgs(op Opcode, a, b Value) error {
	vm.push(a)
	vm.push(b)
	return vm.binaryOp(op)
}

func (vm *VM) comparisonOpWithArgs(op Opcode, a, b Value) error {
	vm.push(a)
	vm.push(b)
	return vm.comparisonOp(op)
}

// callPartialApplication calls a partially applied function with additional args
func (vm *VM) callPartialApplication(pa *evaluator.PartialApplication, argCount int) error {
	newArgs := make([]evaluator.Object, argCount)
	for i := argCount - 1; i >= 0; i-- {
		newArgs[i] = vm.pop().AsObject()
	}
	vm.pop()

	allArgs := make([]evaluator.Object, len(pa.AppliedArgs)+argCount)
	copy(allArgs, pa.AppliedArgs)
	copy(allArgs[len(pa.AppliedArgs):], newArgs)

	var fn evaluator.Object
	if pa.VMClosure != nil {
		fn = pa.VMClosure
	} else if pa.Function != nil {
		fn = pa.Function
	} else if pa.Builtin != nil {
		fn = pa.Builtin
	} else if pa.Constructor != nil {
		fn = pa.Constructor
	} else {
		return vm.runtimeError("invalid partial application")
	}

	vm.push(ObjectToValue(fn))
	for _, arg := range allArgs {
		vm.push(ObjectToValue(arg))
	}
	return vm.callValue(ObjectToValue(fn), len(allArgs))
}

// callBoundMethod calls a method bound to a receiver
func (vm *VM) callBoundMethod(bm *evaluator.BoundMethod, argCount int) error {
	args := make([]evaluator.Object, argCount)
	for i := argCount - 1; i >= 0; i-- {
		args[i] = vm.pop().AsObject()
	}
	vm.pop()

	allArgs := make([]evaluator.Object, argCount+1)
	allArgs[0] = bm.Receiver
	copy(allArgs[1:], args)

	vm.push(ObjVal(bm.Function))
	for _, arg := range allArgs {
		vm.push(ObjectToValue(arg))
	}
	return vm.callValue(ObjVal(bm.Function), len(allArgs))
}

// callClassMethod calls a trait method natively
func (vm *VM) callClassMethod(cm *evaluator.ClassMethod, argCount int) error {
	// Check for hidden type hint argument (injected by Compiler/VM)
	// pure(val, TypeHint) -> pure(val) with context TypeHint
	var explicitTypeHint typesystem.Type

	if cm.Arity >= 0 && argCount == cm.Arity+1 {
		lastArg := vm.peek(0)
		if typeObj, ok := lastArg.AsObject().(*evaluator.TypeObject); ok {
			explicitTypeHint = typeObj.TypeVal
			vm.pop() // Remove hint from stack
			argCount--
		} else if list, ok := lastArg.AsObject().(*evaluator.List); ok {
			// Legacy string hint logic
			if evaluator.IsStringList(list) {
				str := evaluator.ListToString(list)
				if str != "" || list.Len() == 0 {
					explicitTypeHint = typesystem.TCon{Name: str}
					vm.pop()
					argCount--
				}
			}
		}
	}

	if argCount < 1 && cm.Arity > 0 {
		return vm.runtimeError("%s expects at least 1 argument", cm.Name)
	}

	var method evaluator.Object
	var resolvedType string

	// For nullary methods, we rely on type context or defaults
	if argCount == 0 {
		ctx := vm.getTypeContext()

		// Prioritize explicit hint if available
		if explicitTypeHint != nil {
			typeName := evaluator.ExtractTypeConstructorName(explicitTypeHint)
			method = vm.LookupTraitMethodAny(cm.ClassName, typeName, cm.Name)
			if method != nil {
				resolvedType = typeName
			}
		}

		if method == nil && ctx != "" {
			method = vm.LookupTraitMethodAny(cm.ClassName, ctx, cm.Name)
			if method != nil {
				resolvedType = ctx
			}
		}

		// Check defaults
		if method == nil {
			defaultKey := cm.ClassName + "." + cm.Name
			if defaultFn, ok := vm.traitDefaults[defaultKey]; ok {
				// For defaults, we need a type name to register against.
				// Use typeContext if available.
				if ctx != "" {
					closure, err := vm.compileTraitDefault(defaultFn, cm.ClassName, ctx)
					if err == nil && closure != nil {
						vm.RegisterTraitMethod(cm.ClassName, ctx, cm.Name, closure)
						method = closure
						resolvedType = ctx
					}
				}
			}
		}

		if method != nil {
			// Found implementation (vm closure or builtin)
			// Need to call it.
			// Stack has [ClassMethod]. We need to replace it with [Method].
			vm.pop()                       // pop ClassMethod
			vm.push(ObjectToValue(method)) // push Method (boxed or value?)
			// We should probably convert method to Value if it's an object
			// vm.push(ObjectToValue(method))

			// Push witness on evaluator
			if explicitTypeHint != nil {
				vm.getEvaluator().PushWitness(map[string][]typesystem.Type{"Applicative": {explicitTypeHint}})
				// Witness will be popped when the builtin call completes
				// For VM, we rely on the builtin to properly manage witness stack
			}

			if resolvedType != "" {
				vm.nextImplicitContext = resolvedType
			}
			result := vm.callValue(ObjectToValue(method), 0)
			// Pop witness after call completes
			if explicitTypeHint != nil {
				vm.getEvaluator().PopWitness()
			}
			return result
		}

		// Fallback to evaluator if not found in VM registry
		vm.pop()
		eval := vm.getEvaluator()
		// Important: Evaluator doesn't share VM's typeContext automatically for ApplyFunction
		// But Evaluator.AnnotatedExpression handles it by setting e.CurrentCallNode
		// However, here we are inside VM call.
		result := eval.ApplyFunction(cm, []evaluator.Object{})
		if err, ok := result.(*evaluator.Error); ok {
			return fmt.Errorf("%s", err.Message)
		}
		vm.push(ObjectToValue(result))
		return nil
	}

	// First try to find method by argument types
	for i := 0; i < argCount; i++ {
		arg := vm.peek(argCount - 1 - i)
		typeName := vm.getTypeName(arg)

		// Fallback to standard lookup (legacy single arg)
		method = vm.LookupTraitMethodAny(cm.ClassName, typeName, cm.Name)
		if method != nil {
			resolvedType = typeName
			break
		}
	}

	// Try Fuzzy MPTC lookup using all available arguments + context
	// This is now the preferred path for complex dispatch
	if argCount > 0 {
		// Collect arguments
		args := make([]evaluator.Object, argCount)
		for i := 0; i < argCount; i++ {
			args[i] = vm.peek(argCount - 1 - i).AsObject()
		}

		// Use context if available to disambiguate
		ctx := vm.getTypeContext()
		fuzzyMethod := vm.LookupTraitMethodFuzzy(cm.ClassName, cm.Name, args, ctx)

		// If fuzzy lookup found something, PREFER it over single-arg lookup
		// especially if we found nothing before, or if the fuzzy match is "better" (handled by score)
		if fuzzyMethod != nil {
			method = fuzzyMethod
			// resolvedType? Fuzzy lookup abstracts the key.
			// We might need it for nextImplicitContext, but usually MPTC methods don't rely on it for inner calls
			// as much as single-dispatch.
		}
	}

	// If not found, try type context (from type annotation)
	ctx := vm.getTypeContext()

	// If still not found, check trait defaults FIRST for arg type (before using context)
	// This ensures that default implementations are used for types without explicit override
	// We do this BEFORE "Context Dispatch" because direct arg type is more specific than context.
	if method == nil && argCount > 0 {
		argTypeName := vm.getTypeName(vm.peek(argCount - 1))
		defaultKey := cm.ClassName + "." + cm.Name
		if defaultFn, ok := vm.traitDefaults[defaultKey]; ok && argTypeName != "" {
			// JIT compile the default method for the argument type
			closure, err := vm.compileTraitDefault(defaultFn, cm.ClassName, argTypeName)
			if err == nil && closure != nil {
				// Register for future use
				vm.RegisterTraitMethod(cm.ClassName, argTypeName, cm.Name, closure)
				method = closure
				resolvedType = argTypeName
			}
		}
	}

	// 1. Try Context Dispatch (Dynamic)
	// This prioritizes Implicit Context (from >>=) which getTypeContext returns first.
	// This allows dynamic dispatch to override static hints (like from TypeMap/inference) when necessary.
	// But ONLY if we haven't found a method yet (e.g. from defaults above).
	if method == nil && ctx != "" {
		method = vm.LookupTraitMethodAny(cm.ClassName, ctx, cm.Name)
		if method != nil {
			resolvedType = ctx
		}
	}

	// 2. Prioritize explicit hint if available (Static)
	if method == nil && explicitTypeHint != nil {
		typeName := evaluator.ExtractTypeConstructorName(explicitTypeHint)
		method = vm.LookupTraitMethodAny(cm.ClassName, typeName, cm.Name)
		if method != nil {
			resolvedType = typeName
		}
	}

	// Use context for nullary methods or if no default was found for arg type
	if method == nil && ctx != "" {
		method = vm.LookupTraitMethodAny(cm.ClassName, ctx, cm.Name)
		if method != nil {
			resolvedType = ctx
		}
	}

	// Last resort: check trait defaults with context type
	if method == nil && ctx != "" {
		defaultKey := cm.ClassName + "." + cm.Name
		if defaultFn, ok := vm.traitDefaults[defaultKey]; ok {
			// JIT compile the default method for the context type
			closure, err := vm.compileTraitDefault(defaultFn, cm.ClassName, ctx)
			if err == nil && closure != nil {
				// Register for future use
				vm.RegisterTraitMethod(cm.ClassName, ctx, cm.Name, closure)
				method = closure
				resolvedType = ctx
			}
		}
	}

	if method == nil {
		args := make([]evaluator.Object, argCount)
		for i := 0; i < argCount; i++ {
			args[i] = vm.stack[vm.sp-argCount+i].AsObject()
		}
		vm.sp -= argCount
		vm.pop()

		eval := vm.getEvaluator()
		// Sync TypeContextStack from VM to Evaluator
		// Copy slice to avoid sharing issues
		if len(vm.typeContextStack) > 0 {
			eval.TypeContextStack = make([]string, len(vm.typeContextStack))
			copy(eval.TypeContextStack, vm.typeContextStack)
		} else {
			eval.TypeContextStack = nil
		}

		result := eval.ApplyFunction(cm, args)
		if err, ok := result.(*evaluator.Error); ok {
			return fmt.Errorf("%s", err.Message)
		}
		vm.push(ObjectToValue(result))
		return nil
	}

	vm.stack[vm.sp-argCount-1] = ObjectToValue(method)

	// Push witness on evaluator
	if explicitTypeHint != nil {
		vm.getEvaluator().PushWitness(map[string][]typesystem.Type{"Applicative": {explicitTypeHint}})
	}

	if resolvedType != "" {
		vm.nextImplicitContext = resolvedType
	}
	result := vm.callValue(ObjectToValue(method), argCount)
	// Pop witness after call completes
	if explicitTypeHint != nil {
		vm.getEvaluator().PopWitness()
	}
	return result
}

// callVMComposedFunction calls a composed function natively
func (vm *VM) callVMComposedFunction(fn *VMComposedFunction, argCount int) error {
	if argCount != 1 {
		return vm.runtimeError("composed function expects 1 argument, got %d", argCount)
	}

	arg := vm.pop() // Value
	vm.pop()        // Function object

	gResult, err := vm.callAndGetResult(ObjectToValue(fn.G), arg)
	if err != nil {
		return err
	}

	fResult, err := vm.callAndGetResult(ObjectToValue(fn.F), gResult)
	if err != nil {
		return err
	}

	vm.push(fResult)
	return nil
}

// callAndGetResult calls a function with one argument and returns the result
func (vm *VM) callAndGetResult(fn Value, arg Value) (Value, error) {
	initialFrameCount := vm.frameCount

	vm.push(fn)
	vm.push(arg)

	if err := vm.callValue(fn, 1); err != nil {
		return NilVal(), err
	}

	for vm.frameCount > initialFrameCount {
		_, _, err := vm.step() // done flag from step() just means *some* frame returned, we rely on frameCount
		if err != nil {
			return NilVal(), err
		}
	}

	return vm.pop(), nil
}

// callNoArgs calls a function with no arguments
func (vm *VM) callNoArgs(fn Value) (Value, error) {
	initialFrameCount := vm.frameCount

	vm.push(fn)

	if err := vm.callValue(fn, 0); err != nil {
		return NilVal(), err
	}

	for vm.frameCount > initialFrameCount {
		_, _, err := vm.step() // done flag from step() just means *some* frame returned, we rely on frameCount
		if err != nil {
			return NilVal(), err
		}
	}

	return vm.pop(), nil
}
