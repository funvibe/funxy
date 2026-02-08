package parser

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/token"
)

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

func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	stmt := &ast.ReturnStatement{Token: p.curToken}

	if !p.peekTokenIs(token.NEWLINE) && !p.peekTokenIs(token.RBRACE) && !p.peekTokenIs(token.EOF) {
		p.nextToken()
		stmt.Value = p.parseExpression(LOWEST)
	}

	return stmt
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

// parseConstKeywordDeclaration parses `const x = ...`
func (p *Parser) parseConstKeywordDeclaration() ast.Statement {
	stmt := &ast.ConstantDeclaration{Token: p.curToken}
	p.nextToken() // consume const

	// Parse expression (which parses assignment too)
	expr := p.parseExpression(LOWEST)
	if expr == nil {
		return nil
	}

	// Check if it resulted in an Assignment (x = y) or PatternAssignment ((a,b) = z)
	if assign, ok := expr.(*ast.AssignExpression); ok {
		// x = 1  or  x: Int = 1
		if ident, ok := assign.Left.(*ast.Identifier); ok {
			stmt.Name = ident
			stmt.TypeAnnotation = assign.AnnotatedType
			stmt.Value = assign.Value
			return stmt
		}
		// Invalid LHS
		p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(diagnostics.ErrP001, assign.Left.GetToken(), "identifier", assign.Left.GetToken().Type))
		return nil
	} else if patAssign, ok := expr.(*ast.PatternAssignExpression); ok {
		// (a, b) = (1, 2)
		stmt.Pattern = patAssign.Pattern
		stmt.Value = patAssign.Value
		stmt.TypeAnnotation = patAssign.AnnotatedType
		return stmt
	}

	// If we got here, we parsed an expression but it wasn't an assignment.
	// E.g. `const x` (without =)
	p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(diagnostics.ErrP005, p.curToken, token.ASSIGN, p.curToken.Type))
	return nil
}

func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {

	stmt := &ast.ExpressionStatement{Token: p.curToken}
	stmt.Expression = p.parseExpression(LOWEST)
	if stmt.Expression == nil {
		return nil
	}
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
		} else if p.curToken.Type == token.CONST {
			stmt = p.parseConstKeywordDeclaration()
			p.nextToken()
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
			traitStmt := p.parseTraitDeclaration()
			if traitStmt != nil {
				stmt = traitStmt
			}
			if p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
			p.nextToken()
		} else if p.curToken.Type == token.INSTANCE {
			instStmt := p.parseInstanceDeclaration()
			if instStmt != nil {
				stmt = instStmt
			}
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
		} else if p.curToken.Type == token.RETURN {
			stmt = p.parseReturnStatement()
			p.nextToken()
		} else {
			stmt = p.parseExpressionStatementOrConstDecl()
			// In recovery, if we're at a block-closer and ELSE follows, let the outer if handle it.
			if !(p.curTokenIs(token.RBRACE) && p.peekTokenIs(token.ELSE)) {
				p.nextToken()
			}
		}

		if stmt != nil && !isNilStatement(stmt) {
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

	block.RBraceToken = p.curToken // Save closing brace
	return block
}
