// Package backend provides an interface for different execution backends.
// This allows switching between tree-walk interpreter and VM.
package backend

import (
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/pipeline"
)

// Backend is the interface for execution backends
type Backend interface {
	// Run executes the program from pipeline context and returns the result
	Run(ctx *pipeline.PipelineContext) (evaluator.Object, error)
	
	// Name returns the backend name for display
	Name() string
}
