package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestArbitraryHover runs a set of data-driven tests for LSP Hover.
// The input code should contain a '$' character to mark the cursor position.
func TestArbitraryHover(t *testing.T) {
	tests := []struct {
		name     string
		code     string // Use '$' to mark cursor position
		expected string // Substring expected in hover content. Empty means expect nil/no info.
	}{
		{
			name: "Space after Right Parenthesis with Imports",
			code: `
import "lib/map" (mapNew, mapSize)
import "lib/test" (assertEquals)
fun main() {
    m1 = mapNew()
    assertEquals(0, mapSize(m1),$ "mapNew creates empty map")
}`,
			expected: "Nil",
		},
		{
			name: "AfterRight Parenthesis with Imports",
			code: `
import "lib/map" (mapNew, mapSize)
import "lib/test" (assertEquals)
fun main() {
    m1 = mapNew()
    assertEquals(0, mapSize(m1)$, "mapNew creates empty map")
}`,
			expected: "Int", // Expect return type of mapSize because of "just after )" logic
		},
		{
			name: "Right Parenthesis with Imports",
			code: `
import "lib/map" (mapNew, mapSize)
import "lib/test" (assertEquals)
fun main() {
    m1 = mapNew()
    assertEquals(0, mapSize(m1$), "mapNew creates empty map")
}`,
			expected: "Int", // Expect return type of mapSize
		},
		{
			name:     "Tuple Empty Right Parenthesis",
			code:     "t = (1, ($))",
			expected: "()",
		},
		{
			name:     "Tuple Empty Left Parenthesis",
			code:     "t = (1, $())",
			expected: "()",
		},
		{
			name:     "Tuple Pair Space",
			code:     "t = (1,$ ())",
			expected: "(Int, ())",
		},
		{
			name:     "Tuple Pair Right Parenthesis",
			code:     "t = (1, ()$)",
			expected: "(Int, ())",
		},
		{
			name:     "Tuple Pair First",
			code:     "t = ($1, ())",
			expected: "Int",
		},
		{
			name:     "Builtin Print Signature",
			code:     "fun main() { print$('A') }",
			expected: "...a",
		},
		{
			name:     "Builtin Print Paren",
			code:     "fun main() { print$('A') }",
			expected: "...a",
		},
		{
			name:     "Builtin Print Paren Exact",
			code:     "fun main() { print$('A') }", // Cursor at '(', expecting signature
			expected: "...a",
		},
		{
			name:     "Hover on Function Name",
			code:     "fun main() { pr$int('A') }", // Cursor inside 'print'
			expected: "...a",
		},
		{
			name:     "Int Literal",
			code:     "fun main() { 12$3 }",
			expected: "Int",
		},
		{
			name:     "String Literal",
			code:     "fun main() { \"He$llo\" }",
			expected: "String",
		},
		{
			name:     "Comment Line",
			code:     "// This is a $comment",
			expected: "", // Expect nil
		},
		{
			name:     "Comment Block",
			code:     "/* Block $comment */",
			expected: "", // Expect nil
		},
		{
			name:     "Comment Start",
			code:     "/$* Comment */", // Right at start
			expected: "",
		},
		{
			name:     "Whitespace",
			code:     "fun main() {  $  }",
			expected: "",
		},
		{
			name:     "Local Variable",
			code:     "fun main(x: Int) { $x }",
			expected: "Int",
		},
		{
			name:     "List Type",
			code:     "fun main() { $[1, 2, 3] }",
			expected: "List", // Type string might be (List Int)
		},
		{
			name:     "Record Type",
			code:     "fun main(r: { x: Int }) { $r }",
			expected: "{ x: Int }",
		},
		{
			name:     "Unknown Symbol",
			code:     "fun main() { unkno$wn }",
			expected: ": a", // Analyzed as inferred type (fresh variable) despite error, prettified to 'a'
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkHover(t, tt.code, tt.expected)
		})
	}
}

func TestArbitraryCompletion(t *testing.T) {
	tests := []struct {
		name     string
		code     string // Use '$' to mark cursor position
		expected string // Label expected in completion items
	}{
		{
			name: "Local Variable",
			code: `
fun main() {
    val = 123
    v$
}`,
			expected: "val",
		},
		{
			name: "Function Parameter",
			code: `
fun add(x, y) {
    x + $
}`,
			expected: "y",
		},
		{
			name: "Shadowed Variable",
			code: `
fun main() {
    x = 1
    {
        x = 2
        $
    }
}`,
			expected: "x",
		},
		{
			name: "Variable inside If block",
			code: `
fun main() {
    outerVar = 42
    if (true) {
        ou$
    }
}`,
			expected: "outerVar",
		},
		{
			name: "Multiple Parameters",
			code: `
fun process(aVar, bVar, cVar) {
    b$
}`,
			expected: "bVar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkCompletion(t, tt.code, tt.expected)
		})
	}
}

func checkCompletion(t *testing.T, codeWithMarker, expected string) {
	// 1. Find cursor position
	idx := strings.Index(codeWithMarker, "$")
	if idx == -1 {
		t.Fatalf("Test code must contain '$' marker: %s", codeWithMarker)
	}

	// 2. Remove marker
	code := strings.Replace(codeWithMarker, "$", "", 1)

	// 3. Calculate Line and Character
	line := 0
	col := 0
	for i, r := range code {
		if i == idx {
			break
		}
		if r == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}

	// 4. Setup Server
	uri := "file:///completion_arb.funxy"
	server, buf := setupServer(t, uri, code)

	// 5. Send Completion Request
	params := CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: col},
	}

	if err := server.handleCompletion(1, params); err != nil {
		t.Fatalf("handleCompletion failed: %v", err)
	}

	// 6. Parse Response
	output := buf.String()
	body := parseLSPOutput(t, output)

	var resp ResponseMessage
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v\nOutput: %s", err, body)
	}

	resBytes, _ := json.Marshal(resp.Result)
	var list CompletionList
	if err := json.Unmarshal(resBytes, &list); err != nil {
		t.Fatalf("Failed to unmarshal CompletionList: %v", err)
	}

	found := false
	for _, item := range list.Items {
		if item.Label == expected {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected completion item %q not found in list", expected)
	}
}

func checkHover(t *testing.T, codeWithMarker, expected string) {
	// 1. Find cursor position
	idx := strings.Index(codeWithMarker, "$")
	if idx == -1 {
		t.Fatalf("Test code must contain '$' marker: %s", codeWithMarker)
	}

	// 2. Remove marker to get actual code
	code := strings.Replace(codeWithMarker, "$", "", 1)

	// 3. Calculate Line and Character
	// Count newlines before idx to get line.
	// Count chars since last newline to get character.
	line := 0
	col := 0
	for i, r := range code {
		if i == idx {
			break
		}
		if r == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}

	// 4. Setup Server
	uri := "file:///arbitrary.funxy"
	server, buf := setupServer(t, uri, code)

	// 5. Send Hover Request
	params := HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: col},
	}

	// Use handleHover directly (bypassing JSON-RPC encoding wrapper usually,
	// but here we call the handler which writes to buf)
	if err := server.handleHover(1, params); err != nil {
		t.Fatalf("handleHover failed: %v", err)
	}

	// 6. Parse Response
	output := buf.String()
	body := parseLSPOutput(t, output)

	var resp ResponseMessage
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v\nOutput: %s", err, body)
	}

	// 7. Assertions
	if expected == "" {
		if resp.Result != nil {
			resBytes, _ := json.Marshal(resp.Result)
			if string(resBytes) != "null" {
				t.Errorf("Expected nil/null result, got: %s", string(resBytes))
			}
		}
	} else {
		if resp.Result == nil {
			t.Fatalf("Expected hover containing %q, got nil result", expected)
		}

		resBytes, _ := json.Marshal(resp.Result)
		var hover Hover
		if err := json.Unmarshal(resBytes, &hover); err != nil {
			t.Fatalf("Failed to unmarshal Hover result: %v", err)
		}

		content := hover.Contents.Value
		if !strings.Contains(content, expected) {
			t.Errorf("Hover content mismatch.\nExpected substring: %q\nActual content: %q", expected, content)
		}
	}
}
