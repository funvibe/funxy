package evaluator

import (
	"math"
	"math/big"

	"github.com/funvibe/funxy/internal/typesystem"
)

// BignumBuiltins returns built-in functions for lib/bignum virtual package
func BignumBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		// BigInt
		"bigIntNew":      {Fn: builtinBigint, Name: "bigIntNew"},
		"bigIntFromInt":  {Fn: builtinBigintFromInt, Name: "bigIntFromInt"},
		"bigIntToString": {Fn: builtinBigintToString, Name: "bigIntToString"},
		"bigIntToInt":    {Fn: builtinBigintToInt, Name: "bigIntToInt"},

		// Rational
		"ratFromInt":  {Fn: builtinRationalFromInt, Name: "ratFromInt"},
		"ratNew":      {Fn: builtinRational, Name: "ratNew"},
		"ratNumer":    {Fn: builtinNumerator, Name: "ratNumer"},
		"ratDenom":    {Fn: builtinDenominator, Name: "ratDenom"},
		"ratToFloat":  {Fn: builtinRationalToFloat, Name: "ratToFloat"},
		"ratToString": {Fn: builtinRationalToString, Name: "ratToString"},
	}
}

// SetBignumBuiltinTypes sets type info for bignum builtins
func SetBignumBuiltinTypes(builtins map[string]*Builtin) {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}
	optionInt := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{typesystem.Int},
	}
	optionFloat := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{typesystem.Float},
	}

	types := map[string]typesystem.Type{
		"bigIntNew":      typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.BigInt},
		"bigIntFromInt":  typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: typesystem.BigInt},
		"bigIntToString": typesystem.TFunc{Params: []typesystem.Type{typesystem.BigInt}, ReturnType: stringType},
		"bigIntToInt":    typesystem.TFunc{Params: []typesystem.Type{typesystem.BigInt}, ReturnType: optionInt},
		"ratFromInt":     typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, typesystem.Int}, ReturnType: typesystem.Rational},
		"ratNew":         typesystem.TFunc{Params: []typesystem.Type{typesystem.BigInt, typesystem.BigInt}, ReturnType: typesystem.Rational},
		"ratNumer":       typesystem.TFunc{Params: []typesystem.Type{typesystem.Rational}, ReturnType: typesystem.BigInt},
		"ratDenom":       typesystem.TFunc{Params: []typesystem.Type{typesystem.Rational}, ReturnType: typesystem.BigInt},
		"ratToFloat":     typesystem.TFunc{Params: []typesystem.Type{typesystem.Rational}, ReturnType: optionFloat},
		"ratToString":    typesystem.TFunc{Params: []typesystem.Type{typesystem.Rational}, ReturnType: stringType},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}

// =============================================================================
// BigInt functions
// =============================================================================

// bigint: (String) -> BigInt
func builtinBigint(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bigint expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("bigint expects a string argument, got %s", args[0].Type())
	}

	s := listToString(str)
	bi := new(big.Int)
	_, success := bi.SetString(s, 10)
	if !success {
		return newError("bigint: invalid number string: %s", s)
	}

	return &BigInt{Value: bi}
}

// bigintFromInt: (Int) -> BigInt
func builtinBigintFromInt(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bigintFromInt expects 1 argument, got %d", len(args))
	}

	i, ok := args[0].(*Integer)
	if !ok {
		return newError("bigintFromInt expects an integer argument, got %s", args[0].Type())
	}

	return &BigInt{Value: big.NewInt(i.Value)}
}

// bigintToString: (BigInt) -> String
func builtinBigintToString(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bigintToString expects 1 argument, got %d", len(args))
	}

	bi, ok := args[0].(*BigInt)
	if !ok {
		return newError("bigintToString expects a BigInt argument, got %s", args[0].Type())
	}

	return stringToList(bi.Value.String())
}

// bigintToInt: (BigInt) -> Option<Int>
func builtinBigintToInt(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bigintToInt expects 1 argument, got %d", len(args))
	}

	bi, ok := args[0].(*BigInt)
	if !ok {
		return newError("bigintToInt expects a BigInt argument, got %s", args[0].Type())
	}

	// Check if value fits in int64
	if bi.Value.IsInt64() {
		return makeSome(&Integer{Value: bi.Value.Int64()})
	}

	return makeNone()
}

// =============================================================================
// Rational functions
// =============================================================================

// rationalFromInt: (Int, Int) -> Rational
func builtinRationalFromInt(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("rationalFromInt expects 2 arguments, got %d", len(args))
	}

	num, ok1 := args[0].(*Integer)
	denom, ok2 := args[1].(*Integer)
	if !ok1 || !ok2 {
		return newError("rationalFromInt expects integer arguments")
	}

	if denom.Value == 0 {
		return newError("rationalFromInt: denominator cannot be zero")
	}

	rat := big.NewRat(num.Value, denom.Value)
	return &Rational{Value: rat}
}

// rational: (BigInt, BigInt) -> Rational
func builtinRational(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("rational expects 2 arguments, got %d", len(args))
	}

	num, ok1 := args[0].(*BigInt)
	denom, ok2 := args[1].(*BigInt)
	if !ok1 || !ok2 {
		return newError("rational expects BigInt arguments")
	}

	if denom.Value.Sign() == 0 {
		return newError("rational: denominator cannot be zero")
	}

	rat := new(big.Rat)
	rat.SetFrac(num.Value, denom.Value)
	return &Rational{Value: rat}
}

// numerator: (Rational) -> BigInt
func builtinNumerator(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("numerator expects 1 argument, got %d", len(args))
	}

	rat, ok := args[0].(*Rational)
	if !ok {
		return newError("numerator expects a Rational argument, got %s", args[0].Type())
	}

	// big.Rat.Num() returns a pointer to the numerator
	num := new(big.Int).Set(rat.Value.Num())
	return &BigInt{Value: num}
}

// denominator: (Rational) -> BigInt
func builtinDenominator(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("denominator expects 1 argument, got %d", len(args))
	}

	rat, ok := args[0].(*Rational)
	if !ok {
		return newError("denominator expects a Rational argument, got %s", args[0].Type())
	}

	// big.Rat.Denom() returns a pointer to the denominator
	denom := new(big.Int).Set(rat.Value.Denom())
	return &BigInt{Value: denom}
}

// rationalToFloat: (Rational) -> Option<Float>
func builtinRationalToFloat(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("rationalToFloat expects 1 argument, got %d", len(args))
	}

	rat, ok := args[0].(*Rational)
	if !ok {
		return newError("rationalToFloat expects a Rational argument, got %s", args[0].Type())
	}

	f, exact := rat.Value.Float64()
	// Check for infinity or NaN
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return makeNone()
	}

	// We return Some even if not exact (precision loss is expected)
	_ = exact
	return makeSome(&Float{Value: f})
}

// rationalToString: (Rational) -> String
func builtinRationalToString(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("rationalToString expects 1 argument, got %d", len(args))
	}

	rat, ok := args[0].(*Rational)
	if !ok {
		return newError("rationalToString expects a Rational argument, got %s", args[0].Type())
	}

	// Format as "num/denom"
	result := rat.Value.RatString()
	return stringToList(result)
}
