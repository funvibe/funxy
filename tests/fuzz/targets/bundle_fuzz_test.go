package targets

import (
	"bytes"
	"context"
	"io"
	"os"
	"github.com/funvibe/funxy/internal/analyzer"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/tests/fuzz/generators"
	"github.com/funvibe/funxy/internal/vm"
	"testing"
	"time"
)

// FuzzBundleRoundTrip tests the serialization roundtrip:
// compile → serialize (Bundle) → deserialize → run, comparing output with direct execution.
// This catches missing gob.Register calls, unexported fields, nil elements in slices, etc.
func FuzzBundleRoundTrip(f *testing.F) {
	capFuzzProcs()

	// Seed corpus
	f.Add([]byte("print(1 + 2)"))
	f.Add([]byte("x = 42\nprint(x)"))
	f.Add([]byte("fun add(a, b) { a + b }\nprint(add(1, 2))"))
	f.Add([]byte("fun greet(name = \"World\") { print(name) }\ngreet()"))
	f.Add([]byte("list = [1, 2, 3]\nfor x in list { print(x) }"))
	f.Add([]byte("i = 0\nfor i < 3 { print(i)\n i = i + 1 }"))
	f.Add([]byte("match Ok(42) { Ok(x) -> print(x), Fail(e) -> print(e) }"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1000 {
			return
		}

		gen := generators.NewFromData(data)
		input := gen.GenerateProgram()

		if len(input) > 10000 {
			return
		}

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

		// 3. Compile
		compiler := vm.NewCompiler()
		compiler.SetSymbolTable(symbolTable)
		compiler.SetTypeMap(a.TypeMap)
		compiler.SetResolutionMap(a.ResolutionMap)

		chunk, err := compiler.Compile(program)
		if err != nil {
			return // Compilation error is expected for some inputs
		}

		// 4. Serialize → Deserialize (the core of this test)
		bundle := &vm.Bundle{
			MainChunk:  chunk,
			Modules:    make(map[string]*vm.BundledModule),
			SourceFile: "fuzz_test.lang",
		}

		serialized, err := bundle.Serialize()
		if err != nil {
			t.Fatalf("Serialize failed.\nInput:\n%s\nError: %v", input, err)
		}

		restored, err := vm.DeserializeAny(serialized)
		if err != nil {
			t.Fatalf("Deserialize failed.\nInput:\n%s\nError: %v", input, err)
		}

		// 5. Run original chunk
		origOutput, origErr := runChunkCaptured(chunk, 200*time.Millisecond)

		// 6. Run deserialized chunk
		restoredOutput, restoredErr := runChunkCaptured(restored.MainChunk, 200*time.Millisecond)

		// 7. Compare
		// If either hit resource limits, skip
		if isResourceExhaustionError(origErr) || isResourceExhaustionError(restoredErr) {
			return
		}

		// Both should either succeed or fail
		if origErr != nil && restoredErr == nil {
			t.Fatalf("Original failed but restored succeeded.\nInput:\n%s\nOriginal Error: %v\nRestored Output: %q", input, origErr, restoredOutput)
		}
		if origErr == nil && restoredErr != nil {
			t.Fatalf("Restored failed but original succeeded.\nInput:\n%s\nOriginal Output: %q\nRestored Error: %v", input, origOutput, restoredErr)
		}

		// Compare stdout output
		if origOutput != restoredOutput {
			t.Fatalf("Output mismatch after bundle roundtrip.\nInput:\n%s\nOriginal:  %q\nRestored: %q", input, origOutput, restoredOutput)
		}
	})
}

// runChunkCaptured runs a chunk on a fresh VM, capturing stdout and returning it.
func runChunkCaptured(chunk *vm.Chunk, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Capture stdout
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}

	var result error
	var output string
	done := make(chan bool, 1)

	go func() {
		os.Stdout = w

		machine := vm.New()
		machine.RegisterBuiltins()
		machine.RegisterFPTraits()
		machine.SetContext(ctx)

		_, runErr := machine.Run(chunk)
		result = runErr

		w.Close()
		done <- true
	}()

	select {
	case <-done:
		os.Stdout = origStdout
		var buf bytes.Buffer
		io.Copy(&buf, r)
		r.Close()
		output = buf.String()
	case <-ctx.Done():
		os.Stdout = origStdout
		w.Close()
		r.Close()
		return "", ctx.Err()
	}

	return output, result
}
