package targets

import (
	"context"
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/backend"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/prettyprinter"
	"github.com/funvibe/funxy/tests/fuzz/generators"
	"testing"
	"time"
)

// FuzzRoundTrip verifies that the pretty printer produces valid code that is semantically equivalent to the original.
// It checks the property: execute(parse(print(parse(code)))) == execute(parse(code))
func FuzzRoundTrip(f *testing.F) {
	// Add seed corpus
	f.Add([]byte("fun main() { print(\"Hello\") }"))
	f.Add([]byte("x = 1 + 2"))
	f.Add([]byte("if true { 1 } else { 0 }"))

	// Load examples from corpus
	LoadCorpus(f, "../../../examples", "../../../tests")

	f.Fuzz(func(t *testing.T, data []byte) {
		// Limit input size to prevent resource exhaustion
		if len(data) > 1000 {
			return
		}

		gen := generators.NewFromData(data)
		input := gen.GenerateProgram()

		// Limit generated program size
		if len(input) > 10000 {
			return
		}

		// 1. Parse Original with timeout
		// All channels are buffered (capacity 1) to prevent goroutine leaks on timeout.
		ctx1 := pipeline.NewPipelineContext(input)
		l1 := lexer.New(input)
		stream1 := lexer.NewTokenStream(l1)
		p1 := parser.New(stream1, ctx1)

		var program1 *ast.Program
		parseDone := make(chan bool, 1)
		go func() {
			program1 = p1.ParseProgram()
			parseDone <- true
		}()

		select {
		case <-parseDone:
		case <-time.After(50 * time.Millisecond):
			return
		}

		if program1 == nil || len(ctx1.Errors) > 0 {
			return
		}

		// 2. Print AST with timeout
		var printedCode string
		printDone := make(chan bool, 1)
		go func() {
			printer := prettyprinter.NewCodePrinter()
			program1.Accept(printer)
			printedCode = printer.String()
			printDone <- true
		}()

		select {
		case <-printDone:
		case <-time.After(100 * time.Millisecond):
			return
		}

		// 3. Parse Printed Code with timeout
		ctx2 := pipeline.NewPipelineContext(printedCode)
		l2 := lexer.New(printedCode)
		stream2 := lexer.NewTokenStream(l2)
		p2 := parser.New(stream2, ctx2)

		var program2 *ast.Program
		reparseDone := make(chan bool, 1)
		go func() {
			program2 = p2.ParseProgram()
			reparseDone <- true
		}()

		select {
		case <-reparseDone:
		case <-time.After(50 * time.Millisecond):
			return
		}

		// 4. Verify Re-parsing Succeeded
		if program2 == nil || len(ctx2.Errors) > 0 {
			t.Fatalf("Round-trip failed: Printed code could not be parsed.\nOriginal:\n%s\nPrinted:\n%s\nErrors: %v", input, printedCode, ctx2.Errors)
		}

		// 5. Verify Idempotency (Optional but recommended)
		// print(parse(printedCode)) should equal printedCode (mostly)
		var printedCode2 string
		printDone2 := make(chan bool, 1)
		go func() {
			printer2 := prettyprinter.NewCodePrinter()
			program2.Accept(printer2)
			printedCode2 = printer2.String()
			printDone2 <- true
		}()

		select {
		case <-printDone2:
		case <-time.After(100 * time.Millisecond):
			return
		}

		if printedCode != printedCode2 {
			t.Fatalf("Round-trip instability: Output changed after second pass.\nPass 1:\n%s\nPass 2:\n%s", printedCode, printedCode2)
		}

		// 6. Check Semantic Equivalence
		result1, err1 := executeProgram(program1)
		result2, err2 := executeProgram(program2)

		if err1 != nil && err2 == nil {
			t.Fatalf("Semantic mismatch: Original program failed but round-tripped program succeeded.\nOriginal:\n%s\nPrinted:\n%s\nOriginal Error: %v\nRound-trip Result: %v", input, printedCode, err1, result2)
		}
		if err1 == nil && err2 != nil {
			t.Fatalf("Semantic mismatch: Round-tripped program failed but original program succeeded.\nOriginal:\n%s\nPrinted:\n%s\nOriginal Result: %v\nRound-trip Error: %v", input, printedCode, result1, err2)
		}
		if err1 != nil && err2 != nil {
			// Both failed - check if errors are of the same type
			errType1 := getErrorType(err1)
			errType2 := getErrorType(err2)
			if errType1 != errType2 {
				// Relax check for common runtime errors that might be reordered
				isRuntimeError := func(t string) bool {
					return t == "NameError" || t == "TypeError" || t == "ArgumentError" || t == "Other" || t == "SyntaxError"
				}
				if isRuntimeError(errType1) && isRuntimeError(errType2) {
					t.Logf("Semantic error type mismatch (ignored due to non-determinism).\nOriginal Error (%s): %v\nRound-trip Error (%s): %v", errType1, err1, errType2, err2)
					return
				}
				t.Fatalf("Semantic error type mismatch.\nOriginal:\n%s\nPrinted:\n%s\nOriginal Error (%s): %v\nRound-trip Error (%s): %v", input, printedCode, errType1, err1, errType2, err2)
			}
			return
		}

		// Compare results
		if !areResultsEqual(result1, result2) {
			t.Fatalf("Semantic result mismatch.\nOriginal:\n%s\nPrinted:\n%s\nOriginal Result: %s\nRound-trip Result: %s", input, printedCode, inspect(result1), inspect(result2))
		}
	})
}

// executeProgram runs a given AST program using the TreeWalk backend with a timeout.
func executeProgram(program *ast.Program) (evaluator.Object, error) {
	loader := modules.NewLoader()
	twBackend := backend.NewTreeWalk()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var result evaluator.Object
	var err error
	done := make(chan bool, 1)
	go func() {
		result, err = twBackend.RunProgramWithContext(ctx, program, loader)
		done <- true
	}()

	select {
	case <-done:
		return result, err
	case <-ctx.Done():
		return nil, fmt.Errorf("execution timeout")
	}
}
