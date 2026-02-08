package evaluator

import (
	"github.com/funvibe/funxy/internal/ast"
)

func (e *Evaluator) evalBlockStatement(block *ast.BlockStatement, env *Environment) Object {
	var result Object
	blockEnv := NewEnclosedEnvironment(env)

	// Predeclare local functions to support mutual recursion within the block.
	for _, stmt := range block.Statements {
		fs, ok := stmt.(*ast.FunctionStatement)
		if !ok || fs == nil || fs.Receiver != nil {
			continue
		}
		fn := &Function{
			Name:          fs.Name.Value,
			Parameters:    fs.Parameters,
			WitnessParams: fs.WitnessParams,
			ReturnType:    fs.ReturnType,
			Body:          fs.Body,
			Env:           blockEnv, // Closure
			Line:          fs.Token.Line,
			Column:        fs.Token.Column,
		}
		blockEnv.Set(fs.Name.Value, fn)
	}

	for _, stmt := range block.Statements {
		result = e.Eval(stmt, blockEnv)
		if result != nil {
			rt := result.Type()
			if rt == ERROR_OBJ {
				return result
			}
			if rt == RETURN_VALUE_OBJ {
				return result
			}
			if rt == BREAK_SIGNAL_OBJ || rt == CONTINUE_SIGNAL_OBJ {
				return result
			}
		}
	}

	if result == nil {
		return &Nil{}
	}
	return result
}

func (e *Evaluator) evalBreakStatement(node *ast.BreakStatement, env *Environment) Object {
	var val Object
	if node.Value != nil {
		val = e.Eval(node.Value, env)
		if isError(val) {
			return val
		}
	} else {
		val = &Nil{}
	}
	return &BreakSignal{Value: val}
}

func (e *Evaluator) evalContinueStatement(node *ast.ContinueStatement, env *Environment) Object {
	return &ContinueSignal{}
}

func (e *Evaluator) evalReturnStatement(node *ast.ReturnStatement, env *Environment) Object {
	if node.Value == nil {
		return &ReturnValue{Value: &Nil{}}
	}
	val := e.Eval(node.Value, env)
	if isError(val) {
		return val
	}
	return &ReturnValue{Value: val}
}
