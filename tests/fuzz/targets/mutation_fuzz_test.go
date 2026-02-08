package targets

import (
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/prettyprinter"
	"github.com/funvibe/funxy/tests/fuzz/mutator"
	"testing"
)

func FuzzMutation(f *testing.F) {
	// Load corpus
	LoadCorpus(f, "../../../examples", "../../../tests")

	f.Fuzz(func(t *testing.T, data []byte) {
		// 1. Parse the input (seed)
		input := string(data)
		l := lexer.New(input)
		stream := lexer.NewTokenStream(l)
		ctx := pipeline.NewPipelineContext(input)
		p := parser.New(stream, ctx)
		program := p.ParseProgram()

		if len(ctx.Errors) > 0 {
			// If seed is invalid, we can't mutate it effectively (or maybe we can, but let's focus on valid seeds first)
			return
		}

		// 2. Mutate the AST
		// Use a deterministic seed based on the input data to ensure reproducibility
		seed := int64(len(data))
		for _, b := range data {
			seed = seed*31 + int64(b)
		}
		m := mutator.NewASTMutator(seed)
		m.Mutate(program)

		// 3. Print the mutated AST back to code
		printer := prettyprinter.NewCodePrinter()
		program.Accept(printer)
		mutatedCode := printer.String()

		// 4. Parse the mutated code
		// It might be invalid, but it shouldn't crash the parser
		l2 := lexer.New(mutatedCode)
		stream2 := lexer.NewTokenStream(l2)
		p2 := parser.New(stream2, pipeline.NewPipelineContext(mutatedCode))
		p2.ParseProgram()

		// We don't check for errors here, as mutations can easily create invalid code.
		// The goal is to ensure the parser doesn't panic.
	})
}
