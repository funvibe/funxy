package parser

import (
	"fmt"

	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
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

			// Skip newlines after 'else' — allows: } else\n if / } else\n {
			for p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}

			if p.peekTokenIs(token.IF) {
				p.nextToken()
				ifExpr := p.parseIfExpression()
				if ifExpr != nil {
					block := &ast.BlockStatement{
						Token:      token.Token{Type: token.LBRACE, Lexeme: "{"},
						Statements: []ast.Statement{&ast.ExpressionStatement{Token: ifExpr.GetToken(), Expression: ifExpr}},
					}
					expression.Alternative = block
				}
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

	// Parse the loop header. It is either an iteration target (an identifier or a
	// destructuring pattern) followed by `in`, or a `while`-style condition.
	// Since `in` is not an infix operator, expression parsing stops right before it,
	// so we disambiguate the two forms by checking for `in` afterwards.
	prev := p.disallowTrailingLambda
	p.disallowTrailingLambda = true
	header := p.parseExpression(LOWEST)
	p.disallowTrailingLambda = prev
	if header == nil {
		return nil
	}

	var destructurePattern ast.Pattern
	var hiddenName string

	if p.peekTokenIs(token.IN) {
		// Iteration loop
		if id, ok := header.(*ast.Identifier); ok {
			expr.ItemName = id
		} else {
			// Destructuring target (e.g. `for (k, v) in xs`). Desugar into a hidden
			// item bound by a pattern assignment prepended to the loop body, reusing
			// the existing pattern-assignment machinery.
			//
			// NOTE: this desugaring happens in the parser, so the AST loses the original
			// surface pattern (ItemName becomes the synthetic `$foritemN`). That's fine
			// today because there is no formatter command. If a `fmt`/round-trip printer
			// is ever added, store the original pattern on ast.ForExpression (e.g. an
			// `ItemPattern` field) and reconstruct `for (k, v) in ...` from it instead of
			// emitting the desugared `$foritemN`.
			destructurePattern = p.exprToPattern(header)
			if destructurePattern == nil {
				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
					diagnostics.ErrP006, header.GetToken(),
					"invalid for-loop binding: expected an identifier or a destructuring pattern",
				))
				return nil
			}
			hiddenName = fmt.Sprintf("$foritem%d", p.forPatternCounter)
			p.forPatternCounter++
			expr.ItemName = &ast.Identifier{Token: header.GetToken(), Value: hiddenName}
		}

		p.nextToken() // consume last header token -> curToken = in
		p.nextToken() // consume in -> curToken = first token of iterable
		// Skip newlines after 'in' — allows: for x in\n    list
		for p.curTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		prev := p.disallowTrailingLambda
		p.disallowTrailingLambda = true
		expr.Iterable = p.parseExpression(LOWEST)
		p.disallowTrailingLambda = prev
	} else {
		// Standard condition loop
		expr.Condition = header
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	expr.Body = p.parseBlockStatement()

	// Prepend `<pattern> = <hidden item>` so the body binds the destructured names.
	if destructurePattern != nil && expr.Body != nil {
		assign := &ast.PatternAssignExpression{
			Token:   header.GetToken(),
			Pattern: destructurePattern,
			Value:   &ast.Identifier{Token: header.GetToken(), Value: hiddenName},
		}
		stmt := &ast.ExpressionStatement{Token: header.GetToken(), Expression: assign}
		expr.Body.Statements = append([]ast.Statement{stmt}, expr.Body.Statements...)
	}

	return expr
}

func (p *Parser) parseMatchExpression() ast.Expression {
	ce := &ast.MatchExpression{Token: p.curToken}

	p.nextToken() // consume 'match'

	prev := p.disallowTrailingLambda
	p.disallowTrailingLambda = true
	ce.Expression = p.parseExpression(LOWEST)
	p.disallowTrailingLambda = prev
	if ce.Expression == nil {
		return nil
	}

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
