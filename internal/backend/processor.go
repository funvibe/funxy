package backend

import (
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/token"
	"strconv"
	"strings"
)

// ExecutionProcessor implements pipeline.Step to run a Backend
type ExecutionProcessor struct {
	Backend Backend
}

// NewExecutionProcessor creates a new pipeline step for the given backend
func NewExecutionProcessor(b Backend) *ExecutionProcessor {
	return &ExecutionProcessor{Backend: b}
}

func formatInt(n int) string {
	return strconv.Itoa(n)
}

func (p *ExecutionProcessor) Process(ctx *pipeline.PipelineContext) *pipeline.PipelineContext {
	// If previous steps failed, don't run execution
	if ctx.AstRoot == nil || len(ctx.Errors) > 0 {
		return ctx
	}

	result, err := p.Backend.Run(ctx)

	if err != nil {
		// Handle specific backend errors that are already wrapped
		// This tries to mimic the error reporting style of the existing system
		p.handleError(ctx, err)
		return ctx
	}

	// Handle result if it's an error object (runtime error returned as object)
	if result != nil && result.Type() == evaluator.ERROR_OBJ {
		if errObj, ok := result.(*evaluator.Error); ok {
			p.handleEvaluatorError(ctx, errObj)
		} else {
			ctx.Errors = append(ctx.Errors, diagnostics.NewError(
				diagnostics.ErrR001,
				token.Token{},
				result.Inspect(),
			))
		}
	} else if result != nil {
		// Success - typically we might print the result here if it's REPL
		// For script execution, we usually rely on explicit print() calls in the code.
		// However, for completeness, we store the result in the context if needed?
		// Currently pipeline context doesn't have a Result field, but that's fine.
	}

	return ctx
}

func (p *ExecutionProcessor) handleError(ctx *pipeline.PipelineContext, err error) {
	msg := err.Error()
	// Strip "runtime error: " prefix if present, as diagnostics.NewError (ErrR001) might imply it
	// or the test expects it without duplication.
	if strings.HasPrefix(msg, "runtime error: ") {
		msg = strings.TrimPrefix(msg, "runtime error: ")
	}

	// Add as a generic runtime error
	ctx.Errors = append(ctx.Errors, diagnostics.NewError(
		diagnostics.ErrR001,
		token.Token{}, // Location might be missing if it's a generic VM error
		msg,
	))
}

func (p *ExecutionProcessor) handleEvaluatorError(ctx *pipeline.PipelineContext, err *evaluator.Error) {
	tok := token.Token{Line: err.Line, Column: err.Column}
	errMsg := err.Message

	// Add stack trace if available
	if len(err.StackTrace) > 0 {
		errMsg += "\nStack trace:"
		for _, frame := range err.StackTrace {
			file := frame.File
			if file == "" {
				file = ctx.FilePath
			}
			errMsg += "\n  at " + file + ":" + formatInt(frame.Line) + " (called " + frame.Name + ")"
		}
	}

	ctx.Errors = append(ctx.Errors, diagnostics.NewError(
		diagnostics.ErrR001,
		tok,
		errMsg,
	))
}
