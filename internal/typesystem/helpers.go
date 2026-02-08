package typesystem

// FuncType creates a function type (from -> to).
// This helper creates a TFunc for single argument.
// For multi-argument functions, construct TFunc directly.
func FuncType(from, to Type) Type {
	return TFunc{
		Params:     []Type{from},
		ReturnType: to,
	}
}

// Primitive Types helpers
var (
	Int      = TCon{Name: "Int"}
	Float    = TCon{Name: "Float"}
	BigInt   = TCon{Name: "BigInt"}
	Rational = TCon{Name: "Rational"}
	Char     = TCon{Name: "Char"}
	Bool     = TCon{Name: "Bool"}
	Nil      = TCon{Name: "Nil"}
	Bytes    = TCon{Name: "Bytes"}
	Bits     = TCon{Name: "Bits"}
	// String is List<Char>, defined as TApp
	String = TApp{
		Constructor: TCon{Name: "List"},
		Args:        []Type{Char},
	}
	// StringOrBytes is String | Bytes
	StringOrBytes = TUnion{
		Types: []Type{String, Bytes},
	}
)
