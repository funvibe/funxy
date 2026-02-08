package main

import (
	"path/filepath"

	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/pipeline"
)

func (s *LanguageServer) publishDiagnostics(uri string, finalCtx *pipeline.PipelineContext) error {
	// Convert diagnostics to LSP format
	lspDiagnostics := s.convertDiagnostics(finalCtx.Errors, s.uriToPath(uri))

	// Send publishDiagnostics notification
	notification := NotificationMessage{
		Jsonrpc: "2.0",
		Method:  "textDocument/publishDiagnostics",
		Params: PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: lspDiagnostics,
		},
	}

	return s.sendNotification(notification)
}

func (s *LanguageServer) convertDiagnostics(errors []*diagnostics.DiagnosticError, filePath string) []Diagnostic {
	result := make([]Diagnostic, 0)
	targetPath := filepath.Clean(filePath)

	for _, err := range errors {
		if err.File != "" && targetPath != "" {
			if filepath.Clean(err.File) != targetPath {
				continue
			}
		}

		diag := Diagnostic{
			Range: Range{
				Start: Position{
					Line:      err.Token.Line - 1, // LSP uses 0-based indexing
					Character: err.Token.Column - 1,
				},
				End: Position{
					Line:      err.Token.Line - 1,
					Character: err.Token.Column + len(err.Token.Lexeme) - 1,
				},
			},
			Severity: SeverityError,
			Code:     string(err.Code),
			Message:  err.Error(),
			Source:   "funxy",
		}
		result = append(result, diag)
	}

	return result
}
