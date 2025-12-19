package ast

import (
	"math/big"
	"github.com/funvibe/funxy/internal/token"
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
}

// Expression is a Node that represents an expression.
type Expression interface {
	Node
	expressionNode()
	GetToken() token.Token
}

// Program is the root node of every AST our parser produces.
type Program struct {
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

func (cd *ConstantDeclaration) Accept(v Visitor)      { v.VisitConstantDeclaration(cd) }
func (cd *ConstantDeclaration) statementNode()        {}
func (cd *ConstantDeclaration) TokenLiteral() string  { return cd.Token.Lexeme }
func (cd *ConstantDeclaration) GetToken() token.Token { return cd.Token }

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

func (es *ExportSpec) GetToken() token.Token { return es.Token }

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

func (pd *PackageDeclaration) Accept(v Visitor)      { v.VisitPackageDeclaration(pd) }
func (pd *PackageDeclaration) statementNode()        {}
func (pd *PackageDeclaration) TokenLiteral() string  { return pd.Token.Lexeme }
func (pd *PackageDeclaration) GetToken() token.Token { return pd.Token }

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

func (is *ImportStatement) Accept(v Visitor)      { v.VisitImportStatement(is) }
func (is *ImportStatement) statementNode()        {}
func (is *ImportStatement) TokenLiteral() string  { return is.Token.Lexeme }
func (is *ImportStatement) GetToken() token.Token { return is.Token }

// Identifier represents an identifier, e.g., a variable name.
type Identifier struct {
	Token token.Token // the token.IDENT token
	Value string
}

func (i *Identifier) Accept(v Visitor)      { v.VisitIdentifier(i) }
func (i *Identifier) expressionNode()       {}
func (i *Identifier) TokenLiteral() string  { return i.Token.Lexeme }
func (i *Identifier) GetToken() token.Token { return i.Token }

// IntegerLiteral represents an integer literal.
type IntegerLiteral struct {
	Token token.Token
	Value int64
}

func (il *IntegerLiteral) Accept(v Visitor)      { v.VisitIntegerLiteral(il) }
func (il *IntegerLiteral) expressionNode()       {}
func (il *IntegerLiteral) TokenLiteral() string  { return il.Token.Lexeme }
func (il *IntegerLiteral) GetToken() token.Token { return il.Token }

// BooleanLiteral represents boolean literals true/false.
type BooleanLiteral struct {
	Token token.Token
	Value bool
}

func (b *BooleanLiteral) Accept(v Visitor)      { v.VisitBooleanLiteral(b) }
func (b *BooleanLiteral) expressionNode()       {}
func (b *BooleanLiteral) TokenLiteral() string  { return b.Token.Lexeme }
func (b *BooleanLiteral) GetToken() token.Token { return b.Token }

// NilLiteral represents the nil literal (the only value of type Nil).
type NilLiteral struct {
	Token token.Token
}

func (n *NilLiteral) Accept(v Visitor)      { v.VisitNilLiteral(n) }
func (n *NilLiteral) expressionNode()       {}
func (n *NilLiteral) TokenLiteral() string  { return n.Token.Lexeme }
func (n *NilLiteral) GetToken() token.Token { return n.Token }

// FloatLiteral represents a floating point literal.
type FloatLiteral struct {
	Token token.Token
	Value float64
}

func (fl *FloatLiteral) Accept(v Visitor)      { v.VisitFloatLiteral(fl) }
func (fl *FloatLiteral) expressionNode()       {}
func (fl *FloatLiteral) TokenLiteral() string  { return fl.Token.Lexeme }
func (fl *FloatLiteral) GetToken() token.Token { return fl.Token }

// BigIntLiteral represents a BigInt literal.
type BigIntLiteral struct {
	Token token.Token
	Value *big.Int
}

func (bi *BigIntLiteral) Accept(v Visitor)      { v.VisitBigIntLiteral(bi) }
func (bi *BigIntLiteral) expressionNode()       {}
func (bi *BigIntLiteral) TokenLiteral() string  { return bi.Token.Lexeme }
func (bi *BigIntLiteral) GetToken() token.Token { return bi.Token }

// RationalLiteral represents a Rational (Rat) literal.
type RationalLiteral struct {
	Token token.Token
	Value *big.Rat
}

func (rl *RationalLiteral) Accept(v Visitor)      { v.VisitRationalLiteral(rl) }
func (rl *RationalLiteral) expressionNode()       {}
func (rl *RationalLiteral) TokenLiteral() string  { return rl.Token.Lexeme }
func (rl *RationalLiteral) GetToken() token.Token { return rl.Token }

// TupleLiteral represents a tuple, e.g. (1, "hello", true)
type TupleLiteral struct {
	Token    token.Token // The '(' token
	Elements []Expression
}

func (tl *TupleLiteral) Accept(v Visitor)      { v.VisitTupleLiteral(tl) }
func (tl *TupleLiteral) expressionNode()       {}
func (tl *TupleLiteral) TokenLiteral() string  { return tl.Token.Lexeme }
func (tl *TupleLiteral) GetToken() token.Token { return tl.Token }

// ListLiteral represents a list, e.g. [1, 2, 3]
type ListLiteral struct {
	Token    token.Token // The '[' token
	Elements []Expression
}

func (ll *ListLiteral) Accept(v Visitor)      { v.VisitListLiteral(ll) }
func (ll *ListLiteral) expressionNode()       {}
func (ll *ListLiteral) TokenLiteral() string  { return ll.Token.Lexeme }
func (ll *ListLiteral) GetToken() token.Token { return ll.Token }

// RecordLiteral represents a record/struct instantiation, e.g. { x: 1, y: 2 }
type RecordLiteral struct {
	Token  token.Token // The '{' token
	Spread Expression  // Optional: { ...base, key: val } - the base expression to spread
	Fields map[string]Expression
}

func (rl *RecordLiteral) Accept(v Visitor)      { v.VisitRecordLiteral(rl) }
func (rl *RecordLiteral) expressionNode()       {}
func (rl *RecordLiteral) TokenLiteral() string  { return rl.Token.Lexeme }
func (rl *RecordLiteral) GetToken() token.Token { return rl.Token }

// MapLiteral represents a map literal, e.g. %{ "key" => value }
type MapLiteral struct {
	Token token.Token                       // The '%{' token
	Pairs []struct{ Key, Value Expression } // Key-value pairs
}

func (ml *MapLiteral) Accept(v Visitor)      { v.VisitMapLiteral(ml) }
func (ml *MapLiteral) expressionNode()       {}
func (ml *MapLiteral) TokenLiteral() string  { return ml.Token.Lexeme }
func (ml *MapLiteral) GetToken() token.Token { return ml.Token }

// IndexExpression represents indexing, e.g. arr[i]
type IndexExpression struct {
	Token token.Token // The '[' token
	Left  Expression
	Index Expression
}

func (ie *IndexExpression) Accept(v Visitor)      { v.VisitIndexExpression(ie) }
func (ie *IndexExpression) expressionNode()       {}
func (ie *IndexExpression) TokenLiteral() string  { return ie.Token.Lexeme }
func (ie *IndexExpression) GetToken() token.Token { return ie.Token }

// MemberExpression represents dot access, e.g. obj.field or obj?.field
type MemberExpression struct {
	Token      token.Token // The '.' or '?.' token
	Left       Expression
	Member     *Identifier
	IsOptional bool // true for ?. (optional chaining)
}

func (me *MemberExpression) Accept(v Visitor)      { v.VisitMemberExpression(me) }
func (me *MemberExpression) expressionNode()       {}
func (me *MemberExpression) TokenLiteral() string  { return me.Token.Lexeme }
func (me *MemberExpression) GetToken() token.Token { return me.Token }

// StringLiteral represents a string, e.g. "hello"
type StringLiteral struct {
	Token token.Token
	Value string
}

func (sl *StringLiteral) Accept(v Visitor)      { v.VisitStringLiteral(sl) }
func (sl *StringLiteral) expressionNode()       {}
func (sl *StringLiteral) TokenLiteral() string  { return sl.Token.Lexeme }
func (sl *StringLiteral) GetToken() token.Token { return sl.Token }

// FormatStringLiteral represents a format string, e.g. %".2f"
type FormatStringLiteral struct {
	Token token.Token // The FORMAT_STRING token
	Value string      // The format string (without quotes), e.g. ".2f"
}

func (fl *FormatStringLiteral) Accept(v Visitor)      { v.VisitFormatStringLiteral(fl) }
func (fl *FormatStringLiteral) expressionNode()       {}
func (fl *FormatStringLiteral) TokenLiteral() string  { return fl.Token.Lexeme }
func (fl *FormatStringLiteral) GetToken() token.Token { return fl.Token }

// InterpolatedString represents a string with embedded expressions, e.g. "Hello, ${name}!"
// Parts is a list of expressions - StringLiteral for text parts, other expressions for ${...}
type InterpolatedString struct {
	Token token.Token
	Parts []Expression
}

func (is *InterpolatedString) Accept(v Visitor)      { v.VisitInterpolatedString(is) }
func (is *InterpolatedString) expressionNode()       {}
func (is *InterpolatedString) TokenLiteral() string  { return is.Token.Lexeme }
func (is *InterpolatedString) GetToken() token.Token { return is.Token }

// CharLiteral represents a character, e.g. 'c'
type CharLiteral struct {
	Token token.Token
	Value int64
}

func (cl *CharLiteral) Accept(v Visitor)      { v.VisitCharLiteral(cl) }
func (cl *CharLiteral) expressionNode()       {}
func (cl *CharLiteral) TokenLiteral() string  { return cl.Token.Lexeme }
func (cl *CharLiteral) GetToken() token.Token { return cl.Token }

// BytesLiteral represents a bytes literal, e.g. @"hello", @x"48656C", @b"01001000"
type BytesLiteral struct {
	Token   token.Token // BYTES_STRING, BYTES_HEX, or BYTES_BIN
	Content string      // Raw content from the literal
	Kind    string      // "string", "hex", or "bin"
}

func (bl *BytesLiteral) Accept(v Visitor)      { v.VisitBytesLiteral(bl) }
func (bl *BytesLiteral) expressionNode()       {}
func (bl *BytesLiteral) TokenLiteral() string  { return bl.Token.Lexeme }
func (bl *BytesLiteral) GetToken() token.Token { return bl.Token }

// BitsLiteral represents a bits literal, e.g. #b"10101010", #x"FF"
type BitsLiteral struct {
	Token   token.Token // BITS_BIN or BITS_HEX
	Content string      // Raw content from the literal (binary or hex string)
	Kind    string      // "bin" or "hex"
}

func (bl *BitsLiteral) Accept(v Visitor)      { v.VisitBitsLiteral(bl) }
func (bl *BitsLiteral) expressionNode()       {}
func (bl *BitsLiteral) TokenLiteral() string  { return bl.Token.Lexeme }
func (bl *BitsLiteral) GetToken() token.Token { return bl.Token }

// AnnotatedExpression represents an expression with an explicit type annotation.
// E.g., x: Int
type AnnotatedExpression struct {
	Token          token.Token // The COLON token
	Expression     Expression
	TypeAnnotation Type
}

func (ae *AnnotatedExpression) Accept(v Visitor)      { v.VisitAnnotatedExpression(ae) }
func (ae *AnnotatedExpression) expressionNode()       {}
func (ae *AnnotatedExpression) TokenLiteral() string  { return ae.Token.Lexeme }
func (ae *AnnotatedExpression) GetToken() token.Token { return ae.Token }

// ExpressionStatement is a statement that consists of a single expression.
type ExpressionStatement struct {
	Token      token.Token // the first token of the expression
	Expression Expression
}

func (es *ExpressionStatement) Accept(v Visitor)      { v.VisitExpressionStatement(es) }
func (es *ExpressionStatement) statementNode()        {}
func (es *ExpressionStatement) TokenLiteral() string  { return es.Token.Lexeme }
func (es *ExpressionStatement) GetToken() token.Token { return es.Token }

// BlockStatement represents a list of statements within curly braces.
type BlockStatement struct {
	Token      token.Token // {
	Statements []Statement
}

func (bs *BlockStatement) Accept(v Visitor)      { v.VisitBlockStatement(bs) }
func (bs *BlockStatement) statementNode()        {}
func (bs *BlockStatement) expressionNode()       {}
func (bs *BlockStatement) TokenLiteral() string  { return bs.Token.Lexeme }
func (bs *BlockStatement) GetToken() token.Token { return bs.Token }

// TypeConstraint represents a generic constraint, e.g. T: Show
type TypeConstraint struct {
	TypeVar string
	Trait   string // The name of the Trait
}

// FunctionStatement represents a function definition.
// fun name<T: Class>(params) returnType { body }
// Or extension method: fun (recv: Type) name(...) ...
// Or operator method in trait: operator (+)(a: T, b: T) -> T
type FunctionStatement struct {
	Token       token.Token // The 'fun' token or 'operator' token
	Name        *Identifier
	Operator    string            // For trait operator methods: "+", "-", "==", "<", etc.
	Receiver    *Parameter        // Optional receiver for extension methods
	TypeParams  []*Identifier     // Generic parameters e.g. <T, U>
	Constraints []*TypeConstraint // T: Show
	Parameters  []*Parameter
	ReturnType  Type // Can be nil if inferred. But user syntax has it.
	Body        *BlockStatement
}

type Parameter struct {
	Token      token.Token
	Name       *Identifier
	Type       Type
	IsVariadic bool
	IsIgnored  bool       // True if parameter is _ (ignored/wildcard)
	Default    Expression // Optional default value (e.g., fun f(x, y = 10))
}

func (fs *FunctionStatement) Accept(v Visitor)      { v.VisitFunctionStatement(fs) }
func (fs *FunctionStatement) statementNode()        {}
func (fs *FunctionStatement) TokenLiteral() string  { return fs.Token.Lexeme }
func (fs *FunctionStatement) GetToken() token.Token { return fs.Token }

// FunctionLiteral represents an anonymous function (lambda).
// fun(x, y) -> x + y
type FunctionLiteral struct {
	Token      token.Token // The 'fun' token
	Parameters []*Parameter
	ReturnType Type            // Optional return type
	Body       *BlockStatement // We normalize body to a block
}

func (fl *FunctionLiteral) Accept(v Visitor)      { v.VisitFunctionLiteral(fl) }
func (fl *FunctionLiteral) expressionNode()       {}
func (fl *FunctionLiteral) TokenLiteral() string  { return fl.Token.Lexeme }
func (fl *FunctionLiteral) GetToken() token.Token { return fl.Token }

// IfExpression represents an if-else expression.
type IfExpression struct {
	Token       token.Token // if
	Condition   Expression
	Consequence *BlockStatement
	Alternative *BlockStatement // else block (optional in struct, but required by semantics?)
}

func (ie *IfExpression) Accept(v Visitor)      { v.VisitIfExpression(ie) }
func (ie *IfExpression) expressionNode()       {}
func (ie *IfExpression) TokenLiteral() string  { return ie.Token.Lexeme }
func (ie *IfExpression) GetToken() token.Token { return ie.Token }

// ForExpression represents a for loop.
// for <condition> { body } or for <item> in <iterable> { body }
type ForExpression struct {
	Token       token.Token // The 'for' token
	Initializer Statement   // Optional, for traditional for loops (not yet implemented)
	Condition   Expression  // For 'while' style loops
	ItemName    *Identifier // For 'for in' loops
	Iterable    Expression  // For 'for in' loops
	Body        *BlockStatement
}

func (fe *ForExpression) Accept(v Visitor)      { v.VisitForExpression(fe) }
func (fe *ForExpression) expressionNode()       {}
func (fe *ForExpression) TokenLiteral() string  { return fe.Token.Lexeme }
func (fe *ForExpression) GetToken() token.Token { return fe.Token }

// BreakStatement represents a break statement.
// break or break <expression>
type BreakStatement struct {
	Token token.Token // The 'break' token
	Value Expression  // Optional value to return from the loop
}

func (bs *BreakStatement) Accept(v Visitor)      { v.VisitBreakStatement(bs) }
func (bs *BreakStatement) statementNode()        {}
func (bs *BreakStatement) TokenLiteral() string  { return bs.Token.Lexeme }
func (bs *BreakStatement) GetToken() token.Token { return bs.Token }

// ContinueStatement represents a continue statement.
// continue
type ContinueStatement struct {
	Token token.Token // The 'continue' token
}

func (cs *ContinueStatement) Accept(v Visitor)      { v.VisitContinueStatement(cs) }
func (cs *ContinueStatement) statementNode()        {}
func (cs *ContinueStatement) TokenLiteral() string  { return cs.Token.Lexeme }
func (cs *ContinueStatement) GetToken() token.Token { return cs.Token }

// PrefixExpression represents a prefix operation, e.g., -5 or !true.
type PrefixExpression struct {
	Token    token.Token // The prefix token, e.g. !
	Operator string
	Right    Expression
}

func (pe *PrefixExpression) Accept(v Visitor)      { v.VisitPrefixExpression(pe) }
func (pe *PrefixExpression) expressionNode()       {}
func (pe *PrefixExpression) TokenLiteral() string  { return pe.Token.Lexeme }
func (pe *PrefixExpression) GetToken() token.Token { return pe.Token }

// InfixExpression represents an infix operation, e.g., 5 + 5.
type InfixExpression struct {
	Token    token.Token // The operator token, e.g. +
	Left     Expression
	Operator string
	Right    Expression
}

func (ie *InfixExpression) Accept(v Visitor)      { v.VisitInfixExpression(ie) }
func (ie *InfixExpression) expressionNode()       {}
func (ie *InfixExpression) TokenLiteral() string  { return ie.Token.Lexeme }
func (ie *InfixExpression) GetToken() token.Token { return ie.Token }

// OperatorAsFunction represents an operator used as a function, e.g., (+), (-)
type OperatorAsFunction struct {
	Token    token.Token // The opening paren
	Operator string      // The operator: +, -, *, /, etc.
}

func (oaf *OperatorAsFunction) Accept(v Visitor)      { v.VisitOperatorAsFunction(oaf) }
func (oaf *OperatorAsFunction) expressionNode()       {}
func (oaf *OperatorAsFunction) TokenLiteral() string  { return oaf.Token.Lexeme }
func (oaf *OperatorAsFunction) GetToken() token.Token { return oaf.Token }
func (oaf *OperatorAsFunction) String() string        { return "(" + oaf.Operator + ")" }

// AssignExpression represents an assignment expression, e.g., x = 5 or x: Int = 5 or obj.x = 5
type AssignExpression struct {
	Token         token.Token // the token.ASSIGN token
	Left          Expression  // Changed from Name *Identifier to Left Expression to support l-values like obj.x
	AnnotatedType Type        // Optional type annotation from x: Int = ...
	Value         Expression
}

func (ae *AssignExpression) Accept(v Visitor)      { v.VisitAssignExpression(ae) }
func (ae *AssignExpression) expressionNode()       {}
func (ae *AssignExpression) TokenLiteral() string  { return ae.Token.Lexeme }
func (ae *AssignExpression) GetToken() token.Token { return ae.Token }

// PatternAssignExpression represents pattern destructuring: (a, b) = expr or [x, xs...] = list
type PatternAssignExpression struct {
	Token   token.Token // the token.ASSIGN token
	Pattern Pattern
	Value   Expression
}

func (pe *PatternAssignExpression) Accept(v Visitor)      { v.VisitPatternAssignExpression(pe) }
func (pe *PatternAssignExpression) expressionNode()       {}
func (pe *PatternAssignExpression) TokenLiteral() string  { return pe.Token.Lexeme }
func (pe *PatternAssignExpression) GetToken() token.Token { return pe.Token }

// CallExpression represents a function call, e.g., print(x, y)
type CallExpression struct {
	Token     token.Token // The '(' token
	Function  Expression  // Identifier or FunctionLiteral
	Arguments []Expression
	IsTail    bool // Set by Analyzer if this call is in a tail position
}

func (ce *CallExpression) Accept(v Visitor)      { v.VisitCallExpression(ce) }
func (ce *CallExpression) expressionNode()       {}
func (ce *CallExpression) TokenLiteral() string  { return ce.Token.Lexeme }
func (ce *CallExpression) GetToken() token.Token { return ce.Token }

// SpreadExpression represents ... in an expression, e.g. args...
type SpreadExpression struct {
	Token      token.Token // The '...' token
	Expression Expression
}

func (se *SpreadExpression) Accept(v Visitor)      { v.VisitSpreadExpression(se) }
func (se *SpreadExpression) expressionNode()       {}
func (se *SpreadExpression) TokenLiteral() string  { return se.Token.Lexeme }
func (se *SpreadExpression) GetToken() token.Token { return se.Token }

// TypeApplicationExpression represents applying types to a generic function/identifier.
// E.g. foo<Int>(...)
type TypeApplicationExpression struct {
	Token         token.Token // The identifier token (or whatever started it)
	Expression    Expression  // The expression being applied (usually Identifier)
	TypeArguments []Type
}

func (tae *TypeApplicationExpression) Accept(v Visitor)      { v.VisitTypeApplicationExpression(tae) }
func (tae *TypeApplicationExpression) expressionNode()       {}
func (tae *TypeApplicationExpression) TokenLiteral() string  { return tae.Token.Lexeme }
func (tae *TypeApplicationExpression) GetToken() token.Token { return tae.Token }

// PostfixExpression represents a postfix operation, e.g. expr?
type PostfixExpression struct {
	Token    token.Token // The postfix token, e.g. ?
	Operator string
	Left     Expression
}

func (pe *PostfixExpression) Accept(v Visitor)      { v.VisitPostfixExpression(pe) }
func (pe *PostfixExpression) expressionNode()       {}
func (pe *PostfixExpression) TokenLiteral() string  { return pe.Token.Lexeme }
func (pe *PostfixExpression) GetToken() token.Token { return pe.Token }

// --- Type System Nodes ---

// Type represents a type node in the AST.
// E.g., Int, List, List a, (Int, Int) -> Bool, { x: Int }
type Type interface {
	Node
	typeNode()
	GetToken() token.Token // Add GetToken to the Type interface
}

// NamedType represents a simple named type like 'Int', 'Money', or 'List'.
type NamedType struct {
	Token token.Token // The type's token, e.g., IDENT_UPPER
	Name  *Identifier
	Args  []Type
}

func (nt *NamedType) Accept(v Visitor)      { v.VisitNamedType(nt) }
func (nt *NamedType) typeNode()             {}
func (nt *NamedType) TokenLiteral() string  { return nt.Token.Lexeme }
func (nt *NamedType) GetToken() token.Token { return nt.Token }

// TupleType represents a tuple type, e.g. (Int, Bool)
type TupleType struct {
	Token token.Token // The '(' token
	Types []Type
}

func (tt *TupleType) Accept(v Visitor)      { v.VisitTupleType(tt) }
func (tt *TupleType) typeNode()             {}
func (tt *TupleType) TokenLiteral() string  { return tt.Token.Lexeme }
func (tt *TupleType) GetToken() token.Token { return tt.Token }

// RecordType represents a record/struct type, e.g. { x: Int, y: Bool }
type RecordType struct {
	Token  token.Token // The '{' token
	Fields map[string]Type
}

func (rt *RecordType) Accept(v Visitor)      { v.VisitRecordType(rt) }
func (rt *RecordType) typeNode()             {}
func (rt *RecordType) TokenLiteral() string  { return rt.Token.Lexeme }
func (rt *RecordType) GetToken() token.Token { return rt.Token }

// FunctionType represents a function type, e.g. Int -> Int or (Int, Int) -> Bool
type FunctionType struct {
	Token      token.Token // The '->' token (or start token?)
	Parameters []Type      // Single type or tuple elements if it was a TupleType
	ReturnType Type
}

func (ft *FunctionType) Accept(v Visitor)      { v.VisitFunctionType(ft) }
func (ft *FunctionType) typeNode()             {}
func (ft *FunctionType) TokenLiteral() string  { return ft.Token.Lexeme }
func (ft *FunctionType) GetToken() token.Token { return ft.Token }

// UnionType represents a union type, e.g. Int | String | Nil
// Also used for T? which desugars to T | Nil
type UnionType struct {
	Token token.Token // The '|' token (or first type's token)
	Types []Type      // The types in the union (at least 2)
}

func (ut *UnionType) Accept(v Visitor)      { v.VisitUnionType(ut) }
func (ut *UnionType) typeNode()             {}
func (ut *UnionType) TokenLiteral() string  { return ut.Token.Lexeme }
func (ut *UnionType) GetToken() token.Token { return ut.Token }

// DataConstructor represents a single case in an ADT definition.
// E.g., 'Triangle Int Int Int' or 'Empty'.
type DataConstructor struct {
	Token      token.Token // The constructor's token, e.g., 'Triangle'
	Name       *Identifier
	Parameters []Type
}

func (dc *DataConstructor) Accept(v Visitor)      { v.VisitDataConstructor(dc) }
func (dc *DataConstructor) TokenLiteral() string  { return dc.Token.Lexeme }
func (dc *DataConstructor) GetToken() token.Token { return dc.Token }

// TypeDeclarationStatement represents a 'type' or 'type alias' definition.
// E.g., 'type alias Money = Float' or 'type List a = Empty | List a (List a)'
type TypeDeclarationStatement struct {
	Token          token.Token // the 'type' token
	Name           *Identifier
	IsAlias        bool
	TypeParameters []*Identifier // For polymorphism, e.g., ['a']
	// For an alias, this holds the target type.
	// For an ADT, this holds the various constructors.
	TargetType   Type
	Constructors []*DataConstructor
}

func (tds *TypeDeclarationStatement) Accept(v Visitor)      { v.VisitTypeDeclarationStatement(tds) }
func (tds *TypeDeclarationStatement) statementNode()        {}
func (tds *TypeDeclarationStatement) TokenLiteral() string  { return tds.Token.Lexeme }
func (tds *TypeDeclarationStatement) GetToken() token.Token { return tds.Token }

// --- Trait System Nodes ---

// TraitDeclaration represents a type class (trait) definition.
// trait Show<T> { fun show(val: T) -> String }
// trait Order<T> : Equal<T> { fun compare(a: T, b: T) -> Ordering }
type TraitDeclaration struct {
	Token       token.Token          // 'trait'
	Name        *Identifier          // 'Show'
	TypeParams  []*Identifier        // ['T']
	SuperTraits []Type               // [Equal<T>] - inherited traits
	Signatures  []*FunctionStatement // Method signatures
}

func (td *TraitDeclaration) Accept(v Visitor)      { v.VisitTraitDeclaration(td) }
func (td *TraitDeclaration) statementNode()        {}
func (td *TraitDeclaration) TokenLiteral() string  { return td.Token.Lexeme }
func (td *TraitDeclaration) GetToken() token.Token { return td.Token }

// InstanceDeclaration represents an implementation of a trait for a type.
// instance Show Int { fun show(val: Int) -> String { ... } }
// instance Functor<Result, E> { ... } -- HKT with extra type params
// instance sql.Model User { ... } -- Qualified trait name
type InstanceDeclaration struct {
	Token      token.Token          // 'instance'
	ModuleName *Identifier          // Optional: module name for qualified trait (e.g., 'sql' in 'sql.Model')
	TraitName  *Identifier          // 'Show' or 'Model' (was ClassName)
	Target     Type                 // 'Int' or '(List a)' or 'Maybe Int' or 'Result' for HKT
	TypeParams []*Identifier        // Extra type params like E in Functor<Result, E>
	Methods    []*FunctionStatement // Implementations
}

func (id *InstanceDeclaration) Accept(v Visitor)      { v.VisitInstanceDeclaration(id) }
func (id *InstanceDeclaration) statementNode()        {}
func (id *InstanceDeclaration) TokenLiteral() string  { return id.Token.Lexeme }
func (id *InstanceDeclaration) GetToken() token.Token { return id.Token }

// --- Pattern Matching ---

type Pattern interface {
	Node
	patternNode()
	GetToken() token.Token
}

// MatchArm represents a single case in a match expression.
// Optional Guard is evaluated after pattern match; arm executes only if guard is true.
type MatchArm struct {
	Pattern    Pattern
	Guard      Expression // Optional: condition after 'if', nil if no guard
	Expression Expression
}

// MatchExpression represents a match expression.
// match <Expression> { <MatchArms> }
type MatchExpression struct {
	Token      token.Token // match
	Expression Expression
	Arms       []*MatchArm
}

func (me *MatchExpression) Accept(v Visitor)      { v.VisitMatchExpression(me) }
func (me *MatchExpression) expressionNode()       {}
func (me *MatchExpression) TokenLiteral() string  { return me.Token.Lexeme }
func (me *MatchExpression) GetToken() token.Token { return me.Token }

// WildcardPattern: _
type WildcardPattern struct {
	Token token.Token
}

func (p *WildcardPattern) Accept(v Visitor)      { v.VisitWildcardPattern(p) }
func (p *WildcardPattern) patternNode()          {}
func (p *WildcardPattern) TokenLiteral() string  { return p.Token.Lexeme }
func (p *WildcardPattern) GetToken() token.Token { return p.Token }

// LiteralPattern: 1, true
type LiteralPattern struct {
	Token token.Token
	Value interface{}
}

func (p *LiteralPattern) Accept(v Visitor)      { v.VisitLiteralPattern(p) }
func (p *LiteralPattern) patternNode()          {}
func (p *LiteralPattern) TokenLiteral() string  { return p.Token.Lexeme }
func (p *LiteralPattern) GetToken() token.Token { return p.Token }

// IdentifierPattern: x
type IdentifierPattern struct {
	Token token.Token
	Value string
}

func (p *IdentifierPattern) Accept(v Visitor)      { v.VisitIdentifierPattern(p) }
func (p *IdentifierPattern) patternNode()          {}
func (p *IdentifierPattern) TokenLiteral() string  { return p.Token.Lexeme }
func (p *IdentifierPattern) GetToken() token.Token { return p.Token }

// ConstructorPattern: List x xs, Empty
type ConstructorPattern struct {
	Token    token.Token // Constructor name
	Name     *Identifier
	Elements []Pattern
}

func (p *ConstructorPattern) Accept(v Visitor)      { v.VisitConstructorPattern(p) }
func (p *ConstructorPattern) patternNode()          {}
func (p *ConstructorPattern) TokenLiteral() string  { return p.Token.Lexeme }
func (p *ConstructorPattern) GetToken() token.Token { return p.Token }

// TuplePattern: (x, y, _)
type TuplePattern struct {
	Token    token.Token // '('
	Elements []Pattern
}

func (p *TuplePattern) Accept(v Visitor)      { v.VisitTuplePattern(p) }
func (p *TuplePattern) patternNode()          {}
func (p *TuplePattern) TokenLiteral() string  { return p.Token.Lexeme }
func (p *TuplePattern) GetToken() token.Token { return p.Token }

// SpreadPattern represents ... in a pattern, e.g. xs...
type SpreadPattern struct {
	Token   token.Token // The '...' token
	Pattern Pattern
}

func (sp *SpreadPattern) Accept(v Visitor)      { v.VisitSpreadPattern(sp) }
func (sp *SpreadPattern) patternNode()          {}
func (sp *SpreadPattern) TokenLiteral() string  { return sp.Token.Lexeme }
func (sp *SpreadPattern) GetToken() token.Token { return sp.Token }

// ListPattern: [], [x, xs...]
type ListPattern struct {
	Token    token.Token // '['
	Elements []Pattern
}

func (p *ListPattern) Accept(v Visitor)      { v.VisitListPattern(p) }
func (p *ListPattern) patternNode()          {}
func (p *ListPattern) TokenLiteral() string  { return p.Token.Lexeme }
func (p *ListPattern) GetToken() token.Token { return p.Token }

// RecordPattern: { x: p1, y: p2 }
type RecordPattern struct {
	Token  token.Token // '{'
	Fields map[string]Pattern
}

func (p *RecordPattern) Accept(v Visitor)      { v.VisitRecordPattern(p) }
func (p *RecordPattern) patternNode()          {}
func (p *RecordPattern) TokenLiteral() string  { return p.Token.Lexeme }
func (p *RecordPattern) GetToken() token.Token { return p.Token }

// TypePattern: n: Int (matches if value has type Int, binds to n)
type TypePattern struct {
	Token token.Token // The identifier token
	Name  string      // Binding name (can be "_" for ignored)
	Type  Type        // The type to match against
}

func (p *TypePattern) Accept(v Visitor)      { v.VisitTypePattern(p) }
func (p *TypePattern) patternNode()          {}
func (p *TypePattern) TokenLiteral() string  { return p.Token.Lexeme }
func (p *TypePattern) GetToken() token.Token { return p.Token }

// StringPattern: "/hello/{name}" with captures
// Matches strings and binds captured parts to variables
type StringPattern struct {
	Token token.Token
	Parts []StringPatternPart
}

// StringPatternPart is either a literal segment or a capture variable
type StringPatternPart struct {
	IsCapture bool
	Value     string // literal text or capture variable name
	Greedy    bool   // for {path...} style captures
}

func (p *StringPattern) Accept(v Visitor)      { v.VisitStringPattern(p) }
func (p *StringPattern) patternNode()          {}
func (p *StringPattern) TokenLiteral() string  { return p.Token.Lexeme }
func (p *StringPattern) GetToken() token.Token { return p.Token }

// PinPattern: ^variable
// Matches if value equals the existing variable's value (like Elixir's pin operator)
type PinPattern struct {
	Token token.Token // The '^' token
	Name  string      // Variable name to compare against
}

func (p *PinPattern) Accept(v Visitor)      { v.VisitPinPattern(p) }
func (p *PinPattern) patternNode()          {}
func (p *PinPattern) TokenLiteral() string  { return p.Token.Lexeme }
func (p *PinPattern) GetToken() token.Token { return p.Token }
