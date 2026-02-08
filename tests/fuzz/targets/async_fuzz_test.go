package targets

import (
	"context"
	"github.com/funvibe/funxy/internal/analyzer"
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

// FuzzAsync tests the system's resilience to heavy async/await usage.
func FuzzAsync(f *testing.F) {
	f.Add([]byte("seed"))
	f.Add([]byte("parallel"))
	f.Add([]byte("recursive"))

	f.Fuzz(func(t *testing.T, data []byte) {
		gen := generators.NewAsyncGenerator(data)
		input := gen.GenerateAsyncProgram()

		// 1. Parse
		ctx := pipeline.NewPipelineContext(input)
		l := lexer.New(input)
		stream := lexer.NewTokenStream(l)
		p := parser.New(stream, ctx)
		program := p.ParseProgram()

		if program == nil || len(ctx.Errors) > 0 {
			return // Invalid generated code, skip
		}

		// 2. Analyze
		symbolTable := symbols.NewSymbolTable()
		a := analyzer.New(symbolTable)
		a.RegisterBuiltins()
		errs := a.Analyze(program)
		if len(errs) > 0 {
			return // Analysis failed, skip
		}

		// 3. Compile and Run with VM
		c := vm.NewCompiler()
		c.SetSymbolTable(symbolTable)
		c.SetTypeMap(a.TypeMap)
		c.SetResolutionMap(a.ResolutionMap)

		bytecode, err := c.Compile(program)
		if err != nil {
			return // Compilation failed, skip
		}

		loader := modules.NewLoader()

		// Context timeout ensures the VM actually stops executing, not just the select.
		vmCtx, vmCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)

		virtualMachine := vm.New()
		virtualMachine.SetLoader(loader)
		virtualMachine.RegisterBuiltins()
		virtualMachine.SetContext(vmCtx)

		// Buffered channel prevents goroutine leak on timeout.
		done := make(chan bool, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("VM panic: %v", r)
				}
				done <- true
			}()
			_, _ = virtualMachine.Run(bytecode)
		}()

		select {
		case <-done:
			vmCancel()
		case <-vmCtx.Done():
			vmCancel()
			return
		}
	})
}
