package modules

import (
	"github.com/funvibe/funxy/internal/typesystem"
)

func initTuplePackage() {
	// Type variables
	A := typesystem.TVar{Name: "A"}
	B := typesystem.TVar{Name: "B"}
	C := typesystem.TVar{Name: "C"}
	D := typesystem.TVar{Name: "D"}
	T := typesystem.TVar{Name: "T"}

	// Common tuple types
	pairAB := typesystem.TTuple{Elements: []typesystem.Type{A, B}}
	pairBA := typesystem.TTuple{Elements: []typesystem.Type{B, A}}
	pairAA := typesystem.TTuple{Elements: []typesystem.Type{A, A}}
	pairCB := typesystem.TTuple{Elements: []typesystem.Type{C, B}}
	pairAC := typesystem.TTuple{Elements: []typesystem.Type{A, C}}
	pairCD := typesystem.TTuple{Elements: []typesystem.Type{C, D}}

	// Function types for mapping
	aToC := typesystem.TFunc{Params: []typesystem.Type{A}, ReturnType: C}
	bToC := typesystem.TFunc{Params: []typesystem.Type{B}, ReturnType: C}
	bToD := typesystem.TFunc{Params: []typesystem.Type{B}, ReturnType: D}
	aToBool := typesystem.TFunc{Params: []typesystem.Type{A}, ReturnType: typesystem.Bool}

	// Function types for curry/uncurry
	pairABToC := typesystem.TFunc{Params: []typesystem.Type{pairAB}, ReturnType: C}
	aToFunc := typesystem.TFunc{Params: []typesystem.Type{A}, ReturnType: typesystem.TFunc{Params: []typesystem.Type{B}, ReturnType: C}}

	// Tuple type for get (using generic Tuple - we'll handle this specially)
	// For now, use a placeholder - actual implementation will handle any tuple size
	genericTuple := typesystem.TVar{Name: "Tuple"}

	pkg := &VirtualPackage{
		Name: "tuple",
		Symbols: map[string]typesystem.Type{
			// Basic access
			"fst":      typesystem.TFunc{Params: []typesystem.Type{pairAB}, ReturnType: A},
			"snd":      typesystem.TFunc{Params: []typesystem.Type{pairAB}, ReturnType: B},
			"tupleGet": typesystem.TFunc{Params: []typesystem.Type{genericTuple, typesystem.Int}, ReturnType: T},

			// Transformation
			"tupleSwap": typesystem.TFunc{Params: []typesystem.Type{pairAB}, ReturnType: pairBA},
			"tupleDup":  typesystem.TFunc{Params: []typesystem.Type{A}, ReturnType: pairAA},

			// Mapping
			"mapFst":  typesystem.TFunc{Params: []typesystem.Type{aToC, pairAB}, ReturnType: pairCB},
			"mapSnd":  typesystem.TFunc{Params: []typesystem.Type{bToC, pairAB}, ReturnType: pairAC},
			"mapPair": typesystem.TFunc{Params: []typesystem.Type{aToC, bToD, pairAB}, ReturnType: pairCD},

			// Currying
			"curry":   typesystem.TFunc{Params: []typesystem.Type{pairABToC}, ReturnType: aToFunc},
			"uncurry": typesystem.TFunc{Params: []typesystem.Type{aToFunc}, ReturnType: pairABToC},

			// Predicates
			"tupleBoth":   typesystem.TFunc{Params: []typesystem.Type{aToBool, pairAA}, ReturnType: typesystem.Bool},
			"tupleEither": typesystem.TFunc{Params: []typesystem.Type{aToBool, pairAA}, ReturnType: typesystem.Bool},
		},
	}

	RegisterVirtualPackage("lib/tuple", pkg)
}
