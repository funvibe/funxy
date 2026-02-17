package modules

import (
	"sort"
	"sync"

	"github.com/funvibe/funxy/internal/typesystem"
)

// Common Type Constructors with defined Kinds to avoid repetition
var (
	// List :: * -> *
	ListCon = typesystem.TCon{
		Name:    "List",
		KindVal: typesystem.KArrow{Left: typesystem.Star, Right: typesystem.Star},
	}
	// Option :: * -> *
	OptionCon = typesystem.TCon{
		Name:    "Option",
		KindVal: typesystem.KArrow{Left: typesystem.Star, Right: typesystem.Star},
	}
	// Result :: * -> * -> *
	ResultCon = typesystem.TCon{
		Name:    "Result",
		KindVal: typesystem.MakeArrow(typesystem.Star, typesystem.Star, typesystem.Star),
	}
	// Map :: * -> * -> *
	MapCon = typesystem.TCon{
		Name:    "Map",
		KindVal: typesystem.MakeArrow(typesystem.Star, typesystem.Star, typesystem.Star),
	}
)

var initVirtualPackagesOnce sync.Once

// InitVirtualPackages initializes all virtual packages.
// Safe to call multiple times; initialization is performed once.
func InitVirtualPackages() {
	initVirtualPackagesOnce.Do(func() {
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
		initYamlPackage()
		initFlagPackage()
		initGrpcPackage()
		initProtoPackage()
		initTermPackage()

		// Register "lib" meta-package (import "lib" imports all lib/*)
		initLibMetaPackage()

		// Initialize documentation for all packages including prelude (builtins)
		InitDocumentation()
	})
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
