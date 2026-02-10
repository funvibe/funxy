package main

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"

	"github.com/funvibe/funxy/internal/analyzer"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/token"
	"github.com/funvibe/funxy/internal/utils"
)

func (s *LanguageServer) analyzeModuleDocument(content string, uri string) (*pipeline.PipelineContext, bool) {
	ctx := pipeline.NewPipelineContext(content)
	ctx.FilePath = s.uriToPath(uri)
	if ctx.FilePath == "" {
		return nil, false
	}

	loader := newLspModuleLoader(s.rootPath)
	ctx.Loader = loader

	moduleDir := utils.GetModuleDir(ctx.FilePath)
	openDocs := s.collectOpenDocuments(uri, content)

	// Check if this is a standalone script (no package declaration)
	isScript := !hasPackageDeclaration(content)

	var mod *modules.Module
	var err error

	if !isScript {
		var modInterface interface{}
		modInterface, err = loader.GetModule(moduleDir)
		if m, ok := modInterface.(*modules.Module); ok {
			mod = m
		}
	}

	var targetProgram *ast.Program
	if isScript || err != nil || mod == nil {
		if !isScript && isMultiplePackagesError(err) {
			return nil, false
		}
		var parseErrors []*diagnostics.DiagnosticError
		mod, targetProgram, parseErrors = s.loadModuleWithOverlays(moduleDir, openDocs, ctx.FilePath, loader.base)
		if len(parseErrors) > 0 {
			ctx.Errors = append(ctx.Errors, parseErrors...)
		}
		if mod == nil {
			return nil, false
		}
	} else {
		targetProgram, _ = s.applyOpenDocumentOverlays(mod, openDocs, ctx.FilePath)
	}

	if mod != nil && len(mod.Files) == 0 {
		var parseErrors []*diagnostics.DiagnosticError
		mod, targetProgram, parseErrors = s.loadModuleWithOverlays(moduleDir, openDocs, ctx.FilePath, loader.base)
		if len(parseErrors) > 0 {
			ctx.Errors = append(ctx.Errors, parseErrors...)
		}
		if mod == nil {
			return nil, false
		}
	}

	ctx.Module = mod
	ctx.SymbolTable = mod.SymbolTable

	if targetProgram != nil {
		ctx.AstRoot = targetProgram
	}

	analyzer.RegisterBuiltins(ctx.SymbolTable)
	sem := analyzer.New(ctx.SymbolTable)
	sem.SetLoader(loader)
	sem.BaseDir = mod.Dir

	// Ensure ResolutionMap is initialized before analysis to avoid resets/overwrites
	sem.ResolutionMap = make(map[ast.Node]symbols.Symbol)

	orderedFiles := mod.OrderedFiles()
	var errors []*diagnostics.DiagnosticError
	for _, fileAST := range orderedFiles {
		errors = append(errors, sem.AnalyzeNaming(fileAST)...)
	}
	for _, fileAST := range orderedFiles {
		errors = append(errors, sem.AnalyzeHeaders(fileAST)...)
	}
	for _, fileAST := range orderedFiles {
		errors = append(errors, sem.AnalyzeInstances(fileAST)...)
	}
	for _, fileAST := range orderedFiles {
		errors = append(errors, sem.AnalyzeBodies(fileAST)...)
	}

	ctx.TypeMap = sem.TypeMap
	ctx.ResolutionMap = sem.ResolutionMap
	ctx.TraitDefaults = sem.TraitDefaults
	ctx.OperatorTraits = ctx.SymbolTable.GetAllOperatorTraits()
	ctx.TraitImplementations = ctx.SymbolTable.GetAllImplementations()

	mod.SetTypeMap(ctx.TypeMap)
	mod.SetTraitDefaults(ctx.TraitDefaults)

	if len(errors) > 0 {
		ctx.Errors = append(ctx.Errors, errors...)
	}

	return ctx, true
}

func (s *LanguageServer) collectOpenDocuments(currentURI string, currentContent string) map[string]string {
	documents := make(map[string]string)

	s.mu.RLock()
	for uri, doc := range s.documents {
		doc.Mu.RLock()
		documents[s.uriToPath(uri)] = doc.Content
		doc.Mu.RUnlock()
	}
	s.mu.RUnlock()

	if currentURI != "" {
		documents[s.uriToPath(currentURI)] = currentContent
	}

	return documents
}

func (s *LanguageServer) applyOpenDocumentOverlays(mod *modules.Module, openDocs map[string]string, targetPath string) (*ast.Program, []*diagnostics.DiagnosticError) {
	if mod == nil {
		return nil, nil
	}

	moduleExt := ""
	if len(mod.Files) > 0 && mod.Files[0] != nil && mod.Files[0].File != "" {
		moduleExt = filepath.Ext(mod.Files[0].File)
	}

	fileIndex := make(map[string]int)
	for i, file := range mod.Files {
		if file == nil || file.File == "" {
			continue
		}
		fileIndex[filepath.Clean(file.File)] = i
	}

	var targetProgram *ast.Program
	var errors []*diagnostics.DiagnosticError
	for path, content := range openDocs {
		if path == "" {
			continue
		}
		cleanPath := filepath.Clean(path)
		if filepath.Dir(cleanPath) != mod.Dir {
			continue
		}
		if moduleExt != "" && filepath.Ext(cleanPath) != moduleExt {
			continue
		}

		program, parseErrors := parseProgramFromContent(cleanPath, content)
		if len(parseErrors) > 0 {
			errors = append(errors, parseErrors...)
		}
		if program == nil {
			continue
		}

		if idx, ok := fileIndex[cleanPath]; ok {
			mod.Files[idx] = program
		} else {
			mod.Files = append(mod.Files, program)
		}

		if cleanPath == filepath.Clean(targetPath) {
			targetProgram = program
		}
	}

	if targetProgram == nil {
		for _, file := range mod.Files {
			if file != nil && filepath.Clean(file.File) == filepath.Clean(targetPath) {
				targetProgram = file
				break
			}
		}
	}

	return targetProgram, errors
}

func parseProgramFromContent(filePath string, content string) (*ast.Program, []*diagnostics.DiagnosticError) {
	ctx := pipeline.NewPipelineContext(content)
	ctx.FilePath = filePath

	processingPipeline := pipeline.New(
		&lexer.LexerProcessor{},
		&parser.ParserProcessor{},
	)

	finalCtx := processingPipeline.Run(ctx)
	program, _ := finalCtx.AstRoot.(*ast.Program)

	return program, finalCtx.Errors
}

func (s *LanguageServer) loadModuleWithOverlays(moduleDir string, openDocs map[string]string, targetPath string, loader *modules.Loader) (*modules.Module, *ast.Program, []*diagnostics.DiagnosticError) {
	absDir, err := filepath.Abs(moduleDir)
	if err != nil {
		return nil, nil, []*diagnostics.DiagnosticError{
			diagnostics.NewError(diagnostics.ErrA003, token.Token{}, err.Error()),
		}
	}

	dirEntries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, nil, []*diagnostics.DiagnosticError{
			diagnostics.NewError(diagnostics.ErrA003, token.Token{}, err.Error()),
		}
	}

	ext := detectModuleExtension(absDir, dirEntries, openDocs)
	if ext == "" {
		ext = config.SourceFileExt
	}

	// Check if targetPath is a standalone script (no package declaration)
	isStandaloneScript := false
	if targetPath != "" {
		var content string
		if c, ok := openDocs[targetPath]; ok {
			content = c
		} else {
			b, err := os.ReadFile(targetPath)
			if err == nil {
				content = string(b)
			}
		}
		if content != "" {
			// Optimized check for package declaration
			// Instead of full parse, just scan the first relevant token
			if !hasPackageDeclaration(content) {
				isStandaloneScript = true
			}
		}
	}

	sourceFiles := make([]string, 0)
	if isStandaloneScript {
		sourceFiles = append(sourceFiles, targetPath)
	} else {
		for _, entry := range dirEntries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if strings.HasSuffix(name, ext) {
				sourceFiles = append(sourceFiles, filepath.Join(absDir, name))
			}
		}
		for path := range openDocs {
			if filepath.Dir(path) != absDir {
				continue
			}
			if strings.HasSuffix(path, ext) {
				found := false
				for _, existing := range sourceFiles {
					if existing == path {
						found = true
						break
					}
				}
				if !found {
					sourceFiles = append(sourceFiles, path)
				}
			}
		}
	}
	if len(sourceFiles) == 0 {
		return nil, nil, nil
	}

	module := &modules.Module{
		Dir:         absDir,
		Exports:     make(map[string]bool),
		Imports:     make(map[string]*modules.Module),
		SymbolTable: symbols.NewSymbolTable(),
	}

	var packageName string
	var targetProgram *ast.Program
	var errors []*diagnostics.DiagnosticError

	for _, filePath := range sourceFiles {
		content, ok := openDocs[filePath]
		if !ok {
			bytes, readErr := os.ReadFile(filePath)
			if readErr != nil {
				errors = append(errors, diagnostics.NewError(diagnostics.ErrA003, token.Token{}, readErr.Error()))
				continue
			}
			content = string(bytes)
		}

		program, parseErrors := parseProgramFromContent(filePath, content)
		if len(parseErrors) > 0 {
			errors = append(errors, parseErrors...)
		}
		if program == nil {
			continue
		}

		module.Files = append(module.Files, program)
		if filePath == targetPath {
			targetProgram = program
		}

		if packageName == "" {
			packageName = extractPackageName(program)
		}
	}

	if packageName == "" {
		packageName = uniqueModuleName(absDir)
	}

	module.Name = packageName
	loader.LoadedModules[absDir] = module
	loader.ModulesByName[packageName] = module

	return module, targetProgram, errors
}

func detectModuleExtension(dirPath string, entries []os.DirEntry, openDocs map[string]string) string {
	dirName := filepath.Base(dirPath)
	for _, ext := range config.SourceFileExtensions {
		mainFile := dirName + ext
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if entry.Name() == mainFile {
				return ext
			}
		}
		for path := range openDocs {
			if filepath.Base(path) == mainFile {
				return ext
			}
		}
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		for _, ext := range config.SourceFileExtensions {
			if strings.HasSuffix(entry.Name(), ext) {
				return ext
			}
		}
	}
	for path := range openDocs {
		for _, ext := range config.SourceFileExtensions {
			if strings.HasSuffix(path, ext) {
				return ext
			}
		}
	}
	return ""
}

func extractPackageName(program *ast.Program) string {
	if program == nil {
		return ""
	}
	for _, stmt := range program.Statements {
		if pkg, ok := stmt.(*ast.PackageDeclaration); ok && pkg.Name != nil {
			return pkg.Name.Value
		}
	}
	return ""
}

func uniqueModuleName(dir string) string {
	clean := filepath.Clean(dir)
	base := filepath.Base(clean)
	if base == "." || base == string(filepath.Separator) || base == "" {
		base = "module"
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(clean))
	return fmt.Sprintf("%s_%08x", base, hash.Sum32())
}

func isMultiplePackagesError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "multiple packages in directory")
}
