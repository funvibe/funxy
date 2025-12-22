package parser

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/token"
)

func (p *Parser) parseType() ast.Type {
	// Parse primary type (including function types)
	t := p.parseNonUnionType()
	if t == nil {
		return nil
	}

	// Check for T? (nullable shorthand for T | Nil)
	if p.peekTokenIs(token.QUESTION) {
		p.nextToken() // consume '?'
		nilType := &ast.NamedType{
			Token: p.curToken,
			Name:  &ast.Identifier{Token: p.curToken, Value: "Nil"},
		}
		return &ast.UnionType{
			Token: t.GetToken(),
			Types: []ast.Type{t, nilType},
		}
	}

	// Check for Union Type '|'
	if p.peekTokenIs(token.PIPE) {
		types := []ast.Type{t}
		for p.peekTokenIs(token.PIPE) {
			p.nextToken() // consume '|'
			p.nextToken() // move to next type
			nextType := p.parseNonUnionType()
			if nextType == nil {
				return nil
			}
			types = append(types, nextType)
		}
		return &ast.UnionType{
			Token: t.GetToken(),
			Types: types,
		}
	}

	return t
}

// parseNonUnionType handles function types and below (no union)
func (p *Parser) parseNonUnionType() ast.Type {
	// Check for Record Type { x: Int }
	if p.curTokenIs(token.LBRACE) {
		return p.parseRecordType()
	}

	t := p.parseTypeApplication()
	if t == nil {
		return nil
	}

	// Check for Function Type '->'
	if p.peekTokenIs(token.ARROW) {
		p.nextToken()            // consume '->'
		p.nextToken()            // move to return type
		retType := p.parseType() // recursive - allows union in return type

		var params []ast.Type
		if tt, ok := t.(*ast.TupleType); ok {
			params = tt.Types
		} else {
			params = []ast.Type{t}
		}

		return &ast.FunctionType{
			Token:      t.GetToken(),
			Parameters: params,
			ReturnType: retType,
		}
	}
	return t
}

func (p *Parser) parseRecordType() ast.Type {
	rt := &ast.RecordType{Token: p.curToken, Fields: make(map[string]ast.Type)}
	p.nextToken() // consume {

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.NEWLINE) {
			p.nextToken()
			continue
		}

		if !p.curTokenIs(token.IDENT_LOWER) && !p.curTokenIs(token.IDENT_UPPER) {
			return nil // Expected key
		}
		key := p.curToken.Literal.(string)
		// Do not consume key yet

		if !p.expectPeek(token.COLON) {
			return nil
		}
		p.nextToken() // consume :

		valType := p.parseType()
		rt.Fields[key] = valType

		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
		}
		p.nextToken()
	}
	return rt
}

func (p *Parser) parseTypeApplication() ast.Type {
	// Parse base type (Constructor)
	base := p.parseAtomicType()
	if base == nil {
		return nil
	}

	// Check for Generic Arguments <A, B>
	if p.peekTokenIs(token.LT) {
		p.nextToken() // consume < (curToken is now <)
		p.nextToken() // move to first type arg

		for !p.curTokenIs(token.GT) && !p.curTokenIs(token.EOF) {
			if p.curTokenIs(token.COMMA) {
				p.nextToken()
				continue
			}

			arg := p.parseType()
			if arg == nil {
				return nil
			}

			if nt, ok := base.(*ast.NamedType); ok {
				nt.Args = append(nt.Args, arg)
			}

			// After parseType(), check where we are
			// Case 1: Recursive call consumed >> and left splitRshift=true
			// curToken is synthetic > from the split
			if p.curTokenIs(token.GT) {
				// We're done with this generic (either normal > or split >)
				break
			}

			// Case 2: Normal case - check peek for what's next
			if p.peekTokenIs(token.COMMA) {
				p.nextToken() // move curToken to comma
				p.nextToken() // move to next type arg
			} else if p.peekTokenIs(token.GT) {
				p.nextToken() // move curToken to GT
			} else if p.peekTokenIs(token.RSHIFT) {
				// We see >> where we expect >
				// Consume it and split: first > closes this generic
				p.nextToken() // move curToken to >>
				// Set flag so next nextToken returns synthetic >
				p.splitRshift = true
				// Now call nextToken to get the synthetic >
				p.nextToken() // curToken becomes synthetic >
				break
			} else {
				// Unexpected token
				return nil
			}
		}

		if !p.curTokenIs(token.GT) {
			return nil
		}
	}
	return base
}

func (p *Parser) parseAtomicType() ast.Type {
	if p.curTokenIs(token.LPAREN) {
		startToken := p.curToken
		p.nextToken() // consume '('

		// Check for empty tuple ()
		if p.curTokenIs(token.RPAREN) {
			return &ast.TupleType{Token: startToken, Types: []ast.Type{}}
		}

		// Parse first type
		t := p.parseType()

		// Check if tuple (comma-separated)
		if p.peekTokenIs(token.COMMA) {
			types := []ast.Type{t}
			for p.peekTokenIs(token.COMMA) {
				p.nextToken()
				p.nextToken()
				types = append(types, p.parseType())
			}
			if !p.expectPeek(token.RPAREN) {
				return nil
			}
			return &ast.TupleType{Token: startToken, Types: types}
		}

		// Check for partial type application: (Result String) or (Either Int)
		// Space-separated type args inside parens
		if p.peekTokenIs(token.IDENT_UPPER) || p.peekTokenIs(token.IDENT_LOWER) {
			// Collect type arguments
			if nt, ok := t.(*ast.NamedType); ok {
				for p.peekTokenIs(token.IDENT_UPPER) || p.peekTokenIs(token.IDENT_LOWER) {
					p.nextToken()
					arg := p.parseAtomicType()
					if arg != nil {
						nt.Args = append(nt.Args, arg)
					}
				}
			}
		}

		if !p.expectPeek(token.RPAREN) {
			return nil
		}
		return t // Grouped type or partial application
	}

	if p.curTokenIs(token.IDENT_UPPER) || p.curTokenIs(token.IDENT_LOWER) {
		nameVal := p.curToken.Literal.(string)
		startToken := p.curToken

		// Check for DOT (Qualified Type) - support multi-level qualification like kit.sql.Model
		for p.peekTokenIs(token.DOT) {
			p.nextToken() // consume ident
			p.nextToken() // consume dot

			if !p.curTokenIs(token.IDENT_UPPER) && !p.curTokenIs(token.IDENT_LOWER) {
				return nil // Error: expected identifier after dot
			}
			nameVal += "." + p.curToken.Literal.(string)
		}

		return &ast.NamedType{Token: startToken, Name: &ast.Identifier{Token: startToken, Value: nameVal}}
	}
	return nil
}

// parseTypeNoArrow parses a type but does not consume top-level arrows.
// This is used for parsing lambda parameters where -> is the delimiter.
func (p *Parser) parseTypeNoArrow() ast.Type {
	t := p.parseNonUnionTypeNoArrow()
	if t == nil {
		return nil
	}

	// Check for T?
	if p.peekTokenIs(token.QUESTION) {
		p.nextToken()
		nilType := &ast.NamedType{
			Token: p.curToken,
			Name:  &ast.Identifier{Token: p.curToken, Value: "Nil"},
		}
		return &ast.UnionType{
			Token: t.GetToken(),
			Types: []ast.Type{t, nilType},
		}
	}

	// Check for Union Type '|'
	if p.peekTokenIs(token.PIPE) {
		types := []ast.Type{t}
		for p.peekTokenIs(token.PIPE) {
			p.nextToken()
			p.nextToken()
			nextType := p.parseNonUnionTypeNoArrow()
			if nextType == nil {
				return nil
			}
			types = append(types, nextType)
		}
		return &ast.UnionType{
			Token: t.GetToken(),
			Types: types,
		}
	}
	return t
}

func (p *Parser) parseNonUnionTypeNoArrow() ast.Type {
	if p.curTokenIs(token.LBRACE) {
		return p.parseRecordType()
	}
	return p.parseTypeApplication()
}
