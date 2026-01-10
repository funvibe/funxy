package analyzer

import (
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/symbols"
	"strings"
	"testing"
)

func TestBuiltinTraitsMatchConfig(t *testing.T) {
	// Reset prelude and builtins for test isolation
	symbols.ResetPrelude()
	ResetBuiltins()

	// Create a new symbol table (this also initializes prelude)
	// and register analyzer builtins to prelude
	userTable := symbols.NewSymbolTable()
	RegisterBuiltins(userTable)

	// Use prelude for checking (where builtins are registered)
	table := symbols.GetPrelude()

	for _, trait := range config.BuiltinTraits {
		// Check trait existence
		if !table.IsDefined(trait.Name) {
			t.Errorf("Trait %s defined in config but not in symbol table", trait.Name)
			continue
		}

		// Check type parameters
		params, ok := table.GetTraitTypeParams(trait.Name)
		if !ok {
			t.Errorf("Trait %s has no type parameters in symbol table", trait.Name)
			continue
		}
		if len(params) != len(trait.TypeParams) {
			t.Errorf("Trait %s: config params %v, symbol table params %v", trait.Name, trait.TypeParams, params)
		}
		for i, p := range params {
			// Compare case-insensitively since we lowercase builtins internally
			if i < len(trait.TypeParams) && p != trait.TypeParams[i] && p != strings.ToLower(trait.TypeParams[i]) {
				t.Errorf("Trait %s: param %d mismatch: config %s, symbol table %s", trait.Name, i, trait.TypeParams[i], p)
			}
		}

		// Check methods
		for _, method := range trait.Methods {
			// If method is just a name, check if it's defined in the table
			// Note: Trait methods are registered in the symbol table as functions
			// But we can also check via GetTraitForMethod

			// Some methods might be operators in config like "(+)" but here listed as "+" ??
			// Config has Operators []string separately. Methods []string are named methods.

			traitName, ok := table.GetTraitForMethod(method)
			if !ok {
				t.Errorf("Trait method %s (of %s) not found in symbol table", method, trait.Name)
			} else if traitName != trait.Name {
				t.Errorf("Method %s should belong to %s, but found in %s", method, trait.Name, traitName)
			}
		}

		// Check operators
		for _, op := range trait.Operators {
			traitName, ok := table.GetTraitForOperator(op)
			if !ok {
				t.Errorf("Operator %s (of %s) not found in symbol table", op, trait.Name)
			} else if traitName != trait.Name {
				t.Errorf("Operator %s should belong to %s, but found in %s", op, trait.Name, traitName)
			}
		}
	}
}
