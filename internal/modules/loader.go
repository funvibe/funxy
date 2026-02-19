package modules

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

// detectPackageExtension determines which extension to use for a package.
// Rule: look for a file named like the directory (e.g., mylib/mylib.lang).
// If found, use that extension for all files in the package.
// If not found, use the first recognized extension found.
func detectPackageExtension(dirPath string) string {
	dirName := filepath.Base(dirPath)
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return config.SourceFileExt // default
	}

	// First, look for main file (dirname.ext)
	for _, ext := range config.SourceFileExtensions {
		mainFile := dirName + ext
		for _, f := range files {
			if !f.IsDir() && f.Name() == mainFile {
				return ext
			}
		}
	}

	// Fallback: use first recognized extension found
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		for _, ext := range config.SourceFileExtensions {
			if strings.HasSuffix(f.Name(), ext) {
				return ext
			}
		}
	}

	return config.SourceFileExt // default
}

// hasSourceFiles checks if directory has any source files with given extension
func hasSourceFiles(dirPath string, ext string) bool {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return false
	}
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ext) {
			return true
		}
	}
	return false
}

// hasAnySourceFiles checks if directory has any recognized source files
func hasAnySourceFiles(dirPath string) bool {
	for _, ext := range config.SourceFileExtensions {
		if hasSourceFiles(dirPath, ext) {
			return true
		}
	}
	return false
}

// Loader handles loading modules and their dependencies.
type Loader struct {
	LoadedModules map[string]*Module // Cache of loaded modules by path
	ModulesByName map[string]*Module // Index by package name for quick lookup
	Processing    map[string]bool    // Cycle detection during loading
	GlobalBundle  interface{}        // Optional global bundle for library-only mode (*vm.Bundle)
}

func NewLoader() *Loader {
	// Initialize virtual packages on first loader creation
	InitVirtualPackages()
	return &Loader{
		LoadedModules: make(map[string]*Module),
		ModulesByName: make(map[string]*Module),
		Processing:    make(map[string]bool),
	}
}

// GetModuleByPackageName returns a module by its package name.
// Used for looking up extension methods and trait implementations
// in the module where the type is defined.
// Returns interface{} to avoid circular import with analyzer.LoadedModule.
func (l *Loader) GetModuleByPackageName(name string) interface{} {
	// Check regular modules
	if mod, ok := l.ModulesByName[name]; ok {
		return mod
	}

	// Check virtual packages
	if vp := GetVirtualPackage("lib/" + name); vp != nil {
		if mod, ok := l.LoadedModules["virtual:lib/"+name]; ok {
			return mod
		}
	}

	return nil
}

func (l *Loader) GetModule(path string) (interface{}, error) {
	// Check for virtual packages first (e.g., "lib/list", "lib")
	if vp := GetVirtualPackage(path); vp != nil {
		// Check if already created
		if mod, ok := l.LoadedModules["virtual:"+path]; ok {
			return mod, nil
		}
		mod := vp.CreateVirtualModule()
		// Mark as already analyzed (no code to analyze)
		mod.HeadersAnalyzed = true
		mod.BodiesAnalyzed = true
		l.LoadedModules["virtual:"+path] = mod
		l.ModulesByName[mod.Name] = mod // Index by package name
		return mod, nil
	}

	// Normalize path to absolute for lookup
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	// Check cache
	if mod, ok := l.LoadedModules[absPath]; ok {
		return mod, nil
	}

	// Check if this is a parent directory containing sub-packages
	if mod, err := l.tryLoadPackageGroup(absPath); err == nil && mod != nil {
		return mod, nil
	}

	// Otherwise try loading as regular module
	return l.Load(path)
}

// RegisterBundle registers a global bundle for resolving modules from memory
func (l *Loader) RegisterBundle(b interface{}) {
	l.GlobalBundle = b
}

// tryLoadPackageGroup checks if a directory contains subdirectories with source files
// and creates a combined module from all sub-packages
func (l *Loader) tryLoadPackageGroup(absPath string) (*Module, error) {
	files, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}

	// Check if directory has subdirectories with source files (sub-packages)
	var subPackages []string
	hasDirectFiles := false

	for _, f := range files {
		if f.IsDir() {
			// Check if subdirectory has any source files
			subPath := filepath.Join(absPath, f.Name())
			if hasAnySourceFiles(subPath) {
				subPackages = append(subPackages, f.Name())
			}
		} else if hasAnySourceFiles(absPath) {
			hasDirectFiles = true
		}
	}

	// If no sub-packages found, return nil (will be handled as regular module)
	if len(subPackages) == 0 {
		return nil, nil
	}

	// If has both direct files and sub-packages, treat as regular module
	if hasDirectFiles {
		return nil, nil
	}

	// Create combined module from all sub-packages
	sort.Strings(subPackages)

	combinedMod := &Module{
		Name:           filepath.Base(absPath),
		Dir:            absPath,
		Exports:        make(map[string]bool),
		SymbolTable:    symbols.NewSymbolTable(),
		Imports:        make(map[string]*Module),
		IsVirtual:      false,
		IsPackageGroup: true, // Mark as package group for special handling
		SubPackages:    subPackages,
	}

	// Load each sub-package and combine exports
	for _, subName := range subPackages {
		subPath := filepath.Join(absPath, subName)
		subMod, err := l.Load(subPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load sub-package %s: %v", subName, err)
		}

		// Add sub-module exports to combined module
		for expName := range subMod.Exports {
			combinedMod.Exports[expName] = true
		}

		// Store sub-module reference
		combinedMod.Imports[subName] = subMod
	}

	l.LoadedModules[absPath] = combinedMod
	l.ModulesByName[combinedMod.Name] = combinedMod // Index by package name
	return combinedMod, nil
}

// Load loads a module (and its dependencies) from a given path.
// Path can be absolute or relative.
// If relative, it's relative to the current working directory (initial entry point).
// Dependency loading will be handled recursively.
func (l *Loader) Load(path string) (*Module, error) {
	// Check global bundle first if available
	if l.GlobalBundle != nil {
		// Try to find module in bundle
		// Bundle keys are project-relative paths (e.g. "pkg/math")
		// The input path might be absolute or relative.
		// If it's absolute, try to make it relative to CWD (project root)
		// Or just check if the path matches a key directly (for "pkg/math" imports)

		// 1. Check exact match (e.g. "pkg/math")
		if mod := l.loadFromBundle(path); mod != nil {
			return mod, nil
		}

		// 2. Check relative match if path is absolute
		if filepath.IsAbs(path) {
			if wd, err := os.Getwd(); err == nil {
				if rel, err := filepath.Rel(wd, path); err == nil {
					if mod := l.loadFromBundle(rel); mod != nil {
						return mod, nil
					}
				}
			}
		}
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	// Check cycle
	if l.Processing[absPath] {
		return nil, fmt.Errorf("circular dependency detected loading module: %s", absPath)
	}
	l.Processing[absPath] = true
	defer func() { delete(l.Processing, absPath) }()

	// 1. Load the module files from the directory
	mod, err := l.loadDir(absPath)
	if err != nil {
		return nil, err
	}

	return mod, nil
}

// BundleInterface defines the interface for accessing the bundle without importing vm package
type BundleInterface interface {
	GetModuleExports(key string) ([]string, bool)
	GetModuleTraits(key string) (map[string][]string, bool)
}

// loadFromBundle attempts to load a module from the registered global bundle
func (l *Loader) loadFromBundle(key string) *Module {
	if l.GlobalBundle == nil {
		return nil
	}

	bundle, ok := l.GlobalBundle.(BundleInterface)
	if !ok {
		return nil
	}

	exports, found := bundle.GetModuleExports(key)
	if !found {
		return nil
	}

	// Check cache
	if mod, ok := l.LoadedModules["bundle:"+key]; ok {
		return mod
	}

	// Create Module from BundledModule
	mod := &Module{
		Name:        filepath.Base(key),
		Dir:         key, // Use key as Dir for bundle modules
		Exports:     make(map[string]bool),
		Imports:     make(map[string]*Module),
		SymbolTable: symbols.NewSymbolTable(),
	}

	traits, _ := bundle.GetModuleTraits(key)

	// Populate exports
	for _, exp := range exports {
		mod.Exports[exp] = true
		// Populate symbol table with dummy symbols so analyzer knows they exist
		tVar := typesystem.TVar{Name: "$bundle_" + exp}

		// If it's a trait, mark it
		if traits != nil {
			if _, isTrait := traits[exp]; isTrait {
				mod.SymbolTable.DefinePendingTrait(exp, "")
			} else {
				// tVar is defined in the outer scope (line 322)
				mod.SymbolTable.Define(exp, tVar, "bundle")
			}
		} else {
			mod.SymbolTable.Define(exp, tVar, "bundle")
		}
	}

	// Populate trait info
	if traits != nil {
		for traitName, methods := range traits {
			// Define trait symbol if not already
			if _, ok := mod.SymbolTable.Find(traitName); !ok {
				mod.SymbolTable.DefinePendingTrait(traitName, "")
			}
			// Register trait methods in symbol table
			for _, method := range methods {
				tVar := typesystem.TVar{Name: "$bundle_" + method}
				mod.SymbolTable.Define(method, tVar, "bundle")
				mod.SymbolTable.RegisterTraitMethod(method, traitName, tVar, "bundle")
			}
		}
	}

	l.LoadedModules["bundle:"+key] = mod
	l.ModulesByName[mod.Name] = mod
	return mod
}

// Check cache

// loadDir loads a module from a directory (single pass, no recursion).
// It enforces "one package per directory" with consistent file extension.
// Extension is determined by the main file (dirname.ext) or first found.
func (l *Loader) loadDir(absPath string) (*Module, error) {
	if mod, ok := l.LoadedModules[absPath]; ok {
		return mod, nil
	}

	// Detect which extension to use for this package
	pkgExt := detectPackageExtension(absPath)

	// Use Walk or ReadDir? ReadDir is safer for "one package per dir" as it doesn't recurse.
	files, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}

	var sourceFiles []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), pkgExt) {
			sourceFiles = append(sourceFiles, filepath.Join(absPath, f.Name()))
		}
	}
	// Sort for deterministic processing order
	sort.Strings(sourceFiles)

	if len(sourceFiles) == 0 {
		return nil, fmt.Errorf("no %s files found in %s (detected extension: %s)", strings.Join(config.SourceFileExtensions, "/"), absPath, pkgExt)
	}

	module := &Module{
		Dir:         absPath,
		Exports:     make(map[string]bool),
		Imports:     make(map[string]*Module),
		SymbolTable: symbols.NewSymbolTable(), // Module SymbolTable has builtins
	}

	var packageName string
	var entryFileExportAll bool
	var entryFileExports []string
	var entryFileIndex int = -1

	// Setup parsing components
	// We can't easily reuse pipeline.NewPipelineContext for multiple files merged into one logic context yet.
	// But we can parse each file individually.

	for i, file := range sourceFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}

		// Parse file manually to avoid dependency cycles with processors if any,
		// and to have fine-grained control.

		// Use LexerProcessor to get a buffered TokenStream
		ctx := pipeline.NewPipelineContext(string(content))
		lexerProc := &lexer.LexerProcessor{}
		ctx = lexerProc.Process(ctx)

		pPar := parser.New(ctx.TokenStream, ctx)

		root := pPar.ParseProgram()
		root.File = file

		if len(ctx.Errors) > 0 {
			// Just return first error for now
			err := ctx.Errors[0]
			err.File = file
			return nil, err
		}

		module.Files = append(module.Files, root)

		// Check Package Declaration
		var currentFilePackage string
		var isExportAll bool
		var currentExports []string

		// Look for PackageDeclaration in statements
		// It should be the first statement if present (enforced by parser mostly, but we check AST)
		for _, stmt := range root.Statements {
			if pkgDecl, ok := stmt.(*ast.PackageDeclaration); ok {
				currentFilePackage = pkgDecl.Name.Value
				isExportAll = pkgDecl.ExportAll
				for _, exp := range pkgDecl.Exports {
					// Only local exports (not re-exports) go into currentExports
					if !exp.IsReexport() && exp.Symbol != nil {
						currentExports = append(currentExports, exp.Symbol.Value)
					} else if exp.IsReexport() {
						// Save re-export specs for later resolution in analyzer
						module.ReexportSpecs = append(module.ReexportSpecs, exp)
					}
				}
				break // Only one package decl per file
			}
		}

		if currentFilePackage == "" {
			// No package decl -> treat as "main" (executable script)
			// BUT rule: "Mandatory package declaration for libraries"
			// If we are loading a directory via Import, it MUST have package decl.
			// If we are running a script, it might not.
			// For now, let's assume if ANY file has package decl, ALL must match.
			// If NO file has package decl, we assume it's a script package "main".
			currentFilePackage = "main"
		}

		if packageName == "" {
			packageName = currentFilePackage
		} else {
			if currentFilePackage != packageName {
				return nil, fmt.Errorf("multiple packages in directory %s: found %s and %s", absPath, packageName, currentFilePackage)
			}
		}

		// Check if this is the entry file (packagename.lang)
		baseName := filepath.Base(file)
		expectedEntryName := packageName + pkgExt
		if baseName == expectedEntryName {
			// This is the entry file - save its export spec for processing after all files are loaded
			entryFileIndex = i
			entryFileExportAll = isExportAll
			entryFileExports = currentExports
		}
	}

	// Process exports ONLY from the entry file for the entire package
	// Entry file controls what the whole package exports
	if entryFileIndex >= 0 {
		if entryFileExportAll {
			// (*) means export everything from ALL files in the package
			for _, file := range module.Files {
				for _, stmt := range file.Statements {
					switch n := stmt.(type) {
					case *ast.FunctionStatement:
						module.Exports[n.Name.Value] = true
					case *ast.TypeDeclarationStatement:
						module.Exports[n.Name.Value] = true
						// ADT Constructors are also exported if type is exported
						for _, c := range n.Constructors {
							module.Exports[c.Name.Value] = true
						}
					case *ast.TraitDeclaration:
						module.Exports[n.Name.Value] = true
						// Also export trait methods as they become global functions
						for _, method := range n.Signatures {
							module.Exports[method.Name.Value] = true
						}
					case *ast.InstanceDeclaration:
						// Instances are exported implicitly with their types
					case *ast.ExpressionStatement:
						if assign, ok := n.Expression.(*ast.AssignExpression); ok {
							if ident, ok := assign.Left.(*ast.Identifier); ok {
								module.Exports[ident.Value] = true
							}
						}
					}
				}
			}
		} else {
			// Explicit export list - export specified symbols from ALL files
			for _, exp := range entryFileExports {
				module.Exports[exp] = true
			}

			// Also export constructors for any exported types
			// AND export extension methods for any exported types
			for _, file := range module.Files {
				for _, stmt := range file.Statements {
					if typeDecl, ok := stmt.(*ast.TypeDeclarationStatement); ok {
						// Check if this type is in the export list
						if module.Exports[typeDecl.Name.Value] {
							// Export all constructors of this type
							for _, c := range typeDecl.Constructors {
								module.Exports[c.Name.Value] = true
							}
						}
					}
					// Export extension methods for exported types
					if funcDecl, ok := stmt.(*ast.FunctionStatement); ok {
						if funcDecl.Receiver != nil {
							// This is an extension method
							// Extract the type name from receiver
							if namedType, ok := funcDecl.Receiver.Type.(*ast.NamedType); ok {
								receiverTypeName := namedType.Name.Value
								// If the receiver type is exported, export the method too
								if module.Exports[receiverTypeName] {
									module.Exports[funcDecl.Name.Value] = true
								}
							}
						}
					}
				}
			}
		}
	}

	module.Name = packageName
	l.LoadedModules[absPath] = module
	l.ModulesByName[packageName] = module // Index by package name
	return module, nil
}
