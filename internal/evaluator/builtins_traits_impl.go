package evaluator

import (
	"math/big"
)

// RegisterBasicTraits registers basic traits like Equal, Order, Numeric, etc.
// for primitive types.
func RegisterBasicTraits(e *Evaluator, env *Environment) {
	registerEqualInstances(e)
	registerOrderInstances(e)
	registerNumericInstances(e)
	registerBitwiseInstances(e)
	registerConcatInstances(e)
	registerDefaultInstances(e)
}

func registerEqualInstances(e *Evaluator) {
	// (==) and (!=)
	// We use e.areObjectsEqual for implementation

	equalFn := func(eval *Evaluator, args ...Object) Object {
		if len(args) != 2 {
			return newError("(==) expects 2 arguments")
		}
		left, right := args[0], args[1]

		// Implicit Int -> Float conversion for equality
		if left.Type() == INTEGER_OBJ && right.Type() == FLOAT_OBJ {
			leftVal := float64(left.(*Integer).Value)
			return eval.nativeBoolToBooleanObject(leftVal == right.(*Float).Value)
		}
		if left.Type() == FLOAT_OBJ && right.Type() == INTEGER_OBJ {
			rightVal := float64(right.(*Integer).Value)
			return eval.nativeBoolToBooleanObject(left.(*Float).Value == rightVal)
		}

		return eval.nativeBoolToBooleanObject(eval.areObjectsEqual(left, right))
	}

	notEqualFn := func(eval *Evaluator, args ...Object) Object {
		if len(args) != 2 {
			return newError("(!=) expects 2 arguments")
		}
		left, right := args[0], args[1]

		// Implicit Int -> Float conversion for inequality
		if left.Type() == INTEGER_OBJ && right.Type() == FLOAT_OBJ {
			leftVal := float64(left.(*Integer).Value)
			return eval.nativeBoolToBooleanObject(leftVal != right.(*Float).Value)
		}
		if left.Type() == FLOAT_OBJ && right.Type() == INTEGER_OBJ {
			rightVal := float64(right.(*Integer).Value)
			return eval.nativeBoolToBooleanObject(left.(*Float).Value != rightVal)
		}

		return eval.nativeBoolToBooleanObject(!eval.areObjectsEqual(left, right))
	}

	methods := map[string]Object{
		"(==)": &Builtin{Name: "(==)", Fn: equalFn},
		"(!=)": &Builtin{Name: "(!=)", Fn: notEqualFn},
	}

	types := []string{"Int", "Float", "Bool", "Char", "String", "BigInt", "Rational", "Bytes", "Bits", "Uuid", "Nil"}
	for _, t := range types {
		if _, ok := e.ClassImplementations["Equal"]; !ok {
			e.ClassImplementations["Equal"] = make(map[string]Object)
		}
		e.ClassImplementations["Equal"][t] = &MethodTable{Methods: methods}
	}

	// Register generic instances handled by runtime reflection or builtins?
	// List, Map, Tuple, Result, Option are generic.
	// areObjectsEqual handles them recursively.
	// So we can register the same implementation for them.
	genericTypes := []string{"List", "Map", "Option", "Result", "Tuple"}
	for _, t := range genericTypes {
		e.ClassImplementations["Equal"][t] = &MethodTable{Methods: methods}
	}
}

func registerOrderInstances(e *Evaluator) {
	// (<), (>), (<=), (>=)
	// We need compareValues

	compareOp := func(op string) func(*Evaluator, ...Object) Object {
		return func(eval *Evaluator, args ...Object) Object {
			if len(args) != 2 {
				return newError("%s expects 2 arguments", op)
			}
			left, right := args[0], args[1]

			// Direct dispatch to avoid recursion loop via EvalInfixExpression -> tryUserDefinedOperator
			if left.Type() == INTEGER_OBJ && right.Type() == INTEGER_OBJ {
				return eval.evalIntegerInfixExpression(op, left, right)
			}
			if left.Type() == FLOAT_OBJ && right.Type() == FLOAT_OBJ {
				return eval.evalFloatInfixExpression(op, left, right)
			}
			if left.Type() == BOOLEAN_OBJ && right.Type() == BOOLEAN_OBJ {
				return eval.evalBooleanInfixExpression(op, left, right)
			}
			if left.Type() == CHAR_OBJ && right.Type() == CHAR_OBJ {
				return eval.evalCharInfixExpression(op, left, right)
			}
			if left.Type() == LIST_OBJ && right.Type() == LIST_OBJ {
				return eval.evalListInfixExpression(op, left, right)
			}
			if left.Type() == BIG_INT_OBJ && right.Type() == BIG_INT_OBJ {
				return eval.evalBigIntInfixExpression(op, left, right)
			}
			if left.Type() == RATIONAL_OBJ && right.Type() == RATIONAL_OBJ {
				return eval.evalRationalInfixExpression(op, left, right)
			}
			if left.Type() == BYTES_OBJ && right.Type() == BYTES_OBJ {
				return eval.evalBytesInfixExpression(op, left, right)
			}
			if left.Type() == TUPLE_OBJ && right.Type() == TUPLE_OBJ {
				return eval.evalTupleInfixExpression(op, left, right)
			}

			// Handle implicit Int -> Float conversion for comparison
			if left.Type() == INTEGER_OBJ && right.Type() == FLOAT_OBJ {
				leftVal := float64(left.(*Integer).Value)
				return eval.evalFloatInfixExpression(op, &Float{Value: leftVal}, right)
			}
			if left.Type() == FLOAT_OBJ && right.Type() == INTEGER_OBJ {
				rightVal := float64(right.(*Integer).Value)
				return eval.evalFloatInfixExpression(op, left, &Float{Value: rightVal})
			}

			return newError("type mismatch in comparison %s: %s vs %s", op, left.Type(), right.Type())
		}
	}

	methods := map[string]Object{
		"(<)":  &Builtin{Name: "(<)", Fn: compareOp("<")},
		"(>)":  &Builtin{Name: "(>)", Fn: compareOp(">")},
		"(<=)": &Builtin{Name: "(<=)", Fn: compareOp("<=")},
		"(>=)": &Builtin{Name: "(>=)", Fn: compareOp(">=")},
	}

	// Order is implemented for primitives + String + Bytes
	types := []string{"Int", "Float", "Bool", "Char", "String", "BigInt", "Rational", "Bytes"}
	for _, t := range types {
		if _, ok := e.ClassImplementations["Order"]; !ok {
			e.ClassImplementations["Order"] = make(map[string]Object)
		}
		e.ClassImplementations["Order"][t] = &MethodTable{Methods: methods}
	}

	// List and Tuple implement Order lexicographically?
	// EvalInfixExpression/CompareValues supports them.
	e.ClassImplementations["Order"]["List"] = &MethodTable{Methods: methods}
	e.ClassImplementations["Order"]["Tuple"] = &MethodTable{Methods: methods}
}

func registerNumericInstances(e *Evaluator) {
	// (+), (-), (*), (/), (%), (**)
	ops := []string{"+", "-", "*", "/", "%", "**"}
	methods := make(map[string]Object)

	for _, op := range ops {
		opName := "(" + op + ")"
		methods[opName] = &Builtin{
			Name: opName,
			Fn: func(eval *Evaluator, args ...Object) Object {
				if len(args) != 2 {
					return newError("%s expects 2 arguments", op)
				}
				left, right := args[0], args[1]

				// Direct dispatch to avoid recursion loop via EvalInfixExpression -> tryUserDefinedOperator
				if left.Type() == INTEGER_OBJ && right.Type() == INTEGER_OBJ {
					return eval.evalIntegerInfixExpression(op, left, right)
				}
				if left.Type() == FLOAT_OBJ && right.Type() == FLOAT_OBJ {
					return eval.evalFloatInfixExpression(op, left, right)
				}
				if left.Type() == BIG_INT_OBJ && right.Type() == BIG_INT_OBJ {
					return eval.evalBigIntInfixExpression(op, left, right)
				}
				if left.Type() == RATIONAL_OBJ && right.Type() == RATIONAL_OBJ {
					return eval.evalRationalInfixExpression(op, left, right)
				}

				// Handle implicit Int -> Float conversion
				if left.Type() == INTEGER_OBJ && right.Type() == FLOAT_OBJ {
					leftVal := float64(left.(*Integer).Value)
					return eval.evalFloatInfixExpression(op, &Float{Value: leftVal}, right)
				}
				if left.Type() == FLOAT_OBJ && right.Type() == INTEGER_OBJ {
					rightVal := float64(right.(*Integer).Value)
					return eval.evalFloatInfixExpression(op, left, &Float{Value: rightVal})
				}

				return newError("type mismatch in numeric operation %s: %s vs %s", op, left.Type(), right.Type())
			},
		}
	}

	types := []string{"Int", "Float", "BigInt", "Rational"}
	for _, t := range types {
		if _, ok := e.ClassImplementations["Numeric"]; !ok {
			e.ClassImplementations["Numeric"] = make(map[string]Object)
		}
		e.ClassImplementations["Numeric"][t] = &MethodTable{Methods: methods}
	}
}

func registerBitwiseInstances(e *Evaluator) {
	// (&), (|), (^), (<<), (>>)
	ops := []string{"&", "|", "^", "<<", ">>"}
	methods := make(map[string]Object)

	for _, op := range ops {
		opName := "(" + op + ")"
		methods[opName] = &Builtin{
			Name: opName,
			Fn: func(eval *Evaluator, args ...Object) Object {
				if len(args) != 2 {
					return newError("%s expects 2 arguments", op)
				}
				left, right := args[0], args[1]

				// Direct dispatch to avoid recursion loop
				if left.Type() == INTEGER_OBJ && right.Type() == INTEGER_OBJ {
					return eval.evalIntegerInfixExpression(op, left, right)
				}
				if left.Type() == BIG_INT_OBJ && right.Type() == BIG_INT_OBJ {
					return eval.evalBigIntInfixExpression(op, left, right)
				}

				return newError("type mismatch in bitwise operation %s: %s vs %s", op, left.Type(), right.Type())
			},
		}
	}

	types := []string{"Int", "BigInt"} // Bitwise usually only for Integers
	for _, t := range types {
		if _, ok := e.ClassImplementations["Bitwise"]; !ok {
			e.ClassImplementations["Bitwise"] = make(map[string]Object)
		}
		e.ClassImplementations["Bitwise"][t] = &MethodTable{Methods: methods}
	}
}

func registerConcatInstances(e *Evaluator) {
	// (++)
	methods := map[string]Object{
		"(++)": &Builtin{
			Name: "(++)",
			Fn: func(eval *Evaluator, args ...Object) Object {
				if len(args) != 2 {
					return newError("(++) expects 2 arguments")
				}
				left, right := args[0], args[1]

				// Direct dispatch to avoid recursion loop via EvalInfixExpression
				if left.Type() == LIST_OBJ && right.Type() == LIST_OBJ {
					return eval.evalListInfixExpression("++", left, right)
				}
				if left.Type() == BYTES_OBJ && right.Type() == BYTES_OBJ {
					return eval.evalBytesInfixExpression("++", left, right)
				}
				if left.Type() == BITS_OBJ && right.Type() == BITS_OBJ {
					return eval.evalBitsInfixExpression("++", left, right)
				}

				return newError("type mismatch in concatenation ++: %s vs %s", left.Type(), right.Type())
			},
		},
	}

	// List (and String), Bytes, Bits support concat
	types := []string{"List", "String", "Bytes", "Bits"}
	for _, t := range types {
		if _, ok := e.ClassImplementations["Concat"]; !ok {
			e.ClassImplementations["Concat"] = make(map[string]Object)
		}
		e.ClassImplementations["Concat"][t] = &MethodTable{Methods: methods}
	}
}

func registerDefaultInstances(e *Evaluator) {
	// default/getDefault :: () -> T
	// We need to implement this for each type.
	// Since we are registering instances, we can provide a specific function for each.

	if _, ok := e.ClassImplementations["Default"]; !ok {
		e.ClassImplementations["Default"] = make(map[string]Object)
	}

	reg := func(typeName string, val Object) {
		m := map[string]Object{
			"default": &Builtin{
				Name: "default",
				Fn:   func(eval *Evaluator, args ...Object) Object { return val },
			},
			"getDefault": &Builtin{
				Name: "getDefault",
				Fn:   func(eval *Evaluator, args ...Object) Object { return val },
			},
		}
		e.ClassImplementations["Default"][typeName] = &MethodTable{Methods: m}
	}

	reg("Int", &Integer{Value: 0})
	reg("Float", &Float{Value: 0.0})
	reg("Bool", &Boolean{Value: false})
	reg("Char", &Char{Value: 0})
	reg("BigInt", &BigInt{Value: big.NewInt(0)})
	reg("Rational", &Rational{Value: big.NewRat(0, 1)})
	reg("Nil", &Nil{})

	// For generic types, we need a constructor that might use inner defaults?
	// But `default` is usually `default<T>()`.
	// For List, default is empty list.
	reg("List", newList([]Object{}))
	reg("String", newList([]Object{})) // Empty string
	reg("Map", NewMap())
	reg("Option", makeZero()) // None
	// Result default? Usually not defined, or Error?
	// Maybe Default is not implemented for Result generally unless E has default?
	// Skip Result for now.
}
