package tests

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/funvibe/funxy/internal/config"
)

var useTreeWalk = flag.Bool("tree", false, "run tests with tree-walk backend")

// TestFunctional runs .lang files through the compiled binary
// and compares output with .want files.
// This tests the actual binary - what users see.
func TestFunctional(t *testing.T) {
	// Get project root (parent of tests/)
	projectRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	binaryPath := filepath.Join(projectRoot, "funxy-test-binary")
	defer os.Remove(binaryPath)

	// Always build fresh binary
	t.Log("Building fresh binary...")
	args := []string{"build"}
	if *useTreeWalk {
		args = append(args, "-ldflags", "-X main.BackendType=tree")
	}
	args = append(args, "-o", binaryPath, "./cmd/funxy")

	cmd := exec.Command("go", args...)
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\n%s", err, output)
	}

	// Find all source files with .want files
	var testFiles []string
	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Check if it's a source file
		for _, ext := range config.SourceFileExtensions {
			if strings.HasSuffix(path, ext) {
				// Check if .want file exists
				wantFile := strings.TrimSuffix(path, ext) + ".want"
				if _, err := os.Stat(wantFile); err == nil {
					testFiles = append(testFiles, path)
				}
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk directory: %v", err)
	}

	if len(testFiles) == 0 {
		t.Skip("No test files with .want found")
	}

	// Tests with unstable output (timestamps, ANSI colors, etc.)
	skipTests := map[string]bool{
		"lib_log": true, // Has timestamps and ANSI color codes
	}

	for _, testFile := range testFiles {
		testFile := testFile
		testName := strings.TrimSuffix(filepath.Base(testFile), filepath.Ext(testFile))

		if skipTests[testName] {
			continue
		}

		t.Run(testName, func(t *testing.T) {
			// Get absolute path for the test file
			absPath, err := filepath.Abs(testFile)
			if err != nil {
				t.Fatalf("Failed to get absolute path: %v", err)
			}

			// Read expected output
			ext := filepath.Ext(testFile)
			wantFile := strings.TrimSuffix(testFile, ext) + ".want"
			wantBytes, err := os.ReadFile(wantFile)
			if err != nil {
				t.Fatalf("Failed to read .want file: %v", err)
			}
			want := strings.TrimSpace(string(wantBytes))

			// Run binary from project root so that imports like "kit/..." work
			cmd := exec.Command(binaryPath, absPath)
			cmd.Dir = projectRoot
			// Set test mode environment variable so the binary knows it's running in test mode
			cmd.Env = append(os.Environ(), "FUNXY_TEST_MODE=1")
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			_ = cmd.Run()

			// Combine stdout and stderr - no normalization, exact output
			var got string
			stdoutStr := strings.TrimSpace(stdout.String())
			stderrStr := strings.TrimSpace(stderr.String())

			// Normalize paths in stderr (errors usually contain full paths now)
			// Replace /abs/path/to/project with nothing or relative path
			if stderrStr != "" {
				// Normalize paths to be relative to project root
				stderrStr = strings.ReplaceAll(stderrStr, projectRoot+"/", "")

				// Regex to remove filename prefix: "- <filename>: (<optional phase> )error" -> "- <optional phase> error"
				// We look for "- ", then text ending in ": ", then optional "[...]" block, then "error"
				re := regexp.MustCompile(`(- )(.*?: )(\[.*?\] )?(error)`)
				stderrStr = re.ReplaceAllString(stderrStr, "$1$3$4")
			}

			// Combine: stdout first, then stderr
			if stdoutStr != "" && stderrStr != "" {
				got = stdoutStr + "\n" + stderrStr
			} else if stdoutStr != "" {
				got = stdoutStr
			} else {
				got = stderrStr
			}

			// Normalize line endings and trim spaces
			got = strings.TrimSpace(strings.ReplaceAll(got, "\r\n", "\n"))
			want = strings.TrimSpace(strings.ReplaceAll(want, "\r\n", "\n"))

			if got != want {
				t.Errorf("Output mismatch:\n--- want ---\n%s\n--- got ---\n%s", want, got)
			}
		})
	}
}
