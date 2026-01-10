package config

// Operators Configuration
//
// This is the SINGLE SOURCE OF TRUTH for all operators.
// Documentation is generated from this file.
//
// When adding a new operator, update:
//   1. token/token.go - add the token type constant
//   2. lexer/lexer.go - add lexer case to recognize the symbol
//   3. parser/parser.go - register precedence (for built-in) or use init() (for user)

// Associativity defines operator associativity
type Associativity int

const (
	AssocLeft Associativity = iota
	AssocRight
)

// Precedence levels (higher = binds tighter)
const (
	PrecPipe       = 0  // |>
	PrecLogicOr    = 1  // || >>=
	PrecLogicAnd   = 2  // &&
	PrecBitwiseOr  = 3  // | ^
	PrecBitwiseAnd = 4  // &
	PrecEquality   = 5  // == != < > <= >= <*>
	PrecRange      = 6  // ..
	PrecShift      = 7  // << >>
	PrecAdditive   = 8  // + - ++ <> ::
	PrecMultiply   = 9  // * / %
	PrecPower      = 10 // ** ,,
	PrecUnary      = 11 // ! - (prefix) ?
	PrecCall       = 12 // f(x) x[i] x.y
)

// OperatorInfo contains all metadata for an operator
type OperatorInfo struct {
	Symbol      string
	Signature   string        // Type signature
	Description string        // What it does
	Trait       string        // Associated trait (empty = built-in)
	Precedence  int           // Precedence level
	Assoc       Associativity // Left or Right
	CanOverride bool          // Can user override via trait instance?
	Category    string        // For grouping in docs
}

// AllOperators is the single source of truth for all operators
var AllOperators = []OperatorInfo{
	// Arithmetic (Numeric trait)
	{Symbol: "+", Signature: "(T, T) -> T", Description: "Addition", Trait: "Numeric", Precedence: PrecAdditive, Assoc: AssocLeft, CanOverride: true, Category: "Arithmetic"},
	{Symbol: "-", Signature: "(T, T) -> T", Description: "Subtraction", Trait: "Numeric", Precedence: PrecAdditive, Assoc: AssocLeft, CanOverride: true, Category: "Arithmetic"},
	{Symbol: "*", Signature: "(T, T) -> T", Description: "Multiplication", Trait: "Numeric", Precedence: PrecMultiply, Assoc: AssocLeft, CanOverride: true, Category: "Arithmetic"},
	{Symbol: "/", Signature: "(T, T) -> T", Description: "Division", Trait: "Numeric", Precedence: PrecMultiply, Assoc: AssocLeft, CanOverride: true, Category: "Arithmetic"},
	{Symbol: "%", Signature: "(T, T) -> T", Description: "Modulo", Trait: "Numeric", Precedence: PrecMultiply, Assoc: AssocLeft, CanOverride: true, Category: "Arithmetic"},
	{Symbol: "**", Signature: "(T, T) -> T", Description: "Exponentiation", Trait: "Numeric", Precedence: PrecPower, Assoc: AssocRight, CanOverride: true, Category: "Arithmetic"},

	// Compound assignment (Numeric trait, desugared to x = x op y)
	{Symbol: "+=", Signature: "(T, T) -> T", Description: "Add and assign", Trait: "Numeric", Precedence: 0, Assoc: AssocRight, CanOverride: false, Category: "Assignment"},
	{Symbol: "-=", Signature: "(T, T) -> T", Description: "Subtract and assign", Trait: "Numeric", Precedence: 0, Assoc: AssocRight, CanOverride: false, Category: "Assignment"},
	{Symbol: "*=", Signature: "(T, T) -> T", Description: "Multiply and assign", Trait: "Numeric", Precedence: 0, Assoc: AssocRight, CanOverride: false, Category: "Assignment"},
	{Symbol: "/=", Signature: "(T, T) -> T", Description: "Divide and assign", Trait: "Numeric", Precedence: 0, Assoc: AssocRight, CanOverride: false, Category: "Assignment"},
	{Symbol: "%=", Signature: "(T, T) -> T", Description: "Modulo and assign", Trait: "Numeric", Precedence: 0, Assoc: AssocRight, CanOverride: false, Category: "Assignment"},
	{Symbol: "**=", Signature: "(T, T) -> T", Description: "Power and assign", Trait: "Numeric", Precedence: 0, Assoc: AssocRight, CanOverride: false, Category: "Assignment"},

	// Comparison (Equal, Order traits)
	{Symbol: "==", Signature: "(T, T) -> Bool", Description: "Equality", Trait: "Equal", Precedence: PrecEquality, Assoc: AssocLeft, CanOverride: true, Category: "Comparison"},
	{Symbol: "!=", Signature: "(T, T) -> Bool", Description: "Inequality", Trait: "Equal", Precedence: PrecEquality, Assoc: AssocLeft, CanOverride: true, Category: "Comparison"},
	{Symbol: "<", Signature: "(T, T) -> Bool", Description: "Less than", Trait: "Order", Precedence: PrecEquality, Assoc: AssocLeft, CanOverride: true, Category: "Comparison"},
	{Symbol: ">", Signature: "(T, T) -> Bool", Description: "Greater than", Trait: "Order", Precedence: PrecEquality, Assoc: AssocLeft, CanOverride: true, Category: "Comparison"},
	{Symbol: "<=", Signature: "(T, T) -> Bool", Description: "Less or equal", Trait: "Order", Precedence: PrecEquality, Assoc: AssocLeft, CanOverride: true, Category: "Comparison"},
	{Symbol: ">=", Signature: "(T, T) -> Bool", Description: "Greater or equal", Trait: "Order", Precedence: PrecEquality, Assoc: AssocLeft, CanOverride: true, Category: "Comparison"},

	// Range
	{Symbol: "..", Signature: "(T, T) -> Range<T>", Description: "Range construction", Trait: "", Precedence: PrecRange, Assoc: AssocLeft, CanOverride: false, Category: "Range"},

	// Logical (built-in, Bool only)
	{Symbol: "&&", Signature: "(Bool, Bool) -> Bool", Description: "Logical AND", Trait: "", Precedence: PrecLogicAnd, Assoc: AssocLeft, CanOverride: false, Category: "Logical"},
	{Symbol: "||", Signature: "(Bool, Bool) -> Bool", Description: "Logical OR", Trait: "", Precedence: PrecLogicOr, Assoc: AssocLeft, CanOverride: false, Category: "Logical"},
	{Symbol: "!", Signature: "(Bool) -> Bool", Description: "Logical NOT (prefix)", Trait: "", Precedence: PrecUnary, Assoc: AssocRight, CanOverride: false, Category: "Logical"},

	// Bitwise (Bitwise trait)
	{Symbol: "&", Signature: "(T, T) -> T", Description: "Bitwise AND", Trait: "Bitwise", Precedence: PrecBitwiseAnd, Assoc: AssocLeft, CanOverride: true, Category: "Bitwise"},
	{Symbol: "|", Signature: "(T, T) -> T", Description: "Bitwise OR", Trait: "Bitwise", Precedence: PrecBitwiseOr, Assoc: AssocLeft, CanOverride: true, Category: "Bitwise"},
	{Symbol: "^", Signature: "(T, T) -> T", Description: "Bitwise XOR", Trait: "Bitwise", Precedence: PrecBitwiseOr, Assoc: AssocLeft, CanOverride: true, Category: "Bitwise"},
	{Symbol: "<<", Signature: "(T, T) -> T", Description: "Left shift", Trait: "Bitwise", Precedence: PrecShift, Assoc: AssocLeft, CanOverride: true, Category: "Bitwise"},
	{Symbol: ">>", Signature: "(T, T) -> T", Description: "Right shift", Trait: "Bitwise", Precedence: PrecShift, Assoc: AssocLeft, CanOverride: true, Category: "Bitwise"},

	// List (built-in and Concat trait)
	{Symbol: "::", Signature: "(T, List<T>) -> List<T>", Description: "Cons (prepend)", Trait: "", Precedence: PrecAdditive, Assoc: AssocRight, CanOverride: false, Category: "List"},
	{Symbol: "++", Signature: "(T, T) -> T", Description: "Concatenation", Trait: "Concat", Precedence: PrecAdditive, Assoc: AssocLeft, CanOverride: true, Category: "List"},

	// Function (built-in)
	{Symbol: "|>", Signature: "(A, (A) -> B) -> B", Description: "Pipe (forward application)", Trait: "", Precedence: PrecPipe, Assoc: AssocLeft, CanOverride: false, Category: "Function"},
	{Symbol: ",,", Signature: "((B)->C, (A)->B) -> (A)->C", Description: "Composition (right-to-left)", Trait: "", Precedence: PrecPower, Assoc: AssocRight, CanOverride: false, Category: "Function"},

	// Error handling (built-in)
	{Symbol: "?", Signature: "Option<T> -> T / Result<T,E> -> T", Description: "Error propagation (postfix)", Trait: "", Precedence: PrecUnary, Assoc: AssocLeft, CanOverride: false, Category: "Error"},

	// FP traits (HKT)
	{Symbol: "<>", Signature: "(A, A) -> A", Description: "Semigroup combine", Trait: "Semigroup", Precedence: PrecAdditive, Assoc: AssocRight, CanOverride: true, Category: "FP"},
	{Symbol: "<*>", Signature: "(F<(A)->B>, F<A>) -> F<B>", Description: "Applicative apply", Trait: "Applicative", Precedence: PrecEquality, Assoc: AssocLeft, CanOverride: true, Category: "FP"},
	{Symbol: ">>=", Signature: "(M<A>, (A)->M<B>) -> M<B>", Description: "Monad bind", Trait: "Monad", Precedence: PrecLogicOr, Assoc: AssocLeft, CanOverride: true, Category: "FP"},
	{Symbol: "??", Signature: "(F<A>, A) -> A", Description: "Null coalescing", Trait: "Optional", Precedence: PrecLogicOr, Assoc: AssocLeft, CanOverride: true, Category: "FP"},
	{Symbol: "?.", Signature: "F<A>.field -> F<B>", Description: "Optional chaining", Trait: "Optional", Precedence: PrecCall, Assoc: AssocLeft, CanOverride: false, Category: "FP"},

	// User-definable slots (for custom DSLs)
	{Symbol: "<|>", Signature: "(T, T) -> T", Description: "User-definable (choice/alternative)", Trait: "UserOpChoose", Precedence: PrecLogicOr, Assoc: AssocLeft, CanOverride: true, Category: "User"},
	{Symbol: "<$>", Signature: "(T, T) -> T", Description: "User-definable (map)", Trait: "UserOpMap", Precedence: PrecMultiply, Assoc: AssocLeft, CanOverride: true, Category: "User"},
	{Symbol: "<:>", Signature: "(T, T) -> T", Description: "User-definable (cons)", Trait: "UserOpCons", Precedence: PrecAdditive, Assoc: AssocRight, CanOverride: true, Category: "User"},
	{Symbol: "<~>", Signature: "(T, T) -> T", Description: "User-definable (swap)", Trait: "UserOpSwap", Precedence: PrecAdditive, Assoc: AssocLeft, CanOverride: true, Category: "User"},
	{Symbol: "=>", Signature: "(T, T) -> T", Description: "User-definable (imply)", Trait: "UserOpImply", Precedence: PrecLogicOr, Assoc: AssocRight, CanOverride: true, Category: "User"},
	{Symbol: "$", Signature: "(A -> B, A) -> B", Description: "Function application (low precedence)", Trait: "", Precedence: PrecPipe, Assoc: AssocRight, CanOverride: false, Category: "Function"},
	{Symbol: "<|", Signature: "(T, T) -> T", Description: "User-definable (pipe left)", Trait: "UserOpPipeLeft", Precedence: PrecLogicOr, Assoc: AssocRight, CanOverride: true, Category: "User"},
}

// GetOperator returns operator info by symbol
func GetOperator(symbol string) *OperatorInfo {
	for i := range AllOperators {
		if AllOperators[i].Symbol == symbol {
			return &AllOperators[i]
		}
	}
	return nil
}

// GetOperatorsByCategory returns operators grouped by category
func GetOperatorsByCategory() map[string][]OperatorInfo {
	result := make(map[string][]OperatorInfo)
	for _, op := range AllOperators {
		result[op.Category] = append(result[op.Category], op)
	}
	return result
}

// GetOperatorsByPrecedence returns operators sorted by precedence
func GetOperatorsByPrecedence() [][]OperatorInfo {
	maxPrec := 0
	for _, op := range AllOperators {
		if op.Precedence > maxPrec {
			maxPrec = op.Precedence
		}
	}

	result := make([][]OperatorInfo, maxPrec+1)
	for _, op := range AllOperators {
		result[op.Precedence] = append(result[op.Precedence], op)
	}
	return result
}

// Legacy support - keep UserOperators for backward compatibility
type OperatorDef struct {
	Symbol     string
	TokenName  string
	Trait      string
	Precedence OperatorPrecedence
	Assoc      Associativity
	ReturnSelf bool
}

type OperatorPrecedence int

const (
	PrecLowest OperatorPrecedence = iota
	PrecLow
	PrecMedium
	PrecHigh
)

var UserOperators = []OperatorDef{
	{Symbol: "<>", TokenName: "USER_OP_COMBINE", Trait: "Semigroup", Precedence: PrecLow, Assoc: AssocRight, ReturnSelf: true},
	{Symbol: "<*>", TokenName: "USER_OP_APPLY", Trait: "Applicative", Precedence: PrecMedium, Assoc: AssocLeft, ReturnSelf: false},
	{Symbol: ">>=", TokenName: "USER_OP_BIND", Trait: "Monad", Precedence: PrecLow, Assoc: AssocLeft, ReturnSelf: false},
	{Symbol: "??", TokenName: "NULL_COALESCE", Trait: "Optional", Precedence: PrecLow, Assoc: AssocLeft, ReturnSelf: false},
	{Symbol: "<|>", TokenName: "USER_OP_CHOOSE", Trait: "UserOpChoose", Precedence: PrecLow, Assoc: AssocLeft, ReturnSelf: true},
	{Symbol: "<$>", TokenName: "USER_OP_MAP", Trait: "UserOpMap", Precedence: PrecHigh, Assoc: AssocLeft, ReturnSelf: true},
	{Symbol: "<:>", TokenName: "USER_OP_CONS", Trait: "UserOpCons", Precedence: PrecMedium, Assoc: AssocRight, ReturnSelf: true},
	{Symbol: "<~>", TokenName: "USER_OP_SWAP", Trait: "UserOpSwap", Precedence: PrecMedium, Assoc: AssocLeft, ReturnSelf: true},
	{Symbol: "=>", TokenName: "USER_OP_IMPLY", Trait: "UserOpImply", Precedence: PrecLow, Assoc: AssocRight, ReturnSelf: true},
	{Symbol: "<|", TokenName: "USER_OP_PIPE_LEFT", Trait: "UserOpPipeLeft", Precedence: PrecLow, Assoc: AssocRight, ReturnSelf: true},
}

// GetUserOperatorBySymbol returns the operator definition for a symbol
func GetUserOperatorBySymbol(symbol string) *OperatorDef {
	for i := range UserOperators {
		if UserOperators[i].Symbol == symbol {
			return &UserOperators[i]
		}
	}
	return nil
}

// GetUserOperatorByTrait returns the operator definition for a trait
func GetUserOperatorByTrait(trait string) *OperatorDef {
	for i := range UserOperators {
		if UserOperators[i].Trait == trait {
			return &UserOperators[i]
		}
	}
	return nil
}

// IsUserOperator checks if a symbol is a user-definable operator
func IsUserOperator(symbol string) bool {
	return GetUserOperatorBySymbol(symbol) != nil
}

// GetUserOperatorByTokenName returns the operator definition by token name
func GetUserOperatorByTokenName(tokenName string) *OperatorDef {
	for i := range UserOperators {
		if UserOperators[i].TokenName == tokenName {
			return &UserOperators[i]
		}
	}
	return nil
}
