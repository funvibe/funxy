package parser

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/token"
)

func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	exp := &ast.IndexExpression{Token: p.curToken, Left: left}

	p.nextToken()
	exp.Index = p.parseExpression(LOWEST)

	if !p.expectPeek(token.RBRACKET) {
		return nil
	}

	return exp
}

func (p *Parser) parseMemberExpression(left ast.Expression) ast.Expression {
	exp := &ast.MemberExpression{Token: p.curToken, Left: left, IsOptional: false}
	p.nextToken() // .
	if !p.curTokenIs(token.IDENT_LOWER) && !p.curTokenIs(token.IDENT_UPPER) {
		return nil // Expected identifier
	}
	exp.Member = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
	return exp
}

func (p *Parser) parseOptionalChainExpression(left ast.Expression) ast.Expression {
	exp := &ast.MemberExpression{Token: p.curToken, Left: left, IsOptional: true}
	p.nextToken() // ?.
	if !p.curTokenIs(token.IDENT_LOWER) && !p.curTokenIs(token.IDENT_UPPER) {
		return nil // Expected identifier
	}
	exp.Member = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
	return exp
}
