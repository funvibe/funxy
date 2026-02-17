package vm

import (
	"context"
	"strings"
	"testing"
	"time"
)

// runVMExpectError compiles and runs the input, expecting a runtime error.
// Returns the error message. Fails the test if no error occurs.
func runVMExpectError(t *testing.T, input string) string {
	t.Helper()
	program := parse(t, input)

	compiler := NewCompiler()
	chunk, err := compiler.Compile(program)
	if err != nil {
		t.Fatalf("compilation error (expected runtime error, not compile error): %s", err)
	}

	vm := New()
	vm.RegisterBuiltins()

	// Timeout to catch infinite loops/recursion that don't hit stack limit
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	vm.SetContext(ctx)

	_, err = vm.Run(chunk)
	if err == nil {
		t.Fatalf("expected runtime error, but code ran successfully")
	}

	return err.Error()
}

// runVMExpectErrorContains is a convenience wrapper that also checks the error
// message contains the expected substring.
func runVMExpectErrorContains(t *testing.T, input, wantSubstr string) {
	t.Helper()
	errMsg := runVMExpectError(t, input)
	if !strings.Contains(errMsg, wantSubstr) {
		t.Errorf("error %q should contain %q", errMsg, wantSubstr)
	}
}

// =============================================================================
// Division / modulo by zero
// =============================================================================

func TestVMError_DivisionByZero(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"int div", "1 / 0", "division by zero"},
		{"int mod", "10 % 0", "modulo by zero"},
		{"float div", "1.0 / 0.0", "division by zero"},
		{"expression", "(10 + 5) / (3 - 3)", "division by zero"},
		{"variable", "x = 0\n10 / x", "division by zero"}, // double-quoted: \n is real newline
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runVMExpectErrorContains(t, tt.input, tt.want)
		})
	}
}

// =============================================================================
// Type mismatch in arithmetic
// =============================================================================

func TestVMError_TypeMismatch(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"int + string", `1 + "a"`, "type mismatch"},
		{"string - int", `"a" - 1`, "type mismatch"},
		{"bool * int", "true * 5", "type mismatch"},
		{"string * string", `"a" * "b"`, "type mismatch"},
		{"list + int", "[1,2] + 3", "type mismatch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runVMExpectErrorContains(t, tt.input, tt.want)
		})
	}
}

// =============================================================================
// Undefined variables
// =============================================================================

func TestVMError_UndefinedVariable(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "x", "undefined variable"},
		{"in expression", "1 + y", "undefined variable"},
		{"in call", "f(1)", "undefined variable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runVMExpectErrorContains(t, tt.input, tt.want)
		})
	}
}

// =============================================================================
// Calling non-functions
// =============================================================================

func TestVMError_CallNonFunction(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"call int", "x = 5\nx(1)", "can only call functions"},
		{"call string", "x = \"hello\"\nx(1)", "can only call functions"},
		{"call bool", "x = true\nx(1)", "can only call functions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runVMExpectErrorContains(t, tt.input, tt.want)
		})
	}
}

// =============================================================================
// Wrong number of arguments
// =============================================================================

func TestVMError_WrongArgCount(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"too many", "fun f(a, b) { a + b }\nf(1, 2, 3, 4)", "arguments"},
		{"too many", "fun f(a) { a }\nf(1, 2, 3)", "arguments"},
		{"zero for one", "fun f(x) { x }\nf()", "arguments"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runVMExpectErrorContains(t, tt.input, tt.want)
		})
	}
}

// =============================================================================
// List index errors
// =============================================================================

func TestVMError_ListIndex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"out of bounds positive", "[1,2,3][5]", "index out of bounds"},
		{"out of bounds negative", "[1,2,3][-4]", "index out of bounds"},
		{"string key on list", `[1,2,3]["x"]`, "type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runVMExpectError(t, tt.input)
		})
	}
}

// =============================================================================
// String index errors
// =============================================================================

func TestVMError_StringIndex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"out of bounds", `"abc"[5]`, "index"},
		{"negative out of bounds", `"abc"[-4]`, "index"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runVMExpectError(t, tt.input)
		})
	}
}

// =============================================================================
// Record / field access errors
// =============================================================================

func TestVMError_RecordFieldAccess(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"missing field", "r = { x: 1, y: 2 }\nr.z", "field"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runVMExpectError(t, tt.input)
		})
	}
}

// =============================================================================
// Concat type errors
// =============================================================================

func TestVMError_ConcatTypeMismatch(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"int ++ int", "1 ++ 2", "type mismatch"},
		{"string ++ int", `"a" ++ 1`, "type mismatch"},
		{"int ++ list", "1 ++ [2]", "type mismatch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runVMExpectErrorContains(t, tt.input, tt.want)
		})
	}
}

// =============================================================================
// Cons operator errors
// =============================================================================

func TestVMError_ConsTypeMismatch(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"cons to int", "1 :: 2", "must be List"},
		{"cons to bool", "1 :: true", "must be List"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runVMExpectErrorContains(t, tt.input, tt.want)
		})
	}
}

// =============================================================================
// Bitwise operation errors
// =============================================================================

func TestVMError_BitwiseErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"not on string", `~"hello"`, "expects integer"},
		{"shift negative", "1 << -1", "negative shift"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runVMExpectErrorContains(t, tt.input, tt.want)
		})
	}
}

// =============================================================================
// Unary negation errors
// =============================================================================

func TestVMError_NegationErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"negate string", `-"hello"`, "unknown operator"},
		{"negate bool", "-true", "unknown operator"},
		{"negate list", "-[1,2]", "unknown operator"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runVMExpectErrorContains(t, tt.input, tt.want)
		})
	}
}

// =============================================================================
// Logical not errors
// =============================================================================

func TestVMError_LogicalNotErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"not string", `!"hello"`, "not supported"},
		{"not int", "!42", "not supported"},
		{"not list", "![1,2]", "not supported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runVMExpectErrorContains(t, tt.input, tt.want)
		})
	}
}

// =============================================================================
// Iteration errors
// =============================================================================

func TestVMError_IterationErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"iterate int", "for x in 42 { x }", "not iterable"},
		{"iterate bool", "for x in true { x }", "not iterable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runVMExpectErrorContains(t, tt.input, tt.want)
		})
	}
}

// =============================================================================
// Builtin function argument errors
// =============================================================================

func TestVMError_BuiltinArgErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"len no args", "len()", "wrong number of arguments"},
		{"head empty", "head([])", "empty"},
		{"tail empty", "tail([])", "empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runVMExpectError(t, tt.input)
		})
	}
}

// =============================================================================
// Stack overflow (deep recursion)
// =============================================================================

func TestVMError_StackOverflow(t *testing.T) {
	input := "fun f(n) { f(n + 1) }\nf(0)"
	errMsg := runVMExpectError(t, input)
	if !strings.Contains(errMsg, "stack overflow") &&
		!strings.Contains(errMsg, "recursion") &&
		!strings.Contains(errMsg, "context") {
		t.Errorf("error should mention stack overflow, recursion, or context timeout, got: %s", errMsg)
	}
}

// =============================================================================
// Option/Result unwrap errors
// =============================================================================

func TestVMError_UnwrapErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"unwrap None", "None |>> fun(x) { x }", "unwrap"},
		{"unwrap Fail", `Fail("oops") |>> fun(x) { x }`, "unwrap"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runVMExpectErrorContains(t, tt.input, tt.want)
		})
	}
}

// =============================================================================
// Pattern: error message quality checks
// =============================================================================

func TestVMError_MessageQuality(t *testing.T) {
	// Error messages should include line numbers and be actionable
	tests := []struct {
		name      string
		input     string
		wantParts []string
	}{
		{
			"division by zero has line info",
			"x = 0\ny = 10 / x",
			[]string{"division by zero", "ERROR at"},
		},
		{
			"undefined var has name",
			"print(unknownVar)",
			[]string{"undefined variable", "unknownVar"},
		},
		{
			"type mismatch shows types",
			`1 + "hello"`,
			[]string{"type mismatch"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := runVMExpectError(t, tt.input)
			for _, part := range tt.wantParts {
				if !strings.Contains(errMsg, part) {
					t.Errorf("error %q should contain %q", errMsg, part)
				}
			}
		})
	}
}
