package analyzer_test

import (
	"github.com/funvibe/funxy/internal/analyzer"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/symbols"
	"strings"
	"testing"
)

func TestAnalyzer_Exhaustiveness(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string // Expected substrings in error messages
	}{
		{
			name: "Basic Exhaustive Match",
			input: `
			match 1 {
				_ -> 0
			}
			`,
			expected: nil,
		},
		{
			name: "Basic Non-Exhaustive Match",
			input: `
			match 1 {
				1 -> 0
			}
			`,
			expected: []string{"Missing cases: other Int values"},
		},
		{
			name: "ADT Exhaustive Match",
			input: `
			type MyOption<t> = MySome t | MyNone
			x: MyOption<Int> = MySome(10)
			match x {
				MySome(y) -> y
				MyNone -> 0
			}
			`,
			expected: nil,
		},
		{
			name: "ADT Non-Exhaustive Match",
			input: `
			type MyOption<t> = MySome t | MyNone
			x: MyOption<Int> = MySome(10)
			match x {
				MySome(y) -> y
			}
			`,
			expected: []string{"Missing cases: [MyNone]"},
		},
		{
			name: "Bool Exhaustive Match",
			input: `
			b = true
			match b {
				true -> 1
				false -> 0
			}
			`,
			expected: nil,
		},
		{
			name: "Bool Non-Exhaustive Match",
			input: `
			b = true
			match b {
				true -> 1
			}
			`,
			expected: []string{"Missing cases: false"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := pipeline.NewPipelineContext("")
			l := lexer.New(tt.input)
			stream := lexer.NewTokenStream(l)
			p := parser.New(stream, ctx)
			prog := p.ParseProgram()

			if len(ctx.Errors) > 0 {
				t.Fatalf("Parser errors: %v", ctx.Errors)
			}

			a := analyzer.New(symbols.NewSymbolTable())
			errs := a.Analyze(prog)

			if len(tt.expected) == 0 {
				if len(errs) > 0 {
					t.Errorf("Expected no errors, got %v", errs)
				}
			} else {
				if len(errs) != len(tt.expected) {
					t.Errorf("Expected %d errors, got %d: %v", len(tt.expected), len(errs), errs)
					return
				}
				for i, expectedSubstr := range tt.expected {
					errMsg := errs[i].Error()
					if !strings.Contains(errMsg, expectedSubstr) {
						t.Errorf("Expected error to contain %q, got %q", expectedSubstr, errMsg)
					}
				}
			}
		})
	}
}
