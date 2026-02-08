package token

import "fmt"

type TokenType string

// Token represents a lexical token.
type Token struct {
	Type    TokenType
	Lexeme  string
	Line    int
	Column  int
	Literal interface{}
}

func (t Token) String() string {
	return fmt.Sprintf("Line %d:%d, Type: %s, Lexeme: '%s'", t.Line, t.Column, t.Type, t.Lexeme)
}

// List of token types
const (
	// Special tokens
	ILLEGAL TokenType = "ILLEGAL"
	EOF     TokenType = "EOF"
	NEWLINE TokenType = "NEWLINE"

	// Operators
	ASSIGN   TokenType = "="
	PLUS     TokenType = "+"
	MINUS    TokenType = "-"
	BANG     TokenType = "!"
	ASTERISK TokenType = "*"
	SLASH    TokenType = "/"
	POWER    TokenType = "**"
	PERCENT  TokenType = "%"

	// Compound assignment operators
	PLUS_ASSIGN     TokenType = "+="
	MINUS_ASSIGN    TokenType = "-="
	ASTERISK_ASSIGN TokenType = "*="
	SLASH_ASSIGN    TokenType = "/="
	PERCENT_ASSIGN  TokenType = "%="
	POWER_ASSIGN    TokenType = "**="

	// Bitwise
	AMPERSAND TokenType = "&"
	PIPE      TokenType = "|"
	CARET     TokenType = "^"
	TILDE     TokenType = "~"
	LSHIFT    TokenType = "<<"
	RSHIFT    TokenType = ">>"

	ELLIPSIS       TokenType = "..."
	QUESTION       TokenType = "?"
	NULL_COALESCE  TokenType = "??" // Optional null coalescing
	OPTIONAL_CHAIN TokenType = "?." // Optional chaining
	CONCAT         TokenType = "++" // List/String concatenation
	CONS           TokenType = "::" // Cons (prepend to list)
	PIPE_GT        TokenType = "|>" // Pipe operator
	COMPOSE        TokenType = ",," // Function composition (right-to-left)

	// User-definable operators (fixed slots)
	USER_OP_COMBINE   TokenType = "<>"  // UserOpCombine trait
	USER_OP_CHOOSE    TokenType = "<|>" // UserOpChoose trait
	USER_OP_APPLY     TokenType = "<*>" // UserOpApply trait
	USER_OP_BIND      TokenType = ">>=" // UserOpBind trait (Monad bind)
	USER_OP_MAP       TokenType = "<$>" // UserOpMap trait
	USER_OP_CONS      TokenType = "<:>" // UserOpCons trait
	USER_OP_SWAP      TokenType = "<~>" // UserOpSwap trait
	USER_OP_IMPLY     TokenType = "=>"  // UserOpImply trait
	USER_OP_APP       TokenType = "$"   // Function application (built-in)
	USER_OP_PIPE_LEFT TokenType = "<|"  // UserOpPipeLeft trait

	LT  TokenType = "<"
	GT  TokenType = ">"
	LTE TokenType = "<="
	GTE TokenType = ">="

	EQ     TokenType = "=="
	NOT_EQ TokenType = "!="
	AND    TokenType = "&&"
	OR     TokenType = "||"

	// Delimiters
	LPAREN         TokenType = "("
	RPAREN         TokenType = ")"
	LBRACE         TokenType = "{"
	RBRACE         TokenType = "}"
	LBRACKET       TokenType = "["
	RBRACKET       TokenType = "]"
	PERCENT_LBRACE TokenType = "%{" // Map literal start
	COMMA          TokenType = ","
	COLON_MINUS    TokenType = ":-"
	COLON          TokenType = ":"
	DOT            TokenType = "."
	DOT_DOT        TokenType = ".."

	// Keywords
	TYPE      TokenType = "TYPE"
	ALIAS     TokenType = "ALIAS"
	IF        TokenType = "IF"
	ELSE      TokenType = "ELSE"
	TRUE      TokenType = "TRUE"
	FALSE     TokenType = "FALSE"
	NIL       TokenType = "NIL"
	MATCH     TokenType = "MATCH"
	FUN       TokenType = "FUN"
	OPERATOR  TokenType = "OPERATOR" // Operator method in trait
	TRAIT     TokenType = "TRAIT"    // Type Class definition
	INSTANCE  TokenType = "INSTANCE" // Type Class implementation
	FOR       TokenType = "FOR"
	IN        TokenType = "IN"
	BREAK     TokenType = "BREAK"
	CONTINUE  TokenType = "CONTINUE"
	RETURN    TokenType = "RETURN"
	PACKAGE   TokenType = "PACKAGE"
	IMPORT    TokenType = "IMPORT"
	DO        TokenType = "DO"     // do notation
	CONST     TokenType = "CONST"  // const keyword
	FORALL    TokenType = "FORALL" // forall a. T
	DIRECTIVE TokenType = "DIRECTIVE"

	// Special symbols
	ARROW      TokenType = "->"
	UNDERSCORE TokenType = "_"
	BACKSLASH  TokenType = "\\"
	L_ARROW    TokenType = "<-"

	// Identifiers & literals
	IDENT_UPPER   TokenType = "IDENT_UPPER" // List, Triangle, Int
	IDENT_LOWER   TokenType = "IDENT_LOWER" // a, myVar
	INT           TokenType = "INT"
	FLOAT         TokenType = "FLOAT"
	BIG_INT       TokenType = "BIG_INT"  // 100n
	RATIONAL      TokenType = "RATIONAL" // 12.34r
	STRING        TokenType = "STRING"
	INTERP_STRING TokenType = "INTERP_STRING" // String with ${...} interpolations
	FORMAT_STRING TokenType = "FORMAT_STRING" // %".2f" - Format string literal
	CHAR          TokenType = "CHAR"

	// Bytes literals
	BYTES_STRING TokenType = "BYTES_STRING" // @"hello" - UTF-8 bytes
	BYTES_HEX    TokenType = "BYTES_HEX"    // @x"48656C6C6F" - hex bytes
	BYTES_BIN    TokenType = "BYTES_BIN"    // @b"01001000" - binary bytes

	// Bits literals
	BITS_BIN TokenType = "BITS_BIN" // #b"10101010" - binary bits
	BITS_HEX TokenType = "BITS_HEX" // #x"FF" - hex bits
	BITS_OCT TokenType = "BITS_OCT" // #o"377" - octal bits
)

var Keywords = map[string]TokenType{
	"type":      TYPE,
	"alias":     ALIAS,
	"if":        IF,
	"else":      ELSE,
	"true":      TRUE,
	"false":     FALSE,
	"nil":       NIL,
	"match":     MATCH,
	"fun":       FUN,
	"operator":  OPERATOR,
	"trait":     TRAIT,
	"instance":  INSTANCE,
	"for":       FOR,
	"in":        IN,
	"break":     BREAK,
	"continue":  CONTINUE,
	"return":    RETURN,
	"package":   PACKAGE,
	"import":    IMPORT,
	"_":         UNDERSCORE,
	"while":     FOR,
	"do":        DO,
	"const":     CONST,
	"forall":    FORALL,
	"directive": DIRECTIVE,
}

// LookupIdent checks the keywords table to see whether the given identifier
// is in fact a keyword.
func LookupIdent(ident string) TokenType {
	if tok, ok := Keywords[ident]; ok {
		return tok
	}
	return IDENT_LOWER // By default, lowercase identifiers are just identifiers
}
