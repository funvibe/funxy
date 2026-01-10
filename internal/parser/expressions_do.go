package parser

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/token"
)

// doItem represents a single item in a do-block
type doItem interface {
	isDoItem()
}

type doBind struct {
	Var  *ast.Identifier
	Expr ast.Expression
}

func (d doBind) isDoItem() {}

type doDecl struct {
	Decl *ast.ConstantDeclaration
}

func (d doDecl) isDoItem() {}

type doExpr struct {
	Expr ast.Expression
}

func (d doExpr) isDoItem() {}

func (p *Parser) parseDoExpression() ast.Expression {
	// 'do' is already consumed by prefixParseFn
	doToken := p.curToken

	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	// expectPeek already consumed LBRACE, so curToken is now LBRACE
	// Enter the block: curToken becomes first token inside
	p.nextToken()

	var items []doItem

	// Parse items until RBRACE, using curToken like parseBlockStatement does
	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		// Skip leading newlines
		if p.curTokenIs(token.NEWLINE) {
			p.nextToken()
			continue
		}

		// Check for Bind: ident <- expr
		// p.curToken should be IDENT_LOWER, peekToken should be L_ARROW
		isBind := p.curToken.Type == token.IDENT_LOWER && p.peekTokenIs(token.L_ARROW)

		if isBind {
			ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
			p.nextToken() // curToken becomes <-
			p.nextToken() // curToken becomes start of expression

			expr := p.parseExpression(LOWEST)
			if expr == nil {
				return nil
			}

			items = append(items, doBind{Var: ident, Expr: expr})
			// After parseExpression, curToken is the last token of the expression.
			// We need to advance to the next token (likely NEWLINE or RBRACE)
			p.nextToken()
		} else {
			// Statement: Decl (ConstantDecl) or Expr
			stmt := p.parseExpressionStatementOrConstDecl()
			if stmt == nil {
				p.nextToken() // advance to avoid infinite loop on error
				continue
			}

			switch s := stmt.(type) {
			case *ast.ConstantDeclaration:
				items = append(items, doDecl{Decl: s})
			case *ast.ExpressionStatement:
				items = append(items, doExpr{Expr: s.Expression})
			default:
				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
					diagnostics.ErrP006,
					s.GetToken(),
					"only bindings, declarations, and expressions are allowed in do-blocks",
				))
			}
			p.nextToken() // advance past statement, like parseBlockStatement does
		}

		// Skip newlines after statement
		if p.curTokenIs(token.NEWLINE) {
			p.nextToken()
		}
	}

	if !p.curTokenIs(token.RBRACE) {
		return nil
	}

	return p.desugarDoItems(items, doToken)
}

func (p *Parser) desugarDoItems(items []doItem, doToken token.Token) ast.Expression {
	if len(items) == 0 {
		p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
			diagnostics.ErrP006,
			doToken,
			"empty do-block",
		))
		return nil
	}

	lastItem := items[len(items)-1]

	// Ensure last item is an expression
	if _, ok := lastItem.(doExpr); !ok {
		p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
			diagnostics.ErrP006,
			doToken,
			"last item in do-block must be an expression",
		))
		return nil
	}

	return p.desugarDoRecursive(items)
}

func (p *Parser) desugarDoRecursive(items []doItem) ast.Expression {
	if len(items) == 0 {
		return nil
	}

	head := items[0]
	tail := items[1:]

	switch item := head.(type) {
	case doBind:
		// x <- m
		// rest
		// => m >>= \x -> rest

		restExpr := p.desugarDoRecursive(tail)

		// Construct lambda: \x -> restExpr
		lambda := &ast.FunctionLiteral{
			Token: token.Token{Type: token.BACKSLASH, Literal: "\\"}, // Synthetic token
			Parameters: []*ast.Parameter{
				{Name: item.Var, Type: nil}, // Inferred type
			},
			Body: &ast.BlockStatement{
				Statements: []ast.Statement{
					&ast.ExpressionStatement{Expression: restExpr},
				},
			},
		}

		// Construct bind call: item.Expr >>= lambda
		bindOp := &ast.InfixExpression{
			Token:    token.Token{Type: token.USER_OP_BIND, Lexeme: ">>=", Literal: ">>="},
			Left:     item.Expr,
			Operator: ">>=",
			Right:    lambda,
		}

		return bindOp

	case doDecl:
		// x :- v
		// rest
		// => (fun() { x :- v; restExpr })()

		restExpr := p.desugarDoRecursive(tail)

		// Wrap in IIFE: (fun() { decl; restExpr })()

		iife := &ast.CallExpression{
			Token: token.Token{Type: token.LPAREN, Literal: "("},
			Function: &ast.FunctionLiteral{
				Token:      token.Token{Type: token.FUN, Literal: "fun"}, // IIFE usually uses standard fun
				Parameters: []*ast.Parameter{},
				Body: &ast.BlockStatement{
					Statements: []ast.Statement{
						item.Decl, // The declaration statement
						&ast.ExpressionStatement{Expression: restExpr},
					},
				},
			},
			Arguments: []ast.Expression{},
		}
		return iife

	case doExpr:
		// expr
		// if tail is empty, this is the result.
		// if tail is not empty, implies: expr >> rest (monadic then/sequence)
		// => expr >>= \_ -> rest

		if len(tail) == 0 {
			return item.Expr
		}

		restExpr := p.desugarDoRecursive(tail)

		// Construct lambda: \_ -> restExpr
		wildcard := &ast.Identifier{Token: token.Token{Type: token.IDENT_LOWER, Literal: "_"}, Value: "_"}
		lambda := &ast.FunctionLiteral{
			Token: token.Token{Type: token.BACKSLASH, Literal: "\\"},
			Parameters: []*ast.Parameter{
				{Name: wildcard, Type: nil},
			},
			Body: &ast.BlockStatement{
				Statements: []ast.Statement{
					&ast.ExpressionStatement{Expression: restExpr},
				},
			},
		}

		bindOp := &ast.InfixExpression{
			Token:    token.Token{Type: token.USER_OP_BIND, Lexeme: ">>=", Literal: ">>="},
			Left:     item.Expr,
			Operator: ">>=",
			Right:    lambda,
		}
		return bindOp
	}

	return nil
}
