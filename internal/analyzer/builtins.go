package analyzer

import (
	"fmt"

	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

// RegisterBuiltins registers the types of built-in functions into the symbol table.
func RegisterBuiltins(table *symbols.SymbolTable) {
	const prelude = "prelude" // Origin for all built-in symbols

	// Register primitive types as type constructors
	// These allow using Int, Float, etc. in type annotations like "type alias X = Int"
	table.Define("Int", typesystem.TType{Type: typesystem.Int}, prelude)
	table.Define("Float", typesystem.TType{Type: typesystem.Float}, prelude)
	table.Define("Bool", typesystem.TType{Type: typesystem.Bool}, prelude)
	table.Define("Char", typesystem.TType{Type: typesystem.Char}, prelude)
	table.Define("BigInt", typesystem.TType{Type: typesystem.BigInt}, prelude)
	table.Define("Rational", typesystem.TType{Type: typesystem.Rational}, prelude)
	table.Define("String", typesystem.TType{Type: typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{typesystem.Char},
	}}, prelude)

	// Register built-in traits
	registerBuiltinTraits(table)

	// Register virtual instances for primitives
	registerPrimitiveInstances(table)
	// print: (args...) -> Nil
	// Returns Nil (prints to stdout as side effect)
	// Evaluator returns first argument for convenience, but type is Nil
	printType := typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.TVar{Name: "a"}},
		ReturnType: typesystem.Nil,
		IsVariadic: true,
	}
	table.Define(config.PrintFuncName, printType, prelude)

	// write: (args...) -> Nil
	// Same as print but without trailing newline
	writeType := typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.TVar{Name: "a"}},
		ReturnType: typesystem.Nil,
		IsVariadic: true,
	}
	table.Define(config.WriteFuncName, writeType, prelude)

	// typeOf: (val: Any, type: Type) -> Bool
	typeOfType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TVar{Name: "val"},
			typesystem.TType{Type: typesystem.TVar{Name: "t"}},
		},
		ReturnType: typesystem.Bool,
		IsVariadic: false,
	}
	table.Define(config.TypeOfFuncName, typeOfType, prelude)

	// panic: (msg: String) -> a
	// It takes a String (List Char) and returns a generic type 'a' (Bottom/Never)
	// so it can be used in any expression.
	panicType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TApp{
				Constructor: typesystem.TCon{Name: config.ListTypeName},
				Args:        []typesystem.Type{typesystem.TCon{Name: "Char"}},
			},
		},
		ReturnType: typesystem.TVar{Name: "panic_ret"}, // Polymorphic return
		IsVariadic: false,
	}
	table.Define(config.PanicFuncName, panicType, prelude)

	// debug: (T) -> Nil - prints value with type and location
	debugType := typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.TVar{Name: "a"}},
		ReturnType: typesystem.Nil,
	}
	table.Define(config.DebugFuncName, debugType, prelude)

	// trace: (T) -> T - prints value with type and location, returns value
	traceType := typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.TVar{Name: "a"}},
		ReturnType: typesystem.TVar{Name: "a"},
	}
	table.Define(config.TraceFuncName, traceType, prelude)

	// fun len<T>(collection: T) -> Int
	// Accepts List or Tuple, checked at runtime
	lenType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TVar{Name: "a"},
		},
		ReturnType: typesystem.Int,
		IsVariadic: false,
	}
	table.Define(config.LenFuncName, lenType, prelude)

	// fun lenBytes(s: String) -> Int
	// Returns byte length of string (not character count)
	// len("Привет") = 6, lenBytes("Привет") = 12
	lenBytesType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TApp{
				Constructor: typesystem.TCon{Name: config.ListTypeName},
				Args:        []typesystem.Type{typesystem.Char},
			},
		},
		ReturnType: typesystem.Int,
		IsVariadic: false,
	}
	table.Define(config.LenBytesFuncName, lenBytesType, prelude)

	// getType: (val: t) -> Type<t>
	getTypeType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TVar{Name: "t"},
		},
		ReturnType: typesystem.TType{Type: typesystem.TVar{Name: "t"}},
		IsVariadic: false,
	}
	table.Define(config.GetTypeFuncName, getTypeType, prelude)

	// default: <T: Default>() -> T
	// Returns the default value for type T
	defaultType := typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.TType{Type: typesystem.TVar{Name: "t"}}},
		ReturnType: typesystem.TVar{Name: "t"},
		IsVariadic: false,
		Constraints: []typesystem.Constraint{
			{TypeVar: "t", Trait: "Default"},
		},
	}
	table.Define(config.DefaultFuncName, defaultType, prelude)

	// show: (a) -> String
	// Converts any value to its string representation
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{typesystem.Char},
	}
	showType := typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.TVar{Name: "a"}},
		ReturnType: stringType,
		IsVariadic: false,
	}
	table.Define(config.ShowFuncName, showType, prelude)

	// id: (a) -> a
	// Identity function - returns its argument unchanged
	idType := typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.TVar{Name: "a"}},
		ReturnType: typesystem.TVar{Name: "a"},
		IsVariadic: false,
	}
	table.Define(config.IdFuncName, idType, prelude)

	// const: (a, b) -> a
	// Constant function - returns first argument, ignores second
	constType := typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.TVar{Name: "a"}, typesystem.TVar{Name: "b"}},
		ReturnType: typesystem.TVar{Name: "a"},
		IsVariadic: false,
	}
	table.Define(config.ConstFuncName, constType, prelude)

	// intToFloat: (Int) -> Float
	intToFloatType := typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.Int},
		ReturnType: typesystem.Float,
	}
	table.Define("intToFloat", intToFloatType, prelude)

	// floatToInt: (Float) -> Int
	floatToIntType := typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.Float},
		ReturnType: typesystem.Int,
	}
	table.Define("floatToInt", floatToIntType, prelude)

	// sprintf: (format: String, args: ...Any) -> String
	sprintfType := typesystem.TFunc{
		Params: []typesystem.Type{
			stringType,
			typesystem.TVar{Name: "a"},
		},
		ReturnType: stringType,
		IsVariadic: true,
	}
	table.Define("sprintf", sprintfType, prelude)

	// read: (String, Type<T>) -> Option<T>
	// Parses a string into a typed value, returns Zero on failure
	readType := typesystem.TFunc{
		Params: []typesystem.Type{
			stringType, // String argument
			typesystem.TType{Type: typesystem.TVar{Name: "t"}}, // Type annotation
		},
		ReturnType: typesystem.TApp{
			Constructor: typesystem.TCon{Name: config.OptionTypeName},
			Args:        []typesystem.Type{typesystem.TVar{Name: "t"}},
		},
		IsVariadic: false,
	}
	table.Define(config.ReadFuncName, readType, prelude)

	// Register Nil as a value with type Nil (for use in expressions like `x = Nil`)
	table.Define("Nil", typesystem.Nil, prelude)
}

// registerBuiltinTraits registers the standard traits from config
func registerBuiltinTraits(table *symbols.SymbolTable) {
	const prelude = "prelude" // Origin for built-in traits

	// Helper types
	tvar := typesystem.TVar{Name: "T"}
	boolType := typesystem.Bool

	// Binary operator type: (T, T) -> T or (T, T) -> Bool
	binaryOp := func(ret typesystem.Type) typesystem.Type {
		return typesystem.TFunc{
			Params:     []typesystem.Type{tvar, tvar},
			ReturnType: ret,
		}
	}

	// Common type variables
	fVar := typesystem.TVar{Name: "F"}
	aVar := typesystem.TVar{Name: "A"}
	bVar := typesystem.TVar{Name: "B"}

	// Iterate over all built-in traits from config
	for _, trait := range config.BuiltinTraits {
		// 1. Define Trait
		table.DefineTrait(trait.Name, trait.TypeParams, trait.SuperTraits, prelude)

		// 2. Register Kind
		if trait.Kind == "* -> *" {
			table.RegisterKind(trait.Name, typesystem.KArrow{Left: typesystem.Star, Right: typesystem.Star})
		} else {
			// Default to * or handle other kinds if needed
			// Currently only * and * -> * are used in builtins
		}

		// 3. Register Operators
		for _, op := range trait.Operators {
			table.RegisterOperatorTrait(op, trait.Name)
		}

		// 4. Register Methods (Specific logic per trait)
		switch trait.Name {
		case "Equal":
			table.RegisterTraitMethod("(==)", "Equal", binaryOp(boolType), prelude)
			table.RegisterTraitMethod("(!=)", "Equal", binaryOp(boolType), prelude)

		case "Order":
			table.RegisterTraitMethod("(<)", "Order", binaryOp(boolType), prelude)
			table.RegisterTraitMethod("(>)", "Order", binaryOp(boolType), prelude)
			table.RegisterTraitMethod("(<=)", "Order", binaryOp(boolType), prelude)
			table.RegisterTraitMethod("(>=)", "Order", binaryOp(boolType), prelude)

		case "Numeric":
			for _, op := range []string{"+", "-", "*", "/", "%", "**"} {
				table.RegisterTraitMethod("("+op+")", "Numeric", binaryOp(tvar), prelude)
			}

		case "Bitwise":
			for _, op := range []string{"&", "|", "^", "<<", ">>"} {
				table.RegisterTraitMethod("("+op+")", "Bitwise", binaryOp(tvar), prelude)
			}

		case "Concat":
			table.RegisterTraitMethod("(++)", "Concat", binaryOp(tvar), prelude)

		case "Default":
			getDefaultMethodType := typesystem.TFunc{
				Params:     []typesystem.Type{tvar},
				ReturnType: tvar,
			}
			table.RegisterTraitMethod("default", "Default", getDefaultMethodType, prelude) // Deprecated name?
			table.RegisterTraitMethod("getDefault", "Default", getDefaultMethodType, prelude)

		case "Functor":
			// fmap : (A -> B) -> F<A> -> F<B>
			fmapType := typesystem.TFunc{
				Params: []typesystem.Type{
					typesystem.TFunc{Params: []typesystem.Type{aVar}, ReturnType: bVar}, // (A) -> B
					typesystem.TApp{Constructor: fVar, Args: []typesystem.Type{aVar}},   // F<A>
				},
				ReturnType: typesystem.TApp{Constructor: fVar, Args: []typesystem.Type{bVar}}, // F<B>
			}
			table.RegisterTraitMethod("fmap", "Functor", fmapType, prelude)
			table.RegisterTraitMethod2("Functor", "fmap")

		case "Applicative":
			// pure : A -> F<A>
			pureType := typesystem.TFunc{
				Params:     []typesystem.Type{aVar},
				ReturnType: typesystem.TApp{Constructor: fVar, Args: []typesystem.Type{aVar}},
			}
			table.RegisterTraitMethod("pure", "Applicative", pureType, prelude)
			table.RegisterTraitMethod2("Applicative", "pure")

			// (<*>) : F<(A) -> B> -> F<A> -> F<B>
			fAtoB := typesystem.TApp{
				Constructor: fVar,
				Args:        []typesystem.Type{typesystem.TFunc{Params: []typesystem.Type{aVar}, ReturnType: bVar}},
			}
			applyType := typesystem.TFunc{
				Params: []typesystem.Type{
					fAtoB,
					typesystem.TApp{Constructor: fVar, Args: []typesystem.Type{aVar}},
				},
				ReturnType: typesystem.TApp{Constructor: fVar, Args: []typesystem.Type{bVar}},
			}
			table.RegisterTraitMethod("(<*>)", "Applicative", applyType, prelude)
			table.RegisterTraitMethod2("Applicative", "(<*>)")

		case "Monad":
			// (>>=) : M<A> -> (A -> M<B>) -> M<B>
			mVar := typesystem.TVar{Name: "M"}
			mA := typesystem.TApp{Constructor: mVar, Args: []typesystem.Type{aVar}}
			mB := typesystem.TApp{Constructor: mVar, Args: []typesystem.Type{bVar}}
			aToMB := typesystem.TFunc{Params: []typesystem.Type{aVar}, ReturnType: mB}
			bindType := typesystem.TFunc{
				Params:     []typesystem.Type{mA, aToMB},
				ReturnType: mB,
			}
			table.RegisterTraitMethod("(>>=)", "Monad", bindType, prelude)
			table.RegisterTraitMethod2("Monad", "(>>=)")

		case "Semigroup":
			semigroupOp := typesystem.TFunc{
				Params:     []typesystem.Type{aVar, aVar},
				ReturnType: aVar,
			}
			table.RegisterTraitMethod("(<>)", "Semigroup", semigroupOp, prelude)
			table.RegisterTraitMethod2("Semigroup", "(<>)")

		case "Monoid":
			table.RegisterTraitMethod("mempty", "Monoid", aVar, prelude) // Returns A
			table.RegisterTraitMethod2("Monoid", "mempty")

		case "Empty":
			// isEmpty : F<A> -> Bool
			isEmptyType := typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.TApp{Constructor: fVar, Args: []typesystem.Type{aVar}}},
				ReturnType: typesystem.Bool,
			}
			table.RegisterTraitMethod("isEmpty", "Empty", isEmptyType, prelude)
			table.RegisterTraitMethod2("Empty", "isEmpty")

		case "Optional":
			// unwrap : F<A> -> A
			unwrapType := typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.TApp{Constructor: fVar, Args: []typesystem.Type{aVar}}},
				ReturnType: aVar,
			}
			table.RegisterTraitMethod("unwrap", "Optional", unwrapType, prelude)
			table.RegisterTraitMethod2("Optional", "unwrap")

			// wrap : A -> F<A>
			wrapType := typesystem.TFunc{
				Params:     []typesystem.Type{aVar},
				ReturnType: typesystem.TApp{Constructor: fVar, Args: []typesystem.Type{aVar}},
			}
			table.RegisterTraitMethod("wrap", "Optional", wrapType, prelude)
			table.RegisterTraitMethod2("Optional", "wrap")

		case "Iter":
			// iter: (C) -> () -> Option<T>
			iterReturnType := typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: typesystem.TApp{Constructor: typesystem.TCon{Name: config.OptionTypeName}, Args: []typesystem.Type{typesystem.TVar{Name: "T"}}},
			}
			iterMethodType := typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.TVar{Name: "C"}},
				ReturnType: iterReturnType,
			}
			table.RegisterTraitMethod(config.IterMethodName, config.IterTraitName, iterMethodType, prelude)
		}
	}

	// User-definable operator traits from centralized config
	// Skip those already handled above
	handledTraits := make(map[string]bool)
	for _, t := range config.BuiltinTraits {
		handledTraits[t.Name] = true
	}

	for _, op := range config.UserOperators {
		if handledTraits[op.Trait] {
			continue // Skip - already defined
		}
		// These traits are implicitly defined by config if not in BuiltinTraits?
		// The old code defined them here.
		if !table.IsDefined(op.Trait) {
			table.DefineTrait(op.Trait, []string{"T"}, nil, prelude)
		}
		table.RegisterOperatorTrait(op.Symbol, op.Trait)
		table.RegisterTraitMethod("("+op.Symbol+")", op.Trait, binaryOp(tvar), prelude)
	}
}

// registerPrimitiveInstances registers virtual instances for built-in types
func registerPrimitiveInstances(table *symbols.SymbolTable) {
	// Numeric types implement Equal, Order, Number
	numericTypes := []typesystem.Type{
		typesystem.Int,
		typesystem.Float,
		typesystem.BigInt,
		typesystem.Rational,
	}

	for _, t := range numericTypes {
		_ = table.RegisterImplementation("Equal", t)
		_ = table.RegisterImplementation("Order", t)
		_ = table.RegisterImplementation("Numeric", t)
		_ = table.RegisterImplementation("Default", t)
	}

	// Integer types implement Bitwise
	_ = table.RegisterImplementation("Bitwise", typesystem.Int)
	_ = table.RegisterImplementation("Bitwise", typesystem.BigInt)

	// Bool implements Equal, Order, Default (false < true)
	_ = table.RegisterImplementation("Equal", typesystem.Bool)
	_ = table.RegisterImplementation("Order", typesystem.Bool)
	_ = table.RegisterImplementation("Default", typesystem.Bool)

	// Char implements Equal, Order, Default
	_ = table.RegisterImplementation("Equal", typesystem.TCon{Name: "Char"})
	_ = table.RegisterImplementation("Order", typesystem.TCon{Name: "Char"})
	_ = table.RegisterImplementation("Default", typesystem.TCon{Name: "Char"})

	// List<T> implements Equal, Order, Default, Concat
	// This covers String (List<Char>) as well since String is just List<Char>
	listType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{typesystem.TVar{Name: "a"}},
	}
	_ = table.RegisterImplementation("Equal", listType)
	_ = table.RegisterImplementation("Order", listType)
	_ = table.RegisterImplementation("Default", listType)
	_ = table.RegisterImplementation("Concat", listType)

	// FP trait implementations for type constructors
	// List implements Empty, Semigroup, Monoid, Functor, Applicative, Monad
	listCon := typesystem.TCon{Name: config.ListTypeName}
	_ = table.RegisterImplementation("Empty", listCon)
	_ = table.RegisterImplementation("Semigroup", listType)
	_ = table.RegisterImplementation("Monoid", listType)
	_ = table.RegisterImplementation("Functor", listCon)
	_ = table.RegisterImplementation("Applicative", listCon)
	_ = table.RegisterImplementation("Monad", listCon)

	// Option implements Empty, Optional, Equal, Order, Default, Semigroup, Monoid, Functor, Applicative, Monad
	optionCon := typesystem.TCon{Name: config.OptionTypeName}
	optionType := typesystem.TApp{
		Constructor: optionCon,
		Args:        []typesystem.Type{typesystem.TVar{Name: "a"}},
	}
	_ = table.RegisterImplementation("Empty", optionCon)
	_ = table.RegisterImplementation("Optional", optionCon)
	_ = table.RegisterImplementation("Equal", optionType)
	_ = table.RegisterImplementation("Order", optionType)
	_ = table.RegisterImplementation("Default", optionType)
	_ = table.RegisterImplementation("Semigroup", optionType)
	_ = table.RegisterImplementation("Monoid", optionType)
	_ = table.RegisterImplementation("Functor", optionCon)
	_ = table.RegisterImplementation("Applicative", optionCon)
	_ = table.RegisterImplementation("Monad", optionCon)

	// Register Optional instance methods for Option
	// unwrap: Option<A> -> A
	optionUnwrapType := typesystem.TFunc{
		Params:     []typesystem.Type{optionType},
		ReturnType: typesystem.TVar{Name: "a"},
	}
	table.RegisterInstanceMethod("Optional", config.OptionTypeName, "unwrap", optionUnwrapType)

	// Result implements Empty, Optional, Equal, Semigroup, Functor, Applicative, Monad
	// Result<E, A> - E is error (first), A is success (last, for Functor/Monad)
	resultCon := typesystem.TCon{Name: config.ResultTypeName}
	resultType := typesystem.TApp{
		Constructor: resultCon,
		Args:        []typesystem.Type{typesystem.TVar{Name: "e"}, typesystem.TVar{Name: "a"}},
	}
	_ = table.RegisterImplementation("Empty", resultCon)
	_ = table.RegisterImplementation("Optional", resultCon)
	_ = table.RegisterImplementation("Equal", resultType)
	_ = table.RegisterImplementation("Semigroup", resultType)
	_ = table.RegisterImplementation("Functor", resultCon)
	_ = table.RegisterImplementation("Applicative", resultCon)
	_ = table.RegisterImplementation("Monad", resultCon)

	// Register Optional instance methods for Result
	// unwrap: Result<E, A> -> A (last type param)
	resultUnwrapType := typesystem.TFunc{
		Params:     []typesystem.Type{resultType},
		ReturnType: typesystem.TVar{Name: "a"},
	}
	table.RegisterInstanceMethod("Optional", config.ResultTypeName, "unwrap", resultUnwrapType)

	// Tuple implements Equal, Order (lexicographic)
	// Register for common arities (2, 3, 4)
	for arity := 2; arity <= 4; arity++ {
		args := make([]typesystem.Type, arity)
		for i := 0; i < arity; i++ {
			args[i] = typesystem.TVar{Name: fmt.Sprintf("t%d", i)}
		}
		tupleType := typesystem.TTuple{Elements: args}
		_ = table.RegisterImplementation("Equal", tupleType)
		_ = table.RegisterImplementation("Order", tupleType)
	}

	// Note: String (List<Char>) is covered by List<T> above for all traits
	// including Equal, Order, Default, Concat, Semigroup, Monoid

	// Note: Functor instances are NOT pre-registered here.
	// Users must define instance methods themselves.
	// The Functor trait is built-in but instances require explicit implementation.

	// Nil implements Default
	_ = table.RegisterImplementation("Default", typesystem.Nil)

	// Map<K, V> implements Empty, Semigroup, Monoid, Equal
	mapCon := typesystem.TCon{Name: config.MapTypeName}
	mapType := typesystem.TApp{
		Constructor: mapCon,
		Args:        []typesystem.Type{typesystem.TVar{Name: "k"}, typesystem.TVar{Name: "v"}},
	}
	_ = table.RegisterImplementation("Empty", mapCon)
	_ = table.RegisterImplementation("Semigroup", mapType)
	_ = table.RegisterImplementation("Monoid", mapType)
	_ = table.RegisterImplementation("Equal", mapType)

	// Bytes implements Equal, Order, Concat
	bytesCon := typesystem.TCon{Name: config.BytesTypeName}
	_ = table.RegisterImplementation("Equal", bytesCon)
	_ = table.RegisterImplementation("Order", bytesCon)
	_ = table.RegisterImplementation("Concat", bytesCon)

	// Bits implements Equal, Concat
	bitsCon := typesystem.TCon{Name: config.BitsTypeName}
	_ = table.RegisterImplementation("Equal", bitsCon)
	_ = table.RegisterImplementation("Concat", bitsCon)

	// Uuid implements Equal
	uuidCon := typesystem.TCon{Name: "Uuid"}
	_ = table.RegisterImplementation("Equal", uuidCon)

	// Iter implementation for List
	_ = table.RegisterImplementation("Iter", listType)
}
