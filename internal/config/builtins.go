package config

// Builtins Configuration
//
// This is the SINGLE SOURCE OF TRUTH for all built-in types, functions, and traits.
// Documentation is generated from this file.

// ============================================================================
// Built-in Types
// ============================================================================

type TypeInfo struct {
	Name         string // e.g., "Int", "Option"
	Kind         string // e.g., "*", "* -> *", "* -> * -> *"
	Description  string
	Example      string   // optional
	Constructors []string // for ADTs
}

var BuiltinTypes = []TypeInfo{
	// Primitives
	{Name: "Int", Kind: "*", Description: "64-bit signed integer"},
	{Name: "Float", Kind: "*", Description: "64-bit floating point number"},
	{Name: "Bool", Kind: "*", Description: "Boolean (true/false)"},
	{Name: "Char", Kind: "*", Description: "Unicode character"},
	{Name: "Nil", Kind: "*", Description: "Unit type with single value Nil"},
	{Name: "BigInt", Kind: "*", Description: "Arbitrary precision integer"},
	{Name: "Rational", Kind: "*", Description: "Arbitrary precision rational number"},

	// Type constructors
	{Name: "List", Kind: "* -> *", Description: "Linked list of elements"},
	{Name: "Map", Kind: "* -> * -> *", Description: "Immutable hash map (HAMT-based)"},
	{Name: "Bytes", Kind: "*", Description: "Immutable byte sequence"},
	{Name: "Bits", Kind: "*", Description: "Immutable bit sequence for binary protocols"},
	{Name: "Option", Kind: "* -> *", Description: "Optional value: Some(x) or Zero",
		Example: "Some(42), Zero", Constructors: []string{"Some(T)", "Zero"}},
	{Name: "Result", Kind: "* -> * -> *", Description: "Success or failure: Ok(x) or Fail(e)",
		Example: "Ok(42), Fail(\"error\")", Constructors: []string{"Ok(T)", "Fail(E)"}},

	// Aliases
	{Name: "String", Kind: "= List<Char>", Description: "String is a list of characters"},
}

// GetTypeInfo returns type info by name
func GetTypeInfo(name string) *TypeInfo {
	for i := range BuiltinTypes {
		if BuiltinTypes[i].Name == name {
			return &BuiltinTypes[i]
		}
	}
	return nil
}

// ============================================================================
// Built-in Traits
// ============================================================================

type TraitInfo struct {
	Name        string   // e.g., "Equal", "Functor"
	TypeParams  []string // e.g., ["T"], ["F"]
	Kind        string   // e.g., "*", "* -> *"
	SuperTraits []string // e.g., ["Equal"] for Order
	Operators   []string // operators bound to this trait
	Methods     []string // method names
	Description string
}

var BuiltinTraits = []TraitInfo{
	// Basic traits
	{Name: "Equal", TypeParams: []string{"T"}, Kind: "*",
		Operators: []string{"==", "!="}, Description: "Equality comparison"},
	{Name: "Order", TypeParams: []string{"T"}, Kind: "*", SuperTraits: []string{"Equal"},
		Operators: []string{"<", ">", "<=", ">="}, Description: "Ordering comparison"},
	{Name: "Numeric", TypeParams: []string{"T"}, Kind: "*",
		Operators: []string{"+", "-", "*", "/", "%", "**"}, Description: "Numeric operations"},
	{Name: "Bitwise", TypeParams: []string{"T"}, Kind: "*",
		Operators: []string{"&", "|", "^", "<<", ">>"}, Description: "Bitwise operations"},
	{Name: "Concat", TypeParams: []string{"T"}, Kind: "*",
		Operators: []string{"++"}, Description: "Concatenation"},
	{Name: "Default", TypeParams: []string{"T"}, Kind: "*",
		Methods: []string{"default"}, Description: "Default value for type"},

	// FP traits (HKT)
	{Name: "Semigroup", TypeParams: []string{"A"}, Kind: "*",
		Operators: []string{"<>"}, Description: "Associative binary operation"},
	{Name: "Monoid", TypeParams: []string{"A"}, Kind: "*", SuperTraits: []string{"Semigroup"},
		Methods: []string{"mempty"}, Description: "Semigroup with identity"},
	{Name: "Functor", TypeParams: []string{"F"}, Kind: "* -> *",
		Methods: []string{"fmap"}, Description: "Mappable containers"},
	{Name: "Applicative", TypeParams: []string{"F"}, Kind: "* -> *", SuperTraits: []string{"Functor"},
		Operators: []string{"<*>"}, Methods: []string{"pure"}, Description: "Functor with application"},
	{Name: "Monad", TypeParams: []string{"M"}, Kind: "* -> *", SuperTraits: []string{"Applicative"},
		Operators: []string{">>="}, Description: "Chainable computations"},
	{Name: "Empty", TypeParams: []string{"F"}, Kind: "* -> *",
		Methods:     []string{"isEmpty"},
		Description: "Containers that can be empty. Methods: isEmpty(F<A>) -> Bool"},
	{Name: "Optional", TypeParams: []string{"F"}, Kind: "* -> *", SuperTraits: []string{"Empty"},
		Operators: []string{"??", "?."}, Methods: []string{"unwrap", "wrap"},
		Description: "Optional values (Option, Result). Methods: unwrap(F<A>) -> A, wrap(A) -> F<A>"},
	{Name: "Iter", TypeParams: []string{"C", "T"}, Kind: "*",
		Methods: []string{"iter"}, Description: "Iterable containers (for loops). Methods: iter(C) -> () -> Option<T>"},
}

// GetTraitInfo returns trait info by name
func GetTraitInfo(name string) *TraitInfo {
	for i := range BuiltinTraits {
		if BuiltinTraits[i].Name == name {
			return &BuiltinTraits[i]
		}
	}
	return nil
}

// ============================================================================
// Built-in Functions (prelude)
// ============================================================================

type FunctionInfo struct {
	Name        string
	Signature   string
	Description string
	Example     string // optional
	Category    string
	Constraint  string // e.g., "Default<T>" for default function
}

var BuiltinFunctions = []FunctionInfo{
	// IO
	{Name: "print", Signature: "(...T) -> Nil", Description: "Print values to stdout with newline", Category: "IO"},
	{Name: "write", Signature: "(...T) -> Nil", Description: "Print values to stdout without newline", Category: "IO"},

	// Error
	{Name: "panic", Signature: "(String) -> !", Description: "Terminate with error message", Category: "Error"},

	// Debug
	{Name: "debug", Signature: "(T) -> Nil", Description: "Print value with type and location to stderr", Category: "Debug"},
	{Name: "trace", Signature: "(T) -> T", Description: "Print value with type and location, return value (for pipes)", Category: "Debug"},

	// Collection
	{Name: "len", Signature: "(a) -> Int", Description: "Length of List, Tuple, Map, Bytes or Bits", Category: "Collection"},
	{Name: "lenBytes", Signature: "(String) -> Int", Description: "Byte length of string (UTF-8)", Category: "String"},

	// Conversion
	{Name: "show", Signature: "(T) -> String", Description: "Convert value to string", Category: "Conversion"},
	{Name: "read", Signature: "(String, Type) -> Option<T>", Description: "Parse string to type",
		Example: "read(\"42\", Int)", Category: "Conversion"},
	{Name: "intToFloat", Signature: "(Int) -> Float", Description: "Convert Int to Float", Category: "Conversion"},
	{Name: "floatToInt", Signature: "(Float) -> Int", Description: "Convert Float to Int (truncate)", Category: "Conversion"},
	{Name: "sprintf", Signature: "(String, ...T) -> String", Description: "Format string (printf style)", Category: "String"},

	// Reflection
	{Name: "getType", Signature: "(T) -> String", Description: "Get type name of value", Category: "Reflection"},
	{Name: "typeOf", Signature: "(T, Type) -> Bool", Description: "Check if value matches type", Category: "Reflection"},

	// Function combinators
	{Name: "id", Signature: "(T) -> T", Description: "Identity function", Category: "Function"},
	{Name: "const", Signature: "(A, B) -> A", Description: "Return first argument, ignore second", Category: "Function"},

	// Trait methods
	{Name: "default", Signature: "(Type) -> T", Description: "Get default value for type",
		Example: "default(Int) // 0", Category: "Trait", Constraint: "Default<T>"},
	{Name: "fmap", Signature: "((A)->B, F<A>) -> F<B>", Description: "Map over container",
		Category: "Trait", Constraint: "Functor<F>"},
	{Name: "pure", Signature: "(A) -> F<A>", Description: "Lift into container",
		Category: "Trait", Constraint: "Applicative<F>"},
	{Name: "mempty", Signature: "() -> T", Description: "Identity element",
		Category: "Trait", Constraint: "Monoid<T>"},

	// Option
	{Name: "isSome", Signature: "(Option<T>) -> Bool", Description: "Check if Option is Some", Category: "Option"},
	{Name: "isZero", Signature: "(Option<T>) -> Bool", Description: "Check if Option is Zero", Category: "Option"},
	{Name: "unwrap", Signature: "(Option<T>) -> T", Description: "Extract value from Some (panics on Zero)", Category: "Option"},
	{Name: "unwrapOr", Signature: "(Option<T>, T) -> T", Description: "Extract value or return default", Category: "Option"},
	{Name: "unwrapOrElse", Signature: "(Option<T>, () -> T) -> T", Description: "Extract value or compute default", Category: "Option"},

	// Result
	{Name: "isOk", Signature: "(Result<E, T>) -> Bool", Description: "Check if Result is Ok", Category: "Result"},
	{Name: "isFail", Signature: "(Result<E, T>) -> Bool", Description: "Check if Result is Fail", Category: "Result"},
	{Name: "unwrapResult", Signature: "(Result<E, T>) -> T", Description: "Extract value from Ok (panics on Fail)", Category: "Result"},
	{Name: "unwrapError", Signature: "(Result<E, T>) -> E", Description: "Extract error from Fail (panics on Ok)", Category: "Result"},
	{Name: "unwrapResultOr", Signature: "(Result<E, T>, T) -> T", Description: "Extract value or return default", Category: "Result"},
}

// GetFunctionInfo returns function info by name
func GetFunctionInfo(name string) *FunctionInfo {
	for i := range BuiltinFunctions {
		if BuiltinFunctions[i].Name == name {
			return &BuiltinFunctions[i]
		}
	}
	return nil
}

// GetFunctionsByCategory returns functions grouped by category
func GetFunctionsByCategory() map[string][]FunctionInfo {
	result := make(map[string][]FunctionInfo)
	for _, fn := range BuiltinFunctions {
		result[fn.Category] = append(result[fn.Category], fn)
	}
	return result
}

// ============================================================================
// Standard library package names
// ============================================================================

// Virtual package paths
const (
	ListPackagePath   = "lib/list"
	StringPackagePath = "lib/string"
	TimePackagePath   = "lib/time"
	IOPackagePath     = "lib/io"
	SysPackagePath    = "lib/sys"
	TuplePackagePath  = "lib/tuple"
	MathPackagePath   = "lib/math"
	BignumPackagePath = "lib/bignum"
	CharPackagePath   = "lib/char"
	TestPackagePath   = "lib/test"
)

// Note: Type names (ListTypeName, OptionTypeName, ResultTypeName) are in constants.go
