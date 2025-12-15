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

// extractTypeConstructorName extracts the type constructor name from a type.
// e.g., Option<Int> → "Option", List<String> → "List", Result<Int, String> → "Result"
func extractTypeConstructorName(t typesystem.Type) string {
	switch typ := t.(type) {
	case typesystem.TCon:
		return typ.Name
	case typesystem.TApp:
		// For TApp, recursively get the constructor
		return extractTypeConstructorName(typ.Constructor)
	default:
		return ""
	}
}

// extractTypeNameFromAST extracts the type name from an AST type annotation.
// Used for dispatch when TypeMap doesn't contain the type.
func extractTypeNameFromAST(t ast.Type) string {
	switch typ := t.(type) {
	case *ast.NamedType:
		return typ.Name.Value
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
	default:
		return string(obj.Type())
	}
}

func (e *Evaluator) resolveCanonicalTypeName(t ast.Type, env *Environment) (string, error) {
	switch t := t.(type) {
	case *ast.NamedType:
		name := t.Name.Value
		if name == "String" {
			return RUNTIME_TYPE_LIST, nil
		}
		// For named types (including record aliases), use the name as-is
		// This preserves nominal typing for trait instances
		// e.g., `type Coord = { x: Int, y: Int }` -> "Coord", not "RECORD"
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
		extendedEnv := NewEnclosedEnvironment(fn.Env)

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

		for {
			result := e.Eval(currentBody, currentEnv)
			result = unwrapReturnValue(result)

			// Error: capture stack trace
			if err, ok := result.(*Error); ok {
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

				nextFn := tc.Func
				nextArgs := tc.Args

				if nextUserFn, ok := nextFn.(*Function); ok {
					nextEnv := NewEnclosedEnvironment(nextUserFn.Env)
					fn = nextUserFn
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
					// Tail call to builtin
					res := e.ApplyFunction(nextFn, nextArgs)
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

			// Set TypeName on RecordInstance if function has a named return type
			if record, ok := result.(*RecordInstance); ok && record.TypeName == "" {
				if fn.ReturnType != nil {
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
		// Note: TypeArgs is not inferred from fields - that would incorrectly treat
		// constructor arguments (e.g., Circle Int) as type parameters.
		// TypeArgs should only be set when the type is actually generic.
		return &DataInstance{Name: fn.Name, Fields: args, TypeName: fn.TypeName}
	case *TypeObject:
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
		// Try to find implementation by checking each argument
		// This supports HKT where the type constructor might not be in the first argument
		// (e.g., Functor.fmap(f, fa) where fa: F<A> is the second argument)
		var foundMethod Object
		var dispatchTypeName string

		if typesMap, ok := e.ClassImplementations[fn.ClassName]; ok {
			for _, arg := range args {
				typeName := getRuntimeTypeName(arg)
				if methodTableObj, ok := typesMap[typeName]; ok {
					if methodTable, ok := methodTableObj.(*MethodTable); ok {
						if method, ok := methodTable.Methods[fn.Name]; ok {
							foundMethod = method
							dispatchTypeName = typeName
							break
						}
					}
				}
			}
		}

		// If no match found from arguments, try to dispatch by ContainerContext
		// This is set by >>= to let `pure` know what container to produce
		if foundMethod == nil && e.ContainerContext != "" {
			if typesMap, ok := e.ClassImplementations[fn.ClassName]; ok {
				if methodTableObj, ok := typesMap[e.ContainerContext]; ok {
					if methodTable, ok := methodTableObj.(*MethodTable); ok {
						if method, ok := methodTable.Methods[fn.Name]; ok {
							foundMethod = method
							dispatchTypeName = e.ContainerContext
						}
					}
				}
			}
		}

		// If no match found from arguments, try to dispatch by expected return type
		// This is crucial for pure/mempty which don't have a container argument
		if foundMethod == nil && e.CurrentCallNode != nil {
			var typeCtorName string

			// First try TypeMap
			if e.TypeMap != nil {
				if expectedType := e.TypeMap[e.CurrentCallNode]; expectedType != nil {
					typeCtorName = extractTypeConstructorName(expectedType)
				}
			}

			// Fallback: check if CurrentCallNode is AssignExpression with AnnotatedType
			if typeCtorName == "" {
				if assign, ok := e.CurrentCallNode.(*ast.AssignExpression); ok && assign.AnnotatedType != nil {
					typeCtorName = extractTypeNameFromAST(assign.AnnotatedType)
				}
			}

			// Fallback: check if CurrentCallNode is AnnotatedExpression
			if typeCtorName == "" {
				if annotated, ok := e.CurrentCallNode.(*ast.AnnotatedExpression); ok && annotated.TypeAnnotation != nil {
					typeCtorName = extractTypeNameFromAST(annotated.TypeAnnotation)
				}
			}

			// Fallback: check if CurrentCallNode is ConstantDeclaration with TypeAnnotation
			if typeCtorName == "" {
				if constant, ok := e.CurrentCallNode.(*ast.ConstantDeclaration); ok && constant.TypeAnnotation != nil {
					typeCtorName = extractTypeNameFromAST(constant.TypeAnnotation)
				}
			}

			if typeCtorName != "" {
				if typesMap, ok := e.ClassImplementations[fn.ClassName]; ok {
					if methodTableObj, ok := typesMap[typeCtorName]; ok {
						if methodTable, ok := methodTableObj.(*MethodTable); ok {
							if method, ok := methodTable.Methods[fn.Name]; ok {
								foundMethod = method
								dispatchTypeName = typeCtorName
							}
						}
					}
				}
			}
		}

		if foundMethod != nil {
			return e.ApplyFunction(foundMethod, args)
		}

		// Determine type name for error message
		if dispatchTypeName == "" && len(args) > 0 {
			dispatchTypeName = getRuntimeTypeName(args[0])
		}
		if dispatchTypeName == "" {
			dispatchTypeName = "unknown"
		}

		// Fallback to trait default implementation
		if e.TraitDefaults != nil {
			key := fn.ClassName + "." + fn.Name
			if fnStmt, ok := e.TraitDefaults[key]; ok {
				// Use GlobalEnv which has trait methods registered
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

		return newError("implementation of class %s for type %s (method %s) not found", fn.ClassName, dispatchTypeName, fn.Name)

	case *BoundMethod:
		newArgs := append([]Object{fn.Receiver}, args...)
		return e.ApplyFunction(fn.Function, newArgs)

	case *OperatorFunction:
		// Operator as function: (+), (-), etc.
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
	// Check if any trait has this method registered
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
