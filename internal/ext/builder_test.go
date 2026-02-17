package ext

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateGoMod_WithVersion(t *testing.T) {
	// Create a temporary workspace
	tmpDir, err := os.MkdirTemp("", "builder_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a dummy go.mod
	goModPath := filepath.Join(tmpDir, "go.mod")
	initialGoMod := "module testbuild\n\ngo 1.25\n\nrequire (\n\tsome/other/module v1.0.0\n)\n"
	if err := os.WriteFile(goModPath, []byte(initialGoMod), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Setup Builder with a versioned module path
	builder := &Builder{
		workDir:         tmpDir,
		funxyModulePath: "github.com/funvibe/funxy",
		funxyVersion:    "dev",
		funxySourceDir:  "", // Simulate remote build (no local source)
	}

	// Run updateGoMod
	if err := builder.updateGoMod(); err != nil {
		t.Fatalf("updateGoMod failed: %v", err)
	}

	// Read back the file
	contentBytes, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("Failed to read updated go.mod: %v", err)
	}
	content := string(contentBytes)

	// Verify require directive
	expectedRequire := "require (\n\tgithub.com/funvibe/funxy dev"
	if !strings.Contains(content, expectedRequire) {
		t.Errorf("Expected go.mod to contain:\n%s\nBut got:\n%s", expectedRequire, content)
	}

	// Verify no replace directive
	if strings.Contains(content, "replace github.com/funvibe/funxy") {
		t.Errorf("Expected go.mod NOT to contain replace directive, but it did:\n%s", content)
	}
}

func TestUpdateGoMod_LocalSource(t *testing.T) {
	// Create a temporary workspace
	tmpDir, err := os.MkdirTemp("", "builder_test_local_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a dummy go.mod
	goModPath := filepath.Join(tmpDir, "go.mod")
	initialGoMod := "module testbuild\n\ngo 1.25\n\nrequire ()\n"
	if err := os.WriteFile(goModPath, []byte(initialGoMod), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create a dummy local source dir
	localSourceDir := filepath.Join(tmpDir, "localsource")
	if err := os.Mkdir(localSourceDir, 0755); err != nil {
		t.Fatalf("Failed to create local source dir: %v", err)
	}

	// Create a dummy go.mod in local source
	if err := os.WriteFile(filepath.Join(localSourceDir, "go.mod"), []byte("module github.com/funvibe/funxy\n"), 0644); err != nil {
		t.Fatalf("Failed to write local go.mod: %v", err)
	}

	// Setup Builder with local source
	builder := &Builder{
		workDir:         tmpDir,
		funxyModulePath: "github.com/funvibe/funxy",
		funxyVersion:    "v0.0.0",
		funxySourceDir:  localSourceDir,
	}

	// Run updateGoMod
	if err := builder.updateGoMod(); err != nil {
		t.Fatalf("updateGoMod failed: %v", err)
	}

	// Read back the file
	contentBytes, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("Failed to read updated go.mod: %v", err)
	}
	content := string(contentBytes)

	// Verify require directive (should default to v0.0.0 since no @version provided)
	expectedRequire := "require (\n\tgithub.com/funvibe/funxy v0.0.0"
	if !strings.Contains(content, expectedRequire) {
		t.Errorf("Expected go.mod to contain:\n%s\nBut got:\n%s", expectedRequire, content)
	}

	// Verify replace directive IS present
	expectedReplace := fmt.Sprintf("replace github.com/funvibe/funxy => %s", localSourceDir)
	if !strings.Contains(content, expectedReplace) {
		t.Errorf("Expected go.mod to contain replace directive:\n%s\nBut got:\n%s", expectedReplace, content)
	}
}
