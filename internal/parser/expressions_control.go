package parser

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/token"
)

func (p *Parser) parseIfExpression() ast.Expression {
	expression := &ast.IfExpression{Token: p.curToken}

	p.nextToken() // consume 'if'

	prev := p.disallowTrailingLambda
	p.disallowTrailingLambda = true
	expression.Condition = p.parseExpression(LOWEST)
	p.disallowTrailingLambda = prev

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	expression.Consequence = p.parseBlockStatement()

	// Check for optional ELSE, looking ahead past newlines without consuming them yet
	hasElse := false
	if p.peekTokenIs(token.ELSE) {
		hasElse = true
	} else if p.peekTokenIs(token.NEWLINE) {
		// Look ahead to find ELSE
		lookahead := p.stream.Peek(50)
		for _, t := range lookahead {
			if t.Type == token.NEWLINE {
				continue
			}
			if t.Type == token.ELSE {
				hasElse = true
			}
			break // Found non-newline token
		}
	}

	if hasElse {
		// Consume newlines now that we know there is an ELSE
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		if p.peekTokenIs(token.ELSE) {
			p.nextToken()

			if p.peekTokenIs(token.IF) {
				p.nextToken()
				ifExpr := p.parseIfExpression()
				block := &ast.BlockStatement{
					Token:      token.Token{Type: token.LBRACE, Lexeme: "{"},
					Statements: []ast.Statement{&ast.ExpressionStatement{Token: ifExpr.GetToken(), Expression: ifExpr}},
				}
				expression.Alternative = block
			} else {
				if !p.expectPeek(token.LBRACE) {
					return nil
				}
				expression.Alternative = p.parseBlockStatement()
			}
		}
	}

	return expression
}

func (p *Parser) parseForExpression() ast.Expression {
	expr := &ast.ForExpression{Token: p.curToken}
	p.nextToken() // consume 'for'

	// Check for iteration: for item in iterable
	// Or condition: for condition
	// If next is IDENT and peek after is IN, then iteration.
	if p.curTokenIs(token.IDENT_LOWER) && p.peekTokenIs(token.IN) {
		// Iteration loop
		expr.ItemName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
		p.nextToken() // consume ident
		p.nextToken() // consume in

		prev := p.disallowTrailingLambda
		p.disallowTrailingLambda = true
		expr.Iterable = p.parseExpression(LOWEST)
		p.disallowTrailingLambda = prev
	} else {
		// Standard condition loop
		prev := p.disallowTrailingLambda
		p.disallowTrailingLambda = true
		expr.Condition = p.parseExpression(LOWEST)
		p.disallowTrailingLambda = prev
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	expr.Body = p.parseBlockStatement()
	return expr
}

func (p *Parser) parseMatchExpression() ast.Expression {
	ce := &ast.MatchExpression{Token: p.curToken}

	p.nextToken() // consume 'match'

	prev := p.disallowTrailingLambda
	p.disallowTrailingLambda = true
	ce.Expression = p.parseExpression(LOWEST)
	p.disallowTrailingLambda = prev

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	// Consume optional newline after '{'
	if p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	for !p.peekTokenIs(token.RBRACE) && !p.peekTokenIs(token.EOF) {
		// Skip newlines between arms
		if p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
			continue
		}

		if p.peekTokenIs(token.RBRACE) {
			break
		}

		// We are at start of an arm (pattern)
		// p.peekToken is the start of pattern.
		// We must advance curToken to it.
		p.nextToken()

		arm := p.parseMatchArm()
		if arm != nil {
			ce.Arms = append(ce.Arms, arm)
		}

		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
		}
	}

	if !p.expectPeek(token.RBRACE) {
		return nil
	}

	return ce
}
