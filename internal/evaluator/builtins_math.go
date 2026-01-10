package evaluator

import (
	"math"

	"github.com/funvibe/funxy/internal/typesystem"
)

// MathBuiltins returns built-in functions for lib/math virtual package
func MathBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		// Basic operations
		"abs":    {Fn: builtinAbs, Name: "abs"},
		"absInt": {Fn: builtinAbsInt, Name: "absInt"},
		"sign":   {Fn: builtinSign, Name: "sign"},
		"min":    {Fn: builtinMin, Name: "min"},
		"max":    {Fn: builtinMax, Name: "max"},
		"minInt": {Fn: builtinMinInt, Name: "minInt"},
		"maxInt": {Fn: builtinMaxInt, Name: "maxInt"},
		"clamp":  {Fn: builtinClamp, Name: "clamp"},

		// Rounding
		"floor": {Fn: builtinFloor, Name: "floor"},
		"ceil":  {Fn: builtinCeil, Name: "ceil"},
		"round": {Fn: builtinRound, Name: "round"},
		"trunc": {Fn: builtinTrunc, Name: "trunc"},

		// Powers and roots
		"sqrt": {Fn: builtinSqrt, Name: "sqrt"},
		"cbrt": {Fn: builtinCbrt, Name: "cbrt"},
		"pow":  {Fn: builtinPow, Name: "pow"},
		"exp":  {Fn: builtinExp, Name: "exp"},

		// Logarithms
		"log":   {Fn: builtinLog, Name: "log"},
		"log10": {Fn: builtinLog10, Name: "log10"},
		"log2":  {Fn: builtinLog2, Name: "log2"},

		// Trigonometry
		"sin":   {Fn: builtinSin, Name: "sin"},
		"cos":   {Fn: builtinCos, Name: "cos"},
		"tan":   {Fn: builtinTan, Name: "tan"},
		"asin":  {Fn: builtinAsin, Name: "asin"},
		"acos":  {Fn: builtinAcos, Name: "acos"},
		"atan":  {Fn: builtinAtan, Name: "atan"},
		"atan2": {Fn: builtinAtan2, Name: "atan2"},

		// Hyperbolic
		"sinh": {Fn: builtinSinh, Name: "sinh"},
		"cosh": {Fn: builtinCosh, Name: "cosh"},
		"tanh": {Fn: builtinTanh, Name: "tanh"},

		// Constants
		"pi": {Fn: builtinPi, Name: "pi"},
		"e":  {Fn: builtinE, Name: "e"},
	}
}

// SetMathBuiltinTypes sets type info for math builtins
func SetMathBuiltinTypes(builtins map[string]*Builtin) {
	floatType := typesystem.Float
	intType := typesystem.Int

	types := map[string]typesystem.Type{
		"abs":    typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
		"absInt": typesystem.TFunc{Params: []typesystem.Type{intType}, ReturnType: intType},
		"sign":   typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: intType},
		"min":    typesystem.TFunc{Params: []typesystem.Type{floatType, floatType}, ReturnType: floatType},
		"max":    typesystem.TFunc{Params: []typesystem.Type{floatType, floatType}, ReturnType: floatType},
		"minInt": typesystem.TFunc{Params: []typesystem.Type{intType, intType}, ReturnType: intType},
		"maxInt": typesystem.TFunc{Params: []typesystem.Type{intType, intType}, ReturnType: intType},
		"clamp":  typesystem.TFunc{Params: []typesystem.Type{floatType, floatType, floatType}, ReturnType: floatType},
		"floor":  typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: intType},
		"ceil":   typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: intType},
		"round":  typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: intType},
		"trunc":  typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: intType},
		"sqrt":   typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
		"cbrt":   typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
		"pow":    typesystem.TFunc{Params: []typesystem.Type{floatType, floatType}, ReturnType: floatType},
		"exp":    typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
		"log":    typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
		"log10":  typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
		"log2":   typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
		"sin":    typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
		"cos":    typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
		"tan":    typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
		"asin":   typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
		"acos":   typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
		"atan":   typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
		"atan2":  typesystem.TFunc{Params: []typesystem.Type{floatType, floatType}, ReturnType: floatType},
		"sinh":   typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
		"cosh":   typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
		"tanh":   typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
		"pi":     typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: floatType},
		"e":      typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: floatType},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}

// =============================================================================
// Helper to get float from Object (accepts both Float and Integer)
// =============================================================================

func toGoFloat(obj Object) (float64, bool) {
	switch v := obj.(type) {
	case *Float:
		return v.Value, true
	case *Integer:
		return float64(v.Value), true
	}
	return 0, false
}

func makeFloat(f float64) *Float {
	return &Float{Value: f}
}

// =============================================================================
// Basic operations
// =============================================================================

func builtinAbs(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("abs expects 1 argument, got %d", len(args))
	}
	f, ok := toGoFloat(args[0])
	if !ok {
		return newError("abs expects a number, got %s", args[0].Type())
	}
	return makeFloat(math.Abs(f))
}

func builtinAbsInt(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("absInt expects 1 argument, got %d", len(args))
	}
	i, ok := args[0].(*Integer)
	if !ok {
		return newError("absInt expects an integer, got %s", args[0].Type())
	}
	if i.Value < 0 {
		return &Integer{Value: -i.Value}
	}
	return i
}

func builtinSign(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("sign expects 1 argument, got %d", len(args))
	}
	f, ok := toGoFloat(args[0])
	if !ok {
		return newError("sign expects a number, got %s", args[0].Type())
	}
	if f > 0 {
		return &Integer{Value: 1}
	} else if f < 0 {
		return &Integer{Value: -1}
	}
	return &Integer{Value: 0}
}

func builtinMin(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("min expects 2 arguments, got %d", len(args))
	}
	a, ok1 := toGoFloat(args[0])
	b, ok2 := toGoFloat(args[1])
	if !ok1 || !ok2 {
		return newError("min expects numbers")
	}
	return makeFloat(math.Min(a, b))
}

func builtinMax(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("max expects 2 arguments, got %d", len(args))
	}
	a, ok1 := toGoFloat(args[0])
	b, ok2 := toGoFloat(args[1])
	if !ok1 || !ok2 {
		return newError("max expects numbers")
	}
	return makeFloat(math.Max(a, b))
}

func builtinMinInt(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("minInt expects 2 arguments, got %d", len(args))
	}
	a, ok1 := args[0].(*Integer)
	b, ok2 := args[1].(*Integer)
	if !ok1 || !ok2 {
		return newError("minInt expects integers")
	}
	if a.Value < b.Value {
		return a
	}
	return b
}

func builtinMaxInt(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("maxInt expects 2 arguments, got %d", len(args))
	}
	a, ok1 := args[0].(*Integer)
	b, ok2 := args[1].(*Integer)
	if !ok1 || !ok2 {
		return newError("maxInt expects integers")
	}
	if a.Value > b.Value {
		return a
	}
	return b
}

func builtinClamp(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("clamp expects 3 arguments, got %d", len(args))
	}
	val, ok1 := toGoFloat(args[0])
	minVal, ok2 := toGoFloat(args[1])
	maxVal, ok3 := toGoFloat(args[2])
	if !ok1 || !ok2 || !ok3 {
		return newError("clamp expects numbers")
	}
	return makeFloat(math.Max(minVal, math.Min(maxVal, val)))
}

// =============================================================================
// Rounding
// =============================================================================

func builtinFloor(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("floor expects 1 argument, got %d", len(args))
	}
	f, ok := toGoFloat(args[0])
	if !ok {
		return newError("floor expects a number, got %s", args[0].Type())
	}
	return &Integer{Value: int64(math.Floor(f))}
}

func builtinCeil(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("ceil expects 1 argument, got %d", len(args))
	}
	f, ok := toGoFloat(args[0])
	if !ok {
		return newError("ceil expects a number, got %s", args[0].Type())
	}
	return &Integer{Value: int64(math.Ceil(f))}
}

func builtinRound(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("round expects 1 argument, got %d", len(args))
	}
	f, ok := toGoFloat(args[0])
	if !ok {
		return newError("round expects a number, got %s", args[0].Type())
	}
	return &Integer{Value: int64(math.Round(f))}
}

func builtinTrunc(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("trunc expects 1 argument, got %d", len(args))
	}
	f, ok := toGoFloat(args[0])
	if !ok {
		return newError("trunc expects a number, got %s", args[0].Type())
	}
	return &Integer{Value: int64(math.Trunc(f))}
}

// =============================================================================
// Powers and roots
// =============================================================================

func builtinSqrt(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("sqrt expects 1 argument, got %d", len(args))
	}
	f, ok := toGoFloat(args[0])
	if !ok {
		return newError("sqrt expects a number, got %s", args[0].Type())
	}
	return makeFloat(math.Sqrt(f))
}

func builtinCbrt(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("cbrt expects 1 argument, got %d", len(args))
	}
	f, ok := toGoFloat(args[0])
	if !ok {
		return newError("cbrt expects a number, got %s", args[0].Type())
	}
	return makeFloat(math.Cbrt(f))
}

func builtinPow(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("pow expects 2 arguments, got %d", len(args))
	}
	base, ok1 := toGoFloat(args[0])
	exp, ok2 := toGoFloat(args[1])
	if !ok1 || !ok2 {
		return newError("pow expects numbers")
	}
	return makeFloat(math.Pow(base, exp))
}

func builtinExp(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("exp expects 1 argument, got %d", len(args))
	}
	f, ok := toGoFloat(args[0])
	if !ok {
		return newError("exp expects a number, got %s", args[0].Type())
	}
	return makeFloat(math.Exp(f))
}

// =============================================================================
// Logarithms
// =============================================================================

func builtinLog(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("log expects 1 argument, got %d", len(args))
	}
	f, ok := toGoFloat(args[0])
	if !ok {
		return newError("log expects a number, got %s", args[0].Type())
	}
	return makeFloat(math.Log(f))
}

func builtinLog10(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("log10 expects 1 argument, got %d", len(args))
	}
	f, ok := toGoFloat(args[0])
	if !ok {
		return newError("log10 expects a number, got %s", args[0].Type())
	}
	return makeFloat(math.Log10(f))
}

func builtinLog2(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("log2 expects 1 argument, got %d", len(args))
	}
	f, ok := toGoFloat(args[0])
	if !ok {
		return newError("log2 expects a number, got %s", args[0].Type())
	}
	return makeFloat(math.Log2(f))
}

// =============================================================================
// Trigonometry
// =============================================================================

func builtinSin(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("sin expects 1 argument, got %d", len(args))
	}
	f, ok := toGoFloat(args[0])
	if !ok {
		return newError("sin expects a number, got %s", args[0].Type())
	}
	return makeFloat(math.Sin(f))
}

func builtinCos(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("cos expects 1 argument, got %d", len(args))
	}
	f, ok := toGoFloat(args[0])
	if !ok {
		return newError("cos expects a number, got %s", args[0].Type())
	}
	return makeFloat(math.Cos(f))
}

func builtinTan(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("tan expects 1 argument, got %d", len(args))
	}
	f, ok := toGoFloat(args[0])
	if !ok {
		return newError("tan expects a number, got %s", args[0].Type())
	}
	return makeFloat(math.Tan(f))
}

func builtinAsin(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("asin expects 1 argument, got %d", len(args))
	}
	f, ok := toGoFloat(args[0])
	if !ok {
		return newError("asin expects a number, got %s", args[0].Type())
	}
	return makeFloat(math.Asin(f))
}

func builtinAcos(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("acos expects 1 argument, got %d", len(args))
	}
	f, ok := toGoFloat(args[0])
	if !ok {
		return newError("acos expects a number, got %s", args[0].Type())
	}
	return makeFloat(math.Acos(f))
}

func builtinAtan(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("atan expects 1 argument, got %d", len(args))
	}
	f, ok := toGoFloat(args[0])
	if !ok {
		return newError("atan expects a number, got %s", args[0].Type())
	}
	return makeFloat(math.Atan(f))
}

func builtinAtan2(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("atan2 expects 2 arguments, got %d", len(args))
	}
	y, ok1 := toGoFloat(args[0])
	x, ok2 := toGoFloat(args[1])
	if !ok1 || !ok2 {
		return newError("atan2 expects numbers")
	}
	return makeFloat(math.Atan2(y, x))
}

// =============================================================================
// Hyperbolic
// =============================================================================

func builtinSinh(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("sinh expects 1 argument, got %d", len(args))
	}
	f, ok := toGoFloat(args[0])
	if !ok {
		return newError("sinh expects a number, got %s", args[0].Type())
	}
	return makeFloat(math.Sinh(f))
}

func builtinCosh(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("cosh expects 1 argument, got %d", len(args))
	}
	f, ok := toGoFloat(args[0])
	if !ok {
		return newError("cosh expects a number, got %s", args[0].Type())
	}
	return makeFloat(math.Cosh(f))
}

func builtinTanh(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("tanh expects 1 argument, got %d", len(args))
	}
	f, ok := toGoFloat(args[0])
	if !ok {
		return newError("tanh expects a number, got %s", args[0].Type())
	}
	return makeFloat(math.Tanh(f))
}

// =============================================================================
// Constants
// =============================================================================

func builtinPi(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("pi expects 0 arguments, got %d", len(args))
	}
	return makeFloat(math.Pi)
}

func builtinE(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("e expects 0 arguments, got %d", len(args))
	}
	return makeFloat(math.E)
}

// =============================================================================
// Conversion
