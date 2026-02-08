package main

import (
	"fmt"
	"github.com/funvibe/funxy/internal/analyzer"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/backend"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/utils"
	"github.com/funvibe/funxy/internal/vm"
	"io"
	"os"
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
	evaluator.RegisterBasicTraits(eval, env)    // Register basic traits (Eq, Ord, etc.)
	evaluator.RegisterStandardTraits(eval, env) // Register Show (and potentially others)
	evaluator.RegisterFPTraits(eval, env)       // Register FP traits
	evaluator.RegisterDictionaryGlobals(eval, env)
	if mod.TraitDefaults != nil {
		eval.TraitDefaults = mod.TraitDefaults
	}

	// Process imports for this module
	for _, file := range mod.Files {
		for _, imp := range file.Imports {
			pathToCheck := utils.ResolveImportPath(mod.Dir, imp.Path.Value)
			modInterface, err := loader.GetModule(pathToCheck)
			if err != nil {
				return nil, err
			}

			depMod, ok := modInterface.(*modules.Module)
			if !ok {
				return nil, fmt.Errorf("invalid module type for %s", imp.Path.Value)
			}

			var depObj evaluator.Object
			if depMod.IsVirtual {
				builtins := evaluator.GetVirtualModuleBuiltins(depMod.Name)
				if builtins == nil {
					return nil, fmt.Errorf("unknown virtual module: %s", depMod.Name)
				}
				fields := make(map[string]evaluator.Object)
				for name, fn := range builtins {
					fields[name] = fn
				}
				rec := evaluator.NewRecord(fields)
				rec.ModuleName = depMod.Name
				depObj = rec
			} else if depMod.IsPackageGroup {
				exports := make(map[string]evaluator.Object)
				for _, subMod := range depMod.Imports {
					subObj, err := evaluateModule(subMod, loader)
					if err != nil {
						return nil, err
					}
					if rec, ok := subObj.(*evaluator.RecordInstance); ok {
						for _, field := range rec.Fields {
							exports[field.Key] = field.Value
						}
					}
				}
				depObj = evaluator.NewRecord(exports)
			} else {
				depObj, err = evaluateModule(depMod, loader)
				if err != nil {
					return nil, err
				}
			}
			alias := getImportName(imp)
			env.Set(alias, depObj)
		}
	}

	// Evaluate files in dependency-aware order
	for _, file := range mod.OrderedFiles() {
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
	var errors []*diagnostics.DiagnosticError
	for _, fileAST := range mod.OrderedFiles() {
		errors = append(errors, analyzer.AnalyzeNaming(fileAST)...)
	}
	for _, fileAST := range mod.OrderedFiles() {
		errors = append(errors, analyzer.AnalyzeHeaders(fileAST)...)
	}
	for _, fileAST := range mod.OrderedFiles() {
		errors = append(errors, analyzer.AnalyzeInstances(fileAST)...)
	}
	for _, fileAST := range mod.OrderedFiles() {
		errors = append(errors, analyzer.AnalyzeBodies(fileAST)...)
	}

	if len(errors) > 0 {
		hasErrors = true
		for _, err := range errors {
			fmt.Fprintf(os.Stderr, "- %s\n", err.Error())
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

	// Test mode flag is already set in main()

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
	runPipeline(string(sourceCode), absPath, useTreeWalk, true, false)
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

// handleCompile compiles a source file to bytecode (.fbc file)
func handleCompile() bool {
	if len(os.Args) < 3 {
		return false
	}

	if os.Args[1] != "-c" && os.Args[1] != "--compile" {
		return false
	}

	sourcePath := os.Args[2]

	// Read source file
	sourceCode, err := os.ReadFile(sourcePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading source file: %s\n", err)
		os.Exit(1)
	}

	// Use pipeline to get to AST
	initialContext := pipeline.NewPipelineContext(string(sourceCode))
	initialContext.FilePath = sourcePath

	processingPipeline := pipeline.New(
		&lexer.LexerProcessor{},
		&parser.ParserProcessor{},
		&analyzer.SemanticAnalyzerProcessor{},
	)

	finalContext := processingPipeline.Run(initialContext)

	if len(finalContext.Errors) > 0 {
		fmt.Fprintln(os.Stderr, "Compilation failed with errors:")
		for _, err := range finalContext.Errors {
			fmt.Fprintf(os.Stderr, "- %s\n", err.Error())
		}
		os.Exit(1)
	}

	// Get the AST
	program, ok := finalContext.AstRoot.(*ast.Program)
	if !ok {
		fmt.Fprintf(os.Stderr, "Internal error: AST root is not a Program\n")
		os.Exit(1)
	}

	// Compile to bytecode
	compiler := vm.NewCompiler()
	compiler.SetBaseDir(filepath.Dir(sourcePath))
	chunk, err := compiler.Compile(program)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Compilation error: %s\n", err)
		os.Exit(1)
	}

	// Set source file info
	chunk.File = sourcePath

	// Serialize to bytes
	data, err := chunk.Serialize()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Serialization error: %s\n", err)
		os.Exit(1)
	}

	// Determine output path
	outputPath := strings.TrimSuffix(sourcePath, filepath.Ext(sourcePath)) + ".fbc"

	// Write to file
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing bytecode file: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Compiled %s -> %s\n", sourcePath, outputPath)
	fmt.Printf("Bytecode size: %d bytes\n", len(data))
	return true
}

// handleRunCompiled runs a pre-compiled .fbc bytecode file
func handleRunCompiled() bool {
	if len(os.Args) < 3 {
		return false
	}

	if os.Args[1] != "-r" && os.Args[1] != "--run" {
		return false
	}

	bytecodePath := os.Args[2]

	// Fix os.Args for sysArgs: remove "-r" flag so os.Args[1] is the script path
	// This ensures sysArgs() returns [scriptPath, args...] consistent with source execution
	// We do this by constructing a new slice to avoid modifying the underlying array in a way that might affect other things
	newArgs := make([]string, 0, len(os.Args)-1)
	newArgs = append(newArgs, os.Args[0])
	newArgs = append(newArgs, os.Args[2:]...)
	os.Args = newArgs

	// Read bytecode file
	data, err := os.ReadFile(bytecodePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading bytecode file: %s\n", err)
		os.Exit(1)
	}

	// Deserialize
	chunk, err := vm.Deserialize(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Deserialization error: %s\n", err)
		os.Exit(1)
	}

	// Initialize VM
	machine := vm.New()
	machine.RegisterBuiltins()
	machine.RegisterFPTraits()

	// Set up file info for error messages
	if chunk.File != "" {
		machine.SetCurrentFile(filepath.Base(chunk.File))
		// Set base directory for import resolution
		machine.SetBaseDir(filepath.Dir(chunk.File))
	}

	// Set up module loader
	loader := modules.NewLoader()
	machine.SetLoader(loader)

	// Process imports before running
	if len(chunk.PendingImports) > 0 {
		if err := machine.ProcessImports(chunk.PendingImports); err != nil {
			fmt.Fprintf(os.Stderr, "Import error: %s\n", err)
			os.Exit(1)
		}
	}

	// Execute
	result, err := machine.Run(chunk)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Runtime error: %s\n", err)
		os.Exit(1)
	}

	// Print result if not nil
	if result != nil && result.Type() != evaluator.NIL_OBJ {
		fmt.Println(result.Inspect())
	}

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
func runPipeline(sourceCode string, filePath string, useTreeWalk bool, isTestMode bool, debugMode bool) {
	// 1. Create the initial pipeline context
	initialContext := pipeline.NewPipelineContext(sourceCode)
	initialContext.FilePath = filePath
	initialContext.IsTestMode = isTestMode

	// 2. Select backend based on flag
	var execBackend backend.Backend
	if useTreeWalk {
		if debugMode {
			fmt.Fprintln(os.Stderr, "Warning: Debug mode is only supported with VM backend. Use VM backend (default) for debugging.")
		}
		execBackend = backend.NewTreeWalk()
	} else {
		execBackend = backend.NewVM(debugMode)
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

	// Set test mode flag once at startup if:
	// 1. First argument is "test" (handled by handleTest)
	// 2. Environment variable FUNXY_TEST_MODE is set (for go test runs)
	if len(os.Args) >= 2 && os.Args[1] == "test" {
		config.IsTestMode = true
	} else if os.Getenv("FUNXY_TEST_MODE") == "1" {
		config.IsTestMode = true
	}

	// Check for debug flag
	debugMode := false
	args := os.Args[1:]
	for _, arg := range args {
		if arg == "-debug" || arg == "--debug" {
			debugMode = true
			break
		}
	}

	// Handle help first
	if handleHelp() {
		return
	}

	// Handle test command
	if handleTest() {
		return
	}

	// Handle compile mode (-c or --compile)
	if handleCompile() {
		return
	}

	// Handle run compiled mode (-r or --run)
	if handleRunCompiled() {
		return
	}

	useTreeWalk := isTreeWalkMode()

	// Restore args for the script:
	// - keep all script flags/args
	// - remove host-only flags (debug)
	// - ensure the file path is at argv[1]
	var fileArg string
	var restArgs []string
	for _, arg := range os.Args[1:] {
		if arg == "-debug" || arg == "--debug" {
			continue
		}
		if fileArg == "" && !strings.HasPrefix(arg, "-") {
			fileArg = arg
			continue
		}
		restArgs = append(restArgs, arg)
	}
	if fileArg != "" {
		os.Args = append([]string{os.Args[0], fileArg}, restArgs...)
	} else {
		os.Args = []string{os.Args[0]}
	}
	args = getArgs()

	if len(args) >= 2 {
		path := args[1]
		fileInfo, err := os.Stat(path)
		if err == nil && fileInfo.IsDir() {
			if !useTreeWalk {
				// In VM mode, resolve directory to its entry file and run via pipeline.
				dirBase := filepath.Base(path)
				entryFile := ""
				for _, ext := range config.SourceFileExtensions {
					candidate := filepath.Join(path, dirBase+ext)
					if _, err := os.Stat(candidate); err == nil {
						entryFile = candidate
						break
					}
				}
				if entryFile == "" {
					fmt.Fprintf(os.Stderr, "Entry file not found for package directory: %s\n", path)
					os.Exit(1)
				}
				// Rewrite args to point at the entry file.
				args[1] = entryFile
				os.Args = append([]string{os.Args[0], entryFile}, os.Args[2:]...)
				args = getArgs()
				fileInfo, err = os.Stat(entryFile)
				_ = fileInfo
				_ = err
			} else {
				runModule(path)
				return
			}
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
	if len(args) >= 2 {
		filePath, _ = filepath.Abs(args[1])
	}

	// Use unified pipeline execution
	runPipeline(sourceCode, filePath, useTreeWalk, false, debugMode)
}

func readInputFromArgs(args []string) (string, error) {
	var input []byte
	var err error

	if len(args) == 1 {
		// Read from stdin
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return "", fmt.Errorf("Usage: %s <file> or pipe from stdin", args[0])
		}
		input, err = io.ReadAll(os.Stdin)
	} else if len(args) >= 2 {
		// Read from file
		path := args[1]
		input, err = os.ReadFile(path)
	}

	if err != nil {
		return "", fmt.Errorf("Error reading input: %w", err)
	}

	return string(input), nil
}
