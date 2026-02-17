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
		gen := generators.NewFromData(data)
		input := gen.GenerateProgram()

		// 1. Parse
		ctx := pipeline.NewPipelineContext(input)
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

		errs := a.Analyze(program)
		if len(errs) > 0 {
			return
		}

		// 3. Compile or Interpret
		if *useTreeWalk {
			// Tree Walk Backend — run with context timeout to prevent hangs on infinite loops.
			b := backend.NewTreeWalk()
			loader := modules.NewLoader()

			twCtx, twCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer twCancel()

			done := make(chan bool, 1)
			go func() {
				_, _ = b.RunProgramWithContext(twCtx, program, loader)
				done <- true
			}()

			select {
			case <-done:
			case <-twCtx.Done():
				// Timeout — acceptable for random inputs
			}
		} else {
			// VM Backend — run with context timeout to prevent hangs on pathological inputs.
			vmDone := make(chan bool, 1)
			go func() {
				c := vm.NewCompiler()
				c.SetSymbolTable(symbolTable)
				c.SetTypeMap(a.TypeMap)
				c.SetResolutionMap(a.ResolutionMap)

				_, _ = c.Compile(program)
				vmDone <- true
			}()

			select {
			case <-vmDone:
			case <-time.After(2 * time.Second):
				// Timeout — acceptable for pathological inputs
			}
		}
	})
}
