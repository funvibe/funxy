package ext

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestInspector_RealPackage tests the inspector against a real Go package
// (github.com/google/uuid, which is already in the project's go.sum).
func TestInspector_RealPackage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	yaml := `
deps:
  - pkg: github.com/google/uuid
    version: v1.6.0
    bind:
      - func: New
        as: uuidNew
      - func: Parse
        as: uuidParse
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

	if len(result.Bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(result.Bindings))
	}

	// Check uuidNew
	newBinding := result.Bindings[0]
	if newBinding.FuncBinding == nil {
		t.Fatal("expected FuncBinding for uuidNew")
	}
	if newBinding.FuncBinding.GoName != "New" {
		t.Errorf("GoName = %q, want New", newBinding.FuncBinding.GoName)
	}
	sig := newBinding.FuncBinding.Signature
	if len(sig.Params) != 0 {
		t.Errorf("New() params = %d, want 0", len(sig.Params))
	}
	if len(sig.Results) != 1 {
		t.Errorf("New() results = %d, want 1", len(sig.Results))
	}

	// Check uuidParse
	parseBinding := result.Bindings[1]
	if parseBinding.FuncBinding == nil {
		t.Fatal("expected FuncBinding for uuidParse")
	}
	if parseBinding.FuncBinding.GoName != "Parse" {
		t.Errorf("GoName = %q, want Parse", parseBinding.FuncBinding.GoName)
	}
	parseSig := parseBinding.FuncBinding.Signature
	if len(parseSig.Params) != 1 {
		t.Errorf("Parse() params = %d, want 1", len(parseSig.Params))
	}
	if parseSig.Params[0].Type.FunxyType != "String" {
		t.Errorf("Parse param type = %q, want String", parseSig.Params[0].Type.FunxyType)
	}
	if !parseSig.HasErrorReturn {
		t.Error("Parse should have error return")
	}
}

// TestCodegen_RealPackage tests the full codegen pipeline with a real package.
func TestCodegen_RealPackage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	yaml := `
deps:
  - pkg: github.com/google/uuid
    version: v1.6.0
    bind:
      - func: New
        as: uuidNew
      - func: Parse
        as: uuidParse
        error_to_result: true
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

	codegen := NewCodeGenerator("parser")
	files, err := codegen.Generate(result)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("expected at least one generated file")
	}

	// Check that binding files were generated
	var foundBinding, foundMain bool
	for _, f := range files {
		t.Logf("Generated: %s (%d bytes)", f.Filename, len(f.Content))

		if f.Filename == "ext_uuid.go" {
			foundBinding = true
			// Verify the content contains expected function names
			if !strings.Contains(f.Content, "uuidNew") {
				t.Error("binding file should contain uuidNew")
			}
			if !strings.Contains(f.Content, "uuidParse") {
				t.Error("binding file should contain uuidParse")
			}
			if !strings.Contains(f.Content, "makeResultOk") {
				t.Error("binding file should contain makeResultOk (for error_to_result)")
			}
			if !strings.Contains(f.Content, "makeResultErr") {
				t.Error("binding file should contain makeResultErr (for error_to_result)")
			}
		}
		if f.Filename == "ext_init.go" {
			foundMain = true
			if !strings.Contains(f.Content, "RegisterExtBuiltins") {
				t.Error("ext_init.go should contain RegisterExtBuiltins")
			}
			if !strings.Contains(f.Content, `"uuid"`) {
				t.Error("ext_init.go should register uuid module")
			}
		}
	}

	if !foundBinding {
		t.Error("no ext_uuid.go binding file generated")
	}
	if !foundMain {
		t.Error("no ext_init.go generated")
	}
}

// TestCodegen_TypeBinding tests codegen for type bindings (methods).
func TestCodegen_TypeBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	yaml := `
deps:
  - pkg: github.com/google/uuid
    version: v1.6.0
    bind:
      - type: UUID
        as: uuid
        methods: [String, URN, Version]
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

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	tb := result.Bindings[0].TypeBinding
	if tb == nil {
		t.Fatal("expected TypeBinding")
	}

	if len(tb.Methods) != 3 {
		t.Errorf("expected 3 methods, got %d", len(tb.Methods))
		for _, m := range tb.Methods {
			t.Logf("  method: %s → %s", m.GoName, m.FunxyName)
		}
	}

	// Check method names
	methodNames := make(map[string]bool)
	for _, m := range tb.Methods {
		methodNames[m.GoName] = true
		t.Logf("method: %s → %s (params: %d, results: %d)",
			m.GoName, m.FunxyName,
			len(m.Signature.Params), len(m.Signature.Results))
	}

	if !methodNames["String"] {
		t.Error("missing String method")
	}
	if !methodNames["URN"] {
		t.Error("missing URN method")
	}
	if !methodNames["Version"] {
		t.Error("missing Version method")
	}

	// Generate code
	codegen := NewCodeGenerator("parser")
	files, err := codegen.Generate(result)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	for _, f := range files {
		if f.Filename == "ext_uuid.go" {
			if !strings.Contains(f.Content, "uuidString") {
				t.Error("binding should contain uuidString")
			}
			if !strings.Contains(f.Content, "uuidURN") {
				t.Error("binding should contain uuidURN")
			}
			if !strings.Contains(f.Content, "uuidVersion") {
				t.Error("binding should contain uuidVersion")
			}
		}
		t.Logf("--- %s ---\n%s", f.Filename, f.Content)
	}
}

// TestFullBuild tests the complete build pipeline.
// This requires the Go toolchain and network access.
func TestFullBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full build test in short mode")
	}

	// Check that go command is available
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
      - func: Parse
        as: uuidParse
        error_to_result: true
`
	cfg, err := ParseConfig([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	// Find Funxy source dir (should be the parent of ext/)
	funxySourceDir := filepath.Dir(mustAbs(t, "."))
	gomodPath := filepath.Join(funxySourceDir, "go.mod")
	if _, err := os.Stat(gomodPath); err != nil {
		t.Skipf("Funxy source dir not found at %s", funxySourceDir)
	}

	outputPath := filepath.Join(t.TempDir(), "funxy-ext-test")

	builder := NewBuilder(cfg, funxySourceDir, "parser", "1.25.3",
		WithOutput(outputPath),
		WithVerbose(true),
	)
	defer builder.Cleanup()

	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Verify binary was created
	info, err := os.Stat(result.BinaryPath)
	if err != nil {
		t.Fatalf("binary not found: %v", err)
	}

	t.Logf("Binary: %s (%.1f MB)", result.BinaryPath, float64(info.Size())/(1024*1024))

	// Run it — should work as a full funxy interpreter.
	// Without args, funxy shows help/usage text.
	cmd := exec.Command(result.BinaryPath, "--help")
	output, err := cmd.CombinedOutput()
	t.Logf("Help output: %s", string(output))
	if err != nil {
		// Exit code may be non-zero for --help, that's fine
		t.Logf("Exit error (may be expected for --help): %v", err)
	}
	// The binary should contain normal funxy help text
	outStr := string(output)
	if !strings.Contains(outStr, "funxy") && !strings.Contains(outStr, "Usage") {
		t.Errorf("expected funxy help output, got: %s", outStr)
	}
}

func mustAbs(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("Abs(%q): %v", path, err)
	}
	return abs
}
