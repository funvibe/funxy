package parser

import (
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/token"
	"github.com/funvibe/funxy/internal/typesystem"
)

// parseKind parses a kind annotation.
// Grammar:
// Kind ::= AtomicKind ("->" Kind)*
// AtomicKind ::= "*" | "(" Kind ")"
func (p *Parser) parseKind() typesystem.Kind {
	// Parse left side
	left := p.parseAtomicKind()
	if left == nil {
		return nil
	}

	// Check for arrow
	if p.peekTokenIs(token.ARROW) {
		p.nextToken() // consume current
		p.nextToken() // consume ARROW
		right := p.parseKind()
		if right == nil {
			p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
				diagnostics.ErrP005,
				p.curToken,
				"expected kind after '->'",
				p.curToken.Literal,
			))
			return nil
		}
		return typesystem.KArrow{Left: left, Right: right}
	}

	return left
}

func (p *Parser) parseAtomicKind() typesystem.Kind {
	if p.curTokenIs(token.ASTERISK) { // '*' is used for Star kind
		return typesystem.Star
	}

	if p.curTokenIs(token.LPAREN) {
		p.nextToken()
		k := p.parseKind()
		if k == nil {
			return nil
		}
		if !p.expectPeek(token.RPAREN) {
			return nil
		}
		return k
	}

	return nil
}
