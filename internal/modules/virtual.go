package modules

import (
	"sort"

	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

// VirtualPackage represents a built-in package with pre-defined symbols
type VirtualPackage struct {
	Name    string
	Symbols map[string]typesystem.Type

	// Types exported by this package (registered on import)
	Types map[string]typesystem.Type

	// ADT constructors exported by this package
	Constructors map[string]typesystem.Type

	// ADT variants for pattern matching
	Variants map[string][]string // TypeName -> [VariantName, ...]

	// Extended support for full packages (traits, instances, etc.)
	SymbolTable *symbols.SymbolTable // Full symbol table (optional, for complex packages)

	// Traits defined in this package: TraitName -> {TypeParams, SuperTraits, Methods}
	Traits map[string]*VirtualTrait

	// Operator -> Trait mappings for this package
	OperatorTraits map[string]string
}

// VirtualTrait represents a trait definition in a virtual package
type VirtualTrait struct {
	TypeParams  []string                   // e.g., ["F"] for Functor<F>
	SuperTraits []string                   // e.g., ["Functor"] for Applicative
	Methods     map[string]typesystem.Type // method name -> type signature
	Kind        typesystem.Kind            // e.g., * -> * for Functor
}

// virtualPackages maps package paths to their definitions
var virtualPackages = map[string]*VirtualPackage{}

// RegisterVirtualPackage registers a virtual package
func RegisterVirtualPackage(path string, pkg *VirtualPackage) {
	virtualPackages[path] = pkg
}

// GetVirtualPackage returns a virtual package by path, or nil if not found
func GetVirtualPackage(path string) *VirtualPackage {
	return virtualPackages[path]
}

// IsVirtualPackage checks if a path is a virtual package
func IsVirtualPackage(path string) bool {
	_, ok := virtualPackages[path]
	return ok
}

// CreateVirtualModule creates a Module from a VirtualPackage
func (vp *VirtualPackage) CreateVirtualModule() *Module {
	mod := &Module{
		Name:        vp.Name,
		Dir:         "virtual:" + vp.Name,
		Exports:     make(map[string]bool),
		SymbolTable: symbols.NewSymbolTable(),
		IsVirtual:   true,
	}

	// If package has its own SymbolTable, use it as base
	if vp.SymbolTable != nil {
		mod.SymbolTable = vp.SymbolTable
	}

	origin := vp.Name // Origin module for all symbols in this package

	// Register types exported by this package
	for name, typ := range vp.Types {
		mod.Exports[name] = true
		mod.SymbolTable.DefineType(name, typ, origin)
	}

	// Register constructors exported by this package
	for name, typ := range vp.Constructors {
		mod.Exports[name] = true
		mod.SymbolTable.DefineConstructor(name, typ, origin)
	}

	// Register variants for ADTs
	for typeName, variants := range vp.Variants {
		for _, variant := range variants {
			mod.SymbolTable.RegisterVariant(typeName, variant)
		}
	}

	// Register all simple symbols as exported
	for name, typ := range vp.Symbols {
		mod.Exports[name] = true
		mod.SymbolTable.Define(name, typ, origin)
	}

	// Register traits
	for traitName, trait := range vp.Traits {
		mod.Exports[traitName] = true
		mod.SymbolTable.DefineTrait(traitName, trait.TypeParams, trait.SuperTraits, origin)

		// Register kind if specified
		if trait.Kind != nil {
			mod.SymbolTable.RegisterKind(traitName, trait.Kind)
		}

		// Register trait methods
		for methodName, methodType := range trait.Methods {
			mod.SymbolTable.RegisterTraitMethod(methodName, traitName, methodType, origin)
			mod.SymbolTable.RegisterTraitMethod2(traitName, methodName)
			mod.Exports[methodName] = true
		}
	}

	// Register operator -> trait mappings
	for op, trait := range vp.OperatorTraits {
		mod.SymbolTable.RegisterOperatorTrait(op, trait)
	}

	return mod
}

// InitVirtualPackages initializes all virtual packages
func InitVirtualPackages() {
	initListPackage()
	initMapPackage()
	initBytesPackage()
	initBitsPackage()
	initTimePackage()
	initIOPackage()
	initSysPackage()
	// Note: FP traits (Semigroup, Monoid, Functor, Applicative, Monad) are built-in
	// and don't require import. See analyzer/builtins.go and evaluator/builtins_fp.go
	initTuplePackage()
	initStringPackage()
	initMathPackage()
	initBignumPackage()
	initCharPackage()
	initJsonPackage()
	initCryptoPackage()
	initRegexPackage()
	initHttpPackage()
	initTestPackage()
	initRandPackage()
	initDatePackage()
	initWsPackage()
	initSqlPackage()
	initUrlPackage()
	initPathPackage()
	initUuidPackage()
	initLogPackage()
	initTaskPackage()
	initCsvPackage()
	initFlagPackage()

	// Register "lib" meta-package (import "lib" imports all lib/*)
	initLibMetaPackage()

	// Initialize documentation for all packages including prelude (builtins)
	InitDocumentation()
}

// GetLibSubPackages returns all lib/* package names dynamically
// by scanning registered virtual packages
func GetLibSubPackages() []string {
	var packages []string
	for path := range virtualPackages {
		if len(path) > 4 && path[:4] == "lib/" {
			packages = append(packages, path[4:]) // strip "lib/" prefix
		}
	}
	// Sort for deterministic order
	sort.Strings(packages)
	return packages
}

// initLibMetaPackage registers the "lib" meta-package
// This combines all symbols from all lib/* packages
func initLibMetaPackage() {
	// Collect all symbols from all lib/* packages
	allSymbols := make(map[string]typesystem.Type)

	for _, pkgName := range GetLibSubPackages() {
		subPkg := GetVirtualPackage("lib/" + pkgName)
		if subPkg != nil {
			for name, typ := range subPkg.Symbols {
				allSymbols[name] = typ
			}
		}
	}

	pkg := &VirtualPackage{
		Name:    "lib",
		Symbols: allSymbols,
	}
	RegisterVirtualPackage("lib", pkg)
}

// initListPackage registers the lib/list virtual package
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
			"range": typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, typesystem.Int}, ReturnType: typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{typesystem.Int}}},

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

	pkg := &VirtualPackage{
		Name: "map",
		Symbols: map[string]typesystem.Type{
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
			"bitsConcat":  typesystem.TFunc{Params: []typesystem.Type{bitsType, bitsType}, ReturnType: bitsType},
			"bitsSet":     typesystem.TFunc{Params: []typesystem.Type{bitsType, intType, intType}, ReturnType: bitsType},
			"bitsPadLeft": typesystem.TFunc{Params: []typesystem.Type{bitsType, intType}, ReturnType: bitsType},
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

// initTimePackage registers the lib/time virtual package
func initTimePackage() {
	pkg := &VirtualPackage{
		Name: "time",
		Symbols: map[string]typesystem.Type{
			// Unix timestamp
			"timeNow": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Int},

			// Monotonic clocks for benchmarking
			"clockNs": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Int},
			"clockMs": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Int},

			// Sleep functions
			"sleep":   typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: typesystem.Nil},
			"sleepMs": typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: typesystem.Nil},
		},
	}

	RegisterVirtualPackage("lib/time", pkg)
}

// initIOPackage registers the lib/io virtual package
func initIOPackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}
	// List<String>
	listString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{stringType},
	}
	// Result<E, A> - E is error type, A is success type (like Haskell Either)
	// Result<String, String>
	resultStringString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, stringType},
	}
	// Result<String, Int> - error is String, success is Int
	resultStringInt := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, typesystem.Int},
	}
	// Result<String, Nil> - error is String, success is Nil
	resultStringNil := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, typesystem.Nil},
	}
	// Result<String, List<String>>
	resultStringListString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, listString},
	}
	// Option<String>
	optionString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{stringType},
	}

	pkg := &VirtualPackage{
		Name: "io",
		Symbols: map[string]typesystem.Type{
			// Console
			"readLine": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: optionString},

			// File reading
			"fileRead":   typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringString},
			"fileReadAt": typesystem.TFunc{Params: []typesystem.Type{stringType, typesystem.Int, typesystem.Int}, ReturnType: resultStringString},

			// File writing
			"fileWrite":  typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: resultStringInt},
			"fileAppend": typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: resultStringInt},

			// File info
			"fileExists": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Bool},
			"fileSize":   typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringInt},

			// File management
			"fileDelete": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringNil},

			// Directory operations
			"dirCreate":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringNil},
			"dirCreateAll": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringNil},
			"dirRemove":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringNil},
			"dirRemoveAll": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringNil},
			"dirList":      typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringListString},
			"dirExists":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Bool},

			// Path type checks
			"isDir":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Bool},
			"isFile": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Bool},
		},
	}

	RegisterVirtualPackage("lib/io", pkg)
}

// initSysPackage registers the lib/sys virtual package
func initSysPackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}
	// List<String>
	listString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{stringType},
	}
	// Option<String>
	optionString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{stringType},
	}

	// ExecResult = { code: Int, stdout: String, stderr: String }
	execResultType := typesystem.TRecord{
		Fields: map[string]typesystem.Type{
			"code":   typesystem.Int,
			"stdout": stringType,
			"stderr": stringType,
		},
	}

	pkg := &VirtualPackage{
		Name: "sys",
		Symbols: map[string]typesystem.Type{
			// Command line arguments
			"sysArgs": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: listString},

			// Environment variable
			"sysEnv": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: optionString},

			// Exit with code
			"sysExit": typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: typesystem.Nil},

			// Execute command: exec(cmd: String, args: List<String>) -> { code: Int, stdout: String, stderr: String }
			"sysExec": typesystem.TFunc{Params: []typesystem.Type{stringType, listString}, ReturnType: execResultType},
		},
	}

	RegisterVirtualPackage("lib/sys", pkg)
}

// initTuplePackage registers the lib/tuple virtual package
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

// initJsonPackage registers the lib/json virtual package
func initJsonPackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}

	// Json type
	jsonType := typesystem.TCon{Name: "Json"}

	// Option<T>
	optionType := func(t typesystem.Type) typesystem.Type {
		return typesystem.TApp{
			Constructor: typesystem.TCon{Name: "Option"},
			Args:        []typesystem.Type{t},
		}
	}

	// Result<String, T> - error is String, success is T
	resultType := func(t typesystem.Type) typesystem.Type {
		return typesystem.TApp{
			Constructor: typesystem.TCon{Name: "Result"},
			Args:        []typesystem.Type{stringType, t},
		}
	}

	// Generic type variable
	tVar := typesystem.TVar{Name: "T"}

	pkg := &VirtualPackage{
		Name: "json",
		Symbols: map[string]typesystem.Type{
			// jsonEncode(value) -> String
			// Encodes any value to JSON string
			"jsonEncode": typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.TVar{Name: "A"}},
				ReturnType: stringType,
			},

			// jsonDecode<T>(json: String) -> Result<T, String>
			// Decodes JSON string to typed value
			"jsonDecode": typesystem.TFunc{
				Params:     []typesystem.Type{stringType},
				ReturnType: resultType(tVar),
			},

			// jsonParse(str: String) -> Result<Json, String>
			// Parses JSON string into Json ADT
			"jsonParse": typesystem.TFunc{
				Params:     []typesystem.Type{stringType},
				ReturnType: resultType(jsonType),
			},

			// jsonFromValue(value) -> Json
			// Converts any value to Json ADT
			"jsonFromValue": typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.TVar{Name: "A"}},
				ReturnType: jsonType,
			},

			// jsonGet(json: Json, key: String) -> Option<Json>
			// Gets a field from a JObj
			"jsonGet": typesystem.TFunc{
				Params:     []typesystem.Type{jsonType, stringType},
				ReturnType: optionType(jsonType),
			},

			// jsonKeys(json: Json) -> List<String>
			// Gets all keys from a JObj
			"jsonKeys": typesystem.TFunc{
				Params:     []typesystem.Type{jsonType},
				ReturnType: typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{stringType}},
			},
		},
	}

	RegisterVirtualPackage("lib/json", pkg)
}

// initCryptoPackage registers the lib/crypto virtual package
func initCryptoPackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}

	pkg := &VirtualPackage{
		Name: "crypto",
		Symbols: map[string]typesystem.Type{
			// Base64 encoding/decoding
			"base64Encode": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"base64Decode": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},

			// Hex encoding/decoding
			"hexEncode": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"hexDecode": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},

			// Hash functions (return hex string)
			"md5":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"sha1":   typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"sha256": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"sha512": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},

			// HMAC (key, message) -> hex string
			"hmacSha256": typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: stringType},
			"hmacSha512": typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: stringType},

			// Cryptographically secure random
			"cryptoRandomBytes": typesystem.TFunc{
				Params: []typesystem.Type{typesystem.Int},
				ReturnType: typesystem.TApp{
					Constructor: typesystem.TCon{Name: "List"},
					Args:        []typesystem.Type{typesystem.Int},
				},
			},
			"cryptoRandomHex": typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.Int},
				ReturnType: stringType,
			},
		},
	}

	RegisterVirtualPackage("lib/crypto", pkg)
}

// initRegexPackage registers the lib/regex virtual package
func initRegexPackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}
	// List<String>
	listString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{stringType},
	}
	// Option<String>
	optionString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{stringType},
	}
	// Option<List<String>>
	optionListString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{listString},
	}
	// Result<String, T> - error is String, success is T
	resultType := func(t typesystem.Type) typesystem.Type {
		return typesystem.TApp{
			Constructor: typesystem.TCon{Name: "Result"},
			Args:        []typesystem.Type{stringType, t},
		}
	}

	pkg := &VirtualPackage{
		Name: "regex",
		Symbols: map[string]typesystem.Type{
			// Basic matching
			"regexMatch": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType},
				ReturnType: typesystem.Bool,
			},

			// Find first match
			"regexFind": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType},
				ReturnType: optionString,
			},

			// Find all matches
			"regexFindAll": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType},
				ReturnType: listString,
			},

			// Capture groups from first match
			"regexCapture": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType},
				ReturnType: optionListString,
			},

			// Replace first match
			"regexReplace": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType, stringType},
				ReturnType: stringType,
			},

			// Replace all matches
			"regexReplaceAll": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType, stringType},
				ReturnType: stringType,
			},

			// Split by pattern
			"regexSplit": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType},
				ReturnType: listString,
			},

			// Validate regex pattern (returns error if invalid)
			"regexValidate": typesystem.TFunc{
				Params:     []typesystem.Type{stringType},
				ReturnType: resultType(typesystem.Nil),
			},
		},
	}

	RegisterVirtualPackage("lib/regex", pkg)
}

// initHttpPackage registers the lib/http virtual package
func initHttpPackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}

	// (String, String) - header tuple
	headerTuple := typesystem.TTuple{
		Elements: []typesystem.Type{stringType, stringType},
	}

	// List<(String, String)> - headers
	headersType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{headerTuple},
	}

	// HttpResponse = { status: Int, body: String, headers: List<(String, String)> }
	responseType := typesystem.TRecord{
		Fields: map[string]typesystem.Type{
			"status":  typesystem.Int,
			"body":    stringType,
			"headers": headersType,
		},
	}

	// Result<String, HttpResponse> - error is String, success is HttpResponse
	resultStringResponse := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, responseType},
	}

	pkg := &VirtualPackage{
		Name: "http",
		Symbols: map[string]typesystem.Type{
			// Simple GET request
			"httpGet": typesystem.TFunc{
				Params:     []typesystem.Type{stringType},
				ReturnType: resultStringResponse,
			},

			// POST with string body
			"httpPost": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType},
				ReturnType: resultStringResponse,
			},

			// POST with JSON body (auto-encodes)
			"httpPostJson": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, typesystem.TVar{Name: "A"}},
				ReturnType: resultStringResponse,
			},

			// PUT with string body
			"httpPut": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType},
				ReturnType: resultStringResponse,
			},

			// DELETE request
			"httpDelete": typesystem.TFunc{
				Params:     []typesystem.Type{stringType},
				ReturnType: resultStringResponse,
			},

			// Full control request (timeout in ms, 0 = use global default)
			// Last 2 params have defaults: body="" and timeout=0
			"httpRequest": typesystem.TFunc{
				Params:       []typesystem.Type{stringType, stringType, headersType, stringType, typesystem.Int},
				ReturnType:   resultStringResponse,
				DefaultCount: 2,
			},

			// Set default timeout (milliseconds)
			"httpSetTimeout": typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.Int},
				ReturnType: typesystem.Nil,
			},

			// ========== Server functions ==========

			// HttpRequest = { method: String, path: String, query: String, headers: List<(String, String)>, body: String }
			// httpServe: (Int, (HttpRequest) -> HttpResponse) -> Result<Nil, String>
			// Starts server and blocks, calling handler for each request
			"httpServe": typesystem.TFunc{
				Params: []typesystem.Type{
					typesystem.Int,
					typesystem.TFunc{
						Params: []typesystem.Type{
							// HttpRequest record
							typesystem.TRecord{
								Fields: map[string]typesystem.Type{
									"method":  stringType,
									"path":    stringType,
									"query":   stringType,
									"headers": headersType,
									"body":    stringType,
								},
							},
						},
						ReturnType: responseType,
					},
				},
				ReturnType: typesystem.TApp{
					Constructor: typesystem.TCon{Name: "Result"},
					Args:        []typesystem.Type{stringType, typesystem.Nil},
				},
			},

			// httpServeAsync: (Int, (HttpRequest) -> HttpResponse) -> Int
			// Starts server in background, returns server ID
			"httpServeAsync": typesystem.TFunc{
				Params: []typesystem.Type{
					typesystem.Int,
					typesystem.TFunc{
						Params: []typesystem.Type{
							typesystem.TRecord{
								Fields: map[string]typesystem.Type{
									"method":  stringType,
									"path":    stringType,
									"query":   stringType,
									"headers": headersType,
									"body":    stringType,
								},
							},
						},
						ReturnType: responseType,
					},
				},
				ReturnType: typesystem.Int,
			},

			// httpServerStop: (Int, Int) -> Nil
			// Stops a running server by ID. Optional timeout.
			"httpServerStop": typesystem.TFunc{
				Params:       []typesystem.Type{typesystem.Int, typesystem.Int},
				ReturnType:   typesystem.Nil,
				DefaultCount: 1,
			},
		},
	}

	RegisterVirtualPackage("lib/http", pkg)
}

// initTestPackage registers the lib/test virtual package
func initTestPackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}

	// Generic type variable
	T := typesystem.TVar{Name: "T"}
	A := typesystem.TVar{Name: "A"}
	E := typesystem.TVar{Name: "E"}

	// Option<T>
	optionT := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{T},
	}

	// Result<E, T> - E is error, T is success
	resultET := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{E, T},
	}

	// Result<String, A> for mock returns - error is String, success is A
	resultStringA := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, A},
	}

	// HttpResponse type (same as in lib/http)
	headerTuple := typesystem.TTuple{
		Elements: []typesystem.Type{stringType, stringType},
	}
	headersType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{headerTuple},
	}
	responseType := typesystem.TRecord{
		Fields: map[string]typesystem.Type{
			"status":  typesystem.Int,
			"body":    stringType,
			"headers": headersType,
		},
	}

	// Test body function type: () -> Nil
	testBodyType := typesystem.TFunc{
		Params:     []typesystem.Type{},
		ReturnType: typesystem.Nil,
	}

	pkg := &VirtualPackage{
		Name: "test",
		Symbols: map[string]typesystem.Type{
			// Test definition
			"testRun": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, testBodyType},
				ReturnType: typesystem.Nil,
			},
			"testSkip": typesystem.TFunc{
				Params:     []typesystem.Type{stringType},
				ReturnType: typesystem.Nil,
			},
			"testExpectFail": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, testBodyType},
				ReturnType: typesystem.Nil,
			},

			// Assertions (all accept optional message as last argument)
			"assert": typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.Bool, stringType},
				ReturnType: typesystem.Nil,
				IsVariadic: true,
			},
			"assertEquals": typesystem.TFunc{
				Params:     []typesystem.Type{T, T, stringType},
				ReturnType: typesystem.Nil,
				IsVariadic: true,
			},
			"assertOk": typesystem.TFunc{
				Params:     []typesystem.Type{resultET, stringType},
				ReturnType: typesystem.Nil,
				IsVariadic: true,
			},
			"assertFail": typesystem.TFunc{
				Params:     []typesystem.Type{resultET, stringType},
				ReturnType: typesystem.Nil,
				IsVariadic: true,
			},
			"assertSome": typesystem.TFunc{
				Params:     []typesystem.Type{optionT, stringType},
				ReturnType: typesystem.Nil,
				IsVariadic: true,
			},
			"assertZero": typesystem.TFunc{
				Params:     []typesystem.Type{optionT, stringType},
				ReturnType: typesystem.Nil,
				IsVariadic: true,
			},

			// HTTP mocks
			"mockHttp": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, responseType},
				ReturnType: typesystem.Nil,
			},
			"mockHttpError": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType},
				ReturnType: typesystem.Nil,
			},
			"mockHttpOff": typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: typesystem.Nil,
			},
			"mockHttpBypass": typesystem.TFunc{
				Params:     []typesystem.Type{A},
				ReturnType: A,
			},

			// File mocks
			"mockFile": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, resultStringA},
				ReturnType: typesystem.Nil,
			},
			"mockFileOff": typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: typesystem.Nil,
			},
			"mockFileBypass": typesystem.TFunc{
				Params:     []typesystem.Type{A},
				ReturnType: A,
			},

			// Env mocks
			"mockEnv": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, stringType},
				ReturnType: typesystem.Nil,
			},
			"mockEnvOff": typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: typesystem.Nil,
			},
			"mockEnvBypass": typesystem.TFunc{
				Params:     []typesystem.Type{A},
				ReturnType: A,
			},
		},
	}

	RegisterVirtualPackage("lib/test", pkg)
}

// initRandPackage registers the lib/rand virtual package
func initRandPackage() {
	// Generic type variable
	typeA := typesystem.TVar{Name: "A"}

	// List<A>
	listA := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typeA},
	}

	// Option<A>
	optionA := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{typeA},
	}

	pkg := &VirtualPackage{
		Name: "rand",
		Symbols: map[string]typesystem.Type{
			// randomInt: () -> Int
			"randomInt": typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: typesystem.Int,
			},
			// randomIntRange: (Int, Int) -> Int
			"randomIntRange": typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.Int, typesystem.Int},
				ReturnType: typesystem.Int,
			},
			// randomFloat: () -> Float
			"randomFloat": typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: typesystem.Float,
			},
			// randomFloatRange: (Float, Float) -> Float
			"randomFloatRange": typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.Float, typesystem.Float},
				ReturnType: typesystem.Float,
			},
			// randomBool: () -> Bool
			"randomBool": typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: typesystem.Bool,
			},
			// randomChoice: List<A> -> Option<A>
			"randomChoice": typesystem.TFunc{
				Params:     []typesystem.Type{listA},
				ReturnType: optionA,
			},
			// randomShuffle: List<A> -> List<A>
			"randomShuffle": typesystem.TFunc{
				Params:     []typesystem.Type{listA},
				ReturnType: listA,
			},
			// randomSample: (List<A>, Int) -> List<A>
			"randomSample": typesystem.TFunc{
				Params:     []typesystem.Type{listA, typesystem.Int},
				ReturnType: listA,
			},
			// randomSeed: Int -> Nil
			"randomSeed": typesystem.TFunc{
				Params:     []typesystem.Type{typesystem.Int},
				ReturnType: typesystem.Nil,
			},
		},
	}

	RegisterVirtualPackage("lib/rand", pkg)
}

// initDatePackage registers the lib/date virtual package
func initDatePackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}

	// Date = { year, month, day, hour, minute, second, offset }
	// offset is in minutes from UTC (e.g., 180 = UTC+3, -300 = UTC-5)
	dateType := typesystem.TRecord{
		Fields: map[string]typesystem.Type{
			"year":   typesystem.Int,
			"month":  typesystem.Int,
			"day":    typesystem.Int,
			"hour":   typesystem.Int,
			"minute": typesystem.Int,
			"second": typesystem.Int,
			"offset": typesystem.Int,
		},
	}

	// Option<Date>
	optionDate := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{dateType},
	}

	pkg := &VirtualPackage{
		Name: "date",
		Symbols: map[string]typesystem.Type{
			// Creation (dateNew and dateNewTime have optional offset, default = local)
			"dateNow":           typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: dateType},
			"dateNowUtc":        typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: dateType},
			"dateFromTimestamp": typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: dateType},
			"dateNew": typesystem.TFunc{
				Params:       []typesystem.Type{typesystem.Int, typesystem.Int, typesystem.Int, typesystem.Int},
				ReturnType:   dateType,
				DefaultCount: 1, // offset is optional
			},
			"dateNewTime": typesystem.TFunc{
				Params:       []typesystem.Type{typesystem.Int, typesystem.Int, typesystem.Int, typesystem.Int, typesystem.Int, typesystem.Int, typesystem.Int},
				ReturnType:   dateType,
				DefaultCount: 1, // offset is optional
			},
			"dateToTimestamp": typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: typesystem.Int},

			// Timezone/Offset
			"dateToUtc":      typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: dateType},
			"dateToLocal":    typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: dateType},
			"dateOffset":     typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: typesystem.Int},
			"dateWithOffset": typesystem.TFunc{Params: []typesystem.Type{dateType, typesystem.Int}, ReturnType: dateType},

			// Formatting
			"dateFormat": typesystem.TFunc{Params: []typesystem.Type{dateType, stringType}, ReturnType: stringType},
			"dateParse":  typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: optionDate},

			// Components
			"dateYear":    typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: typesystem.Int},
			"dateMonth":   typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: typesystem.Int},
			"dateDay":     typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: typesystem.Int},
			"dateWeekday": typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: typesystem.Int},
			"dateHour":    typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: typesystem.Int},
			"dateMinute":  typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: typesystem.Int},
			"dateSecond":  typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: typesystem.Int},

			// Arithmetic
			"dateAddDays":    typesystem.TFunc{Params: []typesystem.Type{dateType, typesystem.Int}, ReturnType: dateType},
			"dateAddMonths":  typesystem.TFunc{Params: []typesystem.Type{dateType, typesystem.Int}, ReturnType: dateType},
			"dateAddYears":   typesystem.TFunc{Params: []typesystem.Type{dateType, typesystem.Int}, ReturnType: dateType},
			"dateAddHours":   typesystem.TFunc{Params: []typesystem.Type{dateType, typesystem.Int}, ReturnType: dateType},
			"dateAddMinutes": typesystem.TFunc{Params: []typesystem.Type{dateType, typesystem.Int}, ReturnType: dateType},
			"dateAddSeconds": typesystem.TFunc{Params: []typesystem.Type{dateType, typesystem.Int}, ReturnType: dateType},

			// Difference
			"dateDiffDays":    typesystem.TFunc{Params: []typesystem.Type{dateType, dateType}, ReturnType: typesystem.Int},
			"dateDiffSeconds": typesystem.TFunc{Params: []typesystem.Type{dateType, dateType}, ReturnType: typesystem.Int},
		},
	}

	RegisterVirtualPackage("lib/date", pkg)
}

// initWsPackage registers WebSocket package
func initWsPackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}

	// Result<E, A> types - E is error, A is success
	resultInt := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, typesystem.Int},
	}
	resultNil := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, typesystem.Nil},
	}
	resultString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, stringType},
	}
	optionString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{stringType},
	}
	// Result<String, Option<String>> - error is String, success is Option<String>
	resultStringOptionString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, optionString},
	}

	// Handler type: (Int, String) -> String
	handlerType := typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.Int, stringType},
		ReturnType: stringType,
	}

	pkg := &VirtualPackage{
		Name: "ws",
		Symbols: map[string]typesystem.Type{
			"wsConnect":        typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultInt},
			"wsConnectTimeout": typesystem.TFunc{Params: []typesystem.Type{stringType, typesystem.Int}, ReturnType: resultInt},
			"wsSend":           typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, stringType}, ReturnType: resultNil},
			"wsRecv":           typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: resultString},
			"wsRecvTimeout":    typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, typesystem.Int}, ReturnType: resultStringOptionString},
			"wsClose":          typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: resultNil},
			"wsServe":          typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, handlerType}, ReturnType: resultNil},
			"wsServeAsync":     typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, handlerType}, ReturnType: resultInt},
			"wsServerStop":     typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: resultNil},
		},
	}

	RegisterVirtualPackage("lib/ws", pkg)
}

func initSqlPackage() {
	stringType := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{typesystem.Char}}
	nilType := typesystem.Nil
	intType := typesystem.Int
	boolType := typesystem.Bool

	// SqlDB and SqlTx are opaque types
	sqlDBType := typesystem.TCon{Name: "SqlDB"}
	sqlTxType := typesystem.TCon{Name: "SqlTx"}

	// SqlValue ADT
	sqlValueType := typesystem.TCon{Name: "SqlValue"}

	// Result types
	resultDB := typesystem.TApp{Constructor: typesystem.TCon{Name: config.ResultTypeName}, Args: []typesystem.Type{stringType, sqlDBType}}
	resultTx := typesystem.TApp{Constructor: typesystem.TCon{Name: config.ResultTypeName}, Args: []typesystem.Type{stringType, sqlTxType}}
	resultNil := typesystem.TApp{Constructor: typesystem.TCon{Name: config.ResultTypeName}, Args: []typesystem.Type{stringType, nilType}}
	resultInt := typesystem.TApp{Constructor: typesystem.TCon{Name: config.ResultTypeName}, Args: []typesystem.Type{stringType, intType}}

	// Row = Map<String, SqlValue>
	rowType := typesystem.TApp{Constructor: typesystem.TCon{Name: "Map"}, Args: []typesystem.Type{stringType, sqlValueType}}
	listRow := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{rowType}}
	optionRow := typesystem.TApp{Constructor: typesystem.TCon{Name: config.OptionTypeName}, Args: []typesystem.Type{rowType}}
	resultListRow := typesystem.TApp{Constructor: typesystem.TCon{Name: config.ResultTypeName}, Args: []typesystem.Type{stringType, listRow}}
	resultOptionRow := typesystem.TApp{Constructor: typesystem.TCon{Name: config.ResultTypeName}, Args: []typesystem.Type{stringType, optionRow}}

	// Params = List<any> (we use a generic param type)
	anyType := typesystem.TVar{Name: "a"}
	paramsType := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{anyType}}

	// Option<SqlValue>
	optionSqlValue := typesystem.TApp{Constructor: typesystem.TCon{Name: config.OptionTypeName}, Args: []typesystem.Type{sqlValueType}}

	// Date type for SqlTime
	dateType := typesystem.TRecord{
		Fields: map[string]typesystem.Type{
			"year":   typesystem.Int,
			"month":  typesystem.Int,
			"day":    typesystem.Int,
			"hour":   typesystem.Int,
			"minute": typesystem.Int,
			"second": typesystem.Int,
			"offset": typesystem.Int,
		},
	}
	bytesType := typesystem.TCon{Name: "Bytes"}
	bigIntType := typesystem.TCon{Name: "BigInt"}

	pkg := &VirtualPackage{
		Name: "sql",
		Types: map[string]typesystem.Type{
			"SqlValue": sqlValueType,
			"SqlDB":    sqlDBType,
			"SqlTx":    sqlTxType,
			"Date":     dateType,
		},
		Constructors: map[string]typesystem.Type{
			"SqlNull":   sqlValueType,
			"SqlInt":    typesystem.TFunc{Params: []typesystem.Type{intType}, ReturnType: sqlValueType},
			"SqlFloat":  typesystem.TFunc{Params: []typesystem.Type{typesystem.Float}, ReturnType: sqlValueType},
			"SqlString": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: sqlValueType},
			"SqlBool":   typesystem.TFunc{Params: []typesystem.Type{boolType}, ReturnType: sqlValueType},
			"SqlBytes":  typesystem.TFunc{Params: []typesystem.Type{bytesType}, ReturnType: sqlValueType},
			"SqlTime":   typesystem.TFunc{Params: []typesystem.Type{dateType}, ReturnType: sqlValueType},
			"SqlBigInt": typesystem.TFunc{Params: []typesystem.Type{bigIntType}, ReturnType: sqlValueType},
		},
		Variants: map[string][]string{
			"SqlValue": {"SqlNull", "SqlInt", "SqlFloat", "SqlString", "SqlBool", "SqlBytes", "SqlTime", "SqlBigInt"},
		},
		Symbols: map[string]typesystem.Type{
			// Connection
			"sqlOpen":  typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: resultDB},
			"sqlClose": typesystem.TFunc{Params: []typesystem.Type{sqlDBType}, ReturnType: resultNil},
			"sqlPing":  typesystem.TFunc{Params: []typesystem.Type{sqlDBType}, ReturnType: resultNil},

			// Query
			"sqlQuery":        typesystem.TFunc{Params: []typesystem.Type{sqlDBType, stringType, paramsType}, ReturnType: resultListRow},
			"sqlQueryRow":     typesystem.TFunc{Params: []typesystem.Type{sqlDBType, stringType, paramsType}, ReturnType: resultOptionRow},
			"sqlExec":         typesystem.TFunc{Params: []typesystem.Type{sqlDBType, stringType, paramsType}, ReturnType: resultInt},
			"sqlLastInsertId": typesystem.TFunc{Params: []typesystem.Type{sqlDBType, stringType, paramsType}, ReturnType: resultInt},

			// Transaction
			"sqlBegin":    typesystem.TFunc{Params: []typesystem.Type{sqlDBType}, ReturnType: resultTx},
			"sqlCommit":   typesystem.TFunc{Params: []typesystem.Type{sqlTxType}, ReturnType: resultNil},
			"sqlRollback": typesystem.TFunc{Params: []typesystem.Type{sqlTxType}, ReturnType: resultNil},
			"sqlTxQuery":  typesystem.TFunc{Params: []typesystem.Type{sqlTxType, stringType, paramsType}, ReturnType: resultListRow},
			"sqlTxExec":   typesystem.TFunc{Params: []typesystem.Type{sqlTxType, stringType, paramsType}, ReturnType: resultInt},

			// Utility
			"sqlUnwrap": typesystem.TFunc{Params: []typesystem.Type{sqlValueType}, ReturnType: optionSqlValue},
			"sqlIsNull": typesystem.TFunc{Params: []typesystem.Type{sqlValueType}, ReturnType: boolType},
		},
	}

	RegisterVirtualPackage("lib/sql", pkg)
}

// initUrlPackage registers the lib/url virtual package
func initUrlPackage() {
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}

	optionInt := typesystem.TApp{Constructor: typesystem.TCon{Name: "Option"}, Args: []typesystem.Type{typesystem.Int}}

	// Url record type
	urlType := typesystem.TRecord{
		Fields: map[string]typesystem.Type{
			"scheme":   stringType,
			"userinfo": stringType,
			"host":     stringType,
			"port":     optionInt,
			"path":     stringType,
			"query":    stringType,
			"fragment": stringType,
		},
	}

	// Result<String, Url>
	resultUrl := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, urlType},
	}

	// Option<String>
	optionString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{stringType},
	}

	// List<String>
	listString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{stringType},
	}

	// Map<String, List<String>>
	mapStringListString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Map"},
		Args:        []typesystem.Type{stringType, listString},
	}

	// Result<String, String>
	resultString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, stringType},
	}

	pkg := &VirtualPackage{
		Name: "url",
		Symbols: map[string]typesystem.Type{
			// Parsing
			"urlParse":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultUrl},
			"urlToString": typesystem.TFunc{Params: []typesystem.Type{urlType}, ReturnType: stringType},

			// Accessors
			"urlScheme":   typesystem.TFunc{Params: []typesystem.Type{urlType}, ReturnType: stringType},
			"urlUserinfo": typesystem.TFunc{Params: []typesystem.Type{urlType}, ReturnType: stringType},
			"urlHost":     typesystem.TFunc{Params: []typesystem.Type{urlType}, ReturnType: stringType},
			"urlPort":     typesystem.TFunc{Params: []typesystem.Type{urlType}, ReturnType: optionInt},
			"urlPath":     typesystem.TFunc{Params: []typesystem.Type{urlType}, ReturnType: stringType},
			"urlQuery":    typesystem.TFunc{Params: []typesystem.Type{urlType}, ReturnType: stringType},
			"urlFragment": typesystem.TFunc{Params: []typesystem.Type{urlType}, ReturnType: stringType},

			// Query params
			"urlQueryParams":   typesystem.TFunc{Params: []typesystem.Type{urlType}, ReturnType: mapStringListString},
			"urlQueryParam":    typesystem.TFunc{Params: []typesystem.Type{urlType, stringType}, ReturnType: optionString},
			"urlQueryParamAll": typesystem.TFunc{Params: []typesystem.Type{urlType, stringType}, ReturnType: listString},

			// Modifiers
			"urlWithScheme":    typesystem.TFunc{Params: []typesystem.Type{urlType, stringType}, ReturnType: urlType},
			"urlWithUserinfo":  typesystem.TFunc{Params: []typesystem.Type{urlType, stringType}, ReturnType: urlType},
			"urlWithHost":      typesystem.TFunc{Params: []typesystem.Type{urlType, stringType}, ReturnType: urlType},
			"urlWithPort":      typesystem.TFunc{Params: []typesystem.Type{urlType, typesystem.Int}, ReturnType: urlType},
			"urlWithPath":      typesystem.TFunc{Params: []typesystem.Type{urlType, stringType}, ReturnType: urlType},
			"urlWithQuery":     typesystem.TFunc{Params: []typesystem.Type{urlType, stringType}, ReturnType: urlType},
			"urlWithFragment":  typesystem.TFunc{Params: []typesystem.Type{urlType, stringType}, ReturnType: urlType},
			"urlAddQueryParam": typesystem.TFunc{Params: []typesystem.Type{urlType, stringType, stringType}, ReturnType: urlType},

			// Utility
			"urlJoin":   typesystem.TFunc{Params: []typesystem.Type{urlType, stringType}, ReturnType: resultUrl},
			"urlEncode": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"urlDecode": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultString},
		},
	}

	RegisterVirtualPackage("lib/url", pkg)
}

// initPathPackage registers the lib/path virtual package
func initPathPackage() {
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}

	listString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{stringType},
	}

	resultString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, stringType},
	}

	resultBool := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, typesystem.Bool},
	}

	pkg := &VirtualPackage{
		Name: "path",
		Symbols: map[string]typesystem.Type{
			// Parsing
			"pathJoin":  typesystem.TFunc{Params: []typesystem.Type{listString}, ReturnType: stringType},
			"pathSplit": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: listString},
			"pathDir":   typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"pathBase":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"pathExt":   typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"pathStem":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},

			// Manipulation
			"pathWithExt":  typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: stringType},
			"pathWithBase": typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: stringType},

			// Query
			"pathIsAbs": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Bool},
			"pathIsRel": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Bool},

			// Normalization
			"pathClean": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"pathAbs":   typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultString},
			"pathRel":   typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: resultString},

			// Matching
			"pathMatch": typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: resultBool},

			// Separator
			"pathSep": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: stringType},

			// Temp directory
			"pathTemp": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: stringType},

			// POSIX-style (handles dotfiles correctly)
			"pathExtPosix":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"pathStemPosix": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
			"pathIsHidden":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Bool},
		},
	}

	RegisterVirtualPackage("lib/path", pkg)
}

// initUuidPackage registers the lib/uuid virtual package
func initUuidPackage() {
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}
	uuidType := typesystem.TCon{Name: "Uuid"}
	bytesType := typesystem.TCon{Name: "Bytes"}
	intType := typesystem.Int
	boolType := typesystem.Bool

	resultUuid := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, uuidType},
	}

	pkg := &VirtualPackage{
		Name: "uuid",
		Types: map[string]typesystem.Type{
			"Uuid": uuidType,
		},
		Symbols: map[string]typesystem.Type{
			// Generation
			"uuidNew": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
			"uuidV4":  typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
			"uuidV5":  typesystem.TFunc{Params: []typesystem.Type{uuidType, stringType}, ReturnType: uuidType},
			"uuidV7":  typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
			"uuidNil": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
			"uuidMax": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},

			// Namespaces for v5
			"uuidNamespaceDNS":  typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
			"uuidNamespaceURL":  typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
			"uuidNamespaceOID":  typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},
			"uuidNamespaceX500": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: uuidType},

			// Parsing
			"uuidParse":     typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultUuid},
			"uuidFromBytes": typesystem.TFunc{Params: []typesystem.Type{bytesType}, ReturnType: resultUuid},

			// Conversion
			"uuidToString":        typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: stringType},
			"uuidToStringCompact": typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: stringType},
			"uuidToStringUrn":     typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: stringType},
			"uuidToStringBraces":  typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: stringType},
			"uuidToStringUpper":   typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: stringType},
			"uuidToBytes":         typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: bytesType},

			// Info
			"uuidVersion": typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: intType},
			"uuidIsNil":   typesystem.TFunc{Params: []typesystem.Type{uuidType}, ReturnType: boolType},
		},
	}

	RegisterVirtualPackage("lib/uuid", pkg)
}

// initLogPackage registers the lib/log virtual package
func initLogPackage() {
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}
	loggerType := typesystem.TCon{Name: "Logger"}
	nilType := typesystem.Nil
	boolType := typesystem.Bool

	// Map<String, a> for fields
	mapStringAny := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Map"},
		Args:        []typesystem.Type{stringType, typesystem.TVar{Name: "a"}},
	}

	resultNil := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, nilType},
	}

	pkg := &VirtualPackage{
		Name: "log",
		Types: map[string]typesystem.Type{
			"Logger": loggerType,
		},
		Symbols: map[string]typesystem.Type{
			// Basic logging
			"logDebug": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: nilType},
			"logInfo":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: nilType},
			"logWarn":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: nilType},
			"logError": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: nilType},
			"logFatal": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: nilType},

			// Fatal with exit
			"logFatalExit": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: nilType},

			// Configuration
			"logLevel":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: nilType},
			"logFormat": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: nilType},
			"logOutput": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultNil},
			"logColor":  typesystem.TFunc{Params: []typesystem.Type{boolType}, ReturnType: nilType},

			// Structured logging
			"logWithFields": typesystem.TFunc{Params: []typesystem.Type{stringType, stringType, mapStringAny}, ReturnType: nilType},

			// Prefixed logger
			"logWithPrefix": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: loggerType},

			// Logger methods
			"loggerDebug":      typesystem.TFunc{Params: []typesystem.Type{loggerType, stringType}, ReturnType: nilType},
			"loggerInfo":       typesystem.TFunc{Params: []typesystem.Type{loggerType, stringType}, ReturnType: nilType},
			"loggerWarn":       typesystem.TFunc{Params: []typesystem.Type{loggerType, stringType}, ReturnType: nilType},
			"loggerError":      typesystem.TFunc{Params: []typesystem.Type{loggerType, stringType}, ReturnType: nilType},
			"loggerFatal":      typesystem.TFunc{Params: []typesystem.Type{loggerType, stringType}, ReturnType: nilType},
			"loggerFatalExit":  typesystem.TFunc{Params: []typesystem.Type{loggerType, stringType}, ReturnType: nilType},
			"loggerWithFields": typesystem.TFunc{Params: []typesystem.Type{loggerType, stringType, stringType, mapStringAny}, ReturnType: nilType},
		},
	}

	RegisterVirtualPackage("lib/log", pkg)
}

// initTaskPackage registers the lib/task virtual package
func initTaskPackage() {
	T := typesystem.TVar{Name: "T"}
	U := typesystem.TVar{Name: "U"}

	taskT := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Task"},
		Args:        []typesystem.Type{T},
	}

	taskU := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Task"},
		Args:        []typesystem.Type{U},
	}

	listTaskT := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{taskT},
	}

	listT := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{T},
	}

	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}

	resultStringT := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, T},
	}

	resultStringListT := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, listT},
	}

	fnVoidT := typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: T}
	fnTU := typesystem.TFunc{Params: []typesystem.Type{T}, ReturnType: U}
	fnTTaskU := typesystem.TFunc{Params: []typesystem.Type{T}, ReturnType: taskU}
	fnStringT := typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: T}

	taskType := typesystem.TCon{Name: "Task"}

	pkg := &VirtualPackage{
		Name: "task",
		Types: map[string]typesystem.Type{
			"Task": taskType,
		},
		Symbols: map[string]typesystem.Type{
			// Creation
			"async":       typesystem.TFunc{Params: []typesystem.Type{fnVoidT}, ReturnType: taskT},
			"taskResolve": typesystem.TFunc{Params: []typesystem.Type{T}, ReturnType: taskT},
			"taskReject":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: taskT},

			// Awaiting
			"await":             typesystem.TFunc{Params: []typesystem.Type{taskT}, ReturnType: resultStringT},
			"awaitTimeout":      typesystem.TFunc{Params: []typesystem.Type{taskT, typesystem.Int}, ReturnType: resultStringT},
			"awaitAll":          typesystem.TFunc{Params: []typesystem.Type{listTaskT}, ReturnType: resultStringListT},
			"awaitAllTimeout":   typesystem.TFunc{Params: []typesystem.Type{listTaskT, typesystem.Int}, ReturnType: resultStringListT},
			"awaitAny":          typesystem.TFunc{Params: []typesystem.Type{listTaskT}, ReturnType: resultStringT},
			"awaitAnyTimeout":   typesystem.TFunc{Params: []typesystem.Type{listTaskT, typesystem.Int}, ReturnType: resultStringT},
			"awaitFirst":        typesystem.TFunc{Params: []typesystem.Type{listTaskT}, ReturnType: resultStringT},
			"awaitFirstTimeout": typesystem.TFunc{Params: []typesystem.Type{listTaskT, typesystem.Int}, ReturnType: resultStringT},

			// Control
			"taskCancel":      typesystem.TFunc{Params: []typesystem.Type{taskT}, ReturnType: typesystem.Nil},
			"taskIsDone":      typesystem.TFunc{Params: []typesystem.Type{taskT}, ReturnType: typesystem.Bool},
			"taskIsCancelled": typesystem.TFunc{Params: []typesystem.Type{taskT}, ReturnType: typesystem.Bool},

			// Pool
			"taskSetGlobalPool": typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: typesystem.Nil},
			"taskGetGlobalPool": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Int},

			// Combinators
			"taskMap":     typesystem.TFunc{Params: []typesystem.Type{taskT, fnTU}, ReturnType: taskU},
			"taskFlatMap": typesystem.TFunc{Params: []typesystem.Type{taskT, fnTTaskU}, ReturnType: taskU},
			"taskCatch":   typesystem.TFunc{Params: []typesystem.Type{taskT, fnStringT}, ReturnType: taskT},
		},
	}

	RegisterVirtualPackage("lib/task", pkg)
}

// initCsvPackage registers the lib/csv virtual package
func initCsvPackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}

	// Result<String, T>
	resultType := func(t typesystem.Type) typesystem.Type {
		return typesystem.TApp{
			Constructor: typesystem.TCon{Name: "Result"},
			Args:        []typesystem.Type{stringType, t},
		}
	}

	// Generic record type (open record)
	recordType := typesystem.TRecord{Fields: map[string]typesystem.Type{}, IsOpen: true}

	// List<Record>
	listRecordType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{recordType},
	}

	// List<List<String>>
	listStringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{stringType},
	}
	listListStringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{listStringType},
	}

	pkg := &VirtualPackage{
		Name: "csv",
		Symbols: map[string]typesystem.Type{
			"csvParse": typesystem.TFunc{
				Params:       []typesystem.Type{stringType, typesystem.Char},
				ReturnType:   resultType(listRecordType),
				DefaultCount: 1,
			},
			"csvParseRaw": typesystem.TFunc{
				Params:       []typesystem.Type{stringType, typesystem.Char},
				ReturnType:   resultType(listListStringType),
				DefaultCount: 1,
			},
			"csvRead": typesystem.TFunc{
				Params:       []typesystem.Type{stringType, typesystem.Char},
				ReturnType:   resultType(listRecordType),
				DefaultCount: 1,
			},
			"csvReadRaw": typesystem.TFunc{
				Params:       []typesystem.Type{stringType, typesystem.Char},
				ReturnType:   resultType(listListStringType),
				DefaultCount: 1,
			},
			"csvEncode": typesystem.TFunc{
				Params:       []typesystem.Type{listRecordType, typesystem.Char},
				ReturnType:   stringType,
				DefaultCount: 1,
			},
			"csvEncodeRaw": typesystem.TFunc{
				Params:       []typesystem.Type{listListStringType, typesystem.Char},
				ReturnType:   stringType,
				DefaultCount: 1,
			},
			"csvWrite": typesystem.TFunc{
				Params:       []typesystem.Type{stringType, listRecordType, typesystem.Char},
				ReturnType:   resultType(typesystem.Nil),
				DefaultCount: 1,
			},
			"csvWriteRaw": typesystem.TFunc{
				Params:       []typesystem.Type{stringType, listListStringType, typesystem.Char},
				ReturnType:   resultType(typesystem.Nil),
				DefaultCount: 1,
			},
		},
	}

	RegisterVirtualPackage("lib/csv", pkg)
}

// initFlagPackage registers the lib/flag virtual package
func initFlagPackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}
	// List<String>
	listString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{stringType},
	}

	// Generic T
	T := typesystem.TVar{Name: "T"}

	pkg := &VirtualPackage{
		Name: "flag",
		Symbols: map[string]typesystem.Type{
			"flagSet": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, T, stringType},
				ReturnType: typesystem.Nil,
			},
			"flagParse": typesystem.TFunc{
				Params:       []typesystem.Type{listString},
				ReturnType:   typesystem.Nil,
				DefaultCount: 1, // args is optional
			},
			"flagGet": typesystem.TFunc{
				Params:     []typesystem.Type{stringType},
				ReturnType: T,
			},
			"flagArgs": typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: listString,
			},
			"flagUsage": typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: typesystem.Nil,
			},
		},
	}

	RegisterVirtualPackage("lib/flag", pkg)
}
