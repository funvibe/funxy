package targets

import (
	"github.com/funvibe/funxy/internal/analyzer"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/tests/fuzz/generators"
	"testing"
	"time"
)

// FuzzTypeChecker is the entry point for fuzzing the type checker.
func FuzzTypeChecker(f *testing.F) {
	// Add seed corpus
	f.Add([]byte("fun main() { print(\"Hello\") }"))
	f.Add([]byte("x = 1 + 2"))
	f.Add([]byte("fun add(a: Int, b: Int) -> Int { a + b }"))

	// Load examples from corpus
	LoadCorpus(f, "../../../examples", "../../../tests")

	f.Fuzz(func(t *testing.T, data []byte) {
		// Limit input size
		if len(data) > 1000 {
			return
		}

		gen := generators.NewFromData(data)
		input := gen.GenerateProgram()

		// Limit generated program size
		if len(input) > 10000 {
			return
		}

		// 1. Parse with timeout
		ctx := pipeline.NewPipelineContext(input)
		l := lexer.New(input)
		stream := lexer.NewTokenStream(l)
		p := parser.New(stream, ctx)

		var program *ast.Program
		parseDone := make(chan bool)
		go func() {
			program = p.ParseProgram()
			parseDone <- true
		}()

		select {
		case <-parseDone:
		case <-time.After(50 * time.Millisecond):
			return // Parse timeout
		}

		if program == nil || len(ctx.Errors) > 0 {
			// If parsing fails, we can't type check
			return
		}

		// 2. Analyze with timeout
		analyzeDone := make(chan bool)
		go func() {
			symbolTable := symbols.NewSymbolTable()
			a := analyzer.New(symbolTable)

			// Register builtins to have a valid environment
			a.RegisterBuiltins()

			// Run analysis
			// Analyze returns a list of errors, but shouldn't panic
			_ = a.Analyze(program)
			analyzeDone <- true
		}()

		select {
		case <-analyzeDone:
		case <-time.After(200 * time.Millisecond):
			return // Analyze timeout
		}
	})
}
