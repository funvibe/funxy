package ext

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// Config negative-path tests
// =============================================================================

func TestParseConfig_InvalidYAMLSyntax(t *testing.T) {
	// Broken YAML: tab characters in wrong place, unclosed bracket
	inputs := []struct {
		name string
		yaml string
	}{
		{"tab indentation", "deps:\n\t- pkg: foo"},
		{"unclosed bracket", "deps:\n  - pkg: [foo"},
		{"bare colon in value", "deps:\n  - pkg: foo:bar:baz"},
		{"completely invalid", "{{{{"},
		{"empty input", ""},
	}
	for _, tc := range inputs {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseConfig([]byte(tc.yaml), "test.yaml")
			if err == nil {
				t.Error("expected error for invalid YAML syntax")
			}
		})
	}
}

func TestLoadConfig_NonExistentFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/funxy.yaml")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "reading config") {
		t.Errorf("error should mention 'reading config', got: %v", err)
	}
}

func TestLoadConfig_DirectoryInsteadOfFile(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error when path is a directory")
	}
}

func TestLoadConfig_ValidYAMLButInvalidConfig(t *testing.T) {
	// Valid YAML syntax but fails validation
	path := filepath.Join(t.TempDir(), "funxy.yaml")
	if err := os.WriteFile(path, []byte("deps: []"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for empty deps")
	}
	if !strings.Contains(err.Error(), "no deps defined") {
		t.Errorf("error should mention 'no deps defined', got: %v", err)
	}
}

func TestLoadConfig_PermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root, permission test not applicable")
	}
	path := filepath.Join(t.TempDir(), "funxy.yaml")
	if err := os.WriteFile(path, []byte("deps: []"), 0o000); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for permission denied")
	}
}

// =============================================================================
// Inspector negative-path tests
// These require the Go toolchain and may download packages (network).
// =============================================================================

func TestInspector_NonExistentPackage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not found")
	}

	// Use .invalid TLD (IANA-reserved, DNS fails instantly — no git auth prompts)
	yaml := `
deps:
  - pkg: nonexistent.invalid/fake-package
    version: v0.0.0
    bind:
      - func: DoSomething
        as: doSomething
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	inspector := NewInspector("1.25.3")
	defer inspector.Cleanup()

	_, err = inspector.Inspect(cfg)
	if err == nil {
		t.Fatal("expected error for non-existent package")
	}

	t.Logf("Expected error: %v", err)
}

func TestInspector_TypeNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not found")
	}

	// Real package, non-existent type
	yaml := `
deps:
  - pkg: github.com/google/uuid
    version: v1.6.0
    bind:
      - type: NonExistentType
        as: phantom
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	inspector := NewInspector("1.25.3")
	defer inspector.Cleanup()

	_, err = inspector.Inspect(cfg)
	if err == nil {
		t.Fatal("expected error for non-existent type")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
	if !strings.Contains(errStr, "NonExistentType") {
		t.Errorf("error should mention the type name 'NonExistentType', got: %v", err)
	}
	t.Logf("Expected error: %v", err)
}

func TestInspector_FunctionNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not found")
	}

	// Real package, non-existent function
	yaml := `
deps:
  - pkg: github.com/google/uuid
    version: v1.6.0
    bind:
      - func: NonExistentFunc
        as: phantom
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	inspector := NewInspector("1.25.3")
	defer inspector.Cleanup()

	_, err = inspector.Inspect(cfg)
	if err == nil {
		t.Fatal("expected error for non-existent function")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
	if !strings.Contains(errStr, "NonExistentFunc") {
		t.Errorf("error should mention 'NonExistentFunc', got: %v", err)
	}
	t.Logf("Expected error: %v", err)
}

func TestInspector_TypeBoundAsFunc(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not found")
	}

	// uuid.UUID is a type, not a function — binding it as func should fail
	yaml := `
deps:
  - pkg: github.com/google/uuid
    version: v1.6.0
    bind:
      - func: UUID
        as: makeUuid
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	inspector := NewInspector("1.25.3")
	defer inspector.Cleanup()

	_, err = inspector.Inspect(cfg)
	if err == nil {
		t.Fatal("expected error when binding a type as func")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "not a function") {
		t.Errorf("error should mention 'not a function', got: %v", err)
	}
	t.Logf("Expected error: %v", err)
}

func TestInspector_FuncBoundAsType(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not found")
	}

	// uuid.New is a function, not a type — binding it as type should fail
	yaml := `
deps:
  - pkg: github.com/google/uuid
    version: v1.6.0
    bind:
      - type: New
        as: newThing
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	inspector := NewInspector("1.25.3")
	defer inspector.Cleanup()

	_, err = inspector.Inspect(cfg)
	if err == nil {
		t.Fatal("expected error when binding a function as type")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "not a type") {
		t.Errorf("error should mention 'not a type', got: %v", err)
	}
	t.Logf("Expected error: %v", err)
}

func TestInspector_MethodWhitelistMismatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not found")
	}

	// Whitelisted method that doesn't exist — this doesn't error (silently ignored)
	// but verifies that the binding resolves with fewer methods than requested.
	yaml := `
deps:
  - pkg: github.com/google/uuid
    version: v1.6.0
    bind:
      - type: UUID
        as: uuid
        methods: [String, NonExistentMethod, AnotherFake]
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	inspector := NewInspector("1.25.3")
	defer inspector.Cleanup()

	result, err := inspector.Inspect(cfg)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	// Should succeed but only find the one real method
	tb := result.Bindings[0].TypeBinding
	if tb == nil {
		t.Fatal("expected TypeBinding")
	}
	if len(tb.Methods) != 1 {
		t.Errorf("expected 1 method (String only), got %d", len(tb.Methods))
		for _, m := range tb.Methods {
			t.Logf("  method: %s", m.GoName)
		}
	}
}

// =============================================================================
// Cache negative-path tests
// =============================================================================

func TestCache_LookupMiss(t *testing.T) {
	cache := NewCache(t.TempDir())
	result := cache.LookupHostBinary([]byte("some config"), "modPath", "", "")
	if result != "" {
		t.Errorf("expected empty string for cache miss, got %q", result)
	}
}

func TestCache_StoreNonExistentBinary(t *testing.T) {
	cache := NewCache(t.TempDir())
	_, err := cache.StoreHostBinary("/nonexistent/binary", []byte("cfg"), "modPath", "", "")
	if err == nil {
		t.Fatal("expected error when storing non-existent binary")
	}
	if !strings.Contains(err.Error(), "reading binary") {
		t.Errorf("error should mention 'reading binary', got: %v", err)
	}
}

func TestCache_StoreToReadOnlyDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root, permission test not applicable")
	}

	// Create a project dir where .funxy can't be created
	projectDir := t.TempDir()
	funxyDir := filepath.Join(projectDir, ".funxy")
	if err := os.MkdirAll(funxyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Make .funxy read-only so ext-cache can't be created
	if err := os.Chmod(funxyDir, 0o444); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(funxyDir, 0o755) // Restore for cleanup

	// Create a real file to store
	src := filepath.Join(t.TempDir(), "binary")
	if err := os.WriteFile(src, []byte("fake binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	cache := NewCache(projectDir)
	_, err := cache.StoreHostBinary(src, []byte("cfg"), "modPath", "", "")
	if err == nil {
		t.Fatal("expected error when cache dir is not writable")
	}
}

func TestConfigFingerprint_NonExistentFile(t *testing.T) {
	_, err := ConfigFingerprint("/nonexistent/funxy.yaml")
	if err == nil {
		t.Fatal("expected error for non-existent config file")
	}
}

func TestConfigFingerprint_Normalization(t *testing.T) {
	// Same content with different trailing whitespace should produce same fingerprint
	dir := t.TempDir()

	path1 := filepath.Join(dir, "a.yaml")
	path2 := filepath.Join(dir, "b.yaml")

	content1 := "deps:\n  - pkg: foo  \n    bind:\n      - type: Bar\n        as: bar\n\n"
	content2 := "deps:\n  - pkg: foo\n    bind:\n      - type: Bar\n        as: bar"

	os.WriteFile(path1, []byte(content1), 0o644)
	os.WriteFile(path2, []byte(content2), 0o644)

	fp1, err := ConfigFingerprint(path1)
	if err != nil {
		t.Fatal(err)
	}
	fp2, err := ConfigFingerprint(path2)
	if err != nil {
		t.Fatal(err)
	}

	if string(fp1) != string(fp2) {
		t.Errorf("fingerprints should match after normalization:\n  fp1: %q\n  fp2: %q", fp1, fp2)
	}
}

// =============================================================================
// Builder negative-path tests
// =============================================================================

func TestBuilder_BuildWithNonExistentPackage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not found")
	}

	yaml := `
deps:
  - pkg: nonexistent.invalid/nope
    version: v0.0.0
    bind:
      - func: Foo
        as: foo
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	funxySourceDir := filepath.Dir(mustAbs(t, "."))
	builder := NewBuilder(cfg, funxySourceDir, "parser", "1.25.3",
		WithOutput(filepath.Join(t.TempDir(), "should-not-exist")),
	)
	defer builder.Cleanup()

	_, err = builder.Build()
	if err == nil {
		t.Fatal("expected error building with non-existent package")
	}
	t.Logf("Expected error: %v", err)
}

func TestBuilder_BuildWithBadFunxySourceDir(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not found")
	}

	yaml := `
deps:
  - pkg: github.com/google/uuid
    version: v1.6.0
    bind:
      - func: New
        as: uuidNew
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	// Non-existent funxy source dir → updateGoMod will fail
	builder := NewBuilder(cfg, "/nonexistent/funxy/source", "parser", "1.25.3",
		WithOutput(filepath.Join(t.TempDir(), "should-not-exist")),
	)
	defer builder.Cleanup()

	_, err = builder.Build()
	if err == nil {
		t.Fatal("expected error with bad funxy source dir")
	}
	t.Logf("Expected error: %v", err)
}

// =============================================================================
// E2E negative-path tests (via CLI)
// =============================================================================

func TestE2E_ExtCheckNonExistentPackage(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	badYaml := `deps:
  - pkg: nonexistent.invalid/no-such-pkg
    version: v0.0.0
    bind:
      - func: Nope
        as: nope
`
	projectDir := writeTestProject(t, badYaml)

	output, err := runFunxy(t, binary, sourceDir, projectDir, "ext", "check")
	t.Logf("ext check output:\n%s", output)

	// Should fail — non-zero exit
	if err == nil {
		t.Fatal("expected ext check to fail for non-existent package")
	}

	// Error message should be informative
	lowerOut := strings.ToLower(output)
	if !strings.Contains(lowerOut, "nonexistent.invalid") && !strings.Contains(lowerOut, "error") &&
		!strings.Contains(lowerOut, "fail") {
		t.Errorf("output should mention the package name or error, got:\n%s", output)
	}
}

func TestE2E_ExtCheckTypeNotFound(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	badYaml := `deps:
  - pkg: github.com/google/uuid
    version: v1.6.0
    bind:
      - type: DoesNotExist
        as: phantom
`
	projectDir := writeTestProject(t, badYaml)

	output, err := runFunxy(t, binary, sourceDir, projectDir, "ext", "check")
	t.Logf("ext check output:\n%s", output)

	if err == nil {
		t.Fatal("expected ext check to fail for non-existent type")
	}

	if !strings.Contains(output, "DoesNotExist") && !strings.Contains(output, "not found") {
		t.Errorf("output should mention 'DoesNotExist' or 'not found', got:\n%s", output)
	}
}

func TestE2E_ExtCheckFuncNotFound(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	badYaml := `deps:
  - pkg: github.com/google/uuid
    version: v1.6.0
    bind:
      - func: DoesNotExist
        as: phantom
`
	projectDir := writeTestProject(t, badYaml)

	output, err := runFunxy(t, binary, sourceDir, projectDir, "ext", "check")
	t.Logf("ext check output:\n%s", output)

	if err == nil {
		t.Fatal("expected ext check to fail for non-existent function")
	}

	if !strings.Contains(output, "DoesNotExist") && !strings.Contains(output, "not found") {
		t.Errorf("output should mention 'DoesNotExist' or 'not found', got:\n%s", output)
	}
}

func TestE2E_ExtCheckNoConfig(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	// Empty directory — no funxy.yaml
	emptyDir := t.TempDir()

	output, err := runFunxy(t, binary, sourceDir, emptyDir, "ext", "check")
	t.Logf("ext check output:\n%s", output)

	if err == nil {
		t.Fatal("expected ext check to fail when no funxy.yaml")
	}

	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "funxy.yaml") &&
		!strings.Contains(lowerOutput, "config") &&
		!strings.Contains(lowerOutput, "not found") {
		t.Errorf("output should mention missing config, got:\n%s", output)
	}
}

func TestE2E_BuildWithInvalidConfig(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	// Write syntactically invalid YAML
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "funxy.yaml"), []byte("{{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := `print("hello")`
	if err := os.WriteFile(filepath.Join(dir, "app.lang"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	output, err := runFunxy(t, binary, sourceDir, dir,
		"build", "app.lang",
		"--config", filepath.Join(dir, "funxy.yaml"),
		"-o", filepath.Join(dir, "app"),
	)
	t.Logf("Build output:\n%s", output)

	if err == nil {
		t.Fatal("expected build to fail with invalid config")
	}
}

func TestE2E_BuildFuncBoundAsType(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	// uuid.New is a function, binding it as a type should produce a clear error
	badYaml := `deps:
  - pkg: github.com/google/uuid
    version: v1.6.0
    bind:
      - type: New
        as: wrong
`
	projectDir := writeTestProject(t, badYaml)
	script := `print("hello")`
	if err := os.WriteFile(filepath.Join(projectDir, "app.lang"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	output, err := runFunxy(t, binary, sourceDir, projectDir,
		"build", "app.lang",
		"--config", filepath.Join(projectDir, "funxy.yaml"),
		"-o", filepath.Join(projectDir, "app"),
	)
	t.Logf("Build output:\n%s", output)

	if err == nil {
		t.Fatal("expected build to fail when func bound as type")
	}

	if !strings.Contains(output, "not a type") {
		t.Errorf("output should mention 'not a type', got:\n%s", output)
	}
}
