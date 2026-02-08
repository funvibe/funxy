package main

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type Expectation struct {
	Line   int    // 0-based line number of the target code
	Col    int    // 0-based column number
	Expect string // The expected string in hover
}

func parseExpectations(t *testing.T, filename string) (string, []Expectation) {
	f, err := os.Open(filename)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer f.Close()

	var lines []string
	var expectations []Expectation
	scanner := bufio.NewScanner(f)

	lineNum := 0
	for scanner.Scan() {
		text := scanner.Text()
		lines = append(lines, text)

		// Look for comments like: // ^ Expect: ...
		if strings.Contains(text, "//") {
			commentIdx := strings.Index(text, "//")
			commentContent := text[commentIdx+2:]

			// Check for caret marker
			if caretIdx := strings.Index(commentContent, "^"); caretIdx != -1 {
				// The caret points to a column in the PREVIOUS line
				// Or the current line? Usually these markers are on a separate line below the code.
				// Let's assume the marker line is below the code line.
				// So if we are on line `lineNum`, the code is on `lineNum-1`.

				// Parse "Expect: <value>"
				expectMarker := "Expect:"
				if expIdx := strings.Index(commentContent, expectMarker); expIdx != -1 {
					expectedVal := strings.TrimSpace(commentContent[expIdx+len(expectMarker):])

					// Calculate column: commentIdx + 2 + caretIdx
					// Because the caret is relative to the start of the comment content?
					// No, usually visually aligned.
					// e.g.
					// code...
					// //      ^ Expect: ...
					// The caret in the comment line matches the column in the code line.

					// Real column index of '^' in the raw string 'text'
					realCaretCol := strings.Index(text, "^")

					if lineNum > 0 {
						expectations = append(expectations, Expectation{
							Line:   lineNum - 1,
							Col:    realCaretCol,
							Expect: expectedVal,
						})
					}
				}
			}
		}
		lineNum++
	}

	return strings.Join(lines, "\n"), expectations
}

func TestComprehensiveHover(t *testing.T) {
	filename := "testdata/hover_comprehensive.lang"
	code, expectations := parseExpectations(t, filename)
	t.Logf("Read code from %s:\n%s\n---", filename, code[:100]) // Log first 100 chars

	if len(expectations) == 0 {
		t.Fatalf("No expectations found in %s", filename)
	}

	uri := "file:///hover_comprehensive.lang"
	server, buf := setupServer(t, uri, code)

	// 1. Check strict expectations
	for _, exp := range expectations {
		params := HoverParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: exp.Line, Character: exp.Col},
		}

		buf.Reset()
		if err := server.handleHover(1, params); err != nil {
			t.Errorf("Hover failed at line %d, col %d: %v", exp.Line+1, exp.Col+1, err)
			continue
		}

		output := buf.String()
		body := parseLSPOutput(t, output)

		var resp ResponseMessage
		if err := json.Unmarshal([]byte(body), &resp); err != nil {
			t.Errorf("Failed to parse JSON at line %d, col %d: %v", exp.Line+1, exp.Col+1, err)
			continue
		}

		if resp.Result == nil {
			t.Errorf("At line %d, col %d: Expected %q, got nil result", exp.Line+1, exp.Col+1, exp.Expect)
		} else {
			resBytes, _ := json.Marshal(resp.Result)
			var hover Hover
			json.Unmarshal(resBytes, &hover)
			if !strings.Contains(hover.Contents.Value, exp.Expect) {
				t.Errorf("At line %d, col %d: Expected %q, got %q", exp.Line+1, exp.Col+1, exp.Expect, hover.Contents.Value)
			}
		}
	}

	// 2. Exhaustive robustness check (run on every char)
	// This ensures no panic for any position
	for i := 0; i < len(code); i++ {
		line, col := offsetToLineCol(code, i)
		params := HoverParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: line, Character: col},
		}
		buf.Reset()
		// We ignore errors/output here, just checking for panics (which would fail the test)
		_ = server.handleHover(i, params)
	}
}

func offsetToLineCol(text string, offset int) (int, int) {
	line := 0
	col := 0
	for i := 0; i < offset; i++ {
		if text[i] == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	return line, col
}
