package analyzer

import (
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/symbols"
	"testing"
)

func TestAnalyzerBug_PartialRecord(t *testing.T) {
	input := `
fun getX(r) {
    match r {
        { x: x } -> x
    }
}
`
	ctx := pipeline.NewPipelineContext(input)

	// Setup token stream using LexerProcessor
	lp := &lexer.LexerProcessor{}
	ctx = lp.Process(ctx)

	p := parser.New(ctx.TokenStream, ctx)
	program := p.ParseProgram()

	if len(ctx.Errors) > 0 {
		t.Fatalf("Parse errors: %v", ctx.Errors)
	}

	symbolTable := symbols.NewSymbolTable()
	a := New(symbolTable)
	errors := a.Analyze(program)

	if len(errors) > 0 {
		for _, e := range errors {
			t.Logf("Analyzer error: %s", e)
		}
		t.Fatalf("Analyzer failed with %d errors", len(errors))
	}
}
