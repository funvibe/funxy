package evaluator

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// =============================================================================
// 1.1 visibleLen — regression test for bug #2 (Unicode/ANSI padding)
// =============================================================================

func TestVisibleLen(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		// Basic cases
		{"ASCII baseline", "hello", 5},
		{"Empty string", "", 0},

		// Unicode multi-byte characters (main regression case)
		{"Single bullet (3 bytes → 1 rune)", "●", 1},
		{"Multiple bullets", "●●●", 3},
		{"Unicode + ASCII", "✓ Done", 6},

		// ANSI + Unicode (exact case from bug)
		{"ANSI + Unicode bullet", "\033[32m●\033[39m", 1},
		{"Nested ANSI codes", "\033[1m\033[31mERROR\033[22m\033[39m", 5},
		{"ANSI in middle", "\033[1mhello\033[22m world", 11},

		// CJK characters (fullwidth: 2 columns each)
		{"CJK symbols", "日本語", 6},
		{"CJK mixed with ASCII", "Hello日本", 9},
		{"Katakana", "テスト", 6},
		{"Hiragana", "あいう", 6},

		// 256-color ANSI
		{"256-color ANSI", "\033[38;5;196mred\033[0m", 3},

		// Truecolor ANSI
		{"Truecolor ANSI", "\033[38;2;255;0;0mred\033[39m", 3},

		// Edge cases
		{"Only ANSI codes", "\033[1m\033[22m", 0},
		{"Multiple ANSI sequences", "\033[31m\033[1mA\033[22m\033[39m", 1},
		{"Tab character", "a\tb", 2}, // \t is control char (width 0)
		{"Newline", "a\nb", 2},       // \n is control char (width 0)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := visibleLen(tt.input)
			if result != tt.expected {
				t.Errorf("visibleLen(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// 1.2 parseHexColor — hex color parsing
// =============================================================================

func TestParseHexColor(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantR     int
		wantG     int
		wantB     int
		wantError bool
	}{
		// Valid formats
		{"Standard format with #", "#FF0000", 255, 0, 0, false},
		{"Without #", "FF0000", 255, 0, 0, false},
		{"Short 3-char format", "#F00", 255, 0, 0, false},
		{"Lowercase", "#00ff00", 0, 255, 0, false},
		{"Black", "#000000", 0, 0, 0, false},
		{"White", "#FFFFFF", 255, 255, 255, false},
		{"Mixed case", "#AbCdEf", 171, 205, 239, false},
		{"Short lowercase", "#abc", 170, 187, 204, false},

		// Invalid formats
		{"Invalid hex chars", "#ZZZZZZ", 0, 0, 0, true},
		{"Too short", "#12", 0, 0, 0, true},
		{"Empty string", "", 0, 0, 0, true},
		{"Too long", "#1234567", 0, 0, 0, true},
		{"Invalid char in middle", "#FF00GG", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, g, b, err := parseHexColor(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("parseHexColor(%q) expected error, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("parseHexColor(%q) unexpected error: %v", tt.input, err)
				}
				if r != tt.wantR || g != tt.wantG || b != tt.wantB {
					t.Errorf("parseHexColor(%q) = (%d, %d, %d), want (%d, %d, %d)",
						tt.input, r, g, b, tt.wantR, tt.wantG, tt.wantB)
				}
			}
		})
	}
}

// =============================================================================
// 1.3 table rendering — regression test for bug #2
// =============================================================================

func TestTableRendering(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		rows    [][]string
		checks  []string // substrings that must be present
	}{
		{
			name:    "ASCII basic",
			headers: []string{"A", "B"},
			rows:    [][]string{{"1", "2"}},
			checks:  []string{"┌", "┐", "└", "┘", "│ A │", "│ 1 │"},
		},
		{
			name:    "Unicode in cells (regression)",
			headers: []string{"Name", "Status"},
			rows:    [][]string{{"api", "●"}, {"db", "✓"}},
			checks:  []string{"│ api", "│ ●", "│ db", "│ ✓"},
		},
		{
			name:    "Header wider than data",
			headers: []string{"LongHeader"},
			rows:    [][]string{{"x"}},
			checks:  []string{"LongHeader", "│ x"},
		},
		{
			name:    "Data wider than header",
			headers: []string{"H"},
			rows:    [][]string{{"very long data"}},
			checks:  []string{"very long data"},
		},
		{
			name:    "Empty cells",
			headers: []string{"A", "B"},
			rows:    [][]string{{"", ""}},
			checks:  []string{"│ A │", "│   │"},
		},
		{
			name:    "Single column",
			headers: []string{"X"},
			rows:    [][]string{{"1"}, {"2"}},
			checks:  []string{"│ X │", "│ 1 │", "│ 2 │"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			e := &Evaluator{Out: &buf}

			// Convert to Objects
			headerObjs := make([]Object, len(tt.headers))
			for i, h := range tt.headers {
				headerObjs[i] = stringToList(h)
			}
			headerList := newList(headerObjs)

			rowObjs := make([]Object, len(tt.rows))
			for i, row := range tt.rows {
				cellObjs := make([]Object, len(row))
				for j, cell := range row {
					cellObjs[j] = stringToList(cell)
				}
				rowObjs[i] = newList(cellObjs)
			}
			rowList := newList(rowObjs)

			result := builtinTable(e, headerList, rowList)
			if isError(result) {
				t.Fatalf("builtinTable returned error: %v", result)
			}

			output := buf.String()
			for _, check := range tt.checks {
				if !strings.Contains(output, check) {
					t.Errorf("table output missing %q\nGot:\n%s", check, output)
				}
			}

			// Verify all table lines have same visible width (alignment check)
			lines := strings.Split(strings.TrimSpace(output), "\n")
			var widths []int
			for _, line := range lines {
				if strings.HasPrefix(line, "│") || strings.HasPrefix(line, "┌") ||
					strings.HasPrefix(line, "├") || strings.HasPrefix(line, "└") {
					widths = append(widths, visibleLen(line))
				}
			}
			if len(widths) > 1 {
				first := widths[0]
				for i, w := range widths {
					if w != first {
						t.Errorf("table line %d has width %d, expected %d (misaligned)", i, w, first)
					}
				}
			}
		})
	}
}

// Test table with ANSI codes in cells
func TestTableWithANSI(t *testing.T) {
	var buf bytes.Buffer
	e := &Evaluator{Out: &buf}

	// Simulate ANSI-colored content
	greenBullet := "\033[32m●\033[39m"
	redX := "\033[31m✗\033[39m"

	headers := newList([]Object{stringToList("S"), stringToList("V")})
	rows := newList([]Object{
		newList([]Object{stringToList(greenBullet), stringToList("ok")}),
		newList([]Object{stringToList(redX), stringToList("no")}),
	})

	result := builtinTable(e, headers, rows)
	if isError(result) {
		t.Fatalf("builtinTable with ANSI returned error: %v", result)
	}

	output := buf.String()

	// Verify alignment - all lines should have same visible width
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var widths []int
	for _, line := range lines {
		if strings.HasPrefix(line, "│") || strings.HasPrefix(line, "┌") ||
			strings.HasPrefix(line, "├") || strings.HasPrefix(line, "└") {
			widths = append(widths, visibleLen(line))
		}
	}
	if len(widths) > 1 {
		first := widths[0]
		for i, w := range widths {
			if w != first {
				t.Errorf("ANSI table line %d has width %d, expected %d", i, w, first)
			}
		}
	}
}

// =============================================================================
// 1.4 fallbackSelect — alternative path for select (non-TTY)
// =============================================================================

func TestFallbackSelect(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		options  []string
		multi    bool
		expected []int
	}{
		// Single select
		{"Select option 2", "2\n", []string{"a", "b", "c"}, false, []int{1}},
		{"Select option 1", "1\n", []string{"a", "b", "c"}, false, []int{0}},
		{"Out of range defaults to 0", "99\n", []string{"a", "b", "c"}, false, []int{0}},
		{"Non-number defaults to 0", "abc\n", []string{"a", "b", "c"}, false, []int{0}},

		// Multi select
		{"Multi: select 1,3", "1,3\n", []string{"a", "b", "c", "d"}, true, []int{0, 2}},
		{"Multi: select single", "2\n", []string{"a", "b", "c", "d"}, true, []int{1}},
		{"Multi: empty input", "\n", []string{"a", "b"}, true, []int{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore stdin
			oldStdin := os.Stdin
			defer func() {
				os.Stdin = oldStdin
				resetStdinReader()
			}()

			// Create pipe for stdin
			r, w, _ := os.Pipe()
			os.Stdin = r
			resetStdinReader()

			go func() {
				w.WriteString(tt.input)
				w.Close()
			}()

			var buf bytes.Buffer
			e := &Evaluator{Out: &buf}

			result, err := fallbackSelect(e, "Test question", tt.options, tt.multi)
			if err != nil {
				t.Fatalf("fallbackSelect error: %v", err)
			}

			if len(result) != len(tt.expected) {
				t.Errorf("fallbackSelect returned %v, want %v", result, tt.expected)
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("fallbackSelect[%d] = %d, want %d", i, v, tt.expected[i])
				}
			}

			// Verify output contains numbered options
			output := buf.String()
			for i, opt := range tt.options {
				expected := strings.Contains(output, opt) || strings.Contains(output, string(rune('1'+i)))
				if !expected {
					// At least the question should be there
					if !strings.Contains(output, "Test question") {
						t.Errorf("output missing question or options")
					}
				}
			}
		})
	}
}

// =============================================================================
// 1.5 confirm parsing
// =============================================================================

func TestConfirmParsing(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultYes bool
		expected   bool
	}{
		{"y with default true", "y\n", true, true},
		{"n with default true", "n\n", true, false},
		{"yes with default false", "yes\n", false, true},
		{"Enter with default true", "\n", true, true},
		{"Enter with default false", "\n", false, false},
		{"Y uppercase", "Y\n", false, true},
		{"no with default true", "no\n", true, false},
		{"anything else with default true", "anything\n", true, false},
		{"anything else with default false", "anything\n", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore stdin
			oldStdin := os.Stdin
			defer func() {
				os.Stdin = oldStdin
				resetStdinReader()
			}()

			r, w, _ := os.Pipe()
			os.Stdin = r
			resetStdinReader()

			go func() {
				w.WriteString(tt.input)
				w.Close()
			}()

			var buf bytes.Buffer
			e := &Evaluator{Out: &buf}

			args := []Object{stringToList("Continue?")}
			if tt.defaultYes {
				args = append(args, TRUE)
			} else {
				args = append(args, FALSE)
			}

			result := builtinConfirm(e, args...)

			var got bool
			if b, ok := result.(*Boolean); ok {
				got = b.Value
			} else {
				t.Fatalf("builtinConfirm returned %T, want *Boolean", result)
			}

			if got != tt.expected {
				t.Errorf("confirm(%q, default=%v) = %v, want %v",
					strings.TrimSpace(tt.input), tt.defaultYes, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// 1.6 prompt parsing
// =============================================================================

func TestPromptParsing(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultVal string
		expected   string
	}{
		{"User input", "alice\n", "", "alice"},
		{"Empty with default", "\n", "bob", "bob"},
		{"Custom overrides default", "custom\n", "bob", "custom"},
		{"Empty without default", "\n", "", ""},
		{"Input with spaces trimmed", "  hello  \n", "", "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStdin := os.Stdin
			defer func() {
				os.Stdin = oldStdin
				resetStdinReader()
			}()

			r, w, _ := os.Pipe()
			os.Stdin = r
			resetStdinReader()

			go func() {
				w.WriteString(tt.input)
				w.Close()
			}()

			var buf bytes.Buffer
			e := &Evaluator{Out: &buf}

			args := []Object{stringToList("Your name")}
			if tt.defaultVal != "" {
				args = append(args, stringToList(tt.defaultVal))
			}

			result := builtinPrompt(e, args...)

			if isError(result) {
				t.Fatalf("builtinPrompt error: %v", result)
			}

			got := listToString(result.(*List))
			if got != tt.expected {
				t.Errorf("prompt(%q, default=%q) = %q, want %q",
					strings.TrimSpace(tt.input), tt.defaultVal, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// 1.7 password — pipe/fallback mode
// =============================================================================

func TestPasswordFallback(t *testing.T) {
	// Tests the fallback path (readLineFallback) by swapping os.Stdin with a pipe.

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Simple password", "secret123\n", "secret123"},
		{"Empty password", "\n", ""},
		{"Password with spaces", "pass with spaces\n", "pass with spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStdin := os.Stdin
			defer func() {
				os.Stdin = oldStdin
				resetStdinReader()
			}()

			r, w, _ := os.Pipe()
			os.Stdin = r
			resetStdinReader() // force re-create reader from new pipe

			go func() {
				w.WriteString(tt.input)
				w.Close()
			}()

			result, err := readLineFallback()
			if err != nil {
				t.Fatalf("readLineFallback error: %v", err)
			}
			got := string(result)
			if got != tt.expected {
				t.Errorf("readLineFallback(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// 1.8 splitString helper
// =============================================================================

func TestSplitString(t *testing.T) {
	tests := []struct {
		input    string
		sep      byte
		expected []string
	}{
		{"1,2,3", ',', []string{"1", "2", "3"}},
		{"1", ',', []string{"1"}},
		{"", ',', []string{""}},
		{",,,", ',', []string{"", "", "", ""}},
		{"a:b:c", ':', []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitString(tt.input, tt.sep)
			if len(result) != len(tt.expected) {
				t.Errorf("splitString(%q, %q) = %v, want %v", tt.input, string(tt.sep), result, tt.expected)
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("splitString(%q, %q)[%d] = %q, want %q", tt.input, string(tt.sep), i, v, tt.expected[i])
				}
			}
		})
	}
}

// =============================================================================
// 1.9 rgb / bgRgb / hex / bgHex
// =============================================================================

func TestRgbFunctions(t *testing.T) {
	e := &Evaluator{Out: io.Discard}

	t.Run("rgb with valid args", func(t *testing.T) {
		result := builtinRgb(e, &Integer{Value: 255}, &Integer{Value: 0}, &Integer{Value: 0}, stringToList("red"))
		if isError(result) {
			t.Errorf("rgb returned error: %v", result)
		}
		// stripAnsi should recover original text
		stripped := ansiRegex.ReplaceAllString(listToString(result.(*List)), "")
		if stripped != "red" {
			t.Errorf("rgb text = %q, want %q", stripped, "red")
		}
	})

	t.Run("rgb with wrong types", func(t *testing.T) {
		result := builtinRgb(e, stringToList("not"), stringToList("int"), stringToList("args"), stringToList("text"))
		if !isError(result) {
			t.Error("rgb with wrong types should return error")
		}
	})

	t.Run("rgb with wrong arity", func(t *testing.T) {
		result := builtinRgb(e, &Integer{Value: 255}, &Integer{Value: 0}, &Integer{Value: 0})
		if !isError(result) {
			t.Error("rgb with 3 args should return error")
		}
	})

	t.Run("bgRgb with valid args", func(t *testing.T) {
		result := builtinBgRgb(e, &Integer{Value: 0}, &Integer{Value: 255}, &Integer{Value: 0}, stringToList("green"))
		if isError(result) {
			t.Errorf("bgRgb returned error: %v", result)
		}
		stripped := ansiRegex.ReplaceAllString(listToString(result.(*List)), "")
		if stripped != "green" {
			t.Errorf("bgRgb text = %q, want %q", stripped, "green")
		}
	})

	t.Run("hex with valid color", func(t *testing.T) {
		result := builtinHex(e, stringToList("#FF0000"), stringToList("red"))
		if isError(result) {
			t.Errorf("hex returned error: %v", result)
		}
		stripped := ansiRegex.ReplaceAllString(listToString(result.(*List)), "")
		if stripped != "red" {
			t.Errorf("hex text = %q, want %q", stripped, "red")
		}
	})

	t.Run("hex with invalid color", func(t *testing.T) {
		// Note: When color level < truecolor, hex returns text unchanged (no error)
		// This test verifies the parseHexColor function directly for error cases
		_, _, _, err := parseHexColor("#ZZZZZZ")
		if err == nil {
			t.Error("parseHexColor with invalid color should return error")
		}
	})

	t.Run("hex with short format", func(t *testing.T) {
		result := builtinHex(e, stringToList("#F00"), stringToList("short"))
		if isError(result) {
			t.Errorf("hex with short format returned error: %v", result)
		}
		stripped := ansiRegex.ReplaceAllString(listToString(result.(*List)), "")
		if stripped != "short" {
			t.Errorf("hex short text = %q, want %q", stripped, "short")
		}
	})

	t.Run("bgHex with valid color", func(t *testing.T) {
		result := builtinBgHex(e, stringToList("#00FF00"), stringToList("green"))
		if isError(result) {
			t.Errorf("bgHex returned error: %v", result)
		}
		stripped := ansiRegex.ReplaceAllString(listToString(result.(*List)), "")
		if stripped != "green" {
			t.Errorf("bgHex text = %q, want %q", stripped, "green")
		}
	})
}

// =============================================================================
// 1.10 Spinner lifecycle
// =============================================================================

func TestSpinnerLifecycle(t *testing.T) {
	var buf bytes.Buffer
	e := &Evaluator{Out: &buf}

	t.Run("spinnerStart returns Handle", func(t *testing.T) {
		result := builtinSpinnerStart(e, stringToList("Loading..."))
		handle, ok := result.(*TermHandle)
		if !ok {
			t.Fatalf("spinnerStart returned %T, want *TermHandle", result)
		}
		if handle.kind != "spinner" {
			t.Errorf("handle.kind = %q, want %q", handle.kind, "spinner")
		}

		// Clean up
		builtinSpinnerStop(e, handle, stringToList("Done"))
	})

	t.Run("spinnerUpdate changes message", func(t *testing.T) {
		handle := builtinSpinnerStart(e, stringToList("Initial"))
		result := builtinSpinnerUpdate(e, handle, stringToList("Updated"))
		if isError(result) {
			t.Errorf("spinnerUpdate returned error: %v", result)
		}
		builtinSpinnerStop(e, handle, stringToList("Done"))
	})

	t.Run("spinnerStop is idempotent", func(t *testing.T) {
		handle := builtinSpinnerStart(e, stringToList("Test"))
		builtinSpinnerStop(e, handle, stringToList("Done1"))
		// Second stop should not panic
		result := builtinSpinnerStop(e, handle, stringToList("Done2"))
		if isError(result) {
			t.Errorf("second spinnerStop returned error: %v", result)
		}
	})

	t.Run("spinnerUpdate on invalid handle", func(t *testing.T) {
		handle := builtinSpinnerStart(e, stringToList("Test"))
		builtinSpinnerStop(e, handle, stringToList("Done"))
		// Update after stop
		result := builtinSpinnerUpdate(e, handle, stringToList("Should fail"))
		if !isError(result) {
			t.Error("spinnerUpdate on stopped spinner should return error")
		}
	})
}

// =============================================================================
// 1.11 Progress lifecycle
// =============================================================================

func TestProgressLifecycle(t *testing.T) {
	var buf bytes.Buffer
	e := &Evaluator{Out: &buf}

	t.Run("progressNew returns Handle", func(t *testing.T) {
		result := builtinProgressNew(e, &Integer{Value: 10}, stringToList("test"))
		handle, ok := result.(*TermHandle)
		if !ok {
			t.Fatalf("progressNew returned %T, want *TermHandle", result)
		}
		if handle.kind != "progress" {
			t.Errorf("handle.kind = %q, want %q", handle.kind, "progress")
		}
		builtinProgressDone(e, handle)
	})

	t.Run("progressTick increments", func(t *testing.T) {
		handle := builtinProgressNew(e, &Integer{Value: 10}, stringToList("test"))
		for i := 0; i < 5; i++ {
			result := builtinProgressTick(e, handle)
			if isError(result) {
				t.Errorf("progressTick %d returned error: %v", i, result)
			}
		}
		builtinProgressDone(e, handle)
	})

	t.Run("progressSet sets value", func(t *testing.T) {
		handle := builtinProgressNew(e, &Integer{Value: 10}, stringToList("test"))
		result := builtinProgressSet(e, handle, &Integer{Value: 5})
		if isError(result) {
			t.Errorf("progressSet returned error: %v", result)
		}
		builtinProgressDone(e, handle)
	})

	t.Run("progressDone is idempotent", func(t *testing.T) {
		handle := builtinProgressNew(e, &Integer{Value: 10}, stringToList("test"))
		builtinProgressDone(e, handle)
		// Second done should not panic
		result := builtinProgressDone(e, handle)
		if isError(result) {
			t.Errorf("second progressDone returned error: %v", result)
		}
	})

	t.Run("progressTick on completed handle", func(t *testing.T) {
		handle := builtinProgressNew(e, &Integer{Value: 10}, stringToList("test"))
		builtinProgressDone(e, handle)
		result := builtinProgressTick(e, handle)
		if !isError(result) {
			t.Error("progressTick on completed progress should return error")
		}
	})

	t.Run("progressNew with zero total", func(t *testing.T) {
		// Should not cause division by zero
		handle := builtinProgressNew(e, &Integer{Value: 0}, stringToList("test"))
		if isError(handle) {
			t.Errorf("progressNew(0) returned error: %v", handle)
		}
		builtinProgressDone(e, handle.(*TermHandle))
	})
}

// =============================================================================
// 1.12 multiSelect — fallback mode
// =============================================================================

func TestMultiSelectFallback(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		options  []string
		expected []string
	}{
		{"Select 1,3", "1,3\n", []string{"a", "b", "c"}, []string{"a", "c"}},
		{"Select single", "2\n", []string{"a", "b", "c"}, []string{"b"}},
		{"Empty selection", "\n", []string{"a", "b"}, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStdin := os.Stdin
			defer func() {
				os.Stdin = oldStdin
				resetStdinReader()
			}()

			r, w, _ := os.Pipe()
			os.Stdin = r
			resetStdinReader()

			go func() {
				w.WriteString(tt.input)
				w.Close()
			}()

			var buf bytes.Buffer
			e := &Evaluator{Out: &buf}

			indices, err := fallbackSelect(e, "Test", tt.options, true)
			if err != nil {
				t.Fatalf("fallbackSelect error: %v", err)
			}

			// Convert indices to values
			var got []string
			for _, idx := range indices {
				if idx >= 0 && idx < len(tt.options) {
					got = append(got, tt.options[idx])
				}
			}

			if len(got) != len(tt.expected) {
				t.Errorf("multiSelect got %v, want %v", got, tt.expected)
				return
			}
			for i, v := range got {
				if v != tt.expected[i] {
					t.Errorf("multiSelect[%d] = %q, want %q", i, v, tt.expected[i])
				}
			}
		})
	}
}

// =============================================================================
// 1.13 Error handling (arity/type checks)
// =============================================================================

func TestErrorHandling(t *testing.T) {
	e := &Evaluator{Out: io.Discard}

	t.Run("select with non-list (Integer)", func(t *testing.T) {
		result := builtinSelect(e, stringToList("Question"), &Integer{Value: 42})
		if !isError(result) {
			t.Error("select with Integer should return error")
		}
		errMsg := result.(*Error).Message
		if !strings.Contains(errMsg, "List") {
			t.Errorf("error message should mention List, got: %s", errMsg)
		}
	})

	t.Run("select with empty list", func(t *testing.T) {
		result := builtinSelect(e, stringToList("Question"), newList([]Object{}))
		if !isError(result) {
			t.Error("select with empty list should return error")
		}
		errMsg := result.(*Error).Message
		if !strings.Contains(errMsg, "empty") {
			t.Errorf("error message should mention empty, got: %s", errMsg)
		}
	})

	t.Run("multiSelect with non-list (Integer)", func(t *testing.T) {
		result := builtinMultiSelect(e, stringToList("Question"), &Integer{Value: 42})
		if !isError(result) {
			t.Error("multiSelect with Integer should return error")
		}
	})

	t.Run("progressNew with non-int", func(t *testing.T) {
		result := builtinProgressNew(e, stringToList("not_int"), stringToList("label"))
		if !isError(result) {
			t.Error("progressNew with non-int should return error")
		}
		errMsg := result.(*Error).Message
		if !strings.Contains(errMsg, "Int") {
			t.Errorf("error message should mention Int, got: %s", errMsg)
		}
	})

	t.Run("progressTick with non-handle", func(t *testing.T) {
		result := builtinProgressTick(e, stringToList("not a handle"))
		if !isError(result) {
			t.Error("progressTick with non-handle should return error")
		}
	})

	t.Run("spinnerUpdate with progress handle", func(t *testing.T) {
		progressHandle := builtinProgressNew(e, &Integer{Value: 10}, stringToList("test"))
		result := builtinSpinnerUpdate(e, progressHandle, stringToList("msg"))
		if !isError(result) {
			t.Error("spinnerUpdate with progress handle should return error")
		}
		builtinProgressDone(e, progressHandle.(*TermHandle))
	})

	t.Run("progressSet with spinner handle", func(t *testing.T) {
		spinnerHandle := builtinSpinnerStart(e, stringToList("test"))
		result := builtinProgressSet(e, spinnerHandle, &Integer{Value: 5})
		if !isError(result) {
			t.Error("progressSet with spinner handle should return error")
		}
		builtinSpinnerStop(e, spinnerHandle.(*TermHandle), stringToList("done"))
	})

	// Arity checks
	t.Run("bold with wrong arity", func(t *testing.T) {
		result := builtinBold(e)
		if !isError(result) {
			t.Error("bold with 0 args should return error")
		}
		result = builtinBold(e, stringToList("a"), stringToList("b"))
		if !isError(result) {
			t.Error("bold with 2 args should return error")
		}
	})

	t.Run("cursorTo with wrong arity", func(t *testing.T) {
		result := builtinCursorTo(e, &Integer{Value: 1})
		if !isError(result) {
			t.Error("cursorTo with 1 arg should return error")
		}
	})

	t.Run("cursorTo with wrong types", func(t *testing.T) {
		result := builtinCursorTo(e, stringToList("not"), stringToList("int"))
		if !isError(result) {
			t.Error("cursorTo with non-int args should return error")
		}
	})

	t.Run("table with non-list headers (Integer)", func(t *testing.T) {
		result := builtinTable(e, &Integer{Value: 42}, newList([]Object{}))
		if !isError(result) {
			t.Error("table with Integer headers should return error")
		}
	})

	t.Run("table with non-list rows (Integer)", func(t *testing.T) {
		result := builtinTable(e, newList([]Object{stringToList("H")}), &Integer{Value: 42})
		if !isError(result) {
			t.Error("table with Integer rows should return error")
		}
	})

	t.Run("table with non-list row element (Integer)", func(t *testing.T) {
		headers := newList([]Object{stringToList("H")})
		rows := newList([]Object{&Integer{Value: 42}})
		result := builtinTable(e, headers, rows)
		if !isError(result) {
			t.Error("table with Integer row element should return error")
		}
	})

	t.Run("cprint with too few args", func(t *testing.T) {
		result := builtinCprint(e, stringToList("only one"))
		if !isError(result) {
			t.Error("cprint with 1 arg should return error")
		}
	})

	t.Run("confirm with wrong default type", func(t *testing.T) {
		// Save and restore stdin
		oldStdin := os.Stdin
		defer func() {
			os.Stdin = oldStdin
			resetStdinReader()
		}()
		r, w, _ := os.Pipe()
		os.Stdin = r
		resetStdinReader()
		go func() {
			w.WriteString("y\n")
			w.Close()
		}()

		result := builtinConfirm(e, stringToList("Question"), stringToList("not bool"))
		if !isError(result) {
			t.Error("confirm with non-bool default should return error")
		}
	})
}

// =============================================================================
// Additional edge case tests
// =============================================================================

func TestStripAnsi(t *testing.T) {
	e := &Evaluator{Out: io.Discard}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Plain text", "hello", "hello"},
		{"Empty", "", ""},
		{"Bold", "\033[1mhello\033[22m", "hello"},
		{"Color", "\033[31mred\033[39m", "red"},
		{"Multiple codes", "\033[1m\033[31mbold red\033[39m\033[22m", "bold red"},
		{"256 color", "\033[38;5;196mred\033[0m", "red"},
		{"Truecolor", "\033[38;2;255;0;0mred\033[39m", "red"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builtinStripAnsi(e, stringToList(tt.input))
			if isError(result) {
				t.Fatalf("stripAnsi returned error: %v", result)
			}
			got := listToString(result.(*List))
			if got != tt.expected {
				t.Errorf("stripAnsi(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestTermColors(t *testing.T) {
	e := &Evaluator{Out: io.Discard}

	result := builtinTermColors(e)
	if isError(result) {
		t.Fatalf("termColors returned error: %v", result)
	}

	level, ok := result.(*Integer)
	if !ok {
		t.Fatalf("termColors returned %T, want *Integer", result)
	}

	// Level should be one of: 0, 1, 256, 16777216
	validLevels := map[int64]bool{0: true, 1: true, 256: true, 16777216: true}
	if !validLevels[level.Value] {
		t.Errorf("termColors returned %d, expected one of 0, 1, 256, 16777216", level.Value)
	}
}

func TestTermSize(t *testing.T) {
	e := &Evaluator{Out: io.Discard}

	result := builtinTermSize(e)
	if isError(result) {
		t.Fatalf("termSize returned error: %v", result)
	}

	tuple, ok := result.(*Tuple)
	if !ok {
		t.Fatalf("termSize returned %T, want *Tuple", result)
	}

	if len(tuple.Elements) != 2 {
		t.Fatalf("termSize returned tuple with %d elements, want 2", len(tuple.Elements))
	}

	cols, ok1 := tuple.Elements[0].(*Integer)
	rows, ok2 := tuple.Elements[1].(*Integer)
	if !ok1 || !ok2 {
		t.Fatal("termSize tuple elements should be Integers")
	}

	// Should return positive values (or fallback 80x24)
	if cols.Value <= 0 || rows.Value <= 0 {
		t.Errorf("termSize returned (%d, %d), expected positive values", cols.Value, rows.Value)
	}
}

func TestObjectToDisplayString(t *testing.T) {
	tests := []struct {
		name     string
		input    Object
		expected string
	}{
		{"Nil object", nil, ""},
		{"String (List<Char>)", stringToList("hello"), "hello"},
		{"Integer", &Integer{Value: 42}, "42"},
		{"Boolean true", TRUE, "true"},
		{"Boolean false", FALSE, "false"},
		{"Nil value", &Nil{}, "Nil"}, // Nil.Inspect() returns "Nil"
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := objectToDisplayString(tt.input)
			if result != tt.expected {
				t.Errorf("objectToDisplayString(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Benchmark tests
// =============================================================================

func BenchmarkVisibleLen(b *testing.B) {
	testCases := []string{
		"hello",
		"●●●",
		"\033[32m●\033[39m",
		"\033[1m\033[31mERROR\033[22m\033[39m",
		"日本語テスト",
	}

	for _, tc := range testCases {
		b.Run(tc[:min(10, len(tc))], func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				visibleLen(tc)
			}
		})
	}
}

func BenchmarkParseHexColor(b *testing.B) {
	testCases := []string{"#FF0000", "FF0000", "#F00", "#AbCdEf"}

	for _, tc := range testCases {
		b.Run(tc, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				parseHexColor(tc)
			}
		})
	}
}
