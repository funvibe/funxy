package evaluator

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
)

func (e *Evaluator) evalForExpression(node *ast.ForExpression, env *Environment) Object {
	loopEnv := NewEnclosedEnvironment(env)

	runBody := func() (Object, bool) {
		res := e.evalBlockStatement(node.Body, loopEnv)

		if isError(res) {
			return res, true
		}

		if breakSig, ok := res.(*BreakSignal); ok {
			return breakSig.Value, true
		}

		if _, ok := res.(*ContinueSignal); ok {
			return nil, false
		}

		if rt := res.Type(); rt == RETURN_VALUE_OBJ {
			return res, true
		}

		return res, false
	}

	var lastResult Object = &Nil{}

	if node.Iterable != nil {
		iterable := e.Eval(node.Iterable, env)
		if isError(iterable) {
			return iterable
		}

		var iteratorFn Object

		// Look up iter method from Iter trait implementation for this type
		iterableTypeName := getRuntimeTypeName(iterable)
		if iterMethod, found := e.lookupTraitMethod(config.IterTraitName, config.IterMethodName, iterableTypeName); found {
			res := e.ApplyFunction(iterMethod, []Object{iterable})
			if !isError(res) {
				iteratorFn = res
			}
		}

		// Fallback: try direct environment lookup (for backward compatibility)
		if iteratorFn == nil {
			if iterSym, ok := env.Get(config.IterMethodName); ok {
				res := e.ApplyFunction(iterSym, []Object{iterable})
				if !isError(res) {
					iteratorFn = res
				}
			}
		}

		if iteratorFn != nil {
			itemName := node.ItemName.Value

			for {
				stepRes := e.ApplyFunction(iteratorFn, []Object{})
				if isError(stepRes) {
					return stepRes
				}

				if data, ok := stepRes.(*DataInstance); ok && data.TypeName == config.OptionTypeName {
					if data.Name == config.SomeCtorName {
						val := data.Fields[0]
						loopEnv.Set(itemName, val)

						res, shouldBreak := runBody()
						if shouldBreak {
							if isError(res) || res.Type() == RETURN_VALUE_OBJ {
								return res
							}
							return res
						}
						if res != nil {
							lastResult = res
						}
					} else if data.Name == config.NoneCtorName {
						break
					} else {
						return newError("iterator returned unexpected Option variant: %s", data.Name)
					}
				} else {
					return newError("iterator must return Option, got %s", stepRes.Type())
				}
			}

		} else {
			var items []Object
			if list, ok := iterable.(*List); ok {
				items = list.ToSlice()
			} else if str, ok := iterable.(*List); ok {
				items = str.ToSlice()
			} else if rng, ok := iterable.(*Range); ok {
				// Expand range to items
				// We support Int and Char ranges for now, similar to VM
				start := rng.Start
				end := rng.End
				next := rng.Next

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
					// Generate items
					// Inclusive end
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
				} else {
					return newError("range iteration only supported for Int and Char in interpreter mode")
				}
			} else {
				return newError("iterable must be List or implement Iter trait, got %s", iterable.Type())
			}

			itemName := node.ItemName.Value

			for _, item := range items {
				loopEnv.Set(itemName, item)

				res, shouldBreak := runBody()
				if shouldBreak {
					if isError(res) || res.Type() == RETURN_VALUE_OBJ {
						return res
					}
					return res
				}
				if res != nil {
					lastResult = res
				}
			}
		}

	} else {
		for {
			cond := e.Eval(node.Condition, loopEnv)
			if isError(cond) {
				return cond
			}

			if !e.isTruthy(cond) {
				break
			}

			res, shouldBreak := runBody()
			if shouldBreak {
				if isError(res) || res.Type() == RETURN_VALUE_OBJ {
					return res
				}
				return res
			}
			if res != nil {
				lastResult = res
			}
		}
	}

	return lastResult
}
