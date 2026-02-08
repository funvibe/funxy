package evaluator

import (
	"github.com/funvibe/funxy/internal/analyzer"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/symbols"
	"testing"
)

func TestDispatchStrategy(t *testing.T) {
	input := `
	trait Producer<a> {
	  fun produce() -> a
	}

	instance Producer<Int> {
	  fun produce() { 42 }
	}

	instance Producer<String> {
	  fun produce() { "test" }
	}

	// Explicit type annotation should trigger DispatchReturn strategy
	i : Int :- produce()
	s : String :- produce()
	`

	l := lexer.New(input)
	stream := lexer.NewTokenStream(l)
	ctx := pipeline.NewPipelineContext(input)
	p := parser.New(stream, ctx)
	program := p.ParseProgram()

	if len(ctx.Errors) > 0 {
		var errMsgs []string
		for _, err := range ctx.Errors {
			errMsgs = append(errMsgs, err.Error())
		}
		t.Fatalf("Parser errors: %v", errMsgs)
	}

	// Analyzer phase
	symbolTable := symbols.NewSymbolTable()
	symbolTable.InitBuiltins()

	a := analyzer.New(symbolTable)
	errs := a.Analyze(program)
	if len(errs) > 0 {
		for _, err := range errs {
			t.Errorf("Analyzer error: %s", err.Error())
		}
		t.FailNow()
	}

	// Check if DispatchStrategy was correctly registered
	sources, found := symbolTable.GetTraitMethodDispatch("Producer", "produce")
	if !found {
		t.Fatalf("Dispatch strategy for Producer.produce not found")
	}
	if len(sources) != 1 {
		t.Errorf("Expected 1 dispatch source, got %d", len(sources))
	}

	// Evaluator phase
	env := NewEnvironment()
	env.SymbolTable = symbolTable
	RegisterBuiltins(env)

	eval := New()
	eval.TypeMap = a.TypeMap

	result := eval.Eval(program, env)
	if isError(result) {
		t.Fatalf("Runtime error: %s", result.Inspect())
	}

	// Verify results
	iVal, ok := env.Get("i")
	if !ok {
		t.Fatalf("Variable 'i' not found")
	}
	if iInt, ok := iVal.(*Integer); !ok || iInt.Value != 42 {
		t.Errorf("Expected i = 42, got %s", iVal.Inspect())
	}

	sVal, ok := env.Get("s")
	if !ok {
		t.Fatalf("Variable 's' not found")
	}
	// String is List<Char>
	if sStr, ok := sVal.(*List); !ok {
		t.Errorf("Expected s to be List (String), got %T", sVal)
	} else {
		str := ListToString(sStr)
		if str != "test" {
			t.Errorf("Expected s = 'test', got '%s'", str)
		}
	}
}

func TestMPTCDispatch(t *testing.T) {
	input := `
	trait Converter<a, b> {
	  fun convert(val: a) -> b
	}

	instance Converter<Int, String> {
	  fun convert(val) { "int_to_string" }
	}

	instance Converter<Int, Int> {
	  fun convert(val) { val + 1 }
	}

	// Dispatch needs both 'a' (from arg) and 'b' (from return context)
	s : String :- convert(10)
	i : Int :- convert(10)
	`

	l := lexer.New(input)
	stream := lexer.NewTokenStream(l)
	ctx := pipeline.NewPipelineContext(input)
	p := parser.New(stream, ctx)
	program := p.ParseProgram()

	if len(ctx.Errors) > 0 {
		var errMsgs []string
		for _, err := range ctx.Errors {
			errMsgs = append(errMsgs, err.Error())
		}
		t.Fatalf("Parser errors: %v", errMsgs)
	}

	symbolTable := symbols.NewSymbolTable()
	symbolTable.InitBuiltins()

	a := analyzer.New(symbolTable)
	errs := a.Analyze(program)
	if len(errs) > 0 {
		for _, err := range errs {
			t.Errorf("Analyzer error: %s", err.Error())
		}
		t.FailNow()
	}

	// Check DispatchStrategy for Converter.convert
	sources, found := symbolTable.GetTraitMethodDispatch("Converter", "convert")
	if !found {
		t.Fatalf("Dispatch strategy for Converter.convert not found")
	}
	if len(sources) != 2 {
		t.Errorf("Expected 2 dispatch sources, got %d", len(sources))
	}

	env := NewEnvironment()
	env.SymbolTable = symbolTable
	RegisterBuiltins(env)

	eval := New()
	eval.TypeMap = a.TypeMap

	result := eval.Eval(program, env)
	if isError(result) {
		t.Fatalf("Runtime error: %s", result.Inspect())
	}

	sVal, ok := env.Get("s")
	if !ok {
		t.Fatalf("Variable 's' not found")
	}
	if sStr, ok := sVal.(*List); !ok {
		// It might be a List<Char> but we need to verify.
		// ListToString handles *List check inside, but here we cast first.
		// If it's not *List, ListToString might panic if we passed nil/wrong type to it (it takes Object).
		// Wait, ListToString takes *List.
		t.Errorf("Expected s to be List (String), got %T", sVal)
	} else {
		str := ListToString(sStr)
		if str != "int_to_string" {
			t.Errorf("Expected s = 'int_to_string', got '%s'", str)
		}
	}

	iVal, ok := env.Get("i")
	if !ok {
		t.Fatalf("Variable 'i' not found")
	}
	if iInt, ok := iVal.(*Integer); !ok || iInt.Value != 11 {
		t.Errorf("Expected i = 11, got %s", iVal.Inspect())
	}
}
