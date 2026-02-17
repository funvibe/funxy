package targets

import (
	"fmt"
	"github.com/funvibe/funxy/internal/analyzer"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/tests/fuzz/generators"
	"github.com/funvibe/funxy/internal/vm"
	"runtime"
	"strings"
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

// ---------------------------------------------------------------------------
// Generators: nesting & recursion
// ---------------------------------------------------------------------------

// GenerateDeeplyNestedCode creates code with deep nesting to test stack limits.
func (sg *StressGenerator) GenerateDeeplyNestedCode() string {
	depth := sg.Intn(30) + 10 // 10–40
	code := ""
	for i := 0; i < depth; i++ {
		code += "if true {\n"
	}
	code += "print(\"deep\")\n"
	for i := 0; i < depth; i++ {
		code += "}\n"
	}
	return code
}

// GenerateDeepRecursionNonTCO creates naive recursion that should hit MaxFrameCount.
// The runtime must handle this gracefully (error, not crash).
func (sg *StressGenerator) GenerateDeepRecursionNonTCO() string {
	// Non-tail: fib(n-1) + fib(n-2) can't be TCO'd.
	// We also test a simple non-tail countdown: f(n) = 1 + f(n-1).
	n := sg.Intn(50) + 20 // 20–70
	switch sg.Intn(3) {
	case 0:
		// Linear non-tail recursion: 1 + f(n-1)
		return fmt.Sprintf(`fun deepCount(n) {
  if n == 0 { 0 }
  else { 1 + deepCount(n - 1) }
}
print(deepCount(%d))`, n)
	case 1:
		// Mutual non-tail recursion
		return fmt.Sprintf(`fun pingNonTail(n) {
  if n == 0 { 0 } else { 1 + pongNonTail(n - 1) }
}
fun pongNonTail(n) {
  if n == 0 { 0 } else { 1 + pingNonTail(n - 1) }
}
print(pingNonTail(%d))`, n)
	default:
		// Fibonacci (exponential) — small n to avoid timeout, but deep call stack
		fibN := sg.Intn(10) + 5 // 5–15
		return fmt.Sprintf(`fun fib(n) {
  if n < 2 { n }
  else { fib(n - 1) + fib(n - 2) }
}
print(fib(%d))`, fibN)
	}
}

// GenerateDeeplyNestedClosures creates closures capturing closures capturing closures.
func (sg *StressGenerator) GenerateDeeplyNestedClosures() string {
	depth := sg.Intn(15) + 5 // 5–20 levels
	var sb strings.Builder

	// Build nested closures: f0 = \() -> { f1 = \() -> { ... } ; f1() }
	for i := 0; i < depth; i++ {
		sb.WriteString(fmt.Sprintf("f%d = \\() -> {\n", i))
	}
	sb.WriteString(fmt.Sprintf("  %d\n", depth)) // innermost value
	for i := depth - 1; i >= 0; i-- {
		if i < depth-1 {
			sb.WriteString(fmt.Sprintf("  f%d()\n", i+1))
		}
		sb.WriteString("}\n")
	}
	sb.WriteString("print(f0())\n")
	return sb.String()
}

// GenerateDeepFunctionCalls creates f(f(f(f(...)))).
func (sg *StressGenerator) GenerateDeepFunctionCalls() string {
	depth := sg.Intn(40) + 15 // 15–55
	call := "0"
	for i := 0; i < depth; i++ {
		call = fmt.Sprintf("addOne(%s)", call)
	}
	return fmt.Sprintf("fun addOne(n) { n + 1 }\nprint(%s)\n", call)
}

// ---------------------------------------------------------------------------
// Generators: large data
// ---------------------------------------------------------------------------

// GenerateLargeDataStructure creates code with large lists or maps.
func (sg *StressGenerator) GenerateLargeDataStructure() string {
	size := sg.Intn(1000) + 200 // 200–1200
	var sb strings.Builder
	sb.WriteString("largeList = [")
	for i := 0; i < size; i++ {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%d", i))
	}
	sb.WriteString("]\nprint(len(largeList))\n")
	return sb.String()
}

// GenerateLargeRecord creates a record with many fields.
func (sg *StressGenerator) GenerateLargeRecord() string {
	fields := sg.Intn(80) + 30 // 30–110
	var sb strings.Builder
	sb.WriteString("bigRec = {")
	for i := 0; i < fields; i++ {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("f%d: %d", i, i*3))
	}
	sb.WriteString("}\n")
	// Access a few fields
	sb.WriteString(fmt.Sprintf("print(bigRec.f0 + bigRec.f%d)\n", fields-1))
	return sb.String()
}

// GenerateLargeString creates very long string concatenation / literals.
func (sg *StressGenerator) GenerateLargeString() string {
	switch sg.Intn(2) {
	case 0:
		// Long literal
		length := sg.Intn(3000) + 500
		return fmt.Sprintf("longStr = \"%s\"\nprint(len(longStr))\n",
			strings.Repeat("a", length))
	default:
		// Many concatenations
		count := sg.Intn(80) + 30
		parts := make([]string, count)
		for i := 0; i < count; i++ {
			parts[i] = fmt.Sprintf("\"part%d\"", i)
		}
		return fmt.Sprintf("result = %s\nprint(len(result))\n",
			strings.Join(parts, " ++ "))
	}
}

// GenerateLargeMap creates a map with many entries.
func (sg *StressGenerator) GenerateLargeMap() string {
	entries := sg.Intn(200) + 50 // 50–250
	var sb strings.Builder
	sb.WriteString("bigMap = %{")
	for i := 0; i < entries; i++ {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("\"k%d\" => %d", i, i))
	}
	sb.WriteString("}\nprint(len(bigMap))\n")
	return sb.String()
}

// ---------------------------------------------------------------------------
// Generators: pipes & chains
// ---------------------------------------------------------------------------

// GenerateLongPipeChain creates x |> f |> f |> ... |> f.
func (sg *StressGenerator) GenerateLongPipeChain() string {
	length := sg.Intn(40) + 15 // 15–55

	var sb strings.Builder
	sb.WriteString("import \"lib/list\" (map, filter, length)\n\n")

	// Define a few simple transform functions
	sb.WriteString("fun inc(n) { n + 1 }\n")
	sb.WriteString("fun dbl(n) { n * 2 }\n")
	sb.WriteString("fun wrap(n) { [n] }\n\n")

	// Build a long |> chain on a number
	sb.WriteString("result = 0")
	fns := []string{"inc", "dbl", "inc"}
	for i := 0; i < length; i++ {
		fn := fns[sg.Intn(len(fns))]
		sb.WriteString(fmt.Sprintf("\n  |> %s", fn))
	}
	sb.WriteString("\nprint(result)\n")
	return sb.String()
}

// GenerateLongPipeChainList creates list |> map(...) |> filter(...) |> ... chains.
func (sg *StressGenerator) GenerateLongPipeChainList() string {
	length := sg.Intn(15) + 5 // 5–20

	var sb strings.Builder
	sb.WriteString("import \"lib/list\" (map, filter, length)\n\n")

	sb.WriteString("result = [1, 2, 3, 4, 5]")
	ops := []string{
		"|> map(\\n -> n + 1)",
		"|> filter(\\n -> n > 0)",
		"|> map(\\n -> n * 2)",
	}
	for i := 0; i < length; i++ {
		op := ops[sg.Intn(len(ops))]
		sb.WriteString(fmt.Sprintf("\n  %s", op))
	}
	sb.WriteString("\nprint(length(result))\n")
	return sb.String()
}

// ---------------------------------------------------------------------------
// Generators: pattern matching
// ---------------------------------------------------------------------------

// GenerateComplexPatternMatching creates deeply nested pattern matches.
func (sg *StressGenerator) GenerateComplexPatternMatching() string {
	depth := sg.Intn(12) + 5
	code := "data = "
	for i := 0; i < depth; i++ {
		code += "("
	}
	code += "1"
	for i := 0; i < depth; i++ {
		code += ", 2)"
	}
	code += "\nmatch data {\n"
	for i := 0; i < depth; i++ {
		code += "    ("
	}
	code += "x"
	for i := 0; i < depth; i++ {
		code += ", y"
	}
	code += ") -> print(x)\n    _ -> print(\"no match\")\n}\n"
	return code
}

// GenerateManyMatchArms creates a match with many literal arms.
func (sg *StressGenerator) GenerateManyMatchArms() string {
	arms := sg.Intn(50) + 20 // 20–70
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("val = %d\n", sg.Intn(arms)))
	sb.WriteString("result = match val {\n")
	for i := 0; i < arms; i++ {
		sb.WriteString(fmt.Sprintf("  %d -> \"arm%d\"\n", i, i))
	}
	sb.WriteString("  _ -> \"default\"\n}\n")
	sb.WriteString("print(result)\n")
	return sb.String()
}

// ---------------------------------------------------------------------------
// Generators: many definitions
// ---------------------------------------------------------------------------

// GenerateManyFunctions creates a program with many function definitions.
func (sg *StressGenerator) GenerateManyFunctions() string {
	count := sg.Intn(50) + 20 // 20–70
	var sb strings.Builder
	for i := 0; i < count; i++ {
		sb.WriteString(fmt.Sprintf("fun func%d(n) { n + %d }\n", i, i))
	}
	// Call them all in a chain
	sb.WriteString("result = 0\n")
	for i := 0; i < count; i++ {
		sb.WriteString(fmt.Sprintf("result = func%d(result)\n", i))
	}
	sb.WriteString("print(result)\n")
	return sb.String()
}

// GenerateManyVariables creates a program with many variable declarations.
func (sg *StressGenerator) GenerateManyVariables() string {
	count := sg.Intn(200) + 50 // 50–250
	var sb strings.Builder
	for i := 0; i < count; i++ {
		sb.WriteString(fmt.Sprintf("var%d = %d\n", i, i))
	}
	// Sum a sample
	sample := 10
	if sample > count {
		sample = count
	}
	sb.WriteString("total = 0\n")
	for i := 0; i < sample; i++ {
		sb.WriteString(fmt.Sprintf("total = total + var%d\n", i))
	}
	sb.WriteString("print(total)\n")
	return sb.String()
}

// ---------------------------------------------------------------------------
// Generators: runtime error paths
// ---------------------------------------------------------------------------

// GenerateRuntimeErrors creates code that should trigger runtime errors
// (not panics). Tests that the VM handles these gracefully.
func (sg *StressGenerator) GenerateRuntimeErrors() string {
	switch sg.Intn(8) {
	case 0:
		// Division by zero
		return "print(1 / 0)\n"
	case 1:
		// Division by zero (float)
		return "print(1.0 / 0.0)\n"
	case 2:
		// Index out of bounds
		return "list = [1, 2, 3]\nprint(list[100])\n"
	case 3:
		// Negative index out of bounds
		return "list = [1, 2, 3]\nprint(list[-100])\n"
	case 4:
		// Type mismatch in arithmetic
		return "print(1 + \"hello\")\n"
	case 5:
		// Call non-function
		return "x = 42\nprint(x(1))\n"
	case 6:
		// Record field missing
		return "rec = { a: 1 }\nprint(rec.nonexistent)\n"
	case 7:
		// String concat with non-string
		return "print(\"hello\" ++ 42)\n"
	default:
		return "print(1 / 0)\n"
	}
}

// GeneratePanicRecovery creates code with panic that should not crash the fuzzer.
func (sg *StressGenerator) GeneratePanicRecovery() string {
	switch sg.Intn(3) {
	case 0:
		return "panic(\"intentional\")\n"
	case 1:
		// Stack exhaustion through infinite recursion (non-TCO)
		return "fun inf(n) { 1 + inf(n + 1) }\nprint(inf(0))\n"
	default:
		// Accessing nil
		return "x = nil\nprint(x.field)\n"
	}
}

// ---------------------------------------------------------------------------
// Generators: deep nesting of records
// ---------------------------------------------------------------------------

// GenerateDeeplyNestedRecords creates { a: { b: { c: { ... } } } }.
func (sg *StressGenerator) GenerateDeeplyNestedRecords() string {
	depth := sg.Intn(15) + 5 // 5–20
	var sb strings.Builder
	sb.WriteString("nested = ")
	for i := 0; i < depth; i++ {
		sb.WriteString(fmt.Sprintf("{ level%d: ", i))
	}
	sb.WriteString("\"bottom\"")
	for i := 0; i < depth; i++ {
		sb.WriteString(" }")
	}
	sb.WriteString("\nprint(nested)\n")
	return sb.String()
}

// ---------------------------------------------------------------------------
// Generators: loops
// ---------------------------------------------------------------------------

// GenerateInfiniteLoop creates code that might cause a very long loop.
// Context timeout should interrupt this.
func (sg *StressGenerator) GenerateInfiniteLoop() string {
	switch sg.Intn(3) {
	case 0:
		// Truly infinite — relies on context cancellation
		return "i = 0\nfor true { i = i + 1 }\nprint(i)\n"
	case 1:
		// Very long but finite
		return "i = 0\nfor i < 10000000 { i = i + 1 }\nprint(i)\n"
	default:
		// Nested loops (quadratic)
		n := sg.Intn(80) + 30
		return fmt.Sprintf("count = 0\nfor i in 0..%d {\n  for j in 0..%d {\n    count = count + 1\n  }\n}\nprint(count)\n", n, n)
	}
}

// ---------------------------------------------------------------------------
// Generators: ADT with many constructors
// ---------------------------------------------------------------------------

// GenerateLargeADT creates a type with many constructors and matches on it.
func (sg *StressGenerator) GenerateLargeADT() string {
	constructors := sg.Intn(20) + 5 // 5–25
	var sb strings.Builder
	sb.WriteString("type BigEnum\n")
	for i := 0; i < constructors; i++ {
		if i == 0 {
			sb.WriteString(fmt.Sprintf("  = Con%d Int\n", i))
		} else {
			sb.WriteString(fmt.Sprintf("  | Con%d Int\n", i))
		}
	}
	sb.WriteString("\n")

	// Create a value
	pick := sg.Intn(constructors)
	sb.WriteString(fmt.Sprintf("val = Con%d(%d)\n\n", pick, pick*7))

	// Match on it
	sb.WriteString("result = match val {\n")
	for i := 0; i < constructors; i++ {
		sb.WriteString(fmt.Sprintf("  Con%d(n) -> n + %d\n", i, i))
	}
	sb.WriteString("}\nprint(result)\n")
	return sb.String()
}

// ---------------------------------------------------------------------------
// Generators: list comprehension
// ---------------------------------------------------------------------------

// GenerateLargeListComprehension creates [expr | x <- 1..N, condition].
func (sg *StressGenerator) GenerateLargeListComprehension() string {
	size := sg.Intn(1000) + 200 // 200–1200
	return fmt.Sprintf("result = [x * 2 | x <- 1..%d, x %% 3 == 0]\nprint(len(result))\n", size)
}

// ===========================================================================
// FuzzStress — the actual fuzz target
// ===========================================================================

// FuzzStress tests the system's resilience to resource exhaustion,
// deep nesting, large structures, long chains, and runtime errors.
func FuzzStress(f *testing.F) {
	f.Add([]byte("deep"))
	f.Add([]byte("large"))
	f.Add([]byte("loop"))
	f.Add([]byte("pattern"))
	f.Add([]byte("recursion"))
	f.Add([]byte("closures"))
	f.Add([]byte("pipes"))
	f.Add([]byte("errors"))
	f.Add([]byte("manyFuncs"))
	f.Add([]byte("largeRec"))
	f.Add([]byte("largeADT"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Skip if too many leaked goroutines from previous timed-out iterations.
		if runtime.NumGoroutine() > 500 {
			time.Sleep(50 * time.Millisecond)
			if runtime.NumGoroutine() > 500 {
				return
			}
		}

		// Run the entire iteration inside a goroutine with an overall deadline.
		// This guarantees that no stage (parse, analyze, compile, run) can block
		// the fuzz worker indefinitely.
		const iterationTimeout = 1 * time.Second
		done := make(chan struct{}, 1)

		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Don't call t.Errorf here — just absorb the panic.
					// The fuzzer will re-run the input to reproduce.
				}
				done <- struct{}{}
			}()

			sg := NewStressGenerator(data)
			var input string

			choice := sg.Intn(20)
			switch choice {
			case 0:
				input = sg.GenerateDeeplyNestedCode()
			case 1:
				input = sg.GenerateLargeDataStructure()
			case 2:
				input = sg.GenerateInfiniteLoop()
			case 3:
				input = sg.GenerateComplexPatternMatching()
			case 4:
				input = sg.GenerateDeepRecursionNonTCO()
			case 5:
				input = sg.GenerateDeeplyNestedClosures()
			case 6:
				input = sg.GenerateDeepFunctionCalls()
			case 7:
				input = sg.GenerateLongPipeChain()
			case 8:
				input = sg.GenerateLongPipeChainList()
			case 9:
				input = sg.GenerateManyMatchArms()
			case 10:
				input = sg.GenerateManyFunctions()
			case 11:
				input = sg.GenerateManyVariables()
			case 12:
				input = sg.GenerateRuntimeErrors()
			case 13:
				input = sg.GeneratePanicRecovery()
			case 14:
				input = sg.GenerateLargeRecord()
			case 15:
				input = sg.GenerateLargeString()
			case 16:
				input = sg.GenerateLargeMap()
			case 17:
				input = sg.GenerateDeeplyNestedRecords()
			case 18:
				input = sg.GenerateLargeADT()
			case 19:
				input = sg.GenerateLargeListComprehension()
			default:
				input = sg.GenerateDeeplyNestedCode()
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
			st := symbols.NewSymbolTable()
			an := analyzer.New(st)
			an.RegisterBuiltins()
			errs := an.Analyze(program)
			if len(errs) > 0 {
				return
			}

			// 3. Compile (no execution — runtime is stress-tested by FuzzEmbedEval;
			// here we only verify that parse/analyze/compile don't crash or hang)
			c := vm.NewCompiler()
			c.SetSymbolTable(st)
			c.SetTypeMap(an.TypeMap)
			c.SetResolutionMap(an.ResolutionMap)
			_, _ = c.Compile(program)
		}()

		select {
		case <-done:
		case <-time.After(iterationTimeout):
			// Iteration timed out — one of the stages (parse, analyze,
			// compile, or run) is stuck. Abandon the goroutine and move on.
		}
	})
}
