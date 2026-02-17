package ext

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateStubs(t *testing.T) {
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

	projectDir := t.TempDir()

	if err := GenerateStubs(cfg, result, projectDir); err != nil {
		t.Fatalf("GenerateStubs: %v", err)
	}

	// Check that stub file was created
	stubPath := filepath.Join(projectDir, ".funxy", "ext", "uuid.d.lang")
	data, err := os.ReadFile(stubPath)
	if err != nil {
		t.Fatalf("reading stub: %v", err)
	}

	content := string(data)
	t.Logf("--- uuid.d.lang ---\n%s", content)

	// Verify header
	if !strings.Contains(content, "ext/uuid") {
		t.Error("stub should mention ext/uuid")
	}
	if !strings.Contains(content, "github.com/google/uuid") {
		t.Error("stub should mention Go package")
	}

	// Verify function declarations
	if !strings.Contains(content, "fun uuidNew()") {
		t.Error("stub should contain uuidNew declaration")
	}
	if !strings.Contains(content, "fun uuidParse(") {
		t.Error("stub should contain uuidParse declaration")
	}
	if !strings.Contains(content, "Result<String,") {
		t.Error("stub should contain Result type for error_to_result")
	}

	// Verify method declarations
	if !strings.Contains(content, "fun uuidString(self: HostObject)") {
		t.Error("stub should contain uuidString method")
	}
	if !strings.Contains(content, "fun uuidURN(self: HostObject)") {
		t.Error("stub should contain uuidURN method")
	}
	if !strings.Contains(content, "fun uuidVersion(self: HostObject)") {
		t.Error("stub should contain uuidVersion method")
	}

	// Verify package hint comment
	if !strings.Contains(content, "package <name> (*)") {
		t.Error("stub should contain package hint comment")
	}

	// Verify .gitignore was created
	gitignorePath := filepath.Join(projectDir, ".funxy", ".gitignore")
	if _, err := os.Stat(gitignorePath); err != nil {
		t.Error("expected .gitignore in .funxy/")
	}
}
