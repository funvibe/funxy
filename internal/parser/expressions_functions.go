package parser

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/token"
)

func (p *Parser) parseFunctionLiteral() ast.Expression {
	lit := &ast.FunctionLiteral{Token: p.curToken}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()

	// Optional return type
	if p.peekTokenIs(token.COLON) {
		p.nextToken()
		p.nextToken()
		lit.ReturnType = p.parseType()
	} else if p.peekTokenIs(token.ARROW) {
		// Check if it's a return type or expression body
		// Heuristic: If `->` is followed by Type-like tokens and then `{`, it's a return type.
		// Otherwise, it's an expression body.

		isReturnType := false

		// After nextToken(), stream is at position AFTER peekToken.
		// So Peek(n) returns n tokens starting from token AFTER peekToken.
		// peekToken is ARROW, so Peek(1)[0] is the token after ARROW.

		lookahead := p.stream.Peek(50)
		if len(lookahead) >= 1 {
			tokenAfterArrow := lookahead[0] // Token after ARROW

			if tokenAfterArrow.Type == token.IDENT_UPPER || tokenAfterArrow.Type == token.IDENT_LOWER {
				// Case: -> Int { ... } or -> Result<T> { ... } or -> t { ... }
				if len(lookahead) >= 2 {
					tokenAfterType := lookahead[1]
					if tokenAfterType.Type == token.LBRACE {
						// -> Int { - simple return type
						isReturnType = true
					} else if tokenAfterType.Type == token.LT {
						// -> Result<...> { - generic return type
						// Find matching > and check if { follows
						balance := 0
						for i := 1; i < len(lookahead); i++ {
							t := lookahead[i]
							if t.Type == token.LT {
								balance++
							} else if t.Type == token.GT {
								balance--
								if balance == 0 {
									// Found closing GT. Check next token.
									if i+1 < len(lookahead) && lookahead[i+1].Type == token.LBRACE {
										isReturnType = true
									}
									break
								}
							}
						}
					}
				}
			} else if tokenAfterArrow.Type == token.LPAREN {
				// Case: -> (Int, Int) { ... } or -> () -> Int { ... }
				// Find matching ) and check if { follows
				balance := 0
				for i := 0; i < len(lookahead); i++ {
					t := lookahead[i]
					if t.Type == token.LPAREN {
						balance++
					} else if t.Type == token.RPAREN {
						balance--
						if balance == 0 {
							// Found closing ). Check next token.
							if i+1 < len(lookahead) && lookahead[i+1].Type == token.LBRACE {
								isReturnType = true
							}
							break
						}
					}
				}
			} else if tokenAfterArrow.Type == token.LBRACE {
				// Case: -> { x: Int } { ... } (Record Return Type)
				// Find matching } and check if { follows
				balance := 0
				for i := 0; i < len(lookahead); i++ {
					t := lookahead[i]
					if t.Type == token.LBRACE {
						balance++
					} else if t.Type == token.RBRACE {
						balance--
						if balance == 0 {
							// Found closing }. Check next token.
							if i+1 < len(lookahead) && lookahead[i+1].Type == token.LBRACE {
								isReturnType = true
							}
							break
						}
					}
				}
			}
		}

		if isReturnType {
			p.nextToken() // consume ARROW, curToken becomes ARROW
			p.nextToken() // move to Start of Type
			lit.ReturnType = p.parseType()
		}
	} else if !p.peekTokenIs(token.ARROW) && !p.peekTokenIs(token.LBRACE) {
		// Try parsing type (implicit start)
		p.nextToken()
		lit.ReturnType = p.parseType()
	}

	// Body: Block or Expression (after `->`)
	if p.peekTokenIs(token.ARROW) {
		p.nextToken() // consume '->'
		p.nextToken() // start of expression
		// Skip newlines after '->' — allows: fun(x) ->\n    expr
		for p.curTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		// Disambiguate { as block vs record for function body.
		// Empty {} is always a block in function context.
		if p.curTokenIs(token.LBRACE) && p.isEmptyBraces() {
			lit.Body = p.parseBlockStatement()
		} else if p.curTokenIs(token.LBRACE) {
			result := p.parseRecordLiteralOrBlock()
			if block, ok := result.(*ast.BlockStatement); ok {
				lit.Body = block
			} else if result != nil {
				lit.Body = &ast.BlockStatement{
					Token:      lit.Token,
					Statements: []ast.Statement{&ast.ExpressionStatement{Token: result.GetToken(), Expression: result}},
				}
			} else {
				return nil
			}
		} else {
			// Expression body: fun(x) -> x + 1
			bodyExpr := p.parseExpression(LOWEST)
			if bodyExpr == nil {
				return nil
			}
			lit.Body = &ast.BlockStatement{
				Token:      lit.Token,
				Statements: []ast.Statement{&ast.ExpressionStatement{Token: bodyExpr.GetToken(), Expression: bodyExpr}},
			}
		}
	} else if p.peekTokenIs(token.LBRACE) {
		p.nextToken()
		lit.Body = p.parseBlockStatement()
	} else {
		return nil
	}

	return lit
}

// parseLessThanOrTypeApp handles both infix '<' (less than) and Type Application 'expr<Type>'.
func (p *Parser) parseLessThanOrTypeApp(left ast.Expression) ast.Expression {
	// We need to decide if this is 'Left < Right' or 'Left<Type>'.
	// Heuristic: If the token following '<' looks like a Type (Uppercase Identifier), we try to parse it as Type Application.
	// But we need to distinguish `List<Int>` (type app) from `None < Some(1)` (comparison).

	isTypeApp := false

	// Check next token.
	// PascalCase usually means Type.
	if p.peekTokenIs(token.IDENT_UPPER) {
		// Look ahead further: if the uppercase identifier is followed by '(' or another operator,
		// it's likely a value (constructor call or comparison), not a type argument.
		// Type arguments are followed by ',' or '>'.
		// E.g., `List<Int>` - Int is followed by >
		// E.g., `None < Some(1)` - Some is followed by (
		// E.g., `Map<String, Int>` - String is followed by ,

		// Save current position info
		peekLit := p.peekToken.Literal

		// Speculatively check: parse `< UPPER ...` and see what follows
		// We need to look 2 tokens ahead: after '<' and after the UPPER identifier
		// Unfortunately our parser doesn't have easy lookahead beyond peek.
		//
		// Simple heuristic: if left is a known value constructor, treat < as comparison
		if ident, ok := left.(*ast.Identifier); ok {
			identName := ident.Value
			// Known ADT constructors that are values, not type constructors
			knownValueConstructors := map[string]bool{
				"None": true, "Some": true, "Ok": true, "Fail": true,
				"True": true, "False": true,
			}
			if knownValueConstructors[identName] {
				// This is a value constructor, so `<` is comparison
				isTypeApp = false
			} else {
				// Check if this looks like a type name (types are usually capitalized)
				// and the thing after `<` is a type, not a function call
				// For safety, assume it's type application if left is UPPER and peek is UPPER
				// unless we know it's a value constructor
				isTypeApp = true
			}
		} else {
			// left is not a simple identifier - probably an expression
			// so `<` is comparison
			isTypeApp = false
		}

		_ = peekLit // suppress unused warning
	} else if p.peekTokenIs(token.LBRACKET) || p.peekTokenIs(token.LPAREN) {
		// [Int] or (Int, Int) or () -> Int are types.
		// But [1, 2] or (1 + 2) are expressions.
		// Simplification: Only support TApp if it starts with IDENT_UPPER.

		if _, ok := left.(*ast.Identifier); ok {
			if p.peekTokenIs(token.IDENT_UPPER) {
				isTypeApp = true
			}
		} else if _, ok := left.(*ast.MemberExpression); ok {
			if p.peekTokenIs(token.IDENT_UPPER) {
				isTypeApp = true
			}
		}
	}

	if isTypeApp {
		return p.parseTypeApplicationExpression(left)
	}

	// Otherwise, it's infix '<'
	return p.parseInfixExpression(left)
}

func (p *Parser) parseTypeApplicationExpression(left ast.Expression) ast.Expression {
	expr := &ast.TypeApplicationExpression{
		Token:      p.curToken, // The '<' token
		Expression: left,
	}

	// We are at '<'. Advance to the first type token.
	p.nextToken()

	for {
		t := p.parseType()
		if t == nil {
			// Error?
			return nil
		}
		expr.TypeArguments = append(expr.TypeArguments, t)

		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // comma
			p.nextToken() // next type
		} else {
			break
		}
	}

	if !p.expectPeek(token.GT) {
		return nil
	}

	return expr
}

func (p *Parser) parseLambdaExpression() ast.Expression {
	lit := &ast.FunctionLiteral{Token: p.curToken}
	p.nextToken() // consume '\'

	lit.Parameters = []*ast.Parameter{}

	// Check if immediate arrow (no params)
	if !p.curTokenIs(token.ARROW) {
		for {
			param := p.parseParameterCommon(false) // false = don't allow arrow in type (ambiguous with lambda arrow)
			if param != nil {
				lit.Parameters = append(lit.Parameters, param)
			} else {
				return nil
			}

			if p.peekTokenIs(token.COMMA) {
				p.nextToken() // consume comma
				p.nextToken() // move to next param
				// Skip newlines
				for p.curTokenIs(token.NEWLINE) {
					p.nextToken()
				}
			} else {
				break
			}
		}

		if !p.expectPeek(token.ARROW) {
			return nil
		}
	}

	p.nextToken() // consume ->

	// Skip newlines before body
	for p.curTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	// Body: Block or Expression
	// Disambiguate { as block vs record for lambda body.
	// Empty {} is always a block in lambda context.
	if p.curTokenIs(token.LBRACE) && p.isEmptyBraces() {
		lit.Body = p.parseBlockStatement()
	} else if p.curTokenIs(token.LBRACE) {
		result := p.parseRecordLiteralOrBlock()
		if block, ok := result.(*ast.BlockStatement); ok {
			lit.Body = block
		} else if result != nil {
			lit.Body = &ast.BlockStatement{
				Token:      lit.Token,
				Statements: []ast.Statement{&ast.ExpressionStatement{Token: result.GetToken(), Expression: result}},
			}
		} else {
			return nil
		}
	} else {
		bodyExpr := p.parseExpression(LOWEST)
		if bodyExpr == nil {
			return nil
		}
		lit.Body = &ast.BlockStatement{
			Token:      lit.Token,
			Statements: []ast.Statement{&ast.ExpressionStatement{Token: bodyExpr.GetToken(), Expression: bodyExpr}},
		}
	}

	return lit
}

// isEmptyBraces checks if curToken is '{' followed only by newlines and then '}'.
// Used in function/lambda body parsing: empty {} should be a block, not a record.
func (p *Parser) isEmptyBraces() bool {
	if p.peekTokenIs(token.RBRACE) {
		return true
	}
	if !p.peekTokenIs(token.NEWLINE) {
		return false
	}
	peekTokens := p.stream.Peek(50)
	for _, t := range peekTokens {
		if t.Type == token.RBRACE {
			return true
		}
		if t.Type != token.NEWLINE {
			return false
		}
	}
	return false
}

func (p *Parser) parsePrefixSpreadExpression() ast.Expression {
	expression := &ast.SpreadExpression{Token: p.curToken}
	p.nextToken() // consume '...'
	expression.Expression = p.parseExpression(PREFIX)
	return expression
}
