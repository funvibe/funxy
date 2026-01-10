package parser

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/token"
)

func (p *Parser) parseMatchArm() *ast.MatchArm {
	pattern := p.parsePattern()
	if pattern == nil {
		return nil
	}

	// Parse optional guard: pattern if condition -> expression
	var guard ast.Expression
	if p.peekTokenIs(token.IF) {
		p.nextToken() // consume current token (end of pattern)
		p.nextToken() // consume 'if', move to guard expression
		// Parse guard expression with precedence just above ARROW
		// to stop before -> but allow most operators
		guard = p.parseExpression(LOWEST)
	}

	if !p.expectPeek(token.ARROW) {
		return nil
	}

	p.nextToken() // consume '->'
	expr := p.parseExpression(LOWEST)

	return &ast.MatchArm{Pattern: pattern, Guard: guard, Expression: expr}
}

func (p *Parser) parsePattern() ast.Pattern {
	if p.curTokenIs(token.IDENT_UPPER) {
		return p.parseConstructorPattern()
	}
	if p.curTokenIs(token.LBRACE) {
		return p.parseRecordPattern()
	}
	// Wildcard '_' is handled in parseAtomicPattern, not here directly as a prefix function
	// because registerPrefix for UNDERSCORE was removed to avoid conflict with expression parsing.
	// But wait, parsePattern is called by parseMatchArm and recursively.
	// If we removed registerPrefix(UNDERSCORE), then p.parseExpression(LOWEST) won't work if it starts with _.
	// But patterns are NOT expressions. They are parsed via parsePattern.
	// So checking here is correct.

	return p.parseAtomicPattern()
}

func (p *Parser) parseRecordPattern() ast.Pattern {
	rp := &ast.RecordPattern{Token: p.curToken, Fields: make(map[string]ast.Pattern)}
	p.nextToken() // consume {

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.NEWLINE) {
			p.nextToken()
			continue
		}

		if !p.curTokenIs(token.IDENT_LOWER) && !p.curTokenIs(token.IDENT_UPPER) {
			return nil // Expected field name
		}
		key := p.curToken.Literal.(string)

		if !p.expectPeek(token.COLON) {
			return nil
		}
		p.nextToken() // consume :

		valPat := p.parsePattern()
		rp.Fields[key] = valPat

		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
		}
		p.nextToken()
	}

	return rp
}

func (p *Parser) parseAtomicPattern() ast.Pattern {
	switch p.curToken.Type {
	case token.CARET:
		// Pin pattern: ^variable (compare with existing value)
		caretToken := p.curToken
		if !p.expectPeek(token.IDENT_LOWER) {
			return nil
		}
		return &ast.PinPattern{Token: caretToken, Name: p.curToken.Literal.(string)}
	case token.INT:
		return &ast.LiteralPattern{Token: p.curToken, Value: p.curToken.Literal}
	case token.TRUE, token.FALSE:
		return &ast.LiteralPattern{Token: p.curToken, Value: p.curTokenIs(token.TRUE)}
	case token.STRING:
		str := p.curToken.Literal.(string)
		// Check if string contains capture patterns {name} or {name...}
		if parts := p.parseStringPatternParts(str); parts != nil {
			return &ast.StringPattern{Token: p.curToken, Parts: parts}
		}
		return &ast.LiteralPattern{Token: p.curToken, Value: str}
	case token.CHAR:
		return &ast.LiteralPattern{Token: p.curToken, Value: p.curToken.Literal}
	case token.UNDERSCORE:
		// Check for type pattern: _: Nil
		if p.peekTokenIs(token.COLON) {
			nameToken := p.curToken
			p.nextToken() // consume '_'
			p.nextToken() // consume ':'
			typeAst := p.parseTypeApplication()
			return &ast.TypePattern{
				Token: nameToken,
				Name:  "_",
				Type:  typeAst,
			}
		}
		return &ast.WildcardPattern{Token: p.curToken}
	case token.IDENT_LOWER:
		// Check for type pattern: n: Int
		if p.peekTokenIs(token.COLON) {
			nameToken := p.curToken
			name := p.curToken.Literal.(string)
			p.nextToken() // consume identifier
			p.nextToken() // consume ':'
			// Use parseTypeApplication to avoid consuming -> as part of function type
			// In patterns, we only want simple types like Int, String, List<Int>
			typeAst := p.parseTypeApplication()
			return &ast.TypePattern{
				Token: nameToken,
				Name:  name,
				Type:  typeAst,
			}
		}
		return &ast.IdentifierPattern{Token: p.curToken, Value: p.curToken.Literal.(string)}
	case token.IDENT_UPPER:
		// 0-ary constructor pattern
		return &ast.ConstructorPattern{
			Token:    p.curToken,
			Name:     &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)},
			Elements: []ast.Pattern{},
		}
	case token.ELLIPSIS:
		// Spread pattern: ...name
		tok := p.curToken
		p.nextToken() // consume ...
		pat := p.parsePattern()
		if pat == nil {
			return nil
		}
		return &ast.SpreadPattern{Token: tok, Pattern: pat}
	case token.LPAREN:
		startToken := p.curToken
		p.nextToken()

		// Empty tuple pattern ()
		if p.curTokenIs(token.RPAREN) {
			return &ast.TuplePattern{Token: startToken, Elements: []ast.Pattern{}}
		}

		pat := p.parsePattern()

		// Check for comma (tuple)
		if p.peekTokenIs(token.COMMA) {
			elements := []ast.Pattern{pat}
			for p.peekTokenIs(token.COMMA) {
				p.nextToken()
				p.nextToken()
				nextPat := p.parsePattern()
				elements = append(elements, nextPat)
			}
			if !p.expectPeek(token.RPAREN) {
				return nil
			}
			return &ast.TuplePattern{Token: startToken, Elements: elements}
		}

		if !p.expectPeek(token.RPAREN) {
			return nil
		}
		return pat // Parenthesized pattern
	case token.LBRACKET:
		startToken := p.curToken
		p.nextToken()

		// Empty list pattern []
		if p.curTokenIs(token.RBRACKET) {
			return &ast.ListPattern{Token: startToken, Elements: []ast.Pattern{}}
		}

		// List pattern elements
		elements := []ast.Pattern{}
		for {
			pat := p.parsePattern()
			elements = append(elements, pat)

			if p.peekTokenIs(token.COMMA) {
				p.nextToken()
				p.nextToken()
			} else {
				break
			}
		}

		if !p.expectPeek(token.RBRACKET) {
			return nil
		}
		return &ast.ListPattern{Token: startToken, Elements: elements}
	case token.LBRACE:
		// Record pattern { x: p1, y: p2 }
		return p.parseRecordPattern()
	default:
		return nil
	}
}

func (p *Parser) parseConstructorPattern() ast.Pattern {
	cp := &ast.ConstructorPattern{
		Token: p.curToken,
		Name:  &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)},
	}

	// Check for named record pattern: Point { x: p1, y: p2 }
	if p.peekTokenIs(token.LBRACE) {
		typeName := p.curToken.Literal.(string)
		p.nextToken() // consume type name, now at '{'
		rp := p.parseRecordPattern()
		if rp != nil {
			if recPat, ok := rp.(*ast.RecordPattern); ok {
				recPat.TypeName = typeName
			}
		}
		return rp
	}

	// Check if we have C-style arguments (a, b)
	if p.peekTokenIs(token.LPAREN) {
		p.nextToken() // consume '('. curToken is now '('

		if p.peekTokenIs(token.RPAREN) {
			p.nextToken() // consume ')'
			return cp
		}

		p.nextToken() // move to start of first argument

		// First arg
		pat := p.parsePattern()
		cp.Elements = append(cp.Elements, pat)

		for p.peekTokenIs(token.COMMA) {
			p.nextToken() // consume comma. curToken is comma.
			p.nextToken() // move to start of next pattern
			pat = p.parsePattern()
			cp.Elements = append(cp.Elements, pat)
		}

		if !p.expectPeek(token.RPAREN) {
			return nil
		}
		return cp
	}

	// Fallback to ML style: Cons arg1 arg2
	// We parse arguments until we hit a token that cannot be start of a pattern,
	// OR we hit tokens that signal end of pattern (->, ,, ), ]).
	// Actually, ML style arguments are atomic patterns.
	// Cons (a, b) c -> Cons has 2 args: (a,b) and c.
	// Cons a b -> Cons has 2 args: a and b.

	// We should look ahead.
	for {
		if p.peekTokenIs(token.ARROW) || p.peekTokenIs(token.COMMA) ||
			p.peekTokenIs(token.RPAREN) || p.peekTokenIs(token.RBRACE) ||
			p.peekTokenIs(token.RBRACKET) || p.peekTokenIs(token.EOF) ||
			p.peekTokenIs(token.COLON) { // Colon for type annotation? Or record field?
			break
		}

		// If next token is NOT a start of atomic pattern, break.
		// Atomic starts: INT, TRUE, FALSE, STRING, CHAR, UNDERSCORE, IDENT, LPAREN, LBRACKET, LBRACE?
		// LBRACE is Record. LBRACKET is List.
		// Wait, Cons {x:1} is valid.

		// Note: parseAtomicPattern handles these.
		// We can try to parse. But if we parse something that is NOT a pattern, we fail.
		// But parser expects pattern.

		// Check peek token type.
		tt := p.peekToken.Type
		if tt != token.INT && tt != token.TRUE && tt != token.FALSE &&
			tt != token.STRING && tt != token.CHAR && tt != token.UNDERSCORE &&
			tt != token.IDENT_LOWER && tt != token.IDENT_UPPER &&
			tt != token.LPAREN && tt != token.LBRACKET && tt != token.LBRACE &&
			tt != token.CARET {
			break
		}

		p.nextToken()
		arg := p.parseAtomicPattern() // Only atomic patterns are allowed as args in ML style (without parens)
		if arg == nil {
			break
		}
		cp.Elements = append(cp.Elements, arg)
	}

	return cp
}

// parseStringPatternParts parses a string looking for {name} or {name...} capture patterns.
// Returns nil if no captures found AND no escape sequences (plain string), otherwise returns the parts.
// Supports escaping: {{ becomes literal {, }} becomes literal }
func (p *Parser) parseStringPatternParts(s string) []ast.StringPatternPart {
	var parts []ast.StringPatternPart
	hasCapture := false
	hasEscape := false
	var currentLiteral string

	i := 0
	for i < len(s) {
		if s[i] == '{' {
			// Check for escaped {{ -> literal {
			if i+1 < len(s) && s[i+1] == '{' {
				currentLiteral += "{"
				hasEscape = true
				i += 2
				continue
			}

			// Flush current literal if any
			if currentLiteral != "" {
				parts = append(parts, ast.StringPatternPart{
					IsCapture: false,
					Value:     currentLiteral,
				})
				currentLiteral = ""
			}

			i++ // skip '{'

			// Find the closing '}'
			nameStart := i
			for i < len(s) && s[i] != '}' {
				i++
			}

			if i >= len(s) {
				// No closing '}' - treat as literal
				currentLiteral += "{" + s[nameStart:]
				break
			}

			name := s[nameStart:i]
			greedy := false

			// Check for greedy pattern {name...}
			if len(name) > 3 && name[len(name)-3:] == "..." {
				name = name[:len(name)-3]
				greedy = true
			}

			// Validate capture name (must be valid identifier)
			if isValidIdentifier(name) {
				parts = append(parts, ast.StringPatternPart{
					IsCapture: true,
					Value:     name,
					Greedy:    greedy,
				})
				hasCapture = true
			} else {
				// Invalid name - treat as literal
				currentLiteral += "{" + s[nameStart:i] + "}"
			}

			i++ // skip '}'
		} else if s[i] == '}' {
			// Check for escaped }} -> literal }
			if i+1 < len(s) && s[i+1] == '}' {
				currentLiteral += "}"
				hasEscape = true
				i += 2
				continue
			}
			// Single } without matching { - just a literal
			currentLiteral += "}"
			i++
		} else {
			currentLiteral += string(s[i])
			i++
		}
	}

	// Flush remaining literal
	if currentLiteral != "" {
		parts = append(parts, ast.StringPatternPart{
			IsCapture: false,
			Value:     currentLiteral,
		})
	}

	// Return parts if we have captures OR escape sequences
	// (escape sequences need StringPattern to properly compare the unescaped value)
	if !hasCapture && !hasEscape {
		return nil // Plain string - use regular LiteralPattern
	}

	return parts
}

// isValidIdentifier checks if a string is a valid lowercase identifier
func isValidIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}
	// First char must be lowercase letter or underscore
	if !((s[0] >= 'a' && s[0] <= 'z') || s[0] == '_') {
		return false
	}
	// Rest can be letters, digits, or underscore
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}
