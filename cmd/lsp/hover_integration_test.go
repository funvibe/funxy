package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHover_WebUiDemo_Integration loads the actual kit/web_ui_demo.lang file
// and verifies that hover works for pattern matching, specifically 'v: VNode'.
func TestHover_WebUiDemo_Integration(t *testing.T) {
	// 1. Setup Server
	var buf bytes.Buffer
	server := NewLanguageServer(&buf)

	wd, _ := os.Getwd()
	// Navigate up from cmd/lsp to project root
	projectRoot := filepath.Join(wd, "../..")
	server.rootPath = projectRoot

	// 2. Read Target File
	targetFile := "kit/web_ui_demo.lang"
	absPath := filepath.Join(projectRoot, targetFile)
	content, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", targetFile, err)
	}

	// 3. Simulate didOpen
	uri := "file://" + absPath
	openParams := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: "funxy",
			Version:    1,
			Text:       string(content),
		},
	}
	if err := server.handleDidOpen(openParams); err != nil {
		t.Fatalf("handleDidOpen failed: %v", err)
	}

	// 4. Find coordinates for 'v: VNode'
	lines := strings.Split(string(content), "\n")
	targetLine := -1
	targetCol := -1
	targetStr := "v: VNode"

	for i, line := range lines {
		if idx := strings.Index(line, targetStr); idx != -1 {
			targetLine = i
			targetCol = idx // 'v' is at the start of the match
			break
		}
	}

	if targetLine == -1 {
		t.Fatalf("Could not find '%s' in %s", targetStr, targetFile)
	}

	t.Logf("Found '%s' at Line %d, Col %d", targetStr, targetLine, targetCol)

	// 5. Test Hover on 'v'
	hoverParams := HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position: Position{
			Line:      targetLine,
			Character: targetCol,
		},
	}

	// Hijack the writer to capture response
	// We need to call handleHover indirectly or direct if we can export it or access it
	// Since we are in package main (test), we can call handleHover if it's receiver is *LanguageServer
	// But handleHover is private. We can use handleRequest with a wrapper.

	// Create request JSON
	reqBody, _ := json.Marshal(struct {
		Jsonrpc string      `json:"jsonrpc"`
		ID      int         `json:"id"`
		Method  string      `json:"method"`
		Params  HoverParams `json:"params"`
	}{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "textDocument/hover",
		Params:  hoverParams,
	})

	// Clear buffer
	buf.Reset()

	if err := server.handleMessage(reqBody); err != nil {
		t.Fatalf("handleMessage failed: %v", err)
	}

	// Parse response
	respBytes := buf.Bytes()
	// Skip header
	parts := bytes.SplitN(respBytes, []byte("\r\n\r\n"), 2)
	if len(parts) < 2 {
		t.Fatalf("Invalid response format: %s", string(respBytes))
	}

	var resp ResponseMessage
	if err := json.Unmarshal(parts[1], &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("LSP Error: %v", resp.Error)
	}

	// Extract Hover result
	var hoverResult Hover
	resBytes, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(resBytes, &hoverResult); err != nil {
		t.Fatalf("Failed to unmarshal hover result: %v", err)
	}

	t.Logf("Hover Content: %v", hoverResult.Contents.Value)

	contentStr := hoverResult.Contents.Value

	if !strings.Contains(contentStr, "VNode") {
		t.Errorf("Hover content for 'v' expected to contain 'VNode', got: %s", contentStr)
	}

	// 6. Test Hover on 'vnode' (Line 12)
	// "vnode = match res"
	targetStr2 := "vnode ="
	targetLine2 := -1
	targetCol2 := -1
	for i, line := range lines {
		if idx := strings.Index(line, targetStr2); idx != -1 {
			targetLine2 = i
			targetCol2 = idx
			break
		}
	}

	if targetLine2 != -1 {
		t.Logf("Found '%s' at Line %d, Col %d", targetStr2, targetLine2, targetCol2)

		hoverParams2 := HoverParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position: Position{
				Line:      targetLine2,
				Character: targetCol2,
			},
		}

		reqBody2, _ := json.Marshal(struct {
			Jsonrpc string      `json:"jsonrpc"`
			ID      int         `json:"id"`
			Method  string      `json:"method"`
			Params  HoverParams `json:"params"`
		}{
			Jsonrpc: "2.0",
			ID:      2,
			Method:  "textDocument/hover",
			Params:  hoverParams2,
		})

		buf.Reset()
		server.handleMessage(reqBody2)

		respBytes2 := buf.Bytes()
		parts2 := bytes.SplitN(respBytes2, []byte("\r\n\r\n"), 2)
		var resp2 ResponseMessage
		json.Unmarshal(parts2[1], &resp2)

		var hoverResult2 Hover
		resBytes2, _ := json.Marshal(resp2.Result)
		json.Unmarshal(resBytes2, &hoverResult2)

		t.Logf("Hover Content for 'vnode': %v", hoverResult2.Contents.Value)

		// Convert content to string for check
		contentStr2 := hoverResult2.Contents.Value

		// 'vnode' should be VNode because the match returns 'v' (VNode) or Text (VNode)
		if !strings.Contains(contentStr2, "VNode") {
			t.Errorf("Hover content for 'vnode' expected to contain 'VNode', got: %s", contentStr2)
		}
	}
}
