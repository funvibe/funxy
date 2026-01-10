package parser

import (
	"math/big"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/token"
)

func (p *Parser) parseIntegerLiteral() ast.Expression {
	return &ast.IntegerLiteral{Token: p.curToken, Value: p.curToken.Literal.(int64)}
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	return &ast.FloatLiteral{Token: p.curToken, Value: p.curToken.Literal.(float64)}
}

func (p *Parser) parseBigIntLiteral() ast.Expression {
	return &ast.BigIntLiteral{Token: p.curToken, Value: p.curToken.Literal.(*big.Int)}
}

func (p *Parser) parseRationalLiteral() ast.Expression {
	return &ast.RationalLiteral{Token: p.curToken, Value: p.curToken.Literal.(*big.Rat)}
}

func (p *Parser) parseBoolean() ast.Expression {
	return &ast.BooleanLiteral{Token: p.curToken, Value: p.curTokenIs(token.TRUE)}
}

func (p *Parser) parseNil() ast.Expression {
	return &ast.NilLiteral{Token: p.curToken}
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal.(string)}
}

func (p *Parser) parseFormatStringLiteral() ast.Expression {
	return &ast.FormatStringLiteral{Token: p.curToken, Value: p.curToken.Literal.(string)}
}

func (p *Parser) parseInterpolatedString() ast.Expression {
	tok := p.curToken
	raw := p.curToken.Literal.(string)

	parts := p.parseInterpolationParts(raw)
	if len(parts) == 1 {
		// Optimize: if only one string part, return StringLiteral
		if sl, ok := parts[0].(*ast.StringLiteral); ok {
			return sl
		}
	}

	return &ast.InterpolatedString{Token: tok, Parts: parts}
}

// parseInterpolationParts splits "Hello, ${name}!" into [StringLiteral("Hello, "), Identifier(name), StringLiteral("!")]
func (p *Parser) parseInterpolationParts(raw string) []ast.Expression {
	var parts []ast.Expression
	i := 0
	start := 0

	for i < len(raw) {
		// Look for ${
		if i+1 < len(raw) && raw[i] == '$' && raw[i+1] == '{' {
			// Add text before ${
			if i > start {
				parts = append(parts, &ast.StringLiteral{
					Token: p.curToken,
					Value: raw[start:i],
				})
			}

			// Find matching }
			j := i + 2
			braceDepth := 1
			for j < len(raw) && braceDepth > 0 {
				// Handle nested strings to avoid confusing braces
				if raw[j] == '"' || raw[j] == '\'' || raw[j] == '`' {
					quote := raw[j]
					j++
					for j < len(raw) {
						if raw[j] == '\\' {
							j += 2 // skip escape and char
							continue
						}
						if raw[j] == quote {
							j++
							break
						}
						j++
					}
					continue
				}

				if raw[j] == '{' {
					braceDepth++
				} else if raw[j] == '}' {
					braceDepth--
				}
				j++
			}

			// Parse expression inside ${...}
			exprStr := raw[i+2 : j-1]
			expr := p.parseEmbeddedExpression(exprStr)
			if expr != nil {
				parts = append(parts, expr)
			}

			i = j
			start = j
		} else {
			i++
		}
	}

	// Add remaining text
	if start < len(raw) {
		parts = append(parts, &ast.StringLiteral{
			Token: p.curToken,
			Value: raw[start:],
		})
	}

	return parts
}

// parseEmbeddedExpression parses a string as an expression
func (p *Parser) parseEmbeddedExpression(exprStr string) ast.Expression {
	// Create a new lexer and parser for the embedded expression
	l := lexer.New(exprStr)
	stream := lexer.NewTokenStream(l)
	embeddedParser := New(stream, p.ctx)
	return embeddedParser.parseExpression(LOWEST)
}

func (p *Parser) parseCharLiteral() ast.Expression {
	return &ast.CharLiteral{Token: p.curToken, Value: p.curToken.Literal.(int64)}
}

// parseBytesLiteral parses bytes literals: @"hello", @x"48656C", @b"01001000"
func (p *Parser) parseBytesLiteral() ast.Expression {
	lit := &ast.BytesLiteral{Token: p.curToken}
	lit.Content = p.curToken.Literal.(string)

	switch p.curToken.Type {
	case token.BYTES_STRING:
		lit.Kind = "string"
	case token.BYTES_HEX:
		lit.Kind = "hex"
	case token.BYTES_BIN:
		lit.Kind = "bin"
	}

	return lit
}

// parseBitsLiteral parses bits literals: #b"10101010", #x"FF"
func (p *Parser) parseBitsLiteral() ast.Expression {
	lit := &ast.BitsLiteral{Token: p.curToken}
	lit.Content = p.curToken.Literal.(string)

	switch p.curToken.Type {
	case token.BITS_BIN:
		lit.Kind = "bin"
	case token.BITS_HEX:
		lit.Kind = "hex"
	case token.BITS_OCT:
		lit.Kind = "oct"
	}

	return lit
}

func (p *Parser) parseListLiteral() ast.Expression {
	startToken := p.curToken

	// Skip newlines after [
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	// Empty list []
	if p.peekTokenIs(token.RBRACKET) {
		p.nextToken()
		return &ast.ListLiteral{Token: startToken, Elements: []ast.Expression{}}
	}

	// Parse first expression with precedence that stops before | (PIPE)
	// This allows us to detect list comprehension syntax [expr | ...]
	// BITWISE_OR is the precedence of |, so we use BITWISE_OR - 1 to stop before it
	p.nextToken()
	firstExpr := p.parseExpression(BITWISE_OR) // Stop before | operator
	if firstExpr == nil {
		return nil
	}

	// Skip newlines after first expression
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	// Check for list comprehension syntax: [expr | ...]
	if p.peekTokenIs(token.PIPE) {
		return p.parseListComprehension(startToken, firstExpr)
	}

	// Not a list comprehension - if we stopped at |, we need to continue parsing
	// the expression with full precedence
	if p.peekPrecedence() > LOWEST {
		p.nextToken()
		infix := p.infixParseFns[p.curToken.Type]
		if infix != nil {
			firstExpr = infix(firstExpr)
		}
	}

	// Regular list literal - continue parsing elements
	list := &ast.ListLiteral{Token: startToken}
	list.Elements = []ast.Expression{firstExpr}

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
		if p.peekTokenIs(token.RBRACKET) {
			p.nextToken()
			return list
		}
		p.nextToken()
		expr := p.parseExpression(LOWEST)
		if expr == nil {
			return nil
		}
		list.Elements = append(list.Elements, expr)
		// Skip newlines after element
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}
	}

	if !p.expectPeek(token.RBRACKET) {
		return nil
	}

	return list
}

// parseListComprehension parses a list comprehension after the output expression and |
// Syntax: [output | clause, clause, ...]
// Clause can be: pattern <- iterable (generator) or expression (filter)
func (p *Parser) parseListComprehension(startToken token.Token, output ast.Expression) ast.Expression {
	comp := &ast.ListComprehension{
		Token:  startToken,
		Output: output,
	}

	p.nextToken() // consume |

	// Parse clauses separated by commas
	for {
		// Skip newlines before clause
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		p.nextToken() // move to start of clause

		// Skip newlines at start of clause
		for p.curTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		// Check for end of comprehension
		if p.curTokenIs(token.RBRACKET) {
			break
		}

		// Try to parse as generator: pattern <- iterable
		clause := p.parseCompClause()
		if clause == nil {
			return nil
		}
		comp.Clauses = append(comp.Clauses, clause)

		// Skip newlines after clause
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		// Check for more clauses
		if !p.peekTokenIs(token.COMMA) {
			break
		}
		p.nextToken() // consume comma
	}

	if !p.expectPeek(token.RBRACKET) {
		return nil
	}

	return comp
}

// parseCompClause parses a single clause in a list comprehension
// Either a generator (pattern <- iterable) or a filter (boolean expression)
func (p *Parser) parseCompClause() ast.CompClause {
	// Save current position to try parsing as generator first
	clauseToken := p.curToken

	// Try to parse as pattern
	pattern := p.parsePattern()
	if pattern == nil {
		// Not a valid pattern, must be a filter expression
		// Re-parse as expression
		return p.parseCompFilter(clauseToken)
	}

	// Check for <- to confirm it's a generator
	if p.peekTokenIs(token.L_ARROW) {
		p.nextToken() // consume <-
		p.nextToken() // move to iterable expression

		iterable := p.parseExpression(LOWEST)
		if iterable == nil {
			return nil
		}

		return &ast.CompGenerator{
			Token:    clauseToken,
			Pattern:  pattern,
			Iterable: iterable,
		}
	}

	// No <-, so this is a filter
	// The pattern we parsed might have been an identifier that's actually an expression
	// We need to convert it back or re-parse
	return p.parseCompFilterFromPattern(clauseToken, pattern)
}

// parseCompFilter parses a filter clause (boolean expression)
func (p *Parser) parseCompFilter(startToken token.Token) *ast.CompFilter {
	// Re-position to start token and parse as expression
	// Note: This is a simplified approach - in practice we'd need to handle this better
	expr := p.parseExpression(LOWEST)
	if expr == nil {
		return nil
	}
	return &ast.CompFilter{
		Token:     startToken,
		Condition: expr,
	}
}

// parseCompFilterFromPattern converts a parsed pattern back to a filter expression
// This handles cases like [x | x > 1] where "x > 1" starts with identifier "x"
func (p *Parser) parseCompFilterFromPattern(startToken token.Token, pattern ast.Pattern) *ast.CompFilter {
	// Convert pattern to expression if possible
	var leftExpr ast.Expression

	switch pat := pattern.(type) {
	case *ast.IdentifierPattern:
		// Convert identifier pattern to identifier expression
		leftExpr = &ast.Identifier{
			Token: startToken,
			Value: pat.Value,
		}
	case *ast.LiteralPattern:
		// Convert literal pattern to literal expression
		switch v := pat.Value.(type) {
		case int64:
			leftExpr = &ast.IntegerLiteral{Token: startToken, Value: v}
		case bool:
			leftExpr = &ast.BooleanLiteral{Token: startToken, Value: v}
		case string:
			leftExpr = &ast.StringLiteral{Token: startToken, Value: v}
		default:
			return nil
		}
	default:
		// Can't convert this pattern to expression
		return nil
	}

	if leftExpr == nil {
		return nil
	}

	// Continue parsing the rest of the expression using Pratt parsing
	// This handles cases like x % 2 == 0 where we need to parse the full expression
	for p.peekPrecedence() > LOWEST {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			break
		}
		p.nextToken()
		leftExpr = infix(leftExpr)
	}

	return &ast.CompFilter{
		Token:     startToken,
		Condition: leftExpr,
	}
}
