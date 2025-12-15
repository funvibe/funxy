package evaluator

import (
	"github.com/funvibe/funxy/internal/typesystem"
)

// TupleBuiltins returns built-in functions for lib/tuple virtual package
func TupleBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		// Basic access
		"fst":      {Fn: builtinFst, Name: "fst"},
		"snd":      {Fn: builtinSnd, Name: "snd"},
		"tupleGet": {Fn: builtinTupleGet, Name: "tupleGet"},

		// Transformation
		"tupleSwap": {Fn: builtinSwap, Name: "tupleSwap"},
		"tupleDup":  {Fn: builtinDup, Name: "tupleDup"},

		// Mapping
		"mapFst":  {Fn: builtinMapFst, Name: "mapFst"},
		"mapSnd":  {Fn: builtinMapSnd, Name: "mapSnd"},
		"mapPair": {Fn: builtinMapPair, Name: "mapPair"},

		// Currying
		"curry":   {Fn: builtinCurry, Name: "curry"},
		"uncurry": {Fn: builtinUncurry, Name: "uncurry"},

		// Predicates
		"tupleBoth":   {Fn: builtinBoth, Name: "tupleBoth"},
		"tupleEither": {Fn: builtinEither, Name: "tupleEither"},
	}
}

// SetTupleBuiltinTypes sets type info for tuple builtins
func SetTupleBuiltinTypes(builtins map[string]*Builtin) {
	A := typesystem.TVar{Name: "A"}
	B := typesystem.TVar{Name: "B"}
	C := typesystem.TVar{Name: "C"}
	D := typesystem.TVar{Name: "D"}
	T := typesystem.TVar{Name: "T"}

	pairAB := typesystem.TTuple{Elements: []typesystem.Type{A, B}}
	pairBA := typesystem.TTuple{Elements: []typesystem.Type{B, A}}
	pairAA := typesystem.TTuple{Elements: []typesystem.Type{A, A}}
	pairCB := typesystem.TTuple{Elements: []typesystem.Type{C, B}}
	pairAC := typesystem.TTuple{Elements: []typesystem.Type{A, C}}
	pairCD := typesystem.TTuple{Elements: []typesystem.Type{C, D}}

	aToC := typesystem.TFunc{Params: []typesystem.Type{A}, ReturnType: C}
	bToC := typesystem.TFunc{Params: []typesystem.Type{B}, ReturnType: C}
	bToD := typesystem.TFunc{Params: []typesystem.Type{B}, ReturnType: D}
	aToBool := typesystem.TFunc{Params: []typesystem.Type{A}, ReturnType: typesystem.Bool}

	pairABToC := typesystem.TFunc{Params: []typesystem.Type{pairAB}, ReturnType: C}
	aToFunc := typesystem.TFunc{Params: []typesystem.Type{A}, ReturnType: typesystem.TFunc{Params: []typesystem.Type{B}, ReturnType: C}}

	genericTuple := typesystem.TVar{Name: "Tuple"}

	types := map[string]typesystem.Type{
		"fst":         typesystem.TFunc{Params: []typesystem.Type{pairAB}, ReturnType: A},
		"snd":         typesystem.TFunc{Params: []typesystem.Type{pairAB}, ReturnType: B},
		"tupleGet":    typesystem.TFunc{Params: []typesystem.Type{genericTuple, typesystem.Int}, ReturnType: T},
		"tupleSwap":   typesystem.TFunc{Params: []typesystem.Type{pairAB}, ReturnType: pairBA},
		"tupleDup":    typesystem.TFunc{Params: []typesystem.Type{A}, ReturnType: pairAA},
		"mapFst":      typesystem.TFunc{Params: []typesystem.Type{aToC, pairAB}, ReturnType: pairCB},
		"mapSnd":      typesystem.TFunc{Params: []typesystem.Type{bToC, pairAB}, ReturnType: pairAC},
		"mapPair":     typesystem.TFunc{Params: []typesystem.Type{aToC, bToD, pairAB}, ReturnType: pairCD},
		"curry":       typesystem.TFunc{Params: []typesystem.Type{pairABToC}, ReturnType: aToFunc},
		"uncurry":     typesystem.TFunc{Params: []typesystem.Type{aToFunc}, ReturnType: pairABToC},
		"tupleBoth":   typesystem.TFunc{Params: []typesystem.Type{aToBool, pairAA}, ReturnType: typesystem.Bool},
		"tupleEither": typesystem.TFunc{Params: []typesystem.Type{aToBool, pairAA}, ReturnType: typesystem.Bool},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}

// =============================================================================
// Basic Access
// =============================================================================

// fst: (A, B) -> A
func builtinFst(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("fst expects 1 argument, got %d", len(args))
	}
	tuple, ok := args[0].(*Tuple)
	if !ok {
		return newError("fst expects a tuple, got %s", args[0].Type())
	}
	if len(tuple.Elements) < 1 {
		return newError("fst: tuple must have at least 1 element")
	}
	return tuple.Elements[0]
}

// snd: (A, B) -> B
func builtinSnd(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("snd expects 1 argument, got %d", len(args))
	}
	tuple, ok := args[0].(*Tuple)
	if !ok {
		return newError("snd expects a tuple, got %s", args[0].Type())
	}
	if len(tuple.Elements) < 2 {
		return newError("snd: tuple must have at least 2 elements")
	}
	return tuple.Elements[1]
}

// get: (Tuple, Int) -> T
func builtinTupleGet(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("get expects 2 arguments, got %d", len(args))
	}
	tuple, ok := args[0].(*Tuple)
	if !ok {
		return newError("get expects a tuple as first argument, got %s", args[0].Type())
	}
	index, ok := args[1].(*Integer)
	if !ok {
		return newError("get expects an integer as second argument, got %s", args[1].Type())
	}
	idx := int(index.Value)
	if idx < 0 || idx >= len(tuple.Elements) {
		return newError("get: index %d out of bounds for tuple of length %d", idx, len(tuple.Elements))
	}
	return tuple.Elements[idx]
}

// =============================================================================
// Transformation
// =============================================================================

// swap: (A, B) -> (B, A)
func builtinSwap(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("swap expects 1 argument, got %d", len(args))
	}
	tuple, ok := args[0].(*Tuple)
	if !ok {
		return newError("swap expects a tuple, got %s", args[0].Type())
	}
	if len(tuple.Elements) != 2 {
		return newError("swap: tuple must have exactly 2 elements, got %d", len(tuple.Elements))
	}
	return &Tuple{Elements: []Object{tuple.Elements[1], tuple.Elements[0]}}
}

// dup: A -> (A, A)
func builtinDup(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dup expects 1 argument, got %d", len(args))
	}
	return &Tuple{Elements: []Object{args[0], args[0]}}
}

// =============================================================================
// Mapping
// =============================================================================

// mapFst: ((A) -> C, (A, B)) -> (C, B)
func builtinMapFst(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("mapFst expects 2 arguments, got %d", len(args))
	}
	fn := args[0]
	tuple, ok := args[1].(*Tuple)
	if !ok {
		return newError("mapFst expects a tuple as second argument, got %s", args[1].Type())
	}
	if len(tuple.Elements) < 2 {
		return newError("mapFst: tuple must have at least 2 elements")
	}

	newFirst := e.ApplyFunction(fn, []Object{tuple.Elements[0]})
	if isError(newFirst) {
		return newFirst
	}

	return &Tuple{Elements: []Object{newFirst, tuple.Elements[1]}}
}

// mapSnd: ((B) -> C, (A, B)) -> (A, C)
func builtinMapSnd(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("mapSnd expects 2 arguments, got %d", len(args))
	}
	fn := args[0]
	tuple, ok := args[1].(*Tuple)
	if !ok {
		return newError("mapSnd expects a tuple as second argument, got %s", args[1].Type())
	}
	if len(tuple.Elements) < 2 {
		return newError("mapSnd: tuple must have at least 2 elements")
	}

	newSecond := e.ApplyFunction(fn, []Object{tuple.Elements[1]})
	if isError(newSecond) {
		return newSecond
	}

	return &Tuple{Elements: []Object{tuple.Elements[0], newSecond}}
}

// mapPair: ((A) -> C, (B) -> D, (A, B)) -> (C, D)
func builtinMapPair(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("mapPair expects 3 arguments, got %d", len(args))
	}
	fnFirst := args[0]
	fnSecond := args[1]
	tuple, ok := args[2].(*Tuple)
	if !ok {
		return newError("mapPair expects a tuple as third argument, got %s", args[2].Type())
	}
	if len(tuple.Elements) < 2 {
		return newError("mapPair: tuple must have at least 2 elements")
	}

	newFirst := e.ApplyFunction(fnFirst, []Object{tuple.Elements[0]})
	if isError(newFirst) {
		return newFirst
	}

	newSecond := e.ApplyFunction(fnSecond, []Object{tuple.Elements[1]})
	if isError(newSecond) {
		return newSecond
	}

	return &Tuple{Elements: []Object{newFirst, newSecond}}
}

// =============================================================================
// Currying
// =============================================================================

// curry: ((A, B) -> C) -> (A) -> (B) -> C
func builtinCurry(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("curry expects 1 argument, got %d", len(args))
	}
	fn := args[0]

	// Return a function that takes A and returns a function that takes B
	return &Builtin{
		Name: "curried",
		Fn: func(e2 *Evaluator, innerArgs ...Object) Object {
			if len(innerArgs) != 1 {
				return newError("curried function expects 1 argument, got %d", len(innerArgs))
			}
			firstArg := innerArgs[0]

			// Return a function that takes B and applies fn to (A, B)
			return &Builtin{
				Name: "curried_inner",
				Fn: func(e3 *Evaluator, innerArgs2 ...Object) Object {
					if len(innerArgs2) != 1 {
						return newError("curried inner function expects 1 argument, got %d", len(innerArgs2))
					}
					secondArg := innerArgs2[0]

					// Create tuple and apply original function
					tuple := &Tuple{Elements: []Object{firstArg, secondArg}}
					return e.ApplyFunction(fn, []Object{tuple})
				},
			}
		},
	}
}

// uncurry: ((A) -> (B) -> C) -> (A, B) -> C
func builtinUncurry(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("uncurry expects 1 argument, got %d", len(args))
	}
	fn := args[0]

	// Return a function that takes (A, B) tuple and applies fn(A)(B)
	return &Builtin{
		Name: "uncurried",
		Fn: func(e2 *Evaluator, innerArgs ...Object) Object {
			if len(innerArgs) != 1 {
				return newError("uncurried function expects 1 argument (tuple), got %d", len(innerArgs))
			}
			tuple, ok := innerArgs[0].(*Tuple)
			if !ok {
				return newError("uncurried function expects a tuple, got %s", innerArgs[0].Type())
			}
			if len(tuple.Elements) < 2 {
				return newError("uncurry: tuple must have at least 2 elements")
			}

			// Apply fn to first element
			intermediate := e.ApplyFunction(fn, []Object{tuple.Elements[0]})
			if isError(intermediate) {
				return intermediate
			}

			// Apply result to second element
			return e.ApplyFunction(intermediate, []Object{tuple.Elements[1]})
		},
	}
}

// =============================================================================
// Predicates
// =============================================================================

// both: ((A) -> Bool, (A, A)) -> Bool
func builtinBoth(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("both expects 2 arguments, got %d", len(args))
	}
	pred := args[0]
	tuple, ok := args[1].(*Tuple)
	if !ok {
		return newError("both expects a tuple as second argument, got %s", args[1].Type())
	}
	if len(tuple.Elements) < 2 {
		return newError("both: tuple must have at least 2 elements")
	}

	// Check first element
	result1 := e.ApplyFunction(pred, []Object{tuple.Elements[0]})
	if isError(result1) {
		return result1
	}
	bool1, ok := result1.(*Boolean)
	if !ok {
		return newError("both: predicate must return Bool, got %s", result1.Type())
	}
	if !bool1.Value {
		return &Boolean{Value: false}
	}

	// Check second element
	result2 := e.ApplyFunction(pred, []Object{tuple.Elements[1]})
	if isError(result2) {
		return result2
	}
	bool2, ok := result2.(*Boolean)
	if !ok {
		return newError("both: predicate must return Bool, got %s", result2.Type())
	}

	return &Boolean{Value: bool2.Value}
}

// either: ((A) -> Bool, (A, A)) -> Bool
func builtinEither(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("either expects 2 arguments, got %d", len(args))
	}
	pred := args[0]
	tuple, ok := args[1].(*Tuple)
	if !ok {
		return newError("either expects a tuple as second argument, got %s", args[1].Type())
	}
	if len(tuple.Elements) < 2 {
		return newError("either: tuple must have at least 2 elements")
	}

	// Check first element
	result1 := e.ApplyFunction(pred, []Object{tuple.Elements[0]})
	if isError(result1) {
		return result1
	}
	bool1, ok := result1.(*Boolean)
	if !ok {
		return newError("either: predicate must return Bool, got %s", result1.Type())
	}
	if bool1.Value {
		return &Boolean{Value: true}
	}

	// Check second element
	result2 := e.ApplyFunction(pred, []Object{tuple.Elements[1]})
	if isError(result2) {
		return result2
	}
	bool2, ok := result2.(*Boolean)
	if !ok {
		return newError("either: predicate must return Bool, got %s", result2.Type())
	}

	return &Boolean{Value: bool2.Value}
}
