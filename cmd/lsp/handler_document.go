package main

import (
	"context"
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
	Content    string                    // Current file content
	Context    *pipeline.PipelineContext // Result of the last analysis (AST, types, symbols)
	Mu         sync.RWMutex              // Mutex to protect access to state
	CancelFunc context.CancelFunc        // Cancel function for the current analysis
	AnalysisID int                       // ID to track current analysis run
}

func (s *LanguageServer) handleDidOpen(params DidOpenTextDocumentParams) error {
	uri := params.TextDocument.URI
	content := params.TextDocument.Text

	ctx, cancel := context.WithCancel(context.Background())

	// Create new DocumentState
	docState := &DocumentState{
		Content:    content,
		CancelFunc: cancel,
		AnalysisID: 1, // First analysis
	}

	// Save to documents map BEFORE analysis so that didChange can cancel it if needed
	s.mu.Lock()
	s.documents[uri] = docState
	s.mu.Unlock()

	// Analyze the document
	finalCtx := s.analyzeDocument(content, uri, ctx)

	// Check if this analysis was cancelled
	if ctx.Err() != nil {
		return nil // Don't store or publish diagnostics for cancelled analysis
	}

	// Store the analysis result
	docState.Mu.Lock()
	docState.Context = finalCtx
	docState.Mu.Unlock()

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

		// Cancel previous analysis
		docState.Mu.Lock()
		if docState.CancelFunc != nil {
			docState.CancelFunc()
		}

		// Create new context and cancel function for this analysis run
		ctx, cancel := context.WithCancel(context.Background())
		docState.CancelFunc = cancel
		docState.AnalysisID++
		currentAnalysisID := docState.AnalysisID

		// Update content
		docState.Content = newContent
		docState.Mu.Unlock()

		// Re-analyze the document
		finalCtx := s.analyzeDocument(newContent, uri, ctx)

		// Check if this analysis was cancelled by a newer change
		if ctx.Err() != nil {
			return nil // Don't store or publish diagnostics for cancelled analysis
		}

		docState.Mu.Lock()
		// Only update context if we weren't cancelled (double check)
		if docState.AnalysisID == currentAnalysisID {
			docState.Context = finalCtx
		}
		docState.Mu.Unlock()

		log.Printf("Changed file: %s", uri)

		// Publish diagnostics
		return s.publishDiagnostics(uri, finalCtx)
	}
	return nil
}

func (s *LanguageServer) handleDidClose(params DidCloseTextDocumentParams) error {
	uri := params.TextDocument.URI

	s.mu.Lock()
	docState, exists := s.documents[uri]
	if exists {
		docState.Mu.Lock()
		if docState.CancelFunc != nil {
			docState.CancelFunc()
		}
		docState.Mu.Unlock()
		delete(s.documents, uri)
	}
	s.mu.Unlock()

	log.Printf("Closed file: %s", uri)
	return nil
}

func (s *LanguageServer) analyzeDocument(content string, uri string, ctx context.Context) *pipeline.PipelineContext {
	if pipeCtx, ok := s.analyzeModuleDocument(content, uri, ctx); ok {
		return pipeCtx
	}

	// Create pipeline context
	pipeCtx := pipeline.NewPipelineContext(content)
	pipeCtx.Context = ctx
	pipeCtx.FilePath = s.uriToPath(uri)

	// Create processing pipeline (lexer -> parser -> analyzer)
	processingPipeline := pipeline.New(
		&lexer.LexerProcessor{},
		&parser.ParserProcessor{},
		&analyzer.SemanticAnalyzerProcessor{},
	)

	// Run pipeline
	finalCtx := processingPipeline.Run(pipeCtx)
	return finalCtx
}

func (s *LanguageServer) uriToPath(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		return strings.TrimPrefix(uri, "file://")
	}
	return uri
}
