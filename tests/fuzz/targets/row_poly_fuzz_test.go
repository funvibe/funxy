package targets

import (
	"github.com/funvibe/funxy/internal/analyzer"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/tests/fuzz/generators"
	"testing"
	"time"
)

func FuzzRowPolymorphism(f *testing.F) {
	f.Add([]byte("chain"))
	f.Add([]byte("recursive"))
	f.Add([]byte("conflict"))

	f.Fuzz(func(t *testing.T, data []byte) {
		sg := generators.NewRowPolyGenerator(data)
		var input string

		// Choose a test type
		choice := sg.Intn(3)
		switch choice {
		case 0:
			input = sg.GenerateChainedRecordOps()
		case 1:
			input = sg.GenerateRecursiveRecord()
		case 2:
			input = sg.GenerateConflictingRecords()
		}

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
		symbolTable.InitBuiltins() // Ensure builtins like 'map' (if not imported) are handled or just standard types
		a := analyzer.New(symbolTable)
		a.RegisterBuiltins()

		// Note: Conflicts (choice 2) SHOULD produce errors.
		// If choice == 2, we expect errors, so if we get them, it's fine.
		// If choice == 0 or 1, we expect NO errors usually, unless random generation created invalid code.
		// But for fuzzing, we mainly care about Panics/Hangs.

		// Use a channel to detect hangs during Analysis (infinite recursion in unifier)
		doneAnalysis := make(chan bool, 1)
		go func() {
			a.Analyze(program)
			doneAnalysis <- true
		}()

		select {
		case <-doneAnalysis:
			// Analysis finished
		case <-time.After(500 * time.Millisecond):
			// Timeout! Likely infinite loop in Unify
			t.Fatalf("Analysis timed out (infinite loop in unification?)\nInput:\n%s", input)
		}

		// If analysis had errors, we stop (unless we want to test compilation of broken code, but that usually fails early)
		// We can't easily check a.Errors here because Analyze returns them.
		// But since we ran it in goroutine, we didn't capture return value.
		// It's fine, the main goal is to catch panics/hangs.

		// 3. Compile and Run (optional, but good for verify execution logic)
		// ... skipping execution to focus on Type System stability ...
	})
}
