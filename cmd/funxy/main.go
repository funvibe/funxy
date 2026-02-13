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
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/token"
	"github.com/funvibe/funxy/internal/utils"
	"github.com/funvibe/funxy/internal/vm"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// handleCompile compiles a source file to bytecode (.fbc file).
// Produces a v2 bundle that includes all user module dependencies.
func handleCompile() bool {
	if len(os.Args) < 3 {
		return false
	}

	if os.Args[1] != "-c" && os.Args[1] != "--compile" {
		return false
	}

	sourcePath := os.Args[2]

	bundle, err := compileToBundle(sourcePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Compilation error: %s\n", err)
		os.Exit(1)
	}

	// Serialize to bytes
	data, err := bundle.Serialize()
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

	modCount := len(bundle.Modules)
	fmt.Printf("Compiled %s -> %s (%d bytes", sourcePath, outputPath, len(data))
	if modCount > 0 {
		fmt.Printf(", %d modules bundled", modCount)
	}
	fmt.Println(")")
	return true
}

// compileToBundle compiles a source file and all its dependencies into a Bundle.
func compileToBundle(sourcePath string) (*vm.Bundle, error) {
	// Initialize virtual packages for lib/* import resolution
	modules.InitVirtualPackages()

	// Read source file
	sourceCode, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("reading source file: %w", err)
	}

	absSourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		absSourcePath = sourcePath
	}

	// Run full pipeline (lex, parse, analyze) — this also loads/analyses all imports
	initialContext := pipeline.NewPipelineContext(string(sourceCode))
	initialContext.FilePath = absSourcePath

	processingPipeline := pipeline.New(
		&lexer.LexerProcessor{},
		&parser.ParserProcessor{},
		&analyzer.SemanticAnalyzerProcessor{},
	)

	finalContext := processingPipeline.Run(initialContext)

	if len(finalContext.Errors) > 0 {
		errMsgs := make([]string, len(finalContext.Errors))
		for i, e := range finalContext.Errors {
			errMsgs[i] = e.Error()
		}
		return nil, fmt.Errorf("analysis failed:\n  %s", strings.Join(errMsgs, "\n  "))
	}

	// Get the AST
	program, ok := finalContext.AstRoot.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("internal error: AST root is not a Program")
	}

	// Compile main script
	compiler := vm.NewCompiler()
	compiler.SetBaseDir(filepath.Dir(absSourcePath))
	if finalContext.TypeMap != nil {
		compiler.SetTypeMap(finalContext.TypeMap)
	}
	if finalContext.SymbolTable != nil {
		compiler.SetSymbolTable(finalContext.SymbolTable)
	}
	if finalContext.ResolutionMap != nil {
		compiler.SetResolutionMap(finalContext.ResolutionMap)
	}

	// If the analyzer discovered a multi-file package, compile ALL files
	// (just like CompileAndExecuteModule does for regular execution).
	var chunk *vm.Chunk
	if finalContext.Module != nil {
		if mod, ok := finalContext.Module.(*modules.Module); ok && len(mod.Files) > 1 {
			chunk, err = compiler.CompileModule(mod.OrderedFiles())
			if err != nil {
				return nil, fmt.Errorf("compiling module %s: %w", mod.Name, err)
			}
		}
	}
	if chunk == nil {
		// Single file (no package or single-file package)
		chunk, err = compiler.Compile(program)
		if err != nil {
			return nil, fmt.Errorf("compiling main script: %w", err)
		}
	}
	chunk.File = absSourcePath

	// Project root = CWD at build time. All bundle keys are relative to this.
	projectRoot, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting project root: %w", err)
	}

	// Make SourceFile project-relative so the bundle is portable
	relSourceFile, err := filepath.Rel(projectRoot, absSourcePath)
	if err != nil {
		relSourceFile = absSourcePath // fallback
	}

	// Build the bundle
	bundle := &vm.Bundle{
		MainChunk:  chunk,
		Modules:    make(map[string]*vm.BundledModule),
		SourceFile: relSourceFile,
	}

	// Pre-compile main script's trait defaults
	if finalContext.TraitDefaults != nil && len(finalContext.TraitDefaults) > 0 {
		bundle.TraitDefaults = make(map[string]*vm.CompiledFunction)
		for key, fn := range finalContext.TraitDefaults {
			compiledFn, err := vm.CompileTraitDefault(fn)
			if err != nil {
				// Non-fatal: trait defaults that fail to compile will be missing
				// in bundle mode (user gets runtime error on first use)
				continue
			}
			bundle.TraitDefaults[key] = compiledFn
		}
	}

	// Get the loader from the pipeline context (contains all loaded/analyzed modules)
	var loader *modules.Loader
	if finalContext.Loader != nil {
		loader, _ = finalContext.Loader.(*modules.Loader)
	}

	// Recursively compile all user module imports
	if loader != nil {
		baseDir := filepath.Dir(absSourcePath)
		if err := bundleModulesRecursive(bundle, chunk.PendingImports, loader, baseDir, projectRoot); err != nil {
			return nil, fmt.Errorf("bundling modules: %w", err)
		}
	}

	return bundle, nil
}

// bundleModulesRecursive recursively compiles user modules into the bundle.
// All bundle keys (module paths, Dir, SubModulePaths) are stored as
// project-relative paths so the binary is portable across machines/directories.
func bundleModulesRecursive(bundle *vm.Bundle, imports []vm.PendingImport, loader *modules.Loader, baseDir, projectRoot string) error {
	for _, imp := range imports {
		// Skip virtual modules (lib/*)
		if isVirtualImport(imp.Path) {
			continue
		}

		// Resolve to absolute path (needed for loader.GetModule to find files on disk)
		importPath := imp.Path
		if len(importPath) > 0 && importPath[0] == '.' {
			importPath = filepath.Join(baseDir, importPath)
		}
		absPath, err := filepath.Abs(importPath)
		if err != nil {
			return fmt.Errorf("resolving path %s: %w", imp.Path, err)
		}

		// Bundle key = project-relative path (e.g. "kit/web", not "/Users/.../kit/web")
		bundleKey, err := filepath.Rel(projectRoot, absPath)
		if err != nil {
			bundleKey = imp.Path // fallback to import path as-is
		}

		// Skip if already bundled
		if _, ok := bundle.Modules[bundleKey]; ok {
			continue
		}

		// Get module from loader (should be loaded and analyzed by the pipeline)
		modInterface, err := loader.GetModule(absPath)
		if err != nil {
			return fmt.Errorf("getting module %s: %w", imp.Path, err)
		}

		mod, ok := modInterface.(*modules.Module)
		if !ok {
			return fmt.Errorf("invalid module type for %s", imp.Path)
		}

		if mod.IsVirtual {
			continue
		}

		if mod.IsPackageGroup {
			// Package group: compile each sub-module
			relDir, _ := filepath.Rel(projectRoot, absPath)
			bm := &vm.BundledModule{
				Dir:            relDir,
				IsPackageGroup: true,
				Exports:        exportNamesList(mod),
			}

			// Store trait info for package groups
			if mod.SymbolTable != nil {
				for _, name := range bm.Exports {
					if sym, ok := mod.SymbolTable.Find(name); ok && sym.Kind == symbols.TraitSymbol {
						if bm.Traits == nil {
							bm.Traits = make(map[string][]string)
						}
						bm.Traits[name] = mod.SymbolTable.GetTraitAllMethods(name)
					}
				}
			}

			for _, subMod := range mod.Imports {
				subAbsPath, _ := filepath.Abs(subMod.Dir)
				subKey, _ := filepath.Rel(projectRoot, subAbsPath)

				// Recursively bundle sub-module
				fakeImport := []vm.PendingImport{{Path: subAbsPath, ImportAll: true}}
				if err := bundleModulesRecursive(bundle, fakeImport, loader, subMod.Dir, projectRoot); err != nil {
					return err
				}

				bm.SubModulePaths = append(bm.SubModulePaths, subKey)
			}

			bundle.Modules[bundleKey] = bm
		} else {
			// Regular module: compile all files
			modCompiler := vm.NewCompiler()
			modCompiler.SetBaseDir(mod.Dir)
			if mod.TypeMap != nil {
				modCompiler.SetTypeMap(mod.TypeMap)
			}
			if mod.SymbolTable != nil {
				modCompiler.SetSymbolTable(mod.SymbolTable)
			}

			modChunk, err := modCompiler.CompileModule(mod.OrderedFiles())
			if err != nil {
				return fmt.Errorf("compiling module %s: %w", mod.Name, err)
			}

			relDir, _ := filepath.Rel(projectRoot, mod.Dir)
			bm := &vm.BundledModule{
				Chunk:          modChunk,
				PendingImports: modCompiler.GetPendingImports(),
				Exports:        exportNamesList(mod),
				Dir:            relDir,
			}

			// Store trait info so bundled imports can resolve `import "mod" (TraitName)`
			if mod.SymbolTable != nil {
				for _, name := range bm.Exports {
					if sym, ok := mod.SymbolTable.Find(name); ok && sym.Kind == symbols.TraitSymbol {
						if bm.Traits == nil {
							bm.Traits = make(map[string][]string)
						}
						bm.Traits[name] = mod.SymbolTable.GetTraitAllMethods(name)
					}
				}
			}

			// Pre-compile trait defaults for this module
			if mod.TraitDefaults != nil && len(mod.TraitDefaults) > 0 {
				bm.TraitDefaults = make(map[string]*vm.CompiledFunction)
				for key, fn := range mod.TraitDefaults {
					compiledFn, err := vm.CompileTraitDefault(fn)
					if err != nil {
						continue // Non-fatal
					}
					bm.TraitDefaults[key] = compiledFn
				}
			}

			bundle.Modules[bundleKey] = bm

			// Recurse for this module's own imports
			if err := bundleModulesRecursive(bundle, bm.PendingImports, loader, mod.Dir, projectRoot); err != nil {
				return err
			}
		}
	}
	return nil
}

// exportNamesList returns a sorted list of exported symbol names from a module.
func exportNamesList(mod *modules.Module) []string {
	names := make([]string, 0, len(mod.Exports))
	for name := range mod.Exports {
		names = append(names, name)
	}
	return names
}

// isVirtualImport checks if a path refers to a virtual (built-in) module.
func isVirtualImport(path string) bool {
	return path == "lib" || (len(path) > 4 && path[:4] == "lib/")
}

// handleRunCompiled runs a pre-compiled .fbc bytecode file (v1 or v2 bundle).
func handleRunCompiled() bool {
	if len(os.Args) < 3 {
		return false
	}

	if os.Args[1] != "-r" && os.Args[1] != "--run" {
		return false
	}

	bytecodePath := os.Args[2]

	// Fix os.Args for sysArgs: remove "-r" flag so os.Args[1] is the script path
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

	// Deserialize (handles both v1 and v2 formats)
	bundle, err := vm.DeserializeAny(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Deserialization error: %s\n", err)
		os.Exit(1)
	}

	// Initialize virtual packages for import resolution
	modules.InitVirtualPackages()

	// Set base dir from source file
	if bundle.MainChunk.File != "" {
		bundle.SourceFile = bundle.MainChunk.File
	}

	_, err = vm.RunBundle(bundle)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Runtime error: %s\n", err)
		os.Exit(1)
	}

	return true
}

// handleBuild compiles a source file into a self-contained native binary.
// Usage: funxy build <source> [-o <output>] [--host <binary>]
func handleBuild() bool {
	if len(os.Args) < 3 {
		return false
	}

	if os.Args[1] != "build" {
		return false
	}

	sourcePath := os.Args[2]

	// Parse flags
	outputPath := ""
	hostBinaryPath := ""
	var embedPaths []string
	for i := 3; i < len(os.Args)-1; i++ {
		switch os.Args[i] {
		case "-o":
			outputPath = os.Args[i+1]
			i++
		case "--host":
			hostBinaryPath = os.Args[i+1]
			i++
		case "--embed":
			// Support comma-separated: --embed static,config,data.json
			// But respect brace expansion: --embed "*.{js,html}" stays as one pattern
			for _, p := range splitEmbedArg(os.Args[i+1]) {
				if p != "" {
					embedPaths = append(embedPaths, p)
				}
			}
			i++
		}
	}

	// Default output path: strip extension from source
	if outputPath == "" {
		outputPath = strings.TrimSuffix(sourcePath, filepath.Ext(sourcePath))
		if runtime.GOOS == "windows" || strings.Contains(hostBinaryPath, "windows") {
			outputPath += ".exe"
		}
	}

	fmt.Printf("Compiling %s...\n", sourcePath)

	// Step 1: Compile to bundle
	bundle, err := compileToBundle(sourcePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Compilation error: %s\n", err)
		os.Exit(1)
	}

	// Step 1.5: Collect embedded resources
	if len(embedPaths) > 0 {
		resources := make(map[string][]byte)
		sourceDir := filepath.Dir(sourcePath)
		for _, embedPath := range embedPaths {
			// Embed paths are relative to CWD (not source directory).
			// sourceDir is only used for computing relative resource keys.

			// Check if path contains glob characters (including brace expansion)
			if strings.ContainsAny(embedPath, "*?[{") {
				// Expand brace patterns like *.{js,html} into [*.js, *.html]
				expandedPatterns := expandBraces(embedPath)
				var allMatches []string
				for _, pattern := range expandedPatterns {
					matches, err := filepath.Glob(pattern)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Invalid glob pattern %s: %s\n", embedPath, err)
						os.Exit(1)
					}
					allMatches = append(allMatches, matches...)
				}
				if len(allMatches) == 0 {
					fmt.Fprintf(os.Stderr, "Warning: glob %s matched no files\n", embedPath)
				}
				for _, match := range allMatches {
					if err := collectResources(match, sourceDir, resources); err != nil {
						fmt.Fprintf(os.Stderr, "Error collecting embedded resource %s: %s\n", match, err)
						os.Exit(1)
					}
				}
			} else {
				// For plain paths, resolve relative to source directory for backwards compat
				absEmbed := embedPath
				if !filepath.IsAbs(absEmbed) {
					// First check if path exists as-is (relative to CWD)
					if _, err := os.Stat(absEmbed); err != nil {
						// Try relative to source directory
						absEmbed = filepath.Join(sourceDir, absEmbed)
					}
				}
				if err := collectResources(absEmbed, sourceDir, resources); err != nil {
					fmt.Fprintf(os.Stderr, "Error collecting embedded resource %s: %s\n", embedPath, err)
					os.Exit(1)
				}
			}
		}
		bundle.Resources = resources
		totalSize := 0
		for _, data := range resources {
			totalSize += len(data)
		}
		fmt.Printf("Embedded %d files (%.1f KB)\n", len(resources), float64(totalSize)/1024)
	}

	// Step 2: Determine host binary
	var hostBinary []byte
	if hostBinaryPath != "" {
		// Cross-compilation: use explicitly provided host binary
		hostData, err := os.ReadFile(hostBinaryPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot read host binary %s: %s\n", hostBinaryPath, err)
			os.Exit(1)
		}
		hostSize := vm.GetHostBinarySize(hostData)
		hostBinary = hostData[:hostSize]
	} else {
		// Default: use own executable as host
		selfPath, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot find own executable: %s\n", err)
			os.Exit(1)
		}
		selfPath, err = filepath.EvalSymlinks(selfPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot resolve executable path: %s\n", err)
			os.Exit(1)
		}
		selfData, err := os.ReadFile(selfPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot read own executable: %s\n", err)
			os.Exit(1)
		}
		hostSize := vm.GetHostBinarySize(selfData)
		hostBinary = selfData[:hostSize]
	}

	// Step 3: Pack self-contained binary
	outputData, err := vm.PackSelfContained(hostBinary, bundle)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Packing error: %s\n", err)
		os.Exit(1)
	}

	// Step 4: Write output
	if err := os.WriteFile(outputPath, outputData, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output binary: %s\n", err)
		os.Exit(1)
	}

	// Step 5: Re-sign on macOS (ad-hoc) if output targets macOS
	if runtime.GOOS == "darwin" && !strings.Contains(hostBinaryPath, "linux") &&
		!strings.Contains(hostBinaryPath, "windows") && !strings.Contains(hostBinaryPath, "freebsd") &&
		!strings.Contains(hostBinaryPath, "openbsd") {
		resignBinary(outputPath)
	}

	modCount := len(bundle.Modules)
	fmt.Printf("Built: %s (%.1f MB", outputPath, float64(len(outputData))/(1024*1024))
	if modCount > 0 {
		fmt.Printf(", %d modules", modCount)
	}
	fmt.Println(")")

	return true
}

// collectResources walks a file or directory and collects all files into resources map.
// Keys are paths relative to baseDir.
func collectResources(path string, baseDir string, resources map[string][]byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot stat %s: %w", path, err)
	}

	if !info.IsDir() {
		// Single file
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("cannot read %s: %w", path, err)
		}
		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			relPath = filepath.Base(path)
		}
		// Normalize to forward slashes for cross-platform consistency
		relPath = filepath.ToSlash(relPath)
		resources[relPath] = data
		return nil
	}

	// Directory: walk recursively
	return filepath.Walk(path, func(filePath string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("cannot read %s: %w", filePath, err)
		}
		relPath, err := filepath.Rel(baseDir, filePath)
		if err != nil {
			relPath = filepath.Base(filePath)
		}
		relPath = filepath.ToSlash(relPath)
		resources[relPath] = data
		return nil
	})
}

// splitEmbedArg splits a --embed argument by commas, but respects brace expansion.
// "static,config" -> ["static", "config"]
// "*.{js,html}" -> ["*.{js,html}"]
// "*.{js,html},config" -> ["*.{js,html}", "config"]
func splitEmbedArg(arg string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(arg); i++ {
		switch arg[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				p := strings.TrimSpace(arg[start:i])
				if p != "" {
					parts = append(parts, p)
				}
				start = i + 1
			}
		}
	}
	// Last segment
	p := strings.TrimSpace(arg[start:])
	if p != "" {
		parts = append(parts, p)
	}
	return parts
}

// expandBraces expands shell-style brace patterns into multiple glob patterns.
// "*.{js,html}" -> ["*.js", "*.html"]
// "dir/{a,b}/*.txt" -> ["dir/a/*.txt", "dir/b/*.txt"]
// "*.js" -> ["*.js"] (no braces, returned as-is)
func expandBraces(pattern string) []string {
	openIdx := strings.IndexByte(pattern, '{')
	if openIdx == -1 {
		return []string{pattern}
	}
	closeIdx := strings.IndexByte(pattern[openIdx:], '}')
	if closeIdx == -1 {
		return []string{pattern}
	}
	closeIdx += openIdx

	prefix := pattern[:openIdx]
	suffix := pattern[closeIdx+1:]
	alternatives := strings.Split(pattern[openIdx+1:closeIdx], ",")

	var results []string
	for _, alt := range alternatives {
		// Recursively expand in case of nested braces
		expanded := expandBraces(prefix + alt + suffix)
		results = append(results, expanded...)
	}
	return results
}

// resignBinary re-signs a binary with ad-hoc signature on macOS.
// This is needed because appending data invalidates the original signature.
func resignBinary(path string) {
	// Try to use codesign (available on all macOS)
	cmd := exec.Command("codesign", "--force", "--sign", "-", path)
	cmd.Stderr = nil // Suppress errors (codesign might not be available)
	cmd.Stdout = nil
	_ = cmd.Run() // Best-effort, don't fail if codesign is missing
}

// shouldSkipEmbeddedBundle checks if the first argument is "$", which is the
// escape hatch to switch a self-contained binary into interpreter mode.
// Usage: ./myapp $ -e '1+2'   or   ./myapp $ script.lang
// The "$" is stripped from os.Args so the rest of the CLI works normally.
func shouldSkipEmbeddedBundle() bool {
	if len(os.Args) >= 2 && os.Args[1] == "$" {
		// Remove "$" from args so the interpreter sees clean arguments
		os.Args = append(os.Args[:1], os.Args[2:]...)
		return true
	}
	return false
}

// runEmbeddedBundle checks if this binary has bundled bytecode appended,
// and if so, runs it. Returns true if embedded bundle was found and executed.
func runEmbeddedBundle() bool {
	// Dual-mode: "$" as first arg switches to interpreter mode
	if shouldSkipEmbeddedBundle() {
		return false
	}

	exePath, err := os.Executable()
	if err != nil {
		return false
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return false
	}

	data, err := os.ReadFile(exePath)
	if err != nil {
		return false
	}

	bundle, err := vm.ExtractEmbeddedBundle(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading embedded bytecode: %s\n", err)
		os.Exit(1)
	}
	if bundle == nil {
		return false // No embedded bundle
	}

	// Make sysArgs() consistent with interpreter mode:
	// In interpreter: os.Args = ["funxy", "script.lang", ...user args...]
	//   → sysArgs() = ["script.lang", "--port", "8080"]
	// In bundle:      os.Args = ["./myapp", ...user args...]
	//   → without fix: sysArgs() = ["--port", "8080"]  (missing "script" path)
	// Insert argv[0] at position 1 so sysArgs()[0] = binary name (as typed by user)
	os.Args = append([]string{os.Args[0], os.Args[0]}, os.Args[1:]...)

	// Initialize virtual packages
	modules.InitVirtualPackages()

	_, err = vm.RunBundle(bundle)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Runtime error: %s\n", err)
		os.Exit(1)
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
	// Check for embedded bytecode FIRST — self-contained binaries skip all CLI parsing
	if runEmbeddedBundle() {
		return
	}

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

	// Handle build command (funxy build <source> [-o <output>])
	if handleBuild() {
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

	// Handle -e mode (expression execution)
	if handleEval(debugMode) {
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

// handleEval handles -e flag for expression execution mode
// Supports combined flags: -pe, -le, -lpe, -ple, etc.
func handleEval(debugMode bool) bool {
	// Find -e flag and expression
	var expression string
	flags := evalFlags{}
	found := false

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Handle combined flags like -pe, -le, -lpe, -ple, etc.
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && len(arg) > 1 {
			flagChars := arg[1:]
			hasE := strings.ContainsRune(flagChars, 'e')
			hasP := strings.ContainsRune(flagChars, 'p')
			hasL := strings.ContainsRune(flagChars, 'l')

			if hasE {
				if i+1 >= len(args) {
					fmt.Fprintf(os.Stderr, "Error: -e requires an expression argument\n")
					os.Exit(1)
				}
				expression = args[i+1]
				found = true
				if hasP {
					flags.autoPrint = true
				}
				if hasL {
					flags.lineMode = true
				}
				i++ // skip next arg (the expression)
				continue
			}

			// Standalone -p or -l without -e
			if hasP {
				flags.autoPrint = true
			}
			if hasL {
				flags.lineMode = true
			}
			continue
		}

		switch arg {
		case "-debug", "--debug":
			// already handled
		}
	}

	if !found {
		return false
	}

	useTreeWalk := isTreeWalkMode()

	// Read stdin data (if piped)
	var stdinData string
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// stdin is a pipe, read all data
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading stdin: %s\n", err)
			os.Exit(1)
		}
		stdinData = strings.TrimRight(string(data), "\r\n")
	}

	if flags.lineMode {
		// -l mode: process each line separately, stdin = current line
		lines := strings.Split(stdinData, "\n")
		for i, line := range lines {
			// Skip trailing empty line from final \n
			if i == len(lines)-1 && line == "" {
				continue
			}
			runEvalExpression(expression, line, useTreeWalk, debugMode, flags.autoPrint)
		}
	} else {
		// Normal -e mode
		runEvalExpression(expression, stdinData, useTreeWalk, debugMode, flags.autoPrint)
	}

	return true
}

type evalFlags struct {
	autoPrint bool
	lineMode  bool
}

func runEvalExpression(expression string, stdinData string, useTreeWalk bool, debugMode bool, autoPrint bool) {
	sourceCode := expression
	if autoPrint {
		sourceCode = "print(" + expression + ")"
	}

	// Initialize virtual packages
	modules.InitVirtualPackages()

	// Smart auto-import: scan expression for identifiers, generate needed imports
	sourceCode = addAutoImports(sourceCode)

	// Create pipeline context
	initialContext := pipeline.NewPipelineContext(sourceCode)
	initialContext.FilePath = "<eval>"
	initialContext.IsEvalMode = true
	initialContext.StdinData = &stdinData

	// Select backend
	var execBackend backend.Backend
	if useTreeWalk {
		execBackend = backend.NewTreeWalk()
	} else {
		execBackend = backend.NewVM(debugMode)
	}

	// Create and run pipeline
	processingPipeline := pipeline.New(
		&lexer.LexerProcessor{},
		&parser.ParserProcessor{},
		&analyzer.SemanticAnalyzerProcessor{},
		backend.NewExecutionProcessor(execBackend),
	)

	finalContext := processingPipeline.Run(initialContext)

	if len(finalContext.Errors) > 0 {
		fmt.Fprintln(os.Stderr, "Processing failed with errors:")
		for _, err := range finalContext.Errors {
			fmt.Fprintf(os.Stderr, "- %s\n", err.Error())
		}
		os.Exit(1)
	}
}

// addAutoImports scans source code for identifiers and generates import statements
// for any that match known lib/* module functions
func addAutoImports(sourceCode string) string {
	// Build reverse index: function_name -> "lib/module"
	index := modules.BuildFunctionToModuleIndex()

	// Tokenize the source to find identifiers
	l := lexer.New(sourceCode)
	usedModules := make(map[string]map[string]bool) // module -> set of used names

	for {
		tok := l.NextToken()
		if tok.Type == token.EOF {
			break
		}
		if tok.Type == token.IDENT_LOWER || tok.Type == token.IDENT_UPPER {
			name := tok.Lexeme
			if modulePath, ok := index[name]; ok {
				if usedModules[modulePath] == nil {
					usedModules[modulePath] = make(map[string]bool)
				}
				usedModules[modulePath][name] = true
			}
		}
	}

	if len(usedModules) == 0 {
		return sourceCode
	}

	// Generate import statements
	var imports strings.Builder
	for modulePath, names := range usedModules {
		nameList := make([]string, 0, len(names))
		for name := range names {
			nameList = append(nameList, name)
		}
		imports.WriteString(fmt.Sprintf("import \"%s\" (%s)\n", modulePath, strings.Join(nameList, ", ")))
	}

	return imports.String() + sourceCode
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
