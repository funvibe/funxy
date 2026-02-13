package backend

import (
	"context"
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

	if ctx.Module != nil {
		if mod, ok := ctx.Module.(*modules.Module); ok {
			result, err := eval.EvaluateModule(mod)
			if err != nil {
				return nil, err
			}
			return result, nil
		}
		return nil, fmt.Errorf("invalid module in context")
	}

	env := evaluator.NewEnvironment()
	env.SymbolTable = ctx.SymbolTable // Share SymbolTable with evaluator for dispatch strategies
	evaluator.RegisterBuiltins(env)
	evaluator.RegisterBasicTraits(eval, env)    // Register basic traits
	evaluator.RegisterStandardTraits(eval, env) // Register Show
	evaluator.RegisterFPTraits(eval, env)
	evaluator.RegisterDictionaryGlobals(eval, env)
	eval.RegisterExtensionMethods()
	eval.GlobalEnv = env

	// Inject stdin variable for -e mode
	if ctx.IsEvalMode && ctx.StdinData != nil {
		env.Set("stdin", evaluator.StringToList(*ctx.StdinData))
	}

	// Push initial stack frame for the script/program to match VM behavior
	programName := "<main>"
	var programFile string
	if ctx.FilePath != "" {
		programFile = ctx.FilePath
		// Use filename without extension as program name if possible, or just the path
		programName = ctx.FilePath
		if idx := len(filepath.Ext(programName)); idx > 0 {
			programName = programName[:len(programName)-idx]
		}
	} else {
		programFile = eval.CurrentFile
	}
	// Initial frame starts at line 1
	eval.PushCall(programName, programFile, 1, 0)

	result := eval.Eval(ctx.AstRoot, env)

	// Check for runtime errors
	if result != nil && result.Type() == evaluator.ERROR_OBJ {
		err := result.(*evaluator.Error)

		// Stack Trace Compatibility with VM:
		// VM does not include the top-level script frame in the stack trace
		// when an error occurs inside a function call. It only shows the script frame
		// if the error occurs directly in the script.
		// TreeWalk explicitly pushes a frame for the script, so we must filter it out
		// if there are deeper frames, to match the VM output expected by tests.

		if len(err.StackTrace) > 1 {
			// Stack is [Main, ..., Inner]
			// We want to remove Main (index 0) if it's the script frame.
			// The VM hides the top-level script frame when inside a function call.
			bottomFrame := err.StackTrace[0]

			// Check if the bottom frame is our script frame
			if bottomFrame.Name == programName || bottomFrame.Name == "<main>" ||
				bottomFrame.Name == filepath.Base(programName) {
				err.StackTrace = err.StackTrace[1:]
			}
		}

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

	// Push initial stack frame
	eval.PushCall("<main>", eval.CurrentFile, 1, 0)

	result := eval.Eval(program, env)

	if result != nil && result.Type() == evaluator.ERROR_OBJ {
		if err, ok := result.(*evaluator.Error); ok {
			err.Column = 0
		}
		return nil, fmt.Errorf("%s", result.Inspect())
	}

	return result, nil
}

// RunProgramWithContext runs the program with a context for cancellation
func (b *TreeWalkBackend) RunProgramWithContext(ctx context.Context, program *ast.Program, loader *modules.Loader) (evaluator.Object, error) {
	eval := evaluator.New()
	eval.Context = ctx
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

	// Push initial stack frame
	eval.PushCall("<main>", eval.CurrentFile, 1, 0)

	result := eval.Eval(program, env)

	if result != nil && result.Type() == evaluator.ERROR_OBJ {
		if err, ok := result.(*evaluator.Error); ok {
			err.Column = 0
		}
		return nil, fmt.Errorf("%s", result.Inspect())
	}

	return result, nil
}
