package evaluator

import (
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/modules"
	"strings"
	"testing"
)

// TestPreludeDocsConsistency verifies that all built-in functions implemented in the evaluator
// are documented in config.BuiltinFunctions. This ensures they appear in -help.
func TestPreludeDocsConsistency(t *testing.T) {
	// 1. Get all evaluator builtins (prelude)
	for name := range Builtins {
		found := false
		for _, fn := range config.BuiltinFunctions {
			if fn.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Builtin function %q is implemented in evaluator (evaluator.Builtins) but missing in config.BuiltinFunctions (so it won't show in -help)", name)
		}
	}

	// Also check OptionBuiltins and ResultBuiltins which are implicitly prelude?
	// If they are available without import, they should be documented in prelude.
	checkMapAgainstConfig(t, "Option", OptionBuiltins())
	checkMapAgainstConfig(t, "Result", ResultBuiltins())
}

func checkMapAgainstConfig(t *testing.T, groupName string, builtins map[string]*Builtin) {
	for name := range builtins {
		found := false
		// Check config.BuiltinFunctions
		for _, fn := range config.BuiltinFunctions {
			if fn.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Builtin function %q (from %s) is implemented in evaluator but missing in config.BuiltinFunctions", name, groupName)
		}
	}
}

// TestVirtualModulesConsistency verifies that virtual package implementations match their documentation/definition.
func TestVirtualModulesConsistency(t *testing.T) {
	modules.InitVirtualPackages()

	pkgNames := []string{
		"lib/list", "lib/math", "lib/string", "lib/flag", "lib/csv",
		"lib/task", "lib/log", "lib/uuid", "lib/path", "lib/url",
		"lib/sql", "lib/ws", "lib/date", "lib/rand", "lib/test",
		"lib/http", "lib/regex", "lib/crypto", "lib/json", "lib/char",
		"lib/bignum", "lib/tuple", "lib/sys", "lib/io", "lib/bytes",
		"lib/bits", "lib/map",
	}

	for _, pkgPath := range pkgNames {
		vp := modules.GetVirtualPackage(pkgPath)
		if vp == nil {
			t.Errorf("Virtual package %q has implementation but no VirtualPackage definition", pkgPath)
			continue
		}

		// Use GetVirtualModuleBuiltins to ensure TypeInfo is populated
		shortName := strings.TrimPrefix(pkgPath, "lib/")
		impls := GetVirtualModuleBuiltins(shortName)
		if impls == nil {
			t.Errorf("Virtual package %q has no implementation (GetVirtualModuleBuiltins returned nil)", pkgPath)
			continue
		}

		// 1. Check that every implemented builtin is defined in the virtual package
		for name := range impls {
			if _, ok := vp.Symbols[name]; !ok {
				t.Errorf("Function %q implemented in %s but not defined in virtual package symbols", name, pkgPath)
			}
		}

		// 2. Check that every symbol in virtual package has an implementation
		for name := range vp.Symbols {
			if _, ok := impls[name]; !ok {
				t.Errorf("Function %q defined in %s symbols but missing implementation", name, pkgPath)
			}

			// Check type consistency: implementation should have TypeInfo
			if builtin, ok := impls[name]; ok {
				if builtin.TypeInfo == nil {
					t.Errorf("Builtin %q in %s has no TypeInfo", name, pkgPath)
				}
			}
		}
	}
}
