package ast

import (
	"github.com/funvibe/funxy/internal/token"
	"github.com/funvibe/funxy/internal/typesystem"
)

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

// ForallType represents a Rank-N polymorphic type: forall A B. A -> B
type ForallType struct {
	Token token.Token   // The 'forall' token
	Vars  []*Identifier // Type parameters
	Type  Type          // The inner type
}

func (ft *ForallType) Accept(v Visitor)      { v.VisitForallType(ft) }
func (ft *ForallType) typeNode()             {}
func (ft *ForallType) TokenLiteral() string  { return ft.Token.Lexeme }
func (ft *ForallType) GetToken() token.Token { return ft.Token }

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

// FunctionalDependency represents a functional dependency rule: a, b -> c
type FunctionalDependency struct {
	From []string // List of type variable names on LHS
	To   []string // List of type variable names on RHS
}

// TraitDeclaration represents a type class (trait) definition.
// trait Show<T> { fun show(val: T) -> String }
// trait Order<T> : Equal<T> { fun compare(a: T, b: T) -> Ordering }
type TraitDeclaration struct {
	Token        token.Token            // 'trait'
	Name         *Identifier            // 'Show'
	TypeParams   []*Identifier          // ['T']
	Constraints  []*TypeConstraint      // [t: Numeric]
	SuperTraits  []Type                 // [Equal<T>] - inherited traits
	Dependencies []FunctionalDependency // FunDeps: | a -> b
	Signatures   []*FunctionStatement   // Method signatures
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
	Token       token.Token          // 'instance'
	ModuleName  *Identifier          // Optional: module name for qualified trait (e.g., 'sql' in 'sql.Model')
	TraitName   *Identifier          // 'Show' or 'Model' (was ClassName)
	Args        []Type               // Multi-parameter type class arguments (e.g. Convert<A, B>)
	TypeParams  []*Identifier        // Extra type params like E in Functor<Result, E>
	Constraints []*TypeConstraint    // Constraints on the instance (e.g. instance Show a => Show (List a))
	Methods     []*FunctionStatement // Implementations

	// Analyzed Data (populated by Analyzer)
	AnalyzedRequirements []typesystem.Constraint // Constraints derived from usage/params
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

// SpreadPattern represents ... in a pattern, e.g. ...xs
type SpreadPattern struct {
	Token   token.Token // The '...' token
	Pattern Pattern
}

func (sp *SpreadPattern) Accept(v Visitor)      { v.VisitSpreadPattern(sp) }
func (sp *SpreadPattern) patternNode()          {}
func (sp *SpreadPattern) TokenLiteral() string  { return sp.Token.Lexeme }
func (sp *SpreadPattern) GetToken() token.Token { return sp.Token }

// ListPattern: [], [x, ...xs]
type ListPattern struct {
	Token    token.Token // '['
	Elements []Pattern
}

func (p *ListPattern) Accept(v Visitor)      { v.VisitListPattern(p) }
func (p *ListPattern) patternNode()          {}
func (p *ListPattern) TokenLiteral() string  { return p.Token.Lexeme }
func (p *ListPattern) GetToken() token.Token { return p.Token }

// RecordPattern: { x: p1, y: p2 } or Point { x: p1, y: p2 }
type RecordPattern struct {
	Token    token.Token // '{' or type name token
	TypeName string      // Optional type name (e.g., "Point" in Point { x: p1 })
	Fields   map[string]Pattern
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
