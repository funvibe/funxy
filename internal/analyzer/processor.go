package analyzer

import (
	"path/filepath"
	"strings"

	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/token"
	"github.com/funvibe/funxy/internal/utils"
)

type SemanticAnalyzerProcessor struct{}

func (sap *SemanticAnalyzerProcessor) Process(ctx *pipeline.PipelineContext) *pipeline.PipelineContext {
	if ctx.AstRoot == nil {
		return ctx
	}

	// Create loader and store in context for sharing with evaluator
	loader := modules.NewLoader()
	ctx.Loader = loader

	program, _ := ctx.AstRoot.(*ast.Program)
	if program != nil && isEntryFile(program, ctx.FilePath) {
		if err := sap.analyzeEntryModule(ctx, loader); err != nil {
			if !isMultiplePackagesError(err) {
				ctx.Errors = append(ctx.Errors, err)
				return ctx
			}
		} else {
			return ctx
		}
	}

	// Register built-in functions (print, typeOf, panic)
	RegisterBuiltins(ctx.SymbolTable)

	analyzer := New(ctx.SymbolTable)
	analyzer.SetLoader(loader)
	if ctx.FilePath != "" {
		analyzer.BaseDir = utils.GetModuleDir(ctx.FilePath)
	}
	errors := analyzer.Analyze(ctx.AstRoot)

	ctx.TypeMap = analyzer.TypeMap                                     // Export inferred types to context
	ctx.ResolutionMap = analyzer.ResolutionMap                         // Export resolved symbols to context
	ctx.TraitDefaults = analyzer.TraitDefaults                         // Export trait defaults for evaluator
	ctx.OperatorTraits = ctx.SymbolTable.GetAllOperatorTraits()        // Export operator -> trait mappings
	ctx.TraitImplementations = ctx.SymbolTable.GetAllImplementations() // Export trait implementations

	if len(errors) > 0 {
		ctx.Errors = append(ctx.Errors, errors...)
	}

	return ctx
}

func (sap *SemanticAnalyzerProcessor) analyzeEntryModule(ctx *pipeline.PipelineContext, loader *modules.Loader) *diagnostics.DiagnosticError {
	if ctx.FilePath == "" {
		return nil
	}

	moduleDir := filepath.Dir(ctx.FilePath)
	mod, err := loader.Load(moduleDir)
	if err != nil {
		return diagnostics.NewError(diagnostics.ErrA001, token.Token{}, err.Error())
	}

	ctx.Module = mod
	ctx.SymbolTable = mod.SymbolTable

	RegisterBuiltins(ctx.SymbolTable)

	analyzer := New(ctx.SymbolTable)
	analyzer.SetLoader(loader)
	analyzer.BaseDir = mod.Dir

	orderedFiles := orderModuleFiles(mod.Files, ctx.FilePath)

	var errors []*diagnostics.DiagnosticError
	for _, fileAST := range orderedFiles {
		errors = append(errors, analyzer.AnalyzeNaming(fileAST)...)
	}
	for _, fileAST := range orderedFiles {
		errors = append(errors, analyzer.AnalyzeHeaders(fileAST)...)
	}
	for _, fileAST := range orderedFiles {
		errors = append(errors, analyzer.AnalyzeInstances(fileAST)...)
	}
	for _, fileAST := range orderedFiles {
		errors = append(errors, analyzer.AnalyzeBodies(fileAST)...)
	}

	ctx.TypeMap = analyzer.TypeMap
	ctx.ResolutionMap = analyzer.ResolutionMap
	ctx.TraitDefaults = analyzer.TraitDefaults
	ctx.OperatorTraits = ctx.SymbolTable.GetAllOperatorTraits()
	ctx.TraitImplementations = ctx.SymbolTable.GetAllImplementations()

	mod.SetTypeMap(ctx.TypeMap)
	mod.SetTraitDefaults(ctx.TraitDefaults)

	if len(errors) > 0 {
		ctx.Errors = append(ctx.Errors, errors...)
	}

	return nil
}

func isMultiplePackagesError(err *diagnostics.DiagnosticError) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "multiple packages in directory")
}

func orderModuleFiles(files []*ast.Program, entryFilePath string) []*ast.Program {
	if entryFilePath == "" {
		return files
	}

	ordered := make([]*ast.Program, 0, len(files))
	var entry *ast.Program
	for _, file := range files {
		if file.File == entryFilePath {
			entry = file
			continue
		}
		ordered = append(ordered, file)
	}
	if entry != nil {
		ordered = append(ordered, entry)
	}
	return ordered
}

func isEntryFile(program *ast.Program, filePath string) bool {
	if program == nil || filePath == "" {
		return false
	}

	pkgName := ""
	for _, stmt := range program.Statements {
		if pkgDecl, ok := stmt.(*ast.PackageDeclaration); ok && pkgDecl.Name != nil {
			pkgName = pkgDecl.Name.Value
			break
		}
	}
	if pkgName == "" {
		return false
	}

	ext := filepath.Ext(filePath)
	if ext == "" {
		return false
	}

	base := filepath.Base(filePath)
	return base == pkgName+ext
}
