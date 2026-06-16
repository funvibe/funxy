package targets

import (
	"fmt"
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

		// Generate the program outside the worker goroutine so that, on
		// timeout, we can report the exact input that caused it.
		gen := generators.NewFromData(data)
		input := gen.GenerateProgram()

		type result struct {
			invalid     bool
			invalidErrs string
			code1       string
			code2       string
			unstable    bool
		}

		// Use a channel to enforce a per-iteration timeout. The timeout exists
		// only to catch genuine infinite loops in the parser/printer. It is a
		// wall-clock deadline, and the fuzz targets run in parallel, so it must
		// be generous enough to tolerate scheduler/GC starvation under load
		// (parse+print of even the largest generated program takes a few ms).
		done := make(chan result, 1)

		go func() {
			var res result

			// 1. Parse Original
			ctx1 := pipeline.NewPipelineContext(input)
			l1 := lexer.New(input)
			stream1 := lexer.NewTokenStream(l1)
			p1 := parser.New(stream1, ctx1)
			program1 := p1.ParseProgram()

			if len(ctx1.Errors) > 0 {
				done <- res // Invalid generated code
				return
			}

			// 2. Print AST (Pass 1)
			printer1 := prettyprinter.NewCodePrinter()
			program1.Accept(printer1)
			res.code1 = printer1.String()

			// 3. Parse Printed Code
			ctx2 := pipeline.NewPipelineContext(res.code1)
			l2 := lexer.New(res.code1)
			stream2 := lexer.NewTokenStream(l2)
			p2 := parser.New(stream2, ctx2)
			program2 := p2.ParseProgram()

			if len(ctx2.Errors) > 0 {
				res.invalid = true
				res.invalidErrs = fmt.Sprintf("%v", ctx2.Errors)
				done <- res
				return
			}

			// 4. Print AST (Pass 2)
			printer2 := prettyprinter.NewCodePrinter()
			program2.Accept(printer2)
			res.code2 = printer2.String()

			// 5. Verify Idempotency
			res.unstable = res.code1 != res.code2
			done <- res
		}()

		select {
		case res := <-done:
			if res.invalid {
				t.Errorf("Formatter produced invalid code:\n%s\nErrors: %s", res.code1, res.invalidErrs)
			} else if res.unstable {
				t.Errorf("Formatter instability:\nPass 1:\n%s\nPass 2:\n%s", res.code1, res.code2)
			}
		case <-time.After(5 * time.Second):
			// Timeout - most likely an infinite loop in parser or printer.
			t.Fatalf("Formatter timed out (>5s) on input:\n%s", input)
		}
	})
}
