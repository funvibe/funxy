package parser

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/token"
)

func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	exp := &ast.CallExpression{Token: p.curToken, Function: function}

	// Parse arguments (handling Named Args sugar)
	exp.Arguments = p.parseCallArguments()

	// Handle Block Syntax (Trailing Lambda/List)
	// If followed by { ... }, treat as list of expressions and append as last argument
	if !p.disallowTrailingLambda && p.peekTokenIs(token.LBRACE) {
		// Only if no newline before brace? RFC doesn't specify, but usually yes for trailing blocks.
		// Check for newline
		if !p.peekTokenIs(token.NEWLINE) {
			p.nextToken() // consume {
			blockExprs := p.parseBlockAsList()
			if blockExprs != nil {
				exp.Arguments = append(exp.Arguments, blockExprs)
			}
		}
	}

	return exp
}

// parseCallArguments parses arguments for a function call, handling Named Args sugar
// func(a: 1, b: 2) -> func({a: 1, b: 2})
// func(1, b: 2) -> func(1, {b: 2})
func (p *Parser) parseCallArguments() []ast.Expression {
	args := []ast.Expression{}
	namedArgs := make(map[string]ast.Expression)
	var namedArgsOrder []string
	isNamedMode := false

	// Move past LPAREN
	p.nextToken()

	// Skip leading newlines
	for p.curTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	// Check for empty call )
	if p.curTokenIs(token.RPAREN) {
		return args
	}

	for {
		// Skip newlines before argument
		for p.curTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		// Check if we hit RPAREN (trailing comma case or just newlines)
		if p.curTokenIs(token.RPAREN) {
			break
		}

		// Check if this is a named argument: IDENT : ...
		isNamed := false
		if (p.curTokenIs(token.IDENT_LOWER) || p.curTokenIs(token.IDENT_UPPER)) && p.peekTokenIs(token.COLON) {
			isNamed = true
		}

		if isNamed {
			isNamedMode = true
			key := p.curToken.Literal.(string)
			p.nextToken() // consume key
			p.nextToken() // consume :

			// Skip newlines after :
			for p.curTokenIs(token.NEWLINE) {
				p.nextToken()
			}

			val := p.parseExpression(LOWEST)
			namedArgs[key] = val
			namedArgsOrder = append(namedArgsOrder, key)
		} else {
			if isNamedMode {
				// Error: Positional argument after named argument
				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
					diagnostics.ErrP005,
					p.curToken,
					"positional argument cannot follow named arguments",
				))
				return nil
			}
			expr := p.parseExpression(LOWEST)
			if expr == nil {
				return nil
			}

			// Spread arguments use prefix syntax: ...args (handled by parsePrefixSpreadExpression)
			// Postfix syntax args... is not supported

			args = append(args, expr)
		}

		// Skip newlines before checking for comma
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		// Check for comma
		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // move to comma
			p.nextToken() // move past comma

			// Skip newlines after comma
			for p.curTokenIs(token.NEWLINE) {
				p.nextToken()
			}

			// Check for trailing comma
			if p.curTokenIs(token.RPAREN) {
				goto Done
			}
		} else {
			// If no comma, we expect RPAREN (possibly after newlines)
			break
		}
	}

	// Skip trailing newlines before RPAREN
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

Done:
	// If we collected named args, bundle them into a RecordLiteral
	if len(namedArgs) > 0 {
		rec := &ast.RecordLiteral{
			Token:  token.Token{Type: token.LBRACE, Lexeme: "{", Line: p.curToken.Line, Column: p.curToken.Column},
			Fields: namedArgs,
		}
		args = append(args, rec)
	}

	return args
}

// parseBlockAsList parses { expr1 \n expr2 } as [expr1, expr2]
func (p *Parser) parseBlockAsList() *ast.ListLiteral {
	list := &ast.ListLiteral{
		Token: p.curToken, // The { token
	}
	elements := []ast.Expression{}

	p.nextToken() // consume {

	// Skip leading newlines
	for p.curTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		stmt := p.parseExpressionStatementOrConstDecl()
		if stmt != nil {
			if exprStmt, ok := stmt.(*ast.ExpressionStatement); ok {
				elements = append(elements, exprStmt.Expression)
			} else {
				// Error: Block syntax only supports expressions
				var tok token.Token
				if prov, ok := stmt.(ast.TokenProvider); ok {
					tok = prov.GetToken()
				} else {
					tok = p.curToken
				}

				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
					diagnostics.ErrP005,
					tok,
					"block syntax in arguments only supports expressions",
				))
			}
		}

		p.nextToken()
		// Skip newlines
		for p.curTokenIs(token.NEWLINE) {
			p.nextToken()
		}
	}

	// We are at RBRACE or EOF.
	// If EOF, it's an error (missing }) but we'll return what we have or let expectation fail?
	// The loop terminates on RBRACE. So curToken is RBRACE.

	// Check if we actually ended with RBRACE (loop could end on EOF)
	if p.curTokenIs(token.EOF) {
		// Error: expected }
		return nil
	}

	list.Elements = elements
	return list
}

func (p *Parser) parseExpressionList(end token.TokenType) []ast.Expression {
	list := []ast.Expression{}

	// Skip newlines after opening bracket (for multiline lists)
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	p.nextToken()
	expr := p.parseExpression(LOWEST)
	if expr == nil {
		return nil // Parse error
	}
	// Spread in lists uses prefix syntax: ...args (handled by parsePrefixSpreadExpression)
	// Postfix syntax args... is not supported
	list = append(list, expr)

	// Skip newlines after element
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // consume comma
		// Skip newlines after comma
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}
		// Handle trailing comma
		if p.peekTokenIs(end) {
			p.nextToken()
			return list
		}
		p.nextToken()
		expr := p.parseExpression(LOWEST)
		if expr == nil {
			return nil // Parse error
		}
		// Spread in lists uses prefix syntax: ...args (handled by parsePrefixSpreadExpression)
		// Postfix syntax args... is not supported
		list = append(list, expr)
		// Skip newlines after element
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}
	}

	if !p.expectPeek(end) {
		return nil
	}

	return list
}
