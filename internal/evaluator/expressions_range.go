package evaluator

import (
	"github.com/funvibe/funxy/internal/ast"
)

func (e *Evaluator) evalRangeExpression(node *ast.RangeExpression, env *Environment) Object {
	start := e.Eval(node.Start, env)
	if isError(start) {
		return start
	}

	var next Object = &Nil{}
	if node.Next != nil {
		nextVal := e.Eval(node.Next, env)
		if isError(nextVal) {
			return nextVal
		}
		next = nextVal
	}

	end := e.Eval(node.End, env)
	if isError(end) {
		return end
	}

	return &Range{
		Start: start,
		Next:  next,
		End:   end,
	}
}
