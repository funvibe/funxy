package parser

import (
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
					diagnostics.ErrP004,
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
			diagnostics.ErrP004,
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
						diagnostics.ErrP004,
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
						diagnostics.ErrP004,
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
	if !p.expectPeek(token.IDENT_UPPER) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}

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

			if !p.curTokenIs(token.IDENT_UPPER) {
				// Type parameter must start with uppercase
				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
					diagnostics.ErrP005, p.curToken,
					"type parameter (uppercase)", p.curToken.Literal,
				))
				return nil
			}
			stmt.TypeParams = append(stmt.TypeParams, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)})
			p.nextToken() // move past IDENT
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
						fn.TypeParams = append(fn.TypeParams, &ast.Identifier{
							Token: p.curToken,
							Value: p.curToken.Literal.(string),
						})
					}
					p.nextToken()
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
	// instance Functor<Option> { ... }  -- HKT style
	if !p.expectPeek(token.IDENT_UPPER) {
		return nil
	}
	stmt.TraitName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}

	// Check for HKT syntax: instance Trait<TypeConstructor> { ... }
	// Or: instance Trait<TypeConstructor, E> { ... } with extra type params
	if p.peekTokenIs(token.LT) {
		p.nextToken() // consume trait name
		p.nextToken() // consume <
		// Parse the type constructor as Target
		stmt.Target = p.parseType()

		// Check for additional type parameters: <Result, E, F>
		for p.peekTokenIs(token.COMMA) {
			p.nextToken() // consume ,
			p.nextToken() // move to type param
			if p.curTokenIs(token.IDENT_UPPER) {
				stmt.TypeParams = append(stmt.TypeParams, &ast.Identifier{
					Token: p.curToken,
					Value: p.curToken.Literal.(string),
				})
			}
		}

		if !p.expectPeek(token.GT) {
			return nil
		}
	} else {
		p.nextToken()
		stmt.Target = p.parseType() // Int, (List A), etc.
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
			if p.curTokenIs(token.IDENT_UPPER) {
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

			if !p.curTokenIs(token.IDENT_UPPER) {
				// Type parameter must start with uppercase
				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
					diagnostics.ErrP005, p.curToken,
					"type parameter (uppercase)", p.curToken.Literal,
				))
				return nil
			}
			typeParam := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
			stmt.TypeParams = append(stmt.TypeParams, typeParam)

			p.nextToken() // move past IDENT

			if p.curTokenIs(token.COLON) {
				p.nextToken() // :
				p.nextToken() // Trait Name
				constraint := &ast.TypeConstraint{TypeVar: typeParam.Value, Trait: p.curToken.Literal.(string)}
				stmt.Constraints = append(stmt.Constraints, constraint)
				p.nextToken() // move past Trait
			}
		}
		// After loop, curToken is GT.
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
		if !p.expectPeek(token.IDENT_LOWER) {
			return nil
		}
		stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
	} else {
		// Normal function
		if !p.expectPeek(token.IDENT_LOWER) {
			return nil
		}
		stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
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

			if !p.curTokenIs(token.IDENT_UPPER) {
				// Type parameter must start with uppercase
				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
					diagnostics.ErrP005, p.curToken,
					"type parameter (uppercase)", p.curToken.Literal,
				))
				return nil
			}
			typeParam := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
			stmt.TypeParams = append(stmt.TypeParams, typeParam)

			p.nextToken() // move past IDENT

			if p.curTokenIs(token.COLON) {
				p.nextToken() // consume ':'
				// curToken is now Trait Name
				constraint := &ast.TypeConstraint{TypeVar: typeParam.Value, Trait: p.curToken.Literal.(string)}
				stmt.Constraints = append(stmt.Constraints, constraint)
				p.nextToken() // consume Trait Name
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

	// Return type is optional technically in many languages, but here syntax implies it might be present.
	// Support '->' prefix for return type (optional)
	if p.peekTokenIs(token.ARROW) {
		p.nextToken() // consume '->'
		p.nextToken() // point to start of type
		stmt.ReturnType = p.parseType()
	} else if !p.peekTokenIs(token.LBRACE) && !p.peekTokenIs(token.NEWLINE) && !p.peekTokenIs(token.WHERE) {
		// If not '{' and not newline and not 'where', assume it is a return type (legacy syntax without '->' or just type)
		// Check if next token looks like a type start?
		// p.parseType() will try.
		p.nextToken()
		stmt.ReturnType = p.parseType()
	}

	// Parse 'where' constraints: fun foo<F>(...) -> ... where Functor<F>, Show<A> { ... }
	if p.peekTokenIs(token.WHERE) {
		p.nextToken() // consume 'where'
		p.nextToken() // move to first constraint

		for {
			// Parse constraint: Trait<TypeVar> or Trait<TypeConstructor>
			if !p.curTokenIs(token.IDENT_UPPER) {
				break
			}
			traitName := p.curToken.Literal.(string)

			// Expect <
			if !p.expectPeek(token.LT) {
				break
			}
			p.nextToken() // move past <

			// Get type variable name
			if !p.curTokenIs(token.IDENT_UPPER) {
				break
			}
			typeVar := p.curToken.Literal.(string)

			// Expect >
			if !p.expectPeek(token.GT) {
				break
			}

			constraint := &ast.TypeConstraint{TypeVar: typeVar, Trait: traitName}
			stmt.Constraints = append(stmt.Constraints, constraint)

			// Check for comma (more constraints)
			if p.peekTokenIs(token.COMMA) {
				p.nextToken() // consume ,
				p.nextToken() // move to next constraint
			} else {
				break
			}
		}
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
	param := &ast.Parameter{Token: p.curToken}

	// Allow underscore as "ignored" parameter
	if p.curTokenIs(token.UNDERSCORE) {
		param.Name = &ast.Identifier{Token: p.curToken, Value: "_"}
		param.IsIgnored = true
	} else if p.curTokenIs(token.IDENT_LOWER) {
		param.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
	} else {
		// Error? Or support destructuring later
		return nil
	}

	// Check for type annotation
	if p.peekTokenIs(token.COLON) {
		p.nextToken()
		p.nextToken()
		param.Type = p.parseType()
	}

	// Check for variadic ...
	if p.peekTokenIs(token.ELLIPSIS) {
		p.nextToken()
		param.IsVariadic = true
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

	if !p.expectPeek(token.IDENT_UPPER) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}

	// 2. Parse Type Parameters <T, U>
	if p.peekTokenIs(token.LT) {
		p.nextToken() // consume <. curToken is <.
		p.nextToken() // move to first type param

		for !p.curTokenIs(token.GT) && !p.curTokenIs(token.EOF) {
			if p.curTokenIs(token.COMMA) {
				p.nextToken()
				continue
			}

			if !p.curTokenIs(token.IDENT_UPPER) {
				// Type parameter must start with uppercase
				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
					diagnostics.ErrP005, p.curToken,
					"type parameter (uppercase)", p.curToken.Literal,
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
		// Implicit alias for Record Type
		stmt.IsAlias = true
		stmt.TargetType = p.parseType()
	} else if stmt.IsAlias {
		stmt.TargetType = p.parseType()
	} else {
		// Heuristic: Check if it looks like an alias (e.g. generic type application)?
		// If RHS is `Name < ...`, it is likely an alias for a type application.
		if p.curTokenIs(token.IDENT_UPPER) && p.peekTokenIs(token.LT) {
			stmt.IsAlias = true
			stmt.TargetType = p.parseType()
			return stmt
		}

		// Heuristic: If RHS starts with '(' it's likely a function type or tuple - treat as alias
		// e.g. type Handler = (Int) -> Nil
		if p.curTokenIs(token.LPAREN) {
			stmt.IsAlias = true
			stmt.TargetType = p.parseType()
			return stmt
		}

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
			diagnostics.ErrP005,
			p.curToken,
			token.IDENT_UPPER,
			p.curToken.Type,
		))
		return nil
	}
	if !p.curTokenIs(token.IDENT_UPPER) {
		return nil
	}
	dc.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}

	// Parse parameters (Types) until next PIPE or NEWLINE/EOF
	// New syntax: Cons T (List<T>)
	// Space separated atomic types?
	// Or `Cons(T, List<T>)`?
	// The user query didn't specify changing constructor definition syntax.
	// But `List<T>` is now atomic (NamedType with args).
	// Previously `List a` was parsed as `parseAtomicType` (List) then loop?
	// No, `List a` is `TApp`.
	// `parseDataConstructor` called `parseAtomicType`.
	// If I have `Cons T`, `T` is atomic.
	// If I have `Cons List<T>`, `List<T>` is atomic (handled by parseTypeApplication -> parseAtomicType + <...>).

	// Wait, `parseType` handles `parseTypeApplication`.
	// `parseAtomicType` handles `(Type)` and `Name`.
	// `parseTypeApplication` calls `parseAtomicType` then checks for `<...>`.

	// If `parseDataConstructor` calls `parseAtomicType`, it won't parse `<T>`.
	// It should call `parseTypeApplication`?
	// Or `parseType` (which includes Arrow)?
	// Constructor params usually don't have arrows unless parenthesized.

	// Let's change `parseAtomicType` to `parseTypeApplication` (or equivalent logic that parses one complete type unit).
	// `parseType` handles arrows, which might be ambiguous if not parenthesized?
	// `Cons Int -> Int` -> `Cons (Int -> Int)`?
	// Usually `Cons Int` means `Cons` takes `Int`.
	// If I use `parseType`, it consumes as much as possible.
	// `Cons Int | Empty` -> `Int | Empty` is not a valid type.
	// `parseType` parses `Int`. Next token is `|`.
	// `parseType` stops at `|`? `|` has precedence?
	// `Arrow` has precedence. `|` is not in type precedence usually (except Sum Type which is statement level here).

	// So `parseType` is safe to call.

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

		// If parseType consumed up to PIPE, we loop check handles it.
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

		// Check if it's :- (COLON_MINUS) immediately?
		// The lexer emits COLON_MINUS as one token.
		// But here we consumed COLON. Wait.
		// If lexer sees `:-`, it emits COLON_MINUS.
		// So `kVAL : Type :- 123` -> IDENT COLON Type COLON_MINUS Expr
		// `kVAL :- 123` -> IDENT COLON_MINUS Expr

		// If we are here, next token was COLON. So it's type annotation.
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
			stmt = p.parseTypeDeclarationStatement()
			if p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
			p.nextToken()
		} else if p.curToken.Type == token.FUN && (p.peekTokenIs(token.IDENT_LOWER) || p.peekTokenIs(token.LT)) {
			// Function inside block
			stmt = p.parseFunctionStatement()
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
			stmt = p.parseExpressionStatement()
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
