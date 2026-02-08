package parser

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/token"
)

func (p *Parser) parseFunctionStatement() *ast.FunctionStatement {
	stmt := p.parseFunctionSignature()
	if stmt == nil {
		return nil
	}

	// Check if Body starts
	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	stmt.Body = p.parseBlockStatement()

	return stmt
}

func (p *Parser) parseFunctionSignature() *ast.FunctionStatement {
	stmt := &ast.FunctionStatement{Token: p.curToken}

	// 1. Check for Early Generics <T> (e.g. for Extension Methods: fun<T> (recv) ...)
	if p.peekTokenIs(token.LT) {
		p.nextToken() // consume <
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
			} else if !p.curTokenIs(token.IDENT_LOWER) {
				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
					diagnostics.ErrP005, p.curToken,
					"expected identifier", p.curToken.Literal,
				))
				return nil
			}
			typeParam := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
			stmt.TypeParams = append(stmt.TypeParams, typeParam)

			p.nextToken() // move past IDENT

			if p.curTokenIs(token.COLON) {
				p.nextToken() // consume :

				// Check for Kind annotation: t: * -> *
				if p.curTokenIs(token.ASTERISK) || p.curTokenIs(token.LPAREN) {
					typeParam.Kind = p.parseKind()
					p.nextToken() // move past Kind

					if p.curTokenIs(token.PLUS) {
						p.nextToken() // consume + and expect traits
					} else {
						// Kind only, done with this param
						continue
					}
				}

				// Parse one or more trait constraints: t: Show, Cmp, Order
				for {
					if !p.curTokenIs(token.IDENT_UPPER) {
						p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
							diagnostics.ErrP005, p.curToken,
							"expected trait name (uppercase identifier)", p.curToken.Literal,
						))
						break
					}
					constraint := &ast.TypeConstraint{TypeVar: typeParam.Value, Trait: p.curToken.Literal.(string)}
					stmt.Constraints = append(stmt.Constraints, constraint)

					// Check for MPTC arguments: Convert<String>
					if p.peekTokenIs(token.LT) {
						p.nextToken() // consume trait name
						// Parse type args
						p.nextToken() // consume <
						for !p.curTokenIs(token.GT) && !p.curTokenIs(token.EOF) {
							if p.curTokenIs(token.COMMA) {
								p.nextToken()
								continue
							}

							// Handle RSHIFT (>>) splitting
							if p.curTokenIs(token.RSHIFT) {
								p.splitRshiftToken()
								break // Found GT
							}

							constraint.Args = append(constraint.Args, p.parseType())

							if p.peekTokenIs(token.COMMA) {
								p.nextToken()
							} else if p.peekTokenIs(token.RSHIFT) {
								// Next is >>. Consume and split.
								p.nextToken() // curToken is >>
								p.splitRshiftToken()
								break
							} else if !p.peekTokenIs(token.GT) {
								// error or done
							}
							p.nextToken()
						}
						// Now at GT
					}

					p.nextToken() // move past Trait (or GT)

					// Check if next is separator followed by another trait (uppercase)
					// Support both COMMA (legacy/ambiguous) and PLUS (preferred)
					isCommaConstraint := p.curTokenIs(token.COMMA) && p.peekTokenIs(token.IDENT_UPPER)
					isPlusConstraint := p.curTokenIs(token.PLUS)

					if isCommaConstraint || isPlusConstraint {
						p.nextToken() // consume separator
						continue      // parse next trait for same type param
					}
					break
				}
			}
		}
		// After loop, curToken is GT or COMMA (before next type param).
	}

	// 2. Check for Extension Method Receiver: fun (recv: Type) ...
	// If generics were parsed, curToken is GT. peekToken is LPAREN.
	// If not, curToken is FUN. peekToken is LPAREN.
	if p.peekTokenIs(token.LPAREN) {
		p.nextToken() // Advance to LPAREN (consumes FUN or GT)
		p.nextToken() // Advance to Identifier inside parens

		stmt.Receiver = p.parseParameter()
		if stmt.Receiver == nil {
			return nil
		}

		if !p.expectPeek(token.RPAREN) {
			return nil
		}

		// After receiver comes the Method Name
		if p.peekTokenIs(token.IDENT_LOWER) {
			p.nextToken()
			stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
		} else if p.peekTokenIs(token.IDENT_UPPER) {
			p.nextToken()
			p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
				diagnostics.ErrP006,
				p.curToken,
				"Extension method name must start with a lowercase letter",
			))
			stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
		} else {
			// Report standard error if neither
			if !p.expectPeek(token.IDENT_LOWER) {
				return nil
			}
		}
	} else {
		// Normal function
		if p.peekTokenIs(token.IDENT_LOWER) {
			p.nextToken()
			stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
		} else if p.peekTokenIs(token.IDENT_UPPER) {
			p.nextToken()
			p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
				diagnostics.ErrP006,
				p.curToken,
				"Function name must start with a lowercase letter",
			))
			stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
		} else {
			// Report standard error if neither
			if !p.expectPeek(token.IDENT_LOWER) {
				return nil
			}
		}
	}

	// 3. Late Generics <T> (Standard syntax: fun name<T>)
	if p.peekTokenIs(token.LT) {
		p.nextToken() // consume <
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
			} else if !p.curTokenIs(token.IDENT_LOWER) {
				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
					diagnostics.ErrP005, p.curToken,
					"expected identifier", p.curToken.Literal,
				))
				return nil
			}
			typeParam := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
			stmt.TypeParams = append(stmt.TypeParams, typeParam)

			p.nextToken() // move past IDENT

			if p.curTokenIs(token.COLON) {
				p.nextToken() // consume :

				// Check for Kind annotation: t: * -> *
				if p.curTokenIs(token.ASTERISK) || p.curTokenIs(token.LPAREN) {
					typeParam.Kind = p.parseKind()
					p.nextToken() // move past Kind

					if p.curTokenIs(token.PLUS) {
						p.nextToken() // consume + and expect traits
					} else {
						// Kind only, done with this param
						continue
					}
				}

				// Parse one or more trait constraints: t: Show, Convert<String>, Order
				for {
					if !p.curTokenIs(token.IDENT_UPPER) {
						p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
							diagnostics.ErrP005, p.curToken,
							"expected trait name (uppercase identifier)", p.curToken.Literal,
						))
						break
					}
					traitName := p.curToken.Literal.(string)

					// Check for MPTC arguments: Convert<String>
					var args []ast.Type
					if p.peekTokenIs(token.LT) {
						p.nextToken() // consume trait name
						// Parse type args
						p.nextToken() // consume <
						for !p.curTokenIs(token.GT) && !p.curTokenIs(token.EOF) {
							if p.curTokenIs(token.COMMA) {
								p.nextToken()
								continue
							}

							// Handle RSHIFT (>>) splitting
							if p.curTokenIs(token.RSHIFT) {
								// We hit >>. Treat first > as closing this list.
								// Split >> into > >
								p.splitRshiftToken()
								break // Found GT
							}

							args = append(args, p.parseType())

							if p.peekTokenIs(token.COMMA) {
								p.nextToken()
							} else if p.peekTokenIs(token.RSHIFT) {
								// Next is >>. Consume and split.
								p.nextToken() // curToken is >>
								p.splitRshiftToken()
								break
							} else if !p.peekTokenIs(token.GT) {
								// error or done
							}
							p.nextToken()
						}
						// Now at GT
					}

					constraint := &ast.TypeConstraint{
						TypeVar: typeParam.Value,
						Trait:   traitName,
						Args:    args,
					}
					stmt.Constraints = append(stmt.Constraints, constraint)
					p.nextToken() // move past Trait (or GT)

					// Check if next is separator followed by another trait (uppercase)
					// Support both COMMA (legacy/ambiguous) and PLUS (preferred)
					isCommaConstraint := p.curTokenIs(token.COMMA) && p.peekTokenIs(token.IDENT_UPPER)
					isPlusConstraint := p.curTokenIs(token.PLUS)

					if isCommaConstraint || isPlusConstraint {
						p.nextToken() // consume separator
						continue      // parse next trait for same type param
					}
					break
				}
			}
		}
	}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	stmt.Parameters = p.parseFunctionParameters()

	// Skip newlines before return type (allow multiline function signatures)
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	// Return type is optional
	// Support '->' prefix for return type
	if p.peekTokenIs(token.ARROW) {
		p.nextToken() // consume '->'
		p.nextToken() // point to start of type
		stmt.ReturnType = p.parseType()
	} else if !p.peekTokenIs(token.LBRACE) && !p.peekTokenIs(token.NEWLINE) && !p.peekTokenIs(token.RBRACE) {
		// Legacy support: return type without '->' prefix
		p.nextToken()
		stmt.ReturnType = p.parseType()
	}

	return stmt
}

func (p *Parser) parseFunctionParameters() []*ast.Parameter {
	params := []*ast.Parameter{}

	// p.curToken is LPAREN
	// Skip newlines after (
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return params
	}

	p.nextToken()
	// Skip newlines before first param
	for p.curTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	for {
		param := p.parseParameter()
		if param != nil {
			params = append(params, param)
		}

		// Skip newlines after param
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // consume comma
			// Skip newlines after comma
			for p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
			// Handle trailing comma
			if p.peekTokenIs(token.RPAREN) {
				break
			}
			p.nextToken()
			// Skip newlines before next param
			for p.curTokenIs(token.NEWLINE) {
				p.nextToken()
			}
		} else {
			break
		}
	}

	// Skip newlines before )
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return params
}

func (p *Parser) parseParameter() *ast.Parameter {
	return p.parseParameterCommon(true)
}

func (p *Parser) parseParameterCommon(allowArrowInType bool) *ast.Parameter {
	param := &ast.Parameter{Token: p.curToken}

	// Check for prefix variadic ...identifier
	if p.curTokenIs(token.ELLIPSIS) {
		param.IsVariadic = true
		p.nextToken()
		// Token update skipped intentionally
	}

	// Allow underscore as "ignored" parameter
	if p.curTokenIs(token.UNDERSCORE) {
		param.Name = &ast.Identifier{Token: p.curToken, Value: "_"}
		param.IsIgnored = true
	} else if p.curTokenIs(token.IDENT_LOWER) {
		param.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
	} else if p.curTokenIs(token.IDENT_UPPER) {
		p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
			diagnostics.ErrP006,
			p.curToken,
			"Parameter name must start with a lowercase letter",
		))
		param.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
	} else {
		// Destructuring patterns not yet supported in parameters
		return nil
	}

	// Check for type annotation
	if p.peekTokenIs(token.COLON) {
		p.nextToken()
		p.nextToken()

		// Check for prefix variadic: args: ...Type
		if p.curTokenIs(token.ELLIPSIS) {
			param.IsVariadic = true
			p.nextToken()
		}

		if allowArrowInType {
			param.Type = p.parseType()
		} else {
			param.Type = p.parseTypeNoArrow()
		}
	}

	// Check for default value (e.g., x = 10)
	if p.peekTokenIs(token.ASSIGN) {
		p.nextToken() // consume =
		p.nextToken() // move to expression
		param.Default = p.parseExpression(LOWEST)
	}

	return param
}
