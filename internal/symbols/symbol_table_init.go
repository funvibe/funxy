package symbols

import (
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/typesystem"
	"sync"
)

// Singleton prelude table containing all built-in symbols
var (
	preludeTable *SymbolTable
	preludeOnce  sync.Once
)

// GetPrelude returns the singleton prelude SymbolTable containing all built-in symbols.
// This table is shared across all compilation units.
func GetPrelude() *SymbolTable {
	preludeOnce.Do(func() {
		preludeTable = NewEmptySymbolTable()
		preludeTable.scopeType = ScopePrelude
		preludeTable.InitBuiltins()
	})
	return preludeTable
}

// NewSymbolTable creates a new symbol table.
// It inherits from Prelude.
func NewSymbolTable() *SymbolTable {
	st := NewEmptySymbolTable()
	st.outer = GetPrelude()
	st.scopeType = ScopeGlobal
	return st
}

// ResetPrelude resets the prelude singleton (for testing only).
func ResetPrelude() {
	preludeOnce = sync.Once{}
	preludeTable = nil
}

func (st *SymbolTable) InitBuiltins() {
	const prelude = "prelude" // Origin for built-in symbols

	// Define built-in types
	st.DefineType("Int", typesystem.TCon{Name: "Int"}, prelude)
	st.RegisterKind("Int", typesystem.Star)
	st.DefineType("Bool", typesystem.TCon{Name: "Bool"}, prelude)
	st.RegisterKind("Bool", typesystem.Star)
	st.DefineType("Float", typesystem.TCon{Name: "Float"}, prelude)
	st.RegisterKind("Float", typesystem.Star)
	st.DefineType("Char", typesystem.TCon{Name: "Char"}, prelude)
	st.RegisterKind("Char", typesystem.Star)
	st.DefineType(config.ListTypeName, typesystem.TCon{Name: config.ListTypeName}, prelude)
	st.RegisterKind(config.ListTypeName, typesystem.KArrow{Left: typesystem.Star, Right: typesystem.Star})
	// Map :: * -> * -> *
	st.DefineType(config.MapTypeName, typesystem.TCon{Name: config.MapTypeName}, prelude)
	st.RegisterKind(config.MapTypeName, typesystem.MakeArrow(typesystem.Star, typesystem.Star, typesystem.Star))
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{typesystem.TCon{Name: "Char"}},
	}
	st.DefineType("String", stringType, prelude)
	// Register as alias so matchTypeWithSubst can resolve it
	st.RegisterTypeAlias("String", stringType)
	st.RegisterKind("String", typesystem.Star)

	// Bytes and Bits are built-in types (available in prelude)
	st.DefineType("Bytes", typesystem.TCon{Name: "Bytes"}, prelude)
	st.RegisterKind("Bytes", typesystem.Star)
	st.DefineType("Bits", typesystem.TCon{Name: "Bits"}, prelude)
	st.RegisterKind("Bits", typesystem.Star)

	// Built-in ADTs for error handling
	// type Result e t = Ok t | Fail e  (like Haskell's Either e a)
	// E is error type (first), T is success type (last) - so Functor/Monad operate on T
	st.DefineType(config.ResultTypeName, typesystem.TCon{Name: config.ResultTypeName}, prelude)
	// Result :: * -> * -> *
	st.RegisterKind(config.ResultTypeName, typesystem.MakeArrow(typesystem.Star, typesystem.Star, typesystem.Star))
	st.DefineConstructor(config.OkCtorName, typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.TVar{Name: "t"}},
		ReturnType: typesystem.TApp{Constructor: typesystem.TCon{Name: config.ResultTypeName}, Args: []typesystem.Type{typesystem.TVar{Name: "e"}, typesystem.TVar{Name: "t"}}},
	}, prelude)
	st.DefineConstructor(config.FailCtorName, typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.TVar{Name: "e"}},
		ReturnType: typesystem.TApp{Constructor: typesystem.TCon{Name: config.ResultTypeName}, Args: []typesystem.Type{typesystem.TVar{Name: "e"}, typesystem.TVar{Name: "t"}}},
	}, prelude)
	st.RegisterVariant(config.ResultTypeName, config.OkCtorName)
	st.RegisterVariant(config.ResultTypeName, config.FailCtorName)

	// type Option t = Some t | None
	st.DefineType(config.OptionTypeName, typesystem.TCon{Name: config.OptionTypeName}, prelude)
	// Option :: * -> *
	st.RegisterKind(config.OptionTypeName, typesystem.KArrow{Left: typesystem.Star, Right: typesystem.Star})
	st.DefineConstructor(config.SomeCtorName, typesystem.TFunc{
		Params:     []typesystem.Type{typesystem.TVar{Name: "t"}},
		ReturnType: typesystem.TApp{Constructor: typesystem.TCon{Name: config.OptionTypeName}, Args: []typesystem.Type{typesystem.TVar{Name: "t"}}},
	}, prelude)
	st.DefineConstructor(config.NoneCtorName, typesystem.TApp{Constructor: typesystem.TCon{Name: config.OptionTypeName}, Args: []typesystem.Type{typesystem.TVar{Name: "t"}}}, prelude)
	st.RegisterVariant(config.OptionTypeName, config.SomeCtorName)
	st.RegisterVariant(config.OptionTypeName, config.NoneCtorName)

	// Reader :: * -> * -> *
	st.DefineType("Reader", typesystem.TCon{Name: "Reader"}, prelude)
	st.RegisterKind("Reader", typesystem.MakeArrow(typesystem.Star, typesystem.Star, typesystem.Star))

	// Identity :: * -> *
	st.DefineType("Identity", typesystem.TCon{Name: "Identity"}, prelude)
	st.RegisterKind("Identity", typesystem.KArrow{Left: typesystem.Star, Right: typesystem.Star})

	// State :: * -> * -> *
	st.DefineType("State", typesystem.TCon{Name: "State"}, prelude)
	st.RegisterKind("State", typesystem.MakeArrow(typesystem.Star, typesystem.Star, typesystem.Star))

	// Writer :: * -> * -> *
	st.DefineType("Writer", typesystem.TCon{Name: "Writer"}, prelude)
	st.RegisterKind("Writer", typesystem.MakeArrow(typesystem.Star, typesystem.Star, typesystem.Star))

	// Note: SqlValue, Uuid, Logger, Task, Date types are registered via virtual packages on import
	// They are NOT available without importing the corresponding lib/* package
}

// Ensure Dictionary type is available
func (s *SymbolTable) InitDictionaryType() {
	s.DefineType("Dictionary", typesystem.TCon{Name: "Dictionary"}, "prelude")
	s.RegisterKind("Dictionary", typesystem.Star)
}
