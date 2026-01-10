package parser

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/pipeline"
	"testing"
)

func TestGenericTypeParsing(t *testing.T) {
	input := `
	type alias MyOption<t> = Option<t>
	type MyResult<t, e> = Ok t | Err e

	fun foo(x: Option<Int>) {}
	fun bar(x: MyResult<Int, String>) {}
	`

	ctx := pipeline.NewPipelineContext(input)
	processor := &lexer.LexerProcessor{}
	ctx = processor.Process(ctx)

	p := New(ctx.TokenStream, ctx)
	program := p.ParseProgram()

	if len(p.ctx.Errors) > 0 {
		t.Fatalf("Parser has errors: %v", p.ctx.Errors)
	}

	// Check statements
	if len(program.Statements) != 4 {
		t.Fatalf("Expected 4 statements, got %d", len(program.Statements))
	}

	// Check Option<Int>
	fn1 := program.Statements[2].(*ast.FunctionStatement)
	param1 := fn1.Parameters[0].Type.(*ast.NamedType)
	if param1.Name.Value != "Option" {
		t.Errorf("Expected Option, got %s", param1.Name.Value)
	}
	if len(param1.Args) != 1 {
		t.Fatalf("Expected 1 generic arg, got %d", len(param1.Args))
	}

	// Check MyResult<Int, String>
	fn2 := program.Statements[3].(*ast.FunctionStatement)
	param2 := fn2.Parameters[0].Type.(*ast.NamedType)
	if param2.Name.Value != "MyResult" {
		t.Errorf("Expected MyResult, got %s", param2.Name.Value)
	}
	if len(param2.Args) != 2 {
		t.Fatalf("Expected 2 generic args, got %d", len(param2.Args))
	}
}
