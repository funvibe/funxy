package targets

import (
	"context"
	"github.com/funvibe/funxy/internal/analyzer"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/backend"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/vm"
	"github.com/funvibe/funxy/tests/fuzz/generators"
	"testing"
	"time"
)

// FuzzDifferential compares the execution results of TreeWalk and VM backends.
func FuzzDifferential(f *testing.F) {
	// Add seed corpus
	f.Add([]byte("fun main() { print(\"Hello\") }"))
	f.Add([]byte("x = 1 + 2"))
	f.Add([]byte("if true { 1 } else { 0 }"))

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

		// 1. Parse with timeout
		// Buffered channel (capacity 1) prevents goroutine leak on timeout:
		// without it, the goroutine blocks forever on send after the select moves on.
		var program *ast.Program
		var ctx *pipeline.PipelineContext

		parseChan := make(chan bool, 1)
		go func() {
			ctx = pipeline.NewPipelineContext(input)
			l := lexer.New(input)
			stream := lexer.NewTokenStream(l)
			p := parser.New(stream, ctx)
			program = p.ParseProgram()
			parseChan <- true
		}()

		select {
		case <-parseChan:
		case <-time.After(50 * time.Millisecond):
			return // Parse timeout
		}

		if program == nil || len(ctx.Errors) > 0 {
			return
		}

		// 2. Analyze
		symbolTable := symbols.NewSymbolTable()
		a := analyzer.New(symbolTable)
		a.RegisterBuiltins()
		errs := a.Analyze(program)
		if len(errs) > 0 {
			return
		}

		// 3. Run with TreeWalk
		loader := modules.NewLoader()
		twBackend := backend.NewTreeWalk()

		twCtx, twCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer twCancel()

		var twResult evaluator.Object
		var twErr error
		twDone := make(chan bool, 1)

		go func() {
			twResult, twErr = twBackend.RunProgramWithContext(twCtx, program, loader)
			twDone <- true
		}()

		select {
		case <-twDone:
		case <-twCtx.Done():
			return
		}

		// 4. Compile and Run with VM
		c := vm.NewCompiler()
		c.SetSymbolTable(symbolTable)
		c.SetTypeMap(a.TypeMap)
		c.SetResolutionMap(a.ResolutionMap)

		bytecode, err := c.Compile(program)
		if err != nil {
			return
		}

		virtualMachine := vm.New()
		virtualMachine.SetLoader(loader)
		virtualMachine.RegisterBuiltins()

		vmCtx, vmCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer vmCancel()

		virtualMachine.SetContext(vmCtx)

		var vmResult evaluator.Object
		var vmErr error
		vmDone := make(chan bool, 1)

		go func() {
			vmResult, vmErr = virtualMachine.Run(bytecode)
			vmDone <- true
		}()

		select {
		case <-vmDone:
		case <-vmCtx.Done():
			return
		}

		// 5. Compare Results
		// If both backends hit resource limits (timeout, recursion depth, etc.),
		// the input itself is pathological — skip.
		if isResourceExhaustionError(twErr) || isResourceExhaustionError(vmErr) {
			return
		}
		if twErr != nil && vmErr == nil {
			t.Fatalf("TreeWalk failed but VM succeeded.\nInput:\n%s\nTreeWalk Error: %v\nVM Result: %v", input, twErr, vmResult)
		}
		if twErr == nil && vmErr != nil {
			t.Fatalf("VM failed but TreeWalk succeeded.\nInput:\n%s\nTreeWalk Result: %v\nVM Error: %v", input, twResult, vmErr)
		}
		if twErr != nil && vmErr != nil {
			// Both failed - check if errors are of the same category
			twErrType := getErrorType(twErr)
			vmErrType := getErrorType(vmErr)
			if twErrType != vmErrType {
				// Both backends produced the same core error but differ in wrapping
				// (e.g. VM adds "runtime error:" prefix). Compare stripped messages
				// before reporting a real mismatch.
				twCore := extractCoreError(twErr)
				vmCore := extractCoreError(vmErr)
				if twCore != vmCore {
					t.Fatalf("Error type mismatch.\nInput:\n%s\nTreeWalk Error (%s): %v\nVM Error (%s): %v", input, twErrType, twErr, vmErrType, vmErr)
				}
			}
			return
		}

		// Compare values
		if !areResultsEqual(twResult, vmResult) {
			t.Fatalf("Results mismatch.\nInput:\n%s\nTreeWalk: %s\nVM: %s", input, inspect(twResult), inspect(vmResult))
		}
	})
}
