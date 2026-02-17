package cli

import (
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/modules"
	"strings"
	"testing"
)

// TestAddAutoImports_Basic verifies that addAutoImports generates correct
// import statements for lib/* functions used in source code.
func TestAddAutoImports_Basic(t *testing.T) {
	modules.InitVirtualPackages()
	evaluator.ClearExtBuiltins()

	code := `show(uuidNew())`
	result := addAutoImports(code)

	if !strings.Contains(result, `import "lib/uuid"`) {
		t.Errorf("expected import of lib/uuid, got:\n%s", result)
	}
	if !strings.Contains(result, "uuidNew") {
		t.Errorf("expected uuidNew in import, got:\n%s", result)
	}
}

// TestAddAutoImports_ExtShadowing verifies that lib/* functions take priority
// over ext/* functions when both provide a function with the same name.
//
// Scenario: lib/uuid defines uuidNew. We register ext/uuid with uuidNew too.
// addAutoImports should resolve uuidNew to lib/uuid (not ext/uuid).
func TestAddAutoImports_ExtShadowing(t *testing.T) {
	modules.InitVirtualPackages()

	// Register ext/uuid builtins with overlapping names
	evaluator.ClearExtBuiltins()
	evaluator.RegisterExtBuiltins("uuid", map[string]evaluator.Object{
		"uuidNew":   &evaluator.Builtin{}, // Same name as lib/uuid.uuidNew
		"uuidParse": &evaluator.Builtin{}, // Same name as lib/uuid.uuidParse
	})
	defer evaluator.ClearExtBuiltins()

	code := `show(uuidNew())`
	result := addAutoImports(code)

	t.Logf("Auto-import result:\n%s", result)

	// Should import from lib/uuid, NOT ext/uuid
	if !strings.Contains(result, `import "lib/uuid"`) {
		t.Errorf("expected import from lib/uuid (priority over ext/uuid), got:\n%s", result)
	}
	if strings.Contains(result, `import "ext/uuid"`) {
		t.Errorf("ext/uuid should NOT be auto-imported when lib/uuid has the same function:\n%s", result)
	}
}

// TestAddAutoImports_ExtOnlyFunction verifies that ext/* functions ARE
// auto-imported when there is no lib/* equivalent.
func TestAddAutoImports_ExtOnlyFunction(t *testing.T) {
	modules.InitVirtualPackages()

	// Register ext/redis with unique function names (no lib/* overlap)
	evaluator.ClearExtBuiltins()
	evaluator.RegisterExtBuiltins("redis", map[string]evaluator.Object{
		"redisConnect": &evaluator.Builtin{},
		"redisGet":     &evaluator.Builtin{},
		"redisSet":     &evaluator.Builtin{},
	})
	defer evaluator.ClearExtBuiltins()

	code := `conn = redisConnect("localhost:6379")`
	result := addAutoImports(code)

	t.Logf("Auto-import result:\n%s", result)

	// Should import from ext/redis (no lib/* conflict)
	if !strings.Contains(result, `import "ext/redis"`) {
		t.Errorf("expected import from ext/redis, got:\n%s", result)
	}
	if !strings.Contains(result, "redisConnect") {
		t.Errorf("expected redisConnect in import, got:\n%s", result)
	}
}

// TestAddAutoImports_MixedLibAndExt verifies that when some functions come
// from lib/* and others from ext/*, both are imported correctly.
func TestAddAutoImports_MixedLibAndExt(t *testing.T) {
	modules.InitVirtualPackages()

	evaluator.ClearExtBuiltins()
	evaluator.RegisterExtBuiltins("redis", map[string]evaluator.Object{
		"redisGet": &evaluator.Builtin{},
	})
	defer evaluator.ClearExtBuiltins()

	// Uses uuidNew (lib/uuid) and redisGet (ext/redis)
	code := `id = uuidNew()
val = redisGet("key")
`
	result := addAutoImports(code)

	t.Logf("Auto-import result:\n%s", result)

	if !strings.Contains(result, `import "lib/uuid"`) {
		t.Errorf("expected lib/uuid import, got:\n%s", result)
	}
	if !strings.Contains(result, `import "ext/redis"`) {
		t.Errorf("expected ext/redis import, got:\n%s", result)
	}
}

// TestAddAutoImports_NoExtRegistered verifies that addAutoImports works
// normally when no ext modules are registered (standard funxy mode).
func TestAddAutoImports_NoExtRegistered(t *testing.T) {
	modules.InitVirtualPackages()
	evaluator.ClearExtBuiltins()

	code := `x = stringToUpper("hello")`
	result := addAutoImports(code)

	t.Logf("Auto-import result:\n%s", result)

	if !strings.Contains(result, `import "lib/string"`) {
		t.Errorf("expected lib/string import, got:\n%s", result)
	}
}

// TestAddAutoImports_PartialShadowing verifies correct behavior when ext/*
// shadows SOME lib/* functions but not all.
//
// lib/uuid has: uuidNew, uuidParse, uuidV4, uuidVersion, etc.
// ext/uuid has: uuidNew, uuidParse (overlap), plus extOnlyFunc (unique)
//
// Expected: uuidNew → lib/uuid, uuidParse → lib/uuid, extOnlyFunc → ext/uuid
func TestAddAutoImports_PartialShadowing(t *testing.T) {
	modules.InitVirtualPackages()

	evaluator.ClearExtBuiltins()
	evaluator.RegisterExtBuiltins("uuid", map[string]evaluator.Object{
		"uuidNew":     &evaluator.Builtin{}, // Shadows lib/uuid
		"uuidParse":   &evaluator.Builtin{}, // Shadows lib/uuid
		"extOnlyFunc": &evaluator.Builtin{}, // Unique to ext
	})
	defer evaluator.ClearExtBuiltins()

	code := `a = uuidNew()
b = extOnlyFunc()
`
	result := addAutoImports(code)

	t.Logf("Auto-import result:\n%s", result)

	// uuidNew should resolve to lib/uuid (priority)
	if !strings.Contains(result, `import "lib/uuid"`) {
		t.Errorf("expected lib/uuid import for uuidNew, got:\n%s", result)
	}

	// extOnlyFunc should resolve to ext/uuid (no lib conflict)
	if !strings.Contains(result, `import "ext/uuid"`) {
		t.Errorf("expected ext/uuid import for extOnlyFunc, got:\n%s", result)
	}
}
