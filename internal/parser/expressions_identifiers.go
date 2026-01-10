package parser

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/token"
)

func (p *Parser) parseIdentifier() ast.Expression {
	ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}

	// Special case: identifier followed by { without () creates a call with trailing block
	// This enables clean DSL syntax: div { ... } instead of div() { ... }
	// And constructor record syntax: MkUser { field: value }
	if !p.disallowTrailingLambda && p.peekTokenIs(token.LBRACE) {
		if p.curTokenIs(token.IDENT_LOWER) || p.curTokenIs(token.IDENT_UPPER) {
			isLower := p.curTokenIs(token.IDENT_LOWER)

			// Create CallExpression with no regular arguments, only trailing block
			call := &ast.CallExpression{
				Token:     p.curToken,
				Function:  ident,
				Arguments: []ast.Expression{},
			}

			p.nextToken() // consume identifier, move to {

			// parseRecordLiteralOrBlock disambiguates between Record and Block
			arg := p.parseRecordLiteralOrBlock()

			// For DSLs (IDENT_LOWER), if we got a BlockStatement, we historically treated it as a ListLiteral
			// containing the expressions in the block.
			// e.g. div { span {}, span {} } -> div([span{}, span{}])
			if isLower {
				// Special case: empty RecordLiteral {} in DSL context should be treated as empty ListLiteral []
				// This handles: div { } -> div([]) instead of div({})
				if rec, ok := arg.(*ast.RecordLiteral); ok && len(rec.Fields) == 0 && rec.Spread == nil {
					arg = &ast.ListLiteral{Token: rec.Token, Elements: []ast.Expression{}}
				} else if block, ok := arg.(*ast.BlockStatement); ok {
					list := &ast.ListLiteral{Token: block.Token}
					allExprs := true
					for _, stmt := range block.Statements {
						if exprStmt, ok := stmt.(*ast.ExpressionStatement); ok {
							list.Elements = append(list.Elements, exprStmt.Expression)
						} else {
							allExprs = false
							// Use type assertion for TokenProvider
							var tok token.Token
							if provider, ok := stmt.(ast.TokenProvider); ok {
								tok = provider.GetToken()
							} else {
								tok = p.curToken // Fallback
							}
							p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
								diagnostics.ErrP005,
								tok,
								"block syntax in DSL arguments only supports expressions",
							))
						}
					}
					if allExprs {
						arg = list
					}
				}
			}

			if arg != nil {
				call.Arguments = append(call.Arguments, arg)
			}

			return call
		}
	}

	return ident
}

// parseUnderscore parses the _ wildcard as an identifier for use in patterns
func (p *Parser) parseUnderscore() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: "_"}
}
