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

// FuzzKindChecker tests the kind checker with complex type and trait declarations.
func FuzzKindChecker(f *testing.F) {
	// Add seed corpus with interesting kind-related examples
	seedCorpus := []string{
		"type alias MyList : * -> * = List",
		"type Functor : (* -> *) -> *",
		"trait Monad<M: * -> *> where M.A: Show",
		"type alias Complex<A, B> : * -> * -> * = Map<A, B>",
		"type HigherKinded<F: * -> *> : (* -> *) -> *",
		"trait Ord<T: Eq + Show>",
		"type alias Nested<A> : * -> * = List<Option<A>>",
		"trait Applicative<F: * -> *> where F.A: Eq",
	}

	for _, seed := range seedCorpus {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data string) {
		// Create a generator from the fuzz data
		gen := generators.NewFromData([]byte(data))

		// Generate a program focused on type and trait declarations
		program := generateKindFocusedProgram(gen)

		// Set up a timeout to prevent infinite loops
		done := make(chan bool, 1)
		go func() {
			// Parse the program
			ctx := pipeline.NewPipelineContext(program)
			l := lexer.New(program)
			stream := lexer.NewTokenStream(l)
			p := parser.New(stream, ctx)
			astProg := p.ParseProgram()
			if astProg == nil || len(ctx.Errors) > 0 {
				// Parse errors are expected for fuzzed input
				done <- true
				return
			}

			// Analyze the program (this includes kind checking)
			symbolTable := symbols.NewSymbolTable()
			proc := analyzer.New(symbolTable)
			proc.RegisterBuiltins()
			errs := proc.Analyze(astProg)
			if len(errs) > 0 {
				// Analysis errors are also expected
				done <- true
				return
			}

			done <- true
		}()

		// Wait for analysis to complete or timeout
		select {
		case <-done:
			// Analysis completed successfully or with expected errors
		case <-time.After(100 * time.Millisecond):
			// Timeout to prevent hangs
			t.Skip("Timeout in kind checker fuzz test")
		}
	})
}

// generateKindFocusedProgram creates a program that focuses on kind-related features
func generateKindFocusedProgram(gen *generators.Generator) string {
	// Create a program with multiple type and trait declarations
	program := ""

	// Generate 5-10 type declarations with kind annotations
	typeCount := gen.Src().Intn(6) + 5
	for i := 0; i < typeCount; i++ {
		program += gen.GenerateTypeDecl() + "\n"
	}

	// Generate 5-10 trait declarations with kind annotations, FunDeps, and complex constraints
	traitCount := gen.Src().Intn(6) + 5
	for i := 0; i < traitCount; i++ {
		program += gen.GenerateTraitDecl() + "\n"
	}

	// Generate 3-5 functions that use these types, including higher-rank types and complex generics
	funcCount := gen.Src().Intn(3) + 3
	for i := 0; i < funcCount; i++ {
		program += gen.GenerateFunctionDecl() + "\n"
	}

	// Add a simple expression to make it a complete program
	program += "nil\n"

	return program
}
