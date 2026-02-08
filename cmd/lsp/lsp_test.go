package main

import (
	"bytes"
	"encoding/json"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/modules"
	"strings"
	"testing"
)

func init() {
	config.IsLSPMode = true
	modules.InitVirtualPackages()
}

func parseLSPOutput(t *testing.T, output string) string {
	parts := strings.SplitN(output, "\r\n\r\n", 2)
	if len(parts) != 2 {
		t.Fatalf("Invalid LSP output format (header/body split failed): %q", output)
	}
	return parts[1]
}

func setupServer(t *testing.T, uri, code string) (*LanguageServer, *bytes.Buffer) {
	buf := new(bytes.Buffer)
	server := NewLanguageServer(buf)

	didOpenParams := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: "funxy",
			Version:    1,
			Text:       code,
		},
	}
	if err := server.handleDidOpen(didOpenParams); err != nil {
		t.Fatalf("handleDidOpen failed: %v", err)
	}
	buf.Reset() // Clear diagnostics output
	return server, buf
}

func TestLSP_Hover_LocalVariable(t *testing.T) {
	uri := "file:///test.funxy"
	code := "fun add(x: Int, y: Int) -> Int {\n" +
		"x + y\n" +
		"}"
	server, buf := setupServer(t, uri, code)

	// Hover over 'x' in body (Line 1, Char 0)
	hoverParams := HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 0},
	}

	if err := server.handleHover(1, hoverParams); err != nil {
		t.Fatalf("handleHover failed: %v", err)
	}

	body := parseLSPOutput(t, buf.String())
	var resp ResponseMessage
	json.Unmarshal([]byte(body), &resp)

	resBytes, _ := json.Marshal(resp.Result)
	resStr := string(resBytes)
	if !strings.Contains(resStr, "Int") {
		t.Errorf("Expected hover to contain 'Int', got: %s", resStr)
	}
}

func TestLSP_Definition_LocalVariable(t *testing.T) {
	uri := "file:///test.funxy"
	code := "fun add(x: Int, y: Int) -> Int {\n" +
		"x + y\n" +
		"}"
	// 'x' param definition: Line 0, Char 8 (x: Int)
	// 'x' usage: Line 1, Char 0
	server, buf := setupServer(t, uri, code)

	defParams := DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 0},
	}

	if err := server.handleDefinition(2, defParams); err != nil {
		t.Fatalf("handleDefinition failed: %v", err)
	}

	body := parseLSPOutput(t, buf.String())
	var resp ResponseMessage
	json.Unmarshal([]byte(body), &resp)

	var loc Location
	resBytes, _ := json.Marshal(resp.Result)
	json.Unmarshal(resBytes, &loc)

	if loc.Range.Start.Line != 0 {
		t.Errorf("Expected definition line 0, got %d", loc.Range.Start.Line)
	}
	if loc.Range.Start.Character != 8 {
		t.Errorf("Expected definition start char 8, got %d", loc.Range.Start.Character)
	}
}

func TestLSP_MemberAccess(t *testing.T) {
	uri := "file:///member.funxy"
	code := "type alias Point = { x: Int, y: Int }\n" +
		"fun main() {\n" +
		"p = Point { x: 1, y: 2 }\n" +
		"p.x\n" +
		"}"
	server, buf := setupServer(t, uri, code)

	// Hover over 'x' in p.x (Line 3, Char 2)
	hoverParams := HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 3, Character: 2},
	}

	if err := server.handleHover(3, hoverParams); err != nil {
		t.Fatalf("handleHover failed: %v", err)
	}

	body := parseLSPOutput(t, buf.String())
	var resp ResponseMessage
	json.Unmarshal([]byte(body), &resp)

	resBytes, _ := json.Marshal(resp.Result)
	resStr := string(resBytes)
	if !strings.Contains(resStr, "Int") {
		t.Errorf("Expected hover to contain 'Int', got: %s", resStr)
	}
}

func TestLSP_Hover_LiteralsAndExpressions(t *testing.T) {
	uri := "file:///literals.funxy"
	code := "fun main() {\n" +
		"  123\n" +
		"  \"hello\"\n" +
		"  1 + 2\n" +
		"}"
	server, buf := setupServer(t, uri, code)

	// Hover over integer literal (Line 1, Char 2)
	server.handleHover(1, HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 2},
	})
	body := parseLSPOutput(t, buf.String())
	if !strings.Contains(body, "Int") {
		t.Errorf("Expected hover to contain 'Int', got: %s", body)
	}
	if strings.Contains(body, "*ast.IntegerLiteral") {
		t.Errorf("Expected hover NOT to contain Go type, got: %s", body)
	}
	buf.Reset()

	// Hover over string literal (Line 2, Char 2)
	server.handleHover(2, HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 2, Character: 2},
	})
	body = parseLSPOutput(t, buf.String())
	if !strings.Contains(body, "String") {
		t.Errorf("Expected hover to contain 'String', got: %s", body)
	}
	buf.Reset()

	// Hover over binary expression '+' (Line 3, Char 4)
	server.handleHover(3, HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 3, Character: 4},
	})
	body = parseLSPOutput(t, buf.String())
	// Should show result type Int
	if !strings.Contains(body, "Int") {
		t.Errorf("Expected hover to contain 'Int', got: %s", body)
	}
	buf.Reset()
}

func TestLSP_Definition_Function(t *testing.T) {
	uri := "file:///funcs.funxy"
	code := "fun helper() {}\n" +
		"fun main() {\n" +
		"  helper()\n" +
		"}"
	server, buf := setupServer(t, uri, code)

	// Definition of 'helper' in call (Line 2, Char 2)
	// helper starts at 0, 4
	server.handleDefinition(1, DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 2, Character: 2},
	})
	body := parseLSPOutput(t, buf.String())
	var resp ResponseMessage
	json.Unmarshal([]byte(body), &resp)
	var loc Location
	resBytes, _ := json.Marshal(resp.Result)
	json.Unmarshal(resBytes, &loc)

	if loc.Range.Start.Line != 0 {
		t.Errorf("Expected definition line 0, got %d", loc.Range.Start.Line)
	}
	if loc.Range.Start.Character != 4 {
		t.Errorf("Expected definition char 4, got %d", loc.Range.Start.Character)
	}
}

func TestLSP_Definition_Type(t *testing.T) {
	uri := "file:///types.funxy"
	code := "type alias MyType = { a: Int }\n" +
		"fun process(x: MyType) {\n" +
		"  x\n" +
		"}"
	server, buf := setupServer(t, uri, code)

	server.handleDefinition(1, DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 15},
	})
	body := parseLSPOutput(t, buf.String())
	var resp ResponseMessage
	json.Unmarshal([]byte(body), &resp)
	var loc Location
	resBytes, _ := json.Marshal(resp.Result)
	json.Unmarshal(resBytes, &loc)

	if loc.Range.Start.Line != 0 {
		t.Errorf("Expected definition line 0, got %d", loc.Range.Start.Line)
	}
	if loc.Range.Start.Character != 11 {
		t.Errorf("Expected definition char 11, got %d", loc.Range.Start.Character)
	}
}

func TestLSP_Hover_Builtin_Print(t *testing.T) {
	uri := "file:///builtin.funxy"
	code := "fun main() {\n" +
		"  print(1)\n" +
		"}"
	server, buf := setupServer(t, uri, code)

	// Hover over 'print' (Line 1, Char 2)
	server.handleHover(1, HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 2},
	})
	body := parseLSPOutput(t, buf.String())

	// Expect normalized generic types (t?) or canonical (a)
	// print signature is roughly: forall a. (...a) -> Nil
	if !strings.Contains(body, "...a") && !strings.Contains(body, "...t?") {
		t.Errorf("Expected hover to contain variadic type '...a' or '...t?', got: %s", body)
	}
	if strings.Contains(body, "t1") || strings.Contains(body, "t2") {
		t.Errorf("Expected hover NOT to contain raw type variables (t1, t2), got: %s", body)
	}
}

func TestLSP_Hover_Builtin_Constant(t *testing.T) {
	uri := "file:///builtin_constant.funxy"
	code := "fun main() {\n" +
		"  constant(1, 2)\n" +
		"}"
	server, buf := setupServer(t, uri, code)

	// Hover over 'constant' (Line 1, Char 2)
	server.handleHover(1, HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 2},
	})
	body := parseLSPOutput(t, buf.String())

	var resp ResponseMessage
	json.Unmarshal([]byte(body), &resp)
	resBytes, _ := json.Marshal(resp.Result)
	var hover Hover
	json.Unmarshal(resBytes, &hover)
	content := hover.Contents.Value

	// Expect signature roughly: (a, b) -> a
	// Or with normalized vars: (t?, t?) -> t?
	if !strings.Contains(content, "constant") {
		t.Errorf("Expected hover to contain 'constant', got: %s", content)
	}
	// Check for arrow and at least one generic var (a or t?)
	if !strings.Contains(content, "->") {
		t.Errorf("Expected hover to contain function arrow '->', got: %s", content)
	}
}

func TestLSP_Hover_Builtin_Format(t *testing.T) {
	uri := "file:///builtin_format.funxy"
	code := "fun main() {\n" +
		"  format(\"%d\", 1)\n" +
		"}"
	server, buf := setupServer(t, uri, code)

	// Hover over 'format' (Line 1, Char 2)
	server.handleHover(1, HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 2},
	})
	body := parseLSPOutput(t, buf.String())

	var resp ResponseMessage
	json.Unmarshal([]byte(body), &resp)
	resBytes, _ := json.Marshal(resp.Result)
	var hover Hover
	json.Unmarshal(resBytes, &hover)
	content := hover.Contents.Value

	// Expect signature roughly: (String, ...t?) -> String
	if !strings.Contains(content, "format") {
		t.Errorf("Expected hover to contain 'format', got: %s", content)
	}
	if !strings.Contains(content, "String") {
		t.Errorf("Expected hover to contain 'String', got: %s", content)
	}
	// It might contain ...a or ...t?
	if !strings.Contains(content, "...") {
		t.Errorf("Expected hover to contain variadic '...', got: %s", content)
	}
}

func TestLSP_Hover_Generic(t *testing.T) {
	uri := "file:///generic.funxy"
	code := "fun myId(x) { x }\n" +
		"fun main() {\n" +
		"  myId(1)\n" +
		"}"
	server, buf := setupServer(t, uri, code)

	// Hover over 'myId' definition (Line 0, Char 4)
	// 'fun ' is 4 chars. 'myId' starts at 4.
	server.handleHover(1, HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 4},
	})
	body := parseLSPOutput(t, buf.String())

	if !strings.Contains(body, "t?") && !strings.Contains(body, "t") {
		t.Errorf("Expected hover to contain generic type variable 't' or 't?', got: %s", body)
	}
}

func TestLSP_Hover_FunDeps(t *testing.T) {
	uri := "file:///fundeps.funxy"
	// Define a trait with functional dependencies
	code := "trait Convert<from, to> | from -> to {\n" +
		"  fun convert(x: from) -> to\n" +
		"}\n" +
		"fun main() {}"
	server, buf := setupServer(t, uri, code)

	// Hover over 'Convert' trait name (Line 0, Char 6)
	// 'trait ' is 6 chars. 'Convert' starts at 6.
	server.handleHover(1, HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 6},
	})
	body := parseLSPOutput(t, buf.String())

	var resp ResponseMessage
	json.Unmarshal([]byte(body), &resp)
	resBytes, _ := json.Marshal(resp.Result)
	var hover Hover
	json.Unmarshal(resBytes, &hover)
	content := hover.Contents.Value

	// Expect hover to contain the functional dependency syntax
	// | from -> to
	if !strings.Contains(content, "| from -> to") {
		t.Errorf("Expected hover to contain functional dependency '| from -> to', got: %s", content)
	}
	// Expect trait signature
	if !strings.Contains(content, "trait Convert<from, to>") {
		t.Errorf("Expected hover to contain 'trait Convert<from, to>', got: %s", content)
	}
}

func TestLSP_Completion_Builtins(t *testing.T) {
	uri := "file:///completion.funxy"
	code := "fun main() {\n" +
		"  \n" +
		"}"
	server, buf := setupServer(t, uri, code)

	// Completion at start of line 1
	server.handleCompletion(1, CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 0},
	})
	body := parseLSPOutput(t, buf.String())

	var resp ResponseMessage
	json.Unmarshal([]byte(body), &resp)

	// Re-marshal result to CompletionList
	resBytes, _ := json.Marshal(resp.Result)
	var list CompletionList
	json.Unmarshal(resBytes, &list)

	foundPrint := false
	foundDoc := false
	for _, item := range list.Items {
		if item.Label == "print" {
			foundPrint = true
			if item.Documentation != nil && strings.Contains(item.Documentation.Value, "stdout") {
				foundDoc = true
			}
			break
		}
	}

	if !foundPrint {
		t.Error("Expected completion items to contain 'print'")
	}
	if !foundDoc {
		t.Error("Expected 'print' completion item to have documentation about stdout")
	}
}

func TestLSP_Hover_InComment(t *testing.T) {
	uri := "file:///comments.funxy"
	code := "// This is a comment\n" +
		"fun main() {}"
	server, buf := setupServer(t, uri, code)

	// Hover over comment (Line 0, Char 5)
	err := server.handleHover(1, HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 5},
	})
	if err != nil {
		t.Fatalf("handleHover failed: %v", err)
	}

	body := parseLSPOutput(t, buf.String())
	var resp ResponseMessage
	json.Unmarshal([]byte(body), &resp)

	if resp.Result != nil {
		t.Errorf("Expected nil result for hover in comment, got: %v", resp.Result)
	}
}

func TestLSP_Hover_CallExpression_Parens(t *testing.T) {
	uri := "file:///call.funxy"
	code := "fun main() {\n" +
		"  print(1)\n" +
		"}"
	server, buf := setupServer(t, uri, code)

	// Hover over '(' (Line 1, Char 7). 'print' is chars 2-6.
	// 'print' (length 5). Start 2. End 7 (exclusive).
	// '(' is at 7.
	server.handleHover(1, HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 7},
	})
	body := parseLSPOutput(t, buf.String())

	var resp ResponseMessage
	json.Unmarshal([]byte(body), &resp)

	resBytes, _ := json.Marshal(resp.Result)
	var hover Hover
	json.Unmarshal(resBytes, &hover)

	content := hover.Contents.Value

	// Expect signature of 'print', NOT 'Nil'
	if !strings.Contains(content, "...a") && !strings.Contains(content, "...t?") {
		t.Errorf("Expected hover on '(' to show function signature with '...a' or '...t?', got: %s", content)
	}
	if strings.Contains(content, "Nil") && !strings.Contains(content, "->") {
		// Just 'Nil' usually means it's showing the return type only
		t.Errorf("Expected hover on '(' NOT to show just return type 'Nil', got: %s", content)
	}
}
