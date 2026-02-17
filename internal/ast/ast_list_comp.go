package ast

import "github.com/funvibe/funxy/internal/token"

// ListComprehension represents a list comprehension expression.
// Syntax: [expr | generator, filter, ...]
// Example: [x * 2 | x <- [1,2,3], x > 1]
type ListComprehension struct {
	Token   token.Token  // The '[' token
	Output  Expression   // The output expression (e.g., x * 2)
	Clauses []CompClause // Generators and filters
}

func (lc *ListComprehension) Accept(v Visitor)     { v.VisitListComprehension(lc) }
func (lc *ListComprehension) expressionNode()      {}
func (lc *ListComprehension) TokenLiteral() string { return lc.Token.Lexeme }
func (lc *ListComprehension) GetToken() token.Token {
	if lc == nil {
		return token.Token{}
	}
	return lc.Token
}

// CompClause represents a clause in a list comprehension.
// It can be either a generator (x <- list) or a filter (predicate).
type CompClause interface {
	compClauseNode()
}

// CompGenerator represents a generator clause: pattern <- expression
// Example: x <- [1,2,3] or (a, b) <- pairs
type CompGenerator struct {
	Token    token.Token // The '<-' token
	Pattern  Pattern     // The binding pattern (usually identifier, but can be tuple/list pattern)
	Iterable Expression  // The expression to iterate over
}

func (cg *CompGenerator) compClauseNode() {}

// CompFilter represents a filter/guard clause: boolean expression
// Example: x > 1
type CompFilter struct {
	Token     token.Token // First token of the condition
	Condition Expression  // The boolean condition
}

func (cf *CompFilter) compClauseNode() {}
