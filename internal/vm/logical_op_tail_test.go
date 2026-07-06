package vm

import (
	"testing"

	"github.com/funvibe/funxy/internal/evaluator"
)

// TestLogicalOpInTailPosition verifies that && and || work correctly when the
// LEFT operand is a function call and the whole logical expression is in tail
// position (e.g. the last expression of a named function body).
//
// This was a bug where the compiler kept inTailPosition=true while compiling the
// left operand, so a call like `returnsFalse() || true` emitted OP_TAIL_CALL for
// `returnsFalse()` and returned from the enclosing function immediately (with
// false), skipping the || entirely.
//
// See: compileLogicalOp must set inTailPosition=false before compiling the left
// operand. The right operand may keep the tail position (it is the result on the
// non-short-circuit path), which also preserves TCO through &&/||.
func TestLogicalOpInTailPosition(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name: "call || true in named fun",
			input: `
				fun returnsFalse() -> Bool { false }
				fun f() -> Bool { returnsFalse() || true }
				f()
			`,
			expected: true,
		},
		{
			name: "call || false in named fun",
			input: `
				fun returnsFalse() -> Bool { false }
				fun f() -> Bool { returnsFalse() || false }
				f()
			`,
			expected: false,
		},
		{
			name: "call && true in named fun",
			input: `
				fun returnsTrue() -> Bool { true }
				fun f() -> Bool { returnsTrue() && true }
				f()
			`,
			expected: true,
		},
		{
			name: "call && false in named fun",
			input: `
				fun returnsTrue() -> Bool { true }
				fun f() -> Bool { returnsTrue() && false }
				f()
			`,
			expected: false,
		},
		{
			name: "call || true in lambda",
			input: `
				returnsFalse = fun() { false }
				f = fun() { returnsFalse() || true }
				f()
			`,
			expected: true,
		},
		{
			name: "left short-circuit skips right (||)",
			input: `
				fun boom() -> Bool { panic("should not run") }
				fun f() -> Bool { true || boom() }
				f()
			`,
			expected: true,
		},
		{
			name: "left short-circuit skips right (&&)",
			input: `
				fun boom() -> Bool { panic("should not run") }
				fun f() -> Bool { false && boom() }
				f()
			`,
			expected: false,
		},
		{
			name: "TCO through || right operand does not overflow",
			input: `
				fun loop(n: Int) -> Bool {
					if n <= 0 { true } else { false || loop(n - 1) }
				}
				loop(1000000)
			`,
			expected: true,
		},
		{
			name: "TCO through && right operand does not overflow",
			input: `
				fun loop(n: Int) -> Bool {
					if n <= 0 { true } else { true && loop(n - 1) }
				}
				loop(1000000)
			`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVM(t, tt.input)

			switch exp := tt.expected.(type) {
			case bool:
				b, ok := result.(*evaluator.Boolean)
				if !ok {
					t.Fatalf("expected Boolean, got %T (%s)", result, result.Inspect())
				}
				if b.Value != exp {
					t.Errorf("expected %v, got %v", exp, b.Value)
				}
			case int64:
				i, ok := result.(*evaluator.Integer)
				if !ok {
					t.Fatalf("expected Integer, got %T (%s)", result, result.Inspect())
				}
				if i.Value != exp {
					t.Errorf("expected %d, got %d", exp, i.Value)
				}
			}
		})
	}
}

// TestInterpolationAndRangeCallInTailPosition covers two more instances of the
// same class of bug, using only Bool/Int/String (no Option/Result prelude):
//   - string interpolation whose last part is a function call
//   - a range bound that is a function call (inside a comprehension)
//
// The ?? and ? operators belong to the same class but need the Option/Result
// prelude, so they are covered end-to-end in
// tests/unit/bugs/tail_call_value_consuming_test.lang instead.
func TestInterpolationAndRangeCallInTailPosition(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "string interpolation with call in last part",
			input: `
				fun n() -> Int { 3 }
				fun f() -> String { "v=${n()}" }
				f()
			`,
			expected: `"v=3"`,
		},
		{
			name: "range end is a call inside comprehension",
			input: `
				fun n() -> Int { 3 }
				fun f() -> List<Int> { [x | x <- 1..n()] }
				f()
			`,
			expected: "[1, 2, 3]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVM(t, tt.input)
			if result.Inspect() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result.Inspect())
			}
		})
	}
}
