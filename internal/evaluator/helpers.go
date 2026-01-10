package evaluator

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/typesystem"
	"strings"
)

// extractListElementType extracts element type name from List<T> annotation
// Returns empty string if not a List type or no type argument
func extractListElementType(t ast.Type) string {
	nt, ok := t.(*ast.NamedType)
	if !ok {
		return ""
	}
	if nt.Name.Value != config.ListTypeName {
		return ""
	}
	if len(nt.Args) == 0 {
		return ""
	}
	// Get the element type name
	if elemType, ok := nt.Args[0].(*ast.NamedType); ok {
		return elemType.Name.Value
	}
	return ""
}

// Helper for method table
type MethodTable struct {
	Methods map[string]Object
}

func (mt *MethodTable) Type() ObjectType             { return "METHOD_TABLE" }
func (mt *MethodTable) Inspect() string              { return "MethodTable" }
func (mt *MethodTable) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "MethodTable"} }
func (mt *MethodTable) Hash() uint32 {
	// Not hashable/useful to hash method tables
	return 0
}

var (
	TRUE  = &Boolean{Value: true}
	FALSE = &Boolean{Value: false}
)

// getTypeName returns a human-readable type name for debugging
func getTypeName(obj Object) string {
	switch v := obj.(type) {
	case *Integer:
		return "Int"
	case *Float:
		return "Float"
	case *Boolean:
		return "Bool"
	case *Char:
		return "Char"
	case *Nil:
		return "Nil"
	case *List:
		if v.len() == 0 {
			return "List<_>"
		}
		// Try to infer element type from first element
		first := v.get(0)
		return "List<" + getTypeName(first) + ">"
	case *Map:
		return "Map<_, _>"
	case *Tuple:
		if len(v.Elements) == 0 {
			return "()"
		}
		types := make([]string, len(v.Elements))
		for i, el := range v.Elements {
			types[i] = getTypeName(el)
		}
		return "(" + strings.Join(types, ", ") + ")"
	case *Function:
		return "Function"
	case *Builtin:
		return "Builtin"
	case *RecordInstance:
		return v.TypeName
	case *DataInstance:
		return v.TypeName
	case *BigInt:
		return "BigInt"
	case *Rational:
		return "Rational"
	case *Bytes:
		return "Bytes"
	case *Bits:
		return "Bits"
	case *Uuid:
		return "Uuid"
	case *Logger:
		return "Logger"
	default:
		return string(obj.Type())
	}
}

func (e *Evaluator) nativeBoolToBooleanObject(input bool) *Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

func intPow(n, m int64) int64 {
	if m < 0 {
		return 0
	}
	if m == 0 {
		return 1
	}
	var result int64 = 1
	for i := int64(0); i < m; i++ {
		result *= n
	}
	return result
}

func newError(format string, a ...interface{}) *Error {
	return &Error{Message: fmt.Sprintf(format, a...)}
}

func newErrorWithLocation(line, column int, format string, a ...interface{}) *Error {
	return &Error{
		Message: fmt.Sprintf(format, a...),
		Line:    line,
		Column:  column,
	}
}

// PushCall adds a call frame to the stack
func (e *Evaluator) PushCall(name string, file string, line, column int) {
	e.CallStack = append(e.CallStack, CallFrame{
		Name:   name,
		File:   file,
		Line:   line,
		Column: column,
	})
}

// PopCall removes the top call frame
func (e *Evaluator) PopCall() {
	if len(e.CallStack) > 0 {
		e.CallStack = e.CallStack[:len(e.CallStack)-1]
	}
}

// newErrorWithStack creates an error with the current stack trace
func (e *Evaluator) newErrorWithStack(format string, a ...interface{}) *Error {
	err := &Error{Message: fmt.Sprintf(format, a...)}

	// Copy stack trace
	if len(e.CallStack) > 0 {
		err.StackTrace = make([]StackFrame, len(e.CallStack))
		for i, frame := range e.CallStack {
			err.StackTrace[i] = StackFrame{
				Name:   frame.Name,
				File:   frame.File,
				Line:   frame.Line,
				Column: frame.Column,
			}
		}
	}

	return err
}

func isError(obj Object) bool {
	if obj != nil {
		return obj.Type() == ERROR_OBJ
	}
	return false
}

func unwrapReturnValue(obj Object) Object {
	if returnValue, ok := obj.(*ReturnValue); ok {
		return returnValue.Value
	}
	return obj
}

// ExtractTypeConstructorName extracts the type constructor name from a type.
// e.g., Option<Int> → "Option", List<String> → "List", Result<Int, String> → "Result"
func ExtractTypeConstructorName(t typesystem.Type) string {
	switch typ := t.(type) {
	case typesystem.TCon:
		return typ.Name
	case typesystem.TApp:
		// For TApp, recursively get the constructor
		return ExtractTypeConstructorName(typ.Constructor)
	default:
		return ""
	}
}

// extractTypeNameFromAST extracts the type name from an AST type annotation.
// Used for dispatch when TypeMap doesn't contain the type.
func extractTypeNameFromAST(t ast.Type) string {
	switch typ := t.(type) {
	case *ast.NamedType:
		name := typ.Name.Value
		return name
	default:
		return ""
	}
}

func getRuntimeTypeName(obj Object) string {
	switch o := obj.(type) {
	case *Integer:
		return RUNTIME_TYPE_INT
	case *Float:
		return RUNTIME_TYPE_FLOAT
	case *BigInt:
		return RUNTIME_TYPE_BIGINT
	case *Rational:
		return RUNTIME_TYPE_RATIONAL
	case *Boolean:
		return RUNTIME_TYPE_BOOL
	case *Char:
		return RUNTIME_TYPE_CHAR
	case *Tuple:
		return RUNTIME_TYPE_TUPLE
	case *List: // And String
		// Check if it's a String (List<Char>)
		if o.ElementType == "Char" {
			return RUNTIME_TYPE_STRING
		}
		// If ElementType is missing (e.g. literal), check content
		if o.len() > 0 {
			if _, ok := o.get(0).(*Char); ok {
				// Heuristic: if first element is Char, treat as String for dispatch
				// This allows "Hello" (List<Char>) to match "String" instances
				return RUNTIME_TYPE_STRING
			}
		}
		// If explicit ElementType is empty but it was created via string literal logic
		// We might not know. But usually string literals set ElementType="Char" or are just Char lists.
		// If empty list [], ElementType might be empty. It acts as List<_>.
		return RUNTIME_TYPE_LIST
	case *RecordInstance:
		if o.TypeName != "" {
			// Extract local name from qualified name (e.g., "m.Vector" -> "Vector")
			if dotIndex := strings.LastIndex(o.TypeName, "."); dotIndex >= 0 {
				return o.TypeName[dotIndex+1:]
			}
			return o.TypeName
		}
		return RUNTIME_TYPE_RECORD
	case *Function:
		return RUNTIME_TYPE_FUNCTION
	case *DataInstance:
		// Extract local name from qualified name
		if dotIndex := strings.LastIndex(o.TypeName, "."); dotIndex >= 0 {
			return o.TypeName[dotIndex+1:]
		}
		return o.TypeName
	case *Range:
		return RUNTIME_TYPE_RANGE
	default:
		return string(obj.Type())
	}
}

func (e *Evaluator) resolveCanonicalTypeName(t ast.Type, env *Environment) (string, error) {
	switch t := t.(type) {
	case *ast.NamedType:
		name := t.Name.Value
		// For named types (including record aliases and String), use the name as-is
		// BUT if it's an alias for a structural type (Function/Tuple), resolve to the structural name
		// because runtime objects (Function/Tuple) don't carry the alias name.
		if e.TypeAliases != nil {
			if underlying, ok := e.TypeAliases[name]; ok {
				if _, isFunc := underlying.(typesystem.TFunc); isFunc {
					return RUNTIME_TYPE_FUNCTION, nil
				}
				if _, isTuple := underlying.(typesystem.TTuple); isTuple {
					return RUNTIME_TYPE_TUPLE, nil
				}
			}
		}
		return name, nil
	case *ast.TupleType:
		return RUNTIME_TYPE_TUPLE, nil
	case *ast.RecordType:
		return RUNTIME_TYPE_RECORD, nil
	case *ast.FunctionType:
		return RUNTIME_TYPE_FUNCTION, nil
	default:
		return "", fmt.Errorf("unsupported target type for instance: %T", t)
	}
}

func (e *Evaluator) areObjectsEqual(a, b Object) bool {
	if a.Type() != b.Type() {
		return false
	}

	switch a := a.(type) {
	case *Integer:
		return a.Value == b.(*Integer).Value
	case *Float:
		return a.Value == b.(*Float).Value
	case *BigInt:
		return a.Value.Cmp(b.(*BigInt).Value) == 0
	case *Rational:
		return a.Value.Cmp(b.(*Rational).Value) == 0
	case *Boolean:
		return a.Value == b.(*Boolean).Value
	case *Char:
		return a.Value == b.(*Char).Value
	case *Nil:
		return true
	case *List:
		bList := b.(*List)
		if a.len() != bList.len() {
			return false
		}
		for i, el := range a.ToSlice() {
			if !e.areObjectsEqual(el, bList.get(i)) {
				return false
			}
		}
		return true
	case *Tuple:
		bTuple := b.(*Tuple)
		if len(a.Elements) != len(bTuple.Elements) {
			return false
		}
		for i, el := range a.Elements {
			if !e.areObjectsEqual(el, bTuple.Elements[i]) {
				return false
			}
		}
		return true
	case *RecordInstance:
		bRec := b.(*RecordInstance)
		if len(a.Fields) != len(bRec.Fields) {
			return false
		}
		for i, field := range a.Fields {
			bField := bRec.Fields[i]
			if field.Key != bField.Key {
				return false
			}
			if !e.areObjectsEqual(field.Value, bField.Value) {
				return false
			}
		}
		return true
	case *DataInstance:
		bData := b.(*DataInstance)
		if a.Name != bData.Name || a.TypeName != bData.TypeName {
			return false
		}
		if len(a.Fields) != len(bData.Fields) {
			return false
		}
		for i, field := range a.Fields {
			if !e.areObjectsEqual(field, bData.Fields[i]) {
				return false
			}
		}
		return true
	case *TypeObject:
		bType := b.(*TypeObject)
		return a.TypeVal.String() == bType.TypeVal.String()
	case *Bytes:
		return a.equals(b.(*Bytes))
	case *Bits:
		return a.equals(b.(*Bits))
	case *Map:
		return a.equals(b.(*Map), e)
	case *Uuid:
		return a.Value == b.(*Uuid).Value
	}
	return false
}

func (e *Evaluator) isTruthy(obj Object) bool {
	switch obj := obj.(type) {
	case *Boolean:
		return obj.Value
	default:
		return false
	}
}

func (e *Evaluator) evalExpressions(exps []ast.Expression, env *Environment) []Object {
	var result []Object
	for _, exp := range exps {
		// Handle spread expression: args...
		if spread, ok := exp.(*ast.SpreadExpression); ok {
			val := e.Eval(spread.Expression, env)
			if isError(val) {
				return []Object{val}
			}
			if tuple, ok := val.(*Tuple); ok {
				result = append(result, tuple.Elements...)
			} else if listObj, ok := val.(*List); ok {
				result = append(result, listObj.ToSlice()...)
			} else {
				return []Object{newError("cannot spread non-sequence type: %s", val.Type())}
			}
			continue
		}

		evaluated := e.Eval(exp, env)
		if isError(evaluated) {
			return []Object{evaluated}
		}
		result = append(result, evaluated)
	}
	return result
}

// ApplyFunction applies a function to arguments (exported for VM)
func (e *Evaluator) ApplyFunction(fn Object, args []Object) Object {
	switch fn := fn.(type) {
	case *Function:
		// Dictionaries are now handled as WitnessParams above

		extendedEnv := NewEnclosedEnvironment(fn.Env)

		// Handle Generic Instantiation (from Analyzer)
		// This binds type variables (e.g. T -> Int) in the environment so they can be resolved
		// by resolveTypeFromEnv during witness lookup or other type-dependent operations.
		if callNode, ok := e.CurrentCallNode.(*ast.CallExpression); ok && callNode.Instantiation != nil {
			for name, t := range callNode.Instantiation {
				extendedEnv.Set(name, &TypeObject{TypeVal: t})
			}
		}

		// Handle Witness Parameters (Dictionary Passing)
		// Hybrid approach: take dictionaries if present, otherwise assume Tree mode dynamic call
		witnessCount := len(fn.WitnessParams)

		// Count how many leading arguments are dictionaries
		dictCount := 0
		for i := 0; i < len(args); i++ {
			if _, ok := args[i].(*Dictionary); ok {
				dictCount++
			} else {
				break
			}
		}

		// If we have at least one dictionary and witness params, try to consume them
		if dictCount > 0 && witnessCount > 0 {
			// Bind the dictionaries we have
			for i := 0; i < dictCount && i < witnessCount; i++ {
				extendedEnv.Set(fn.WitnessParams[i], args[i])
			}

			// For remaining witness params (e.g., super traits), try to extract from passed dictionaries
			if dictCount < witnessCount {
				// Try to extract super trait dictionaries from the first dictionary
				if dict, ok := args[0].(*Dictionary); ok {
					for i := dictCount; i < witnessCount; i++ {
						// The witness param name is like $dict_t_SubTrait or $dict_t_BaseTrait
						// Try to find a matching super dictionary
						superIdx := i - dictCount
						if superIdx < len(dict.Supers) {
							extendedEnv.Set(fn.WitnessParams[i], dict.Supers[superIdx])
						}
					}
				}
			}
		}

		// Always consume dictionary args, even if the function doesn't explicitly expect them (witnessCount == 0)
		// This handles cases where implicit dictionaries are passed to lambdas or generic functions
		// that don't have explicit witness parameters in their AST definition.
		if dictCount > 0 {
			args = args[dictCount:]
		}

		isVariadic := false
		if len(fn.Parameters) > 0 && fn.Parameters[len(fn.Parameters)-1].IsVariadic {
			isVariadic = true
		}

		// Bind normal parameters
		paramCount := len(fn.Parameters)
		if isVariadic {
			paramCount--
		}

		// Count required parameters (those without defaults)
		requiredParams := 0
		for i := 0; i < paramCount; i++ {
			if fn.Parameters[i].Default == nil {
				requiredParams++
			}
		}

		// Check arg count - support partial application
		if len(args) < requiredParams {
			if len(args) == 0 {
				return e.newErrorWithStack("wrong number of arguments: expected %d, got 0", requiredParams)
			}
			return &PartialApplication{
				Function:        fn,
				Builtin:         nil,
				AppliedArgs:     args,
				RemainingParams: requiredParams - len(args),
			}
		}
		if !isVariadic && len(args) > paramCount {
			return e.newErrorWithStack("wrong number of arguments: expected at most %d, got %d", paramCount, len(args))
		}

		// Bind parameters with args or defaults
		for i := 0; i < paramCount; i++ {
			param := fn.Parameters[i]
			if param.IsIgnored {
				continue
			}
			if i < len(args) {
				extendedEnv.Set(param.Name.Value, args[i])
			} else if param.Default != nil {
				defaultVal := e.Eval(param.Default, fn.Env)
				if isError(defaultVal) {
					return defaultVal
				}
				extendedEnv.Set(param.Name.Value, defaultVal)
			}
		}

		if isVariadic {
			variadicParam := fn.Parameters[paramCount]
			variadicArgs := args[paramCount:]
			if !variadicParam.IsIgnored {
				extendedEnv.Set(variadicParam.Name.Value, newList(variadicArgs))
			}
		}

		// Trampoline Loop for TCO
		currentBody := fn.Body
		currentEnv := extendedEnv

		// Stack Capture/Restore Logic
		var previousStack []map[string][]typesystem.Type
		// If closure captured a stack, use it. Otherwise, assume current stack is valid foundation.
		// NOTE: User-defined lambdas capture stack to prevent "lost context".
		// We replace the current stack with the captured one for the duration of the call.
		if fn.CapturedWitnessStack != nil {
			previousStack = e.WitnessStack
			// Copy to ensure we don't mutate the captured source
			newStack := make([]map[string][]typesystem.Type, len(fn.CapturedWitnessStack))
			copy(newStack, fn.CapturedWitnessStack)
			e.WitnessStack = newStack
		} else {
			// For global functions or those without captured stack, we might want to preserve current stack
			// or start fresh? "Tree Mode works... partially relies on TypeMap".
			// Let's keep current behavior (inherit stack) but track depth for cleanup.
			previousStack = nil // Marker that we didn't swap entire stack
		}

		// Push Return Type Witness (Stage 3)
		// "If -> Option<t>, calculate this type and put in WitnessStack"
		if fn.ReturnType != nil {
			// Resolve type from AST
			sysType := astTypeToTypesystem(fn.ReturnType)
			// Resolve generics using Env (e.g. t -> Int)
			resolvedType := e.resolveTypeFromEnv(sysType, extendedEnv)

			// Determine which traits to witness.
			// Use generic context dispatch: inform the function about expected result type.
			// e.g. Option<Int> -> witness $ContextType: Option<Int>
			constructorName := ExtractTypeConstructorName(resolvedType)
			if constructorName != "" {
				// Create witness map
				witness := make(map[string][]typesystem.Type)
				// Generic context dispatch: pass expected result type
				witness["$ContextType"] = []typesystem.Type{resolvedType}
				// Also push general return context for backward compatibility
				witness["$Return"] = []typesystem.Type{resolvedType}

				e.PushWitness(witness)
			}
		}

		// Track witness depth relative to the (possibly swapped) stack
		initialWitnessDepth := len(e.WitnessStack)

		// Restore stack helper
		restoreStack := func() {
			// Restore local changes (pops)
			e.RestoreWitnessStack(initialWitnessDepth)
			// Restore swapped stack if any
			if previousStack != nil {
				e.WitnessStack = previousStack
			}
		}

		for {
			result := e.Eval(currentBody, currentEnv)
			result = unwrapReturnValue(result)

			// Error: capture stack trace
			if err, ok := result.(*Error); ok {
				restoreStack()
				if len(err.StackTrace) == 0 && len(e.CallStack) > 0 {
					err.StackTrace = make([]StackFrame, len(e.CallStack))
					for i, frame := range e.CallStack {
						err.StackTrace[i] = StackFrame{
							Name:   frame.Name,
							File:   frame.File,
							Line:   frame.Line,
							Column: frame.Column,
						}
					}
				}
				return result
			}

			// Tail call handling
			if tc, ok := result.(*TailCall); ok {
				// Push tail call frame for stack trace
				e.PushCall(tc.Name, tc.File, tc.Line, tc.Column)

				// Restore Witness from TailCall (Proposal 002: Dynamic Dictionary Passing)
				// We need to pop previous iteration's witness if any was pushed.
				e.RestoreWitnessStack(initialWitnessDepth)

				if tc.Witness != nil {
					e.PushWitness(tc.Witness)
				}

				nextFn := tc.Func
				nextArgs := tc.Args

				if nextUserFn, ok := nextFn.(*Function); ok {
					nextEnv := NewEnclosedEnvironment(nextUserFn.Env)
					fn = nextUserFn

					// Handle Witness Params in Tail Call
					witnessCount := len(fn.WitnessParams)
					if witnessCount > 0 {
						if len(nextArgs) < witnessCount {
							e.RestoreWitnessStack(initialWitnessDepth)
							return e.newErrorWithStack("wrong number of arguments: expected at least %d witnesses, got %d", witnessCount, len(nextArgs))
						}
						for i, name := range fn.WitnessParams {
							nextEnv.Set(name, nextArgs[i])
						}
						nextArgs = nextArgs[witnessCount:]
					}

					isVariadic = len(fn.Parameters) > 0 && fn.Parameters[len(fn.Parameters)-1].IsVariadic
					paramCount = len(fn.Parameters)
					if isVariadic {
						paramCount--
					}

					requiredParams := 0
					for i := 0; i < paramCount; i++ {
						if fn.Parameters[i].Default == nil {
							requiredParams++
						}
					}

					if len(nextArgs) < requiredParams {
						e.RestoreWitnessStack(initialWitnessDepth)
						return &PartialApplication{
							Function:        fn,
							AppliedArgs:     nextArgs,
							RemainingParams: requiredParams - len(nextArgs),
						}
					}

					for i := 0; i < paramCount; i++ {
						param := fn.Parameters[i]
						if param.IsIgnored {
							continue
						}
						if i < len(nextArgs) {
							nextEnv.Set(param.Name.Value, nextArgs[i])
						} else if param.Default != nil {
							defaultVal := e.Eval(param.Default, fn.Env)
							if isError(defaultVal) {
								e.RestoreWitnessStack(initialWitnessDepth)
								return defaultVal
							}
							nextEnv.Set(param.Name.Value, defaultVal)
						}
					}

					if isVariadic {
						variadicParam := fn.Parameters[paramCount]
						variadicArgs := nextArgs[paramCount:]
						if !variadicParam.IsIgnored {
							nextEnv.Set(variadicParam.Name.Value, newList(variadicArgs))
						}
					}

					currentBody = fn.Body
					currentEnv = nextEnv
					continue
				} else {
					// Tail call to builtin - restore CurrentCallNode for ClassMethod dispatch
					if tc.CallNode != nil {
						e.CurrentCallNode = tc.CallNode
					}
					res := e.ApplyFunction(nextFn, nextArgs)
					e.RestoreWitnessStack(initialWitnessDepth) // Clean up before returning
					if err, ok := res.(*Error); ok {
						if err.Line == 0 && tc.Line > 0 {
							err.Line = tc.Line
							err.Column = tc.Column
						}
						if len(err.StackTrace) == 0 && len(e.CallStack) > 0 {
							err.StackTrace = make([]StackFrame, len(e.CallStack))
							for i, frame := range e.CallStack {
								err.StackTrace[i] = StackFrame{
									Name:   frame.Name,
									File:   frame.File,
									Line:   frame.Line,
									Column: frame.Column,
								}
							}
						}
					}
					// Pop the tail call frame before returning
					e.PopCall()
					return res
				}
			}

			// Restore witness stack before returning final result
			e.RestoreWitnessStack(initialWitnessDepth)

			// Set TypeName on RecordInstance if function has a named return type
			// But don't override if record was extended via Row Polymorphism
			if record, ok := result.(*RecordInstance); ok && record.TypeName == "" && !record.RowPolyExtended {
				if fn.ReturnType != nil {
					// Check if return type is a named type (type alias)
					if namedType, ok := fn.ReturnType.(*ast.NamedType); ok {
						record.TypeName = namedType.Name.Value
					}
				}
			}

			return result
		}

	case *Builtin:
		// Check if we have TypeInfo to determine expected params
		if fn.TypeInfo != nil {
			if fnType, ok := fn.TypeInfo.(typesystem.TFunc); ok && !fnType.IsVariadic {
				totalParams := len(fnType.Params)
				requiredParams := totalParams - fnType.DefaultCount

				if len(args) < requiredParams {
					// Partial application requires at least 1 argument
					if len(args) == 0 {
						return newError("wrong number of arguments: expected at least %d, got 0", requiredParams)
					}
					// Partial application for builtin
					return &PartialApplication{
						Function:        nil,
						Builtin:         fn,
						AppliedArgs:     args,
						RemainingParams: requiredParams - len(args),
					}
				}

				// Fill in default arguments if not all provided
				if len(args) < totalParams && len(fn.DefaultArgs) > 0 {
					// How many defaults do we need?
					missingCount := totalParams - len(args)
					// DefaultArgs are for the trailing parameters
					// If we have 5 params and 2 defaults, defaults are for params 3,4 (0-indexed)
					// If user provides 3 args, we need 2 defaults
					defaultStartIdx := len(fn.DefaultArgs) - missingCount
					if defaultStartIdx >= 0 && defaultStartIdx < len(fn.DefaultArgs) {
						args = append(args, fn.DefaultArgs[defaultStartIdx:]...)
					}
				}
			}
		}
		return fn.Fn(e, args...)

	case *PartialApplication:
		// Combine applied args with new args
		allArgs := append(fn.AppliedArgs, args...)

		if fn.Function != nil {
			return e.ApplyFunction(fn.Function, allArgs)
		}
		if fn.Builtin != nil {
			return e.ApplyFunction(fn.Builtin, allArgs)
		}
		if fn.Constructor != nil {
			return e.ApplyFunction(fn.Constructor, allArgs)
		}
		return newError("invalid partial application")
	case *Constructor:
		// Support partial application for constructors
		if fn.Arity > 0 && len(args) < fn.Arity {
			// Partial application requires at least 1 argument
			if len(args) == 0 {
				return newError("wrong number of arguments: expected %d, got 0", fn.Arity)
			}
			return &PartialApplication{
				Function:        nil,
				Builtin:         nil,
				Constructor:     fn,
				AppliedArgs:     args,
				RemainingParams: fn.Arity - len(args),
			}
		}

		// Extract TypeArgs from leading TypeObject arguments (Reified Generics)
		var typeArgs []typesystem.Type
		var valueArgs []Object
		for _, arg := range args {
			if typeObj, ok := arg.(*TypeObject); ok {
				typeArgs = append(typeArgs, typeObj.TypeVal)
			} else {
				valueArgs = append(valueArgs, arg)
			}
		}

		return &DataInstance{Name: fn.Name, Fields: valueArgs, TypeName: fn.TypeName, TypeArgs: typeArgs}
	case *TypeObject:
		// Check for value construction/casting (e.g. Sum({x:1}))
		// If arguments are values (not Types), treat as construction.
		isConstruction := false
		if len(args) > 0 {
			if _, ok := args[0].(*TypeObject); !ok {
				isConstruction = true
			}
		}

		if isConstruction {
			if len(args) != 1 {
				return newError("type constructor expects 1 argument, got %d", len(args))
			}
			val := args[0]
			// Tag RecordInstance with type name for nominal typing
			if rec, ok := val.(*RecordInstance); ok {
				return &RecordInstance{
					Fields:   rec.Fields,
					TypeName: ExtractTypeConstructorName(fn.TypeVal),
				}
			}
			// Return other values as-is (aliases)
			return val
		}

		var typeArgs []typesystem.Type
		for _, arg := range args {
			if tArg, ok := arg.(*TypeObject); ok {
				typeArgs = append(typeArgs, tArg.TypeVal)
			} else {
				return newError("type application expects types as arguments, got %s", arg.Type())
			}
		}
		return &TypeObject{TypeVal: typesystem.TApp{Constructor: fn.TypeVal, Args: typeArgs}}
	case *ClassMethod:
		var foundMethod Object
		var dispatchTypeName string

		// 0. First try argument-based dispatch (like VM does)
		// This ensures we use the correct instance based on runtime argument types
		var argCandidate Object
		var argTypeName string
		var argCandidateIsExact bool

		if typesMap, ok := e.ClassImplementations[fn.ClassName]; ok {
			// Strategy 0a: Exact Key Match (Priority)
			// If the number of arguments matches the number of type parameters (inferred from keys),
			// try to construct a key directly from argument types. This solves MPTC ambiguity.
			var traitArity int = -1
			for k := range typesMap {
				traitArity = strings.Count(k, "_") + 1
				break // Assume all keys have same arity (valid for MPTC)
			}

			if traitArity > 0 && len(args) == traitArity {
				var exactKeyParts []string
				for _, arg := range args {
					exactKeyParts = append(exactKeyParts, e.getDispatchTypeName(arg))
				}

				// Add context if available (like VM does)
				contextType := ""
				if len(e.TypeContextStack) > 0 {
					contextType = e.TypeContextStack[len(e.TypeContextStack)-1]
				}

				exactKey := strings.Join(exactKeyParts, "_")
				if contextType != "" {
					exactKey = exactKey + "_" + contextType
				}

				if methodTableObj, ok := typesMap[exactKey]; ok {
					if methodTable, ok := methodTableObj.(*MethodTable); ok {
						if method, ok := methodTable.Methods[fn.Name]; ok {
							if e.checkArgsMatch(method, args) {
								argCandidate = method
								argTypeName = exactKey
								argCandidateIsExact = true
							}
						}
					}
				}
			}

			// Strategy 0b: Fuzzy Match (Fallback) - like VM LookupTraitMethodFuzzy
			if argCandidate == nil {
				var bestCandidate Object
				bestScore := -1
				bestKey := ""

				// Get context for scoring
				contextType := ""
				if len(e.TypeContextStack) > 0 {
					contextType = e.TypeContextStack[len(e.TypeContextStack)-1]
				}
				// Skip dictionary args for scoring
				actualArgs := args
				for len(actualArgs) > 0 {
					if _, ok := actualArgs[0].(*Dictionary); ok {
						actualArgs = actualArgs[1:]
					} else {
						break
					}
				}
				// Debug: print available context
				if contextType == "" {
					// Try to get from CurrentCallNode if it's an assignment
					if callNode, ok := e.CurrentCallNode.(*ast.CallExpression); ok {
						if len(callNode.TypeArgs) > 0 {
							// This is a workaround - in real code this should be set properly
							// For testing: assume the call result type is in some context
						}
					}
				}

				for key := range typesMap {
					parts := strings.Split(key, "_")

					// Check prefix match with actual args (like VM)
					match := true
					score := 0
					argCount := len(actualArgs)

					// We only check args we have. If key is longer, that's fine (partial match on key).
					// If args are longer than key? Then key is too short, mismatch.
					if argCount > len(parts) {
						match = false
					} else {
						for i, arg := range actualArgs {
							argType := e.getDispatchTypeName(arg)
							if parts[i] != argType {
								match = false
								break
							}
							score++ // +1 for each arg match
						}
					}

					if match {
						// Boost score if context matches any remaining part of the key (like VM)
						if contextType != "" && len(parts) > argCount {
							for i := argCount; i < len(parts); i++ {
								if parts[i] == contextType {
									score++
									break // Only count once
								}
							}
						}

						if score > bestScore {
							// Check if we can get the method
							if method, found := e.lookupTraitMethod(fn.ClassName, fn.Name, parts...); found {
								// For trait methods, just check arity (MPT C methods have generic params)
								if fn.Arity < 0 || len(actualArgs) == fn.Arity {
									bestCandidate = method
									bestScore = score
									bestKey = key
								}
							}
						}
					}
				}

				if bestCandidate != nil {
					argCandidate = bestCandidate
					argTypeName = bestKey
				}
			}

			// If we found a method via argument-based dispatch, use it
			if argCandidate != nil {
				return e.ApplyFunction(argCandidate, args)
			}
		}

		// Continue with context-based dispatch and other strategies

		// 1. Try Explicit Witness Argument (Proposal 002)
		// If the first argument is a Dictionary, check if it contains the method.
		// This handles cases where the compiler/analyzer explicitly passed a witness.
		// Also strip placeholder dictionaries or other explicit dictionaries that don't match this method.
		// Builtins must loop to consume all leading dictionaries if needed, but ApplyFunction can help.

		// Loop to strip leading dictionaries and find the method
		var remainingArgs = args
		for len(remainingArgs) > 0 {
			if dict, ok := remainingArgs[0].(*Dictionary); ok {
				if method := FindMethodInDictionary(dict, fn.Name); method != nil {
					// Check if it's actually implemented (not Nil placeholder)
					if _, isNil := method.(*Nil); !isNil {
						// Validate the method matches the runtime arguments
						// Handle two cases:
						// 1. Method has WitnessParams (Tree mode instance): expects dictionary argument
						// 2. Method has NO WitnessParams (VM mode closure): does not expect dictionary argument

						wantsWitness := false
						if fnObj, ok := method.(*Function); ok && len(fnObj.WitnessParams) > 0 {
							wantsWitness = true
						}

						if wantsWitness {
							// Method expects witness. Pass full args. checkArgsMatch handles skipping.
							if e.checkArgsMatch(method, remainingArgs) {
								return e.ApplyFunction(method, remainingArgs)
							}
						} else {
							// Method does not expect witness. Strip dictionary manually before checking/calling.
							if e.checkArgsMatch(method, remainingArgs[1:]) {
								return e.ApplyFunction(method, remainingArgs[1:])
							}
						}
						// Method found but doesn't match args - continue searching
					}
				}
				// Strip dictionary (whether method found as Nil, doesn't match, or not found at all)
				remainingArgs = remainingArgs[1:]
			} else {
				// Not a dictionary, stop checking.
				break
			}
		}

		// Update args to remainingArgs after witness stripping
		args = remainingArgs

		// 0b. Try Implicit Witness from Stack (Legacy/Context)
		// ONLY for nullary methods (Arity == 0) like pure, mempty.
		// For non-nullary methods, prefer argument-based dispatch because:
		// 1. The argument runtime type is more reliable
		// 2. Multiple constraints on different type vars can overwrite each other's witness
		if fn.Arity == 0 {
			if witnessTypes := e.GetWitness(fn.ClassName); witnessTypes != nil {
				// MPTC Support: witnessTypes is a slice
				var typeNames []string
				for _, t := range witnessTypes {
					typeNames = append(typeNames, ExtractTypeConstructorName(t))
				}

				// Form lookup key
				witnessTypeName := ""
				if len(typeNames) == 1 {
					witnessTypeName = typeNames[0]
				} else {
					witnessTypeName = strings.Join(typeNames, "_")
				}

				if typesMap, ok := e.ClassImplementations[fn.ClassName]; ok {
					if methodTableObj, ok := typesMap[witnessTypeName]; ok {
						if methodTable, ok := methodTableObj.(*MethodTable); ok {
							if method, ok := methodTable.Methods[fn.Name]; ok {
								// Found via witness!
								return e.ApplyFunction(method, args)
							}
						}
					}
				}
			}
		}

		// 0c. Try Generic Context Dispatch via $ContextType
		// If method call failed, check for $ContextType witness to dispatch by expected result type
		if foundMethod == nil {
			if contextTypes := e.GetWitness("$ContextType"); contextTypes != nil && len(contextTypes) > 0 {
				// Get expected result type from context
				expectedType := contextTypes[0]
				expectedTypeName := ExtractTypeConstructorName(expectedType)

				// Check if the trait is implemented for this expected type
				if typesMap, ok := e.ClassImplementations[fn.ClassName]; ok {
					if methodTableObj, ok := typesMap[expectedTypeName]; ok {
						if methodTable, ok := methodTableObj.(*MethodTable); ok {
							if method, ok := methodTable.Methods[fn.Name]; ok {
								// Found via generic context dispatch!
								foundMethod = method
								dispatchTypeName = expectedTypeName
							}
						}
					}
				}
			}
		}

		// Check for hidden type hint argument (injected by Compiler/VM)
		// If explicit arguments > Arity, and last arg is String, treat as hint.
		// pure(val, "List") -> pure(val) with context "List"
		// mempty("List") -> mempty() with context "List"
		if fn.Arity >= 0 && len(args) == fn.Arity+1 {
			if typeHint, ok := args[len(args)-1].(*TypeObject); ok {
				// It's a Type object hint
				// Push witness for trait dispatch (Proposal 002)
				e.PushWitness(map[string][]typesystem.Type{"Applicative": {typeHint.TypeVal}})
				defer e.PopWitness()
				// Use the type name
				dispatchTypeName = ExtractTypeConstructorName(typeHint.TypeVal)
				// Remove hint from args
				args = args[:len(args)-1]
			} else if hintList, ok := args[len(args)-1].(*List); ok {
				// String hint (deprecated but supported for simple strings)
				// Convert List<Char> to string
				if str := ListToString(hintList); str != "" || hintList.Len() == 0 {
					dispatchTypeName = str
					args = args[:len(args)-1]
				}
			}
		}

		// 1. Try to dispatch by Context (ReturnType or ContainerContext)
		// This handles cases like pure(x) -> F<x> where F is determined by context, not by x.

		var contextTypeName string
		var expectedType typesystem.Type
		var contextFromExplicitAnnotation bool // true if context comes from user annotation

		// 1a. Container Context (from >>=)
		if e.ContainerContext != "" {
			contextTypeName = e.ContainerContext
			contextFromExplicitAnnotation = true
		}

		// 1b. Return Type Context (from annotations or inferred types)
		if contextTypeName == "" {
			// Check TypeContextStack first (AnnotatedExpression stack)
			if len(e.TypeContextStack) > 0 {
				contextTypeName = e.TypeContextStack[len(e.TypeContextStack)-1]
				contextFromExplicitAnnotation = true
			} else if e.CurrentCallNode != nil {
				// Check AST nodes for explicit annotations first
				if assign, ok := e.CurrentCallNode.(*ast.AssignExpression); ok && assign.AnnotatedType != nil {
					contextTypeName = extractTypeNameFromAST(assign.AnnotatedType)
					contextFromExplicitAnnotation = true
				} else if annotated, ok := e.CurrentCallNode.(*ast.AnnotatedExpression); ok && annotated.TypeAnnotation != nil {
					contextTypeName = extractTypeNameFromAST(annotated.TypeAnnotation)
					contextFromExplicitAnnotation = true
				} else if constant, ok := e.CurrentCallNode.(*ast.ConstantDeclaration); ok && constant.TypeAnnotation != nil {
					contextTypeName = extractTypeNameFromAST(constant.TypeAnnotation)
					contextFromExplicitAnnotation = true
				}

			}
		}

		// Always try to get expectedType from TypeMap if available (needed for contextIsContainer)
		if e.TypeMap != nil && e.CurrentCallNode != nil {
			if t := e.TypeMap[e.CurrentCallNode]; t != nil {
				// Priority 1: Explicit Witness from AST (Tree Mode - Explicit Witness Passing)
				// Resolve generic types using CurrentEnv if available
				t = e.resolveTypeFromEnv(t, e.CurrentEnv)

				if contextTypeName == "" {
					contextTypeName = ExtractTypeConstructorName(t)
				}
				if expectedType == nil {
					expectedType = t
				}
			}
		}

		var contextCandidate Object
		if contextTypeName != "" {
			if typesMap, ok := e.ClassImplementations[fn.ClassName]; ok {
				// Strategy: 2-Level Lookup
				// 1. Try Nominal Match (e.g. "String")
				// 2. Try Underlying/Alias Match (e.g. "List")

				// Helper to find candidate
				findCandidate := func(targetType string) Object {
					// MPTC Support: Check if targetType matches any part of the instance key
					for key, methodTableObj := range typesMap {
						// FIX: Check if we have exact match for the key first
						if key == targetType {
							if methodTable, ok := methodTableObj.(*MethodTable); ok {
								if method, ok := methodTable.Methods[fn.Name]; ok {
									if e.checkArgsMatch(method, args) {
										return method
									}
								}
							}
						}

						parts := strings.Split(key, "_")
						match := false
						for _, part := range parts {
							if part == targetType {
								match = true
								break
							}
						}

						if match {
							if methodTable, ok := methodTableObj.(*MethodTable); ok {
								if method, ok := methodTable.Methods[fn.Name]; ok {
									// Validate against args to avoid false positives (e.g. MPTC context match but arg mismatch)
									if e.checkArgsMatch(method, args) {
										return method
									}
								}
							}
						}
					}
					return nil
				}

				// Attempt 1: Nominal
				contextCandidate = findCandidate(contextTypeName)

				// Attempt 2: Underlying/Alias (if not found)
				if contextCandidate == nil && e.TypeAliases != nil {
					if underlying, ok := e.TypeAliases[contextTypeName]; ok {
						underlyingName := ExtractTypeConstructorName(underlying)
						if underlyingName != "" && underlyingName != contextTypeName {
							contextCandidate = findCandidate(underlyingName)
						}
					}
				}
			}
		}

		// 2. Try to dispatch by Arguments (continued from section 0)

		if typesMap, ok := e.ClassImplementations[fn.ClassName]; ok {
			// Strategy 2a: Exact Key Match (Priority)
			// If the number of arguments matches the number of type parameters (inferred from keys),
			// try to construct a key directly from argument types. This solves MPTC ambiguity.
			var traitArity int = -1
			for k := range typesMap {
				traitArity = strings.Count(k, "_") + 1
				break // Assume all keys have same arity (valid for MPTC)
			}

			if traitArity > 0 && len(args) == traitArity {
				var exactKeyParts []string
				for _, arg := range args {
					exactKeyParts = append(exactKeyParts, e.getDispatchTypeName(arg))
				}
				exactKey := strings.Join(exactKeyParts, "_")

				if methodTableObj, ok := typesMap[exactKey]; ok {
					if methodTable, ok := methodTableObj.(*MethodTable); ok {
						if method, ok := methodTable.Methods[fn.Name]; ok {
							if e.checkArgsMatch(method, args) {
								argCandidate = method
								argTypeName = exactKey
								argCandidateIsExact = true
							}
						}
					}
				}
			}

			// Strategy 2b: Fuzzy Match (Fallback)
			// Attempt to use unified lookupTraitMethod by finding the correct key first.
			// Strict key lookup (e.g. "Int_String") fails for HKT/MPTC when args are partial (e.g. fmap(fn, list)).
			// We restore the "fuzzy match" loop but delegate the actual retrieval to lookupTraitMethod for consistency.
			if argCandidate == nil {
				for key := range typesMap {
					parts := strings.Split(key, "_")
					match := false

					// Check if any argument matches any part of the key
					for _, part := range parts {
						for _, arg := range args {
							argType := e.getDispatchTypeName(arg)
							// Match if types are equal OR if part is a type variable (starts with lowercase)
							// This enables flexible instances like instance C<a, b> to match concrete types
							isVar := len(part) > 0 && part[0] >= 'a' && part[0] <= 'z'
							if argType == part || isVar {
								match = true
								break
							}
							// General alias fallback
							if e.TypeAliases != nil {
								if underlying, ok := e.TypeAliases[argType]; ok {
									if ExtractTypeConstructorName(underlying) == part {
										match = true
										break
									}
								}
							}
						}
						if match {
							break
						}
					}

					if match {
						// Use unified lookup to get the method (handles inheritance etc.)
						// We pass 'parts' to reconstruct the key inside lookupTraitMethod
						if method, found := e.lookupTraitMethod(fn.ClassName, fn.Name, parts...); found {
							// Validate against args to ensure correct overload selection
							if e.checkArgsMatch(method, args) {
								argCandidate = method
								argTypeName = key
								break
							}
						}
					}
				}
			}
		}

		// 3. Decide which one to use
		// Two dispatch strategies:
		// - Arg-based: for traits where type param is in argument (show, eq, stringify)
		// - Context-based: for traits where type param is container in return (pure, fmap)
		//
		// Heuristic: if context type is a CONTAINER that wraps the arg type, use context.
		// Example: pure("hello") with context Option<String> → String is inside Option<String> → context
		// Example: stringify(42) with context String → Int is NOT inside String → arg
		contextIsContainer := false
		if expectedType != nil && len(args) > 0 {
			// Check if context type is a container (TApp) that contains the arg's type
			if tapp, ok := expectedType.(typesystem.TApp); ok {
				// Context is a container type like Option<T>, List<T>, State<S, T>
				// Check if any arg's type matches one of the container's type arguments
				for _, arg := range args {
					argRuntimeType := normalizeTypeName(getRuntimeTypeName(arg))
					for _, typeArg := range tapp.Args {
						typeArgName := normalizeTypeName(ExtractTypeConstructorName(typeArg))

						// Check direct match
						if typeArgName == argRuntimeType {
							contextIsContainer = true
							break
						}

						// Check alias match (forward: slot is alias)
						if e.TypeAliases != nil {
							if underlying, ok := e.TypeAliases[typeArgName]; ok {
								underlyingName := normalizeTypeName(ExtractTypeConstructorName(underlying))
								if underlyingName == argRuntimeType {
									contextIsContainer = true
									break
								}
							}

							// Check alias match (reverse: arg is alias)
							// e.g. slot is "List", arg is "String" (alias for List<Char>)
							if underlying, ok := e.TypeAliases[argRuntimeType]; ok {
								underlyingName := normalizeTypeName(ExtractTypeConstructorName(underlying))
								if underlyingName == typeArgName {
									contextIsContainer = true
									break
								}
							}
						}
					}
					if contextIsContainer {
						break
					}
				}
			}
		}

		// Special case: pure matches on return type (context), not argument type
		// pure :: a -> f a. The trait 'f' does not appear in the argument 'a'.
		// Argument-based dispatch would incorrectly select the instance for 'a' (if any) instead of 'f'.
		if fn.ClassName == "Applicative" && fn.Name == "pure" {
			argCandidate = nil
			argTypeName = ""
		}

		if fn.ClassName == "Show" && argCandidate != nil {
			// Show trait: always use arg-based dispatch (return is always String)
			foundMethod = argCandidate
			dispatchTypeName = argTypeName
		} else if contextCandidate != nil && contextIsContainer {
			// Context is a container that wraps the arg → use context
			foundMethod = contextCandidate
			dispatchTypeName = contextTypeName
		} else if argCandidate != nil && argCandidateIsExact {
			// Exact argument match trumps fuzzy context match
			foundMethod = argCandidate
			dispatchTypeName = argTypeName
		} else if fn.Arity == 0 && contextCandidate != nil {
			// Nullary method: must use context
			foundMethod = contextCandidate
			dispatchTypeName = contextTypeName
		} else if contextCandidate != nil && contextIsContainer {
			// Context is a container that wraps the arg → use context
			foundMethod = contextCandidate
			dispatchTypeName = contextTypeName
		} else if contextCandidate != nil && contextFromExplicitAnnotation && contextTypeName != argTypeName {
			// Explicit annotation differs from arg: respect user's intent
			foundMethod = contextCandidate
			dispatchTypeName = contextTypeName
		} else if argCandidate != nil {
			// Default: use arg-based dispatch
			foundMethod = argCandidate
			dispatchTypeName = argTypeName
		} else if contextCandidate != nil && contextFromExplicitAnnotation {
			// Only use context if from explicit annotation, not inferred return type
			// This prevents inferred return type (e.g. Int) from overriding default dispatch
			foundMethod = contextCandidate
			dispatchTypeName = contextTypeName
		}
		// If no candidate found, fall through to trait defaults
		if foundMethod != nil {
			// Push witness if we dispatched via context/TypeMap (Proposal 002)
			// This is crucial for generic types like OptionT<M> which need M's witness
			if expectedType != nil && dispatchTypeName == contextTypeName {
				e.PushWitness(map[string][]typesystem.Type{fn.ClassName: {expectedType}})
				defer e.PopWitness()
			}
			return e.ApplyFunction(foundMethod, args)
		}

		// Determine type name for error message
		if dispatchTypeName == "" && len(args) > 0 {
			dispatchTypeName = getRuntimeTypeName(args[0])
		}
		if dispatchTypeName == "" {
			dispatchTypeName = "unknown"
		}

		// Fallback to trait default implementation (from user-defined traits)
		if e.TraitDefaults != nil {
			key := fn.ClassName + "." + fn.Name
			if fnStmt, ok := e.TraitDefaults[key]; ok {
				// JIT register default implementation in ClassImplementations (similar to VM mode)
				if dispatchTypeName != "" && dispatchTypeName != "unknown" {
					// Ensure trait map exists
					if _, ok := e.ClassImplementations[fn.ClassName]; !ok {
						e.ClassImplementations[fn.ClassName] = make(map[string]Object)
					}

					// Get or create method table for this type
					var table *MethodTable
					if existing, ok := e.ClassImplementations[fn.ClassName][dispatchTypeName]; ok {
						if mt, ok := existing.(*MethodTable); ok {
							table = mt
						} else {
							// If it's not a MethodTable, create new one
							table = &MethodTable{Methods: make(map[string]Object)}
						}
					} else {
						table = &MethodTable{Methods: make(map[string]Object)}
					}

					// Add default method if not already present
					if _, exists := table.Methods[fn.Name]; !exists {
						defaultFn := &Function{
							Name:       fn.Name,
							Parameters: fnStmt.Parameters,
							Body:       fnStmt.Body,
							Env:        e.GlobalEnv,
							Line:       fnStmt.Token.Line,
							Column:     fnStmt.Token.Column,
						}
						table.Methods[fn.Name] = defaultFn
						e.ClassImplementations[fn.ClassName][dispatchTypeName] = table
					}
				}

				// Create function for immediate call (use registered one if available)
				defaultFn := &Function{
					Name:       fn.Name,
					Parameters: fnStmt.Parameters,
					Body:       fnStmt.Body,
					Env:        e.GlobalEnv,
					Line:       fnStmt.Token.Line,
					Column:     fnStmt.Token.Column,
				}
				return e.ApplyFunction(defaultFn, args)
			}
		}

		// Fallback to builtin trait default implementation (for built-in traits like Show)
		if e.BuiltinTraitDefaults != nil {
			key := fn.ClassName + "." + fn.Name
			if builtin, ok := e.BuiltinTraitDefaults[key]; ok {
				return e.ApplyFunction(builtin, args)
			}
		}

		return newError("implementation of class %s for type %s (method %s) not found", fn.ClassName, dispatchTypeName, fn.Name)

	case *BoundMethod:
		// FIX: Strip witness from args if Function doesn't expect it
		if userFn, ok := fn.Function.(*Function); ok {
			if len(userFn.WitnessParams) == 0 && len(args) > 0 {
				if _, ok := args[0].(*Dictionary); ok {
					args = args[1:]
				}
			}
		}
		newArgs := append([]Object{fn.Receiver}, args...)
		return e.ApplyFunction(fn.Function, newArgs)

	case *OperatorFunction:
		// Operator as function: (+), (-), etc.
		// FIX: Strip witness if present
		if len(args) > 2 {
			if _, ok := args[0].(*Dictionary); ok {
				args = args[1:]
			}
		}

		if len(args) != 2 {
			return newError("operator function %s expects 2 arguments, got %d", fn.Inspect(), len(args))
		}
		// Use fn.Evaluator if available, otherwise use current evaluator
		eval := fn.Evaluator
		if eval == nil {
			eval = e
		}
		return eval.EvalInfixExpression(fn.Operator, args[0], args[1])

	case *ComposedFunction:
		// Composed function: (f ,, g)(x) = f(g(x))
		// FIX: Strip witness if present
		if len(args) > 1 {
			if _, ok := args[0].(*Dictionary); ok {
				args = args[1:]
			}
		}

		if len(args) != 1 {
			return newError("composed function expects 1 argument, got %d", len(args))
		}
		// First apply g to the argument
		gResult := fn.Evaluator.ApplyFunction(fn.G, args)
		if isError(gResult) {
			return gResult
		}
		// Then apply f to the result
		return fn.Evaluator.ApplyFunction(fn.F, []Object{gResult})

	default:
		// Try VM call handler for VM closures
		if e.VMCallHandler != nil {
			result := e.VMCallHandler(fn, args)
			if result != nil {
				return result
			}
		}
		return newError("not a function: %s", fn.Type())
	}
}

// lookupTraitMethodByName looks up a trait method by name across all traits.
// Returns a ClassMethod wrapper if found, nil otherwise.
// This is used for identifier lookup (e.g., when calling `fmap(...)` directly).
func (e *Evaluator) lookupTraitMethodByName(methodName string) Object {
	// Check if any trait has this method registered (explicit implementation)
	for traitName, typesMap := range e.ClassImplementations {
		// If at least one type has an implementation, return a ClassMethod dispatcher
		for _, methodTableObj := range typesMap {
			if methodTable, ok := methodTableObj.(*MethodTable); ok {
				if _, ok := methodTable.Methods[methodName]; ok {
					// Found! Return a ClassMethod dispatcher
					// Arity -1 means unknown/don't auto-call
					return &ClassMethod{
						Name:      methodName,
						ClassName: traitName,
						Arity:     -1,
					}
				}
			}
		}
	}

	// Check default implementations (e.g. lte in OrderSemantics)
	// This is necessary if no type explicitly overrides the default method yet
	if e.TraitDefaults != nil {
		suffix := "." + methodName
		for key, fnStmt := range e.TraitDefaults {
			if strings.HasSuffix(key, suffix) {
				traitName := strings.TrimSuffix(key, suffix)
				return &ClassMethod{
					Name:      methodName,
					ClassName: traitName,
					Arity:     len(fnStmt.Parameters),
				}
			}
		}
	}

	return nil
}

// matchStringPattern matches a string against a pattern with captures.
// Pattern parts can be literals or capture groups like {name} or {path...} (greedy).
// Returns (matched, captures) where captures maps variable names to captured values.
// MatchStringPattern matches a string against a pattern with captures (exported for VM)
func MatchStringPattern(parts []ast.StringPatternPart, str string) (bool, map[string]string) {
	captures := make(map[string]string)
	pos := 0

	for i, part := range parts {
		if !part.IsCapture {
			// Literal part - must match exactly
			if !strings.HasPrefix(str[pos:], part.Value) {
				return false, nil
			}
			pos += len(part.Value)
		} else {
			// Capture part
			if part.Greedy {
				// Greedy: capture everything until end or next literal
				if i+1 < len(parts) && !parts[i+1].IsCapture {
					// Find next literal
					nextLit := parts[i+1].Value
					idx := strings.Index(str[pos:], nextLit)
					if idx == -1 {
						return false, nil
					}
					captures[part.Value] = str[pos : pos+idx]
					pos += idx
				} else {
					// No next literal - capture rest of string
					captures[part.Value] = str[pos:]
					pos = len(str)
				}
			} else {
				// Non-greedy: capture until next '/' or end
				end := pos
				for end < len(str) && str[end] != '/' {
					end++
				}
				// Also stop at next literal if present
				if i+1 < len(parts) && !parts[i+1].IsCapture {
					nextLit := parts[i+1].Value
					idx := strings.Index(str[pos:], nextLit)
					if idx != -1 && pos+idx < end {
						end = pos + idx
					}
				}
				captures[part.Value] = str[pos:end]
				pos = end
			}
		}
	}

	// All parts matched and consumed entire string (or at least matched all parts)
	return pos == len(str), captures
}

// checkArgsMatch checks if the arguments match the function parameters
// This is used to disambiguate trait dispatch (Context vs Args)
func (e *Evaluator) checkArgsMatch(fn Object, args []Object) bool {
	switch f := fn.(type) {
	case *Function:
		// Skip witness arguments for validation ONLY if they are actually provided
		witnessCount := len(f.WitnessParams)
		actualArgs := args

		hasWitnesses := false
		if witnessCount > 0 && len(args) >= witnessCount {
			hasWitnesses = true
			for i := 0; i < witnessCount; i++ {
				if _, ok := args[i].(*Dictionary); !ok {
					hasWitnesses = false
					break
				}
			}
		}

		if hasWitnesses {
			actualArgs = args[witnessCount:]
		}

		paramCount := len(f.Parameters)
		isVariadic := false
		if paramCount > 0 && f.Parameters[paramCount-1].IsVariadic {
			isVariadic = true
			paramCount--
		}

		for i, arg := range actualArgs {
			if i >= paramCount {
				if isVariadic {
					// Variadic args match generic list usually, hard to check strict type without detailed info
					return true
				}
				return false // Too many args
			}

			param := f.Parameters[i]
			if !e.fuzzyMatchAstType(arg, param.Type) {
				return false
			}
		}
		return true

	case *Builtin:
		if f.TypeInfo == nil {
			return true // Optimistic
		}
		if tFunc, ok := f.TypeInfo.(typesystem.TFunc); ok {
			params := tFunc.Params
			isVariadic := tFunc.IsVariadic

			for i, arg := range args {
				if i >= len(params) {
					if isVariadic {
						return true
					}
					return false
				}
				if !e.fuzzyMatchSystemType(arg, params[i]) {
					return false
				}
			}
			return true
		}
		return true

	default:
		return true // Optimistic
	}
}

func (f *Function) countDefaults() int {
	count := 0
	for _, p := range f.Parameters {
		if p.Default != nil {
			count++
		}
	}
	return count
}

// fuzzyMatchAstType checks if val matches type, being optimistic about generics
func (e *Evaluator) fuzzyMatchAstType(val Object, t ast.Type) bool {
	// Nil type means Any (untyped parameter) - always match
	if t == nil {
		return true
	}

	// If it's a NamedType and not standard, assume generic and match
	if nt, ok := t.(*ast.NamedType); ok {
		if !isStandardType(nt.Name.Value) {
			return true
		}
	}
	// Use strict check for standard types
	return e.matchesType(val, t)
}

// fuzzyMatchSystemType checks if val matches system type
func (e *Evaluator) fuzzyMatchSystemType(val Object, t typesystem.Type) bool {
	switch typ := t.(type) {
	case typesystem.TCon:
		if !isStandardType(typ.Name) {
			return true
		}
		// Check strict runtime type match
		runtimeType := getRuntimeTypeName(val)
		if runtimeType == typ.Name {
			return true
		}
		// Check aliases systemically
		if e.TypeAliases != nil {
			if alias, ok := e.TypeAliases[typ.Name]; ok {
				aliasName := ExtractTypeConstructorName(alias)
				if aliasName == runtimeType {
					return true
				}
			}
		}
		return false
	case typesystem.TVar:
		return true // Generic matches all
	default:
		return true // Complex types - optimistic
	}
}

func isStandardType(name string) bool {
	switch name {
	case "Int", "Float", "Bool", "Char", "String", "List", "Map", "Bytes", "Bits",
		"Option", "Result", "Nil", "BigInt", "Rational", "Tuple":
		return true
	}
	return false
}

// normalizeTypeName normalizes type names for runtime comparison (e.g. String -> List)
func normalizeTypeName(name string) string {
	// We want to preserve String distinct from List for trait dispatch
	return name
}

// Helper to extract return type name from Function/Builtin
func (e *Evaluator) getReturnTypeName(obj Object) string {
	switch fn := obj.(type) {
	case *Function:
		if fn.ReturnType != nil {
			return extractTypeNameFromAST(fn.ReturnType)
		}
	case *Builtin:
		if fn.TypeInfo != nil {
			if tFunc, ok := fn.TypeInfo.(typesystem.TFunc); ok {
				return ExtractTypeConstructorName(tFunc.ReturnType)
			}
		}
	case *BoundMethod:
		return e.getReturnTypeName(fn.Function)
	}
	return ""
}

// ============================================================================
// Helpers for Dictionary Passing (Proposal 002)
// ============================================================================

// FindMethodInDictionary searches for a method in a dictionary or its supers.
// Returns nil if not found.
func FindMethodInDictionary(d *Dictionary, methodName string) Object {
	// Check if this dictionary belongs to a trait that has the method
	if methods, ok := TraitMethods[d.TraitName]; ok {
		for i, name := range methods {
			if name == methodName {
				// Found the method name in this trait definition
				if i < len(d.Methods) {
					return d.Methods[i]
				}
				return nil // Should not happen if dictionary is well-formed
			}
		}
	}

	// Recurse into supers
	for _, super := range d.Supers {
		if m := FindMethodInDictionary(super, methodName); m != nil {
			return m
		}
	}
	return nil
}

// extractWitnessMethod tries to extract a method from an explicit witness dictionary
// passed as the first argument.
// Returns: (method, remainingArgs, found)
func extractWitnessMethod(args []Object, methodName string) (Object, []Object, bool) {
	if len(args) < 1 {
		return nil, args, false
	}

	dict, ok := args[0].(*Dictionary)
	if !ok {
		return nil, args, false
	}

	// If it's a placeholder dictionary (from Analyzer), strip it even if method not found
	// This ensures fall-back to dynamic lookup in Tree mode
	if dict.TraitName == "$placeholder" {
		return nil, args[1:], true
	}

	if method := FindMethodInDictionary(dict, methodName); method != nil {
		// Found it! Consume the dictionary argument
		return method, args[1:], true
	}

	return nil, args, false
}

// resolveTypeFromEnv resolves type variables in a type using the environment.
// This supports explicit witness passing where generic types are resolved to concrete types
// passed as arguments (or inferred and stored in Env).
func (e *Evaluator) resolveTypeFromEnv(t typesystem.Type, env *Environment) typesystem.Type {
	if env == nil {
		return t
	}
	switch typ := t.(type) {
	case typesystem.TVar:
		// Look for type variable in env
		// We expect type variables to be stored in Env if they are in scope.
		// The key might be the name itself.
		if val, ok := env.Get(typ.Name); ok {
			// If it's a TypeObject, unwrap it
			if typeObj, ok := val.(*TypeObject); ok {
				// Proposal 002 (Step 3): Use Alias if available to preserve nominal typing
				if typeObj.Alias != "" {
					return typesystem.TCon{Name: typeObj.Alias}
				}
				return typeObj.TypeVal
			}
			// If it's a RecordInstance/DataInstance (e.g. Dictionary?), maybe not a type.
		}
		return typ
	case typesystem.TApp:
		newConstructor := e.resolveTypeFromEnv(typ.Constructor, env)
		newArgs := make([]typesystem.Type, len(typ.Args))
		for i, arg := range typ.Args {
			newArgs[i] = e.resolveTypeFromEnv(arg, env)
		}
		// Always return new TApp as comparison of TApp (containing slice) causes panic
		return typesystem.TApp{Constructor: newConstructor, Args: newArgs}
	case typesystem.TFunc:
		newParams := make([]typesystem.Type, len(typ.Params))
		for i, p := range typ.Params {
			newParams[i] = e.resolveTypeFromEnv(p, env)
		}
		newRet := e.resolveTypeFromEnv(typ.ReturnType, env)
		return typesystem.TFunc{
			Params:       newParams,
			ReturnType:   newRet,
			IsVariadic:   typ.IsVariadic,
			DefaultCount: typ.DefaultCount,
		}
	// TTuple, TRecord, etc. should be handled too but TApp/TVar are most important for Witnesses.
	// Adding TTuple/TRecord for completeness.
	case typesystem.TTuple:
		newElements := make([]typesystem.Type, len(typ.Elements))
		for i, el := range typ.Elements {
			newElements[i] = e.resolveTypeFromEnv(el, env)
		}
		return typesystem.TTuple{Elements: newElements}
	default:
		return typ
	}
}

func getKeys(m map[string]Object) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
