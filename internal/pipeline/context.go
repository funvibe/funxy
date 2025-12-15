package pipeline

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
)

// PipelineContext holds all the data passed between pipeline stages.
type PipelineContext struct {
	SourceCode  string
	FilePath    string // Path to the source file (if any)
	TokenStream TokenStream
	AstRoot     ast.Node
	SymbolTable *symbols.SymbolTable // Add the symbol table here
	TypeMap     map[ast.Node]typesystem.Type // Stores inferred types for expressions
	Errors      []*diagnostics.DiagnosticError

	// Trait default method implementations: "TraitName.methodName" -> FunctionStatement
	TraitDefaults map[string]*ast.FunctionStatement

	// Operator -> Trait mapping for dispatch: "+" -> "Add", "==" -> "Equal"
	OperatorTraits map[string]string

	// Trait implementations: trait -> []types that implement it
	TraitImplementations map[string][]typesystem.Type

	// Module loader - shared between analyzer and evaluator
	// Using interface{} to avoid import cycle with modules package
	Loader interface{}

	// IsTestMode indicates if we are running in test mode (enables test builtins)
	IsTestMode bool
}

// NewPipelineContext creates and initializes a new PipelineContext.
func NewPipelineContext(source string) *PipelineContext {
	return &PipelineContext{
		SourceCode:           source,
		SymbolTable:          symbols.NewSymbolTable(),
		TypeMap:              make(map[ast.Node]typesystem.Type),
		Errors:               []*diagnostics.DiagnosticError{},
		TraitDefaults:        make(map[string]*ast.FunctionStatement),
		OperatorTraits:       make(map[string]string),
		TraitImplementations: make(map[string][]typesystem.Type),
	}
}
