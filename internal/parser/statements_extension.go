package parser

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/token"
)

// parseStatement parses a single statement.
// It matches the logic inside parseBlockStatement loop but for a single item.
func (p *Parser) parseStatement() ast.Statement {
	if p.curToken.Type == token.TYPE {
		stmt := p.parseTypeDeclarationStatement()
		if p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}
		p.nextToken() // consume the end of statement token if needed?
		// parseTypeDeclarationStatement leaves curToken at end.
		// Usually caller consumes newline.
		// parseStatement should behave like parseExpressionStatementOrConstDecl:
		// return the statement node.
		// The caller (parseDoExpression) handles newlines.
		return stmt
	} else if p.curToken.Type == token.FUN && (p.peekTokenIs(token.IDENT_LOWER) || p.peekTokenIs(token.LT) || p.peekTokenIs(token.LPAREN)) {
		// Function declaration
		// Same logic as in ParseProgram/parseBlockStatement
		isExtension := false
		if p.peekTokenIs(token.LPAREN) {
			tokens := p.stream.Peek(50)
			balance := 1
			foundRParen := false
			idx := 0
			for i, t := range tokens {
				if t.Type == token.LPAREN {
					balance++
				} else if t.Type == token.RPAREN {
					balance--
					if balance == 0 {
						foundRParen = true
						idx = i
						break
					}
				}
			}

			if foundRParen && idx+1 < len(tokens) {
				nextToken := tokens[idx+1]
				if nextToken.Type == token.IDENT_LOWER {
					isExtension = true
				}
			}
		} else {
			isExtension = true
		}

		if isExtension {
			stmt := p.parseFunctionStatement()
			if p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
			p.nextToken() // consume last part
			return stmt
		} else {
			// Function literal expression statement
			stmt := p.parseExpressionStatement()
			p.nextToken()
			return stmt
		}
	} else if p.curToken.Type == token.TRAIT {
		stmt := p.parseTraitDeclaration()
		if p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}
		p.nextToken()
		return stmt
	} else if p.curToken.Type == token.INSTANCE {
		stmt := p.parseInstanceDeclaration()
		if p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}
		p.nextToken()
		return stmt
	} else if p.curToken.Type == token.BREAK {
		stmt := p.parseBreakStatement()
		p.nextToken()
		return stmt
	} else if p.curToken.Type == token.CONTINUE {
		stmt := p.parseContinueStatement()
		p.nextToken()
		return stmt
	} else if p.curToken.Type == token.PACKAGE || p.curToken.Type == token.IMPORT {
		p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
			diagnostics.ErrP005,
			p.curToken,
			"package or import declaration must be at the top of the file",
		))
		p.nextToken()
		for !p.curTokenIs(token.NEWLINE) && !p.curTokenIs(token.EOF) {
			p.nextToken()
		}
		return nil
	} else {
		// Expression statement or Constant declaration
		// parseExpressionStatementOrConstDecl leaves curToken at the last token of expr
		stmt := p.parseExpressionStatementOrConstDecl()
		p.nextToken()
		return stmt
	}
}
