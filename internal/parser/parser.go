package parser

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/token"
)

// Parser holds the state of our parser.
type Parser struct {
	stream    pipeline.TokenStream
	curToken  token.Token
	peekToken token.Token
	ctx       *pipeline.PipelineContext

	prefixParseFns map[token.TokenType]prefixParseFn
	infixParseFns  map[token.TokenType]infixParseFn

	// splitRshift tracks when we've consumed one > from >>
	// When true, the next nextToken() call will return > instead of reading from stream
	splitRshift bool

	// disallowTrailingLambda allows disabling trailing lambda syntax in contexts like if/match conditions
	disallowTrailingLambda bool
}

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

// Precedence constants
const (
	LOWEST           = iota
	USER_OP_APP_PREC // $ (lowest precedence operator, but above LOWEST)
	PIPE_PREC        // |>
	LOGIC_OR         // ||
	LOGIC_AND        // &&
	EQUALS           // ==
	LESSGREATER      // > or <
	BITWISE_OR       // | ^
	BITWISE_AND      // &
	RANGE            // ..
	SHIFT            // << >>
	SUM              // +
	PRODUCT          // *
	POWER            // **
	PREFIX           // -X or !X
	POSTFIX          // X?
	CALL             // myFunction(X)
	INDEX            // array[index]
	ANNOTATION       // x: Int
)

var precedences = map[token.TokenType]int{
	token.USER_OP_APP:     USER_OP_APP_PREC, // $ (lowest, function application)
	token.PIPE_GT:         PIPE_PREC,
	token.OR:              LOGIC_OR,
	token.NULL_COALESCE:   LOGIC_OR, // ?? same precedence as ||
	token.AND:             LOGIC_AND,
	token.EQ:              EQUALS,
	token.NOT_EQ:          EQUALS,
	token.LT:              LESSGREATER,
	token.GT:              LESSGREATER,
	token.LTE:             LESSGREATER,
	token.GTE:             LESSGREATER,
	token.PIPE:            BITWISE_OR,
	token.CARET:           BITWISE_OR,
	token.AMPERSAND:       BITWISE_AND,
	token.DOT_DOT:         RANGE,
	token.LSHIFT:          SHIFT,
	token.RSHIFT:          SHIFT,
	token.PLUS:            SUM,
	token.MINUS:           SUM,
	token.CONCAT:          SUM, // ++ same as +
	token.CONS:            SUM, // :: same level (right-associative handled in parseInfix)
	token.SLASH:           PRODUCT,
	token.ASTERISK:        PRODUCT,
	token.PERCENT:         PRODUCT,
	token.POWER:           POWER,
	token.COMPOSE:         POWER, // ,, composition (right-to-left, high precedence like Haskell's .)
	token.LPAREN:          CALL,
	token.ASSIGN:          EQUALS, // Assignment has low precedence? Usually lowest. But here we treat as expr.
	token.PLUS_ASSIGN:     EQUALS, // Compound assignment operators
	token.MINUS_ASSIGN:    EQUALS,
	token.ASTERISK_ASSIGN: EQUALS,
	token.SLASH_ASSIGN:    EQUALS,
	token.PERCENT_ASSIGN:  EQUALS,
	token.POWER_ASSIGN:    EQUALS,
	token.COLON:           ANNOTATION,
	token.LBRACKET:        INDEX,
	token.QUESTION:        POSTFIX,
	token.DOT:             CALL,
	token.OPTIONAL_CHAIN:  CALL, // ?. has same precedence as .
	// User-definable operators are registered dynamically in init()
}

func init() {
	// Register user-definable operators from centralized config
	for _, op := range config.UserOperators {
		tokenType := token.TokenType(op.Symbol)
		var prec int
		switch op.Precedence {
		case config.PrecLowest:
			prec = USER_OP_APP_PREC
		case config.PrecLow:
			prec = PIPE_PREC
		case config.PrecMedium:
			prec = SUM
		case config.PrecHigh:
			prec = PRODUCT
		}
		precedences[tokenType] = prec
	}
}

func New(stream pipeline.TokenStream, ctx *pipeline.PipelineContext) *Parser {
	p := &Parser{
		stream: stream,
		ctx:    ctx,
	}

	p.prefixParseFns = make(map[token.TokenType]prefixParseFn)
	p.registerPrefix(token.IDENT_LOWER, p.parseIdentifier)
	p.registerPrefix(token.IDENT_UPPER, p.parseIdentifier)
	p.registerPrefix(token.UNDERSCORE, p.parseUnderscore)
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.FLOAT, p.parseFloatLiteral)
	p.registerPrefix(token.BIG_INT, p.parseBigIntLiteral)
	p.registerPrefix(token.RATIONAL, p.parseRationalLiteral)
	p.registerPrefix(token.BANG, p.parsePrefixExpression)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)
	p.registerPrefix(token.TILDE, p.parsePrefixExpression)
	p.registerPrefix(token.LPAREN, p.parseGroupedExpression)
	p.registerPrefix(token.LBRACE, p.parseRecordLiteralOrBlock) // Start of block or record literal
	p.registerPrefix(token.TRUE, p.parseBoolean)
	p.registerPrefix(token.FALSE, p.parseBoolean)
	p.registerPrefix(token.NIL, p.parseNil)
	p.registerPrefix(token.IF, p.parseIfExpression)
	p.registerPrefix(token.MATCH, p.parseMatchExpression)
	p.registerPrefix(token.STRING, p.parseStringLiteral)
	p.registerPrefix(token.FORMAT_STRING, p.parseFormatStringLiteral)
	p.registerPrefix(token.INTERP_STRING, p.parseInterpolatedString)
	p.registerPrefix(token.CHAR, p.parseCharLiteral)
	p.registerPrefix(token.BYTES_STRING, p.parseBytesLiteral)
	p.registerPrefix(token.BYTES_HEX, p.parseBytesLiteral)
	p.registerPrefix(token.BYTES_BIN, p.parseBytesLiteral)
	p.registerPrefix(token.BITS_BIN, p.parseBitsLiteral)
	p.registerPrefix(token.BITS_HEX, p.parseBitsLiteral)
	p.registerPrefix(token.BITS_OCT, p.parseBitsLiteral)
	p.registerPrefix(token.LBRACKET, p.parseListLiteral)
	p.registerPrefix(token.PERCENT_LBRACE, p.parseMapLiteral)
	p.registerPrefix(token.FUN, p.parseFunctionLiteral)
	p.registerPrefix(token.BACKSLASH, p.parseLambdaExpression)
	p.registerPrefix(token.FOR, p.parseForExpression) // for loop
	p.registerPrefix(token.ELLIPSIS, p.parsePrefixSpreadExpression)
	p.registerPrefix(token.DO, p.parseDoExpression)
	p.registerPrefix(token.LPAREN, p.parseGroupedExpression) // Re-register explicitly to be safe

	p.infixParseFns = make(map[token.TokenType]infixParseFn)
	p.registerInfix(token.PLUS, p.parseInfixExpression)
	p.registerInfix(token.MINUS, p.parseInfixExpression)
	p.registerInfix(token.SLASH, p.parseInfixExpression)
	p.registerInfix(token.ASTERISK, p.parseInfixExpression)
	p.registerInfix(token.PERCENT, p.parseInfixExpression)
	p.registerInfix(token.POWER, p.parseRightAssocInfixExpression)
	p.registerInfix(token.AMPERSAND, p.parseInfixExpression)
	p.registerInfix(token.DOT_DOT, p.parseRangeExpression)
	p.registerInfix(token.PIPE, p.parseInfixExpression)
	p.registerInfix(token.CARET, p.parseInfixExpression)
	p.registerInfix(token.LSHIFT, p.parseInfixExpression)
	p.registerInfix(token.RSHIFT, p.parseInfixExpression)
	p.registerInfix(token.AND, p.parseInfixExpression)
	p.registerInfix(token.OR, p.parseInfixExpression)
	p.registerInfix(token.NULL_COALESCE, p.parseInfixExpression)
	p.registerInfix(token.PIPE_GT, p.parseInfixExpression)
	p.registerInfix(token.USER_OP_APP, p.parseRightAssocInfixExpression) // $ right-associative
	p.registerInfix(token.CONCAT, p.parseInfixExpression)
	p.registerInfix(token.CONS, p.parseRightAssocInfixExpression)    // Right-associative
	p.registerInfix(token.COMPOSE, p.parseRightAssocInfixExpression) // Right-associative composition
	p.registerInfix(token.EQ, p.parseInfixExpression)
	p.registerInfix(token.NOT_EQ, p.parseInfixExpression)
	p.registerInfix(token.LT, p.parseLessThanOrTypeApp) // Changed from parseInfixExpression
	p.registerInfix(token.GT, p.parseInfixExpression)
	p.registerInfix(token.LTE, p.parseInfixExpression)
	p.registerInfix(token.GTE, p.parseInfixExpression)
	p.registerInfix(token.ASSIGN, p.parseAssignExpression)
	p.registerInfix(token.PLUS_ASSIGN, p.parseCompoundAssignExpression)
	p.registerInfix(token.MINUS_ASSIGN, p.parseCompoundAssignExpression)
	p.registerInfix(token.ASTERISK_ASSIGN, p.parseCompoundAssignExpression)
	p.registerInfix(token.SLASH_ASSIGN, p.parseCompoundAssignExpression)
	p.registerInfix(token.PERCENT_ASSIGN, p.parseCompoundAssignExpression)
	p.registerInfix(token.POWER_ASSIGN, p.parseCompoundAssignExpression)
	p.registerInfix(token.LPAREN, p.parseCallExpression)
	p.registerInfix(token.COLON, p.parseAnnotatedExpression)
	p.registerInfix(token.LBRACKET, p.parseIndexExpression)
	p.registerInfix(token.QUESTION, p.parsePostfixExpression)
	p.registerInfix(token.DOT, p.parseMemberExpression)
	p.registerInfix(token.OPTIONAL_CHAIN, p.parseOptionalChainExpression)

	// User-definable operators - registered from centralized config
	for _, op := range config.UserOperators {
		tokenType := token.TokenType(op.Symbol)
		if op.Assoc == config.AssocRight {
			p.registerInfix(tokenType, p.parseRightAssocInfixExpression)
		} else {
			p.registerInfix(tokenType, p.parseInfixExpression)
		}
	}

	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) nextToken() {
	// Handle split >> case: after consuming one > from >>, inject a synthetic >
	if p.splitRshift {
		p.splitRshift = false
		// Create synthetic > token at the same position
		p.curToken = token.Token{
			Type:    token.GT,
			Lexeme:  ">",
			Literal: ">",
			Line:    p.curToken.Line,
			Column:  p.curToken.Column + 1,
		}
		// peekToken remains unchanged (it was already peeked)
		return
	}

	p.curToken = p.peekToken
	peekResult := p.stream.Peek(1)
	if len(peekResult) > 0 {
		p.peekToken = peekResult[0]
	} else {
		p.peekToken = token.Token{Type: token.EOF}
	}
	p.stream.Next()
}

// parseExpressionStatementOrConstDecl parses an expression statement OR a constant declaration
// kVAL :- 123
// kVAL : Int :- 123
// (a, b) :- pair
func (p *Parser) parseExpressionStatementOrConstDecl() ast.Statement {
	expr := p.parseExpression(LOWEST)
	if expr == nil {
		return nil
	}

	if p.peekTokenIs(token.COLON_MINUS) {
		p.nextToken() // consume last token of expr
		p.nextToken() // consume :-

		// validate LHS - can be identifier, annotated identifier, or tuple pattern
		var name *ast.Identifier
		var pattern ast.Pattern
		var typeAnnot ast.Type

		if ident, ok := expr.(*ast.Identifier); ok {
			if ident.Token.Type == token.IDENT_UPPER {
				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
					diagnostics.ErrP006,
					ident.Token,
					"Constant name must start with a lowercase letter",
				))
			}
			name = ident
		} else if anno, ok := expr.(*ast.AnnotatedExpression); ok {
			if ident, ok := anno.Expression.(*ast.Identifier); ok {
				if ident.Token.Type == token.IDENT_UPPER {
					p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
						diagnostics.ErrP006,
						ident.Token,
						"Constant name must start with a lowercase letter",
					))
				}
				name = ident
				typeAnnot = anno.TypeAnnotation
			} else {
				// Error: LHS of constant decl must be identifier or pattern
				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(diagnostics.ErrP001, expr.GetToken(), "identifier or pattern", expr.GetToken().Type))
				return nil
			}
		} else if tuple, ok := expr.(*ast.TupleLiteral); ok {
			// Convert tuple literal to tuple pattern
			pattern = p.tupleExprToPattern(tuple)
			if pattern == nil {
				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(diagnostics.ErrP006, expr.GetToken(), "invalid pattern in tuple destructuring"))
				return nil
			}
		} else if list, ok := expr.(*ast.ListLiteral); ok {
			// Convert list literal to list pattern
			pattern = p.listExprToPattern(list)
			if pattern == nil {
				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(diagnostics.ErrP006, expr.GetToken(), "invalid pattern in list destructuring"))
				return nil
			}
		} else if rec, ok := expr.(*ast.RecordLiteral); ok {
			// Convert record literal to record pattern
			pattern = p.recordExprToPattern(rec)
			if pattern == nil {
				p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(diagnostics.ErrP006, expr.GetToken(), "invalid pattern in record destructuring"))
				return nil
			}
		} else {
			p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(diagnostics.ErrP001, expr.GetToken(), "identifier or pattern", expr.GetToken().Type))
			return nil
		}

		val := p.parseExpression(LOWEST)

		return &ast.ConstantDeclaration{
			Token:          expr.GetToken(),
			Name:           name,
			Pattern:        pattern,
			TypeAnnotation: typeAnnot,
			Value:          val,
		}
	}

	return &ast.ExpressionStatement{Token: expr.GetToken(), Expression: expr}
}

// tupleExprToPattern converts a TupleLiteral expression to a TuplePattern
func (p *Parser) tupleExprToPattern(tuple *ast.TupleLiteral) ast.Pattern {
	elements := make([]ast.Pattern, len(tuple.Elements))
	for i, elem := range tuple.Elements {
		pat := p.exprToPattern(elem)
		if pat == nil {
			return nil
		}
		elements[i] = pat
	}
	return &ast.TuplePattern{Token: tuple.Token, Elements: elements}
}

// listExprToPattern converts a ListLiteral expression to a ListPattern
func (p *Parser) listExprToPattern(list *ast.ListLiteral) ast.Pattern {
	elements := make([]ast.Pattern, len(list.Elements))
	for i, elem := range list.Elements {
		pat := p.exprToPattern(elem)
		if pat == nil {
			return nil
		}
		elements[i] = pat
	}
	return &ast.ListPattern{Token: list.Token, Elements: elements}
}

// recordExprToPattern converts a RecordLiteral expression to a RecordPattern
func (p *Parser) recordExprToPattern(rec *ast.RecordLiteral) ast.Pattern {
	fields := make(map[string]ast.Pattern)
	for key, val := range rec.Fields {
		pat := p.exprToPattern(val)
		if pat == nil {
			return nil
		}
		fields[key] = pat
	}
	return &ast.RecordPattern{Token: rec.Token, Fields: fields}
}

// exprToPattern converts an expression to a pattern (for destructuring)
func (p *Parser) exprToPattern(expr ast.Expression) ast.Pattern {
	switch e := expr.(type) {
	case *ast.Identifier:
		if e.Value == "_" {
			return &ast.WildcardPattern{Token: e.Token}
		}
		return &ast.IdentifierPattern{Token: e.Token, Value: e.Value}
	case *ast.TupleLiteral:
		return p.tupleExprToPattern(e)
	case *ast.ListLiteral:
		return p.listExprToPattern(e)
	case *ast.RecordLiteral:
		return p.recordExprToPattern(e)
	default:
		return nil
	}
}

// parseDirectiveStatement parses a directive statement.
// directive "name"
func (p *Parser) parseDirectiveStatement() *ast.DirectiveStatement {
	stmt := &ast.DirectiveStatement{Token: p.curToken}

	if !p.expectPeek(token.STRING) {
		return nil
	}

	stmt.Name = p.curToken.Literal.(string)
	p.nextToken() // consume string

	return stmt
}

func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	program.Statements = []ast.Statement{}

	// Skip leading newlines (allows comments at start of file)
	for p.curToken.Type == token.NEWLINE {
		p.nextToken()
	}

	// 1. Check for Package Declaration
	if p.curToken.Type == token.PACKAGE {
		pkgDecl := p.parsePackageDeclaration()
		if pkgDecl != nil {
			program.Statements = append(program.Statements, pkgDecl)
		}
		p.nextToken() // consume end of package decl
		// Consume newlines
		for p.curToken.Type == token.NEWLINE {
			p.nextToken()
		}
	}

	// 2. Check for Imports
	for p.curToken.Type == token.IMPORT {
		imp := p.parseImportStatement()
		if imp != nil {
			program.Imports = append(program.Imports, imp)
			program.Statements = append(program.Statements, imp)
		}
		p.nextToken()
		// Consume newlines
		for p.curToken.Type == token.NEWLINE {
			p.nextToken()
		}
	}

	// 2.5 Check for Directives (must be top-level, after imports)
	for p.curToken.Type == token.DIRECTIVE {
		dir := p.parseDirectiveStatement()
		if dir != nil {
			program.Statements = append(program.Statements, dir)
		}
		// Directive is followed by string then newline
		// parseDirectiveStatement consumes string and nextToken calls Next().
		// We expect newline after.
		if p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}
		p.nextToken()
		// Consume newlines
		for p.curToken.Type == token.NEWLINE {
			p.nextToken()
		}
	}

	// 3. Parse Statements
	for p.curToken.Type != token.EOF {
		if p.curToken.Type == token.NEWLINE {
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
		} else if p.curToken.Type == token.FUN && (p.peekTokenIs(token.IDENT_LOWER) || p.peekTokenIs(token.IDENT_UPPER) || p.peekTokenIs(token.LT) || p.peekTokenIs(token.LPAREN)) {
			// Function declaration (named)
			// Or generic function fun foo<T>(...)
			// Or extension method fun (recv) foo(...)

			// Disambiguate fun (recv) vs fun (params) -> expr (literal)
			isExtension := false
			if p.peekTokenIs(token.LPAREN) {
				// It's extension only if followed by identifier (MethodName)
				tokens := p.stream.Peek(50) // Peek ahead enough tokens
				balance := 1                // Initialize balance to 1 (accounting for peekToken which is LPAREN)
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
					// If next token is IDENT, it is likely Extension Method Name: fun (recv) Name
					// If next token is LBRACE, ARROW, or COLON (ret type of lambda?), it is Literal.
					if nextToken.Type == token.IDENT_LOWER || nextToken.Type == token.IDENT_UPPER {
						isExtension = true
					}
				}
			} else if p.peekTokenIs(token.IDENT_UPPER) {
				// Invalid function name starting with uppercase, but treat as function statement to report correct error
				isExtension = true
			} else {
				// Normal function `fun Name` or `fun <T> Name`
				isExtension = true
			}

			if isExtension {
				fnStmt := p.parseFunctionStatement()
				if fnStmt != nil {
					stmt = fnStmt
				}
				if p.peekTokenIs(token.NEWLINE) {
					p.nextToken()
				}
				p.nextToken()
			} else {
				// It's a function literal expression statement (e.g. `fun() {}`)
				stmt = p.parseExpressionStatement()
				p.nextToken()
				// Newline handling same as default
			}
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
			p.nextToken() // consume break or value
		} else if p.curToken.Type == token.CONTINUE {
			stmt = p.parseContinueStatement()
			p.nextToken() // consume continue
		} else if p.curToken.Type == token.PACKAGE || p.curToken.Type == token.IMPORT {
			// Error: package/import must be at top
			p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
				diagnostics.ErrP006, // Or generic syntax error
				p.curToken,
				"package or import declaration must be at the top of the file",
			))
			p.nextToken()
			// consume line
			for !p.curTokenIs(token.NEWLINE) && !p.curTokenIs(token.EOF) {
				p.nextToken()
			}
		} else {
			// Check for Constant Declaration: kVAL :- 123 or kVAL : Type :- 123
			// This is tricky because `kVAL` is an expression start (Identifier).
			// But `kVAL :-` is a statement.
			// We can peek ahead.
			// Case 1: IDENT COLON_MINUS ...
			// Case 2: IDENT COLON ... (could be annotation expr `x: Int` OR const decl `x: Int :- ...`)

			// Peek for :-
			if p.curToken.Type == token.IDENT_LOWER && p.peekTokenIs(token.COLON_MINUS) {
				// Direct match: kVAL :- ...
				name := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal.(string)}
				stmt = p.parseConstantDeclaration(name)
				p.nextToken()
			} else {
				// Try parsing expression.
				// If expression parses successfully, check if next token is `:-`.
				// This handles `x : Int :- ...` because `x : Int` is `AnnotatedExpression`.

				// However, `parseExpressionStatement` calls `parseExpression` and consumes tokens.
				// We can't easily backtrack.

				// Option: Add `:-` as an infix operator (CONST_ASSIGN) with precedence like ASSIGN.
				// Then `x :- 5` parses as `AssignExpression` (or new type) inside `parseExpression`.
				// Then we convert it to `ConstantDeclaration` statement.
				// But `x: Int :- 5`? `(x: Int) :- 5`.
				// `AnnotatedExpression` binds tightly? `COLON` precedence is ANNOTATION.
				// If `:-` has precedence ASSIGN (LOWEST), then `x: Int` binds first. Correct.

				// So, plan:
				// 1. Register `COLON_MINUS` as infix operator in Parser.
				// 2. Parse it as `ConstAssignExpression` (new AST node? or reuse ConstantDeclaration as Expr?).
				// 3. In `parseExpressionStatement`, if result is `ConstAssignExpression`, return it (wrapped).
				// 4. But `ConstantDeclaration` is a Statement.

				// Let's register `:-` in `infixParseFns` to call `p.parseConstantDefinitionExpr`.
				// It will return `ConstantDeclaration`? No, `parseExpression` expects `Expression`.
				// So `ConstantDeclaration` must implement `Expression` interface?
				// Or we return a temporary Expression wrapper.

				// Simpler: In `parseExpressionStatement`:
				// Parse expr.
				// If next token is `:-`, it's a constant decl with LHS expr.
				// Check if LHS is Identifier or AnnotatedExpression.
				// If so, convert to ConstantDeclaration.

				stmt = p.parseExpressionStatementOrConstDecl()
				p.nextToken()
			}

			if p.curToken.Type == token.NEWLINE {
				// p.nextToken() // Loop handles skipping
			}
		}

		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}

		// Handle comma as statement separator (multiple statements on one line)
		if p.curToken.Type == token.COMMA {
			p.nextToken() // consume comma
			// Skip any newlines after comma
			for p.curToken.Type == token.NEWLINE {
				p.nextToken()
			}
			continue
		}

		if p.curToken.Type == token.EOF {
			break
		}
	}
	return program
}

func (p *Parser) curTokenIs(t token.TokenType) bool {
	return p.curToken.Type == t
}

// isOperatorToken checks if current token is an operator that can be used as a function
func (p *Parser) isOperatorToken() bool {
	switch p.curToken.Type {
	case token.PLUS, token.MINUS, token.ASTERISK, token.SLASH, token.PERCENT, token.POWER,
		token.EQ, token.NOT_EQ, token.LT, token.GT, token.LTE, token.GTE,
		token.AMPERSAND, token.PIPE, token.CARET, token.LSHIFT, token.RSHIFT,
		token.CONCAT, token.CONS, token.AND, token.OR, token.NULL_COALESCE:
		return true
	}
	// Check user-definable operators from config
	return config.IsUserOperator(string(p.curToken.Type))
}

func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	} else {
		p.peekError(t)
		return false
	}
}

func (p *Parser) peekError(t token.TokenType) {
	p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
		diagnostics.ErrP005,
		p.peekToken,
		t,
		p.peekToken.Type,
	))
}

func (p *Parser) noPrefixParseFnError(t token.TokenType) {
	p.ctx.Errors = append(p.ctx.Errors, diagnostics.NewError(
		diagnostics.ErrP004,
		p.curToken,
		t,
	))
}

func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) parseRangeExpression(left ast.Expression) ast.Expression {
	tok := p.curToken
	precedence := p.curPrecedence()
	p.nextToken() // consume ..

	// Parse end expression
	end := p.parseExpression(precedence)

	var start, next ast.Expression
	start = left

	// Check for (start, next)..end syntax
	// We only support literal tuples for this syntactic sugar
	if tuple, ok := left.(*ast.TupleLiteral); ok && len(tuple.Elements) == 2 {
		start = tuple.Elements[0]
		next = tuple.Elements[1]
	}

	return &ast.RangeExpression{
		Token: tok,
		Start: start,
		Next:  next,
		End:   end,
	}
}

func (p *Parser) registerPrefix(tokenType token.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

func (p *Parser) registerInfix(tokenType token.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}
