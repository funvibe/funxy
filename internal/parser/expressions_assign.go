package parser

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/token"
)

func (p *Parser) parseAssignExpression(left ast.Expression) ast.Expression {
	target, pattern, annotatedType, ok := p.validateAssignmentTarget(left)
	if !ok {
		return nil
	}

	tok := p.curToken
	p.nextToken() // consume '='
	value := p.parseExpression(LOWEST)
	if value == nil {
		return nil
	}

	if pattern != nil {
		return &ast.PatternAssignExpression{
			Token:         tok,
			Pattern:       pattern,
			Value:         value,
			AnnotatedType: annotatedType,
		}
	}

	return &ast.AssignExpression{
		Token:         tok,
		Left:          target,
		AnnotatedType: annotatedType,
		Value:         value,
	}
}

// parseCompoundAssignExpression handles +=, -=, *=, /=, %=, **=
// Desugars `x += y` to `x = x + y`
func (p *Parser) parseCompoundAssignExpression(left ast.Expression) ast.Expression {
	// Determine the operator from the compound assignment token
	compoundTok := p.curToken
	var operator string
	var opToken token.Token

	switch compoundTok.Type {
	case token.PLUS_ASSIGN:
		operator = "+"
		opToken = token.Token{Type: token.PLUS, Lexeme: "+", Line: compoundTok.Line, Column: compoundTok.Column}
	case token.MINUS_ASSIGN:
		operator = "-"
		opToken = token.Token{Type: token.MINUS, Lexeme: "-", Line: compoundTok.Line, Column: compoundTok.Column}
	case token.ASTERISK_ASSIGN:
		operator = "*"
		opToken = token.Token{Type: token.ASTERISK, Lexeme: "*", Line: compoundTok.Line, Column: compoundTok.Column}
	case token.SLASH_ASSIGN:
		operator = "/"
		opToken = token.Token{Type: token.SLASH, Lexeme: "/", Line: compoundTok.Line, Column: compoundTok.Column}
	case token.PERCENT_ASSIGN:
		operator = "%"
		opToken = token.Token{Type: token.PERCENT, Lexeme: "%", Line: compoundTok.Line, Column: compoundTok.Column}
	case token.POWER_ASSIGN:
		operator = "**"
		opToken = token.Token{Type: token.POWER, Lexeme: "**", Line: compoundTok.Line, Column: compoundTok.Column}
	default:
		return nil
	}

	// Validate target is a valid l-value (identifier or member expression)
	var target ast.Expression
	if anno, ok := left.(*ast.AnnotatedExpression); ok {
		target = anno.Expression
	} else {
		target = left
	}

	switch target.(type) {
	case *ast.Identifier, *ast.MemberExpression:
		// OK - valid l-value for compound assignment
	default:
		p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
			diagnostics.ErrP002,
			compoundTok,
		))
		return nil
	}

	p.nextToken() // consume the compound assignment operator
	right := p.parseExpression(LOWEST)

	// Create the infix expression: left OP right
	infixExpr := &ast.InfixExpression{
		Token:    opToken,
		Left:     target,
		Operator: operator,
		Right:    right,
	}

	// Create the assignment: left = (left OP right)
	assignTok := token.Token{Type: token.ASSIGN, Lexeme: "=", Line: compoundTok.Line, Column: compoundTok.Column}
	return &ast.AssignExpression{
		Token: assignTok,
		Left:  target,
		Value: infixExpr,
	}
}

func (p *Parser) parseAnnotatedExpression(left ast.Expression) ast.Expression {
	// Left is the expression being annotated
	expr := &ast.AnnotatedExpression{
		Token:      p.curToken,
		Expression: left,
	}
	p.nextToken() // Consume ':'
	expr.TypeAnnotation = p.parseType()
	return expr
}

// validateAssignmentTarget validates the LHS of an assignment or declaration.
// It returns the target expression (if simple identifier/member), the pattern (if pattern destructuring),
// the type annotation (if present), and a boolean indicating validity.
func (p *Parser) validateAssignmentTarget(left ast.Expression) (ast.Expression, ast.Pattern, ast.Type, bool) {
	var annotatedType ast.Type
	var target ast.Expression

	// Handle annotated expression: x: Int = 5
	if anno, ok := left.(*ast.AnnotatedExpression); ok {
		target = anno.Expression
		annotatedType = anno.TypeAnnotation
	} else {
		target = left
	}

	// Validate target is l-value or pattern
	switch t := target.(type) {
	case *ast.Identifier:
		// OK - simple assignment
		if t.Token.Type == token.IDENT_UPPER {
			p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
				diagnostics.ErrP006,
				t.Token,
				"Variable name must start with a lowercase letter",
			))
		}
		return target, nil, annotatedType, true
	case *ast.MemberExpression:
		// OK - member assignment
		return target, nil, annotatedType, true
	case *ast.IndexExpression:
		// ERROR - Index assignment (list[0] = 1) is not supported for immutable lists
		p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
			diagnostics.ErrP007,
			t.Token,
		))
		return nil, nil, nil, false
	case *ast.TupleLiteral:
		// Pattern destructuring: (a, b) = expr
		pattern := p.tupleExprToPattern(t)
		if pattern == nil {
			return nil, nil, nil, false
		}
		return nil, pattern, annotatedType, true
	case *ast.ListLiteral:
		// Pattern destructuring: [a, b, rest...] = expr
		pattern := p.listExprToPattern(t)
		if pattern == nil {
			return nil, nil, nil, false
		}
		return nil, pattern, annotatedType, true
	case *ast.RecordLiteral:
		// Pattern destructuring: { x: a, y: b } = expr
		pattern := p.recordExprToPattern(t)
		if pattern == nil {
			return nil, nil, nil, false
		}
		return nil, pattern, annotatedType, true
	default:
		return nil, nil, nil, false
	}
}
