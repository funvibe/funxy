package generators

import (
	"encoding/json"
	"fmt"
)

// LSPGenerator generates a sequence of LSP messages.
type LSPGenerator struct {
	*Generator
	openDocs map[string]string // URI -> Content
	docURIs  []string
	msgID    int
}

func NewLSPGenerator(data []byte) *LSPGenerator {
	return &LSPGenerator{
		Generator: NewFromData(data),
		openDocs:  make(map[string]string),
		docURIs:   []string{},
		msgID:     1,
	}
}

// GenerateLSPSequence generates a sequence of JSON-RPC messages.
func (g *LSPGenerator) GenerateLSPSequence() []string {
	var messages []string

	// Always start with initialize
	messages = append(messages, g.generateInitialize())

	count := g.Src().Intn(20) + 5
	for i := 0; i < count; i++ {
		messages = append(messages, g.generateRandomMessage())
	}

	// Always end with shutdown and exit
	messages = append(messages, g.generateShutdown())
	messages = append(messages, g.generateExit())

	return messages
}

func (g *LSPGenerator) generateRandomMessage() string {
	// Weighted choice
	choice := g.Src().Intn(100)
	switch {
	case choice < 10: // 10% Open
		return g.generateDidOpen()
	case choice < 40: // 30% Change (typing)
		return g.generateDidChange()
	case choice < 60: // 20% Hover
		return g.generateHover()
	case choice < 80: // 20% Completion
		return g.generateCompletion()
	case choice < 90: // 10% Definition
		return g.generateDefinition()
	default: // 10% Close
		return g.generateDidClose()
	}
}

func (g *LSPGenerator) nextID() int {
	id := g.msgID
	g.msgID++
	return id
}

func (g *LSPGenerator) formatMessage(method string, params interface{}, isRequest bool) string {
	msg := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if isRequest {
		msg["id"] = g.nextID()
	}
	if params != nil {
		msg["params"] = params
	}

	data, _ := json.Marshal(msg)
	content := string(data)
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(content), content)
}

func (g *LSPGenerator) generateInitialize() string {
	return g.formatMessage("initialize", map[string]interface{}{
		"processId": 1234,
		"rootUri":   "file:///tmp/funxy-project",
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"synchronization": map[string]interface{}{
					"didSave":  true,
					"willSave": false,
				},
			},
		},
	}, true)
}

func (g *LSPGenerator) generateShutdown() string {
	return g.formatMessage("shutdown", nil, true)
}

func (g *LSPGenerator) generateExit() string {
	return g.formatMessage("exit", nil, false)
}

func (g *LSPGenerator) generateDidOpen() string {
	uri := fmt.Sprintf("file:///tmp/file_%d.lang", g.Src().Intn(1000))
	// Check if already open
	for _, u := range g.docURIs {
		if u == uri {
			return g.generateDidChange() // Fallback
		}
	}

	content := g.GenerateProgram()
	g.openDocs[uri] = content
	g.docURIs = append(g.docURIs, uri)

	return g.formatMessage("textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        uri,
			"languageId": "funxy",
			"version":    1,
			"text":       content,
		},
	}, false)
}

func (g *LSPGenerator) generateDidChange() string {
	if len(g.docURIs) == 0 {
		return g.generateDidOpen()
	}
	uri := g.docURIs[g.Src().Intn(len(g.docURIs))]

	// Generate new content (simulating a full replace for simplicity, or partial edit)
	// For fuzzing, full replace is easier to track state, but partial edits stress the incremental parser more.
	// Let's do full replace for now as the server likely handles full text sync mostly.
	// Wait, protocol.go has TextDocumentContentChangeEvent with Range.
	// If Range is nil, it's a full text replacement.

	newContent := g.GenerateProgram()
	g.openDocs[uri] = newContent

	return g.formatMessage("textDocument/didChange", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":     uri,
			"version": g.Src().Intn(100) + 2,
		},
		"contentChanges": []map[string]interface{}{
			{
				"text": newContent,
			},
		},
	}, false)
}

func (g *LSPGenerator) generateDidClose() string {
	if len(g.docURIs) == 0 {
		return g.generateDidOpen()
	}
	idx := g.Src().Intn(len(g.docURIs))
	uri := g.docURIs[idx]

	// Remove from state
	delete(g.openDocs, uri)
	g.docURIs = append(g.docURIs[:idx], g.docURIs[idx+1:]...)

	return g.formatMessage("textDocument/didClose", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
	}, false)
}

func (g *LSPGenerator) generateHover() string {
	if len(g.docURIs) == 0 {
		return g.generateDidOpen()
	}
	uri := g.docURIs[g.Src().Intn(len(g.docURIs))]

	// Pick a random position
	// Ideally we should pick a valid position, but random is fine for fuzzing robustness
	line := g.Src().Intn(50)
	char := g.Src().Intn(50)

	return g.formatMessage("textDocument/hover", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": char,
		},
	}, true)
}

func (g *LSPGenerator) generateCompletion() string {
	if len(g.docURIs) == 0 {
		return g.generateDidOpen()
	}
	uri := g.docURIs[g.Src().Intn(len(g.docURIs))]

	line := g.Src().Intn(50)
	char := g.Src().Intn(50)

	return g.formatMessage("textDocument/completion", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": char,
		},
	}, true)
}

func (g *LSPGenerator) generateDefinition() string {
	if len(g.docURIs) == 0 {
		return g.generateDidOpen()
	}
	uri := g.docURIs[g.Src().Intn(len(g.docURIs))]

	line := g.Src().Intn(50)
	char := g.Src().Intn(50)

	return g.formatMessage("textDocument/definition", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": char,
		},
	}, true)
}
