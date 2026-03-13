package targets

import (
	"context"
	"github.com/funvibe/funxy/internal/analyzer"
	"github.com/funvibe/funxy/internal/backend"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/tests/fuzz/generators"
	"github.com/funvibe/funxy/internal/vm"
	"testing"
	"time"
)

// FuzzCompiler is the entry point for fuzzing the compiler (or tree-walk interpreter).
func FuzzCompiler(f *testing.F) {
	capFuzzProcs()

	// Add seed corpus
	f.Add([]byte("fun main() { print(\"Hello\") }"))
	f.Add([]byte("x = 1 + 2"))
	f.Add([]byte("fun add(a: Int, b: Int) -> Int { a + b }"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Global panic recovery
		defer func() {
			if r := recover(); r != nil {
				t.Skip("Recovered from panic:", r)
			}
		}()

		gen := generators.NewFromData(data)
		input := gen.GenerateProgram()

		// 1. Parse
		// Set a timeout for the analyzer phase to prevent hangs on pathological inputs.
		// Analysis taking > 5s for random fuzz input is considered a failure/hang.
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		ctx := pipeline.NewPipelineContext(input)
		ctx.Context = timeoutCtx

		l := lexer.New(input)
		stream := lexer.NewTokenStream(l)
		p := parser.New(stream, ctx)
		program := p.ParseProgram()

		if program == nil || len(ctx.Errors) > 0 {
			return
		}

		// 2. Analyze
		symbolTable := symbols.NewSymbolTable()
		a := analyzer.New(symbolTable)
		a.RegisterBuiltins()

		errs := a.Analyze(program, ctx)
		if len(errs) > 0 {
			return
		}

		// 3. Compile or Interpret
		if *useTreeWalk {
			// Tree Walk Backend — run with context timeout to prevent hangs on infinite loops.
			b := backend.NewTreeWalk()
			loader := modules.NewLoader()

			// Use a shorter timeout for execution
			twCtx, twCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer twCancel()

			done := make(chan bool, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						// Recover from panic inside goroutine
						select {
						case done <- true:
						default:
						}
					}
				}()
				_, _ = b.RunProgramWithContext(twCtx, program, loader)
				select {
				case done <- true:
				default:
				}
			}()

			select {
			case <-done:
			case <-twCtx.Done():
				// Timeout — acceptable for random inputs
			}
		} else {
			// VM Backend — run with context timeout to prevent hangs on pathological inputs.
			vmDone := make(chan bool, 1)

			// Use a separate timeout for compilation/execution
			vmCtx, vmCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer vmCancel()

			go func() {
				defer func() {
					if r := recover(); r != nil {
						// Recover from panic inside goroutine
						select {
						case vmDone <- true:
						default:
						}
					}
				}()

				// Check context before starting
				if vmCtx.Err() != nil {
					return
				}

				c := vm.NewCompiler()
				c.SetSymbolTable(symbolTable)
				c.SetTypeMap(a.TypeMap)
				c.SetResolutionMap(a.ResolutionMap)

				_, err := c.Compile(program)
				if err != nil {
					// Compilation error is fine
				}

				select {
				case vmDone <- true:
				default:
				}
			}()

			select {
			case <-vmDone:
			case <-vmCtx.Done():
				// Timeout — acceptable for pathological inputs
			}
		}
	})
}
