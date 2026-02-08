package parser

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/pipeline"
	"testing"
)

func checkParserErrors(t *testing.T, p *Parser) {
	errors := p.ctx.Errors
	if len(errors) == 0 {
		return
	}

	t.Errorf("parser has %d errors", len(errors))
	for _, msg := range errors {
		t.Errorf("parser error: %q", msg.Error())
	}
	t.FailNow()
}

func TestNestedGenericsInTraits(t *testing.T) {
	input := `
	trait Convert<a, b> {
		fun convert(x: a) -> b
	}

	trait Nested<t> : Convert<List<List<t>>, Map<String, List<t>>> {
		fun nested(x: t)
	}

	instance Convert<List<List<Int>>, Map<String, List<Int>>> {
		fun convert(x: List<List<Int>>) -> Map<String, List<Int>> {
			%{}
		}
	}
	`

	l := lexer.New(input)
	stream := lexer.NewTokenStream(l)
	p := New(stream, pipeline.NewPipelineContext(input))
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 3 {
		for i, stmt := range program.Statements {
			t.Logf("Statement %d: %T", i, stmt)
		}
		t.Fatalf("program.Statements does not contain 3 statements. got=%d",
			len(program.Statements))
	}

	// 1. Check Trait Declaration with Nested Generics in SuperTraits
	traitStmt, ok := program.Statements[1].(*ast.TraitDeclaration)
	if !ok {
		t.Fatalf("stmt is not ast.TraitDeclaration. got=%T", program.Statements[1])
	}

	if traitStmt.Name.Value != "Nested" {
		t.Errorf("traitStmt.Name.Value not 'Nested'. got=%s", traitStmt.Name.Value)
	}

	if len(traitStmt.SuperTraits) != 1 {
		t.Fatalf("traitStmt.SuperTraits length wrong. got=%d", len(traitStmt.SuperTraits))
	}

	superTrait := traitStmt.SuperTraits[0].(*ast.NamedType)
	if superTrait.Name.Value != "Convert" {
		t.Errorf("superTrait.Name.Value not 'Convert'. got=%s", superTrait.Name.Value)
	}

	if len(superTrait.Args) != 2 {
		t.Fatalf("superTrait.Args length wrong. got=%d", len(superTrait.Args))
	}

	// Check List<List<T>>
	arg1 := superTrait.Args[0].(*ast.NamedType)
	if arg1.Name.Value != "List" {
		t.Errorf("arg1.Name.Value not 'List'. got=%s", arg1.Name.Value)
	}
	if len(arg1.Args) != 1 {
		t.Fatalf("arg1.Args length wrong. got=%d", len(arg1.Args))
	}
	arg1Inner := arg1.Args[0].(*ast.NamedType)
	if arg1Inner.Name.Value != "List" {
		t.Errorf("arg1Inner.Name.Value not 'List'. got=%s", arg1Inner.Name.Value)
	}

	// 2. Check Instance Declaration with Nested Generics
	instStmt, ok := program.Statements[2].(*ast.InstanceDeclaration)
	if !ok {
		t.Fatalf("stmt is not ast.InstanceDeclaration. got=%T", program.Statements[2])
	}

	if instStmt.TraitName.Value != "Convert" {
		t.Errorf("instStmt.TraitName.Value not 'Convert'. got=%s", instStmt.TraitName.Value)
	}

	if len(instStmt.Args) != 2 {
		t.Fatalf("instStmt.Args length wrong. got=%d", len(instStmt.Args))
	}

	// Check List<List<Int>>
	instArg1 := instStmt.Args[0].(*ast.NamedType)
	if instArg1.Name.Value != "List" {
		t.Errorf("instArg1.Name.Value not 'List'. got=%s", instArg1.Name.Value)
	}
	instArg1Inner := instArg1.Args[0].(*ast.NamedType)
	if instArg1Inner.Name.Value != "List" {
		t.Errorf("instArg1Inner.Name.Value not 'List'. got=%s", instArg1Inner.Name.Value)
	}
}
