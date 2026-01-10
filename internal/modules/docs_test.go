package modules

import (
	"sort"
	"testing"
)

// TestDocsMatchVirtualPackages verifies that documentation covers all functions
// from virtual packages. This ensures we never have undocumented functions.
func TestDocsMatchVirtualPackages(t *testing.T) {
	// Initialize all packages
	InitVirtualPackages()

	// Check each virtual package
	for path, vp := range virtualPackages {
		// Skip meta-packages
		if path == "lib" {
			continue
		}

		dp := GetDocPackage(path)
		if dp == nil {
			t.Errorf("Virtual package %q has no documentation", path)
			continue
		}

		// Build set of documented functions
		docFuncs := make(map[string]bool)
		for _, f := range dp.Functions {
			docFuncs[f.Name] = true
		}

		// Check all virtual package symbols are documented
		var missing []string
		for name := range vp.Symbols {
			if !docFuncs[name] {
				missing = append(missing, name)
			}
		}

		if len(missing) > 0 {
			sort.Strings(missing)
			t.Errorf("Package %q: missing documentation for functions: %v", path, missing)
		}

		// Check for documented functions that don't exist in virtual package
		var extra []string
		for name := range docFuncs {
			if _, exists := vp.Symbols[name]; !exists {
				extra = append(extra, name)
			}
		}

		if len(extra) > 0 {
			sort.Strings(extra)
			t.Errorf("Package %q: documented functions not in virtual package: %v", path, extra)
		}
	}
}

// TestAllVirtualPackagesHaveDocs verifies every virtual package has documentation
func TestAllVirtualPackagesHaveDocs(t *testing.T) {
	InitVirtualPackages()

	for path := range virtualPackages {
		if path == "lib" {
			continue
		}

		dp := GetDocPackage(path)
		if dp == nil {
			t.Errorf("Virtual package %q has no documentation entry", path)
		}
	}
}

// TestDocsHaveDescriptions checks that documentation entries have descriptions
func TestDocsHaveDescriptions(t *testing.T) {
	InitVirtualPackages()

	for path, dp := range docPackages {
		// Skip prelude which has auto-generated docs
		if path == "prelude" {
			continue
		}

		if dp.Description == "" {
			t.Errorf("Package %q has no description", path)
		}

		// Check functions have descriptions (warning only)
		for _, f := range dp.Functions {
			if f.Description == "" {
				t.Logf("Warning: %s.%s has no description", path, f.Name)
			}
		}
	}
}

// TestLibFunctionNamesUnique verifies all function names across lib/* packages are unique.
// This ensures no naming conflicts when using `import "lib" (*)`.
func TestLibFunctionNamesUnique(t *testing.T) {
	InitVirtualPackages()

	// Map: function name -> list of packages that define it
	funcToPackages := make(map[string][]string)

	for path, vp := range virtualPackages {
		// Only check lib/* packages
		if len(path) <= 4 || path[:4] != "lib/" {
			continue
		}
		// Skip meta-package
		if path == "lib" {
			continue
		}

		for name := range vp.Symbols {
			funcToPackages[name] = append(funcToPackages[name], path)
		}
	}

	// Check for duplicates
	var conflicts []string
	for name, packages := range funcToPackages {
		if len(packages) > 1 {
			sort.Strings(packages)
			conflicts = append(conflicts, name)
		}
	}

	if len(conflicts) > 0 {
		sort.Strings(conflicts)
		for _, name := range conflicts {
			packages := funcToPackages[name]
			sort.Strings(packages)
			t.Errorf("Function %q is defined in multiple packages: %v", name, packages)
		}
	}
}
