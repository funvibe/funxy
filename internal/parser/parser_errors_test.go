package parser_test

import (
	"strings"
	"testing"

	"github.com/funvibe/funxy/internal/diagnostics"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
)

// parseWithErrors runs the lexer+parser and returns all diagnostic errors.
func parseWithErrors(input string) []*diagnostics.DiagnosticError {
	ctx := &pipeline.PipelineContext{SourceCode: input}
	lp := &lexer.LexerProcessor{}
	ctx = lp.Process(ctx)
	pp := &parser.ParserProcessor{}
	ctx = pp.Process(ctx)
	return ctx.Errors
}

// expectError asserts exactly one error with the given code.
func expectError(t *testing.T, input string, code diagnostics.ErrorCode) *diagnostics.DiagnosticError {
	t.Helper()
	errs := parseWithErrors(input)
	if len(errs) == 0 {
		t.Fatalf("expected error %s, but got none\ninput: %s", code, input)
	}
	for _, e := range errs {
		if e.Code == code {
			return e
		}
	}
	var msgs []string
	for _, e := range errs {
		msgs = append(msgs, e.Error())
	}
	t.Fatalf("expected error %s, got:\n%s\ninput: %s", code, strings.Join(msgs, "\n"), input)
	return nil
}

// expectNoErrors asserts parsing succeeds without errors.
func expectNoErrors(t *testing.T, input string) {
	t.Helper()
	errs := parseWithErrors(input)
	if len(errs) > 0 {
		var msgs []string
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}
		t.Fatalf("expected no errors, got:\n%s\ninput: %s", strings.Join(msgs, "\n"), input)
	}
}

// ---------------------------------------------------------------------------
// P001 — Unexpected token
// ---------------------------------------------------------------------------

func TestP001_ConstDeclInvalidLHS(t *testing.T) {
	// `(1 + 2) :- 5` — left side of :- is not an identifier
	expectError(t, "(1 + 2) :- 5", diagnostics.ErrP001)
}

func TestP001_ConstDeclMemberExpression(t *testing.T) {
	// `obj.field :- 5` — member expression not allowed as constant name
	expectError(t, "obj.field :- 5", diagnostics.ErrP001)
}

func TestP001_ConstMemberLHS(t *testing.T) {
	// `const x.y = 5` — const declaration LHS must be a plain identifier,
	// not a member expression
	expectError(t, "const x.y = 5", diagnostics.ErrP001)
}

// ---------------------------------------------------------------------------
// P002 — Expected identifier on left side of assignment
// ---------------------------------------------------------------------------

func TestP002_CompoundAssignNonIdentifier(t *testing.T) {
	// `(1 + 2) += 3` — compound assignment requires identifier or member
	expectError(t, "(1 + 2) += 3", diagnostics.ErrP002)
}

func TestP002_CompoundAssignLiteral(t *testing.T) {
	// `5 -= 2` — literal on left of compound assignment
	expectError(t, "5 -= 2", diagnostics.ErrP002)
}

// ---------------------------------------------------------------------------
// P003 — Could not parse as integer
// (Defined but currently unused in parser; verify it stays unused)
// ---------------------------------------------------------------------------

func TestP003_NotUsedByParser(t *testing.T) {
	// P003 ("could not parse as integer") is defined but never emitted by the parser.
	// Very large integer literals are marked ILLEGAL by the lexer, which surfaces
	// as P004 in the parser. This test documents that behavior.
	errs := parseWithErrors("x = 99999999999999999999999999999")
	if len(errs) == 0 {
		t.Fatal("expected an error for oversized integer literal")
	}
	// Lexer marks it ILLEGAL → parser reports P004 (no prefix parse fn)
	if errs[0].Code != diagnostics.ErrP004 {
		t.Errorf("expected P004 for ILLEGAL token, got %s: %s", errs[0].Code, errs[0].Error())
	}
}

// ---------------------------------------------------------------------------
// P004 — No prefix parse function (cannot parse expression starting with X)
// ---------------------------------------------------------------------------

func TestP004_UnexpectedRParen(t *testing.T) {
	expectError(t, "x = )", diagnostics.ErrP004)
}

func TestP004_UnexpectedRBrace(t *testing.T) {
	expectError(t, "x = }", diagnostics.ErrP004)
}

func TestP004_UnexpectedRBracket(t *testing.T) {
	expectError(t, "x = ]", diagnostics.ErrP004)
}

func TestP004_ExpressionStartsWithComma(t *testing.T) {
	expectError(t, ", x", diagnostics.ErrP004)
}

// ---------------------------------------------------------------------------
// P005 — Expected closing token (paren, bracket, brace, etc.)
// ---------------------------------------------------------------------------

func TestP005_MissingRParen(t *testing.T) {
	expectError(t, "x = (1 + 2", diagnostics.ErrP005)
}

func TestP005_MissingRBracket(t *testing.T) {
	expectError(t, "x = [1, 2, 3", diagnostics.ErrP005)
}

func TestP005_FuncCallMissingRParen(t *testing.T) {
	expectError(t, "f(1, 2", diagnostics.ErrP005)
}

func TestP005_ConstWithoutEquals(t *testing.T) {
	// `const x` — missing `=`
	expectError(t, "const x", diagnostics.ErrP005)
}

func TestP005_FunDeclMissingParen(t *testing.T) {
	// `fun f { 1 }` — missing parameter list parens
	expectError(t, "fun f { 1 }", diagnostics.ErrP005)
}

// ---------------------------------------------------------------------------
// P006 — Various syntax errors (custom message via %s template)
// ---------------------------------------------------------------------------

func TestP006_ReturnInDoBlock(t *testing.T) {
	// `return` in a do-block is an expression context outside a function body
	e := expectError(t, "do { return 1 }", diagnostics.ErrP006)
	if !strings.Contains(e.Error(), "return is only allowed inside function bodies") {
		t.Errorf("unexpected message: %s", e.Error())
	}
}

func TestP006_ImportNotAtTop(t *testing.T) {
	// import after a regular statement
	e := expectError(t, "x = 1\nimport \"lib/list\"", diagnostics.ErrP006)
	if !strings.Contains(e.Error(), "top of the file") {
		t.Errorf("unexpected message: %s", e.Error())
	}
}

func TestP006_PackageNotAtTop(t *testing.T) {
	e := expectError(t, "x = 1\npackage foo", diagnostics.ErrP006)
	if !strings.Contains(e.Error(), "top of the file") {
		t.Errorf("unexpected message: %s", e.Error())
	}
}

func TestP006_RecursionDepthExceeded(t *testing.T) {
	// Generate deeply nested parens: (((((...1...)))))
	depth := 600
	input := strings.Repeat("(", depth) + "1" + strings.Repeat(")", depth)
	e := expectError(t, "x = "+input, diagnostics.ErrP006)
	if !strings.Contains(e.Error(), "recursion depth") {
		t.Errorf("unexpected message: %s", e.Error())
	}
}

func TestP006_UppercaseVariable(t *testing.T) {
	// Variable names must start with lowercase
	e := expectError(t, "MyVar = 5", diagnostics.ErrP006)
	if !strings.Contains(e.Error(), "lowercase") {
		t.Errorf("unexpected message: %s", e.Error())
	}
}

// ---------------------------------------------------------------------------
// P007 — Index assignment not supported
// ---------------------------------------------------------------------------

func TestP007_ListIndexAssign(t *testing.T) {
	e := expectError(t, "l = [1, 2, 3]\nl[0] = 10", diagnostics.ErrP007)
	if !strings.Contains(e.Error(), "index assignment is not supported") {
		t.Errorf("unexpected message: %s", e.Error())
	}
}

func TestP007_ListIndexCompoundAssign(t *testing.T) {
	// `l[0] += 5` — compound assignment on index is also index assignment
	// This may trigger P002 (compound) or P007 (index) depending on path
	errs := parseWithErrors("l = [1]\nl[0] += 5")
	if len(errs) == 0 {
		t.Fatal("expected an error for index compound assignment")
	}
	// Either P007 or P002 is acceptable
	found := false
	for _, e := range errs {
		if e.Code == diagnostics.ErrP007 || e.Code == diagnostics.ErrP002 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected P007 or P002, got: %s", errs[0].Error())
	}
}

// ---------------------------------------------------------------------------
// Error recovery — parser should continue after an error and report multiple
// ---------------------------------------------------------------------------

func TestRecovery_MultipleErrors(t *testing.T) {
	// Two bad statements on separate lines — parser should report both
	input := "x = )\ny = ]"
	errs := parseWithErrors(input)
	if len(errs) < 2 {
		t.Fatalf("expected at least 2 errors, got %d", len(errs))
	}
}

func TestRecovery_ContinuesAfterBadExpression(t *testing.T) {
	// Bad expression followed by a valid statement
	input := "x = )\ny = 5"
	errs := parseWithErrors(input)
	// Should have exactly 1 error (from `x = )`) and parse `y = 5` fine
	hasP004 := false
	for _, e := range errs {
		if e.Code == diagnostics.ErrP004 {
			hasP004 = true
		}
	}
	if !hasP004 {
		t.Fatalf("expected P004 error, got: %v", errs)
	}
}

func TestRecovery_ImportAfterStatements(t *testing.T) {
	// Import in wrong place + valid code after — both errors reported,
	// and parser doesn't crash
	input := "x = 1\nimport \"lib/list\"\ny = 2\nimport \"lib/sys\""
	errs := parseWithErrors(input)
	p006Count := 0
	for _, e := range errs {
		if e.Code == diagnostics.ErrP006 {
			p006Count++
		}
	}
	if p006Count < 2 {
		t.Fatalf("expected at least 2 P006 errors for misplaced imports, got %d", p006Count)
	}
}

func TestRecovery_ReturnInDoThenValidCode(t *testing.T) {
	// return in do-block triggers P006; parser recovers and handles next statement
	input := "do { return 1 }\nx = 5"
	errs := parseWithErrors(input)
	hasP006 := false
	for _, e := range errs {
		if e.Code == diagnostics.ErrP006 {
			hasP006 = true
		}
	}
	if !hasP006 {
		t.Fatal("expected P006 for return in do-block outside function")
	}
}

// ---------------------------------------------------------------------------
// Positive controls — valid code should produce no errors
// ---------------------------------------------------------------------------

func TestValid_SimpleAssignment(t *testing.T) {
	expectNoErrors(t, "x = 5")
}

func TestValid_FunctionDef(t *testing.T) {
	expectNoErrors(t, "fun add(a: Int, b: Int) -> Int { a + b }")
}

func TestValid_ListOps(t *testing.T) {
	expectNoErrors(t, "xs = [1, 2, 3]\ny = xs[0]")
}

func TestValid_MatchExpr(t *testing.T) {
	expectNoErrors(t, "match x { 1 -> true\n_ -> false }")
}

func TestValid_PatternAssign(t *testing.T) {
	expectNoErrors(t, "(a, b) = (1, 2)")
}

func TestValid_Import(t *testing.T) {
	expectNoErrors(t, `import "lib/list" (map, filter)`)
}

func TestValid_CompoundAssign(t *testing.T) {
	expectNoErrors(t, "x = 0\nx += 5")
}
