package analyzer

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

// getOptionalInnerType extracts the "inner type" from a type that implements Optional.
// Uses the trait's unwrap method signature to derive this properly.
func getOptionalInnerType(t typesystem.Type, table *symbols.SymbolTable) (typesystem.Type, bool) {
	return table.GetOptionalUnwrapReturnType(t)
}

// typeHasConstraint checks if a type (typically TVar/TCon) has a constraint in the current context
func typeHasConstraint(ctx *InferenceContext, t typesystem.Type, traitName string, table *symbols.SymbolTable) bool {
	var name string
	switch tv := t.(type) {
	case typesystem.TVar:
		name = tv.Name
	case typesystem.TCon:
		// TCon with same name as a constrained type param (used in function bodies)
		name = tv.Name
	default:
		return false
	}

	if ctx.HasConstraint(name, traitName) {
		return true
	}

	// Check super traits
	if constraints, ok := ctx.ActiveConstraints[name]; ok {
		for _, c := range constraints {
			if isTraitSubclass(c.Trait, traitName, table) {
				return true
			}
		}
	}
	return false
}

// eitherHasConstraint checks if either of two types has the trait (via implementation or constraint)
// This is needed because in expressions like `x < pivot` inside a lambda, `x` may be a fresh TVar
// while `pivot` has the constraint from the outer function.
func eitherHasConstraint(ctx *InferenceContext, table *symbols.SymbolTable, l, r typesystem.Type, traitName string) bool {
	// First check local implementations
	// Unwrap type aliases before checking to handle cases with type aliases
	unwrappedL := typesystem.UnwrapUnderlying(l)
	if unwrappedL == nil {
		unwrappedL = l
	}
	unwrappedR := typesystem.UnwrapUnderlying(r)
	if unwrappedR == nil {
		unwrappedR = r
	}
	if table.IsImplementationExists(traitName, []typesystem.Type{unwrappedL}) ||
		table.IsImplementationExists(traitName, []typesystem.Type{unwrappedR}) {
		return true
	}

	// Then check in source modules (for types with module prefix)
	if isImplementationInSourceModule(ctx, traitName, l, table) ||
		isImplementationInSourceModule(ctx, traitName, r, table) {
		return true
	}

	// Finally check constraints
	return typeHasConstraint(ctx, l, traitName, table) ||
		typeHasConstraint(ctx, r, traitName, table)
}

// getTraitForOp returns the expected trait name for an operator (for error messages)
func getTraitForOp(op string) string {
	switch op {
	case "+":
		return "Add"
	case "-":
		return "Sub"
	case "*":
		return "Mul"
	case "/":
		return "Div"
	case "==", "!=":
		return "Equal"
	case "<", ">", "<=", ">=":
		return "Order"
	default:
		return "unknown"
	}
}

func inferPrefixExpression(ctx *InferenceContext, n *ast.PrefixExpression, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	right, s1, err := inferFn(n.Right, table)
	if err != nil {
		return nil, nil, err
	}
	totalSubst := s1
	right = right.Apply(ctx.GlobalSubst).Apply(totalSubst)

	switch n.Operator {
	case "-":
		if subst, err := typesystem.Unify(right, typesystem.Int); err == nil {
			totalSubst = subst.Compose(totalSubst)
			return typesystem.Int, totalSubst, nil
		} else if subst, err := typesystem.Unify(right, typesystem.Float); err == nil {
			totalSubst = subst.Compose(totalSubst)
			return typesystem.Float, totalSubst, nil
		} else if subst, err := typesystem.Unify(right, typesystem.BigInt); err == nil {
			totalSubst = subst.Compose(totalSubst)
			return typesystem.BigInt, totalSubst, nil
		} else if subst, err := typesystem.Unify(right, typesystem.Rational); err == nil {
			totalSubst = subst.Compose(totalSubst)
			return typesystem.Rational, totalSubst, nil
		} else {
			return nil, nil, inferErrorf(n, "operator '-' expects Int, Float, BigInt or Rational, got %s", right)
		}
	case "!":
		subst, err := typesystem.Unify(right, typesystem.Bool)
		if err != nil {
			return nil, nil, inferErrorf(n, "operator '!' expects Bool, got %s", right)
		}
		totalSubst = subst.Compose(totalSubst)
		return typesystem.Bool, totalSubst, nil
	case "~":
		subst, err := typesystem.Unify(right, typesystem.Int)
		if err != nil {
			return nil, nil, inferErrorf(n, "operator '~' expects Int, got %s", right)
		}
		totalSubst = subst.Compose(totalSubst)
		return typesystem.Int, totalSubst, nil
	default:
		return nil, nil, inferErrorf(n, "unknown prefix operator: %s", n.Operator)
	}
}

func inferInfixExpression(ctx *InferenceContext, n *ast.InfixExpression, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	l, s1, err := inferFn(n.Left, table)
	if err != nil {
		return nil, nil, err
	}
	totalSubst := s1
	l = l.Apply(ctx.GlobalSubst).Apply(totalSubst)

	// Contextual Typing: Try to determine expected type for Right operand based on Left and Operator
	// This helps with lambda inference (e.g. pure(10) >>= fun(x) -> ...)
	traitName, hasTrait := table.GetTraitForOperator(n.Operator)
	isHKT := hasTrait && table.IsHKTTrait(traitName)
	if isHKT {
		// Logic similar to inferHKTOperator but just for context propagation
		methodName := "(" + n.Operator + ")"
		if sym, ok := table.Find(methodName); ok {
			if methodType, ok := sym.Type.(typesystem.TFunc); ok && len(methodType.Params) == 2 {
				// Instantiate method
				freshMethodType := InstantiateWithContext(ctx, methodType).(typesystem.TFunc)
				// Unify first param with Left to constrain type vars
				// We use a temporary subst to avoid polluting the main inference if it fails
				// (Validation happens properly later in inferHKTOperator)
				if subst, err := typesystem.Unify(freshMethodType.Params[0], l); err == nil {
					// Apply subst to second param to get expected type for Right
					expectedRight := freshMethodType.Params[1].Apply(subst)

					if ctx.ExpectedTypes == nil {
						ctx.ExpectedTypes = make(map[ast.Node]typesystem.Type)
					}
					ctx.ExpectedTypes[n.Right] = expectedRight

					// Merge substitution from Contextual Typing to preserve learned type information
					// (e.g. M -> OptionT) which might be needed for deferred constraints.
					// We must update both GlobalSubst (for recursive calls) and totalSubst (for local flow).
					ctx.GlobalSubst = subst.Compose(ctx.GlobalSubst)
					totalSubst = subst.Compose(totalSubst)
					l = l.Apply(totalSubst)
				}
			}
		}
	}

	if n.Operator == "|>" {
		// Pipe operator needs special handling to support placeholders in arguments
		// e.g. x |> f(_, y)
		// We delegate to inferPipeExpression BEFORE inferring n.Right
		return inferPipeExpression(ctx, n, table, inferFn)
	}

	r, s2, err := inferFn(n.Right, table)
	if err != nil {
		return nil, nil, err
	}
	totalSubst = s2.Compose(totalSubst)

	l = l.Apply(ctx.GlobalSubst).Apply(totalSubst)
	r = r.Apply(ctx.GlobalSubst).Apply(totalSubst)

	switch n.Operator {
	case "+", "-", "*", "/", "%", "**":
		// First, check if there's a trait implementation for this operator on the type
		if traitName, ok := table.GetTraitForOperator(n.Operator); ok {
			if eitherHasConstraint(ctx, table, l, r, traitName) {
				// Both operands should have the same type
				subst, err := typesystem.Unify(l, r)
				if err != nil {
					// Implicit conversion check
					isLInt := false
					if _, err := typesystem.Unify(l, typesystem.Int); err == nil {
						isLInt = true
					}
					isLFloat := false
					if _, err := typesystem.Unify(l, typesystem.Float); err == nil {
						isLFloat = true
					}
					isRInt := false
					if _, err := typesystem.Unify(r, typesystem.Int); err == nil {
						isRInt = true
					}
					isRFloat := false
					if _, err := typesystem.Unify(r, typesystem.Float); err == nil {
						isRFloat = true
					}

					if (isLInt && isRFloat) || (isLFloat && isRInt) {
						return typesystem.Float, totalSubst, nil
					}

					return nil, nil, inferErrorf(n, "type mismatch in %s: %s vs %s", n.Operator, l, r)
				}
				totalSubst = subst.Compose(totalSubst)
				// Return the unified type
				return l.Apply(totalSubst), totalSubst, nil
			}
		}

		// Fallback to built-in numeric types
		if subst, err := typesystem.Unify(l, typesystem.Int); err == nil {
			totalSubst = subst.Compose(totalSubst)
			// Apply to r
			r = r.Apply(totalSubst)
			if subst2, err := typesystem.Unify(r, typesystem.Int); err == nil {
				totalSubst = subst2.Compose(totalSubst)
				return typesystem.Int, totalSubst, nil
			} else if subst2, err := typesystem.Unify(r, typesystem.Float); err == nil {
				// Int + Float -> Float
				totalSubst = subst2.Compose(totalSubst)
				return typesystem.Float, totalSubst, nil
			} else {
				return nil, nil, inferErrorf(n.Right, "right operand of %s must be Int or Float, got %s", n.Operator, r)
			}
		} else if subst, err := typesystem.Unify(l, typesystem.Float); err == nil {
			totalSubst = subst.Compose(totalSubst)
			r = r.Apply(totalSubst)
			if subst2, err := typesystem.Unify(r, typesystem.Float); err == nil {
				totalSubst = subst2.Compose(totalSubst)
				return typesystem.Float, totalSubst, nil
			} else if subst2, err := typesystem.Unify(r, typesystem.Int); err == nil {
				// Float + Int -> Float
				totalSubst = subst2.Compose(totalSubst)
				return typesystem.Float, totalSubst, nil
			} else {
				return nil, nil, inferErrorf(n.Right, "right operand of %s must be Float or Int, got %s", n.Operator, r)
			}
		} else if subst, err := typesystem.Unify(l, typesystem.BigInt); err == nil {
			totalSubst = subst.Compose(totalSubst)
			r = r.Apply(totalSubst)
			if subst2, err := typesystem.Unify(r, typesystem.BigInt); err == nil {
				totalSubst = subst2.Compose(totalSubst)
				return typesystem.BigInt, totalSubst, nil
			} else {
				return nil, nil, inferErrorf(n.Right, "right operand of %s must be BigInt, got %s", n.Operator, r)
			}
		} else if subst, err := typesystem.Unify(l, typesystem.Rational); err == nil {
			totalSubst = subst.Compose(totalSubst)
			r = r.Apply(totalSubst)
			if subst2, err := typesystem.Unify(r, typesystem.Rational); err == nil {
				totalSubst = subst2.Compose(totalSubst)
				return typesystem.Rational, totalSubst, nil
			} else {
				return nil, nil, inferErrorf(n.Right, "right operand of %s must be Rational, got %s", n.Operator, r)
			}
		} else {
			return nil, nil, inferErrorf(n.Left, "left operand of %s must be Int, Float, BigInt, Rational, or implement %s trait, got %s", n.Operator, getTraitForOp(n.Operator), l)
		}

	case "&", "|", "^", "<<", ">>":
		// First, check if there's a trait implementation for this operator on the type
		if traitName, ok := table.GetTraitForOperator(n.Operator); ok {
			if eitherHasConstraint(ctx, table, l, r, traitName) {
				// Both operands should have the same type
				subst, err := typesystem.Unify(l, r)
				if err != nil {
					return nil, nil, inferErrorf(n, "type mismatch in %s: %s vs %s", n.Operator, l, r)
				}
				totalSubst = subst.Compose(totalSubst)
				return l.Apply(totalSubst), totalSubst, nil
			}
		}

		// Fallback to built-in Int operations
		subst, err := typesystem.Unify(l, typesystem.Int)
		if err != nil {
			return nil, nil, inferErrorf(n.Left, "left operand of %s must be Int, got %s", n.Operator, l)
		}
		totalSubst = subst.Compose(totalSubst)
		r = r.Apply(totalSubst)

		subst2, err := typesystem.Unify(r, typesystem.Int)
		if err != nil {
			return nil, nil, inferErrorf(n.Right, "right operand of %s must be Int, got %s", n.Operator, r)
		}
		totalSubst = subst2.Compose(totalSubst)

		return typesystem.Int, totalSubst, nil

	case "<", ">", "<=", ">=":
		// First, check if there's a trait implementation for this operator on the type
		if traitName, ok := table.GetTraitForOperator(n.Operator); ok {
			// Check if EITHER type has implementation OR constraint (for lambdas where fresh TVar unifies with constrained type)
			if eitherHasConstraint(ctx, table, l, r, traitName) {
				// Both operands should have the same type
				subst, err := typesystem.Unify(l, r)
				if err != nil {
					// Implicit conversion check for comparison
					isLInt := false
					if _, err := typesystem.Unify(l, typesystem.Int); err == nil {
						isLInt = true
					}
					isLFloat := false
					if _, err := typesystem.Unify(l, typesystem.Float); err == nil {
						isLFloat = true
					}
					isRInt := false
					if _, err := typesystem.Unify(r, typesystem.Int); err == nil {
						isRInt = true
					}
					isRFloat := false
					if _, err := typesystem.Unify(r, typesystem.Float); err == nil {
						isRFloat = true
					}

					if (isLInt && isRFloat) || (isLFloat && isRInt) {
						return typesystem.Bool, totalSubst, nil
					}

					return nil, nil, inferErrorf(n, "type mismatch in %s: %s vs %s", n.Operator, l, r)
				}
				totalSubst = subst.Compose(totalSubst)
				return typesystem.Bool, totalSubst, nil
			}
		}

		// Fallback to built-in comparison for numeric types
		if subst, err := typesystem.Unify(l, typesystem.Int); err == nil {
			totalSubst = subst.Compose(totalSubst)
			r = r.Apply(totalSubst)
			if subst2, err := typesystem.Unify(r, typesystem.Int); err == nil {
				totalSubst = subst2.Compose(totalSubst)
			} else if subst2, err := typesystem.Unify(r, typesystem.Float); err == nil {
				totalSubst = subst2.Compose(totalSubst)
			} else {
				return nil, nil, inferErrorf(n.Right, "right operand of %s must be Int or Float, got %s", n.Operator, r)
			}
		} else if subst, err := typesystem.Unify(l, typesystem.Float); err == nil {
			totalSubst = subst.Compose(totalSubst)
			r = r.Apply(totalSubst)
			if subst2, err := typesystem.Unify(r, typesystem.Float); err == nil {
				totalSubst = subst2.Compose(totalSubst)
			} else if subst2, err := typesystem.Unify(r, typesystem.Int); err == nil {
				totalSubst = subst2.Compose(totalSubst)
			} else {
				return nil, nil, inferErrorf(n.Right, "right operand of %s must be Float or Int, got %s", n.Operator, r)
			}
		} else if subst, err := typesystem.Unify(l, typesystem.BigInt); err == nil {
			totalSubst = subst.Compose(totalSubst)
			r = r.Apply(totalSubst)
			if subst2, err := typesystem.Unify(r, typesystem.BigInt); err != nil {
				return nil, nil, inferErrorf(n.Right, "right operand of %s must be BigInt, got %s", n.Operator, r)
			} else {
				totalSubst = subst2.Compose(totalSubst)
			}
		} else if subst, err := typesystem.Unify(l, typesystem.Rational); err == nil {
			totalSubst = subst.Compose(totalSubst)
			r = r.Apply(totalSubst)
			if subst2, err := typesystem.Unify(r, typesystem.Rational); err != nil {
				return nil, nil, inferErrorf(n.Right, "right operand of %s must be Rational, got %s", n.Operator, r)
			} else {
				totalSubst = subst2.Compose(totalSubst)
			}
		} else if subst, err := typesystem.Unify(l, typesystem.TCon{Name: config.BytesTypeName}); err == nil {
			// Bytes comparison (lexicographic)
			totalSubst = subst.Compose(totalSubst)
			r = r.Apply(totalSubst)
			if subst2, err := typesystem.Unify(r, typesystem.TCon{Name: config.BytesTypeName}); err != nil {
				return nil, nil, inferErrorf(n.Right, "right operand of %s must be Bytes, got %s", n.Operator, r)
			} else {
				totalSubst = subst2.Compose(totalSubst)
			}
		} else {
			return nil, nil, inferErrorf(n.Left, "comparison expects Int, Float, BigInt, Rational, Bytes, or implement %s trait, got %s", getTraitForOp(n.Operator), l)
		}
		return typesystem.Bool, totalSubst, nil

	case "==", "!=":
		// Special case: any Type<T> can be compared with any Type<U>
		// (runtime type objects are always comparable)
		_, lIsType := l.(typesystem.TType)
		_, rIsType := r.(typesystem.TType)
		if lIsType && rIsType {
			return typesystem.Bool, totalSubst, nil
		}

		// First, check if there's a trait implementation for this operator on the type
		if traitName, ok := table.GetTraitForOperator(n.Operator); ok {
			if eitherHasConstraint(ctx, table, l, r, traitName) {
				// Both operands should have the same type
				subst, err := typesystem.Unify(l, r)
				if err != nil {
					// Implicit conversion check for equality
					isLInt := false
					if _, err := typesystem.Unify(l, typesystem.Int); err == nil {
						isLInt = true
					}
					isLFloat := false
					if _, err := typesystem.Unify(l, typesystem.Float); err == nil {
						isLFloat = true
					}
					isRInt := false
					if _, err := typesystem.Unify(r, typesystem.Int); err == nil {
						isRInt = true
					}
					isRFloat := false
					if _, err := typesystem.Unify(r, typesystem.Float); err == nil {
						isRFloat = true
					}

					if (isLInt && isRFloat) || (isLFloat && isRInt) {
						return typesystem.Bool, totalSubst, nil
					}

					return nil, nil, inferErrorf(n, "type mismatch in %s: %s vs %s", n.Operator, l, r)
				}
				totalSubst = subst.Compose(totalSubst)
				return typesystem.Bool, totalSubst, nil
			}
		}

		// Fallback to built-in equality
		subst, err := typesystem.Unify(l, r)
		if err != nil {
			// Implicit conversion check for built-in equality
			isLInt := false
			if _, err := typesystem.Unify(l, typesystem.Int); err == nil {
				isLInt = true
			}
			isLFloat := false
			if _, err := typesystem.Unify(l, typesystem.Float); err == nil {
				isLFloat = true
			}
			isRInt := false
			if _, err := typesystem.Unify(r, typesystem.Int); err == nil {
				isRInt = true
			}
			isRFloat := false
			if _, err := typesystem.Unify(r, typesystem.Float); err == nil {
				isRFloat = true
			}

			if (isLInt && isRFloat) || (isLFloat && isRInt) {
				return typesystem.Bool, totalSubst, nil
			}

			return nil, nil, inferErrorf(n, "type mismatch in %s: %s vs %s", n.Operator, l, r)
		}
		totalSubst = subst.Compose(totalSubst)
		return typesystem.Bool, totalSubst, nil

	case "&&", "||":
		// Logical AND/OR - both operands must be Bool
		subst, err := typesystem.Unify(l, typesystem.Bool)
		if err != nil {
			return nil, nil, inferErrorf(n.Left, "left operand of %s must be Bool, got %s", n.Operator, l)
		}
		totalSubst = subst.Compose(totalSubst)

		subst2, err := typesystem.Unify(r, typesystem.Bool)
		if err != nil {
			return nil, nil, inferErrorf(n.Right, "right operand of %s must be Bool, got %s", n.Operator, r)
		}
		totalSubst = subst2.Compose(totalSubst)

		return typesystem.Bool, totalSubst, nil

	case "??":
		// Null coalescing via Optional trait: F<A> ?? A -> A
		// Left must implement Optional, right must match inner type
		if !CheckTraitImplementation(l, "Optional", table) {
			return nil, nil, inferErrorf(n.Left, "type %s does not implement Optional trait", l)
		}
		// Extract inner type A from F<A> using trait method signature
		innerType, ok := getOptionalInnerType(l, table)
		if !ok {
			return nil, nil, inferErrorf(n.Left, "left operand of ?? must be a type constructor F<A>, got %s", l)
		}
		subst, err := typesystem.Unify(r, innerType)
		if err != nil {
			return nil, nil, inferErrorf(n.Right, "fallback value must be %s, got %s", innerType, r)
		}
		totalSubst = subst.Compose(totalSubst)
		return innerType.Apply(totalSubst), totalSubst, nil

	case "++":
		// First check if type implements Concat trait
		// Unwrap type aliases before checking to handle cases with type aliases
		checkType := typesystem.UnwrapUnderlying(l)
		if checkType == nil {
			checkType = l
		}
		if table.IsImplementationExists("Concat", []typesystem.Type{checkType}) {
			// User-defined Concat - operands must be same type, return same type
			subst, err := typesystem.Unify(l, r)
			if err != nil {
				return nil, nil, inferErrorf(n, "++ operands must be same type, got %s and %s", l, r)
			}
			totalSubst = subst.Compose(totalSubst)
			return l.Apply(totalSubst), totalSubst, nil
		}

		// Check for Bytes concatenation
		bytesType := typesystem.TCon{Name: config.BytesTypeName}
		if subst, err := typesystem.Unify(l, bytesType); err == nil {
			totalSubst = subst.Compose(totalSubst)
			// Right must also be Bytes
			if subst2, err := typesystem.Unify(r, bytesType); err == nil {
				totalSubst = subst2.Compose(totalSubst)
				return bytesType, totalSubst, nil
			}
			return nil, nil, inferErrorf(n.Right, "right operand of ++ must be Bytes, got %s", r)
		}

		// Check for Bits concatenation
		bitsType := typesystem.TCon{Name: config.BitsTypeName}
		if subst, err := typesystem.Unify(l, bitsType); err == nil {
			totalSubst = subst.Compose(totalSubst)
			// Right must also be Bits
			if subst2, err := typesystem.Unify(r, bitsType); err == nil {
				totalSubst = subst2.Compose(totalSubst)
				return bitsType, totalSubst, nil
			}
			return nil, nil, inferErrorf(n.Right, "right operand of ++ must be Bits, got %s", r)
		}

		// Concatenation - both operands must be lists of the same type
		// l and r must be List<T> for some T
		elemVar := ctx.FreshVar()
		listType := typesystem.TApp{
			Constructor: typesystem.TCon{Name: config.ListTypeName},
			Args:        []typesystem.Type{elemVar},
		}

		subst, err := typesystem.Unify(l, listType)
		if err != nil {
			return nil, nil, inferErrorf(n.Left, "left operand of ++ must be List, Bytes, or implement Concat, got %s", l)
		}
		totalSubst = subst.Compose(totalSubst)

		// Apply subst to expected type for right operand
		expectedRight := listType.Apply(totalSubst)
		r = r.Apply(totalSubst)

		subst2, err := typesystem.Unify(r, expectedRight)
		if err != nil {
			return nil, nil, inferErrorf(n.Right, "right operand of ++ must be %s, got %s", expectedRight, r)
		}
		totalSubst = subst2.Compose(totalSubst)

		return listType.Apply(totalSubst), totalSubst, nil

	case "::":
		// Cons - prepend element to list
		// T :: List<T> -> List<T>
		elemVar := ctx.FreshVar()
		listType := typesystem.TApp{
			Constructor: typesystem.TCon{Name: config.ListTypeName},
			Args:        []typesystem.Type{elemVar},
		}

		// Left is the element
		subst, err := typesystem.Unify(l, elemVar)
		if err != nil {
			return nil, nil, inferErrorf(n.Left, "cons (::) element type mismatch")
		}
		totalSubst = subst.Compose(totalSubst)

		// Right must be List of the same element type
		expectedList := listType.Apply(totalSubst)
		r = r.Apply(totalSubst)

		subst2, err := typesystem.Unify(r, expectedList)
		if err != nil {
			return nil, nil, inferErrorf(n.Right, "right operand of :: must be List<%s>, got %s", l.Apply(totalSubst), r)
		}
		totalSubst = subst2.Compose(totalSubst)

		return expectedList.Apply(totalSubst), totalSubst, nil

	case "$":
		// Function application: f $ x = f(x)
		// Left operand must be a function, right operand is the argument

		// Check if l is already a function type
		if fnType, ok := l.(typesystem.TFunc); ok {
			if len(fnType.Params) >= 1 {
				// Unify right operand with first parameter
				subst, err := typesystem.Unify(r, fnType.Params[0])
				if err != nil {
					return nil, nil, inferErrorf(n.Right, "cannot apply function expecting %s to %s", fnType.Params[0], r)
				}
				totalSubst = subst.Compose(totalSubst)

				// If function has more params, return partial application
				if len(fnType.Params) > 1 {
					newParams := make([]typesystem.Type, len(fnType.Params)-1)
					for i, p := range fnType.Params[1:] {
						newParams[i] = p.Apply(totalSubst)
					}
					return typesystem.TFunc{
						Params:     newParams,
						ReturnType: fnType.ReturnType.Apply(totalSubst),
						IsVariadic: fnType.IsVariadic,
					}, totalSubst, nil
				}

				return fnType.ReturnType.Apply(totalSubst), totalSubst, nil
			}
			// Zero-param function - just return its result type
			return fnType.ReturnType.Apply(totalSubst), totalSubst, nil
		}

		// General case: unify with expected function type
		resultVar := ctx.FreshVar()
		expectedFnType := typesystem.TFunc{
			Params:     []typesystem.Type{r},
			ReturnType: resultVar,
		}

		subst, err := typesystem.Unify(l, expectedFnType)
		if err != nil {
			return nil, nil, inferErrorf(n.Left, "left operand of $ must be a function, got %s", l)
		}
		totalSubst = subst.Compose(totalSubst)

		return resultVar.Apply(totalSubst), totalSubst, nil

	case ",,":
		// Function composition (right-to-left): (f ,, g)(x) = f(g(x))
		// f: (b) -> c, g: (a) -> b => f ,, g: (a) -> c

		// Both operands must be functions
		aVar := ctx.FreshVar() // input type of g
		bVar := ctx.FreshVar() // output of g / input of f
		cVar := ctx.FreshVar() // output type of f

		// g: (a) -> b
		gType := typesystem.TFunc{
			Params:     []typesystem.Type{aVar},
			ReturnType: bVar,
		}
		subst1, err := typesystem.Unify(r, gType)
		if err != nil {
			return nil, nil, inferErrorf(n.Right, "right operand of ,, must be a function, got %s", r)
		}
		totalSubst = subst1.Compose(totalSubst)

		// f: (b) -> c
		fType := typesystem.TFunc{
			Params:     []typesystem.Type{bVar.Apply(totalSubst)},
			ReturnType: cVar,
		}
		subst2, err := typesystem.Unify(l.Apply(totalSubst), fType)
		if err != nil {
			return nil, nil, inferErrorf(n.Left, "left operand of ,, must be a function, got %s", l)
		}
		totalSubst = subst2.Compose(totalSubst)

		// Result: (a) -> c
		resultType := typesystem.TFunc{
			Params:     []typesystem.Type{aVar.Apply(totalSubst)},
			ReturnType: cVar.Apply(totalSubst),
		}
		return resultType, totalSubst, nil

	default:
		// Check if it's a user-definable operator from config
		if config.IsUserOperator(n.Operator) {
			traitName, ok := table.GetTraitForOperator(n.Operator)
			if !ok {
				return nil, nil, inferErrorf(n, "no trait registered for operator %s", n.Operator)
			}

			// Check if this is an HKT trait (like Applicative, Monad)
			// HKT operators have different parameter types, not just A -> A -> A
			if table.IsHKTTrait(traitName) {
				return inferHKTOperator(ctx, n, l, r, traitName, table, totalSubst)
			}

			// Non-HKT trait: both operands should have the same type
			if eitherHasConstraint(ctx, table, l, r, traitName) {
				subst, err := typesystem.Unify(l, r)
				if err != nil {
					return nil, nil, inferErrorf(n, "type mismatch in %s: %s vs %s", n.Operator, l, r)
				}
				totalSubst = subst.Compose(totalSubst)
				return l.Apply(totalSubst), totalSubst, nil
			}

			return nil, nil, inferErrorf(n, "type %s does not implement trait %s (required for operator %s)", l, traitName, n.Operator)
		}

		return nil, nil, inferErrorf(n, "unknown infix operator: %s", n.Operator)
	}
}

// inferHKTOperator handles type inference for Higher-Kinded Type operators
// like <*> (Applicative) and >>= (Monad) where operand types are different.
// For example: <*> : F<(A -> B)> -> F<A> -> F<B>
func inferHKTOperator(ctx *InferenceContext, n *ast.InfixExpression, l, r typesystem.Type, traitName string, table *symbols.SymbolTable, totalSubst typesystem.Subst) (typesystem.Type, typesystem.Subst, error) {
	// Get the operator's method name (e.g., "(<*>)" for <*>)
	methodName := "(" + n.Operator + ")"

	// Look up the method type from the symbol table
	sym, ok := table.Find(methodName)
	if !ok {
		return nil, nil, inferErrorf(n, "method %s not found for trait %s", methodName, traitName)
	}

	methodType, ok := sym.Type.(typesystem.TFunc)
	if !ok {
		return nil, nil, inferErrorf(n, "method %s has invalid type", methodName)
	}

	// Check we have exactly 2 parameters for binary operator
	if len(methodType.Params) != 2 {
		return nil, nil, inferErrorf(n, "method %s expected 2 parameters, got %d", methodName, len(methodType.Params))
	}

	// Instantiate the method type with fresh type variables
	// This replaces F, A, B, M, etc. with fresh t1, t2, t3...
	freshMethodType := InstantiateWithContext(ctx, methodType).(typesystem.TFunc)

	// Unify left operand with first parameter
	// e.g., Option<(Int) -> Int> with F<(A -> B)>
	subst1, err := typesystem.Unify(freshMethodType.Params[0], l)
	if err != nil {
		return nil, nil, inferErrorf(n, "left operand type %s does not match expected %s for %s", l, freshMethodType.Params[0], n.Operator)
	}
	totalSubst = subst1.Compose(totalSubst)

	// Apply substitution to the second parameter before unifying
	expectedRight := freshMethodType.Params[1].Apply(totalSubst)

	// Unify right operand with second parameter (after applying subst from first unification)
	// e.g., Option<Int> with F<A> where F=Option from previous step
	subst2, err := typesystem.Unify(expectedRight, r)
	if err != nil {
		return nil, nil, inferErrorf(n, "right operand type %s does not match expected %s for %s", r, expectedRight, n.Operator)
	}
	totalSubst = subst2.Compose(totalSubst)

	// Apply all substitutions to the return type
	resultType := freshMethodType.ReturnType.Apply(totalSubst)

	// Check constraints from the method (e.g. M: Monad)
	for _, c := range freshMethodType.Constraints {
		// Apply substitution to get the concrete type for the type variable
		tvar := typesystem.TVar{Name: c.TypeVar}
		concreteType := tvar.Apply(totalSubst)

		// Check if it's a constrained type param (TCon like "T" in recursive calls)
		// Unwrap type aliases before checking to handle cases like Writer<IntList, Int>
		checkType := typesystem.UnwrapUnderlying(concreteType)
		if checkType == nil {
			checkType = concreteType
		}

		// If type is still a variable, defer constraint check
		if _, stillVar := concreteType.(typesystem.TVar); stillVar {
			ctx.AddDeferredConstraint(Constraint{
				Kind:  ConstraintImplements,
				Left:  concreteType,
				Trait: c.Trait,
				Node:  n,
			})
			continue
		}

		if !table.IsImplementationExists(c.Trait, []typesystem.Type{checkType}) && !typeHasConstraint(ctx, concreteType, c.Trait, table) {
			return nil, nil, inferErrorf(n, "type %s does not implement trait %s", concreteType, c.Trait)
		}
	}

	return resultType, totalSubst, nil
}

func inferPostfixExpression(ctx *InferenceContext, n *ast.PostfixExpression, table *symbols.SymbolTable, inferFn func(ast.Node, *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error)) (typesystem.Type, typesystem.Subst, error) {
	leftType, s1, err := inferFn(n.Left, table)
	if err != nil {
		return nil, nil, err
	}
	totalSubst := s1
	leftType = leftType.Apply(ctx.GlobalSubst).Apply(totalSubst)

	if n.Operator == "?" {
		if tApp, ok := leftType.(typesystem.TApp); ok {
			if tCon, ok := tApp.Constructor.(typesystem.TCon); ok {
				if tCon.Name == config.ResultTypeName && len(tApp.Args) == 2 {
					// Result<E, A> - success type is Args[1]
					return tApp.Args[1], totalSubst, nil
				}
				if tCon.Name == config.OptionTypeName && len(tApp.Args) == 1 {
					return tApp.Args[0], totalSubst, nil
				}
			}
		}
		return nil, nil, inferErrorf(n, "operator '?' expects Result or Option, got %s", leftType)
	} else {
		return nil, nil, inferErrorf(n, "unknown postfix operator: %s", n.Operator)
	}
}

// inferOperatorAsFunction infers the type of an operator used as a function, e.g., (+)
// Returns a function type with appropriate constraints
func inferOperatorAsFunction(ctx *InferenceContext, n *ast.OperatorAsFunction, table *symbols.SymbolTable) (typesystem.Type, typesystem.Subst, error) {
	op := n.Operator
	tvar := ctx.FreshVar()

	// Determine return type and constraints based on operator
	var returnType typesystem.Type
	var constraints []typesystem.Constraint

	switch op {
	// Comparison operators return Bool
	case "==", "!=":
		returnType = typesystem.Bool
		constraints = []typesystem.Constraint{{TypeVar: tvar.Name, Trait: "Equal"}}
	case "<", ">", "<=", ">=":
		returnType = typesystem.Bool
		constraints = []typesystem.Constraint{{TypeVar: tvar.Name, Trait: "Order"}}

	// Arithmetic operators return same type
	case "+", "-", "*", "/", "%", "**":
		returnType = tvar
		constraints = []typesystem.Constraint{{TypeVar: tvar.Name, Trait: "Numeric"}}

	// Bitwise operators return same type
	case "&", "|", "^", "<<", ">>":
		returnType = tvar
		constraints = []typesystem.Constraint{{TypeVar: tvar.Name, Trait: "Bitwise"}}

	// Logical operators work on Bool, return Bool
	case "&&", "||":
		return typesystem.TFunc{
			Params:     []typesystem.Type{typesystem.Bool, typesystem.Bool},
			ReturnType: typesystem.Bool,
		}, typesystem.Subst{}, nil

	// Concatenation - returns same type, no constraint (checked at callsite)
	case "++":
		returnType = tvar
		// No constraint here - ++ works on Lists and types implementing Concat

	// Cons operator: a -> List<a> -> List<a>
	case "::":
		listType := typesystem.TApp{
			Constructor: typesystem.TCon{Name: config.ListTypeName},
			Args:        []typesystem.Type{tvar},
		}
		return typesystem.TFunc{
			Params:     []typesystem.Type{tvar, listType},
			ReturnType: listType,
		}, typesystem.Subst{}, nil

	default:
		// Check if it's a user-definable operator from config
		if userOp := config.GetUserOperatorBySymbol(op); userOp != nil {
			returnType = tvar
			constraints = []typesystem.Constraint{{TypeVar: tvar.Name, Trait: userOp.Trait}}
		} else {
			return nil, nil, inferErrorf(n, "unknown operator for function conversion: %s", op)
		}
	}

	// Binary operator: (T, T) -> R
	fnType := typesystem.TFunc{
		Params:      []typesystem.Type{tvar, tvar},
		ReturnType:  returnType,
		Constraints: constraints,
	}

	return fnType, typesystem.Subst{}, nil
}
