package evaluator

import (
	"github.com/funvibe/funxy/internal/ast"
)

func (e *Evaluator) evalExtensionMethod(node *ast.FunctionStatement, env *Environment) Object {
	typeName, err := e.resolveCanonicalTypeName(node.Receiver.Type, env)
	if err != nil {
		return newError("%s", err.Error())
	}

	methodName := node.Name.Value

	fn := &Function{
		Name:          node.Name.Value,
		Parameters:    node.Parameters,
		WitnessParams: node.WitnessParams,
		ReturnType:    node.ReturnType,
		Body:          node.Body,
		Env:           env,
		Line:          node.Token.Line,
		Column:        node.Token.Column,
	}
	newParams := append([]*ast.Parameter{node.Receiver}, node.Parameters...)
	fn.Parameters = newParams

	if _, ok := e.ExtensionMethods[typeName]; !ok {
		e.ExtensionMethods[typeName] = make(map[string]Object)
	}
	e.ExtensionMethods[typeName][methodName] = fn

	return &Nil{}
}
