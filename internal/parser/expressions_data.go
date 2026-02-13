package parser

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/token"
)

// parseMapLiteral parses a map literal: %{ key => value, key2 => value2 }
func (p *Parser) parseMapLiteral() ast.Expression {
	mapLit := &ast.MapLiteral{Token: p.curToken}
	mapLit.Pairs = []struct{ Key, Value ast.Expression }{}

	// Skip newlines after %{
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	// Empty map: %{}
	if p.peekTokenIs(token.RBRACE) {
		p.nextToken()
		return mapLit
	}

	// Parse first pair
	p.nextToken()
	// Skip newlines before key
	for p.curTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	// Use PIPE_PREC to stop before => (which has PIPE_PREC precedence)
	key := p.parseExpression(PIPE_PREC)

	// Skip newlines before =>
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	if !p.expectPeek(token.USER_OP_IMPLY) { // =>
		return nil
	}
	p.nextToken()
	// Skip newlines after =>
	for p.curTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	// Value uses PIPE_PREC to stop before , or }
	value := p.parseExpression(PIPE_PREC)
	mapLit.Pairs = append(mapLit.Pairs, struct{ Key, Value ast.Expression }{key, value})

	// Skip newlines after value
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	// Parse remaining pairs
	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // consume comma
		// Skip newlines after comma
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}
		// Handle trailing comma
		if p.peekTokenIs(token.RBRACE) {
			break
		}
		p.nextToken()
		// Skip newlines before key
		for p.curTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		// Use PIPE_PREC to stop before =>
		key := p.parseExpression(PIPE_PREC)

		// Skip newlines before =>
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		if !p.expectPeek(token.USER_OP_IMPLY) { // =>
			return nil
		}
		p.nextToken()
		// Skip newlines after =>
		for p.curTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		// Value uses PIPE_PREC to stop before , or }
		value := p.parseExpression(PIPE_PREC)
		mapLit.Pairs = append(mapLit.Pairs, struct{ Key, Value ast.Expression }{key, value})

		// Skip newlines after value
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}
	}

	if !p.expectPeek(token.RBRACE) {
		return nil
	}

	return mapLit
}

func (p *Parser) parseRecordLiteralOrBlock() ast.Expression {
	// Disambiguate Block vs Record

	// 1. Check for {} (Empty Record)
	if p.peekTokenIs(token.RBRACE) {
		rec := p.parseRecordLiteral()
		if rec == nil {
			return nil
		}
		return rec
	}

	// 2. Check for { ...expr } (Record Spread) - this is always a record
	if p.peekTokenIs(token.ELLIPSIS) {
		rec := p.parseRecordLiteral()
		if rec == nil {
			return nil
		}
		return rec
	}

	// 3. Check for { key: val } (Non-empty Record) - single line
	isRecord := false
	if p.peekTokenIs(token.IDENT_LOWER) || p.peekTokenIs(token.IDENT_UPPER) {
		peekNext := p.stream.Peek(1)
		if len(peekNext) >= 1 && peekNext[0].Type == token.COLON {
			isRecord = true
		}
	}

	// 4. Check for multiline record: { \n key: val } or { \n ...expr }
	// But NOT type annotation: { \n var: Type = expr }
	// Type annotation has = after type, record doesn't
	if !isRecord && p.peekTokenIs(token.NEWLINE) {
		peekTokens := p.stream.Peek(50) // tokens AFTER peekToken (which is NEWLINE)
		// Find first non-newline token
		idx := 0
		for idx < len(peekTokens) && peekTokens[idx].Type == token.NEWLINE {
			idx++
		}
		if idx < len(peekTokens) {
			first := peekTokens[idx]
			if first.Type == token.RBRACE {
				// Empty record with newlines: { \n }
				isRecord = true
			} else if first.Type == token.ELLIPSIS {
				// Record spread: { \n ...expr }
				isRecord = true
			} else if first.Type == token.IDENT_LOWER || first.Type == token.IDENT_UPPER {
				// Find colon after ident
				colonIdx := idx + 1
				for colonIdx < len(peekTokens) && peekTokens[colonIdx].Type == token.NEWLINE {
					colonIdx++
				}
				if colonIdx < len(peekTokens) && peekTokens[colonIdx].Type == token.COLON {
					// Found ident: - look for = to detect type annotation
					// Type annotation: var: Type = expr OR var: Type<A, B> = expr
					// Record: key: value (no = before newline/comma/})
					hasAssign := false
					angleBalance := 0
					for checkIdx := colonIdx + 1; checkIdx < len(peekTokens); checkIdx++ {
						tt := peekTokens[checkIdx].Type
						if tt == token.LT {
							angleBalance++
						} else if tt == token.GT {
							angleBalance--
						} else if angleBalance == 0 {
							// Only check for terminators when not inside <...>
							if tt == token.NEWLINE || tt == token.RBRACE {
								break // End of field/statement
							}
							if tt == token.COMMA {
								break // End of record field
							}
							if tt == token.ASSIGN || tt == token.COLON_MINUS {
								hasAssign = true
								break
							}
						}
					}
					if !hasAssign {
						isRecord = true // No = found, so it's a record field
					}
				}
			}
		}
	}

	if isRecord {
		rec := p.parseRecordLiteral()
		if rec == nil {
			return nil // Return untyped nil for proper nil check
		}
		return rec
	}

	// 4. Default to Block
	return p.parseBlockStatement()
}

func (p *Parser) parseRecordLiteral() *ast.RecordLiteral {
	rl := &ast.RecordLiteral{Token: p.curToken, Fields: make(map[string]ast.Expression)}
	p.nextToken() // consume {

	// Skip leading newlines
	for p.curTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	// Check for spread: { ...expr, ... }
	if p.curTokenIs(token.ELLIPSIS) {
		p.nextToken() // consume ...
		rl.Spread = p.parseExpression(LOWEST)

		// Skip newlines after spread expression
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		// After spread, expect comma or }
		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // consume comma
			p.nextToken() // move to next token
		} else if p.peekTokenIs(token.RBRACE) {
			p.nextToken() // consume }
			return rl
		} else {
			p.nextToken() // move forward
		}

		// Skip newlines
		for p.curTokenIs(token.NEWLINE) {
			p.nextToken()
		}
	}

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.NEWLINE) {
			p.nextToken()
			continue
		}

		if !p.curTokenIs(token.IDENT_LOWER) && !p.curTokenIs(token.IDENT_UPPER) {
			p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(diagnostics.ErrP004, p.curToken, p.curToken.Type))
			return nil // Expected identifier key
		}
		key := p.curToken.Literal.(string)

		if !p.expectPeek(token.COLON) {
			return nil
		}
		p.nextToken() // consume :

		// Skip newlines before value
		for p.curTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		val := p.parseExpression(LOWEST)
		if val == nil {
			return nil // Failed to parse value expression
		}
		rl.Fields[key] = val

		// Skip newlines after value
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // consume comma
			// Skip newlines after comma
			for p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
		}
		p.nextToken()
	}

	return rl
}
