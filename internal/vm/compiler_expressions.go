package vm

import (
	"fmt"
	"hash/fnv"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/typesystem"
	"sort"
	"strings"
)

func (c *Compiler) compileExpression(expr ast.Expression) error {
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		c.emitConstant(&evaluator.Integer{Value: e.Value}, e.Token.Line)
		c.slotCount++
		return nil

	case *ast.FloatLiteral:
		c.emitConstant(&evaluator.Float{Value: e.Value}, e.Token.Line)
		c.slotCount++
		return nil

	case *ast.BigIntLiteral:
		c.emitConstant(&evaluator.BigInt{Value: e.Value}, e.Token.Line)
		c.slotCount++
		return nil

	case *ast.RationalLiteral:
		c.emitConstant(&evaluator.Rational{Value: e.Value}, e.Token.Line)
		c.slotCount++
		return nil

	case *ast.BooleanLiteral:
		if e.Value {
			c.emit(OP_TRUE, e.Token.Line)
		} else {
			c.emit(OP_FALSE, e.Token.Line)
		}
		c.slotCount++
		return nil

	case *ast.StringLiteral:
		// String is List<Char>
		c.emitConstant(evaluator.StringToList(e.Value), e.Token.Line)
		c.slotCount++
		return nil

	case *ast.CharLiteral:
		c.emitConstant(&evaluator.Char{Value: e.Value}, e.Token.Line)
		c.slotCount++
		return nil

	case *ast.NilLiteral:
		c.emit(OP_NIL, e.Token.Line)
		c.slotCount++
		return nil

	case *ast.Identifier:
		return c.compileIdentifier(e)

	case *ast.PrefixExpression:
		return c.compilePrefixExpression(e)

	case *ast.InfixExpression:
		return c.compileInfixExpression(e)

	case *ast.IfExpression:
		return c.compileIfExpression(e)

	case *ast.MatchExpression:
		return c.compileMatchExpression(e)

	case *ast.BlockStatement:
		return c.compileBlockExpression(e)

	case *ast.AssignExpression:
		return c.compileAssignExpression(e)

	case *ast.CallExpression:
		return c.compileCallExpression(e)

	case *ast.ForExpression:
		return c.compileForExpression(e)

	case *ast.ListLiteral:
		return c.compileListLiteral(e)

	case *ast.IndexExpression:
		return c.compileIndexExpression(e)

	case *ast.MemberExpression:
		return c.compileMemberExpression(e)

	case *ast.TupleLiteral:
		return c.compileTupleLiteral(e)

	case *ast.RecordLiteral:
		return c.compileRecordLiteral(e)

	case *ast.MapLiteral:
		return c.compileMapLiteral(e)

	case *ast.FunctionLiteral:
		return c.compileFunctionLiteral(e)

	case *ast.InterpolatedString:
		return c.compileInterpolatedString(e)

	case *ast.FormatStringLiteral:
		return c.compileFormatStringLiteral(e)

	case *ast.BytesLiteral:
		return c.compileBytesLiteral(e)

	case *ast.BitsLiteral:
		return c.compileBitsLiteral(e)

	case *ast.SpreadExpression:
		// SpreadExpression in isolation just evaluates its inner expression
		return c.compileExpression(e.Expression)

	case *ast.PatternAssignExpression:
		return c.compilePatternAssignExpression(e)

	case *ast.PostfixExpression:
		return c.compilePostfixExpression(e)

	case *ast.AnnotatedExpression:
		// Compile inner expression with type context
		typeName := c.resolveTypeName(e.TypeAnnotation)
		err := c.withTypeContext(typeName, func() error {
			return c.compileExpression(e.Expression)
		})
		if err != nil {
			return err
		}

		// Auto-call if nullary method
		line := e.Token.Line
		if typeName != "" {
			typeHintIdx := c.currentChunk().AddConstant(&stringConstant{Value: typeName})
			c.emit(OP_SET_TYPE_CONTEXT, line)
			c.currentChunk().Write(byte(typeHintIdx>>8), line)
			c.currentChunk().Write(byte(typeHintIdx), line)

			c.emit(OP_AUTO_CALL, line)

			c.emit(OP_CLEAR_TYPE_CONTEXT, line)
		}

		// If annotation is List<T>, set element type
		if named, ok := e.TypeAnnotation.(*ast.NamedType); ok {
			if named.Name.Value == "List" && len(named.Args) > 0 {
				if elemNamed, ok := named.Args[0].(*ast.NamedType); ok {
					c.emit(OP_SET_LIST_ELEM_TYPE, line)
					// Resolve element type name
					elemTypeName := c.resolveTypeName(elemNamed)
					elemTypeIdx := c.currentChunk().AddConstant(&stringConstant{Value: elemTypeName})
					c.currentChunk().Write(byte(elemTypeIdx>>8), line)
					c.currentChunk().Write(byte(elemTypeIdx), line)
				}
			}
		}
		return nil

	case *ast.OperatorAsFunction:
		// Operator used as function, e.g., (+), (<|>)
		// Create an OperatorFunction wrapper that will dispatch at runtime
		opFn := &evaluator.OperatorFunction{Operator: e.Operator}
		c.emitConstant(opFn, e.Token.Line)
		c.slotCount++
		return nil

	case *ast.TypeApplicationExpression:
		// If witnesses are present, compile them and bind using _bindWitness
		if len(e.Witnesses) > 0 {
			// Emit _bindWitness
			builtinNameIdx := c.currentChunk().AddConstant(&stringConstant{Value: "_bindWitness"})
			c.emit(OP_GET_GLOBAL, e.Token.Line)
			c.currentChunk().Write(byte(builtinNameIdx>>8), e.Token.Line)
			c.currentChunk().Write(byte(builtinNameIdx), e.Token.Line)
			c.slotCount++

			// Compile Function
			if err := c.compileExpression(e.Expression); err != nil {
				return err
			}

			// Compile Witnesses
			for _, w := range e.Witnesses {
				if err := c.compileExpression(w); err != nil {
					return err
				}
			}

			// Emit Call
			argCount := 1 + len(e.Witnesses) // fn + witnesses
			c.emit(OP_CALL, e.Token.Line)
			c.currentChunk().Write(byte(argCount), e.Token.Line)
			c.slotCount -= argCount // Consumes _bindWitness + fn + witnesses, pushes Result
			return nil
		}

		// Generic type application (erased) - just compile inner expression
		return c.compileExpression(e.Expression)

	case *ast.ListComprehension:
		return c.compileListComprehension(e)

	case *ast.RangeExpression:
		return c.compileRangeExpression(e)

	default:
		return fmt.Errorf("unknown expression type: %T", expr)
	}
}

// Compile range expression: start..end or (start, next)..end
func (c *Compiler) compileRangeExpression(expr *ast.RangeExpression) error {
	line := expr.Token.Line

	// Compile start
	if err := c.compileExpression(expr.Start); err != nil {
		return err
	}

	// Compile next (if present, else Nil)
	if expr.Next != nil {
		if err := c.compileExpression(expr.Next); err != nil {
			return err
		}
	} else {
		c.emit(OP_NIL, line)
		c.slotCount++
	}

	// Compile end
	if err := c.compileExpression(expr.End); err != nil {
		return err
	}

	// Emit OP_RANGE
	c.emit(OP_RANGE, line)
	// Consumes 3 values (Start, Next, End), pushes 1 (Range)
	c.slotCount -= 2

	return nil
}

// Compile function call
func (c *Compiler) compileCallExpression(call *ast.CallExpression) error {
	line := call.Token.Line
	col := call.Token.Column

	// Determine type context: prioritize specific TypeMap info, fallback to propagated context
	var typeContextName string
	if c.typeMap != nil {
		if t, ok := c.typeMap[call]; ok {
			if c.subst != nil {
				t = t.Apply(c.subst)
			}
			typeContextName = evaluator.ExtractTypeConstructorName(t)

			// Special case: List<Char> is String
			// Check if it's actually List<Char>
			if typeContextName == "List" {
				if tApp, ok := t.(typesystem.TApp); ok {
					// Need to handle nested App if curried? List is * -> *. So List<Char> is App(List, Char).
					// If it was App(App(..)), name wouldn't be List.
					// But we should verify args.
					if len(tApp.Args) == 1 {
						if argCon, ok := tApp.Args[0].(typesystem.TCon); ok && argCon.Name == "Char" {
							typeContextName = "String"
						}
					}
				}
			}
		}
	}

	// If TypeMap didn't provide context, use propagated context
	if typeContextName == "" {
		typeContextName = c.typeContext
	}

	// Special handling for default(Type) - calls Default trait
	if ident, ok := call.Function.(*ast.Identifier); ok && ident.Value == "default" {
		if len(call.Arguments) != 1 {
			return fmt.Errorf("default expects 1 argument, got %d", len(call.Arguments))
		}
		// Compile the type argument
		if err := c.compileExpression(call.Arguments[0]); err != nil {
			return err
		}
		c.emit(OP_DEFAULT, line)
		// OP_DEFAULT pops type, pushes default value
		return nil
	}

	// Special handling for extension method calls: obj.method(args)
	// Compiles as: CALL_METHOD(methodName, receiver, args)
	if memberExpr, ok := call.Function.(*ast.MemberExpression); ok {
		methodName := memberExpr.Member.Value

		// Compile the receiver first - clear context as receiver usually doesn't need it
		if err := c.withTypeContext("", func() error {
			return c.compileExpression(memberExpr.Left)
		}); err != nil {
			return err
		}

		// Compile remaining arguments - clear context
		argCount := 0
		for _, arg := range call.Arguments {
			if spread, ok := arg.(*ast.SpreadExpression); ok {
				if err := c.withTypeContext("", func() error {
					return c.compileExpression(spread.Expression)
				}); err != nil {
					return err
				}
				c.emit(OP_SPREAD_ARG, line)
			} else {
				if err := c.withTypeContext("", func() error {
					return c.compileExpression(arg)
				}); err != nil {
					return err
				}
			}
			argCount++
		}

		// Set context if found (JUST BEFORE CALL)
		if typeContextName != "" {
			typeHintIdx := c.currentChunk().AddConstant(&stringConstant{Value: typeContextName})
			c.emit(OP_SET_TYPE_CONTEXT, line)
			c.currentChunk().Write(byte(typeHintIdx>>8), line)
			c.currentChunk().Write(byte(typeHintIdx), line)
		}

		// Emit CALL_METHOD opcode with method name
		nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: methodName})
		c.emit(OP_CALL_METHOD, line)
		c.currentChunk().Write(byte(nameIdx>>8), line)
		c.currentChunk().Write(byte(nameIdx), line)
		c.currentChunk().Write(byte(argCount), line)
		c.slotCount -= argCount // receiver + args consumed, result pushed

		// Clear type context if we set it
		if typeContextName != "" {
			c.emit(OP_CLEAR_TYPE_CONTEXT, line)
		}
		return nil
	}

	// Check if any argument is a spread expression
	hasSpread := false
	for _, arg := range call.Arguments {
		if _, ok := arg.(*ast.SpreadExpression); ok {
			hasSpread = true
			break
		}
	}

	// Compile the function (callee)
	// Check for specialization
	var specializedName string
	var ident *ast.Identifier
	var isIdent bool
	if ident, isIdent = call.Function.(*ast.Identifier); isIdent && len(call.Instantiation) > 0 {
		// Resolve types in instantiation using current substitution (recursive specialization)
		finalInstantiation := make(map[string]typesystem.Type)
		for k, v := range call.Instantiation {
			if c.subst != nil {
				finalInstantiation[k] = v.Apply(c.subst)
			} else {
				finalInstantiation[k] = v
			}
		}

		name, err := c.specialize(ident.Value, finalInstantiation)
		if err == nil {
			specializedName = name
		}
	}

	// Save tail position state - callee is not in tail position
	wasTail := c.inTailPosition
	c.inTailPosition = false
	// Compile function without context (it's the function itself)
	if err := c.withTypeContext("", func() error {
		if specializedName != "" && specializedName != ident.Value {
			// Emit GET_GLOBAL for specialized name
			nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: specializedName})
			c.emit(OP_GET_GLOBAL, line)
			c.currentChunk().Write(byte(nameIdx>>8), line)
			c.currentChunk().Write(byte(nameIdx), line)
			c.slotCount++
			return nil
		}
		return c.compileExpression(call.Function)
	}); err != nil {
		return err
	}

	// Compile arguments (also not in tail position)
	argCount := 0

	// Handle TypeArgs for data constructors (Reified Generics)
	// If this call has TypeArgs, prepend them as TypeObject arguments
	if call.TypeArgs != nil {
		for _, typeArg := range call.TypeArgs {
			typeObj := &evaluator.TypeObject{TypeVal: typeArg}
			c.emitConstant(typeObj, line)
			c.slotCount++
			argCount++
		}
	}

	for _, arg := range call.Arguments {
		if spread, ok := arg.(*ast.SpreadExpression); ok {
			// Spread expression - compile the inner value (tuple/list)
			// Arguments shouldn't inherit the call's return type context
			if err := c.withTypeContext("", func() error {
				return c.compileExpression(spread.Expression)
			}); err != nil {
				return err
			}
			// Mark this as a spread argument
			c.emit(OP_SPREAD_ARG, line)
		} else {
			if err := c.withTypeContext("", func() error {
				return c.compileExpression(arg)
			}); err != nil {
				return err
			}
		}
		argCount++
	}

	// Pass hidden type hint argument for pure/mempty if context is known
	// This allows runtime to dispatch to the correct implementation (e.g. Reader.pure vs List.pure)
	var functionIdent *ast.Identifier
	var explicitTypeArgs []ast.Type

	if ident, ok := call.Function.(*ast.Identifier); ok {
		functionIdent = ident
	} else if typeApp, ok := call.Function.(*ast.TypeApplicationExpression); ok {
		if ident, ok := typeApp.Expression.(*ast.Identifier); ok {
			functionIdent = ident
			explicitTypeArgs = typeApp.TypeArguments
		}
	}

	if functionIdent != nil {
		// Use Dispatch Strategy from SymbolTable if available
		var needsHint bool
		var traitName string
		var methodName string

		// Check if it's a trait method that requires a hint
		if c.symbolTable != nil {
			// Look up the symbol for this identifier
			// If we have resolution map, use it
			foundInResolution := false

			if c.resolutionMap != nil {
				// Use original node if possible, or identifier
				node := call.Function
				if typeApp, ok := call.Function.(*ast.TypeApplicationExpression); ok {
					node = typeApp.Expression // Look up identifier in resolution map
				}

				if sym, ok := c.resolutionMap[node]; ok {
					foundInResolution = true
					if sym.IsTraitMethod {
						if tName, found := c.symbolTable.GetTraitForMethod(functionIdent.Value); found {
							traitName = tName
							methodName = functionIdent.Value
						}
					}
				}
			}

			// Fallback: lookup by name if not found via resolution map
			if !foundInResolution && traitName == "" {
				if tName, found := c.symbolTable.GetTraitForMethod(functionIdent.Value); found {
					traitName = tName
					methodName = functionIdent.Value
				}
			}

			if traitName != "" {
				if sources, found := c.symbolTable.GetTraitMethodDispatch(traitName, methodName); found {
					for _, source := range sources {
						// If any source is DispatchReturn or DispatchHint, we need to pass the type hint
						if source.Kind == typesystem.DispatchReturn || source.Kind == typesystem.DispatchHint {
							needsHint = true
							break
						}
					}
				}
			}
		}

		if needsHint {
			var typeInfo typesystem.Type

			// Priority 1: Explicit type arguments (e.g. getName<Int>)
			if len(explicitTypeArgs) > 0 {
				// Use the first type argument as the hint
				typeInfo = c.astTypeToTypesystemType(explicitTypeArgs[0])
			} else if c.typeMap != nil {
				// Priority 2: Inferred type from TypeMap
				typeInfo = c.typeMap[call]
			}

			if typeInfo != nil {
				if c.subst != nil {
					typeInfo = typeInfo.Apply(c.subst)
				}
				// Emit TypeObject with full type info
				typeObj := &evaluator.TypeObject{TypeVal: typeInfo}
				c.emitConstant(typeObj, line)
				c.slotCount++
				argCount++
			} else if typeContextName != "" {
				// Priority 3: Fallback to context name
				typeObj := &evaluator.TypeObject{TypeVal: typesystem.TCon{Name: typeContextName}}
				c.emitConstant(typeObj, line)
				c.slotCount++
				argCount++
			}
		}
	}

	// Restore tail position for decision
	c.inTailPosition = wasTail

	// Set context if found (JUST BEFORE CALL)
	if typeContextName != "" {
		typeHintIdx := c.currentChunk().AddConstant(&stringConstant{Value: typeContextName})
		c.emit(OP_SET_TYPE_CONTEXT, line)
		c.currentChunk().Write(byte(typeHintIdx>>8), line)
		c.currentChunk().Write(byte(typeHintIdx), line)
	}

	// Emit call instruction with column info for better error messages
	if hasSpread {
		// Use special spread call that unpacks spread args at runtime
		c.emitWithCol(OP_CALL_SPREAD, line, col)
		c.currentChunk().WriteWithCol(byte(argCount), line, col)
	} else if c.inTailPosition && c.funcType == TYPE_FUNCTION {
		c.emitWithCol(OP_TAIL_CALL, line, col)
		c.currentChunk().WriteWithCol(byte(argCount), line, col)
	} else {
		c.emitWithCol(OP_CALL, line, col)
		c.currentChunk().WriteWithCol(byte(argCount), line, col)
	}

	// After call: function and args are consumed, result is pushed
	// Net effect: -(1 + argCount) + 1 = -argCount
	c.slotCount -= argCount

	// Clear type context if we set it
	if typeContextName != "" {
		c.emit(OP_CLEAR_TYPE_CONTEXT, line)
	}

	return nil
}

// resolveTypeName extracts type constructor name from AST type, applying substitution
func (c *Compiler) resolveTypeName(typeExpr ast.Type) string {
	switch t := typeExpr.(type) {
	case *ast.NamedType:
		name := t.Name.Value

		// Apply substitution if available
		if c.subst != nil {
			if sub, ok := c.subst[name]; ok {
				return extractTypeConstructorName(sub)
			}
		}

		// Check for List<Char> -> String alias
		if name == "List" && len(t.Args) == 1 {
			argName := c.resolveTypeName(t.Args[0])
			if argName == "Char" {
				return "String"
			}
		}

		// Special case: Option<String> -> List logic
		if (name == "Option" || name == "Result") && len(t.Args) > 0 {
			argName := c.resolveTypeName(t.Args[0])
			if argName == "String" {
				return "List"
			}
		}

		return name
	default:
		return ""
	}
}

// stringConstant is used internally for global variable names in constants pool
type stringConstant struct {
	Value string
}

// StringPatternParts holds pattern parts for string pattern matching
type StringPatternParts struct {
	Parts []ast.StringPatternPart
}

func (s *StringPatternParts) Type() evaluator.ObjectType   { return "STRING_PATTERN" }
func (s *StringPatternParts) Inspect() string              { return "<string-pattern>" }
func (s *StringPatternParts) RuntimeType() typesystem.Type { return nil }
func (s *StringPatternParts) Hash() uint32                 { return 0 }

func (s *stringConstant) Type() evaluator.ObjectType   { return "STRING_CONST" }
func (s *stringConstant) Inspect() string              { return s.Value }
func (s *stringConstant) RuntimeType() typesystem.Type { return nil }
func (s *stringConstant) Hash() uint32 {
	h := fnv.New32a()
	h.Write([]byte(s.Value))
	return h.Sum32()
}

// Compile identifier (variable access)
func (c *Compiler) compileIdentifier(ident *ast.Identifier) error {
	line := ident.Token.Line

	// First, look for local variable
	if slot := c.resolveLocal(ident.Value); slot != -1 {
		c.emit(OP_GET_LOCAL, line)
		c.currentChunk().Write(byte(slot), line)
		c.slotCount++
		return nil
	}

	// Second, look for upvalue (captured variable from enclosing scope)
	if upvalue := c.resolveUpvalue(ident.Value); upvalue != -1 {
		c.emit(OP_GET_UPVALUE, line)
		c.currentChunk().Write(byte(upvalue), line)
		c.slotCount++
		return nil
	}

	// Global variable
	nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: ident.Value})
	c.emit(OP_GET_GLOBAL, line)
	c.currentChunk().Write(byte(nameIdx>>8), line)
	c.currentChunk().Write(byte(nameIdx), line)
	c.slotCount++
	return nil
}

// Compile assign expression (x = 5)
// Semantics: if variable exists (local, upvalue, or global), update it.
// If not found anywhere, create new variable in current scope (local if in scope, global if top-level).
func (c *Compiler) compileAssignExpression(expr *ast.AssignExpression) error {
	line := expr.Token.Line

	// Check for MemberExpression assignment: record.field = value
	if memberExpr, ok := expr.Left.(*ast.MemberExpression); ok {
		return c.compileMemberAssign(memberExpr, expr.Value, line)
	}

	// Check for IndexExpression assignment: list[i] = value
	if indexExpr, ok := expr.Left.(*ast.IndexExpression); ok {
		return c.compileIndexAssign(indexExpr, expr.Value, line)
	}

	// Get type info from annotation if present
	var typeName string
	var listElemType string
	if expr.AnnotatedType != nil {
		if named, ok := expr.AnnotatedType.(*ast.NamedType); ok {
			// Check if it's List<T> (NamedType with Args)
			if named.Name.Value == "List" && len(named.Args) > 0 {
				if elemNamed, ok := named.Args[0].(*ast.NamedType); ok {
					listElemType = c.resolveTypeName(elemNamed)
				}
			} else {
				// Simple type like Point or generic like Box<Int>
				typeName = named.Name.Value
			}
		}
	}

	// Compile the value - use type hint if annotation is present
	if expr.AnnotatedType != nil {
		typeName := c.resolveTypeName(expr.AnnotatedType)
		// Emit runtime type context so operator dispatch can use expected type
		if typeName != "" {
			typeHintIdx := c.currentChunk().AddConstant(&stringConstant{Value: typeName})
			c.emit(OP_SET_TYPE_CONTEXT, line)
			c.currentChunk().Write(byte(typeHintIdx>>8), line)
			c.currentChunk().Write(byte(typeHintIdx), line)
		}

		// Use withTypeContext to propagate type expectation during compilation
		if err := c.withTypeContext(typeName, func() error {
			return c.compileExpression(expr.Value)
		}); err != nil {
			return err
		}

		if typeName != "" {
			c.emit(OP_CLEAR_TYPE_CONTEXT, line)
		}
	} else {
		// No type annotation, compile normally
		if err := c.compileExpression(expr.Value); err != nil {
			return err
		}
	}

	// If there is a type annotation, try to auto-call if it's a nullary method (e.g. mempty)
	if expr.AnnotatedType != nil {
		typeName := c.resolveTypeName(expr.AnnotatedType)
		if typeName != "" {
			// Set context, auto-call, clear context
			typeHintIdx := c.currentChunk().AddConstant(&stringConstant{Value: typeName})
			c.emit(OP_SET_TYPE_CONTEXT, line)
			c.currentChunk().Write(byte(typeHintIdx>>8), line)
			c.currentChunk().Write(byte(typeHintIdx), line)

			c.emit(OP_AUTO_CALL, line)

			c.emit(OP_CLEAR_TYPE_CONTEXT, line)
		}
	}

	// If type annotation exists and value is a record, set the TypeName
	if typeName != "" {
		// Strip module prefix if present (e.g. "testlib.Point" -> "Point")
		if idx := strings.LastIndex(typeName, "."); idx != -1 {
			typeName = typeName[idx+1:]
		}
		c.emit(OP_SET_TYPE_NAME, line)
		typeNameIdx := c.currentChunk().AddConstant(&stringConstant{Value: typeName})
		c.currentChunk().Write(byte(typeNameIdx>>8), line)
		c.currentChunk().Write(byte(typeNameIdx), line)
	}

	// If type annotation is List<T>, set element type
	if listElemType != "" {
		c.emit(OP_SET_LIST_ELEM_TYPE, line)
		elemTypeIdx := c.currentChunk().AddConstant(&stringConstant{Value: listElemType})
		c.currentChunk().Write(byte(elemTypeIdx>>8), line)
		c.currentChunk().Write(byte(elemTypeIdx), line)
	}

	// Get name from Left (must be identifier)
	ident, ok := expr.Left.(*ast.Identifier)
	if !ok {
		return fmt.Errorf("assignment target must be identifier, got %T", expr.Left)
	}

	name := ident.Value

	// 1. Check if variable exists as local (reassignment)
	if slot := c.resolveLocal(name); slot != -1 {
		// SET_LOCAL uses peek, value stays on stack as result
		c.emit(OP_SET_LOCAL, line)
		c.currentChunk().Write(byte(slot), line)
		return nil
	}

	// 2. Check if it's an upvalue (captured variable from enclosing scope)
	if upvalue := c.resolveUpvalue(name); upvalue != -1 {
		// SET_UPVALUE uses peek, value stays on stack as result
		c.emit(OP_SET_UPVALUE, line)
		c.currentChunk().Write(byte(upvalue), line)
		return nil
	}

	// 3. Check if it's a known global (defined earlier in this script)
	if c.isKnownGlobal(name) {
		// Global variables cannot be mutated from within functions
		if c.funcType == TYPE_FUNCTION {
			return fmt.Errorf("cannot mutate global variable '%s' from within a function", name)
		}

		// SET_GLOBAL uses peek, value stays on stack as result
		nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: name})
		c.emit(OP_SET_GLOBAL, line)
		c.currentChunk().Write(byte(nameIdx>>8), line)
		c.currentChunk().Write(byte(nameIdx), line)
		return nil
	}

	// 4. Not found anywhere - create new variable
	if c.scopeDepth > 0 || c.funcType == TYPE_FUNCTION {
		// New local variable in current scope
		// Value is already on stack at slotCount-1, DUP creates copy for expression result
		slot := c.slotCount - 1
		c.emit(OP_DUP, line)
		c.slotCount++
		c.addLocal(name, slot)
		return nil
	}

	// Global variable (top-level assignment) - register it
	// Stack: [value] -> [value, value] -> SET_GLOBAL uses peek -> [value]
	// No DUP needed: SET_GLOBAL uses peek, value stays as result
	c.registerGlobal(name)
	nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: name})
	c.emit(OP_SET_GLOBAL, line)
	c.currentChunk().Write(byte(nameIdx>>8), line)
	c.currentChunk().Write(byte(nameIdx), line)
	// Value remains on stack as expression result (+1 from compileExpression)
	return nil
}

// isKnownGlobal checks if name is a known global variable
func (c *Compiler) isKnownGlobal(name string) bool {
	// Walk up compiler chain to find the script compiler
	for comp := c; comp != nil; comp = comp.enclosing {
		if comp.funcType == TYPE_SCRIPT && comp.globals != nil {
			if comp.globals[name] {
				return true
			}
		}
	}
	return false
}

// compileMemberAssign compiles record.field = value
func (c *Compiler) compileMemberAssign(member *ast.MemberExpression, value ast.Expression, line int) error {
	// Compile record object
	if err := c.compileExpression(member.Left); err != nil {
		return err
	}

	// Compile value
	if err := c.compileExpression(value); err != nil {
		return err
	}

	// Emit SET_FIELD opcode
	fieldIdx := c.currentChunk().AddConstant(&stringConstant{Value: member.Member.Value})
	c.emit(OP_SET_FIELD, line)
	c.currentChunk().Write(byte(fieldIdx>>8), line)
	c.currentChunk().Write(byte(fieldIdx), line)
	c.slotCount-- // consumes record and value, pushes new record

	// Now store the new record back to the variable
	// If Left is an identifier, store back to that variable
	if ident, ok := member.Left.(*ast.Identifier); ok {
		name := ident.Value

		// Try local first
		if local := c.resolveLocal(name); local != -1 {
			c.emit(OP_SET_LOCAL, line)
			c.currentChunk().Write(byte(local), line)
			return nil
		}

		// Try upvalue
		if upvalue := c.resolveUpvalue(name); upvalue != -1 {
			c.emit(OP_SET_UPVALUE, line)
			c.currentChunk().Write(byte(upvalue), line)
			return nil
		}

		// Global
		globalIdx := c.currentChunk().AddConstant(&stringConstant{Value: name})
		c.emit(OP_SET_GLOBAL, line)
		c.currentChunk().Write(byte(globalIdx>>8), line)
		c.currentChunk().Write(byte(globalIdx), line)
	}

	return nil
}

// compileIndexAssign compiles list[i] = value
func (c *Compiler) compileIndexAssign(indexExpr *ast.IndexExpression, value ast.Expression, line int) error {
	// Compile collection
	if err := c.compileExpression(indexExpr.Left); err != nil {
		return err
	}

	// Compile index
	if err := c.compileExpression(indexExpr.Index); err != nil {
		return err
	}

	// Compile value
	if err := c.compileExpression(value); err != nil {
		return err
	}

	// Emit SET_INDEX opcode
	c.emit(OP_SET_INDEX, line)
	c.slotCount -= 2 // consumes collection, index, value; pushes new collection

	return nil
}

// registerGlobal registers a global variable name
func (c *Compiler) registerGlobal(name string) {
	// Only script compiler tracks globals
	if c.funcType == TYPE_SCRIPT && c.globals != nil {
		c.globals[name] = true
	}
}

// Compile constant declaration (x :- 5 or x: Type :- value)
func (c *Compiler) compileConstantDeclaration(stmt *ast.ConstantDeclaration) error {
	// Compile value with type context if annotation is present
	if stmt.TypeAnnotation != nil {
		typeName := c.resolveTypeName(stmt.TypeAnnotation)
		// Use withTypeContext to propagate type expectation
		if err := c.withTypeContext(typeName, func() error {
			return c.compileExpression(stmt.Value)
		}); err != nil {
			return err
		}
	} else {
		// No type annotation, compile normally
		if err := c.compileExpression(stmt.Value); err != nil {
			return err
		}
	}

	line := stmt.Token.Line

	// If there is a type annotation, try to auto-call if it's a nullary method (e.g. mempty)
	if stmt.TypeAnnotation != nil {
		typeName := c.resolveTypeName(stmt.TypeAnnotation)
		if typeName != "" {
			// Set context, auto-call, clear context
			typeHintIdx := c.currentChunk().AddConstant(&stringConstant{Value: typeName})
			c.emit(OP_SET_TYPE_CONTEXT, line)
			c.currentChunk().Write(byte(typeHintIdx>>8), line)
			c.currentChunk().Write(byte(typeHintIdx), line)

			c.emit(OP_AUTO_CALL, line)

			c.emit(OP_CLEAR_TYPE_CONTEXT, line)
		}
	}

	// If there is a type annotation, set the type name on the value (for records/lists)
	// This ensures extension method lookup works for type aliases
	if stmt.TypeAnnotation != nil {
		var typeName string
		switch t := stmt.TypeAnnotation.(type) {
		case *ast.NamedType:
			typeName = t.Name.Value
		}

		if typeName != "" {
			// Strip module prefix if present (e.g. "testlib.Point" -> "Point")
			// This ensures the type name matches the definition in the module
			if idx := strings.LastIndex(typeName, "."); idx != -1 {
				typeName = typeName[idx+1:]
			}
			nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: typeName})
			c.emit(OP_SET_TYPE_NAME, line)
			c.currentChunk().Write(byte(nameIdx>>8), line)
			c.currentChunk().Write(byte(nameIdx), line)
		}
	}

	// Handle pattern bindings
	if stmt.Pattern != nil {
		// Value is on stack at c.slotCount - 1
		slotsBeforeBinding := c.slotCount

		if err := c.compilePatternBinding(stmt.Pattern, line); err != nil {
			return err
		}

		// Pattern binding might have added locals on top of value.
		// We want to remove the original value (which is now buried).
		// If locals were added, use OP_POP_BELOW.
		// If no locals were added, just pop.

		bindingsAdded := c.slotCount - slotsBeforeBinding
		if bindingsAdded > 0 {
			c.emit(OP_POP_BELOW, line)
			c.currentChunk().Write(byte(bindingsAdded), line)
			// Adjust slots of locals we kept
			valueSlot := slotsBeforeBinding - 1
			c.removeSlotFromStack(valueSlot)
		} else {
			c.emit(OP_POP, line)
			c.slotCount--
		}

		return nil
	}

	// Get name from Name
	name := stmt.Name.Value

	// Check if this is a local variable
	if c.scopeDepth > 0 || c.funcType == TYPE_FUNCTION {
		slot := c.slotCount - 1
		c.emit(OP_DUP, line)
		c.slotCount++
		c.addLocal(name, slot)
		return nil
	}

	// Global variable
	// Value is already on stack (from compileExpression)
	// OP_SET_GLOBAL sets the variable to the value on top of stack
	// and leaves the value on the stack (peek).
	nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: name})
	c.emit(OP_SET_GLOBAL, line)
	c.currentChunk().Write(byte(nameIdx>>8), line)
	c.currentChunk().Write(byte(nameIdx), line)
	return nil
}

// compilePatternBinding handles destructuring patterns like (a, b) = tuple
func (c *Compiler) compilePatternBinding(pattern ast.Pattern, line int) error {
	switch p := pattern.(type) {
	case *ast.TuplePattern:
		// Value to match is on stack at matchValSlot
		matchValSlot := c.slotCount - 1

		for i, elem := range p.Elements {
			// Get match value (Tuple)
			c.emit(OP_GET_LOCAL, line)
			c.currentChunk().Write(byte(matchValSlot), line)
			c.slotCount++

			// Get element i from tuple
			c.emitConstant(&evaluator.Integer{Value: int64(i)}, line)
			c.slotCount++
			c.emit(OP_GET_TUPLE_ELEM, line)
			c.slotCount-- // index consumed

			// Bind to pattern element
			if ident, ok := elem.(*ast.IdentifierPattern); ok {
				if ident.Value != "_" {
					if c.scopeDepth > 0 || c.funcType == TYPE_FUNCTION {
						slot := c.slotCount - 1
						c.addLocal(ident.Value, slot)
					} else {
						nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: ident.Value})
						c.emit(OP_SET_GLOBAL, line)
						c.currentChunk().Write(byte(nameIdx>>8), line)
						c.currentChunk().Write(byte(nameIdx), line)
						c.emit(OP_POP, line)
						c.slotCount--
					}
				} else {
					c.emit(OP_POP, line)
					c.slotCount--
				}
			} else {
				// Nested pattern
				slotsBeforeNested := c.slotCount
				if err := c.compilePatternBinding(elem, line); err != nil {
					return err
				}
				// Clean up the intermediate value
				bindingsAdded := c.slotCount - slotsBeforeNested
				if bindingsAdded > 0 {
					c.emit(OP_POP_BELOW, line)
					c.currentChunk().Write(byte(bindingsAdded), line)
					// Adjust slots of locals we kept
					valueSlot := slotsBeforeNested - 1
					c.removeSlotFromStack(valueSlot)
				} else {
					c.emit(OP_POP, line)
					c.slotCount--
				}
			}
		}
		return nil

	case *ast.ListPattern:
		// Value to match is on stack at matchValSlot
		matchValSlot := c.slotCount - 1

		for i, elem := range p.Elements {
			// Get match value (List)
			c.emit(OP_GET_LOCAL, line)
			c.currentChunk().Write(byte(matchValSlot), line)
			c.slotCount++

			// Push index and get element
			idxConst := c.currentChunk().AddConstant(&evaluator.Integer{Value: int64(i)})
			c.emit(OP_CONST, line)
			c.currentChunk().Write(byte(idxConst>>8), line)
			c.currentChunk().Write(byte(idxConst), line)
			c.slotCount++
			c.emit(OP_GET_INDEX, line)
			c.slotCount--

			if ident, ok := elem.(*ast.IdentifierPattern); ok {
				if ident.Value != "_" {
					if c.scopeDepth > 0 || c.funcType == TYPE_FUNCTION {
						slot := c.slotCount - 1
						c.addLocal(ident.Value, slot)
					} else {
						nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: ident.Value})
						c.emit(OP_SET_GLOBAL, line)
						c.currentChunk().Write(byte(nameIdx>>8), line)
						c.currentChunk().Write(byte(nameIdx), line)
						c.emit(OP_POP, line)
						c.slotCount--
					}
				} else {
					c.emit(OP_POP, line)
					c.slotCount--
				}
			} else {
				// Nested pattern
				slotsBeforeNested := c.slotCount
				if err := c.compilePatternBinding(elem, line); err != nil {
					return err
				}
				// Clean up the intermediate value
				bindingsAdded := c.slotCount - slotsBeforeNested
				if bindingsAdded > 0 {
					c.emit(OP_POP_BELOW, line)
					c.currentChunk().Write(byte(bindingsAdded), line)
					// Adjust slots of locals we kept
					valueSlot := slotsBeforeNested - 1
					c.removeSlotFromStack(valueSlot)
				} else {
					c.emit(OP_POP, line)
					c.slotCount--
				}
			}
		}
		return nil

	case *ast.RecordPattern:
		// Value to match is on stack at matchValSlot
		matchValSlot := c.slotCount - 1

		// Sort keys for deterministic compilation
		keys := make([]string, 0, len(p.Fields))
		for k := range p.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, fieldName := range keys {
			fieldPattern := p.Fields[fieldName]

			// Get match value (Record)
			c.emit(OP_GET_LOCAL, line)
			c.currentChunk().Write(byte(matchValSlot), line)
			c.slotCount++

			// Get field
			nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: fieldName})
			c.emit(OP_GET_FIELD, line)
			c.currentChunk().Write(byte(nameIdx>>8), line)
			c.currentChunk().Write(byte(nameIdx), line)
			// Stack: [..., record, fieldValue]

			// Bind fieldValue
			if ident, ok := fieldPattern.(*ast.IdentifierPattern); ok {
				if ident.Value != "_" {
					if c.scopeDepth > 0 || c.funcType == TYPE_FUNCTION {
						slot := c.slotCount - 1
						c.addLocal(ident.Value, slot)
					} else {
						nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: ident.Value})
						c.emit(OP_SET_GLOBAL, line)
						c.currentChunk().Write(byte(nameIdx>>8), line)
						c.currentChunk().Write(byte(nameIdx), line)
						c.emit(OP_POP, line)
						c.slotCount--
					}
				} else {
					c.emit(OP_POP, line)
					c.slotCount--
				}
			} else {
				// Nested pattern
				slotsBeforeNested := c.slotCount
				if err := c.compilePatternBinding(fieldPattern, line); err != nil {
					return err
				}
				// Clean up the intermediate value
				bindingsAdded := c.slotCount - slotsBeforeNested
				if bindingsAdded > 0 {
					c.emit(OP_POP_BELOW, line)
					c.currentChunk().Write(byte(bindingsAdded), line)
					// Adjust slots of locals we kept
					valueSlot := slotsBeforeNested - 1
					c.removeSlotFromStack(valueSlot)
				} else {
					c.emit(OP_POP, line)
					c.slotCount--
				}
			}
		}
		return nil

	case *ast.IdentifierPattern:
		if p.Value != "_" {
			if c.scopeDepth > 0 || c.funcType == TYPE_FUNCTION {
				slot := c.slotCount - 1
				c.addLocal(p.Value, slot)
			} else {
				nameIdx := c.currentChunk().AddConstant(&stringConstant{Value: p.Value})
				c.emit(OP_SET_GLOBAL, line)
				c.currentChunk().Write(byte(nameIdx>>8), line)
				c.currentChunk().Write(byte(nameIdx), line)
			}
		}
		return nil

	case *ast.WildcardPattern:
		// Wildcard (_) matches anything but doesn't bind.
		// Value remains on stack (consistent with other patterns).
		return nil

	default:
		return fmt.Errorf("ERROR at %d:%d: unsupported pattern type in binding: %T", line, pattern.GetToken().Column, pattern)
	}
}

// Compile block statement (not expression)
func (c *Compiler) compileBlockStatement(block *ast.BlockStatement) error {
	c.beginScope()

	// Predeclare local functions to support mutual recursion within the block.
	for _, stmt := range block.Statements {
		fs, ok := stmt.(*ast.FunctionStatement)
		if !ok || fs == nil || fs.Receiver != nil {
			continue
		}
		if c.resolveLocal(fs.Name.Value) != -1 {
			continue
		}
		line := fs.Token.Line
		c.emit(OP_NIL, line)
		c.slotCount++
		c.addLocal(fs.Name.Value, c.slotCount-1)
	}

	for _, stmt := range block.Statements {
		localsBefore := c.localCount
		slotsBefore := c.slotCount

		if err := c.compileStatement(stmt); err != nil {
			return err
		}

		// Pop all results (statement block is void)
		localsAdded := c.localCount - localsBefore
		slotsAdded := c.slotCount - slotsBefore
		resultsAdded := slotsAdded - localsAdded

		for k := 0; k < resultsAdded; k++ {
			c.emit(OP_POP, 0)
			c.slotCount--
		}
	}

	c.endScope(block.Token.Line)
	return nil
}

// Compile block as expression (returns last value)
func (c *Compiler) compileBlockExpression(block *ast.BlockStatement) error {
	localsBeforeScope := c.localCount
	slotsBeforeScope := c.slotCount
	c.beginScope()

	// Save tail position - only last statement in block inherits it
	wasTail := c.inTailPosition

	for i, stmt := range block.Statements {
		isLast := i == len(block.Statements)-1

		// Only last statement in block is in tail position (if block was)
		c.inTailPosition = wasTail && isLast

		localsBeforeStmt := c.localCount
		slotsBeforeStmt := c.slotCount

		// Clear context for non-final statements
		if !isLast {
			if err := c.withTypeContext("", func() error {
				return c.compileStatement(stmt)
			}); err != nil {
				return err
			}
		} else {
			// Propagate context to last statement
			if err := c.compileStatement(stmt); err != nil {
				return err
			}
		}

		localsAdded := c.localCount - localsBeforeStmt
		slotsAdded := c.slotCount - slotsBeforeStmt
		resultsAdded := slotsAdded - localsAdded

		if !isLast {
			// Pop intermediate values
			for k := 0; k < resultsAdded; k++ {
				c.emit(OP_POP, 0)
				c.slotCount--
			}
		} else {
			// Last statement: ensure exactly one result
			if resultsAdded == 0 {
				c.emit(OP_NIL, stmt.GetToken().Line)
				c.slotCount++
			} else if resultsAdded > 1 {
				for k := 0; k < resultsAdded-1; k++ {
					c.emit(OP_POP, 0)
					c.slotCount--
				}
			}
		}
	}

	// Restore tail position
	c.inTailPosition = wasTail
	_ = slotsBeforeScope // silence unused warning

	// If block is empty, push nil
	if len(block.Statements) == 0 {
		c.emit(OP_NIL, block.Token.Line)
		c.slotCount++
	}

	// Emit CLOSE_SCOPE to remove locals but keep result
	line := block.Token.Line
	localsAddedInScope := c.localCount - localsBeforeScope
	if localsAddedInScope > 0 {
		c.emit(OP_CLOSE_SCOPE, line)
		c.currentChunk().Write(byte(localsAddedInScope), line)
	}

	// Update compiler state
	c.scopeDepth--
	c.localCount = localsBeforeScope
	// We expect exactly 1 result to remain on stack after block expression
	// If slotCount was N, we added 1 result.
	// But slotCount tracks current stack.
	// We popped all locals (via CLOSE_SCOPE logic conceptually, but slotCount must reflect stack state).
	// CLOSE_SCOPE pops 1 result, pops N locals, pushes 1 result.
	// Net change to slots: -N.
	// So c.slotCount should decrease by localsAddedInScope.
	c.slotCount -= localsAddedInScope

	return nil
}

// Compile prefix expression
func (c *Compiler) compilePrefixExpression(expr *ast.PrefixExpression) error {
	// Operand of a prefix expression is NOT in tail position because
	// the result is used by the operator (e.g. !f(x) must apply NOT
	// after the call returns, so f(x) must not be a tail call).
	wasTail := c.inTailPosition
	c.inTailPosition = false

	if err := c.compileExpression(expr.Right); err != nil {
		return err
	}

	c.inTailPosition = wasTail

	switch expr.Operator {
	case "-":
		c.emit(OP_NEG, expr.Token.Line)
	case "!":
		c.emit(OP_NOT, expr.Token.Line)
	case "~":
		c.emit(OP_BNOT, expr.Token.Line)
	default:
		return fmt.Errorf("unknown prefix operator: %s", expr.Operator)
	}
	return nil
}

// Compile infix expression
func (c *Compiler) compileInfixExpression(expr *ast.InfixExpression) error {
	if expr.Operator == "&&" || expr.Operator == "||" {
		return c.compileLogicalOp(expr)
	}

	// Pipe operator: x |> f  compiles to f(x)
	if expr.Operator == "|>" {
		return c.compilePipeOp(expr)
	}

	// Pipe + unwrap operator: x |>> f  compiles to unwrap(f(x))
	if expr.Operator == "|>>" {
		return c.compilePipeUnwrapOp(expr)
	}

	// Function application: f $ x compiles to f(x)
	if expr.Operator == "$" {
		return c.compileApplyOp(expr)
	}

	// Function composition: f ,, g compiles to composed function
	if expr.Operator == ",," {
		return c.compileComposeOp(expr)
	}

	// Null coalescing operator ?? with short-circuit evaluation
	if expr.Operator == "??" {
		return c.compileCoalesceOp(expr)
	}

	// Operands of infix expressions are NOT in tail position
	// because the result is used in the operation
	wasTail := c.inTailPosition
	c.inTailPosition = false

	if err := c.compileExpression(expr.Left); err != nil {
		return err
	}

	if err := c.compileExpression(expr.Right); err != nil {
		return err
	}

	c.inTailPosition = wasTail

	line := expr.Token.Line
	switch expr.Operator {
	case "+":
		c.emit(OP_ADD, line)
	case "-":
		c.emit(OP_SUB, line)
	case "*":
		c.emit(OP_MUL, line)
	case "/":
		c.emit(OP_DIV, line)
	case "%":
		c.emit(OP_MOD, line)
	case "**":
		c.emit(OP_POW, line)
	case "++":
		c.emit(OP_CONCAT, line)
	case "::":
		c.emit(OP_CONS, line)
	case "&":
		c.emit(OP_BAND, line)
	case "|":
		c.emit(OP_BOR, line)
	case "^":
		c.emit(OP_BXOR, line)
	case "<<":
		c.emit(OP_LSHIFT, line)
	case ">>":
		c.emit(OP_RSHIFT, line)
	case "==":
		c.emit(OP_EQ, line)
	case "!=":
		c.emit(OP_NE, line)
	case "<":
		c.emit(OP_LT, line)
	case "<=":
		c.emit(OP_LE, line)
	case ">":
		c.emit(OP_GT, line)
	case ">=":
		c.emit(OP_GE, line)
	default:
		// All other operators (trait-based, user-defined) - dispatch through evaluator
		opIdx := c.currentChunk().AddConstant(&stringConstant{Value: expr.Operator})
		c.emit(OP_TRAIT_OP, line)
		c.currentChunk().Write(byte(opIdx>>8), line)
		c.currentChunk().Write(byte(opIdx), line)
	}
	c.slotCount--
	return nil
}

// Compile logical operators with short-circuit evaluation
func (c *Compiler) compileLogicalOp(expr *ast.InfixExpression) error {
	if err := c.compileExpression(expr.Left); err != nil {
		return err
	}

	line := expr.Token.Line

	if expr.Operator == "&&" {
		jumpAddr := c.emitJump(OP_JUMP_IF_FALSE, line)
		c.emit(OP_POP, line)
		c.slotCount--
		if err := c.compileExpression(expr.Right); err != nil {
			return err
		}
		c.patchJump(jumpAddr)
	} else {
		elseJump := c.emitJump(OP_JUMP_IF_FALSE, line)
		endJump := c.emitJump(OP_JUMP, line)
		c.patchJump(elseJump)
		c.emit(OP_POP, line)
		c.slotCount--
		if err := c.compileExpression(expr.Right); err != nil {
			return err
		}
		c.patchJump(endJump)
	}
	return nil
}

// Compile pipe operator: x |> f → f(x)
//
// Evaluation order: LEFT side is always evaluated FIRST (left-to-right),
// matching TreeWalk semantics and pipe intuition (data flows left-to-right).
// Since the VM calling convention requires [fn, arg1, ...argN] on the stack,
// we use OP_SWAP (simple pipe) or a hidden temp local (call pipe) to
// rearrange after compiling in the correct eval order.
func (c *Compiler) compilePipeOp(expr *ast.InfixExpression) error {
	wasTail := c.inTailPosition
	c.inTailPosition = false
	line := expr.Token.Line

	// Check if right side is a call expression: x |> f(a, b) -> f(a, b, x)
	if call, ok := expr.Right.(*ast.CallExpression); ok {
		// Call-expression pipe: left value goes at placeholder or end of args.
		// We evaluate left FIRST, store in a hidden temp local, then compile
		// function + args normally, and load temp at the right position.
		//
		// Stack lifecycle:
		//   compile left → [pipe_val]
		//   (pipe_val becomes hidden local at tempSlot)
		//   compile f    → [pipe_val, f]
		//   compile args → [pipe_val, f, a1, ..., aN]
		//   GET_LOCAL    → [pipe_val, f, a1, ..., aN, pipe_val_copy]
		//   OP_CALL N+1  → [pipe_val, result]
		//   POP_BELOW 1  → [result]

		// 1. Evaluate left first (correct left-to-right order)
		if err := c.compileExpression(expr.Left); err != nil {
			return err
		}
		// stack: [..., pipe_val], pipe_val is at slotCount-1

		// 2. Register as hidden temp local
		tempSlot := c.slotCount - 1
		c.beginScope()
		c.addLocal(" pipe", tempSlot)

		// 3. Compile function
		if err := c.compileExpression(call.Function); err != nil {
			return err
		}

		// 4. Compile arguments, inserting piped value at placeholder or end
		placeholderFound := false
		for _, arg := range call.Arguments {
			if ident, ok := arg.(*ast.Identifier); ok && ident.Value == "_" {
				if !placeholderFound {
					c.emit(OP_GET_LOCAL, line)
					c.currentChunk().Write(byte(tempSlot), line)
					c.slotCount++
					placeholderFound = true
				} else {
					return fmt.Errorf("multiple placeholders in pipe expression not supported")
				}
			} else {
				if err := c.compileExpression(arg); err != nil {
					return err
				}
			}
		}

		if !placeholderFound {
			// Append piped value as last argument
			c.emit(OP_GET_LOCAL, line)
			c.currentChunk().Write(byte(tempSlot), line)
			c.slotCount++
		}

		// 5. Call (no tail call — hidden temp needs cleanup after)
		argCount := len(call.Arguments)
		if !placeholderFound {
			argCount++
		}
		c.emit(OP_CALL, line)
		c.currentChunk().Write(byte(argCount), line)
		c.slotCount -= argCount

		// 6. Clean up hidden temp: stack is [..., pipe_val, result]
		c.emit(OP_POP_BELOW, line)
		c.currentChunk().Write(byte(1), line) // keep 1 (result), remove pipe_val below
		c.slotCount--

		c.endScopeNoEmit()

		c.inTailPosition = wasTail
		return nil
	}

	// Simple pipe: x |> f becomes f(x)
	// 1. Compile LEFT first (left-to-right eval order)
	if err := c.compileExpression(expr.Left); err != nil {
		return err
	}

	// 2. Compile RIGHT (the function)
	if err := c.compileExpression(expr.Right); err != nil {
		return err
	}

	// 3. Swap to get calling convention order: [left, f] → [f, left]
	c.emit(OP_SWAP, line)

	// 4. Call
	if wasTail && c.funcType == TYPE_FUNCTION {
		c.emit(OP_TAIL_CALL, line)
	} else {
		c.emit(OP_CALL, line)
	}
	c.currentChunk().Write(byte(1), line)
	c.slotCount-- // fn+arg consumed, result pushed. Net: -1

	c.inTailPosition = wasTail
	return nil
}

// compilePipeUnwrapOp compiles x |>> f as unwrap(f(x))
// Reuses pipe compilation logic, then emits OP_UNWRAP_OR_PANIC
func (c *Compiler) compilePipeUnwrapOp(expr *ast.InfixExpression) error {
	// Disable tail position: the pipe call is NOT the final operation —
	// we still need OP_UNWRAP_OR_PANIC after it. Without this, compilePipeOp
	// emits OP_TAIL_CALL which returns immediately, skipping the unwrap.
	savedTail := c.inTailPosition
	c.inTailPosition = false

	// Compile as normal pipe first
	if err := c.compilePipeOp(expr); err != nil {
		return err
	}

	c.inTailPosition = savedTail
	// Then unwrap the result (panic on Fail/None, pass through otherwise)
	c.emit(OP_UNWRAP_OR_PANIC, expr.Token.Line)
	return nil
}

// compileApplyOp compiles f $ x as f(x)
func (c *Compiler) compileApplyOp(expr *ast.InfixExpression) error {
	// Compile function first
	if err := c.compileExpression(expr.Left); err != nil {
		return err
	}

	// Compile argument
	if err := c.compileExpression(expr.Right); err != nil {
		return err
	}

	line := expr.Token.Line
	c.emit(OP_CALL, line)
	c.currentChunk().Write(byte(1), line) // 1 argument
	c.slotCount--                         // call consumes fn+arg, pushes result

	return nil
}

// compileComposeOp compiles f ,, g as composed function
func (c *Compiler) compileComposeOp(expr *ast.InfixExpression) error {
	// Compile both functions
	if err := c.compileExpression(expr.Left); err != nil {
		return err
	}
	if err := c.compileExpression(expr.Right); err != nil {
		return err
	}

	line := expr.Token.Line
	c.emit(OP_COMPOSE, line)
	c.slotCount-- // consumes 2 fns, pushes 1 composed fn

	return nil
}

// Compile null coalescing operator: x ?? default
// Some(v) ?? d = v, None ?? d = d
// Ok(v) ?? d = v, Fail(_) ?? d = d
func (c *Compiler) compileCoalesceOp(expr *ast.InfixExpression) error {
	line := expr.Token.Line

	// Compile left (the Option/Result value)
	if err := c.compileExpression(expr.Left); err != nil {
		return err
	}

	// Emit OP_COALESCE which checks if value isEmpty
	// If empty, jump to default; otherwise unwrap
	c.emit(OP_COALESCE, line)
	c.slotCount++
	c.slotCount++ // OP_COALESCE pushes a bool
	jumpIfEmpty := c.emitJump(OP_JUMP_IF_FALSE, line)
	c.emit(OP_POP, line)
	c.slotCount--

	// Not empty - value is already unwrapped on stack, skip default
	skipDefault := c.emitJump(OP_JUMP, line)

	// Empty - pop bool and compile default
	c.patchJump(jumpIfEmpty)
	c.emit(OP_POP, line) // pop bool
	c.emit(OP_POP, line) // pop the empty Option
	c.slotCount -= 2
	if err := c.compileExpression(expr.Right); err != nil {
		return err
	}

	c.patchJump(skipDefault)
	return nil
}

// Compile if expression
func (c *Compiler) compileIfExpression(expr *ast.IfExpression) error {
	// Condition is not in tail position, and should not inherit type context
	wasTail := c.inTailPosition
	c.inTailPosition = false
	if err := c.withTypeContext("", func() error {
		return c.compileExpression(expr.Condition)
	}); err != nil {
		return err
	}

	line := expr.Token.Line
	slotsBefore := c.slotCount - 1

	thenJump := c.emitJump(OP_JUMP_IF_FALSE, line)
	c.emit(OP_POP, line)
	c.slotCount--

	// Consequence is in tail position if the if expression is
	c.inTailPosition = wasTail
	if err := c.compileBlockExpression(expr.Consequence); err != nil {
		return err
	}

	elseJump := c.emitJump(OP_JUMP, line)
	c.patchJump(thenJump)

	c.slotCount = slotsBefore + 1
	c.emit(OP_POP, line)
	c.slotCount--

	// Alternative is also in tail position if the if expression is
	c.inTailPosition = wasTail
	if expr.Alternative != nil {
		if err := c.compileBlockExpression(expr.Alternative); err != nil {
			return err
		}
	} else {
		c.emit(OP_NIL, line)
		c.slotCount++
	}

	c.patchJump(elseJump)
	c.slotCount = slotsBefore + 1
	c.inTailPosition = wasTail
	return nil
}

// Compile match expression
func (c *Compiler) compileMatchExpression(expr *ast.MatchExpression) error {
	line := expr.Token.Line

	// Compile the matched value - clear context as it's the subject, not result
	// Also clear tail position - the subject is not a tail call even if match is!
	wasTail := c.inTailPosition
	c.inTailPosition = false
	if err := c.withTypeContext("", func() error {
		return c.compileExpression(expr.Expression)
	}); err != nil {
		return err
	}
	c.inTailPosition = wasTail
	// Value is now on stack

	slotsBefore := c.slotCount - 1 // excluding the matched value
	endJumps := []int{}

	for armIdx, arm := range expr.Arms {
		// Track how many extra slots pattern check added (for cleanup on failure)
		slotsAtArmStart := c.slotCount

		c.beginScope()

		// Stack state: [matched_value]
		// Compile pattern - this checks if pattern matches and creates bindings
		// Pattern leaves matched_value on stack, may add bindings as locals
		failJump, err := c.compilePatternCheck(arm.Pattern, line)
		if err != nil {
			return err
		}

		// Track slots after pattern (includes bindings)
		slotsAfterPattern := c.slotCount

		// Compile guard if present - should be boolean, no context
		var guardJump int = -1
		if arm.Guard != nil {
			if err := c.withTypeContext("", func() error {
				return c.compileExpression(arm.Guard)
			}); err != nil {
				return err
			}
			guardJump = c.emitJump(OP_JUMP_IF_FALSE, line)
			c.emit(OP_POP, line)
			c.slotCount--
		}

		// Compile the arm body (matched value still on stack as binding)
		if err := c.compileExpression(arm.Expression); err != nil {
			return err
		}
		// Stack: [matched_value, result]

		// Close scope and remove matched value + any bindings, keeping result
		// CLOSE_SCOPE pops result, removes n slots, pushes result back
		c.endScopeNoEmit() // don't emit POPs, CLOSE_SCOPE handles cleanup
		// Calculate how many slots to remove: everything between slotsBefore and result
		// slotsBefore = before matched_value was pushed
		// c.slotCount = includes result
		// Need to remove: matched_value + any pattern bindings
		slotsToRemove := c.slotCount - slotsBefore - 1
		if slotsToRemove < 0 {
			slotsToRemove = 0
		}
		c.emit(OP_CLOSE_SCOPE, line)
		c.currentChunk().Write(byte(slotsToRemove), line)
		c.slotCount = slotsBefore + 1 // just result on stack

		// Jump to end of match
		endJumps = append(endJumps, c.emitJump(OP_JUMP, line))

		// --- Failure path cleanup ---
		// Guard failure and pattern failure have DIFFERENT stack states!
		// Guard failure: [matched_value, ...bindings, guard_result(false)]
		// Pattern failure: [matched_value] (pattern did its own cleanup)

		var guardCleanupJump int = -1
		if guardJump >= 0 {
			c.patchJump(guardJump)
			// Guard failure: stack has [matched_value, ...bindings, guard_result(false)]
			c.emit(OP_POP, line) // pop guard result
			// Pop bindings to return to state with just matched_value
			extraBindings := slotsAfterPattern - slotsAtArmStart
			for i := 0; i < extraBindings; i++ {
				c.emit(OP_POP, line)
			}
			// End scope for this arm - already done by endScopeNoEmit above for the linear flow
			// Jump over failJump target to next arm
			guardCleanupJump = c.emitJump(OP_JUMP, line)
		}

		// Pattern failure: stack already has just [matched_value]
		if failJump >= 0 {
			c.patchJump(failJump)
		}

		// Both paths converge here with [matched_value] on stack
		if guardCleanupJump >= 0 {
			c.patchJump(guardCleanupJump)
		}
		c.slotCount = slotsAtArmStart

		// Restore stack for next arm (still have matched value)
		if armIdx < len(expr.Arms)-1 {
			c.slotCount = slotsBefore + 1
		}
	}

	// Pop matched value and push nil for non-exhaustive match
	c.emit(OP_POP, line)
	c.emit(OP_NIL, line)

	// Patch all end jumps
	for _, jump := range endJumps {
		c.patchJump(jump)
	}

	c.slotCount = slotsBefore + 1 // result on stack
	return nil
}
