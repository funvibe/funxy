package analyzer

import (
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/symbols"
	"strings"
	"testing"
)

func analyzeSource(input string) []error {
	ctx := pipeline.NewPipelineContext(input)
	lp := &lexer.LexerProcessor{}
	ctx = lp.Process(ctx)
	p := parser.New(ctx.TokenStream, ctx)
	program := p.ParseProgram()

	if len(ctx.Errors) > 0 {
		var errs []error
		for _, e := range ctx.Errors {
			errs = append(errs, e)
		}
		return errs
	}

	symbolTable := symbols.NewSymbolTable()
	// Ensure builtins are available
	symbolTable.InitBuiltins()
	a := New(symbolTable)
	diags := a.Analyze(program)
	var errs []error
	for _, d := range diags {
		errs = append(errs, d)
	}
	return errs
}

func TestStrictMode_Enabled(t *testing.T) {
	input := `
	directive "strict_types"

	type alias MyUnion = Int | String

	fun takeInt(x: Int) { x }

	fun main() {
		u: MyUnion = 10
		takeInt(u) // Should fail
	}
	`
	errs := analyzeSource(input)
	if len(errs) == 0 {
		t.Fatal("Expected strict mode error, got none")
	}

	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "type is not a member of union") || strings.Contains(e.Error(), "type mismatch") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Expected type error, got: %v", errs)
	}
}

func TestStrictMode_Disabled(t *testing.T) {
	input := `
	type alias MyUnion = Int | String

	fun takeInt(x: Int) { x }

	fun main() {
		u: MyUnion = 10
		takeInt(u) // Should pass (unsafe)
	}
	`
	errs := analyzeSource(input)
	if len(errs) > 0 {
		t.Fatalf("Expected no errors, got: %v", errs)
	}
}
