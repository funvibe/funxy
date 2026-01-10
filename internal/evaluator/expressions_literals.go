package evaluator

import (
	"encoding/hex"
	"strings"

	"github.com/funvibe/funxy/internal/ast"
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

// evalListComprehension evaluates a list comprehension expression
// [output | clause, clause, ...]
// Clauses can be generators (pattern <- iterable) or filters (boolean expression)
func (e *Evaluator) evalListComprehension(node *ast.ListComprehension, env *Environment) Object {
	// Start with a single empty environment (representing one "iteration")
	envs := []*Environment{NewEnclosedEnvironment(env)}

	// Process each clause
	for _, clause := range node.Clauses {
		switch c := clause.(type) {
		case *ast.CompGenerator:
			// For each current environment, iterate over the iterable and create new environments
			var newEnvs []*Environment
			for _, currentEnv := range envs {
				// Evaluate the iterable in the current environment
				iterable := e.Eval(c.Iterable, currentEnv)
				if isError(iterable) {
					return iterable
				}

				// Get elements from the iterable
				elements := e.getIterableElements(iterable)
				if elements == nil {
					return newError("cannot iterate over %s", iterable.Type())
				}

				// For each element, create a new environment with the pattern bound
				for _, elem := range elements {
					newEnv := NewEnclosedEnvironment(currentEnv)
					if !e.bindPattern(c.Pattern, elem, newEnv) {
						continue // Pattern didn't match, skip this element
					}
					newEnvs = append(newEnvs, newEnv)
				}
			}
			envs = newEnvs

		case *ast.CompFilter:
			// Filter: keep only environments where the condition is true
			var newEnvs []*Environment
			for _, currentEnv := range envs {
				cond := e.Eval(c.Condition, currentEnv)
				if isError(cond) {
					return cond
				}
				if e.isTruthy(cond) {
					newEnvs = append(newEnvs, currentEnv)
				}
			}
			envs = newEnvs
		}
	}

	// Evaluate the output expression for each remaining environment
	var results []Object
	for _, currentEnv := range envs {
		result := e.Eval(node.Output, currentEnv)
		if isError(result) {
			return result
		}
		results = append(results, result)
	}

	return newList(results)
}

// getIterableElements extracts elements from an iterable object
func (e *Evaluator) getIterableElements(obj Object) []Object {
	switch o := obj.(type) {
	case *List:
		return o.ToSlice()
	case *Tuple:
		return o.Elements
	case *Map:
		// For maps, iterate over key-value pairs as tuples
		return o.items().ToSlice()
	case *Range:
		var items []Object
		start := o.Start
		end := o.End
		next := o.Next

		var current int64
		var endVal int64
		var step int64 = 1
		isChar := false
		isNumeric := false

		if sInt, ok := start.(*Integer); ok {
			if eInt, ok := end.(*Integer); ok {
				current = sInt.Value
				endVal = eInt.Value
				isNumeric = true
				if nInt, ok := next.(*Integer); ok {
					step = nInt.Value - current
				}
			}
		} else if sChar, ok := start.(*Char); ok {
			if eChar, ok := end.(*Char); ok {
				current = sChar.Value
				endVal = eChar.Value
				isChar = true
				isNumeric = true
				if nChar, ok := next.(*Char); ok {
					step = nChar.Value - current
				}
			}
		}

		if isNumeric {
			for {
				if step > 0 {
					if current > endVal {
						break
					}
				} else {
					if current < endVal {
						break
					}
				}

				var item Object
				if isChar {
					item = &Char{Value: current}
				} else {
					item = &Integer{Value: current}
				}
				items = append(items, item)

				current += step
			}
			return items
		}
		return nil
	default:
		return nil
	}
}

// bindPattern binds a pattern to a value in the given environment
// Returns true if the pattern matches, false otherwise
func (e *Evaluator) bindPattern(pattern ast.Pattern, value Object, env *Environment) bool {
	switch p := pattern.(type) {
	case *ast.IdentifierPattern:
		if p.Value != "_" {
			env.Set(p.Value, value)
		}
		return true

	case *ast.WildcardPattern:
		return true

	case *ast.TuplePattern:
		tuple, ok := value.(*Tuple)
		if !ok || len(tuple.Elements) != len(p.Elements) {
			return false
		}
		for i, elem := range p.Elements {
			if !e.bindPattern(elem, tuple.Elements[i], env) {
				return false
			}
		}
		return true

	case *ast.ListPattern:
		list, ok := value.(*List)
		if !ok {
			return false
		}
		elements := list.ToSlice()
		if len(elements) != len(p.Elements) {
			return false
		}
		for i, elem := range p.Elements {
			if !e.bindPattern(elem, elements[i], env) {
				return false
			}
		}
		return true

	case *ast.LiteralPattern:
		// Compare literal values
		switch v := p.Value.(type) {
		case int64:
			if intVal, ok := value.(*Integer); ok {
				return intVal.Value == v
			}
		case bool:
			if boolVal, ok := value.(*Boolean); ok {
				return boolVal.Value == v
			}
		case string:
			if strVal, ok := value.(*List); ok && strVal.ElementType == "Char" {
				return listToString(strVal) == v
			}
		}
		return false

	default:
		return false
	}
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
	spreadFieldCount := 0
	var baseRec *RecordInstance

	// Handle spread expression first: { ...base, key: val }
	if node.Spread != nil {
		spreadVal := e.Eval(node.Spread, env)
		if isError(spreadVal) {
			return spreadVal
		}
		// Spread value must be a record
		if rec, ok := spreadVal.(*RecordInstance); ok {
			baseRec = rec
			// Copy all fields from the spread base
			for _, f := range rec.Fields {
				fields[f.Key] = f.Value
				spreadFieldCount++
			}
			// Only preserve TypeName if no explicit fields are added
			// If we're adding/overriding fields, we'll check later if they're new
			if len(node.Fields) == 0 {
				typeName = rec.TypeName
			}
		} else {
			return newError("spread expression must evaluate to a record, got %s", spreadVal.Type())
		}
	}

	// Override/add fields from explicit field definitions
	addedNewField := false
	for k, v := range node.Fields {
		val := e.Eval(v, env)
		if isError(val) {
			return val
		}
		// Check if this is a new field (not in spread base)
		if baseRec != nil {
			if _, exists := fields[k]; !exists {
				addedNewField = true
			}
		}
		fields[k] = val
	}

	newRec := NewRecord(fields)

	// Mark as Row Polymorphism extension if we added NEW fields via spread
	// If we only override existing fields, preserve TypeName
	if baseRec != nil && addedNewField {
		newRec.RowPolyExtended = true
	} else if baseRec != nil && !addedNewField {
		// Only override existing fields - preserve TypeName from spread base
		newRec.TypeName = baseRec.TypeName
		typeName = baseRec.TypeName // Also set typeName so it's not overwritten below
	}

	// If no typeName from spread (or spread was cleared), try to get it from TypeMap
	// (nominal typing for record aliases).
	// Also, if TypeMap contains a structural type (TRecord), don't use nominal type from it.
	if typeName == "" && !newRec.RowPolyExtended && e.TypeMap != nil {
		if inferredType := e.TypeMap[node]; inferredType != nil {
			// Only use nominal type if TypeMap contains TCon (not TRecord)
			// This ensures Row Polymorphism works: structural types don't get converted to nominal
			if _, isRecord := inferredType.(typesystem.TRecord); !isRecord {
				typeName = ExtractTypeConstructorName(inferredType)
			}
		}
	}

	// Only set TypeName if it wasn't already set from baseRec
	if newRec.TypeName == "" {
		newRec.TypeName = typeName
	}
	return newRec
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
	// Format string value: e.g., ".2f" or "%s, %d"
	// For short form (no % in string), prepend % to make it a valid format specifier
	// For full form (contains %), use as-is
	fmtStr := node.Value
	if !strings.Contains(fmtStr, "%") {
		fmtStr = "%" + fmtStr
	}

	// Return a variadic builtin function that calls sprintf with captured format string
	return &Builtin{
		Name: "formatter",
		Fn: func(eval *Evaluator, args ...Object) Object {
			// Get sprintf builtin
			sprintf, ok := Builtins["sprintf"]
			if !ok {
				return newError("internal error: sprintf not found")
			}
			// Prepend format string to args
			allArgs := make([]Object, 0, len(args)+1)
			allArgs = append(allArgs, StringToList(fmtStr))
			allArgs = append(allArgs, args...)
			return sprintf.Fn(eval, allArgs...)
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
