package parser

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/token"
	"github.com/funvibe/funxy/internal/typesystem"
)

func (p *Parser) parseTypeDeclarationStatement() *ast.TypeDeclarationStatement {
	stmt := &ast.TypeDeclarationStatement{Token: p.curToken}

	// 1. Parse Type Name (Constructor) or 'alias'
	if p.peekTokenIs(token.ALIAS) {
		p.nextToken()
		stmt.IsAlias = true
	}

	if p.peekTokenIs(token.IDENT_LOWER) {
		p.nextToken()
		p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
			diagnostics.ErrP006,
			p.curToken,
			"Type name must start with an uppercase letter",
		))
		stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
	} else if !p.expectPeek(token.IDENT_UPPER) {
		return nil
	} else {
		stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
	}

	// 2. Parse Type Parameters <T, U>
	if p.peekTokenIs(token.LT) {
		p.nextToken() // consume <. curToken is <.
		p.nextToken() // move to first type param

		for !p.curTokenIs(token.GT) && !p.curTokenIs(token.EOF) {
			if p.curTokenIs(token.COMMA) {
				p.nextToken()
				continue
			}

			if p.curTokenIs(token.IDENT_UPPER) {
				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
					diagnostics.ErrP006, p.curToken,
					fmt.Sprintf("Type variables must start with a lowercase letter (got '%s')", p.curToken.Literal),
				))
				// recover by treating it as if it were valid for parsing purposes
			} else if !p.curTokenIs(token.IDENT_LOWER) {
				// Type parameter must start with lowercase
				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
					diagnostics.ErrP005, p.curToken,
					"expected identifier", p.curToken.Literal,
				))
				return nil
			}
			tp := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
			stmt.TypeParameters = append(stmt.TypeParameters, tp)
			p.nextToken() // move past IDENT

			// Check for constraints: t: Kind + Trait...
			if p.curTokenIs(token.COLON) {
				p.nextToken() // consume :

				// Try to parse Kind first if it starts with * or (
				var kind typesystem.Kind
				if p.curTokenIs(token.ASTERISK) || p.curTokenIs(token.LPAREN) {
					kind = p.parseKind()
					tp.Kind = kind
					p.nextToken() // consume last token of kind
				}

				// If we parsed a kind, check if there are traits following (separated by +)
				// If we didn't parse a kind, we expect traits immediately (Identifier)
				for {
					// Check for + separator if we already parsed something
					if kind != nil || len(tp.Constraints) > 0 {
						if p.curTokenIs(token.PLUS) {
							p.nextToken() // consume +
						} else {
							break // No more constraints/kind parts
						}
					}

					// Expect Trait Name (Identifier)
					if p.curTokenIs(token.IDENT_UPPER) {
						traitName := p.curToken.Literal.(string)
						constraint := &ast.TypeConstraint{
							TypeVar: tp.Value,
							Trait:   traitName,
						}
						// Check for MPTC args <...>
						if p.peekTokenIs(token.LT) {
							// ... parse MPTC args logic ...
							// Reusing logic from statements_functions.go or extracting it would be better
							// For brevity, simple parsing here:
							p.nextToken() // trait name
							p.nextToken() // <
							for !p.curTokenIs(token.GT) && !p.curTokenIs(token.EOF) {
								if p.curTokenIs(token.COMMA) {
									p.nextToken()
									continue
								}
								// Parsing logic... omit for now or duplicate
								// Just minimal to skip
								arg := p.parseType()
								constraint.Args = append(constraint.Args, arg)
								if p.peekTokenIs(token.COMMA) {
									p.nextToken()
								}
								p.nextToken()
							}
							// At GT
						}
						tp.Constraints = append(tp.Constraints, constraint)
						p.nextToken() // consume Trait Name (or GT)

						// Check if next is separator followed by another trait (uppercase)
						isCommaConstraint := p.curTokenIs(token.COMMA) && p.peekTokenIs(token.IDENT_UPPER)
						isPlusConstraint := p.curTokenIs(token.PLUS)

						if isCommaConstraint || isPlusConstraint {
							p.nextToken() // consume separator
							continue      // parse next trait for same type param
						}
						break
					} else {
						// If we expected a trait but found something else
						if kind == nil && len(tp.Constraints) == 0 {
							// If we haven't parsed ANYTHING yet, and it's not a trait, error
							p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
								diagnostics.ErrP005, p.curToken,
								"expected kind or trait constraint", p.curToken.Literal,
							))
							break // Recover
						}
						// Otherwise, maybe we are done (e.g. at comma or GT)
						break
					}
				}
			}
		}

		if !p.curTokenIs(token.GT) {
			return nil
		}
	}

	// 3. Expect '=' (allow newlines before it for multiline type declarations)
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}
	if !p.expectPeek(token.ASSIGN) {
		return nil
	}
	// Skip newlines after '='
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}
	p.nextToken() // Move to RHS

	// 4. Parse Right Hand Side
	if stmt.IsAlias {
		// Explicit alias: type alias X = SomeType
		stmt.TargetType = p.parseType()
	} else {
		// ADT: Constructor | Constructor ...
		// Loop separated by PIPE (allow newlines before |)
		for {
			// Skip newlines before checking for |
			for p.curTokenIs(token.NEWLINE) {
				p.nextToken()
			}

			// Handle leading pipe (e.g. type T = | A | B)
			// If first constructor hasn't been parsed yet, and we see a pipe, consume it.
			if len(stmt.Constructors) == 0 && p.curTokenIs(token.PIPE) {
				p.nextToken() // consume leading pipe
				// Skip newlines after leading pipe
				for p.curTokenIs(token.NEWLINE) {
					p.nextToken()
				}
			}

			constructor := p.parseDataConstructor()
			if constructor != nil {
				stmt.Constructors = append(stmt.Constructors, constructor)
			}

			// Skip newlines before checking for |
			for p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}

			if p.peekTokenIs(token.PIPE) {
				p.nextToken() // consume current token
				p.nextToken() // consume |
				// Skip newlines after |
				for p.curTokenIs(token.NEWLINE) {
					p.nextToken()
				}
			} else {
				break
			}
		}
	}

	return stmt
}

func (p *Parser) parseDataConstructor() *ast.DataConstructor {
	dc := &ast.DataConstructor{Token: p.curToken}
	if p.curTokenIs(token.IDENT_LOWER) {
		p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
			diagnostics.ErrP006,
			p.curToken,
			"Constructor name must start with an uppercase letter",
		))
		// Continue parsing to allow recovery
	} else if !p.curTokenIs(token.IDENT_UPPER) {
		return nil
	}
	dc.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}

	// Check for C-style syntax: Constructor(Type, Type, ...) or Constructor(Type)
	// Rule: If there's a comma at the TOP LEVEL inside parens, it's C-style (multiple args).
	//       If there's only one type with no comma, it's C-style with single arg.
	//       For a tuple argument, use double parens: Constructor((A, B))
	// ML-style: Constructor Type Type (space-separated, no parens around args)
	if p.peekTokenIs(token.LPAREN) {
		p.nextToken() // now at '('
		p.nextToken() // move into the parens

		if p.curTokenIs(token.RPAREN) {
			// Empty parens: Constructor() - zero args
			return dc
		}

		// Parse first type
		firstType := p.parseNonUnionType()
		dc.Parameters = append(dc.Parameters, firstType)

		// Check for comma - if present, continue parsing C-style args
		for p.peekTokenIs(token.COMMA) {
			p.nextToken() // move to comma
			p.nextToken() // move to next type
			t := p.parseNonUnionType()
			if t != nil {
				dc.Parameters = append(dc.Parameters, t)
			}
		}

		if !p.expectPeek(token.RPAREN) {
			return nil
		}
		return dc
	}

	// ML-style syntax: Constructor Type Type ...
	// Parse parameters (Types) until next PIPE or NEWLINE/EOF
	for !p.peekTokenIs(token.PIPE) && !p.peekTokenIs(token.NEWLINE) && !p.peekTokenIs(token.EOF) {
		p.nextToken()
		// Use parseNonUnionType to avoid consuming | as part of union type
		// ADT syntax: Constructor Type Type | Constructor Type
		// The | here separates constructors, not union type members
		t := p.parseNonUnionType()
		if t == nil {
			break
		}
		dc.Parameters = append(dc.Parameters, t)
	}
	return dc
}
