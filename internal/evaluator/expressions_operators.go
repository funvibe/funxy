package evaluator

import (
	"math"
	"math/big"

	"github.com/funvibe/funxy/internal/config"
)

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

	// Implicit Int -> Float conversion
	if left.Type() == INTEGER_OBJ && right.Type() == FLOAT_OBJ {
		leftVal := float64(left.(*Integer).Value)
		return e.evalFloatInfixExpression(operator, &Float{Value: leftVal}, right)
	}
	if left.Type() == FLOAT_OBJ && right.Type() == INTEGER_OBJ {
		rightVal := float64(right.(*Integer).Value)
		return e.evalFloatInfixExpression(operator, left, &Float{Value: rightVal})
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
		// Implicit Int -> Float conversion for equality
		if left.Type() == INTEGER_OBJ && right.Type() == FLOAT_OBJ {
			leftVal := float64(left.(*Integer).Value)
			return e.nativeBoolToBooleanObject(leftVal == right.(*Float).Value)
		}
		if left.Type() == FLOAT_OBJ && right.Type() == INTEGER_OBJ {
			rightVal := float64(right.(*Integer).Value)
			return e.nativeBoolToBooleanObject(left.(*Float).Value == rightVal)
		}
		return e.nativeBoolToBooleanObject(e.areObjectsEqual(left, right))
	}
	if operator == "!=" {
		// Implicit Int -> Float conversion for inequality
		if left.Type() == INTEGER_OBJ && right.Type() == FLOAT_OBJ {
			leftVal := float64(left.(*Integer).Value)
			return e.nativeBoolToBooleanObject(leftVal != right.(*Float).Value)
		}
		if left.Type() == FLOAT_OBJ && right.Type() == INTEGER_OBJ {
			rightVal := float64(right.(*Integer).Value)
			return e.nativeBoolToBooleanObject(left.(*Float).Value != rightVal)
		}
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
	// First, try dispatch using exact runtime type (e.g. "String" if left is a string)
	typeName := getRuntimeTypeName(left)
	methodName := "(" + operator + ")"

	findImplementation := func(targetType string) Object {
		// Search all traits for this operator
		for _, typesMap := range e.ClassImplementations {
			if methodTableObj, ok := typesMap[targetType]; ok {
				if methodTable, ok := methodTableObj.(*MethodTable); ok {
					if method, ok := methodTable.Methods[methodName]; ok {
						oldContainer := e.ContainerContext
						e.ContainerContext = targetType
						defer func() { e.ContainerContext = oldContainer }()
						return e.ApplyFunction(method, []Object{left, right})
					}
				}
			}
		}
		return nil
	}

	// Attempt 1: Exact type (Nominal)
	if res := findImplementation(typeName); res != nil {
		return res
	}

	// Attempt 2: General Alias Fallback
	if e.TypeAliases != nil {
		if underlying, ok := e.TypeAliases[typeName]; ok {
			// Use helper that takes typesystem.Type
			underlyingName := ExtractTypeConstructorName(underlying)
			if underlyingName != "" && underlyingName != typeName {
				if res := findImplementation(underlyingName); res != nil {
					return res
				}
			}
		}
	}

	// Attempt 3: Expected type context (from annotations or TypeMap)
	contextTypeName := ""
	if len(e.TypeContextStack) > 0 {
		contextTypeName = e.TypeContextStack[len(e.TypeContextStack)-1]
	} else if e.TypeMap != nil && e.CurrentCallNode != nil {
		if t := e.TypeMap[e.CurrentCallNode]; t != nil {
			contextTypeName = ExtractTypeConstructorName(t)
		}
	}

	if contextTypeName != "" && contextTypeName != typeName {
		if res := findImplementation(contextTypeName); res != nil {
			return res
		}
		if e.TypeAliases != nil {
			if underlying, ok := e.TypeAliases[contextTypeName]; ok {
				underlyingName := ExtractTypeConstructorName(underlying)
				if underlyingName != "" && underlyingName != contextTypeName {
					if res := findImplementation(underlyingName); res != nil {
						return res
					}
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

	// Fallback to expected type context (from annotations or TypeMap)
	contextTypeName := ""
	if len(e.TypeContextStack) > 0 {
		contextTypeName = e.TypeContextStack[len(e.TypeContextStack)-1]
	} else if e.TypeMap != nil && e.CurrentCallNode != nil {
		if t := e.TypeMap[e.CurrentCallNode]; t != nil {
			contextTypeName = ExtractTypeConstructorName(t)
		}
	}

	if contextTypeName != "" && contextTypeName != typeName {
		if typesMap, ok := e.ClassImplementations[traitName]; ok {
			if methodTableObj, ok := typesMap[contextTypeName]; ok {
				if methodTable, ok := methodTableObj.(*MethodTable); ok {
					if method, ok := methodTable.Methods[methodName]; ok {
						oldContainer := e.ContainerContext
						e.ContainerContext = contextTypeName
						defer func() { e.ContainerContext = oldContainer }()
						return e.ApplyFunction(method, []Object{left, right})
					}
				}
			}
			if e.TypeAliases != nil {
				if underlying, ok := e.TypeAliases[contextTypeName]; ok {
					underlyingName := ExtractTypeConstructorName(underlying)
					if underlyingName != "" && underlyingName != contextTypeName {
						if methodTableObj, ok := typesMap[underlyingName]; ok {
							if methodTable, ok := methodTableObj.(*MethodTable); ok {
								if method, ok := methodTable.Methods[methodName]; ok {
									oldContainer := e.ContainerContext
									e.ContainerContext = underlyingName
									defer func() { e.ContainerContext = oldContainer }()
									return e.ApplyFunction(method, []Object{left, right})
								}
							}
						}
					}
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
		if rightVal == 0 {
			err := e.newErrorWithStack("division by zero")
			// Match VM behavior: VM reports column 0 for division by zero (likely due to instruction encoding)
			// TreeWalk has accurate column from AST, but we suppress it to match VM output in tests.
			if len(e.CallStack) > 0 {
				err.Line = e.CallStack[len(e.CallStack)-1].Line
				err.Column = 0
			}
			return err
		}
		return &Integer{Value: leftVal / rightVal}
	case "%":
		if rightVal == 0 {
			err := e.newErrorWithStack("modulo by zero")
			if len(e.CallStack) > 0 {
				err.Line = e.CallStack[len(e.CallStack)-1].Line
				err.Column = 0
			}
			return err
		}
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
		if rightVal == 0.0 {
			err := e.newErrorWithStack("division by zero")
			if len(e.CallStack) > 0 {
				err.Line = e.CallStack[len(e.CallStack)-1].Line
				err.Column = 0
			}
			return err
		}
		return &Float{Value: leftVal / rightVal}
	case "%":
		if rightVal == 0.0 {
			err := e.newErrorWithStack("modulo by zero")
			if len(e.CallStack) > 0 {
				err.Line = e.CallStack[len(e.CallStack)-1].Line
				err.Column = 0
			}
			return err
		}
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
			err := e.newErrorWithStack("division by zero")
			if len(e.CallStack) > 0 {
				err.Line = e.CallStack[len(e.CallStack)-1].Line
				err.Column = 0
			}
			return err
		}
		return &BigInt{Value: new(big.Int).Div(leftVal, rightVal)}
	case "%":
		if rightVal.Sign() == 0 {
			err := e.newErrorWithStack("modulo by zero")
			if len(e.CallStack) > 0 {
				err.Line = e.CallStack[len(e.CallStack)-1].Line
				err.Column = 0
			}
			return err
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
			err := e.newErrorWithStack("division by zero")
			if len(e.CallStack) > 0 {
				err.Line = e.CallStack[len(e.CallStack)-1].Line
				err.Column = 0
			}
			return err
		}
		return &Rational{Value: new(big.Rat).Quo(leftVal, rightVal)}
	case "%":
		if rightVal.Sign() == 0 {
			err := e.newErrorWithStack("modulo by zero")
			if len(e.CallStack) > 0 {
				err.Line = e.CallStack[len(e.CallStack)-1].Line
				err.Column = 0
			}
			return err
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

	// Compare Options (None < Some)
	if leftData, ok := left.(*DataInstance); ok {
		if rightData, ok := right.(*DataInstance); ok {
			if leftData.TypeName == config.OptionTypeName && rightData.TypeName == config.OptionTypeName {
				// None < Some
				if leftData.Name == config.NoneCtorName && rightData.Name == config.SomeCtorName {
					return -1
				} else if leftData.Name == config.SomeCtorName && rightData.Name == config.NoneCtorName {
					return 1
				} else if leftData.Name == config.NoneCtorName && rightData.Name == config.NoneCtorName {
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
				} else if data.Name == config.NoneCtorName {
					return &ReturnValue{Value: left}
				}
			}
		}
		return newError("operator ? not supported for %s", left.Inspect())
	default:
		return newError("unknown operator: %s", operator)
	}
}
