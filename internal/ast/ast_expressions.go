package ast

import (
	"github.com/funvibe/funxy/internal/token"
	"github.com/funvibe/funxy/internal/typesystem"
)

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
	Token       token.Token // The '.' or '?.' token
	Left        Expression
	Member      *Identifier
	IsOptional  bool       // true for ?. (optional chaining)
	Dictionary  Expression // For dictionary passing: the dictionary instance
	MethodIndex int        // For dictionary passing: index in the vtable
}

func (me *MemberExpression) Accept(v Visitor)      { v.VisitMemberExpression(me) }
func (me *MemberExpression) expressionNode()       {}
func (me *MemberExpression) TokenLiteral() string  { return me.Token.Lexeme }
func (me *MemberExpression) GetToken() token.Token { return me.Token }

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
	Token       token.Token // {
	Statements  []Statement
	RBraceToken token.Token // } - New: Closing brace token
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
	Args    []Type // For MPTC: [T1, T2...]
}

// FunctionStatement represents a function definition.
// fun name<T: Class>(params) returnType { body }
// Or extension method: fun (recv: Type) name(...) ...
// Or operator method in trait: operator (+)(a: T, b: T) -> T
type FunctionStatement struct {
	Token         token.Token // The 'fun' token or 'operator' token
	Name          *Identifier
	Operator      string            // For trait operator methods: "+", "-", "==", "<", etc.
	Receiver      *Parameter        // Optional receiver for extension methods
	TypeParams    []*Identifier     // Generic parameters e.g. <T, U>
	Constraints   []*TypeConstraint // T: Show
	Parameters    []*Parameter
	WitnessParams []string // New: Names of implicit dictionary parameters
	ReturnType    Type     // Can be nil if inferred. But user syntax has it.
	Body          *BlockStatement
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
	Token         token.Token // The 'fun' token
	Parameters    []*Parameter
	WitnessParams []string        // New: Names of implicit dictionary parameters (e.g. "$dict_T_Show")
	ReturnType    Type            // Optional return type
	Body          *BlockStatement // We normalize body to a block
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

// ReturnStatement represents a return statement.
// return or return <expression>
type ReturnStatement struct {
	Token token.Token // The 'return' token
	Value Expression  // Optional return value
}

func (rs *ReturnStatement) Accept(v Visitor)      { v.VisitReturnStatement(rs) }
func (rs *ReturnStatement) statementNode()        {}
func (rs *ReturnStatement) TokenLiteral() string  { return rs.Token.Lexeme }
func (rs *ReturnStatement) GetToken() token.Token { return rs.Token }

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

// PatternAssignExpression represents pattern destructuring: (a, b) = expr or [x, ...xs] = list
type PatternAssignExpression struct {
	Token         token.Token // the token.ASSIGN token
	Pattern       Pattern
	AnnotatedType Type // Optional type annotation from (a, b): Type = ...
	Value         Expression
}

func (pe *PatternAssignExpression) Accept(v Visitor)      { v.VisitPatternAssignExpression(pe) }
func (pe *PatternAssignExpression) expressionNode()       {}
func (pe *PatternAssignExpression) TokenLiteral() string  { return pe.Token.Lexeme }
func (pe *PatternAssignExpression) GetToken() token.Token { return pe.Token }

// CallExpression represents a function call, e.g., print(x, y)
type CallExpression struct {
	Token         token.Token // The '(' token
	Function      Expression  // Identifier or FunctionLiteral
	Arguments     []Expression
	IsTail        bool                       // Set by Analyzer if this call is in a tail position
	Witness       interface{}                // DEPRECATED: Old witness system
	Witnesses     []Expression               // New: Explicit dictionary arguments passed BEFORE regular args
	Instantiation map[string]typesystem.Type // Concrete types for generic parameters (e.g. "T" -> Int)
	TypeArgs      []typesystem.Type          // Type arguments for data constructors (e.g. [String, Int] for Result<String, Int>)
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

// RangeExpression represents a range, e.g., 1..10 or 1, 2..10.
type RangeExpression struct {
	Token token.Token // The '..' token
	Start Expression
	Next  Expression // Optional step (second element)
	End   Expression
}

func (re *RangeExpression) Accept(v Visitor)      { v.VisitRangeExpression(re) }
func (re *RangeExpression) expressionNode()       {}
func (re *RangeExpression) TokenLiteral() string  { return re.Token.Lexeme }
func (re *RangeExpression) GetToken() token.Token { return re.Token }

// TypeApplicationExpression represents applying types to a generic function/identifier.
// E.g. foo<Int>(...)
type TypeApplicationExpression struct {
	Token         token.Token // The identifier token (or whatever started it)
	Expression    Expression  // The expression being applied (usually Identifier)
	TypeArguments []Type
	Witnesses     []Expression // Explicit dictionary arguments resolved during inference
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
