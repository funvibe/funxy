package ast

import (
	"math/big"
	"github.com/funvibe/funxy/internal/token"
	"github.com/funvibe/funxy/internal/typesystem"
)

// TokenProvider is an interface for any AST node that can provide its primary token.
// This is useful for error reporting.
type TokenProvider interface {
	GetToken() token.Token
}

// Node is the base interface for all AST nodes.
type Node interface {
	TokenLiteral() string
	Accept(v Visitor)
}

// Statement is a Node that represents a statement.
type Statement interface {
	Node
	statementNode()
	GetToken() token.Token
}

// Expression is a Node that represents an expression.
type Expression interface {
	Node
	expressionNode()
	GetToken() token.Token
}

// DirectiveStatement represents a compiler directive.
// directive "name"
type DirectiveStatement struct {
	Token token.Token // The 'directive' token
	Name  string      // The directive name (e.g., "strict_types")
}

func (ds *DirectiveStatement) Accept(v Visitor)     { v.VisitDirectiveStatement(ds) }
func (ds *DirectiveStatement) statementNode()       {}
func (ds *DirectiveStatement) TokenLiteral() string { return ds.Token.Lexeme }
func (ds *DirectiveStatement) GetToken() token.Token {
	if ds == nil {
		return token.Token{}
	}
	return ds.Token
}

// Program is the root node of every AST our parser produces.
type Program struct {
	File       string // Source file path
	Package    *PackageDeclaration
	Imports    []*ImportStatement
	Statements []Statement
}

func (p *Program) Accept(v Visitor) { v.VisitProgram(p) }
func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	} else {
		return ""
	}
}

// ConstantDeclaration represents a constant binding.
// kVAL :- 123 or kVAL : Int :- 123 or (a, b) :- pair
type ConstantDeclaration struct {
	Token          token.Token // The identifier token or the colon-minus? Let's use first token.
	Name           *Identifier // Simple binding: x :- 1
	Pattern        Pattern     // Pattern binding: (a, b) :- pair (mutually exclusive with Name)
	TypeAnnotation Type        // Optional
	Value          Expression
}

func (cd *ConstantDeclaration) Accept(v Visitor)     { v.VisitConstantDeclaration(cd) }
func (cd *ConstantDeclaration) statementNode()       {}
func (cd *ConstantDeclaration) TokenLiteral() string { return cd.Token.Lexeme }
func (cd *ConstantDeclaration) GetToken() token.Token {
	if cd == nil {
		return token.Token{}
	}
	return cd.Token
}

// PackageDeclaration represents a package declaration at the top of a file.
// package my_package (ExportedSymbol1, ExportedSymbol2)
// ExportSpec represents a single export specification in package declaration.
// Can be either a local symbol or a module re-export.
type ExportSpec struct {
	Token       token.Token   // Token for error reporting
	Symbol      *Identifier   // For local exports: the symbol name (e.g., localFun)
	ModuleName  *Identifier   // For re-exports: module name/alias (e.g., shapes)
	Symbols     []*Identifier // For re-exports: specific symbols (e.g., Circle, Square)
	ReexportAll bool          // For re-exports: true if (*) is used
}

func (es *ExportSpec) GetToken() token.Token {
	if es == nil {
		return token.Token{}
	}
	return es.Token
}

// IsReexport returns true if this is a module re-export (not a local symbol export)
func (es *ExportSpec) IsReexport() bool {
	return es.ModuleName != nil
}

type PackageDeclaration struct {
	Token     token.Token // The 'package' token
	Name      *Identifier
	Exports   []*ExportSpec // List of export specifications
	ExportAll bool          // True if '*' is used for local exports
}

func (pd *PackageDeclaration) Accept(v Visitor)     { v.VisitPackageDeclaration(pd) }
func (pd *PackageDeclaration) statementNode()       {}
func (pd *PackageDeclaration) TokenLiteral() string { return pd.Token.Lexeme }
func (pd *PackageDeclaration) GetToken() token.Token {
	if pd == nil {
		return token.Token{}
	}
	return pd.Token
}

// ImportStatement represents an import declaration.
// import "path/to/module" [as alias]
type ImportStatement struct {
	Token     token.Token // The 'import' token
	Path      *StringLiteral
	Alias     *Identifier   // Optional alias for the imported package
	Symbols   []*Identifier // Specific symbols to import: (a, b, c)
	Exclude   []*Identifier // Symbols to exclude: !(a, b, c)
	ImportAll bool          // (*) import all
}

func (is *ImportStatement) Accept(v Visitor)     { v.VisitImportStatement(is) }
func (is *ImportStatement) statementNode()       {}
func (is *ImportStatement) TokenLiteral() string { return is.Token.Lexeme }
func (is *ImportStatement) GetToken() token.Token {
	if is == nil {
		return token.Token{}
	}
	return is.Token
}

// Identifier represents an identifier, e.g., a variable name.
type Identifier struct {
	Token          token.Token // the token.IDENT token
	Value          string
	TypeVarMapping map[string]typesystem.Type // Mapping from generic var name to fresh var (e.g. "T" -> "t1")
	Constraints    []*TypeConstraint          // Constraints for implicit type params (e.g. List<t: Show>)
	Kind           typesystem.Kind            // Explicit kind annotation (e.g., t: * -> *)
}

func (i *Identifier) Accept(v Visitor)     { v.VisitIdentifier(i) }
func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Lexeme }
func (i *Identifier) GetToken() token.Token {
	if i == nil {
		return token.Token{}
	}
	return i.Token
}

// IntegerLiteral represents an integer literal.
type IntegerLiteral struct {
	Token token.Token
	Value int64
}

func (il *IntegerLiteral) Accept(v Visitor)     { v.VisitIntegerLiteral(il) }
func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Lexeme }
func (il *IntegerLiteral) GetToken() token.Token {
	if il == nil {
		return token.Token{}
	}
	return il.Token
}

// BooleanLiteral represents boolean literals true/false.
type BooleanLiteral struct {
	Token token.Token
	Value bool
}

func (b *BooleanLiteral) Accept(v Visitor)     { v.VisitBooleanLiteral(b) }
func (b *BooleanLiteral) expressionNode()      {}
func (b *BooleanLiteral) TokenLiteral() string { return b.Token.Lexeme }
func (b *BooleanLiteral) GetToken() token.Token {
	if b == nil {
		return token.Token{}
	}
	return b.Token
}

// NilLiteral represents the nil literal (the only value of type Nil).
type NilLiteral struct {
	Token token.Token
}

func (n *NilLiteral) Accept(v Visitor)     { v.VisitNilLiteral(n) }
func (n *NilLiteral) expressionNode()      {}
func (n *NilLiteral) TokenLiteral() string { return n.Token.Lexeme }
func (n *NilLiteral) GetToken() token.Token {
	if n == nil {
		return token.Token{}
	}
	return n.Token
}

// FloatLiteral represents a floating point literal.
type FloatLiteral struct {
	Token token.Token
	Value float64
}

func (fl *FloatLiteral) Accept(v Visitor)     { v.VisitFloatLiteral(fl) }
func (fl *FloatLiteral) expressionNode()      {}
func (fl *FloatLiteral) TokenLiteral() string { return fl.Token.Lexeme }
func (fl *FloatLiteral) GetToken() token.Token {
	if fl == nil {
		return token.Token{}
	}
	return fl.Token
}

// BigIntLiteral represents a BigInt literal.
type BigIntLiteral struct {
	Token token.Token
	Value *big.Int
}

func (bi *BigIntLiteral) Accept(v Visitor)     { v.VisitBigIntLiteral(bi) }
func (bi *BigIntLiteral) expressionNode()      {}
func (bi *BigIntLiteral) TokenLiteral() string { return bi.Token.Lexeme }
func (bi *BigIntLiteral) GetToken() token.Token {
	if bi == nil {
		return token.Token{}
	}
	return bi.Token
}

// RationalLiteral represents a Rational (Rat) literal.
type RationalLiteral struct {
	Token token.Token
	Value *big.Rat
}

func (rl *RationalLiteral) Accept(v Visitor)     { v.VisitRationalLiteral(rl) }
func (rl *RationalLiteral) expressionNode()      {}
func (rl *RationalLiteral) TokenLiteral() string { return rl.Token.Lexeme }
func (rl *RationalLiteral) GetToken() token.Token {
	if rl == nil {
		return token.Token{}
	}
	return rl.Token
}

// TupleLiteral represents a tuple, e.g. (1, "hello", true)
type TupleLiteral struct {
	Token    token.Token // The '(' token
	Elements []Expression
}

func (tl *TupleLiteral) Accept(v Visitor)     { v.VisitTupleLiteral(tl) }
func (tl *TupleLiteral) expressionNode()      {}
func (tl *TupleLiteral) TokenLiteral() string { return tl.Token.Lexeme }
func (tl *TupleLiteral) GetToken() token.Token {
	if tl == nil {
		return token.Token{}
	}
	return tl.Token
}

// ListLiteral represents a list, e.g. [1, 2, 3]
type ListLiteral struct {
	Token    token.Token // The '[' token
	Elements []Expression
}

func (ll *ListLiteral) Accept(v Visitor)     { v.VisitListLiteral(ll) }
func (ll *ListLiteral) expressionNode()      {}
func (ll *ListLiteral) TokenLiteral() string { return ll.Token.Lexeme }
func (ll *ListLiteral) GetToken() token.Token {
	if ll == nil {
		return token.Token{}
	}
	return ll.Token
}

// RecordLiteral represents a record/struct instantiation, e.g. { x: 1, y: 2 }
type RecordLiteral struct {
	Token  token.Token // The '{' token
	Spread Expression  // Optional: { ...base, key: val } - the base expression to spread
	Fields map[string]Expression
}

func (rl *RecordLiteral) Accept(v Visitor)     { v.VisitRecordLiteral(rl) }
func (rl *RecordLiteral) expressionNode()      {}
func (rl *RecordLiteral) TokenLiteral() string { return rl.Token.Lexeme }
func (rl *RecordLiteral) GetToken() token.Token {
	if rl == nil {
		return token.Token{}
	}
	return rl.Token
}

// MapLiteral represents a map literal, e.g. %{ "key" => value }
type MapLiteral struct {
	Token token.Token                       // The '%{' token
	Pairs []struct{ Key, Value Expression } // Key-value pairs
}

func (ml *MapLiteral) Accept(v Visitor)     { v.VisitMapLiteral(ml) }
func (ml *MapLiteral) expressionNode()      {}
func (ml *MapLiteral) TokenLiteral() string { return ml.Token.Lexeme }
func (ml *MapLiteral) GetToken() token.Token {
	if ml == nil {
		return token.Token{}
	}
	return ml.Token
}

// StringLiteral represents a string, e.g. "hello"
type StringLiteral struct {
	Token token.Token
	Value string
}

func (sl *StringLiteral) Accept(v Visitor)     { v.VisitStringLiteral(sl) }
func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Lexeme }
func (sl *StringLiteral) GetToken() token.Token {
	if sl == nil {
		return token.Token{}
	}
	return sl.Token
}

// FormatStringLiteral represents a format string, e.g. %".2f"
type FormatStringLiteral struct {
	Token token.Token // The FORMAT_STRING token
	Value string      // The format string (without quotes), e.g. ".2f"
}

func (fl *FormatStringLiteral) Accept(v Visitor)     { v.VisitFormatStringLiteral(fl) }
func (fl *FormatStringLiteral) expressionNode()      {}
func (fl *FormatStringLiteral) TokenLiteral() string { return fl.Token.Lexeme }
func (fl *FormatStringLiteral) GetToken() token.Token {
	if fl == nil {
		return token.Token{}
	}
	return fl.Token
}

// InterpolatedString represents a string with embedded expressions, e.g. "Hello, ${name}!"
// Parts is a list of expressions - StringLiteral for text parts, other expressions for ${...}
type InterpolatedString struct {
	Token token.Token
	Parts []Expression
}

func (is *InterpolatedString) Accept(v Visitor)     { v.VisitInterpolatedString(is) }
func (is *InterpolatedString) expressionNode()      {}
func (is *InterpolatedString) TokenLiteral() string { return is.Token.Lexeme }
func (is *InterpolatedString) GetToken() token.Token {
	if is == nil {
		return token.Token{}
	}
	return is.Token
}

// CharLiteral represents a character, e.g. 'c'
type CharLiteral struct {
	Token token.Token
	Value int64
}

func (cl *CharLiteral) Accept(v Visitor)     { v.VisitCharLiteral(cl) }
func (cl *CharLiteral) expressionNode()      {}
func (cl *CharLiteral) TokenLiteral() string { return cl.Token.Lexeme }
func (cl *CharLiteral) GetToken() token.Token {
	if cl == nil {
		return token.Token{}
	}
	return cl.Token
}

// BytesLiteral represents a bytes literal, e.g. @"hello", @x"48656C", @b"01001000"
type BytesLiteral struct {
	Token   token.Token // BYTES_STRING, BYTES_HEX, or BYTES_BIN
	Content string      // Raw content from the literal
	Kind    string      // "string", "hex", or "bin"
}

func (bl *BytesLiteral) Accept(v Visitor)     { v.VisitBytesLiteral(bl) }
func (bl *BytesLiteral) expressionNode()      {}
func (bl *BytesLiteral) TokenLiteral() string { return bl.Token.Lexeme }
func (bl *BytesLiteral) GetToken() token.Token {
	if bl == nil {
		return token.Token{}
	}
	return bl.Token
}

// BitsLiteral represents a bits literal, e.g. #b"10101010", #x"FF"
type BitsLiteral struct {
	Token   token.Token // BITS_BIN or BITS_HEX
	Content string      // Raw content from the literal (binary or hex string)
	Kind    string      // "bin" or "hex"
}

func (bl *BitsLiteral) Accept(v Visitor)     { v.VisitBitsLiteral(bl) }
func (bl *BitsLiteral) expressionNode()      {}
func (bl *BitsLiteral) TokenLiteral() string { return bl.Token.Lexeme }
func (bl *BitsLiteral) GetToken() token.Token {
	if bl == nil {
		return token.Token{}
	}
	return bl.Token
}
