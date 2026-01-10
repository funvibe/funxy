package parser

import (
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/pipeline"
	"testing"
)

func TestExtensionMethodParsing(t *testing.T) {
	input := `
	type MyOption<t> = Yes t | NoVal

	fun<t, r> (o: MyOption<t>) map(f: (t) -> r) -> MyOption<r> {
		match o {
			Yes(v) -> Yes(f(v))
			NoVal -> NoVal
		}
	}
	`

	ctx := pipeline.NewPipelineContext(input)
	processor := &lexer.LexerProcessor{}
	ctx = processor.Process(ctx)

	p := New(ctx.TokenStream, ctx)
	program := p.ParseProgram()

	if len(p.ctx.Errors) > 0 {
		t.Fatalf("Parser has errors: %v", p.ctx.Errors)
	}

	if len(program.Statements) != 2 {
		t.Fatalf("program.Statements does not contain 2 statements. got=%d", len(program.Statements))
	}
}
