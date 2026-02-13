package lexer

import (
	"fmt"
	"math/big"
	"github.com/funvibe/funxy/internal/token"
	"strconv"
	"unicode"
	"unicode/utf8"
)

type Lexer struct {
	input        string
	position     int  // current position in input (points to current char)
	readPosition int  // current reading position in input (after current char)
	ch           rune // current char under examination
	line         int  // current line number
	column       int  // current column number
}

func New(input string) *Lexer {
	l := &Lexer{input: input, line: 1, column: 0}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.ch == '\n' {
		l.line++
		l.column = 0
	}

	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		r, w := utf8.DecodeRuneInString(l.input[l.readPosition:])
		l.ch = r
		l.position = l.readPosition
		l.readPosition += w
		l.column++
		return
	}

	l.position = l.readPosition
	l.readPosition++
	l.column++
}

func (l *Lexer) NextToken() token.Token {
	var tok token.Token

	l.skipWhitespace()

	switch l.ch {
	case '\n':
		tok = newToken(token.NEWLINE, l.ch, l.line, l.column)
	case '=':
		// =, ==, =>
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.EQ, Lexeme: "==", Literal: "==", Line: l.line, Column: l.column}
		} else if l.peekChar() == '>' {
			l.readChar()
			tok = token.Token{Type: token.USER_OP_IMPLY, Lexeme: "=>", Literal: "=>", Line: l.line, Column: l.column}
		} else {
			tok = newToken(token.ASSIGN, l.ch, l.line, l.column)
		}
	case '+':
		if l.peekChar() == '+' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.CONCAT, Lexeme: literal, Literal: literal, Line: l.line, Column: l.column}
		} else if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.PLUS_ASSIGN, Lexeme: "+=", Literal: "+=", Line: l.line, Column: l.column}
		} else {
			tok = newToken(token.PLUS, l.ch, l.line, l.column)
		}
	case '-':
		if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.ARROW, Lexeme: literal, Literal: literal, Line: l.line, Column: l.column}
		} else if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.MINUS_ASSIGN, Lexeme: "-=", Literal: "-=", Line: l.line, Column: l.column}
		} else {
			tok = newToken(token.MINUS, l.ch, l.line, l.column)
		}
	case '/':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.SLASH_ASSIGN, Lexeme: "/=", Literal: "/=", Line: l.line, Column: l.column}
		} else {
			tok = newToken(token.SLASH, l.ch, l.line, l.column)
		}
	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.NOT_EQ, Lexeme: literal, Literal: literal, Line: l.line, Column: l.column}
		} else {
			tok = newToken(token.BANG, l.ch, l.line, l.column)
		}
	case '*':
		if l.peekChar() == '*' {
			l.readChar()
			if l.peekChar() == '=' {
				l.readChar()
				tok = token.Token{Type: token.POWER_ASSIGN, Lexeme: "**=", Literal: "**=", Line: l.line, Column: l.column}
			} else {
				tok = token.Token{Type: token.POWER, Lexeme: "**", Literal: "**", Line: l.line, Column: l.column}
			}
		} else if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.ASTERISK_ASSIGN, Lexeme: "*=", Literal: "*=", Line: l.line, Column: l.column}
		} else {
			tok = newToken(token.ASTERISK, l.ch, l.line, l.column)
		}
	case '%':
		if l.peekChar() == '"' {
			startLine, startCol := l.line, l.column
			l.readChar() // consume %, now at "
			content, _ := l.readStringWithInterpolation()
			tok = token.Token{Type: token.FORMAT_STRING, Lexeme: fmt.Sprintf("%%%q", content), Literal: content, Line: startLine, Column: startCol}
		} else if l.peekChar() == '{' {
			l.readChar()
			tok = token.Token{Type: token.PERCENT_LBRACE, Lexeme: "%{", Literal: "%{", Line: l.line, Column: l.column}
		} else if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.PERCENT_ASSIGN, Lexeme: "%=", Literal: "%=", Line: l.line, Column: l.column}
		} else {
			tok = newToken(token.PERCENT, l.ch, l.line, l.column)
		}
	case '.':
		if l.peekChar() == '.' {
			l.readChar() // .
			if l.peekChar() == '.' {
				l.readChar() // .
				literal := "..."
				tok = token.Token{Type: token.ELLIPSIS, Lexeme: literal, Literal: literal, Line: l.line, Column: l.column}
			} else {
				// Two dots: Range operator ..
				literal := ".."
				tok = token.Token{Type: token.DOT_DOT, Lexeme: literal, Literal: literal, Line: l.line, Column: l.column}
			}
		} else {
			tok = newToken(token.DOT, l.ch, l.line, l.column)
		}
	case '<':
		// <, <=, <<, <>, <|>, <*>, <$>, <:>, <~>, <-
		if l.peekChar() == '-' {
			l.readChar()
			tok = token.Token{Type: token.L_ARROW, Lexeme: "<-", Literal: "<-", Line: l.line, Column: l.column}
		} else if l.peekChar() == '<' {
			l.readChar()
			tok = token.Token{Type: token.LSHIFT, Lexeme: "<<", Literal: "<<", Line: l.line, Column: l.column}
		} else if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.LTE, Lexeme: "<=", Literal: "<=", Line: l.line, Column: l.column}
		} else if l.peekChar() == '>' {
			l.readChar()
			tok = token.Token{Type: token.USER_OP_COMBINE, Lexeme: "<>", Literal: "<>", Line: l.line, Column: l.column}
		} else if l.peekChar() == '|' {
			l.readChar()
			if l.peekChar() == '>' {
				l.readChar()
				tok = token.Token{Type: token.USER_OP_CHOOSE, Lexeme: "<|>", Literal: "<|>", Line: l.line, Column: l.column}
			} else {
				// <| operator (UserOpPipeLeft)
				tok = token.Token{Type: token.USER_OP_PIPE_LEFT, Lexeme: "<|", Literal: "<|", Line: l.line, Column: l.column}
			}
		} else if l.peekChar() == '*' {
			l.readChar()
			if l.peekChar() == '>' {
				l.readChar()
				tok = token.Token{Type: token.USER_OP_APPLY, Lexeme: "<*>", Literal: "<*>", Line: l.line, Column: l.column}
			} else {
				tok = newToken(token.ILLEGAL, l.ch, l.line, l.column)
			}
		} else if l.peekChar() == '$' {
			l.readChar()
			if l.peekChar() == '>' {
				l.readChar()
				tok = token.Token{Type: token.USER_OP_MAP, Lexeme: "<$>", Literal: "<$>", Line: l.line, Column: l.column}
			} else {
				tok = newToken(token.ILLEGAL, l.ch, l.line, l.column)
			}
		} else if l.peekChar() == ':' {
			l.readChar()
			if l.peekChar() == '>' {
				l.readChar()
				tok = token.Token{Type: token.USER_OP_CONS, Lexeme: "<:>", Literal: "<:>", Line: l.line, Column: l.column}
			} else {
				tok = newToken(token.ILLEGAL, l.ch, l.line, l.column)
			}
		} else if l.peekChar() == '~' {
			l.readChar()
			if l.peekChar() == '>' {
				l.readChar()
				tok = token.Token{Type: token.USER_OP_SWAP, Lexeme: "<~>", Literal: "<~>", Line: l.line, Column: l.column}
			} else {
				tok = newToken(token.ILLEGAL, l.ch, l.line, l.column)
			}
		} else {
			tok = newToken(token.LT, l.ch, l.line, l.column)
		}
	case '>':
		// >, >=, >>, >>=
		if l.peekChar() == '>' {
			l.readChar()
			if l.peekChar() == '=' {
				l.readChar()
				tok = token.Token{Type: token.USER_OP_BIND, Lexeme: ">>=", Literal: ">>=", Line: l.line, Column: l.column}
			} else {
				tok = token.Token{Type: token.RSHIFT, Lexeme: ">>", Literal: ">>", Line: l.line, Column: l.column}
			}
		} else if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.GTE, Lexeme: ">=", Literal: ">=", Line: l.line, Column: l.column}
		} else {
			tok = newToken(token.GT, l.ch, l.line, l.column)
		}
	case '(':
		tok = newToken(token.LPAREN, l.ch, l.line, l.column)
	case ')':
		tok = newToken(token.RPAREN, l.ch, l.line, l.column)
	case ',':
		if l.peekChar() == ',' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.COMPOSE, Lexeme: literal, Literal: literal, Line: l.line, Column: l.column}
		} else {
			tok = newToken(token.COMMA, l.ch, l.line, l.column)
		}
	case ':':
		if l.peekChar() == '-' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.COLON_MINUS, Lexeme: literal, Literal: literal, Line: l.line, Column: l.column}
		} else if l.peekChar() == ':' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.CONS, Lexeme: literal, Literal: literal, Line: l.line, Column: l.column}
		} else {
			tok = newToken(token.COLON, l.ch, l.line, l.column)
		}
	case '|':
		if l.peekChar() == '|' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.OR, Lexeme: literal, Literal: literal, Line: l.line, Column: l.column}
		} else if l.peekChar() == '>' {
			l.readChar()
			if l.peekChar() == '>' {
				l.readChar()
				tok = token.Token{Type: token.PIPE_GT_UNWRAP, Lexeme: "|>>", Literal: "|>>", Line: l.line, Column: l.column}
			} else {
				tok = token.Token{Type: token.PIPE_GT, Lexeme: "|>", Literal: "|>", Line: l.line, Column: l.column}
			}
		} else {
			tok = newToken(token.PIPE, l.ch, l.line, l.column)
		}
	case '&':
		if l.peekChar() == '&' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.AND, Lexeme: literal, Literal: literal, Line: l.line, Column: l.column}
		} else {
			tok = newToken(token.AMPERSAND, l.ch, l.line, l.column)
		}
	case '^':
		tok = newToken(token.CARET, l.ch, l.line, l.column)
	case '~':
		tok = newToken(token.TILDE, l.ch, l.line, l.column)
	case '?':
		if l.peekChar() == '?' {
			l.readChar()
			tok = token.Token{Type: token.NULL_COALESCE, Lexeme: "??", Literal: "??", Line: l.line, Column: l.column}
		} else if l.peekChar() == '.' {
			l.readChar()
			tok = token.Token{Type: token.OPTIONAL_CHAIN, Lexeme: "?.", Literal: "?.", Line: l.line, Column: l.column}
		} else {
			tok = newToken(token.QUESTION, l.ch, l.line, l.column)
		}
	case '\\':
		tok = newToken(token.BACKSLASH, l.ch, l.line, l.column)
	case '@':
		// @"...", @x"...", @b"..." - Bytes literals
		if l.peekChar() == '"' {
			// @"..." - UTF-8 bytes literal
			l.readChar() // consume @, now at "
			content := l.readString()
			tok = token.Token{Type: token.BYTES_STRING, Lexeme: fmt.Sprintf("@%q", content), Literal: content, Line: l.line, Column: l.column}
		} else if l.peekChar() == 'x' {
			// Check for @x"..."
			l.readChar() // consume @, now at x
			if l.peekChar() == '"' {
				l.readChar() // consume x, now at "
				content := l.readString()
				tok = token.Token{Type: token.BYTES_HEX, Lexeme: fmt.Sprintf("@x%q", content), Literal: content, Line: l.line, Column: l.column}
			} else {
				// @x without " is illegal
				tok = newToken(token.ILLEGAL, l.ch, l.line, l.column)
			}
		} else if l.peekChar() == 'b' {
			// Check for @b"..."
			l.readChar() // consume @, now at b
			if l.peekChar() == '"' {
				l.readChar() // consume b, now at "
				content := l.readString()
				tok = token.Token{Type: token.BYTES_BIN, Lexeme: fmt.Sprintf("@b%q", content), Literal: content, Line: l.line, Column: l.column}
			} else {
				// @b without " is illegal
				tok = newToken(token.ILLEGAL, l.ch, l.line, l.column)
			}
		} else {
			// @ without valid suffix is illegal
			tok = newToken(token.ILLEGAL, l.ch, l.line, l.column)
		}
	case '#':
		// #b"...", #x"...", #o"..." - Bits literals
		if l.peekChar() == 'b' {
			// Check for #b"..."
			l.readChar() // consume #, now at b
			if l.peekChar() == '"' {
				l.readChar() // consume b, now at "
				content := l.readString()
				tok = token.Token{Type: token.BITS_BIN, Lexeme: fmt.Sprintf("#b%q", content), Literal: content, Line: l.line, Column: l.column}
			} else {
				// #b without " is illegal
				tok = newToken(token.ILLEGAL, l.ch, l.line, l.column)
			}
		} else if l.peekChar() == 'x' {
			// Check for #x"..."
			l.readChar() // consume #, now at x
			if l.peekChar() == '"' {
				l.readChar() // consume x, now at "
				content := l.readString()
				tok = token.Token{Type: token.BITS_HEX, Lexeme: fmt.Sprintf("#x%q", content), Literal: content, Line: l.line, Column: l.column}
			} else {
				// #x without " is illegal
				tok = newToken(token.ILLEGAL, l.ch, l.line, l.column)
			}
		} else if l.peekChar() == 'o' {
			// Check for #o"..."
			l.readChar() // consume #, now at o
			if l.peekChar() == '"' {
				l.readChar() // consume o, now at "
				content := l.readString()
				tok = token.Token{Type: token.BITS_OCT, Lexeme: fmt.Sprintf("#o%q", content), Literal: content, Line: l.line, Column: l.column}
			} else {
				// #o without " is illegal
				tok = newToken(token.ILLEGAL, l.ch, l.line, l.column)
			}
		} else {
			// # without valid suffix is illegal
			tok = newToken(token.ILLEGAL, l.ch, l.line, l.column)
		}
	case '$':
		tok = token.Token{Type: token.USER_OP_APP, Lexeme: "$", Literal: "$", Line: l.line, Column: l.column}
	case '{':
		tok = newToken(token.LBRACE, l.ch, l.line, l.column)
	case '}':
		tok = newToken(token.RBRACE, l.ch, l.line, l.column)
	case '[':
		tok = newToken(token.LBRACKET, l.ch, l.line, l.column)
	case ']':
		tok = newToken(token.RBRACKET, l.ch, l.line, l.column)
	case '"':
		startLine, startCol := l.line, l.column
		content, hasInterp := l.readStringWithInterpolation()
		if hasInterp {
			tok.Type = token.INTERP_STRING
		} else {
			tok.Type = token.STRING
		}
		tok.Literal = content
		tok.Lexeme = fmt.Sprintf("%q", content)
		tok.Line = startLine
		tok.Column = startCol
	case '`':
		// Check for triple backticks ```
		if l.peekChar() == '`' {
			peek2 := l.peekChar2()
			if peek2 == '`' {
				// Triple backticks - read until closing triple backticks
				startLine, startCol := l.line, l.column
				tok.Type = token.STRING
				tok.Literal = l.readTripleRawString()
				tok.Lexeme = fmt.Sprintf("```%s```", tok.Literal)
				tok.Line = startLine
				tok.Column = startCol
				// Don't call readChar() here - readTripleRawString() already consumed everything
				return tok
			} else {
				// Single backtick
				startLine, startCol := l.line, l.column
				tok.Type = token.STRING
				tok.Literal = l.readRawString()
				tok.Lexeme = fmt.Sprintf("`%s`", tok.Literal)
				tok.Line = startLine
				tok.Column = startCol
			}
		} else {
			// Single backtick
			startLine, startCol := l.line, l.column
			tok.Type = token.STRING
			tok.Literal = l.readRawString()
			tok.Lexeme = fmt.Sprintf("`%s`", tok.Literal)
			tok.Line = startLine
			tok.Column = startCol
		}
	case '\'':
		startLine, startCol := l.line, l.column
		val, err := l.readCharLiteral()
		if err != nil {
			tok.Type = token.ILLEGAL
			tok.Literal = err.Error()
			end := l.readPosition
			if end > len(l.input) {
				end = len(l.input)
			}
			if l.position > end {
				tok.Lexeme = ""
			} else {
				tok.Lexeme = l.input[l.position:end] // Approximate
			}
		} else {
			tok.Type = token.CHAR
			tok.Literal = val
			tok.Lexeme = fmt.Sprintf("'%c'", val)
		}
		tok.Line = startLine
		tok.Column = startCol
	case 0:
		tok.Lexeme = ""
		tok.Type = token.EOF
		tok.Line = l.line
		tok.Column = l.column
	default:
		if isLetter(l.ch) {
			startLine, startCol := l.line, l.column
			lexeme := l.readIdentifier()
			tok.Lexeme = lexeme
			tok.Type = l.determineIdentifierType(lexeme)
			tok.Literal = lexeme
			tok.Line = startLine
			tok.Column = startCol
			return tok
		} else if isDigit(l.ch) {
			return l.readNumber()
		} else {
			if l.ch == 0 {
				tok = newToken(token.EOF, 0, l.line, l.column)
			} else {
				tok = newToken(token.ILLEGAL, l.ch, l.line, l.column)
			}
		}
	}

	l.readChar()
	return tok
}

func (l *Lexer) readString() string {
	position := l.position + 1
	for {
		l.readChar()
		if l.ch == '"' || l.ch == 0 {
			break
		}
	}
	return l.input[position:l.position]
}

// readStringWithInterpolation reads a string and detects ${...} interpolations.
// Returns the processed content (with escape sequences resolved) and true if interpolations were found.
func (l *Lexer) readStringWithInterpolation() (string, bool) {
	var result []byte
	hasInterp := false
	// Stack of expected closing delimiters:
	// '}' for code blocks
	// '"', '\'', '`' for strings/chars
	// -1 for triple backticks
	var stack []rune
	buf := make([]byte, 4)

	for {
		l.readChar()
		if l.ch == 0 {
			break
		}

		inInterpolation := len(stack) > 0

		// End of Top-Level String
		if !inInterpolation && l.ch == '"' {
			break
		}

		// --- State Machine Logic ---

		// 1. Inside a Quote (String, Char, RawString) - deeper than top level
		// We check the top of the stack.
		if len(stack) > 0 {
			top := stack[len(stack)-1]

			if top == '"' || top == '\'' || top == '`' || top == -1 {
				// Handle Triple Backtick Closing
				if top == -1 {
					if l.ch == '`' && l.peekChar() == '`' && l.peekChar2() == '`' {
						stack = stack[:len(stack)-1]
						n := utf8.EncodeRune(buf, l.ch)
						result = append(result, buf[:n]...)
						l.readChar() // 2nd
						n = utf8.EncodeRune(buf, l.ch)
						result = append(result, buf[:n]...)
						l.readChar() // 3rd
						n = utf8.EncodeRune(buf, l.ch)
						result = append(result, buf[:n]...)
						continue
					}
					// Just append char in raw string
					n := utf8.EncodeRune(buf, l.ch)
					result = append(result, buf[:n]...)
					continue
				}

				if l.ch == top {
					// Found closing quote
					stack = stack[:len(stack)-1]
					n := utf8.EncodeRune(buf, l.ch)
					result = append(result, buf[:n]...)
					continue
				}

				// Raw strings (backticks) don't process escapes or interpolation
				if top == '`' {
					n := utf8.EncodeRune(buf, l.ch)
					result = append(result, buf[:n]...)
					continue
				}

				// Handle escapes in " and '
				if l.ch == '\\' {
					n := utf8.EncodeRune(buf, l.ch)
					result = append(result, buf[:n]...) // Append backslash

					l.readChar() // Consume escaped char
					if l.ch == 0 {
						break
					}
					n = utf8.EncodeRune(buf, l.ch)
					result = append(result, buf[:n]...) // Append escaped char
					continue
				}

				// Handle Interpolation inside Double Quotes
				if top == '"' && l.ch == '$' && l.peekChar() == '{' {
					stack = append(stack, '}') // Push expectation of closing brace
					n := utf8.EncodeRune(buf, l.ch)
					result = append(result, buf[:n]...) // $
					l.readChar()                        // {
					n = utf8.EncodeRune(buf, l.ch)
					result = append(result, buf[:n]...) // {
					continue
				}

				// Regular char inside quote
				n := utf8.EncodeRune(buf, l.ch)
				result = append(result, buf[:n]...)
				continue
			}
		}

		// 2. Inside Code Block ${ ... } or { ... }
		// Top of stack is '}'
		if len(stack) > 0 && stack[len(stack)-1] == '}' {
			if l.ch == '}' {
				// Closing brace
				stack = stack[:len(stack)-1]
				n := utf8.EncodeRune(buf, l.ch)
				result = append(result, buf[:n]...)
				continue
			}

			if l.ch == '{' {
				// Nested brace
				stack = append(stack, '}')
				n := utf8.EncodeRune(buf, l.ch)
				result = append(result, buf[:n]...)
				continue
			}

			// Start of String/Char/RawString
			if l.ch == '"' || l.ch == '\'' {
				stack = append(stack, l.ch)
				n := utf8.EncodeRune(buf, l.ch)
				result = append(result, buf[:n]...)
				continue
			}

			if l.ch == '`' {
				isTriple := false
				if l.peekChar() == '`' && l.peekChar2() == '`' {
					isTriple = true
				}

				n := utf8.EncodeRune(buf, l.ch)
				result = append(result, buf[:n]...)

				if isTriple {
					l.readChar() // 2nd `
					n = utf8.EncodeRune(buf, l.ch)
					result = append(result, buf[:n]...)
					l.readChar() // 3rd `
					n = utf8.EncodeRune(buf, l.ch)
					result = append(result, buf[:n]...)
					stack = append(stack, -1) // Marker for triple backtick
				} else {
					stack = append(stack, '`')
				}
				continue
			}

			// Handle Comments (// and /* */) to avoid matching braces inside them
			if l.ch == '/' {
				peek := l.peekChar()
				if peek == '/' {
					// Line comment
					n := utf8.EncodeRune(buf, l.ch)
					result = append(result, buf[:n]...)
					l.readChar() // /
					n = utf8.EncodeRune(buf, l.ch)
					result = append(result, buf[:n]...)

					for l.ch != '\n' && l.ch != 0 {
						l.readChar()
						n = utf8.EncodeRune(buf, l.ch)
						result = append(result, buf[:n]...)
					}
					continue
				} else if peek == '*' {
					// Block comment
					n := utf8.EncodeRune(buf, l.ch)
					result = append(result, buf[:n]...)
					l.readChar() // *
					n = utf8.EncodeRune(buf, l.ch)
					result = append(result, buf[:n]...)

					for l.ch != 0 {
						if l.ch == '*' && l.peekChar() == '/' {
							l.readChar() // *
							n = utf8.EncodeRune(buf, l.ch)
							result = append(result, buf[:n]...)
							l.readChar() // /
							n = utf8.EncodeRune(buf, l.ch)
							result = append(result, buf[:n]...)
							break
						}
						l.readChar()
						n = utf8.EncodeRune(buf, l.ch)
						result = append(result, buf[:n]...)
					}
					continue
				}
			}

			// Regular char in code
			n := utf8.EncodeRune(buf, l.ch)
			result = append(result, buf[:n]...)
			continue
		}

		// 3. Top-Level String Content (Stack Empty)
		// Process escapes and detect interpolation
		if l.ch == '$' && l.peekChar() == '{' {
			hasInterp = true
			stack = append(stack, '}') // Push code scope
			result = append(result, '$')
			l.readChar() // {
			result = append(result, '{')
			continue
		}

		if l.ch == '\\' {
			l.readChar() // consume backslash
			// Process escapes for the resulting string literal
			// Note: We keep the escape logic from original function for top-level
			switch l.ch {
			case 'n':
				result = append(result, '\n')
			case 't':
				result = append(result, '\t')
			case 'r':
				result = append(result, '\r')
			case '0':
				result = append(result, 0)
			case '\\':
				result = append(result, '\\')
			case '"':
				result = append(result, '"')
			case '$':
				result = append(result, '$')
			case 'u':
				val, ok := l.readHexEscape(4)
				if ok {
					n := utf8.EncodeRune(buf, rune(val))
					result = append(result, buf[:n]...)
				} else {
					// Invalid escape - keep raw
					result = append(result, '\\', 'u')
				}
			case 'U':
				val, ok := l.readHexEscape(8)
				if ok {
					n := utf8.EncodeRune(buf, rune(val))
					result = append(result, buf[:n]...)
				} else {
					// Invalid escape - keep raw
					result = append(result, '\\', 'U')
				}
			default:
				// Unknown escape - keep both
				result = append(result, '\\')
				n := utf8.EncodeRune(buf, l.ch)
				result = append(result, buf[:n]...)
			}
			continue
		}

		n := utf8.EncodeRune(buf, l.ch)
		result = append(result, buf[:n]...)
	}

	return string(result), hasInterp
}

func (l *Lexer) readHexEscape(n int) (int64, bool) {
	var val int64
	for i := 0; i < n; i++ {
		l.readChar()
		var d int64
		if l.ch >= '0' && l.ch <= '9' {
			d = int64(l.ch - '0')
		} else if l.ch >= 'a' && l.ch <= 'f' {
			d = int64(l.ch - 'a' + 10)
		} else if l.ch >= 'A' && l.ch <= 'F' {
			d = int64(l.ch - 'A' + 10)
		} else {
			return 0, false
		}
		val = val*16 + d
	}
	return val, true
}

// readRawString reads a backtick-delimited raw string that can span multiple lines.
// No escape sequences are processed - content is taken as-is.
func (l *Lexer) readRawString() string {
	position := l.position + 1
	for {
		l.readChar()
		if l.ch == '`' || l.ch == 0 {
			break
		}
		// Note: l.readChar() already handles line counting for '\n'
	}
	return l.input[position:l.position]
}

// readTripleRawString reads a triple backtick-delimited raw string that can span multiple lines.
// No escape sequences are processed - content is taken as-is.
// When this function returns, curToken will be positioned on the first backtick of the closing triple backticks.
func (l *Lexer) readTripleRawString() string {
	// Skip the opening triple backticks
	l.readChar() // consume second `
	l.readChar() // consume third `
	position := l.position
	for {
		l.readChar()
		if l.ch == 0 {
			break
		}
		if l.ch == '`' {
			// Check if next two chars are also backticks
			if l.peekChar() == '`' && l.peekChar2() == '`' {
				// Found closing triple backticks - stop here, don't consume them yet
				// l.position points to the first backtick of the closing triple
				break
			}
		}
		// Note: l.readChar() already handles line counting for '\n'
	}
	result := l.input[position:l.position]
	// Consume the closing triple backticks (NextToken will call readChar() which consumes the first `)
	l.readChar() // consume first closing `
	l.readChar() // consume second closing `
	l.readChar() // consume third closing `
	return result
}

func (l *Lexer) readCharLiteral() (int64, error) {
	l.readChar() // skip opening '
	if l.ch == '\'' {
		return 0, fmt.Errorf("empty character literal")
	}

	var char int64
	if l.ch == '\\' {
		// Escape sequence
		l.readChar() // consume backslash
		switch l.ch {
		case 'n':
			char = '\n'
		case 't':
			char = '\t'
		case 'r':
			char = '\r'
		case '0':
			char = 0
		case '\\':
			char = '\\'
		case '\'':
			char = '\''
		case 'u':
			val, ok := l.readHexEscape(4)
			if ok {
				char = val
			} else {
				return 0, fmt.Errorf("invalid unicode escape sequence \\uXXXX")
			}
		case 'U':
			val, ok := l.readHexEscape(8)
			if ok {
				char = val
			} else {
				return 0, fmt.Errorf("invalid unicode escape sequence \\UXXXXXXXX")
			}
		default:
			// Unknown escape, just use the char after backslash
			char = int64(l.ch)
		}
		l.readChar() // consume escaped char
	} else {
		// Read rune
		char = int64(l.ch)
		l.readChar() // consume the char
	}
	// Expect closing '
	if l.ch != '\'' {
		// Strict check: character literal must be closed with '
		return 0, fmt.Errorf("unterminated character literal, expected '")
	}
	// readChar called by NextToken will consume closing ' if we are there.
	return char, nil
}

func (l *Lexer) readIdentifier() string {
	position := l.position
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

func (l *Lexer) determineIdentifierType(ident string) token.TokenType {
	if len(ident) == 0 {
		return token.ILLEGAL
	}

	firstChar := ident[0]
	if 'A' <= firstChar && firstChar <= 'Z' {
		return token.IDENT_UPPER
	}

	// If it's lowercase, check if it's a keyword
	return token.LookupIdent(ident)
}

func (l *Lexer) readNumber() token.Token {
	startLine, startCol := l.line, l.column
	position := l.position
	base := 10
	isFloat := false

	// Check for base prefixes: 0x, 0b, 0o
	if l.ch == '0' {
		peek := l.peekChar()
		if peek == 'x' || peek == 'X' {
			l.readChar()
			l.readChar()
			base = 16
		} else if peek == 'b' || peek == 'B' {
			l.readChar()
			l.readChar()
			base = 2
		} else if peek == 'o' || peek == 'O' {
			l.readChar()
			l.readChar()
			base = 8
		}
	}

	// Read digits
	for {
		if base == 16 {
			if !isHexDigit(l.ch) {
				break
			}
		} else {
			if !isDigit(l.ch) {
				break
			}
		}
		l.readChar()
	}

	// Check for float dot (only if base 10)
	if base == 10 && l.ch == '.' && isDigit(l.peekChar()) {
		isFloat = true
		l.readChar() // .
		for isDigit(l.ch) {
			l.readChar()
		}
	}

	// Check suffixes
	isBigInt := false
	isRational := false

	if l.ch == 'n' {
		isBigInt = true
		l.readChar()
	} else if l.ch == 'r' {
		isRational = true
		l.readChar()
	}

	lexeme := l.input[position:l.position]
	literalText := lexeme

	// Validate and Parse
	if isBigInt {
		if isFloat {
			return token.Token{Type: token.ILLEGAL, Lexeme: lexeme, Literal: "BigInt cannot have decimal point", Line: startLine, Column: startCol}
		}
		if base != 10 && base != 16 && base != 2 && base != 8 { // Should be covered by logic
		}

		// Remove 'n' suffix
		literalText = lexeme[:len(lexeme)-1]

		val := new(big.Int)
		// SetString(s, 0) auto-detects base 0x, 0b, 0o
		if _, ok := val.SetString(literalText, 0); !ok {
			return token.Token{Type: token.ILLEGAL, Lexeme: lexeme, Literal: "Invalid BigInt", Line: startLine, Column: startCol}
		}
		return token.Token{Type: token.BIG_INT, Lexeme: lexeme, Literal: val, Line: startLine, Column: startCol}
	}

	if isRational {
		if base != 10 {
			return token.Token{Type: token.ILLEGAL, Lexeme: lexeme, Literal: "Rational must be base 10", Line: startLine, Column: startCol}
		}

		// Remove 'r' suffix
		literalText = lexeme[:len(lexeme)-1]

		val := new(big.Rat)
		if _, ok := val.SetString(literalText); !ok {
			return token.Token{Type: token.ILLEGAL, Lexeme: lexeme, Literal: "Invalid Rational", Line: startLine, Column: startCol}
		}
		return token.Token{Type: token.RATIONAL, Lexeme: lexeme, Literal: val, Line: startLine, Column: startCol}
	}

	// Regular Int or Float
	if isFloat {
		val, err := strconv.ParseFloat(literalText, 64)
		if err != nil {
			return token.Token{Type: token.ILLEGAL, Lexeme: lexeme, Literal: err.Error(), Line: startLine, Column: startCol}
		}
		return token.Token{Type: token.FLOAT, Lexeme: lexeme, Literal: val, Line: startLine, Column: startCol}
	} else {
		// Regular int (int64)
		// strconv.ParseInt(s, 0, 64) auto-detects base
		val, err := strconv.ParseInt(literalText, 0, 64)
		if err != nil {
			return token.Token{Type: token.ILLEGAL, Lexeme: lexeme, Literal: "Integer overflow (use 'n' suffix for BigInt)", Line: startLine, Column: startCol}
		}
		return token.Token{Type: token.INT, Lexeme: lexeme, Literal: val, Line: startLine, Column: startCol}
	}
}

func isHexDigit(ch rune) bool {
	return isDigit(ch) || ('a' <= ch && ch <= 'f') || ('A' <= ch && ch <= 'F')
}

func isLetter(ch rune) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_' || (ch >= 0x80 && unicode.IsLetter(ch))
}

func isDigit(ch rune) bool {
	return '0' <= ch && ch <= '9'
}

func (l *Lexer) peekChar() rune {
	if l.readPosition >= len(l.input) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.input[l.readPosition:])
	return r
}

func (l *Lexer) peekChar2() rune {
	if l.readPosition >= len(l.input) {
		return 0
	}
	_, w := utf8.DecodeRuneInString(l.input[l.readPosition:])
	pos2 := l.readPosition + w
	if pos2 >= len(l.input) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.input[pos2:])
	return r
}

func newToken(tokenType token.TokenType, ch rune, line, col int) token.Token {
	literal := string(ch)
	return token.Token{Type: tokenType, Lexeme: literal, Literal: literal, Line: line, Column: col}
}

func (l *Lexer) skipWhitespace() {
	for {
		for l.ch == ' ' || l.ch == '\t' || l.ch == '\r' {
			l.readChar()
		}
		// Handle comments
		if l.ch == '/' {
			if l.peekChar() == '/' {
				l.readChar() // consume first /
				l.readChar() // consume second /
				for l.ch != '\n' && l.ch != 0 {
					l.readChar()
				}
				continue
			} else if l.peekChar() == '*' {
				l.readChar() // consume /
				l.readChar() // consume *
				for l.ch != 0 {
					if l.ch == '*' && l.peekChar() == '/' {
						l.readChar() // consume *
						l.readChar() // consume /
						break
					}
					l.readChar()
				}
				continue
			}
		}
		break
	}
}
