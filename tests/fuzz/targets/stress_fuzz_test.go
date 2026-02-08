package targets

import (
	"context"
	"fmt"
	"github.com/funvibe/funxy/internal/analyzer"
	"github.com/funvibe/funxy/internal/backend"
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

// StressGenerator generates code specifically for stress testing.
type StressGenerator struct {
	*generators.Generator
}

func NewStressGenerator(data []byte) *StressGenerator {
	return &StressGenerator{
		Generator: generators.NewFromData(data),
	}
}

// Intn exposes the random source's Intn method.
func (sg *StressGenerator) Intn(n int) int {
	return sg.Generator.Src().Intn(n)
}

// GenerateDeeplyNestedCode creates code with deep nesting to test stack limits.
func (sg *StressGenerator) GenerateDeeplyNestedCode() string {
	depth := sg.Intn(20) + 10 // Generate depth between 10 and 30
	code := "fun main() {\n"
	for i := 0; i < depth; i++ {
		code += fmt.Sprintf("  if true {\n")
	}
	code += "    print(\"deep\")\n"
	for i := 0; i < depth; i++ {
		code += "  }\n"
	}
	code += "}"
	return code
}

// GenerateLargeDataStructure creates code with large lists or maps.
func (sg *StressGenerator) GenerateLargeDataStructure() string {
	size := sg.Intn(5000) + 1000 // Generate size between 1000 and 6000
	code := "fun main() {\n  let large_list = ["
	for i := 0; i < size; i++ {
		if i > 0 {
			code += ", "
		}
		code += fmt.Sprintf("%d", i)
	}
	code += "]\n  print(len(large_list))\n}"
	return code
}

// GenerateInfiniteLoop creates code that might cause an infinite loop.
func (sg *StressGenerator) GenerateInfiniteLoop() string {
	// A loop that is hard to break out of
	return "fun main() {\n  let mut i = 0\n  while i < 1000000 {\n    i = i + 1\n  }\n  print(\"done\")\n}"
}

// GenerateComplexPatternMatching creates deeply nested pattern matches.
func (sg *StressGenerator) GenerateComplexPatternMatching() string {
	depth := sg.Intn(10) + 5
	code := "fun main() {\n  let data = "
	for i := 0; i < depth; i++ {
		code += "("
	}
	code += "1"
	for i := 0; i < depth; i++ {
		code += ", 2)"
	}
	code += "\n  match data {\n"
	for i := 0; i < depth; i++ {
		code += "    ("
	}
	code += "x"
	for i := 0; i < depth; i++ {
		code += ", y"
	}
	code += ") -> print(x)\n    _ -> print(\"no match\")\n  }\n}"
	return code
}

// FuzzStress tests the system's resilience to resource exhaustion.
func FuzzStress(f *testing.F) {
	f.Add([]byte("deep"))
	f.Add([]byte("large"))
	f.Add([]byte("loop"))
	f.Add([]byte("pattern"))

	f.Fuzz(func(t *testing.T, data []byte) {
		sg := NewStressGenerator(data)
		var input string

		// Choose a stress test type based on input
		choice := sg.Intn(4)
		switch choice {
		case 0:
			input = sg.GenerateDeeplyNestedCode()
		case 1:
			input = sg.GenerateLargeDataStructure()
		case 2:
			input = sg.GenerateInfiniteLoop()
		case 3:
			input = sg.GenerateComplexPatternMatching()
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
		a := analyzer.New(symbolTable)
		a.RegisterBuiltins()
		errs := a.Analyze(program)
		if len(errs) > 0 {
			return // Analysis failed, skip
		}

		// 3. Run with TreeWalk (with context timeout)
		// Context cancellation ensures the evaluator actually stops, not just the select.
		// Buffered channel prevents goroutine leak if it finishes after we move on.
		loader := modules.NewLoader()
		twBackend := backend.NewTreeWalk()

		twCtx, twCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)

		done := make(chan bool, 1)
		go func() {
			_, _ = twBackend.RunProgramWithContext(twCtx, program, loader)
			done <- true
		}()

		select {
		case <-done:
			twCancel()
		case <-twCtx.Done():
			twCancel()
			return
		}

		// 4. Compile and Run with VM (with context timeout)
		c := vm.NewCompiler()
		c.SetSymbolTable(symbolTable)
		c.SetTypeMap(a.TypeMap)
		c.SetResolutionMap(a.ResolutionMap)

		bytecode, err := c.Compile(program)
		if err != nil {
			return // Compilation failed, skip
		}

		vmCtx, vmCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)

		virtualMachine := vm.New()
		virtualMachine.SetLoader(loader)
		virtualMachine.RegisterBuiltins()
		virtualMachine.SetContext(vmCtx)

		doneVM := make(chan bool, 1)
		go func() {
			_, _ = virtualMachine.Run(bytecode)
			doneVM <- true
		}()

		select {
		case <-doneVM:
			vmCancel()
		case <-vmCtx.Done():
			vmCancel()
			return
		}
	})
}
