package backend

import (
	"fmt"
	"os"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/vm"
	"path/filepath"
)

// VMBackend executes programs using the bytecode VM
type VMBackend struct {
	debugMode bool
}

// NewVM creates a new VM backend
func NewVM(debugMode ...bool) *VMBackend {
	debug := false
	if len(debugMode) > 0 {
		debug = debugMode[0]
	}
	return &VMBackend{debugMode: debug}
}

// Run compiles and executes the program using the VM
func (b *VMBackend) Run(ctx *pipeline.PipelineContext) (evaluator.Object, error) {
	if ctx.AstRoot == nil {
		return nil, fmt.Errorf("no AST to compile")
	}

	if ctx.Module != nil {
		if mod, ok := ctx.Module.(*modules.Module); ok {
			return b.runModule(ctx, mod)
		}
		return nil, fmt.Errorf("invalid module in context")
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
	// Pass TypeMap from analyzer to compiler for nominal record types
	if ctx.TypeMap != nil {
		compiler.SetTypeMap(ctx.TypeMap)
	}
	// Pass SymbolTable and ResolutionMap for trait dispatch strategy
	if ctx.SymbolTable != nil {
		compiler.SetSymbolTable(ctx.SymbolTable)
	}
	if ctx.ResolutionMap != nil {
		compiler.SetResolutionMap(ctx.ResolutionMap)
	}
	chunk, err := compiler.Compile(program)
	if err != nil {
		return nil, fmt.Errorf("compilation error: %w", err)
	}

	// Set file path in chunk for debugging
	if ctx.FilePath != "" {
		chunk.File = ctx.FilePath
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
		machine.SetCurrentFile(ctx.FilePath)
	}

	// Inject stdin variable for -e mode
	if ctx.IsEvalMode && ctx.StdinData != nil {
		machine.SetGlobal("stdin", evaluator.StringToList(*ctx.StdinData))
	}

	// Process imports collected during compilation
	pendingImports := compiler.GetPendingImports()
	if err := machine.ProcessImports(pendingImports); err != nil {
		return nil, fmt.Errorf("import error: %w", err)
	}

	// Enable debugger if debug mode is on
	if b.debugMode {
		machine.EnableDebugger()
		debugger := machine.GetDebugger()
		cli := vm.NewDebuggerCLI(debugger, machine)
		cli.SetInput(os.Stdin)
		cli.SetOutput(os.Stdout)
		cli.Run() // Initialize CLI (sets up OnStop callback and prints welcome message)

		// Start in step mode to stop at first line
		// This allows user to set breakpoints before continuing
		debugger.Step()
	}

	// Execute bytecode
	result, err := machine.Run(chunk)
	if err != nil {
		// Return runtime error as is (ExecutionProcessor will handle formatting)
		return nil, err
	}

	return result, nil
}

func (b *VMBackend) runModule(ctx *pipeline.PipelineContext, mod *modules.Module) (evaluator.Object, error) {
	machine := vm.New()
	machine.RegisterBuiltins()
	machine.RegisterFPTraits()

	if ctx.IsTestMode {
		for name, b := range evaluator.TestBuiltins() {
			machine.SetGlobal(name, b)
		}
	}

	machine.SetTraitDefaults(ctx.TraitDefaults)

	if ctx.Loader != nil {
		if loader, ok := ctx.Loader.(*modules.Loader); ok {
			machine.SetLoader(loader)
		}
	} else {
		machine.SetLoader(modules.NewLoader())
	}

	machine.SetBaseDir(mod.Dir)
	if ctx.FilePath != "" {
		machine.SetCurrentFile(ctx.FilePath)
	}

	return machine.CompileAndExecuteModule(mod)
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
	if ctx.TypeMap != nil {
		compiler.SetTypeMap(ctx.TypeMap)
	}
	chunk, err := compiler.Compile(program)
	if err != nil {
		return "", fmt.Errorf("compilation error: %w", err)
	}

	return vm.Disassemble(chunk, "main"), nil
}
