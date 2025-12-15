package main

import (
	"fmt"
	"io"
	"os"
	"github.com/funvibe/funxy/internal/analyzer"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/backend"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"path/filepath"
	"strings"
)

// BackendType determines the execution backend.
// Can be set at build time using: -ldflags "-X main.BackendType=tree"
// Default is "vm".
var BackendType = "vm"

var moduleCache = make(map[string]evaluator.Object)

// isSourceFile checks if a file has a recognized source extension
func isSourceFile(path string) bool {
	for _, ext := range config.SourceFileExtensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

func getImportName(imp *ast.ImportStatement) string {
	if imp.Alias != nil {
		return imp.Alias.Value
	}
	path := imp.Path.Value
	// heuristic: last part
	_, file := filepath.Split(path)
	return file
}

func resolvePath(baseDir, importPath string) string {
	if strings.HasPrefix(importPath, ".") {
		return filepath.Join(baseDir, importPath)
	}
	abs, _ := filepath.Abs(importPath)
	return abs
}

func evaluateModule(mod *modules.Module, loader *modules.Loader) (evaluator.Object, error) {
	if cached, ok := moduleCache[mod.Dir]; ok {
		return cached, nil
	}

	// Create env for this module
	env := evaluator.NewEnvironment()
	// Register builtins
	for name, builtin := range evaluator.Builtins {
		env.Set(name, builtin)
	}
	evaluator.RegisterBuiltins(env)

	eval := evaluator.New()
	evaluator.RegisterFPTraits(eval, env) // Register FP traits

	// Process imports for this module
	for _, file := range mod.Files {
		for _, imp := range file.Imports {
			absPath := resolvePath(mod.Dir, imp.Path.Value)
			depMod, err := loader.Load(absPath)
			if err != nil {
				return nil, err
			}

			depObj, err := evaluateModule(depMod, loader)
			if err != nil {
				return nil, err
			}

			alias := getImportName(imp)
			env.Set(alias, depObj)
		}
	}

	// Evaluate files
	for _, file := range mod.Files {
		res := eval.Eval(file, env)
		if res != nil && res.Type() == evaluator.ERROR_OBJ {
			return nil, fmt.Errorf("runtime error in %s: %s", mod.Name, res.Inspect())
		}
	}

	// Collect exports
	exports := make(map[string]evaluator.Object)
	for name := range mod.Exports {
		if val, ok := env.Get(name); ok {
			exports[name] = val
		}
	}

	modObj := evaluator.NewRecord(exports)
	moduleCache[mod.Dir] = modObj
	return modObj, nil
}

func runModule(path string) {
	loader := modules.NewLoader()
	mod, err := loader.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading module: %s\n", err)
		os.Exit(1)
	}

	analyzer := analyzer.New(mod.SymbolTable)
	analyzer.SetLoader(loader)
	analyzer.BaseDir = mod.Dir // Set BaseDir for relative import resolution
	analyzer.RegisterBuiltins()

	hasErrors := false
	for _, fileAST := range mod.Files {
		errors := analyzer.Analyze(fileAST)
		if len(errors) > 0 {
			hasErrors = true
			for _, err := range errors {
				fmt.Fprintf(os.Stderr, "- %s\n", err.Error())
			}
		}
	}

	if hasErrors {
		os.Exit(1)
	}

	_, err = evaluateModule(mod, loader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func handleTest() bool {
	if len(os.Args) < 2 {
		return false
	}

	if os.Args[1] != "test" {
		return false
	}

	// Initialize virtual packages
	modules.InitVirtualPackages()

	// Collect test files
	var testFiles []string

	if len(os.Args) == 2 {
		// No files specified - error
		fmt.Fprintf(os.Stderr, "Usage: %s test <file> [file2...]\n", os.Args[0])
		os.Exit(1)
	}

	for _, arg := range os.Args[2:] {
		// Skip flags
		if strings.HasPrefix(arg, "-") {
			continue
		}

		fileInfo, err := os.Stat(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

		if fileInfo.IsDir() {
			// Find all source files in directory
			entries, err := os.ReadDir(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading directory: %s\n", err)
				os.Exit(1)
			}
			for _, entry := range entries {
				if !entry.IsDir() && isSourceFile(entry.Name()) {
					testFiles = append(testFiles, filepath.Join(arg, entry.Name()))
				}
			}
		} else {
			testFiles = append(testFiles, arg)
		}
	}

	if len(testFiles) == 0 {
		fmt.Println("No test files found")
		return true
	}

	useTreeWalk := isTreeWalkMode()

	// Initialize test runner
	// Note: We pass nil to InitTestRunner if using VM, as VM handles execution internally
	// But InitTestRunner expects an evaluator reference.
	// For Tree-walk, we pass 'eval'. For VM, we pass nil (and VM will use its own).
	var eval *evaluator.Evaluator
	if useTreeWalk {
		eval = evaluator.New()
	}
	evaluator.InitTestRunner(eval)

	// Run each test file
	for _, testFile := range testFiles {
		fmt.Printf("\n=== %s ===\n", testFile)
		runTestFile(testFile, useTreeWalk)
	}

	// Print summary
	evaluator.PrintTestSummary()

	// Exit with error if any tests failed
	results := evaluator.GetTestResults()
	for _, r := range results {
		if !r.Passed && !r.Skipped {
			os.Exit(1)
		}
	}

	return true
}

func runTestFile(path string, useTreeWalk bool) {
	sourceCode, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
		return
	}

	// Use absolute path for proper module resolution
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Use unified pipeline logic with test mode enabled
	runPipeline(string(sourceCode), absPath, useTreeWalk, true)
}

func handleHelp() bool {
	if len(os.Args) < 2 {
		return false
	}

	if os.Args[1] != "-help" && os.Args[1] != "--help" && os.Args[1] != "help" {
		return false
	}

	// Initialize virtual packages and documentation
	modules.InitVirtualPackages()

	if len(os.Args) == 2 {
		// General help
		fmt.Print(modules.PrintHelp())
		return true
	}

	arg := os.Args[2]

	if arg == "packages" {
		// List all packages
		fmt.Println("Available packages:")
		fmt.Println()
		for _, pkg := range modules.GetAllDocPackages() {
			fmt.Printf("  %-15s %s\n", pkg.Path, pkg.Description)
		}
		return true
	}

	if arg == "precedence" {
		fmt.Print(modules.PrintPrecedence())
		return true
	}

	if arg == "search" && len(os.Args) > 3 {
		// Search documentation
		term := os.Args[3]
		results := modules.SearchDocs(term)
		if len(results) == 0 {
			fmt.Printf("No results found for '%s'\n", term)
		} else {
			fmt.Printf("Search results for '%s':\n\n", term)
			for _, entry := range results {
				fmt.Print(modules.FormatDocEntry(entry))
			}
		}
		return true
	}

	// Try to find package documentation
	pkg := modules.GetDocPackage(arg)
	if pkg != nil {
		fmt.Print(modules.FormatDocPackage(pkg))
		return true
	}

	// Try with "lib/" prefix
	pkg = modules.GetDocPackage("lib/" + arg)
	if pkg != nil {
		fmt.Print(modules.FormatDocPackage(pkg))
		return true
	}

	fmt.Printf("Unknown topic: %s\n", arg)
	fmt.Println("Use '-help packages' to see available packages")
	return true
}

// isTreeWalkMode returns true if the backend is configured to use Tree-Walk interpreter.
// This is now determined at build time via BackendType variable.
func isTreeWalkMode() bool {
	return BackendType == "tree"
}

// Get args - simply returns os.Args as we don't strip flags anymore
func getArgs() []string {
	return os.Args
}

// Run code using the unified pipeline
func runPipeline(sourceCode string, filePath string, useTreeWalk bool, isTestMode bool) {
	// 1. Create the initial pipeline context
	initialContext := pipeline.NewPipelineContext(sourceCode)
	initialContext.FilePath = filePath
	initialContext.IsTestMode = isTestMode

	// 2. Select backend based on flag
	var execBackend backend.Backend
	if useTreeWalk {
		execBackend = backend.NewTreeWalk()
	} else {
		execBackend = backend.NewVM()
	}

	// 3. Create and configure the processing pipeline
	processingPipeline := pipeline.New(
		&lexer.LexerProcessor{},
		&parser.ParserProcessor{},
		&analyzer.SemanticAnalyzerProcessor{},
		backend.NewExecutionProcessor(execBackend),
	)

	// 4. Run the pipeline
	finalContext := processingPipeline.Run(initialContext)

	// 5. Check the results and print errors
	if len(finalContext.Errors) > 0 {
		fmt.Fprintln(os.Stderr, "Processing failed with errors:")
		for _, err := range finalContext.Errors {
			fmt.Fprintf(os.Stderr, "- %s\n", err.Error())
		}
		// If running a script (not test), exit with error code
		if !isTestMode {
		os.Exit(1)
	}
	}
}

func main() {
	// Catch panics and show user-friendly error
	defer func() {
		if r := recover(); r != nil {
			// Print stack trace for debugging
			if os.Getenv("DEBUG") == "1" {
				panic(r) // Re-panic to get stack trace
			}
			fmt.Fprintf(os.Stderr, "Internal error: %v\n", r)
			fmt.Fprintln(os.Stderr, "This is a bug. Please report it.")
			os.Exit(1)
		}
	}()

	// Handle help first
	if handleHelp() {
		return
	}

	// Handle test command
	if handleTest() {
		return
	}

	useTreeWalk := isTreeWalkMode()
	args := getArgs()

	if len(args) == 2 {
		path := args[1]
		fileInfo, err := os.Stat(path)
		if err == nil && fileInfo.IsDir() {
			if !useTreeWalk {
				fmt.Fprintln(os.Stderr, "VM mode not supported for modules yet, please rebuild with -ldflags \"-X main.BackendType=tree\"")
				os.Exit(1)
			}
			runModule(path)
			return
		}
	}

	sourceCode, err := readInputFromArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
	if sourceCode == "" {
		return // Nothing to do
	}

	filePath := ""
	if len(args) == 2 {
		filePath, _ = filepath.Abs(args[1])
	}

	// Use unified pipeline execution
	runPipeline(sourceCode, filePath, useTreeWalk, false)
}

func readInputFromArgs(args []string) (string, error) {
	var input []byte
	var err error

	switch len(args) {
	case 1:
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return "", fmt.Errorf("Usage: %s <file> or pipe from stdin", args[0])
		}
		input, err = io.ReadAll(os.Stdin)
	case 2:
		filepath := args[1]
		input, err = os.ReadFile(filepath)
	default:
		return "", fmt.Errorf("Usage: %s <file> or pipe from stdin", args[0])
	}

	if err != nil {
		return "", fmt.Errorf("Error reading input: %w", err)
	}

	return string(input), nil
}
