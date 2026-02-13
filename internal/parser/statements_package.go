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
		// Skip newlines before closing ')'
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}
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
			// Supports multi-line: (a, b,\n  c, d)
			is.Symbols = p.parseIdentifierList()
			// Skip newlines before closing ')'
			for p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
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

	// Skip newlines after opening '(' — allows:
	//   import "lib/x" (
	//       foo, bar)
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

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
		// Skip newlines after comma — allows multi-line imports:
		//   import "lib/term" (red, green,
		//                      spinnerStart, spinnerStop)
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}
		p.nextToken() // move to next identifier
		if p.curToken.Type != token.IDENT_LOWER && p.curToken.Type != token.IDENT_UPPER {
			return nil
		}
		identifiers = append(identifiers, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)})
	}

	return identifiers
}
