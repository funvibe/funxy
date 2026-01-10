package parser

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/token"
)

func (p *Parser) parsePackageDeclaration() *ast.PackageDeclaration {
	// package my_pkg
	// package my_pkg (*)
	// package my_pkg (A, B)
	// package my_pkg (*, shapes(*))
	// package my_pkg (localFun, shapes(Circle, Square))
	pd := &ast.PackageDeclaration{Token: p.curToken}

	if !p.expectPeek(token.IDENT_LOWER) {
		return nil
	}
	pd.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}

	// Check for export list
	if p.peekTokenIs(token.LPAREN) {
		p.nextToken() // consume (

		pd.Exports = []*ast.ExportSpec{}

		// Parse export specs until )
		for !p.peekTokenIs(token.RPAREN) {
			spec := p.parseExportSpec()
			if spec == nil {
				return nil
			}

			// Check if it's a local wildcard export
			if spec.Symbol != nil && spec.Symbol.Value == "*" {
				pd.ExportAll = true
			} else {
				pd.Exports = append(pd.Exports, spec)
			}

			// Check for comma or end
			if p.peekTokenIs(token.COMMA) {
				p.nextToken() // consume comma
			} else if !p.peekTokenIs(token.RPAREN) {
				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
					diagnostics.ErrP006,
					p.peekToken,
					"expected ',' or ')' in export list",
				))
				return nil
			}
		}

		if !p.expectPeek(token.RPAREN) {
			return nil
		}
	}

	return pd
}

// parseExportSpec parses a single export specification:
// - * (local wildcard)
// - ident (local symbol)
// - ident(*) (re-export all from module)
// - ident(A, B) (re-export specific symbols from module)
func (p *Parser) parseExportSpec() *ast.ExportSpec {
	spec := &ast.ExportSpec{Token: p.peekToken}

	// Check for * (local wildcard)
	if p.peekTokenIs(token.ASTERISK) {
		p.nextToken() // consume *
		spec.Symbol = &ast.Identifier{Token: p.curToken, Value: "*"}
		return spec
	}

	// Expect identifier
	if !p.peekTokenIs(token.IDENT_UPPER) && !p.peekTokenIs(token.IDENT_LOWER) {
		p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
			diagnostics.ErrP006,
			p.peekToken,
			"expected identifier or '*' in export list",
		))
		return nil
	}

	p.nextToken() // consume identifier
	ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}

	// Check if followed by ( — this means it's a module re-export
	if p.peekTokenIs(token.LPAREN) {
		p.nextToken() // consume (
		spec.ModuleName = ident

		// Check for * (re-export all from module)
		if p.peekTokenIs(token.ASTERISK) {
			p.nextToken() // consume *
			spec.ReexportAll = true
		} else {
			// Parse list of symbols to re-export
			spec.Symbols = []*ast.Identifier{}
			for !p.peekTokenIs(token.RPAREN) {
				if !p.peekTokenIs(token.IDENT_UPPER) && !p.peekTokenIs(token.IDENT_LOWER) {
					p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
						diagnostics.ErrP006,
						p.peekToken,
						"expected identifier in re-export list",
					))
					return nil
				}
				p.nextToken()
				spec.Symbols = append(spec.Symbols, &ast.Identifier{
					Token: p.curToken,
					Value: p.curToken.Literal.(string),
				})

				if p.peekTokenIs(token.COMMA) {
					p.nextToken() // consume comma
				} else if !p.peekTokenIs(token.RPAREN) {
					p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
						diagnostics.ErrP006,
						p.peekToken,
						"expected ',' or ')' in re-export list",
					))
					return nil
				}
			}
		}

		if !p.expectPeek(token.RPAREN) {
			return nil
		}
	} else {
		// Simple local symbol export
		spec.Symbol = ident
	}

	return spec
}

func (p *Parser) parseImportStatement() *ast.ImportStatement {
	// import "path/to/module" [as alias]
	// import "path" (a, b, c)      -- import only these
	// import "path" !(a, b, c)     -- import all except these
	// import "path" (*)            -- import all
	is := &ast.ImportStatement{Token: p.curToken}

	if !p.expectPeek(token.STRING) {
		return nil
	}
	is.Path = &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal.(string)}

	// Check for alias
	if p.peekTokenIs(token.IDENT_LOWER) && p.peekToken.Lexeme == "as" {
		p.nextToken() // consume 'as'

		// Alias can be either lowercase or uppercase identifier
		if p.peekTokenIs(token.IDENT_LOWER) || p.peekTokenIs(token.IDENT_UPPER) {
			p.nextToken()
			is.Alias = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
		} else {
			return nil
		}
	}

	// Check for import specification: (a, b, c) or !(a, b, c) or (*)
	// Note: alias and symbol imports are mutually exclusive
	if p.peekTokenIs(token.BANG) {
		if is.Alias != nil {
			p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
				diagnostics.ErrP006,
				p.curToken,
				"cannot use 'as' alias with exclude import; use either 'import \"path\" as alias' or 'import \"path\" !(symbols)'",
			))
			return nil
		}
		p.nextToken() // consume '!'
		if !p.expectPeek(token.LPAREN) {
			return nil
		}
		is.Exclude = p.parseIdentifierList()
		// Note: ImportAll is NOT set here - the presence of Exclude implies import all except excluded
		if !p.expectPeek(token.RPAREN) {
			return nil
		}
	} else if p.peekTokenIs(token.LPAREN) {
		if is.Alias != nil {
			p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
				diagnostics.ErrP006,
				p.curToken,
				"cannot use 'as' alias with selective import; use either 'import \"path\" as alias' or 'import \"path\" (symbols)'",
			))
			return nil
		}
		p.nextToken() // consume '('

		// Check for (*) - import all
		if p.peekTokenIs(token.ASTERISK) {
			p.nextToken() // consume '*'
			is.ImportAll = true
			if !p.expectPeek(token.RPAREN) {
				return nil
			}
		} else {
			// Parse specific symbols: (a, b, c)
			is.Symbols = p.parseIdentifierList()
			if !p.expectPeek(token.RPAREN) {
				return nil
			}
		}
	}

	return is
}

// parseIdentifierList parses a comma-separated list of identifiers
// Used for import specifications like (a, b, c)
func (p *Parser) parseIdentifierList() []*ast.Identifier {
	var identifiers []*ast.Identifier

	// Handle empty list
	if p.peekTokenIs(token.RPAREN) {
		return identifiers
	}

	// First identifier
	p.nextToken()
	if p.curToken.Type != token.IDENT_LOWER && p.curToken.Type != token.IDENT_UPPER {
		return nil
	}
	identifiers = append(identifiers, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)})

	// Subsequent identifiers
	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // consume ','
		p.nextToken() // move to next identifier
		if p.curToken.Type != token.IDENT_LOWER && p.curToken.Type != token.IDENT_UPPER {
			return nil
		}
		identifiers = append(identifiers, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)})
	}

	return identifiers
}

func (p *Parser) parseBreakStatement() *ast.BreakStatement {
	stmt := &ast.BreakStatement{Token: p.curToken}

	// Check if next token is start of expression
	if !p.peekTokenIs(token.NEWLINE) && !p.peekTokenIs(token.RBRACE) && !p.peekTokenIs(token.EOF) {
		p.nextToken()
		stmt.Value = p.parseExpression(LOWEST)
	}

	return stmt
}

func (p *Parser) parseContinueStatement() *ast.ContinueStatement {
	return &ast.ContinueStatement{Token: p.curToken}
}

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
			stmt.TypeParams = append(stmt.TypeParams, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)})
			p.nextToken() // move past IDENT

			if p.curTokenIs(token.COLON) {
				// Parse one or more trait constraints: t: Show, Cmp, Order
				for {
					p.nextToken() // move to Trait Name (or past :)
					if !p.curTokenIs(token.IDENT_UPPER) {
						p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
							diagnostics.ErrP005, p.curToken,
							"expected trait name (uppercase identifier)", p.curToken.Literal,
						))
						break
					}
					// Use the last added type param as the constrained type var
					lastParam := stmt.TypeParams[len(stmt.TypeParams)-1]
					constraint := &ast.TypeConstraint{TypeVar: lastParam.Value, Trait: p.curToken.Literal.(string)}
					stmt.Constraints = append(stmt.Constraints, constraint)
					p.nextToken() // move past Trait

					// Check if next is comma followed by another trait (uppercase)
					if p.curTokenIs(token.COMMA) && p.peekTokenIs(token.IDENT_UPPER) {
						continue // parse next trait for same type param
					}
					break
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
			fn := &ast.FunctionStatement{Token: p.curToken}
			if !p.expectPeek(token.IDENT_LOWER) {
				return nil
			}
			fn.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}

			// Parse type parameters <A, B> if present
			if p.peekTokenIs(token.LT) {
				p.nextToken() // consume name
				p.nextToken() // consume <
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
						fn.TypeParams = append(fn.TypeParams, &ast.Identifier{
							Token: p.curToken,
							Value: p.curToken.Literal.(string),
						})
						p.nextToken()
					} else if p.curTokenIs(token.IDENT_LOWER) {
						typeParam := &ast.Identifier{
							Token: p.curToken,
							Value: p.curToken.Literal.(string),
						}
						fn.TypeParams = append(fn.TypeParams, typeParam)
						p.nextToken() // move past IDENT

						// Check for constraints: t: Numeric, Show
						if p.curTokenIs(token.COLON) {
							p.nextToken() // consume ':'
							// Parse constraint list: Trait1, Trait2, ...
							for p.curTokenIs(token.IDENT_UPPER) {
								constraint := &ast.TypeConstraint{
									TypeVar: typeParam.Value,
									Trait:   p.curToken.Literal.(string),
								}
								fn.Constraints = append(fn.Constraints, constraint)
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
					} else {
						p.nextToken()
					}
				}
				// curToken is now GT
			}

			if !p.expectPeek(token.LPAREN) {
				return nil
			}
			fn.Parameters = p.parseFunctionParameters()

			// Return type
			if p.peekTokenIs(token.ARROW) {
				p.nextToken() // consume previous token
				p.nextToken() // consume ARROW, point to start of type
				fn.ReturnType = p.parseType()
			}

			// Optional: default implementation body
			if p.peekTokenIs(token.LBRACE) {
				p.nextToken() // move to LBRACE
				fn.Body = p.parseBlockStatement()
			}

			stmt.Signatures = append(stmt.Signatures, fn)

			if p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
			p.nextToken() // consume last part of signature
		} else if p.curTokenIs(token.OPERATOR) {
			// Parse operator method: operator (+)<A, B>(a: T, b: T) -> T
			fn := &ast.FunctionStatement{Token: p.curToken}

			// Expect ( after operator
			if !p.expectPeek(token.LPAREN) {
				return nil
			}

			// Get operator symbol: +, -, *, /, ==, !=, <, >, <=, >=
			p.nextToken()
			op := p.curToken.Lexeme
			fn.Operator = op
			// Create a synthetic name for the operator method
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
					if p.curTokenIs(token.IDENT_UPPER) {
						p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
							diagnostics.ErrP006, p.curToken,
							fmt.Sprintf("Type variables must start with a lowercase letter (got '%s')", p.curToken.Literal),
						))
						fn.TypeParams = append(fn.TypeParams, &ast.Identifier{
							Token: p.curToken,
							Value: p.curToken.Literal.(string),
						})
					} else if p.curTokenIs(token.IDENT_LOWER) {
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
				p.nextToken() // consume previous token
				p.nextToken() // consume ARROW, point to start of type
				fn.ReturnType = p.parseType()
			}

			// Optional: default implementation body
			if p.peekTokenIs(token.LBRACE) {
				p.nextToken() // move to LBRACE
				fn.Body = p.parseBlockStatement()
			}

			stmt.Signatures = append(stmt.Signatures, fn)

			if p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
			p.nextToken() // consume last part of signature
		} else {
			// Unexpected token in trait body
			p.nextToken()
		}
	}

	return stmt
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
			argType := p.parseType()
			if argType != nil {
				stmt.Args = append(stmt.Args, argType)
				p.nextToken()
			} else {
				p.nextToken()
			}
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

	// Body is required for instance implementations
	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	fn.Body = p.parseBlockStatement()

	return fn
}

func (p *Parser) parseFunctionStatement() *ast.FunctionStatement {
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
				// Parse one or more trait constraints: t: Show, Cmp, Order
				for {
					p.nextToken() // move to Trait Name (or past :)
					if !p.curTokenIs(token.IDENT_UPPER) {
						p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
							diagnostics.ErrP005, p.curToken,
							"expected trait name (uppercase identifier)", p.curToken.Literal,
						))
						break
					}
					constraint := &ast.TypeConstraint{TypeVar: typeParam.Value, Trait: p.curToken.Literal.(string)}
					stmt.Constraints = append(stmt.Constraints, constraint)
					p.nextToken() // move past Trait

					// Check if next is comma followed by another trait (uppercase)
					if p.curTokenIs(token.COMMA) && p.peekTokenIs(token.IDENT_UPPER) {
						continue // parse next trait for same type param
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
				// Parse one or more trait constraints: t: Show, Convert<String>, Order
				for {
					p.nextToken() // move to Trait Name
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
								p.splitRshift = true // Next nextToken will return >
								p.curToken.Type = token.GT
								p.curToken.Literal = ">"
								p.curToken.Lexeme = ">"
								break // Found GT
							}

							args = append(args, p.parseType())

							if p.peekTokenIs(token.COMMA) {
								p.nextToken()
							} else if p.peekTokenIs(token.RSHIFT) {
								// Next is >>. Consume and split.
								p.nextToken() // curToken is >>
								p.splitRshift = true
								p.curToken.Type = token.GT
								p.curToken.Literal = ">"
								p.curToken.Lexeme = ">"
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

					// Check if next is comma followed by another trait (uppercase)
					if p.curTokenIs(token.COMMA) && p.peekTokenIs(token.IDENT_UPPER) {
						continue // parse next trait for same type param
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
	} else if !p.peekTokenIs(token.LBRACE) && !p.peekTokenIs(token.NEWLINE) {
		// Legacy support: return type without '->' prefix
		p.nextToken()
		stmt.ReturnType = p.parseType()
	}

	// Check if Body starts
	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	stmt.Body = p.parseBlockStatement()

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
	if p.curTokenIs(token.LBRACE) {
		// Implicit alias for Record Type: type Point = { x: Int, y: Int }
		stmt.IsAlias = true
		stmt.TargetType = p.parseType()
	} else if stmt.IsAlias {
		// Explicit alias: type alias X = SomeType
		stmt.TargetType = p.parseType()
	} else {
		// ADT: Constructor | Constructor ...
		// Loop separated by PIPE (allow newlines before |)
		for {
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

func (p *Parser) parseConstantDeclaration(name *ast.Identifier) *ast.ConstantDeclaration {
	// kVAL :- 123
	// kVAL : Type :- 123
	stmt := &ast.ConstantDeclaration{Token: name.Token, Name: name}

	// Optional Type Annotation
	if p.peekTokenIs(token.COLON) {
		p.nextToken() // :
		p.nextToken() // Start of Type
		stmt.TypeAnnotation = p.parseType()
	}

	if !p.expectPeek(token.COLON_MINUS) {
		return nil
	}

	p.nextToken() // Consume :-
	stmt.Value = p.parseExpression(LOWEST)

	return stmt
}

func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {

	stmt := &ast.ExpressionStatement{Token: p.curToken}
	stmt.Expression = p.parseExpression(LOWEST)
	return stmt
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	p.nextToken()

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		// Skip leading newlines
		if p.curTokenIs(token.NEWLINE) {
			p.nextToken()
			continue
		}

		var stmt ast.Statement
		if p.curToken.Type == token.TYPE {
			// Type definitions are not allowed inside blocks (functions, if, match, etc.)
			p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
				diagnostics.ErrP006,
				p.curToken,
				"type definitions are only allowed at module level",
			))
			// Skip the entire type definition to recover
			// Type definitions can span multiple lines with | constructors
			for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
				// Skip until we find a line that doesn't start with | (new constructor)
				// or until we hit a keyword that starts a new statement
				if p.curTokenIs(token.NEWLINE) {
					p.nextToken()
					// Check if next line continues the type definition (starts with |)
					if !p.curTokenIs(token.PIPE) {
						break
					}
				} else {
					p.nextToken()
				}
			}
			continue
		} else if p.curToken.Type == token.FUN && (p.peekTokenIs(token.IDENT_LOWER) || p.peekTokenIs(token.LT)) {
			// Function inside block
			fnStmt := p.parseFunctionStatement()
			if fnStmt != nil {
				stmt = fnStmt
			}
			if p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
			p.nextToken()
		} else if p.curToken.Type == token.TRAIT {
			stmt = p.parseTraitDeclaration()
			if p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
			p.nextToken()
		} else if p.curToken.Type == token.INSTANCE {
			stmt = p.parseInstanceDeclaration()
			if p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
			p.nextToken()
		} else if p.curToken.Type == token.BREAK {
			stmt = p.parseBreakStatement()
			p.nextToken()
		} else if p.curToken.Type == token.CONTINUE {
			stmt = p.parseContinueStatement()
			p.nextToken()
		} else {
			stmt = p.parseExpressionStatementOrConstDecl()
			p.nextToken()
		}

		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}

		// Handle comma as statement separator
		if p.curTokenIs(token.COMMA) {
			p.nextToken() // consume comma
			// Skip any newlines after comma
			for p.curTokenIs(token.NEWLINE) {
				p.nextToken()
			}
		}
	}

	return block
}
