package analyzer

import (
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

// ResolverWrapper wraps SymbolTable and InferenceContext to implement typesystem.Resolver and VariableGenerator.
type ResolverWrapper struct {
	Table *symbols.SymbolTable
	Ctx   *InferenceContext
}

// ResolveTypeAlias delegates to SymbolTable
func (w *ResolverWrapper) ResolveTypeAlias(t typesystem.Type) typesystem.Type {
	if w.Table == nil {
		return t
	}
	return w.Table.ResolveTypeAlias(t)
}

// ResolveTCon delegates to SymbolTable
func (w *ResolverWrapper) ResolveTCon(name string) (typesystem.TCon, bool) {
	if w.Table == nil {
		return typesystem.TCon{}, false
	}
	return w.Table.ResolveTCon(name)
}

// IsStrictMode delegates to SymbolTable
func (w *ResolverWrapper) IsStrictMode() bool {
	if w.Table == nil {
		return false
	}
	return w.Table.IsStrictMode()
}

// FreshTVar delegates to InferenceContext
func (w *ResolverWrapper) FreshTVar() typesystem.TVar {
	if w.Ctx == nil {
		// Fallback if context is missing (should not happen in main inference)
		return typesystem.TVar{Name: "$fresh_fallback"}
	}
	return w.Ctx.FreshVar()
}
