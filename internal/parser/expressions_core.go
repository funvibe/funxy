package parser

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/token"
)

func (p *Parser) parseExpression(precedence int) ast.Expression {
	p.depth++
	defer func() { p.depth-- }()

	if p.depth > MaxRecursionDepth {
		if !p.inRecursionRecovery {
			p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
				diagnostics.ErrP006,
				p.curToken,
				"expression too complex: recursion depth limit exceeded",
			))
			p.inRecursionRecovery = true
		}
		// Skip the rest of the statement to avoid a cascade of errors.
		p.skipToStatementBoundary()
		p.inRecursionRecovery = false
		return nil
	}

	if p.curTokenIs(token.RETURN) {
		p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
			diagnostics.ErrP006,
			p.curToken,
			"return is only allowed inside function bodies",
		))
		// Attempt to recover by skipping to a likely expression boundary.
		for !p.curTokenIs(token.NEWLINE) &&
			!p.curTokenIs(token.RBRACE) &&
			!p.curTokenIs(token.RBRACKET) &&
			!p.curTokenIs(token.RPAREN) &&
			!p.curTokenIs(token.COMMA) &&
			!p.curTokenIs(token.EOF) {
			p.nextToken()
		}
		return nil
	}

	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}
	leftExp := prefix()

	for {
		// Check if we should continue parsing infix operators
		if p.peekTokenIs(token.NEWLINE) {
			// Look ahead past newlines for continuation operators
			if !p.hasContinuationOperator() {
				break
			}
			// Skip newlines to get to the operator
			for p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
		}

		if precedence >= p.peekPrecedence() {
			break
		}

		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}
		p.nextToken()
		nextExp := infix(leftExp)
		if nextExp == nil {
			return nil
		}
		leftExp = nextExp
	}

	return leftExp
}

// hasContinuationOperator checks if there's an infix operator after newlines
// that should continue the current expression (e.g., |>, >>=, ++, etc.)
func (p *Parser) hasContinuationOperator() bool {
	// Peek ahead past newlines
	tokens := p.stream.Peek(10)
	for _, tok := range tokens {
		if tok.Type == token.NEWLINE {
			continue
		}
		// Check if it's a continuation operator
		return isContinuationOperator(tok.Type)
	}
	return false
}

// isContinuationOperator returns true for operators that can continue on next line
func isContinuationOperator(t token.TokenType) bool {
	switch t {
	case token.PIPE_GT, // |>
		token.CONCAT,      // ++
		token.COMPOSE,     // ,,
		token.USER_OP_APP: // $
		return true
	}
	return false
}

func (p *Parser) parsePrefixExpression() ast.Expression {
	expression := &ast.PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal.(string),
	}
	p.nextToken()
	expression.Right = p.parseExpression(PREFIX)
	return expression
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	expression := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal.(string),
		Left:     left,
	}

	precedence := p.curPrecedence()
	p.nextToken()
	// Allow newline after operator (e.g., x && \n y)
	for p.curToken.Type == token.NEWLINE {
		p.nextToken()
	}
	expression.Right = p.parseExpression(precedence)

	return expression
}

// parseRightAssocInfixExpression parses right-associative operators like ::
// 1 :: 2 :: [] parses as 1 :: (2 :: [])
func (p *Parser) parseRightAssocInfixExpression(left ast.Expression) ast.Expression {
	expression := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal.(string),
		Left:     left,
	}

	precedence := p.curPrecedence()
	p.nextToken()
	// Allow newline after operator
	for p.curToken.Type == token.NEWLINE {
		p.nextToken()
	}
	// Use precedence - 1 to make it right-associative
	expression.Right = p.parseExpression(precedence - 1)

	return expression
}

func (p *Parser) parsePostfixExpression(left ast.Expression) ast.Expression {
	return &ast.PostfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal.(string),
		Left:     left,
	}
}

func (p *Parser) parseGroupedExpression() ast.Expression {
	startToken := p.curToken
	p.nextToken() // consume '('

	// Skip newlines after (
	for p.curTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	// Check for empty tuple ()
	if p.curTokenIs(token.RPAREN) {
		return &ast.TupleLiteral{Token: startToken, Elements: []ast.Expression{}}
	}

	// Check for operator-as-function: (+), (-), (*), etc.
	if p.isOperatorToken() && p.peekTokenIs(token.RPAREN) {
		op := p.curToken.Lexeme
		p.nextToken() // consume operator
		// curToken is now RPAREN, no need to expectPeek
		return &ast.OperatorAsFunction{Token: startToken, Operator: op}
	}

	exp := p.parseExpression(LOWEST)
	if exp == nil {
		// Recover: consume a closing paren if present, or bail out at a boundary.
		for !p.curTokenIs(token.RPAREN) &&
			!p.curTokenIs(token.NEWLINE) &&
			!p.curTokenIs(token.RBRACE) &&
			!p.curTokenIs(token.EOF) {
			p.nextToken()
		}
		if p.peekTokenIs(token.RPAREN) {
			p.nextToken()
		}
		return nil
	}

	// Skip newlines after expression
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	// If we see a comma, it's a tuple
	if p.peekTokenIs(token.COMMA) {
		elements := []ast.Expression{exp}
		for p.peekTokenIs(token.COMMA) {
			p.nextToken() // consume comma
			// Skip newlines after comma (for non-bracket-aware parsers)
			for p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
			// Handle trailing comma
			if p.peekTokenIs(token.RPAREN) {
				break
			}
			p.nextToken() // move to next expression start
			// Skip newlines before expression (for non-bracket-aware parsers)
			for p.curTokenIs(token.NEWLINE) {
				p.nextToken()
			}
			elem := p.parseExpression(LOWEST)
			if elem == nil {
				return nil
			}
			elements = append(elements, elem)
			// Skip newlines after expression (for non-bracket-aware parsers)
			for p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
		}

		if !p.expectPeek(token.RPAREN) {
			return nil
		}
		return &ast.TupleLiteral{Token: startToken, Elements: elements}
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}
	return exp
}
