package ast

import "github.com/funvibe/funxy/internal/token"

// MapComprehension represents a map comprehension expression.
// Syntax: %{ keyExpr => valExpr | generator, filter, ... }
// Example: %{ "key_" ++ show(x) => x * 2 | x <- [1,2,3], x > 1 }
type MapComprehension struct {
	Token   token.Token  // The '%{' token
	Key     Expression   // The key expression
	Value   Expression   // The value expression
	Clauses []CompClause // Generators and filters (reused from ListComprehension)
}

func (mc *MapComprehension) Accept(v Visitor)     { v.VisitMapComprehension(mc) }
func (mc *MapComprehension) expressionNode()      {}
func (mc *MapComprehension) TokenLiteral() string { return mc.Token.Lexeme }
func (mc *MapComprehension) GetToken() token.Token {
	if mc == nil {
		return token.Token{}
	}
	return mc.Token
}
