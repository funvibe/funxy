package ext

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// e2eBinaryOnce ensures we build the funxy binary only once per test run.
var (
	e2eBinaryOnce sync.Once
	e2eBinary     string
	e2eSourceDir  string
	e2eBuildErr   error
)

// buildFunxyBinary builds the funxy binary once and caches the result.
func buildFunxyBinary(t *testing.T) (binary string, sourceDir string) {
	t.Helper()
	e2eBinaryOnce.Do(func() {
		e2eSourceDir = filepath.Dir(mustAbs(t, "."))
		if _, err := os.Stat(filepath.Join(e2eSourceDir, "go.mod")); err != nil {
			e2eBuildErr = err
			return
		}
		tmpDir, err := os.MkdirTemp("", "funxy-e2e-binary-*")
		if err != nil {
			e2eBuildErr = err
			return
		}
		e2eBinary = filepath.Join(tmpDir, "funxy")
		cmd := exec.Command("go", "build", "-o", e2eBinary, ".")
		cmd.Dir = e2eSourceDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			e2eBuildErr = &buildError{output: string(output), err: err}
		}
	})
	if e2eBuildErr != nil {
		t.Skipf("cannot build funxy: %v", e2eBuildErr)
	}
	return e2eBinary, e2eSourceDir
}

type buildError struct {
	output string
	err    error
}

func (e *buildError) Error() string {
	return e.output + "\n" + e.err.Error()
}

// writeTestProject creates a temporary directory with funxy.yaml.
func writeTestProject(t *testing.T, funxyYaml string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "funxy.yaml"), []byte(funxyYaml), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// runFunxy runs the funxy binary with the given args in the given directory.
func runFunxy(t *testing.T, binary, sourceDir, workDir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "FUNXY_HOME="+sourceDir)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// uuidYaml is the standard test funxy.yaml with uuid bindings.
const uuidYaml = `deps:
  - pkg: github.com/google/uuid
    version: v1.6.0
    bind:
      - func: New
        as: uuidNew
      - func: Parse
        as: uuidParse
        error_to_result: true
`

// uuidTypeYaml adds a type binding for uuid.UUID (methods).
const uuidTypeYaml = `deps:
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
        methods: [String, Version]
`

// skipIfShortOrNoGo skips the test in short mode or if Go is not available.
func skipIfShortOrNoGo(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not found")
	}
}

// --- E2E Tests ---

// TestE2E_BuildAndRunWithExt tests the complete build-and-run flow:
// funxy.yaml + script → binary → run → verify output.
func TestE2E_BuildAndRunWithExt(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	projectDir := writeTestProject(t, uuidYaml)

	// Write script
	script := `import "ext/uuid" (uuidNew, uuidParse)

myUuid = uuidNew()
print("Generated UUID: " ++ show(myUuid))

match uuidParse("550e8400-e29b-41d4-a716-446655440000") {
  Ok(parsed) -> print("Parsed OK: " ++ show(parsed))
  Fail(msg) -> print("Parse error: " ++ show(msg))
}

match uuidParse("not-a-uuid") {
  Ok(_) -> print("Should have failed!")
  Fail(msg) -> print("Expected error: " ++ show(msg))
}
`
	if err := os.WriteFile(filepath.Join(projectDir, "app.lang"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	// Build
	appBinary := filepath.Join(projectDir, "app")
	output, err := runFunxy(t, binary, sourceDir, projectDir,
		"build", "app.lang",
		"--config", filepath.Join(projectDir, "funxy.yaml"),
		"-o", appBinary,
		"--ext-verbose",
	)
	t.Logf("Build output:\n%s", output)
	if err != nil {
		t.Fatalf("funxy build failed: %v", err)
	}

	// Verify binary
	info, err := os.Stat(appBinary)
	if err != nil {
		t.Fatalf("app binary not found: %v", err)
	}
	t.Logf("App binary: %.1f MB", float64(info.Size())/(1024*1024))

	// Run
	runCmd := exec.Command(appBinary)
	runOutput, err := runCmd.CombinedOutput()
	t.Logf("App output:\n%s", string(runOutput))

	outStr := string(runOutput)
	for _, want := range []string{"Generated UUID:", "Parsed OK:", "Expected error:"} {
		if !strings.Contains(outStr, want) {
			t.Errorf("expected %q in output", want)
		}
	}
}

// TestE2E_BuildAndRunWithTypeMethods tests type bindings with struct methods.
func TestE2E_BuildAndRunWithTypeMethods(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	projectDir := writeTestProject(t, uuidTypeYaml)

	// Script that uses both function and type bindings
	script := `import "ext/uuid" (uuidNew, uuidParse, uuidString, uuidVersion)

// Create a UUID and call methods on it
myUuid = uuidNew()
s = uuidString(myUuid)
print("UUID string: " ++ s)

v = uuidVersion(myUuid)
print("UUID version: " ++ show(v))

// Parse and call String method
match uuidParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8") {
  Ok(parsed) -> {
    print("Parsed string: " ++ uuidString(parsed))
  }
  Fail(msg) -> print("Error: " ++ show(msg))
}
`
	if err := os.WriteFile(filepath.Join(projectDir, "app.lang"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	// Build
	appBinary := filepath.Join(projectDir, "app")
	output, err := runFunxy(t, binary, sourceDir, projectDir,
		"build", "app.lang",
		"--config", filepath.Join(projectDir, "funxy.yaml"),
		"-o", appBinary,
		"--ext-verbose",
	)
	t.Logf("Build output:\n%s", output)
	if err != nil {
		t.Fatalf("funxy build failed: %v", err)
	}

	// Run
	runCmd := exec.Command(appBinary)
	runOutput, err := runCmd.CombinedOutput()
	t.Logf("App output:\n%s", string(runOutput))

	outStr := string(runOutput)
	for _, want := range []string{"UUID string:", "UUID version:", "Parsed string: 6ba7b810-9dad-11d1-80b4-00c04fd430c8"} {
		if !strings.Contains(outStr, want) {
			t.Errorf("expected %q in output", want)
		}
	}
}

// TestE2E_ExtCheck tests the `funxy ext check` CLI command.
func TestE2E_ExtCheck(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	projectDir := writeTestProject(t, uuidYaml)

	output, err := runFunxy(t, binary, sourceDir, projectDir, "ext", "check")
	t.Logf("ext check output:\n%s", output)
	if err != nil {
		t.Fatalf("funxy ext check failed: %v", err)
	}

	for _, want := range []string{
		"Config:",
		"Dependencies: 1",
		"github.com/google/uuid",
		"Resolved 2 bindings",
		"All checks passed",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in ext check output", want)
		}
	}
}

// TestE2E_ExtList tests the `funxy ext list` CLI command.
func TestE2E_ExtList(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	projectDir := writeTestProject(t, uuidYaml)

	output, err := runFunxy(t, binary, sourceDir, projectDir, "ext", "list")
	t.Logf("ext list output:\n%s", output)
	if err != nil {
		t.Fatalf("funxy ext list failed: %v", err)
	}

	for _, want := range []string{
		"ext/uuid",
		"github.com/google/uuid",
		"uuidNew",
		"uuidParse",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in ext list output", want)
		}
	}
}

// TestE2E_ExtStubs tests the `funxy ext stubs` CLI command.
func TestE2E_ExtStubs(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	projectDir := writeTestProject(t, uuidYaml)

	output, err := runFunxy(t, binary, sourceDir, projectDir, "ext", "stubs")
	t.Logf("ext stubs output:\n%s", output)
	if err != nil {
		t.Fatalf("funxy ext stubs failed: %v", err)
	}

	if !strings.Contains(output, "Generated stubs") {
		t.Error("expected 'Generated stubs' in output")
	}
	if !strings.Contains(output, "uuid.d.lang") {
		t.Error("expected 'uuid.d.lang' in output")
	}

	// Verify stub file was created
	stubPath := filepath.Join(projectDir, ".funxy", "ext", "uuid.d.lang")
	content, err := os.ReadFile(stubPath)
	if err != nil {
		t.Fatalf("stub file not found: %v", err)
	}

	stubStr := string(content)
	t.Logf("Stub content:\n%s", stubStr)

	for _, want := range []string{
		"fun uuidNew()",
		"fun uuidParse(",
		"Result<String,",
	} {
		if !strings.Contains(stubStr, want) {
			t.Errorf("expected %q in stub file", want)
		}
	}

	// Verify .gitignore was created
	gitignorePath := filepath.Join(projectDir, ".funxy", ".gitignore")
	if _, err := os.Stat(gitignorePath); err != nil {
		t.Errorf("expected .gitignore at %s", gitignorePath)
	}
}

// --- ext build tests ---

// TestE2E_ExtBuild tests the `funxy ext build` command:
// creates a custom Funxy interpreter with ext bindings compiled in (no .lang file).
func TestE2E_ExtBuild(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	projectDir := writeTestProject(t, uuidYaml)

	// Build custom interpreter
	extBinary := filepath.Join(projectDir, "funxy-custom")
	output, err := runFunxy(t, binary, sourceDir, projectDir,
		"ext", "build", "-o", extBinary, "--verbose",
	)
	t.Logf("ext build output:\n%s", output)
	if err != nil {
		t.Fatalf("funxy ext build failed: %v\n%s", err, output)
	}

	// Verify binary was created
	info, err := os.Stat(extBinary)
	if err != nil {
		t.Fatalf("custom binary not found: %v", err)
	}
	t.Logf("Custom binary: %.1f MB", float64(info.Size())/(1024*1024))

	// Verify the binary is executable (should not crash)
	cmd := exec.Command(extBinary, "--version")
	verOutput, _ := cmd.CombinedOutput()
	t.Logf("Version output: %s", string(verOutput))
}

// TestE2E_ExtBuild_RunScript verifies the ext build binary can directly run
// .lang scripts — it's a full funxy interpreter, not a stub.
func TestE2E_ExtBuild_RunScript(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	projectDir := writeTestProject(t, uuidYaml)

	// Build custom interpreter
	extBinary := filepath.Join(projectDir, "funxy-custom")
	output, err := runFunxy(t, binary, sourceDir, projectDir,
		"ext", "build", "-o", extBinary, "--verbose",
	)
	t.Logf("ext build output:\n%s", output)
	if err != nil {
		t.Fatalf("funxy ext build failed: %v\n%s", err, output)
	}

	// Write a script that uses ext/uuid
	script := `import "ext/uuid" (uuidNew)
myUuid = uuidNew()
print("OK: " ++ show(myUuid))
`
	scriptPath := filepath.Join(projectDir, "test.lang")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run the script directly with the custom interpreter
	runCmd := exec.Command(extBinary, scriptPath)
	runCmd.Dir = projectDir
	runOutput, err := runCmd.CombinedOutput()
	t.Logf("Script output:\n%s", string(runOutput))

	if err != nil {
		t.Fatalf("custom binary failed to run script: %v\n%s", err, string(runOutput))
	}

	if !strings.Contains(string(runOutput), "OK:") {
		t.Errorf("expected 'OK:' in output, got: %s", string(runOutput))
	}
}

// TestE2E_ExtBuild_EvalMode verifies the ext build binary works in -e eval mode.
func TestE2E_ExtBuild_EvalMode(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	projectDir := writeTestProject(t, uuidYaml)

	// Build custom interpreter
	extBinary := filepath.Join(projectDir, "funxy-custom")
	output, err := runFunxy(t, binary, sourceDir, projectDir,
		"ext", "build", "-o", extBinary, "--verbose",
	)
	t.Logf("ext build output:\n%s", output)
	if err != nil {
		t.Fatalf("funxy ext build failed: %v\n%s", err, output)
	}

	// Eval mode: -pe prints the result of the expression
	runCmd := exec.Command(extBinary, "-pe", `1 + 2`)
	runCmd.Dir = projectDir
	runOutput, err := runCmd.CombinedOutput()
	t.Logf("Eval output:\n%s", string(runOutput))

	if err != nil {
		t.Fatalf("eval mode failed: %v\n%s", err, string(runOutput))
	}

	outStr := strings.TrimSpace(string(runOutput))
	if outStr != "3" {
		t.Errorf("expected '3', got %q", outStr)
	}
}

// TestE2E_ExtBuild_Help verifies the ext build binary shows proper help output.
func TestE2E_ExtBuild_Help(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	projectDir := writeTestProject(t, uuidYaml)

	extBinary := filepath.Join(projectDir, "funxy-custom")
	output, err := runFunxy(t, binary, sourceDir, projectDir,
		"ext", "build", "-o", extBinary, "--verbose",
	)
	t.Logf("ext build output:\n%s", output)
	if err != nil {
		t.Fatalf("funxy ext build failed: %v\n%s", err, output)
	}

	// --help should show full funxy help
	runCmd := exec.Command(extBinary, "--help")
	runOutput, _ := runCmd.CombinedOutput()
	t.Logf("Help output:\n%s", string(runOutput))

	outStr := string(runOutput)
	if !strings.Contains(outStr, "funxy") {
		t.Errorf("expected 'funxy' in help output, got: %s", outStr)
	}
}

// TestE2E_ExtBuild_AsHost verifies the custom binary can serve as --host for funxy build.
// This is the primary use case: funxy ext build → funxy build --host <ext-binary> app.lang
func TestE2E_ExtBuild_AsHost(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	projectDir := writeTestProject(t, uuidYaml)

	// Step 1: Build custom ext host binary
	extBinary := filepath.Join(projectDir, "funxy-custom")
	output, err := runFunxy(t, binary, sourceDir, projectDir,
		"ext", "build", "-o", extBinary, "--verbose",
	)
	t.Logf("ext build output:\n%s", output)
	if err != nil {
		t.Fatalf("funxy ext build failed: %v\n%s", err, output)
	}

	// Step 2: Write a script that uses ext/uuid
	script := `import "ext/uuid" (uuidNew)
print("BUILT: " ++ show(uuidNew()))
`
	scriptPath := filepath.Join(projectDir, "app.lang")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	// Step 3: Use the custom binary as --host for funxy build.
	// Pass --config so the builder registers ext/* virtual packages for the analyzer.
	appBinary := filepath.Join(projectDir, "app-binary")
	buildOutput, err := runFunxy(t, binary, sourceDir, projectDir,
		"build", "app.lang",
		"--host", extBinary,
		"--config", filepath.Join(projectDir, "funxy.yaml"),
		"-o", appBinary,
	)
	t.Logf("Build with host output:\n%s", buildOutput)
	if err != nil {
		t.Fatalf("funxy build with --host failed: %v\n%s", err, buildOutput)
	}

	// Step 4: Run the resulting self-contained binary
	runCmd := exec.Command(appBinary)
	runOutput, err := runCmd.CombinedOutput()
	t.Logf("App output:\n%s", string(runOutput))

	if err != nil {
		t.Fatalf("built binary failed: %v\n%s", err, string(runOutput))
	}

	if !strings.Contains(string(runOutput), "BUILT:") {
		t.Errorf("expected 'BUILT:' in output, got: %s", string(runOutput))
	}
}

// TestE2E_ExtBuild_AsHost_WithExtImport verifies the full build pipeline:
// ext build → funxy build --host → self-contained binary with ext/uuid.
// This tests that the ext module functions are actually available at runtime.
func TestE2E_ExtBuild_AsHost_WithExtImport(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	projectDir := writeTestProject(t, uuidTypeYaml)

	// Build ext host
	extBinary := filepath.Join(projectDir, "funxy-custom")
	output, err := runFunxy(t, binary, sourceDir, projectDir,
		"ext", "build", "-o", extBinary, "--verbose",
	)
	t.Logf("ext build output:\n%s", output)
	if err != nil {
		t.Fatalf("funxy ext build failed: %v\n%s", err, output)
	}

	// Script uses multiple ext functions
	script := `import "ext/uuid" (uuidNew, uuidString, uuidVersion)
myUuid = uuidNew()
print("UUID: " ++ uuidString(myUuid))
print("Version: " ++ show(uuidVersion(myUuid)))
`
	if err := os.WriteFile(filepath.Join(projectDir, "app.lang"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	// Build self-contained binary using ext host
	appBinary := filepath.Join(projectDir, "app")
	buildOutput, err := runFunxy(t, binary, sourceDir, projectDir,
		"build", "app.lang",
		"--host", extBinary,
		"--config", filepath.Join(projectDir, "funxy.yaml"),
		"-o", appBinary,
	)
	t.Logf("Build output:\n%s", buildOutput)
	if err != nil {
		t.Fatalf("funxy build --host failed: %v\n%s", err, buildOutput)
	}

	// Run
	runCmd := exec.Command(appBinary)
	runOutput, err := runCmd.CombinedOutput()
	t.Logf("App output:\n%s", string(runOutput))

	outStr := string(runOutput)
	if !strings.Contains(outStr, "UUID:") {
		t.Errorf("expected 'UUID:' in output, got: %s", outStr)
	}
	if !strings.Contains(outStr, "Version:") {
		t.Errorf("expected 'Version:' in output, got: %s", outStr)
	}
}

// TestE2E_ExtBuild_BuildBundle verifies the ext build binary can itself
// run `build` to produce a self-contained bundled binary.
// This tests: ./myfunxy build app.lang -o app (no --host, no funxy build).
func TestE2E_ExtBuild_BuildBundle(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	projectDir := writeTestProject(t, uuidYaml)

	// Step 1: Build custom ext interpreter
	extBinary := filepath.Join(projectDir, "funxy-custom")
	output, err := runFunxy(t, binary, sourceDir, projectDir,
		"ext", "build", "-o", extBinary, "--verbose",
	)
	t.Logf("ext build output:\n%s", output)
	if err != nil {
		t.Fatalf("funxy ext build failed: %v\n%s", err, output)
	}

	// Step 2: Write a script that uses ext/uuid
	script := `import "ext/uuid" (uuidNew)
print("BUNDLED: " ++ show(uuidNew()))
`
	scriptPath := filepath.Join(projectDir, "app.lang")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	// Step 3: Use the ext binary itself to build a bundle (no --host, no funxy)
	appBinary := filepath.Join(projectDir, "app-bundled")
	buildCmd := exec.Command(extBinary, "build", "app.lang", "-o", appBinary)
	buildCmd.Dir = projectDir
	buildOutput, err := buildCmd.CombinedOutput()
	t.Logf("ext binary 'build' output:\n%s", string(buildOutput))
	if err != nil {
		t.Fatalf("ext binary 'build' failed: %v\n%s", err, string(buildOutput))
	}

	// Step 4: Run the bundled binary — it should work standalone
	runCmd := exec.Command(appBinary)
	runOutput, err := runCmd.CombinedOutput()
	t.Logf("Bundled app output:\n%s", string(runOutput))

	if err != nil {
		t.Fatalf("bundled binary failed: %v\n%s", err, string(runOutput))
	}

	if !strings.Contains(string(runOutput), "BUNDLED:") {
		t.Errorf("expected 'BUNDLED:' in output, got: %s", string(runOutput))
	}
}

// TestE2E_ExtBuild_BuildBundle_NoYaml verifies that the ext build binary
// can bundle scripts even when there's no funxy.yaml in the script directory.
// The ext modules are already compiled in, so no config is needed.
func TestE2E_ExtBuild_BuildBundle_NoYaml(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	projectDir := writeTestProject(t, uuidYaml)

	// Step 1: Build custom ext interpreter
	extBinary := filepath.Join(projectDir, "funxy-custom")
	output, err := runFunxy(t, binary, sourceDir, projectDir,
		"ext", "build", "-o", extBinary, "--verbose",
	)
	t.Logf("ext build output:\n%s", output)
	if err != nil {
		t.Fatalf("funxy ext build failed: %v\n%s", err, output)
	}

	// Step 2: Write a script in a SEPARATE directory (no funxy.yaml there)
	cleanDir := t.TempDir()
	script := `import "ext/uuid" (uuidNew)
print("CLEAN: " ++ show(uuidNew()))
`
	scriptPath := filepath.Join(cleanDir, "app.lang")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	// Step 3: Build from the clean directory — no funxy.yaml present
	appBinary := filepath.Join(cleanDir, "app-clean")
	buildCmd := exec.Command(extBinary, "build", "app.lang", "-o", appBinary)
	buildCmd.Dir = cleanDir
	buildOutput, err := buildCmd.CombinedOutput()
	t.Logf("ext binary 'build' (no yaml) output:\n%s", string(buildOutput))
	if err != nil {
		t.Fatalf("ext binary 'build' without yaml failed: %v\n%s", err, string(buildOutput))
	}

	// Step 4: Run the bundled binary
	runCmd := exec.Command(appBinary)
	runOutput, err := runCmd.CombinedOutput()
	t.Logf("Bundled app output:\n%s", string(runOutput))

	if err != nil {
		t.Fatalf("bundled binary failed: %v\n%s", err, string(runOutput))
	}

	if !strings.Contains(string(runOutput), "CLEAN:") {
		t.Errorf("expected 'CLEAN:' in output, got: %s", string(runOutput))
	}
}

// TestE2E_ExtBuild_NoYaml verifies that ext build fails gracefully when
// no funxy.yaml exists.
func TestE2E_ExtBuild_NoYaml(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	// Empty dir — no funxy.yaml
	emptyDir := t.TempDir()

	output, err := runFunxy(t, binary, sourceDir, emptyDir, "ext", "build")
	t.Logf("ext build (no yaml) output:\n%s", output)

	if err == nil {
		t.Fatal("expected error when funxy.yaml is missing")
	}

	if !strings.Contains(output, "funxy.yaml not found") {
		t.Errorf("expected 'funxy.yaml not found' in error, got: %s", output)
	}
}

// TestE2E_ExtBuild_DefaultOutput verifies that ext build uses "funxy-ext" as
// the default output name when -o is not specified.
func TestE2E_ExtBuild_DefaultOutput(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	projectDir := writeTestProject(t, uuidYaml)

	output, err := runFunxy(t, binary, sourceDir, projectDir,
		"ext", "build", "--verbose",
	)
	t.Logf("ext build (default output) output:\n%s", output)
	if err != nil {
		t.Fatalf("funxy ext build failed: %v\n%s", err, output)
	}

	// Should have created "funxy-ext" in the working directory
	defaultPath := filepath.Join(projectDir, "funxy-ext")
	info, err := os.Stat(defaultPath)
	if err != nil {
		t.Fatalf("default binary not found at %s: %v", defaultPath, err)
	}
	t.Logf("Default binary: %.1f MB", float64(info.Size())/(1024*1024))

	// Cleanup
	os.Remove(defaultPath)
}

// --- Local dependency tests ---

// writeLocalGoPackage creates a local Go package in the given directory.
// Returns the module path and directory.
func writeLocalGoPackage(t *testing.T, parentDir, modName string) string {
	t.Helper()
	pkgDir := filepath.Join(parentDir, "golib")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// go.mod
	gomod := fmt.Sprintf("module %s\n\ngo 1.21\n", modName)
	if err := os.WriteFile(filepath.Join(pkgDir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}

	// helpers.go
	goSrc := `package golib

import "fmt"

func Greet(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}

func Add(a, b int) int {
	return a + b
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "helpers.go"), []byte(goSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	return pkgDir
}

// TestE2E_LocalDep_ExtBuild tests ext build with a local Go package.
func TestE2E_LocalDep_ExtBuild(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	projectDir := t.TempDir()

	// Create local Go package (module path needs dot in first element for Go toolchain)
	localModName := "local.dev/golib"
	writeLocalGoPackage(t, projectDir, localModName)

	// funxy.yaml with local dep
	yamlContent := fmt.Sprintf(`deps:
  - pkg: %s
    local: ./golib
    bind:
      - func: Greet
        as: greet
      - func: Add
        as: add
`, localModName)
	if err := os.WriteFile(filepath.Join(projectDir, "funxy.yaml"), []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Build custom interpreter
	extBinary := filepath.Join(projectDir, "funxy-local")
	output, err := runFunxy(t, binary, sourceDir, projectDir,
		"ext", "build", "-o", extBinary, "--verbose",
	)
	t.Logf("ext build output:\n%s", output)
	if err != nil {
		t.Fatalf("funxy ext build failed: %v\n%s", err, output)
	}

	// Write script that uses local functions
	script := `import "ext/golib" (greet, add)
print(greet("World"))
print("Sum: " ++ show(add(2, 3)))
`
	scriptPath := filepath.Join(projectDir, "test.lang")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run
	runCmd := exec.Command(extBinary, scriptPath)
	runCmd.Dir = projectDir
	runOutput, err := runCmd.CombinedOutput()
	t.Logf("Script output:\n%s", string(runOutput))

	if err != nil {
		t.Fatalf("script failed: %v\n%s", err, string(runOutput))
	}

	outStr := string(runOutput)
	if !strings.Contains(outStr, "Hello, World!") {
		t.Errorf("expected 'Hello, World!' in output, got: %s", outStr)
	}
	if !strings.Contains(outStr, "Sum: 5") {
		t.Errorf("expected 'Sum: 5' in output, got: %s", outStr)
	}
}

// TestE2E_LocalDep_Validation tests that invalid local paths are caught.
func TestE2E_LocalDep_Validation(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	projectDir := t.TempDir()

	// funxy.yaml pointing to non-existent local path
	yamlContent := `deps:
  - pkg: myproject/missing
    local: ./does_not_exist
    bind:
      - func: Foo
        as: foo
`
	if err := os.WriteFile(filepath.Join(projectDir, "funxy.yaml"), []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	output, err := runFunxy(t, binary, sourceDir, projectDir, "ext", "check")
	t.Logf("ext check output:\n%s", output)

	if err == nil {
		t.Fatal("expected error for non-existent local path")
	}

	if !strings.Contains(output, "not found") {
		t.Errorf("expected 'not found' in error, got: %s", output)
	}
}

// TestE2E_LocalDep_MixedRemoteAndLocal tests mixing remote and local deps.
func TestE2E_LocalDep_MixedRemoteAndLocal(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	projectDir := t.TempDir()

	// Create local Go package
	localModName := "local.dev/golib"
	writeLocalGoPackage(t, projectDir, localModName)

	// funxy.yaml with both remote (uuid) and local dep
	yamlContent := fmt.Sprintf(`deps:
  - pkg: github.com/google/uuid
    version: v1.6.0
    bind:
      - func: New
        as: uuidNew
  - pkg: %s
    local: ./golib
    bind:
      - func: Greet
        as: greet
`, localModName)
	if err := os.WriteFile(filepath.Join(projectDir, "funxy.yaml"), []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Build
	extBinary := filepath.Join(projectDir, "funxy-mixed")
	output, err := runFunxy(t, binary, sourceDir, projectDir,
		"ext", "build", "-o", extBinary, "--verbose",
	)
	t.Logf("ext build output:\n%s", output)
	if err != nil {
		t.Fatalf("funxy ext build failed: %v\n%s", err, output)
	}

	// Script uses both
	script := `import "ext/uuid" (uuidNew)
import "ext/golib" (greet)
print(greet("Funxy"))
print("UUID: " ++ show(uuidNew()))
`
	scriptPath := filepath.Join(projectDir, "test.lang")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	runCmd := exec.Command(extBinary, scriptPath)
	runCmd.Dir = projectDir
	runOutput, err := runCmd.CombinedOutput()
	t.Logf("Script output:\n%s", string(runOutput))

	if err != nil {
		t.Fatalf("script failed: %v\n%s", err, string(runOutput))
	}

	outStr := string(runOutput)
	if !strings.Contains(outStr, "Hello, Funxy!") {
		t.Errorf("expected 'Hello, Funxy!' in output, got: %s", outStr)
	}
	if !strings.Contains(outStr, "UUID:") {
		t.Errorf("expected 'UUID:' in output, got: %s", outStr)
	}
}

func TestE2E_HyphenatedPackageAndPointers(t *testing.T) {
	skipIfShortOrNoGo(t)
	binary, sourceDir := buildFunxyBinary(t)

	projectDir := t.TempDir()

	// Create local package
	writeHyphenatedPackage(t, projectDir)

	// funxy.yaml
	yamlContent := `deps:
  - pkg: example.com/my-lib
    local: ./my-lib
    bind:
      - type: Options
        as: options
      - func: NewOptions
        as: newOptions
      - func: TakePointer
        as: takePointer
      - func: TakeAny
        as: takeAny
      - func: TakeMyInt
        as: takeMyInt
      - func: TakeDuration
        as: takeDuration
`
	if err := os.WriteFile(filepath.Join(projectDir, "funxy.yaml"), []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Build
	extBinary := filepath.Join(projectDir, "funxy-hyphen")
	output, err := runFunxy(t, binary, sourceDir, projectDir,
		"ext", "build", "-o", extBinary, "--verbose",
	)
	t.Logf("ext build output:\n%s", output)
	if err != nil {
		t.Fatalf("funxy ext build failed: %v\n%s", err, output)
	}

	// Script
	scriptWithImport := `import "ext/my-lib" (newOptions, takePointer, takeAny, takeMyInt, takeDuration)
opts = newOptions("Tested")
res = takePointer(opts)
print(res)
print(takeAny("hello"))
print(takeAny(123))
print(takeMyInt(42))
print(takeDuration(100))
`
	scriptPath := filepath.Join(projectDir, "test.lang")
	if err := os.WriteFile(scriptPath, []byte(scriptWithImport), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run
	runCmd := exec.Command(extBinary, scriptPath)
	runCmd.Dir = projectDir
	runOutput, err := runCmd.CombinedOutput()
	t.Logf("Script output:\n%s", string(runOutput))

	if err != nil {
		t.Fatalf("script failed: %v\n%s", err, string(runOutput))
	}

	outputStr := string(runOutput)
	if !strings.Contains(outputStr, "Options: Tested") {
		t.Errorf("expected 'Options: Tested' in output, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Any: hello") {
		t.Errorf("expected 'Any: hello' in output, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Any: 123") {
		t.Errorf("expected 'Any: 123' in output, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "MyInt: 42") {
		t.Errorf("expected 'MyInt: 42' in output, got: %s", outputStr)
	}
	// 100ns because Duration is int64 nanoseconds
	if !strings.Contains(outputStr, "Duration: 100ns") {
		t.Errorf("expected 'Duration: 100ns' in output, got: %s", outputStr)
	}
}

func writeHyphenatedPackage(t *testing.T, parentDir string) string {
	t.Helper()
	pkgDir := filepath.Join(parentDir, "my-lib")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// go.mod
	gomod := "module example.com/my-lib\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(pkgDir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}

	// lib.go
	goSrc := `package mylib

import (
	"fmt"
	"time"
)

type Options struct {
	Name string
}

type MyInt int

func NewOptions(name string) *Options {
	return &Options{Name: name}
}

func TakePointer(opts *Options) string {
	if opts == nil {
		return "nil"
	}
	return "Options: " + opts.Name
}

func TakeAny(v interface{}) string {
	return fmt.Sprintf("Any: %v", v)
}

func TakeMyInt(v MyInt) string {
	return fmt.Sprintf("MyInt: %d", v)
}

func TakeDuration(d time.Duration) string {
	return fmt.Sprintf("Duration: %v", d)
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "lib.go"), []byte(goSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	return pkgDir
}
