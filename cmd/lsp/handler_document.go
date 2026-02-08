package main

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/funvibe/funxy/internal/analyzer"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
)

// DocumentState stores the state of a single open document
type DocumentState struct {
	Content string                    // Current file content
	Context *pipeline.PipelineContext // Result of the last analysis (AST, types, symbols)
	Mu      sync.RWMutex              // Mutex to protect access to state
}

func (s *LanguageServer) handleDidOpen(params DidOpenTextDocumentParams) error {
	uri := params.TextDocument.URI
	content := params.TextDocument.Text

	// Create new DocumentState
	docState := &DocumentState{
		Content: content,
	}

	// Analyze the document
	finalCtx := s.analyzeDocument(content, uri)

	// Store the analysis result
	docState.Context = finalCtx

	// Save to documents map
	s.mu.Lock()
	s.documents[uri] = docState
	s.mu.Unlock()

	log.Printf("Opened file: %s", uri)

	// Publish diagnostics
	return s.publishDiagnostics(uri, finalCtx)
}

func (s *LanguageServer) handleDidChange(params DidChangeTextDocumentParams) error {
	// For now, assume full content sync (TextDocumentSyncKind.Full)
	if len(params.ContentChanges) > 0 {
		uri := params.TextDocument.URI
		newContent := params.ContentChanges[0].Text

		// Get document state
		s.mu.RLock()
		docState, exists := s.documents[uri]
		s.mu.RUnlock()

		if !exists {
			return fmt.Errorf("document %s not found", uri)
		}

		// Update content
		docState.Mu.Lock()
		docState.Content = newContent
		docState.Mu.Unlock()

		// Re-analyze the document
		finalCtx := s.analyzeDocument(newContent, uri)
		docState.Mu.Lock()
		docState.Context = finalCtx
		docState.Mu.Unlock()

		log.Printf("Changed file: %s", uri)

		// Publish diagnostics
		return s.publishDiagnostics(uri, finalCtx)
	}
	return nil
}

func (s *LanguageServer) handleDidClose(params DidCloseTextDocumentParams) error {
	s.mu.Lock()
	delete(s.documents, params.TextDocument.URI)
	s.mu.Unlock()
	log.Printf("Closed file: %s", params.TextDocument.URI)
	return nil
}

func (s *LanguageServer) analyzeDocument(content string, uri string) *pipeline.PipelineContext {
	if ctx, ok := s.analyzeModuleDocument(content, uri); ok {
		return ctx
	}

	// Create pipeline context
	ctx := pipeline.NewPipelineContext(content)
	ctx.FilePath = s.uriToPath(uri)

	// Create processing pipeline (lexer -> parser -> analyzer)
	processingPipeline := pipeline.New(
		&lexer.LexerProcessor{},
		&parser.ParserProcessor{},
		&analyzer.SemanticAnalyzerProcessor{},
	)

	// Run pipeline
	finalCtx := processingPipeline.Run(ctx)
	return finalCtx
}

func (s *LanguageServer) uriToPath(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		return strings.TrimPrefix(uri, "file://")
	}
	return uri
}
