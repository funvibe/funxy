package analyzer

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/token"
	"github.com/funvibe/funxy/internal/typesystem"
	"sort"
)

// Analyzer performs semantic analysis on the AST.
type Analyzer struct {
	symbolTable   *symbols.SymbolTable
	inLoop        bool // Track if we are inside a loop
	loader        ModuleLoader
	BaseDir       string
	TypeMap       map[ast.Node]typesystem.Type    // Stores inferred types
	inferCtx      *InferenceContext               // Shared inference context for consistent TVar naming
	TraitDefaults map[string]*ast.FunctionStatement // "TraitName.methodName" -> FunctionStatement
}

// ModuleLoader interface to break dependency cycle
type ModuleLoader interface {
	GetModule(path string) (interface{}, error)       // Returns *modules.Module (which implements LoadedModule)
	GetModuleByPackageName(name string) interface{}   // Returns module by package name (for extension methods/traits lookup)
}

// LoadedModule interface representing a fully loaded AND analyzed module
type LoadedModule interface {
	GetName() string                                   // Returns package name
	GetExportedSymbols() map[string]typesystem.Type    // Deprecated: Use GetExports()
	GetExports() map[string]symbols.Symbol

	IsHeadersAnalyzed() bool
	SetHeadersAnalyzed(bool)
	IsHeadersAnalyzing() bool
	SetHeadersAnalyzing(bool)

	IsBodiesAnalyzed() bool
	SetBodiesAnalyzed(bool)
	IsBodiesAnalyzing() bool
	SetBodiesAnalyzing(bool)

	GetFiles() []*ast.Program
	GetSymbolTable() *symbols.SymbolTable
	SetTypeMap(map[ast.Node]typesystem.Type)

	// Package group support
	IsPackageGroupModule() bool
	GetSubModulesRaw() map[string]interface{} // Returns sub-modules (cast to LoadedModule)

	// Re-export support
	GetReexportSpecs() []*ast.ExportSpec
	AddExport(name string)

	// Trait defaults
	SetTraitDefaults(map[string]*ast.FunctionStatement)
	GetTraitDefaults() map[string]*ast.FunctionStatement
}

// New creates a new Analyzer with a given symbol table.
func New(symbolTable *symbols.SymbolTable) *Analyzer {
	return &Analyzer{
		symbolTable:   symbolTable,
		inLoop:        false,
		BaseDir:       ".", // Default to CWD
		TraitDefaults: make(map[string]*ast.FunctionStatement),
	}
}

func (a *Analyzer) SetLoader(l ModuleLoader) {
	a.loader = l
}

func (a *Analyzer) RegisterBuiltins() {
	RegisterBuiltins(a.symbolTable)
}

type walker struct {
	symbolTable       *symbols.SymbolTable
	errorSet          map[string]*diagnostics.DiagnosticError // Key: "line:col:code" for deduplication
	errors            []*diagnostics.DiagnosticError          // Temporary slice for compatibility with BuildType etc.
	inLoop            bool // Track if we are inside a loop
	inInstance        bool // Track if we are inside an instance declaration
	loader            ModuleLoader
	BaseDir           string
	TypeMap           map[ast.Node]typesystem.Type
	inferCtx          *InferenceContext // Context for type inference
	mode              AnalysisMode
	TraitDefaults     map[string]*ast.FunctionStatement // "TraitName.methodName" -> FunctionStatement
	currentModuleName string // Name of the module being analyzed (for OriginModule tracking)
}

// addError adds an error to the walker, deduplicating by position and message
func (w *walker) addError(err *diagnostics.DiagnosticError) {
	key := fmt.Sprintf("%d:%d:%s", err.Token.Line, err.Token.Column, err.Code)
	if w.errorSet == nil {
		w.errorSet = make(map[string]*diagnostics.DiagnosticError)
	}
	w.errorSet[key] = err
}

// addErrors adds multiple errors to the walker
func (w *walker) addErrors(errs []*diagnostics.DiagnosticError) {
	for _, err := range errs {
		w.addError(err)
	}
}


// getErrors returns all unique errors as a slice, sorted by position
func (w *walker) getErrors() []*diagnostics.DiagnosticError {
	// First, merge any errors from the compatibility slice into errorSet
	for _, err := range w.errors {
		w.addError(err)
	}

	result := make([]*diagnostics.DiagnosticError, 0, len(w.errorSet))
	for _, err := range w.errorSet {
		result = append(result, err)
	}

	// Sort by line, then column for deterministic output
	sort.Slice(result, func(i, j int) bool {
		if result[i].Token.Line != result[j].Token.Line {
			return result[i].Token.Line < result[j].Token.Line
		}
		return result[i].Token.Column < result[j].Token.Column
	})

	return result
}

type AnalysisMode int

const (
	ModeFull    AnalysisMode = iota // Legacy/Single file
	ModeNaming                      // Pass 1: Name Discovery (Pending Symbols)
	ModeHeaders                     // Pass 2: Imports and Signature Resolution
	ModeInstances                   // Pass 3: Trait Instances
	ModeBodies                      // Pass 4: Bodies and Expressions
)

// AnalyzeNaming runs the naming pass (discovery)
func (a *Analyzer) AnalyzeNaming(node ast.Node) []*diagnostics.DiagnosticError {
	// Simple walker for naming only
	w := &walker{
		symbolTable:   a.symbolTable,
		errorSet:      make(map[string]*diagnostics.DiagnosticError),
		errors:        []*diagnostics.DiagnosticError{},
		loader:        a.loader,
		BaseDir:       a.BaseDir,
		mode:          ModeNaming,
	}
	node.Accept(w)
	return w.getErrors()
}

func (a *Analyzer) AnalyzeHeaders(node ast.Node) []*diagnostics.DiagnosticError {
	typeMap := make(map[ast.Node]typesystem.Type)

	// Create shared InferenceContext if not exists
	if a.inferCtx == nil {
		a.inferCtx = NewInferenceContextWithLoader(a.loader)
	}
	a.inferCtx.TypeMap = typeMap

	w := &walker{
		symbolTable:   a.symbolTable,
		errorSet:      make(map[string]*diagnostics.DiagnosticError),
		errors:        []*diagnostics.DiagnosticError{},
		inLoop:        false,
		loader:        a.loader,
		BaseDir:       a.BaseDir,
		TypeMap:       typeMap,
		inferCtx:      a.inferCtx, // Use shared context
		mode:          ModeHeaders,
		TraitDefaults: a.TraitDefaults,
	}
	node.Accept(w)

	// Merge TypeMap
	if a.TypeMap == nil {
		a.TypeMap = make(map[ast.Node]typesystem.Type)
	}
	for k, v := range w.TypeMap {
		a.TypeMap[k] = v
	}

	return w.getErrors()
}

func (a *Analyzer) AnalyzeInstances(node ast.Node) []*diagnostics.DiagnosticError {
	// Reuse existing TypeMap and InferenceContext from Headers pass
	if a.TypeMap == nil {
		a.TypeMap = make(map[ast.Node]typesystem.Type)
	}
	if a.inferCtx == nil {
		a.inferCtx = NewInferenceContextWithLoader(a.loader)
	}
	a.inferCtx.TypeMap = a.TypeMap

	w := &walker{
		symbolTable:   a.symbolTable,
		errorSet:      make(map[string]*diagnostics.DiagnosticError),
		errors:        []*diagnostics.DiagnosticError{},
		inLoop:        false,
		loader:        a.loader,
		BaseDir:       a.BaseDir,
		TypeMap:       a.TypeMap,
		inferCtx:      a.inferCtx,
		mode:          ModeInstances,
		TraitDefaults: a.TraitDefaults,
	}
	node.Accept(w)

	return w.getErrors()
}

func (a *Analyzer) AnalyzeBodies(node ast.Node) []*diagnostics.DiagnosticError {
	// Reuse existing TypeMap if possible
	if a.TypeMap == nil {
		a.TypeMap = make(map[ast.Node]typesystem.Type)
	}

	// Reuse shared InferenceContext (counter continues from Headers pass)
	if a.inferCtx == nil {
		a.inferCtx = NewInferenceContextWithLoader(a.loader)
	}
	a.inferCtx.TypeMap = a.TypeMap

	w := &walker{
		symbolTable:   a.symbolTable,
		errorSet:      make(map[string]*diagnostics.DiagnosticError),
		errors:        []*diagnostics.DiagnosticError{},
		inLoop:        false,
		loader:        a.loader,
		BaseDir:       a.BaseDir,
		TypeMap:       a.TypeMap,
		inferCtx:      a.inferCtx, // Use shared context
		mode:          ModeBodies,
		TraitDefaults: a.TraitDefaults,
	}
	node.Accept(w)

	return w.getErrors()
}

// Analyze performs semantic analysis on the given node.
func (a *Analyzer) Analyze(node ast.Node) []*diagnostics.DiagnosticError {
	// If node is Program, use multi-pass analysis
	if prog, ok := node.(*ast.Program); ok {
		// Pass 1: Naming
		errs := a.AnalyzeNaming(prog)
		if len(errs) > 0 {
			return errs
		}
		// Pass 2: Headers
		errs = a.AnalyzeHeaders(prog)
		if len(errs) > 0 {
			return errs
		}
		// Pass 3: Instances
		errs = a.AnalyzeInstances(prog)
		if len(errs) > 0 {
			return errs
		}
		// Pass 4: Bodies
		return a.AnalyzeBodies(prog)
	}

	// Fallback for partial nodes (Expressions, etc.) - ModeFull
	typeMap := make(map[ast.Node]typesystem.Type)
	w := &walker{
		symbolTable:   a.symbolTable,
		errorSet:      make(map[string]*diagnostics.DiagnosticError),
		errors:        []*diagnostics.DiagnosticError{},
		inLoop:        a.inLoop, // Inherit loop state from Analyzer
		loader:        a.loader,
		BaseDir:       a.BaseDir,
		TypeMap:       typeMap,
		inferCtx:      NewInferenceContextWithTypeMap(typeMap),
		mode:          ModeFull,
		TraitDefaults: a.TraitDefaults,
	}
	node.Accept(w)

	a.TypeMap = w.TypeMap

	// Validate exports after processing the whole program
	if prog, ok := node.(*ast.Program); ok {
		for _, stmt := range prog.Statements {
			if pkg, ok := stmt.(*ast.PackageDeclaration); ok {
				if !pkg.ExportAll {
					for _, exp := range pkg.Exports {
						// Only validate local exports, not re-exports
						if !exp.IsReexport() && exp.Symbol != nil {
							if !w.symbolTable.IsDefined(exp.Symbol.Value) {
								w.addError(diagnostics.NewError(
									diagnostics.ErrA001,
									exp.GetToken(),
									"exported symbol not defined: "+exp.Symbol.Value,
								))
							}
						}
						// TODO: validate re-exports (check if module alias exists and symbols are importable)
					}
				}
			}
		}
	}

	return w.getErrors()
}

// freshVar generates a fresh type variable using the walker's inference context.
func (w *walker) freshVar() typesystem.TVar {
	return w.inferCtx.FreshVar()
}

func (w *walker) markTailCalls(node ast.Node) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case *ast.BlockStatement:
		if len(n.Statements) > 0 {
			lastStmt := n.Statements[len(n.Statements)-1]
			w.markTailCalls(lastStmt)
		}
	case *ast.ExpressionStatement:
		w.markTailCalls(n.Expression)
	case *ast.CallExpression:
		n.IsTail = true
	case *ast.IfExpression:
		w.markTailCalls(n.Consequence)
		if n.Alternative != nil {
			w.markTailCalls(n.Alternative)
		}
	case *ast.MatchExpression:
		for _, arm := range n.Arms {
			w.markTailCalls(arm.Expression)
		}
	}
}

// getNodeToken extracts token from AST node if possible
func getNodeToken(node ast.Node) token.Token {
	if node == nil {
		return token.Token{}
	}
	if getter, ok := node.(interface{ GetToken() token.Token }); ok {
		return getter.GetToken()
	}
	return token.Token{}
}

// appendError adds an error to the walker's error list.
// If the error is already a DiagnosticError, it's added directly.
// Otherwise, it's wrapped with the given node's location.
// Combined errors are unpacked into individual errors.
func (w *walker) appendError(node ast.Node, err error) {
	// Handle combined errors by unpacking them
	if ce, ok := err.(*combinedError); ok {
		for _, e := range ce.errors {
			w.appendError(node, e)
		}
		return
	}
	if ce, ok := err.(*diagnostics.DiagnosticError); ok {
		w.addError(ce)
	} else {
		w.addError(diagnostics.NewError(diagnostics.ErrA003, getNodeToken(node), err.Error()))
	}
}
