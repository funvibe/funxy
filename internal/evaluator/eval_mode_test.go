package evaluator

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestEvalMode tests the -e flag functionality (expression execution mode).
// This covers -e, -p, -l flags, stdin injection, auto-import, and |>> operator.
func TestEvalMode(t *testing.T) {
	// Get project root (parent of tests/)
	projectRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	binaryPath := filepath.Join(projectRoot, "funxy-eval-test-binary")
	defer os.Remove(binaryPath)

	// Build fresh binary
	t.Log("Building fresh binary for eval tests...")
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/funxy")
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\n%s", err, output)
	}

	tests := []struct {
		name   string
		args   []string
		stdin  string
		want   string
		errMsg string // if we expect error output instead
	}{
		// ==========================================
		// Basic -e expression evaluation
		// ==========================================
		{
			name: "basic expression",
			args: []string{"-e", "print(1 + 2)"},
			want: "3",
		},
		{
			name: "string expression",
			args: []string{"-e", `print("hello")`},
			want: "hello",
		},
		{
			name: "lambda in expression",
			args: []string{"-e", `print((\x -> x * 2)(5))`},
			want: "10",
		},

		// ==========================================
		// -p flag (auto-print)
		// ==========================================
		{
			name: "auto-print number",
			args: []string{"-p", "-e", "1 + 2"},
			want: "3",
		},
		{
			name: "auto-print string",
			args: []string{"-p", "-e", `"hello"`},
			want: "hello",
		},
		{
			name: "auto-print list",
			args: []string{"-p", "-e", "[1, 2, 3]"},
			want: "[1, 2, 3]",
		},

		// ==========================================
		// Combined flags (-pe, -le, -lpe, etc.)
		// ==========================================
		{
			name: "combined -pe flag",
			args: []string{"-pe", "1 + 2 * 3"},
			want: "7",
		},
		{
			name: "combined -pe with string",
			args: []string{"-pe", `"hello world"`},
			want: "hello world",
		},
		{
			name: "combined -pe with auto-import",
			args: []string{"-pe", `stringToUpper("hello")`},
			want: "HELLO",
		},
		{
			name:  "combined -pe with stdin",
			args:  []string{"-pe", `stdin |> stringTrim`},
			stdin: "  trimmed  \n",
			want:  "trimmed",
		},
		{
			name:  "combined -pe with pipe unwrap",
			args:  []string{"-pe", `stdin |>> jsonDecode |> \x -> x.name`},
			stdin: `{"name":"Alice"}`,
			want:  "Alice",
		},
		{
			name: "combined -ep flag (reverse order)",
			args: []string{"-ep", "10 + 20"},
			want: "30",
		},
		{
			name:  "combined -le flag (line mode without auto-print)",
			args:  []string{"-le", `print(stringToUpper(stdin))`},
			stdin: "hello\nworld\n",
			want:  "HELLO\nWORLD",
		},
		{
			name:  "combined -lpe flag (line mode with auto-print)",
			args:  []string{"-lpe", `stringToUpper(stdin)`},
			stdin: "hello\nworld\n",
			want:  "HELLO\nWORLD",
		},
		{
			name:  "combined -ple flag (different order)",
			args:  []string{"-ple", `stringToUpper(stdin)`},
			stdin: "foo\nbar\n",
			want:  "FOO\nBAR",
		},
		{
			name:  "combined -elp flag (yet another order)",
			args:  []string{"-elp", `stringToUpper(stdin)`},
			stdin: "abc\ndef\n",
			want:  "ABC\nDEF",
		},

		// ==========================================
		// stdin injection
		// ==========================================
		{
			name:  "stdin available as variable",
			args:  []string{"-e", "print(stdin)"},
			stdin: "hello world",
			want:  "hello world",
		},
		{
			name:  "stdin with pipe operator",
			args:  []string{"-p", "-e", `stdin |> stringTrim`},
			stdin: "  trimmed  \n",
			want:  "trimmed",
		},

		// ==========================================
		// Auto-import
		// ==========================================
		{
			name: "auto-import stringToUpper",
			args: []string{"-p", "-e", `stringToUpper("hello")`},
			want: "HELLO",
		},
		{
			name: "auto-import jsonEncode",
			args: []string{"-p", "-e", `jsonEncode(42)`},
			want: "42",
		},
		{
			name: "auto-import multiple modules",
			args: []string{"-e", `print(jsonEncode(stringToUpper("hello")))`},
			want: `"HELLO"`,
		},

		// ==========================================
		// -l flag (line mode)
		// ==========================================
		{
			name:  "line mode processes each line",
			args:  []string{"-l", "-p", "-e", `stringToUpper(stdin)`},
			stdin: "hello\nworld\n",
			want:  "HELLO\nWORLD",
		},
		{
			name:  "line mode with pipe",
			args:  []string{"-l", "-p", "-e", `stdin |> stringTrim |> stringToUpper`},
			stdin: "  foo  \n  bar  \n",
			want:  "FOO\nBAR",
		},

		// ==========================================
		// |>> operator in -e mode
		// ==========================================
		{
			name:  "pipe unwrap with jsonDecode",
			args:  []string{"-e", `stdin |>> jsonDecode |> \x -> x.name |> print`},
			stdin: `{"name":"Alice","active":true}`,
			want:  "Alice",
		},
		{
			name:  "pipe unwrap with jsonDecode and filter",
			args:  []string{"-p", "-e", `stdin |>> jsonDecode |> \x -> x.name`},
			stdin: `{"name":"Bob","age":30}`,
			want:  "Bob",
		},

		// ==========================================
		// Complex pipelines
		// ==========================================
		{
			name: "pipe chain with math",
			args: []string{"-p", "-e", `range(1, 6) |> map(\x -> x * x)`},
			want: "[1, 4, 9, 16, 25]",
		},
		{
			name:  "stdin to stringLines",
			args:  []string{"-p", "-e", `stdin |> stringLines |> len`},
			stdin: "line1\nline2\nline3",
			want:  "3",
		},
		{
			name:  "full pipeline: decode, access field, print",
			args:  []string{"-e", `stdin |>> jsonDecode |> \x -> x.items |> len |> print`},
			stdin: `{"items":[1,2,3]}`,
			want:  "3",
		},

		// ==========================================
		// Section 3.3: |>> error message is descriptive
		// ==========================================
		{
			name:   "|>> with Fail shows error message in stderr",
			args:   []string{"-e", `Fail("detailed error") |>> \x -> x`},
			errMsg: "detailed error",
		},

		// ==========================================
		// Section 6: -e error handling
		// ==========================================
		{
			name:   "-e without expression",
			args:   []string{"-e"},
			errMsg: "expression",
		},
		{
			name:   "-e with syntax error",
			args:   []string{"-e", "1 + + 2"},
			errMsg: "error",
		},
		{
			name:   "-pe with runtime error (division by zero)",
			args:   []string{"-pe", "1 / 0"},
			errMsg: "division",
		},
		{
			name: "-e with empty string",
			args: []string{"-e", ""},
			want: "", // empty string yields empty output (no error)
		},
		{
			name:  "-l with empty stdin",
			args:  []string{"-lpe", "stringToUpper(stdin)"},
			stdin: "",
			want:  "",
		},
		{
			name:  "-l without trailing newline",
			args:  []string{"-lpe", "stringToUpper(stdin)"},
			stdin: "hello",
			want:  "HELLO",
		},
		{
			name: "multiline expression in -e",
			args: []string{"-pe", "1 +\n2"},
			want: "3",
		},
	}

	// ==========================================
	// Script flags must NOT trigger eval mode
	// ==========================================
	// When a source file is present, ALL flags are script args — not eval flags.
	t.Run("script with -verbose flag", func(t *testing.T) {
		tmpDir := t.TempDir()
		script := filepath.Join(tmpDir, "test.lang")
		os.WriteFile(script, []byte(`import "lib/sys" (sysArgs)
for a in sysArgs() { print(a) }`), 0644)

		cmd := exec.Command(binaryPath, script, "-verbose", "--port", "8080")
		cmd.Dir = projectRoot
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("Script with flags failed: %v\nstderr: %s", err, stderr.String())
		}
		got := strings.TrimSpace(stdout.String())
		// sysArgs should contain the script path and all flags
		if !strings.Contains(got, "-verbose") {
			t.Errorf("Expected -verbose in output, got:\n%s", got)
		}
		if !strings.Contains(got, "--port") {
			t.Errorf("Expected --port in output, got:\n%s", got)
		}
		if !strings.Contains(got, "8080") {
			t.Errorf("Expected 8080 in output, got:\n%s", got)
		}
	})

	t.Run("script with -e flag as argument", func(t *testing.T) {
		tmpDir := t.TempDir()
		script := filepath.Join(tmpDir, "test.lang")
		os.WriteFile(script, []byte(`import "lib/sys" (sysArgs)
for a in sysArgs() { print(a) }`), 0644)

		cmd := exec.Command(binaryPath, script, "-e", "not_an_expression")
		cmd.Dir = projectRoot
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("Script with -e arg failed: %v\nstderr: %s", err, stderr.String())
		}
		got := strings.TrimSpace(stdout.String())
		if !strings.Contains(got, "-e") {
			t.Errorf("Expected -e in script output, got:\n%s", got)
		}
		if !strings.Contains(got, "not_an_expression") {
			t.Errorf("Expected not_an_expression in script output, got:\n%s", got)
		}
	})

	// Special test: -pe without stdin must not hang.
	// Use a pipe that never sends EOF (simulates sandbox/CI/chained commands).
	t.Run("no hang without stdin", func(t *testing.T) {
		pr, pw, err := os.Pipe()
		if err != nil {
			t.Fatalf("Failed to create pipe: %v", err)
		}
		defer pr.Close()
		defer pw.Close()

		cmd := exec.Command(binaryPath, "-pe", "1 + 2")
		cmd.Dir = projectRoot
		cmd.Stdin = pr // pipe that never gets EOF

		var stdout bytes.Buffer
		cmd.Stdout = &stdout

		// Must complete within 5 seconds — previously hung forever
		done := make(chan error, 1)
		go func() { done <- cmd.Run() }()

		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Command failed: %v", err)
			}
			got := strings.TrimSpace(stdout.String())
			if got != "3" {
				t.Errorf("want %q, got %q", "3", got)
			}
		case <-time.After(5 * time.Second):
			cmd.Process.Kill()
			t.Fatal("HANG: -pe '1 + 2' without stdin blocked for 5s — stdin is being read unnecessarily")
		}
	})

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tt.args...)
			cmd.Dir = projectRoot

			if tt.stdin != "" {
				cmd.Stdin = bytes.NewBufferString(tt.stdin)
			}

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()

			got := strings.TrimSpace(stdout.String())
			errOut := strings.TrimSpace(stderr.String())

			if tt.errMsg != "" {
				// Expecting an error
				if errOut == "" {
					t.Errorf("Expected error containing %q, got no error", tt.errMsg)
				} else if !strings.Contains(errOut, tt.errMsg) {
					t.Errorf("Expected error containing %q, got:\n%s", tt.errMsg, errOut)
				}
				return
			}

			if err != nil {
				t.Errorf("Command failed: %v\nStderr: %s", err, errOut)
				return
			}

			if got != tt.want {
				t.Errorf("Output mismatch:\n--- want ---\n%s\n--- got ---\n%s\n--- stderr ---\n%s", tt.want, got, errOut)
			}
		})
	}
}
