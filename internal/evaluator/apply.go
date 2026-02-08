package evaluator

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/typesystem"
	"reflect"
	"strings"
)

// getRuntimeTypeName helper
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
	case *BoundMethod:
		return RUNTIME_TYPE_FUNCTION
	case *PartialApplication:
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
		// Handle VM types that might not be directly accessible (e.g. VMClosure)
		if string(obj.Type()) == "CLOSURE" {
			return RUNTIME_TYPE_FUNCTION
		}
		return string(obj.Type())
	}
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
		// We use a "$typevar_" prefix so that type parameters are NOT accessible as regular
		// values (preventing leakage of type-level names into the value namespace).
		if callNode, ok := e.CurrentCallNode.(*ast.CallExpression); ok && callNode.Instantiation != nil {
			for name, t := range callNode.Instantiation {
				extendedEnv.Set("$typevar_"+name, &TypeObject{TypeVal: t})
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

				// Restore Witness from TailCall
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

					// Check if nextArgs actually contains dictionaries
					dictCount := 0
					for i := 0; i < len(nextArgs); i++ {
						if _, ok := nextArgs[i].(*Dictionary); ok {
							dictCount++
						} else {
							break
						}
					}

					if witnessCount > 0 && dictCount > 0 {
						if len(nextArgs) < witnessCount {
							e.RestoreWitnessStack(initialWitnessDepth)
							return e.newErrorWithStack("wrong number of arguments: expected at least %d witnesses, got %d", witnessCount, len(nextArgs))
						}
						for i, name := range fn.WitnessParams {
							// Bind witnesses if dictionaries are available. Witnesses typically precede explicit arguments.
							if i < dictCount {
								nextEnv.Set(name, nextArgs[i])
							}
						}
						// Strip witnesses from args
						if dictCount >= witnessCount {
							nextArgs = nextArgs[witnessCount:]
						} else {
							// If fewer dictionaries are provided than expected witnesses, consume available ones.
							// This supports scenarios where some witnesses are applied later or implicitly.
							nextArgs = nextArgs[dictCount:]
						}
					} else if dictCount > 0 {
						// Consume implicit dictionaries even if witnessCount == 0
						nextArgs = nextArgs[dictCount:]
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
						if len(nextArgs) == 0 {
							return e.newErrorWithStack("wrong number of arguments: expected %d, got 0", requiredParams)
						}
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
		if fn.ClassMethod != nil {
			return e.ApplyFunction(fn.ClassMethod, allArgs)
		}
		if fn.VMClosure != nil && e.VMCallHandler != nil {
			return e.VMCallHandler(fn.VMClosure, allArgs)
		}
		return newError("invalid partial application: fn is nil")
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

		// FIX: For MPTC methods (like processPoly), we MUST NOT prematurely strip the dictionary from 'args'
		// if the method expects it (witness param). But Strategy 3 needs to peek at arguments.
		// So we calculate 'hintCheckArgs' which are the logical arguments, but keep 'args' as physical arguments.

		// =========================================================================================
		// 1. Calculate Context Information (Needed for Strategy 3 and Heuristics)
		// =========================================================================================
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
				findCandidate := func(targetType string) Object {
					for key, methodTableObj := range typesMap {
						if key == targetType {
							if methodTable, ok := methodTableObj.(*MethodTable); ok {
								if method, ok := methodTable.Methods[fn.Name]; ok {
									// We can't check args match easily here as args might have dicts
									// But for context candidate, we usually ignore args or check leniently
									return method
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
									return method
								}
							}
						}
					}
					return nil
				}
				contextCandidate = findCandidate(contextTypeName)
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

		// =========================================================================================
		// 2. Helper: Identify actual args (strip dicts) for dispatch strategies
		// =========================================================================================
		hintCheckArgs := args
		for len(hintCheckArgs) > 0 {
			if _, ok := hintCheckArgs[0].(*Dictionary); ok {
				hintCheckArgs = hintCheckArgs[1:]
			} else {
				break
			}
		}

		// =========================================================================================
		// 3. Extract Hint Argument (Needed for Strategy 3)
		// =========================================================================================
		if fn.Arity >= 0 && len(hintCheckArgs) == fn.Arity+1 {
			if typeHint, ok := hintCheckArgs[len(hintCheckArgs)-1].(*TypeObject); ok {
				// It's a Type object hint
				// Use the type name
				dispatchTypeName = ExtractTypeConstructorName(typeHint.TypeVal)
				// Remove hint from args (need to remove from original args too)
				// We assume hint is the LAST argument.
				// NOTE: This modifies 'args' for subsequent strategies!
				// If we have dictionaries, we need to be careful.
				// The hint is at the end.
				args = args[:len(args)-1]
				hintCheckArgs = hintCheckArgs[:len(hintCheckArgs)-1] // Keep consistent
			} else if hintList, ok := hintCheckArgs[len(hintCheckArgs)-1].(*List); ok {
				// String hint (deprecated but supported for simple strings)
				if str := ListToString(hintList); str != "" || hintList.Len() == 0 {
					dispatchTypeName = str
					args = args[:len(args)-1]
					hintCheckArgs = hintCheckArgs[:len(hintCheckArgs)-1]
				}
			}
		}

		// =========================================================================================
		// 4. Strategy: DispatchSources (Priority Strategy)
		// =========================================================================================
		// Use Dispatch Strategy from ClassMethod if available.
		if len(fn.DispatchSources) > 0 {
			// Construct key from sources
			var keyParts []string
			validKey := true

			for _, source := range fn.DispatchSources {
				switch source.Kind {
				case typesystem.DispatchArg:
					if source.Index >= 0 && source.Index < len(hintCheckArgs) {
						keyParts = append(keyParts, e.getDispatchTypeName(hintCheckArgs[source.Index]))
					} else {
						validKey = false
					}
				case typesystem.DispatchReturn:
					if contextTypeName != "" {
						keyParts = append(keyParts, contextTypeName)
					} else {
						validKey = false
					}
				case typesystem.DispatchHint:
					if dispatchTypeName != "" && dispatchTypeName != "unknown" {
						keyParts = append(keyParts, dispatchTypeName)
					} else {
						validKey = false
					}
				default:
					validKey = false
				}
			}

			if validKey {
				// Try to find method with constructed key
				key := strings.Join(keyParts, "_")

				if typesMap, ok := e.ClassImplementations[fn.ClassName]; ok {
					if methodTableObj, ok := typesMap[key]; ok {
						if methodTable, ok := methodTableObj.(*MethodTable); ok {
							if method, ok := methodTable.Methods[fn.Name]; ok {
								// Found via strategy!
								// Push witness if we dispatched via context/TypeMap (crucial for generics)
								// Check if we used DispatchReturn or if hint matches context
								usedContext := false
								if dispatchTypeName == contextTypeName {
									usedContext = true
								} else {
									for _, source := range fn.DispatchSources {
										if source.Kind == typesystem.DispatchReturn {
											usedContext = true
											break
										}
									}
								}

								if expectedType != nil && usedContext {
									e.PushWitness(map[string][]typesystem.Type{fn.ClassName: {expectedType}})
									defer e.PopWitness()
								}
								// Ensure arguments are correctly passed to the resolved method.
								// Note that 'args' may still contain dictionary witnesses which will be
								// handled by the recursive ApplyFunction call.
								return e.ApplyFunction(method, args)
							}
						}
					}
				}
			}
		}

		// =========================================================================================
		// 5. Strategy: Explicit Witness Argument
		// =========================================================================================
		// Loop to strip leading dictionaries and find the method
		var remainingArgs = args
		for len(remainingArgs) > 0 {
			if dict, ok := remainingArgs[0].(*Dictionary); ok {
				if method := FindMethodInDictionary(dict, fn.Name); method != nil {
					// Check if it's actually implemented (not Nil placeholder)
					if _, isNil := method.(*Nil); !isNil {
						// Validate the method matches the runtime arguments
						wantsWitness := false
						if fnObj, ok := method.(*Function); ok && len(fnObj.WitnessParams) > 0 {
							wantsWitness = true
						}

						if wantsWitness {
							if e.checkArgsMatch(method, remainingArgs) {
								return e.ApplyFunction(method, remainingArgs)
							}
						} else {
							if e.checkArgsMatch(method, remainingArgs[1:]) {
								return e.ApplyFunction(method, remainingArgs[1:])
							}
						}
					}
				}
				remainingArgs = remainingArgs[1:]
			} else {
				break
			}
		}

		// Update args to remainingArgs after witness stripping for fallback strategies
		// NOTE: This changes 'args' to 'hintCheckArgs' effectively
		args = remainingArgs

		// =========================================================================================
		// 6. Strategy: Argument-Based Dispatch
		// =========================================================================================
		var argCandidate Object
		var argTypeName string
		var argCandidateIsExact bool

		if typesMap, ok := e.ClassImplementations[fn.ClassName]; ok {
			// Strategy 0a: Exact Key Match (Priority)
			var traitArity int = -1
			for k := range typesMap {
				traitArity = strings.Count(k, "_") + 1
				break
			}

			if traitArity > 0 && len(args) == traitArity {
				var exactKeyParts []string
				for _, arg := range args {
					exactKeyParts = append(exactKeyParts, e.getDispatchTypeName(arg))
				}

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

			// Strategy 0b: Fuzzy Match (Fallback)
			if argCandidate == nil {
				var bestCandidate Object
				bestScore := -1
				bestKey := ""

				contextType := ""
				if len(e.TypeContextStack) > 0 {
					contextType = e.TypeContextStack[len(e.TypeContextStack)-1]
				}

				for key := range typesMap {
					parts := strings.Split(key, "_")
					match := true
					score := 0
					argIdx := 0

					for _, part := range parts {
						isVar := len(part) > 0 && part[0] >= 'a' && part[0] <= 'z'

						// Check if we have an argument for this part
						if argIdx < len(args) {
							arg := args[argIdx]
							argType := e.getDispatchTypeName(arg)

							// 1. Exact Type Match
							if argType == part {
								argIdx++
								score += 2
								continue
							}

							// 2. Variable Match
							if isVar {
								argIdx++
								score += 1
								continue
							}

							// 3. Type Alias Match
							if e.TypeAliases != nil {
								if underlying, ok := e.TypeAliases[argType]; ok {
									if ExtractTypeConstructorName(underlying) == part {
										argIdx++
										score += 2
										continue
									}
								}
							}

							// 4. Constraint/Trait Skip (e.g. "Numeric" in "a_Numeric_b")
							// If the part is a known trait (and didn't match the arg), assume it's a constraint
							// and skip it without consuming an argument.
							if _, isTrait := e.ClassImplementations[part]; isTrait {
								continue
							}

							// Mismatch
							match = false
							break
						} else {
							// No more args.

							// 1. Allow skipping variables (implicit/inferred parameters)
							// This handles cases like UltraConstrained<a,b,c> with 1 arg.
							if isVar {
								continue
							}

							// 2. Check if part matches context (return type)
							if contextType != "" && part == contextType {
								score += 1
								continue
							}

							// 3. If part is a trait, we can skip it
							if _, isTrait := e.ClassImplementations[part]; isTrait {
								continue
							}

							// 4. Relaxed Tail Matching:
							// If we have no more arguments, but we still have key parts,
							// we allow them to be skipped if they don't directly contradict anything.
							// This handles MPTC cases like "Int_String" where we match "Int" (Arg 0)
							// but "String" is a return type/context that we might not have info for.
							// Since Strategy 2 is a fallback/fuzzy match, we accept this partial match.
							continue
						}
					}

					// Ensure we consumed all arguments (unless variadic/permissive)
					// For MPTC, we should be reasonably strict, but let's allow partial for now if it works.
					// Actually, if we didn't consume all args, it's a bad match.
					if match && argIdx < len(args) {
						match = false
					}

					if match {
						if score > bestScore {
							if method, found := e.lookupTraitMethod(fn.ClassName, fn.Name, parts...); found {
								if fn.Arity < 0 || len(args) == fn.Arity {
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

			if argCandidate != nil {
				return e.ApplyFunction(argCandidate, args)
			}
		}

		// Continue with implicit witness and fallbacks...

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
		// Removed to avoid duplication (handled in block 3)

		// Fallback to legacy heuristics if strategy failed or not available
		if foundMethod == nil {
			// Legacy heuristics...
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

			if contextCandidate != nil && contextIsContainer {
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
		}
		// If no candidate found, fall through to trait defaults
		if foundMethod != nil {
			// Push witness if we dispatched via context/TypeMap
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

	case *HostObject:
		if e.HostCallHandler != nil {
			val := reflect.ValueOf(fn.Value)
			if val.Kind() == reflect.Func {
				res, err := e.HostCallHandler(val, args)
				if err != nil {
					return newError("host call error: %v", err)
				}
				return res
			}
		}
		return newError("object %s is not callable", fn.Inspect())

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
