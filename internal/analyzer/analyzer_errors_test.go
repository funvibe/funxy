package analyzer

import (
	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/token"
	"strings"
	"testing"
)

// analyzeSource is defined in strict_mode_test.go — reuse it.
// It lexes, parses, then analyzes the input, returning all errors.

// expectAnalyzerError asserts that at least one error with the given code is produced.
func expectAnalyzerError(t *testing.T, input string, code diagnostics.ErrorCode) error {
	t.Helper()
	errs := analyzeSource(input)
	if len(errs) == 0 {
		t.Fatalf("expected error %s, but got none\ninput: %s", code, input)
	}
	for _, e := range errs {
		if de, ok := e.(*diagnostics.DiagnosticError); ok {
			if de.Code == code {
				return e
			}
		}
	}
	var msgs []string
	for _, e := range errs {
		msgs = append(msgs, e.Error())
	}
	t.Fatalf("expected error %s, got:\n%s\ninput: %s", code, strings.Join(msgs, "\n"), input)
	return nil
}

// expectAnalyzerErrorContains asserts an error with the given code whose message contains substr.
func expectAnalyzerErrorContains(t *testing.T, input string, code diagnostics.ErrorCode, substr string) {
	t.Helper()
	e := expectAnalyzerError(t, input, code)
	if !strings.Contains(e.Error(), substr) {
		t.Errorf("expected error message to contain %q, got: %s", substr, e.Error())
	}
}

// expectNoAnalyzerErrors asserts that analysis produces no errors.
func expectNoAnalyzerErrors(t *testing.T, input string) {
	t.Helper()
	errs := analyzeSource(input)
	if len(errs) > 0 {
		var msgs []string
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}
		t.Fatalf("expected no errors, got:\n%s\ninput: %s", strings.Join(msgs, "\n"), input)
	}
}

// ---------------------------------------------------------------------------
// A001 — Undeclared variable
//
// A001 is primarily triggered by module/import resolution (undefined module,
// exported symbol not found, unknown trait in instance). In unit tests without
// module loading, undefined constructors in patterns are caught by the
// inference layer as A003 instead.
// ---------------------------------------------------------------------------

func TestA001_UndefinedConstructorReportedAsA003(t *testing.T) {
	// Documents actual behavior: undefined constructor in match goes through
	// the inference layer and surfaces as A003 ("type error: undefined symbol: Foo"),
	// not A001. A001 requires module/import context.
	input := `
fun f(x: Int) -> Int {
    match x {
        Foo -> 0
        _ -> 1
    }
}
`
	e := expectAnalyzerError(t, input, diagnostics.ErrA003)
	if !strings.Contains(e.Error(), "Foo") {
		t.Errorf("expected error to mention 'Foo', got: %s", e.Error())
	}
}

// ---------------------------------------------------------------------------
// A002 — Undeclared type
// ---------------------------------------------------------------------------

func TestA002_UndeclaredTypeInAnnotation(t *testing.T) {
	input := `
fun f(x: Foo) -> Foo { x }
`
	expectAnalyzerError(t, input, diagnostics.ErrA002)
}

func TestA002_UndeclaredTypeInAlias(t *testing.T) {
	input := `
type alias MyType = Foo
`
	expectAnalyzerError(t, input, diagnostics.ErrA002)
}

// ---------------------------------------------------------------------------
// A003 — Type error
// ---------------------------------------------------------------------------

func TestA003_TypeMismatchReturnType(t *testing.T) {
	input := `
fun f() -> Int { "hello" }
`
	expectAnalyzerError(t, input, diagnostics.ErrA003)
}

func TestA003_ConstReassignment(t *testing.T) {
	// Constants cannot be reassigned
	input := `
x :- 10
x = 20
`
	expectAnalyzerError(t, input, diagnostics.ErrA003)
}

func TestA003_BreakOutsideLoop(t *testing.T) {
	input := `
fun f() { break }
`
	expectAnalyzerError(t, input, diagnostics.ErrA003)
}

func TestA003_ContinueOutsideLoop(t *testing.T) {
	input := `
fun f() { continue }
`
	expectAnalyzerError(t, input, diagnostics.ErrA003)
}

// ---------------------------------------------------------------------------
// A004 — Redefinition of symbol
// ---------------------------------------------------------------------------

func TestA004_FunctionRedefinition(t *testing.T) {
	input := `
fun f() -> Int { 1 }
fun f() -> Int { 2 }
`
	expectAnalyzerError(t, input, diagnostics.ErrA004)
}

func TestA004_ConstRedefinition(t *testing.T) {
	input := `
x :- 1
x :- 2
`
	expectAnalyzerError(t, input, diagnostics.ErrA004)
}

func TestA004_TypeRedefinition(t *testing.T) {
	input := `
type alias Foo = Int
type alias Foo = String
`
	expectAnalyzerError(t, input, diagnostics.ErrA004)
}

// ---------------------------------------------------------------------------
// A005 — Type mismatch in assignment (used for fundep validation)
// ---------------------------------------------------------------------------

func TestA005_FundepUnknownTypeVariable(t *testing.T) {
	// Functional dependency references a type variable not in the trait's params.
	// Previously used A005 (template mismatch bug: expected 2 args, got 1).
	// Fixed to use A003. This test verifies the fix.
	input := `
trait Convert<a, b> | a -> c {
    fun convert(x: a) -> b
}
`
	e := expectAnalyzerError(t, input, diagnostics.ErrA003)
	if !strings.Contains(e.Error(), "unknown type variable") {
		t.Errorf("expected 'unknown type variable' message, got: %s", e.Error())
	}
	// Verify the message is well-formed (no %!s(MISSING))
	if strings.Contains(e.Error(), "MISSING") {
		t.Errorf("malformed error message (template bug): %s", e.Error())
	}
}

// ---------------------------------------------------------------------------
// A006 — Undefined symbol
// ---------------------------------------------------------------------------

func TestA006_UndefinedVariable(t *testing.T) {
	input := `
y = unknownVar
`
	expectAnalyzerError(t, input, diagnostics.ErrA006)
}

func TestA006_UndefinedInExpression(t *testing.T) {
	input := `
fun f() -> Int { x + 1 }
`
	expectAnalyzerError(t, input, diagnostics.ErrA006)
}

// ---------------------------------------------------------------------------
// A007 — Match not exhaustive
// ---------------------------------------------------------------------------

func TestA007_NonExhaustiveBool(t *testing.T) {
	input := `
fun f(x: Bool) -> Int {
    match x {
        true -> 1
    }
}
`
	expectAnalyzerError(t, input, diagnostics.ErrA007)
}

func TestA007_NonExhaustiveADT(t *testing.T) {
	// Use a unique name to avoid conflict with builtins
	input := `
type MyOpt = MySome(Int) | MyNone

fun f(x: MyOpt) -> Int {
    match x {
        MySome(n) -> n
    }
}
`
	expectAnalyzerError(t, input, diagnostics.ErrA007)
}

// ---------------------------------------------------------------------------
// A008 — Naming convention
//
// In practice, A008 is hard to trigger through source code because:
// - Uppercase identifiers in patterns are parsed as ConstructorPattern (not IdentifierPattern)
// - In expression-to-pattern conversion (tuple/list destructuring), uppercase identifiers
//   create IdentifierPatterns, but the parser catches them earlier as P006
//
// We test the underlying checkValueName/checkTypeName helpers directly.
// ---------------------------------------------------------------------------

func TestA008_CheckValueName(t *testing.T) {
	// Unit test for the checkValueName helper
	tok := token.Token{Type: token.IDENT_UPPER, Line: 1, Column: 1, Lexeme: "Bad"}
	var errs []*diagnostics.DiagnosticError
	ok := checkValueName("Bad", tok, &errs)
	if ok {
		t.Fatal("expected checkValueName to return false for uppercase name")
	}
	if len(errs) == 0 {
		t.Fatal("expected A008 error")
	}
	if errs[0].Code != diagnostics.ErrA008 {
		t.Errorf("expected A008, got %s", errs[0].Code)
	}
}

func TestA008_CheckValueNameValid(t *testing.T) {
	tok := token.Token{Type: token.IDENT_LOWER, Line: 1, Column: 1, Lexeme: "good"}
	var errs []*diagnostics.DiagnosticError
	ok := checkValueName("good", tok, &errs)
	if !ok {
		t.Fatal("expected checkValueName to return true for lowercase name")
	}
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestA008_CheckTypeName(t *testing.T) {
	tok := token.Token{Type: token.IDENT_LOWER, Line: 1, Column: 1, Lexeme: "bad"}
	var errs []*diagnostics.DiagnosticError
	ok := checkTypeName("bad", tok, &errs)
	if ok {
		t.Fatal("expected checkTypeName to return false for lowercase name")
	}
	if len(errs) == 0 {
		t.Fatal("expected A008 error")
	}
	if errs[0].Code != diagnostics.ErrA008 {
		t.Errorf("expected A008, got %s", errs[0].Code)
	}
}

// ---------------------------------------------------------------------------
// Error recovery — analyzer should report multiple errors
// ---------------------------------------------------------------------------

func TestRecovery_MultipleAnalyzerErrors(t *testing.T) {
	// Two independent errors: type mismatch + undefined variable
	input := `
fun f() -> Int { "hello" }
y = unknownVar
`
	errs := analyzeSource(input)
	if len(errs) < 2 {
		t.Fatalf("expected at least 2 errors, got %d: %v", len(errs), errs)
	}
}

// ---------------------------------------------------------------------------
// Positive controls — valid code should produce no errors
// ---------------------------------------------------------------------------

func TestValid_SimpleFunction(t *testing.T) {
	expectNoAnalyzerErrors(t, `fun f(x: Int) -> Int { x + 1 }`)
}

func TestValid_PatternMatch(t *testing.T) {
	expectNoAnalyzerErrors(t, `
fun f(x: Bool) -> Int {
    match x {
        true -> 1
        false -> 0
    }
}
`)
}

func TestValid_TypeAlias(t *testing.T) {
	expectNoAnalyzerErrors(t, `
type alias Pair = (Int, String)
`)
}

func TestValid_Constant(t *testing.T) {
	expectNoAnalyzerErrors(t, `
x :- 42
`)
}
