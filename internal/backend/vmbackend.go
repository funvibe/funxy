package backend

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/vm"
	"path/filepath"
)

// VMBackend executes programs using the bytecode VM
type VMBackend struct{}

// NewVM creates a new VM backend
func NewVM() *VMBackend {
	return &VMBackend{}
}

// Run compiles and executes the program using the VM
func (b *VMBackend) Run(ctx *pipeline.PipelineContext) (evaluator.Object, error) {
	if ctx.AstRoot == nil {
		return nil, fmt.Errorf("no AST to compile")
	}

	// Type assert to *ast.Program
	program, ok := ctx.AstRoot.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("AST root is not a Program: %T", ctx.AstRoot)
	}

	// Compile AST to bytecode
	compiler := vm.NewCompiler()
	if ctx.FilePath != "" {
		compiler.SetBaseDir(filepath.Dir(ctx.FilePath))
	}
	chunk, err := compiler.Compile(program)
	if err != nil {
		return nil, fmt.Errorf("compilation error: %w", err)
	}

	// Initialize VM
	machine := vm.New()
	machine.RegisterBuiltins()
	machine.RegisterFPTraits()

	// Register test builtins if in test mode
	if ctx.IsTestMode {
		for name, b := range evaluator.TestBuiltins() {
			machine.SetGlobal(name, b)
		}
	}

	machine.SetTypeAliases(compiler.GetTypeAliases())
	machine.SetTraitDefaults(ctx.TraitDefaults)

	// Set up module loading
	if ctx.Loader != nil {
		if loader, ok := ctx.Loader.(*modules.Loader); ok {
			machine.SetLoader(loader)
		}
	} else {
		// Fallback to new loader
		machine.SetLoader(modules.NewLoader())
	}

	if ctx.FilePath != "" {
		machine.SetBaseDir(filepath.Dir(ctx.FilePath))
		machine.SetCurrentFile(filepath.Base(ctx.FilePath))
	}

	// Process imports collected during compilation
	pendingImports := compiler.GetPendingImports()
	if err := machine.ProcessImports(pendingImports); err != nil {
		return nil, fmt.Errorf("import error: %w", err)
	}

	// Execute bytecode
	result, err := machine.Run(chunk)
	if err != nil {
		// Return runtime error as is (ExecutionProcessor will handle formatting)
		return nil, err
	}

	return result, nil
}

// Name returns the backend name
func (b *VMBackend) Name() string {
	return "vm"
}

// Disassemble returns the bytecode disassembly for debugging
func (b *VMBackend) Disassemble(ctx *pipeline.PipelineContext) (string, error) {
	if ctx.AstRoot == nil {
		return "", fmt.Errorf("no AST to compile")
	}

	program, ok := ctx.AstRoot.(*ast.Program)
	if !ok {
		return "", fmt.Errorf("AST root is not a Program: %T", ctx.AstRoot)
	}

	compiler := vm.NewCompiler()
	if ctx.FilePath != "" {
		compiler.SetBaseDir(filepath.Dir(ctx.FilePath))
	}
	chunk, err := compiler.Compile(program)
	if err != nil {
		return "", fmt.Errorf("compilation error: %w", err)
	}

	return vm.Disassemble(chunk, "main"), nil
}
