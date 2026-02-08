package parser

import (
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/pipeline"
	"testing"
)

func TestRecursiveTypeAliasParsing(t *testing.T) {
	input := `
	// 1. Simple recursive type alias
	type alias Tree = { val: Int, children: List<Tree> }

	// 2. Recursive type alias with generic parameter
	type alias Node<t> = { value: t, next: Option<Node<t>> }

	// 3. Mutually recursive type aliases
	type alias A = List<B>
	type alias B = List<A>
	`

	ctx := pipeline.NewPipelineContext(input)
	processor := &lexer.LexerProcessor{}
	ctx = processor.Process(ctx)

	p := New(ctx.TokenStream, ctx)
	p.ParseProgram()

	if len(p.ctx.Errors) > 0 {
		t.Fatalf("Parser has errors: %v", p.ctx.Errors)
	}
}
