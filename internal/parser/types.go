package parser

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/token"
)

func (p *Parser) parseType() ast.Type {
	// Check for 'forall' type
	if p.curTokenIs(token.FORALL) {
		return p.parseForallType()
	}

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

func (p *Parser) parseForallType() ast.Type {
	tok := p.curToken // 'forall'
	p.nextToken()     // consume 'forall'

	var typeParams []*ast.Identifier

	// Parse type variables with optional constraints: t: Numeric, u, v: Show
	for p.curTokenIs(token.IDENT_UPPER) || p.curTokenIs(token.IDENT_LOWER) {
		if p.curTokenIs(token.IDENT_UPPER) {
			p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
				diagnostics.ErrP006,
				p.curToken,
				fmt.Sprintf("Type variables must start with a lowercase letter (got '%s')",
					p.curToken.Literal),
			))
		}

		// Create identifier
		ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
		typeParams = append(typeParams, ident)
		p.nextToken()

		// Check for constraints: t: Numeric, Show
		if p.curTokenIs(token.COLON) {
			p.nextToken() // consume ':'
			// Parse constraint list: Trait1, Trait2, ...
			for p.curTokenIs(token.IDENT_UPPER) {
				constraint := &ast.TypeConstraint{
					TypeVar: ident.Value,
					Trait:   p.curToken.Literal.(string),
				}
				ident.Constraints = append(ident.Constraints, constraint)
				p.nextToken()

				// Check for comma (more constraints for this param)
				if p.curTokenIs(token.COMMA) {
					p.nextToken() // consume comma
					// Continue to parse next constraint for this param
				} else {
					// No comma, end of constraints for this param
					break
				}
			}
		}

		// Check for comma (more variables) or dot (end of variables)
		if p.curTokenIs(token.COMMA) {
			p.nextToken() // consume comma, continue parsing
		} else if p.curTokenIs(token.DOT) {
			break // end of variables
		} else {
			// Unexpected token
			p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
				diagnostics.ErrP005, p.curToken,
				"expected ',' or '.' after type variable in forall", p.curToken.Literal,
			))
			return nil
		}
	}

	if !p.curTokenIs(token.DOT) {
		return nil
	}
	p.nextToken() // consume '.'

	body := p.parseType()
	if body == nil {
		return nil
	}

	return &ast.ForallType{
		Token: tok,
		Vars:  typeParams,
		Type:  body,
	}
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

		for {
			if p.curTokenIs(token.EOF) {
				return nil
			}

			if p.curTokenIs(token.COMMA) {
				p.nextToken()
				continue
			}

			// If we're at a >, check if it closes this generic or an inner one
			if p.curTokenIs(token.GT) {
				// Check if there's another > coming (closing the outer generic)
				if p.peekTokenIs(token.GT) {
					// Consume the second > which closes this generic
					p.nextToken()
					break
				} else if p.peekTokenIs(token.RSHIFT) {
					// We see >> where we expect >.
					// The scanner produces RSHIFT (>>) for two adjacent > characters.
					// In nested generics (e.g. List<List<T>>), this is ambiguous.
					// We must manually split the RSHIFT token into two GT tokens to correctly parse the closing brackets.
					p.nextToken() // move curToken to >>
					// Set flag so next nextToken returns synthetic >
					p.splitRshift = true
					// Now call nextToken to get the synthetic >
					p.nextToken() // curToken becomes synthetic >
					break
				} else if p.peekTokenIs(token.COMMA) {
					// This > closed an inner generic, more args coming
					p.nextToken() // move to comma
					p.nextToken() // move to next type arg
					continue
				} else {
					// This > closed an inner generic, but unexpected token follows
					return nil
				}
			}

			arg := p.parseType()
			if arg == nil {
				return nil
			}

			if nt, ok := base.(*ast.NamedType); ok {
				nt.Args = append(nt.Args, arg)
			}

			// Check for Type Constraints: List<t: Show>
			if p.peekTokenIs(token.COLON) {
				p.nextToken() // move to COLON
				// Ensure arg is a NamedType (type variable)
				if nt, ok := arg.(*ast.NamedType); ok {
					for {
						p.nextToken() // move to Trait Name
						if !p.curTokenIs(token.IDENT_UPPER) {
							p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
								diagnostics.ErrP005, p.curToken,
								"expected trait name (uppercase identifier)", p.curToken.Literal,
							))
							break
						}
						constraint := &ast.TypeConstraint{
							TypeVar: nt.Name.Value,
							Trait:   p.curToken.Literal.(string),
						}
						nt.Name.Constraints = append(nt.Name.Constraints, constraint)

						// Check for more constraints (comma separated)
						// Heuristic: If comma followed by UPPER identifier, it's another constraint.
						// Otherwise, it's the start of the next type argument.
						if p.peekTokenIs(token.COMMA) {
							p.nextToken() // Consume comma
							if p.peekTokenIs(token.IDENT_UPPER) {
								continue
							}
							// Not a constraint (next arg is likely lower case type var or concrete type)
							// We consumed the comma, so we break and let outer loop handle curToken==COMMA
							break
						}
						break
					}
				} else {
					// Error: constraint on non-variable type
					p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
						diagnostics.ErrP006, p.curToken,
						"Type constraints only allowed on type variables", p.curToken.Literal,
					))
				}
			}

			// After parseType(), check what's next

			// Handle case where we consumed the comma in constraint check
			if p.curTokenIs(token.COMMA) {
				p.nextToken() // move to next type arg
				continue
			}

			// If parseType() parsed a nested generic type, curToken might be at the >
			// that closed the inner generic. Check peekToken to see what's next.
			if p.curTokenIs(token.GT) {
				// parseType() left us at a > that closed an inner generic
				// Check peekToken to see if we're done or need to continue
				if p.peekTokenIs(token.GT) || p.splitRshift {
					// Another > closes the outer generic
					p.nextToken()
					break
				} else if p.peekTokenIs(token.RSHIFT) {
					// We see >> where we expect >
					p.nextToken() // move curToken to >>
					p.splitRshift = true
					p.nextToken() // curToken becomes synthetic >
					break
				} else if p.peekTokenIs(token.COMMA) {
					// More args coming
					p.nextToken() // move to comma
					p.nextToken() // move to next type arg
					continue
				} else {
					// Unexpected token
					return nil
				}
			}

			// Normal case - check peek for what's next
			if p.peekTokenIs(token.COMMA) {
				p.nextToken() // move curToken to comma
				p.nextToken() // move to next type arg
			} else if p.peekTokenIs(token.GT) {
				p.nextToken() // move curToken to GT
				break
			} else if p.peekTokenIs(token.RSHIFT) {
				// We see >> where we expect >
				// Consume it and split: first > closes this generic
				p.nextToken() // move curToken to >>
				// Set flag so next nextToken returns synthetic >
				p.splitRshift = true
				// Manually convert current token to > to close this generic
				p.curToken.Type = token.GT
				p.curToken.Literal = ">"
				p.curToken.Lexeme = ">"
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
