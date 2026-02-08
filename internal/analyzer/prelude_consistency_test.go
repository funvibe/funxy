package analyzer

import (
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/symbols"
	"testing"
)

// TestPreludeHelpBuiltinsRegistered ensures that all documented prelude functions
// are actually registered in the prelude symbol table (or as trait methods).
func TestPreludeHelpBuiltinsRegistered(t *testing.T) {
	symbols.ResetPrelude()
	ResetBuiltins()

	RegisterBuiltins(nil)
	prelude := symbols.GetPrelude()

	for _, fn := range config.BuiltinFunctions {
		if _, ok := prelude.Find(fn.Name); ok {
			continue
		}
		if _, ok := prelude.GetTraitForMethod(fn.Name); ok {
			continue
		}
		t.Errorf("prelude function %q is documented but not registered in prelude", fn.Name)
	}
}
