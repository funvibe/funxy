package targets

import (
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/tests/fuzz/generators"
	"testing"
)

// FuzzParser is the entry point for fuzzing the parser.
// It takes a byte slice as input, tokenizes it, and attempts to parse it.
func FuzzParser(f *testing.F) {
	// Add seed corpus
	f.Add([]byte("fun main() { print(\"Hello\") }"))
	f.Add([]byte("x = 1 + 2"))
	f.Add([]byte("if true { x } else { y }"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Use generator to create structured input
		gen := generators.NewFromData(data)
		input := gen.GenerateProgram()

		// Create pipeline context
		ctx := pipeline.NewPipelineContext(input)

		// Create lexer
		l := lexer.New(input)
		stream := lexer.NewTokenStream(l)

		// Create parser
		p := parser.New(stream, ctx)

		// Parse program
		program := p.ParseProgram()

		// Basic validation
		if program == nil {
			// It's okay if program is nil on error, but we shouldn't panic
			return
		}

		// Check for panic-inducing inputs is handled by the fuzzer automatically.
		// We can add more specific invariants here if needed.
		// For example, ensuring that if there are no errors, the AST is not empty.
		if len(ctx.Errors) == 0 && len(program.Statements) == 0 && len(input) > 0 {
			// This might be valid (e.g. comments only), but worth noting if we want strict checks
		}
	})
}
