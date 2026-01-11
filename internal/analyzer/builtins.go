package analyzer

import (
	"fmt"
	"strings"
	"sync"

	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

var builtinsOnce sync.Once

// RegisterBuiltins registers the types of built-in functions into the prelude symbol table.
// This function is idempotent - it only registers once, subsequent calls are no-ops.
// The 'table' parameter is ignored; registration always goes to prelude.
func RegisterBuiltins(table *symbols.SymbolTable) {
	builtinsOnce.Do(func() {
		registerBuiltinsToPrelude()
	})
}

// ResetBuiltins resets the builtins registration (for testing only).
// Must be called together with symbols.ResetPrelude().
func ResetBuiltins() {
	builtinsOnce = sync.Once{}
}

// registerBuiltinsToPrelude does the actual registration to prelude.
func registerBuiltinsToPrelude() {
	table := symbols.GetPrelude()
	const prelude = "prelude" // Origin for all built-in symbols

	// Register primitive types as type constructors
	// These allow using Int, Float, etc. in type annotations like "type alias X = Int"
	table.DefineConstant("Int", typesystem.TType{Type: typesystem.Int}, prelude)
	table.DefineConstant("Float", typesystem.TType{Type: typesystem.Float}, prelude)
	table.DefineConstant("Bool", typesystem.TType{Type: typesystem.Bool}, prelude)
	table.DefineConstant("Char", typesystem.TType{Type: typesystem.Char}, prelude)
	table.DefineConstant("BigInt", typesystem.TType{Type: typesystem.BigInt}, prelude)
	table.DefineConstant("Rational", typesystem.TType{Type: typesystem.Rational}, prelude)
	table.DefineConstant("String", typesystem.TType{Type: typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{typesystem.Char},
	}}, prelude)
	table.DefineConstant("Reader", typesystem.TType{Type: typesystem.TCon{Name: "Reader"}}, prelude)
	table.DefineConstant("Identity", typesystem.TType{Type: typesystem.TCon{Name: "Identity"}}, prelude)
	table.DefineConstant("State", typesystem.TType{Type: typesystem.TCon{Name: "State"}}, prelude)
	table.DefineConstant("Writer", typesystem.TType{Type: typesystem.TCon{Name: "Writer"}}, prelude)
	table.DefineConstant("OptionT", typesystem.TType{Type: typesystem.TCon{Name: "OptionT"}}, prelude)
	table.DefineConstant("ResultT", typesystem.TType{Type: typesystem.TCon{Name: "ResultT"}}, prelude)
	table.DefineConstant("Range", typesystem.TType{Type: typesystem.TCon{Name: "Range", KindVal: typesystem.KArrow{Left: typesystem.Star, Right: typesystem.Star}}}, prelude)
	// Register Kind for Range
	table.RegisterKind("Range", typesystem.KArrow{Left: typesystem.Star, Right: typesystem.Star})

	// Register Kinds for Transformers
	// OptionT :: (*->*) -> * -> *
	hkt := typesystem.KArrow{Left: typesystem.Star, Right: typesystem.Star}
	optKind := typesystem.KArrow{Left: hkt, Right: typesystem.KArrow{Left: typesystem.Star, Right: typesystem.Star}}
	table.RegisterKind("OptionT", optKind)

	// ResultT :: (*->*) -> * -> * -> *
	resKind := typesystem.KArrow{
		Left: hkt,
		Right: typesystem.KArrow{
			Left: typesystem.Star,
			Right: typesystem.KArrow{
				Left:  typesystem.Star,
				Right: typesystem.Star,
			},
		},
	}
	table.RegisterKind("ResultT", resKind)

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
	table.DefineConstant(config.PrintFuncName, printType, prelude)

	// write: (args...) -> Nil
	// Same as print but without trailing newline
	writeType := typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.TVar{Name: "a"}},
		ReturnType: typesystem.Nil,
		IsVariadic: true,
	}
	table.DefineConstant(config.WriteFuncName, writeType, prelude)

	// String type helper (List<Char>)
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{typesystem.Char},
	}

	// typeOf: (val: Any, type: Type) -> Bool
	typeOfType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TVar{Name: "val"},
			typesystem.TType{Type: typesystem.TVar{Name: "t", KindVal: typesystem.AnyKind}},
		},
		ReturnType: typesystem.Bool,
		IsVariadic: false,
	}
	table.DefineConstant(config.TypeOfFuncName, typeOfType, prelude)

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
	table.DefineConstant(config.PanicFuncName, panicType, prelude)

	// debug: (T) -> Nil - prints value with type and location
	debugType := typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.TVar{Name: "a"}},
		ReturnType: typesystem.Nil,
	}
	table.DefineConstant(config.DebugFuncName, debugType, prelude)

	// trace: (T) -> T - prints value with type and location, returns value
	traceType := typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.TVar{Name: "a"}},
		ReturnType: typesystem.TVar{Name: "a"},
	}
	table.DefineConstant(config.TraceFuncName, traceType, prelude)

	// fun len<T>(collection: T) -> Int
	// Accepts List or Tuple, checked at runtime
	lenType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TVar{Name: "a"},
		},
		ReturnType: typesystem.Int,
		IsVariadic: false,
	}
	table.DefineConstant(config.LenFuncName, lenType, prelude)

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
	table.DefineConstant(config.LenBytesFuncName, lenBytesType, prelude)

	// getType: (val: t) -> Type<t>
	getTypeType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TVar{Name: "t", KindVal: typesystem.AnyKind},
		},
		ReturnType: typesystem.TType{Type: typesystem.TVar{Name: "t", KindVal: typesystem.AnyKind}},
		IsVariadic: false,
	}
	table.DefineConstant(config.GetTypeFuncName, getTypeType, prelude)

	// kindOf: (T) -> String
	kindOfType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TVar{Name: "t", KindVal: typesystem.AnyKind},
		},
		ReturnType: stringType,
	}
	table.DefineConstant("kindOf", kindOfType, prelude)

	// debugType: (T) -> String
	debugTypeType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TVar{Name: "t", KindVal: typesystem.AnyKind},
		},
		ReturnType: stringType,
	}
	table.DefineConstant("debugType", debugTypeType, prelude)

	// debugRepr: (T) -> String
	debugReprType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TVar{Name: "t", KindVal: typesystem.AnyKind},
		},
		ReturnType: stringType,
	}
	table.DefineConstant("debugRepr", debugReprType, prelude)

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
	table.DefineConstant(config.DefaultFuncName, defaultType, prelude)

	// show is now a trait method (Show trait), registered in registerBuiltinTraits

	// id: forall a. (a) -> a
	// Identity function - returns its argument unchanged
	// Using TForall to preserve polymorphism for Rank-N usage
	idType := typesystem.TForall{
		Vars: []typesystem.TVar{{Name: "a"}},
		Type: typesystem.TFunc{
			Params:     []typesystem.Type{typesystem.TVar{Name: "a"}},
			ReturnType: typesystem.TVar{Name: "a"},
			IsVariadic: false,
		},
	}
	table.DefineConstant(config.IdFuncName, idType, prelude)

	// const: (a, b) -> a
	// Constant function - returns first argument, ignores second
	constType := typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.TVar{Name: "a"}, typesystem.TVar{Name: "b"}},
		ReturnType: typesystem.TVar{Name: "a"},
		IsVariadic: false,
	}
	table.DefineConstant(config.ConstFuncName, constType, prelude)

	// intToFloat: (Int) -> Float
	intToFloatType := typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.Int},
		ReturnType: typesystem.Float,
	}
	table.DefineConstant("intToFloat", intToFloatType, prelude)

	// floatToInt: (Float) -> Int
	floatToIntType := typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.Float},
		ReturnType: typesystem.Int,
	}
	table.DefineConstant("floatToInt", floatToIntType, prelude)

	// sprintf: (format: String, args: ...Any) -> String
	sprintfType := typesystem.TFunc{
		Params: []typesystem.Type{
			stringType,
			typesystem.TVar{Name: "a"},
		},
		ReturnType: stringType,
		IsVariadic: true,
	}
	table.DefineConstant("sprintf", sprintfType, prelude)

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
	table.DefineConstant(config.ReadFuncName, readType, prelude)

	// reader: (E -> A) -> Reader<E, A>
	readerType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.TVar{Name: "E"}},
				ReturnType: typesystem.TVar{Name: "A"},
			},
		},
		ReturnType: typesystem.TApp{
			Constructor: typesystem.TCon{Name: "Reader"},
			Args:        []typesystem.Type{typesystem.TVar{Name: "E"}, typesystem.TVar{Name: "A"}},
		},
	}
	table.DefineConstant("reader", readerType, prelude)

	// runReader: (Reader<E, A>, E) -> A
	runReaderType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TApp{
				Constructor: typesystem.TCon{Name: "Reader"},
				Args:        []typesystem.Type{typesystem.TVar{Name: "E"}, typesystem.TVar{Name: "A"}},
			},
			typesystem.TVar{Name: "E"},
		},
		ReturnType: typesystem.TVar{Name: "A"},
	}
	table.DefineConstant("runReader", runReaderType, prelude)

	// Identity functions
	// identity: (A) -> Identity<A>
	identityType := typesystem.TFunc{
		Params: []typesystem.Type{typesystem.TVar{Name: "A"}},
		ReturnType: typesystem.TApp{
			Constructor: typesystem.TCon{Name: "Identity"},
			Args:        []typesystem.Type{typesystem.TVar{Name: "A"}},
		},
	}
	table.DefineConstant("identity", identityType, prelude)

	// runIdentity: (Identity<A>) -> A
	runIdentityType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TApp{
				Constructor: typesystem.TCon{Name: "Identity"},
				Args:        []typesystem.Type{typesystem.TVar{Name: "A"}},
			},
		},
		ReturnType: typesystem.TVar{Name: "A"},
	}
	table.DefineConstant("runIdentity", runIdentityType, prelude)

	// State functions
	// state: (S -> (A, S)) -> State<S, A>
	stateType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TFunc{
				Params: []typesystem.Type{typesystem.TVar{Name: "S"}},
				ReturnType: typesystem.TTuple{
					Elements: []typesystem.Type{typesystem.TVar{Name: "A"}, typesystem.TVar{Name: "S"}},
				},
			},
		},
		ReturnType: typesystem.TApp{
			Constructor: typesystem.TCon{Name: "State"},
			Args:        []typesystem.Type{typesystem.TVar{Name: "S"}, typesystem.TVar{Name: "A"}},
		},
	}
	table.DefineConstant("state", stateType, prelude)

	// runState: (State<S, A>, S) -> (A, S)
	runStateType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TApp{
				Constructor: typesystem.TCon{Name: "State"},
				Args:        []typesystem.Type{typesystem.TVar{Name: "S"}, typesystem.TVar{Name: "A"}},
			},
			typesystem.TVar{Name: "S"},
		},
		ReturnType: typesystem.TTuple{
			Elements: []typesystem.Type{typesystem.TVar{Name: "A"}, typesystem.TVar{Name: "S"}},
		},
	}
	table.DefineConstant("runState", runStateType, prelude)

	// evalState: (State<S, A>, S) -> A
	evalStateType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TApp{
				Constructor: typesystem.TCon{Name: "State"},
				Args:        []typesystem.Type{typesystem.TVar{Name: "S"}, typesystem.TVar{Name: "A"}},
			},
			typesystem.TVar{Name: "S"},
		},
		ReturnType: typesystem.TVar{Name: "A"},
	}
	table.DefineConstant("evalState", evalStateType, prelude)

	// execState: (State<S, A>, S) -> S
	execStateType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TApp{
				Constructor: typesystem.TCon{Name: "State"},
				Args:        []typesystem.Type{typesystem.TVar{Name: "S"}, typesystem.TVar{Name: "A"}},
			},
			typesystem.TVar{Name: "S"},
		},
		ReturnType: typesystem.TVar{Name: "S"},
	}
	table.DefineConstant("execState", execStateType, prelude)

	// sGet: () -> State<S, S>
	sGetType := typesystem.TFunc{
		Params: []typesystem.Type{},
		ReturnType: typesystem.TApp{
			Constructor: typesystem.TCon{Name: "State"},
			Args:        []typesystem.Type{typesystem.TVar{Name: "S"}, typesystem.TVar{Name: "S"}},
		},
	}
	table.DefineConstant("sGet", sGetType, prelude)

	// sPut: (S) -> State<S, ()>
	sPutType := typesystem.TFunc{
		Params: []typesystem.Type{typesystem.TVar{Name: "S"}},
		ReturnType: typesystem.TApp{
			Constructor: typesystem.TCon{Name: "State"},
			Args:        []typesystem.Type{typesystem.TVar{Name: "S"}, typesystem.Nil},
		},
	}
	table.DefineConstant("sPut", sPutType, prelude)

	// Writer functions
	// writer: (A, W) -> Writer<W, A>
	writerType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TVar{Name: "A"},
			typesystem.TVar{Name: "W"},
		},
		ReturnType: typesystem.TApp{
			Constructor: typesystem.TCon{Name: "Writer"},
			Args:        []typesystem.Type{typesystem.TVar{Name: "W"}, typesystem.TVar{Name: "A"}},
		},
	}
	table.DefineConstant("writer", writerType, prelude)

	// runWriter: (Writer<W, A>) -> (A, W)
	runWriterType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TApp{
				Constructor: typesystem.TCon{Name: "Writer"},
				Args:        []typesystem.Type{typesystem.TVar{Name: "W"}, typesystem.TVar{Name: "A"}},
			},
		},
		ReturnType: typesystem.TTuple{
			Elements: []typesystem.Type{typesystem.TVar{Name: "A"}, typesystem.TVar{Name: "W"}},
		},
	}
	table.DefineConstant("runWriter", runWriterType, prelude)

	// execWriter: (Writer<W, A>) -> W
	execWriterType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TApp{
				Constructor: typesystem.TCon{Name: "Writer"},
				Args:        []typesystem.Type{typesystem.TVar{Name: "W"}, typesystem.TVar{Name: "A"}},
			},
		},
		ReturnType: typesystem.TVar{Name: "W"},
	}
	table.DefineConstant("execWriter", execWriterType, prelude)

	// wTell: (W) -> Writer<W, ()>
	wTellType := typesystem.TFunc{
		Params: []typesystem.Type{typesystem.TVar{Name: "W"}},
		ReturnType: typesystem.TApp{
			Constructor: typesystem.TCon{Name: "Writer"},
			Args:        []typesystem.Type{typesystem.TVar{Name: "W"}, typesystem.Nil},
		},
	}
	table.DefineConstant("wTell", wTellType, prelude)

	// optionT: (M<Option<A>>) -> OptionT<M, A>
	mVar := typesystem.TVar{Name: "M", KindVal: hkt}
	aVar := typesystem.TVar{Name: "A"}

	optionTConstructorType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TApp{
				Constructor: mVar,
				Args: []typesystem.Type{
					typesystem.TApp{
						Constructor: typesystem.TCon{Name: config.OptionTypeName},
						Args:        []typesystem.Type{aVar},
					},
				},
			},
		},
		ReturnType: typesystem.TApp{
			Constructor: typesystem.TCon{Name: "OptionT"},
			Args:        []typesystem.Type{mVar, aVar},
		},
	}
	table.DefineConstant("optionT", optionTConstructorType, prelude)

	// resultT: (M<Result<A, E>>) -> ResultT<M, A>
	// Note: We used 3 args for ResultT in ReturnType: <M, E, A>
	eVar := typesystem.TVar{Name: "E"}
	resultTConstructorType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TApp{
				Constructor: mVar,
				Args: []typesystem.Type{
					typesystem.TApp{
						Constructor: typesystem.TCon{Name: config.ResultTypeName},
						Args:        []typesystem.Type{eVar, aVar},
					},
				},
			},
		},
		ReturnType: typesystem.TApp{
			Constructor: typesystem.TCon{Name: "ResultT"},
			Args:        []typesystem.Type{mVar, eVar, aVar},
		},
	}
	table.DefineConstant("resultT", resultTConstructorType, prelude)

	// runOptionT: (OptionT<M, A>) -> M<Option<A>>
	runOptionTType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TApp{
				Constructor: typesystem.TCon{Name: "OptionT"},
				Args:        []typesystem.Type{mVar, aVar},
			},
		},
		ReturnType: typesystem.TApp{
			Constructor: mVar,
			Args: []typesystem.Type{
				typesystem.TApp{
					Constructor: typesystem.TCon{Name: config.OptionTypeName},
					Args:        []typesystem.Type{aVar},
				},
			},
		},
	}
	table.DefineConstant("runOptionT", runOptionTType, prelude)

	// runResultT: (ResultT<M, E, A>) -> M<Result<A, E>>
	runResultTType := typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TApp{
				Constructor: typesystem.TCon{Name: "ResultT"},
				Args:        []typesystem.Type{mVar, eVar, aVar},
			},
		},
		ReturnType: typesystem.TApp{
			Constructor: mVar,
			Args: []typesystem.Type{
				typesystem.TApp{
					Constructor: typesystem.TCon{Name: config.ResultTypeName},
					Args:        []typesystem.Type{eVar, aVar},
				},
			},
		},
	}
	table.DefineConstant("runResultT", runResultTType, prelude)

	// Register Nil as a value with type Nil (for use in expressions like `x = Nil`)
	table.DefineConstant("Nil", typesystem.Nil, prelude)

	// Register Dictionary type
	table.InitDictionaryType()

	// Internal builtin for dictionary creation (Analyzer usage)
	// __make_dictionary(name: String, methods: Tuple/Any, supers: List<Dictionary>) -> Dictionary
	makeDictType := typesystem.TFunc{
		Params: []typesystem.Type{
			stringType,
			typesystem.TVar{Name: "methods"}, // Any type (Tuple)
			typesystem.TApp{Constructor: typesystem.TCon{Name: config.ListTypeName}, Args: []typesystem.Type{typesystem.TCon{Name: "Dictionary"}}},
		},
		ReturnType: typesystem.TCon{Name: "Dictionary"},
	}
	table.DefineConstant("__make_dictionary", makeDictType, prelude)
}

// registerBuiltinTraits registers the standard traits from config
func registerBuiltinTraits(table *symbols.SymbolTable) {
	const prelude = "prelude" // Origin for built-in traits

	// Helper types
	tvar := typesystem.TVar{Name: "t"}
	boolType := typesystem.Bool
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{typesystem.Char},
	}

	// Binary operator type: (T, T) -> T or (T, T) -> Bool
	binaryOp := func(ret typesystem.Type) typesystem.Type {
		return typesystem.TFunc{
			Params:     []typesystem.Type{tvar, tvar},
			ReturnType: ret,
		}
	}

	// Common type variables
	// F and M are higher-kinded (* -> *)
	hkt := typesystem.MakeArrow(typesystem.Star, typesystem.Star)
	fVar := typesystem.TVar{Name: "f", KindVal: hkt}
	mVar := typesystem.TVar{Name: "m", KindVal: hkt}

	aVar := typesystem.TVar{Name: "a"} // Star
	bVar := typesystem.TVar{Name: "b"} // Star
	// tvar is already defined above
	for _, trait := range config.BuiltinTraits {
		// Convert params to lowercase to match language conventions
		var lowerParams []string
		for _, p := range trait.TypeParams {
			lowerParams = append(lowerParams, strings.ToLower(p))
		}

		// Define Trait
		table.DefineTrait(trait.Name, lowerParams, trait.SuperTraits, prelude)

		// Register Kind
		var traitParamKind typesystem.Kind = typesystem.Star
		if trait.Kind == "* -> *" {
			traitParamKind = typesystem.KArrow{Left: typesystem.Star, Right: typesystem.Star}
			table.RegisterKind(trait.Name, traitParamKind)
		} else {
			// Default to * or handle other kinds if needed
			// Currently only * and * -> * are used in builtins
		}

		// Register kind for all type parameters
		for _, param := range lowerParams {
			table.RegisterTraitTypeParamKind(trait.Name, param, traitParamKind)
		}

		// Register Operators
		for _, op := range trait.Operators {
			table.RegisterOperatorTrait(op, trait.Name)
		}

		// Register Methods (Specific logic per trait)
		switch trait.Name {
		case "Show":
			// show : T -> String
			showMethodType := typesystem.TFunc{
				Params:     []typesystem.Type{tvar},
				ReturnType: stringType,
			}
			table.RegisterTraitMethod("show", "Show", showMethodType, prelude)

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
				Constraints: []typesystem.Constraint{
					{TypeVar: "f", Trait: "Functor"},
				},
			}
			table.RegisterTraitMethod("fmap", "Functor", fmapType, prelude)
			table.RegisterTraitMethod2("Functor", "fmap")

		case "Applicative":
			// pure : A -> F<A>
			pureType := typesystem.TFunc{
				Params:     []typesystem.Type{aVar},
				ReturnType: typesystem.TApp{Constructor: fVar, Args: []typesystem.Type{aVar}},
				Constraints: []typesystem.Constraint{
					{TypeVar: "f", Trait: "Applicative"},
				},
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
				Constraints: []typesystem.Constraint{
					{TypeVar: "f", Trait: "Applicative"},
				},
			}
			table.RegisterTraitMethod("(<*>)", "Applicative", applyType, prelude)
			table.RegisterTraitMethod2("Applicative", "(<*>)")

		case "Monad":
			// (>>=) : M<A> -> (A -> M<B>) -> M<B>
			// mVar is defined above with Kind * -> *
			mA := typesystem.TApp{Constructor: mVar, Args: []typesystem.Type{aVar}}
			mB := typesystem.TApp{Constructor: mVar, Args: []typesystem.Type{bVar}}
			aToMB := typesystem.TFunc{Params: []typesystem.Type{aVar}, ReturnType: mB}
			bindType := typesystem.TFunc{
				Params:     []typesystem.Type{mA, aToMB},
				ReturnType: mB,
				Constraints: []typesystem.Constraint{
					{TypeVar: "m", Trait: "Monad"},
				},
			}
			table.RegisterTraitMethod("(>>=)", "Monad", bindType, prelude)
			table.RegisterTraitMethod2("Monad", "(>>=)")

		case "Semigroup":
			semigroupOp := typesystem.TFunc{
				Params:     []typesystem.Type{aVar, aVar},
				ReturnType: aVar,
				Constraints: []typesystem.Constraint{
					{TypeVar: "a", Trait: "Semigroup"},
				},
			}
			table.RegisterTraitMethod("(<>)", "Semigroup", semigroupOp, prelude)
			table.RegisterTraitMethod2("Semigroup", "(<>)")

		case "Monoid":
			// mempty : () -> A
			// Defined as a nullary function that returns a value of type A.
			// Evaluator handles the dispatch to the correct implementation.
			memptyType := typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: aVar,
			}
			table.RegisterTraitMethod("mempty", "Monoid", memptyType, prelude)
			table.RegisterTraitMethod2("Monoid", "mempty")

		case "Empty":
			// isEmpty : F<A> -> Bool
			isEmptyType := typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.TApp{Constructor: fVar, Args: []typesystem.Type{aVar}}},
				ReturnType: typesystem.Bool,
				Constraints: []typesystem.Constraint{
					{TypeVar: "f", Trait: "Empty"},
				},
			}
			table.RegisterTraitMethod("isEmpty", "Empty", isEmptyType, prelude)
			table.RegisterTraitMethod2("Empty", "isEmpty")

		case "Optional":
			// unwrap : F<A> -> A
			unwrapType := typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.TApp{Constructor: fVar, Args: []typesystem.Type{aVar}}},
				ReturnType: aVar,
				Constraints: []typesystem.Constraint{
					{TypeVar: "f", Trait: "Optional"},
				},
			}
			table.RegisterTraitMethod("unwrap", "Optional", unwrapType, prelude)
			table.RegisterTraitMethod2("Optional", "unwrap")

			// wrap : A -> F<A>
			wrapType := typesystem.TFunc{
				Params:     []typesystem.Type{aVar},
				ReturnType: typesystem.TApp{Constructor: fVar, Args: []typesystem.Type{aVar}},
				Constraints: []typesystem.Constraint{
					{TypeVar: "f", Trait: "Optional"},
				},
			}
			table.RegisterTraitMethod("wrap", "Optional", wrapType, prelude)
			table.RegisterTraitMethod2("Optional", "wrap")

		case "Iter":
			// iter: (C) -> () -> Option<T>
			iterReturnType := typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: typesystem.TApp{Constructor: typesystem.TCon{Name: config.OptionTypeName}, Args: []typesystem.Type{typesystem.TVar{Name: "t"}}},
			}
			iterMethodType := typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.TVar{Name: "c"}},
				ReturnType: iterReturnType,
				Constraints: []typesystem.Constraint{
					{TypeVar: "c", Trait: "Iter"},
				},
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
			// Define trait with lowercase type parameter "t"
			typeParam := "t"
			table.DefineTrait(op.Trait, []string{typeParam}, nil, prelude)

			// Register kind for the type parameter (default to *)
			// Since these are user operators on values, they usually operate on * types
			table.RegisterTraitTypeParamKind(op.Trait, typeParam, typesystem.Star)
		}
		table.RegisterOperatorTrait(op.Symbol, op.Trait)
		table.RegisterTraitMethod("("+op.Symbol+")", op.Trait, binaryOp(tvar), prelude)
	}
}

// registerPrimitiveInstances registers virtual instances for built-in types
func registerPrimitiveInstances(table *symbols.SymbolTable) {
	// Numeric types implement Show, Equal, Order, Numeric, Default
	numericTypes := []typesystem.Type{
		typesystem.Int,
		typesystem.Float,
		typesystem.BigInt,
		typesystem.Rational,
	}

	for _, t := range numericTypes {
		reg(table, "Show", t)
		reg(table, "Equal", t)
		reg(table, "Order", t)
		reg(table, "Numeric", t)
		reg(table, "Default", t)
	}

	// Integer types implement Bitwise
	reg(table, "Bitwise", typesystem.Int)
	reg(table, "Bitwise", typesystem.BigInt)

	// Bool implements Show, Equal, Order, Default (false < true)
	reg(table, "Show", typesystem.Bool)
	reg(table, "Equal", typesystem.Bool)
	reg(table, "Order", typesystem.Bool)
	reg(table, "Default", typesystem.Bool)

	// Char implements Show, Equal, Order, Default
	charCon := typesystem.TCon{Name: "Char"}
	reg(table, "Show", charCon)
	reg(table, "Equal", charCon)
	reg(table, "Order", charCon)
	reg(table, "Default", charCon)

	// List<T> implements Show, Equal, Order, Default, Concat
	// This covers String (List<Char>) as well since String is just List<Char>
	listType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{typesystem.TVar{Name: "a"}},
	}
	reg(table, "Show", listType)
	reg(table, "Equal", listType)
	reg(table, "Order", listType)
	reg(table, "Default", listType)
	reg(table, "Concat", listType)

	// FP trait implementations for type constructors
	// List implements Empty, Semigroup, Monoid, Functor, Applicative, Monad
	listCon := typesystem.TCon{Name: config.ListTypeName}
	reg(table, "Empty", listCon)
	reg(table, "Semigroup", listType)
	reg(table, "Monoid", listType)
	reg(table, "Functor", listCon)
	reg(table, "Applicative", listCon)
	reg(table, "Monad", listCon)

	// Option implements Show, Empty, Optional, Equal, Order, Default, Semigroup, Monoid, Functor, Applicative, Monad
	optionCon := typesystem.TCon{Name: config.OptionTypeName}
	optionType := typesystem.TApp{
		Constructor: optionCon,
		Args:        []typesystem.Type{typesystem.TVar{Name: "a"}},
	}
	reg(table, "Show", optionType)
	reg(table, "Empty", optionCon)
	reg(table, "Optional", optionCon)
	reg(table, "Equal", optionType)
	reg(table, "Order", optionType)
	reg(table, "Default", optionType)
	reg(table, "Semigroup", optionType)
	reg(table, "Monoid", optionType)
	reg(table, "Functor", optionCon)
	reg(table, "Applicative", optionCon)
	reg(table, "Monad", optionCon)

	// Register Optional instance methods for Option
	// unwrap: Option<A> -> A
	optionUnwrapType := typesystem.TFunc{
		Params:     []typesystem.Type{optionType},
		ReturnType: typesystem.TVar{Name: "a"},
	}
	table.RegisterInstanceMethod("Optional", config.OptionTypeName, "unwrap", optionUnwrapType)
	table.RegisterExtensionMethod(config.OptionTypeName, "unwrap", optionUnwrapType)

	// Result implements Show, Empty, Optional, Equal, Semigroup, Functor, Applicative, Monad
	// Result<E, A> - E is error (first), A is success (last, for Functor/Monad)
	resultCon := typesystem.TCon{Name: config.ResultTypeName}
	resultType := typesystem.TApp{
		Constructor: resultCon,
		Args:        []typesystem.Type{typesystem.TVar{Name: "e"}, typesystem.TVar{Name: "a"}},
	}
	reg(table, "Show", resultType)
	reg(table, "Empty", resultCon)
	reg(table, "Optional", resultCon)
	reg(table, "Equal", resultType)
	reg(table, "Semigroup", resultType)
	reg(table, "Functor", resultCon)
	reg(table, "Applicative", resultCon)
	reg(table, "Monad", resultCon)

	// Register Optional instance methods for Result
	// unwrap: Result<E, A> -> A (last type param)
	resultUnwrapType := typesystem.TFunc{
		Params:     []typesystem.Type{resultType},
		ReturnType: typesystem.TVar{Name: "a"},
	}
	table.RegisterInstanceMethod("Optional", config.ResultTypeName, "unwrap", resultUnwrapType)
	table.RegisterExtensionMethod(config.ResultTypeName, "unwrap", resultUnwrapType)

	// Tuple implements Show, Equal, Order (lexicographic)
	// Register for common arities (2, 3, 4)
	for arity := 2; arity <= 4; arity++ {
		args := make([]typesystem.Type, arity)
		for i := 0; i < arity; i++ {
			args[i] = typesystem.TVar{Name: fmt.Sprintf("t%d", i)}
		}
		tupleType := typesystem.TTuple{Elements: args}
		reg(table, "Show", tupleType)
		reg(table, "Equal", tupleType)
		reg(table, "Order", tupleType)
	}

	// Note: String (List<Char>) is covered by List<T> above for all traits
	// including Equal, Order, Default, Concat, Semigroup, Monoid

	// Note: Functor instances are NOT pre-registered here.
	// Users must define instance methods themselves.
	// The Functor trait is built-in but instances require explicit implementation.

	// Nil implements Show, Default
	reg(table, "Show", typesystem.Nil)
	reg(table, "Default", typesystem.Nil)

	// Map<K, V> implements Show, Empty, Semigroup, Monoid, Equal
	mapCon := typesystem.TCon{Name: config.MapTypeName}
	mapType := typesystem.TApp{
		Constructor: mapCon,
		Args:        []typesystem.Type{typesystem.TVar{Name: "k"}, typesystem.TVar{Name: "v"}},
	}
	reg(table, "Show", mapType)
	reg(table, "Empty", mapCon)
	reg(table, "Semigroup", mapType)
	reg(table, "Monoid", mapType)
	reg(table, "Equal", mapType)

	// Bytes implements Show, Equal, Order, Concat
	bytesCon := typesystem.TCon{Name: config.BytesTypeName}
	reg(table, "Show", bytesCon)
	reg(table, "Equal", bytesCon)
	reg(table, "Order", bytesCon)
	reg(table, "Concat", bytesCon)

	// Bits implements Show, Equal, Concat
	bitsCon := typesystem.TCon{Name: config.BitsTypeName}
	reg(table, "Show", bitsCon)
	reg(table, "Equal", bitsCon)
	reg(table, "Concat", bitsCon)

	// Uuid implements Equal
	uuidCon := typesystem.TCon{Name: "Uuid"}
	reg(table, "Equal", uuidCon)

	// Reader implements Functor, Applicative, Monad
	readerCon := typesystem.TCon{Name: "Reader"}
	reg(table, "Functor", readerCon)
	reg(table, "Applicative", readerCon)
	reg(table, "Monad", readerCon)

	// Identity implements Functor, Applicative, Monad
	identityCon := typesystem.TCon{Name: "Identity"}
	reg(table, "Functor", identityCon)
	reg(table, "Applicative", identityCon)
	reg(table, "Monad", identityCon)

	// State implements Functor, Applicative, Monad
	stateCon := typesystem.TCon{Name: "State"}
	reg(table, "Functor", stateCon)
	reg(table, "Applicative", stateCon)
	reg(table, "Monad", stateCon)

	// Writer implements Functor, Applicative, Monad
	writerCon := typesystem.TCon{Name: "Writer"}
	reg(table, "Functor", writerCon)
	reg(table, "Applicative", writerCon)
	reg(table, "Monad", writerCon)

	// OptionT implements Functor, Applicative, Monad
	optionTCon := typesystem.TCon{Name: "OptionT"}
	reg(table, "Functor", optionTCon)
	reg(table, "Applicative", optionTCon)
	reg(table, "Monad", optionTCon)

	// ResultT implements Functor, Applicative, Monad
	resultTCon := typesystem.TCon{Name: "ResultT"}
	reg(table, "Functor", resultTCon)
	reg(table, "Applicative", resultTCon)
	reg(table, "Monad", resultTCon)

	// Iter implementation for List
	reg(table, "Iter", listType)

	// Iter implementation for Range<Int> and Range<Char>
	rangeCon := typesystem.TCon{Name: "Range"}
	rangeIntType := typesystem.TApp{
		Constructor: rangeCon,
		Args:        []typesystem.Type{typesystem.Int},
	}
	reg(table, "Iter", rangeIntType)

	rangeCharType := typesystem.TApp{
		Constructor: rangeCon,
		Args:        []typesystem.Type{typesystem.Char},
	}
	reg(table, "Iter", rangeCharType)
}

// reg registers both implementation and evidence for a built-in instance
func reg(table *symbols.SymbolTable, traitName string, t typesystem.Type) {
	args := []typesystem.Type{t}

	// Calculate evidence name first
	// Construct key: Trait[Type]
	// Type name extraction logic matches SolveWitness
	key := GetEvidenceKey(traitName, args)

	checkType := typesystem.UnwrapUnderlying(t)
	if checkType == nil {
		checkType = t
	}

	typeName := ""
	isGeneric := false

	switch tt := checkType.(type) {
	case typesystem.TCon:
		typeName = tt.Name
	case typesystem.TApp:
		if tCon, ok := tt.Constructor.(typesystem.TCon); ok {
			typeName = tCon.Name
			isGeneric = true
		}
	default:
		typeName = t.String()
	}

	var evidenceName string
	if isGeneric {
		evidenceName = GetDictionaryConstructorName(traitName, typeName)
	} else {
		evidenceName = GetDictionaryName(traitName, typeName)
	}

	_ = table.RegisterImplementation(traitName, args, nil, evidenceName)

	// Register evidence
	// Register evidence name
	table.RegisterEvidence(key, evidenceName)

	// Define the symbol for the dictionary/constructor
	// We don't have the full AST body here, but we define the symbol type
	// so the analyzer knows it exists.
	// The actual value will be provided by Evaluator builtins.
	var symType typesystem.Type
	if isGeneric {
		// Constructor: (dicts...) -> Dictionary
		// Simplification: we treat it as function returning Dictionary
		// Evaluator must provide a builtin function with this name.
		symType = typesystem.TFunc{
			Params:     nil, // Simplified
			ReturnType: typesystem.TCon{Name: "Dictionary"},
			IsVariadic: true, // Allow variable args for dicts
		}
	} else {
		// Constant: Dictionary
		symType = typesystem.TCon{Name: "Dictionary"}
	}

	table.DefineConstant(evidenceName, symType, "prelude")
}
