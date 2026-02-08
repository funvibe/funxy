package targets

import (
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/tests/fuzz/generators"
	"testing"
)

func FuzzStdLib(f *testing.F) {
	// Add seed corpus
	f.Add(int64(12345))
	f.Add(int64(67890))

	f.Fuzz(func(t *testing.T, seed int64) {
		gen := generators.NewStdLibGenerator(seed)
		input := gen.GenerateStdLibProgram()

		// 1. Parse
		l := lexer.New(input)
		stream := lexer.NewTokenStream(l)
		ctx := pipeline.NewPipelineContext(input)
		p := parser.New(stream, ctx)
		p.ParseProgram()

		// We expect parsing to succeed for generated code, but if it fails due to random noise or invalid generation logic, it shouldn't panic.
		// Ideally, we would also run the TypeChecker here to verify the stdlib calls are valid,
		// but that requires a full environment with stdlib definitions loaded.
		// For now, we focus on parser robustness against these specific constructs.
	})
}
