package parser

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/token"
	"github.com/funvibe/funxy/internal/typesystem"
)

func (p *Parser) parseTraitDeclaration() *ast.TraitDeclaration {
	stmt := &ast.TraitDeclaration{Token: p.curToken}

	// trait Show<T> { ... }
	// trait Order<T> : Equal<T> { ... }
	if p.curTokenIs(token.IDENT_LOWER) {
		p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
			diagnostics.ErrP006,
			p.curToken,
			"Trait name must start with an uppercase letter",
		))
		stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
		// Important: Consume the invalid token so we can proceed to parse body
		// But don't call nextToken() twice if we were already on the token
	} else if p.peekTokenIs(token.IDENT_UPPER) {
		p.nextToken()
		stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
	} else if p.peekTokenIs(token.IDENT_LOWER) {
		p.nextToken()
		p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
			diagnostics.ErrP006,
			p.curToken,
			"Trait name must start with an uppercase letter",
		))
		stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
	} else {
		if !p.expectPeek(token.IDENT_UPPER) {
			return nil
		}
	}

	// Parse generic type parameters <T>
	stmt.TypeParams = []*ast.Identifier{}
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
				// Continue parsing to allow error recovery
			} else if !p.curTokenIs(token.IDENT_LOWER) {
				// Type parameter must be an identifier
				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
					diagnostics.ErrP005, p.curToken,
					"expected identifier", p.curToken.Literal,
				))
				return nil
			}
			tp := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
			stmt.TypeParams = append(stmt.TypeParams, tp)
			p.nextToken() // move past IDENT

			if p.curTokenIs(token.COLON) {
				p.nextToken() // consume :

				// Try parse Kind
				var kind typesystem.Kind
				if p.curTokenIs(token.ASTERISK) || p.curTokenIs(token.LPAREN) {
					kind = p.parseKind()
					tp.Kind = kind
					p.nextToken()

					if p.curTokenIs(token.PLUS) {
						p.nextToken()
					} else {
						// No traits following kind, continue to next type param (or end)
						continue
					}
				}

				// Traits
				for {
					if p.curTokenIs(token.IDENT_UPPER) {
						traitName := p.curToken.Literal.(string)
						constraint := &ast.TypeConstraint{TypeVar: tp.Value, Trait: traitName}

						// MPTC
						if p.peekTokenIs(token.LT) {
							p.nextToken()
							p.nextToken()
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

								arg := p.parseType()
								constraint.Args = append(constraint.Args, arg)

								if p.peekTokenIs(token.COMMA) {
									p.nextToken()
								} else if p.peekTokenIs(token.RSHIFT) {
									// Next is >>. Consume and split.
									p.nextToken() // curToken is >>
									p.splitRshiftToken()
									break
								}
								p.nextToken()
							}
						}
						stmt.Constraints = append(stmt.Constraints, constraint)
						tp.Constraints = append(tp.Constraints, constraint)
						p.nextToken()

						// Check if next is separator followed by another trait (uppercase)
						// Support both COMMA (legacy/ambiguous) and PLUS (preferred)
						isCommaConstraint := p.curTokenIs(token.COMMA) && p.peekTokenIs(token.IDENT_UPPER)
						isPlusConstraint := p.curTokenIs(token.PLUS)

						if isCommaConstraint || isPlusConstraint {
							p.nextToken() // consume separator
							continue      // parse next trait for same type param
						}
						break
					} else {
						// If we are here, we expected a trait (e.g. after + or first in list if no kind)
						// But strictly speaking, "t:" without constraints is not valid?
						// Or "t: Kind + <not trait>" is invalid.
						if kind != nil || len(tp.Constraints) > 0 {
							p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
								diagnostics.ErrP005, p.curToken,
								"expected trait name", p.curToken.Literal,
							))
						}
						break
					}
				}
			}
		}

		if !p.curTokenIs(token.GT) {
			return nil
		}
	}

	// Parse super traits: trait Order<T> : Equal<T>, Show<T> { ... }
	stmt.SuperTraits = []ast.Type{}
	if p.peekTokenIs(token.COLON) {
		p.nextToken() // consume current (GT or Name)
		p.nextToken() // consume COLON, move to first super trait

		for {
			superTrait := p.parseType()
			if superTrait != nil {
				stmt.SuperTraits = append(stmt.SuperTraits, superTrait)
			}

			if p.peekTokenIs(token.COMMA) {
				p.nextToken() // consume type
				p.nextToken() // consume comma
			} else {
				break
			}
		}
	}

	// Parse functional dependencies: | a, b -> c
	// Supports multiple dependencies: | a -> b | c -> d
	stmt.Dependencies = []ast.FunctionalDependency{}
	if p.peekTokenIs(token.PIPE) {
		p.nextToken() // consume current token (end of previous part)
		p.nextToken() // consume PIPE

		for {
			dep := ast.FunctionalDependency{
				From: []string{},
				To:   []string{},
			}

			// Parse LHS: a, b
			for {
				if !p.curTokenIs(token.IDENT_LOWER) {
					p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
						diagnostics.ErrP005, p.curToken,
						"expected type variable (lowercase identifier)", p.curToken.Literal,
					))
					return nil
				}
				dep.From = append(dep.From, p.curToken.Literal.(string))

				if p.peekTokenIs(token.COMMA) {
					p.nextToken() // consume ident
					p.nextToken() // consume comma
					continue
				}
				break
			}

			if !p.expectPeek(token.ARROW) {
				return nil
			}
			p.nextToken() // consume ->

			// Parse RHS: c, d
			for {
				if !p.curTokenIs(token.IDENT_LOWER) {
					p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
						diagnostics.ErrP005, p.curToken,
						"expected type variable (lowercase identifier)", p.curToken.Literal,
					))
					return nil
				}
				dep.To = append(dep.To, p.curToken.Literal.(string))

				if p.peekTokenIs(token.COMMA) {
					// Ambiguity: Comma could separate variables in RHS (a -> b, c)
					// OR separate dependencies (a -> b, c -> d)

					// Look ahead to disambiguate
					tokens := p.stream.Peek(20)
					isDepSep := false
					idx := 1 // Skip the comma itself (Peek[0])

					for idx < len(tokens) {
						t := tokens[idx]
						if t.Type == token.IDENT_LOWER {
							idx++
							// Optional comma between LHS vars of next dep
							if idx < len(tokens) && tokens[idx].Type == token.COMMA {
								idx++
							}
							continue
						} else if t.Type == token.ARROW {
							isDepSep = true
							break
						} else {
							// e.g. LBRACE, NEWLINE -> End of dependencies block
							// Or IDENT_UPPER -> Invalid for FunDep variable
							break
						}
					}

					if isDepSep {
						// Comma separates dependencies. Break inner loop.
						break
					} else {
						// Comma separates variables. Consume and continue inner loop.
						p.nextToken() // consume ident
						p.nextToken() // consume comma
						continue
					}
				}
				break
			}

			stmt.Dependencies = append(stmt.Dependencies, dep)

			if p.peekTokenIs(token.COMMA) {
				p.nextToken() // consume ident
				p.nextToken() // consume COMMA
				continue
			}
			break
		}
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	// Parse function signatures inside block
	p.nextToken() // enter block

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.NEWLINE) {
			p.nextToken()
			continue
		}

		if p.curTokenIs(token.FUN) {
			fn := p.parseFunctionSignature()
			if fn != nil {
				// Optional body for default implementation
				if p.peekTokenIs(token.LBRACE) {
					p.nextToken()
					fn.Body = p.parseBlockStatement()
				}
				stmt.Signatures = append(stmt.Signatures, fn)
			}
			if p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
			p.nextToken()
		} else if p.curTokenIs(token.OPERATOR) {
			// Parse operator implementation: operator (+)(a: Int, b: Int) -> Int { a + b }
			// For traits, body is optional
			fn := p.parseOperatorMethod()
			if fn != nil {
				stmt.Signatures = append(stmt.Signatures, fn)
			}
			if p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
			p.nextToken()
		} else {
			p.nextToken()
		}
	}

	return stmt
}

// parseOperatorMethod parses operator (+)<A, B>(a: T, b: T) -> T { body }
// Supports optional generic type params: operator (<~>)<A, B>(...)
func (p *Parser) parseOperatorMethod() *ast.FunctionStatement {
	fn := &ast.FunctionStatement{Token: p.curToken}

	// Expect ( after operator
	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	// Get operator symbol
	p.nextToken()
	op := p.curToken.Lexeme
	fn.Operator = op
	fn.Name = &ast.Identifier{Token: p.curToken, Value: "(" + op + ")"}

	// Expect closing )
	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	// Optional: generic type parameters <A, B>
	if p.peekTokenIs(token.LT) {
		p.nextToken() // move to current position
		p.nextToken() // consume <
		for !p.curTokenIs(token.GT) && !p.curTokenIs(token.EOF) {
			if p.curTokenIs(token.COMMA) {
				p.nextToken()
				continue
			}
			if p.curTokenIs(token.IDENT_UPPER) || p.curTokenIs(token.IDENT_LOWER) {
				fn.TypeParams = append(fn.TypeParams, &ast.Identifier{
					Token: p.curToken,
					Value: p.curToken.Literal.(string),
				})
			}
			p.nextToken()
		}
		// curToken is now GT
	}

	// Expect ( for parameters
	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	fn.Parameters = p.parseFunctionParameters()

	// Return type
	if p.peekTokenIs(token.ARROW) {
		p.nextToken()
		p.nextToken()
		fn.ReturnType = p.parseType()
	}

	// Body is optional for trait signatures, but required for instances.
	// We parse it if present.
	if p.peekTokenIs(token.LBRACE) {
		p.nextToken()
		fn.Body = p.parseBlockStatement()
	}

	return fn
}

func (p *Parser) parseInstanceDeclaration() *ast.InstanceDeclaration {
	stmt := &ast.InstanceDeclaration{Token: p.curToken}

	// instance Show Int { ... }
	// instance sql.Model User { ... } -- Qualified trait name
	// instance kit.sql.Model User { ... } -- Multi-level qualified trait name
	// instance Functor<Option> { ... }  -- HKT style

	// Check if we have a qualified name (module.submodule...Trait)
	if p.peekTokenIs(token.IDENT_LOWER) {
		// Could be qualified name syntax: module.Trait or module.submodule.Trait
		p.nextToken() // consume and save first identifier
		qualifiedPath := p.curToken.Literal.(string)
		startToken := p.curToken

		// Collect all parts of the qualified path (support multi-level: kit.sql.Model)
		for p.peekTokenIs(token.DOT) {
			p.nextToken() // consume dot

			// Check what comes after the dot
			if p.peekTokenIs(token.IDENT_LOWER) {
				// Another module segment: kit.sql
				p.nextToken()
				qualifiedPath += "." + p.curToken.Literal.(string)
			} else if p.peekTokenIs(token.IDENT_UPPER) {
				// Final trait name: Model
				p.nextToken()
				traitName := p.curToken.Literal.(string)

				// Split qualified path: "kit.sql" -> ModuleName, "Model" -> TraitName
				stmt.ModuleName = &ast.Identifier{Token: startToken, Value: qualifiedPath}
				stmt.TraitName = &ast.Identifier{Token: p.curToken, Value: traitName}
				break
			} else {
				// Error: expected identifier after dot
				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
					diagnostics.ErrP005,
					p.peekToken,
					"expected identifier after '.' in qualified trait name",
				))
				return nil
			}
		}

		// If we exited the loop without finding an uppercase trait name, it's an error
		if stmt.TraitName == nil {
			p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
				diagnostics.ErrP006,
				p.curToken,
				"expected trait name (uppercase identifier) in qualified name",
			))
			return nil
		}
	} else if !p.expectPeek(token.IDENT_UPPER) {
		// Simple case: instance Trait
		return nil
	} else {
		stmt.TraitName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
	}

	// Check for HKT syntax / MPTC: instance Trait<TypeConstructor> { ... }
	// Or: instance Trait<TypeConstructor, E> { ... }
	// Or: instance Convert<Int, String> { ... }
	if p.peekTokenIs(token.LT) {
		p.nextToken() // consume trait name
		p.nextToken() // consume <

		// Parse all comma-separated types as arguments
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

			argType := p.parseType()
			if argType != nil {
				stmt.Args = append(stmt.Args, argType)
			}

			if p.peekTokenIs(token.COMMA) {
				p.nextToken()
			} else if p.peekTokenIs(token.RSHIFT) {
				// Next is >>. Consume and split.
				p.nextToken() // curToken is >>
				p.splitRshiftToken()
				break
			}
			p.nextToken()
		}

		if len(stmt.Args) > 0 {
			// stmt.TypeParams are NOT populated here. Analyzer should handle implicit generics in Args.
		}

		// Ensure we consumed the GT
		// parseType usually advances past the type.
		// If we are at GT, loop terminates.
		// curToken should be GT (or EOF)
		if !p.curTokenIs(token.GT) {
			p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
				diagnostics.ErrP005, p.curToken,
				"expected '>' to close instance type arguments", p.curToken.Literal,
			))
			return nil
		}
	} else {
		p.nextToken()
		t := p.parseType()
		if t != nil {
			stmt.Args = []ast.Type{t}
		}
	}
	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	// Parse method implementations
	p.nextToken()

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.NEWLINE) {
			p.nextToken()
			continue
		}

		if p.curTokenIs(token.FUN) {
			fn := p.parseFunctionStatement()
			stmt.Methods = append(stmt.Methods, fn)
			if p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
			p.nextToken()
		} else if p.curTokenIs(token.OPERATOR) {
			// Parse operator implementation: operator (+)(a: Int, b: Int) -> Int { a + b }
			fn := p.parseOperatorMethod()
			if fn != nil {
				stmt.Methods = append(stmt.Methods, fn)
			}
			if p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
			p.nextToken()
		} else {
			p.nextToken()
		}
	}

	return stmt
}
