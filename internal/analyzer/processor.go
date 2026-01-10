package analyzer

import (
	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/utils"
)

type SemanticAnalyzerProcessor struct{}

func (sap *SemanticAnalyzerProcessor) Process(ctx *pipeline.PipelineContext) *pipeline.PipelineContext {
	if ctx.AstRoot == nil {
		return ctx
	}

	// Register built-in functions (print, typeOf, panic)
	RegisterBuiltins(ctx.SymbolTable)

	// Create loader and store in context for sharing with evaluator
	loader := modules.NewLoader()
	ctx.Loader = loader

	analyzer := New(ctx.SymbolTable)
	analyzer.SetLoader(loader)
	if ctx.FilePath != "" {
		analyzer.BaseDir = utils.GetModuleDir(ctx.FilePath)
	}
	errors := analyzer.Analyze(ctx.AstRoot)

	ctx.TypeMap = analyzer.TypeMap                                     // Export inferred types to context
	ctx.TraitDefaults = analyzer.TraitDefaults                         // Export trait defaults for evaluator
	ctx.OperatorTraits = ctx.SymbolTable.GetAllOperatorTraits()        // Export operator -> trait mappings
	ctx.TraitImplementations = ctx.SymbolTable.GetAllImplementations() // Export trait implementations

	if len(errors) > 0 {
		ctx.Errors = append(ctx.Errors, errors...)
	}

	return ctx
}
