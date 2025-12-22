package evaluator

import (
	"encoding/hex"
	"math"
	"math/big"
	"strings"

	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/typesystem"
)

func (e *Evaluator) evalTupleLiteral(node *ast.TupleLiteral, env *Environment) Object {
	elements := e.evalExpressions(node.Elements, env)
	if len(elements) == 1 && isError(elements[0]) {
		return elements[0]
	}
	return &Tuple{Elements: elements}
}

func (e *Evaluator) evalListLiteral(node *ast.ListLiteral, env *Environment) Object {
	elements := e.evalExpressions(node.Elements, env)
	if len(elements) == 1 && isError(elements[0]) {
		return elements[0]
	}
	return newList(elements)
}

func (e *Evaluator) evalMapLiteral(node *ast.MapLiteral, env *Environment) Object {
	result := newMap()
	for _, pair := range node.Pairs {
		key := e.Eval(pair.Key, env)
		if isError(key) {
			return key
		}
		value := e.Eval(pair.Value, env)
		if isError(value) {
			return value
		}
		result = result.put(key, value)
	}
	return result
}

func (e *Evaluator) evalRecordLiteral(node *ast.RecordLiteral, env *Environment) Object {
	fields := make(map[string]Object)
	var typeName string

	// Handle spread expression first: { ...base, key: val }
	if node.Spread != nil {
		spreadVal := e.Eval(node.Spread, env)
		if isError(spreadVal) {
			return spreadVal
		}
		// Spread value must be a record
		if rec, ok := spreadVal.(*RecordInstance); ok {
			// Copy all fields from the spread base
			for _, f := range rec.Fields {
				fields[f.Key] = f.Value
			}
			// Preserve TypeName from spread base
			typeName = rec.TypeName
		} else {
			return newError("spread expression must evaluate to a record, got %s", spreadVal.Type())
		}
	}

	// Override/add fields from explicit field definitions
	for k, v := range node.Fields {
		val := e.Eval(v, env)
		if isError(val) {
			return val
		}
		fields[k] = val
	}

	newRec := NewRecord(fields)

	// If no typeName from spread, try to get it from TypeMap (nominal typing for record aliases)
	if typeName == "" && e.TypeMap != nil {
		if inferredType := e.TypeMap[node]; inferredType != nil {
			typeName = extractTypeConstructorName(inferredType)
		}
	}

	newRec.TypeName = typeName
	return newRec
}

func (e *Evaluator) evalMemberExpression(node *ast.MemberExpression, env *Environment) Object {
	left := e.Eval(node.Left, env)
	if isError(left) {
		return left
	}

	// Handle optional chaining (?.)
	if node.IsOptional {
		return e.evalOptionalChain(left, node, env)
	}

	if record, ok := left.(*RecordInstance); ok {
		if val := record.Get(node.Member.Value); val != nil {
			return val
		}
	}

	// Try Extension Method lookup
	typeName := getRuntimeTypeName(left)

	if methods, ok := e.ExtensionMethods[typeName]; ok {
		if fn, ok := methods[node.Member.Value]; ok {
			return &BoundMethod{Receiver: left, Function: fn}
		}
	}

	if _, ok := left.(*RecordInstance); ok {
		return newError("field '%s' not found in record", node.Member.Value)
	}

	return newError("dot access expects Record or Extension Method, got %s", left.Type())
}

// evalOptionalChain handles the ?. operator using Optional trait
// F<A>?.field -> F<B> where F implements Optional
func (e *Evaluator) evalOptionalChain(left Object, node *ast.MemberExpression, env *Environment) Object {
	// Get the type name for trait dispatch
	typeName := getRuntimeTypeName(left)

	// Find isEmpty (in Optional or its super trait Empty)
	isEmptyMethod, hasIsEmpty := e.lookupTraitMethod("Optional", typeName, "isEmpty")
	if !hasIsEmpty {
		return newError("type %s does not implement Optional trait (missing isEmpty)", typeName)
	}

	isEmpty := e.ApplyFunction(isEmptyMethod, []Object{left})
	if isError(isEmpty) {
		return isEmpty
	}

	// If empty, return as is (short-circuit)
	if isEmpty == TRUE {
		return left
	}

	// Not empty - unwrap, access member, wrap
	unwrapMethod, hasUnwrap := e.lookupTraitMethod("Optional", typeName, "unwrap")
	if !hasUnwrap {
		return newError("type %s does not implement Optional trait (missing unwrap)", typeName)
	}

	inner := e.ApplyFunction(unwrapMethod, []Object{left})
	if isError(inner) {
		return inner
	}

	// Access the member on the inner value
	result := e.accessMember(inner, node, env)
	if isError(result) {
		return result
	}

	// Wrap the result back
	wrapMethod, hasWrap := e.lookupTraitMethod("Optional", typeName, "wrap")
	if !hasWrap {
		return newError("type %s does not implement Optional trait (missing wrap)", typeName)
	}

	return e.ApplyFunction(wrapMethod, []Object{result})
}

// accessMember performs the actual member access on an object
func (e *Evaluator) accessMember(obj Object, node *ast.MemberExpression, env *Environment) Object {
	if record, ok := obj.(*RecordInstance); ok {
		if val := record.Get(node.Member.Value); val != nil {
			return val
		}
		return newError("field '%s' not found in record", node.Member.Value)
	}

	// Try Extension Method lookup
	typeName := getRuntimeTypeName(obj)
	if methods, ok := e.ExtensionMethods[typeName]; ok {
		if fn, ok := methods[node.Member.Value]; ok {
			return &BoundMethod{Receiver: obj, Function: fn}
		}
	}

	return newError("cannot access member '%s' on %s", node.Member.Value, obj.Type())
}

func (e *Evaluator) evalIndexExpression(node *ast.IndexExpression, env *Environment) Object {
	left := e.Eval(node.Left, env)
	if isError(left) {
		return left
	}

	index := e.Eval(node.Index, env)
	if isError(index) {
		return index
	}

	// Map indexing: m[key] -> Option<V>
	if mapObj, ok := left.(*Map); ok {
		val := mapObj.get(index)
		if val == nil {
			return makeZero() // Zero (None)
		}
		return makeSome(val) // Some(value)
	}

	// For List/Tuple/Bytes, index must be integer
	idxObj, ok := index.(*Integer)
	if !ok {
		return newError("index must be integer")
	}
	idx := int(idxObj.Value)

	switch obj := left.(type) {
	case *Bytes:
		// Bytes indexing: b[i] -> Option<Int>
		max := obj.Len()
		if idx < 0 {
			idx = max + idx
		}
		if idx < 0 || idx >= max {
			return makeZero() // Out of bounds returns Zero
		}
		return makeSome(&Integer{Value: int64(obj.get(idx))})

	case *List:
		max := obj.Len()
		if idx < 0 {
			idx = max + idx
		}
		if idx < 0 || idx >= max {
			return newError("index out of bounds")
		}
		return obj.get(idx)

	case *Tuple:
		max := len(obj.Elements)
		if idx < 0 {
			idx = max + idx
		}
		if idx < 0 || idx >= max {
			return newError("tuple index out of bounds: %d (tuple has %d elements)", idxObj.Value, max)
		}
		return obj.Elements[idx]

	default:
		return newError("index operator not supported: %s", left.Type())
	}
}

func (e *Evaluator) evalStringLiteral(node *ast.StringLiteral, env *Environment) Object {
	var elements []Object
	for _, r := range node.Value {
		elements = append(elements, &Char{Value: int64(r)})
	}
	// Strings are always List<Char>
	return newListWithType(elements, "Char")
}

func (e *Evaluator) evalFormatStringLiteral(node *ast.FormatStringLiteral, env *Environment) Object {
	// Prepend % if missing (for short form %".2f")
	fmtStr := node.Value
	if !strings.HasPrefix(fmtStr, "%") {
		fmtStr = "%" + fmtStr
	}

	// Return a function that calls sprintf
	return &Function{
		Parameters: []*ast.Parameter{
			{Name: &ast.Identifier{Value: "val"}},
		},
		Env: env,
		Body: &ast.BlockStatement{
			Statements: []ast.Statement{
				&ast.ExpressionStatement{
					Expression: &ast.CallExpression{
						Function: &ast.Identifier{Value: "sprintf"},
						Arguments: []ast.Expression{
							&ast.StringLiteral{Value: fmtStr}, // Format string with %
							&ast.Identifier{Value: "val"},     // Argument
						},
					},
				},
			},
		},
	}
}

func (e *Evaluator) evalInterpolatedString(node *ast.InterpolatedString, env *Environment) Object {
	var result []Object

	for _, part := range node.Parts {
		val := e.Eval(part, env)
		if isError(val) {
			return val
		}

		// Convert value to string (List<Char>)
		chars := e.objectToChars(val)
		result = append(result, chars...)
	}

	return newListWithType(result, "Char")
}

// objectToChars converts any object to its string representation as []Object of Char
func (e *Evaluator) objectToChars(obj Object) []Object {
	var str string

	switch o := obj.(type) {
	case *List:
		// If it's already a string (List<Char>), extract it
		if o.ElementType == "Char" {
			return o.ToSlice()
		}
		// Otherwise use Inspect
		str = o.Inspect()
	case *Integer:
		str = o.Inspect()
	case *Float:
		str = o.Inspect()
	case *Boolean:
		str = o.Inspect()
	case *Nil:
		str = "nil"
	default:
		str = obj.Inspect()
	}

	var result []Object
	for _, r := range str {
		result = append(result, &Char{Value: int64(r)})
	}
	return result
}

func (e *Evaluator) evalCharLiteral(node *ast.CharLiteral, env *Environment) Object {
	return &Char{Value: node.Value}
}

func (e *Evaluator) evalBytesLiteral(node *ast.BytesLiteral, env *Environment) Object {
	switch node.Kind {
	case "string":
		// @"hello" - UTF-8 encoded string
		return bytesFromString(node.Content)
	case "hex":
		// @x"48656C6C6F" - hex encoded bytes
		data, err := hex.DecodeString(node.Content)
		if err != nil {
			return newError("invalid hex string in bytes literal: %s", err.Error())
		}
		return bytesFromSlice(data)
	case "bin":
		// @b"01001000" - binary encoded bytes (must be multiple of 8)
		if len(node.Content)%8 != 0 {
			return newError("binary bytes literal must be a multiple of 8 bits, got %d bits", len(node.Content))
		}
		data := make([]byte, len(node.Content)/8)
		for i := 0; i < len(data); i++ {
			byteStr := node.Content[i*8 : (i+1)*8]
			var b byte
			for j, c := range byteStr {
				if c == '1' {
					b |= 1 << (7 - j)
				} else if c != '0' {
					return newError("invalid character in binary bytes literal: %c", c)
				}
			}
			data[i] = b
		}
		return bytesFromSlice(data)
	default:
		return newError("unknown bytes literal kind: %s", node.Kind)
	}
}

func (e *Evaluator) evalBitsLiteral(node *ast.BitsLiteral, env *Environment) Object {
	switch node.Kind {
	case "bin":
		// #b"10101010" - binary bits (any length)
		bits, err := bitsFromBinary(node.Content)
		if err != nil {
			return newError("invalid binary bits literal: %s", err.Error())
		}
		return bits
	case "hex":
		// #x"FF" - hex bits (4 bits per hex digit)
		bits, err := bitsFromHex(node.Content)
		if err != nil {
			return newError("invalid hex bits literal: %s", err.Error())
		}
		return bits
	case "oct":
		// #o"377" - octal bits (3 bits per octal digit)
		bits, err := bitsFromOctal(node.Content)
		if err != nil {
			return newError("invalid octal bits literal: %s", err.Error())
		}
		return bits
	default:
		return newError("unknown bits literal kind: %s", node.Kind)
	}
}

func (e *Evaluator) evalIfExpression(ie *ast.IfExpression, env *Environment) Object {
	condition := e.Eval(ie.Condition, env)
	if isError(condition) {
		return condition
	}

	if e.isTruthy(condition) {
		return e.Eval(ie.Consequence, env)
	} else if ie.Alternative != nil {
		return e.Eval(ie.Alternative, env)
	} else {
		return &Nil{}
	}
}

func (e *Evaluator) evalMatchExpression(node *ast.MatchExpression, env *Environment) Object {
	val := e.Eval(node.Expression, env)
	if isError(val) {
		return val
	}

	for _, arm := range node.Arms {
		matched, newBindings := e.matchPattern(arm.Pattern, val, env)
		if matched {
			armEnv := NewEnclosedEnvironment(env)
			for k, v := range newBindings {
				armEnv.Set(k, v)
			}

			// Evaluate guard if present
			if arm.Guard != nil {
				guardResult := e.Eval(arm.Guard, armEnv)
				if isError(guardResult) {
					return guardResult
				}
				// Guard must evaluate to true for arm to execute
				if boolVal, ok := guardResult.(*Boolean); !ok || !boolVal.Value {
					continue // Guard failed, try next arm
				}
			}

			return e.Eval(arm.Expression, armEnv)
		}
	}

	// Provide detailed error message
	tok := node.GetToken()
	return newErrorWithLocation(tok.Line, tok.Column,
		"non-exhaustive match: no pattern matched value %s of type %s",
		val.Inspect(), val.Type())
}

func (e *Evaluator) matchPattern(pat ast.Pattern, val Object, env *Environment) (bool, map[string]Object) {
	bindings := make(map[string]Object)

	switch p := pat.(type) {
	case *ast.WildcardPattern:
		return true, bindings

	case *ast.IdentifierPattern:
		bindings[p.Value] = val
		return true, bindings

	case *ast.PinPattern:
		// Pin pattern: compare with existing variable value
		pinnedVal, ok := env.Get(p.Name)
		if !ok {
			return false, bindings
		}
		// Compare values using equality
		return objectsEqual(pinnedVal, val), bindings

	case *ast.LiteralPattern:
		if intVal, ok := val.(*Integer); ok {
			if litVal, ok := p.Value.(int64); ok {
				return intVal.Value == litVal, bindings
			}
		}
		if boolVal, ok := val.(*Boolean); ok {
			if litVal, ok := p.Value.(bool); ok {
				return boolVal.Value == litVal, bindings
			}
		}
		if listVal, ok := val.(*List); ok {
			// Check if literal is string
			if strVal, ok := p.Value.(string); ok {
				// Convert List<Char> to string
				runes := []rune{}
				isString := true
				for _, el := range listVal.ToSlice() {
					if charObj, ok := el.(*Char); ok {
						runes = append(runes, rune(charObj.Value))
					} else {
						isString = false
						break
					}
				}
				if isString {
					return string(runes) == strVal, bindings
				}
			}
		}
		if charVal, ok := val.(*Char); ok {
			if litVal, ok := p.Value.(rune); ok {
				return charVal.Value == int64(litVal), bindings
			}
		}
		return false, bindings

	case *ast.StringPattern:
		// Match string with capture patterns like "/hello/{name}"
		listVal, ok := val.(*List)
		if !ok {
			return false, bindings
		}
		// Convert List<Char> to string
		str := listToString(listVal)
		// Build regex and match
		matched, captures := MatchStringPattern(p.Parts, str)
		if !matched {
			return false, bindings
		}
		// Bind captured values
		for name, value := range captures {
			bindings[name] = stringToList(value)
		}
		return true, bindings

	case *ast.ConstructorPattern:
		dataVal, ok := val.(*DataInstance)
		if !ok {
			return false, bindings
		}
		if dataVal.Name != p.Name.Value {
			return false, bindings
		}

		if len(dataVal.Fields) != len(p.Elements) {
			return false, bindings
		}

		for i, el := range p.Elements {
			matched, subBindings := e.matchPattern(el, dataVal.Fields[i], env)
			if !matched {
				return false, bindings
			}
			for k, v := range subBindings {
				bindings[k] = v
			}
		}
		return true, bindings

	case *ast.ListPattern:
		listVal, ok := val.(*List)
		if !ok {
			return false, bindings
		}

		hasSpread := false
		if len(p.Elements) > 0 {
			if _, ok := p.Elements[len(p.Elements)-1].(*ast.SpreadPattern); ok {
				hasSpread = true
			}
		}

		if hasSpread {
			fixedCount := len(p.Elements) - 1
			if listVal.len() < fixedCount {
				return false, bindings
			}

			for i := 0; i < fixedCount; i++ {
				matched, subBindings := e.matchPattern(p.Elements[i], listVal.get(i), env)
				if !matched {
					return false, bindings
				}
				for k, v := range subBindings {
					bindings[k] = v
				}
			}

			spreadPat := p.Elements[fixedCount].(*ast.SpreadPattern)
			restList := listVal.Slice(fixedCount, listVal.len())

			matched, subBindings := e.matchPattern(spreadPat.Pattern, restList, env)
			if !matched {
				return false, bindings
			}
			for k, v := range subBindings {
				bindings[k] = v
			}
			return true, bindings

		} else {
			if listVal.len() != len(p.Elements) {
				return false, bindings
			}
			for i, el := range p.Elements {
				matched, subBindings := e.matchPattern(el, listVal.get(i), env)
				if !matched {
					return false, bindings
				}
				for k, v := range subBindings {
					bindings[k] = v
				}
			}
			return true, bindings
		}

	case *ast.TuplePattern:
		tupleVal, ok := val.(*Tuple)
		if !ok {
			// Allow matching List with TuplePattern (for variadic args compatibility)
			if listVal, ok := val.(*List); ok {
				hasSpread := false
				if len(p.Elements) > 0 {
					if _, ok := p.Elements[len(p.Elements)-1].(*ast.SpreadPattern); ok {
						hasSpread = true
					}
				}

				if hasSpread {
					fixedCount := len(p.Elements) - 1
					if listVal.len() < fixedCount {
						return false, bindings
					}

					for i := 0; i < fixedCount; i++ {
						matched, subBindings := e.matchPattern(p.Elements[i], listVal.get(i), env)
						if !matched {
							return false, bindings
						}
						for k, v := range subBindings {
							bindings[k] = v
						}
					}

					spreadPat := p.Elements[fixedCount].(*ast.SpreadPattern)
					restList := listVal.Slice(fixedCount, listVal.len())

					matched, subBindings := e.matchPattern(spreadPat.Pattern, restList, env)
					if !matched {
						return false, bindings
					}
					for k, v := range subBindings {
						bindings[k] = v
					}
					return true, bindings

				} else {
					if listVal.len() != len(p.Elements) {
						return false, bindings
					}
					for i, el := range p.Elements {
						matched, subBindings := e.matchPattern(el, listVal.get(i), env)
						if !matched {
							return false, bindings
						}
						for k, v := range subBindings {
							bindings[k] = v
						}
					}
					return true, bindings
				}
			}
			return false, bindings
		}

		hasSpread := false
		if len(p.Elements) > 0 {
			if _, ok := p.Elements[len(p.Elements)-1].(*ast.SpreadPattern); ok {
				hasSpread = true
			}
		}

		if hasSpread {
			fixedCount := len(p.Elements) - 1
			if len(tupleVal.Elements) < fixedCount {
				return false, bindings
			}

			for i := 0; i < fixedCount; i++ {
				matched, subBindings := e.matchPattern(p.Elements[i], tupleVal.Elements[i], env)
				if !matched {
					return false, bindings
				}
				for k, v := range subBindings {
					bindings[k] = v
				}
			}

			spreadPat := p.Elements[fixedCount].(*ast.SpreadPattern)
			restElements := tupleVal.Elements[fixedCount:]
			restTuple := &Tuple{Elements: restElements}

			matched, subBindings := e.matchPattern(spreadPat.Pattern, restTuple, env)
			if !matched {
				return false, bindings
			}
			for k, v := range subBindings {
				bindings[k] = v
			}

			return true, bindings

		} else {
			if len(tupleVal.Elements) != len(p.Elements) {
				return false, bindings
			}
			for i, el := range p.Elements {
				matched, subBindings := e.matchPattern(el, tupleVal.Elements[i], env)
				if !matched {
					return false, bindings
				}
				for k, v := range subBindings {
					bindings[k] = v
				}
			}
			return true, bindings
		}
	case *ast.RecordPattern:
		recordVal, ok := val.(*RecordInstance)
		if !ok {
			return false, bindings
		}

		for k, subPat := range p.Fields {
			fieldVal := recordVal.Get(k)
			if fieldVal == nil {
				return false, bindings // Field missing
			}
			matched, subBindings := e.matchPattern(subPat, fieldVal, env)
			if !matched {
				return false, bindings
			}
			for bk, bv := range subBindings {
				bindings[bk] = bv
			}
		}
		return true, bindings

	case *ast.TypePattern:
		// Type pattern: n: Int matches if val has type Int
		if e.matchesType(val, p.Type) {
			if p.Name != "_" {
				bindings[p.Name] = val
			}
			return true, bindings
		}
		return false, bindings
	}
	return false, bindings
}

// matchesType checks if a runtime value matches the given AST type
func (e *Evaluator) matchesType(val Object, astType ast.Type) bool {
	switch t := astType.(type) {
	case *ast.NamedType:
		typeName := t.Name.Value
		switch typeName {
		case "Int":
			_, ok := val.(*Integer)
			return ok
		case "Float":
			_, ok := val.(*Float)
			return ok
		case "Bool":
			_, ok := val.(*Boolean)
			return ok
		case "Char":
			_, ok := val.(*Char)
			return ok
		case "String":
			// String is List<Char>
			if list, ok := val.(*List); ok {
				if list.ElementType == "Char" {
					return true
				}
				// Check if all elements are Char
				for _, el := range list.ToSlice() {
					if _, ok := el.(*Char); !ok {
						return false
					}
				}
				return true
			}
			return false
		case "Nil":
			_, ok := val.(*Nil)
			return ok
		case "BigInt":
			_, ok := val.(*BigInt)
			return ok
		case "Rational":
			_, ok := val.(*Rational)
			return ok
		case "List":
			_, ok := val.(*List)
			return ok
		case "Option":
			// Option<T> is Some(T) or Zero
			if di, ok := val.(*DataInstance); ok {
				return di.Name == "Some" || di.Name == "Zero"
			}
			if _, ok := val.(*Nil); ok {
				return true // Zero
			}
			return false
		case "Result":
			if di, ok := val.(*DataInstance); ok {
				return di.Name == "Ok" || di.Name == "Fail"
			}
			return false
		default:
			// Check for user-defined ADT constructors
			if di, ok := val.(*DataInstance); ok {
				return di.TypeName == typeName || di.Name == typeName
			}
			// Check for named record types (type User = { ... })
			if rec, ok := val.(*RecordInstance); ok {
				// First check exact TypeName match
				if rec.TypeName == typeName {
					return true
				}
				// Then check structural match against type alias
				if e.TypeAliases != nil {
					if underlying, exists := e.TypeAliases[typeName]; exists {
						if tRec, ok := underlying.(typesystem.TRecord); ok {
							// Check if all fields from type definition exist in record
							for fieldName := range tRec.Fields {
								if rec.Get(fieldName) == nil {
									return false
								}
							}
							return true
						}
					}
				}
			}
			return false
		}
	case *ast.UnionType:
		// Union type: check if val matches any member
		for _, member := range t.Types {
			if e.matchesType(val, member) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func (e *Evaluator) evalIdentifier(node *ast.Identifier, env *Environment) Object {
	if val, ok := env.Get(node.Value); ok {
		return val
	}
	if builtin, ok := Builtins[node.Value]; ok {
		return builtin
	}
	// Check if it's a trait method (e.g., fmap from Functor)
	if traitMethod := e.lookupTraitMethodByName(node.Value); traitMethod != nil {
		return traitMethod
	}
	return newError("identifier not found: %s", node.Value)
}

func (e *Evaluator) evalAssignExpression(node *ast.AssignExpression, env *Environment) Object {
	// Set type context BEFORE evaluating value, so nullary ClassMethod calls can dispatch
	oldCallNode := e.CurrentCallNode
	if node.AnnotatedType != nil {
		e.CurrentCallNode = node
	}

	val := e.Eval(node.Value, env)

	// Restore previous call node
	e.CurrentCallNode = oldCallNode

	if isError(val) {
		return val
	}

	// If there's a type annotation and value is a nullary ClassMethod (Arity == 0),
	// auto-call it with type context for proper dispatch
	if node.AnnotatedType != nil {
		if cm, ok := val.(*ClassMethod); ok && cm.Arity == 0 {
			// Ensure context is set for the call
			prevCallNode := e.CurrentCallNode
			e.CurrentCallNode = node
			result := e.ApplyFunction(cm, []Object{})
			e.CurrentCallNode = prevCallNode
			if !isError(result) {
				val = result
			}
		}
	}

	// If there's a type annotation and value is a List, propagate element type
	if node.AnnotatedType != nil {
		if list, ok := val.(*List); ok {
			if elemType := extractListElementType(node.AnnotatedType); elemType != "" {
				list.ElementType = elemType
			}
		}
		// If value is a RecordInstance and type annotation is a named type, set TypeName
		if record, ok := val.(*RecordInstance); ok {
			// Handle simple named type (e.g. Point)
			if namedType, ok := node.AnnotatedType.(*ast.NamedType); ok {
				record.TypeName = namedType.Name.Value
			}
			// Handle generic named type (e.g. Box<Int>)
			// We only set the base TypeName ("Box") because runtime erasure
			// TApp also has Constructor which is usually NamedType or TCon
			// AST node for Box<Int> is NamedType with Args
			// No change needed for AST NamedType structure (it includes Args)
		}
	}

	if ident, ok := node.Left.(*ast.Identifier); ok {
		if !env.Update(ident.Value, val) {
			env.Set(ident.Value, val)
		}
		return val
	} else if ma, ok := node.Left.(*ast.MemberExpression); ok {
		obj := e.Eval(ma.Left, env)
		if isError(obj) {
			return obj
		}

		if record, ok := obj.(*RecordInstance); ok {
			record.Set(ma.Member.Value, val)
			return val
		}
		return newError("assignment to member expects Record, got %s", obj.Type())
	}
	return newError("invalid assignment target")
}

func (e *Evaluator) evalCallExpression(node *ast.CallExpression, env *Environment) Object {
	// Special handling for default() to avoid init cycle
	if ident, ok := node.Function.(*ast.Identifier); ok && ident.Value == "default" {
		args := e.evalExpressions(node.Arguments, env)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}
		if len(args) != 1 {
			return newError("wrong number of arguments to default. got=%d, want=1", len(args))
		}
		typeObj, ok := args[0].(*TypeObject)
		if !ok {
			return newError("argument to default must be a Type, got %s", args[0].Type())
		}
		return e.GetDefaultForType(typeObj.TypeVal)
	}

	if node.IsTail {
		function := e.Eval(node.Function, env)
		if isError(function) {
			return function
		}
		args := e.evalExpressions(node.Arguments, env)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}
		tc := &TailCall{Func: function, Args: args}
		if tok := node.GetToken(); tok.Type != "" {
			tc.Line = tok.Line
			tc.Column = tok.Column
		}
		// Store call info for stack trace (even though it's a tail call)
		tc.Name = getFunctionName(function)
		tc.File = e.CurrentFile
		return tc
	}

	function := e.Eval(node.Function, env)
	if isError(function) {
		return function
	}
	args := e.evalExpressions(node.Arguments, env)
	if len(args) == 1 && isError(args[0]) {
		return args[0]
	}

	// Push call frame with call site info (where the call is made from)
	funcName := getFunctionName(function)
	tok := node.GetToken()
	e.PushCall(funcName, e.CurrentFile, tok.Line, tok.Column)

	// Store current call node for type-based dispatch (pure/mempty)
	// Preserve parent context (e.g., AssignExpression with type annotation)
	oldCallNode := e.CurrentCallNode
	if oldCallNode == nil {
		e.CurrentCallNode = node
	}
	result := e.ApplyFunction(function, args)
	e.CurrentCallNode = oldCallNode

	// Add stack trace to errors if not already present
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
	}

	e.PopCall()
	return result
}

// getFunctionName extracts the name from a function object
func getFunctionName(fn Object) string {
	switch f := fn.(type) {
	case *Function:
		if f.Name != "" {
			return f.Name
		}
		return "<lambda>"
	case *Builtin:
		return f.Name
	case *BoundMethod:
		if f.Function != nil {
			return f.Function.Name
		}
		return "<method>"
	case *Constructor:
		return f.TypeName
	case *PartialApplication:
		if f.Function != nil {
			return f.Function.Name + " (partial)"
		}
		if f.Builtin != nil {
			return f.Builtin.Name + " (partial)"
		}
		return "<partial>"
	default:
		return "<unknown>"
	}
}

func (e *Evaluator) evalPrefixExpression(operator string, right Object) Object {
	switch operator {
	case "!":
		return e.evalBangOperatorExpression(right)
	case "-":
		if right.Type() == INTEGER_OBJ {
			value := right.(*Integer).Value
			return &Integer{Value: -value}
		} else if right.Type() == FLOAT_OBJ {
			value := right.(*Float).Value
			return &Float{Value: -value}
		} else if right.Type() == BIG_INT_OBJ {
			value := right.(*BigInt).Value
			return &BigInt{Value: new(big.Int).Neg(value)}
		} else if right.Type() == RATIONAL_OBJ {
			value := right.(*Rational).Value
			return &Rational{Value: new(big.Rat).Neg(value)}
		}
		return newError("unknown operator: %s%s", operator, right.Type())
	case "~":
		if right.Type() != INTEGER_OBJ {
			return newError("unknown operator: %s%s", operator, right.Type())
		}
		value := right.(*Integer).Value
		return &Integer{Value: ^value}
	default:
		return newError("unknown operator: %s%s", operator, right.Type())
	}
}

func (e *Evaluator) evalBangOperatorExpression(right Object) Object {
	switch right {
	case TRUE:
		return FALSE
	case FALSE:
		return TRUE
	default:
		if right.Type() == BOOLEAN_OBJ {
			val := right.(*Boolean).Value
			return e.nativeBoolToBooleanObject(!val)
		}
		return newError("operator ! not supported for %s", right.Type())
	}
}

// EvalInfixExpression evaluates an infix expression (exported for VM)
func (e *Evaluator) EvalInfixExpression(operator string, left, right Object) Object {
	// First, try trait-based dispatch if we have operator traits configured
	if e.OperatorTraits != nil {
		if traitName, ok := e.OperatorTraits[operator]; ok {
			// Try to find and call the operator method via trait
			result := e.tryOperatorDispatch(traitName, operator, left, right)
			if result != nil {
				return result
			}
			// If no implementation found, fall through to built-in logic
		}
	}

	// Try user-defined operator dispatch (search all traits for this operator)
	result := e.tryUserDefinedOperator(operator, left, right)
	if result != nil {
		return result
	}

	// Built-in operator implementations (fallback and for primitive types)
	if left.Type() == INTEGER_OBJ && right.Type() == INTEGER_OBJ {
		return e.evalIntegerInfixExpression(operator, left, right)
	}
	if left.Type() == FLOAT_OBJ && right.Type() == FLOAT_OBJ {
		return e.evalFloatInfixExpression(operator, left, right)
	}
	if left.Type() == BIG_INT_OBJ && right.Type() == BIG_INT_OBJ {
		return e.evalBigIntInfixExpression(operator, left, right)
	}
	if left.Type() == RATIONAL_OBJ && right.Type() == RATIONAL_OBJ {
		return e.evalRationalInfixExpression(operator, left, right)
	}
	if left.Type() == BOOLEAN_OBJ && right.Type() == BOOLEAN_OBJ {
		return e.evalBooleanInfixExpression(operator, left, right)
	}
	if left.Type() == CHAR_OBJ && right.Type() == CHAR_OBJ {
		return e.evalCharInfixExpression(operator, left, right)
	}

	if left.Type() == LIST_OBJ && right.Type() == LIST_OBJ {
		return e.evalListInfixExpression(operator, left, right)
	}

	// Bytes operations
	if left.Type() == BYTES_OBJ && right.Type() == BYTES_OBJ {
		return e.evalBytesInfixExpression(operator, left, right)
	}

	// Bits operations
	if left.Type() == BITS_OBJ && right.Type() == BITS_OBJ {
		return e.evalBitsInfixExpression(operator, left, right)
	}

	// Tuple comparison
	if left.Type() == TUPLE_OBJ && right.Type() == TUPLE_OBJ {
		return e.evalTupleInfixExpression(operator, left, right)
	}

	// Option comparison
	if leftData, ok := left.(*DataInstance); ok {
		if rightData, ok := right.(*DataInstance); ok {
			if leftData.TypeName == config.OptionTypeName && rightData.TypeName == config.OptionTypeName {
				return e.evalOptionInfixExpression(operator, leftData, rightData)
			}
			if leftData.TypeName == config.ResultTypeName && rightData.TypeName == config.ResultTypeName {
				return e.evalResultInfixExpression(operator, leftData, rightData)
			}
		}
	}

	if operator == "==" {
		return e.nativeBoolToBooleanObject(e.areObjectsEqual(left, right))
	}
	if operator == "!=" {
		return e.nativeBoolToBooleanObject(!e.areObjectsEqual(left, right))
	}

	// Cons operator - prepend element to list
	if operator == "::" {
		if rightList, ok := right.(*List); ok {
			return rightList.prepend(left)
		}
		return newError("right operand of :: must be List, got %s", right.Type())
	}

	return newError("type mismatch: %s %s %s", left.Type(), operator, right.Type())
}

// tryUserDefinedOperator searches all traits for a user-defined operator implementation.
// Returns nil if no implementation is found.
func (e *Evaluator) tryUserDefinedOperator(operator string, left, right Object) Object {
	typeName := getRuntimeTypeName(left)
	methodName := "(" + operator + ")"

	// Search all traits for this operator
	for _, typesMap := range e.ClassImplementations {
		if methodTableObj, ok := typesMap[typeName]; ok {
			if methodTable, ok := methodTableObj.(*MethodTable); ok {
				if method, ok := methodTable.Methods[methodName]; ok {
					oldContainer := e.ContainerContext
					e.ContainerContext = typeName
					defer func() { e.ContainerContext = oldContainer }()
					return e.ApplyFunction(method, []Object{left, right})
				}
			}
		}
	}
	return nil
}

// tryOperatorDispatch attempts to dispatch an operator through the trait system.
// Returns nil if no implementation is found (allowing fallback to built-in).
func (e *Evaluator) tryOperatorDispatch(traitName, operator string, left, right Object) Object {
	typeName := getRuntimeTypeName(left)
	methodName := "(" + operator + ")"

	// Look for implementation in ClassImplementations
	if typesMap, ok := e.ClassImplementations[traitName]; ok {
		if methodTableObj, ok := typesMap[typeName]; ok {
			if methodTable, ok := methodTableObj.(*MethodTable); ok {
				if method, ok := methodTable.Methods[methodName]; ok {
					// Set container context so methods like pure/mempty can dispatch correctly
					// Works for any trait-based operator, not just Monad
					oldContainer := e.ContainerContext
					e.ContainerContext = typeName
					defer func() { e.ContainerContext = oldContainer }()
					return e.ApplyFunction(method, []Object{left, right})
				}
			}
		}
	}

	// Try trait default implementation
	if e.TraitDefaults != nil {
		key := traitName + "." + methodName
		if fnStmt, ok := e.TraitDefaults[key]; ok {
			defaultFn := &Function{
				Name:       methodName,
				Parameters: fnStmt.Parameters,
				Body:       fnStmt.Body,
				Env:        e.GlobalEnv,
				Line:       fnStmt.Token.Line,
				Column:     fnStmt.Token.Column,
			}
			return e.ApplyFunction(defaultFn, []Object{left, right})
		}
	}

	// No trait implementation found
	return nil
}

func (e *Evaluator) evalIntegerInfixExpression(operator string, left, right Object) Object {
	leftVal := left.(*Integer).Value
	rightVal := right.(*Integer).Value

	switch operator {
	case "+":
		return &Integer{Value: leftVal + rightVal}
	case "-":
		return &Integer{Value: leftVal - rightVal}
	case "*":
		return &Integer{Value: leftVal * rightVal}
	case "/":
		return &Integer{Value: leftVal / rightVal}
	case "%":
		return &Integer{Value: leftVal % rightVal}
	case "**":
		return &Integer{Value: intPow(leftVal, rightVal)}
	case "&":
		return &Integer{Value: leftVal & rightVal}
	case "|":
		return &Integer{Value: leftVal | rightVal}
	case "^":
		return &Integer{Value: leftVal ^ rightVal}
	case "<<":
		return &Integer{Value: leftVal << rightVal}
	case ">>":
		return &Integer{Value: leftVal >> rightVal}
	case "<":
		return e.nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return e.nativeBoolToBooleanObject(leftVal > rightVal)
	case "==":
		return e.nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return e.nativeBoolToBooleanObject(leftVal != rightVal)
	case "<=":
		return e.nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return e.nativeBoolToBooleanObject(leftVal >= rightVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func (e *Evaluator) evalFloatInfixExpression(operator string, left, right Object) Object {
	leftVal := left.(*Float).Value
	rightVal := right.(*Float).Value

	switch operator {
	case "+":
		return &Float{Value: leftVal + rightVal}
	case "-":
		return &Float{Value: leftVal - rightVal}
	case "*":
		return &Float{Value: leftVal * rightVal}
	case "/":
		return &Float{Value: leftVal / rightVal}
	case "%":
		return &Float{Value: math.Mod(leftVal, rightVal)}
	case "**":
		return &Float{Value: math.Pow(leftVal, rightVal)}
	case "<":
		return e.nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return e.nativeBoolToBooleanObject(leftVal > rightVal)
	case "==":
		return e.nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return e.nativeBoolToBooleanObject(leftVal != rightVal)
	case "<=":
		return e.nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return e.nativeBoolToBooleanObject(leftVal >= rightVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func (e *Evaluator) evalBigIntInfixExpression(operator string, left, right Object) Object {
	leftVal := left.(*BigInt).Value
	rightVal := right.(*BigInt).Value

	switch operator {
	case "+":
		return &BigInt{Value: new(big.Int).Add(leftVal, rightVal)}
	case "-":
		return &BigInt{Value: new(big.Int).Sub(leftVal, rightVal)}
	case "*":
		return &BigInt{Value: new(big.Int).Mul(leftVal, rightVal)}
	case "/":
		if rightVal.Sign() == 0 {
			return newError("division by zero")
		}
		return &BigInt{Value: new(big.Int).Div(leftVal, rightVal)}
	case "%":
		if rightVal.Sign() == 0 {
			return newError("modulo by zero")
		}
		return &BigInt{Value: new(big.Int).Mod(leftVal, rightVal)}
	case "**":
		return &BigInt{Value: new(big.Int).Exp(leftVal, rightVal, nil)}
	case "<":
		return e.nativeBoolToBooleanObject(leftVal.Cmp(rightVal) < 0)
	case ">":
		return e.nativeBoolToBooleanObject(leftVal.Cmp(rightVal) > 0)
	case "==":
		return e.nativeBoolToBooleanObject(leftVal.Cmp(rightVal) == 0)
	case "!=":
		return e.nativeBoolToBooleanObject(leftVal.Cmp(rightVal) != 0)
	case "<=":
		return e.nativeBoolToBooleanObject(leftVal.Cmp(rightVal) <= 0)
	case ">=":
		return e.nativeBoolToBooleanObject(leftVal.Cmp(rightVal) >= 0)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func (e *Evaluator) evalRationalInfixExpression(operator string, left, right Object) Object {
	leftVal := left.(*Rational).Value
	rightVal := right.(*Rational).Value

	switch operator {
	case "+":
		return &Rational{Value: new(big.Rat).Add(leftVal, rightVal)}
	case "-":
		return &Rational{Value: new(big.Rat).Sub(leftVal, rightVal)}
	case "*":
		return &Rational{Value: new(big.Rat).Mul(leftVal, rightVal)}
	case "/":
		if rightVal.Sign() == 0 {
			return newError("division by zero")
		}
		return &Rational{Value: new(big.Rat).Quo(leftVal, rightVal)}
	case "%":
		if rightVal.Sign() == 0 {
			return newError("modulo by zero")
		}
		// a % b = a - b * floor(a/b)
		quotient := new(big.Rat).Quo(leftVal, rightVal)
		// Floor: convert to integer (truncate towards negative infinity)
		num := quotient.Num()
		den := quotient.Denom()
		floorVal := new(big.Int).Div(num, den)
		// Adjust for negative quotients
		if quotient.Sign() < 0 && new(big.Int).Mod(num, den).Sign() != 0 {
			floorVal.Sub(floorVal, big.NewInt(1))
		}
		floorRat := new(big.Rat).SetInt(floorVal)
		// result = a - b * floor(a/b)
		result := new(big.Rat).Sub(leftVal, new(big.Rat).Mul(rightVal, floorRat))
		return &Rational{Value: result}
	case "<":
		return e.nativeBoolToBooleanObject(leftVal.Cmp(rightVal) < 0)
	case ">":
		return e.nativeBoolToBooleanObject(leftVal.Cmp(rightVal) > 0)
	case "==":
		return e.nativeBoolToBooleanObject(leftVal.Cmp(rightVal) == 0)
	case "!=":
		return e.nativeBoolToBooleanObject(leftVal.Cmp(rightVal) != 0)
	case "<=":
		return e.nativeBoolToBooleanObject(leftVal.Cmp(rightVal) <= 0)
	case ">=":
		return e.nativeBoolToBooleanObject(leftVal.Cmp(rightVal) >= 0)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func (e *Evaluator) evalBooleanInfixExpression(operator string, left, right Object) Object {
	leftVal := left.(*Boolean).Value
	rightVal := right.(*Boolean).Value

	// Convert bool to int for comparison: false=0, true=1
	leftInt := 0
	rightInt := 0
	if leftVal {
		leftInt = 1
	}
	if rightVal {
		rightInt = 1
	}

	switch operator {
	case "==":
		return e.nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return e.nativeBoolToBooleanObject(leftVal != rightVal)
	case "<":
		return e.nativeBoolToBooleanObject(leftInt < rightInt)
	case ">":
		return e.nativeBoolToBooleanObject(leftInt > rightInt)
	case "<=":
		return e.nativeBoolToBooleanObject(leftInt <= rightInt)
	case ">=":
		return e.nativeBoolToBooleanObject(leftInt >= rightInt)
	case "&&":
		return e.nativeBoolToBooleanObject(leftVal && rightVal)
	case "||":
		return e.nativeBoolToBooleanObject(leftVal || rightVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func (e *Evaluator) evalCharInfixExpression(operator string, left, right Object) Object {
	leftVal := left.(*Char).Value
	rightVal := right.(*Char).Value

	switch operator {
	case "==":
		return e.nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return e.nativeBoolToBooleanObject(leftVal != rightVal)
	case "<":
		return e.nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return e.nativeBoolToBooleanObject(leftVal > rightVal)
	case "<=":
		return e.nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return e.nativeBoolToBooleanObject(leftVal >= rightVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func (e *Evaluator) evalBytesInfixExpression(operator string, left, right Object) Object {
	leftBytes := left.(*Bytes)
	rightBytes := right.(*Bytes)

	switch operator {
	case "++":
		// Concatenation
		return leftBytes.Concat(rightBytes)
	case "==":
		return e.nativeBoolToBooleanObject(leftBytes.equals(rightBytes))
	case "!=":
		return e.nativeBoolToBooleanObject(!leftBytes.equals(rightBytes))
	case "<", ">", "<=", ">=":
		cmp := leftBytes.compare(rightBytes)
		switch operator {
		case "<":
			return e.nativeBoolToBooleanObject(cmp < 0)
		case ">":
			return e.nativeBoolToBooleanObject(cmp > 0)
		case "<=":
			return e.nativeBoolToBooleanObject(cmp <= 0)
		case ">=":
			return e.nativeBoolToBooleanObject(cmp >= 0)
		}
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
	return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
}

func (e *Evaluator) evalBitsInfixExpression(operator string, left, right Object) Object {
	leftBits := left.(*Bits)
	rightBits := right.(*Bits)

	switch operator {
	case "++":
		// Concatenation
		return leftBits.Concat(rightBits)
	case "==":
		return e.nativeBoolToBooleanObject(leftBits.equals(rightBits))
	case "!=":
		return e.nativeBoolToBooleanObject(!leftBits.equals(rightBits))
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func (e *Evaluator) evalListInfixExpression(operator string, left, right Object) Object {
	leftList := left.(*List)
	rightList := right.(*List)

	switch operator {
	case "++":
		// Concatenation
		return leftList.Concat(rightList)
	case "::":
		// Cons - prepend left element to right list (even if left is also a List, e.g. String)
		return rightList.prepend(leftList)
	case "==":
		return e.nativeBoolToBooleanObject(e.areObjectsEqual(left, right))
	case "!=":
		return e.nativeBoolToBooleanObject(!e.areObjectsEqual(left, right))
	case "<", ">", "<=", ">=":
		// Lexicographic comparison
		cmp := e.compareListsLexicographic(leftList, rightList)
		switch operator {
		case "<":
			return e.nativeBoolToBooleanObject(cmp < 0)
		case ">":
			return e.nativeBoolToBooleanObject(cmp > 0)
		case "<=":
			return e.nativeBoolToBooleanObject(cmp <= 0)
		case ">=":
			return e.nativeBoolToBooleanObject(cmp >= 0)
		}
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
	return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
}

// compareListsLexicographic compares two lists lexicographically
// Returns -1 if left < right, 0 if equal, 1 if left > right
func (e *Evaluator) compareListsLexicographic(left, right *List) int {
	minLen := left.len()
	if right.len() < minLen {
		minLen = right.len()
	}

	for i := 0; i < minLen; i++ {
		cmp := e.compareObjects(left.get(i), right.get(i))
		if cmp != 0 {
			return cmp
		}
	}

	// All compared elements are equal, shorter list is smaller
	if left.len() < right.len() {
		return -1
	} else if left.len() > right.len() {
		return 1
	}
	return 0
}

// compareObjects compares two objects
// Returns -1 if left < right, 0 if equal, 1 if left > right
func (e *Evaluator) compareObjects(left, right Object) int {
	// Compare integers
	if leftInt, ok := left.(*Integer); ok {
		if rightInt, ok := right.(*Integer); ok {
			if leftInt.Value < rightInt.Value {
				return -1
			} else if leftInt.Value > rightInt.Value {
				return 1
			}
			return 0
		}
	}

	// Compare floats
	if leftFloat, ok := left.(*Float); ok {
		if rightFloat, ok := right.(*Float); ok {
			if leftFloat.Value < rightFloat.Value {
				return -1
			} else if leftFloat.Value > rightFloat.Value {
				return 1
			}
			return 0
		}
	}

	// Compare booleans (false < true)
	if leftBool, ok := left.(*Boolean); ok {
		if rightBool, ok := right.(*Boolean); ok {
			if !leftBool.Value && rightBool.Value {
				return -1
			} else if leftBool.Value && !rightBool.Value {
				return 1
			}
			return 0
		}
	}

	// Compare chars
	if leftChar, ok := left.(*Char); ok {
		if rightChar, ok := right.(*Char); ok {
			if leftChar.Value < rightChar.Value {
				return -1
			} else if leftChar.Value > rightChar.Value {
				return 1
			}
			return 0
		}
	}

	// Compare strings (lists of chars)
	if leftList, ok := left.(*List); ok {
		if rightList, ok := right.(*List); ok {
			return e.compareListsLexicographic(leftList, rightList)
		}
	}

	// Compare Options (Zero < Some)
	if leftData, ok := left.(*DataInstance); ok {
		if rightData, ok := right.(*DataInstance); ok {
			if leftData.TypeName == config.OptionTypeName && rightData.TypeName == config.OptionTypeName {
				// Zero < Some
				if leftData.Name == config.ZeroCtorName && rightData.Name == config.SomeCtorName {
					return -1
				} else if leftData.Name == config.SomeCtorName && rightData.Name == config.ZeroCtorName {
					return 1
				} else if leftData.Name == config.ZeroCtorName && rightData.Name == config.ZeroCtorName {
					return 0
				} else {
					// Both are Some, compare inner values
					return e.compareObjects(leftData.Fields[0], rightData.Fields[0])
				}
			}
		}
	}

	// Compare Tuples (lexicographic)
	if leftTuple, ok := left.(*Tuple); ok {
		if rightTuple, ok := right.(*Tuple); ok {
			minLen := len(leftTuple.Elements)
			if len(rightTuple.Elements) < minLen {
				minLen = len(rightTuple.Elements)
			}
			for i := 0; i < minLen; i++ {
				cmp := e.compareObjects(leftTuple.Elements[i], rightTuple.Elements[i])
				if cmp != 0 {
					return cmp
				}
			}
			if len(leftTuple.Elements) < len(rightTuple.Elements) {
				return -1
			} else if len(leftTuple.Elements) > len(rightTuple.Elements) {
				return 1
			}
			return 0
		}
	}

	// Default: compare by string representation
	if left.Inspect() < right.Inspect() {
		return -1
	} else if left.Inspect() > right.Inspect() {
		return 1
	}
	return 0
}

func (e *Evaluator) evalTupleInfixExpression(operator string, left, right Object) Object {
	leftTuple := left.(*Tuple)
	rightTuple := right.(*Tuple)

	switch operator {
	case "==":
		return e.nativeBoolToBooleanObject(e.areObjectsEqual(left, right))
	case "!=":
		return e.nativeBoolToBooleanObject(!e.areObjectsEqual(left, right))
	case "<", ">", "<=", ">=":
		cmp := e.compareObjects(left, right)
		switch operator {
		case "<":
			return e.nativeBoolToBooleanObject(cmp < 0)
		case ">":
			return e.nativeBoolToBooleanObject(cmp > 0)
		case "<=":
			return e.nativeBoolToBooleanObject(cmp <= 0)
		case ">=":
			return e.nativeBoolToBooleanObject(cmp >= 0)
		}
	}
	return newError("unknown operator: %s %s %s", leftTuple.Type(), operator, rightTuple.Type())
}

func (e *Evaluator) evalOptionInfixExpression(operator string, left, right *DataInstance) Object {
	switch operator {
	case "==":
		return e.nativeBoolToBooleanObject(e.areObjectsEqual(left, right))
	case "!=":
		return e.nativeBoolToBooleanObject(!e.areObjectsEqual(left, right))
	case "<", ">", "<=", ">=":
		cmp := e.compareObjects(left, right)
		switch operator {
		case "<":
			return e.nativeBoolToBooleanObject(cmp < 0)
		case ">":
			return e.nativeBoolToBooleanObject(cmp > 0)
		case "<=":
			return e.nativeBoolToBooleanObject(cmp <= 0)
		case ">=":
			return e.nativeBoolToBooleanObject(cmp >= 0)
		}
	}
	return newError("unknown operator: Option %s Option", operator)
}

func (e *Evaluator) evalResultInfixExpression(operator string, left, right *DataInstance) Object {
	switch operator {
	case "==":
		return e.nativeBoolToBooleanObject(e.areObjectsEqual(left, right))
	case "!=":
		return e.nativeBoolToBooleanObject(!e.areObjectsEqual(left, right))
	}
	return newError("unknown operator: Result %s Result", operator)
}

func (e *Evaluator) evalPostfixExpression(operator string, left Object) Object {
	switch operator {
	case "?":
		if data, ok := left.(*DataInstance); ok {
			if data.TypeName == config.ResultTypeName {
				if data.Name == config.OkCtorName {
					return data.Fields[0]
				} else if data.Name == config.FailCtorName {
					return &ReturnValue{Value: left}
				}
			} else if data.TypeName == config.OptionTypeName {
				if data.Name == config.SomeCtorName {
					return data.Fields[0]
				} else if data.Name == config.ZeroCtorName {
					return &ReturnValue{Value: left}
				}
			}
		}
		return newError("operator ? not supported for %s", left.Inspect())
	default:
		return newError("unknown operator: %s", operator)
	}
}
