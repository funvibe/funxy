package evaluator

import (
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/token"
	"path/filepath"
	"strconv"
)

func formatInt(n int) string {
	return strconv.Itoa(n)
}

type EvaluatorProcessor struct{}

func (ep *EvaluatorProcessor) Process(ctx *pipeline.PipelineContext) *pipeline.PipelineContext {
	if ctx.AstRoot == nil || len(ctx.Errors) > 0 {
		return ctx
	}

	eval := New()
	// Use shared loader from analyzer (with type assertion)
	if loader, ok := ctx.Loader.(*modules.Loader); ok {
		eval.SetLoader(loader)
	} else {
		eval.SetLoader(modules.NewLoader())
	}
	eval.TraitDefaults = ctx.TraitDefaults   // Pass trait defaults from analyzer
	eval.OperatorTraits = ctx.OperatorTraits // Pass operator -> trait mappings
	eval.TypeMap = ctx.TypeMap               // Pass inferred types from analyzer

	// Set BaseDir and CurrentFile from ctx.FilePath
	if ctx.FilePath != "" {
		dir := filepath.Dir(ctx.FilePath)
		eval.BaseDir = dir
		eval.CurrentFile = filepath.Base(ctx.FilePath)
	} else {
		eval.BaseDir = "."
		eval.CurrentFile = "<stdin>"
	}

	env := NewEnvironment()
	RegisterBuiltins(env)
	RegisterFPTraits(eval, env) // Register FP traits (Semigroup, Monoid, Functor, Applicative, Monad)
	eval.GlobalEnv = env        // Store for default implementations

	result := eval.Eval(ctx.AstRoot, env)
	if result != nil && result.Type() == ERROR_OBJ {
		// Convert evaluator error to diagnostic error with location and stack trace
		if err, ok := result.(*Error); ok {
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
		} else {
			ctx.Errors = append(ctx.Errors, diagnostics.NewError(
				diagnostics.ErrR001,
				token.Token{},
				result.Inspect(),
			))
		}
	}

	return ctx
}
