package main

import (
	"log"
	"strings"

	"github.com/funvibe/funxy/internal/prettyprinter"
)

func (s *LanguageServer) handleFormatting(id interface{}, params DocumentFormattingParams) error {
	log.Printf("Handling formatting request for %s", params.TextDocument.URI)

	// Get document state from cache
	s.mu.RLock()
	docState, exists := s.documents[params.TextDocument.URI]
	s.mu.RUnlock()

	if !exists {
		return s.sendResponse(ResponseMessage{
			Jsonrpc: "2.0",
			ID:      id,
			Result:  []TextEdit{},
		})
	}

	// Read from cache
	docState.Mu.RLock()
	content := docState.Content
	finalCtx := docState.Context
	docState.Mu.RUnlock()

	if finalCtx == nil || finalCtx.AstRoot == nil {
		return s.sendResponse(ResponseMessage{
			Jsonrpc: "2.0",
			ID:      id,
			Result:  []TextEdit{},
		})
	}

	// Format the AST
	printer := prettyprinter.NewCodePrinter()
	if params.Options.TabSize > 0 {
		// Set line width based on tab size preference
		printer.SetLineWidth(params.Options.TabSize * 20) // rough estimate
	}
	finalCtx.AstRoot.Accept(printer)
	formatted := printer.String()

	// Create a text edit that replaces the entire document
	edit := TextEdit{
		Range: Range{
			Start: Position{Line: 0, Character: 0},
			End: Position{
				Line:      strings.Count(content, "\n"),
				Character: len(strings.Split(content, "\n")[strings.Count(content, "\n")]),
			},
		},
		NewText: formatted,
	}

	return s.sendResponse(ResponseMessage{
		Jsonrpc: "2.0",
		ID:      id,
		Result:  []TextEdit{edit},
	})
}
