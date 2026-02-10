package targets

import (
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/prettyprinter"
	"github.com/funvibe/funxy/tests/fuzz/generators"
	"testing"
	"time"
)

// FuzzFormatter verifies that the pretty printer is idempotent.
// code1 = print(ast1)
// ast2 = parse(code1)
// code2 = print(ast2)
// code1 == code2
func FuzzFormatter(f *testing.F) {
	// Add seed corpus
	f.Add([]byte("fun main() { print(\"Hello\") }"))
	f.Add([]byte("x = 1 + 2"))
	f.Add([]byte("if true { 1 } else { 0 }"))

	// Load examples from corpus
	LoadCorpus(f, "../../../examples", "../../../tests")

	f.Fuzz(func(t *testing.T, data []byte) {
		// Limit input size
		if len(data) > 2000 {
			return
		}

		// Use a channel to enforce per-iteration timeout
		done := make(chan struct{})

		go func() {
			defer close(done)

			// Generate random program
			gen := generators.NewFromData(data)
			input := gen.GenerateProgram()

			// 1. Parse Original
			ctx1 := pipeline.NewPipelineContext(input)
			l1 := lexer.New(input)
			stream1 := lexer.NewTokenStream(l1)
			p1 := parser.New(stream1, ctx1)
			program1 := p1.ParseProgram()

			if len(ctx1.Errors) > 0 {
				return // Invalid generated code
			}

			// 2. Print AST (Pass 1)
			printer1 := prettyprinter.NewCodePrinter()
			program1.Accept(printer1)
			code1 := printer1.String()

			// 3. Parse Printed Code
			ctx2 := pipeline.NewPipelineContext(code1)
			l2 := lexer.New(code1)
			stream2 := lexer.NewTokenStream(l2)
			p2 := parser.New(stream2, ctx2)
			program2 := p2.ParseProgram()

			if len(ctx2.Errors) > 0 {
				t.Errorf("Formatter produced invalid code:\n%s\nErrors: %v", code1, ctx2.Errors)
				return
			}

			// 4. Print AST (Pass 2)
			printer2 := prettyprinter.NewCodePrinter()
			program2.Accept(printer2)
			code2 := printer2.String()

			// 5. Verify Idempotency
			if code1 != code2 {
				t.Errorf("Formatter instability:\nPass 1:\n%s\nPass 2:\n%s", code1, code2)
			}
		}()

		select {
		case <-done:
			// Completed successfully
		case <-time.After(500 * time.Millisecond):
			// Timeout - most likely an infinite loop in parser or printer
			t.Fatal("Formatter timed out (>500ms) on generated input")
		}
	})
}
