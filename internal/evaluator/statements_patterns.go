package evaluator

import (
	"github.com/funvibe/funxy/internal/ast"
)

func (e *Evaluator) evalPatternAssignExpression(node *ast.PatternAssignExpression, env *Environment) Object {
	val := e.Eval(node.Value, env)
	if isError(val) {
		return val
	}
	return e.bindPatternToValue(node.Pattern, val, env)
}

// bindPatternToValue binds variables from a pattern to their values
func (e *Evaluator) bindPatternToValue(pat ast.Pattern, val Object, env *Environment) Object {
	switch p := pat.(type) {
	case *ast.IdentifierPattern:
		env.Set(p.Value, val)
		return &Nil{}

	case *ast.TuplePattern:
		tuple, ok := val.(*Tuple)
		if !ok {
			return newError("cannot destructure non-tuple value with tuple pattern")
		}
		if len(tuple.Elements) != len(p.Elements) {
			return newError("tuple pattern has %d elements but value has %d", len(p.Elements), len(tuple.Elements))
		}
		for i, elem := range p.Elements {
			result := e.bindPatternToValue(elem, tuple.Elements[i], env)
			if isError(result) {
				return result
			}
		}
		return &Nil{}

	case *ast.ListPattern:
		list, ok := val.(*List)
		if !ok {
			return newError("cannot destructure non-list value with list pattern")
		}
		if list.len() < len(p.Elements) {
			return newError("list pattern has %d elements but value has %d", len(p.Elements), list.len())
		}
		for i, elem := range p.Elements {
			result := e.bindPatternToValue(elem, list.get(i), env)
			if isError(result) {
				return result
			}
		}
		return &Nil{}

	case *ast.WildcardPattern:
		// Ignore - don't bind anything
		return &Nil{}

	case *ast.RecordPattern:
		record, ok := val.(*RecordInstance)
		if !ok {
			return newError("cannot destructure non-record value with record pattern")
		}
		for fieldName, fieldPat := range p.Fields {
			fieldVal := record.Get(fieldName)
			if fieldVal == nil {
				return newError("record does not have field '%s'", fieldName)
			}
			result := e.bindPatternToValue(fieldPat, fieldVal, env)
			if isError(result) {
				return result
			}
		}
		return &Nil{}

	default:
		return newError("unsupported pattern in destructuring")
	}
}
