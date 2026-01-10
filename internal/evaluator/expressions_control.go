package evaluator

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/typesystem"
)

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

			// Handle case where Pattern is nil (just ...xs)
			if spreadPat.Pattern == nil {
				return true, bindings
			}

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

					// Handle case where Pattern is nil (just ...xs)
					if spreadPat.Pattern == nil {
						return true, bindings
					}

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

			// Handle case where Pattern is nil (just ...xs)
			if spreadPat.Pattern == nil {
				return true, bindings
			}

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

		// First check direct type match
		if e.matchesDirectType(val, typeName) {
			return true
		}

		// Then check aliases systemically
		if e.TypeAliases != nil {
			if alias, ok := e.TypeAliases[typeName]; ok {
				aliasName := ExtractTypeConstructorName(alias)
				if aliasName != "" && e.matchesDirectType(val, aliasName) {
					return true
				}
				// Handle structural aliases (Function, Tuple, Record)
				switch t := alias.(type) {
				case typesystem.TFunc:
					// Check if value is function-like
					switch val.(type) {
					case *Function, *Builtin, *ClassMethod, *BoundMethod, *OperatorFunction, *ComposedFunction, *PartialApplication:
						return true
					}
				case typesystem.TTuple:
					if tuple, ok := val.(*Tuple); ok {
						if len(tuple.Elements) == len(t.Elements) {
							return true
						}
					}
				case typesystem.TRecord:
					if rec, ok := val.(*RecordInstance); ok {
						match := true
						for fieldName := range t.Fields {
							if rec.Get(fieldName) == nil {
								match = false
								break
							}
						}
						if match {
							return true
						}
					}
				}
			}
		}

		return false
	case *ast.UnionType:
		// Union type: check if val matches any member
		for _, member := range t.Types {
			if e.matchesType(val, member) {
				return true
			}
		}
		return false
	case *ast.FunctionType:
		// Check if value is a function-like object
		switch val.(type) {
		case *Function, *Builtin, *ClassMethod, *BoundMethod, *OperatorFunction, *ComposedFunction, *PartialApplication:
			return true
		default:
			return false
		}
	case *ast.ForallType:
		// Check if the underlying type matches
		// We ignore the type parameters in runtime check for now (erasure)
		return e.matchesType(val, t.Type)
	case *ast.TupleType:
		if tuple, ok := val.(*Tuple); ok {
			// Check element count matches
			return len(tuple.Elements) == len(t.Types)
		}
		return false
	case *ast.RecordType:
		_, ok := val.(*RecordInstance)
		return ok
	default:
		return false
	}
}

func (e *Evaluator) matchesDirectType(val Object, typeName string) bool {
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
}
