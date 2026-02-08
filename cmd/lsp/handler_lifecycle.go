package main

import (
	"log"
)

func (s *LanguageServer) handleInitialize(id interface{}, params InitializeParams) error {
	log.Printf("Handling initialize request with ID: %v", id)

	if params.RootURI != nil && *params.RootURI != "" {
		s.rootPath = s.uriToPath(*params.RootURI)
	} else if params.RootPath != nil && *params.RootPath != "" {
		s.rootPath = *params.RootPath
	}

	result := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync:           1, // Full sync
			HoverProvider:              true,
			DefinitionProvider:         true,
			CompletionProvider:         nil,   // Disable completion as it's not ready/needed yet
			DocumentFormattingProvider: false, // Disable formatting
		},
	}

	response := ResponseMessage{
		Jsonrpc: "2.0",
		ID:      id,
		Result:  result,
	}

	log.Printf("Sending initialize response")
	return s.sendResponse(response)
}

func (s *LanguageServer) handleShutdown(id interface{}) error {
	response := ResponseMessage{
		Jsonrpc: "2.0",
		ID:      id,
		Result:  nil,
	}

	return s.sendResponse(response)
}
