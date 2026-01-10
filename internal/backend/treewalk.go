package backend

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/internal/pipeline"
	"path/filepath"
)

// TreeWalkBackend wraps the existing tree-walk interpreter
type TreeWalkBackend struct{}

// NewTreeWalk creates a new tree-walk backend
func NewTreeWalk() *TreeWalkBackend {
	return &TreeWalkBackend{}
}

// Run executes the program using tree-walk interpretation
func (b *TreeWalkBackend) Run(ctx *pipeline.PipelineContext) (evaluator.Object, error) {
	if ctx.AstRoot == nil {
		return nil, fmt.Errorf("no AST to execute")
	}

	if len(ctx.Errors) > 0 {
		return nil, ctx.Errors[0]
	}

	eval := evaluator.New()

	// Use shared loader from analyzer
	if loader, ok := ctx.Loader.(*modules.Loader); ok {
		eval.SetLoader(loader)
	} else {
		eval.SetLoader(modules.NewLoader())
	}

	eval.TraitDefaults = ctx.TraitDefaults
	eval.OperatorTraits = ctx.OperatorTraits
	eval.TypeMap = ctx.TypeMap

	// Set BaseDir and CurrentFile from ctx.FilePath
	if ctx.FilePath != "" {
		dir := filepath.Dir(ctx.FilePath)
		eval.BaseDir = dir
		eval.CurrentFile = ctx.FilePath // Keep full path for stack traces
	} else {
		eval.BaseDir = "."
		eval.CurrentFile = "<stdin>"
	}

	env := evaluator.NewEnvironment()
	evaluator.RegisterBuiltins(env)
	evaluator.RegisterBasicTraits(eval, env)    // Register basic traits
	evaluator.RegisterStandardTraits(eval, env) // Register Show
	evaluator.RegisterFPTraits(eval, env)
	evaluator.RegisterDictionaryGlobals(eval, env)
	eval.RegisterExtensionMethods()
	eval.GlobalEnv = env

	result := eval.Eval(ctx.AstRoot, env)

	// Check for runtime errors
	if result != nil && result.Type() == evaluator.ERROR_OBJ {
		return nil, fmt.Errorf("%s", result.Inspect())
	}

	return result, nil
}

// Name returns the backend name
func (b *TreeWalkBackend) Name() string {
	return "tree-walk"
}

// RunProgram is a convenience method that takes a Program directly
func (b *TreeWalkBackend) RunProgram(program *ast.Program, loader *modules.Loader) (evaluator.Object, error) {
	eval := evaluator.New()
	eval.SetLoader(loader)
	eval.BaseDir = "."
	eval.CurrentFile = "<stdin>"

	env := evaluator.NewEnvironment()
	evaluator.RegisterBuiltins(env)
	evaluator.RegisterBasicTraits(eval, env)    // Register basic traits
	evaluator.RegisterStandardTraits(eval, env) // Register Show
	evaluator.RegisterFPTraits(eval, env)
	evaluator.RegisterDictionaryGlobals(eval, env)
	eval.RegisterExtensionMethods()
	eval.GlobalEnv = env

	result := eval.Eval(program, env)

	if result != nil && result.Type() == evaluator.ERROR_OBJ {
		return nil, fmt.Errorf("%s", result.Inspect())
	}

	return result, nil
}
