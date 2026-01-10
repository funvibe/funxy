package modules

import (
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/typesystem"
)

func initListPackage() {
	// Type variables for generic functions
	T := typesystem.TVar{Name: "T"}
	U := typesystem.TVar{Name: "U"}
	A := typesystem.TVar{Name: "A"}
	B := typesystem.TVar{Name: "B"}
	listT := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{T}}
	listU := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{U}}
	listA := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{A}}
	listB := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{B}}
	listListT := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{listT}}
	tupleAB := typesystem.TTuple{Elements: []typesystem.Type{A, B}}
	listTupleAB := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{tupleAB}}
	tupleListAListB := typesystem.TTuple{Elements: []typesystem.Type{listA, listB}}
	optionInt := typesystem.TApp{Constructor: typesystem.TCon{Name: "Option"}, Args: []typesystem.Type{typesystem.Int}}
	optionT := typesystem.TApp{Constructor: typesystem.TCon{Name: "Option"}, Args: []typesystem.Type{T}}
	tupleListTListT := typesystem.TTuple{Elements: []typesystem.Type{listT, listT}}
	predicateT := typesystem.TFunc{Params: []typesystem.Type{T}, ReturnType: typesystem.Bool}
	pkg := &VirtualPackage{
		Name: "list",
		Symbols: map[string]typesystem.Type{
			// Access
			"head":   typesystem.TFunc{Params: []typesystem.Type{listT}, ReturnType: T},
			"headOr": typesystem.TFunc{Params: []typesystem.Type{listT, T}, ReturnType: T},
			"last":   typesystem.TFunc{Params: []typesystem.Type{listT}, ReturnType: T},
			"lastOr": typesystem.TFunc{Params: []typesystem.Type{listT, T}, ReturnType: T},
			"nth":    typesystem.TFunc{Params: []typesystem.Type{listT, typesystem.Int}, ReturnType: T},
			"nthOr":  typesystem.TFunc{Params: []typesystem.Type{listT, typesystem.Int, T}, ReturnType: T},
			// Sublist
			"tail":  typesystem.TFunc{Params: []typesystem.Type{listT}, ReturnType: listT},
			"init":  typesystem.TFunc{Params: []typesystem.Type{listT}, ReturnType: listT},
			"take":  typesystem.TFunc{Params: []typesystem.Type{listT, typesystem.Int}, ReturnType: listT},
			"drop":  typesystem.TFunc{Params: []typesystem.Type{listT, typesystem.Int}, ReturnType: listT},
			"slice": typesystem.TFunc{Params: []typesystem.Type{listT, typesystem.Int, typesystem.Int}, ReturnType: listT},
			// Predicates
			"length":   typesystem.TFunc{Params: []typesystem.Type{listT}, ReturnType: typesystem.Int},
			"contains": typesystem.TFunc{Params: []typesystem.Type{listT, T}, ReturnType: typesystem.Bool},
			// Higher-order (function-first for pipe compatibility)
			"filter": typesystem.TFunc{Params: []typesystem.Type{typesystem.TFunc{Params: []typesystem.Type{T}, ReturnType: typesystem.Bool}, listT}, ReturnType: listT},
			"map":    typesystem.TFunc{Params: []typesystem.Type{typesystem.TFunc{Params: []typesystem.Type{T}, ReturnType: U}, listT}, ReturnType: listU},
			"foldl":  typesystem.TFunc{Params: []typesystem.Type{typesystem.TFunc{Params: []typesystem.Type{U, T}, ReturnType: U}, U, listT}, ReturnType: U},
			"foldr":  typesystem.TFunc{Params: []typesystem.Type{typesystem.TFunc{Params: []typesystem.Type{T, U}, ReturnType: U}, U, listT}, ReturnType: U},
			// Search (function-first)
			"indexOf":   typesystem.TFunc{Params: []typesystem.Type{listT, T}, ReturnType: optionInt},
			"find":      typesystem.TFunc{Params: []typesystem.Type{predicateT, listT}, ReturnType: optionT},
			"findIndex": typesystem.TFunc{Params: []typesystem.Type{predicateT, listT}, ReturnType: optionInt},
			// Predicates with function (function-first)
			"any": typesystem.TFunc{Params: []typesystem.Type{predicateT, listT}, ReturnType: typesystem.Bool},
			"all": typesystem.TFunc{Params: []typesystem.Type{predicateT, listT}, ReturnType: typesystem.Bool},
			// Conditional (function-first)
			"takeWhile": typesystem.TFunc{Params: []typesystem.Type{predicateT, listT}, ReturnType: listT},
			"dropWhile": typesystem.TFunc{Params: []typesystem.Type{predicateT, listT}, ReturnType: listT},
			// Transformation
			"reverse":   typesystem.TFunc{Params: []typesystem.Type{listT}, ReturnType: listT},
			"concat":    typesystem.TFunc{Params: []typesystem.Type{listT, listT}, ReturnType: listT},
			"flatten":   typesystem.TFunc{Params: []typesystem.Type{listListT}, ReturnType: listT},
			"unique":    typesystem.TFunc{Params: []typesystem.Type{listT}, ReturnType: listT},
			"partition": typesystem.TFunc{Params: []typesystem.Type{predicateT, listT}, ReturnType: tupleListTListT},
			"forEach":   typesystem.TFunc{Params: []typesystem.Type{typesystem.TFunc{Params: []typesystem.Type{T}, ReturnType: typesystem.Nil}, listT}, ReturnType: typesystem.Nil},
			// Combining
			"zip":   typesystem.TFunc{Params: []typesystem.Type{listA, listB}, ReturnType: listTupleAB},
			"unzip": typesystem.TFunc{Params: []typesystem.Type{listTupleAB}, ReturnType: tupleListAListB},
			// Generation
			"range":  typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, typesystem.Int}, ReturnType: typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{typesystem.Int}}},
			"append": typesystem.TFunc{Params: []typesystem.Type{listT, T}, ReturnType: listT},
			// Sorting
			"sort": typesystem.TFunc{
				Params:      []typesystem.Type{listT},
				ReturnType:  listT,
				Constraints: []typesystem.Constraint{{TypeVar: "T", Trait: "Order"}},
			},
			"sortBy": typesystem.TFunc{
				Params: []typesystem.Type{
					listT,
					typesystem.TFunc{Params: []typesystem.Type{T, T}, ReturnType: typesystem.Int},
				},
				ReturnType: listT,
			},
		},
	}
	RegisterVirtualPackage("lib/list", pkg)
}

// initMapPackage registers the lib/map virtual package
func initMapPackage() {
	// Type variables
	K := typesystem.TVar{Name: "K"}
	V := typesystem.TVar{Name: "V"}
	// Map<K, V>
	mapKV := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.MapTypeName},
		Args:        []typesystem.Type{K, V},
	}
	// Option<V>
	optionV := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.OptionTypeName},
		Args:        []typesystem.Type{V},
	}
	// List<K>
	listK := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{K},
	}
	// List<V>
	listV := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{V},
	}
	// List<(K, V)>
	pairKV := typesystem.TTuple{Elements: []typesystem.Type{K, V}}
	listPairs := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{pairKV},
	}
	// For mapFromRecord: Map<String, V>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{typesystem.Char},
	}
	// Use V as the value type placeholder. This assumes generic/mixed values unify to V.
	// Since we don't have Any, we rely on V being inferred or treated as generic.
	mapStringV := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.MapTypeName},
		Args:        []typesystem.Type{stringType, V},
	}
	recordType := typesystem.TVar{Name: "R"}
	pkg := &VirtualPackage{
		Name: "map",
		Symbols: map[string]typesystem.Type{
			// mapNew: () -> Map<K, V>
			"mapNew": typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: mapKV,
			},
			// mapFromRecord: (Record) -> Map<String, V>
			"mapFromRecord": typesystem.TFunc{
				Params:     []typesystem.Type{recordType},
				ReturnType: mapStringV,
			},
			// mapGet: (Map<K, V>, K) -> Option<V>
			"mapGet": typesystem.TFunc{
				Params:     []typesystem.Type{mapKV, K},
				ReturnType: optionV,
			},
			// mapGetOr: (Map<K, V>, K, V) -> V
			"mapGetOr": typesystem.TFunc{
				Params:     []typesystem.Type{mapKV, K, V},
				ReturnType: V,
			},
			// mapPut: (Map<K, V>, K, V) -> Map<K, V>
			"mapPut": typesystem.TFunc{
				Params:     []typesystem.Type{mapKV, K, V},
				ReturnType: mapKV,
			},
			// mapRemove: (Map<K, V>, K) -> Map<K, V>
			"mapRemove": typesystem.TFunc{
				Params:     []typesystem.Type{mapKV, K},
				ReturnType: mapKV,
			},
			// mapKeys: (Map<K, V>) -> List<K>
			"mapKeys": typesystem.TFunc{
				Params:     []typesystem.Type{mapKV},
				ReturnType: listK,
			},
			// mapValues: (Map<K, V>) -> List<V>
			"mapValues": typesystem.TFunc{
				Params:     []typesystem.Type{mapKV},
				ReturnType: listV,
			},
			// mapItems: (Map<K, V>) -> List<(K, V)>
			"mapItems": typesystem.TFunc{
				Params:     []typesystem.Type{mapKV},
				ReturnType: listPairs,
			},
			// mapContains: (Map<K, V>, K) -> Bool
			"mapContains": typesystem.TFunc{
				Params:     []typesystem.Type{mapKV, K},
				ReturnType: typesystem.Bool,
			},
			// mapSize: (Map<K, V>) -> Int
			"mapSize": typesystem.TFunc{
				Params:     []typesystem.Type{mapKV},
				ReturnType: typesystem.Int,
			},
			// mapMerge: (Map<K, V>, Map<K, V>) -> Map<K, V>
			"mapMerge": typesystem.TFunc{
				Params:     []typesystem.Type{mapKV, mapKV},
				ReturnType: mapKV,
			},
		},
	}
	RegisterVirtualPackage("lib/map", pkg)
}

// initBytesPackage registers the lib/bytes virtual package
func initBytesPackage() {
	// Base types
	bytesType := typesystem.TCon{Name: config.BytesTypeName}
	intType := typesystem.TCon{Name: "Int"}
	charType := typesystem.TCon{Name: "Char"}
	// String is List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{charType},
	}
	boolType := typesystem.TCon{Name: "Bool"}
	// Option<Int>
	optionInt := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.OptionTypeName},
		Args:        []typesystem.Type{intType},
	}
	// List<Int>
	listInt := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{intType},
	}
	// List<Bytes>
	listBytes := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{bytesType},
	}
	// Result<String, T>
	resultStringBytes := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ResultTypeName},
		Args:        []typesystem.Type{stringType, bytesType},
	}
	resultStringString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ResultTypeName},
		Args:        []typesystem.Type{stringType, stringType},
	}
	pkg := &VirtualPackage{
		Name: "bytes",
		Symbols: map[string]typesystem.Type{
			// Creation
			"bytesNew":        typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: bytesType},
			"bytesFromString": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: bytesType},
			"bytesFromList":   typesystem.TFunc{Params: []typesystem.Type{listInt}, ReturnType: bytesType},
			"bytesFromHex":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringBytes},
			"bytesFromBin":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringBytes},
			"bytesFromOct":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringBytes},
			// Access
			"bytesSlice": typesystem.TFunc{Params: []typesystem.Type{bytesType, intType, intType}, ReturnType: bytesType},
			// Conversion
			"bytesToString": typesystem.TFunc{Params: []typesystem.Type{bytesType}, ReturnType: resultStringString},
			"bytesToList":   typesystem.TFunc{Params: []typesystem.Type{bytesType}, ReturnType: listInt},
			"bytesToHex":    typesystem.TFunc{Params: []typesystem.Type{bytesType}, ReturnType: stringType},
			"bytesToBin":    typesystem.TFunc{Params: []typesystem.Type{bytesType}, ReturnType: stringType},
			"bytesToOct":    typesystem.TFunc{Params: []typesystem.Type{bytesType}, ReturnType: stringType},
			// Modification
			"bytesConcat": typesystem.TFunc{Params: []typesystem.Type{bytesType, bytesType}, ReturnType: bytesType},
			// Numeric encoding/decoding (with default big-endian)
			"bytesEncodeInt":   typesystem.TFunc{Params: []typesystem.Type{intType, intType, stringType}, ReturnType: bytesType, DefaultCount: 1},
			"bytesDecodeInt":   typesystem.TFunc{Params: []typesystem.Type{bytesType, stringType}, ReturnType: intType, DefaultCount: 1},
			"bytesEncodeFloat": typesystem.TFunc{Params: []typesystem.Type{typesystem.TCon{Name: "Float"}, intType}, ReturnType: bytesType},
			"bytesDecodeFloat": typesystem.TFunc{Params: []typesystem.Type{bytesType, intType}, ReturnType: typesystem.TCon{Name: "Float"}},
			// Search
			"bytesContains":   typesystem.TFunc{Params: []typesystem.Type{bytesType, bytesType}, ReturnType: boolType},
			"bytesIndexOf":    typesystem.TFunc{Params: []typesystem.Type{bytesType, bytesType}, ReturnType: optionInt},
			"bytesStartsWith": typesystem.TFunc{Params: []typesystem.Type{bytesType, bytesType}, ReturnType: boolType},
			"bytesEndsWith":   typesystem.TFunc{Params: []typesystem.Type{bytesType, bytesType}, ReturnType: boolType},
			// Split/Join
			"bytesSplit": typesystem.TFunc{Params: []typesystem.Type{bytesType, bytesType}, ReturnType: listBytes},
			"bytesJoin":  typesystem.TFunc{Params: []typesystem.Type{listBytes, bytesType}, ReturnType: bytesType},
		},
	}
	RegisterVirtualPackage("lib/bytes", pkg)
}

// initBitsPackage registers the lib/bits virtual package
func initBitsPackage() {
	// Base types
	bitsType := typesystem.TCon{Name: config.BitsTypeName}
	bytesType := typesystem.TCon{Name: config.BytesTypeName}
	intType := typesystem.TCon{Name: "Int"}
	charType := typesystem.TCon{Name: "Char"}
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{charType},
	}
	// Option<Int>
	optionInt := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.OptionTypeName},
		Args:        []typesystem.Type{intType},
	}
	// Result<String, T>
	resultStringBits := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ResultTypeName},
		Args:        []typesystem.Type{stringType, bitsType},
	}
	// Map<String, Any> for extracted fields - use a type variable
	T := typesystem.TVar{Name: "T"}
	mapStringT := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.MapTypeName},
		Args:        []typesystem.Type{stringType, T},
	}
	resultStringMapT := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ResultTypeName},
		Args:        []typesystem.Type{stringType, mapStringT},
	}
	// List<Spec> for extraction specs
	specType := typesystem.TVar{Name: "Spec"}
	listSpec := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{specType},
	}
	pkg := &VirtualPackage{
		Name: "bits",
		Symbols: map[string]typesystem.Type{
			// Creation
			"bitsNew":        typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: bitsType},
			"bitsFromBytes":  typesystem.TFunc{Params: []typesystem.Type{bytesType}, ReturnType: bitsType},
			"bitsFromBinary": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringBits},
			"bitsFromHex":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringBits},
			"bitsFromOctal":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringBits},
			// Conversion
			"bitsToBytes":  typesystem.TFunc{Params: []typesystem.Type{bitsType, stringType}, ReturnType: bytesType, DefaultCount: 1},
			"bitsToBinary": typesystem.TFunc{Params: []typesystem.Type{bitsType}, ReturnType: stringType},
			"bitsToHex":    typesystem.TFunc{Params: []typesystem.Type{bitsType}, ReturnType: stringType},
			// Access
			"bitsSlice": typesystem.TFunc{Params: []typesystem.Type{bitsType, intType, intType}, ReturnType: bitsType},
			"bitsGet":   typesystem.TFunc{Params: []typesystem.Type{bitsType, intType}, ReturnType: optionInt},
			// Modification
			"bitsConcat":   typesystem.TFunc{Params: []typesystem.Type{bitsType, bitsType}, ReturnType: bitsType},
			"bitsSet":      typesystem.TFunc{Params: []typesystem.Type{bitsType, intType, intType}, ReturnType: bitsType},
			"bitsPadLeft":  typesystem.TFunc{Params: []typesystem.Type{bitsType, intType}, ReturnType: bitsType},
			"bitsPadRight": typesystem.TFunc{Params: []typesystem.Type{bitsType, intType}, ReturnType: bitsType},
			// Numeric operations
			"bitsAddInt":   typesystem.TFunc{Params: []typesystem.Type{bitsType, intType, intType, stringType}, ReturnType: bitsType, DefaultCount: 1},
			"bitsAddFloat": typesystem.TFunc{Params: []typesystem.Type{bitsType, typesystem.Float, intType}, ReturnType: bitsType},
			// Pattern matching API
			"bitsExtract": typesystem.TFunc{Params: []typesystem.Type{bitsType, listSpec}, ReturnType: resultStringMapT},
			"bitsInt":     typesystem.TFunc{Params: []typesystem.Type{stringType, intType, stringType}, ReturnType: specType},
			"bitsBytes":   typesystem.TFunc{Params: []typesystem.Type{stringType, intType}, ReturnType: specType},
			"bitsRest":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: specType},
		},
	}
	RegisterVirtualPackage("lib/bits", pkg)
}

// initStringPackage registers the lib/string virtual package
func initStringPackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}
	listString := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{stringType}}
	optionInt := typesystem.TApp{Constructor: typesystem.TCon{Name: "Option"}, Args: []typesystem.Type{typesystem.Int}}
	pkg := &VirtualPackage{
		Name: "string",
		Symbols: map[string]typesystem.Type{
			// Split/Join
			"stringSplit": typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: listString},
			"stringJoin":  typesystem.TFunc{Params: []typesystem.Type{listString, stringType}, ReturnType: stringType},
			"stringLines": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: listString},
			"stringWords": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: listString},
			// Trimming
			"stringTrim":      typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"stringTrimStart": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"stringTrimEnd":   typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			// Case conversion
			"stringToUpper":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"stringToLower":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"stringCapitalize": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			// Search/Replace
			"stringReplace":    typesystem.TFunc{Params: []typesystem.Type{stringType, stringType, stringType}, ReturnType: stringType},
			"stringReplaceAll": typesystem.TFunc{Params: []typesystem.Type{stringType, stringType, stringType}, ReturnType: stringType},
			"stringStartsWith": typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: typesystem.Bool},
			"stringEndsWith":   typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: typesystem.Bool},
			"stringIndexOf":    typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: optionInt},
			// Other
			"stringRepeat":   typesystem.TFunc{Params: []typesystem.Type{stringType, typesystem.Int}, ReturnType: stringType},
			"stringPadLeft":  typesystem.TFunc{Params: []typesystem.Type{stringType, typesystem.Int, typesystem.Char}, ReturnType: stringType},
			"stringPadRight": typesystem.TFunc{Params: []typesystem.Type{stringType, typesystem.Int, typesystem.Char}, ReturnType: stringType},
		},
	}
	RegisterVirtualPackage("lib/string", pkg)
}

// initMathPackage registers the lib/math virtual package
func initMathPackage() {
	// Float = Decimal in our type system
	floatType := typesystem.Float
	intType := typesystem.Int
	pkg := &VirtualPackage{
		Name: "math",
		Symbols: map[string]typesystem.Type{
			// Basic operations
			"abs":    typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
			"absInt": typesystem.TFunc{Params: []typesystem.Type{intType}, ReturnType: intType},
			"sign":   typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: intType},
			"min":    typesystem.TFunc{Params: []typesystem.Type{floatType, floatType}, ReturnType: floatType},
			"max":    typesystem.TFunc{Params: []typesystem.Type{floatType, floatType}, ReturnType: floatType},
			"minInt": typesystem.TFunc{Params: []typesystem.Type{intType, intType}, ReturnType: intType},
			"maxInt": typesystem.TFunc{Params: []typesystem.Type{intType, intType}, ReturnType: intType},
			"clamp":  typesystem.TFunc{Params: []typesystem.Type{floatType, floatType, floatType}, ReturnType: floatType},
			// Rounding
			"floor": typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: intType},
			"ceil":  typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: intType},
			"round": typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: intType},
			"trunc": typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: intType},
			// Powers and roots
			"sqrt": typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
			"cbrt": typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
			"pow":  typesystem.TFunc{Params: []typesystem.Type{floatType, floatType}, ReturnType: floatType},
			"exp":  typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
			// Logarithms
			"log":   typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
			"log10": typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
			"log2":  typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
			// Trigonometry
			"sin":   typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
			"cos":   typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
			"tan":   typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
			"asin":  typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
			"acos":  typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
			"atan":  typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
			"atan2": typesystem.TFunc{Params: []typesystem.Type{floatType, floatType}, ReturnType: floatType},
			// Hyperbolic
			"sinh": typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
			"cosh": typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
			"tanh": typesystem.TFunc{Params: []typesystem.Type{floatType}, ReturnType: floatType},
			// Constants (as functions for simplicity)
			"pi": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: floatType},
			"e":  typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: floatType},
		},
	}
	RegisterVirtualPackage("lib/math", pkg)
}

// initBignumPackage registers the lib/bignum virtual package
func initBignumPackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}
	// Option<Int>
	optionInt := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{typesystem.Int},
	}
	// Option<Float>
	optionFloat := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{typesystem.Float},
	}
	pkg := &VirtualPackage{
		Name: "bignum",
		Symbols: map[string]typesystem.Type{
			// BigInt
			"bigIntNew":      typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.BigInt},
			"bigIntFromInt":  typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: typesystem.BigInt},
			"bigIntToString": typesystem.TFunc{Params: []typesystem.Type{typesystem.BigInt}, ReturnType: stringType},
			"bigIntToInt":    typesystem.TFunc{Params: []typesystem.Type{typesystem.BigInt}, ReturnType: optionInt},
			// Rational
			"ratFromInt":  typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, typesystem.Int}, ReturnType: typesystem.Rational},
			"ratNew":      typesystem.TFunc{Params: []typesystem.Type{typesystem.BigInt, typesystem.BigInt}, ReturnType: typesystem.Rational},
			"ratNumer":    typesystem.TFunc{Params: []typesystem.Type{typesystem.Rational}, ReturnType: typesystem.BigInt},
			"ratDenom":    typesystem.TFunc{Params: []typesystem.Type{typesystem.Rational}, ReturnType: typesystem.BigInt},
			"ratToFloat":  typesystem.TFunc{Params: []typesystem.Type{typesystem.Rational}, ReturnType: optionFloat},
			"ratToString": typesystem.TFunc{Params: []typesystem.Type{typesystem.Rational}, ReturnType: stringType},
		},
	}
	RegisterVirtualPackage("lib/bignum", pkg)
}

// initCharPackage registers the lib/char virtual package
func initCharPackage() {
	pkg := &VirtualPackage{
		Name: "char",
		Symbols: map[string]typesystem.Type{
			// Conversion
			"charToCode":   typesystem.TFunc{Params: []typesystem.Type{typesystem.Char}, ReturnType: typesystem.Int},
			"charFromCode": typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: typesystem.Char},
			// Classification
			"charIsUpper": typesystem.TFunc{Params: []typesystem.Type{typesystem.Char}, ReturnType: typesystem.Bool},
			"charIsLower": typesystem.TFunc{Params: []typesystem.Type{typesystem.Char}, ReturnType: typesystem.Bool},
			// Case conversion
			"charToUpper": typesystem.TFunc{Params: []typesystem.Type{typesystem.Char}, ReturnType: typesystem.Char},
			"charToLower": typesystem.TFunc{Params: []typesystem.Type{typesystem.Char}, ReturnType: typesystem.Char},
		},
	}
	RegisterVirtualPackage("lib/char", pkg)
}
