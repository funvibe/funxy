package parser

import (
	"math/big"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/token"
)

func (p *Parser) parseExpression(precedence int) ast.Expression {
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}
	leftExp := prefix()

	for {
		// Check if we should continue parsing infix operators
		if p.peekTokenIs(token.NEWLINE) {
			// Look ahead past newlines for continuation operators
			if !p.hasContinuationOperator() {
				break
			}
			// Skip newlines to get to the operator
			for p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
		}

		if precedence >= p.peekPrecedence() {
			break
		}

		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}
		p.nextToken()
		leftExp = infix(leftExp)
	}

	return leftExp
}

// hasContinuationOperator checks if there's an infix operator after newlines
// that should continue the current expression (e.g., |>, >>=, ++, etc.)
func (p *Parser) hasContinuationOperator() bool {
	// Peek ahead past newlines
	tokens := p.stream.Peek(10)
	for _, tok := range tokens {
		if tok.Type == token.NEWLINE {
			continue
		}
		// Check if it's a continuation operator
		return isContinuationOperator(tok.Type)
	}
	return false
}

// isContinuationOperator returns true for operators that can continue on next line
func isContinuationOperator(t token.TokenType) bool {
	switch t {
	case token.PIPE_GT, // |>
		token.CONCAT,      // ++
		token.COMPOSE,     // ,,
		token.USER_OP_APP: // $
		return true
	}
	return false
}

func (p *Parser) parsePrefixExpression() ast.Expression {
	expression := &ast.PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal.(string),
	}
	p.nextToken()
	expression.Right = p.parseExpression(PREFIX)
	return expression
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	expression := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal.(string),
		Left:     left,
	}

	precedence := p.curPrecedence()
	p.nextToken()
	// Allow newline after operator (e.g., x && \n y)
	for p.curToken.Type == token.NEWLINE {
		p.nextToken()
	}
	expression.Right = p.parseExpression(precedence)

	return expression
}

// parseRightAssocInfixExpression parses right-associative operators like ::
// 1 :: 2 :: [] parses as 1 :: (2 :: [])
func (p *Parser) parseRightAssocInfixExpression(left ast.Expression) ast.Expression {
	expression := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal.(string),
		Left:     left,
	}

	precedence := p.curPrecedence()
	p.nextToken()
	// Allow newline after operator
	for p.curToken.Type == token.NEWLINE {
		p.nextToken()
	}
	// Use precedence - 1 to make it right-associative
	expression.Right = p.parseExpression(precedence - 1)

	return expression
}

func (p *Parser) parsePostfixExpression(left ast.Expression) ast.Expression {
	return &ast.PostfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal.(string),
		Left:     left,
	}
}

func (p *Parser) parseGroupedExpression() ast.Expression {
	startToken := p.curToken
	p.nextToken() // consume '('

	// Skip newlines after (
	for p.curTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	// Check for empty tuple ()
	if p.curTokenIs(token.RPAREN) {
		return &ast.TupleLiteral{Token: startToken, Elements: []ast.Expression{}}
	}

	// Check for operator-as-function: (+), (-), (*), etc.
	if p.isOperatorToken() && p.peekTokenIs(token.RPAREN) {
		op := p.curToken.Lexeme
		p.nextToken() // consume operator
		// curToken is now RPAREN, no need to expectPeek
		return &ast.OperatorAsFunction{Token: startToken, Operator: op}
	}

	exp := p.parseExpression(LOWEST)

	// Skip newlines after expression
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	// If we see a comma, it's a tuple
	if p.peekTokenIs(token.COMMA) {
		elements := []ast.Expression{exp}
		for p.peekTokenIs(token.COMMA) {
			p.nextToken() // consume comma
			// Skip newlines after comma (for non-bracket-aware parsers)
			for p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
			// Handle trailing comma
			if p.peekTokenIs(token.RPAREN) {
				break
			}
			p.nextToken() // move to next expression start
			// Skip newlines before expression (for non-bracket-aware parsers)
			for p.curTokenIs(token.NEWLINE) {
				p.nextToken()
			}
			elements = append(elements, p.parseExpression(LOWEST))
			// Skip newlines after expression (for non-bracket-aware parsers)
			for p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
		}

		if !p.expectPeek(token.RPAREN) {
			return nil
		}
		return &ast.TupleLiteral{Token: startToken, Elements: elements}
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}
	return exp
}

func (p *Parser) parseIfExpression() ast.Expression {
	expression := &ast.IfExpression{Token: p.curToken}

	p.nextToken() // consume 'if'

	prev := p.disallowTrailingLambda
	p.disallowTrailingLambda = true
	expression.Condition = p.parseExpression(LOWEST)
	p.disallowTrailingLambda = prev

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	expression.Consequence = p.parseBlockStatement()

	// Skip newlines
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	if p.peekTokenIs(token.ELSE) {
		p.nextToken()

		if p.peekTokenIs(token.IF) {
			p.nextToken()
			ifExpr := p.parseIfExpression()
			block := &ast.BlockStatement{
				Token:      token.Token{Type: token.LBRACE, Lexeme: "{"},
				Statements: []ast.Statement{&ast.ExpressionStatement{Token: ifExpr.GetToken(), Expression: ifExpr}},
			}
			expression.Alternative = block
		} else {
			if !p.expectPeek(token.LBRACE) {
				return nil
			}
			expression.Alternative = p.parseBlockStatement()
		}
	}

	return expression
}

func (p *Parser) parseForExpression() ast.Expression {
	expr := &ast.ForExpression{Token: p.curToken}
	p.nextToken() // consume 'for'

	// Check for iteration: for item in iterable
	// Or condition: for condition
	// If next is IDENT and peek after is IN, then iteration.
	if p.curTokenIs(token.IDENT_LOWER) && p.peekTokenIs(token.IN) {
		// Iteration loop
		expr.ItemName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
		p.nextToken() // consume ident
		p.nextToken() // consume in

		prev := p.disallowTrailingLambda
		p.disallowTrailingLambda = true
		expr.Iterable = p.parseExpression(LOWEST)
		p.disallowTrailingLambda = prev
	} else {
		// Standard condition loop
		prev := p.disallowTrailingLambda
		p.disallowTrailingLambda = true
		expr.Condition = p.parseExpression(LOWEST)
		p.disallowTrailingLambda = prev
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	expr.Body = p.parseBlockStatement()
	return expr
}

func (p *Parser) parseMatchExpression() ast.Expression {
	ce := &ast.MatchExpression{Token: p.curToken}

	p.nextToken() // consume 'match'

	prev := p.disallowTrailingLambda
	p.disallowTrailingLambda = true
	ce.Expression = p.parseExpression(LOWEST)
	p.disallowTrailingLambda = prev

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	// Consume optional newline after '{'
	if p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	for !p.peekTokenIs(token.RBRACE) && !p.peekTokenIs(token.EOF) {
		// Skip newlines between arms
		if p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
			continue
		}

		if p.peekTokenIs(token.RBRACE) {
			break
		}

		// We are at start of an arm (pattern)
		// p.peekToken is the start of pattern.
		// We must advance curToken to it.
		p.nextToken()

		arm := p.parseMatchArm()
		if arm != nil {
			ce.Arms = append(ce.Arms, arm)
		}

		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
		}
	}

	if !p.expectPeek(token.RBRACE) {
		return nil
	}

	return ce
}

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
				if block, ok := arg.(*ast.BlockStatement); ok {
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

func (p *Parser) parseIntegerLiteral() ast.Expression {
	return &ast.IntegerLiteral{Token: p.curToken, Value: p.curToken.Literal.(int64)}
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	return &ast.FloatLiteral{Token: p.curToken, Value: p.curToken.Literal.(float64)}
}

func (p *Parser) parseBigIntLiteral() ast.Expression {
	return &ast.BigIntLiteral{Token: p.curToken, Value: p.curToken.Literal.(*big.Int)}
}

func (p *Parser) parseRationalLiteral() ast.Expression {
	return &ast.RationalLiteral{Token: p.curToken, Value: p.curToken.Literal.(*big.Rat)}
}

func (p *Parser) parseBoolean() ast.Expression {
	return &ast.BooleanLiteral{Token: p.curToken, Value: p.curTokenIs(token.TRUE)}
}

func (p *Parser) parseNil() ast.Expression {
	return &ast.NilLiteral{Token: p.curToken}
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal.(string)}
}

func (p *Parser) parseFormatStringLiteral() ast.Expression {
	return &ast.FormatStringLiteral{Token: p.curToken, Value: p.curToken.Literal.(string)}
}

func (p *Parser) parseInterpolatedString() ast.Expression {
	tok := p.curToken
	raw := p.curToken.Literal.(string)

	parts := p.parseInterpolationParts(raw)
	if len(parts) == 1 {
		// Optimize: if only one string part, return StringLiteral
		if sl, ok := parts[0].(*ast.StringLiteral); ok {
			return sl
		}
	}

	return &ast.InterpolatedString{Token: tok, Parts: parts}
}

// parseInterpolationParts splits "Hello, ${name}!" into [StringLiteral("Hello, "), Identifier(name), StringLiteral("!")]
func (p *Parser) parseInterpolationParts(raw string) []ast.Expression {
	var parts []ast.Expression
	i := 0
	start := 0

	for i < len(raw) {
		// Look for ${
		if i+1 < len(raw) && raw[i] == '$' && raw[i+1] == '{' {
			// Add text before ${
			if i > start {
				parts = append(parts, &ast.StringLiteral{
					Token: p.curToken,
					Value: raw[start:i],
				})
			}

			// Find matching }
			j := i + 2
			braceDepth := 1
			for j < len(raw) && braceDepth > 0 {
				if raw[j] == '{' {
					braceDepth++
				} else if raw[j] == '}' {
					braceDepth--
				}
				j++
			}

			// Parse expression inside ${...}
			exprStr := raw[i+2 : j-1]
			expr := p.parseEmbeddedExpression(exprStr)
			if expr != nil {
				parts = append(parts, expr)
			}

			i = j
			start = j
		} else {
			i++
		}
	}

	// Add remaining text
	if start < len(raw) {
		parts = append(parts, &ast.StringLiteral{
			Token: p.curToken,
			Value: raw[start:],
		})
	}

	return parts
}

// parseEmbeddedExpression parses a string as an expression
func (p *Parser) parseEmbeddedExpression(exprStr string) ast.Expression {
	// Create a new lexer and parser for the embedded expression
	l := lexer.New(exprStr)
	stream := lexer.NewTokenStream(l)
	embeddedParser := New(stream, p.ctx)
	return embeddedParser.parseExpression(LOWEST)
}

func (p *Parser) parseCharLiteral() ast.Expression {
	return &ast.CharLiteral{Token: p.curToken, Value: p.curToken.Literal.(int64)}
}

// parseBytesLiteral parses bytes literals: @"hello", @x"48656C", @b"01001000"
func (p *Parser) parseBytesLiteral() ast.Expression {
	lit := &ast.BytesLiteral{Token: p.curToken}
	lit.Content = p.curToken.Literal.(string)

	switch p.curToken.Type {
	case token.BYTES_STRING:
		lit.Kind = "string"
	case token.BYTES_HEX:
		lit.Kind = "hex"
	case token.BYTES_BIN:
		lit.Kind = "bin"
	}

	return lit
}

// parseBitsLiteral parses bits literals: #b"10101010", #x"FF"
func (p *Parser) parseBitsLiteral() ast.Expression {
	lit := &ast.BitsLiteral{Token: p.curToken}
	lit.Content = p.curToken.Literal.(string)

	switch p.curToken.Type {
	case token.BITS_BIN:
		lit.Kind = "bin"
	case token.BITS_HEX:
		lit.Kind = "hex"
	case token.BITS_OCT:
		lit.Kind = "oct"
	}

	return lit
}

func (p *Parser) parseListLiteral() ast.Expression {
	list := &ast.ListLiteral{Token: p.curToken}
	list.Elements = p.parseExpressionList(token.RBRACKET)
	return list
}

// parseMapLiteral parses a map literal: %{ key => value, key2 => value2 }
func (p *Parser) parseMapLiteral() ast.Expression {
	mapLit := &ast.MapLiteral{Token: p.curToken}
	mapLit.Pairs = []struct{ Key, Value ast.Expression }{}

	// Skip newlines after %{
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	// Empty map: %{}
	if p.peekTokenIs(token.RBRACE) {
		p.nextToken()
		return mapLit
	}

	// Parse first pair
	p.nextToken()
	// Skip newlines before key
	for p.curTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	// Use PIPE_PREC to stop before => (which has PIPE_PREC precedence)
	key := p.parseExpression(PIPE_PREC)

	// Skip newlines before =>
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	if !p.expectPeek(token.USER_OP_IMPLY) { // =>
		return nil
	}
	p.nextToken()
	// Skip newlines after =>
	for p.curTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	// Value uses PIPE_PREC to stop before , or }
	value := p.parseExpression(PIPE_PREC)
	mapLit.Pairs = append(mapLit.Pairs, struct{ Key, Value ast.Expression }{key, value})

	// Skip newlines after value
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	// Parse remaining pairs
	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // consume comma
		// Skip newlines after comma
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}
		// Handle trailing comma
		if p.peekTokenIs(token.RBRACE) {
			break
		}
		p.nextToken()
		// Skip newlines before key
		for p.curTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		// Use PIPE_PREC to stop before =>
		key := p.parseExpression(PIPE_PREC)

		// Skip newlines before =>
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		if !p.expectPeek(token.USER_OP_IMPLY) { // =>
			return nil
		}
		p.nextToken()
		// Skip newlines after =>
		for p.curTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		// Value uses PIPE_PREC to stop before , or }
		value := p.parseExpression(PIPE_PREC)
		mapLit.Pairs = append(mapLit.Pairs, struct{ Key, Value ast.Expression }{key, value})

		// Skip newlines after value
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}
	}

	if !p.expectPeek(token.RBRACE) {
		return nil
	}

	return mapLit
}

func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	exp := &ast.IndexExpression{Token: p.curToken, Left: left}

	p.nextToken()
	exp.Index = p.parseExpression(LOWEST)

	if !p.expectPeek(token.RBRACKET) {
		return nil
	}

	return exp
}

func (p *Parser) parseAssignExpression(left ast.Expression) ast.Expression {
	// 'left' must be a valid l-value (identifier, member expression, or pattern)
	// Or identifier with annotation

	var annotatedType ast.Type
	var target ast.Expression

	// Handle annotated expression: x: Int = 5
	if anno, ok := left.(*ast.AnnotatedExpression); ok {
		target = anno.Expression
		annotatedType = anno.TypeAnnotation
	} else {
		target = left
	}

	// Validate target is l-value or pattern
	switch target.(type) {
	case *ast.Identifier:
		// OK - simple assignment
	case *ast.MemberExpression:
		// OK - member assignment
	case *ast.TupleLiteral:
		// Pattern destructuring: (a, b) = expr
		// Convert to ConstantDeclaration with pattern
		pattern := p.tupleExprToPattern(target.(*ast.TupleLiteral))
		if pattern == nil {
			return nil
		}
		tok := p.curToken
		p.nextToken() // consume '='
		value := p.parseExpression(LOWEST)
		return &ast.PatternAssignExpression{Token: tok, Pattern: pattern, Value: value}
	case *ast.ListLiteral:
		// Pattern destructuring: [a, b, rest...] = expr
		pattern := p.listExprToPattern(target.(*ast.ListLiteral))
		if pattern == nil {
			return nil
		}
		tok := p.curToken
		p.nextToken() // consume '='
		value := p.parseExpression(LOWEST)
		return &ast.PatternAssignExpression{Token: tok, Pattern: pattern, Value: value}
	case *ast.RecordLiteral:
		// Pattern destructuring: { x: a, y: b } = expr
		pattern := p.recordExprToPattern(target.(*ast.RecordLiteral))
		if pattern == nil {
			return nil
		}
		tok := p.curToken
		p.nextToken() // consume '='
		value := p.parseExpression(LOWEST)
		return &ast.PatternAssignExpression{Token: tok, Pattern: pattern, Value: value}
	default:
		return nil // Invalid assignment target
	}

	stmt := &ast.AssignExpression{Token: p.curToken, Left: target, AnnotatedType: annotatedType}
	p.nextToken() // consume '='
	stmt.Value = p.parseExpression(LOWEST)
	return stmt
}

// parseCompoundAssignExpression handles +=, -=, *=, /=, %=, **=
// Desugars `x += y` to `x = x + y`
func (p *Parser) parseCompoundAssignExpression(left ast.Expression) ast.Expression {
	// Determine the operator from the compound assignment token
	compoundTok := p.curToken
	var operator string
	var opToken token.Token

	switch compoundTok.Type {
	case token.PLUS_ASSIGN:
		operator = "+"
		opToken = token.Token{Type: token.PLUS, Lexeme: "+", Line: compoundTok.Line, Column: compoundTok.Column}
	case token.MINUS_ASSIGN:
		operator = "-"
		opToken = token.Token{Type: token.MINUS, Lexeme: "-", Line: compoundTok.Line, Column: compoundTok.Column}
	case token.ASTERISK_ASSIGN:
		operator = "*"
		opToken = token.Token{Type: token.ASTERISK, Lexeme: "*", Line: compoundTok.Line, Column: compoundTok.Column}
	case token.SLASH_ASSIGN:
		operator = "/"
		opToken = token.Token{Type: token.SLASH, Lexeme: "/", Line: compoundTok.Line, Column: compoundTok.Column}
	case token.PERCENT_ASSIGN:
		operator = "%"
		opToken = token.Token{Type: token.PERCENT, Lexeme: "%", Line: compoundTok.Line, Column: compoundTok.Column}
	case token.POWER_ASSIGN:
		operator = "**"
		opToken = token.Token{Type: token.POWER, Lexeme: "**", Line: compoundTok.Line, Column: compoundTok.Column}
	default:
		return nil
	}

	// Validate target is a valid l-value (identifier or member expression)
	var target ast.Expression
	if anno, ok := left.(*ast.AnnotatedExpression); ok {
		target = anno.Expression
	} else {
		target = left
	}

	switch target.(type) {
	case *ast.Identifier, *ast.MemberExpression:
		// OK - valid l-value for compound assignment
	default:
		p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
			diagnostics.ErrP002,
			compoundTok,
		))
		return nil
	}

	p.nextToken() // consume the compound assignment operator
	right := p.parseExpression(LOWEST)

	// Create the infix expression: left OP right
	infixExpr := &ast.InfixExpression{
		Token:    opToken,
		Left:     target,
		Operator: operator,
		Right:    right,
	}

	// Create the assignment: left = (left OP right)
	assignTok := token.Token{Type: token.ASSIGN, Lexeme: "=", Line: compoundTok.Line, Column: compoundTok.Column}
	return &ast.AssignExpression{
		Token: assignTok,
		Left:  target,
		Value: infixExpr,
	}
}

func (p *Parser) parseAnnotatedExpression(left ast.Expression) ast.Expression {
	// Left is the expression being annotated
	expr := &ast.AnnotatedExpression{
		Token:      p.curToken,
		Expression: left,
	}
	p.nextToken() // Consume ':'
	expr.TypeAnnotation = p.parseType()
	return expr
}

func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	exp := &ast.CallExpression{Token: p.curToken, Function: function}

	// Parse arguments (handling Named Args sugar)
	exp.Arguments = p.parseCallArguments()

	// Handle Block Syntax (Trailing Lambda/List)
	// If followed by { ... }, treat as list of expressions and append as last argument
	if !p.disallowTrailingLambda && p.peekTokenIs(token.LBRACE) {
		// Only if no newline before brace? RFC doesn't specify, but usually yes for trailing blocks.
		// Check for newline
		if !p.peekTokenIs(token.NEWLINE) {
			p.nextToken() // consume {
			blockExprs := p.parseBlockAsList()
			exp.Arguments = append(exp.Arguments, blockExprs)
		}
	}

	return exp
}

// parseCallArguments parses arguments for a function call, handling Named Args sugar
// func(a: 1, b: 2) -> func({a: 1, b: 2})
// func(1, b: 2) -> func(1, {b: 2})
func (p *Parser) parseCallArguments() []ast.Expression {
	args := []ast.Expression{}
	namedArgs := make(map[string]ast.Expression)
	var namedArgsOrder []string
	isNamedMode := false

	// Move past LPAREN
	p.nextToken()

	// Skip leading newlines
	for p.curTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	// Check for empty call )
	if p.curTokenIs(token.RPAREN) {
		return args
	}

	for {
		// Skip newlines before argument
		for p.curTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		// Check if we hit RPAREN (trailing comma case or just newlines)
		if p.curTokenIs(token.RPAREN) {
			break
		}

		// Check if this is a named argument: IDENT : ...
		isNamed := false
		if (p.curTokenIs(token.IDENT_LOWER) || p.curTokenIs(token.IDENT_UPPER)) && p.peekTokenIs(token.COLON) {
			isNamed = true
		}

		if isNamed {
			isNamedMode = true
			key := p.curToken.Literal.(string)
			p.nextToken() // consume key
			p.nextToken() // consume :

			// Skip newlines after :
			for p.curTokenIs(token.NEWLINE) {
				p.nextToken()
			}

			val := p.parseExpression(LOWEST)
			namedArgs[key] = val
			namedArgsOrder = append(namedArgsOrder, key)
		} else {
			if isNamedMode {
				// Error: Positional argument after named argument
				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
					diagnostics.ErrP005,
					p.curToken,
					"positional argument cannot follow named arguments",
				))
				return nil
			}
			expr := p.parseExpression(LOWEST)

			// Handle spread arguments: args...
			if p.peekTokenIs(token.ELLIPSIS) {
				p.nextToken() // consume ...
				expr = &ast.SpreadExpression{Token: p.curToken, Expression: expr}
			}

			args = append(args, expr)
		}

		// Skip newlines before checking for comma
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		// Check for comma
		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // move to comma
			p.nextToken() // move past comma

			// Skip newlines after comma
			for p.curTokenIs(token.NEWLINE) {
				p.nextToken()
			}

			// Check for trailing comma
			if p.curTokenIs(token.RPAREN) {
				goto Done
			}
		} else {
			// If no comma, we expect RPAREN (possibly after newlines)
			break
		}
	}

	// Skip trailing newlines before RPAREN
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

Done:
	// If we collected named args, bundle them into a RecordLiteral
	if len(namedArgs) > 0 {
		rec := &ast.RecordLiteral{
			Token:  token.Token{Type: token.LBRACE, Lexeme: "{", Line: p.curToken.Line, Column: p.curToken.Column},
			Fields: namedArgs,
		}
		args = append(args, rec)
	}

	return args
}

// parseBlockAsList parses { expr1 \n expr2 } as [expr1, expr2]
func (p *Parser) parseBlockAsList() *ast.ListLiteral {
	list := &ast.ListLiteral{
		Token: p.curToken, // The { token
	}
	elements := []ast.Expression{}

	p.nextToken() // consume {

	// Skip leading newlines
	for p.curTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		stmt := p.parseExpressionStatementOrConstDecl()
		if stmt != nil {
			if exprStmt, ok := stmt.(*ast.ExpressionStatement); ok {
				elements = append(elements, exprStmt.Expression)
			} else {
				// Error: Block syntax only supports expressions
				var tok token.Token
				if prov, ok := stmt.(ast.TokenProvider); ok {
					tok = prov.GetToken()
				} else {
					tok = p.curToken
				}

				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
					diagnostics.ErrP005,
					tok,
					"block syntax in arguments only supports expressions",
				))
			}
		}

		p.nextToken()
		// Skip newlines
		for p.curTokenIs(token.NEWLINE) {
			p.nextToken()
		}
	}

	// We are at RBRACE or EOF.
	// If EOF, it's an error (missing }) but we'll return what we have or let expectation fail?
	// The loop terminates on RBRACE. So curToken is RBRACE.

	// Check if we actually ended with RBRACE (loop could end on EOF)
	if p.curTokenIs(token.EOF) {
		// Error: expected }
		return nil
	}

	list.Elements = elements
	return list
}

func (p *Parser) parseExpressionList(end token.TokenType) []ast.Expression {
	list := []ast.Expression{}

	// Skip newlines after opening bracket (for multiline lists)
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	p.nextToken()
	expr := p.parseExpression(LOWEST)
	if expr == nil {
		return nil // Parse error
	}
	if p.peekTokenIs(token.ELLIPSIS) {
		p.nextToken()
		expr = &ast.SpreadExpression{Token: p.curToken, Expression: expr}
	}
	list = append(list, expr)

	// Skip newlines after element
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // consume comma
		// Skip newlines after comma
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}
		// Handle trailing comma
		if p.peekTokenIs(end) {
			p.nextToken()
			return list
		}
		p.nextToken()
		expr := p.parseExpression(LOWEST)
		if expr == nil {
			return nil // Parse error
		}
		if p.peekTokenIs(token.ELLIPSIS) {
			p.nextToken()
			expr = &ast.SpreadExpression{Token: p.curToken, Expression: expr}
		}
		list = append(list, expr)
		// Skip newlines after element
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}
	}

	if !p.expectPeek(end) {
		return nil
	}

	return list
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

func (p *Parser) parseRecordLiteralOrBlock() ast.Expression {
	// Disambiguate Block vs Record

	// 1. Check for {} (Empty Record)
	if p.peekTokenIs(token.RBRACE) {
		rec := p.parseRecordLiteral()
		if rec == nil {
			return nil
		}
		return rec
	}

	// 2. Check for { ...expr } (Record Spread) - this is always a record
	if p.peekTokenIs(token.ELLIPSIS) {
		rec := p.parseRecordLiteral()
		if rec == nil {
			return nil
		}
		return rec
	}

	// 3. Check for { key: val } (Non-empty Record) - single line
	isRecord := false
	if p.peekTokenIs(token.IDENT_LOWER) || p.peekTokenIs(token.IDENT_UPPER) {
		peekNext := p.stream.Peek(1)
		if len(peekNext) >= 1 && peekNext[0].Type == token.COLON {
			isRecord = true
		}
	}

	// 4. Check for multiline record: { \n key: val } or { \n ...expr }
	// But NOT type annotation: { \n var: Type = expr }
	// Type annotation has = after type, record doesn't
	if !isRecord && p.peekTokenIs(token.NEWLINE) {
		peekTokens := p.stream.Peek(50) // tokens AFTER peekToken (which is NEWLINE)
		// Find first non-newline token
		idx := 0
		for idx < len(peekTokens) && peekTokens[idx].Type == token.NEWLINE {
			idx++
		}
		if idx < len(peekTokens) {
			first := peekTokens[idx]
			if first.Type == token.RBRACE {
				// Empty record with newlines: { \n }
				isRecord = true
			} else if first.Type == token.ELLIPSIS {
				// Record spread: { \n ...expr }
				isRecord = true
			} else if first.Type == token.IDENT_LOWER || first.Type == token.IDENT_UPPER {
				// Find colon after ident
				colonIdx := idx + 1
				for colonIdx < len(peekTokens) && peekTokens[colonIdx].Type == token.NEWLINE {
					colonIdx++
				}
				if colonIdx < len(peekTokens) && peekTokens[colonIdx].Type == token.COLON {
					// Found ident: - look for = to detect type annotation
					// Type annotation: var: Type = expr OR var: Type<A, B> = expr
					// Record: key: value (no = before newline/comma/})
					hasAssign := false
					angleBalance := 0
					for checkIdx := colonIdx + 1; checkIdx < len(peekTokens); checkIdx++ {
						tt := peekTokens[checkIdx].Type
						if tt == token.LT {
							angleBalance++
						} else if tt == token.GT {
							angleBalance--
						} else if angleBalance == 0 {
							// Only check for terminators when not inside <...>
							if tt == token.NEWLINE || tt == token.RBRACE {
								break // End of field/statement
							}
							if tt == token.COMMA {
								break // End of record field
							}
							if tt == token.ASSIGN {
								hasAssign = true
								break
							}
						}
					}
					if !hasAssign {
						isRecord = true // No = found, so it's a record field
					}
				}
			}
		}
	}

	if isRecord {
		rec := p.parseRecordLiteral()
		if rec == nil {
			return nil // Return untyped nil for proper nil check
		}
		return rec
	}

	// 4. Default to Block
	return p.parseBlockStatement()
}

func (p *Parser) parseRecordLiteral() *ast.RecordLiteral {
	rl := &ast.RecordLiteral{Token: p.curToken, Fields: make(map[string]ast.Expression)}
	p.nextToken() // consume {

	// Skip leading newlines
	for p.curTokenIs(token.NEWLINE) {
		p.nextToken()
	}

	// Check for spread: { ...expr, ... }
	if p.curTokenIs(token.ELLIPSIS) {
		p.nextToken() // consume ...
		rl.Spread = p.parseExpression(LOWEST)

		// Skip newlines after spread expression
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		// After spread, expect comma or }
		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // consume comma
			p.nextToken() // move to next token
		} else if p.peekTokenIs(token.RBRACE) {
			p.nextToken() // consume }
			return rl
		} else {
			p.nextToken() // move forward
		}

		// Skip newlines
		for p.curTokenIs(token.NEWLINE) {
			p.nextToken()
		}
	}

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.NEWLINE) {
			p.nextToken()
			continue
		}

		if !p.curTokenIs(token.IDENT_LOWER) && !p.curTokenIs(token.IDENT_UPPER) {
			p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(diagnostics.ErrP004, p.curToken, p.curToken.Type))
			return nil // Expected identifier key
		}
		key := p.curToken.Literal.(string)

		if !p.expectPeek(token.COLON) {
			return nil
		}
		p.nextToken() // consume :

		// Skip newlines before value
		for p.curTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		val := p.parseExpression(LOWEST)
		if val == nil {
			return nil // Failed to parse value expression
		}
		rl.Fields[key] = val

		// Skip newlines after value
		for p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}

		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // consume comma
			// Skip newlines after comma
			for p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
			}
		}
		p.nextToken()
	}

	return rl
}

func (p *Parser) parseFunctionLiteral() ast.Expression {
	lit := &ast.FunctionLiteral{Token: p.curToken}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()

	// Optional return type
	if p.peekTokenIs(token.COLON) {
		p.nextToken()
		p.nextToken()
		lit.ReturnType = p.parseType()
	} else if p.peekTokenIs(token.ARROW) {
		// Check if it's a return type or expression body
		// Heuristic: If `->` is followed by Type-like tokens and then `{`, it's a return type.
		// Otherwise, it's an expression body.

		isReturnType := false

		// After nextToken(), stream is at position AFTER peekToken.
		// So Peek(n) returns n tokens starting from token AFTER peekToken.
		// peekToken is ARROW, so Peek(1)[0] is the token after ARROW.

		lookahead := p.stream.Peek(50)
		if len(lookahead) >= 1 {
			tokenAfterArrow := lookahead[0] // Token after ARROW

			if tokenAfterArrow.Type == token.IDENT_UPPER {
				// Case: -> Int { ... } or -> Result<T> { ... }
				if len(lookahead) >= 2 {
					tokenAfterType := lookahead[1]
					if tokenAfterType.Type == token.LBRACE {
						// -> Int { - simple return type
						isReturnType = true
					} else if tokenAfterType.Type == token.LT {
						// -> Result<...> { - generic return type
						// Find matching > and check if { follows
						balance := 0
						for i := 1; i < len(lookahead); i++ {
							t := lookahead[i]
							if t.Type == token.LT {
								balance++
							} else if t.Type == token.GT {
								balance--
								if balance == 0 {
									// Found closing GT. Check next token.
									if i+1 < len(lookahead) && lookahead[i+1].Type == token.LBRACE {
										isReturnType = true
									}
									break
								}
							}
						}
					}
				}
			} else if tokenAfterArrow.Type == token.LPAREN {
				// Case: -> (Int, Int) { ... } or -> () -> Int { ... }
				// Find matching ) and check if { follows
				balance := 0
				for i := 0; i < len(lookahead); i++ {
					t := lookahead[i]
					if t.Type == token.LPAREN {
						balance++
					} else if t.Type == token.RPAREN {
						balance--
						if balance == 0 {
							// Found closing ). Check next token.
							if i+1 < len(lookahead) && lookahead[i+1].Type == token.LBRACE {
								isReturnType = true
							}
							break
						}
					}
				}
			}
		}

		if isReturnType {
			p.nextToken() // consume ARROW, curToken becomes ARROW
			p.nextToken() // move to Start of Type
			lit.ReturnType = p.parseType()
		}
	} else if !p.peekTokenIs(token.ARROW) && !p.peekTokenIs(token.LBRACE) {
		// Try parsing type (implicit start)
		p.nextToken()
		lit.ReturnType = p.parseType()
	}

	// Body: Block or Expression (after `->`)
	if p.peekTokenIs(token.ARROW) {
		p.nextToken() // consume '->'
		p.nextToken() // start of expression

		// If body starts with { and it's a block (not a record literal),
		// parse as block to enable IIFE: fun(x) -> { body }(args)
		// Record literal: { key: value, ... } - has IDENT followed by COLON
		// Block: { statements } - has statement or expression
		isBlock := false
		if p.curTokenIs(token.LBRACE) {
			// Look ahead to distinguish block from record, skipping newlines
			// Lookahead: peekToken, then stream.Peek(n) for tokens after peekToken
			lookahead := p.stream.Peek(10) // get enough tokens to skip newlines

			// Find first non-newline token after {
			firstTokenType := p.peekToken.Type
			firstIdx := -1 // index in lookahead for second token

			if firstTokenType == token.NEWLINE {
				// Skip newlines in peekToken position
				for i, t := range lookahead {
					if t.Type != token.NEWLINE {
						firstTokenType = t.Type
						firstIdx = i + 1 // second token is at firstIdx in lookahead
						break
					}
				}
			} else {
				firstIdx = 0 // second token is at index 0 in lookahead
			}

			// Find second token (after first non-newline), skipping newlines
			secondTokenType := token.EOF
			if firstIdx >= 0 && firstIdx < len(lookahead) {
				for i := firstIdx; i < len(lookahead); i++ {
					if lookahead[i].Type != token.NEWLINE {
						secondTokenType = lookahead[i].Type
						break
					}
				}
			}

			// If { is followed by IDENT_LOWER and then COLON, it's a record
			// If { is followed by ELLIPSIS, it's a spread record
			// If { is followed by }, it could be empty block or empty record
			// Otherwise it's a block
			if firstTokenType == token.IDENT_LOWER && secondTokenType == token.COLON {
				isBlock = false // record literal { key: value }
			} else if firstTokenType == token.ELLIPSIS {
				isBlock = false // spread record { ...x, key: value }
			} else if firstTokenType == token.RBRACE {
				isBlock = true // empty block {}
			} else {
				isBlock = true // block with statements
			}
		}

		if isBlock && p.curTokenIs(token.LBRACE) {
			lit.Body = p.parseBlockStatement()
		} else {
			bodyExpr := p.parseExpression(LOWEST)
			if bodyExpr == nil {
				return nil
			}
			// Wrap expression in BlockStatement
			lit.Body = &ast.BlockStatement{
				Token:      lit.Token,
				Statements: []ast.Statement{&ast.ExpressionStatement{Token: bodyExpr.GetToken(), Expression: bodyExpr}},
			}
		}
	} else if p.peekTokenIs(token.LBRACE) {
		p.nextToken()
		lit.Body = p.parseBlockStatement()
	} else {
		return nil
	}

	return lit
}

// parseLessThanOrTypeApp handles both infix '<' (less than) and Type Application 'expr<Type>'.
func (p *Parser) parseLessThanOrTypeApp(left ast.Expression) ast.Expression {
	// We need to decide if this is 'Left < Right' or 'Left<Type>'.
	// Heuristic: If the token following '<' looks like a Type (Uppercase Identifier), we try to parse it as Type Application.
	// But we need to distinguish `List<Int>` (type app) from `Zero < Some(1)` (comparison).

	isTypeApp := false

	// Check next token.
	// PascalCase usually means Type.
	if p.peekTokenIs(token.IDENT_UPPER) {
		// Look ahead further: if the uppercase identifier is followed by '(' or another operator,
		// it's likely a value (constructor call or comparison), not a type argument.
		// Type arguments are followed by ',' or '>'.
		// E.g., `List<Int>` - Int is followed by >
		// E.g., `Zero < Some(1)` - Some is followed by (
		// E.g., `Map<String, Int>` - String is followed by ,

		// Save current position info
		peekLit := p.peekToken.Literal

		// Speculatively check: parse `< UPPER ...` and see what follows
		// We need to look 2 tokens ahead: after '<' and after the UPPER identifier
		// Unfortunately our parser doesn't have easy lookahead beyond peek.
		//
		// Simple heuristic: if left is a known value constructor, treat < as comparison
		if ident, ok := left.(*ast.Identifier); ok {
			identName := ident.Value
			// Known ADT constructors that are values, not type constructors
			knownValueConstructors := map[string]bool{
				"Zero": true, "Some": true, "Ok": true, "Fail": true,
				"True": true, "False": true,
			}
			if knownValueConstructors[identName] {
				// This is a value constructor, so `<` is comparison
				isTypeApp = false
			} else {
				// Check if this looks like a type name (types are usually capitalized)
				// and the thing after `<` is a type, not a function call
				// For safety, assume it's type application if left is UPPER and peek is UPPER
				// unless we know it's a value constructor
				isTypeApp = true
			}
		} else {
			// left is not a simple identifier - probably an expression
			// so `<` is comparison
			isTypeApp = false
		}

		_ = peekLit // suppress unused warning
	} else if p.peekTokenIs(token.LBRACKET) || p.peekTokenIs(token.LPAREN) {
		// [Int] or (Int, Int) or () -> Int are types.
		// But [1, 2] or (1 + 2) are expressions.
		// Simplification: Only support TApp if it starts with IDENT_UPPER.

		if _, ok := left.(*ast.Identifier); ok {
			if p.peekTokenIs(token.IDENT_UPPER) {
				isTypeApp = true
			}
		} else if _, ok := left.(*ast.MemberExpression); ok {
			if p.peekTokenIs(token.IDENT_UPPER) {
				isTypeApp = true
			}
		}
	}

	if isTypeApp {
		return p.parseTypeApplicationExpression(left)
	}

	// Otherwise, it's infix '<'
	return p.parseInfixExpression(left)
}

func (p *Parser) parseTypeApplicationExpression(left ast.Expression) ast.Expression {
	expr := &ast.TypeApplicationExpression{
		Token:      p.curToken, // The '<' token
		Expression: left,
	}

	p.nextToken() // consume '<'?
	// parseLessThanOrTypeApp is called with curToken = '<'.
	// so we need to consume it to start parsing types?
	// In Pratt, infix functions are called with curToken = Operator.
	// The Loop in parseExpression calls `nextToken` THEN `infix(leftExp)`.
	// So curToken IS `<`.

	// We need to parse types.
	// Since we are at `<`, we advance.
	p.nextToken() // Move to first Type

	for {
		t := p.parseType()
		if t == nil {
			// Error?
			return nil
		}
		expr.TypeArguments = append(expr.TypeArguments, t)

		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // comma
			p.nextToken() // next type
		} else {
			break
		}
	}

	if !p.expectPeek(token.GT) {
		return nil
	}

	return expr
}

func (p *Parser) parsePrefixSpreadExpression() ast.Expression {
	expression := &ast.SpreadExpression{Token: p.curToken}
	p.nextToken() // consume '...'
	expression.Expression = p.parseExpression(PREFIX)
	return expression
}
